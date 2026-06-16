package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"engineering-flow-platform-tools/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	result commandResult
	err    error
	calls  []fakeCall
}

type fakeCall struct {
	command string
	args    []string
	env     []string
}

func (r *fakeRunner) Run(ctx context.Context, command string, args []string, env []string) (commandResult, error) {
	r.calls = append(r.calls, fakeCall{command: command, args: append([]string{}, args...), env: append([]string{}, env...)})
	return r.result, r.err
}

func runJSON(t *testing.T, runner *fakeRunner, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithRunner(runner)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\n%s", err, out.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	return obj
}

func runJSONInput(t *testing.T, runner *fakeRunner, input string, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithRunner(runner)
	var out bytes.Buffer
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\n%s", err, out.String())
	}
	var obj map[string]any
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	return obj
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoginRunsAdfsAssumeWithConfiguredADPass(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)
	runner := &fakeRunner{}
	obj := runJSON(t, runner, "--config", cfg, "login", "--account", "123456", "--role", "ADFS-ReadOnly", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.command != "adfs-assume" {
		t.Fatalf("unexpected command: %s", call.command)
	}
	args := strings.Join(call.args, " ")
	for _, token := range []string{"--domain HBEU", "--username aws-user", "--role ADFS-ReadOnly", "--account 123456", "--profile default", "--no-warning", "--display-token", "--jenkins"} {
		if !strings.Contains(args, token) {
			t.Fatalf("missing %q in args %#v", token, call.args)
		}
	}
	if strings.Contains(args, "aws-password") {
		t.Fatalf("password leaked into args: %#v", call.args)
	}
	if got := envValue(call.env, "AD_PASS"); got != "aws-password" {
		t.Fatalf("AD_PASS not passed to provider: %q", got)
	}
	out, _ := json.Marshal(obj)
	if strings.Contains(string(out), "aws-password") {
		t.Fatalf("password leaked into output: %s", string(out))
	}
	data := obj["data"].(map[string]any)
	if data["authenticated"] != true {
		t.Fatalf("expected authenticated data: %#v", data)
	}
	if !strings.Contains(data["command"].(string), "adfs-assume --domain HBEU --username aws-user --role ADFS-ReadOnly --account 123456 --profile default --no-warning --display-token --jenkins") {
		t.Fatalf("unexpected command data: %#v", data)
	}
}

func TestLoginRunsAdfsAssumeWithCustomProfile(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)
	runner := &fakeRunner{}
	obj := runJSON(t, runner, "--config", cfg, "login", "--account", "123456", "--role", "ADFS-ReadOnly", "--profile", "sandbox", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(runner.calls))
	}
	args := strings.Join(runner.calls[0].args, " ")
	if !strings.Contains(args, "--profile sandbox") {
		t.Fatalf("missing custom profile in args %#v", runner.calls[0].args)
	}
}

func TestAWSAuthIgnoresAtlassianConfigAndUsesDefaultEFPConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigPath, "")
	atlassianPath := filepath.Join(t.TempDir(), "atlassian.json")
	writeFile(t, atlassianPath, `{"version":1,"jira":{"instances":[]}}`)
	t.Setenv(config.EnvLegacyConfigPath, atlassianPath)

	defaultPath := filepath.Join(home, ".efp", "config.yaml")
	writeFile(t, defaultPath, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)

	obj := runJSON(t, &fakeRunner{}, "auth", "status", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["config_path"] != defaultPath {
		t.Fatalf("expected default EFP config path, got %#v", data["config_path"])
	}
	if data["configured"] != true {
		t.Fatalf("expected configured aws auth: %#v", data)
	}
}

func TestAWSAuthReadFallsBackToAdapterStateConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigPath, "")
	atlassianPath := filepath.Join(t.TempDir(), "atlassian.json")
	writeFile(t, atlassianPath, `{"version":1,"jira":{"instances":[]}}`)
	t.Setenv(config.EnvLegacyConfigPath, atlassianPath)
	stateDir := t.TempDir()
	t.Setenv(envAdapterStateDir, stateDir)

	stateConfig := filepath.Join(stateDir, "efp", "config.yaml")
	writeFile(t, stateConfig, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)

	obj := runJSON(t, &fakeRunner{}, "auth", "status", "--json")
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["config_path"] != stateConfig {
		t.Fatalf("expected adapter state EFP config path, got %#v", data["config_path"])
	}
	if data["configured"] != true {
		t.Fatalf("expected configured aws auth: %#v", data)
	}
}

func TestAuthLoginStoresAWSConfigWithoutPrintingPassword(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	obj := runJSONInput(
		t,
		&fakeRunner{},
		"aws-password\n",
		"--config", cfgPath,
		"auth", "login",
		"--domain", "HBEU",
		"--username", "GB-SVC-XXX-XXX",
		"--password-stdin",
		"--json",
	)
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	raw, _ := json.Marshal(obj)
	if strings.Contains(string(raw), "aws-password") {
		t.Fatalf("password leaked into response: %s", string(raw))
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AWS.Enabled == nil || !*cfg.AWS.Enabled || cfg.AWS.Domain != "HBEU" || cfg.AWS.Username != "GB-SVC-XXX-XXX" || cfg.AWS.Password != "aws-password" {
		t.Fatalf("bad aws config: %#v", cfg.AWS)
	}
}

func TestAuthLoginWritesDefaultEFPConfigNotAtlassianConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigPath, "")
	atlassianPath := filepath.Join(t.TempDir(), "atlassian.json")
	writeFile(t, atlassianPath, `{"version":1,"jira":{"instances":[]}}`)
	t.Setenv(config.EnvLegacyConfigPath, atlassianPath)

	obj := runJSONInput(
		t,
		&fakeRunner{},
		"aws-password\n",
		"auth", "login",
		"--domain", "HBEU",
		"--username", "GB-SVC-XXX-XXX",
		"--password-stdin",
		"--json",
	)
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	defaultPath := filepath.Join(home, ".efp", "config.yaml")
	data := obj["data"].(map[string]any)
	if data["config_path"] != defaultPath {
		t.Fatalf("expected default EFP config path, got %#v", data["config_path"])
	}
	atlassianBytes, err := os.ReadFile(atlassianPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(atlassianBytes), "aws-password") {
		t.Fatalf("password should not be written to ATLASSIAN_CONFIG: %s", atlassianBytes)
	}
	cfg, err := config.Load(defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AWS.Username != "GB-SVC-XXX-XXX" || cfg.AWS.Password != "aws-password" {
		t.Fatalf("bad default aws config: %#v", cfg.AWS)
	}
}

func TestAuthLoginDryRunDoesNotWriteConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	obj := runJSONInput(
		t,
		&fakeRunner{},
		"aws-password\n",
		"--config", cfgPath,
		"--dry-run",
		"auth", "login",
		"--domain", "HBEU",
		"--username", "GB-SVC-XXX-XXX",
		"--password-stdin",
		"--json",
	)
	if obj["ok"] != true {
		t.Fatalf("expected ok: %#v", obj)
	}
	data := obj["data"].(map[string]any)
	if data["dry_run"] != true || data["configured"] != false {
		t.Fatalf("expected dry-run data: %#v", data)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config should not be written during dry-run: %v", err)
	}
}

func TestLoginPromptsForMissingAccountAndRole(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)
	runner := &fakeRunner{}
	cmd := NewRootWithRunner(runner)
	var out bytes.Buffer
	cmd.SetIn(strings.NewReader("123456\nADFS-ReadOnly\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config", cfg, "login"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v\n%s", err, out.String())
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one provider call, got %d", len(runner.calls))
	}
	args := strings.Join(runner.calls[0].args, " ")
	for _, token := range []string{"--account 123456", "--role ADFS-ReadOnly"} {
		if !strings.Contains(args, token) {
			t.Fatalf("missing prompted %q in args %#v", token, runner.calls[0].args)
		}
	}
	if !strings.Contains(out.String(), "AWS account: ") || !strings.Contains(out.String(), "AWS role: ") {
		t.Fatalf("missing prompts in output: %q", out.String())
	}
}

func TestLoginJSONRequiresAccountAndRole(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: aws-password
`)
	runner := &fakeRunner{}
	obj := runJSON(t, runner, "--config", cfg, "login", "--json")
	if obj["ok"] != false {
		t.Fatalf("expected failure: %#v", obj)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("provider should not be called: %#v", runner.calls)
	}
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestLoginMissingConfigReturnsStableFailure(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  enabled: true
  domain: HBEU
  username: aws-user
  password: "***REDACTED***"
`)
	runner := &fakeRunner{}
	obj := runJSON(t, runner, "--config", cfg, "login", "--json")
	if obj["ok"] != false {
		t.Fatalf("expected failure: %#v", obj)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("provider should not be called: %#v", runner.calls)
	}
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != "config_missing" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestLoginFailureRedactsConfiguredPassword(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
aws:
  domain: HBEU
  username: aws-user
  password: aws-password
`)
	runner := &fakeRunner{result: commandResult{ExitCode: 1, Stderr: "login failed for aws-password"}}
	obj := runJSON(t, runner, "--config", cfg, "login", "--account", "123456", "--role", "ADFS-ReadOnly", "--json")
	if obj["ok"] != false {
		t.Fatalf("expected failure: %#v", obj)
	}
	out, _ := json.Marshal(obj)
	if strings.Contains(string(out), "aws-password") {
		t.Fatalf("password leaked into failure: %s", string(out))
	}
	if !strings.Contains(string(out), "***REDACTED***") {
		t.Fatalf("redaction marker missing: %s", string(out))
	}
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != "auth_failed" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestCommandsAndSchemaExposeLogin(t *testing.T) {
	runner := &fakeRunner{}
	commands := runJSON(t, runner, "commands", "--json")
	if commands["ok"] != true {
		t.Fatalf("commands failed: %#v", commands)
	}
	data := commands["data"].(map[string]any)
	found := false
	foundAuthLogin := false
	for _, item := range data["commands"].([]any) {
		cmd := item.(map[string]any)
		if cmd["name"] == "login" {
			found = true
		}
		if cmd["name"] == "auth.login" {
			foundAuthLogin = true
		}
	}
	if !found {
		t.Fatalf("login missing from commands: %#v", data)
	}
	if !foundAuthLogin {
		t.Fatalf("auth.login missing from commands: %#v", data)
	}

	schema := runJSON(t, runner, "schema", "login", "--json")
	if schema["ok"] != true {
		t.Fatalf("schema failed: %#v", schema)
	}
	authSchema := runJSON(t, runner, "schema", "auth.login", "--json")
	if authSchema["ok"] != true {
		t.Fatalf("auth schema failed: %#v", authSchema)
	}
}
