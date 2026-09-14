package automation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

func (m lockedPersistentSessionManager) ListTabs(ctx context.Context, name string) (TabListResult, error) {
	return m.manager.tabListUnlocked(ctx, name)
}

func (m lockedPersistentSessionManager) ActivateTab(ctx context.Context, name, targetID string) (TabResult, error) {
	return m.manager.activateTabUnlocked(ctx, name, targetID)
}

// EnsurePersistentResult describes a managed session made ready for one page:
// the browser runs and a tab of the page's origin is in front.
type EnsurePersistentResult struct {
	Session   Session `json:"session"`
	Reused    bool    `json:"reused"`
	Target    Target  `json:"target"`
	TabOpened bool    `json:"tab_opened"`
}

type persistentEnsureOps interface {
	persistentOpenOps
	ListTabs(context.Context, string) (TabListResult, error)
	ActivateTab(context.Context, string, string) (TabResult, error)
}

// How long a freshly launched browser gets to list the tab it was started on
// before a tab is opened for it. Variables so tests can shorten the wait.
var (
	ensureTabWait = 5 * time.Second
	ensureTabPoll = 150 * time.Millisecond
)

// EnsurePersistent makes the managed session ready for opts.URL without adding
// tabs: a stopped browser is launched directly on the URL, so no New Tab page
// precedes it, and a running browser keeps its tabs while the first one at the
// URL's origin (else the tab already in front) is activated. A tab is opened
// only when the window has none. An empty URL only ensures the session. This
// is the bridge's start and reopen path; browser open keeps its contract of
// always opening the URL in a new tab.
func (m *Manager) EnsurePersistent(ctx context.Context, opts StartOptions) (EnsurePersistentResult, error) {
	if err := m.ensureStore(); err != nil {
		return EnsurePersistentResult{}, err
	}
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return EnsurePersistentResult{}, err
	}
	rawURL := strings.TrimSpace(opts.URL)
	if rawURL != "" {
		if err := validateHTTPURL(rawURL, "--url"); err != nil {
			return EnsurePersistentResult{}, err
		}
	}
	release, err := m.acquireSessionLock(ctx, name, 75*time.Second)
	if err != nil {
		return EnsurePersistentResult{}, err
	}
	defer release()
	opts.Name = name
	opts.URL = rawURL
	return ensurePersistent(ctx, lockedPersistentSessionManager{manager: m}, opts)
}

func ensurePersistent(ctx context.Context, manager persistentEnsureOps, opts StartOptions) (EnsurePersistentResult, error) {
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return EnsurePersistentResult{}, err
	}
	rawURL := strings.TrimSpace(opts.URL)
	if rawURL != "" {
		if err := validateHTTPURL(rawURL, "--url"); err != nil {
			return EnsurePersistentResult{}, err
		}
	}
	// Unlike openPersistent, the URL stays on the launch options: a browser
	// started on it shows the page as its only tab.
	opts.Name = name
	opts.URL = rawURL
	session, reused, err := manager.EnsureSession(ctx, opts)
	if err != nil {
		return EnsurePersistentResult{}, err
	}
	result := EnsurePersistentResult{Session: session, Reused: reused}
	if rawURL == "" {
		return result, nil
	}
	target, found, err := ensureTab(ctx, manager, name, rawURL, !reused)
	if err != nil {
		return EnsurePersistentResult{}, err
	}
	var tab TabResult
	if found {
		tab, err = manager.ActivateTab(ctx, name, target.ID)
	} else {
		tab, err = manager.OpenTab(ctx, name, rawURL)
		result.TabOpened = true
	}
	if err != nil {
		return EnsurePersistentResult{}, err
	}
	result.Session.ActiveTargetID = tab.Tab.ID
	result.Target = tab.Tab
	return result, nil
}

// ensureTab picks the tab to bring to the front for rawURL. A freshly launched
// browser (wait=true) lists the tab it was started on a moment after DevTools
// answers, sometimes behind a transient blank target, so it is given
// ensureTabWait to show a tab at the URL's origin before another tab (or, for
// an empty window, a newly opened one) is settled for.
func ensureTab(ctx context.Context, manager persistentEnsureOps, name, rawURL string, wait bool) (Target, bool, error) {
	deadline := time.Now().Add(ensureTabWait)
	for {
		list, err := manager.ListTabs(ctx, name)
		if err != nil {
			return Target{}, false, err
		}
		target, sameOrigin, found := pickEnsureTab(list.Tabs, rawURL)
		if sameOrigin || (found && !wait) {
			return target, true, nil
		}
		if !wait || time.Now().After(deadline) {
			return target, found, nil
		}
		select {
		case <-ctx.Done():
			return Target{}, false, ctx.Err()
		case <-time.After(ensureTabPoll):
		}
	}
}

// pickEnsureTab prefers the tab already in front when it shows the origin of
// rawURL, then any tab of that origin (sameOrigin reports either), then the
// tab in front (a page tab that is mid-redirect to a login provider still
// belongs to this page), then the first tab. found is false only for a window
// without page tabs.
func pickEnsureTab(tabs []Target, rawURL string) (target Target, sameOrigin, found bool) {
	origin := originOf(rawURL)
	var (
		matched, front     Target
		hasMatch, hasFront bool
	)
	for _, tab := range tabs {
		same := origin != "" && originOf(tab.URL) == origin
		if same && tab.Active {
			return tab, true, true
		}
		if same && !hasMatch {
			matched, hasMatch = tab, true
		}
		if tab.Active && !hasFront {
			front, hasFront = tab, true
		}
	}
	switch {
	case hasMatch:
		return matched, true, true
	case hasFront:
		return front, false, true
	case len(tabs) > 0:
		return tabs[0], false, true
	}
	return Target{}, false, false
}

// originOf reduces an http(s) URL to scheme://host[:port] with the default
// port dropped; anything else (about:blank, chrome://newtab/) is "".
func originOf(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return u.Scheme + "://" + host
}
