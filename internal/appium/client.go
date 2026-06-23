package appium

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browserstack"
	"engineering-flow-platform-tools/internal/mobile"
)

const w3cElementKey = "element-6066-11e4-a52e-4f735466cecf"
const maxErrorSnippet = 2048
const defaultHTTPTimeout = 10 * time.Minute

var authPattern = regexp.MustCompile(`(?i)\b(?:basic|bearer)\s+[A-Za-z0-9._~+/\-=]+`)

type Client struct {
	baseURL string
	http    *http.Client
	creds   browserstack.Credentials
}

func New(baseURL string, creds browserstack.Credentials, verifySSL bool, caCert string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://hub.browserstack.com/wd/hub"
	}
	if err := validateAppiumURL(baseURL); err != nil {
		return nil, err
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	tr.TLSClientConfig = &tls.Config{}
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = 60 * time.Second
	tr.IdleConnTimeout = 90 * time.Second
	if !verifySSL {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	if strings.TrimSpace(caCert) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, mobile.NewError("config_error", "invalid Appium CA certificate", "Provide a valid PEM ca_cert or remove it.", 400)
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: defaultHTTPTimeout, Transport: tr}, creds: creds}, nil
}

func validateAppiumURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return mobile.NewError("config_error", "invalid Appium hub URL", "Use an absolute https:// BrowserStack Appium hub URL.", 400)
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return mobile.NewError("config_error", "Appium hub URL must use https", "Only loopback HTTP is allowed for tests.", 400)
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".browserstack.com") && !strings.EqualFold(host, "hub.browserstack.com") && !isLoopbackHost(host) {
		return mobile.NewError("config_error", "off-provider Appium hub URL rejected", "Use hub.browserstack.com or a BrowserStack-owned host.", 400)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error) {
	if err := validateCreateSessionRequest(req); err != nil {
		return Session{}, err
	}
	caps := map[string]any{}
	for k, v := range req.ExtraCaps {
		caps[k] = v
	}
	if req.PlatformName != "" {
		caps["platformName"] = canonicalPlatform(req.PlatformName)
	}
	if req.AutomationName != "" {
		caps["appium:automationName"] = req.AutomationName
	}
	if req.App != "" {
		caps["appium:app"] = req.App
	}
	if req.DeviceName != "" {
		caps["appium:deviceName"] = req.DeviceName
	}
	if req.PlatformVersion != "" {
		caps["appium:platformVersion"] = req.PlatformVersion
	}
	if req.NewCommandTimeoutSeconds > 0 {
		caps["appium:newCommandTimeout"] = req.NewCommandTimeoutSeconds
	}
	bstack := map[string]any{}
	if req.ProjectName != "" {
		bstack["projectName"] = req.ProjectName
	}
	if req.BuildName != "" {
		bstack["buildName"] = req.BuildName
	}
	if req.SessionName != "" {
		bstack["sessionName"] = req.SessionName
	}
	bstack["interactiveDebugging"] = req.InteractiveDebugging
	if req.Debug {
		bstack["debug"] = true
	}
	bstack["video"] = req.Video
	if req.IdleTimeoutSeconds > 0 {
		bstack["idleTimeout"] = req.IdleTimeoutSeconds
	}
	switch req.NetworkMode {
	case "private-managed", "private-external":
		bstack["local"] = true
		if req.LocalIdentifier != "" {
			bstack["localIdentifier"] = req.LocalIdentifier
		}
	}
	if len(bstack) > 0 {
		caps["bstack:options"] = bstack
	}
	body := map[string]any{"capabilities": map[string]any{"alwaysMatch": caps}}
	var raw map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/session", body, &raw); err != nil {
		return Session{}, err
	}
	value, _ := raw["value"].(map[string]any)
	id := stringFrom(raw["sessionId"])
	if id == "" && value != nil {
		id = stringFrom(value["sessionId"])
	}
	if id == "" {
		id = stringFrom(raw["id"])
	}
	capabilities, _ := value["capabilities"].(map[string]any)
	if capabilities == nil {
		capabilities, _ = raw["capabilities"].(map[string]any)
	}
	if id == "" {
		return Session{}, mobile.NewError("session_creation_failed", "Appium session response did not include a session id", "Inspect BrowserStack Appium response shape.", 502)
	}
	return Session{ID: id, Capabilities: capabilities}, nil
}

func validateCreateSessionRequest(req CreateSessionRequest) error {
	if req.IdleTimeoutSeconds < 0 || req.IdleTimeoutSeconds > 300 {
		return mobile.NewError("invalid_args", "idle timeout must be between 1 and 300 seconds", "Set mobile.defaults.idle_timeout_seconds to a BrowserStack-supported value.", 400)
	}
	if req.NewCommandTimeoutSeconds < 0 {
		return mobile.NewError("invalid_args", "new command timeout cannot be negative", "Set mobile.defaults.new_command_timeout_seconds to a positive value.", 400)
	}
	return nil
}

func canonicalPlatform(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ios":
		return "iOS"
	default:
		return "Android"
	}
}

func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/session/"+url.PathEscape(sessionID), nil, nil)
}

func (c *Client) GetSource(ctx context.Context, sessionID string) (string, error) {
	var out valueResponse[string]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/source", nil, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

func (c *Client) Screenshot(ctx context.Context, sessionID string) ([]byte, error) {
	var out valueResponse[string]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/screenshot", nil, &out); err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(out.Value)
	if err != nil {
		return nil, mobile.NewError("server_error", "Appium screenshot response was not valid base64", "Retry observe or inspect the Appium response.", 502)
	}
	return b, nil
}

func (c *Client) FindElements(ctx context.Context, sessionID string, locator Locator) ([]RemoteElement, error) {
	var out valueResponse[[]map[string]any]
	body := map[string]any{"using": locator.Using, "value": locator.Value}
	if err := c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/elements", body, &out); err != nil {
		return nil, err
	}
	elements := make([]RemoteElement, 0, len(out.Value))
	for _, raw := range out.Value {
		if id := elementID(raw); id != "" {
			elements = append(elements, RemoteElement{ID: id})
		}
	}
	return elements, nil
}

func (c *Client) Click(ctx context.Context, sessionID, elementID string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/click", map[string]any{}, nil)
}

func (c *Client) Clear(ctx context.Context, sessionID, elementID string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/clear", map[string]any{}, nil)
}

func (c *Client) SendKeys(ctx context.Context, sessionID, elementID, text string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/value", map[string]any{"text": text}, nil)
}

func (c *Client) PerformActions(ctx context.Context, sessionID string, actions ActionsRequest) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/actions", actions, nil)
}

func (c *Client) Back(ctx context.Context, sessionID string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/back", map[string]any{}, nil)
}

func (c *Client) Contexts(ctx context.Context, sessionID string) ([]string, error) {
	var out valueResponse[[]string]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/contexts", nil, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

func (c *Client) SwitchContext(ctx context.Context, sessionID, contextName string) error {
	return c.doJSON(ctx, http.MethodPost, "/session/"+url.PathEscape(sessionID)+"/context", map[string]any{"name": contextName}, nil)
}

func (c *Client) SessionStatus(ctx context.Context, sessionID string) (map[string]any, error) {
	var raw map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID), nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) ElementDisplayed(ctx context.Context, sessionID, elementID string) (bool, error) {
	var out valueResponse[bool]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/displayed", nil, &out); err != nil {
		return false, err
	}
	return out.Value, nil
}

func (c *Client) ElementEnabled(ctx context.Context, sessionID, elementID string) (bool, error) {
	var out valueResponse[bool]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/enabled", nil, &out); err != nil {
		return false, err
	}
	return out.Value, nil
}

func (c *Client) ElementSelected(ctx context.Context, sessionID, elementID string) (bool, error) {
	var out valueResponse[bool]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/selected", nil, &out); err != nil {
		return false, err
	}
	return out.Value, nil
}

func (c *Client) ElementText(ctx context.Context, sessionID, elementID string) (string, error) {
	var out valueResponse[string]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/text", nil, &out); err != nil {
		return "", err
	}
	return out.Value, nil
}

func (c *Client) ElementRect(ctx context.Context, sessionID, elementID string) (Rect, error) {
	var out valueResponse[Rect]
	if err := c.doJSON(ctx, http.MethodGet, "/session/"+url.PathEscape(sessionID)+"/element/"+url.PathEscape(elementID)+"/rect", nil, &out); err != nil {
		return Rect{}, err
	}
	return out.Value, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(c.creds.Username, c.creds.AccessKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return mobile.RetryableError("network_error", "Appium request failed: "+sanitize(err.Error(), c.creds), "Check BrowserStack Appium hub connectivity.", "retry", 503)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return statusError(resp, c.creds)
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return mobile.NewError("server_error", "Appium returned invalid JSON", "Retry or inspect BrowserStack Appium logs.", 502)
	}
	return nil
}

type valueResponse[T any] struct {
	Value T `json:"value"`
}

func elementID(raw map[string]any) string {
	if id := stringFrom(raw[w3cElementKey]); id != "" {
		return id
	}
	return stringFrom(raw["ELEMENT"])
}

func stringFrom(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func statusError(resp *http.Response, creds browserstack.Credentials) *mobile.Error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorSnippet+1))
	msg := fmt.Sprintf("Appium request failed with HTTP %d", resp.StatusCode)
	if snippet := sanitize(string(body), creds); strings.TrimSpace(snippet) != "" {
		msg += ": " + snippet
	}
	code := "server_error"
	retryable := false
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = "auth_error"
	case http.StatusNotFound:
		code = "session_lost"
	case http.StatusTooManyRequests:
		code = "rate_limited"
		retryable = true
	default:
		if resp.StatusCode >= 500 {
			retryable = true
		}
	}
	return &mobile.Error{Code: code, Message: msg, Hint: "Inspect session status and BrowserStack Appium logs.", Status: resp.StatusCode, Retryable: retryable, RecommendedAction: "observe"}
}

func sanitize(s string, creds browserstack.Credentials) string {
	truncated := len(s) > maxErrorSnippet
	s = strings.ReplaceAll(s, creds.Username, "***REDACTED***")
	s = strings.ReplaceAll(s, creds.AccessKey, "***REDACTED***")
	s = authPattern.ReplaceAllString(s, "***REDACTED***")
	return boundedSnippet(s, truncated)
}

func boundedSnippet(s string, truncated bool) string {
	s = strings.TrimSpace(s)
	if !truncated && len(s) <= maxErrorSnippet {
		return s
	}
	if maxErrorSnippet <= 3 {
		if len(s) > maxErrorSnippet {
			return s[:maxErrorSnippet]
		}
		return s
	}
	limit := maxErrorSnippet - 3
	if len(s) > limit {
		s = strings.TrimSpace(s[:limit])
	}
	if truncated {
		return s + "..."
	}
	return s
}
