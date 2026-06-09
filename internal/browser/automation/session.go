package automation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browser/probe"
)

type Manager struct {
	Store  *Store
	Client *DevToolsClient
	Now    func() time.Time
}

func DefaultManager() (*Manager, error) {
	store, err := DefaultStore()
	if err != nil {
		return nil, err
	}
	return NewManager(store, nil), nil
}

func NewManager(store *Store, client *DevToolsClient) *Manager {
	return &Manager{
		Store:  store,
		Client: client,
		Now:    func() time.Time { return time.Now().UTC() },
	}
}

func (m *Manager) Start(ctx context.Context, opts StartOptions) (Session, error) {
	if err := m.ensureStore(); err != nil {
		return Session{}, err
	}
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return Session{}, err
	}
	if strings.TrimSpace(opts.URL) != "" {
		if err := validateHTTPURL(opts.URL, "--url"); err != nil {
			return Session{}, err
		}
	}
	if opts.Port < 0 || opts.Port > 65535 {
		return Session{}, invalidArgs("--port must be between 0 and 65535", "Use --port 0 to pick a free local DevTools port.")
	}
	if existing, err := m.Store.Load(name); err == nil {
		refreshed := m.Refresh(ctx, existing)
		if refreshed.Alive {
			return refreshed, nil
		}
	}

	profileDir := strings.TrimSpace(opts.ProfileDir)
	if profileDir == "" {
		var err error
		profileDir, err = DefaultProfileDir(name)
		if err != nil {
			return Session{}, err
		}
	}
	profileDir, err := ValidateProfileDir(profileDir)
	if err != nil {
		return Session{}, err
	}
	if opts.CleanProfile {
		if err := os.RemoveAll(profileDir); err != nil {
			return Session{}, NewError("artifact_write_failed", err.Error(), "Dedicated browser profile could not be cleaned.", 500)
		}
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return Session{}, NewError("artifact_write_failed", err.Error(), "Dedicated browser profile could not be created.", 500)
	}
	downloadDir := strings.TrimSpace(opts.DownloadDir)
	if downloadDir == "" {
		downloadDir, err = DefaultDownloadDir(name)
		if err != nil {
			return Session{}, err
		}
	}
	downloadDir, err = ValidateDownloadDir(downloadDir)
	if err != nil {
		return Session{}, err
	}
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return Session{}, NewError("artifact_write_failed", err.Error(), "Dedicated browser download directory could not be created.", 500)
	}
	if err := ensureDownloadPreferences(profileDir, downloadDir); err != nil {
		return Session{}, err
	}

	browserPath, err := probe.FindBrowser(defaultBrowserName(opts.Browser), opts.BrowserExe)
	if err != nil {
		return Session{}, mapProbeError(err)
	}
	port := opts.Port
	if port == 0 {
		port, err = freeLocalPort()
		if err != nil {
			return Session{}, NewError("devtools_unavailable", err.Error(), "Choose a DevTools port with --port.", 500)
		}
	}

	devNull, closeNull := openDevNull()
	defer closeNull()
	cmd, err := startBrowserProcess(browserPath, browserArgs(profileDir, port, opts.Headless, opts.URL), devNull)
	if err != nil {
		return Session{}, NewError("browser_launch_failed", err.Error(), "Check --browser-exe and whether the browser can be launched.", 500)
	}
	client := NewDevToolsClient(LocalDebugAddr, port)
	version, err := waitForDevTools(ctx, client, 20*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return Session{}, err
	}
	_ = configureBrowserDownloadBehavior(ctx, version.WebSocketDebuggerURL, downloadDir)
	_ = cmd.Process.Release()

	now := m.now()
	session := Session{
		Name:                name,
		BrowserPath:         browserPath,
		ProfileDir:          profileDir,
		DownloadDir:         downloadDir,
		DebugAddr:           LocalDebugAddr,
		DebugPort:           port,
		BrowserWebSocketURL: version.WebSocketDebuggerURL,
		PID:                 cmd.Process.Pid,
		CreatedAt:           now,
		LastSeenAt:          now,
		Alive:               true,
	}
	if err := m.Store.Save(session); err != nil {
		return Session{}, err
	}
	session.MetadataPath, _ = m.Store.MetadataPath(session.Name)
	return session, nil
}

func (m *Manager) List(ctx context.Context) ([]Session, error) {
	if err := m.ensureStore(); err != nil {
		return nil, err
	}
	sessions, err := m.Store.List()
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		sessions[i] = m.Refresh(ctx, sessions[i])
	}
	return sessions, nil
}

func (m *Manager) Status(ctx context.Context, name string) (Session, error) {
	if err := m.ensureStore(); err != nil {
		return Session{}, err
	}
	session, err := m.Store.Load(defaultSessionName(name))
	if err != nil {
		return Session{}, err
	}
	return m.Refresh(ctx, session), nil
}

func (m *Manager) Stop(ctx context.Context, opts StopOptions) (Session, error) {
	if err := m.ensureStore(); err != nil {
		return Session{}, err
	}
	name := defaultSessionName(opts.Name)
	session, err := m.Store.Load(name)
	if err != nil {
		return Session{}, err
	}
	session = m.Refresh(ctx, session)
	if !session.Alive {
		if !opts.KeepMetadata {
			if err := m.Store.Remove(name); err != nil {
				return session, err
			}
			session.MetadataPath = ""
		}
		return session, nil
	}
	if session.PID <= 0 {
		session.Alive = false
		session.BrowserWebSocketURL = ""
		if !opts.KeepMetadata {
			if err := m.Store.Remove(name); err != nil {
				return session, err
			}
			session.MetadataPath = ""
		} else if err := m.Store.Save(session); err != nil {
			return session, err
		}
		return session, nil
	}
	process, err := os.FindProcess(session.PID)
	if err != nil {
		return Session{}, NewError("automation_failed", err.Error(), "The stored browser process could not be opened.", 500)
	}
	if err := stopProcess(process); err != nil && !processAlreadyDone(err) {
		return Session{}, NewError("automation_failed", err.Error(), "Browser process could not be stopped.", 500)
	}
	session.Alive = false
	session.BrowserWebSocketURL = ""
	if !opts.KeepMetadata {
		if err := m.Store.Remove(name); err != nil {
			return session, err
		}
		session.MetadataPath = ""
	} else if err := m.Store.Save(session); err != nil {
		return session, err
	}
	return session, nil
}

func (m *Manager) Attach(ctx context.Context, opts AttachOptions) (Session, error) {
	if err := m.ensureStore(); err != nil {
		return Session{}, err
	}
	name := defaultSessionName(opts.Name)
	if err := ValidateSessionName(name); err != nil {
		return Session{}, err
	}
	addr := strings.TrimSpace(opts.DebugAddr)
	if addr == "" {
		addr = LocalDebugAddr
	}
	if addr != LocalDebugAddr {
		return Session{}, invalidArgs("--debug-addr must be 127.0.0.1", "Attach only to an explicitly exposed local DevTools endpoint.")
	}
	if opts.DebugPort <= 0 || opts.DebugPort > 65535 {
		return Session{}, invalidArgs("--debug-port must be between 1 and 65535", "Launch Chrome/Edge with --remote-debugging-port=<port>, then pass that port explicitly.")
	}
	client := NewDevToolsClient(addr, opts.DebugPort)
	version, err := client.Version(ctx)
	if err != nil {
		return Session{}, err
	}
	now := m.now()
	session := Session{
		Name:                name,
		DebugAddr:           addr,
		DebugPort:           opts.DebugPort,
		BrowserWebSocketURL: version.WebSocketDebuggerURL,
		CreatedAt:           now,
		LastSeenAt:          now,
		Alive:               strings.TrimSpace(version.WebSocketDebuggerURL) != "",
	}
	if err := m.Store.Save(session); err != nil {
		return Session{}, err
	}
	session.MetadataPath, _ = m.Store.MetadataPath(session.Name)
	return session, nil
}

func (m *Manager) Discover(ctx context.Context, opts DiscoverOptions) ([]DiscoveredSession, error) {
	addr := strings.TrimSpace(opts.DebugAddr)
	if addr == "" {
		addr = LocalDebugAddr
	}
	if addr != LocalDebugAddr {
		return nil, invalidArgs("--debug-addr must be 127.0.0.1", "Discovery is intentionally limited to explicit local DevTools endpoints.")
	}
	ports := sanitizeDiscoverPorts(opts.Ports)
	if len(ports) == 0 {
		ports = []int{9222, 9223, 9224}
	}
	out := make([]DiscoveredSession, 0, len(ports))
	now := m.now()
	for _, port := range ports {
		client := NewDevToolsClient(addr, port)
		item := DiscoveredSession{DebugAddr: addr, DebugPort: port, CheckedAt: now}
		version, err := client.Version(ctx)
		if err == nil {
			item.Alive = strings.TrimSpace(version.WebSocketDebuggerURL) != ""
			item.Browser = RedactString(version.Browser)
			item.ProtocolVersion = RedactString(version.ProtocolVersion)
			item.BrowserWebSocketURL = version.WebSocketDebuggerURL
			if targets, listErr := client.ListTargets(ctx); listErr == nil {
				for _, target := range PageTargets(targets) {
					item.Targets = append(item.Targets, RedactedTarget(target))
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (m *Manager) Refresh(ctx context.Context, session Session) Session {
	client := NewDevToolsClient(session.DebugAddr, session.DebugPort)
	version, err := refreshDevToolsVersion(ctx, client)
	if err != nil {
		session.Alive = false
		session.BrowserWebSocketURL = ""
		_ = m.Store.Save(session)
		return session
	}
	session.Alive = true
	session.BrowserWebSocketURL = version.WebSocketDebuggerURL
	session.LastSeenAt = m.now()
	_ = m.Store.Save(session)
	return session
}

func refreshDevToolsVersion(ctx context.Context, client *DevToolsClient) (VersionInfo, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		version, err := client.Version(ctx)
		if err == nil && strings.TrimSpace(version.WebSocketDebuggerURL) != "" {
			return version, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = NewError("devtools_unavailable", "DevTools endpoint did not return a browser WebSocket URL.", "Check whether the browser session is still running.", 503)
		}
		if attempt == 3 {
			break
		}
		select {
		case <-ctx.Done():
			return VersionInfo{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return VersionInfo{}, lastErr
}

func (m *Manager) RunningSession(ctx context.Context, name string) (Session, error) {
	session, err := m.Status(ctx, name)
	if err != nil {
		return Session{}, err
	}
	if !session.Alive || strings.TrimSpace(session.BrowserWebSocketURL) == "" {
		return Session{}, NewError("session_not_running", "Browser session is not running.", "Run browser session start --json, or restart the stored session.", 409)
	}
	return session, nil
}

func (m *Manager) ensureStore() error {
	if m.Store != nil {
		return nil
	}
	store, err := DefaultStore()
	if err != nil {
		return err
	}
	m.Store = store
	return nil
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

func defaultSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultSessionName
	}
	return name
}

func defaultBrowserName(browser string) string {
	browser = strings.ToLower(strings.TrimSpace(browser))
	if browser == "" {
		return "auto"
	}
	return browser
}

func browserArgs(profileDir string, port int, headless bool, initialURL string) []string {
	args := []string{
		"--remote-debugging-address=" + LocalDebugAddr,
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if headless {
		args = append(args, "--headless=new")
	}
	if strings.TrimSpace(initialURL) != "" {
		args = append(args, initialURL)
	}
	return args
}

func waitForDevTools(ctx context.Context, client *DevToolsClient, timeout time.Duration) (VersionInfo, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		version, err := client.Version(ctx)
		if err == nil && strings.TrimSpace(version.WebSocketDebuggerURL) != "" {
			return version, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return VersionInfo{}, NewError("timeout", ctx.Err().Error(), "Browser DevTools endpoint did not become available before the command was canceled.", 408)
		case <-time.After(150 * time.Millisecond):
		}
	}
	msg := "Browser DevTools endpoint did not become available."
	if lastErr != nil {
		msg += " " + lastErr.Error()
	}
	return VersionInfo{}, NewError("devtools_unavailable", msg, "Check browser policy and whether remote debugging is allowed.", 503)
}

func freeLocalPort() (int, error) {
	ln, err := net.Listen("tcp", LocalDebugAddr+":0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener did not return a TCP address")
	}
	return addr.Port, nil
}

func sanitizeDiscoverPorts(raw []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(raw))
	for _, port := range raw {
		if port <= 0 || port > 65535 || seen[port] {
			continue
		}
		seen[port] = true
		out = append(out, port)
	}
	return out
}

func openDevNull() (*os.File, func()) {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, func() {}
	}
	return f, func() { _ = f.Close() }
}

func stopProcess(process *os.Process) error {
	if runtime.GOOS != "windows" {
		if err := process.Signal(os.Interrupt); err == nil {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return process.Kill()
}

func processAlreadyDone(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "process already finished") || strings.Contains(msg, "no such process") || strings.Contains(msg, "os: process already finished")
}

func mapProbeError(err error) error {
	var probeErr *probe.ProbeError
	if errors.As(err, &probeErr) {
		return NewError(probeErr.Code, probeErr.Message, probeErr.Hint, probeErr.Status)
	}
	return NewError("automation_failed", err.Error(), "", 500)
}
