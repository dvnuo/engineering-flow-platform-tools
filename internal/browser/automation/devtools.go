package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browser/probe"
)

type DevToolsClient struct {
	Addr       string
	Port       int
	BaseURL    string
	HTTPClient *http.Client
}

func NewDevToolsClient(addr string, port int) *DevToolsClient {
	if strings.TrimSpace(addr) == "" {
		addr = LocalDebugAddr
	}
	return &DevToolsClient{
		Addr:       addr,
		Port:       port,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *DevToolsClient) Version(ctx context.Context) (VersionInfo, error) {
	var raw devToolsVersionJSON
	if err := c.getJSON(ctx, "/json/version", &raw); err != nil {
		return VersionInfo{}, err
	}
	return VersionInfo{
		Browser:              raw.Browser,
		ProtocolVersion:      raw.ProtocolVersion,
		UserAgent:            raw.UserAgent,
		V8Version:            raw.V8Version,
		WebKitVersion:        raw.WebKitVersion,
		WebSocketDebuggerURL: raw.WebSocketDebuggerURL,
	}, nil
}

func (c *DevToolsClient) ListTargets(ctx context.Context) ([]Target, error) {
	var raw []devToolsTargetJSON
	if err := c.getJSON(ctx, "/json/list", &raw); err != nil {
		return nil, err
	}
	out := make([]Target, 0, len(raw))
	for _, item := range raw {
		out = append(out, Target{
			ID:                   item.ID,
			Type:                 item.Type,
			Title:                item.Title,
			URL:                  item.URL,
			WebSocketDebuggerURL: item.WebSocketDebuggerURL,
		})
	}
	return out, nil
}

func (c *DevToolsClient) Activate(ctx context.Context, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return invalidArgs("--target-id is required.", "Run browser tab list --json and pass the page target id.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlPath("/json/activate/"+url.PathEscape(targetID)), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return NewError("devtools_unavailable", err.Error(), "Check browser session status.", 503)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return NewError("target_not_found", fmt.Sprintf("DevTools activate returned HTTP %d.", resp.StatusCode), "Run browser tab list --json and choose an existing page target.", 404)
	}
	return nil
}

func (c *DevToolsClient) Open(ctx context.Context, rawURL string) (Target, error) {
	if err := validateHTTPURL(rawURL, "--url"); err != nil {
		return Target{}, err
	}
	targetURL := c.urlPath("/json/new") + "?" + url.QueryEscape(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, nil)
	if err != nil {
		return Target{}, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Target{}, NewError("devtools_unavailable", err.Error(), "Check browser session status.", 503)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return c.openWithGET(ctx, rawURL)
	}
	if resp.StatusCode >= 400 {
		return Target{}, NewError("automation_failed", fmt.Sprintf("DevTools new tab returned HTTP %d.", resp.StatusCode), "Check the target URL and browser DevTools status.", 500)
	}
	return decodeTarget(resp)
}

func (c *DevToolsClient) openWithGET(ctx context.Context, rawURL string) (Target, error) {
	targetURL := c.urlPath("/json/new") + "?" + url.QueryEscape(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return Target{}, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Target{}, NewError("devtools_unavailable", err.Error(), "Check browser session status.", 503)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Target{}, NewError("automation_failed", fmt.Sprintf("DevTools new tab returned HTTP %d.", resp.StatusCode), "Check the target URL and browser DevTools status.", 500)
	}
	return decodeTarget(resp)
}

func decodeTarget(resp *http.Response) (Target, error) {
	var raw devToolsTargetJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Target{}, NewError("automation_failed", err.Error(), "DevTools tab response was not valid JSON.", 500)
	}
	return Target{
		ID:                   raw.ID,
		Type:                 raw.Type,
		Title:                raw.Title,
		URL:                  raw.URL,
		WebSocketDebuggerURL: raw.WebSocketDebuggerURL,
	}, nil
}

func (c *DevToolsClient) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlPath(path), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return NewError("devtools_unavailable", err.Error(), "Check whether the browser session is still running.", 503)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return NewError("devtools_unavailable", fmt.Sprintf("DevTools endpoint returned HTTP %d.", resp.StatusCode), "Check whether the browser session is still running.", 503)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return NewError("devtools_unavailable", err.Error(), "DevTools endpoint did not return valid JSON.", 503)
	}
	return nil
}

func (c *DevToolsClient) urlPath(path string) string {
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/") + path
	}
	return fmt.Sprintf("http://%s:%d%s", c.Addr, c.Port, path)
}

func (c *DevToolsClient) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func RedactedTarget(t Target) Target {
	t.Title = probe.RedactText(t.Title)
	t.URL = probe.RedactURL(t.URL)
	return t
}

func PageTargets(targets []Target) []Target {
	out := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.Type == "page" {
			out = append(out, target)
		}
	}
	return out
}
