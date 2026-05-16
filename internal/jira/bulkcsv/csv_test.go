package bulkcsv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCSVSupportsBOMAndQuotedMultiline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testcases.csv")
	content := "\ufeffCase Title,Steps\nLogin,\"Step 1\nStep 2\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := ParseCSV(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := data.Summary.Columns[0]; got != "Case Title" {
		t.Fatalf("first header = %q", got)
	}
	if got := data.Rows[0].Values["Steps"]; got != "Step 1\nStep 2" {
		t.Fatalf("multiline cell = %q", got)
	}
	if got := data.Rows[0].RowNumber; got != 2 {
		t.Fatalf("row number = %d", got)
	}
}

func TestParseCSVRejectsDuplicateHeaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte("Title, Title\nA,B\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseCSV(path, 5)
	if err == nil || !strings.Contains(err.Error(), "duplicate CSV header") {
		t.Fatalf("expected duplicate header error, got %v", err)
	}
}
