package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogWindowFileLineRequiresSourceInRun(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("2026-06-03T10:00:00Z INFO start\n2026-06-03T10:01:00Z ERROR timeout\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	runLog(t, "analyze", "--source", logPath, "--run", runDir, "--json")

	_, sameRun := runLog(t, "window", "--run", runDir, "--file", logPath, "--line", "2", "--before", "1", "--after", "0", "--json")
	if sameRun["ok"] != true {
		b, _ := json.Marshal(sameRun)
		t.Fatalf("expected same-run source window to pass: %s", string(b))
	}
	lines := sameRun["data"].(map[string]any)["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("expected before+target lines: %#v", sameRun)
	}
	if lines[1].(map[string]any)["target"] != true {
		t.Fatalf("expected target line marker: %#v", sameRun)
	}

	secretPath := filepath.Join(dir, "secret.log")
	if err := os.WriteFile(secretPath, []byte("this must not leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, outsideRun := runLog(t, "window", "--run", runDir, "--file", secretPath, "--line", "1", "--before", "0", "--after", "0", "--json")
	if outsideRun["ok"] != false {
		b, _ := json.Marshal(outsideRun)
		t.Fatalf("expected outside-run source window to fail: %s", string(b))
	}
	errObj := outsideRun["error"].(map[string]any)
	if errObj["code"] != "source_not_in_run" {
		b, _ := json.Marshal(outsideRun)
		t.Fatalf("expected source_not_in_run: %s", string(b))
	}
	if strings.Contains(string(out), "this must not leak") {
		t.Fatalf("outside-run source leaked in output: %s", string(out))
	}
}
