package automation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakePersistentSessionManager struct {
	ensureSession Session
	ensureReused  bool
	ensureErr     error
	tabResult     TabResult
	tabErr        error

	ensureCalls int
	openCalls   int
	ensureOpts  StartOptions
	openSession string
	openURL     string
}

func (f *fakePersistentSessionManager) EnsureSession(_ context.Context, opts StartOptions) (Session, bool, error) {
	f.ensureCalls++
	f.ensureOpts = opts
	return f.ensureSession, f.ensureReused, f.ensureErr
}

func (f *fakePersistentSessionManager) OpenTab(_ context.Context, sessionName, rawURL string) (TabResult, error) {
	f.openCalls++
	f.openSession = sessionName
	f.openURL = rawURL
	return f.tabResult, f.tabErr
}

func TestOpenPersistentStartsSessionThenOpensRequestedURL(t *testing.T) {
	fake := &fakePersistentSessionManager{
		ensureSession: Session{
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
	if fake.ensureCalls != 1 || fake.openCalls != 1 {
		t.Fatalf("calls ensure=%d open=%d", fake.ensureCalls, fake.openCalls)
	}
	if fake.ensureOpts.URL != "" || fake.ensureOpts.Name != "demo" || fake.ensureOpts.Browser != "edge" {
		t.Fatalf("ensure options = %#v", fake.ensureOpts)
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

func TestOpenPersistentPropagatesEnsureReusedState(t *testing.T) {
	recovered := Session{
		Name:                "default",
		DebugAddr:           "127.0.0.1",
		DebugPort:           9222,
		PID:                 1234,
		CreatedAt:           time.Now().UTC().Add(-time.Minute),
		Alive:               true,
		BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/recovered",
	}
	fake := &fakePersistentSessionManager{
		ensureSession: recovered,
		ensureReused:  true,
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

func TestManagerOpenPersistentReportsRecoveredStoredSessionAsReused(t *testing.T) {
	var newTabCalls atomic.Int32
	var openedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/recovered"}`))
		case "/json/new":
			newTabCalls.Add(1)
			openedURL, _ = url.QueryUnescape(r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"id":"page-recovered","type":"page","title":"Login","url":"https://example.test/login","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/page-recovered"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	created := time.Now().UTC().Add(-time.Minute)
	if err := store.Save(Session{
		Name:                "default",
		DebugAddr:           host,
		DebugPort:           port,
		BrowserWebSocketURL: "",
		PID:                 1234,
		CreatedAt:           created,
		Alive:               false,
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := NewManager(store, nil).OpenPersistent(ctx, StartOptions{
		Name: "default",
		URL:  "https://example.test/login",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reused || !result.BrowserAlive || result.Target.ID != "page-recovered" {
		t.Fatalf("recovered result = %#v", result)
	}
	if !result.Session.CreatedAt.Equal(created) || result.Session.PID != 1234 {
		t.Fatalf("recovered session identity changed: %#v", result.Session)
	}
	if newTabCalls.Load() != 1 || openedURL != "https://example.test/login" {
		t.Fatalf("new tab calls=%d opened URL=%q", newTabCalls.Load(), openedURL)
	}
}

func TestManagerOpenPersistentRejectsCorruptSessionMetadata(t *testing.T) {
	store := NewStore(t.TempDir())
	metadataPath, err := store.MetadataPath("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = NewManager(store, nil).OpenPersistent(context.Background(), StartOptions{
		Name:       "default",
		URL:        "https://example.test/login",
		BrowserExe: filepath.Join(t.TempDir(), "browser-that-must-not-be-resolved"),
	})
	if err == nil {
		t.Fatal("expected corrupt session metadata error")
	}
	automationErr, ok := err.(*Error)
	if !ok || automationErr.Code != "automation_failed" || !strings.Contains(automationErr.Hint, "not valid JSON") {
		t.Fatalf("error = %#v", err)
	}
}

func TestOpenPersistentReusesRunningSessionAndAlwaysOpensNewTab(t *testing.T) {
	fake := &fakePersistentSessionManager{
		ensureSession: Session{
			Name:                "default",
			Alive:               true,
			BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing",
			ActiveTargetID:      "page-old",
		},
		ensureReused: true,
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
	if fake.ensureCalls != 1 || fake.openCalls != 1 {
		t.Fatalf("ensure calls=%d open calls=%d", fake.ensureCalls, fake.openCalls)
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
	if fake.ensureCalls != 0 || fake.openCalls != 0 {
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
