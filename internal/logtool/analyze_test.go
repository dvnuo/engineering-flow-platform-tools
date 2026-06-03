package logtool

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeSearchWindowExtractAndRunFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO service started",
		"2026-06-03T10:01:00Z ERROR database password=secret timeout after 3000ms",
		"java.lang.RuntimeException: boom",
		"    at example.Main.main(Main.java:10)",
		"2026-06-03T10:02:00Z ERROR database password=secret timeout after 5000ms",
		"2026-06-03T10:03:00Z ERROR database password=secret timeout after 7000ms",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	result, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir, FormatHint: "auto", MaxLineBytes: 65536, ToolVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.EntriesCount != 4 || result.TemplatesCount == 0 {
		t.Fatalf("bad analyze result: %#v", result)
	}
	for _, name := range []string{"manifest.json", "entries.jsonl", "templates.json"} {
		b, err := readTestFile(filepath.Join(runDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "secret") {
			t.Fatalf("secret leaked in %s: %s", name, string(b))
		}
	}
	search, err := Search(runDir, SearchOptions{Query: "timeout", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if search.Matches != 3 || len(search.Items) != 1 || search.NextCursor == "" {
		t.Fatalf("bad search page: %#v", search)
	}
	next, err := Search(runDir, SearchOptions{Query: "timeout", Limit: 1, Cursor: search.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Items) != 1 || next.Items[0].EntryID == search.Items[0].EntryID {
		t.Fatalf("bad next page: %#v", next)
	}
	window, err := WindowByEntry(runDir, "entry_000002", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Lines) != 5 || !window.Lines[1].Target || strings.Contains(window.Lines[1].Text, "secret") {
		t.Fatalf("bad window: %#v", window)
	}
	extract, err := Extract(runDir, "stacktrace", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(extract.Items) != 1 || extract.Items[0].RepresentativeEntryID != "entry_000002" {
		t.Fatalf("bad extract: %#v", extract)
	}
	templates, err := Templates(runDir, "all", "count", 20)
	if err != nil {
		t.Fatal(err)
	}
	foundRepeated := false
	for _, tpl := range templates.Templates {
		if strings.Contains(tpl.Template, "timeout") && tpl.Count == 2 {
			foundRepeated = true
		}
	}
	if !foundRepeated {
		t.Fatalf("timeout template was not aggregated: %#v", templates)
	}
}

func readTestFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func TestWindowSourceMissing(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	if err := os.WriteFile(logPath, []byte("2026-06-03T10:00:00Z ERROR boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	if _, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	_, err := WindowByEntry(runDir, "entry_000001", 1, 1)
	var toolErr *ToolError
	if err == nil || !strings.Contains(err.Error(), "Source log file") {
		t.Fatalf("expected source_missing, got %v", err)
	}
	if ok := asToolError(err, &toolErr); !ok || toolErr.Code != "source_missing" {
		t.Fatalf("expected source_missing, got %#v", err)
	}
}

func asToolError(err error, target **ToolError) bool {
	if e, ok := err.(*ToolError); ok {
		*target = e
		return true
	}
	return false
}
