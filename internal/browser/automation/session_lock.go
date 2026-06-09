package automation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	sessionLockPollInterval = 100 * time.Millisecond
	sessionLockMinStaleAge  = 2 * time.Minute
)

func (m *Manager) acquireSessionLock(ctx context.Context, sessionName string, commandTimeout time.Duration) (func(), error) {
	if err := m.ensureStore(); err != nil {
		return nil, err
	}
	path, err := m.Store.SessionLockPath(sessionName)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, NewError("artifact_write_failed", err.Error(), "Check permissions for browser session locks.", 500)
	}
	staleAfter := commandTimeout * 2
	if staleAfter < sessionLockMinStaleAge {
		staleAfter = sessionLockMinStaleAge
	}
	for {
		release, err := tryCreateSessionLock(path)
		if err == nil {
			return release, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, NewError("artifact_write_failed", err.Error(), "Browser session lock could not be created.", 500)
		}
		removed, err := removeStaleSessionLock(path, staleAfter)
		if err != nil {
			return nil, err
		}
		if removed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, NewError("session_busy", "Browser session is busy with another page command.", "Retry after the current browser command finishes, or run commands sequentially for the same --session.", 409)
		case <-time.After(sessionLockPollInterval):
		}
	}
}

func tryCreateSessionLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(f, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = f.Close()
			_ = os.Remove(path)
		})
	}, nil
}

func removeStaleSessionLock(path string, staleAfter time.Duration) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, NewError("automation_failed", err.Error(), "Browser session lock could not be inspected.", 500)
	}
	if time.Since(info.ModTime()) < staleAfter {
		return false, nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, NewError("artifact_write_failed", err.Error(), "Stale browser session lock could not be removed.", 500)
	}
	return true, nil
}
