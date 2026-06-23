package tests

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
)

type visualRegistry struct {
	Version   int                    `json:"version"`
	Expected  visualRegistryExpected `json:"expected"`
	Templates []visualRegistryEntry  `json:"templates"`
}

type visualRegistryExpected struct {
	CanonicalCount int            `json:"canonical_count"`
	Categories     map[string]int `json:"categories"`
}

type visualRegistryEntry struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Category        string   `json:"category"`
	Path            string   `json:"path"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	InputSchema     string   `json:"input_schema"`
	InputSchemaKind string   `json:"input_schema_kind"`
	Renderer        string   `json:"renderer"`
	LayoutPreset    string   `json:"layout_preset"`
	Tags            []string `json:"tags"`
	Aliases         []string `json:"aliases"`
}

var mermaidTemplates = []string{
	"mermaid.architecture",
	"mermaid.block",
	"mermaid.c4",
	"mermaid.class",
	"mermaid.er",
	"mermaid.event_modeling",
	"mermaid.flowchart",
	"mermaid.gantt",
	"mermaid.gitgraph",
	"mermaid.ishikawa",
	"mermaid.journey",
	"mermaid.kanban",
	"mermaid.mindmap",
	"mermaid.packet",
	"mermaid.pie",
	"mermaid.quadrant",
	"mermaid.radar",
	"mermaid.requirement",
	"mermaid.sankey",
	"mermaid.sequence",
	"mermaid.state",
	"mermaid.timeline",
	"mermaid.treemap",
	"mermaid.treeview",
	"mermaid.venn",
	"mermaid.wardley",
	"mermaid.xy",
	"mermaid.zenuml",
}

func TestVisualCommandsJSONContract(t *testing.T) {
	obj := runVisualOK(t, "commands", "--json")
	names := map[string]bool{}
	for _, item := range objectMap(t, obj["data"])["commands"].([]any) {
		m := objectMap(t, item)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"render", "inspect-input", "inspect-plan", "inspect-render", "inspect-browser", "validate", "template.categories", "template.list", "template.get", "template.schema", "template.guide", "template.doctor", "inspect-output", "schema", "help.llm", "version"} {
		if !names[name] {
			t.Fatalf("missing visual command %s in %#v", name, names)
		}
	}
}

func TestVisualMermaidCatalogOnly(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	if registry.Version != 4 {
		t.Fatalf("expected registry version 4, got %d", registry.Version)
	}
	if registry.Expected.CanonicalCount != 28 || registry.Expected.Categories["mermaid"] != 28 {
		t.Fatalf("unexpected registry expected metadata: %#v", registry.Expected)
	}
	if len(registry.Templates) != 28 {
		t.Fatalf("expected 28 Mermaid templates, got %d", len(registry.Templates))
	}
	expected := map[string]bool{}
	for _, id := range mermaidTemplates {
		expected[id] = true
	}
	seen := map[string]bool{}
	for _, entry := range registry.Templates {
		if !expected[entry.ID] {
			t.Fatalf("unexpected public visual template %s", entry.ID)
		}
		if seen[entry.ID] {
			t.Fatalf("duplicate template %s", entry.ID)
		}
		seen[entry.ID] = true
		if entry.Category != "mermaid" || !strings.HasPrefix(entry.ID, "mermaid.") {
			t.Fatalf("public template must be mermaid category/id: %#v", entry)
		}
		assertMermaidTemplateFiles(t, templateDir, entry)
		runVisualOK(t, "validate", "--template", entry.ID, "--template-dir", templateDir, "--input", filepath.Join(templateDir, entry.ID, "examples", "basic.mmd"), "--json")
	}
	if len(seen) != len(expected) {
		t.Fatalf("expected Mermaid templates missing: seen=%#v", seen)
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range entries {
		if !item.IsDir() {
			continue
		}
		name := item.Name()
		if name == "_shared" {
			continue
		}
		if !strings.HasPrefix(name, "mermaid.") {
			t.Fatalf("non-Mermaid visual template directory should not remain: %s", name)
		}
	}
}

func TestVisualPublicMermaidTemplatesDoNotExposeCustomJSONInput(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	for _, entry := range loadRegistry(t, templateDir).Templates {
		t.Run(entry.ID, func(t *testing.T) {
			base := filepath.Join(templateDir, entry.ID)
			exampleDir := filepath.Join(base, "examples")
			err := filepath.WalkDir(exampleDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if strings.HasSuffix(path, ".input.json") || strings.HasSuffix(path, ".json") {
					t.Fatalf("%s public examples must use Mermaid .mmd input, found %s", entry.ID, path)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			guide := mustRead(t, filepath.Join(base, "agent-guide.md"))
			if strings.Contains(guide, ".input.json") {
				t.Fatalf("%s guide still mentions non-Mermaid input", entry.ID)
			}
			schema := loadJSONMap(t, filepath.Join(base, "schema.input.json"))
			if schema["input_format"] != "mermaid" || schema["mermaid_syntax"] == "" {
				t.Fatalf("%s schema must expose Mermaid input contract: %#v", entry.ID, schema)
			}
		})
	}
	jsonInput := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(jsonInput, []byte(`{"nodes":[{"id":"a"}],"edges":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fail := runVisual(t, "validate", "--template", "mermaid.flowchart", "--template-dir", templateDir, "--input", jsonInput, "--json")
	assertErrorCode(t, fail, "mermaid_input_required")
}

func TestVisualTemplateDiscoveryAndDoctor(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	categories := runVisualOK(t, "template", "categories", "--template-dir", templateDir, "--json")
	catData := objectMap(t, categories["data"])
	if catData["canonical_count"].(float64) != 28 || catData["total_count"].(float64) < 28 || catData["alias_count"].(float64) < 1 {
		t.Fatalf("unexpected category counts: %#v", catData)
	}
	categoryItems := catData["categories"].([]any)
	if len(categoryItems) != 1 || objectMap(t, categoryItems[0])["id"] != "mermaid" || objectMap(t, categoryItems[0])["count"].(float64) != 28 {
		t.Fatalf("expected only mermaid category: %#v", categoryItems)
	}

	list := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--json")
	listData := objectMap(t, list["data"])
	if listData["canonical_count"].(float64) != 28 || listData["matched_count"].(float64) != 28 {
		t.Fatalf("unexpected template list data: %#v", listData)
	}
	schema := runVisualOK(t, "template", "schema", "mermaid.sequence", "--template-dir", templateDir, "--json")
	schemaData := objectMap(t, schema["data"])
	if schemaData["input_format"] != "mermaid" || schemaData["example_file"] != "mermaid.sequence/examples/basic.mmd" || strings.TrimSpace(schemaData["mermaid_example"].(string)) == "" {
		t.Fatalf("template schema must expose Mermaid example: %#v", schemaData)
	}
	guide := runVisualOK(t, "template", "guide", "mermaid.sequence", "--template-dir", templateDir, "--json")
	if !strings.Contains(objectMap(t, guide["data"])["raw_markdown"].(string), "Mermaid") {
		t.Fatalf("Mermaid guide should describe Mermaid input: %#v", guide)
	}

	doctor := runVisualOK(t, "template", "doctor", "--template-dir", templateDir, "--json")
	doctorData := objectMap(t, doctor["data"])
	for key, want := range map[string]float64{
		"checked_templates":            28,
		"checked_examples":             28,
		"rendered_examples":            28,
		"canonical_templates":          28,
		"expected_canonical_templates": 28,
		"canonical_template_dirs":      28,
	} {
		if doctorData[key].(float64) != want {
			t.Fatalf("doctor expected %s=%.0f, got %#v", key, want, doctorData)
		}
	}
	if doctorData["offline"] != true || len(doctorData["orphan_template_dirs"].([]any)) != 0 {
		t.Fatalf("doctor should pass offline with no orphan dirs: %#v", doctorData)
	}
}

func TestVisualMermaidInferenceAndRender(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	flow := filepath.Join(t.TempDir(), "flow.mmd")
	if err := os.WriteFile(flow, []byte("---\ntitle: Runtime Flow\n---\nflowchart LR\n  Browser[Browser] -->|API| Gateway[API Gateway]\n  Gateway --> Service[Order Service]\n  Service --> Database[(Order DB)]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inspect := runVisualOK(t, "inspect-input", "--template-dir", templateDir, "--input", flow, "--json")
	inspectData := objectMap(t, inspect["data"])
	if inspectData["template_id"] != "mermaid.flowchart" {
		t.Fatalf("flowchart should infer mermaid.flowchart: %#v", inspectData)
	}
	out := filepath.Join(t.TempDir(), "flowchart")
	rendered := runVisualOK(t, "render", "--template-dir", templateDir, "--input", flow, "--out", out, "--json")
	artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
	assertArtifactContract(t, artifact, "mermaid.flowchart", "Runtime Flow")
	renderInspect := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", out, "--json")
	renderData := objectMap(t, renderInspect["data"])
	renderChecks := objectMap(t, renderData["checks"])
	for _, field := range []string{"output_files", "offline_scan", "runtime_assets", "renderer_contract_match"} {
		if renderChecks[field] != true {
			t.Fatalf("rendered Mermaid flowchart failed artifact check %s: %#v", field, renderChecks)
		}
	}

	for _, templateID := range []string{"mermaid.sequence", "mermaid.timeline", "mermaid.sankey", "mermaid.mindmap", "mermaid.pie", "mermaid.wardley"} {
		t.Run(templateID, func(t *testing.T) {
			input := filepath.Join(templateDir, templateID, "examples", "basic.mmd")
			out := filepath.Join(t.TempDir(), strings.ReplaceAll(templateID, ".", "-"))
			rendered := runVisualOK(t, "render", "--template", templateID, "--template-dir", templateDir, "--input", input, "--out", out, "--json")
			artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
			assertArtifactContract(t, artifact, templateID, "")
		})
	}
}

func TestVisualMermaidArchitectureBrowserContract(t *testing.T) {
	if os.Getenv("EFP_SKIP_BROWSER_SMOKE") == "1" {
		t.Skip("browser smoke disabled by EFP_SKIP_BROWSER_SMOKE")
	}
	browserPath := findTestBrowser()
	if browserPath == "" {
		t.Skip("Chrome/Chromium not available for browser smoke")
	}
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	out := filepath.Join(t.TempDir(), "mermaid-architecture")
	input := filepath.Join(templateDir, "mermaid.architecture", "examples", "basic.mmd")
	runVisualOK(t, "render", "--template", "mermaid.architecture", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	screenshot := filepath.Join(out, "screenshot.png")
	inspected := runVisualOK(t, "inspect-browser", "--template-dir", templateDir, "--out", out, "--screenshot", screenshot, "--browser", browserPath, "--timeout", "90", "--json")
	data := objectMap(t, inspected["data"])
	if data["ready"] != true || data["browser_ready"] != true || data["render_ready"] != true {
		t.Fatalf("Mermaid architecture browser inspect should be ready: %#v", data)
	}
	summary := objectMap(t, data["visual_summary"])
	for field, min := range map[string]float64{
		"entity_label_count":                  4,
		"label_icon_loaded_count":             1,
		"ground_route_rail_segment_count":     3,
		"ground_route_rail_arrowhead_count":   3,
		"ground_route_rail_visible_count":     3,
		"explicit_route_link_count":           3,
		"explicit_link_label_count":           3,
		"relation_depth_test_enabled_count":   1,
		"primary_route_count":                 2,
		"secondary_route_count":               1,
		"entity_component_count":              4,
		"relation_component_count":            3,
		"html_label_component_count":          7,
		"leader_line_component_count":         4,
		"relation_components_own_path_count":  3,
		"relation_components_own_arrow_count": 3,
		"relation_components_own_hit_count":   3,
		"relation_components_own_label_count": 3,
		"entity_components_with_ports_count":  4,
		"path_arrow_cap_count":                3,
		"path_hit_area_count":                 3,
		"entity_body_registry_count":          12,
		"entity_known_body_count":             4,
		"route_plan_route_count":              3,
		"route_plan_lane_count":               5,
	} {
		if summary[field].(float64) < min {
			t.Fatalf("browser visual summary expected %s >= %.0f, got %#v", field, min, summary)
		}
	}
	if summary["broken_label_icon_count"].(float64) != 0 ||
		summary["scene_component_tree_present"] != true ||
		summary["ground_path_builder_present"] != true ||
		summary["ground_path_builder_version"] != "v6" ||
		summary["route_plan_present"] != true ||
		summary["route_plan_rendered_match"] != true ||
		summary["path_join_style"] != "bevel" ||
		summary["path_hover_halo_supported"] != true ||
		summary["camera_fit_includes_labels"] != true ||
		summary["camera_fit_reserved_inspector_margin"] != true ||
		summary["screen_svg_relation_layer_visible"] != false ||
		summary["generic_link_label_count"].(float64) != 0 ||
		summary["isolated_arrowhead_count"].(float64) != 0 ||
		summary["relation_render_mode"] != "ground_decal" ||
		summary["relation_depth_test_disabled_count"].(float64) != 0 ||
		summary["route_entity_intersection_count"].(float64) != 0 ||
		summary["route_port_hint_violation_count"].(float64) != 0 ||
		summary["route_direction_violation_count"].(float64) != 0 ||
		summary["relation_layer_mode"] != "world_ground" ||
		summary["link_label_mode"] != "html_billboard" ||
		summary["inspector_raw_json_default"] != false {
		t.Fatalf("browser visual summary failed quality checks: %#v", summary)
	}
	if _, err := os.Stat(screenshot); err != nil {
		t.Fatalf("inspect-browser did not write screenshot: %v", err)
	}
}

func TestVisualRenderOverwriteAndDryRunContracts(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	input := filepath.Join(templateDir, "mermaid.sequence", "examples", "basic.mmd")
	out := filepath.Join(t.TempDir(), "render")
	runVisualOK(t, "render", "--template", "mermaid.sequence", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	fail := runVisual(t, "render", "--template", "mermaid.sequence", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	assertErrorCode(t, fail, "output_exists")
	runVisualOK(t, "render", "--template", "mermaid.sequence", "--template-dir", templateDir, "--input", input, "--out", out, "--overwrite", "--json")
	dryOut := filepath.Join(t.TempDir(), "dry-run")
	dry := runVisualOK(t, "render", "--template", "mermaid.sequence", "--template-dir", templateDir, "--input", input, "--out", dryOut, "--dry-run", "--json")
	data := objectMap(t, dry["data"])
	if data["dry_run"] != true || len(data["planned_files"].([]any)) == 0 {
		t.Fatalf("dry-run contract is incomplete: %#v", data)
	}
	if _, err := os.Stat(dryOut); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output directory: %s", dryOut)
	}
}

func TestVisualOfflineSourceGuards(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{filepath.Join(root, "internal", "visual"), filepath.Join(root, "cmd", "visual")} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			text := mustRead(t, path)
			if strings.Contains(text, "go:embed") {
				t.Fatalf("%s contains go:embed", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	err := filepath.WalkDir(filepath.Join(root, "templates", "visual"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.Contains(filepath.ToSlash(path), "templates/visual/_shared/vendor/three/") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".js" && ext != ".css" && ext != ".html" && ext != ".yaml" && ext != ".json" && ext != ".mmd" && ext != ".md" {
			return nil
		}
		text := mustRead(t, path)
		for _, token := range []string{"http://", "https://", "//cdn", "unpkg", "cdnjs", "jsdelivr", "fonts.googleapis.com", "fetch(", "XMLHttpRequest", "WebSocket", "EventSource", "navigator.sendBeacon", `src="/`, `href="/`} {
			if strings.Contains(text, token) {
				t.Fatalf("%s contains forbidden offline token %q", path, token)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVisualSmokeScriptsUseMermaidTemplates(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"scripts/smoke.sh", "scripts/smoke.bat"} {
		text := mustRead(t, filepath.Join(root, filepath.FromSlash(rel)))
		for _, want := range []string{"mermaid.sequence", "mermaid.flowchart", "mermaid.architecture", "basic.mmd", "template doctor", "template schema", "template guide", "inspect-plan", "inspect-render"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %s", rel, want)
			}
		}
		if strings.Contains(text, ".input.json") {
			t.Fatalf("%s still references non-Mermaid example input", rel)
		}
	}
}

func TestVisualWindowsBatchScriptsContract(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"scripts/smoke.bat", "scripts/build.bat"} {
		text := mustRead(t, filepath.Join(root, filepath.FromSlash(rel)))
		if lineCount(text) <= 20 {
			t.Fatalf("%s appears collapsed: only %d lines", rel, lineCount(text))
		}
		if len(splitLines(text)) <= 1 {
			t.Fatalf("%s appears to be a single-line batch script", rel)
		}
		if !strings.Contains(text, "./cmd/visual") {
			t.Fatalf("%s missing visual command coverage", rel)
		}
	}

	smoke := mustRead(t, filepath.Join(root, "scripts", "smoke.bat"))
	for _, token := range []string{"go run ./cmd/visual", "template doctor", "render --template", "mermaid.sequence", "basic.mmd"} {
		if !strings.Contains(smoke, token) {
			t.Fatalf("scripts/smoke.bat missing visual smoke token %q", token)
		}
	}

	build := mustRead(t, filepath.Join(root, "scripts", "build.bat"))
	for _, token := range []string{"--snapshot", "--os", "--arch", "TARGET_OS", "TARGET_ARCH", "go build", "./cmd/visual"} {
		if !strings.Contains(build, token) {
			t.Fatalf("scripts/build.bat missing build token %q", token)
		}
	}
}

func TestVisualWorkflowUsesWindowsBatchSmoke(t *testing.T) {
	root := repoRoot(t)
	text := mustRead(t, filepath.Join(root, ".github", "workflows", "test.yml"))
	if lineCount(text) <= 20 {
		t.Fatalf(".github/workflows/test.yml appears collapsed: only %d lines", lineCount(text))
	}
	for _, token := range []string{"windows-latest", "scripts\\smoke.bat", "go build ./cmd/visual", "shell: cmd"} {
		if !strings.Contains(text, token) {
			t.Fatalf(".github/workflows/test.yml missing expected workflow token %q", token)
		}
	}
}

func TestVisualRegistrySortedByID(t *testing.T) {
	root := repoRoot(t)
	registry := loadRegistry(t, filepath.Join(root, "templates", "visual"))
	for i := 1; i < len(registry.Templates); i++ {
		prev := registry.Templates[i-1]
		cur := registry.Templates[i]
		if prev.ID > cur.ID {
			t.Fatalf("registry ids should be sorted: %s before %s", prev.ID, cur.ID)
		}
	}
}

func assertMermaidTemplateFiles(t *testing.T, templateDir string, entry visualRegistryEntry) {
	t.Helper()
	base := filepath.Join(templateDir, entry.ID)
	for _, rel := range []string{"template.yaml", "schema.input.json", "style.css", "agent-guide.md", "quality.rules.json", "examples/basic.mmd"} {
		info, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil || info.IsDir() || info.Size() == 0 {
			t.Fatalf("%s missing non-empty %s", entry.ID, rel)
		}
	}
	if filepath.ToSlash(entry.Path) != entry.ID+"/template.yaml" {
		t.Fatalf("%s registry path should point to its Mermaid template directory: %s", entry.ID, entry.Path)
	}
}

func assertArtifactContract(t *testing.T, artifact map[string]any, templateID, title string) {
	t.Helper()
	if templateID != "" && artifact["template_id"] != templateID {
		t.Fatalf("artifact template_id mismatch: %#v", artifact)
	}
	if title != "" && artifact["title"] != title {
		t.Fatalf("artifact title mismatch: %#v", artifact)
	}
	for _, field := range []string{"template_version", "out_dir", "out", "entrypoint", "relative_entrypoint", "files"} {
		if artifact[field] == nil || artifact[field] == "" {
			t.Fatalf("artifact missing %s: %#v", field, artifact)
		}
	}
	if artifact["relative_entrypoint"] != "index.html" || artifact["offline"] != true || artifact["file_url_safe"] != true || artifact["http_subpath_safe"] != true {
		t.Fatalf("artifact compatibility fields invalid: %#v", artifact)
	}
	files := stringSetFromAny(artifact["files"].([]any))
	for _, file := range []string{"index.html", "manifest.json", "manifest.js", "data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js", "assets/runtime/efp-visual-runtime.css"} {
		if !files[file] {
			t.Fatalf("artifact missing file %s in %#v", file, files)
		}
	}
}

func runVisualOK(t *testing.T, args ...string) map[string]any {
	t.Helper()
	out := runVisual(t, args...)
	return testutil.AssertOKEnvelope(t, out)
}

func runVisual(t *testing.T, args ...string) []byte {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/visual"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot(t)
	out, _ := cmd.CombinedOutput()
	return out
}

func assertErrorCode(t *testing.T, out []byte, code string) map[string]any {
	t.Helper()
	return testutil.AssertErrorCode(t, out, code)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root")
		}
		dir = parent
	}
}

func loadRegistry(t *testing.T, templateDir string) visualRegistry {
	t.Helper()
	var registry visualRegistry
	decodeJSONFile(t, filepath.Join(templateDir, "registry.json"), &registry)
	return registry
}

func loadJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	var out map[string]any
	decodeJSONFile(t, path, &out)
	return out
}

func decodeJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s is invalid JSON: %v", path, err)
	}
}

func objectMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %#v", value)
	}
	return m
}

func lineCount(content string) int {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func stringSetFromAny(items []any) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		if s, ok := item.(string); ok {
			out[filepath.ToSlash(s)] = true
		}
	}
	return out
}

func findTestBrowser() string {
	if env := strings.TrimSpace(os.Getenv("EFP_BROWSER")); env != "" {
		if info, err := os.Stat(env); err == nil && !info.IsDir() {
			return env
		}
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "chrome", "microsoft-edge", "msedge"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	for _, path := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
	} {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
