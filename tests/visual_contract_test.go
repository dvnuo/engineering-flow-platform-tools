package tests

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
	vcmd "engineering-flow-platform-tools/internal/visual/commands"
)

type visualRegistry struct {
	Templates []struct {
		ID string `json:"id"`
	} `json:"templates"`
}

func TestVisualVersionJSONContract(t *testing.T) {
	obj := runVisualOK(t, "version", "--json")
	data := obj["data"].(map[string]any)
	for _, k := range []string{"version", "commit", "date"} {
		if strings.TrimSpace(data[k].(string)) == "" {
			t.Fatalf("missing %s in %v", k, data)
		}
	}
}

func TestVisualCommandsJSONContract(t *testing.T) {
	obj := runVisualOK(t, "commands", "--json")
	commands := obj["data"].(map[string]any)["commands"].([]any)
	names := map[string]bool{}
	for _, item := range commands {
		m := item.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"render", "validate", "template.list", "template.get", "template.doctor", "inspect-output", "schema", "help.llm", "version"} {
		if !names[name] {
			t.Fatalf("missing visual command %s in %#v", name, names)
		}
	}
}

func TestVisualSchemaRenderJSONContract(t *testing.T) {
	obj := runVisualOK(t, "schema", "render", "--json")
	flags := obj["data"].(map[string]any)["flags"].([]any)
	names := map[string]bool{}
	for _, item := range flags {
		m := item.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"template", "template-dir", "input", "out", "title", "overwrite", "dry-run", "json"} {
		if !names[name] {
			t.Fatalf("missing render flag %s in %#v", name, names)
		}
	}
}

func TestVisualTemplateListGetDoctor(t *testing.T) {
	templateDir := visualTemplateDir()
	list := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--json")
	templates := list["data"].(map[string]any)["templates"].([]any)
	if len(templates) != 20 {
		t.Fatalf("expected 20 templates, got %d", len(templates))
	}

	got := runVisualOK(t, "template", "get", "agent.run_trace", "--template-dir", templateDir, "--json")
	data := got["data"].(map[string]any)
	if data["id"] != "agent.run_trace" || data["version"] == "" || data["input_schema_kind"] != "graph_events_v1" {
		t.Fatalf("unexpected template get data: %#v", data)
	}
	renderer := data["renderer"].(map[string]any)
	if renderer["contract"] != "offline.graph.v1" {
		t.Fatalf("unexpected renderer: %#v", renderer)
	}

	doctor := runVisualOK(t, "template", "doctor", "--template-dir", templateDir, "--json")
	checked := doctor["data"].(map[string]any)["checked_templates"].(float64)
	if checked != 20 {
		t.Fatalf("expected 20 checked templates, got %v", checked)
	}
}

func TestVisualValidateEveryExample(t *testing.T) {
	templateDir := visualTemplateDir()
	for _, id := range visualTemplateIDs(t) {
		t.Run(id, func(t *testing.T) {
			runVisualOK(t, "validate", "--template", id, "--template-dir", templateDir, "--input", filepath.Join(templateDir, id, "examples", "basic.input.json"), "--json")
		})
	}
}

func TestVisualRenderEveryExample(t *testing.T) {
	templateDir := visualTemplateDir()
	for _, id := range visualTemplateIDs(t) {
		t.Run(id, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "artifact")
			runVisualOK(t, "render", "--template", id, "--template-dir", templateDir, "--input", filepath.Join(templateDir, id, "examples", "basic.input.json"), "--out", out, "--json")
			for _, rel := range []string{"index.html", "manifest.json", "manifest.js", "data.js", "assets/runtime/efp-visual-runtime.css", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js"} {
				if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
					t.Fatalf("%s missing: %v", rel, err)
				}
			}
			index := mustRead(t, filepath.Join(out, "index.html"))
			for _, token := range []string{"http://", "https://", "//cdn", "fetch(", "XMLHttpRequest", "WebSocket", "EventSource"} {
				if strings.Contains(index, token) {
					t.Fatalf("index.html contains forbidden token %s", token)
				}
			}
			assertRelativeHTMLCSSJS(t, out)
		})
	}
}

func TestVisualOutputExistsOverwriteAndDryRun(t *testing.T) {
	templateDir := visualTemplateDir()
	input := filepath.Join(templateDir, "agent.run_trace", "examples", "basic.input.json")
	out := filepath.Join(t.TempDir(), "artifact")
	runVisualOK(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	fail := runVisual(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	assertErrorCode(t, fail, "output_exists")
	runVisualOK(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", out, "--overwrite", "--json")

	dryOut := filepath.Join(t.TempDir(), "dry-run-artifact")
	dry := runVisualOK(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", dryOut, "--dry-run", "--json")
	if _, err := os.Stat(dryOut); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output directory: %v", err)
	}
	if planned, _ := dry["data"].(map[string]any)["planned_files"].([]any); len(planned) == 0 {
		t.Fatalf("dry-run missing planned_files: %#v", dry)
	}
}

func TestVisualStableFailures(t *testing.T) {
	templateDir := visualTemplateDir()
	input := filepath.Join(templateDir, "agent.run_trace", "examples", "basic.input.json")
	out := filepath.Join(t.TempDir(), "artifact")
	assertErrorCode(t, runVisual(t, "render", "--template", "missing.template", "--template-dir", templateDir, "--input", input, "--out", out, "--json"), "template_not_found")

	invalid := filepath.Join(t.TempDir(), "invalid.input.json")
	if err := os.WriteFile(invalid, []byte(`{"schema":"efp.visual.input.graph.v1","nodes":[{"id":"a"}],"edges":[{"from":"a","to":"missing"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	assertErrorCode(t, runVisual(t, "validate", "--template", "runtime.topology", "--template-dir", templateDir, "--input", invalid, "--json"), "template_input_invalid")
}

func TestVisualPathTraversalAssetRejected(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(filepath.Join(templateDir, "bad", "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(templateDir, "registry.json"), `{"version":1,"templates":[{"id":"bad","version":"1.0.0","path":"bad/template.yaml","title":"Bad","description":"Bad","input_schema":"graph_v1","renderer":"offline.graph.v1"}]}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "schema.input.json"), `{}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "style.css"), `:root { --accent: #fff; }`)
	mustWrite(t, filepath.Join(templateDir, "bad", "examples", "basic.input.json"), `{"nodes":[{"id":"a"}]}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "template.yaml"), `id: bad
version: 1.0.0
title: Bad
description: Bad template
input_schema: graph_v1
input_schema_kind: graph_v1
renderer:
  contract: offline.graph.v1
layout:
  preset: dag
offline:
  required: true
  forbid_network: true
  data_mode: js-file
assets:
  - from: ../../go.mod
    to: assets/go.mod
styles:
  - assets/runtime/efp-visual-runtime.css
  - assets/template/style.css
scripts:
  - manifest.js
  - data.js
  - assets/runtime/efp-visual-runtime.iife.js
  - assets/runtime/efp-visual-renderers.iife.js
`)
	assertErrorCode(t, runVisual(t, "template", "doctor", "--template-dir", templateDir, "--json"), "template_asset_outside_root")
}

func TestVisualOfflineViolationRejected(t *testing.T) {
	templateDir := filepath.Join(t.TempDir(), "visual")
	copyTree(t, visualTemplateDir(), templateDir)
	mustWrite(t, filepath.Join(templateDir, "agent.run_trace", "style.css"), `@import "bad.css";`)
	out := filepath.Join(t.TempDir(), "artifact")
	input := filepath.Join(templateDir, "agent.run_trace", "examples", "basic.input.json")
	assertErrorCode(t, runVisual(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", out, "--json"), "offline_violation")
}

func TestVisualInspectOutputRejectsProtocolRelativeDataString(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
	}{
		{name: "domain", url: "//example.com/app.js"},
		{name: "host_path", url: "//cdn/app.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			mustWrite(t, filepath.Join(out, "index.html"), `<!doctype html><script src="data.js"></script>`)
			mustWrite(t, filepath.Join(out, "manifest.json"), `{}`)
			mustWrite(t, filepath.Join(out, "manifest.js"), `window.__EFP_VISUAL_MANIFEST__ = {};`)
			mustWrite(t, filepath.Join(out, "data.js"), `window.__EFP_VISUAL_DATA__ = {"u":"`+tc.url+`"};`)

			assertErrorCode(t, runVisual(t, "inspect-output", "--out", out, "--json"), "offline_violation")
		})
	}
}

func TestVisualInspectOutputAllowsFileURLText(t *testing.T) {
	out := t.TempDir()
	mustWrite(t, filepath.Join(out, "index.html"), `<!doctype html><script src="data.js"></script>`)
	mustWrite(t, filepath.Join(out, "manifest.json"), `{}`)
	mustWrite(t, filepath.Join(out, "manifest.js"), `window.__EFP_VISUAL_MANIFEST__ = {};`)
	mustWrite(t, filepath.Join(out, "data.js"), `window.__EFP_VISUAL_DATA__ = {"u":"file:///tmp/artifact/app.js"};`)

	runVisualOK(t, "inspect-output", "--out", out, "--json")
}

func TestVisualDataAndManifestJS(t *testing.T) {
	templateDir := visualTemplateDir()
	out := filepath.Join(t.TempDir(), "artifact")
	runVisualOK(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "agent.run_trace", "examples", "basic.input.json"), "--out", out, "--json")
	if !strings.Contains(mustRead(t, filepath.Join(out, "data.js")), "window.__EFP_VISUAL_DATA__") {
		t.Fatal("data.js missing window assignment")
	}
	if !strings.Contains(mustRead(t, filepath.Join(out, "manifest.js")), "window.__EFP_VISUAL_MANIFEST__") {
		t.Fatal("manifest.js missing window assignment")
	}
}

func TestVisualNoGoEmbed(t *testing.T) {
	for _, root := range []string{"../internal/visual", "../cmd/visual"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
				return err
			}
			if strings.Contains(mustRead(t, path), "//go:embed") {
				t.Fatalf("go embed directive found in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func runVisualOK(t *testing.T, args ...string) map[string]any {
	t.Helper()
	obj := runVisual(t, args...)
	if obj["ok"] != true {
		t.Fatalf("visual command failed: args=%v obj=%#v", args, obj)
	}
	return obj
}

func runVisual(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	cmd := vcmd.NewRoot()
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("visual command returned error for args %v: %v\n%s", args, err, b.String())
	}
	return testutil.AssertJSONEnvelope(t, b.Bytes())
}

func assertErrorCode(t *testing.T, obj map[string]any, code string) {
	t.Helper()
	if obj["ok"] != false {
		t.Fatalf("expected failure %s, got %#v", code, obj)
	}
	errObj := obj["error"].(map[string]any)
	if errObj["code"] != code {
		t.Fatalf("expected error code %s, got %#v", code, errObj)
	}
}

func visualTemplateDir() string {
	return filepath.Clean("../templates/visual")
}

func visualTemplateIDs(t *testing.T) []string {
	t.Helper()
	var registry visualRegistry
	b, err := os.ReadFile(filepath.Join(visualTemplateDir(), "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, item := range registry.Templates {
		ids = append(ids, item.ID)
	}
	return ids
}

func assertRelativeHTMLCSSJS(t *testing.T, out string) {
	t.Helper()
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".css", ".js":
		default:
			return nil
		}
		s := mustRead(t, path)
		for _, token := range []string{`src="/`, `href="/`} {
			if strings.Contains(s, token) {
				t.Fatalf("%s contains absolute asset token %s", path, token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
