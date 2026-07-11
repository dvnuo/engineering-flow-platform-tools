package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PersistentOpenResult describes a URL opened in a browser that remains
// available after the short-lived CLI process exits.
type PersistentOpenResult struct {
	Persistent        bool     `json:"persistent"`
	KeepOpenRequested bool     `json:"keep_open_requested"`
	BrowserAlive      bool     `json:"browser_alive"`
	Session           Session  `json:"session"`
	Reused            bool     `json:"reused"`
	Target            Target   `json:"target"`
	NextCommands      []string `json:"next_commands"`
}

type persistentOpenOps interface {
	EnsureSession(context.Context, StartOptions) (Session, bool, error)
	OpenTab(context.Context, string, string) (TabResult, error)
}

type lockedPersistentSessionManager struct{ manager *Manager }

func (m lockedPersistentSessionManager) EnsureSession(ctx context.Context, opts StartOptions) (Session, bool, error) {
	return m.manager.ensureSessionUnlocked(ctx, opts)
}

func (m lockedPersistentSessionManager) OpenTab(ctx context.Context, name, rawURL string) (TabResult, error) {
	return m.manager.openTabUnlocked(ctx, name, rawURL)
}

// OpenPersistent starts a managed browser at rawURL when needed. When the
// session already runs, it always opens rawURL in a new tab.
func (m *Manager) OpenPersistent(ctx context.Context, opts StartOptions) (PersistentOpenResult, error) {
	if err := m.ensureStore(); err != nil {
		return PersistentOpenResult{}, err
	}
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return PersistentOpenResult{}, err
	}
	rawURL := strings.TrimSpace(opts.URL)
	if rawURL == "" {
		return PersistentOpenResult{}, invalidArgs("--url is required", "Pass an HTTP or HTTPS URL to keep open in a persistent browser session.")
	}
	if err := validateHTTPURL(rawURL, "--url"); err != nil {
		return PersistentOpenResult{}, err
	}
	release, err := m.acquireSessionLock(ctx, name, 75*time.Second)
	if err != nil {
		return PersistentOpenResult{}, err
	}
	defer release()
	opts.Name = name
	return openPersistent(ctx, lockedPersistentSessionManager{manager: m}, opts)
}

func openPersistent(ctx context.Context, manager persistentOpenOps, opts StartOptions) (PersistentOpenResult, error) {
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return PersistentOpenResult{}, err
	}
	rawURL := strings.TrimSpace(opts.URL)
	if rawURL == "" {
		return PersistentOpenResult{}, invalidArgs("--url is required", "Pass an HTTP or HTTPS URL to keep open in a persistent browser session.")
	}
	if err := validateHTTPURL(rawURL, "--url"); err != nil {
		return PersistentOpenResult{}, err
	}

	// Session creation is lifecycle-only. The requested URL is always opened
	// here through DevTools so browser open owns the exact-target contract.
	opts.Name = name
	opts.URL = ""
	session, reused, err := manager.EnsureSession(ctx, opts)
	if err != nil {
		return PersistentOpenResult{}, err
	}
	tab, err := manager.OpenTab(ctx, name, rawURL)
	if err != nil {
		return PersistentOpenResult{}, err
	}
	session.ActiveTargetID = tab.Tab.ID

	return PersistentOpenResult{
		Persistent:        true,
		KeepOpenRequested: true,
		BrowserAlive:      session.Alive,
		Session:           session,
		Reused:            reused,
		Target:            tab.Tab,
		NextCommands:      persistentOpenNextCommands(name),
	}, nil
}

func isSessionNotFound(err error) bool {
	var automationErr *Error
	return errors.As(err, &automationErr) && automationErr.Code == "session_not_found"
}

func persistentOpenNextCommands(sessionName string) []string {
	return []string{
		fmt.Sprintf("browser session status %s --json", sessionName),
		fmt.Sprintf("browser tab list --session %s --json", sessionName),
		fmt.Sprintf("browser tab current --session %s --json", sessionName),
		fmt.Sprintf("browser page snapshot --session %s --json", sessionName),
		fmt.Sprintf("browser page ax --session %s --json", sessionName),
	}
}
