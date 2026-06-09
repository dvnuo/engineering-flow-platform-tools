package automation

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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
