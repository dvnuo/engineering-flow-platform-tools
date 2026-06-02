package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
