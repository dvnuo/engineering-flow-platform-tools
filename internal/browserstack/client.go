package browserstack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/mobile"
)

const maxErrorSnippet = 2048

var authPattern = regexp.MustCompile(`(?i)\b(?:basic|bearer)\s+[A-Za-z0-9._~+/\-=]+`)

type Client struct {
	baseURL string
	http    *http.Client
	creds   Credentials
}

func New(baseURL string, creds Credentials, verifySSL bool, caCert string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api-cloud.browserstack.com"
	}
	if err := validateBrowserStackURL(baseURL, "api-cloud.browserstack.com"); err != nil {
		return nil, err
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	tr.TLSClientConfig = &tls.Config{}
	if !verifySSL {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	if strings.TrimSpace(caCert) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, mobile.NewError("config_error", "invalid BrowserStack CA certificate", "Provide a valid PEM ca_cert or remove it.", 400)
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return &Client{baseURL: baseURL, http: &http.Client{Transport: tr}, creds: creds}, nil
}

func validateBrowserStackURL(raw, host string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return mobile.NewError("config_error", "invalid BrowserStack URL", "Use an absolute https:// BrowserStack URL.", 400)
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return mobile.NewError("config_error", "BrowserStack URLs must use https", "Only loopback HTTP is allowed for tests.", 400)
	}
	if !strings.EqualFold(u.Hostname(), host) && !strings.HasSuffix(strings.ToLower(u.Hostname()), ".browserstack.com") && !isLoopbackHost(u.Hostname()) {
		return mobile.NewError("config_error", "off-provider BrowserStack URL rejected", "Use api-cloud.browserstack.com or a BrowserStack-owned host.", 400)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (c *Client) AuthTest(ctx context.Context) error {
	var plan Plan
	return c.get(ctx, "/app-automate/plan.json", nil, &plan)
}

func (c *Client) UploadApp(ctx context.Context, req UploadAppRequest) (UploadedApp, error) {
	if (strings.TrimSpace(req.FilePath) == "") == (strings.TrimSpace(req.URL) == "") {
		return UploadedApp{}, mobile.NewError("invalid_args", "exactly one of --file or --url is required", "Pass a local app file or a public app URL.", 400)
	}
	body, contentType, err := uploadAppBody(req)
	if err != nil {
		return UploadedApp{}, err
	}
	defer body.Close()
	var out UploadedApp
	if err := c.doJSON(ctx, http.MethodPost, "/app-automate/upload", nil, contentType, body, &out); err != nil {
		return UploadedApp{}, err
	}
	out.SHA256 = req.SHA256
	return out, nil
}

func uploadAppBody(req UploadAppRequest) (io.ReadCloser, string, error) {
	if req.FilePath != "" {
		return streamingUploadAppBody(req)
	}
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	if err := writeUploadAppFields(mw, req); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return io.NopCloser(buf), mw.FormDataContentType(), nil
}

func streamingUploadAppBody(req UploadAppRequest) (io.ReadCloser, string, error) {
	f, err := os.Open(req.FilePath)
	if err != nil {
		return nil, "", mobile.NewError("invalid_args", "app file could not be opened", "Check --file points to a readable .apk, .aab, .xapk, or .ipa file.", 400)
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		defer f.Close()
		if err := writeUploadAppFile(mw, req, f); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := mw.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()
	return pr, mw.FormDataContentType(), nil
}

func writeUploadAppFile(mw *multipart.Writer, req UploadAppRequest, f *os.File) error {
	fw, err := mw.CreateFormFile("file", filepath.Base(req.FilePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return err
	}
	return writeUploadAppFields(mw, req)
}

func writeUploadAppFields(mw *multipart.Writer, req UploadAppRequest) error {
	if req.URL != "" {
		if err := mw.WriteField("url", req.URL); err != nil {
			return err
		}
	}
	if req.CustomID != "" {
		if err := mw.WriteField("custom_id", req.CustomID); err != nil {
			return err
		}
	}
	if req.IOSKeychainSupport {
		if err := mw.WriteField("ios_keychain_support", "true"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ListApps(ctx context.Context, req ListAppsRequest) ([]UploadedApp, error) {
	path := "/app-automate/recent_apps"
	q := url.Values{}
	if req.Group {
		path = "/app-automate/recent_group_apps"
	}
	if req.CustomID != "" && !req.Group {
		path += "/" + url.PathEscape(req.CustomID)
	} else if req.CustomID != "" {
		q.Set("custom_id", req.CustomID)
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	if req.Offset > 0 {
		q.Set("offset", strconv.Itoa(req.Offset))
	}
	var apps []UploadedApp
	if err := c.get(ctx, path, q, &apps); err != nil {
		return nil, err
	}
	return apps, nil
}

func (c *Client) DeleteApp(ctx context.Context, appID string) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodDelete, "/app-automate/app/delete/"+url.PathEscape(appID), nil, "", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var devices []Device
	if err := c.get(ctx, "/app-automate/devices.json", nil, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func (c *Client) DeviceTierUsage(ctx context.Context) ([]map[string]any, error) {
	var usage []map[string]any
	if err := c.get(ctx, "/app-automate/device_tier_limits.json", nil, &usage); err != nil {
		return nil, err
	}
	return usage, nil
}

func (c *Client) GetPlan(ctx context.Context) (Plan, error) {
	var raw map[string]any
	if err := c.get(ctx, "/app-automate/plan.json", nil, &raw); err != nil {
		return Plan{}, err
	}
	b, _ := json.Marshal(raw)
	var plan Plan
	_ = json.Unmarshal(b, &plan)
	plan.Raw = raw
	return plan, nil
}

func (c *Client) CurrentParallelQueueUsage(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.get(ctx, "/app-automate/current_parallel_queue_usage", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListProjects(ctx context.Context, limit, offset int, status string) ([]Project, error) {
	var raw []map[string]any
	q := pagingQuery(limit, offset, status)
	if err := c.get(ctx, "/app-automate/projects.json", q, &raw); err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(raw))
	for _, item := range raw {
		projectRaw, _ := item["automation_project"].(map[string]any)
		if projectRaw == nil {
			projectRaw = item
		}
		out = append(out, Project{Name: stringValue(projectRaw["name"]), Status: stringValue(projectRaw["status"]), ID: projectRaw["id"], HashedID: stringValue(projectRaw["hashed_id"]), Duration: projectRaw["duration"], Raw: projectRaw})
	}
	return out, nil
}

func (c *Client) GetProject(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.get(ctx, "/app-automate/projects/"+url.PathEscape(id)+".json", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListBuilds(ctx context.Context, limit, offset int, status, projectID string) ([]Build, error) {
	var raw []map[string]any
	q := pagingQuery(limit, offset, status)
	if projectID != "" {
		q.Set("projectId", projectID)
	}
	if err := c.get(ctx, "/app-automate/builds.json", q, &raw); err != nil {
		return nil, err
	}
	out := make([]Build, 0, len(raw))
	for _, item := range raw {
		buildRaw, _ := item["automation_build"].(map[string]any)
		if buildRaw == nil {
			buildRaw = item
		}
		out = append(out, Build{Name: stringValue(buildRaw["name"]), Status: stringValue(buildRaw["status"]), HashedID: stringValue(buildRaw["hashed_id"]), Duration: buildRaw["duration"], Raw: buildRaw})
	}
	return out, nil
}

func (c *Client) ListBuildSessions(ctx context.Context, buildID string, limit, offset int, status string) ([]Session, error) {
	var raw []map[string]any
	if err := c.get(ctx, "/app-automate/builds/"+url.PathEscape(buildID)+"/sessions.json", pagingQuery(limit, offset, status), &raw); err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(raw))
	for _, item := range raw {
		sessionRaw, _ := item["automation_session"].(map[string]any)
		if sessionRaw == nil {
			sessionRaw = item
		}
		out = append(out, decodeSession(sessionRaw))
	}
	return out, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var raw map[string]any
	if err := c.get(ctx, "/app-automate/sessions/"+url.PathEscape(sessionID)+".json", nil, &raw); err != nil {
		return Session{}, err
	}
	sessionRaw, _ := raw["automation_session"].(map[string]any)
	if sessionRaw == nil {
		sessionRaw = raw
	}
	return decodeSession(sessionRaw), nil
}

func (c *Client) UpdateSession(ctx context.Context, sessionID string, req UpdateSessionRequest) (Session, error) {
	var raw map[string]any
	if err := c.doJSON(ctx, http.MethodPut, "/app-automate/sessions/"+url.PathEscape(sessionID)+".json", nil, "application/json", jsonBody(req), &raw); err != nil {
		return Session{}, err
	}
	sessionRaw, _ := raw["automation_session"].(map[string]any)
	if sessionRaw == nil {
		sessionRaw = raw
	}
	return decodeSession(sessionRaw), nil
}

func (c *Client) DeleteSessions(ctx context.Context, sessionIDs []string) (map[string]any, error) {
	q := url.Values{}
	for _, id := range sessionIDs {
		q.Add("sessionId", id)
	}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodDelete, "/app-automate/sessions", q, "", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DownloadArtifact(ctx context.Context, buildID, sessionID, kind string) ([]byte, string, error) {
	allowed := map[string]string{
		"appiumlogs":  "appiumlogs",
		"devicelogs":  "devicelogs",
		"crashlogs":   "crashlogs",
		"networklogs": "networklogs",
	}
	path, ok := allowed[kind]
	if !ok {
		return nil, "", mobile.NewError("invalid_args", "unsupported artifact kind", "Use appiumlogs, devicelogs, crashlogs, or networklogs.", 400)
	}
	resp, err := c.do(ctx, http.MethodGet, "/app-automate/builds/"+url.PathEscape(buildID)+"/sessions/"+url.PathEscape(sessionID)+"/"+path, nil, "", nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("Content-Type"), nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, "", nil, out)
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, contentType string, body io.Reader, out any) error {
	resp, err := c.do(ctx, method, path, query, contentType, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return mobile.NewError("server_error", "BrowserStack returned invalid JSON", "Retry or inspect BrowserStack status.", 502)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.SetBasicAuth(c.creds.Username, c.creds.AccessKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, mobile.RetryableError("network_error", "BrowserStack request failed: "+redact(err.Error(), c.creds), "Check DNS, proxy, TLS, and BrowserStack availability.", "retry", 503)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, statusError(resp, c.creds)
	}
	return resp, nil
}

func statusError(resp *http.Response, creds Credentials) *mobile.Error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorSnippet+1))
	msg := fmt.Sprintf("BrowserStack request failed with HTTP %d", resp.StatusCode)
	if snippet := sanitizeSnippet(body, creds); snippet != "" {
		msg += ": " + snippet
	}
	code := "server_error"
	retryable := false
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = "auth_error"
	case http.StatusTooManyRequests:
		code = "rate_limited"
		retryable = true
	case http.StatusNotFound:
		code = "not_found"
	default:
		if resp.StatusCode >= 500 {
			retryable = true
		}
	}
	return &mobile.Error{Code: code, Message: msg, Hint: "Inspect BrowserStack credentials, resource IDs, and service status.", Status: resp.StatusCode, Retryable: retryable, RecommendedAction: "retry"}
}

func sanitizeSnippet(body []byte, creds Credentials) string {
	truncated := len(body) > maxErrorSnippet
	if len(body) > maxErrorSnippet {
		body = body[:maxErrorSnippet]
	}
	s := strings.TrimSpace(string(body))
	s = strings.ReplaceAll(s, creds.Username, "***REDACTED***")
	s = strings.ReplaceAll(s, creds.AccessKey, "***REDACTED***")
	s = authPattern.ReplaceAllString(s, "***REDACTED***")
	return boundedSnippet(s, truncated)
}

func redact(s string, creds Credentials) string {
	return sanitizeSnippet([]byte(s), creds)
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

func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

func pagingQuery(limit, offset int, status string) url.Values {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	if status != "" {
		q.Set("status", status)
	}
	return q
}

func decodeSession(raw map[string]any) Session {
	return Session{
		Name:                  stringValue(raw["name"]),
		Duration:              raw["duration"],
		OS:                    stringValue(raw["os"]),
		OSVersion:             stringValue(raw["os_version"]),
		BrowserVersion:        stringValue(raw["browser_version"]),
		Browser:               raw["browser"],
		Device:                stringValue(raw["device"]),
		Status:                stringValue(raw["status"]),
		HashedID:              stringValue(raw["hashed_id"]),
		Reason:                stringValue(raw["reason"]),
		BuildName:             stringValue(raw["build_name"]),
		ProjectName:           stringValue(raw["project_name"]),
		Logs:                  stringValue(raw["logs"]),
		BrowserURL:            stringValue(raw["browser_url"]),
		PublicURL:             stringValue(raw["public_url"]),
		AppiumLogsURL:         stringValue(raw["appium_logs_url"]),
		VideoURL:              stringValue(raw["video_url"]),
		DeviceLogsURL:         stringValue(raw["device_logs_url"]),
		CrashLogsURL:          stringValue(raw["crash_logs_url"]),
		NetworkLogsURL:        stringValue(raw["network_logs_url"]),
		BrowserConsoleLogsURL: stringValue(raw["browser_console_logs_url"]),
		HARLogsURL:            stringValue(raw["har_logs_url"]),
		AppDetails:            mapValue(raw["app_details"]),
		Raw:                   raw,
	}
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return ""
	}
}

func mapValue(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ValidAppExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".apk", ".aab", ".xapk", ".ipa":
		return true
	default:
		return false
	}
}

func AppIDFromURL(appURL string) string {
	return strings.TrimPrefix(strings.TrimSpace(appURL), "bs://")
}
