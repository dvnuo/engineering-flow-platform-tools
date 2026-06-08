package automation

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowRecordFileRedactsTypedText(t *testing.T) {
	events, file := workflowRecordFileFromEvents([]workflowRecordRawEvent{
		{Action: "page.click", Selector: "button#go"},
		{Action: "page.type", Selector: "input#secret", TextBytes: len("typed-secret")},
		{Action: "page.select", Selector: "select#role", TextBytes: len("admin")},
	}, "default", time.Unix(0, 0).UTC())
	if len(events) != 3 || len(file.Steps) != 3 {
		t.Fatalf("unexpected record file: %#v %#v", events, file)
	}
	if file.Steps[1].Text != "{{vars.recorded_text_1}}" || file.Vars["recorded_text_1"] != "" {
		t.Fatalf("typed text placeholder not generated: %#v", file)
	}
	if file.Steps[2].Label != "{{vars.recorded_select_1}}" || file.Vars["recorded_select_1"] != "" {
		t.Fatalf("selected option placeholder not generated: %#v", file)
	}
	if strings.Contains(file.Steps[1].Text, "typed-secret") {
		t.Fatalf("typed text leaked: %#v", file.Steps[1])
	}
	if strings.Contains(file.Steps[2].Label, "admin") {
		t.Fatalf("selected option leaked: %#v", file.Steps[2])
	}
}

func TestDiffPNGsWritesDiff(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.png")
	actual := filepath.Join(dir, "actual.png")
	diff := filepath.Join(dir, "diff.png")
	writeTinyPNG(t, base, color.RGBA{R: 1, A: 255})
	writeTinyPNG(t, actual, color.RGBA{R: 2, A: 255})
	ratio, pixels, err := diffPNGs(base, actual, diff)
	if err != nil {
		t.Fatalf("diffPNGs failed: %v", err)
	}
	if ratio != 1 || pixels != 1 {
		t.Fatalf("unexpected diff result ratio=%v pixels=%d", ratio, pixels)
	}
	if _, err := os.Stat(diff); err != nil {
		t.Fatalf("diff not written: %v", err)
	}
}

func TestLoadExtractSchemaAndFormValues(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.yaml")
	if err := os.WriteFile(schema, []byte("fields:\n  title:\n    selector: h1\n    attr: text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadExtractSchemaFile(schema)
	if err != nil {
		t.Fatalf("LoadExtractSchemaFile failed: %v", err)
	}
	if loaded.Fields["title"].Selector != "h1" {
		t.Fatalf("unexpected schema: %#v", loaded)
	}
	values := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(values, []byte("fields:\n  email: user@example.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	form, err := loadFormFillFile(values)
	if err != nil {
		t.Fatalf("loadFormFillFile failed: %v", err)
	}
	if form.Fields["email"] == nil {
		t.Fatalf("unexpected form values: %#v", form)
	}
}

func TestWorkflowLocatorsAllowSelectorFreeActions(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
steps:
  - action: page.click
    locators:
      - role: button
        name: Save
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	if len(def.Steps) != 1 || len(def.Steps[0].Locators) != 1 {
		t.Fatalf("locators not parsed: %#v", def)
	}
	result, err := RunWorkflow(contextWithoutBrowser(t), nil, WorkflowRunOptions{Definition: def, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if result.Steps[0].Plan.LocatorCount != 1 {
		t.Fatalf("locator count missing from plan: %#v", result.Steps[0].Plan)
	}
}

func TestTableAndListCSVExports(t *testing.T) {
	tableBytes, err := tableCSV(TableResult{Tables: []PageTable{{
		Index: 1,
		Rows:  []TableRow{{Index: 2, Cells: []TableCell{{Index: 3, Text: "Ada"}}}},
	}}})
	if err != nil {
		t.Fatalf("tableCSV failed: %v", err)
	}
	if !strings.Contains(string(tableBytes), "table_index,row_index,cell_index") || !strings.Contains(string(tableBytes), "Ada") {
		t.Fatalf("unexpected table csv: %s", tableBytes)
	}
	listBytes, err := listCSV(PageListResult{Lists: []PageList{{
		Index: 1,
		Items: []PageListItem{{Index: 2, Level: 3, Text: "Item", Href: "https://example.test"}},
	}}})
	if err != nil {
		t.Fatalf("listCSV failed: %v", err)
	}
	if !strings.Contains(string(listBytes), "list_index,item_index,level") || !strings.Contains(string(listBytes), "Item") {
		t.Fatalf("unexpected list csv: %s", listBytes)
	}
}

func TestPageDiffEnvelopeData(t *testing.T) {
	dir := t.TempDir()
	before := filepath.Join(dir, "before.json")
	after := filepath.Join(dir, "after.json")
	writeJSON(t, before, map[string]any{"ok": true, "data": map[string]any{"title": "Before", "url": "https://example.test/?token=secret"}})
	writeJSON(t, after, map[string]any{"ok": true, "data": map[string]any{"title": "After", "url": "https://example.test/?token=secret"}})
	result, err := PageDiff(PageDiffOptions{BeforeFile: before, AfterFile: after, Limit: 10})
	if err != nil {
		t.Fatalf("PageDiff failed: %v", err)
	}
	if result.ChangeCount == 0 || result.Changes[0].Path == "" {
		t.Fatalf("diff missing changes: %#v", result)
	}
	for _, change := range result.Changes {
		if strings.Contains(change.BeforePreview, "secret") || strings.Contains(change.AfterPreview, "secret") {
			t.Fatalf("diff leaked secret: %#v", change)
		}
	}
}

func writeTinyPNG(t *testing.T, path string, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func contextWithoutBrowser(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
