package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
	"engineering-flow-platform-tools/internal/inspectimage/vision"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type fakeClient struct{}

func (f *fakeClient) Responses(ctx context.Context, req vision.Request) (map[string]any, error) {
	return map[string]any{"output_text": `{"answer":"ok"}`}, nil
}

type secretClient struct{}

func (f *secretClient) Responses(ctx context.Context, req vision.Request) (map[string]any, error) {
	return map[string]any{"output_text": `{"answer":"Authorization: Bearer secret-token-should-not-appear","temporaryCredentials":"secret-password-should-not-appear"}`}, nil
}

func TestCommandsJSONListsCoreCommands(t *testing.T) {
	out := run(t, nil, "commands", "--json")
	commands := out["data"].(map[string]any)["commands"].([]any)
	joined := ""
	for _, item := range commands {
		joined += item.(map[string]any)["usage"].(string) + "\n"
	}
	for _, want := range []string{"inspect-image inspect", "inspect-image auth status", "inspect-image auth test", "inspect-image doctor", "inspect-image models", "inspect-image version"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	for _, item := range commands {
		cmd := item.(map[string]any)
		if cmd["name"] == "auth.login" && len(cmd["required"].([]any)) != 0 {
			t.Fatalf("auth.login should not inherit Atlassian auth requirements: %#v", cmd["required"])
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
	if props["image"] == nil || props["prompt"] == nil || props["out"] == nil || props["model"] == nil || props["reasoning"] == nil {
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

func TestHelpLLMAcceptsTwoWordCommand(t *testing.T) {
	out := run(t, nil, "help", "llm", "--json")
	if out["ok"] != true {
		t.Fatalf("help llm failed: %#v", out)
	}
}

func TestHelpLLMRequiresInspectImageOnlyForImageAnalysis(t *testing.T) {
	out := run(t, nil, "help", "llm", "--json")
	tips := out["data"].(map[string]any)["tips"].([]any)
	joined := ""
	for _, tip := range tips {
		joined += tip.(string) + "\n"
	}
	for _, want := range []string{
		"For agents, --json is the default",
		"Always add --json",
		"only image-analysis path",
		"Do not use OCR tools as the primary path",
		"do not write Python",
		"auth_required or auth_expired",
		"Do not fall back to OCR",
		"Windows cmd",
		"--verbose --out",
		"Stdout is the primary output path",
		"%CD%\\inspect-image-result.json",
		"file-read tool",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help llm tips missing %q\n%s", want, joined)
		}
	}
}

func TestHelpIncludesDetailedCommandGuidance(t *testing.T) {
	assertHelpAnnotated(t, NewRootWithClient(&fakeClient{}))
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: []string{"text-only agents", "AI Platform /chat/completions is the default", "GitHub Copilot /responses", "EFP_CONFIG"},
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
			want: []string{"verification URL", "token_state"},
		},
		{
			name: "doctor",
			args: []string{"doctor", "--help"},
			want: []string{"readiness checks", "error.code=auth_required"},
		},
		{
			name: "schema",
			args: []string{"schema", "--help"},
			want: []string{"reasoning enum", "model default", "image size and MIME type limits"},
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

func assertHelpAnnotated(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	if !cmd.Hidden {
		if strings.TrimSpace(cmd.Short) == "" {
			t.Fatalf("%s missing Short", cmd.CommandPath())
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Fatalf("%s missing Long", cmd.CommandPath())
		}
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s persistent flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
	}
	for _, child := range cmd.Commands() {
		if child.Hidden || (child.Name() == "help" && strings.TrimSpace(child.Use) == "help") {
			continue
		}
		assertHelpAnnotated(t, child)
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

func TestAuthStatusRefreshableGitHubTokenIsOK(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderGitHubCopilot
	cfg.Auth.GitHubAccessToken = "github-token"
	cfg.Auth.CopilotToken = "stale-copilot-token"
	cfg.Auth.CopilotTokenExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "auth", "status", "--config", cfgPath, "--json")
	if out["ok"] != true {
		t.Fatalf("refreshable auth should be ok: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["copilot_token_valid"] != false || data["copilot_token_refreshable"] != true || data["token_state"] != "refreshable" {
		t.Fatalf("bad refreshable status: %#v", data)
	}
}

func TestDoctorAcceptsRefreshableGitHubToken(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderGitHubCopilot
	cfg.Auth.GitHubAccessToken = "github-token"
	cfg.Auth.CopilotToken = "stale-copilot-token"
	cfg.Auth.CopilotTokenExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "doctor", "--config", cfgPath, "--json")
	if out["ok"] != true {
		t.Fatalf("refreshable doctor should be ok: %#v", out)
	}
	checks := out["data"].(map[string]any)["checks"].(map[string]any)
	if checks["auth"] != "refreshable" || checks["copilot_token_refreshable"] != true {
		t.Fatalf("bad doctor checks: %#v", checks)
	}
}

func TestAuthTestRefreshesExpiredCopilotToken(t *testing.T) {
	oldExchange := exchangeCopilotToken
	t.Cleanup(func() { exchangeCopilotToken = oldExchange })
	expires := time.Now().Add(30 * time.Minute).UTC()
	exchangeCopilotToken = func(ctx context.Context, cfg config.Config) (string, time.Time, string, error) {
		return "tid=abc;proxy-ep=proxy.individual.githubcopilot.com;", expires, "https://api.individual.githubcopilot.com", nil
	}
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderGitHubCopilot
	cfg.Auth.GitHubAccessToken = "github-token"
	cfg.Auth.CopilotToken = "stale-copilot-token"
	cfg.Auth.CopilotTokenExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "auth", "test", "--config", cfgPath, "--json")
	if out["ok"] != true {
		t.Fatalf("auth test should refresh: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["refreshed"] != true || data["copilot_token_valid"] != true || data["token_state"] != "valid" {
		t.Fatalf("bad auth test data: %#v", data)
	}
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Auth.CopilotToken != "tid=abc;proxy-ep=proxy.individual.githubcopilot.com;" || saved.API.BaseURL != "https://api.individual.githubcopilot.com" {
		t.Fatalf("refresh was not saved: %#v", saved)
	}
}

func TestInspectRefreshesAfterResponsesAuthError(t *testing.T) {
	oldExchange := exchangeCopilotToken
	t.Cleanup(func() { exchangeCopilotToken = oldExchange })
	expires := time.Now().Add(30 * time.Minute).UTC()
	exchangeCopilotToken = func(ctx context.Context, cfg config.Config) (string, time.Time, string, error) {
		return "new-copilot-token", expires, cfg.API.BaseURL, nil
	}
	var responsesCalls int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		responsesCalls++
		if r.Header.Get("Authorization") == "Bearer old-copilot-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"expired"}}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer new-copilot-token" {
			t.Fatalf("unexpected authorization header")
		}
		_, _ = w.Write([]byte(`{"output_text":"{\"answer\":\"refreshed\"}"}`))
	}))
	defer s.Close()
	path := writePNG(t)
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderGitHubCopilot
	cfg.API.BaseURL = s.URL
	cfg.Auth.GitHubAccessToken = "github-token"
	cfg.Auth.CopilotToken = "old-copilot-token"
	cfg.Auth.CopilotTokenExpiresAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "inspect", "--config", cfgPath, "--image", path, "--prompt", "x", "--json")
	if out["ok"] != true {
		t.Fatalf("inspect should refresh and retry: %#v", out)
	}
	if responsesCalls != 2 {
		t.Fatalf("responses calls=%d", responsesCalls)
	}
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Auth.CopilotToken != "new-copilot-token" {
		t.Fatalf("token was not refreshed: %#v", saved.Auth)
	}
}

func TestInspectAIPlatformExchangesIB2BTokenAndCallsChatCompletions(t *testing.T) {
	var tokenCalls, chatCalls int
	var chatBody map[string]any
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ib2b":
			tokenCalls++
			if r.Method != http.MethodPost {
				t.Fatalf("bad token method %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			input := body["input_token_state"].(map[string]any)
			if input["token_type"] != "CREDENTIAL" || input["username"] != "alice" || input["password"] != "secret-password" {
				t.Fatalf("bad token request: %#v", body)
			}
			_, _ = w.Write([]byte(`{"issued_token":"eyJheader.eyJpayload.signature"}`))
		case "/chat/completions":
			chatCalls++
			if r.Header.Get("X-XXXX-E2E-Trust-Token") != "eyJheader.eyJpayload.signature" {
				t.Fatalf("missing trust token header: %#v", r.Header)
			}
			if r.Header.Get("x-correlation-id") == "" || r.Header.Get("x-usersession-id") == "" {
				t.Fatalf("missing tracking headers: %#v", r.Header)
			}
			if err := json.NewDecoder(r.Body).Decode(&chatBody); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"answer\":\"ai platform ok\",\"visible_text\":[]}"}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	path := writePNG(t)
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderAIPlatform
	cfg.AIPlatform.Chat.Host = s.URL
	cfg.AIPlatform.Chat.URI = "/chat/completions"
	cfg.AIPlatform.IB2B.Host = s.URL
	cfg.AIPlatform.IB2B.URI = "/ib2b"
	cfg.AIPlatform.Auth.Username = "alice"
	cfg.AIPlatform.Auth.Password = "secret-password"
	cfg.AIPlatform.Auth.Usercase = "case-123"
	cfg.AIPlatform.Auth.TokenFile = filepath.Join(t.TempDir(), "ai_platform_token")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "inspect", "--config", cfgPath, "--image", path, "--prompt", "x", "--model", "custom-model", "--json")
	if out["ok"] != true {
		t.Fatalf("bad ai platform inspect: %#v", out)
	}
	if tokenCalls != 1 || chatCalls != 1 {
		t.Fatalf("calls token=%d chat=%d", tokenCalls, chatCalls)
	}
	if chatBody["model"] != "custom-model" || chatBody["user"] != "case-123" || chatBody["reasoning_effort"] != "medium" {
		t.Fatalf("bad chat body: %#v", chatBody)
	}
	messages := chatBody["messages"].([]any)
	content := messages[1].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "text" || content[1].(map[string]any)["type"] != "image_url" {
		t.Fatalf("bad content: %#v", content)
	}
	data := out["data"].(map[string]any)
	if data["provider"] != config.ProviderAIPlatform || data["model"] != "custom-model" {
		t.Fatalf("bad result provider/model: %#v", data)
	}
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AIPlatform.Auth.Token == "" || saved.AIPlatform.Auth.TokenExpiresAt == "" {
		t.Fatalf("ai platform token was not saved: %#v", saved.AIPlatform.Auth)
	}
}

func TestAuthTestRefreshesAIPlatformToken(t *testing.T) {
	oldExchange := exchangeAIPlatformToken
	t.Cleanup(func() { exchangeAIPlatformToken = oldExchange })
	expires := time.Now().Add(30 * time.Second).UTC()
	exchangeAIPlatformToken = func(ctx context.Context, cfg config.Config, timeout time.Duration) (string, time.Time, error) {
		return "fresh-ai-platform-token", expires, nil
	}
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	cfg := config.Default()
	cfg.Provider = config.ProviderAIPlatform
	cfg.AIPlatform.Chat.Host = "https://ai.example"
	cfg.AIPlatform.IB2B.Host = "https://ib2b.example"
	cfg.AIPlatform.Auth.Username = "alice"
	cfg.AIPlatform.Auth.Password = "secret-password"
	cfg.AIPlatform.Auth.Usercase = "case-123"
	cfg.AIPlatform.Auth.Token = "stale-token"
	cfg.AIPlatform.Auth.TokenExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	cfg.AIPlatform.Auth.TokenFile = filepath.Join(t.TempDir(), "ai_platform_token")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	out := run(t, nil, "auth", "test", "--config", cfgPath, "--json")
	if out["ok"] != true {
		t.Fatalf("bad ai auth test: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["refreshed"] != true || data["token_valid"] != true || data["token_state"] != "valid" || data["provider"] != config.ProviderAIPlatform {
		t.Fatalf("bad ai auth test data: %#v", data)
	}
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.AIPlatform.Auth.Token != "fresh-ai-platform-token" {
		t.Fatalf("token was not refreshed: %#v", saved.AIPlatform.Auth)
	}
}

func TestInspectWithInjectedClient(t *testing.T) {
	path := writePNG(t)
	out := run(t, &fakeClient{}, "inspect", "--image", path, "--prompt", "x", "--json")
	if out["ok"] != true {
		t.Fatalf("bad inspect: %#v", out)
	}
}

func TestInspectOutWritesSuccessEnvelope(t *testing.T) {
	path := writePNG(t)
	outPath := filepath.Join(t.TempDir(), "result.json")
	out := run(t, &fakeClient{}, "inspect", "--image", path, "--prompt", "x", "--out", outPath, "--json")
	if out["ok"] != true {
		t.Fatalf("bad inspect: %#v", out)
	}
	fileOut := readJSONFile(t, outPath)
	if fileOut["ok"] != true {
		t.Fatalf("bad file output: %#v", fileOut)
	}
	if !reflect.DeepEqual(out, fileOut) {
		t.Fatalf("--out should write the same envelope that stdout receives\nstdout=%#v\nfile=%#v", out, fileOut)
	}
}

func TestInspectOutRedactsEnvelopeCopy(t *testing.T) {
	path := writePNG(t)
	outPath := filepath.Join(t.TempDir(), "result.json")
	out := run(t, &secretClient{}, "inspect", "--image", path, "--prompt", "x", "--out", outPath, "--json")
	if out["ok"] != true {
		t.Fatalf("bad inspect: %#v", out)
	}
	fileText, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	stdoutBytes, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	combined := string(stdoutBytes) + "\n" + string(fileText)
	for _, leaked := range []string{"secret-token-should-not-appear", "secret-password-should-not-appear", "Bearer secret"} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("secret leaked %q:\n%s", leaked, combined)
		}
	}
	if !strings.Contains(combined, "***REDACTED***") {
		t.Fatalf("expected redacted marker in stdout and file:\n%s", combined)
	}
}

func TestInspectOutWritesErrorEnvelope(t *testing.T) {
	path := writePNG(t)
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "error.json")
	out := run(t, nil, "inspect", "--config", cfgPath, "--image", path, "--prompt", "x", "--out", outPath, "--json")
	if out["error"].(map[string]any)["code"] != "auth_required" {
		t.Fatalf("bad stdout error: %#v", out)
	}
	fileOut := readJSONFile(t, outPath)
	if fileOut["error"].(map[string]any)["code"] != "auth_required" {
		t.Fatalf("bad file error: %#v", fileOut)
	}
	if !reflect.DeepEqual(out, fileOut) {
		t.Fatalf("--out should write the same error envelope that stdout receives\nstdout=%#v\nfile=%#v", out, fileOut)
	}
}

func TestInspectVerboseDiagnostics(t *testing.T) {
	path := writePNG(t)
	outPath := filepath.Join(t.TempDir(), "result.json")
	stdout, stderr := runSplit(t, &fakeClient{}, "inspect", "--image", path, "--prompt", "x", "--out", outPath, "--verbose", "--json")
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid stdout json: %v out=%s stderr=%s", err, stdout, stderr)
	}
	for _, want := range []string{"sending provider image request", "provider image response received", "wrote JSON envelope", "process_exit_code=0"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q\nstdout=%s\nstderr=%s", want, stdout, stderr)
		}
	}
}

func TestExecutePrintsJSONForCobraError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"inspect", "--unknown", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid stdout json: %v out=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	errData := out["error"].(map[string]any)
	if errData["code"] != "invalid_args" || !strings.Contains(errData["message"].(string), "unknown flag") {
		t.Fatalf("bad cobra error envelope: %#v", out)
	}
}

func TestExecutePrintsFallbackForBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"models", "--format", "xml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "invalid_args") || !strings.Contains(stdout.String(), "unknown_output_format") {
		t.Fatalf("bad fallback output: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func run(t *testing.T, client interface {
	Responses(context.Context, vision.Request) (map[string]any, error)
}, args ...string) map[string]any {
	t.Helper()
	isolateInspectImageConfig(t, args)
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

func runSplit(t *testing.T, client interface {
	Responses(context.Context, vision.Request) (map[string]any, error)
}, args ...string) (string, string) {
	t.Helper()
	isolateInspectImageConfig(t, args)
	cmd := NewRootWithClient(client)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func runText(t *testing.T, client interface {
	Responses(context.Context, vision.Request) (map[string]any, error)
}, args ...string) string {
	t.Helper()
	isolateInspectImageConfig(t, args)
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

func isolateInspectImageConfig(t *testing.T, args []string) {
	t.Helper()
	for i, arg := range args {
		if arg == "--config" || strings.HasPrefix(arg, "--config=") {
			if arg == "--config" && i+1 >= len(args) {
				return
			}
			return
		}
	}
	cfgPath := filepath.Join(t.TempDir(), "inspect-image.json")
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvConfigPath, cfgPath)
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid json file err=%v out=%s", err, string(data))
	}
	return out
}

func writePNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(path, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
