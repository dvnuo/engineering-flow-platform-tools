package inspect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/vision"
)

type fakeResponses struct {
	req vision.Request
}

func (f *fakeResponses) Responses(ctx context.Context, req vision.Request) (map[string]any, error) {
	f.req = req
	return map[string]any{"output_text": `{"answer":"seen","visible_text":[]}`}, nil
}

func TestRunInspectsImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	fake := &fakeResponses{}
	res, err := Run(context.Background(), cfg, fake, Options{ImagePath: path, Prompt: "what?", Model: "gpt-5.4", Reasoning: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tool != "inspect_image" || res.Result.(map[string]any)["answer"] != "seen" {
		t.Fatalf("bad result: %#v", res)
	}
	if fake.req.Input[0].Content[0].Type != "input_text" || fake.req.Input[0].Content[1].Type != "input_image" {
		t.Fatalf("bad request: %#v", fake.req)
	}
}

func TestRunUsesAIPlatformClientByDefault(t *testing.T) {
	var calls int
	var body map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		calls++
		if r.Header.Get("X-XXXX-E2E-Trust-Token") != "ai-token" {
			t.Fatalf("missing trust token header: %#v", r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"answer\":\"ai default\",\"visible_text\":[]}"}}]}`))
	}))
	defer s.Close()
	path := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.AIPlatform.Chat.Host = s.URL
	cfg.AIPlatform.Chat.URI = "/chat/completions"
	cfg.AIPlatform.Auth.Token = "ai-token"
	cfg.AIPlatform.Auth.Usercase = "case-123"
	res, err := Run(context.Background(), cfg, nil, Options{ImagePath: path, Prompt: "what?"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if res.Provider != config.ProviderAIPlatform || res.Result.(map[string]any)["answer"] != "ai default" {
		t.Fatalf("bad result: %#v", res)
	}
	if body["user"] != "case-123" || body["model"] != config.DefaultModel {
		t.Fatalf("bad chat body: %#v", body)
	}
}

func TestReadPromptRequiresPrompt(t *testing.T) {
	if _, err := ReadPrompt("", ""); err == nil {
		t.Fatal("expected prompt_required")
	}
}
