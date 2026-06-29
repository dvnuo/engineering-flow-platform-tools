package copilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
	"engineering-flow-platform-tools/internal/inspectimage/vision"
)

func TestResponsesUsesResponsesPathAndShape(t *testing.T) {
	var gotPath string
	var got map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth")
		}
		if r.Header.Get("Accept") != "application/vnd.github.copilot-chat-preview+json" || r.Header.Get("Copilot-Integration-Id") != "vscode-chat" || r.Header.Get("Openai-Intent") != "conversation-edits" {
			t.Fatalf("missing copilot headers: %#v", r.Header)
		}
		if r.Header.Get("X-GitHub-Api-Version") != "" {
			t.Fatalf("responses request must not send X-GitHub-Api-Version: %#v", r.Header)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"output_text":"{\"answer\":\"ok\"}"}`))
	}))
	defer s.Close()
	img := imagecheck.ImageInfo{MIMEType: "image/png", Data: []byte{1, 2, 3}}
	req, err := vision.BuildRequest("gpt-5.4", "medium", "task", img)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{BaseURL: s.URL, Token: "token", HTTPClient: s.Client()}
	if _, err := c.Responses(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/responses" {
		t.Fatalf("path=%q", gotPath)
	}
	if got["model"] != "gpt-5.4" || got["reasoning"].(map[string]any)["effort"] != "medium" {
		t.Fatalf("bad request: %#v", got)
	}
	content := got["input"].([]any)[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "input_text" || !strings.HasPrefix(content[1].(map[string]any)["image_url"].(string), "data:image/png;base64,") {
		t.Fatalf("bad content: %#v", content)
	}
}

func TestResponsesHTTPErrorCodes(t *testing.T) {
	for status, code := range map[int]string{401: "auth_required", 429: "rate_limited", 500: "responses_api_unavailable"} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "no", status)
		}))
		c := &Client{BaseURL: s.URL, Token: "token", HTTPClient: s.Client()}
		err := c.postJSON(context.Background(), "/responses", map[string]any{}, &map[string]any{})
		s.Close()
		if err == nil || err.(*APIError).Code != code {
			t.Fatalf("status %d code err=%v", status, err)
		}
	}
}

func TestResponsesHTTPErrorIncludesSanitizedDetail(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request: error: invalid apiVersion gho_SECRET tid=SECRET","code":"invalid_api_version","type":"invalid_request_error","param":"apiVersion"},"request_id":"req_123"}`))
	}))
	defer s.Close()
	c := &Client{BaseURL: s.URL, Token: "token", HTTPClient: s.Client()}
	err := c.postJSON(context.Background(), "/responses", map[string]any{}, &map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"bad request", "invalid apiVersion", "code=invalid_api_version", "type=invalid_request_error", "param=apiVersion", "request_id=req_123"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in sanitized error: %s", want, msg)
		}
	}
	for _, secret := range []string{"gho_SECRET", "tid=SECRET", "data:image/png;base64"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("secret leaked in sanitized error: %s", msg)
		}
	}
}

func TestResponsesParseErrorIncludesSanitizedBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json gho_SECRET tid=SECRET`))
	}))
	defer s.Close()
	c := &Client{BaseURL: s.URL, Token: "token", HTTPClient: s.Client()}
	err := c.postJSON(context.Background(), "/responses", map[string]any{}, &map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not-json") || strings.Contains(msg, "gho_SECRET") || strings.Contains(msg, "tid=SECRET") {
		t.Fatalf("bad sanitized error: %s", msg)
	}
}
