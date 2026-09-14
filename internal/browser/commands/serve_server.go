package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/browser/bookmarks"
	"engineering-flow-platform-tools/internal/browser/probe"
	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/version"
)

// The local bridge implements section 6 of the Portal CONNECTORS_CONTRACT:
// a loopback HTTP server that the Portal page calls from the user's browser.
const (
	bridgeProtocolVersion    = 1
	bridgeDefaultTimeout     = 30 * time.Second
	bridgeMaxTimeout         = 120 * time.Second
	bridgeMaxRequestBytes    = 1 << 20
	bridgePingTimeout        = 3 * time.Second
	bridgeScreenshotMaxSide  = 1280
	bridgeScreenshotQuality  = 80
	bridgePortAttempts       = 6
	bridgeDefaultTextBytes   = 20000
	bridgeSnapshotTextBytes  = 4000
	bridgeCORSMaxAgeSeconds  = "600"
	bridgeAllowMethodsHeader = "GET, POST, OPTIONS"
	bridgeAllowHeadersHeader = "Content-Type"
)

// bridgeCommands maps the connector action names (contract section 3) to the
// in-process Manager calls. Session lifecycle other than session.ensure (which
// only reopens the managed window), page.eval, page.fetch, uploads, and
// downloads are intentionally absent.
var bridgeCommands = map[string]bool{
	"tab.list":        true,
	"tab.current":     true,
	"tab.activate":    true,
	"tab.open":        true,
	"page.snapshot":   true,
	"page.text":       true,
	"page.outline":    true,
	"page.ax":         true,
	"page.find":       true,
	"page.extract":    true,
	"page.table":      true,
	"page.wait":       true,
	"page.click":      true,
	"page.type":       true,
	"page.select":     true,
	"page.check":      true,
	"page.uncheck":    true,
	"page.press":      true,
	"page.screenshot": true,
	"bookmark.list":   true,
	"session.status":  true,
	"session.ensure":  true,
}

func bridgeCommandNames() []string {
	names := make([]string, 0, len(bridgeCommands))
	for name := range bridgeCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type bridgeRunRequest struct {
	Command        string          `json:"command"`
	Params         json.RawMessage `json:"params"`
	Session        string          `json:"session"`
	TimeoutSeconds int             `json:"timeout_seconds"`
}

// bridgeParams is the union of every params key accepted by the bridge
// commands. Keys are snake_case versions of the CLI flag names.
type bridgeParams struct {
	TargetID       string   `json:"target_id"`
	URL            string   `json:"url"`
	Selector       string   `json:"selector"`
	Ref            string   `json:"ref"`
	Text           string   `json:"text"`
	Value          string   `json:"value"`
	Label          string   `json:"label"`
	Index          *int     `json:"index"`
	Key            string   `json:"key"`
	Role           string   `json:"role"`
	Name           string   `json:"name"`
	Placeholder    string   `json:"placeholder"`
	NearText       string   `json:"near_text"`
	Nth            int      `json:"nth"`
	Limit          int      `json:"limit"`
	LimitRows      int      `json:"limit_rows"`
	LimitCells     int      `json:"limit_cells"`
	IncludeHidden  bool     `json:"include_hidden"`
	IncludeHTML    bool     `json:"include_html"`
	Pierce         bool     `json:"pierce"`
	Clear          bool     `json:"clear"`
	Yes            bool     `json:"yes"`
	AllowRisky     bool     `json:"allow_risky"`
	FullPage       bool     `json:"full_page"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	DurationMS     int      `json:"duration_ms"`
	URLContains    string   `json:"url_contains"`
	NetworkIdleMS  int      `json:"network_idle_ms"`
	DOMStableMS    int      `json:"dom_stable_ms"`
	MaxTextBytes   int      `json:"max_text_bytes"`
	MaxHTMLBytes   int      `json:"max_html_bytes"`
	Source         string   `json:"source"`
	Sources        []string `json:"sources"`
}

type bridgeSessionStatus struct {
	Name      string `json:"name"`
	Alive     bool   `json:"alive"`
	DebugPort int    `json:"debug_port"`
	TabCount  int    `json:"tab_count"`
}

type bridgeScreenshotResult struct {
	Session  string `json:"session"`
	TargetID string `json:"target_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	MIME     string `json:"mime"`
	Base64   string `json:"base64"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int    `json:"bytes"`
	Mode     string `json:"mode"`
	FullPage bool   `json:"full_page"`
	Selector string `json:"selector,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

// bridgeServer serves /ping, /commands, and /run on 127.0.0.1 for one
// configured Portal origin. Requests for the same session run one at a time so
// concurrent Portal calls queue instead of colliding on the session file lock.
type bridgeServer struct {
	manager *automation.Manager
	origin  string
	session string
	opts    *Opts
	logger  *log.Logger
	now     func() time.Time
	tempDir string

	// start and startURL carry the browser and first-tab settings of browser
	// serve; ensureSession applies them whenever the managed window has to be
	// opened or reopened. ensure is a test seam replacing that Manager call.
	start    automation.StartOptions
	startURL string
	ensure   func(ctx context.Context, sessionName, startURL string) (automation.EnsurePersistentResult, error)

	mu     sync.Mutex
	queues map[string]chan struct{}
}

func newBridgeServer(manager *automation.Manager, origin, session string, o *Opts, logger *log.Logger) *bridgeServer {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if o == nil {
		o = &Opts{}
	}
	return &bridgeServer{
		manager: manager,
		origin:  origin,
		session: automationSessionName(session),
		opts:    o,
		logger:  logger,
		now:     time.Now,
		tempDir: os.TempDir(),
		queues:  map[string]chan struct{}{},
	}
}

func automationSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return automation.DefaultSessionName
	}
	return name
}

// normalizeBridgeOrigin reduces --origin to scheme://host[:port] (lowercase,
// default ports dropped) or "*". Paths, queries, fragments, and credentials
// are rejected so the CORS echo is always a bare origin.
func normalizeBridgeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "*" {
		return "*", nil
	}
	if raw == "" {
		return "", automation.NewError("invalid_args", "--origin is required", "Pass the Portal origin such as --origin https://portal.example.com, or set browser.serve.allowed_origin / EFP_BROWSER_SERVE_ALLOWED_ORIGIN.", 400)
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", automation.NewError("invalid_args", "--origin must be an http or https origin", "Pass the Portal origin such as --origin https://portal.example.com, or * to allow any origin.", 400)
	}
	if u.User != nil {
		return "", automation.NewError("invalid_args", "--origin must not include credentials", "Pass only scheme and host, such as https://portal.example.com.", 400)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", automation.NewError("invalid_args", "--origin must not include a path, query, or fragment", "Pass only scheme and host, such as https://portal.example.com.", 400)
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func (s *bridgeServer) originAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" || s.origin == "*" {
		return true
	}
	return strings.EqualFold(strings.TrimRight(origin, "/"), s.origin)
}

func (s *bridgeServer) applyCORS(h http.Header) {
	h.Set("Access-Control-Allow-Origin", s.origin)
	h.Set("Vary", "Origin")
	h.Set("Cache-Control", "no-store")
}

func (s *bridgeServer) applyPreflight(h http.Header) {
	h.Set("Access-Control-Allow-Methods", bridgeAllowMethodsHeader)
	h.Set("Access-Control-Allow-Headers", bridgeAllowHeadersHeader)
	// Chrome's Private Network Access preflight header, plus the newer Local
	// Network Access spelling so either Chrome generation accepts the loopback call.
	h.Set("Access-Control-Allow-Private-Network", "true")
	h.Set("Access-Control-Allow-Local-Network", "true")
	h.Set("Access-Control-Max-Age", bridgeCORSMaxAgeSeconds)
}

// Handler returns the HTTP handler for the bridge.
func (s *bridgeServer) Handler() http.Handler {
	return s
}

type bridgeResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *bridgeResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *bridgeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := s.now()
	rw := &bridgeResponseWriter{ResponseWriter: w, status: http.StatusOK}
	command := "-"
	defer func() {
		s.logger.Printf("%s %s %s %d %s", r.Method, r.URL.Path, command, rw.status, s.now().Sub(start).Round(time.Millisecond))
	}()
	s.applyCORS(rw.Header())
	if !s.originAllowed(r) {
		s.writeEnvelope(rw, http.StatusForbidden, output.Failure("origin_denied", "Request origin is not allowed by this bridge.", "Start browser serve with --origin set to the Portal origin that hosts the page.", http.StatusForbidden), nil)
		return
	}
	if r.Method == http.MethodOptions {
		s.applyPreflight(rw.Header())
		rw.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.URL.Path {
	case "/ping":
		if !s.requireMethod(rw, r, http.MethodGet) {
			return
		}
		s.handlePing(rw, r)
	case "/commands":
		if !s.requireMethod(rw, r, http.MethodGet) {
			return
		}
		s.writeEnvelope(rw, http.StatusOK, output.Success("", map[string]any{"commands": bridgeCommandNames()}), nil)
	case "/run":
		if !s.requireMethod(rw, r, http.MethodPost) {
			return
		}
		command = s.handleRun(rw, r)
	default:
		s.writeEnvelope(rw, http.StatusNotFound, output.Failure("not_found", "Unknown bridge path.", "Use GET /ping, GET /commands, or POST /run.", http.StatusNotFound), nil)
	}
}

func (s *bridgeServer) requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method+", OPTIONS")
	s.writeEnvelope(w, http.StatusMethodNotAllowed, output.Failure("method_not_allowed", "HTTP method is not allowed for this path.", "Use "+method+" for "+r.URL.Path+".", http.StatusMethodNotAllowed), nil)
	return false
}

func (s *bridgeServer) handlePing(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), bridgePingTimeout)
	defer cancel()
	data := map[string]any{
		"version":          version.Version,
		"protocol_version": bridgeProtocolVersion,
		// The origin this bridge serves. /ping answers header-less callers too,
		// so a page that gets 403 on every other call can read this and say
		// which Portal the running bridge belongs to instead of reporting it as
		// missing and asking the member to start one that will never bind.
		"origin":  s.origin,
		"session": s.sessionStatus(ctx),
	}
	s.writeEnvelope(w, http.StatusOK, output.Success("", data), nil)
}

// sessionStatus reports the managed session without taking the session file
// lock so /ping keeps answering while a page command runs. Failures degrade to
// alive=false / tab_count=0 instead of failing the ping.
func (s *bridgeServer) sessionStatus(ctx context.Context) bridgeSessionStatus {
	status := bridgeSessionStatus{Name: s.session}
	if s.manager == nil || s.manager.Store == nil {
		return status
	}
	session, err := s.manager.Store.Load(s.session)
	if err != nil {
		return status
	}
	status.DebugPort = session.DebugPort
	client := s.manager.Client
	if client == nil {
		client = automation.NewDevToolsClient(session.DebugAddr, session.DebugPort)
	}
	info, err := client.Version(ctx)
	if err != nil || strings.TrimSpace(info.WebSocketDebuggerURL) == "" {
		return status
	}
	status.Alive = true
	if targets, err := client.ListTargets(ctx); err == nil {
		status.TabCount = len(automation.PageTargets(targets))
	}
	return status
}

func (s *bridgeServer) handleRun(w http.ResponseWriter, r *http.Request) string {
	var req bridgeRunRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bridgeMaxRequestBytes))
	if err != nil {
		s.writeEnvelope(w, http.StatusBadRequest, output.Failure("invalid_args", "Request body could not be read or exceeds 1 MB.", "Send a JSON object with command, params, session, and timeout_seconds.", http.StatusBadRequest), nil)
		return "-"
	}
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeEnvelope(w, http.StatusBadRequest, output.Failure("invalid_args", "Request body must be a JSON object with command, params, session, and timeout_seconds.", "Example: {\"command\":\"tab.list\",\"params\":{},\"session\":\"default\",\"timeout_seconds\":30}", http.StatusBadRequest), nil)
		return "-"
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		s.writeEnvelope(w, http.StatusBadRequest, output.Failure("invalid_args", "command is required.", "Run GET /commands to list the allowed command names.", http.StatusBadRequest), nil)
		return "-"
	}
	if !bridgeCommands[command] {
		s.writeEnvelope(w, http.StatusBadRequest, output.Failure("command_not_allowed", "Command is not exposed by the local bridge: "+command, "Run GET /commands for the allowed list; session lifecycle, page.eval, page.fetch, uploads, and downloads are not available through the bridge.", http.StatusBadRequest), nil)
		return command
	}
	sessionName := strings.TrimSpace(req.Session)
	if sessionName == "" {
		sessionName = s.session
	}
	if err := automation.ValidateSessionName(sessionName); err != nil {
		s.writeEnvelope(w, http.StatusBadRequest, automationFailure(err), nil)
		return command
	}
	params, err := decodeBridgeParams(req.Params)
	if err != nil {
		s.writeEnvelope(w, http.StatusBadRequest, output.Failure("invalid_args", "params must be a JSON object with the documented keys: "+err.Error(), "Run GET /commands and check the command's params in docs/BROWSER.md.", http.StatusBadRequest), nil)
		return command
	}
	timeout := bridgeTimeout(req.TimeoutSeconds)
	if params.TimeoutSeconds > 0 {
		timeout = bridgeTimeout(params.TimeoutSeconds)
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	release, ok := s.acquire(ctx, sessionName)
	if !ok {
		s.writeEnvelope(w, http.StatusGatewayTimeout, output.Failure("bridge_timeout", "Another command for this session did not finish within the request timeout.", "Retry after the current command finishes, or raise timeout_seconds (maximum 120).", http.StatusGatewayTimeout), nil)
		return command
	}
	defer release()
	env, patch := s.execute(ctx, command, sessionName, int(timeout/time.Second), params)
	status := http.StatusOK
	if !env.OK && env.Error != nil && env.Error.Status > 0 {
		status = env.Error.Status
	}
	s.writeEnvelope(w, status, env, patch)
	return command
}

func decodeBridgeParams(raw json.RawMessage) (bridgeParams, error) {
	params := bridgeParams{}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return params, nil
	}
	if !strings.HasPrefix(trimmed, "{") {
		return params, errors.New("params is not a JSON object")
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, err
	}
	return params, nil
}

func bridgeTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return bridgeDefaultTimeout
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > bridgeMaxTimeout {
		return bridgeMaxTimeout
	}
	return timeout
}

// acquire serializes Manager calls per session. It waits at most until ctx is
// done and reports false when the slot could not be taken in time.
func (s *bridgeServer) acquire(ctx context.Context, sessionName string) (func(), bool) {
	s.mu.Lock()
	queue, ok := s.queues[sessionName]
	if !ok {
		queue = make(chan struct{}, 1)
		s.queues[sessionName] = queue
	}
	s.mu.Unlock()
	select {
	case queue <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-queue }) }, true
	case <-ctx.Done():
		return nil, false
	}
}

func (s *bridgeServer) pageOptions(sessionName string, timeoutSeconds int, params bridgeParams) automation.PageOptions {
	return automation.PageOptions{SessionName: sessionName, TargetID: strings.TrimSpace(params.TargetID), TimeoutSeconds: automation.PageTimeoutSeconds(timeoutSeconds)}
}

// executeOnce runs one bridge command against the Manager and returns the CLI
// envelope plus an optional patch applied after redaction.
func (s *bridgeServer) executeOnce(ctx context.Context, command, sessionName string, timeoutSeconds int, p bridgeParams) (output.Envelope, func(map[string]any)) {
	mgr := s.manager
	page := s.pageOptions(sessionName, timeoutSeconds, p)
	var result any
	var err error
	switch command {
	case "tab.list":
		result, err = mgr.TabList(ctx, sessionName)
	case "tab.current":
		result, err = mgr.CurrentTab(ctx, sessionName)
	case "tab.activate":
		result, err = mgr.ActivateTab(ctx, sessionName, strings.TrimSpace(p.TargetID))
	case "tab.open":
		result, err = mgr.OpenTab(ctx, sessionName, strings.TrimSpace(p.URL))
	case "page.snapshot":
		result, err = mgr.Snapshot(ctx, automation.SnapshotOptions{PageOptions: page, IncludeHTML: p.IncludeHTML, MaxTextBytes: firstPositive(p.MaxTextBytes, bridgeSnapshotTextBytes), MaxHTMLBytes: p.MaxHTMLBytes})
	case "page.text":
		if strings.TrimSpace(p.Selector) != "" {
			result, err = mgr.Extract(ctx, automation.ExtractOptions{PageOptions: page, Selector: p.Selector, Limit: p.Limit, IncludeHTML: p.IncludeHTML, Pierce: p.Pierce, MaxHTMLBytes: p.MaxHTMLBytes})
		} else {
			result, err = mgr.Snapshot(ctx, automation.SnapshotOptions{PageOptions: page, IncludeHTML: p.IncludeHTML, MaxTextBytes: firstPositive(p.MaxTextBytes, bridgeDefaultTextBytes), MaxHTMLBytes: p.MaxHTMLBytes})
		}
	case "page.outline":
		result, err = mgr.Outline(ctx, automation.OutlineOptions{PageOptions: page, Limit: firstPositive(p.Limit, 100), IncludeHidden: p.IncludeHidden, Pierce: p.Pierce})
	case "page.ax":
		result, err = mgr.AX(ctx, automation.AXOptions{PageOptions: page, Limit: p.Limit, IncludeHidden: p.IncludeHidden, Pierce: p.Pierce})
	case "page.find":
		result, err = mgr.Find(ctx, automation.PageFindOptions{
			PageOptions:   page,
			Locator:       automation.ElementLocator{Selector: p.Selector, Role: p.Role, Name: p.Name, Text: p.Text, Label: p.Label, Placeholder: p.Placeholder, NearText: p.NearText, Nth: p.Nth},
			Limit:         p.Limit,
			IncludeHidden: p.IncludeHidden,
		})
	case "page.extract":
		result, err = mgr.Extract(ctx, automation.ExtractOptions{PageOptions: page, Selector: p.Selector, Limit: p.Limit, IncludeHTML: p.IncludeHTML, Pierce: p.Pierce, MaxHTMLBytes: p.MaxHTMLBytes})
	case "page.table":
		result, err = mgr.Table(ctx, automation.TableOptions{PageOptions: page, Selector: p.Selector, LimitRows: firstPositive(p.LimitRows, 50), LimitCells: firstPositive(p.LimitCells, 20), IncludeHTML: p.IncludeHTML})
	case "page.wait":
		result, err = mgr.Wait(ctx, automation.WaitOptions{
			PageOptions:             page,
			Selector:                p.Selector,
			DurationMilliseconds:    p.DurationMS,
			URLContains:             p.URLContains,
			Text:                    p.Text,
			NetworkIdleMilliseconds: p.NetworkIdleMS,
			DOMStableMilliseconds:   p.DOMStableMS,
		})
	case "page.click":
		result, err = mgr.Click(ctx, automation.ClickOptions{PageOptions: page, Selector: p.Selector, Ref: p.Ref, AllowRisky: p.Yes || p.AllowRisky})
	case "page.type":
		result, err = mgr.Type(ctx, automation.TypeOptions{PageOptions: page, Selector: p.Selector, Ref: p.Ref, Text: p.Text, Clear: p.Clear})
	case "page.select":
		index := -1
		if p.Index != nil {
			index = *p.Index
		}
		result, err = mgr.Select(ctx, automation.SelectOptions{PageOptions: page, Selector: p.Selector, Ref: p.Ref, Value: p.Value, Label: p.Label, Index: index})
	case "page.check", "page.uncheck":
		result, err = mgr.Check(ctx, automation.CheckOptions{PageOptions: page, Selector: p.Selector, Ref: p.Ref, Checked: command == "page.check"})
	case "page.press":
		result, err = mgr.Press(ctx, automation.PressOptions{PageOptions: page, Selector: p.Selector, Ref: p.Ref, Key: p.Key})
	case "page.screenshot":
		return s.screenshot(ctx, page, p)
	case "bookmark.list":
		return s.bookmarkList(ctx, p), nil
	case "session.status":
		result, err = mgr.Status(ctx, sessionName)
	case "session.ensure":
		result, err = s.ensureSession(ctx, sessionName, strings.TrimSpace(p.URL))
	default:
		return output.Failure("command_not_allowed", "Command is not exposed by the local bridge: "+command, "Run GET /commands for the allowed list.", http.StatusBadRequest), nil
	}
	if err != nil {
		return automationFailure(err), nil
	}
	return output.Success("", result), nil
}

func firstPositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// screenshot captures through the Manager (which writes a PNG artifact), then
// converts the file to a bounded JPEG for the Portal and removes the artifact.
func (s *bridgeServer) screenshot(ctx context.Context, page automation.PageOptions, p bridgeParams) (output.Envelope, func(map[string]any)) {
	outPath := filepath.Join(s.tempDir, fmt.Sprintf("efp-bridge-screenshot-%d-%d.png", os.Getpid(), s.now().UnixNano()))
	defer os.Remove(outPath)
	result, err := s.manager.Screenshot(ctx, automation.ScreenshotOptions{
		PageOptions: page,
		OutPath:     outPath,
		FullPage:    p.FullPage,
		FullPageSet: true,
		Selector:    p.Selector,
		Ref:         p.Ref,
	})
	if err != nil {
		return automationFailure(err), nil
	}
	raw, err := os.ReadFile(result.Path)
	if err != nil {
		return output.Failure("artifact_read_failed", probe.RedactErrorMessage(err.Error()), "The screenshot artifact could not be read back.", http.StatusInternalServerError), nil
	}
	if result.Path != outPath {
		_ = os.Remove(result.Path)
	}
	jpeg, width, height, err := encodeBridgeScreenshot(raw, bridgeScreenshotMaxSide, bridgeScreenshotQuality)
	if err != nil {
		return output.Failure("screenshot_encode_failed", probe.RedactErrorMessage(err.Error()), "The PNG screenshot could not be converted to JPEG.", http.StatusInternalServerError), nil
	}
	encoded := base64.StdEncoding.EncodeToString(jpeg)
	data := bridgeScreenshotResult{
		Session:  result.Session,
		TargetID: result.TargetID,
		URL:      result.URL,
		Title:    result.Title,
		MIME:     "image/jpeg",
		Width:    width,
		Height:   height,
		Bytes:    len(jpeg),
		Mode:     result.Mode,
		FullPage: result.FullPage,
		Selector: result.Selector,
		Ref:      result.Ref,
	}
	// The base64 payload is injected after envelope redaction: redaction is a
	// text scanner and must not rewrite image bytes.
	return output.Success("", data), func(m map[string]any) { m["base64"] = encoded }
}

func (s *bridgeServer) bookmarkList(ctx context.Context, p bridgeParams) output.Envelope {
	cfg, err := loadBookmarkConfig(s.opts, false)
	if err != nil {
		return output.Failure("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", http.StatusBadRequest)
	}
	requested := append([]string{}, p.Sources...)
	if strings.TrimSpace(p.Source) != "" {
		requested = append(requested, p.Source)
	}
	sources, err := filterBookmarkSources(bookmarkSources(cfg), requested)
	if err != nil {
		return bookmarkFailure(err)
	}
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, listErr := bookmarks.NewLister().List(listCtx, sources)
	if listErr == nil {
		return output.Success("", result)
	}
	var bookmarkErr *bookmarks.Error
	if errors.As(listErr, &bookmarkErr) {
		env := output.Failure(bookmarkErr.Code, bookmarkErr.Message, bookmarkErr.Hint, bookmarkErr.Status)
		env.Data = result
		return env
	}
	return output.Failure("bookmark_list_failed", "Bookmark sources could not be listed.", "Verify the source configuration and retry.", http.StatusInternalServerError)
}

func bookmarkFailure(err error) output.Envelope {
	var bookmarkErr *bookmarks.Error
	if errors.As(err, &bookmarkErr) {
		return output.Failure(bookmarkErr.Code, bookmarkErr.Message, bookmarkErr.Hint, bookmarkErr.Status)
	}
	return output.Failure("bookmark_error", "The bookmark operation failed.", "Inspect the configured bookmark source and retry.", http.StatusInternalServerError)
}

// automationFailure mirrors printAutomationError for HTTP responses.
func automationFailure(err error) output.Envelope {
	var autoErr *automation.Error
	if errors.As(err, &autoErr) {
		return output.Failure(autoErr.Code, probe.RedactErrorMessage(autoErr.Message), autoErr.Hint, autoErr.Status)
	}
	return output.Failure("automation_failed", probe.RedactErrorMessage(err.Error()), "", http.StatusInternalServerError)
}

// writeEnvelope applies the same redaction as CLI output, lets the caller
// patch the redacted data map, and writes JSON with the given HTTP status.
func (s *bridgeServer) writeEnvelope(w http.ResponseWriter, status int, env output.Envelope, patch func(map[string]any)) {
	env = output.RedactEnvelope(env)
	if patch != nil {
		if m, ok := env.Data.(map[string]any); ok {
			patch(m)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(env); err != nil {
		s.logger.Printf("write response failed: %v", err)
	}
}

// ensureSession reopens the managed browser when its window was closed (the
// bridge outlives Chrome, so /ping keeps answering with session.alive=false)
// and brings a tab at the page's origin to the front without adding tabs.
// startURL overrides the bridge's configured first tab for this call, so a
// page whose start page setting changed after the bridge started still gets
// the current one; empty keeps the bridge default.
func (s *bridgeServer) ensureSession(ctx context.Context, sessionName, startURL string) (automation.EnsurePersistentResult, error) {
	if startURL == "" {
		startURL = s.startURL
	}
	if s.ensure != nil {
		return s.ensure(ctx, sessionName, startURL)
	}
	start := s.start
	start.Name = sessionName
	start.URL = startURL
	return s.manager.EnsurePersistent(ctx, start)
}

// execute runs one bridge command, reopening the managed browser first when
// the member closed its window: the composer shows the browser as switched
// on, so a tab or page command is replayed after the reopen instead of
// failing with session_not_running. Status queries report the closed window
// as it is.
func (s *bridgeServer) execute(ctx context.Context, command, sessionName string, timeoutSeconds int, p bridgeParams) (output.Envelope, func(map[string]any)) {
	env, patch := s.executeOnce(ctx, command, sessionName, timeoutSeconds, p)
	if env.OK || !sessionGone(env) || command == "session.status" || command == "session.ensure" {
		return env, patch
	}
	if _, err := s.ensureSession(ctx, sessionName, ""); err != nil {
		s.logger.Printf("%s: browser session %s could not be reopened: %v", command, sessionName, err)
		return env, patch
	}
	return s.executeOnce(ctx, command, sessionName, timeoutSeconds, p)
}

func sessionGone(env output.Envelope) bool {
	if env.Error == nil {
		return false
	}
	return env.Error.Code == "session_not_running" || env.Error.Code == "session_not_found"
}
