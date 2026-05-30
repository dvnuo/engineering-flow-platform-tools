package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/copilot"
)

type fakeClient struct{}

func (f *fakeClient) Responses(ctx context.Context, req copilot.ResponsesRequest) (map[string]any, error) {
	return map[string]any{"output_text": `{"answer":"ok"}`}, nil
}

func TestCommandsJSONListsCoreCommands(t *testing.T) {
	out := run(t, nil, "commands", "--json")
	commands := out["data"].(map[string]any)["commands"].([]any)
	joined := ""
	for _, item := range commands {
		joined += item.(map[string]any)["usage"].(string) + "\n"
	}
	for _, want := range []string{"inspect-image inspect", "inspect-image auth status", "inspect-image doctor", "inspect-image models", "inspect-image version"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
}

func TestSchemaInspectJSON(t *testing.T) {
	out := run(t, nil, "schema", "inspect", "--json")
	data := out["data"].(map[string]any)
	if data["name"] != "inspect" {
		t.Fatalf("bad schema: %#v", data)
	}
	props := data["properties"].(map[string]any)
	if props["image"] == nil || props["prompt"] == nil || props["model"] == nil || props["reasoning"] == nil {
		t.Fatalf("missing properties: %#v", props)
	}
}

func TestInspectMissingImageInvalidArgs(t *testing.T) {
	out := run(t, nil, "inspect", "--prompt", "x", "--json")
	if out["error"].(map[string]any)["code"] != "invalid_args" {
		t.Fatalf("bad error: %#v", out)
	}
}

func TestInspectMissingPrompt(t *testing.T) {
	path := writePNG(t)
	out := run(t, nil, "inspect", "--image", path, "--json")
	if out["error"].(map[string]any)["code"] != "prompt_required" {
		t.Fatalf("bad error: %#v", out)
	}
}

func TestInspectNoAuth(t *testing.T) {
	path := writePNG(t)
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "inspect", "--config", cfgPath, "--image", path, "--prompt", "x", "--json")
	if out["error"].(map[string]any)["code"] != "auth_required" {
		t.Fatalf("bad error: %#v", out)
	}
}

func TestVersionJSON(t *testing.T) {
	out := run(t, nil, "version", "--json")
	if out["ok"] != true {
		t.Fatalf("version failed: %#v", out)
	}
}

func TestHelpIncludesDetailedCommandGuidance(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: []string{"text-only agents", "GitHub Copilot plugin /responses endpoint", "INSPECT_IMAGE_CONFIG"},
		},
		{
			name: "inspect",
			args: []string{"inspect", "--help"},
			want: []string{"Validate one local JPEG", "Remote image URLs", "--preset"},
		},
		{
			name: "auth",
			args: []string{"auth", "--help"},
			want: []string{"device-flow login", "Token values are never printed"},
		},
		{
			name: "auth login",
			args: []string{"auth", "login", "--help"},
			want: []string{"verification URL", "copilot_token_expires_at"},
		},
		{
			name: "doctor",
			args: []string{"doctor", "--help"},
			want: []string{"readiness checks", "error.code=auth_required"},
		},
		{
			name: "schema",
			args: []string{"schema", "--help"},
			want: []string{"model and reasoning enums", "image size and MIME type limits"},
		},
		{
			name: "help llm",
			args: []string{"help", "llm", "--help"},
			want: []string{"LLM agents", "command catalog"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runText(t, nil, tc.args...)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("help missing %q\n%s", want, out)
				}
			}
		})
	}
}

func TestAuthStatusMissingConfigJSON(t *testing.T) {
	out := run(t, nil, "auth", "status", "--config", filepath.Join(t.TempDir(), "missing.json"), "--json")
	if out["ok"] != false || out["error"].(map[string]any)["code"] != "auth_required" {
		t.Fatalf("bad status: %#v", out)
	}
}

func TestAuthStatusInvalidConfigReturnsParseDetail(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	if err := os.WriteFile(cfgPath, []byte(`{"auth":`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "auth", "status", "--config", cfgPath, "--json")
	errOut := out["error"].(map[string]any)
	if errOut["code"] != "config_error" || !strings.Contains(errOut["message"].(string), "unexpected end") {
		t.Fatalf("bad status: %#v", out)
	}
}

func TestInspectWithInjectedClient(t *testing.T) {
	path := writePNG(t)
	out := run(t, &fakeClient{}, "inspect", "--image", path, "--prompt", "x", "--json")
	if out["ok"] != true {
		t.Fatalf("bad inspect: %#v", out)
	}
}

func run(t *testing.T, client interface {
	Responses(context.Context, copilot.ResponsesRequest) (map[string]any, error)
}, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithClient(client)
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var out map[string]any
	if uerr := json.Unmarshal(b.Bytes(), &out); uerr != nil {
		t.Fatalf("invalid json err=%v execErr=%v out=%s", uerr, err, b.String())
	}
	return out
}

func runText(t *testing.T, client interface {
	Responses(context.Context, copilot.ResponsesRequest) (map[string]any, error)
}, args ...string) string {
	t.Helper()
	cmd := NewRootWithClient(client)
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	return b.String()
}

func writePNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
