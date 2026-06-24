package aiplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
)

const tokenTTL = 30 * time.Second

type Client struct {
	ChatHost         string
	ChatURI          string
	IB2BHost         string
	IB2BURI          string
	Token            string
	User             string
	TrustTokenHeader string
	TrackingPrefix   string
	HTTPClient       *http.Client
	Now              func() time.Time
}

type APIError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type ExchangeResult struct {
	Token     string
	ExpiresAt time.Time
}

type Status struct {
	Provider               string `json:"provider"`
	AuthConfigured         bool   `json:"auth_configured"`
	UsernameConfigured     bool   `json:"username_configured"`
	PasswordConfigured     bool   `json:"password_configured"`
	UsercaseConfigured     bool   `json:"usercase_configured"`
	EndpointsConfigured    bool   `json:"endpoints_configured"`
	TokenValid             bool   `json:"token_valid"`
	TokenRefreshable       bool   `json:"token_refreshable"`
	TokenExpiresAt         string `json:"token_expires_at,omitempty"`
	TokenState             string `json:"token_state"`
	NextAction             string `json:"next_action,omitempty"`
	ChatEndpointConfigured bool   `json:"chat_endpoint_configured"`
	IB2BEndpointConfigured bool   `json:"ib2b_endpoint_configured"`
}

func NewClient(cfg config.Config, timeout time.Duration) *Client {
	return &Client{
		ChatHost:         cfg.AIPlatform.Chat.Host,
		ChatURI:          cfg.AIPlatform.Chat.URI,
		IB2BHost:         cfg.AIPlatform.IB2B.Host,
		IB2BURI:          cfg.AIPlatform.IB2B.URI,
		Token:            cfg.AIPlatform.Auth.Token,
		User:             cfg.AIPlatform.Auth.Usercase,
		TrustTokenHeader: cfg.AIPlatform.Auth.TrustTokenHeader,
		TrackingPrefix:   cfg.AIPlatform.Auth.TrackingPrefix,
		HTTPClient:       NewHTTPClient(timeout),
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	return &http.Client{Timeout: timeout, Transport: tr}
}

func (c *Client) Responses(ctx context.Context, req copilot.ResponsesRequest) (map[string]any, error) {
	if strings.TrimSpace(c.Token) == "" {
		return nil, &APIError{Code: "auth_required", Message: "AI Platform authentication is required.", Hint: "Configure ai_platform.auth.username, password, and usercase, then run inspect-image auth test --json.", Status: 401}
	}
	endpoint, err := joinURL(c.ChatHost, c.ChatURI)
	if err != nil {
		return nil, err
	}
	body := buildChatRequest(req, c.User)
	var raw map[string]any
	if err := c.postJSON(ctx, endpoint, body, c.chatHeaders(), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) ExchangeToken(ctx context.Context, username, password string) (ExchangeResult, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return ExchangeResult{}, &APIError{Code: "auth_required", Message: "AI Platform username and password are required.", Hint: "Set ai_platform.auth.username and ai_platform.auth.password in ~/.efp/config.yaml or run inspect-image auth login --provider ai_platform.", Status: 401}
	}
	endpoint, err := joinURL(c.IB2BHost, c.IB2BURI)
	if err != nil {
		return ExchangeResult{}, err
	}
	body := map[string]any{
		"input_token_state": map[string]string{
			"token_type": "CREDENTIAL",
			"username":   username,
			"password":   password,
		},
		"output_token_state": map[string]string{"token_type": "JWT"},
	}
	var resp struct {
		IssuedToken string `json:"issued_token"`
	}
	if err := c.postJSON(ctx, endpoint, body, map[string]string{"Content-Type": "application/json", "Accept": "application/json"}, &resp); err != nil {
		return ExchangeResult{}, err
	}
	if strings.TrimSpace(resp.IssuedToken) == "" {
		return ExchangeResult{}, &APIError{Code: "auth_failed", Message: "AI Platform iB2B token response did not include issued_token.", Hint: "Check username, password, iB2B host/uri, and account access.", Status: 401}
	}
	return ExchangeResult{Token: resp.IssuedToken, ExpiresAt: c.now().Add(tokenTTL)}, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, body any, headers map[string]string, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return &APIError{Code: "timeout", Message: "The AI Platform request timed out.", Hint: "Retry later or increase --timeout.", Status: 408}
		}
		msg := config.RedactString(err.Error())
		code := "ai_platform_api_unavailable"
		if strings.Contains(strings.ToLower(msg), "proxy") {
			code = "proxy_error"
		}
		return &APIError{Code: code, Message: "The AI Platform endpoint could not be reached. " + msg, Hint: "Check network, proxy, and AI Platform availability.", Status: 502}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "ai_platform_api_error"
		hint := "Retry later or check AI Platform service status."
		switch resp.StatusCode {
		case 401, 403:
			code = "auth_required"
			hint = "Run inspect-image auth test --json to refresh the AI Platform token, or check configured credentials."
		case 429:
			code = "rate_limited"
			hint = "Wait and retry with the same request."
		default:
			if resp.StatusCode >= 500 {
				code = "ai_platform_api_unavailable"
			}
		}
		detail := sanitizedResponseDetail(data)
		message := "The AI Platform endpoint returned an error."
		if detail != "" {
			message += " " + detail
		}
		return &APIError{Code: code, Message: message, Hint: hint, Status: resp.StatusCode}
	}
	if err := json.Unmarshal(data, out); err != nil {
		detail := sanitizedResponseDetail(data)
		message := "The AI Platform response could not be parsed. " + config.RedactString(err.Error())
		if detail != "" {
			message += " Body: " + detail
		}
		return &APIError{Code: "response_parse_failed", Message: message, Hint: "Retry later; if it persists, report the sanitized response shape.", Status: 502}
	}
	return nil
}

func (c *Client) chatHeaders() map[string]string {
	header := strings.TrimSpace(c.TrustTokenHeader)
	if header == "" {
		header = config.DefaultAIPlatformTrustTokenHeader
	}
	tracking := c.trackingID()
	return map[string]string{
		"Content-Type":     "application/json",
		"Accept":           "application/json",
		header:             c.Token,
		"x-correlation-id": tracking,
		"x-usersession-id": tracking,
	}
}

func (c *Client) trackingID() string {
	prefix := strings.TrimSpace(c.TrackingPrefix)
	if prefix == "" {
		prefix = config.DefaultAIPlatformTrackingPrefix
	}
	return fmt.Sprintf("%s-%s", prefix, c.now().UTC().Format("20060102150405000"))
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func buildChatRequest(req copilot.ResponsesRequest, user string) map[string]any {
	messages := []map[string]any{}
	if strings.TrimSpace(req.Instructions) != "" {
		messages = append(messages, map[string]any{"role": "developer", "content": req.Instructions})
	}
	for _, input := range req.Input {
		content := []map[string]any{}
		for _, item := range input.Content {
			switch item.Type {
			case "input_text":
				content = append(content, map[string]any{"type": "text", "text": item.Text})
			case "input_image":
				content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": item.ImageURL}})
			}
		}
		role := input.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, map[string]any{"role": role, "content": content})
	}
	body := map[string]any{
		"model":                 req.Model,
		"messages":              messages,
		"reasoning_effort":      req.Reasoning.Effort,
		"max_completion_tokens": req.MaxOutputTokens,
		"response_format":       map[string]string{"type": "json_object"},
	}
	if strings.TrimSpace(user) != "" {
		body["user"] = strings.TrimSpace(user)
	}
	return body
}

func joinURL(host, uri string) (string, error) {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	uri = strings.TrimSpace(uri)
	if host == "" || uri == "" {
		return "", &APIError{Code: "config_error", Message: "AI Platform host and uri are required.", Hint: "Set ai_platform.chat.host/uri and ai_platform.ib2b.host/uri in ~/.efp/config.yaml.", Status: 400}
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri, nil
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	return host + uri, nil
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
