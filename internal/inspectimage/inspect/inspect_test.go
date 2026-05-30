package inspect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
)

type fakeResponses struct {
	req copilot.ResponsesRequest
}

func (f *fakeResponses) Responses(ctx context.Context, req copilot.ResponsesRequest) (map[string]any, error) {
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

func TestReadPromptRequiresPrompt(t *testing.T) {
	if _, err := ReadPrompt("", ""); err == nil {
		t.Fatal("expected prompt_required")
	}
}
