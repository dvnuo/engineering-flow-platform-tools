package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/browser/automation"
	"engineering-flow-platform-tools/internal/config"
)

const testPortalOrigin = "https://portal.example.test"

// fakeDevTools serves the DevTools HTTP endpoints the Manager uses for tab
// commands so the bridge can be exercised without a real Chrome.
type fakeDevTools struct {
	server      *httptest.Server
	host        string
	port        int
	listDelay   time.Duration
	inFlight    int32
	maxInFlight int32
	mu          sync.Mutex
	activated   []string
	opened      []string
}

func newFakeDevTools(t *testing.T, listDelay time.Duration) *fakeDevTools {
	t.Helper()
	f := &fakeDevTools{listDelay: listDelay}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&f.inFlight, 1)
		defer atomic.AddInt32(&f.inFlight, -1)
		for {
			max := atomic.LoadInt32(&f.maxInFlight)
			if current <= max || atomic.CompareAndSwapInt32(&f.maxInFlight, max, current) {
				break
			}
		}
		switch {
		case r.URL.Path == "/json/version":
			_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/browser/abc"}`))
		case r.URL.Path == "/json/list":
			if f.listDelay > 0 {
				time.Sleep(f.listDelay)
			}
			_, _ = w.Write([]byte(`[
				{"id":"page-1","type":"page","title":"Home","url":"https://portal.example.test/app?code=secret","webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/page/page-1"},
				{"id":"page-2","type":"page","title":"Other","url":"https://portal.example.test/other","webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/page/page-2"},
				{"id":"worker-1","type":"service_worker","title":"Worker","url":"https://portal.example.test/sw.js","webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/page/worker-1"}
			]`))
		case strings.HasPrefix(r.URL.Path, "/json/activate/"):
			f.mu.Lock()
			f.activated = append(f.activated, strings.TrimPrefix(r.URL.Path, "/json/activate/"))
			f.mu.Unlock()
			_, _ = w.Write([]byte("Target activated"))
		case r.URL.Path == "/json/new":
			f.mu.Lock()
			f.opened = append(f.opened, r.URL.RawQuery)
			f.mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"page-3","type":"page","title":"New","url":"https://portal.example.test/new","webSocketDebuggerUrl":"ws://127.0.0.1:1/devtools/page/page-3"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(f.server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	f.host = host
	f.port = port
	return f
}

type bridgeFixture struct {
	devtools *fakeDevTools
	manager  *automation.Manager
	server   *bridgeServer
	http     *httptest.Server
}

func newBridgeFixture(t *testing.T, origin string, listDelay time.Duration) *bridgeFixture {
	t.Helper()
	t.Setenv("EFP_BROWSER_HOME", t.TempDir())
	devtools := newFakeDevTools(t, listDelay)
	store := automation.NewStore(t.TempDir())
	if err := store.Save(automation.Session{
		Name:      "default",
		DebugAddr: devtools.host,
		DebugPort: devtools.port,
		CreatedAt: time.Now().UTC(),
		Alive:     true,
	}); err != nil {
		t.Fatal(err)
	}
	manager := automation.NewManager(store, nil)
	server := newBridgeServer(manager, origin, "default", &Opts{}, log.New(io.Discard, "", 0))
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	return &bridgeFixture{devtools: devtools, manager: manager, server: server, http: httpServer}
}

func (f *bridgeFixture) do(t *testing.T, method, path, origin string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, f.http.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s %s did not return JSON: %v body=%s", method, path, err, raw)
		}
	}
	return resp, out
}

func (f *bridgeFixture) run(t *testing.T, command string, params map[string]any, timeoutSeconds int) (*http.Response, map[string]any) {
	t.Helper()
	body := map[string]any{"command": command, "params": params, "session": "default"}
	if timeoutSeconds > 0 {
		body["timeout_seconds"] = timeoutSeconds
	}
	return f.do(t, http.MethodPost, "/run", testPortalOrigin, body)
}

func errorCode(t *testing.T, out map[string]any) string {
	t.Helper()
	if out == nil {
		t.Fatal("missing envelope")
	}
	errObj, _ := out["error"].(map[string]any)
	code, _ := errObj["code"].(string)
	return code
}

func TestBridgePreflightReturnsContractHeaders(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	req, err := http.NewRequest(http.MethodOptions, f.http.URL+"/run", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", testPortalOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d body=%s", resp.StatusCode, body)
	}
	if len(body) != 0 {
		t.Fatalf("preflight body should be empty: %s", body)
	}
	want := map[string]string{
		"Access-Control-Allow-Origin":          testPortalOrigin,
		"Access-Control-Allow-Methods":         "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers":         "Content-Type",
		"Access-Control-Allow-Private-Network": "true",
		"Access-Control-Max-Age":               "600",
		"Vary":                                 "Origin",
	}
	for name, value := range want {
		if got := resp.Header.Get(name); got != value {
			t.Fatalf("preflight header %s = %q want %q (all=%v)", name, got, value, resp.Header)
		}
	}
	// Every non-preflight response also carries the origin echo.
	pingResp, _ := f.do(t, http.MethodGet, "/ping", testPortalOrigin, nil)
	if pingResp.Header.Get("Access-Control-Allow-Origin") != testPortalOrigin || pingResp.Header.Get("Vary") != "Origin" {
		t.Fatalf("ping headers missing CORS echo: %v", pingResp.Header)
	}
}

func TestBridgeRejectsForeignOrigin(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	resp, out := f.do(t, http.MethodGet, "/ping", "https://evil.test", nil)
	if resp.StatusCode != http.StatusForbidden || errorCode(t, out) != "origin_denied" {
		t.Fatalf("foreign origin: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != testPortalOrigin {
		t.Fatalf("denied response should still echo the configured origin: %v", resp.Header)
	}
	resp, out = f.do(t, http.MethodPost, "/run", "https://evil.test", map[string]any{"command": "tab.list"})
	if resp.StatusCode != http.StatusForbidden || errorCode(t, out) != "origin_denied" {
		t.Fatalf("foreign origin run: status=%d out=%#v", resp.StatusCode, out)
	}
	req, _ := http.NewRequest(http.MethodOptions, f.http.URL+"/run", nil)
	req.Header.Set("Origin", "https://evil.test")
	preflight, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = preflight.Body.Close()
	if preflight.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign preflight status = %d", preflight.StatusCode)
	}
	// Same origin with different letter case is still the same origin; no Origin header (curl) is allowed.
	if resp, _ := f.do(t, http.MethodGet, "/ping", "HTTPS://Portal.Example.Test", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("case-insensitive origin status = %d", resp.StatusCode)
	}
	if resp, _ := f.do(t, http.MethodGet, "/ping", "", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("missing origin status = %d", resp.StatusCode)
	}
}

func TestBridgeWildcardOriginEchoesStar(t *testing.T) {
	f := newBridgeFixture(t, "*", 0)
	resp, out := f.do(t, http.MethodGet, "/ping", "https://anything.example.test", nil)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("wildcard origin: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("wildcard should echo *: %v", resp.Header)
	}
}

func TestBridgeUnknownCommandNotAllowed(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	for _, command := range []string{"session.stop", "session.start", "page.eval", "page.fetch", "page.upload", "download.list", "nonsense"} {
		resp, out := f.run(t, command, nil, 0)
		if resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "command_not_allowed" {
			t.Fatalf("%s: status=%d out=%#v", command, resp.StatusCode, out)
		}
	}
	resp, out := f.do(t, http.MethodPost, "/run", testPortalOrigin, map[string]any{"command": ""})
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("empty command: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestBridgeCommandsListsAllowedNames(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	resp, out := f.do(t, http.MethodGet, "/commands", testPortalOrigin, nil)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("commands: status=%d out=%#v", resp.StatusCode, out)
	}
	names := map[string]bool{}
	for _, raw := range out["data"].(map[string]any)["commands"].([]any) {
		names[raw.(string)] = true
	}
	for _, want := range []string{"tab.list", "tab.current", "tab.activate", "tab.open", "page.snapshot", "page.text", "page.outline", "page.ax", "page.find", "page.extract", "page.table", "page.wait", "page.click", "page.type", "page.select", "page.check", "page.uncheck", "page.press", "page.screenshot", "bookmark.list", "session.status"} {
		if !names[want] {
			t.Fatalf("commands missing %s: %v", want, names)
		}
	}
	for _, forbidden := range []string{"page.eval", "page.fetch", "session.start", "session.stop", "page.upload", "download.list", "download.wait"} {
		if names[forbidden] {
			t.Fatalf("commands must not expose %s", forbidden)
		}
	}
}

func TestBridgePingReportsSession(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	resp, out := f.do(t, http.MethodGet, "/ping", testPortalOrigin, nil)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("ping: status=%d out=%#v", resp.StatusCode, out)
	}
	data := out["data"].(map[string]any)
	if data["version"] == "" || data["protocol_version"] != float64(bridgeProtocolVersion) {
		t.Fatalf("ping data = %#v", data)
	}
	session := data["session"].(map[string]any)
	if session["name"] != "default" || session["alive"] != true || session["tab_count"] != float64(2) || session["debug_port"] != float64(f.devtools.port) {
		t.Fatalf("ping session = %#v", session)
	}

	// A dead session degrades to alive=false instead of failing the ping.
	f.devtools.server.Close()
	resp, out = f.do(t, http.MethodGet, "/ping", testPortalOrigin, nil)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("ping with dead session: status=%d out=%#v", resp.StatusCode, out)
	}
	session = out["data"].(map[string]any)["session"].(map[string]any)
	if session["alive"] != false || session["tab_count"] != float64(0) {
		t.Fatalf("dead session = %#v", session)
	}
}

func TestBridgeRunTabCommands(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	resp, out := f.run(t, "tab.list", nil, 0)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("tab.list: status=%d out=%#v", resp.StatusCode, out)
	}
	data := out["data"].(map[string]any)
	tabs := data["tabs"].([]any)
	if data["session"] != "default" || len(tabs) != 2 {
		t.Fatalf("tab.list data = %#v", data)
	}
	first := tabs[0].(map[string]any)
	if first["id"] != "page-1" || strings.Contains(first["url"].(string), "secret") || !strings.Contains(first["url"].(string), "code=REDACTED") {
		t.Fatalf("tab.list first tab = %#v", first)
	}

	resp, out = f.run(t, "tab.activate", map[string]any{"target_id": "page-2"}, 0)
	if resp.StatusCode != http.StatusOK || out["ok"] != true || out["data"].(map[string]any)["tab"].(map[string]any)["id"] != "page-2" {
		t.Fatalf("tab.activate: status=%d out=%#v", resp.StatusCode, out)
	}
	if len(f.devtools.activated) != 1 || f.devtools.activated[0] != "page-2" {
		t.Fatalf("activate calls = %v", f.devtools.activated)
	}
	resp, out = f.run(t, "tab.activate", map[string]any{"target_id": "missing"}, 0)
	if resp.StatusCode != http.StatusNotFound || errorCode(t, out) != "target_not_found" {
		t.Fatalf("tab.activate missing: status=%d out=%#v", resp.StatusCode, out)
	}

	resp, out = f.run(t, "tab.current", nil, 0)
	if resp.StatusCode != http.StatusOK || out["data"].(map[string]any)["tab"].(map[string]any)["id"] != "page-2" {
		t.Fatalf("tab.current: status=%d out=%#v", resp.StatusCode, out)
	}

	resp, out = f.run(t, "tab.open", map[string]any{"url": "https://portal.example.test/new"}, 0)
	if resp.StatusCode != http.StatusOK || out["data"].(map[string]any)["tab"].(map[string]any)["id"] != "page-3" || len(f.devtools.opened) != 1 {
		t.Fatalf("tab.open: status=%d out=%#v opened=%v", resp.StatusCode, out, f.devtools.opened)
	}
	resp, out = f.run(t, "tab.open", map[string]any{"url": "file:///etc/passwd"}, 0)
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("tab.open file url: status=%d out=%#v", resp.StatusCode, out)
	}

	resp, out = f.run(t, "session.status", nil, 0)
	if resp.StatusCode != http.StatusOK || out["data"].(map[string]any)["alive"] != true {
		t.Fatalf("session.status: status=%d out=%#v", resp.StatusCode, out)
	}

	// The request session defaults to the configured serve session; an invalid name is rejected.
	resp, out = f.do(t, http.MethodPost, "/run", testPortalOrigin, map[string]any{"command": "tab.list", "session": "../etc"})
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("bad session name: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestBridgeSerializesRequestsPerSession(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 150*time.Millisecond)
	const workers = 4
	var wg sync.WaitGroup
	results := make([]int, workers)
	codes := make([]string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, out := f.run(t, "tab.list", nil, 30)
			results[i] = resp.StatusCode
			if out["ok"] != true {
				codes[i] = errorCode(t, out)
			}
		}(i)
	}
	wg.Wait()
	for i := range results {
		if results[i] != http.StatusOK || codes[i] != "" {
			t.Fatalf("concurrent request %d: status=%d code=%q (all=%v codes=%v)", i, results[i], codes[i], results, codes)
		}
	}
	if got := atomic.LoadInt32(&f.devtools.maxInFlight); got != 1 {
		t.Fatalf("DevTools requests overlapped: max in flight = %d", got)
	}
}

func TestBridgeQueueTimeoutReturnsBridgeTimeout(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	release, ok := f.server.acquire(context.Background(), "default")
	if !ok {
		t.Fatal("could not take the session slot")
	}
	start := time.Now()
	resp, out := f.run(t, "tab.list", nil, 1)
	release()
	if resp.StatusCode != http.StatusGatewayTimeout || errorCode(t, out) != "bridge_timeout" {
		t.Fatalf("queued request: status=%d out=%#v", resp.StatusCode, out)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("queue wait was not bounded by timeout_seconds: %s", elapsed)
	}
	// The slot is usable again after release.
	if resp, out := f.run(t, "tab.list", nil, 0); resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("after release: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestBridgeRejectsMalformedRequests(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	req, _ := http.NewRequest(http.MethodPost, f.http.URL+"/run", strings.NewReader("{not json"))
	req.Header.Set("Origin", testPortalOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("bad json: status=%d body=%s err=%v", resp.StatusCode, raw, err)
	}
	if resp, out := f.do(t, http.MethodGet, "/run", testPortalOrigin, nil); resp.StatusCode != http.StatusMethodNotAllowed || errorCode(t, out) != "method_not_allowed" {
		t.Fatalf("GET /run: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp, out := f.do(t, http.MethodGet, "/nope", testPortalOrigin, nil); resp.StatusCode != http.StatusNotFound || errorCode(t, out) != "not_found" {
		t.Fatalf("GET /nope: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp, out := f.do(t, http.MethodPost, "/run", testPortalOrigin, map[string]any{"command": "tab.list", "params": 5}); resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("non-object params: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp, out := f.run(t, "page.outline", map[string]any{"limit": "ten"}, 0); resp.StatusCode != http.StatusBadRequest || errorCode(t, out) != "invalid_args" {
		t.Fatalf("mistyped params: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestBridgePageCommandWithoutBrowserReturnsErrorEnvelope(t *testing.T) {
	f := newBridgeFixture(t, testPortalOrigin, 0)
	resp, out := f.run(t, "page.snapshot", nil, 5)
	if out == nil || out["ok"] != false {
		t.Fatalf("page.snapshot without a browser should fail cleanly: status=%d out=%#v", resp.StatusCode, out)
	}
	if resp.StatusCode < 400 || errorCode(t, out) == "" {
		t.Fatalf("page.snapshot error mapping: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestBridgeBookmarkListUsesSharedConfig(t *testing.T) {
	setBookmarkTestHome(t)
	dir := t.TempDir()
	manifest := filepath.Join(dir, "team.yaml")
	if err := os.WriteFile(manifest, []byte("version: 1\nbookmarks:\n  - name: Runbooks\n    description: Read team runbooks.\n    url: https://runbooks.example.test/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("browser:\n  bookmarks:\n    sources:\n      - name: team\n        url: "+manifest+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := newBridgeFixture(t, testPortalOrigin, 0)
	f.server.opts = &Opts{Config: configPath}
	resp, out := f.run(t, "bookmark.list", nil, 0)
	if resp.StatusCode != http.StatusOK || out["ok"] != true {
		t.Fatalf("bookmark.list: status=%d out=%#v", resp.StatusCode, out)
	}
	items := out["data"].(map[string]any)["bookmarks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "Runbooks" {
		t.Fatalf("bookmarks = %#v", items)
	}
	resp, out = f.run(t, "bookmark.list", map[string]any{"sources": []string{"missing"}}, 0)
	if resp.StatusCode != http.StatusNotFound || errorCode(t, out) != "bookmark_source_not_found" {
		t.Fatalf("bookmark.list unknown source: status=%d out=%#v", resp.StatusCode, out)
	}
}

func TestParseBridgeLaunchURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    bridgeLaunchRequest
		wantErr string
	}{
		{name: "encoded origin and port", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test&port=8765", want: bridgeLaunchRequest{Origin: "https://portal.example.test", Port: 8765}},
		{name: "trailing slash from the browser", raw: "efp-bridge://start/?origin=https%3A%2F%2Fportal.example.test%3A8443&port=8766", want: bridgeLaunchRequest{Origin: "https://portal.example.test:8443", Port: 8766}},
		{name: "opaque form without slashes", raw: "efp-bridge:start?origin=http%3A%2F%2Flocalhost%3A8000", want: bridgeLaunchRequest{Origin: "http://localhost:8000"}},
		{name: "session parameter", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test&session=portal", want: bridgeLaunchRequest{Origin: "https://portal.example.test", Session: "portal"}},
		{name: "uppercase scheme and default port stripped", raw: "EFP-BRIDGE://START?origin=HTTPS%3A%2F%2FPortal.Example.Test%3A443", want: bridgeLaunchRequest{Origin: "https://portal.example.test"}},
		{name: "wrong scheme", raw: "https://portal.example.test/start?origin=x", wantErr: "invalid_args"},
		{name: "unknown action", raw: "efp-bridge://stop?origin=https%3A%2F%2Fportal.example.test", wantErr: "invalid_args"},
		{name: "missing origin", raw: "efp-bridge://start?port=8765", wantErr: "invalid_args"},
		{name: "origin with path", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test%2Fapp", wantErr: "invalid_args"},
		{name: "wildcard origin", raw: "efp-bridge://start?origin=*", wantErr: "invalid_args"},
		{name: "privileged port", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test&port=80", wantErr: "invalid_args"},
		{name: "non-numeric port", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test&port=abc", wantErr: "invalid_args"},
		{name: "bad session", raw: "efp-bridge://start?origin=https%3A%2F%2Fportal.example.test&session=..%2Fx", wantErr: "invalid_args"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBridgeLaunchURL(tc.raw)
			if tc.wantErr != "" {
				autoErr, ok := err.(*automation.Error)
				if !ok || autoErr.Code != tc.wantErr {
					t.Fatalf("err = %v, want code %s", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("parsed = %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestNormalizeBridgeOrigin(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "https://portal.example.test", want: "https://portal.example.test"},
		{raw: "https://portal.example.test/", want: "https://portal.example.test"},
		{raw: "HTTPS://Portal.Example.Test:443", want: "https://portal.example.test"},
		{raw: "http://localhost:8000", want: "http://localhost:8000"},
		{raw: "http://localhost:80", want: "http://localhost"},
		{raw: "*", want: "*"},
		{raw: "", wantErr: true},
		{raw: "portal.example.test", wantErr: true},
		{raw: "ftp://portal.example.test", wantErr: true},
		{raw: "https://user:pw@portal.example.test", wantErr: true},
		{raw: "https://portal.example.test/app", wantErr: true},
		{raw: "https://portal.example.test?x=1", wantErr: true},
	}
	for _, tc := range tests {
		got, err := normalizeBridgeOrigin(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error, got %q", tc.raw, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%q: got %q err=%v want %q", tc.raw, got, err, tc.want)
		}
	}
}

func TestBridgeTimeoutClamps(t *testing.T) {
	if got := bridgeTimeout(0); got != 30*time.Second {
		t.Fatalf("default timeout = %s", got)
	}
	if got := bridgeTimeout(-5); got != 30*time.Second {
		t.Fatalf("negative timeout = %s", got)
	}
	if got := bridgeTimeout(5); got != 5*time.Second {
		t.Fatalf("explicit timeout = %s", got)
	}
	if got := bridgeTimeout(500); got != 120*time.Second {
		t.Fatalf("capped timeout = %s", got)
	}
}

func encodeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestEncodeBridgeScreenshotDownscalesToLongestSide(t *testing.T) {
	tests := []struct {
		width, height         int
		wantWidth, wantHeight int
	}{
		{2000, 1000, 1280, 640},
		{1000, 2000, 640, 1280},
		{800, 600, 800, 600},
		{1280, 720, 1280, 720},
		{3000, 300, 1280, 128},
	}
	for _, tc := range tests {
		jpegBytes, width, height, err := encodeBridgeScreenshot(encodeTestPNG(t, tc.width, tc.height), bridgeScreenshotMaxSide, bridgeScreenshotQuality)
		if err != nil {
			t.Fatalf("%dx%d: %v", tc.width, tc.height, err)
		}
		if width != tc.wantWidth || height != tc.wantHeight {
			t.Fatalf("%dx%d: got %dx%d want %dx%d", tc.width, tc.height, width, height, tc.wantWidth, tc.wantHeight)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(jpegBytes))
		if err != nil {
			t.Fatalf("%dx%d: output is not JPEG: %v", tc.width, tc.height, err)
		}
		if b := decoded.Bounds(); b.Dx() != tc.wantWidth || b.Dy() != tc.wantHeight {
			t.Fatalf("%dx%d: decoded bounds %v", tc.width, tc.height, b)
		}
	}
	if _, _, _, err := encodeBridgeScreenshot([]byte("not a png"), bridgeScreenshotMaxSide, bridgeScreenshotQuality); err == nil {
		t.Fatal("invalid PNG should fail")
	}
}

func TestListenBridgeSkipsBusyPorts(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	ln, got, err := listenBridge(port, bridgePortAttempts)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got == port || got < port+1 || got > port+bridgePortAttempts-1 {
		t.Fatalf("listenBridge chose %d after busy %d", got, port)
	}
	if host, _, _ := net.SplitHostPort(ln.Addr().String()); host != "127.0.0.1" {
		t.Fatalf("listener is not loopback-only: %s", ln.Addr())
	}
	_, _, err = listenBridge(port, 1)
	autoErr, ok := err.(*automation.Error)
	if !ok || autoErr.Code != "port_unavailable" {
		t.Fatalf("single busy port err = %v", err)
	}
}

func TestResolveServeSettingsUsesConfigDefaults(t *testing.T) {
	setBookmarkTestHome(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("browser:\n  serve:\n    port: 9001\n    allowed_origin: https://Portal.Example.Test/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	o := &Opts{Config: configPath}
	settings, err := resolveServeSettings(o, serveOptions{Port: config.DefaultBrowserServePort, Session: "default"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Port != 9001 || settings.Origin != "https://portal.example.test" || settings.URL != "https://portal.example.test" || settings.Session != "default" {
		t.Fatalf("settings from config = %#v", settings)
	}
	settings, err = resolveServeSettings(o, serveOptions{Port: 8765, Origin: "*", URL: "https://portal.example.test/app", Session: ""}, true)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Port != 8765 || settings.Origin != "*" || settings.URL != "https://portal.example.test/app" || settings.Session != "default" {
		t.Fatalf("explicit settings = %#v", settings)
	}
	settings, err = resolveServeSettings(o, serveOptions{Port: 8765, Origin: "*"}, true)
	if err != nil || settings.URL != "" {
		t.Fatalf("wildcard origin without --url should not open a startup tab: %#v err=%v", settings, err)
	}
	if _, err := resolveServeSettings(&Opts{}, serveOptions{Port: 8765}, false); err == nil {
		t.Fatal("missing origin should fail")
	}
	if _, err := resolveServeSettings(o, serveOptions{Port: 8765, URL: "ftp://x"}, true); err == nil {
		t.Fatal("non-http startup url should fail")
	}
	if _, err := resolveServeSettings(o, serveOptions{Port: 70000}, true); err == nil {
		t.Fatal("out-of-range port should fail")
	}
}

func TestServeCommandValidatesFlagsBeforeListening(t *testing.T) {
	setBookmarkTestHome(t)
	out := run(t, &fakeRunner{}, "serve", "--json")
	if out["ok"] != false || errorCode(t, out) != "invalid_args" {
		t.Fatalf("serve without origin: %#v", out)
	}
	out = run(t, &fakeRunner{}, "serve", "--origin", "https://portal.example.test", "--register-protocol", "--unregister-protocol", "--json")
	if out["ok"] != false || errorCode(t, out) != "invalid_args" {
		t.Fatalf("conflicting protocol flags: %#v", out)
	}
	out = run(t, &fakeRunner{}, "bridge-launch", "https://not-a-bridge-link", "--json")
	if out["ok"] != false || errorCode(t, out) != "invalid_args" {
		t.Fatalf("bridge-launch with a bad link: %#v", out)
	}
}

func TestServeCommandIsVisibleAndBridgeLaunchHidden(t *testing.T) {
	root := NewRootWithRunner(&fakeRunner{})
	var serve, launch bool
	for _, child := range root.Commands() {
		switch child.Name() {
		case "serve":
			serve = !child.Hidden
		case "bridge-launch":
			launch = child.Hidden
		}
	}
	if !serve || !launch {
		t.Fatalf("serve visible=%t bridge-launch hidden=%t", serve, launch)
	}
	out := run(t, &fakeRunner{}, "schema", "serve", "--json")
	if out["ok"] != true {
		t.Fatalf("schema serve: %#v", out)
	}
	flags := map[string]bool{}
	for _, raw := range out["data"].(map[string]any)["flags"].([]any) {
		flags[raw.(map[string]any)["name"].(string)] = true
	}
	for _, name := range []string{"origin", "port", "session", "browser", "url", "register-protocol", "unregister-protocol", "json"} {
		if !flags[name] {
			t.Fatalf("schema serve missing flag %s: %v", name, flags)
		}
	}
}
