package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

const defaultGitHubClientID = "Iv1.b507a08c87ecfe98"

type DeviceClient struct {
	HTTPClient              *http.Client
	GitHubBaseURL           string
	CopilotGitHubAPIBaseURL string
	ClientID                string
	Scopes                  string
	Now                     func() time.Time
}

type DeviceStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type LoginResult struct {
	AuthConfigured        bool   `json:"auth_configured"`
	GitHubHost            string `json:"github_host"`
	GitHubUser            string `json:"github_user,omitempty"`
	CopilotTokenExpiresAt string `json:"copilot_token_expires_at,omitempty"`
}

type APIError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func (c *DeviceClient) Login(ctx context.Context, cfg config.Config, out io.Writer) (config.Config, LoginResult, error) {
	start, err := c.Start(ctx)
	if err != nil {
		return cfg, LoginResult{}, err
	}
	if out != nil {
		fmt.Fprintf(out, "Open: %s\nCode: %s\n", start.VerificationURI, start.UserCode)
	}
	ghToken, err := c.Poll(ctx, start)
	if err != nil {
		return cfg, LoginResult{}, err
	}
	copilotToken, expiresAt, apiBaseURL, err := c.ExchangeCopilotToken(ctx, ghToken)
	if err != nil {
		return cfg, LoginResult{}, err
	}
	now := c.now().UTC().Format(time.RFC3339)
	cfg.Auth.GitHubAccessToken = ghToken
	cfg.Auth.CopilotToken = copilotToken
	cfg.Auth.CopilotTokenExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	cfg.Auth.UpdatedAt = now
	if apiBaseURL != "" {
		cfg.API.BaseURL = apiBaseURL
	}
	if cfg.Auth.GitHubHost == "" {
		cfg.Auth.GitHubHost = "github.com"
	}
	user, _ := c.User(ctx, ghToken)
	cfg.Auth.GitHubUser = user
	return cfg, LoginResult{AuthConfigured: true, GitHubHost: cfg.Auth.GitHubHost, GitHubUser: cfg.Auth.GitHubUser, CopilotTokenExpiresAt: cfg.Auth.CopilotTokenExpiresAt}, nil
}

func (c *DeviceClient) Start(ctx context.Context) (DeviceStart, error) {
	body := map[string]string{"client_id": c.clientID(), "scope": c.scopes()}
	req, err := c.newJSONRequest(ctx, strings.TrimRight(c.githubBase(), "/")+"/login/device/code", body)
	if err != nil {
		return DeviceStart{}, err
	}
	var start DeviceStart
	if err := c.doJSON(req, &start); err != nil {
		return start, err
	}
	return start, nil
}

func (c *DeviceClient) Poll(ctx context.Context, start DeviceStart) (string, error) {
	interval := time.Duration(start.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := c.now().Add(time.Duration(start.ExpiresIn) * time.Second)
	if start.ExpiresIn <= 0 {
		deadline = c.now().Add(15 * time.Minute)
	}
	for {
		body := map[string]string{
			"client_id":   c.clientID(),
			"device_code": start.DeviceCode,
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		}
		req, err := c.newJSONRequest(ctx, strings.TrimRight(c.githubBase(), "/")+"/login/oauth/access_token", body)
		if err != nil {
			return "", err
		}
		var resp struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		err = c.doJSON(req, &resp)
		if err != nil {
			return "", err
		}
		if resp.AccessToken != "" {
			return resp.AccessToken, nil
		}
		switch resp.Error {
		case "authorization_pending":
			if c.now().After(deadline) {
				return "", &APIError{Code: "auth_poll_pending", Message: "GitHub device authorization is still pending.", Hint: "Re-run inspect-image auth login and complete the device flow before it expires.", Status: 408}
			}
			select {
			case <-ctx.Done():
				return "", &APIError{Code: "timeout", Message: "Authentication timed out.", Hint: "Retry inspect-image auth login.", Status: 408}
			case <-time.After(interval):
			}
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token":
			return "", &APIError{Code: "auth_expired", Message: "GitHub device code expired.", Hint: "Run inspect-image auth login again.", Status: 401}
		case "access_denied":
			return "", &APIError{Code: "auth_failed", Message: "GitHub device authorization was denied.", Hint: "Run inspect-image auth login again and approve the request.", Status: 401}
		default:
			return "", &APIError{Code: "auth_failed", Message: "GitHub authentication failed.", Hint: "Run inspect-image auth login again.", Status: 401}
		}
	}
}

type ExchangedToken struct {
	Token      string
	ExpiresAt  time.Time
	APIBaseURL string
}

func (c *DeviceClient) ExchangeCopilotToken(ctx context.Context, sourceCredential string) (string, time.Time, string, error) {
	token, err := c.ExchangeCopilotInternalToken(ctx, sourceCredential)
	if err != nil {
		return "", time.Time{}, "", err
	}
	return token.Token, token.ExpiresAt, token.APIBaseURL, nil
}

func (c *DeviceClient) ExchangeCopilotInternalToken(ctx context.Context, sourceCredential string) (ExchangedToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.copilotGitHubAPIBase(), "/")+"/copilot_internal/v2/token", nil)
	if err != nil {
		return ExchangedToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+sourceCredential)
	req.Header.Set("Accept", "application/json")
	setCopilotPluginHeaders(req.Header)
	var resp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return ExchangedToken{}, err
	}
	if resp.Token == "" || resp.ExpiresAt <= 0 {
		return ExchangedToken{}, &APIError{Code: "auth_failed", Message: "Copilot token exchange returned an invalid response.", Hint: "Run inspect-image auth login again and confirm GitHub Copilot access for this account.", Status: 401}
	}
	return ExchangedToken{
		Token:      resp.Token,
		ExpiresAt:  time.Unix(resp.ExpiresAt, 0).UTC(),
		APIBaseURL: ParseCopilotAPIBaseURL(resp.Token),
	}, nil
}

func (c *DeviceClient) User(ctx context.Context, githubToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.githubBase(), "/")+"/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/json")
	var resp struct {
		Login string `json:"login"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return "", err
	}
	return resp.Login, nil
}

func (c *DeviceClient) newJSONRequest(ctx context.Context, endpoint string, body any) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	setCopilotPluginHeaders(req.Header)
	return req, nil
}

func (c *DeviceClient) doJSON(req *http.Request, out any) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &APIError{Code: "auth_failed", Message: "Authentication request failed. " + config.RedactString(err.Error()), Hint: "Check network, proxy, and GitHub availability.", Status: 502}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := sanitizedResponseDetail(body)
		message := "Authentication endpoint returned an error."
		if detail != "" {
			message += " " + detail
		}
		return &APIError{Code: "auth_failed", Message: message, Hint: "Retry inspect-image auth login. If it repeats, check the displayed endpoint error and GitHub Copilot access for this account.", Status: resp.StatusCode}
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(out); err != nil {
		detail := sanitizedResponseDetail(body)
		message := "Authentication response could not be parsed. " + config.RedactString(err.Error())
		if detail != "" {
			message += " Body: " + detail
		}
		return &APIError{Code: "response_parse_failed", Message: message, Hint: "Retry later or check GitHub service status.", Status: 502}
	}
	return nil
}

func (c *DeviceClient) githubBase() string {
	if c.GitHubBaseURL != "" {
		return c.GitHubBaseURL
	}
	return "https://github.com"
}

func (c *DeviceClient) copilotGitHubAPIBase() string {
	if c.CopilotGitHubAPIBaseURL != "" {
		return c.CopilotGitHubAPIBaseURL
	}
	return "https://api.github.com"
}

func (c *DeviceClient) clientID() string {
	if c.ClientID != "" {
		return c.ClientID
	}
	return defaultGitHubClientID
}

func (c *DeviceClient) scopes() string {
	if c.Scopes != "" {
		return c.Scopes
	}
	return "read:user"
}

func (c *DeviceClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func sanitizedResponseDetail(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var payload any
	if err := json.Unmarshal(trimmed, &payload); err == nil {
		if detail := detailFromValue(payload); detail != "" {
			return config.RedactString(limitDetail(detail, 1000))
		}
	}
	return config.RedactString(limitDetail(strings.TrimSpace(string(trimmed)), 1000))
}

func detailFromValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case map[string]any:
		return detailFromMap(x)
	case []any:
		parts := make([]string, 0, len(x))
		for i, item := range x {
			if i >= 5 {
				parts = append(parts, "...")
				break
			}
			if detail := detailFromValue(item); detail != "" {
				parts = append(parts, detail)
			}
		}
		return strings.Join(parts, "; ")
	case nil:
		return ""
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
}

func detailFromMap(m map[string]any) string {
	keys := []string{"message", "error_description", "code", "type", "param", "detail", "details", "status", "statusCode", "request_id", "requestId"}
	parts := make([]string, 0, len(keys)+1)
	if errValue, ok := m["error"]; ok {
		if detail := detailFromValue(errValue); detail != "" {
			parts = append(parts, "error="+detail)
		}
	}
	for _, key := range keys {
		value, ok := m[key]
		if !ok {
			continue
		}
		if detail := detailFromValue(value); detail != "" {
			parts = append(parts, key+"="+detail)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

func limitDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func setCopilotPluginHeaders(h http.Header) {
	h.Set("User-Agent", "GitHubCopilotChat/0.35.0")
	h.Set("Editor-Version", "vscode/1.107.0")
	h.Set("Editor-Plugin-Version", "copilot-chat/0.35.0")
	h.Set("Copilot-Integration-Id", "vscode-chat")
}

var proxyEndpointPattern = regexp.MustCompile(`(?:^|[;&,\s])proxy-ep=([^;&,\s]+)`)

func ParseCopilotAPIBaseURL(token string) string {
	match := proxyEndpointPattern.FindStringSubmatch(token)
	if len(match) < 2 {
		return ""
	}
	raw, err := url.QueryUnescape(strings.TrimSpace(match[1]))
	if err != nil {
		raw = strings.TrimSpace(match[1])
	}
	parsed, err := url.Parse(raw)
	host := ""
	if err == nil {
		host = parsed.Host
		if host == "" {
			host = strings.SplitN(parsed.Path, "/", 2)[0]
		}
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "proxy.") {
		host = "api." + strings.TrimPrefix(host, "proxy.")
	}
	return "https://" + strings.TrimRight(host, "/")
}
