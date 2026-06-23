package mobile

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	LocalIdentifier string    `json:"local_identifier"`
	LogPath         string    `json:"log_path,omitempty"`
	StartedAt       time.Time `json:"started_at"`
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
}

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
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
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
		LocalIdentifier: identifier,
		LogPath:         logPath,
		StartedAt:       time.Now().UTC(),
		Deadline:        time.Now().UTC().Add(req.HoldFor),
		Owner:           "efp-mobile",
		Status:          "running",
	}
	if err := m.Save(state); err != nil {
		cleanupStartedTunnel(cmd, localConfigPath)
		return state, err
	}
	exited := m.watchTunnelExit(cmd, state)
	select {
	case err := <-exited:
		_ = os.Remove(localConfigPath)
		state.Status = "exited"
		_ = m.Save(state)
		msg := "BrowserStack Local exited during startup"
		if err != nil {
			msg += ": " + err.Error()
		}
		return state, NewError("local_tunnel_start_failed", msg, "Inspect tunnel.log for BrowserStack Local diagnostics.", 500)
	case <-time.After(300 * time.Millisecond):
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

func localBinaryArgs(configPath, identifier string, cfg config.MobileLocalConfig) []string {
	args := []string{"--config-file", configPath, "--local-identifier", identifier}
	if cfg.ForceLocal != nil && *cfg.ForceLocal {
		args = append(args, "--force-local")
	}
	if len(cfg.IncludeHosts) > 0 {
		args = append(args, "--include-hosts", strings.Join(cfg.IncludeHosts, ","))
	}
	if len(cfg.ExcludeHosts) > 0 {
		args = append(args, "--exclude-hosts", strings.Join(cfg.ExcludeHosts, ","))
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
	return st, nil
}

func (m *TunnelManager) CleanupOrphans() ([]TunnelState, error) {
	runs, err := m.Store.ListRuns()
	if err != nil {
		return nil, err
	}
	var stopped []TunnelState
	for _, run := range runs {
		if run.Status == StatusRunning || run.Status == StatusWaitingForHuman || run.Network.LocalIdentifier == "" {
			continue
		}
		st, err := m.Load(run.RunID, run.Network.LocalIdentifier)
		if err != nil || !st.Managed || st.Owner != "efp-mobile" || st.Status == "stopped" || st.Status == "exited" {
			continue
		}
		st, _ = m.Stop(st)
		stopped = append(stopped, st)
	}
	return stopped, nil
}

func (m *TunnelManager) tunnelPath(runID, identifier string) string {
	return filepath.Join(m.Store.RunDir(firstNonEmpty(runID, "tunnel-"+identifier)), "tunnel.json")
}
