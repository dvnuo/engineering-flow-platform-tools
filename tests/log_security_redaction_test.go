package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogSecretsDoNotAppearInOutputsOrRunFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := strings.Join([]string{
		`2026-06-03T10:00:00Z ERROR Authorization: Bearer bearersecretshouldnotappear password=secret api_key=xyz token=tok secret=hidden user@example.test timeout after 3000ms`,
		"Traceback (most recent call last):",
		`  File "/srv/app.py", line 10, in main`,
		"Exception: boom",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	var combined strings.Builder
	for _, args := range [][]string{
		{"analyze", "--source", logPath, "--run", runDir, "--json"},
		{"search", "--run", runDir, "--query", "timeout", "--json"},
		{"window", "--run", runDir, "--entry-id", "entry_000001", "--before", "0", "--after", "3", "--json"},
		{"extract", "--run", runDir, "--kind", "stacktrace", "--json"},
	} {
		out, _ := runLog(t, args...)
		combined.Write(out)
	}
	for _, name := range []string{"entries.jsonl", "templates.json"} {
		b, err := os.ReadFile(filepath.Join(runDir, name))
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(b)
	}
	for _, leak := range []string{"bearersecretshouldnotappear", "password=secret", "api_key=xyz", "token=tok", "secret=hidden", "user@example.test"} {
		assertNoLiteral(t, combined.String(), leak)
	}
}
