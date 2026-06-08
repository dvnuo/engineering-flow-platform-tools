package automation

import (
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
	}, "default", time.Unix(0, 0).UTC())
	if len(events) != 2 || len(file.Steps) != 2 {
		t.Fatalf("unexpected record file: %#v %#v", events, file)
	}
	if file.Steps[1].Text != "{{vars.recorded_text_1}}" || file.Vars["recorded_text_1"] != "" {
		t.Fatalf("typed text placeholder not generated: %#v", file)
	}
	if strings.Contains(file.Steps[1].Text, "typed-secret") {
		t.Fatalf("typed text leaked: %#v", file.Steps[1])
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
