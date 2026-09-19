// Package appd is the AppDynamics classic Controller REST client used by the
// appd CLI. It is read-only: every request is a GET under /controller/rest/
// except the OAuth token exchange that API Client credentials require.
package appd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
)

// Auth types the appd client understands. api_client is the OAuth
// client-credentials grant of AppDynamics API Clients (username = API client
// name, api_key = client secret); basic_password is user@account basic
// authentication; bearer_token is a pre-issued token such as a temporary
// access token generated in the Controller UI (config or env only).
const (
	AuthAPIClient = "api_client"
	AuthBasic     = "basic_password"
	AuthBearer    = "bearer_token"
)

const (
	// OAuthTokenPath is the Controller endpoint that exchanges API Client
	// credentials for a short-lived bearer token.
	OAuthTokenPath = "/controller/api/oauth/access_token"
	// RESTPrefix is the only path prefix appd api get may call.
	RESTPrefix = "/controller/rest/"

	maxErrorSnippet   = 2048
	maxTokenBody      = 64 << 10
	maxResponseBytes  = 64 << 20
	defaultTokenTTL   = 5 * time.Minute
	tokenExpiryGrace  = 30 * time.Second
	requestTimeout    = 60 * time.Second
	redactedPlacehold = "***REDACTED***"
)

// Client talks to one configured Controller instance. Secrets (client secret,
// password, bearer token) and the cached OAuth access token live only in
// memory for the lifetime of the process and are never returned by any method.
type Client struct {
	inst     config.InstanceConfig
	base     string
	http     *http.Client
	authType string
	identity string // api_client: <name>@<account>; basic: <user>@<account>; bearer: ""
	secret   string
	now      func() time.Time

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// New builds a client for the instance. The base URL may or may not end with
// /controller; the client always appends the full /controller/... path itself.
func New(inst config.InstanceConfig) (*Client, error) {
	base, err := NormalizeBaseURL(inst.BaseURL)
	if err != nil {
		return nil, err
	}
	verify := inst.VerifySSL == nil || *inst.VerifySSL
	tr, _, err := httpclient.NewTransport(httpclient.TransportOptions{
		BaseURL:               base,
		VerifySSL:             verify,
		CACert:                inst.CACert,
		ResponseHeaderTimeout: requestTimeout,
	})
	if err != nil {
		return nil, configError("invalid AppDynamics HTTP transport configuration: "+err.Error(), "Check base_url, verify_ssl, and ca_cert of the selected appd instance.")
	}
	c := &Client{inst: inst, base: base, http: &http.Client{Timeout: requestTimeout, Transport: tr}, now: time.Now}
	if err := c.configureAuth(inst); err != nil {
		return nil, err
	}
	return c, nil
}

// NormalizeBaseURL validates the configured base URL and strips a trailing
// slash and a trailing /controller segment so paths can always start with
// /controller/.
func NormalizeBaseURL(raw string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", configError("invalid appd base_url", "Use an absolute http(s) Controller URL such as https://appd.example.test:8090.")
	}
	if strings.HasSuffix(strings.ToLower(base), "/controller") {
		base = base[:len(base)-len("/controller")]
	}
	return strings.TrimRight(base, "/"), nil
}

func (c *Client) configureAuth(inst config.InstanceConfig) error {
	auth := inst.Auth
	auth.NormalizeType()
	account := strings.TrimSpace(inst.Account)
	switch auth.Type {
	case AuthAPIClient:
		name := strings.TrimSpace(auth.Username)
		if name == "" || auth.APIKey == "" {
			return configError("appd api_client auth needs auth.username (API client name) and auth.api_key (client secret)", "Run appd auth login --auth-type api_client --username <api-client-name> --api-key-stdin.")
		}
		id, err := qualify(name, account, "API client name")
		if err != nil {
			return err
		}
		c.authType, c.identity, c.secret = AuthAPIClient, id, auth.APIKey
	case AuthBasic:
		user := strings.TrimSpace(auth.Username)
		if user == "" || auth.Password == "" {
			return configError("appd basic_password auth needs auth.username and auth.password", "Run appd auth login --auth-type basic_password --username <user> --password-stdin.")
		}
		id, err := qualify(user, account, "username")
		if err != nil {
			return err
		}
		c.authType, c.identity, c.secret = AuthBasic, id, auth.Password
	case AuthBearer:
		if strings.TrimSpace(auth.Token) == "" {
			return configError("appd bearer_token auth needs auth.token", "Store a Controller access token in auth.token or switch to api_client credentials.")
		}
		c.authType, c.identity, c.secret = AuthBearer, "", strings.TrimSpace(auth.Token)
	case "":
		return configError("no AppDynamics credentials configured for instance "+inst.Name, "Run appd auth login --auth-type api_client --username <api-client-name> --api-key-stdin (or --auth-type basic_password --username <user> --password-stdin).")
	default:
		return configError("auth type "+auth.Type+" is not supported by appd", "Set auth.type to api_client (username = API client name, api_key = client secret), basic_password (username and password), or bearer_token.")
	}
	return nil
}

// qualify appends @account to a bare API client name or username.
func qualify(name, account, label string) (string, error) {
	if strings.Contains(name, "@") {
		return name, nil
	}
	if account == "" {
		return "", configError("appd instance has no account, so the "+label+" "+name+" cannot be qualified as name@account", "Set the Controller account with appd instance update <name> --account <account>, or use a "+label+" written as name@account.")
	}
	return name + "@" + account, nil
}

func configError(message, hint string) *httpclient.HTTPError {
	return &httpclient.HTTPError{Code: "config_error", Message: message, Hint: hint, Status: 400}
}

// AuthType reports the canonical auth type in use.
func (c *Client) AuthType() string { return c.authType }

// Identity reports the non-secret principal the client authenticates as
// (name@account); it is empty for bearer_token auth.
func (c *Client) Identity() string { return c.identity }

// BaseURL reports the normalized Controller base URL.
func (c *Client) BaseURL() string { return c.base }

// Get performs one GET under the Controller, always with output=JSON, and
// returns the decoded JSON value (numbers are json.Number).
func (c *Client) Get(rawPath string, query map[string]string) (any, error) {
	resp, err := c.do(http.MethodGet, rawPath, query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, c.networkError(err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, &httpclient.HTTPError{Code: "server_error", Message: "AppDynamics returned a non-JSON response: " + c.snippet(body), Hint: "Check that base_url points at the Controller (the /controller path is implied) and that the credentials are valid.", Status: 502}
	}
	return v, nil
}

// Preview describes the request Get would send without contacting the
// Controller (used by --dry-run). It never includes credentials.
func (c *Client) Preview(rawPath string, query map[string]string) (map[string]any, error) {
	full, err := c.ResolveURL(rawPath, query)
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(full)
	q := map[string]string{}
	for k, v := range u.Query() {
		if len(v) > 0 {
			q[k] = v[0]
		}
	}
	u.RawQuery = ""
	return map[string]any{"dry_run": true, "method": http.MethodGet, "url": u.String(), "path": u.Path, "query": q, "auth_type": c.authType}, nil
}

// ResolveURL turns a Controller path (or an absolute URL that belongs to the
// instance) plus query parameters into the full request URL, adding
// output=JSON unless the caller set output explicitly.
func (c *Client) ResolveURL(rawPath string, query map[string]string) (string, error) {
	var u *url.URL
	if isAbsolute(rawPath) {
		if !urlBelongsToBase(rawPath, c.base) {
			return "", offInstanceError()
		}
		parsed, err := url.Parse(rawPath)
		if err != nil {
			return "", offInstanceError()
		}
		u = parsed
	} else {
		p := strings.TrimSpace(rawPath)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		parsed, err := url.Parse(c.base + p)
		if err != nil {
			return "", &httpclient.HTTPError{Code: "invalid_args", Message: "invalid request path: " + err.Error(), Status: 400}
		}
		u = parsed
	}
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	if q.Get("output") == "" {
		q.Set("output", "JSON")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// RESTPath validates the argument of appd api get: a path or absolute
// instance URL under /controller/rest/. It returns the clean path and any
// query parameters embedded in the argument.
func (c *Client) RESTPath(raw string) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	q := map[string]string{}
	p := raw
	if isAbsolute(raw) {
		if !urlBelongsToBase(raw, c.base) {
			return "", nil, offInstanceError()
		}
		u, err := url.Parse(raw)
		if err != nil {
			return "", nil, offInstanceError()
		}
		p = u.Path
		if b, err := url.Parse(c.base); err == nil {
			if basePath := strings.TrimRight(b.Path, "/"); basePath != "" && strings.HasPrefix(strings.ToLower(p), strings.ToLower(basePath)) {
				p = p[len(basePath):]
			}
		}
		for k, v := range u.Query() {
			if len(v) > 0 {
				q[k] = v[0]
			}
		}
	} else if i := strings.Index(p, "?"); i >= 0 {
		values, err := url.ParseQuery(p[i+1:])
		if err != nil {
			return "", nil, &httpclient.HTTPError{Code: "invalid_args", Message: "invalid query string: " + err.Error(), Status: 400}
		}
		for k, v := range values {
			if len(v) > 0 {
				q[k] = v[0]
			}
		}
		p = p[:i]
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	if !strings.HasPrefix(strings.ToLower(clean), RESTPrefix) {
		return "", nil, &httpclient.HTTPError{Code: "invalid_args", Message: "path must start with " + RESTPrefix, Hint: "appd api get only reads Controller REST resources, for example /controller/rest/applications or /controller/rest/applications/ecommerce/tiers.", Status: 400}
	}
	return clean, q, nil
}

func offInstanceError() *httpclient.HTTPError {
	return &httpclient.HTTPError{Code: "instance_url_mismatch", Message: "off-instance url", Hint: "Use a URL that belongs to the selected appd instance.", Status: 400}
}

func (c *Client) do(method, rawPath string, query map[string]string) (*http.Response, error) {
	full, err := c.ResolveURL(rawPath, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, full, nil)
	if err != nil {
		return nil, &httpclient.HTTPError{Code: "invalid_args", Message: "invalid request path: " + err.Error(), Status: 400}
	}
	header, err := c.authorization()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", header)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.networkError(err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, c.statusError(resp)
	}
	return resp, nil
}

func (c *Client) authorization() (string, error) {
	switch c.authType {
	case AuthAPIClient:
		token, err := c.accessToken()
		if err != nil {
			return "", err
		}
		return "Bearer " + token, nil
	case AuthBasic:
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.identity+":"+c.secret)), nil
	case AuthBearer:
		return "Bearer " + c.secret, nil
	default:
		return "", configError("no AppDynamics credentials configured", "Run appd auth login.")
	}
}

// accessToken returns the cached OAuth token or exchanges the API Client
// credentials for a new one. The token is cached in memory only.
func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.tokenExp) {
		return c.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.identity)
	form.Set("client_secret", c.secret)
	req, err := http.NewRequest(http.MethodPost, c.base+OAuthTokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return "", configError("invalid OAuth token URL: "+err.Error(), "Check the appd base_url.")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", c.networkError(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenBody))
	if resp.StatusCode >= 400 {
		code := "auth_failed"
		if resp.StatusCode >= 500 {
			code = "server_error"
		}
		return "", &httpclient.HTTPError{Code: code, Message: fmt.Sprintf("AppDynamics OAuth token request failed with HTTP %d%s", resp.StatusCode, c.snippetSuffix(body)), Hint: "Check the API client name, client secret, and account (client_id " + c.identity + "); the API client must exist and be enabled in the Controller.", Status: resp.StatusCode}
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil || strings.TrimSpace(tr.AccessToken) == "" {
		return "", &httpclient.HTTPError{Code: "auth_failed", Message: "AppDynamics OAuth token response did not include access_token", Hint: "Check that base_url points at the Controller and that the API client is enabled.", Status: resp.StatusCode}
	}
	ttl := defaultTokenTTL
	if secs := asInt64(tr.ExpiresIn); secs > 0 {
		ttl = time.Duration(secs) * time.Second
	}
	exp := c.now().Add(ttl - tokenExpiryGrace)
	if ttl <= tokenExpiryGrace {
		exp = c.now().Add(ttl / 2)
	}
	c.token = strings.TrimSpace(tr.AccessToken)
	c.tokenExp = exp
	return c.token, nil
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		return 0
	}
}

func (c *Client) networkError(err error) *httpclient.HTTPError {
	message := "request failed"
	if detail := c.scrub(err.Error()); detail != "" {
		message += ": " + detail
	}
	return &httpclient.HTTPError{Code: "network_error", Message: message, Hint: "Check network connectivity, proxy settings, and the selected appd instance base_url."}
}

func (c *Client) statusError(resp *http.Response) *httpclient.HTTPError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorSnippet+1))
	message := fmt.Sprintf("request failed with HTTP %d%s", resp.StatusCode, c.snippetSuffix(body))
	code := mapStatus(resp.StatusCode)
	hint := ""
	switch code {
	case "auth_failed":
		hint = "Refresh the credentials with appd auth login and validate them with appd auth test --json."
	case "permission_denied":
		hint = "The credentials lack permission for this application or endpoint; use an API client or user with read access."
	case "not_found":
		hint = "Check the application, tier, node, GUID, or path; run appd app list --json to discover names and ids."
	case "rate_limited":
		hint = "The Controller rate-limited the request; retry later with a narrower time range."
	case "invalid_args":
		hint = "AppDynamics rejected the request parameters; check the application name or id, metric path, and time range."
	}
	return &httpclient.HTTPError{Code: code, Message: message, Hint: hint, Status: resp.StatusCode}
}

func mapStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "auth_failed"
	case http.StatusForbidden:
		return "permission_denied"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limited"
	}
	if status >= 500 {
		return "server_error"
	}
	return "invalid_args"
}

// scrub removes the configured secret and cached token from text before it
// can reach an envelope, then applies the shared sanitizer.
func (c *Client) scrub(s string) string {
	for _, secret := range []string{c.secret, c.token} {
		if strings.TrimSpace(secret) != "" {
			s = strings.ReplaceAll(s, secret, redactedPlacehold)
		}
	}
	return httpclient.SanitizeErrorText(s)
}

func (c *Client) snippet(body []byte) string {
	truncated := len(body) > maxErrorSnippet
	if truncated {
		body = body[:maxErrorSnippet]
	}
	s := c.scrub(string(body))
	if truncated && s != "" {
		s += "..."
	}
	return s
}

func (c *Client) snippetSuffix(body []byte) string {
	if s := c.snippet(body); s != "" {
		return ": " + s
	}
	return ""
}

func isAbsolute(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func urlBelongsToBase(raw, base string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, b.Scheme) || !strings.EqualFold(u.Host, b.Host) {
		return false
	}
	basePath := "/" + strings.Trim(strings.ToLower(b.Path), "/")
	rawPath := "/" + strings.Trim(strings.ToLower(u.Path), "/")
	if basePath == "/" {
		return true
	}
	return rawPath == basePath || strings.HasPrefix(rawPath, strings.TrimRight(basePath, "/")+"/")
}
