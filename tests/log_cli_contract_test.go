package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lcmd "engineering-flow-platform-tools/internal/logtool/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func runLog(t *testing.T, args ...string) ([]byte, map[string]any) {
	t.Helper()
	var b bytes.Buffer
	cmd := lcmd.NewRoot()
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("log %v failed: %v\n%s", args, err, b.String())
	}
	return b.Bytes(), testutil.AssertJSONEnvelope(t, b.Bytes())
}

func TestLogGoRunVersionJSONEnvelope(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/log", "version", "--json")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run ./cmd/log version --json failed: %v\n%s", err, string(out))
	}
	obj := testutil.AssertOKEnvelope(t, out)
	data := obj["data"].(map[string]any)
	if data["version"] == "" {
		t.Fatalf("missing version: %s", string(out))
	}
}

func TestLogCommandsJSONMetadata(t *testing.T) {
	_, obj := runLog(t, "commands", "--json")
	data := obj["data"].(map[string]any)
	if data["product"] != "log" {
		t.Fatalf("bad product: %#v", data)
	}
	commands := data["commands"].([]any)
	if len(commands) < 10 {
		t.Fatalf("expected at least 10 commands, got %d", len(commands))
	}
	for _, raw := range commands {
		item := raw.(map[string]any)
		for _, key := range []string{"name", "usage", "risk", "description", "examples", "flags"} {
			if _, ok := item[key]; !ok {
				t.Fatalf("missing %s in %#v", key, item)
			}
		}
		desc := strings.TrimSpace(item["description"].(string))
		if desc == "" || strings.HasPrefix(desc, "Run ") || strings.Contains(strings.ToLower(desc), "placeholder") {
			t.Fatalf("placeholder description: %#v", item)
		}
		if len(item["examples"].([]any)) == 0 || len(item["flags"].([]any)) == 0 {
			t.Fatalf("missing examples or flags: %#v", item)
		}
	}
}

func TestLogAnalyzeProfileSearchWindowExtractCLI(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO service started",
		"2026-06-03T10:01:00Z ERROR database password=secret timeout after 3000ms",
		"java.lang.RuntimeException: boom",
		"    at example.Main.main(Main.java:10)",
		"2026-06-03T10:02:00Z ERROR database password=secret timeout after 5000ms",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	out, obj := runLog(t, "analyze", "--source", logPath, "--run", runDir, "--json")
	assertNoLiteral(t, string(out), "secret")
	data := obj["data"].(map[string]any)
	if data["entries_count"].(float64) != 3 {
		t.Fatalf("bad analyze data: %#v", data)
	}
	for _, name := range []string{"manifest.json", "entries.jsonl", "templates.json"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	_, profile := runLog(t, "profile", "--run", runDir, "--json")
	if profile["ok"] != true {
		t.Fatalf("profile failed: %#v", profile)
	}
	_, search := runLog(t, "search", "--run", runDir, "--query", "timeout", "--limit", "1", "--json")
	searchData := search["data"].(map[string]any)
	if searchData["matches"].(float64) != 2 || len(searchData["items"].([]any)) != 1 || searchData["next_cursor"] == "" {
		t.Fatalf("bad search: %#v", searchData)
	}
	_, next := runLog(t, "search", "--run", runDir, "--query", "timeout", "--limit", "1", "--cursor", searchData["next_cursor"].(string), "--json")
	if len(next["data"].(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("bad search next: %#v", next)
	}
	_, window := runLog(t, "window", "--run", runDir, "--entry-id", "entry_000002", "--before", "1", "--after", "1", "--json")
	lines := window["data"].(map[string]any)["lines"].([]any)
	if len(lines) == 0 {
		t.Fatalf("missing window lines: %#v", window)
	}
	targetSeen := false
	for _, raw := range lines {
		line := raw.(map[string]any)
		if line["target"] == true {
			targetSeen = true
		}
	}
	if !targetSeen {
		t.Fatalf("window did not mark target: %#v", window)
	}
	_, extract := runLog(t, "extract", "--run", runDir, "--kind", "stacktrace", "--json")
	items := extract["data"].(map[string]any)["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("missing stacktrace extract: %#v", extract)
	}
}

func TestLogInvalidRunDirJSONError(t *testing.T) {
	_, obj := runLog(t, "profile", "--run", filepath.Join(t.TempDir(), "missing"), "--json")
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != "run_not_found" {
		b, _ := json.Marshal(obj)
		t.Fatalf("expected run_not_found: %s", string(b))
	}
}

func TestLogRunBasedCommandsRequireRun(t *testing.T) {
	cases := [][]string{
		{"profile", "--json"},
		{"templates", "--json"},
		{"entries", "--json"},
		{"search", "--query", "timeout", "--json"},
		{"extract", "--kind", "stacktrace", "--json"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, obj := runLog(t, args...)
			if obj["ok"] != false {
				t.Fatalf("expected failure: %#v", obj)
			}
			errObj := obj["error"].(map[string]any)
			if errObj["code"] != "invalid_args" {
				b, _ := json.Marshal(obj)
				t.Fatalf("expected invalid_args: %s", string(b))
			}
			if !strings.Contains(errObj["message"].(string), "--run is required") {
				t.Fatalf("expected --run message: %#v", errObj)
			}
		})
	}
}

func TestLogSourceMissingWindowJSONError(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("2026-06-03T10:00:00Z ERROR boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	runLog(t, "analyze", "--source", logPath, "--run", runDir, "--json")
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	_, obj := runLog(t, "window", "--run", runDir, "--entry-id", "entry_000001", "--json")
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != "source_missing" {
		b, _ := json.Marshal(obj)
		t.Fatalf("expected source_missing: %s", string(b))
	}
}

func assertNoLiteral(t *testing.T, s, literal string) {
	t.Helper()
	if strings.Contains(s, literal) {
		t.Fatalf("unexpected literal %q in %s", literal, s)
	}
}
