package copilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
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
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"output_text":"{\"answer\":\"ok\"}"}`))
	}))
	defer s.Close()
	img := imagecheck.ImageInfo{MIMEType: "image/png", Data: []byte{1, 2, 3}}
	req, err := BuildRequest("gpt-5.4", "medium", "task", img)
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

func TestBuildRequestRejectsAllowlists(t *testing.T) {
	img := imagecheck.ImageInfo{MIMEType: "image/png", Data: []byte{1}}
	if _, err := BuildRequest("bad", "medium", "task", img); err == nil {
		t.Fatal("expected model rejection")
	}
	if _, err := BuildRequest("gpt-5.4", "bad", "task", img); err == nil {
		t.Fatal("expected reasoning rejection")
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
		_, _ = w.Write([]byte(`{"error":{"message":"bad request gho_SECRET tid=SECRET"}}`))
	}))
	defer s.Close()
	c := &Client{BaseURL: s.URL, Token: "token", HTTPClient: s.Client()}
	err := c.postJSON(context.Background(), "/responses", map[string]any{}, &map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad request") || strings.Contains(msg, "gho_SECRET") || strings.Contains(msg, "tid=SECRET") {
		t.Fatalf("bad sanitized error: %s", msg)
	}
}
