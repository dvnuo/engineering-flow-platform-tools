package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
	vcmd "engineering-flow-platform-tools/internal/visual/commands"
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
	Offline struct {
		Required      bool   `yaml:"required"`
		ForbidNetwork bool   `yaml:"forbid_network"`
		DataMode      string `yaml:"data_mode"`
	} `yaml:"offline"`
	Styles  []string `yaml:"styles"`
	Scripts []string `yaml:"scripts"`
	Tags    []string `yaml:"tags"`
}

var expectedVisualCategoryCounts = map[string]int{
	"foundation": 20,
	"agent":      15,
	"codebase":   20,
	"runtime":    20,
	"debug":      20,
	"project":    20,
	"knowledge":  20,
	"planning":   20,
	"business":   20,
	"education":  20,
}

var allowedVisualSchemaKinds = map[string]bool{
	"graph_v1":        true,
	"graph_events_v1": true,
	"timeline_v1":     true,
	"evidence_v1":     true,
	"matrix_v1":       true,
}

var allowedVisualRenderers = map[string]bool{
	"offline.graph.v1":    true,
	"offline.timeline.v1": true,
	"offline.evidence.v1": true,
	"offline.matrix.v1":   true,
}

var genericVisualDescriptionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Visualize .+ as a complete offline .+ view for .+ workflows using .+ layout\.$`),
	regexp.MustCompile(`^Visualize .+ structure, flow, and status.+$`),
	regexp.MustCompile(`^Offline .+ visualization for .+ workflows\.$`),
	regexp.MustCompile(`^.+ template for visual artifacts\.$`),
}

var highValueTemplateKeywords = map[string][]string{
	"foundation.timeline_tunnel":         {"release", "architecture", "prototype", "signoff"},
	"foundation.layered_stack":           {"domain", "adapter", "telemetry", "release"},
	"foundation.constellation":           {"capability", "billing", "analytics", "compliance"},
	"foundation.control_room":            {"queue", "latency", "error", "rollback"},
	"agent.run_trace":                    {"user", "plan", "tool", "test"},
	"agent.thinking_timeline":            {"goal", "context", "hypothesis", "verification"},
	"agent.tool_call_constellation":      {"shell", "registry", "schema", "push"},
	"agent.permission_gate_map":          {"permission", "network", "destructive", "commit"},
	"agent.active_run_monitor":           {"queue", "command", "verification", "handoff"},
	"agent.session_state_panel":          {"context", "worktree", "commit", "push"},
	"codebase.galaxy":                    {"repository", "package", "tests", "scripts"},
	"codebase.module_dependency_graph":   {"module", "manifest", "schema", "render"},
	"codebase.diff_impact_ripple":        {"changed", "tests", "contract", "risk"},
	"codebase.test_failure_map":          {"failure", "assertion", "fixture", "patch"},
	"runtime.service_topology":           {"service", "registry", "worker", "health"},
	"runtime.event_bus_flow":             {"producer", "broker", "consumer", "metrics"},
	"runtime.event_reconcile_loop":       {"portal", "opencode", "message.part.updated", "duplicate"},
	"runtime.session_binding_map":        {"browser", "session", "artifact", "audit"},
	"runtime.agent_fleet_dashboard":      {"fleet", "latency", "retry", "sla"},
	"debug.incident_timeline":            {"incident", "rollback", "customer", "postmortem"},
	"debug.root_cause_tree":              {"symptom", "deploy", "retry", "cause"},
	"project.issue_dependency_graph":     {"jira", "blocker", "release", "approval"},
	"project.requirements_to_code_trace": {"requirement", "confluence", "github", "tests"},
	"project.doc_freshness_map":          {"documentation", "alias", "doctor", "build"},
	"knowledge.evidence_board":           {"claim", "source", "confidence", "reliability"},
	"knowledge.answer_lineage_view":      {"answer", "validation", "powershell", "branch"},
	"planning.plan_dag":                  {"plan", "metadata", "tests", "verification"},
	"planning.critical_path_view":        {"critical", "alias", "smoke", "push"},
	"business.kpi_control_room":          {"users", "conversion", "revenue", "churn"},
	"education.auth_flow_animation":      {"browser", "identity", "token", "cookie"},
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
	for _, name := range []string{"render", "validate", "template.categories", "template.list", "template.get", "template.schema", "template.doctor", "inspect-output", "schema", "help.llm", "version"} {
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

func TestVisualSchemaTemplateSchemaJSONContract(t *testing.T) {
	obj := runVisualOK(t, "schema", "template.schema", "--json")
	data := obj["data"].(map[string]any)
	args := data["argument_details"].([]any)
	hasTemplateID := false
	for _, item := range args {
		m := item.(map[string]any)
		if m["name"] == "template_id" && m["required"] == true {
			hasTemplateID = true
		}
	}
	if !hasTemplateID {
		t.Fatalf("template.schema missing template_id argument: %#v", args)
	}
	flags := data["flags"].([]any)
	names := map[string]bool{}
	for _, item := range flags {
		m := item.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"template-dir", "json"} {
		if !names[name] {
			t.Fatalf("missing template.schema flag %s in %#v", name, names)
		}
	}
}

func TestVisualSchemaTemplateListJSONContract(t *testing.T) {
	obj := runVisualOK(t, "schema", "template.list", "--json")
	data := obj["data"].(map[string]any)
	flags := data["flags"].([]any)
	names := map[string]bool{}
	for _, item := range flags {
		m := item.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, name := range []string{"template-dir", "category", "query", "renderer", "schema-kind", "json"} {
		if !names[name] {
			t.Fatalf("missing template.list flag %s in %#v", name, names)
		}
	}
}

func TestVisualTemplateListGetDoctor(t *testing.T) {
	templateDir := visualTemplateDir()
	list := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--json")
	listData := list["data"].(map[string]any)
	templates := listData["templates"].([]any)
	if len(templates) != 195 || listData["canonical_count"].(float64) != 195 {
		t.Fatalf("expected 195 canonical templates, got len=%d data=%#v", len(templates), listData)
	}
	if listData["total_count"].(float64) < 195 || listData["alias_count"].(float64) < 10 {
		t.Fatalf("total_count should include canonical templates and aliases: %#v", listData)
	}

	categories := runVisualOK(t, "template", "categories", "--template-dir", templateDir, "--json")
	categoryData := categories["data"].(map[string]any)
	if categoryData["canonical_count"].(float64) != 195 {
		t.Fatalf("unexpected category canonical_count: %#v", categoryData)
	}
	categoryCounts := map[string]int{}
	for _, item := range categoryData["categories"].([]any) {
		m := item.(map[string]any)
		categoryCounts[m["id"].(string)] = int(m["count"].(float64))
	}
	if len(categoryCounts) != 10 {
		t.Fatalf("expected 10 categories, got %#v", categoryCounts)
	}
	for category, expected := range expectedVisualCategoryCounts {
		if categoryCounts[category] != expected {
			t.Fatalf("category %s expected %d, got %#v", category, expected, categoryCounts)
		}
	}

	agentList := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--category", "agent", "--json")
	if agentList["data"].(map[string]any)["matched_count"].(float64) != 15 {
		t.Fatalf("agent category filter failed: %#v", agentList)
	}
	dependencyList := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--query", "dependency", "--json")
	if dependencyList["data"].(map[string]any)["matched_count"].(float64) == 0 {
		t.Fatalf("query filter returned no templates: %#v", dependencyList)
	}
	graphList := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--renderer", "offline.graph.v1", "--schema-kind", "graph_v1", "--json")
	if graphList["data"].(map[string]any)["matched_count"].(float64) == 0 {
		t.Fatalf("renderer/schema-kind filter returned no templates: %#v", graphList)
	}

	got := runVisualOK(t, "template", "get", "agent.run_trace", "--template-dir", templateDir, "--json")
	data := got["data"].(map[string]any)
	if data["id"] != "agent.run_trace" || data["canonical_id"] != "agent.run_trace" || data["category"] != "agent" || data["version"] == "" || data["input_schema_kind"] != "graph_events_v1" {
		t.Fatalf("unexpected template get data: %#v", data)
	}
	renderer := data["renderer"].(map[string]any)
	if renderer["contract"] != "offline.graph.v1" {
		t.Fatalf("unexpected renderer: %#v", renderer)
	}
	if strings.TrimSpace(data["schema_file"].(string)) == "" || strings.TrimSpace(data["example_file"].(string)) == "" {
		t.Fatalf("template get missing schema/example files: %#v", data)
	}

	aliasGot := runVisualOK(t, "template", "get", "agent.tool_constellation", "--template-dir", templateDir, "--json")
	aliasData := aliasGot["data"].(map[string]any)
	if aliasData["requested_id"] != "agent.tool_constellation" || aliasData["canonical_id"] != "agent.tool_call_constellation" {
		t.Fatalf("alias get did not resolve canonical template: %#v", aliasData)
	}

	doctor := runVisualOK(t, "template", "doctor", "--template-dir", templateDir, "--json")
	doctorData := doctor["data"].(map[string]any)
	checked := doctorData["checked_templates"].(float64)
	if checked != 195 || doctorData["canonical_templates"].(float64) != 195 {
		t.Fatalf("expected 195 checked templates, got %#v", doctorData)
	}
	if doctorData["expected_canonical_templates"].(float64) != 195 {
		t.Fatalf("doctor missing expected canonical count: %#v", doctorData)
	}
	expectedCategories := doctorData["expected_categories"].(map[string]any)
	actualCategories := doctorData["categories"].(map[string]any)
	for category, expected := range expectedVisualCategoryCounts {
		if expectedCategories[category].(float64) != float64(expected) || actualCategories[category].(float64) != float64(expected) {
			t.Fatalf("doctor category mismatch for %s: expected=%#v actual=%#v", category, expectedCategories, actualCategories)
		}
	}
	if doctorData["checked_examples"].(float64) != 195 {
		t.Fatalf("expected 195 checked examples, got %#v", doctorData)
	}
	if doctorData["rendered_examples"].(float64) != 195 {
		t.Fatalf("expected 195 rendered examples, got %#v", doctorData)
	}
	if doctorData["offline"] != true {
		t.Fatalf("doctor did not report offline: %#v", doctorData)
	}
	if doctorData["unique_example_hashes"].(float64) < 190 {
		t.Fatalf("doctor did not report enough unique examples: %#v", doctorData)
	}
	if doctorData["canonical_template_dirs"].(float64) != 195 {
		t.Fatalf("doctor did not report canonical_template_dirs=195: %#v", doctorData)
	}
	orphanDirs := doctorData["orphan_template_dirs"].([]any)
	if len(orphanDirs) != 0 {
		t.Fatalf("doctor reported orphan template dirs: %#v", orphanDirs)
	}
	doctorTemplates := doctorData["templates"].([]any)
	if len(doctorTemplates) != 195 {
		t.Fatalf("expected 195 doctor template results, got %d", len(doctorTemplates))
	}
	for _, item := range doctorTemplates {
		m := item.(map[string]any)
		if m["rendered"] != true || strings.TrimSpace(m["example"].(string)) == "" || strings.TrimSpace(m["category"].(string)) == "" {
			t.Fatalf("unexpected doctor template result: %#v", m)
		}
	}
}

func TestVisualBackwardCompatibleAliases(t *testing.T) {
	templateDir := visualTemplateDir()
	aliases := map[string]string{
		"service.topology":           "runtime.service_topology",
		"runtime.session_binding":    "runtime.session_binding_map",
		"runtime.event_flow":         "runtime.event_bus_flow",
		"project.issue_graph":        "project.issue_dependency_graph",
		"project.requirements_trace": "project.requirements_to_code_trace",
		"knowledge.doc_freshness":    "project.doc_freshness_map",
		"agent.fleet_dashboard":      "runtime.agent_fleet_dashboard",
		"agent.permission_gate":      "agent.permission_gate_map",
		"agent.tool_constellation":   "agent.tool_call_constellation",
		"codebase.diff_impact":       "codebase.diff_impact_ripple",
	}
	for alias, canonical := range aliases {
		t.Run(alias, func(t *testing.T) {
			got := runVisualOK(t, "template", "get", alias, "--template-dir", templateDir, "--json")
			getData := got["data"].(map[string]any)
			if getData["requested_id"] != alias || getData["canonical_id"] != canonical {
				t.Fatalf("alias get mismatch: %#v", getData)
			}
			template := getData["template"].(map[string]any)
			if template["id"] != canonical {
				t.Fatalf("alias get returned wrong template: %#v", template)
			}

			schema := runVisualOK(t, "template", "schema", alias, "--template-dir", templateDir, "--json")
			schemaData := schema["data"].(map[string]any)
			schemaTemplate := schemaData["template"].(map[string]any)
			if schemaTemplate["requested_id"] != alias || schemaTemplate["canonical_id"] != canonical {
				t.Fatalf("alias schema mismatch: %#v", schemaTemplate)
			}
			jsonSchema, ok := schemaData["json_schema"].(map[string]any)
			if !ok || len(jsonSchema) <= 3 {
				t.Fatalf("alias schema missing json_schema: %#v", schemaData)
			}

			input := filepath.Join(templateDir, canonical, "examples", "basic.input.json")
			validated := runVisualOK(t, "validate", "--template", alias, "--template-dir", templateDir, "--input", input, "--json")
			if validated["data"].(map[string]any)["template_id"] != canonical {
				t.Fatalf("alias validate did not use canonical template: %#v", validated)
			}

			out := filepath.Join(t.TempDir(), strings.ReplaceAll(alias, ".", "-"))
			rendered := runVisualOK(t, "render", "--template", alias, "--template-dir", templateDir, "--input", input, "--out", out, "--title", "Alias Smoke", "--json")
			artifact := rendered["data"].(map[string]any)["artifact"].(map[string]any)
			if artifact["template_id"] != canonical {
				t.Fatalf("alias render did not use canonical template: %#v", artifact)
			}
			if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
				t.Fatalf("alias render did not write index.html: %v", err)
			}
		})
	}

	list := runVisualOK(t, "template", "list", "--template-dir", templateDir, "--json")
	listData := list["data"].(map[string]any)
	if listData["canonical_count"].(float64) != 195 || listData["total_count"].(float64) < 195 || listData["alias_count"].(float64) < float64(len(aliases)) {
		t.Fatalf("list count contract failed: %#v", listData)
	}
	ids := map[string]bool{}
	for _, item := range listData["templates"].([]any) {
		m := item.(map[string]any)
		id := m["id"].(string)
		if ids[id] {
			t.Fatalf("duplicate canonical id in list: %s", id)
		}
		ids[id] = true
	}

	doctor := runVisualOK(t, "template", "doctor", "--template-dir", templateDir, "--json")
	doctorData := doctor["data"].(map[string]any)
	if doctorData["checked_templates"].(float64) != 195 || doctorData["checked_examples"].(float64) != 195 || doctorData["rendered_examples"].(float64) != 195 || doctorData["offline"] != true {
		t.Fatalf("doctor alias contract failed: %#v", doctorData)
	}
}

func TestVisualDoctorUsesRegistryExpectedCounts(t *testing.T) {
	templateDir := filepath.Join(t.TempDir(), "visual")
	copyTree(t, visualTemplateDir(), templateDir)
	registry := visualRegistryDataFromDir(t, templateDir)
	registry.Expected.CanonicalCount = 196
	b, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(templateDir, "registry.json"), string(append(b, '\n')))

	fail := runVisual(t, "template", "doctor", "--template-dir", templateDir, "--json")
	assertErrorCode(t, fail, "template_doctor_failed")
	errObj := fail["error"].(map[string]any)
	text := strings.ToLower(fmt.Sprint(errObj["message"]) + " " + fmt.Sprint(errObj["hint"]))
	if !strings.Contains(text, "expected") || !strings.Contains(text, "mismatch") || !strings.Contains(text, "196") || !strings.Contains(text, "195") {
		t.Fatalf("doctor did not explain expected mismatch: %#v", errObj)
	}
}

func TestVisualNoUnregisteredTemplateDirectories(t *testing.T) {
	templateDir := visualTemplateDir()
	canonicalDirs := canonicalTemplateDirsFromRegistry(t, visualRegistryData(t))
	if len(canonicalDirs) != 195 {
		t.Fatalf("expected 195 canonical template dirs, got %d", len(canonicalDirs))
	}
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatal(err)
	}
	var orphan []string
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_shared" {
			continue
		}
		if !canonicalDirs[entry.Name()] {
			orphan = append(orphan, entry.Name())
		}
	}
	sort.Strings(orphan)
	if len(orphan) > 0 {
		t.Fatalf("templates/visual contains unregistered template directories: %#v", orphan)
	}
}

func TestVisualDoctorRejectsUnregisteredTemplateDirectories(t *testing.T) {
	templateDir := filepath.Join(t.TempDir(), "visual")
	copyTree(t, visualTemplateDir(), templateDir)
	if err := os.MkdirAll(filepath.Join(templateDir, "legacy.alias_only"), 0o755); err != nil {
		t.Fatal(err)
	}

	fail := runVisual(t, "template", "doctor", "--template-dir", templateDir, "--json")
	assertErrorCode(t, fail, "template_doctor_failed")
	errObj := fail["error"].(map[string]any)
	orphan := stringSetFromAny(errObj["orphan_template_dirs"].([]any))
	if !orphan["legacy.alias_only"] {
		t.Fatalf("doctor did not report orphan template dir: %#v", errObj)
	}
	text := strings.ToLower(fmt.Sprint(errObj["message"]) + " " + fmt.Sprint(errObj["hint"]))
	if !strings.Contains(text, "not registered") || !strings.Contains(text, "legacy") {
		t.Fatalf("doctor did not explain orphan template dirs: %#v", errObj)
	}
}

func TestVisualTemplateSchemaCommand(t *testing.T) {
	templateDir := visualTemplateDir()
	obj := runVisualOK(t, "template", "schema", "agent.run_trace", "--template-dir", templateDir, "--json")
	data := obj["data"].(map[string]any)
	template := data["template"].(map[string]any)
	if template["id"] != "agent.run_trace" || template["canonical_id"] != "agent.run_trace" || template["category"] != "agent" || template["version"] != "1.0.0" || template["renderer"] != "offline.graph.v1" || template["input_schema_kind"] != "graph_events_v1" {
		t.Fatalf("unexpected template schema metadata: %#v", template)
	}
	if data["schema_file"] != "agent.run_trace/schema.input.json" {
		t.Fatalf("unexpected schema_file: %#v", data["schema_file"])
	}
	jsonSchema, ok := data["json_schema"].(map[string]any)
	if !ok || len(jsonSchema) <= 3 {
		t.Fatalf("template schema returned stub json_schema: %#v", data["json_schema"])
	}
	props := jsonSchema["properties"].(map[string]any)
	if _, ok := props["nodes"]; !ok {
		t.Fatalf("json_schema missing nodes property: %#v", props)
	}
	if _, ok := props["events"]; !ok {
		t.Fatalf("json_schema missing events property: %#v", props)
	}
	example, ok := data["example"].(map[string]any)
	if !ok || len(example) == 0 {
		t.Fatalf("template schema missing example: %#v", data)
	}
	if data["example_file"] != "agent.run_trace/examples/basic.input.json" {
		t.Fatalf("unexpected example_file: %#v", data["example_file"])
	}
}

func TestVisualRegistryAndTemplateManifests(t *testing.T) {
	templateDir := visualTemplateDir()
	registry := visualRegistryData(t)
	if registry.Version != 2 {
		t.Fatalf("expected registry version 2, got %d", registry.Version)
	}
	if registry.Expected.CanonicalCount != 195 {
		t.Fatalf("expected registry metadata canonical_count 195, got %#v", registry.Expected)
	}
	for category, expected := range expectedVisualCategoryCounts {
		if registry.Expected.Categories[category] != expected {
			t.Fatalf("registry expected category %s should be %d, got %#v", category, expected, registry.Expected.Categories)
		}
	}
	if len(registry.Templates) != 195 {
		t.Fatalf("expected 195 registry templates, got %d", len(registry.Templates))
	}
	ids := map[string]bool{}
	aliases := map[string]string{}
	counts := map[string]int{}
	for _, entry := range registry.Templates {
		if ids[entry.ID] {
			t.Fatalf("duplicate registry id %s", entry.ID)
		}
		ids[entry.ID] = true
		counts[entry.Category]++
		if entry.Path != filepath.ToSlash(filepath.Join(entry.ID, "template.yaml")) {
			t.Fatalf("registry path for %s is not flat id/template.yaml: %s", entry.ID, entry.Path)
		}
		if _, err := os.Stat(filepath.Join(templateDir, filepath.FromSlash(entry.Path))); err != nil {
			t.Fatalf("registry path missing for %s: %v", entry.ID, err)
		}
		for _, alias := range entry.Aliases {
			if ids[alias] {
				t.Fatalf("alias %s conflicts with canonical id", alias)
			}
			if owner, exists := aliases[alias]; exists && owner != entry.ID {
				t.Fatalf("alias %s owned by both %s and %s", alias, owner, entry.ID)
			}
			aliases[alias] = entry.ID
		}
		var manifest visualTemplateManifest
		raw := mustRead(t, filepath.Join(templateDir, entry.ID, "template.yaml"))
		if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
			t.Fatalf("template.yaml invalid for %s: %v", entry.ID, err)
		}
		if manifest.ID != entry.ID {
			t.Fatalf("%s manifest id mismatch: %#v", entry.ID, manifest)
		}
		if manifest.Category != entry.Category || expectedVisualCategoryCounts[manifest.Category] == 0 {
			t.Fatalf("%s has invalid category: manifest=%s registry=%s", entry.ID, manifest.Category, entry.Category)
		}
		if manifest.InputSchema != "schema.input.json" {
			t.Fatalf("%s input_schema should be schema.input.json, got %s", entry.ID, manifest.InputSchema)
		}
		if manifest.InputSchemaKind != entry.InputSchemaKind || !allowedVisualSchemaKinds[manifest.InputSchemaKind] {
			t.Fatalf("%s invalid schema kind: manifest=%s registry=%s", entry.ID, manifest.InputSchemaKind, entry.InputSchemaKind)
		}
		if manifest.Renderer.Contract != entry.Renderer || !allowedVisualRenderers[manifest.Renderer.Contract] {
			t.Fatalf("%s invalid renderer: manifest=%s registry=%s", entry.ID, manifest.Renderer.Contract, entry.Renderer)
		}
		if strings.TrimSpace(manifest.Layout.Preset) == "" || manifest.Layout.Preset != entry.LayoutPreset {
			t.Fatalf("%s invalid layout preset: manifest=%s registry=%s", entry.ID, manifest.Layout.Preset, entry.LayoutPreset)
		}
		if strings.TrimSpace(manifest.Description) == "" || strings.Contains(strings.ToLower(manifest.Description), "basic example") {
			t.Fatalf("%s has generic/empty description: %q", entry.ID, manifest.Description)
		}
		if !manifest.Offline.Required || !manifest.Offline.ForbidNetwork || manifest.Offline.DataMode != "js-file" {
			t.Fatalf("%s offline contract invalid: %#v", entry.ID, manifest.Offline)
		}
		if !stringSliceContains(manifest.Scripts, "manifest.js") || !stringSliceContains(manifest.Scripts, "data.js") {
			t.Fatalf("%s missing manifest.js/data.js scripts: %#v", entry.ID, manifest.Scripts)
		}
		if !stringSliceContains(manifest.Styles, "assets/runtime/efp-visual-runtime.css") || !stringSliceContains(manifest.Styles, filepath.ToSlash(filepath.Join("assets", "templates", entry.ID, "style.css"))) {
			t.Fatalf("%s missing required styles: %#v", entry.ID, manifest.Styles)
		}
		if len(manifest.Tags) == 0 || len(entry.Tags) == 0 {
			t.Fatalf("%s missing tags", entry.ID)
		}
	}
	if len(aliases) < 10 {
		t.Fatalf("expected at least 10 aliases, got %#v", aliases)
	}
	for category, expected := range expectedVisualCategoryCounts {
		if counts[category] != expected {
			t.Fatalf("category %s expected %d, got %#v", category, expected, counts)
		}
	}
}

func TestVisualAllTemplatesHaveNonGenericMetadata(t *testing.T) {
	templateDir := visualTemplateDir()
	for _, entry := range visualRegistryData(t).Templates {
		t.Run(entry.ID, func(t *testing.T) {
			var manifest visualTemplateManifest
			raw := mustRead(t, filepath.Join(templateDir, entry.ID, "template.yaml"))
			if err := yaml.Unmarshal([]byte(raw), &manifest); err != nil {
				t.Fatalf("template.yaml invalid for %s: %v", entry.ID, err)
			}
			if strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(manifest.Title) == "" {
				t.Fatalf("%s has empty title: registry=%q manifest=%q", entry.ID, entry.Title, manifest.Title)
			}
			assertNonGenericVisualDescription(t, entry.ID, "registry", entry.Description)
			assertNonGenericVisualDescription(t, entry.ID, "template.yaml", manifest.Description)
			if len(entry.Tags) < 3 || len(manifest.Tags) < 3 {
				t.Fatalf("%s needs at least 3 tags: registry=%#v manifest=%#v", entry.ID, entry.Tags, manifest.Tags)
			}
			if strings.TrimSpace(entry.LayoutPreset) == "" || strings.TrimSpace(manifest.Layout.Preset) == "" {
				t.Fatalf("%s is missing layout preset: registry=%q manifest=%q", entry.ID, entry.LayoutPreset, manifest.Layout.Preset)
			}
			if !expectedVisualCategoryExists(entry.Category) || !expectedVisualCategoryExists(manifest.Category) {
				t.Fatalf("%s has invalid category: registry=%q manifest=%q", entry.ID, entry.Category, manifest.Category)
			}

			var schemaDoc map[string]any
			schemaRaw := mustRead(t, filepath.Join(templateDir, entry.ID, "schema.input.json"))
			if err := json.Unmarshal([]byte(schemaRaw), &schemaDoc); err != nil {
				t.Fatalf("schema.input.json invalid for %s: %v", entry.ID, err)
			}
			if _, ok := schemaDoc["json_schema"].(map[string]any); !ok {
				t.Fatalf("%s schema.input.json missing json_schema: %#v", entry.ID, schemaDoc)
			}
			if _, ok := schemaDoc["example"].(map[string]any); !ok {
				t.Fatalf("%s schema.input.json missing example: %#v", entry.ID, schemaDoc)
			}

			var example map[string]any
			exampleRaw := mustRead(t, filepath.Join(templateDir, entry.ID, "examples", "basic.input.json"))
			if err := json.Unmarshal([]byte(exampleRaw), &example); err != nil {
				t.Fatalf("basic.input.json invalid for %s: %v", entry.ID, err)
			}
			title := strings.TrimSpace(fmt.Sprint(example["title"]))
			for _, generic := range []string{"Basic Example", "Example", entry.Title + " Example", manifest.Title + " Example"} {
				if strings.EqualFold(title, strings.TrimSpace(generic)) {
					t.Fatalf("%s has generic example title: %q", entry.ID, title)
				}
			}
			style, err := os.Stat(filepath.Join(templateDir, entry.ID, "style.css"))
			if err != nil || style.IsDir() || style.Size() == 0 {
				t.Fatalf("%s style.css missing or empty: %v", entry.ID, err)
			}
		})
	}
}

func TestVisualTemplateInputSchemaFilesAreDiscoverable(t *testing.T) {
	templateDir := visualTemplateDir()
	for _, id := range visualTemplateIDs(t) {
		t.Run(id, func(t *testing.T) {
			var doc map[string]any
			b, err := os.ReadFile(filepath.Join(templateDir, id, "schema.input.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			if doc["template_id"] != id {
				t.Fatalf("schema.input.json missing template_id: %#v", doc)
			}
			if strings.TrimSpace(doc["input_schema_kind"].(string)) == "" {
				t.Fatalf("schema.input.json missing input_schema_kind: %#v", doc)
			}
			jsonSchema, ok := doc["json_schema"].(map[string]any)
			if !ok || len(jsonSchema) <= 3 {
				t.Fatalf("schema.input.json missing non-stub json_schema: %#v", doc)
			}
			if schemaURI, _ := jsonSchema["$schema"].(string); strings.Contains(schemaURI, "http://") || strings.Contains(schemaURI, "https://") {
				t.Fatalf("schema.input.json uses remote schema uri: %#v", doc)
			}
			if _, ok := doc["example"].(map[string]any); !ok {
				t.Fatalf("schema.input.json missing example: %#v", doc)
			}
			if len(doc) <= 3 {
				t.Fatalf("schema.input.json is still a stub: %#v", doc)
			}
			if _, old := doc["template"]; old {
				t.Fatalf("schema.input.json still uses old template field: %#v", doc)
			}
		})
	}
}

func TestVisualExamplesHaveRequiredShapeAndUniqueContent(t *testing.T) {
	templateDir := visualTemplateDir()
	hashes := map[string]string{}
	for _, entry := range visualRegistryData(t).Templates {
		t.Run(entry.ID, func(t *testing.T) {
			path := filepath.Join(templateDir, entry.ID, "examples", "basic.input.json")
			raw := []byte(mustRead(t, path))
			hashes[hashString(raw)] = entry.ID
			var data map[string]any
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			title, _ := data["title"].(string)
			if strings.TrimSpace(title) == "" || strings.EqualFold(title, "Basic Example") {
				t.Fatalf("%s has generic title: %#v", entry.ID, data["title"])
			}
			switch entry.InputSchemaKind {
			case "graph_v1":
				assertGraphShape(t, data, false)
			case "graph_events_v1":
				assertGraphShape(t, data, true)
			case "timeline_v1":
				events := anySlice(t, data["events"])
				if len(events) < 6 {
					t.Fatalf("timeline example needs at least 6 events, got %d", len(events))
				}
				previousTime := ""
				for i, item := range events {
					m := objectMap(t, item)
					for _, field := range []string{"id", "time", "kind", "label", "status", "summary"} {
						if strings.TrimSpace(fmt.Sprint(m[field])) == "" {
							t.Fatalf("timeline event %d missing %s: %#v", i, field, m)
						}
					}
					currentTime := fmt.Sprint(m["time"])
					if previousTime != "" && currentTime <= previousTime {
						t.Fatalf("timeline event times are not increasing: previous=%s current=%s", previousTime, currentTime)
					}
					previousTime = currentTime
				}
			case "evidence_v1":
				claims := anySlice(t, data["claims"])
				sources := anySlice(t, data["sources"])
				links := anySlice(t, data["links"])
				if len(claims) < 3 || len(sources) < 4 || len(links) < 5 {
					t.Fatalf("evidence example too small: claims=%d sources=%d links=%d", len(claims), len(sources), len(links))
				}
				relations := map[string]bool{}
				for _, item := range links {
					m := objectMap(t, item)
					relations[fmt.Sprint(m["relation"])] = true
				}
				needed := 0
				for _, relation := range []string{"supports", "refutes", "mentions"} {
					if relations[relation] {
						needed++
					}
				}
				if needed < 2 {
					t.Fatalf("evidence links need at least two relation types, got %#v", relations)
				}
			case "matrix_v1":
				items := anySlice(t, data["items"])
				if len(items) < 8 {
					t.Fatalf("matrix example needs at least 8 items, got %d", len(items))
				}
				kinds := map[string]bool{}
				statuses := map[string]bool{}
				hasMetadata := false
				for _, item := range items {
					m := objectMap(t, item)
					if _, ok := m["x"].(float64); !ok {
						t.Fatalf("matrix item missing numeric x: %#v", m)
					}
					if _, ok := m["y"].(float64); !ok {
						t.Fatalf("matrix item missing numeric y: %#v", m)
					}
					kinds[fmt.Sprint(m["kind"])] = true
					statuses[fmt.Sprint(m["status"])] = true
					if _, ok := m["metadata"].(map[string]any); ok {
						hasMetadata = true
					}
				}
				if len(kinds) < 3 || len(statuses) < 2 || !hasMetadata {
					t.Fatalf("matrix example lacks kind/status/metadata variety: kinds=%#v statuses=%#v metadata=%v", kinds, statuses, hasMetadata)
				}
			default:
				t.Fatalf("unsupported schema kind %s", entry.InputSchemaKind)
			}
		})
	}
	if len(hashes) < 190 {
		t.Fatalf("expected at least 190 unique example hashes, got %d", len(hashes))
	}
}

func TestVisualHighValueTemplateSemanticExamples(t *testing.T) {
	templateDir := visualTemplateDir()
	entries := map[string]visualRegistryEntry{}
	for _, entry := range visualRegistryData(t).Templates {
		entries[entry.ID] = entry
	}
	for id, keywords := range highValueTemplateKeywords {
		t.Run(id, func(t *testing.T) {
			entry, ok := entries[id]
			if !ok {
				t.Fatalf("high value template missing from registry: %s", id)
			}
			var manifest visualTemplateManifest
			rawManifest := mustRead(t, filepath.Join(templateDir, id, "template.yaml"))
			if err := yaml.Unmarshal([]byte(rawManifest), &manifest); err != nil {
				t.Fatalf("template.yaml invalid for %s: %v", id, err)
			}
			for _, description := range []string{entry.Description, manifest.Description} {
				lower := strings.ToLower(description)
				if strings.Contains(lower, "visualize ") && strings.Contains(lower, " as a complete offline ") {
					t.Fatalf("%s has generic description: %q", id, description)
				}
			}
			rawExample := mustRead(t, filepath.Join(templateDir, id, "examples", "basic.input.json"))
			var example map[string]any
			if err := json.Unmarshal([]byte(rawExample), &example); err != nil {
				t.Fatalf("example invalid for %s: %v", id, err)
			}
			title := strings.TrimSpace(fmt.Sprint(example["title"]))
			if strings.EqualFold(title, manifest.Title+" Example") || strings.EqualFold(title, "Basic Example") {
				t.Fatalf("%s has generic example title: %q", id, title)
			}
			lowerExample := strings.ToLower(rawExample)
			matched := 0
			for _, keyword := range keywords {
				if strings.Contains(lowerExample, strings.ToLower(keyword)) {
					matched++
				}
			}
			if matched < 3 {
				t.Fatalf("%s example matched %d domain keywords from %#v", id, matched, keywords)
			}
		})
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
			obj := runVisualOK(t, "render", "--template", id, "--template-dir", templateDir, "--input", filepath.Join(templateDir, id, "examples", "basic.input.json"), "--out", out, "--json")
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
			artifact := obj["data"].(map[string]any)["artifact"].(map[string]any)
			if artifact["template_version"] != "1.0.0" {
				t.Fatalf("artifact missing template_version: %#v", artifact)
			}
			if strings.TrimSpace(artifact["title"].(string)) == "" {
				t.Fatalf("artifact missing title: %#v", artifact)
			}
			if artifact["out_dir"] != filepath.ToSlash(out) || artifact["out"] != filepath.ToSlash(out) {
				t.Fatalf("artifact missing out compatibility fields: %#v", artifact)
			}
			if artifact["relative_entrypoint"] != "index.html" || artifact["file_url_safe"] != true || artifact["http_subpath_safe"] != true {
				t.Fatalf("artifact missing compatibility flags: %#v", artifact)
			}
			files := stringSetFromAny(artifact["files"].([]any))
			templateStyle := filepath.ToSlash(filepath.Join("assets", "templates", id, "style.css"))
			for _, rel := range []string{"index.html", "manifest.json", "manifest.js", "data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js", "assets/runtime/efp-visual-runtime.css", templateStyle} {
				if !files[rel] {
					t.Fatalf("artifact files missing %s in %#v", rel, files)
				}
			}
		})
	}
}

func TestVisualRenderArtifactAndInspectOutputContract(t *testing.T) {
	templateDir := visualTemplateDir()
	input := filepath.Join(templateDir, "agent.run_trace", "examples", "basic.input.json")
	out := filepath.Join(t.TempDir(), "artifact")
	rendered := runVisualOK(t, "render", "--template", "agent.run_trace", "--template-dir", templateDir, "--input", input, "--out", out, "--title", "Contract Title", "--json")
	artifact := rendered["data"].(map[string]any)["artifact"].(map[string]any)
	if artifact["template_id"] != "agent.run_trace" || artifact["template_version"] != "1.0.0" || artifact["title"] != "Contract Title" {
		t.Fatalf("unexpected render artifact: %#v", artifact)
	}
	if artifact["relative_entrypoint"] != "index.html" || artifact["file_url_safe"] != true || artifact["http_subpath_safe"] != true {
		t.Fatalf("render artifact missing compatibility fields: %#v", artifact)
	}

	inspected := runVisualOK(t, "inspect-output", "--out", out, "--json")
	inspectData := inspected["data"].(map[string]any)
	inspectArtifact := inspectData["artifact"].(map[string]any)
	if inspectArtifact["out_dir"] != filepath.ToSlash(out) || inspectArtifact["out"] != filepath.ToSlash(out) {
		t.Fatalf("inspect artifact missing out compatibility fields: %#v", inspectArtifact)
	}
	if inspectArtifact["relative_entrypoint"] != "index.html" || inspectArtifact["offline"] != true || inspectArtifact["file_url_safe"] != true || inspectArtifact["http_subpath_safe"] != true {
		t.Fatalf("inspect artifact missing compatibility fields: %#v", inspectArtifact)
	}
	checks := inspectData["checks"].(map[string]any)
	for _, name := range []string{"index_html", "manifest_json", "manifest_js", "data_js", "runtime_js", "runtime_renderers_js", "runtime_css", "offline_scan"} {
		if checks[name] != true {
			t.Fatalf("inspect check %s was not true: %#v", name, checks)
		}
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
	mustWrite(t, filepath.Join(templateDir, "registry.json"), `{"version":2,"templates":[{"id":"bad","version":"1.0.0","category":"agent","path":"bad/template.yaml","title":"Bad","description":"Bad","input_schema_kind":"graph_v1","renderer":"offline.graph.v1","layout_preset":"dag","tags":["bad"],"aliases":[]}]}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "schema.input.json"), `{"schema":"efp.visual.template_input_schema.v1","template_id":"bad","input_schema_kind":"graph_v1","json_schema":{"type":"object","required":["nodes"],"properties":{"nodes":{"type":"array"}}},"example":{"nodes":[{"id":"a"}]}}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "style.css"), `:root { --accent: #fff; }`)
	mustWrite(t, filepath.Join(templateDir, "bad", "examples", "basic.input.json"), `{"nodes":[{"id":"a"}]}`)
	mustWrite(t, filepath.Join(templateDir, "bad", "template.yaml"), `id: bad
version: 1.0.0
category: agent
title: Bad
description: Bad template
input_schema: schema.input.json
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
	assertErrorCode(t, runVisual(t, "template", "doctor", "--template-dir", templateDir, "--json"), "template_doctor_failed")
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
			writeRequiredRuntimeFiles(t, out)

			assertErrorCode(t, runVisual(t, "inspect-output", "--out", out, "--json"), "offline_violation")
		})
	}
}

func TestVisualInspectOutputMissingFilesContract(t *testing.T) {
	out := t.TempDir()
	mustWrite(t, filepath.Join(out, "index.html"), `<!doctype html><script src="data.js"></script>`)
	mustWrite(t, filepath.Join(out, "manifest.json"), `{}`)
	mustWrite(t, filepath.Join(out, "manifest.js"), `window.__EFP_VISUAL_MANIFEST__ = {};`)
	fail := runVisual(t, "inspect-output", "--out", out, "--json")
	assertErrorCode(t, fail, "visual_output_invalid")
	errObj := fail["error"].(map[string]any)
	missing := stringSetFromAny(errObj["missing_files"].([]any))
	for _, rel := range []string{"data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js", "assets/runtime/efp-visual-runtime.css"} {
		if !missing[rel] {
			t.Fatalf("missing_files did not include %s: %#v", rel, missing)
		}
	}
}

func TestVisualInspectOutputAllowsFileURLText(t *testing.T) {
	out := t.TempDir()
	mustWrite(t, filepath.Join(out, "index.html"), `<!doctype html><script src="data.js"></script>`)
	mustWrite(t, filepath.Join(out, "manifest.json"), `{}`)
	mustWrite(t, filepath.Join(out, "manifest.js"), `window.__EFP_VISUAL_MANIFEST__ = {};`)
	mustWrite(t, filepath.Join(out, "data.js"), `window.__EFP_VISUAL_DATA__ = {"u":"file:///tmp/artifact/app.js"};`)
	writeRequiredRuntimeFiles(t, out)

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

func TestVisualBuildAndSmokeScriptsContract(t *testing.T) {
	buildSH := mustRead(t, "../scripts/build.sh")
	for _, token := range []string{"--snapshot", "--os", "--arch", "TARGET_OS", "TARGET_ARCH", "./cmd/visual"} {
		if !strings.Contains(buildSH, token) {
			t.Fatalf("scripts/build.sh missing %s support", token)
		}
	}

	buildPS := mustRead(t, "../scripts/build.ps1")
	for _, token := range []string{"-Snapshot", "-OS", "-Arch", "$TargetOS", "$TargetArch", "./cmd/visual"} {
		if !strings.Contains(buildPS, token) {
			t.Fatalf("scripts/build.ps1 missing %s support", token)
		}
	}

	smokePS := mustRead(t, "../scripts/smoke.ps1")
	for _, token := range []string{"visual commands --json", "visual schema render --json", "visual template categories", "visual template schema", "visual template doctor", "render --template"} {
		if !strings.Contains(smokePS, token) {
			t.Fatalf("scripts/smoke.ps1 missing visual smoke token %s", token)
		}
	}

	testWorkflow := mustRead(t, "../.github/workflows/test.yml")
	for _, token := range []string{"windows-latest", "shell: pwsh", "scripts/smoke.ps1"} {
		if !strings.Contains(testWorkflow, token) {
			t.Fatalf(".github/workflows/test.yml missing Windows smoke token %s", token)
		}
	}
}

func TestVisualTemplateTreeOfflineAndStyles(t *testing.T) {
	forbidden := []string{
		"http://",
		"https://",
		"//",
		"unpkg",
		"cdnjs",
		"jsdelivr",
		"fonts.googleapis.com",
		"fonts.gstatic.com",
		"@import",
		"fetch(",
		"XMLHttpRequest",
		"WebSocket",
		"EventSource",
		"navigator.sendBeacon",
		"import(",
		`<script type="module`,
		`src="/`,
		`href="/`,
	}
	err := filepath.WalkDir(visualTemplateDir(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".js", ".css", ".json", ".yaml", ".yml":
		default:
			return nil
		}
		content := mustRead(t, path)
		lower := strings.ToLower(content)
		for _, token := range forbidden {
			if strings.Contains(lower, strings.ToLower(token)) {
				t.Fatalf("%s contains forbidden offline token %s", path, token)
			}
		}
		if filepath.Base(path) == "style.css" && strings.TrimSpace(content) == "" {
			t.Fatalf("%s is empty", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
	registry := visualRegistryData(t)
	var ids []string
	for _, item := range registry.Templates {
		ids = append(ids, item.ID)
	}
	sort.Strings(ids)
	return ids
}

func visualRegistryData(t *testing.T) visualRegistry {
	t.Helper()
	return visualRegistryDataFromDir(t, visualTemplateDir())
}

func visualRegistryDataFromDir(t *testing.T, templateDir string) visualRegistry {
	t.Helper()
	var registry visualRegistry
	b, err := os.ReadFile(filepath.Join(templateDir, "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &registry); err != nil {
		t.Fatal(err)
	}
	return registry
}

func canonicalTemplateDirsFromRegistry(t *testing.T, registry visualRegistry) map[string]bool {
	t.Helper()
	dirs := map[string]bool{}
	for _, entry := range registry.Templates {
		dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(entry.Path)))
		if dir == "." || strings.TrimSpace(dir) == "" {
			t.Fatalf("registry entry %s has invalid template path: %s", entry.ID, entry.Path)
		}
		dirs[dir] = true
	}
	return dirs
}

func assertNonGenericVisualDescription(t *testing.T, id, source, description string) {
	t.Helper()
	description = strings.TrimSpace(description)
	if description == "" {
		t.Fatalf("%s has empty %s description", id, source)
	}
	for _, pattern := range genericVisualDescriptionPatterns {
		if pattern.MatchString(description) {
			t.Fatalf("%s has generic %s description: %q", id, source, description)
		}
	}
}

func expectedVisualCategoryExists(category string) bool {
	return expectedVisualCategoryCounts[category] > 0
}

func assertGraphShape(t *testing.T, data map[string]any, withEvents bool) {
	t.Helper()
	nodes := anySlice(t, data["nodes"])
	edges := anySlice(t, data["edges"])
	if len(nodes) < 5 || len(edges) < 4 {
		t.Fatalf("graph example too small: nodes=%d edges=%d", len(nodes), len(edges))
	}
	nodeIDs := map[string]bool{}
	kinds := map[string]bool{}
	groups := map[string]bool{}
	statuses := map[string]bool{}
	hasMetadata := false
	for _, item := range nodes {
		m := objectMap(t, item)
		id := fmt.Sprint(m["id"])
		if strings.TrimSpace(id) == "" {
			t.Fatalf("graph node missing id: %#v", m)
		}
		nodeIDs[id] = true
		kinds[fmt.Sprint(m["kind"])] = true
		groups[fmt.Sprint(m["group"])] = true
		statuses[fmt.Sprint(m["status"])] = true
		if _, ok := m["metadata"].(map[string]any); ok {
			hasMetadata = true
		}
	}
	if len(kinds) < 3 || len(groups) < 3 || len(statuses) < 2 || !hasMetadata {
		t.Fatalf("graph example lacks kind/group/status/metadata variety: kinds=%#v groups=%#v statuses=%#v metadata=%v", kinds, groups, statuses, hasMetadata)
	}
	for i, item := range edges {
		m := objectMap(t, item)
		from := fmt.Sprint(m["from"])
		to := fmt.Sprint(m["to"])
		if !nodeIDs[from] || !nodeIDs[to] {
			t.Fatalf("edge %d references unknown node: %#v", i, m)
		}
	}
	if !withEvents {
		return
	}
	events := anySlice(t, data["events"])
	if len(events) < 5 {
		t.Fatalf("graph_events example needs at least 5 events, got %d", len(events))
	}
	eventStatuses := map[string]bool{}
	for i, item := range events {
		m := objectMap(t, item)
		nodeID := fmt.Sprint(m["node_id"])
		if !nodeIDs[nodeID] {
			t.Fatalf("event %d references unknown node_id: %#v", i, m)
		}
		eventStatuses[fmt.Sprint(m["status"])] = true
	}
	requiredStatusVariety := 0
	for _, status := range []string{"running", "success", "error", "warning"} {
		if eventStatuses[status] {
			requiredStatusVariety++
		}
	}
	if requiredStatusVariety < 2 {
		t.Fatalf("graph_events needs at least two key statuses, got %#v", eventStatuses)
	}
}

func anySlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected JSON array, got %#v", value)
	}
	return items
}

func objectMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %#v", value)
	}
	return m
}

func hashString(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:])
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if filepath.ToSlash(filepath.Clean(item)) == filepath.ToSlash(filepath.Clean(want)) {
			return true
		}
	}
	return false
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

func stringSetFromAny(items []any) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		if s, ok := item.(string); ok {
			out[s] = true
		}
	}
	return out
}

func writeRequiredRuntimeFiles(t *testing.T, out string) {
	t.Helper()
	mustWrite(t, filepath.Join(out, "assets", "runtime", "efp-visual-runtime.iife.js"), `window.__EFP_VISUAL_RUNTIME__ = {};`)
	mustWrite(t, filepath.Join(out, "assets", "runtime", "efp-visual-renderers.iife.js"), `window.__EFP_VISUAL_RENDERERS__ = {};`)
	mustWrite(t, filepath.Join(out, "assets", "runtime", "efp-visual-runtime.css"), `:root { color-scheme: light; }`)
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
