package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBrowserExplicitMissing(t *testing.T) {
	_, err := FindBrowser("auto", filepath.Join(t.TempDir(), "missing-browser"))
	if err == nil {
		t.Fatal("expected error")
	}
	probeErr, ok := err.(*ProbeError)
	if !ok || probeErr.Code != "browser_not_found" {
		t.Fatalf("err = %#v", err)
	}
}

func TestDefaultProfileDirNonEmpty(t *testing.T) {
	if DefaultProfileDir() == "" {
		t.Fatal("default profile dir is empty")
	}
}

func TestFindBrowserExplicitExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser")
	if err := os.WriteFile(path, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := FindBrowser("auto", path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}

func TestLooksLikeDefaultBrowserProfile(t *testing.T) {
	if !LooksLikeDefaultBrowserProfile("/home/user/.config/google-chrome") {
		t.Fatal("expected default Chrome profile to be rejected")
	}
	if LooksLikeDefaultBrowserProfile("/tmp/browser-probe-profile") {
		t.Fatal("dedicated probe profile should be allowed")
	}
}

func TestUnsafeProfileDir(t *testing.T) {
	if !UnsafeProfileDir("/") {
		t.Fatal("root profile path should be unsafe")
	}
	if UnsafeProfileDir(filepath.Join(t.TempDir(), "profile")) {
		t.Fatal("temp profile should be safe")
	}
}
