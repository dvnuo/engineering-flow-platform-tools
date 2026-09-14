package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

const (
	bridgeShutdownTimeout = 2 * time.Second
	bridgeStartupTimeout  = 90 * time.Second
)

type serveOptions struct {
	Origin             string
	Port               int
	Session            string
	Browser            string
	BrowserExe         string
	Headless           bool
	URL                string
	RegisterProtocol   bool
	UnregisterProtocol bool
}

// serveSettings is the effective configuration after flags, the shared EFP
// config (browser.serve.*), and the EFP_BROWSER_SERVE_* env defaults merge.
type serveSettings struct {
	Origin  string
	Port    int
	Session string
	URL     string
}

func serveCmd(o *Opts) *cobra.Command {
	opts := serveOptions{Session: automation.DefaultSessionName, Browser: "chrome", Port: config.DefaultBrowserServePort}
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the EFP Portal local browser bridge on 127.0.0.1",
		Long:  "Run the loopback HTTP bridge (GET /ping, GET /commands, POST /run) that the EFP Portal local browser connector calls from the user's Portal tab. Commands run in-process against the managed browser session, one at a time per session, and answer with the same JSON envelope as the CLI. The bridge is started by install-bridge.cmd or the efp-bridge:// protocol link, not by interactive agents; agents should keep using browser open and the page commands directly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.RegisterProtocol && opts.UnregisterProtocol {
				return print(cmd, o, output.Failure("invalid_args", "--register-protocol and --unregister-protocol cannot be combined", "Pass only one of the protocol flags.", 400))
			}
			if opts.RegisterProtocol {
				return runRegisterProtocol(cmd, o, opts)
			}
			if opts.UnregisterProtocol {
				return runUnregisterProtocol(cmd, o)
			}
			return runServe(cmd, o, opts)
		},
	}
	c.Flags().StringVar(&opts.Origin, "origin", "", "Portal origin allowed to call the bridge, such as https://portal.example.com; * allows any origin. Defaults to browser.serve.allowed_origin or EFP_BROWSER_SERVE_ALLOWED_ORIGIN.")
	c.Flags().IntVar(&opts.Port, "port", config.DefaultBrowserServePort, "Loopback port to listen on; the next five ports are tried when it is busy. Defaults to browser.serve.port or EFP_BROWSER_SERVE_PORT.")
	c.Flags().StringVar(&opts.Session, "session", automation.DefaultSessionName, defaultSessionFlagUsage)
	c.Flags().StringVar(&opts.Browser, "browser", "chrome", "Browser family for a new session (chrome, edge, chromium, or auto).")
	c.Flags().StringVar(&opts.BrowserExe, "browser-exe", "", "Explicit Edge/Chrome/Chromium executable path for a new session.")
	c.Flags().BoolVar(&opts.Headless, "headless", false, "Run a newly started persistent browser without a visible UI.")
	c.Flags().StringVar(&opts.URL, "url", "", "HTTP or HTTPS URL opened as the first tab at startup; defaults to the configured origin.")
	c.Flags().BoolVar(&opts.RegisterProtocol, "register-protocol", false, "Register the efp-bridge:// protocol handler for the current user (Windows registry, macOS launcher app, Linux desktop entry) so the Portal page can start this bridge, store --origin as the default, then exit.")
	c.Flags().BoolVar(&opts.UnregisterProtocol, "unregister-protocol", false, "Remove the efp-bridge:// protocol handler for the current user, then exit.")
	return c
}

// resolveServeSettings merges flags with browser.serve.* config defaults.
// portChanged reports whether --port was passed explicitly; otherwise the
// config value wins over the flag default.
func resolveServeSettings(o *Opts, opts serveOptions, portChanged bool) (serveSettings, error) {
	cfg, err := loadBookmarkConfig(o, false)
	if err != nil {
		return serveSettings{}, automation.NewError("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", 400)
	}
	port := opts.Port
	if !portChanged && cfg.Browser.Serve.Port > 0 {
		port = cfg.Browser.Serve.Port
	}
	if port <= 0 {
		port = config.DefaultBrowserServePort
	}
	if port > 65535 {
		return serveSettings{}, automation.NewError("invalid_args", "--port must be between 1 and 65535", "Pass a loopback port such as --port 8765.", 400)
	}
	originRaw := strings.TrimSpace(opts.Origin)
	if originRaw == "" {
		originRaw = strings.TrimSpace(cfg.Browser.Serve.AllowedOrigin)
	}
	origin, err := normalizeBridgeOrigin(originRaw)
	if err != nil {
		return serveSettings{}, err
	}
	session := automationSessionName(opts.Session)
	if err := automation.ValidateSessionName(session); err != nil {
		return serveSettings{}, err
	}
	startURL := strings.TrimSpace(opts.URL)
	if startURL == "" && origin != "*" {
		startURL = origin
	}
	if startURL != "" {
		u, err := url.Parse(startURL)
		if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return serveSettings{}, automation.NewError("invalid_args", "--url must be an http or https URL", "Pass a full URL such as https://portal.example.com/app.", 400)
		}
	}
	return serveSettings{Origin: origin, Port: port, Session: session, URL: startURL}, nil
}

// listenBridge binds 127.0.0.1:<port>; when the port is busy the following
// ports are tried up to attempts-1 more times (8765-8770 by default).
func listenBridge(port, attempts int) (net.Listener, int, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		candidate := port + i
		if candidate > 65535 {
			break
		}
		ln, err := net.Listen("tcp", net.JoinHostPort(automation.LocalDebugAddr, strconv.Itoa(candidate)))
		if err == nil {
			return ln, candidate, nil
		}
		lastErr = err
	}
	message := fmt.Sprintf("No free loopback port between %d and %d.", port, port+attempts-1)
	if lastErr != nil {
		message += " " + lastErr.Error()
	}
	return nil, 0, automation.NewError("port_unavailable", message, "Stop the process that holds the port, or pass --port with a free loopback port.", 503)
}

func runServe(cmd *cobra.Command, o *Opts, opts serveOptions) error {
	settings, err := resolveServeSettings(o, opts, cmd.Flags().Changed("port"))
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	mgr, err := automation.DefaultManager()
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	logger := log.New(cmd.ErrOrStderr(), "browser serve: ", log.LstdFlags)
	ln, port, err := listenBridge(settings.Port, bridgePortAttempts)
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	server := newBridgeServer(mgr, settings.Origin, settings.Session, o, logger)
	httpServer := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second}
	listening := fmt.Sprintf("http://%s:%d", automation.LocalDebugAddr, port)
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(ln) }()
	if err := announceServe(cmd, o, listening, settings); err != nil {
		return err
	}
	logger.Printf("listening on %s origin=%s session=%s pid=%d", listening, settings.Origin, settings.Session, os.Getpid())

	server.start = automation.StartOptions{Name: settings.Session, Browser: opts.Browser, BrowserExe: opts.BrowserExe, Headless: opts.Headless, Verbose: o.Verbose}
	server.startURL = settings.URL
	go server.openStartupSession(context.Background())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		logger.Printf("received %s; shutting down", sig)
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("server stopped: %v", err)
			return printAutomationError(cmd, o, automation.NewError("server_error", err.Error(), "The bridge listener stopped unexpectedly.", 500))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), bridgeShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Printf("shutdown did not finish cleanly: %v", err)
	}
	logger.Printf("stopped; browser session %s is left running", settings.Session)
	return nil
}

// announceServe prints one startup line: a compact JSON envelope for --json,
// otherwise a human-readable sentence. Request logs go to stderr only.
func announceServe(cmd *cobra.Command, o *Opts, listening string, settings serveSettings) error {
	data := map[string]any{
		"listening":        listening,
		"origin":           settings.Origin,
		"session":          settings.Session,
		"pid":              os.Getpid(),
		"protocol_version": bridgeProtocolVersion,
	}
	if fmtOut(o) == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(output.RedactEnvelope(output.Success("", data)))
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "browser serve listening on %s (origin %s, session %s). Press Ctrl+C to stop; the browser session stays open.\n", listening, settings.Origin, settings.Session)
	return err
}

// openStartupSession makes the managed session ready so /ping can report
// alive=true: a stopped browser is launched on the configured first-tab URL
// (that tab only, no New Tab page) and a running one is reused with its tabs
// as they are. Failures are logged; the bridge keeps serving with alive=false.
func (s *bridgeServer) openStartupSession(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, bridgeStartupTimeout)
	defer cancel()
	release, ok := s.acquire(ctx, s.session)
	if !ok {
		s.logger.Printf("startup: session %s queue was busy; skipping startup open", s.session)
		return
	}
	defer release()
	result, err := s.ensureSession(ctx, s.session, "")
	if err != nil {
		s.logger.Printf("startup: browser session %s could not be opened: %v (serving anyway; /ping reports alive=false)", s.session, err)
		return
	}
	s.logger.Printf("startup: session %s alive=%t debug_port=%d reused=%t target=%s tab_opened=%t", result.Session.Name, result.Session.Alive, result.Session.DebugPort, result.Reused, result.Target.ID, result.TabOpened)
}

func runRegisterProtocol(cmd *cobra.Command, o *Opts, opts serveOptions) error {
	origin, err := normalizeBridgeOrigin(opts.Origin)
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	data, err := registerBridgeProtocol(origin)
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	port := 0
	if cmd.Flags().Changed("port") {
		port = opts.Port
	}
	path, saveErr := saveServeDefaults(o, origin, port)
	data["config_saved"] = saveErr == nil
	if path != "" {
		data["config_path"] = path
	}
	if saveErr != nil {
		data["config_warning"] = "Default origin was not stored: " + saveErr.Error()
	}
	data["next_steps"] = []string{
		"Open the Portal Connectors page and click Start bridge (efp-bridge://start?origin=...&port=...).",
		"Allow the EFP Bridge protocol prompt once; the bridge starts browser serve in the background.",
		"Click Test connection on the Portal page; a dedicated Chrome window opens for the managed session.",
	}
	return print(cmd, o, output.Success("", data))
}

func runUnregisterProtocol(cmd *cobra.Command, o *Opts) error {
	data, err := unregisterBridgeProtocol()
	if err != nil {
		return printAutomationError(cmd, o, err)
	}
	return print(cmd, o, output.Success("", data))
}

// saveServeDefaults stores browser.serve.allowed_origin (and port when given)
// in the shared EFP config so a later browser serve needs no flags.
func saveServeDefaults(o *Opts, origin string, port int) (string, error) {
	cfg, path, err := config.LoadShared(o.Config)
	if err != nil {
		// A missing config file (default location or an explicit --config path
		// that does not exist yet) is created, like browser bookmark source add.
		if !os.IsNotExist(err) {
			return path, err
		}
		cfg = config.RootConfig{}
		if path, err = config.ResolvePath(o.Config); err != nil {
			return "", err
		}
	}
	if config.EnvManaged(o.Config) {
		return path, config.ErrEnvManaged
	}
	cfg.Browser.Serve.AllowedOrigin = origin
	if port > 0 {
		cfg.Browser.Serve.Port = port
	}
	if err := config.SaveShared(o.Config, cfg); err != nil {
		return path, err
	}
	return path, nil
}

// bridgeExecutablePath returns the absolute path of the running binary for
// the protocol handler command and for detached bridge-launch children.
func bridgeExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", automation.NewError("automation_failed", err.Error(), "The browser executable path could not be resolved.", 500)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", automation.NewError("automation_failed", err.Error(), "The browser executable path could not be resolved.", 500)
	}
	return abs, nil
}
