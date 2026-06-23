package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/config"
)

type TunnelState struct {
	Version         int       `json:"version"`
	TunnelID        string    `json:"tunnel_id"`
	RunID           string    `json:"run_id,omitempty"`
	Managed         bool      `json:"managed"`
	PID             int       `json:"pid,omitempty"`
	BinaryPath      string    `json:"binary_path,omitempty"`
	LocalIdentifier string    `json:"local_identifier"`
	LogPath         string    `json:"log_path,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	ReadyAt         time.Time `json:"ready_at,omitempty"`
	Deadline        time.Time `json:"deadline,omitempty"`
	Owner           string    `json:"owner"`
	Status          string    `json:"status"`
}

type TunnelManager struct {
	Store       *StateStore
	Config      config.MobileLocalConfig
	Credentials Credentials
}

type TunnelStartRequest struct {
	RunID           string
	NetworkMode     string
	LocalIdentifier string
	HoldFor         time.Duration
	ReadyTimeout    time.Duration
}

var localErrorMarkers = []string{"[error]", "could not connect", "failed to", "invalid auth", "authentication failed"}

func (m *TunnelManager) Start(req TunnelStartRequest) (TunnelState, error) {
	if req.NetworkMode == "" || req.NetworkMode == "public" {
		return TunnelState{Managed: false, LocalIdentifier: "", Status: "not_required"}, nil
	}
	if req.NetworkMode == "private-external" {
		if strings.TrimSpace(req.LocalIdentifier) == "" {
			return TunnelState{}, NewError("local_tunnel_missing", "private-external requires --local-identifier", "Start BrowserStack Local outside this CLI and pass its identifier.", 400)
		}
		return TunnelState{Version: 1, TunnelID: "external-" + req.LocalIdentifier, Managed: false, LocalIdentifier: req.LocalIdentifier, Status: "external"}, nil
	}
	if req.NetworkMode != "private-managed" {
		return TunnelState{}, NewError("invalid_args", "--network must be public, private-managed, or private-external", "Use public unless the app needs private/internal hosts.", 400)
	}
	if strings.TrimSpace(m.Credentials.AccessKey) == "" {
		return TunnelState{}, NewError("auth_error", "BrowserStack access key is required to start Local", "Set BROWSERSTACK_ACCESS_KEY.", 401)
	}
	bin := firstNonEmpty(os.Getenv(m.Config.BinaryEnv), m.Config.Binary)
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return TunnelState{}, NewError("local_binary_not_found", "BrowserStack Local binary was not found", "Set BROWSERSTACK_LOCAL_BINARY or mobile.browserstack.local.binary, or put BrowserStackLocal on PATH.", 404)
	}
	identifier := req.LocalIdentifier
	if identifier == "" {
		identifier = "efp-" + shortRandom() + "-" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	effectiveRunID := firstNonEmpty(req.RunID, "tunnel-"+identifier)
	runDir := m.Store.RunDir(effectiveRunID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return TunnelState{}, err
	}
	logPath := filepath.Join(runDir, "tunnel.log")
	localConfigPath := filepath.Join(runDir, "browserstack-local.yml")
	if err := writeLocalBinaryConfig(localConfigPath, m.Credentials); err != nil {
		return TunnelState{}, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		_ = os.Remove(localConfigPath)
		return TunnelState{}, err
	}
	args := localBinaryArgs(localConfigPath, identifier, m.Config)
	cmd := exec.Command(resolved, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "BROWSERSTACK_LOCAL_IDENTIFIER="+identifier)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.Remove(localConfigPath)
		return TunnelState{}, NewError("local_tunnel_start_failed", "BrowserStack Local failed to start", "Inspect the sanitized tunnel log and binary permissions.", 500)
	}
	_ = logFile.Close()
	state := TunnelState{
		Version:         1,
		TunnelID:        "managed-" + identifier,
		RunID:           effectiveRunID,
		Managed:         true,
		PID:             cmd.Process.Pid,
		BinaryPath:      resolved,
		LocalIdentifier: identifier,
		LogPath:         logPath,
		StartedAt:       time.Now().UTC(),
		Deadline:        time.Now().UTC().Add(req.HoldFor),
		Owner:           "efp-mobile",
		Status:          "starting",
	}
	if err := m.Save(state); err != nil {
		cleanupStartedTunnel(cmd, localConfigPath)
		return state, err
	}
	exited := m.watchTunnelExit(cmd, state)
	readyTimeout := req.ReadyTimeout
	if readyTimeout <= 0 && m.Config.ReadyTimeoutSeconds > 0 {
		readyTimeout = time.Duration(m.Config.ReadyTimeoutSeconds) * time.Second
	}
	if readyTimeout <= 0 {
		readyTimeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), readyTimeout)
	defer cancel()
	state, err = m.WaitReady(ctx, state, exited)
	if err != nil {
		terminateStartedTunnel(cmd, exited)
		_ = os.Remove(localConfigPath)
		if state.Status == "starting" {
			state.Status = "start_timeout"
		}
		_ = m.Save(state)
		return state, err
	}
	_ = os.Remove(localConfigPath)
	return state, nil
}

func cleanupStartedTunnel(cmd *exec.Cmd, localConfigPath string) {
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = os.Remove(localConfigPath)
}

func (m *TunnelManager) watchTunnelExit(cmd *exec.Cmd, state TunnelState) <-chan error {
	exited := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		m.markExitedIfRunning(state)
		exited <- err
	}()
	return exited
}

func (m *TunnelManager) WaitReady(ctx context.Context, state TunnelState, exited <-chan error) (TunnelState, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready, failed := inspectLocalReadyLog(state.LogPath); failed != "" {
			state.Status = "exited"
			_ = m.Save(state)
			return state, NewError("local_tunnel_start_failed", "BrowserStack Local failed during startup", "Inspect tunnel.log for BrowserStack Local diagnostics.", 500)
		} else if ready {
			if state.PID != 0 && !processRunning(state.PID) {
				state.Status = "exited"
				_ = m.Save(state)
				return state, NewError("local_tunnel_start_failed", "BrowserStack Local exited before it became reusable", "Inspect tunnel.log for BrowserStack Local diagnostics.", 500)
			}
			state.Status = "running"
			state.ReadyAt = time.Now().UTC()
			_ = m.Save(state)
			return state, nil
		}
		select {
		case err := <-exited:
			state.Status = "exited"
			_ = m.Save(state)
			msg := "BrowserStack Local exited during startup"
			if err != nil {
				msg += ": " + err.Error()
			}
			return state, NewError("local_tunnel_start_failed", msg, "Inspect tunnel.log for BrowserStack Local diagnostics.", 500)
		case <-ctx.Done():
			state.Status = "start_timeout"
			_ = m.Save(state)
			return state, RetryableError("local_tunnel_not_ready", "timed out waiting for BrowserStack Local readiness", "Inspect tunnel.log, credentials, proxy, and private network reachability.", "retry", 504)
		case <-ticker.C:
		}
	}
}

func inspectLocalReadyLog(path string) (bool, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, ""
	}
	lastReady := false
	lastFailure := ""
	for _, line := range strings.Split(strings.ToLower(string(b)), "\n") {
		if strings.Contains(line, "you can now access your local server") {
			lastReady = true
			lastFailure = ""
		}
		for _, marker := range localErrorMarkers {
			if strings.Contains(line, marker) {
				lastReady = false
				lastFailure = marker
			}
		}
	}
	return lastReady, lastFailure
}

func terminateStartedTunnel(cmd *exec.Cmd, exited <-chan error) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
	}
}

func localBinaryArgs(configPath, identifier string, cfg config.MobileLocalConfig) []string {
	args := []string{"--config-file", configPath, "--local-identifier", identifier, "--enable-logging-for-api"}
	if cfg.ForceLocal != nil && *cfg.ForceLocal {
		args = append(args, "--force-local")
	}
	if len(cfg.IncludeHosts) > 0 {
		args = append(args, "--include-hosts")
		args = append(args, cfg.IncludeHosts...)
	}
	if len(cfg.ExcludeHosts) > 0 {
		args = append(args, "--exclude-hosts")
		args = append(args, cfg.ExcludeHosts...)
	}
	return args
}

func writeLocalBinaryConfig(path string, creds Credentials) error {
	return os.WriteFile(path, localBinaryConfig(creds), 0o600)
}

func localBinaryConfig(creds Credentials) []byte {
	var b bytes.Buffer
	b.WriteString("key: ")
	b.WriteString(strconv.Quote(creds.AccessKey))
	b.WriteByte('\n')
	return b.Bytes()
}

func (m *TunnelManager) Save(st TunnelState) error {
	path := m.tunnelPath(st.RunID, st.LocalIdentifier)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicWriteJSON(path, st, 0o600)
}

func (m *TunnelManager) markExitedIfRunning(st TunnelState) {
	current, err := m.Load(st.RunID, st.LocalIdentifier)
	if err != nil || current.Status != "running" {
		return
	}
	st.Status = "exited"
	_ = m.Save(st)
}

func (m *TunnelManager) Load(runID, identifier string) (TunnelState, error) {
	var st TunnelState
	b, err := os.ReadFile(m.tunnelPath(runID, identifier))
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(b, &st)
}

func (m *TunnelManager) Stop(st TunnelState) (TunnelState, error) {
	if !st.Managed || st.PID == 0 {
		st.Status = "external_or_not_required"
		return st, nil
	}
	if !processRunning(st.PID) {
		st.Status = "exited"
		_ = m.Save(st)
		return st, nil
	}
	if !processOwnedByTunnel(st) {
		return st, NewError("local_tunnel_ownership_mismatch", "managed tunnel process ownership could not be verified", "Refusing to kill the PID because it may have been reused. Inspect tunnel status and stop BrowserStack Local manually if needed.", 409)
	}
	p, err := os.FindProcess(st.PID)
	if err == nil {
		_ = p.Kill()
	}
	st.Status = "stopped"
	_ = m.Save(st)
	return st, nil
}

func (m *TunnelManager) Status(runID, identifier string) (TunnelState, error) {
	st, err := m.Load(runID, identifier)
	if err != nil {
		return TunnelState{}, err
	}
	if st.Managed && st.PID != 0 && st.Status == "" {
		st.Status = "running"
	}
	if st.Managed && (st.Status == "running" || st.Status == "starting") && st.PID != 0 && !processRunning(st.PID) {
		st.Status = "exited"
		_ = m.Save(st)
		return st, nil
	}
	if st.Status == "running" && tunnelDeadlineExpired(st, time.Now().UTC()) {
		st.Status = "expired"
		_ = m.Save(st)
		return st, nil
	}
	if st.Managed && (st.Status == "running" || st.Status == "starting") && st.PID != 0 && !processOwnedByTunnel(st) {
		st.Status = "ownership_unverified"
		_ = m.Save(st)
		return st, nil
	}
	if st.Managed && st.Status == "running" {
		if _, failed := inspectLocalReadyLog(st.LogPath); failed != "" {
			st.Status = "connection_failed"
			_ = m.Save(st)
			return st, nil
		}
	}
	return st, nil
}

func TunnelReusable(st TunnelState, now time.Time) bool {
	return st.Status == "running" && !tunnelDeadlineExpired(st, now) && st.Managed && processRunning(st.PID) && processOwnedByTunnel(st)
}

func tunnelDeadlineExpired(st TunnelState, now time.Time) bool {
	return !st.Deadline.IsZero() && now.After(st.Deadline)
}

func (m *TunnelManager) CleanupOrphans() ([]TunnelState, error) {
	tunnels, err := m.listTunnels()
	if err != nil {
		return nil, err
	}
	var stopped []TunnelState
	for _, st := range tunnels {
		if !m.orphanedTunnel(st) {
			continue
		}
		stoppedState, err := m.Stop(st)
		if err != nil {
			st.Status = "ownership_unverified"
			_ = m.Save(st)
			stopped = append(stopped, st)
			continue
		}
		stopped = append(stopped, stoppedState)
	}
	return stopped, nil
}

func (m *TunnelManager) listTunnels() ([]TunnelState, error) {
	runsDir := filepath.Join(m.Store.RootDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []TunnelState
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(runsDir, entry.Name(), "tunnel.json"))
		if err != nil {
			continue
		}
		var st TunnelState
		if err := json.Unmarshal(b, &st); err == nil {
			out = append(out, st)
		}
	}
	return out, nil
}

func (m *TunnelManager) orphanedTunnel(st TunnelState) bool {
	if !st.Managed || st.Owner != "efp-mobile" || st.Status == "stopped" || st.Status == "exited" {
		return false
	}
	if tunnelDeadlineExpired(st, time.Now().UTC()) {
		return true
	}
	run, err := m.Store.LoadRun(st.RunID)
	if err != nil {
		return true
	}
	if run.Status == StatusRunning || run.Status == StatusWaitingForHuman {
		return run.Network.LocalIdentifier != st.LocalIdentifier
	}
	return true
}

func (m *TunnelManager) tunnelPath(runID, identifier string) string {
	return filepath.Join(m.Store.RunDir(firstNonEmpty(runID, "tunnel-"+identifier)), "tunnel.json")
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		return err == nil && strings.Contains(string(out), strconv.Itoa(pid))
	}
	return exec.Command("kill", "-0", strconv.Itoa(pid)).Run() == nil
}

func processOwnedByTunnel(st TunnelState) bool {
	if st.PID <= 0 || strings.TrimSpace(st.BinaryPath) == "" {
		return false
	}
	path, ok := processExecutablePath(st.PID)
	if !ok {
		return false
	}
	return sameExecutablePath(path, st.BinaryPath)
}

func processExecutablePath(pid int) (string, bool) {
	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId=%d\").ExecutablePath", pid)
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return "", false
		}
		path := strings.TrimSpace(string(out))
		return path, path != ""
	case "darwin":
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		if err != nil {
			return "", false
		}
		path := strings.TrimSpace(string(out))
		return path, path != ""
	default:
		path, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		if err != nil {
			return "", false
		}
		return path, true
	}
}

func sameExecutablePath(a, b string) bool {
	a = cleanExecutablePath(a)
	b = cleanExecutablePath(b)
	return strings.EqualFold(a, b)
}

func cleanExecutablePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}
