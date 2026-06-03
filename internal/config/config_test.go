package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNormalizeAuthCompatibility(t *testing.T) {
	c := RootConfig{Jira: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Username: "u", Password: "p"}}, {Auth: AuthConfig{Username: "u", APIKey: "k"}}, {Auth: AuthConfig{Token: "t"}}, {Auth: AuthConfig{Username: "u", Token: "legacy"}}}}, Jenkins: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Username: "jenkins", Token: "api-token"}}}}}
	c.Normalize()
	if c.Jira.Instances[0].Auth.Type != "basic_password" || c.Jira.Instances[1].Auth.Type != "basic_api_key" || c.Jira.Instances[2].Auth.Type != "bearer_token" || c.Jira.Instances[3].Auth.Type != "basic_api_key" {
		t.Fatalf("normalization failed")
	}
	if c.Jenkins.Instances[0].Auth.Type != "basic_api_key" || c.Jenkins.Instances[0].Auth.APIKey != "api-token" {
		t.Fatalf("jenkins normalization failed")
	}
	if c.Jira.Instances[3].Auth.APIKey != "legacy" || c.Jira.Instances[3].Auth.Token != "" {
		t.Fatalf("legacy token+username should become api_key")
	}
}

func TestRedactAuth(t *testing.T) {
	a := AuthConfig{Password: "p", APIKey: "k", Token: "t"}
	r := RedactAuth(a)
	if r.Password == "p" || r.APIKey == "k" || r.Token == "t" {
		t.Fatalf("secret leaked")
	}
}

func TestDefaultPathUsesEFPConfig(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	t.Setenv(EnvLegacyConfigPath, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".efp", "config.yaml") {
		t.Fatalf("path=%q", got)
	}
}

func TestSavePreservesOtherTopLevelNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte("version: 1\n# keep copilot node\ncopilot:\n  # keep provider comment\n  provider: github_copilot_plugin\ninspect_image:\n  defaults:\n    model: gpt-5.4\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := RootConfig{Version: 1, Jira: ProductConfig{DefaultInstance: "jira-main"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	if root["copilot"] == nil || root["inspect_image"] == nil || root["jira"] == nil {
		t.Fatalf("top-level nodes were not preserved: %s", string(b))
	}
	if !strings.Contains(string(b), "keep copilot node") || !strings.Contains(string(b), "keep provider comment") {
		t.Fatalf("comments were not preserved: %s", string(b))
	}
}

func TestSaveWritesJenkinsNode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := RootConfig{Version: 1, Jenkins: ProductConfig{DefaultInstance: "ci", Instances: []InstanceConfig{{Name: "ci", BaseURL: "https://jenkins.example.test", CrumbMode: "auto", Auth: AuthConfig{Type: "pat", Token: "secret"}}}}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Jenkins.DefaultInstance != "ci" || len(loaded.Jenkins.Instances) != 1 || loaded.Jenkins.Instances[0].CrumbMode != "auto" {
		t.Fatalf("jenkins config not preserved: %#v", loaded.Jenkins)
	}
	redacted := RedactRoot(loaded)
	if redacted.Jenkins.Instances[0].Auth.APIKey == "secret" || redacted.Jenkins.Instances[0].Auth.Token == "secret" {
		t.Fatalf("jenkins secret leaked after redaction")
	}
}
