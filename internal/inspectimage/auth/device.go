package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

const defaultGitHubClientID = "Iv1.b507a08c87ecfe98"

type DeviceClient struct {
	HTTPClient    *http.Client
	GitHubBaseURL string
	ClientID      string
	Scopes        string
	Now           func() time.Time
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
	now := c.now().UTC().Format(time.RFC3339)
	cfg.Auth.GitHubAccessToken = ghToken
	cfg.Auth.CopilotToken = ghToken
	cfg.Auth.CopilotTokenExpiresAt = ""
	cfg.Auth.UpdatedAt = now
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
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.35.0")
	return req, nil
}

func (c *DeviceClient) doJSON(req *http.Request, out any) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &APIError{Code: "auth_failed", Message: "Authentication request failed.", Hint: "Check network, proxy, and GitHub availability.", Status: 502}
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
		return &APIError{Code: "response_parse_failed", Message: "Authentication response could not be parsed.", Hint: "Retry later or check GitHub service status.", Status: 502}
	}
	return nil
}

func (c *DeviceClient) githubBase() string {
	if c.GitHubBaseURL != "" {
		return c.GitHubBaseURL
	}
	return "https://github.com"
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
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"message", "error_description", "error", "details"} {
			if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
				return config.RedactString(strings.TrimSpace(v))
			}
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		text = text[:300]
	}
	return config.RedactString(text)
}
