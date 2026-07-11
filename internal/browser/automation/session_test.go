package automation

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestStoreSaveLoadListDelete(t *testing.T) {
	store := NewStore(t.TempDir())
	created := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	session := Session{
		Name:       "default",
		DebugAddr:  LocalDebugAddr,
		DebugPort:  9222,
		CreatedAt:  created,
		LastSeenAt: created,
		Alive:      true,
	}
	if err := store.Save(session); err != nil {
		t.Fatal(err)
	}
	path, err := store.SessionPath("default")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "default.json" {
		t.Fatalf("metadata path = %s", path)
	}
	loaded, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "default" || loaded.MetadataPath != path {
		t.Fatalf("loaded session = %#v", loaded)
	}
	sessions, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Name != "default" {
		t.Fatalf("sessions = %#v", sessions)
	}
	if err := store.Delete("default"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("default"); err == nil {
		t.Fatal("expected deleted session to be missing")
	}
}

func TestStatusMarksStaleSessionAliveFalse(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	mgr := NewManager(store, nil)
	created := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	if err := store.Save(Session{
		Name:                "default",
		DebugAddr:           host,
		DebugPort:           port,
		BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/stale",
		CreatedAt:           created,
		LastSeenAt:          created,
		Alive:               true,
	}); err != nil {
		t.Fatal(err)
	}

	status, err := mgr.Status(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if status.Alive {
		t.Fatalf("stale session should not be alive: %#v", status)
	}
	if status.BrowserWebSocketURL != "" {
		t.Fatalf("stale websocket URL should be cleared: %#v", status)
	}
	reloaded, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Alive {
		t.Fatalf("stale status was not persisted: %#v", reloaded)
	}
}

func TestStatusRetriesTransientDevToolsVersionFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) < 3 {
			http.Error(w, "not yet", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/recovered"}`))
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	mgr := NewManager(store, nil)
	created := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	if err := store.Save(Session{
		Name:                "default",
		DebugAddr:           host,
		DebugPort:           port,
		BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/old",
		CreatedAt:           created,
		LastSeenAt:          created,
		Alive:               true,
	}); err != nil {
		t.Fatal(err)
	}

	status, err := mgr.Status(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Alive || status.BrowserWebSocketURL != "ws://127.0.0.1/devtools/browser/recovered" {
		t.Fatalf("session should recover after transient failures: %#v", status)
	}
	if calls.Load() < 3 {
		t.Fatalf("expected retries, calls=%d", calls.Load())
	}
	reloaded, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Alive || reloaded.BrowserWebSocketURL != status.BrowserWebSocketURL {
		t.Fatalf("recovered status was not persisted: %#v", reloaded)
	}
}

func TestStartURLCompatibilityDelegatesToPersistentOpen(t *testing.T) {
	var newTabCalls atomic.Int32
	var openedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/existing"}`))
		case "/json/new":
			newTabCalls.Add(1)
			openedURL, _ = url.QueryUnescape(r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"id":"page-new","type":"page","title":"Next","url":"https://example.test/next","webSocketDebuggerUrl":"ws://127.0.0.1/devtools/page/page-new"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	host, port := splitHostPort(t, srv.Listener.Addr().String())

	store := NewStore(t.TempDir())
	if err := store.Save(Session{
		Name:                "default",
		DebugAddr:           host,
		DebugPort:           port,
		BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing",
		CreatedAt:           time.Now().UTC(),
		Alive:               true,
		ActiveTargetID:      "page-old",
	}); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(store, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	session, err := mgr.Start(ctx, StartOptions{
		Name: "default",
		URL:  "https://example.test/next",
	})
	if err != nil {
		t.Fatal(err)
	}
	if newTabCalls.Load() != 1 || openedURL != "https://example.test/next" {
		t.Fatalf("new tab calls=%d opened URL=%q", newTabCalls.Load(), openedURL)
	}
	if session.ActiveTargetID != "page-new" {
		t.Fatalf("session = %#v", session)
	}
	reloaded, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ActiveTargetID != "page-new" {
		t.Fatalf("active target was not persisted: %#v", reloaded)
	}
}

func TestStartWithoutURLOnlyEnsuresExistingSession(t *testing.T) {
	for _, rawURL := range []string{"", "   "} {
		t.Run("url="+strconv.Quote(rawURL), func(t *testing.T) {
			var newTabCalls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/json/version":
					_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1/devtools/browser/existing"}`))
				case "/json/new":
					newTabCalls.Add(1)
					_, _ = w.Write([]byte(`{"id":"unexpected","type":"page","url":"https://example.test"}`))
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
				BrowserWebSocketURL: "ws://127.0.0.1/devtools/browser/existing",
				PID:                 1234,
				CreatedAt:           created,
				Alive:               true,
				ActiveTargetID:      "page-old",
			}); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			session, err := NewManager(store, nil).Start(ctx, StartOptions{Name: "default", URL: rawURL})
			if err != nil {
				t.Fatal(err)
			}
			if newTabCalls.Load() != 0 {
				t.Fatalf("session lifecycle start unexpectedly opened %d tabs", newTabCalls.Load())
			}
			if session.PID != 1234 || !session.CreatedAt.Equal(created) || session.ActiveTargetID != "page-old" {
				t.Fatalf("session identity changed: %#v", session)
			}
		})
	}
}

func TestSessionLockTimesOutAndReleases(t *testing.T) {
	store := NewStore(t.TempDir())
	mgr := NewManager(store, nil)
	release, err := mgr.acquireSessionLock(context.Background(), "default", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = mgr.acquireSessionLock(ctx, "default", time.Second)
	if err == nil {
		t.Fatal("expected busy lock acquisition to time out")
	}
	autoErr, ok := err.(*Error)
	if !ok || autoErr.Code != "session_busy" {
		t.Fatalf("busy error = %#v", err)
	}
	release()
	releaseAgain, err := mgr.acquireSessionLock(context.Background(), "default", time.Second)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	releaseAgain()
}

func TestSessionLockRemovesStaleFile(t *testing.T) {
	store := NewStore(t.TempDir())
	path, err := store.SessionLockPath("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-3 * time.Minute)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(store, nil)
	release, err := mgr.acquireSessionLock(context.Background(), "default", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestSessionLifecycleMutationsUseSharedLock(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *Manager) error
	}{
		{name: "start", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.Start(ctx, StartOptions{Name: "default"})
			return err
		}},
		{name: "start-with-url", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.Start(ctx, StartOptions{Name: "default", URL: "https://example.test"})
			return err
		}},
		{name: "stop", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.Stop(ctx, StopOptions{Name: "default"})
			return err
		}},
		{name: "attach", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.Attach(ctx, AttachOptions{Name: "default", DebugAddr: LocalDebugAddr, DebugPort: 9222})
			return err
		}},
		{name: "list", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.List(ctx)
			return err
		}},
		{name: "status", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.Status(ctx, "default")
			return err
		}},
		{name: "running-session", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.RunningSession(ctx, "default")
			return err
		}},
		{name: "tab-list", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.TabList(ctx, "default")
			return err
		}},
		{name: "tab-current", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.CurrentTab(ctx, "default")
			return err
		}},
		{name: "resolve-target", run: func(ctx context.Context, mgr *Manager) error {
			_, _, err := mgr.ResolveTarget(ctx, "default", "page-1")
			return err
		}},
		{name: "tab-open", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.OpenTab(ctx, "default", "https://example.test")
			return err
		}},
		{name: "tab-activate", run: func(ctx context.Context, mgr *Manager) error {
			_, err := mgr.ActivateTab(ctx, "default", "page-1")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewManager(NewStore(t.TempDir()), nil)
			if err := mgr.Store.Save(Session{
				Name:      "default",
				DebugAddr: LocalDebugAddr,
				DebugPort: 9222,
				CreatedAt: time.Now().UTC(),
				Alive:     true,
			}); err != nil {
				t.Fatal(err)
			}
			release, err := mgr.acquireSessionLock(context.Background(), "default", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer release()

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			err = tc.run(ctx, mgr)
			var automationErr *Error
			if !errors.As(err, &automationErr) || automationErr.Code != "session_busy" {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestValidateProfileDirRejectsRootsAndDefaultProfiles(t *testing.T) {
	if _, err := ValidateProfileDir("/"); err == nil {
		t.Fatal("filesystem root should be rejected")
	}
	if _, err := ValidateProfileDir("/home/user/.config/google-chrome"); err == nil {
		t.Fatal("default Chrome profile should be rejected")
	}
	profile := filepath.Join(t.TempDir(), "profile")
	got, err := ValidateProfileDir(profile)
	if err != nil {
		t.Fatal(err)
	}
	if got != profile {
		t.Fatalf("profile = %q want %q", got, profile)
	}
}

func TestDefaultBrowserNameUsesChrome(t *testing.T) {
	if got := defaultBrowserName(""); got != "chrome" {
		t.Fatalf("default browser = %q want chrome", got)
	}
	if got := defaultBrowserName("auto"); got != "auto" {
		t.Fatalf("explicit auto browser = %q want auto", got)
	}
}

func splitHostPort(t *testing.T, raw string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}
