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
	seen := map[string]bool{}
	for _, raw := range commands {
		item := raw.(map[string]any)
		seen[item["name"].(string)] = true
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
	for _, name := range []string{"doctor", "run.list", "run.get", "run.delete", "template.list", "template.get", "template.entries", "group", "timeline", "summarize", "export.evidence"} {
		if !seen[name] {
			t.Fatalf("missing P0 design command %s in log commands", name)
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
	_, cursorOnly := runLog(t, "search", "--run", runDir, "--limit", "1", "--cursor", searchData["next_cursor"].(string), "--json")
	if len(cursorOnly["data"].(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("bad cursor-only search next: %#v", cursorOnly)
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

func TestLogP0DesignCommandsCLI(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO service started",
		"2026-06-03T10:01:00Z ERROR api database password=secret timeout after 3000ms",
		"java.lang.RuntimeException: boom",
		"    at example.Main.main(Main.java:10)",
		"2026-06-03T10:02:00Z ERROR api database password=secret timeout after 5000ms",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	dryRunDir := filepath.Join(dir, "dry-run")
	out, dryRun := runLog(t, "analyze", "--source", logPath, "--run", dryRunDir, "--dry-run", "--json")
	assertNoLiteral(t, string(out), "secret")
	if dryRun["data"].(map[string]any)["dry_run"] != true {
		t.Fatalf("expected dry-run analyze: %#v", dryRun)
	}
	if _, err := os.Stat(filepath.Join(dryRunDir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote manifest: %v", err)
	}

	runDir := filepath.Join(dir, "run")
	runLog(t, "analyze", "--source", logPath, "--run", runDir, "--json")
	runLog(t, "doctor", "--json")
	runLog(t, "run", "get", runDir, "--json")
	runLog(t, "run", "verify", runDir, "--json")
	_, runs := runLog(t, "run", "list", "--workspace", dir, "--json")
	if len(runs["data"].(map[string]any)["runs"].([]any)) == 0 {
		t.Fatalf("run list did not include temp run: %#v", runs)
	}

	_, templateList := runLog(t, "template", "list", runDir, "--only", "non-info", "--json")
	templates := templateList["data"].(map[string]any)["templates"].([]any)
	if len(templates) == 0 {
		t.Fatalf("missing templates: %#v", templateList)
	}
	templateID := templates[0].(map[string]any)["template_id"].(string)
	runLog(t, "template", "get", runDir, "--template", templateID, "--json")
	runLog(t, "template", "entries", runDir, "--template", templateID, "--limit", "2", "--json")
	runLog(t, "template", "variables", runDir, "--template", templateID, "--json")

	_, group := runLog(t, "group", runDir, "--by", "error_signature", "--json")
	if len(group["data"].(map[string]any)["groups"].([]any)) == 0 {
		t.Fatalf("missing groups: %#v", group)
	}
	_, timeline := runLog(t, "timeline", runDir, "--bucket", "1m", "--json")
	if len(timeline["data"].(map[string]any)["series"].([]any)) == 0 {
		t.Fatalf("missing timeline: %#v", timeline)
	}
	_, summary := runLog(t, "summarize", runDir, "--focus", "database errors", "--json")
	if len(summary["data"].(map[string]any)["findings"].([]any)) == 0 {
		t.Fatalf("missing summary findings: %#v", summary)
	}

	exportPath := filepath.Join(dir, "evidence.md")
	_, exportDryRun := runLog(t, "export", "evidence", runDir, "--evidence", "entry_000002", "--format", "markdown", "--output", exportPath, "--dry-run", "--json")
	if exportDryRun["data"].(map[string]any)["written"] == true {
		t.Fatalf("dry-run export wrote file: %#v", exportDryRun)
	}
	if _, err := os.Stat(exportPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run export created file: %v", err)
	}
	runLog(t, "export", "evidence", runDir, "--evidence", "entry_000002", "--format", "markdown", "--output", exportPath, "--json")
	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	assertNoLiteral(t, string(exported), "secret")
	_, exists := runLog(t, "export", "evidence", runDir, "--evidence", "entry_000002", "--format", "markdown", "--output", exportPath, "--json")
	if exists["ok"] != false || exists["error"].(map[string]any)["code"] != "log_export_exists" {
		t.Fatalf("expected export exists error: %#v", exists)
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
