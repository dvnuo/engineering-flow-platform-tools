package splunk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/instance"
)

const (
	// DefaultMaxResults caps how many results one search may return when the
	// instance does not set max_results.
	DefaultMaxResults = 1000
	// DefaultEarliest is the earliest_time used when neither --earliest nor
	// the instance default_earliest is set.
	DefaultEarliest = "-1h"
	// DefaultLatest is the latest_time used when --latest is not set.
	DefaultLatest = "now"
	// JobTTLSeconds is the search job time-to-live (the Splunk `timeout`
	// job parameter) requested for every job the CLI creates.
	JobTTLSeconds = 600
	// DefaultRequestTimeout bounds one management REST round trip.
	DefaultRequestTimeout = 60 * time.Second

	maxErrorBodySnippet = 2048
)

// Request describes one management REST call. Path must be absolute under
// the instance base URL (for example /services/search/jobs). output_mode=json
// is added automatically: as a query parameter for GET/DELETE and as a form
// field for POST.
type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Form    url.Values
	Timeout time.Duration
}

// Client talks to the Splunk Enterprise management REST API of one instance.
// bearer_token instances send `Authorization: Bearer <token>`; basic_password
// instances log in once per process through /services/auth/login and keep the
// resulting session key in memory only.
type Client struct {
	inst    config.InstanceConfig
	http    *http.Client
	mu      sync.Mutex
	authHdr string
	secrets []string
}

// JobStatus is the compact view of a search job's entry[0].content.
type JobStatus struct {
	SID           string
	DispatchState string
	IsDone        bool
	IsFailed      bool
	DoneProgress  float64
	EventCount    int
	ResultCount   int
	ScanCount     int
	Messages      []map[string]any
	Content       map[string]any
}

var indexClausePattern = regexp.MustCompile(`(?i)(^|[\s(])index\s*(=|!=|\s+in\s*\()`)

// ResolveInstance selects the configured splunk instance.
func ResolveInstance(cfg config.RootConfig, explicit string) (config.InstanceConfig, error) {
	res, err := instance.Resolve(cfg.Splunk, explicit, "", "splunk")
	if err != nil {
		return config.InstanceConfig{}, err
	}
	return res.Instance, nil
}

// New builds a client for the instance and validates its base URL and auth
// type without sending anything.
func New(inst config.InstanceConfig) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(inst.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &httpclient.HTTPError{Code: "config_error", Message: "invalid splunk base_url", Hint: "Use the management URL of the instance, for example https://splunk-api.example.test:8089.", Status: 400}
	}
	inst.BaseURL = base
	verify := inst.VerifySSL == nil || *inst.VerifySSL
	tr, _, err := httpclient.NewTransport(httpclient.TransportOptions{BaseURL: base, VerifySSL: verify, CACert: inst.CACert})
	if err != nil {
		return nil, &httpclient.HTTPError{Code: "config_error", Message: "invalid splunk transport configuration: " + err.Error(), Hint: "Check verify_ssl, ca_cert, and proxy settings for the selected instance.", Status: 400}
	}
	inst.Auth.NormalizeType()
	c := &Client{inst: inst, http: &http.Client{Transport: tr}}
	switch inst.Auth.Type {
	case "bearer_token":
		if inst.Auth.Token == "" {
			return nil, authConfigError()
		}
		c.authHdr = "Bearer " + inst.Auth.Token
		c.secrets = append(c.secrets, inst.Auth.Token)
	case "basic_password":
		if inst.Auth.Username == "" || inst.Auth.Password == "" {
			return nil, authConfigError()
		}
		c.secrets = append(c.secrets, inst.Auth.Password)
	default:
		return nil, authConfigError()
	}
	return c, nil
}

func authConfigError() *httpclient.HTTPError {
	return &httpclient.HTTPError{Code: "config_error", Message: "splunk instance has no usable credentials", Hint: "Run splunk auth login with --token-stdin (authentication token) or --username plus --password-stdin (session login).", Status: 400}
}

// Instance returns the resolved instance configuration.
func (c *Client) Instance() config.InstanceConfig { return c.inst }

// MaxResults returns the instance result cap.
func (c *Client) MaxResults() int {
	if c.inst.MaxResults > 0 {
		return c.inst.MaxResults
	}
	return DefaultMaxResults
}

// EffectiveEarliest returns the earliest_time to use for a search.
func (c *Client) EffectiveEarliest(flag string) string {
	if v := strings.TrimSpace(flag); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.inst.DefaultEarliest); v != "" {
		return v
	}
	return DefaultEarliest
}

// ensureAuth performs the session login for basic_password instances on the
// first request. The session key never leaves the process.
func (c *Client) ensureAuth() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.authHdr != "" {
		return nil
	}
	form := url.Values{}
	form.Set("username", c.inst.Auth.Username)
	form.Set("password", c.inst.Auth.Password)
	resp, err := c.send(Request{Method: http.MethodPost, Path: "/services/auth/login", Form: form}, "")
	if err != nil {
		var httpErr *httpclient.HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusUnauthorized {
			httpErr.Message = "session login failed: " + httpErr.Message
			httpErr.Hint = "Check auth.username and auth.password for the selected instance, then run splunk auth test --json."
		}
		return err
	}
	defer resp.Body.Close()
	var body struct {
		SessionKey string `json:"sessionKey"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil || strings.TrimSpace(body.SessionKey) == "" {
		return &httpclient.HTTPError{Code: "auth_failed", Message: "session login returned no sessionKey", Hint: "Verify that base_url points at the Splunk management port (usually 8089), not the web UI.", Status: 401}
	}
	c.authHdr = "Splunk " + body.SessionKey
	c.secrets = append(c.secrets, body.SessionKey)
	return nil
}

// Do sends an authenticated request. Responses with status >= 400 return
// both the response and an *httpclient.HTTPError with a stable code.
func (c *Client) Do(r Request) (*http.Response, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	authHdr := c.authHdr
	c.mu.Unlock()
	return c.send(r, authHdr)
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (c *Client) send(r Request, authHdr string) (*http.Response, error) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !strings.HasPrefix(r.Path, "/") {
		return nil, &httpclient.HTTPError{Code: "invalid_args", Message: "REST path must start with /", Hint: "Use a path under /services/ or /servicesNS/.", Status: 400}
	}
	query := url.Values{}
	for k, vs := range r.Query {
		for _, v := range vs {
			query.Add(k, v)
		}
	}
	var body io.Reader
	contentType := ""
	if method == http.MethodPost || method == http.MethodPut {
		form := url.Values{}
		for k, vs := range r.Form {
			for _, v := range vs {
				form.Add(k, v)
			}
		}
		form.Set("output_mode", "json")
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	} else {
		query.Set("output_mode", "json")
	}
	full := c.inst.BaseURL + r.Path
	if enc := query.Encode(); enc != "" {
		full += "?" + enc
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		cancel()
		return nil, &httpclient.HTTPError{Code: "invalid_args", Message: "invalid request: " + c.sanitize(err.Error()), Status: 400}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if authHdr != "" {
		req.Header.Set("Authorization", authHdr)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &httpclient.HTTPError{Code: "wait_timeout", Message: fmt.Sprintf("Splunk did not answer %s %s within %s", method, r.Path, timeout), Hint: "Narrow the time range or query, or increase --timeout-sec.", Status: 408}
		}
		return nil, &httpclient.HTTPError{Code: "network_error", Message: "request failed: " + c.sanitize(err.Error()), Hint: "Check network connectivity, proxy settings, and the selected instance base_url (management port, usually 8089)."}
	}
	resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: cancel}
	if resp.StatusCode >= 400 {
		return resp, c.statusError(resp)
	}
	return resp, nil
}

func (c *Client) statusError(resp *http.Response) *httpclient.HTTPError {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySnippet+1))
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	message := fmt.Sprintf("request failed with HTTP %d", resp.StatusCode)
	if text := ErrorText(raw); text != "" {
		message += ": " + c.sanitize(text)
	} else if snippet := c.sanitize(string(raw)); snippet != "" {
		message += ": " + snippet
	}
	code, hint := mapStatus(resp.StatusCode)
	return &httpclient.HTTPError{Code: code, Message: message, Hint: hint, Status: resp.StatusCode}
}

func mapStatus(status int) (string, string) {
	switch status {
	case http.StatusUnauthorized:
		return "auth_failed", "The Splunk token expired or the session is invalid; store fresh credentials with splunk auth login and verify with splunk auth test --json."
	case http.StatusForbidden:
		return "permission_denied", "The Splunk role lacks the capability or index access this request needs."
	case http.StatusNotFound:
		return "not_found", "Verify the sid, saved search name, or REST path on the selected instance; search jobs expire after their TTL."
	case http.StatusConflict:
		return "conflict", ""
	case http.StatusTooManyRequests:
		return "rate_limited", "Retry after a short delay."
	}
	if status >= 500 {
		return "server_error", ""
	}
	return "invalid_args", "Check the SPL, time modifiers, and parameters; see error.message for the Splunk text."
}

// ErrorText extracts the ERROR/FATAL message texts from a Splunk JSON error
// body ({"messages":[{"type":"ERROR","text":"..."}]}).
func ErrorText(raw []byte) string {
	var body struct {
		Messages []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	var texts []string
	for _, m := range body.Messages {
		if t := strings.TrimSpace(m.Text); t != "" {
			texts = append(texts, t)
		}
	}
	return strings.Join(texts, "; ")
}

func (c *Client) sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 {
			return -1
		}
		return r
	}, s)
	for _, secret := range c.secrets {
		if strings.TrimSpace(secret) != "" {
			s = strings.ReplaceAll(s, secret, "***REDACTED***")
		}
	}
	if len(s) > maxErrorBodySnippet {
		s = s[:maxErrorBodySnippet] + "..."
	}
	return httpclient.SanitizeErrorText(strings.TrimSpace(s))
}

// DoJSON sends the request and decodes a JSON object response.
func (c *Client) DoJSON(r Request) (map[string]any, error) {
	v, err := c.DoValue(r)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	return m, nil
}

// DoValue sends the request and decodes any JSON response value.
func (c *Client) DoValue(r Request) (any, error) {
	resp, err := c.Do(r)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return map[string]any{}, nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &httpclient.HTTPError{Code: "network_error", Message: "reading response failed: " + c.sanitize(err.Error())}
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, &httpclient.HTTPError{Code: "server_error", Message: "Splunk returned a non-JSON response: " + c.sanitize(string(raw)), Hint: "Verify that base_url points at the management port (usually 8089).", Status: 502}
	}
	return v, nil
}

// PrepareSearch normalizes the SPL string that is submitted as the job's
// `search` parameter: leading/trailing whitespace is trimmed, `search ` is
// prepended when the query starts with neither `search` nor `|`, and
// `index=<defaultIndex>` is injected when defaultIndex is set and the query
// does not already constrain the index. Generating searches (starting with
// `|`) are never rewritten.
func PrepareSearch(query, defaultIndex string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if strings.HasPrefix(q, "|") {
		return q
	}
	keyword := "search"
	body := q
	if len(q) > len(keyword) && strings.EqualFold(q[:len(keyword)], keyword) && (q[len(keyword)] == ' ' || q[len(keyword)] == '\t') {
		keyword = q[:len(keyword)]
		body = strings.TrimSpace(q[len(keyword):])
	}
	if idx := strings.TrimSpace(defaultIndex); idx != "" && !indexClausePattern.MatchString(body) {
		body = "index=" + idx + " " + body
	}
	return keyword + " " + body
}

// JobForm builds the form for POST /services/search/jobs.
func JobForm(search, earliest, latest, execMode string, maxCount int) url.Values {
	form := url.Values{}
	form.Set("search", search)
	form.Set("earliest_time", earliest)
	form.Set("latest_time", latest)
	form.Set("exec_mode", execMode)
	form.Set("max_count", strconv.Itoa(maxCount))
	form.Set("status_buckets", "0")
	form.Set("timeout", strconv.Itoa(JobTTLSeconds))
	form.Set("adhoc_search_level", "smart")
	return form
}

// CreateJob submits a normal-mode search job and returns its sid.
func (c *Client) CreateJob(form url.Values) (string, error) {
	res, err := c.DoJSON(Request{Method: http.MethodPost, Path: "/services/search/jobs", Form: form})
	if err != nil {
		return "", err
	}
	sid := strings.TrimSpace(asString(res["sid"]))
	if sid == "" {
		return "", &httpclient.HTTPError{Code: "server_error", Message: "search job creation returned no sid", Status: 502}
	}
	return sid, nil
}

// Oneshot runs a oneshot search and returns the raw results document.
func (c *Client) Oneshot(form url.Values, timeout time.Duration) (map[string]any, error) {
	return c.DoJSON(Request{Method: http.MethodPost, Path: "/services/search/jobs", Form: form, Timeout: timeout})
}

// GetJob reads the job status entry.
func (c *Client) GetJob(sid string) (JobStatus, error) {
	res, err := c.DoJSON(Request{Method: http.MethodGet, Path: "/services/search/jobs/" + url.PathEscape(sid)})
	if err != nil {
		return JobStatus{SID: sid}, err
	}
	content := firstEntryContent(res)
	st := JobStatus{
		SID:           sid,
		DispatchState: asString(content["dispatchState"]),
		IsDone:        asBool(content["isDone"]),
		IsFailed:      asBool(content["isFailed"]),
		DoneProgress:  asFloat(content["doneProgress"]),
		EventCount:    asInt(content["eventCount"]),
		ResultCount:   asInt(content["resultCount"]),
		ScanCount:     asInt(content["scanCount"]),
		Messages:      Messages(content["messages"]),
		Content:       content,
	}
	if st.DispatchState == "" {
		st.DispatchState = "UNKNOWN"
	}
	return st, nil
}

// JobResults reads /results (finished jobs) or /events (running jobs).
func (c *Client) JobResults(sid, endpoint string, count, offset int, fields []string) (map[string]any, error) {
	q := url.Values{}
	q.Set("count", strconv.Itoa(count))
	q.Set("offset", strconv.Itoa(offset))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			q.Add("f", f)
		}
	}
	return c.DoJSON(Request{Method: http.MethodGet, Path: "/services/search/jobs/" + url.PathEscape(sid) + "/" + endpoint, Query: q})
}

// ControlJob posts a control action (cancel, finalize, pause, unpause, touch).
func (c *Client) ControlJob(sid, action string) error {
	form := url.Values{}
	form.Set("action", action)
	_, err := c.DoJSON(Request{Method: http.MethodPost, Path: "/services/search/jobs/" + url.PathEscape(sid) + "/control", Form: form})
	return err
}

// SavedSearches lists saved search entries, optionally filtered by name text.
func (c *Client) SavedSearches(count, offset int, filter string) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("count", strconv.Itoa(count))
	q.Set("offset", strconv.Itoa(offset))
	if filter = strings.TrimSpace(filter); filter != "" {
		q.Set("search", filter)
	}
	res, err := c.DoJSON(Request{Method: http.MethodGet, Path: "/services/saved/searches", Query: q})
	if err != nil {
		return nil, err
	}
	return Entries(res), nil
}

// SavedSearch reads one saved search entry by name.
func (c *Client) SavedSearch(name string) (map[string]any, error) {
	res, err := c.DoJSON(Request{Method: http.MethodGet, Path: "/services/saved/searches/" + url.PathEscape(name)})
	if err != nil {
		return nil, err
	}
	entries := Entries(res)
	if len(entries) == 0 {
		return nil, &httpclient.HTTPError{Code: "not_found", Message: "saved search not found", Hint: "Run splunk saved list --json to see the saved searches visible to this account.", Status: 404}
	}
	return entries[0], nil
}

// DispatchSaved dispatches a saved search and returns the job sid.
func (c *Client) DispatchSaved(name string, form url.Values) (string, error) {
	res, err := c.DoJSON(Request{Method: http.MethodPost, Path: "/services/saved/searches/" + url.PathEscape(name) + "/dispatch", Form: form})
	if err != nil {
		return "", err
	}
	sid := strings.TrimSpace(asString(res["sid"]))
	if sid == "" {
		return "", &httpclient.HTTPError{Code: "server_error", Message: "saved search dispatch returned no sid", Status: 502}
	}
	return sid, nil
}

// Indexes lists index entries.
func (c *Client) Indexes(count int) ([]map[string]any, error) {
	q := url.Values{}
	q.Set("count", strconv.Itoa(count))
	res, err := c.DoJSON(Request{Method: http.MethodGet, Path: "/services/data/indexes", Query: q})
	if err != nil {
		return nil, err
	}
	return Entries(res), nil
}

// CurrentContext returns entry[0].content of /services/authentication/current-context.
func (c *Client) CurrentContext() (map[string]any, error) {
	res, err := c.DoJSON(Request{Method: http.MethodGet, Path: "/services/authentication/current-context"})
	if err != nil {
		return nil, err
	}
	return firstEntryContent(res), nil
}

// Entries returns the entry[] objects of a Splunk collection response.
func Entries(res map[string]any) []map[string]any {
	raw, _ := res["entry"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// EntryContent returns the content object of one entry.
func EntryContent(entry map[string]any) map[string]any {
	content, _ := entry["content"].(map[string]any)
	if content == nil {
		return map[string]any{}
	}
	return content
}

func firstEntryContent(res map[string]any) map[string]any {
	entries := Entries(res)
	if len(entries) == 0 {
		return map[string]any{}
	}
	return EntryContent(entries[0])
}

// Messages normalizes a Splunk messages list to [{type, text}].
func Messages(v any) []map[string]any {
	out := []map[string]any{}
	raw, _ := v.([]any)
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{"type": asString(m["type"]), "text": asString(m["text"])})
	}
	return out
}

// Results returns the results[] objects of a results document.
func Results(res map[string]any) []map[string]any {
	raw, _ := res["results"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(x))
		return b
	case float64:
		return x != 0
	default:
		return false
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	case bool:
		if x {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func asInt(v any) int {
	return int(asFloat(v))
}
