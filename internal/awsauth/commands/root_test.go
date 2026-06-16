package commands

import (
	"bytes"
	"context"
	"encoding/json"
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
	obj := runJSON(t, runner, "--config", cfg, "login", "--json")
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
	for _, token := range []string{"--jenkins", "-n", "-d HBEU", "-u aws-user"} {
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
	if !strings.Contains(data["command"].(string), "adfs-assume --jenkins -n -d HBEU -u aws-user") {
		t.Fatalf("unexpected command data: %#v", data)
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
	obj := runJSON(t, runner, "--config", cfg, "login", "--json")
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
	for _, item := range data["commands"].([]any) {
		cmd := item.(map[string]any)
		if cmd["name"] == "login" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("login missing from commands: %#v", data)
	}

	schema := runJSON(t, runner, "schema", "login", "--json")
	if schema["ok"] != true {
		t.Fatalf("schema failed: %#v", schema)
	}
}
