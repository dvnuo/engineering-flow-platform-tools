package logtool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowByFileLineInRunBoundsVeryLongLine(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	longLine := "2026-06-03T10:01:00Z ERROR password=supersecret " + strings.Repeat("A", 2*1024*1024)
	content := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO start",
		longLine,
		"2026-06-03T10:02:00Z INFO done",
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	if _, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir, FormatHint: "auto", MaxLineBytes: 1024, ToolVersion: "test"}); err != nil {
		t.Fatal(err)
	}

	window, err := WindowByFileLineInRun(runDir, logPath, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Lines) != 1 || !window.Lines[0].Target {
		t.Fatalf("bad window lines: %#v", window)
	}
	text := window.Lines[0].Text
	if !strings.Contains(text, lineTruncatedMarker) {
		t.Fatalf("missing truncation marker: %q", text)
	}
	if strings.Contains(text, "supersecret") {
		t.Fatalf("secret leaked in window text: %q", text)
	}
	if len(text) >= len(longLine)/10 {
		t.Fatalf("window line was not bounded: got=%d source=%d", len(text), len(longLine))
	}
}

func TestWindowByEntryBoundsVeryLongLine(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	longLine := "2026-06-03T10:01:00Z ERROR api_key=supersecret " + strings.Repeat("B", 2*1024*1024)
	content := strings.Join([]string{
		"2026-06-03T10:00:00Z INFO start",
		longLine,
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	if _, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir, FormatHint: "auto", MaxLineBytes: 1024, ToolVersion: "test"}); err != nil {
		t.Fatal(err)
	}

	window, err := WindowByEntry(runDir, "entry_000002", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Lines) != 1 || !window.Lines[0].Target {
		t.Fatalf("bad window lines: %#v", window)
	}
	text := window.Lines[0].Text
	if !strings.Contains(text, lineTruncatedMarker) {
		t.Fatalf("missing truncation marker: %q", text)
	}
	if strings.Contains(text, "supersecret") {
		t.Fatalf("secret leaked in entry window text: %q", text)
	}
	if len(text) >= len(longLine)/10 {
		t.Fatalf("window line was not bounded: got=%d source=%d", len(text), len(longLine))
	}
}

func TestWindowByEntryRejectsTamperedSourcePath(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := "2026-06-03T10:00:00Z INFO start\n2026-06-03T10:01:00Z ERROR timeout\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(dir, "outside.log")
	if err := os.WriteFile(outsidePath, []byte("outside content must not be read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	if _, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir, FormatHint: "auto", ToolVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	tamperEntry(t, runDir, "entry_000002", func(entry *Entry) {
		entry.SourcePath = outsidePath
	})

	_, err := WindowByEntry(runDir, "entry_000002", 0, 0)
	requireToolErrorCode(t, err, "entry_source_not_in_run")
	if strings.Contains(err.Error(), "outside content") {
		t.Fatalf("outside source content leaked in error: %v", err)
	}
}

func TestWindowByEntryRejectsTamperedLineRange(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	content := "2026-06-03T10:00:00Z INFO start\n2026-06-03T10:01:00Z ERROR timeout\n"
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(dir, "run")
	if _, err := Analyze(context.Background(), AnalyzeOptions{Source: logPath, RunDir: runDir, FormatHint: "auto", ToolVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	tamperEntry(t, runDir, "entry_000002", func(entry *Entry) {
		entry.LineStart = 999999
		entry.LineEnd = 999999
	})

	_, err := WindowByEntry(runDir, "entry_000002", 0, 0)
	requireToolErrorCode(t, err, "entry_outside_run_source_range")
}

func tamperEntry(t *testing.T, runDir, entryID string, mutate func(*Entry)) {
	t.Helper()
	path := filepath.Join(runDir, entriesFile)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	found := false
	var out strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.EntryID == entryID {
			mutate(&entry)
			found = true
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		out.Write(encoded)
		out.WriteByte('\n')
	}
	if !found {
		t.Fatalf("entry %s not found", entryID)
	}
	if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireToolErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var toolErr *ToolError
	if err == nil || !errors.As(err, &toolErr) {
		t.Fatalf("expected ToolError %s, got %v", code, err)
	}
	if toolErr.Code != code {
		t.Fatalf("expected %s, got %#v", code, toolErr)
	}
}
