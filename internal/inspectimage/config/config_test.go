package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/configenv"
	"gopkg.in/yaml.v3"
)

func TestLegacyEnvConfigOverridesPath(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	path := filepath.Join(t.TempDir(), "inspect-image.json")
	t.Setenv(EnvLegacyConfigPath, path)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path=%q want %q", got, path)
	}
}

func TestEnvConfigOverridesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(EnvConfigPath, path)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("path=%q want %q", got, path)
	}
}

func TestDefaultPathUsesHomeCopilot(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	t.Setenv(EnvLegacyConfigPath, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".efp", "config.yaml") {
		t.Fatalf("path=%q", got)
	}
}

func TestSaveUses0600WhereSupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX 0600 semantics through os.FileMode")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Default()
	cfg.Auth.CopilotTokenFile = filepath.Join(dir, "tmp", "copilot_token")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode too open: %v", info.Mode().Perm())
	}
}

func TestUnifiedConfigLoadsCopilotTokenFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	tokenPath := filepath.Join(dir, "tmp", "copilot_token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("copilot_token: short-lived\ncopilot_token_expires_at: \"2099-01-01T00:00:00Z\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := []byte(`
version: 1
copilot:
  provider: github_copilot_plugin
  auth:
    method: device_code
    github_host: github.com
    github_access_token: long-lived
    copilot_token_file: ` + tokenPath + `
inspect_image:
  api:
    endpoint_kind: responses
    base_url: https://api.githubcopilot.com
    timeout_seconds: 90
    use_system_proxy: true
`)
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.GitHubAccessToken != "long-lived" || cfg.Auth.CopilotToken != "short-lived" {
		t.Fatalf("bad auth load: %#v", cfg.Auth)
	}
}

func TestUnifiedConfigLoadsCopilotAPIFromRootNode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	body := []byte(`
version: 1
copilot:
  provider: github_copilot_plugin
  api:
    endpoint_kind: responses
    base_url: https://copilot.example
    timeout_seconds: 45
    use_system_proxy: true
  auth:
    method: device_code
inspect_image:
  provider: github_copilot_plugin
`)
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.API.BaseURL != "https://copilot.example" || cfg.API.TimeoutSeconds != 45 {
		t.Fatalf("bad copilot api load: %#v", cfg.API)
	}
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	copilotNode := root["copilot"].(map[string]any)
	if copilotNode["api"] == nil {
		t.Fatalf("copilot api missing after save:\n%s", string(b))
	}
	inspectNode := root["inspect_image"].(map[string]any)
	if _, ok := inspectNode["api"]; ok {
		t.Fatalf("inspect_image api should not be saved in new layout:\n%s", string(b))
	}
}

func TestUnifiedConfigResolvesAndPreservesEnvironmentReferences(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	copilotTokenPath := filepath.Join(dir, "copilot_token")
	aiTokenPath := filepath.Join(dir, "ai_token")
	t.Setenv("TOOLS_GITHUB_TOKEN", "resolved-github-token")
	t.Setenv("TOOLS_AI_USERNAME", "resolved-ai-user")
	t.Setenv("TOOLS_AI_PASSWORD", "resolved-ai-password")
	body := []byte(`
version: 1
copilot:
  provider: github_copilot_plugin
  auth:
    method: device_code
    github_access_token: "${TOOLS_GITHUB_TOKEN}"
    copilot_token_file: ` + copilotTokenPath + `
inspect_image:
  provider: ai_platform
ai_platform:
  auth:
    username: "%TOOLS_AI_USERNAME%"
    password: "${TOOLS_AI_PASSWORD}"
    token_file: ` + aiTokenPath + `
`)
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.GitHubAccessToken != "resolved-github-token" || cfg.AIPlatform.Auth.Username != "resolved-ai-user" || cfg.AIPlatform.Auth.Password != "resolved-ai-password" {
		t.Fatalf("environment references were not resolved: %#v %#v", cfg.Auth, cfg.AIPlatform.Auth)
	}
	cfg.Defaults.Model = "updated-model"
	t.Setenv("TOOLS_GITHUB_TOKEN", "")
	t.Setenv("TOOLS_AI_USERNAME", "rotated-ai-user")
	t.Setenv("TOOLS_AI_PASSWORD", "")
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	text := string(mustReadConfigFile(t, cfgPath))
	for _, reference := range []string{"${TOOLS_GITHUB_TOKEN}", "%TOOLS_AI_USERNAME%", "${TOOLS_AI_PASSWORD}"} {
		if !strings.Contains(text, reference) {
			t.Fatalf("environment reference %q was not preserved:\n%s", reference, text)
		}
	}
	for _, resolved := range []string{"resolved-github-token", "resolved-ai-user", "resolved-ai-password", "rotated-ai-user"} {
		if strings.Contains(text, resolved) {
			t.Fatalf("resolved value %q was materialized:\n%s", resolved, text)
		}
	}
	if !strings.Contains(text, "model: updated-model") {
		t.Fatalf("unrelated config update was not saved:\n%s", text)
	}

	cfg.AIPlatform.Auth.Username = "replacement-ai-user"
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	text = string(mustReadConfigFile(t, cfgPath))
	if strings.Contains(text, "%TOOLS_AI_USERNAME%") || !strings.Contains(text, "replacement-ai-user") {
		t.Fatalf("changed AI Platform username reference was not replaced:\n%s", text)
	}
	if !strings.Contains(text, "${TOOLS_AI_PASSWORD}") || !strings.Contains(text, "${TOOLS_GITHUB_TOKEN}") {
		t.Fatalf("unchanged references were not preserved:\n%s", text)
	}
}

func TestUnifiedConfigRejectsMissingEnvironmentReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_MISSING_GITHUB_TOKEN", "")
	if err := os.WriteFile(path, []byte(`copilot:
  auth:
    github_access_token: "${TOOLS_MISSING_GITHUB_TOKEN}"
inspect_image: {}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	var missing *configenv.MissingEnvReferenceError
	if !errors.As(err, &missing) || missing.Name != "TOOLS_MISSING_GITHUB_TOKEN" {
		t.Fatalf("expected missing environment reference error, got %v", err)
	}
}

func TestUnifiedConfigReportsInvalidCopilotTokenFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	tokenPath := filepath.Join(dir, "tmp", "copilot_token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("copilot_token: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	body := []byte(`
version: 1
copilot:
  auth:
    copilot_token_file: ` + tokenPath + `
inspect_image: {}
`)
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected invalid token file to be reported")
	}
}

func TestSaveMigratesLegacyInspectJSONWithoutTopLevelAuth(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "inspect-image.json")
	tokenPath := filepath.Join(dir, "tmp", "copilot_token")
	legacy := []byte(`{
  "version": 1,
  "provider": "github_copilot_plugin",
  "api": {"endpoint_kind": "responses", "base_url": "https://api.githubcopilot.com", "timeout_seconds": 90, "use_system_proxy": true},
  "defaults": {"model": "gpt-5.4", "reasoning": "medium", "output": "text"},
  "limits": {"max_image_bytes": 3145728, "max_images_per_call": 1, "allowed_mime_types": ["image/png"]},
  "auth": {"method": "device_code", "github_host": "github.com", "github_access_token": "gh-secret", "copilot_token": "cp-secret", "copilot_token_expires_at": "2099-01-01T00:00:00Z"},
  "privacy": {"redact_tokens_in_logs": true}
}`)
	if err := os.WriteFile(cfgPath, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Auth.CopilotTokenFile = tokenPath
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	for _, legacyKey := range []string{"provider", "api", "defaults", "limits", "auth", "privacy"} {
		if _, ok := root[legacyKey]; ok {
			t.Fatalf("legacy top-level key %q was preserved:\n%s", legacyKey, string(b))
		}
	}
	if strings.Contains(string(b), "cp-secret") {
		t.Fatalf("copilot token leaked into main config:\n%s", string(b))
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tokenBytes), "cp-secret") {
		t.Fatalf("copilot token was not written to token file: %s", string(tokenBytes))
	}
}

func TestSaveStoresAIPlatformTokenOutsideMainConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	tokenPath := filepath.Join(dir, "tmp", "ai_platform_token")
	cfg := Default()
	cfg.Provider = ProviderAIPlatform
	cfg.AIPlatform.Chat.Host = "https://ai.example"
	cfg.AIPlatform.IB2B.Host = "https://ib2b.example"
	cfg.AIPlatform.Auth.Username = "alice"
	cfg.AIPlatform.Auth.Password = "secret-password"
	cfg.AIPlatform.Auth.Usercase = "case-123"
	cfg.AIPlatform.Auth.Token = "short-lived-token"
	cfg.AIPlatform.Auth.TokenExpiresAt = "2099-01-01T00:00:00Z"
	cfg.AIPlatform.Auth.TokenFile = tokenPath
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "short-lived-token") {
		t.Fatalf("ai platform token leaked into main config:\n%s", string(b))
	}
	if !strings.Contains(string(b), "provider: ai_platform") || !strings.Contains(string(b), "usercase: case-123") {
		t.Fatalf("ai platform config missing expected fields:\n%s", string(b))
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tokenBytes), "short-lived-token") {
		t.Fatalf("ai platform token was not written to token file: %s", string(tokenBytes))
	}
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Provider != ProviderAIPlatform || loaded.AIPlatform.Auth.Token != "short-lived-token" || loaded.AIPlatform.Auth.Usercase != "case-123" {
		t.Fatalf("bad loaded ai platform config: %#v", loaded)
	}
}

func TestSavePreservesUnrelatedNodeComments(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	tokenPath := filepath.Join(dir, "tmp", "copilot_token")
	initial := []byte(`
version: 1
# keep jira comment
jira:
  # keep default comment
  default_instance: local
  instances: []
`)
	if err := os.WriteFile(cfgPath, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.Auth.CopilotTokenFile = tokenPath
	if err := Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "keep jira comment") || !strings.Contains(string(b), "keep default comment") {
		t.Fatalf("comments were not preserved:\n%s", string(b))
	}
}

func mustReadConfigFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
