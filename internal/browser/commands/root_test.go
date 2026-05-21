package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/browser/probe"
)

type fakeRunner struct {
	result probe.ProbeResult
	err    error
	got    probe.ProbeOptions
	calls  int
}

func (f *fakeRunner) Probe(ctx context.Context, opts probe.ProbeOptions) (probe.ProbeResult, error) {
	f.calls++
	f.got = opts
	return f.result, f.err
}

func TestCommandsJSONIncludesProbe(t *testing.T) {
	out := run(t, &fakeRunner{}, "commands", "--json")
	data := out["data"].(map[string]any)
	commands := data["commands"].([]any)
	for _, item := range commands {
		m := item.(map[string]any)
		if m["name"] == "probe" || strings.Contains(m["usage"].(string), "browser probe") {
			return
		}
	}
	t.Fatalf("commands did not contain probe: %#v", commands)
}

func TestSchemaProbeRequiresURL(t *testing.T) {
	out := run(t, &fakeRunner{}, "schema", "probe", "--json")
	data := out["data"].(map[string]any)
	required := data["required"].([]any)
	for _, item := range required {
		if item == "url" {
			return
		}
	}
	t.Fatalf("schema did not require url: %#v", data)
}

func TestVersionJSON(t *testing.T) {
	out := run(t, &fakeRunner{}, "version", "--json")
	if out["ok"] != true {
		t.Fatalf("version failed: %#v", out)
	}
}

func TestProbeRequiresURL(t *testing.T) {
	out := run(t, &fakeRunner{}, "probe", "--json")
	if out["ok"] != false {
		t.Fatalf("missing url should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" {
		t.Fatalf("code = %#v", errObj)
	}
}

func TestProbeUsesRunner(t *testing.T) {
	fake := &fakeRunner{result: probe.ProbeResult{Selector: ".user", SelectorFound: true}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--selector", ".user", "--json")
	if out["ok"] != true {
		t.Fatalf("probe failed: %#v", out)
	}
	if fake.calls != 1 || fake.got.URL != "https://intranet.test" || fake.got.Selector != ".user" {
		t.Fatalf("runner not called with flags: calls=%d opts=%#v", fake.calls, fake.got)
	}
	data := out["data"].(map[string]any)
	if data["selector_found"] != true {
		t.Fatalf("selector_found = %#v", data)
	}
}

func TestProbeErrorEnvelope(t *testing.T) {
	fake := &fakeRunner{err: &probe.ProbeError{Code: "browser_not_found", Message: "missing", Hint: "install browser", Status: 404}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--json")
	if out["ok"] != false {
		t.Fatalf("probe should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "browser_not_found" {
		t.Fatalf("code = %#v", errObj)
	}
}

func TestRequireSelectorFailsWhenRunnerDoesNot(t *testing.T) {
	fake := &fakeRunner{result: probe.ProbeResult{Selector: ".user", SelectorFound: false}}
	out := run(t, fake, "probe", "--url", "https://intranet.test", "--selector", ".user", "--require-selector", "--json")
	if out["ok"] != false {
		t.Fatalf("probe should fail: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "selector_not_found" {
		t.Fatalf("code = %#v", errObj)
	}
}

func run(t *testing.T, r probe.Runner, args ...string) map[string]any {
	t.Helper()
	cmd := NewRootWithRunner(r)
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
