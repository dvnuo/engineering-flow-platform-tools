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

type persistentSessionManager interface {
	Status(context.Context, string) (Session, error)
	Start(context.Context, StartOptions) (Session, error)
	OpenTab(context.Context, string, string) (TabResult, error)
}

type lockedPersistentSessionManager struct{ manager *Manager }

func (m lockedPersistentSessionManager) Status(ctx context.Context, name string) (Session, error) {
	return m.manager.statusUnlocked(ctx, name)
}

func (m lockedPersistentSessionManager) Start(ctx context.Context, opts StartOptions) (Session, error) {
	return m.manager.startUnlocked(ctx, opts)
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

func openPersistent(ctx context.Context, manager persistentSessionManager, opts StartOptions) (PersistentOpenResult, error) {
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

	session, err := manager.Status(ctx, name)
	observedSession := session
	reused := err == nil && session.Alive && strings.TrimSpace(session.BrowserWebSocketURL) != ""
	if err != nil && !isSessionNotFound(err) {
		return PersistentOpenResult{}, err
	}
	var tab TabResult
	if reused {
		tab, err = manager.OpenTab(ctx, name, rawURL)
	} else {
		// Open through DevTools after startup so the returned target is always the
		// exact page created for this request, even when target creation or profile
		// restoration order differs across browsers.
		opts.Name = name
		opts.URL = ""
		session, err = manager.Start(ctx, opts)
		if err == nil {
			reused = sameSessionIdentity(observedSession, session)
			tab, err = manager.OpenTab(ctx, name, rawURL)
		}
	}
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

func sameSessionIdentity(before, after Session) bool {
	if before.Name == "" || after.Name == "" || before.Name != after.Name {
		return false
	}
	return before.PID == after.PID &&
		before.DebugAddr == after.DebugAddr &&
		before.DebugPort == after.DebugPort &&
		before.CreatedAt.Equal(after.CreatedAt)
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
