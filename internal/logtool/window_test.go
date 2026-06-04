package logtool

import (
	"context"
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
