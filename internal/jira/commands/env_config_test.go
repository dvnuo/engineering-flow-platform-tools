package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func runEnvJSON(t *testing.T, stdin string, args ...string) map[string]any {
	t.Helper()
	cmd := NewRoot()
	b := &bytes.Buffer{}
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(b)
	cmd.SetErr(b)
	cmd.SetArgs(append([]string{"--json"}, args...))
	_ = cmd.Execute()
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil || out == nil {
		t.Fatalf("no JSON envelope: %v output=%s", err, b.String())
	}
	return out
}

func TestInstanceListReadsEnvConfigJSONWithoutFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigPath, "")
	t.Setenv(config.EnvLegacyConfigPath, "")
	t.Setenv(config.EnvConfigJSON, `{"jira":{"default_instance":"env-jira","instances":[{"name":"env-jira","base_url":"https://jira.example.test","rest_path":"/rest/api/2","auth":{"type":"bearer_token","token":"secret-token"}}]}}`)

	out := runEnvJSON(t, "", "instance", "list")
	if out["ok"] != true {
		t.Fatalf("instance list must resolve from EFP_CONFIG_JSON with no file: %#v", out)
	}
	raw, _ := json.Marshal(out)
	if !strings.Contains(string(raw), "env-jira") {
		t.Fatalf("env instance missing from output: %s", raw)
	}
	if strings.Contains(string(raw), "secret-token") {
		t.Fatalf("secret leaked in instance list output: %s", raw)
	}
}

func TestInstanceAddRefusedWhenEnvManaged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigPath, "")
	t.Setenv(config.EnvLegacyConfigPath, "")
	t.Setenv(config.EnvConfigJSON, `{"jira":{"instances":[]}}`)

	out := runEnvJSON(t, "tok\n", "instance", "add", "new-jira", "--base-url", "https://jira.example.test", "--token-stdin")
	if out["ok"] == true {
		t.Fatalf("instance add must be refused under EFP_CONFIG_JSON management: %#v", out)
	}
	raw, _ := json.Marshal(out)
	if !strings.Contains(string(raw), "config_env_managed") {
		t.Fatalf("expected config_env_managed refusal, got: %s", raw)
	}
}
