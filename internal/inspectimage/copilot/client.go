package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	iconfig "engineering-flow-platform-tools/internal/inspectimage/config"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type APIError struct {
	Code    string
	Message string
	Hint    string
	Status  int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func NewHTTPClient(timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	return &http.Client{Timeout: timeout, Transport: tr}
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	if c.Token == "" {
		return &APIError{Code: "auth_required", Message: "GitHub Copilot authentication is required.", Hint: "Run inspect-image auth login.", Status: 401}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.copilot-chat-preview+json")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.35.0")
	req.Header.Set("Editor-Version", "vscode/1.107.0")
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/0.35.0")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")
	req.Header.Set("Openai-Intent", "conversation-edits")
	req.Header.Set("x-initiator", "agent")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return &APIError{Code: "timeout", Message: "The /responses request timed out.", Hint: "Retry later or increase --timeout.", Status: 408}
		}
		msg := err.Error()
		code := "responses_api_unavailable"
		if strings.Contains(strings.ToLower(msg), "proxy") {
			code = "proxy_error"
		}
		return &APIError{Code: code, Message: "The /responses endpoint could not be reached. " + iconfig.RedactString(msg), Hint: "Check network, proxy, and Copilot availability.", Status: 502}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "responses_api_error"
		hint := "Retry later or check Copilot service status."
		switch resp.StatusCode {
		case 401, 403:
			code = "auth_required"
			hint = "Run inspect-image auth login."
		case 429:
			code = "rate_limited"
			hint = "Wait and retry with the same request."
		default:
			if resp.StatusCode >= 500 {
				code = "responses_api_unavailable"
			}
		}
		detail := sanitizedResponseDetail(data)
		message := "The /responses endpoint returned an error."
		if detail != "" {
			message += " " + detail
		}
		return &APIError{Code: code, Message: message, Hint: hint, Status: resp.StatusCode}
	}
	if err := json.Unmarshal(data, out); err != nil {
		detail := sanitizedResponseDetail(data)
		message := "The /responses response could not be parsed. " + iconfig.RedactString(err.Error())
		if detail != "" {
			message += " Body: " + detail
		}
		return &APIError{Code: "response_parse_failed", Message: message, Hint: "Retry later; if it persists, report the response shape without tokens.", Status: 502}
	}
	return nil
}

func sanitizedResponseDetail(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var payload any
	if err := json.Unmarshal(trimmed, &payload); err == nil {
		if detail := detailFromValue(payload); detail != "" {
			return iconfig.RedactString(limitDetail(detail, 1000))
		}
	}
	return iconfig.RedactString(limitDetail(strings.TrimSpace(string(trimmed)), 1000))
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
