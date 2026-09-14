package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

const (
	bridgeLaunchPingTimeout  = 2 * time.Second
	bridgeLaunchReadyTimeout = 5 * time.Second
	bridgeLaunchPollInterval = 200 * time.Millisecond
)

// bridgeLaunchRequest is the parsed efp-bridge://start?origin=...&port=... link.
type bridgeLaunchRequest struct {
	Origin  string
	Port    int
	Session string
}

// bridgeLaunchCmd is the hidden protocol-handler entry point registered by
// `browser serve --register-protocol`. Windows invokes it with the clicked
// efp-bridge:// URL; it starts `browser serve` unless a bridge already answers.
func bridgeLaunchCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:    "bridge-launch <url>",
		Short:  "Start the Portal local bridge from an efp-bridge:// link",
		Long:   "Hidden protocol-handler entry point: parse efp-bridge://start?origin=<urlencoded>&port=<n>, exit when a bridge already answers on the port, otherwise start browser serve as a detached background process.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBridgeLaunch(cmd, o, args[0])
		},
	}
}

// parseBridgeLaunchURL validates an efp-bridge://start link. origin is
// required; port (1024-65535) and session are optional.
func parseBridgeLaunchURL(raw string) (bridgeLaunchRequest, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u == nil || !strings.EqualFold(u.Scheme, "efp-bridge") {
		return bridgeLaunchRequest{}, automation.NewError("invalid_args", "bridge-launch expects an efp-bridge://start URL", "Example: efp-bridge://start?origin=https%3A%2F%2Fportal.example.com&port=8765", 400)
	}
	action := strings.ToLower(strings.Trim(u.Host, "/"))
	if action == "" {
		action = strings.ToLower(strings.Trim(u.Opaque, "/"))
	}
	if action == "" {
		action = strings.ToLower(strings.Trim(u.Path, "/"))
	}
	if action != "start" {
		return bridgeLaunchRequest{}, automation.NewError("invalid_args", "unsupported efp-bridge action: "+action, "Only efp-bridge://start is supported.", 400)
	}
	query := u.Query()
	origin, err := normalizeBridgeOrigin(query.Get("origin"))
	if err != nil {
		return bridgeLaunchRequest{}, automation.NewError("invalid_args", "efp-bridge link is missing a valid origin query parameter", "Example: efp-bridge://start?origin=https%3A%2F%2Fportal.example.com&port=8765", 400)
	}
	if origin == "*" {
		return bridgeLaunchRequest{}, automation.NewError("invalid_args", "efp-bridge link must name a single Portal origin", "Pass origin=https%3A%2F%2Fportal.example.com; * is not accepted from a link.", 400)
	}
	req := bridgeLaunchRequest{Origin: origin}
	if rawPort := strings.TrimSpace(query.Get("port")); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1024 || port > 65535 {
			return bridgeLaunchRequest{}, automation.NewError("invalid_args", "efp-bridge link port must be an integer between 1024 and 65535", "Example: efp-bridge://start?origin=https%3A%2F%2Fportal.example.com&port=8765", 400)
		}
		req.Port = port
	}
	if session := strings.TrimSpace(query.Get("session")); session != "" {
		if err := automation.ValidateSessionName(session); err != nil {
			return bridgeLaunchRequest{}, err
		}
		req.Session = session
	}
	return req, nil
}

func runBridgeLaunch(cmd *cobra.Command, o *Opts, raw string) error {
	// Nobody reads this command's stdout: the shell started it for a clicked
	// link and its console is hidden, so the outcome goes to the bridge log
	// where the Connectors panel's troubleshooting steps point.
	logBridgeLaunch("invoked (console hidden: %t)", consoleHidden)
	req, err := parseBridgeLaunchURL(raw)
	if err != nil {
		logBridgeLaunch("rejected %s: %v", raw, err)
		return printAutomationError(cmd, o, err)
	}
	settings, err := resolveServeSettings(o, serveOptions{Origin: req.Origin, Port: req.Port, Session: req.Session}, req.Port > 0)
	if err != nil {
		logBridgeLaunch("could not resolve settings for %s: %v", req.Origin, err)
		return printAutomationError(cmd, o, err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), bridgeLaunchReadyTimeout+bridgeLaunchPingTimeout)
	defer cancel()
	if port, ping, ok := findRunningBridge(ctx, settings.Port, bridgePortAttempts); ok {
		// A bridge serving another Portal answers /ping (which is header-less)
		// but rejects every call the page makes. Starting a second one would
		// land on the next port and be found second, so say what is wrong
		// instead of reporting success the member cannot act on.
		if running := bridgePingOrigin(ping); running != "" && running != settings.Origin && running != "*" {
			logBridgeLaunch("a bridge for %s already holds %s; refusing to start one for %s", running, bridgeListenURL(port), settings.Origin)
			return printAutomationError(cmd, o, automation.NewError(
				"bridge_origin_mismatch",
				fmt.Sprintf("A local bridge on %s is serving %s, not %s.", bridgeListenURL(port), running, settings.Origin),
				"Stop that bridge (close its window or end the browser serve process) and click Start bridge again.",
				409,
			))
		}
		logBridgeLaunch("a bridge already answers on %s; nothing to start", bridgeListenURL(port))
		return print(cmd, o, output.Success("", map[string]any{
			"already_running": true,
			"started":         false,
			"listening":       bridgeListenURL(port),
			"origin":          settings.Origin,
			"session":         settings.Session,
			"ping":            ping,
		}))
	}
	executable, err := bridgeExecutablePath()
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	args := []string{"serve", "--origin", settings.Origin, "--port", strconv.Itoa(settings.Port), "--session", settings.Session, "--json"}
	if strings.TrimSpace(o.Config) != "" {
		args = append(args, "--config", o.Config)
	}
	logPath, logFile := openBridgeLog()
	child, err := startDetachedBridgeProcess(executable, args, logFile)
	if logFile != nil {
		_ = logFile.Close()
	}
	if err != nil {
		logBridgeLaunch("could not start %s: %v", executable, err)
		return printAutomationError(cmd, o, automation.NewError("bridge_launch_failed", err.Error(), "Start the bridge manually with browser serve --origin <portal-origin>.", 500))
	}
	pid := child.Process.Pid
	logBridgeLaunch("started %s serve --origin %s --port %d --session %s (pid %d)", executable, settings.Origin, settings.Port, settings.Session, pid)
	_ = child.Process.Release()
	port, ping, ready := waitForBridge(ctx, settings.Port, bridgePortAttempts, bridgeLaunchReadyTimeout)
	data := map[string]any{
		"already_running": false,
		"started":         true,
		"ready":           ready,
		"pid":             pid,
		"origin":          settings.Origin,
		"session":         settings.Session,
		"log_path":        logPath,
	}
	if ready {
		data["listening"] = bridgeListenURL(port)
		data["ping"] = ping
	} else {
		data["listening"] = bridgeListenURL(settings.Port)
		data["hint"] = "The bridge did not answer /ping within 5 seconds; check log_path."
		logBridgeLaunch("pid %d did not answer /ping within %s", pid, bridgeLaunchReadyTimeout)
	}
	return print(cmd, o, output.Success("", data))
}

// logBridgeLaunch appends one line to the shared bridge log. It is best effort:
// a launch must never fail because the log could not be written.
func logBridgeLaunch(format string, args ...any) {
	_, file := openBridgeLog()
	if file == nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "bridge-launch: %s %s\n", time.Now().Format("2006/01/02 15:04:05"), fmt.Sprintf(format, args...))
}

func bridgeListenURL(port int) string {
	return fmt.Sprintf("http://%s:%d", automation.LocalDebugAddr, port)
}

// bridgePingOrigin reads the origin a running bridge reports. Bridges older
// than this build do not send one, which reads as "unknown" and is treated as
// a match so an upgrade is never required to start one.
func bridgePingOrigin(ping map[string]any) string {
	origin, _ := ping["origin"].(string)
	return strings.TrimSpace(origin)
}

// probeBridgePing reports whether a bridge answers GET /ping on port.
func probeBridgePing(ctx context.Context, port int) (map[string]any, bool) {
	ctx, cancel := context.WithTimeout(ctx, bridgeLaunchPingTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bridgeListenURL(port)+"/ping", nil)
	if err != nil {
		return nil, false
	}
	resp, err := (&http.Client{Timeout: bridgeLaunchPingTimeout}).Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil || !env.OK {
		return nil, false
	}
	if _, ok := env.Data["protocol_version"]; !ok {
		return nil, false
	}
	return env.Data, true
}

// findRunningBridge probes port..port+attempts-1, the same range the Portal page scans.
func findRunningBridge(ctx context.Context, port, attempts int) (int, map[string]any, bool) {
	for i := 0; i < attempts; i++ {
		candidate := port + i
		if candidate > 65535 {
			break
		}
		if ping, ok := probeBridgePing(ctx, candidate); ok {
			return candidate, ping, true
		}
		if ctx.Err() != nil {
			break
		}
	}
	return 0, nil, false
}

func waitForBridge(ctx context.Context, port, attempts int, timeout time.Duration) (int, map[string]any, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if found, ping, ok := findRunningBridge(ctx, port, attempts); ok {
			return found, ping, true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return 0, nil, false
		}
		select {
		case <-ctx.Done():
			return 0, nil, false
		case <-time.After(bridgeLaunchPollInterval):
		}
	}
}

// openBridgeLog opens ~/.efp/browser/logs/bridge-serve.log for the detached
// child's stdout/stderr. A nil file means the child output is discarded.
func openBridgeLog() (string, *os.File) {
	root, err := automation.DefaultBrowserHome()
	if err != nil {
		return "", nil
	}
	dir := filepath.Join(root, "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil
	}
	path := filepath.Join(dir, "bridge-serve.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil
	}
	return path, f
}

func newDetachedCommand(executable string, args []string, logFile *os.File) *exec.Cmd {
	cmd := exec.Command(executable, args...)
	cmd.Stdin = nil
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	return cmd
}
