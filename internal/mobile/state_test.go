package mobile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveRunOverwritesState(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	st := RunState{RunID: "run-1", Status: StatusRunning}
	if err := store.SaveRun(st); err != nil {
		t.Fatal(err)
	}
	st.Status = StatusFinished
	if err := store.SaveRun(st); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRun("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusFinished {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestWithRunLockRecoversStaleLock(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "artifacts"))
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	dir := store.RunDir("run-1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, "lock")
	if err := os.WriteFile(lock, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-staleLockAge - time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := store.WithRunLock("run-1", func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("lock callback not called")
	}
}

func TestAppCacheRequiresFutureExpiry(t *testing.T) {
	now := time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC)
	if AppCacheReusable(AppRef{AppURL: "bs://old"}, now) {
		t.Fatal("cache without expiry should not be reusable")
	}
	ref := NormalizeAppCacheRef(AppRef{AppURL: "bs://app", SHA256: "sha"}, now)
	if ref.UploadedAt.IsZero() || ref.ExpiresAt.IsZero() {
		t.Fatalf("cache times were not populated: %#v", ref)
	}
	if !AppCacheReusable(ref, now.Add(time.Hour)) {
		t.Fatal("fresh cache should be reusable")
	}
	if AppCacheReusable(ref, now.Add(AppCacheReuseWindow+time.Hour)) {
		t.Fatal("expired cache should not be reusable")
	}
}
