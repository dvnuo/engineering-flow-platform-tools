package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandsJSONIncludesRunStart(t *testing.T) {
	out := runMobile(t, "commands", "--json")
	data := out["data"].(map[string]any)
	commands := data["commands"].([]any)
	for _, raw := range commands {
		item := raw.(map[string]any)
		if item["name"] == "run.start" {
			return
		}
	}
	t.Fatalf("run.start not found in commands: %#v", commands)
}

func TestSchemaRunStartIncludesCoreFlags(t *testing.T) {
	out := runMobile(t, "schema", "run.start", "--json")
	data := out["data"].(map[string]any)
	flags := data["flags"].([]any)
	have := map[string]bool{}
	for _, raw := range flags {
		flag := raw.(map[string]any)
		have[flag["name"].(string)] = true
	}
	for _, want := range []string{"app", "file", "platform", "device", "network", "wait-capacity", "json"} {
		if !have[want] {
			t.Fatalf("missing --%s in %#v", want, flags)
		}
	}
}

func TestInvalidArgsReturnsJSON(t *testing.T) {
	out := runMobile(t, "app", "upload", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--file") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestTypeMissingTextEnvFailsBeforeServiceSetup(t *testing.T) {
	t.Setenv("MOBILE_TEST_SECRET_MISSING", "")
	out := runMobile(t, "type", "--run-id", "run-1", "--ref", "obs-1:e1", "--text-env", "MOBILE_TEXT_ENV_DOES_NOT_EXIST", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "MOBILE_TEXT_ENV_DOES_NOT_EXIST") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestRunStartInvalidTimeoutFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "run", "start", "--timeout", "soon", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--timeout") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestRunFinishInvalidStatusFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "run", "finish", "--run-id", "run-1", "--status", "done", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--status") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestTunnelStopMissingMetadataFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "tunnel", "stop", "--yes", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--run-id") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestAppResolveInvalidAppURLFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "app", "resolve", "--app-url", " app-id-without-prefix ", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "bs://") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func runMobile(t *testing.T, args ...string) map[string]any {
	t.Helper()
	cmd := NewRoot()
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, b.String())
	}
	return out
}
