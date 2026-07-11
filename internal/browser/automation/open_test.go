package automation

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakePersistentSessionManager struct {
	statusSession Session
	statusErr     error
	startSession  Session
	startErr      error
	tabResult     TabResult
	tabErr        error

	statusCalls int
	startCalls  int
	openCalls   int
	startOpts   StartOptions
	openSession string
	openURL     string
}

func (f *fakePersistentSessionManager) Status(context.Context, string) (Session, error) {
	f.statusCalls++
	return f.statusSession, f.statusErr
}

func (f *fakePersistentSessionManager) Start(_ context.Context, opts StartOptions) (Session, error) {
	f.startCalls++
	f.startOpts = opts
	return f.startSession, f.startErr
}

func (f *fakePersistentSessionManager) OpenTab(_ context.Context, sessionName, rawURL string) (TabResult, error) {
	f.openCalls++
	f.openSession = sessionName
	f.openURL = rawURL
	return f.tabResult, f.tabErr
}

func TestOpenPersistentStartsSessionThenOpensRequestedURL(t *testing.T) {
	fake := &fakePersistentSessionManager{
		statusErr: NewError("session_not_found", "missing", "", 404),
		startSession: Session{
			Name:                "demo",
			Alive:               true,
			BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/new",
		},
		tabResult: TabResult{
			Session: "demo",
			Tab:     Target{ID: "page-new", Type: "page", URL: "https://example.test/login", Active: true},
		},
	}

	result, err := openPersistent(context.Background(), fake, StartOptions{
		Name:    "demo",
		URL:     "https://example.test/login",
		Browser: "edge",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.statusCalls != 1 || fake.startCalls != 1 || fake.openCalls != 1 {
		t.Fatalf("calls status=%d start=%d open=%d", fake.statusCalls, fake.startCalls, fake.openCalls)
	}
	if fake.startOpts.URL != "" || fake.startOpts.Name != "demo" || fake.startOpts.Browser != "edge" {
		t.Fatalf("start options = %#v", fake.startOpts)
	}
	if fake.openURL != "https://example.test/login" || fake.openSession != "demo" {
		t.Fatalf("open session=%q URL=%q", fake.openSession, fake.openURL)
	}
	if result.Reused || !result.Persistent || !result.KeepOpenRequested || !result.BrowserAlive {
		t.Fatalf("lifecycle result = %#v", result)
	}
	if result.Session.ActiveTargetID != "page-new" || result.Target.ID != "page-new" {
		t.Fatalf("target result = %#v", result)
	}
	if len(result.NextCommands) != 5 {
		t.Fatalf("next commands = %#v", result.NextCommands)
	}
	for _, command := range result.NextCommands {
		if command == "" || strings.Contains(command, "--target-id") {
			t.Fatalf("next command must resolve the post-handoff active tab: %q", command)
		}
	}
}

func TestOpenPersistentReportsRecoveredStoredSessionAsReused(t *testing.T) {
	created := time.Now().UTC().Add(-time.Minute)
	stored := Session{
		Name:      "default",
		DebugAddr: "127.0.0.1",
		DebugPort: 9222,
		PID:       1234,
		CreatedAt: created,
		Alive:     false,
	}
	recovered := stored
	recovered.Alive = true
	recovered.BrowserWebSocketURL = "ws://127.0.0.1/devtools/browser/recovered"
	fake := &fakePersistentSessionManager{
		statusSession: stored,
		startSession:  recovered,
		tabResult: TabResult{
			Session: "default",
			Tab:     Target{ID: "page-new", Type: "page", URL: "https://example.test", Active: true},
		},
	}

	result, err := openPersistent(context.Background(), fake, StartOptions{Name: "default", URL: "https://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reused {
		t.Fatalf("recovered stored session should be reported as reused: %#v", result)
	}
}

func TestOpenPersistentReusesRunningSessionAndAlwaysOpensNewTab(t *testing.T) {
	fake := &fakePersistentSessionManager{
		statusSession: Session{
			Name:                "default",
			Alive:               true,
			BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing",
			ActiveTargetID:      "page-old",
		},
		tabResult: TabResult{
			Session: "default",
			Tab:     Target{ID: "page-new", Type: "page", URL: "https://example.test/next", Active: true},
		},
	}

	result, err := openPersistent(context.Background(), fake, StartOptions{
		Name: "default",
		URL:  "https://example.test/next",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.startCalls != 0 || fake.openCalls != 1 {
		t.Fatalf("start calls=%d open calls=%d", fake.startCalls, fake.openCalls)
	}
	if !result.Reused || result.Target.ID != "page-new" || result.Session.ActiveTargetID != "page-new" {
		t.Fatalf("result = %#v", result)
	}
	if fake.openURL != "https://example.test/next" {
		t.Fatalf("opened URL = %q", fake.openURL)
	}
}

func TestOpenPersistentRejectsInvalidURLBeforeSessionLookup(t *testing.T) {
	fake := &fakePersistentSessionManager{}
	_, err := openPersistent(context.Background(), fake, StartOptions{URL: "file:///tmp/private"})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	automationErr, ok := err.(*Error)
	if !ok || automationErr.Code != "invalid_args" {
		t.Fatalf("error = %#v", err)
	}
	if fake.statusCalls != 0 || fake.startCalls != 0 || fake.openCalls != 0 {
		t.Fatalf("manager was called: %#v", fake)
	}
}

func TestManagerOpenPersistentUsesSessionLifecycleLock(t *testing.T) {
	mgr := NewManager(NewStore(t.TempDir()), nil)
	release, err := mgr.acquireSessionLock(context.Background(), "default", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = mgr.OpenPersistent(context.Background(), StartOptions{Name: "default", URL: "file:///tmp/private"})
	if automationErr, ok := err.(*Error); !ok || automationErr.Code != "invalid_args" {
		t.Fatalf("invalid URL should be rejected before waiting for the session lock: %#v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = mgr.OpenPersistent(ctx, StartOptions{Name: "default", URL: "https://example.test"})
	if err == nil {
		t.Fatal("expected session_busy while another lifecycle operation owns the lock")
	}
	automationErr, ok := err.(*Error)
	if !ok || automationErr.Code != "session_busy" {
		t.Fatalf("error = %#v", err)
	}
}
