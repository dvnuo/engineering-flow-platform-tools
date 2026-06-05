package tests

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
	"gopkg.in/yaml.v3"
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

type visualTemplateManifest struct {
	ID              string `yaml:"id"`
	Version         string `yaml:"version"`
	Category        string `yaml:"category"`
	Title           string `yaml:"title"`
	Description     string `yaml:"description"`
	InputSchema     string `yaml:"input_schema"`
	InputSchemaKind string `yaml:"input_schema_kind"`
	Renderer        struct {
		Contract string `yaml:"contract"`
	} `yaml:"renderer"`
	Layout struct {
		Preset string `yaml:"preset"`
	} `yaml:"layout"`
	Effects struct {
		Engine string `yaml:"engine"`
		Scene  string `yaml:"scene"`
	} `yaml:"effects"`
	VisualDesign struct {
		InitialView     string   `yaml:"initial_view"`
		AgentGuidance   []string `yaml:"agent_guidance"`
		Supports        []string `yaml:"supports"`
		MaxInitialNodes int      `yaml:"max_initial_nodes"`
		MaxInitialEdges int      `yaml:"max_initial_edges"`
	} `yaml:"visual_design"`
	Offline struct {
		Required      bool   `yaml:"required"`
		ForbidNetwork bool   `yaml:"forbid_network"`
		DataMode      string `yaml:"data_mode"`
	} `yaml:"offline"`
	Styles  []string `yaml:"styles"`
	Scripts []string `yaml:"scripts"`
	Tags    []string `yaml:"tags"`
}

var semanticCategoryCounts = map[string]int{
	"uml":          5,
	"relationship": 4,
	"temporal":     4,
	"flow":         4,
	"hierarchy":    4,
	"evidence":     4,
	"matrix":       4,
	"spatial":      4,
}

var semanticSchemaKinds = map[string]bool{
	"graph_v1":                    true,
	"graph_events_v1":             true,
	"timeline_v1":                 true,
	"evidence_v1":                 true,
	"matrix_v1":                   true,
	"uml_sequence_v1":             true,
	"uml_class_v1":                true,
	"uml_state_machine_v1":        true,
	"uml_activity_v1":             true,
	"uml_component_deployment_v1": true,
}

var semanticRenderers = map[string]bool{
	"offline.graph.v1":            true,
	"offline.timeline.v1":         true,
	"offline.evidence.v1":         true,
	"offline.matrix.v1":           true,
	"offline.uml.sequence.3d.v1":  true,
	"offline.uml.class.2_5d.v1":   true,
	"offline.uml.state.3d.v1":     true,
	"offline.uml.activity.3d.v1":  true,
	"offline.uml.component.3d.v1": true,
}

var genericDescriptionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Visualize .+ as a complete offline .+ view`),
	regexp.MustCompile(`^Offline .+ visualization for .+ workflows\.$`),
	regexp.MustCompile(`^.+ template for visual artifacts\.$`),
}

func TestVisualCommandsJSONContract(t *testing.T) {
	obj := runVisualOK(t, "commands", "--json")
	commands := objectMap(t, obj["data"])["commands"].([]any)
	names := map[string]bool{}
	for _, item := range commands {
		m := objectMap(t, item)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"render", "inspect-input", "inspect-plan", "inspect-render", "validate", "template.categories", "template.list", "template.get", "template.schema", "template.guide", "template.doctor", "inspect-output", "schema", "help.llm", "version"} {
		if !names[name] {
			t.Fatalf("missing visual command %s in %#v", name, names)
		}
	}
}

func TestVisualSemanticCatalogContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	if registry.Version != 3 {
		t.Fatalf("expected registry version 3, got %d", registry.Version)
	}
	if registry.Expected.CanonicalCount != 33 || len(registry.Templates) != 33 {
		t.Fatalf("expected 33 semantic templates, got expected=%#v len=%d", registry.Expected, len(registry.Templates))
	}
	for category, want := range semanticCategoryCounts {
		if got := registry.Expected.Categories[category]; got != want {
			t.Fatalf("expected category %s=%d, got %d", category, want, got)
		}
	}

	seen := map[string]bool{}
	counts := map[string]int{}
	for _, entry := range registry.Templates {
		if seen[entry.ID] {
			t.Fatalf("duplicate template id %s", entry.ID)
		}
		seen[entry.ID] = true
		counts[entry.Category]++
		if len(entry.Aliases) != 0 {
			t.Fatalf("new semantic catalog must not expose aliases: %#v", entry)
		}
		assertRegistryEntryQuality(t, entry)
		manifest := loadTemplateManifest(t, templateDir, entry)
		assertManifestMatchesRegistry(t, entry, manifest)
		assertTemplateFiles(t, templateDir, entry)
		runVisualOK(t, "validate", "--template", entry.ID, "--template-dir", templateDir, "--input", filepath.Join(templateDir, entry.ID, "examples", "basic.input.json"), "--json")
	}
	for category, want := range semanticCategoryCounts {
		if counts[category] != want {
			t.Fatalf("category %s expected %d templates, got %d", category, want, counts[category])
		}
	}
}

func TestVisualNoUnregisteredTemplateDirectories(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	canonical := map[string]bool{}
	for _, entry := range registry.Templates {
		canonical[filepath.Dir(filepath.ToSlash(entry.Path))] = true
	}
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_shared" {
			continue
		}
		if !canonical[entry.Name()] {
			t.Fatalf("unregistered visual template directory: %s", entry.Name())
		}
	}
}

func TestVisualTemplateDiscoveryCommands(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	categories := runVisualOK(t, "template", "categories", "--template-dir", templateDir, "--json")
	catData := objectMap(t, categories["data"])
	if catData["canonical_count"].(float64) != 33 || catData["total_count"].(float64) != 33 || catData["alias_count"].(float64) != 0 {
		t.Fatalf("unexpected categories counts: %#v", catData)
	}
	items := catData["categories"].([]any)
	gotCategories := map[string]float64{}
	for _, item := range items {
		obj := objectMap(t, item)
		gotCategories[obj["id"].(string)] = obj["count"].(float64)
	}
	for category, want := range semanticCategoryCounts {
		if gotCategories[category] != float64(want) {
			t.Fatalf("category %s expected %.0f, got %#v", category, float64(want), gotCategories)
		}
	}

	list := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--category", "uml", "--json")
	listData := objectMap(t, list["data"])
	if listData["matched_count"].(float64) != 5 || listData["alias_count"].(float64) != 0 {
		t.Fatalf("unexpected uml list data: %#v", listData)
	}
	filtered := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--schema-kind", "uml_sequence_v1", "--renderer", "offline.uml.sequence.3d.v1", "--json")
	if objectMap(t, filtered["data"])["matched_count"].(float64) != 1 {
		t.Fatalf("expected one UML sequence match: %#v", filtered)
	}
}

func TestVisualTemplateAgentGuideCommandAndMetadata(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	guide := runVisualOK(t, "template", "guide", "uml.sequence_3d", "--template-dir", templateDir, "--json")
	guideData := objectMap(t, guide["data"])
	if guideData["template_id"] != "uml.sequence_3d" || guideData["agent_guide_available"] != true {
		t.Fatalf("unexpected guide response: %#v", guideData)
	}
	if !strings.Contains(guideData["raw_markdown"].(string), "participant = semantic lifeline") {
		t.Fatalf("uml.sequence_3d guide missing sequence semantic rules")
	}
	sections := objectMap(t, guideData["guide"])
	for _, section := range []string{"when_to_use_this_template", "semantic_model", "required_construction_rules", "recommended_fields", "visual_encoding_rules", "common_mistakes_to_avoid", "quality_checklist_before_render", "minimal_good_example"} {
		if strings.TrimSpace(sections[section].(string)) == "" {
			t.Fatalf("guide section %s missing: %#v", section, sections)
		}
	}
	get := runVisualOK(t, "template", "get", "uml.sequence_3d", "--template-dir", templateDir, "--json")
	getData := objectMap(t, get["data"])
	if getData["agent_guide_available"] != true || getData["quality_rules_available"] != true {
		t.Fatalf("template get did not expose guide/rules metadata: %#v", getData)
	}
	schema := runVisualOK(t, "template", "schema", "uml.sequence_3d", "--template-dir", templateDir, "--json")
	schemaData := objectMap(t, schema["data"])
	if schemaData["agent_guide_available"] != true || len(schemaData["agent_guide_summary"].([]any)) == 0 {
		t.Fatalf("template schema did not expose guide summary: %#v", schemaData)
	}
	cmdSchema := runVisualOK(t, "schema", "template.guide", "--json")
	if objectMap(t, cmdSchema["data"])["command"] != "template.guide" {
		t.Fatalf("template.guide command schema missing: %#v", cmdSchema)
	}
}

func TestVisualAllTemplatesHaveAgentGuidesAndQualityRules(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	for _, entry := range registry.Templates {
		t.Run(entry.ID, func(t *testing.T) {
			guidePath := filepath.Join(templateDir, entry.ID, "agent-guide.md")
			guide := mustRead(t, guidePath)
			for _, want := range []string{"## When to use this template", "## Semantic model", "## Required construction rules", "## Recommended fields", "## Visual encoding rules", "## Common mistakes to avoid", "## Quality checklist before render", "## Minimal good example", "common-visual-quality.md"} {
				if !strings.Contains(guide, want) {
					t.Fatalf("%s guide missing %q", entry.ID, want)
				}
			}
			rules := loadJSONMap(t, filepath.Join(templateDir, entry.ID, "quality.rules.json"))
			if rules["schema"] != "efp.visual.template_quality_rules.v1" || rules["template_id"] != entry.ID {
				t.Fatalf("%s quality rules invalid: %#v", entry.ID, rules)
			}
			for _, example := range []string{"good-small.input.json", "good-medium.input.json", "bad-dense.input.json", "fixed-dense.input.json"} {
				if _, err := os.Stat(filepath.Join(templateDir, entry.ID, "examples", example)); err != nil {
					t.Fatalf("%s missing example %s: %v", entry.ID, example, err)
				}
			}
		})
	}
	for _, shared := range []string{"common-visual-quality.md", "agent-guide.schema.json", "quality-rules.schema.json"} {
		if _, err := os.Stat(filepath.Join(templateDir, "_shared", "agent-guidance", shared)); err != nil {
			t.Fatalf("missing shared guidance file %s: %v", shared, err)
		}
	}
}

func TestVisualInspectInputTemplateQualityRulesWarnings(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	badSeq := runVisualOK(t, "inspect-input", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "uml.sequence_3d", "examples", "bad-dense.input.json"), "--json")
	badSeqCodes := warningCodeSet(t, badSeq)
	for _, code := range []string{"participant_display_name_missing", "phase_color_missing", "message_label_priority_missing", "visual_guidance_missing", "message_phase_missing", "message_curve_missing"} {
		if !badSeqCodes[code] {
			t.Fatalf("bad sequence missing warning %s in %#v", code, badSeqCodes)
		}
	}
	goodSeq := runVisualOK(t, "inspect-input", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "uml.sequence_3d", "examples", "game-session-flow.input.json"), "--json")
	if len(warningCodes(t, goodSeq)) >= len(warningCodes(t, badSeq)) {
		t.Fatalf("good sequence should have fewer warnings than bad sequence")
	}
	for _, forbidden := range []string{"visual_guidance_unknown_refs", "message_phase_missing", "unknown_participant_ref", "duplicate_order"} {
		if warningCodeSet(t, goodSeq)[forbidden] {
			t.Fatalf("good sequence unexpectedly has warning %s", forbidden)
		}
	}

	badGraph := runVisualOK(t, "inspect-input", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "relationship.dependency_graph", "examples", "dependency-dense-bad.input.json"), "--json")
	badGraphCodes := warningCodeSet(t, badGraph)
	for _, code := range []string{"ungrouped_nodes_high", "dominant_edge_kind", "edge_visibility_missing", "node_importance_missing", "label_too_long"} {
		if !badGraphCodes[code] {
			t.Fatalf("bad graph missing warning %s in %#v", code, badGraphCodes)
		}
	}
	for _, w := range objectMap(t, badGraph["data"])["warnings"].([]any) {
		warning := objectMap(t, w)
		if strings.TrimSpace(warning["suggestion"].(string)) == "" {
			t.Fatalf("warning missing machine-readable suggestion: %#v", warning)
		}
	}
	fixedGraph := runVisualOK(t, "inspect-input", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "relationship.dependency_graph", "examples", "dependency-dense-fixed.input.json"), "--json")
	if len(warningCodes(t, fixedGraph)) >= len(warningCodes(t, badGraph)) {
		t.Fatalf("fixed graph should have fewer warnings than bad graph")
	}
}

func TestVisualInspectPlanContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")

	cmdSchema := runVisualOK(t, "schema", "inspect-plan", "--json")
	cmdData := objectMap(t, cmdSchema["data"])
	if cmdData["command"] != "inspect-plan" {
		t.Fatalf("inspect-plan command schema missing: %#v", cmdData)
	}
	required := stringSetFromAny(cmdData["required"].([]any))
	for _, name := range []string{"template", "input"} {
		if !required[name] {
			t.Fatalf("inspect-plan schema missing required %s: %#v", name, cmdData)
		}
	}
	flags := map[string]map[string]any{}
	for _, item := range cmdData["flags"].([]any) {
		flag := objectMap(t, item)
		flags[flag["name"].(string)] = flag
	}
	for _, name := range []string{"template", "input", "out"} {
		if flags[name] == nil {
			t.Fatalf("inspect-plan schema missing flag %s: %#v", name, flags)
		}
	}
	if flags["template"]["required"] != true || flags["input"]["required"] != true || flags["out"]["required"] == true {
		t.Fatalf("inspect-plan required flag metadata invalid: %#v", flags)
	}

	good := runVisualOK(t, "inspect-plan", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "uml.sequence_3d", "examples", "game-session-flow.input.json"), "--out", filepath.Join(t.TempDir(), "sequence"), "--json")
	data := objectMap(t, good["data"])
	if data["ready"] != true || data["quality_score"].(float64) < 90 {
		t.Fatalf("good inspect-plan should be ready with high quality: %#v", data)
	}
	plan := objectMap(t, data["visual_plan"])
	if plan["schema"] != "efp.visual.plan.v1" || plan["template_id"] != "uml.sequence_3d" {
		t.Fatalf("visual plan contract mismatch: %#v", plan)
	}
	ir := objectMap(t, plan["ir"])
	if ir["schema"] != "efp.visual.ir.v1" || ir["kind"] != "uml_sequence_v1" {
		t.Fatalf("visual IR contract mismatch: %#v", ir)
	}
	counts := objectMap(t, ir["counts"])
	if counts["objects"].(float64) < 15 || counts["relationships"].(float64) < 18 {
		t.Fatalf("visual IR too small for sequence example: %#v", counts)
	}
	view := objectMap(t, plan["view"])
	if len(view["initial_focus_ids"].([]any)) == 0 || view["max_initial_objects"].(float64) == 0 {
		t.Fatalf("visual plan view missing focus or budgets: %#v", view)
	}
	labels := objectMap(t, plan["labels"])
	if labels["mode"] == "" {
		t.Fatalf("visual plan labels missing mode: %#v", labels)
	}
	legend := objectMap(t, plan["legend"])
	if legend["show"] != true || len(legend["items"].([]any)) == 0 {
		t.Fatalf("visual plan legend missing items: %#v", legend)
	}
	render := objectMap(t, plan["render"])
	command := render["command"].([]any)
	if len(command) < 9 || command[0] != "visual" || command[1] != "render" || render["offline"] != true {
		t.Fatalf("visual plan render hints invalid: %#v", render)
	}
	actions := plan["agent_next_actions"].([]any)
	if len(actions) == 0 {
		t.Fatalf("visual plan missing agent next actions: %#v", plan)
	}

	bad := runVisualOK(t, "inspect-plan", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "relationship.dependency_graph", "examples", "dependency-dense-bad.input.json"), "--json")
	badData := objectMap(t, bad["data"])
	if badData["ready"] != false || badData["quality_score"].(float64) >= 70 {
		t.Fatalf("bad inspect-plan should not be ready: %#v", badData)
	}
	qualityLoop := objectMap(t, badData["visual_plan"])["quality_loop"].([]any)
	if len(qualityLoop) == 0 {
		t.Fatalf("bad inspect-plan missing quality loop: %#v", badData)
	}
	codes := map[string]bool{}
	for _, item := range qualityLoop {
		code := objectMap(t, item)["code"].(string)
		codes[code] = true
	}
	for _, code := range []string{"ungrouped_nodes_high", "edge_visibility_missing", "dominant_edge_kind", "visual_guidance_missing"} {
		if !codes[code] {
			t.Fatalf("bad inspect-plan missing quality loop code %s in %#v", code, codes)
		}
	}
}

func TestVisualInspectRenderContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")

	cmdSchema := runVisualOK(t, "schema", "inspect-render", "--json")
	cmdData := objectMap(t, cmdSchema["data"])
	if cmdData["command"] != "inspect-render" {
		t.Fatalf("inspect-render command schema missing: %#v", cmdData)
	}
	required := stringSetFromAny(cmdData["required"].([]any))
	if !required["out"] {
		t.Fatalf("inspect-render schema missing required out: %#v", cmdData)
	}
	flags := map[string]map[string]any{}
	for _, item := range cmdData["flags"].([]any) {
		flag := objectMap(t, item)
		flags[flag["name"].(string)] = flag
	}
	if flags["screenshot"] == nil || flags["screenshot"]["required"] == true {
		t.Fatalf("inspect-render schema missing optional screenshot flag: %#v", flags)
	}

	goodOut := filepath.Join(t.TempDir(), "sequence")
	runVisualOK(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "uml.sequence_3d", "examples", "game-session-flow.input.json"), "--out", goodOut, "--json")
	good := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", goodOut, "--json")
	goodData := objectMap(t, good["data"])
	if goodData["ready"] != true || goodData["render_score"].(float64) < 90 {
		t.Fatalf("good inspect-render should be ready with high score: %#v", goodData)
	}
	checks := objectMap(t, goodData["checks"])
	for _, field := range []string{"output_files", "offline_scan", "runtime_assets", "three_asset", "renderer_contract_match", "template_version_match", "plan_ready", "focus_declared", "first_view_objects_within_budget", "first_view_relationships_within_budget", "labels_bounded", "relationships_visible"} {
		if checks[field] != true {
			t.Fatalf("inspect-render good output check %s failed: %#v", field, checks)
		}
	}
	plan := objectMap(t, goodData["visual_plan"])
	if plan["schema"] != "efp.visual.plan.v1" || objectMap(t, plan["ir"])["schema"] != "efp.visual.ir.v1" {
		t.Fatalf("inspect-render missing visual plan contract: %#v", plan)
	}

	blankScreenshot := filepath.Join(t.TempDir(), "blank.png")
	writeSolidPNG(t, blankScreenshot, 320, 180, color.RGBA{R: 8, G: 8, B: 8, A: 255})
	blank := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", goodOut, "--screenshot", blankScreenshot, "--json")
	blankData := objectMap(t, blank["data"])
	if blankData["ready"] != false || objectMap(t, blankData["checks"])["screenshot_non_blank"] != false {
		t.Fatalf("blank screenshot should make inspect-render not ready: %#v", blankData)
	}
	if !warningCodeSet(t, blank)["screenshot_blank"] {
		t.Fatalf("blank screenshot did not report screenshot_blank: %#v", blankData["warnings"])
	}

	badOut := filepath.Join(t.TempDir(), "bad-graph")
	runVisualOK(t, "render", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", filepath.Join(templateDir, "relationship.dependency_graph", "examples", "dependency-dense-bad.input.json"), "--out", badOut, "--json")
	bad := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", badOut, "--json")
	badData := objectMap(t, bad["data"])
	if badData["ready"] != false || badData["render_score"].(float64) >= 70 {
		t.Fatalf("bad inspect-render should not be ready: %#v", badData)
	}
	badChecks := objectMap(t, badData["checks"])
	if badChecks["plan_ready"] != false || badChecks["labels_bounded"] != false {
		t.Fatalf("bad inspect-render checks should report plan and label problems: %#v", badChecks)
	}
	codes := warningCodeSet(t, bad)
	for _, code := range []string{"visual_guidance_missing", "dominant_edge_kind", "render_plan_not_ready", "render_label_pressure_high"} {
		if !codes[code] {
			t.Fatalf("bad inspect-render missing warning %s in %#v", code, codes)
		}
	}
}

func TestVisualUMLTemplateSchemaInspectAndRender(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	input := filepath.Join(templateDir, "uml.sequence_3d", "examples", "basic.input.json")
	schema := runVisualOK(t, "template", "schema", "uml.sequence_3d", "--template-dir", templateDir, "--json")
	data := objectMap(t, schema["data"])
	if data["schema_file"] != "uml.sequence_3d/schema.input.json" || data["example_file"] != "uml.sequence_3d/examples/basic.input.json" {
		t.Fatalf("unexpected schema file locations: %#v", data)
	}
	jsonSchema := objectMap(t, data["json_schema"])
	properties := objectMap(t, jsonSchema["properties"])
	for _, field := range []string{"participants", "messages", "activations", "fragments", "phases", "visual"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("uml.sequence_3d schema missing %s: %#v", field, properties)
		}
	}
	participantProps := objectMap(t, objectMap(t, objectMap(t, properties["participants"])["items"])["properties"])
	for _, field := range []string{"display_name", "subtitle", "lane_index", "depth", "color"} {
		if _, ok := participantProps[field]; !ok {
			t.Fatalf("uml sequence participant schema missing %s: %#v", field, participantProps)
		}
	}
	messageProps := objectMap(t, objectMap(t, objectMap(t, properties["messages"])["items"])["properties"])
	for _, field := range []string{"summary", "importance", "label_priority", "curve", "depth"} {
		if _, ok := messageProps[field]; !ok {
			t.Fatalf("uml sequence message schema missing %s: %#v", field, messageProps)
		}
	}
	example := objectMap(t, data["example"])
	if len(example["participants"].([]any)) < 6 || len(example["messages"].([]any)) < 12 {
		t.Fatalf("uml sequence example is too small: %#v", example)
	}
	visual := objectMap(t, example["visual"])
	if len(visual["initial_focus_ids"].([]any)) < 3 || len(visual["annotations"].([]any)) < 2 {
		t.Fatalf("uml sequence example visual guidance is weak: %#v", visual)
	}

	inspect := runVisualOK(t, "inspect-input", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--json")
	summary := objectMap(t, objectMap(t, inspect["data"])["summary"])
	for field, want := range map[string]float64{"participants": 6, "messages": 12, "phases": 4, "activations": 4, "fragments": 2} {
		if summary[field].(float64) != want {
			t.Fatalf("inspect-input summary missing %s=%v: %#v", field, want, summary)
		}
	}
	for _, field := range []string{"visual_focus_ids", "visual_annotations", "visual_narrative_steps"} {
		if _, ok := summary[field]; !ok {
			t.Fatalf("inspect-input summary missing visual field %s: %#v", field, summary)
		}
	}

	out := filepath.Join(t.TempDir(), "uml-sequence")
	rendered := runVisualOK(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--out", out, "--title", "Sequence Contract", "--json")
	artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
	assertArtifactContract(t, artifact, "uml.sequence_3d", "Sequence Contract")
	files := stringSetFromAny(artifact["files"].([]any))
	for _, file := range []string{"index.html", "manifest.json", "manifest.js", "data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js", "assets/runtime/efp-visual-runtime.css", "assets/vendor/three/efp-three.module.min.js"} {
		if !files[file] {
			t.Fatalf("rendered artifact missing file %s in %#v", file, files)
		}
	}

	inspected := runVisualOK(t, "inspect-output", "--out", out, "--json")
	inspectArtifact := objectMap(t, objectMap(t, inspected["data"])["artifact"])
	assertArtifactContract(t, inspectArtifact, "", "")
}

func TestVisualAllTemplatesExposeVisualAuthoringContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	for _, entry := range registry.Templates {
		t.Run(entry.ID, func(t *testing.T) {
			schemaDoc := loadJSONMap(t, filepath.Join(templateDir, entry.ID, "schema.input.json"))
			jsonSchema := objectMap(t, schemaDoc["json_schema"])
			properties := objectMap(t, jsonSchema["properties"])
			visualSchema, ok := properties["visual"].(map[string]any)
			if !ok {
				t.Fatalf("%s schema missing visual authoring contract: %#v", entry.ID, properties)
			}
			visualProps := objectMap(t, visualSchema["properties"])
			for _, field := range []string{"goal", "initial_focus_ids", "hidden_detail_ids", "narrative_steps", "annotations"} {
				if _, ok := visualProps[field]; !ok {
					t.Fatalf("%s visual schema missing %s: %#v", entry.ID, field, visualProps)
				}
			}
			example := loadJSONMap(t, filepath.Join(templateDir, entry.ID, "examples", "basic.input.json"))
			visual := objectMap(t, example["visual"])
			if visual["goal"] == "" || len(visual["initial_focus_ids"].([]any)) == 0 || len(visual["narrative_steps"].([]any)) == 0 || len(visual["annotations"].([]any)) == 0 {
				t.Fatalf("%s example visual guidance incomplete: %#v", entry.ID, visual)
			}
			inspect := runVisualOK(t, "inspect-input", "--template", entry.ID, "--template-dir", templateDir, "--input", filepath.Join(templateDir, entry.ID, "examples", "basic.input.json"), "--json")
			summary := objectMap(t, objectMap(t, inspect["data"])["summary"])
			if summary["visual_focus_ids"] == nil || summary["visual_annotations"] == nil {
				t.Fatalf("%s inspect-input did not report visual guidance counts: %#v", entry.ID, summary)
			}
		})
	}
}

func TestVisualAllTemplatesExposeMarkSystemAuthoringContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	registry := loadRegistry(t, templateDir)
	for _, entry := range registry.Templates {
		t.Run(entry.ID, func(t *testing.T) {
			schemaDoc := loadJSONMap(t, filepath.Join(templateDir, entry.ID, "schema.input.json"))
			jsonSchema := objectMap(t, schemaDoc["json_schema"])
			properties := objectMap(t, jsonSchema["properties"])
			for _, field := range markObjectArrays(entry.InputSchemaKind) {
				assertObjectArrayMarkPresentation(t, entry.ID, properties, field)
			}
			for _, field := range []string{"edges", "relationships", "messages", "links", "flows", "transitions"} {
				if _, ok := properties[field]; ok {
					assertRelationshipArrayMarkPresentation(t, entry.ID, properties, field)
				}
			}
		})
	}
}

func TestVisualUMLFamilyExamplesValidateAndRender(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	templates := []string{
		"uml.sequence_3d",
		"uml.class_structure_2_5d",
		"uml.state_machine_3d",
		"uml.activity_flow_3d",
		"uml.component_deployment_3d",
	}
	for _, templateID := range templates {
		t.Run(templateID, func(t *testing.T) {
			input := filepath.Join(templateDir, templateID, "examples", "basic.input.json")
			runVisualOK(t, "template", "schema", templateID, "--template-dir", templateDir, "--json")
			runVisualOK(t, "validate", "--template", templateID, "--template-dir", templateDir, "--input", input, "--json")
			runVisualOK(t, "inspect-input", "--template", templateID, "--template-dir", templateDir, "--input", input, "--json")
			out := filepath.Join(t.TempDir(), strings.ReplaceAll(templateID, ".", "-"))
			rendered := runVisualOK(t, "render", "--template", templateID, "--template-dir", templateDir, "--input", input, "--out", out, "--json")
			artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
			assertArtifactContract(t, artifact, templateID, "")
		})
	}
}

func TestVisualDoctorRendersAllSemanticTemplates(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	obj := runVisualOK(t, "template", "doctor", "--template-dir", templateDir, "--json")
	data := objectMap(t, obj["data"])
	for key, want := range map[string]float64{
		"checked_templates":            33,
		"checked_examples":             33,
		"rendered_examples":            33,
		"canonical_templates":          33,
		"expected_canonical_templates": 33,
		"canonical_template_dirs":      33,
	} {
		if data[key].(float64) != want {
			t.Fatalf("doctor expected %s=%.0f, got %#v", key, want, data)
		}
	}
	if data["offline"] != true {
		t.Fatalf("doctor did not report offline=true: %#v", data)
	}
	if len(data["orphan_template_dirs"].([]any)) != 0 {
		t.Fatalf("doctor reported orphan dirs: %#v", data["orphan_template_dirs"])
	}
}

func TestVisualRepresentativeSemanticRenders(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	for _, templateID := range []string{
		"relationship.dependency_graph",
		"temporal.event_trace",
		"flow.pipeline",
		"hierarchy.layered_architecture",
		"evidence.claim_source_board",
		"matrix.kpi_control",
		"spatial.codebase_galaxy",
	} {
		t.Run(templateID, func(t *testing.T) {
			input := filepath.Join(templateDir, templateID, "examples", "basic.input.json")
			out := filepath.Join(t.TempDir(), strings.ReplaceAll(templateID, ".", "-"))
			rendered := runVisualOK(t, "render", "--template", templateID, "--template-dir", templateDir, "--input", input, "--out", out, "--json")
			artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
			assertArtifactContract(t, artifact, templateID, "")
		})
	}
}

func TestVisualRenderOverwriteAndDryRunContracts(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	input := filepath.Join(templateDir, "uml.sequence_3d", "examples", "basic.input.json")
	out := filepath.Join(t.TempDir(), "render")
	runVisualOK(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	fail := runVisual(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	assertErrorCode(t, fail, "output_exists")
	runVisualOK(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--out", out, "--overwrite", "--json")
	dryOut := filepath.Join(t.TempDir(), "dry-run")
	dry := runVisualOK(t, "render", "--template", "uml.sequence_3d", "--template-dir", templateDir, "--input", input, "--out", dryOut, "--dry-run", "--json")
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
		if ext != ".js" && ext != ".css" && ext != ".html" && ext != ".yaml" && ext != ".json" {
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

func TestVisualSmokeScriptsUseSemanticTemplates(t *testing.T) {
	root := repoRoot(t)
	sh := mustRead(t, filepath.Join(root, "scripts", "smoke.sh"))
	ps := mustRead(t, filepath.Join(root, "scripts", "smoke.ps1"))
	for _, text := range []string{sh, ps} {
		for _, want := range []string{"uml.sequence_3d", "relationship.dependency_graph", "temporal.event_trace", "template doctor", "template schema", "template guide", "inspect-plan", "inspect-render"} {
			if !strings.Contains(text, want) {
				t.Fatalf("smoke script missing %s", want)
			}
		}
		for _, old := range []string{"agent.run_trace", "codebase.module_dependency_graph", "foundation.graph_3d"} {
			if strings.Contains(text, old) {
				t.Fatalf("smoke script still references old template id %s", old)
			}
		}
	}
}

func TestVisualGraphInteractionKeepsCameraStableForNodeDragAndExpand(t *testing.T) {
	root := repoRoot(t)
	renderer := mustRead(t, filepath.Join(root, "templates", "visual", "_shared", "runtime", "efp-visual-renderers.iife.js"))
	for _, want := range []string{
		"function anchorExpandedLayout(positions)",
		"anchorExpandedLayout(positions);",
		`} else if (dragMode === "pendingNode") {`,
		`dragMode = "idle";`,
		`} else if (dragMode === "orbit") {`,
	} {
		if !strings.Contains(renderer, want) {
			t.Fatalf("visual 3D graph interaction contract missing %q", want)
		}
	}
	if strings.Contains(renderer, `} else {
            orbit.theta -= dx * 0.006;
            orbit.phi -= dy * 0.005;
          }`) {
		t.Fatal("node drag fallback can still fall through into orbit camera movement")
	}
}

func TestVisualRendererConsumesVisualAuthoringHints(t *testing.T) {
	root := repoRoot(t)
	renderer := mustRead(t, filepath.Join(root, "templates", "visual", "_shared", "runtime", "efp-visual-renderers.iife.js"))
	for _, want := range []string{
		"function readVisualHints(data)",
		"function resolveMarkSpec",
		"function createMarkMesh",
		"function resolveEdgeSpec",
		"function createArrowHead",
		"renderMatrix(ctx)",
		"data-mark-shape",
		"TubeGeometry",
		"initial_focus_ids",
		"hidden_detail_ids",
		"visual-three-annotation-label",
		"visual-uml-phase-legend",
		"visual-legend-overlay",
		"lane_index",
		"CatmullRomCurve3",
	} {
		if !strings.Contains(renderer, want) {
			t.Fatalf("visual renderer missing shared visual hint support %q", want)
		}
	}
	css := mustRead(t, filepath.Join(root, "templates", "visual", "_shared", "runtime", "efp-visual-runtime.css"))
	for _, want := range []string{"visual-card-focus", "visual-card-annotation", "visual-three-annotation-label", "visual-uml-annotation-label"} {
		if !strings.Contains(css, want) {
			t.Fatalf("visual runtime css missing %s", want)
		}
	}
}

func TestVisualMarkSystemCloudArchitectureContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	shared := filepath.Join(templateDir, "_shared")
	for _, rel := range []string{
		"agent-guidance/mark-grammar.md",
		"mark-registry.json",
		"asset-registry.json",
		"assets/ATTRIBUTIONS.md",
		"assets/models/generic/placeholder.json",
	} {
		if info, err := os.Stat(filepath.Join(shared, filepath.FromSlash(rel))); err != nil || info.IsDir() || info.Size() == 0 {
			t.Fatalf("missing non-empty shared mark asset %s", rel)
		}
	}
	for _, rel := range []string{
		"assets/icons/generic/service.svg",
		"assets/icons/generic/api.svg",
		"assets/icons/generic/database.svg",
		"assets/icons/generic/storage.svg",
		"assets/icons/generic/queue.svg",
		"assets/icons/generic/stream.svg",
		"assets/icons/generic/pipeline.svg",
		"assets/icons/generic/job.svg",
		"assets/icons/generic/user.svg",
		"assets/icons/generic/external.svg",
		"assets/icons/generic/warning.svg",
		"assets/icons/generic/decision.svg",
		"assets/icons/aws/lambda.svg",
		"assets/icons/aws/s3.svg",
		"assets/icons/aws/rds.svg",
		"assets/icons/aws/dynamodb.svg",
		"assets/icons/aws/ec2.svg",
		"assets/icons/aws/eks.svg",
		"assets/icons/aws/sqs.svg",
		"assets/icons/aws/sns.svg",
		"assets/icons/aws/eventbridge.svg",
		"assets/icons/aws/api_gateway.svg",
		"assets/icons/aws/cloudfront.svg",
		"assets/icons/aws/cloudwatch.svg",
		"assets/icons/aws/secrets_manager.svg",
		"assets/icons/jenkins/jenkins.svg",
	} {
		if info, err := os.Stat(filepath.Join(shared, filepath.FromSlash(rel))); err != nil || info.IsDir() || info.Size() == 0 {
			t.Fatalf("missing non-empty mark icon %s", rel)
		}
	}

	badInput := filepath.Join(templateDir, "relationship.dependency_graph", "examples", "cloud-architecture-bad.input.json")
	goodInput := filepath.Join(templateDir, "relationship.dependency_graph", "examples", "cloud-architecture-good.input.json")
	bad := runVisualOK(t, "inspect-input", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", badInput, "--json")
	badCodes := warningCodeSet(t, bad)
	for _, code := range []string{"generic_sphere_overuse", "mark_shape_missing", "edge_direction_missing", "arrow_encoding_missing", "color_encoding_missing", "legend_missing", "asset_icon_unknown"} {
		if !badCodes[code] {
			t.Fatalf("cloud bad example missing mark warning %s in %#v", code, badCodes)
		}
	}
	good := runVisualOK(t, "inspect-input", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", goodInput, "--json")
	if objectMap(t, objectMap(t, good["data"]))["quality_score"].(float64) <= objectMap(t, objectMap(t, bad["data"]))["quality_score"].(float64) {
		t.Fatalf("cloud good example should score higher than bad")
	}
	goodCodes := warningCodeSet(t, good)
	for _, code := range []string{"generic_sphere_overuse", "edge_direction_missing", "arrow_encoding_missing", "color_encoding_missing", "legend_missing", "asset_icon_unknown"} {
		if goodCodes[code] {
			t.Fatalf("cloud good example unexpectedly has warning %s in %#v", code, goodCodes)
		}
	}

	out := filepath.Join(t.TempDir(), "cloud-arch")
	planObj := runVisualOK(t, "inspect-plan", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", goodInput, "--out", out, "--json")
	planData := objectMap(t, planObj["data"])
	if planData["ready"] != true || planData["quality_score"].(float64) < 90 {
		t.Fatalf("cloud inspect-plan should be ready with high quality: %#v", planData)
	}
	plan := objectMap(t, planData["visual_plan"])
	marks := objectMap(t, plan["marks"])
	if marks["fallback_sphere_count"].(float64) != 0 {
		t.Fatalf("cloud mark plan should not fall back to spheres: %#v", marks)
	}
	shapeCounts := objectMap(t, marks["shape_counts"])
	for _, shape := range []string{"service_box", "database_cylinder", "queue_capsule", "cloud_plate", "ci_card"} {
		if shapeCounts[shape] == nil {
			t.Fatalf("cloud mark plan missing shape %s in %#v", shape, shapeCounts)
		}
	}
	if len(shapeCounts) < 5 {
		t.Fatalf("cloud mark plan should use diverse shapes: %#v", shapeCounts)
	}
	edges := objectMap(t, plan["edges"])
	if edges["directed_count"].(float64) < 8 || edges["arrow_count"].(float64) < 8 || edges["undirected_count"].(float64) != 0 {
		t.Fatalf("cloud edge plan should use visible directed arrows: %#v", edges)
	}
	colors := objectMap(t, plan["colors"])
	if colors["colorBy"] != "provider" || colors["single_color"] == true || len(colors["legend_items"].([]any)) < 4 {
		t.Fatalf("cloud color plan should expose provider legend: %#v", colors)
	}
	assets := objectMap(t, plan["assets"])
	iconsUsed := stringSetFromAny(assets["icons_used"].([]any))
	for _, icon := range []string{"aws.lambda", "aws.rds", "aws.sqs", "jenkins"} {
		if !iconsUsed[icon] {
			t.Fatalf("cloud asset plan missing icon %s in %#v", icon, iconsUsed)
		}
	}
	if anyArrayLen(assets["missing_icons"]) != 0 || anyArrayLen(assets["attributions"]) == 0 {
		t.Fatalf("cloud asset plan should include icons and attributions: %#v", assets)
	}

	rendered := runVisualOK(t, "render", "--template", "relationship.dependency_graph", "--template-dir", templateDir, "--input", goodInput, "--out", out, "--json")
	artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
	assertArtifactContract(t, artifact, "relationship.dependency_graph", "")
	files := stringSetFromAny(artifact["files"].([]any))
	for _, rel := range []string{
		"assets/agent-guidance/mark-grammar.md",
		"assets/asset-registry.json",
		"assets/mark-registry.json",
		"assets/ATTRIBUTIONS.md",
		"assets/icons/generic/database.svg",
		"assets/icons/aws/lambda.svg",
		"assets/icons/aws/rds.svg",
		"assets/icons/aws/sqs.svg",
		"assets/icons/jenkins/jenkins.svg",
	} {
		if !files[rel] {
			t.Fatalf("cloud artifact missing mark asset %s in %#v", rel, files)
		}
	}
	manifest := loadJSONMap(t, filepath.Join(out, "manifest.json"))
	manifestAssets := objectMap(t, manifest["assets"])
	if len(manifestAssets["icons"].([]any)) < 20 || len(manifestAssets["attributions"].([]any)) == 0 {
		t.Fatalf("output manifest missing mark asset metadata: %#v", manifestAssets)
	}
	if _, ok := manifestAssets["mark_registry"].(map[string]any); !ok {
		t.Fatalf("output manifest missing embedded mark registry: %#v", manifestAssets)
	}
	if _, ok := manifestAssets["asset_registry"].(map[string]any); !ok {
		t.Fatalf("output manifest missing embedded asset registry: %#v", manifestAssets)
	}

	inspected := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", out, "--json")
	renderData := objectMap(t, inspected["data"])
	if renderData["ready"] != true || renderData["render_score"].(float64) < 90 {
		t.Fatalf("cloud inspect-render should be ready: %#v", renderData)
	}
	checks := objectMap(t, renderData["checks"])
	for _, field := range []string{"shape_diversity", "arrows_visible", "color_diversity", "legend_present", "icon_assets_present", "attributions_present"} {
		if checks[field] != true {
			t.Fatalf("cloud inspect-render check %s failed: %#v", field, checks)
		}
	}
}

func TestVisualTimelineAndEvidenceMarkContracts(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")

	timelineInput := filepath.Join(templateDir, "temporal.incident_timeline", "examples", "marked-event-trace.input.json")
	timelineOut := filepath.Join(t.TempDir(), "marked-timeline")
	timelinePlanObj := runVisualOK(t, "inspect-plan", "--template", "temporal.incident_timeline", "--template-dir", templateDir, "--input", timelineInput, "--out", timelineOut, "--json")
	timelineData := objectMap(t, timelinePlanObj["data"])
	if timelineData["ready"] != true {
		t.Fatalf("marked timeline inspect-plan should be ready: %#v", timelineData)
	}
	timelinePlan := objectMap(t, timelineData["visual_plan"])
	timelineMarks := objectMap(t, timelinePlan["marks"])
	timelineShapes := objectMap(t, timelineMarks["shape_counts"])
	if len(timelineShapes) < 5 || timelineMarks["fallback_sphere_count"].(float64) > 1 {
		t.Fatalf("marked timeline should expose diverse non-fallback marks: %#v", timelineMarks)
	}
	timelineColors := objectMap(t, timelinePlan["colors"])
	if timelineColors["colorBy"] != "provider" || len(timelineColors["legend_items"].([]any)) < 4 {
		t.Fatalf("marked timeline should expose provider color legend: %#v", timelineColors)
	}
	if objectMap(t, timelinePlan["legend"])["show"] != true {
		t.Fatalf("marked timeline legend should be present: %#v", timelinePlan["legend"])
	}
	timelineAssets := objectMap(t, timelinePlan["assets"])
	if anyArrayLen(timelineAssets["icons_used"]) == 0 || anyArrayLen(timelineAssets["missing_icons"]) != 0 {
		t.Fatalf("marked timeline should use local registered icons: %#v", timelineAssets)
	}
	runVisualOK(t, "render", "--template", "temporal.incident_timeline", "--template-dir", templateDir, "--input", timelineInput, "--out", timelineOut, "--json")
	timelineRender := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", timelineOut, "--json")
	timelineRenderData := objectMap(t, timelineRender["data"])
	if timelineRenderData["ready"] != true {
		t.Fatalf("marked timeline inspect-render should be ready: %#v", timelineRenderData)
	}
	timelineChecks := objectMap(t, timelineRenderData["checks"])
	for _, field := range []string{"shape_diversity", "color_diversity", "legend_present", "icon_assets_present", "attributions_present"} {
		if timelineChecks[field] != true {
			t.Fatalf("marked timeline inspect-render check %s failed: %#v", field, timelineChecks)
		}
	}

	evidenceInput := filepath.Join(templateDir, "evidence.claim_source_board", "examples", "marked-evidence-board.input.json")
	evidenceOut := filepath.Join(t.TempDir(), "marked-evidence")
	evidencePlanObj := runVisualOK(t, "inspect-plan", "--template", "evidence.claim_source_board", "--template-dir", templateDir, "--input", evidenceInput, "--out", evidenceOut, "--json")
	evidenceData := objectMap(t, evidencePlanObj["data"])
	if evidenceData["ready"] != true {
		t.Fatalf("marked evidence inspect-plan should be ready: %#v", evidenceData)
	}
	evidencePlan := objectMap(t, evidenceData["visual_plan"])
	evidenceMarks := objectMap(t, evidencePlan["marks"])
	evidenceShapes := objectMap(t, evidenceMarks["shape_counts"])
	for _, shape := range []string{"hex_service", "queue_capsule", "warning_prism", "diamond", "database_cylinder", "ci_card"} {
		if evidenceShapes[shape] == nil {
			t.Fatalf("marked evidence plan missing shape %s in %#v", shape, evidenceShapes)
		}
	}
	evidenceAssets := objectMap(t, evidencePlan["assets"])
	evidenceIcons := stringSetFromAny(evidenceAssets["icons_used"].([]any))
	for _, icon := range []string{"generic.api", "generic.warning", "generic.decision", "aws.sqs", "jenkins"} {
		if !evidenceIcons[icon] {
			t.Fatalf("marked evidence plan missing icon %s in %#v", icon, evidenceIcons)
		}
	}
	evidenceEdges := objectMap(t, evidencePlan["edges"])
	if evidenceEdges["directed_count"].(float64) < 7 || evidenceEdges["arrow_count"].(float64) < 7 {
		t.Fatalf("marked evidence should count directed relation arrows: %#v", evidenceEdges)
	}
	evidenceColors := objectMap(t, evidencePlan["colors"])
	if evidenceColors["colorBy"] != "relation" || len(evidenceColors["legend_items"].([]any)) < 3 {
		t.Fatalf("marked evidence should expose relation color legend: %#v", evidenceColors)
	}
	runVisualOK(t, "render", "--template", "evidence.claim_source_board", "--template-dir", templateDir, "--input", evidenceInput, "--out", evidenceOut, "--json")
	evidenceRender := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", evidenceOut, "--json")
	evidenceRenderData := objectMap(t, evidenceRender["data"])
	if evidenceRenderData["ready"] != true {
		t.Fatalf("marked evidence inspect-render should be ready: %#v", evidenceRenderData)
	}
	evidenceChecks := objectMap(t, evidenceRenderData["checks"])
	for _, field := range []string{"shape_diversity", "arrows_visible", "color_diversity", "legend_present", "icon_assets_present", "attributions_present"} {
		if evidenceChecks[field] != true {
			t.Fatalf("marked evidence inspect-render check %s failed: %#v", field, evidenceChecks)
		}
	}
}

func TestVisualMatrixMarkSystemContract(t *testing.T) {
	root := repoRoot(t)
	templateDir := filepath.Join(root, "templates", "visual")
	input := filepath.Join(templateDir, "matrix.capability", "examples", "marked-cloud-capability.input.json")
	out := filepath.Join(t.TempDir(), "marked-matrix")

	planObj := runVisualOK(t, "inspect-plan", "--template", "matrix.capability", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	planData := objectMap(t, planObj["data"])
	if planData["ready"] != true || planData["quality_score"].(float64) < 90 {
		t.Fatalf("marked matrix inspect-plan should be ready with high quality: %#v", planData)
	}
	plan := objectMap(t, planData["visual_plan"])
	marks := objectMap(t, plan["marks"])
	if marks["fallback_sphere_count"].(float64) != 0 {
		t.Fatalf("marked matrix should not fall back to spheres: %#v", marks)
	}
	shapeCounts := objectMap(t, marks["shape_counts"])
	for _, shape := range []string{"service_box", "hex_service", "database_cylinder", "queue_capsule", "event_bus", "bucket", "cloud_plate", "warning_prism", "ci_card"} {
		if shapeCounts[shape] == nil {
			t.Fatalf("marked matrix shape_counts missing %s in %#v", shape, shapeCounts)
		}
	}
	colors := objectMap(t, plan["colors"])
	if colors["colorBy"] != "provider" || colors["single_color"] == true || len(colors["legend_items"].([]any)) < 6 {
		t.Fatalf("marked matrix should expose provider color legend: %#v", colors)
	}
	assets := objectMap(t, plan["assets"])
	iconsUsed := stringSetFromAny(assets["icons_used"].([]any))
	for _, icon := range []string{"aws.api_gateway", "aws.lambda", "aws.rds", "aws.sqs", "aws.eventbridge", "aws.s3", "aws.secrets_manager", "jenkins"} {
		if !iconsUsed[icon] {
			t.Fatalf("marked matrix asset plan missing icon %s in %#v", icon, iconsUsed)
		}
	}
	if anyArrayLen(assets["missing_icons"]) != 0 || anyArrayLen(assets["attributions"]) == 0 {
		t.Fatalf("marked matrix should include local icons and attributions: %#v", assets)
	}

	rendered := runVisualOK(t, "render", "--template", "matrix.capability", "--template-dir", templateDir, "--input", input, "--out", out, "--json")
	artifact := objectMap(t, objectMap(t, rendered["data"])["artifact"])
	assertArtifactContract(t, artifact, "matrix.capability", "")
	files := stringSetFromAny(artifact["files"].([]any))
	for _, rel := range []string{"assets/agent-guidance/mark-grammar.md", "assets/asset-registry.json", "assets/mark-registry.json", "assets/icons/aws/api_gateway.svg", "assets/icons/aws/rds.svg", "assets/icons/jenkins/jenkins.svg"} {
		if !files[rel] {
			t.Fatalf("marked matrix artifact missing mark asset %s in %#v", rel, files)
		}
	}

	inspected := runVisualOK(t, "inspect-render", "--template-dir", templateDir, "--out", out, "--json")
	renderData := objectMap(t, inspected["data"])
	if renderData["ready"] != true || renderData["render_score"].(float64) < 90 {
		t.Fatalf("marked matrix inspect-render should be ready: %#v", renderData)
	}
	checks := objectMap(t, renderData["checks"])
	for _, field := range []string{"shape_diversity", "color_diversity", "legend_present", "icon_assets_present", "attributions_present"} {
		if checks[field] != true {
			t.Fatalf("marked matrix inspect-render check %s failed: %#v", field, checks)
		}
	}
}

func markObjectArrays(kind string) []string {
	switch kind {
	case "graph_v1":
		return []string{"nodes"}
	case "graph_events_v1":
		return []string{"nodes", "events"}
	case "timeline_v1":
		return []string{"events"}
	case "evidence_v1":
		return []string{"claims", "sources"}
	case "matrix_v1":
		return []string{"items"}
	case "uml_sequence_v1":
		return []string{"participants"}
	case "uml_class_v1":
		return []string{"classes"}
	case "uml_state_machine_v1":
		return []string{"states"}
	case "uml_activity_v1":
		return []string{"actions"}
	case "uml_component_deployment_v1":
		return []string{"components", "deployments"}
	default:
		return nil
	}
}

func assertObjectArrayMarkPresentation(t *testing.T, templateID string, properties map[string]any, field string) {
	t.Helper()
	fieldSchema, ok := properties[field].(map[string]any)
	if !ok {
		t.Fatalf("%s schema missing object array %s: %#v", templateID, field, properties)
	}
	itemProps := objectMap(t, objectMap(t, fieldSchema["items"])["properties"])
	presentation := objectMap(t, itemProps["presentation"])
	presentationProps := objectMap(t, presentation["properties"])
	for _, name := range []string{"shape", "icon", "color"} {
		if _, ok := presentationProps[name]; !ok {
			t.Fatalf("%s %s[] presentation missing %s: %#v", templateID, field, name, presentationProps)
		}
	}
}

func assertRelationshipArrayMarkPresentation(t *testing.T, templateID string, properties map[string]any, field string) {
	t.Helper()
	fieldSchema := objectMap(t, properties[field])
	itemProps := objectMap(t, objectMap(t, fieldSchema["items"])["properties"])
	if _, ok := itemProps["directed"]; !ok {
		t.Fatalf("%s %s[] missing directed field: %#v", templateID, field, itemProps)
	}
	presentation := objectMap(t, itemProps["presentation"])
	presentationProps := objectMap(t, presentation["properties"])
	for _, name := range []string{"arrow", "color"} {
		if _, ok := presentationProps[name]; !ok {
			t.Fatalf("%s %s[] presentation missing %s: %#v", templateID, field, name, presentationProps)
		}
	}
}

func assertRegistryEntryQuality(t *testing.T, entry visualRegistryEntry) {
	t.Helper()
	if entry.Title == "" || entry.Description == "" {
		t.Fatalf("registry entry missing title or description: %#v", entry)
	}
	for _, re := range genericDescriptionPatterns {
		if re.MatchString(entry.Description) {
			t.Fatalf("%s has generic description: %s", entry.ID, entry.Description)
		}
	}
	if !semanticCategoryCountsHas(entry.Category) {
		t.Fatalf("unsupported semantic category %s for %s", entry.Category, entry.ID)
	}
	if !semanticSchemaKinds[entry.InputSchemaKind] {
		t.Fatalf("unsupported schema kind %s for %s", entry.InputSchemaKind, entry.ID)
	}
	if !semanticRenderers[entry.Renderer] {
		t.Fatalf("unsupported renderer %s for %s", entry.Renderer, entry.ID)
	}
	if len(entry.Tags) < 3 {
		t.Fatalf("%s should have at least 3 tags: %#v", entry.ID, entry.Tags)
	}
}

func assertManifestMatchesRegistry(t *testing.T, entry visualRegistryEntry, manifest visualTemplateManifest) {
	t.Helper()
	if manifest.ID != entry.ID || manifest.Category != entry.Category || manifest.Title != entry.Title || manifest.Description != entry.Description {
		t.Fatalf("manifest does not match registry for %s: entry=%#v manifest=%#v", entry.ID, entry, manifest)
	}
	if manifest.InputSchema != "schema.input.json" || manifest.InputSchemaKind != entry.InputSchemaKind || manifest.Renderer.Contract != entry.Renderer || manifest.Layout.Preset != entry.LayoutPreset {
		t.Fatalf("manifest contract does not match registry for %s: entry=%#v manifest=%#v", entry.ID, entry, manifest)
	}
	if manifest.Effects.Engine != "three.v1" || manifest.Effects.Scene == "" {
		t.Fatalf("%s manifest must declare three.v1 effects with a scene id: %#v", entry.ID, manifest.Effects)
	}
	if manifest.VisualDesign.InitialView == "" || len(manifest.VisualDesign.AgentGuidance) < 3 || len(manifest.VisualDesign.Supports) < 4 {
		t.Fatalf("%s manifest lacks visual design guidance: %#v", entry.ID, manifest.VisualDesign)
	}
	if !manifest.Offline.Required || !manifest.Offline.ForbidNetwork || manifest.Offline.DataMode != "js-file" {
		t.Fatalf("%s offline contract incomplete: %#v", entry.ID, manifest.Offline)
	}
	if len(manifest.Styles) < 2 || len(manifest.Scripts) < 4 || len(manifest.Tags) < 3 {
		t.Fatalf("%s manifest styles/scripts/tags incomplete: %#v", entry.ID, manifest)
	}
}

func assertTemplateFiles(t *testing.T, templateDir string, entry visualRegistryEntry) {
	t.Helper()
	base := filepath.Join(templateDir, entry.ID)
	for _, rel := range []string{"template.yaml", "schema.input.json", "examples/basic.input.json", "style.css"} {
		path := filepath.Join(base, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 {
			t.Fatalf("%s missing non-empty %s", entry.ID, rel)
		}
	}
	schema := loadJSONMap(t, filepath.Join(base, "schema.input.json"))
	if schema["template_id"] != entry.ID || schema["input_schema_kind"] != entry.InputSchemaKind {
		t.Fatalf("%s schema metadata mismatch: %#v", entry.ID, schema)
	}
	if _, ok := schema["json_schema"].(map[string]any); !ok {
		t.Fatalf("%s schema.input.json missing json_schema: %#v", entry.ID, schema)
	}
	if _, ok := schema["example"].(map[string]any); !ok {
		t.Fatalf("%s schema.input.json missing example: %#v", entry.ID, schema)
	}
	example := loadJSONMap(t, filepath.Join(base, "examples", "basic.input.json"))
	title, _ := example["title"].(string)
	if title == "" || title == "Basic Example" || title == "Example" || title == entry.Title+" Example" {
		t.Fatalf("%s example title is generic: %q", entry.ID, title)
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
	for _, field := range []string{"out_dir", "out", "entrypoint", "relative_entrypoint", "files"} {
		if artifact[field] == nil {
			t.Fatalf("artifact missing %s: %#v", field, artifact)
		}
	}
	if artifact["relative_entrypoint"] != "index.html" || artifact["offline"] != true || artifact["file_url_safe"] != true || artifact["http_subpath_safe"] != true {
		t.Fatalf("artifact compatibility fields invalid: %#v", artifact)
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out
	}
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

func loadTemplateManifest(t *testing.T, templateDir string, entry visualRegistryEntry) visualTemplateManifest {
	t.Helper()
	var manifest visualTemplateManifest
	raw := mustRead(t, filepath.Join(templateDir, filepath.FromSlash(entry.Path)))
	if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
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

func semanticCategoryCountsHas(category string) bool {
	_, ok := semanticCategoryCounts[category]
	return ok
}

func warningCodes(t *testing.T, obj map[string]any) []string {
	t.Helper()
	data := objectMap(t, obj["data"])
	raw, ok := data["warnings"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		warning := objectMap(t, item)
		out = append(out, warning["code"].(string))
	}
	return out
}

func warningCodeSet(t *testing.T, obj map[string]any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, code := range warningCodes(t, obj) {
		out[code] = true
	}
	return out
}

func objectMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %#v", value)
	}
	return m
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

func anyArrayLen(value any) int {
	items, _ := value.([]any)
	return len(items)
}

func writeSolidPNG(t *testing.T, path string, width, height int, fill color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
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

func TestVisualRegistrySortedByCategoryThenID(t *testing.T) {
	root := repoRoot(t)
	registry := loadRegistry(t, filepath.Join(root, "templates", "visual"))
	categoryOrder := map[string]int{"uml": 0, "relationship": 1, "temporal": 2, "flow": 3, "hierarchy": 4, "evidence": 5, "matrix": 6, "spatial": 7}
	for i := 1; i < len(registry.Templates); i++ {
		prev := registry.Templates[i-1]
		cur := registry.Templates[i]
		if categoryOrder[prev.Category] > categoryOrder[cur.Category] {
			t.Fatalf("registry category order regressed at %s before %s", prev.ID, cur.ID)
		}
		if prev.Category == cur.Category && prev.ID > cur.ID {
			t.Fatalf("registry ids should be sorted inside category %s: %s before %s", cur.Category, prev.ID, cur.ID)
		}
	}
}
