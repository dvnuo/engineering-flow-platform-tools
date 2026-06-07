package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

func ValidateTemplateManifest(templateDir string, entry RegistryEntry, m *TemplateManifest) error {
	if m == nil {
		return metadata.NewError("template_manifest_invalid", "visual template manifest is empty.", "Provide a complete template.yaml.", 400)
	}
	if strings.TrimSpace(m.ID) != entry.ID {
		return metadata.NewError("template_manifest_invalid", "visual template manifest id does not match registry id.", "Set template.yaml id to "+entry.ID+".", 400)
	}
	if filepath.Base(filepath.Dir(filepath.ToSlash(filepath.Clean(entry.Path)))) != entry.ID {
		return metadata.NewError("template_manifest_invalid", "visual template registry path directory must equal template id.", "Use "+entry.ID+"/template.yaml as the registry path.", 400)
	}
	if strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.Category) == "" || strings.TrimSpace(m.Title) == "" || strings.TrimSpace(m.Description) == "" || strings.TrimSpace(m.InputSchema) == "" || strings.TrimSpace(m.InputSchemaKind) == "" || strings.TrimSpace(m.Renderer.Contract) == "" || strings.TrimSpace(m.Layout.Preset) == "" {
		return metadata.NewError("template_manifest_invalid", "visual template manifest is missing required fields.", "Set version, category, title, description, input_schema, input_schema_kind, renderer.contract, and layout.preset.", 400)
	}
	m.Category = normalizeManifestValue(m.Category)
	if !SupportedCategories[m.Category] {
		return metadata.NewError("template_manifest_invalid", "visual template category is not supported: "+m.Category, "Use one of: "+strings.Join(SupportedCategoryOrder, ", ")+".", 400)
	}
	if strings.TrimSpace(entry.Category) != "" && normalizeManifestValue(entry.Category) != m.Category {
		return metadata.NewError("template_manifest_invalid", "visual template category does not match registry category.", "Set template.yaml category and registry category to "+m.Category+".", 400)
	}
	if filepath.ToSlash(filepath.Clean(m.InputSchema)) != "schema.input.json" {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema must be schema.input.json.", "Set input_schema: schema.input.json.", 400)
	}
	if !metadata.SupportedRenderers[m.Renderer.Contract] {
		return metadata.NewError("unsupported_renderer", "visual template renderer is not supported: "+m.Renderer.Contract, "Use a supported offline visual renderer contract.", 400)
	}
	if !SupportedInputSchemaKinds[normalizeInputSchemaKind(m.InputSchemaKind)] {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema_kind is not supported: "+m.InputSchemaKind, "Use a supported semantic visual input schema kind.", 400)
	}
	m.InputSchemaKind = normalizeInputSchemaKind(m.InputSchemaKind)
	if strings.TrimSpace(entry.InputSchemaKind) != "" && normalizeInputSchemaKind(entry.InputSchemaKind) != m.InputSchemaKind {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema_kind does not match registry entry.", "Set registry input_schema_kind to "+m.InputSchemaKind+".", 400)
	}
	if strings.TrimSpace(entry.Renderer) != "" && strings.TrimSpace(entry.Renderer) != m.Renderer.Contract {
		return metadata.NewError("template_manifest_invalid", "visual template renderer does not match registry entry.", "Set registry renderer to "+m.Renderer.Contract+".", 400)
	}
	m.Layout.Preset = normalizeManifestValue(m.Layout.Preset)
	if !SupportedLayoutPresets[m.Layout.Preset] {
		return metadata.NewError("template_manifest_invalid", "visual template layout.preset is not supported: "+m.Layout.Preset, "Use a supported offline layout preset.", 400)
	}
	if strings.TrimSpace(entry.LayoutPreset) != "" && normalizeManifestValue(entry.LayoutPreset) != m.Layout.Preset {
		return metadata.NewError("template_manifest_invalid", "visual template layout.preset does not match registry entry.", "Set registry layout_preset to "+m.Layout.Preset+".", 400)
	}
	if err := validateEffectsSpec(&m.Effects); err != nil {
		return err
	}
	normalizeVisualDesign(&m.VisualDesign)
	if err := validateInputSchemaFile(templateDir, entry, m.InputSchema); err != nil {
		return err
	}
	if !m.Offline.Required {
		return metadata.NewError("template_manifest_invalid", "visual template offline.required must be true.", "Set offline.required: true.", 400)
	}
	if !m.Offline.ForbidNetwork {
		return metadata.NewError("template_manifest_invalid", "visual template offline.forbid_network must be true.", "Set offline.forbid_network: true.", 400)
	}
	mode := normalizeDataMode(m.Offline.DataMode)
	if mode != "js-file" {
		return metadata.NewError("unsupported_data_mode", "visual template data mode is not supported: "+m.Offline.DataMode, "Use offline.data_mode: js-file.", 400)
	}
	m.Offline.DataMode = mode
	if m.Limits.MaxNodes == 0 {
		m.Limits.MaxNodes = 1000
	}
	if m.Limits.MaxEdges == 0 {
		m.Limits.MaxEdges = 3000
	}
	if m.Limits.MaxEvents == 0 {
		m.Limits.MaxEvents = 5000
	}
	if m.Limits.MaxItems == 0 {
		m.Limits.MaxItems = 2000
	}
	for _, asset := range m.Assets {
		if err := validateAsset(templateDir, entry.Path, asset); err != nil {
			return err
		}
	}
	for _, style := range m.Styles {
		if err := validateRelativeReference("style", style); err != nil {
			return err
		}
	}
	for _, script := range m.Scripts {
		if err := validateRelativeReference("script", script); err != nil {
			return err
		}
	}
	for _, required := range []string{"manifest.js", "data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js"} {
		if !containsSlashPath(m.Scripts, required) {
			return metadata.NewError("template_manifest_invalid", "visual template scripts are missing "+required+".", "Include manifest.js, data.js, and the shared runtime scripts.", 400)
		}
	}
	if !containsSlashPath(m.Styles, "assets/runtime/efp-visual-runtime.css") {
		return metadata.NewError("template_manifest_invalid", "visual template styles are missing assets/runtime/efp-visual-runtime.css.", "Include the shared runtime CSS.", 400)
	}
	templateStyle := "assets/templates/" + m.ID + "/style.css"
	if !containsSlashPath(m.Styles, templateStyle) && !containsSlashPath(m.Styles, "assets/template/style.css") {
		return metadata.NewError("template_manifest_invalid", "visual template styles are missing template style.css.", "Include "+templateStyle+".", 400)
	}
	return nil
}

func ValidateRegistry(r Registry) error {
	ids := map[string]bool{}
	for _, entry := range r.Templates {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return metadata.NewError("template_registry_invalid", "visual template registry contains an empty id.", "Set every registry template id.", 400)
		}
		if ids[id] {
			return metadata.NewError("template_registry_invalid", "visual template registry contains duplicate id: "+id, "Remove duplicate canonical template ids.", 400)
		}
		ids[id] = true
		if !SupportedCategories[normalizeManifestValue(entry.Category)] {
			return metadata.NewError("template_registry_invalid", "visual template registry contains unsupported category for "+id+": "+entry.Category, "Use one of: "+strings.Join(SupportedCategoryOrder, ", ")+".", 400)
		}
	}
	aliases := map[string]string{}
	for _, entry := range r.Templates {
		id := strings.TrimSpace(entry.ID)
		for _, alias := range entry.Aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				return metadata.NewError("template_registry_invalid", "visual template registry contains an empty alias for "+id+".", "Remove empty aliases.", 400)
			}
			if ids[alias] {
				return metadata.NewError("template_registry_invalid", "visual template alias conflicts with canonical id: "+alias, "Aliases must not match canonical ids.", 400)
			}
			if owner, ok := aliases[alias]; ok && owner != id {
				return metadata.NewError("template_registry_invalid", "visual template alias is duplicated: "+alias, "Alias "+alias+" is already assigned to "+owner+".", 400)
			}
			aliases[alias] = id
		}
	}
	return nil
}

func ValidateExpectedCategoryCounts(counts, expected map[string]int) error {
	for _, category := range SupportedCategoryOrder {
		want, ok := expected[category]
		if !ok {
			return metadata.NewError("template_registry_invalid", "visual template registry expected category count is missing for "+category+".", "Add registry.expected.categories."+category+" to templates/visual/registry.json.", 400)
		}
		if counts[category] != want {
			return metadata.NewError("template_registry_invalid", "visual template registry expected category count mismatch for "+category+".", "Expected "+category+"="+itoa(want)+", got "+itoa(counts[category])+".", 400)
		}
	}
	for category := range expected {
		if !SupportedCategories[category] {
			return metadata.NewError("template_registry_invalid", "visual template registry expected category is not supported: "+category, "Use one of: "+strings.Join(SupportedCategoryOrder, ", ")+".", 400)
		}
	}
	for category, count := range counts {
		if count > 0 && !SupportedCategories[category] {
			return metadata.NewError("template_registry_invalid", "visual template category is not supported: "+category, "Use one of: "+strings.Join(SupportedCategoryOrder, ", ")+".", 400)
		}
	}
	return nil
}

func SortedCategoryCounts(counts map[string]int) []CategoryCount {
	out := make([]CategoryCount, 0, len(SupportedCategoryOrder))
	for _, category := range SupportedCategoryOrder {
		if count := counts[category]; count > 0 {
			out = append(out, CategoryCount{ID: category, Count: count})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type CategoryCount struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

var SupportedInputSchemaKinds = map[string]bool{
	"graph_v1":                    true,
	"graph_events_v1":             true,
	"timeline_v1":                 true,
	"evidence_v1":                 true,
	"matrix_v1":                   true,
	"studio_v1":                   true,
	"uml_sequence_v1":             true,
	"uml_class_v1":                true,
	"uml_state_machine_v1":        true,
	"uml_activity_v1":             true,
	"uml_component_deployment_v1": true,
}

var SupportedCategoryOrder = []string{
	"uml",
	"relationship",
	"temporal",
	"flow",
	"hierarchy",
	"evidence",
	"matrix",
	"spatial",
	"studio",
}

var SupportedCategories = map[string]bool{
	"uml":          true,
	"relationship": true,
	"temporal":     true,
	"flow":         true,
	"hierarchy":    true,
	"evidence":     true,
	"matrix":       true,
	"spatial":      true,
	"studio":       true,
}

var SupportedEffectEngines = map[string]bool{
	"three.v1": true,
}

const DefaultExpectedCanonicalCount = 37

var ExpectedCategoryCounts = map[string]int{
	"uml":          5,
	"relationship": 4,
	"temporal":     4,
	"flow":         4,
	"hierarchy":    4,
	"evidence":     4,
	"matrix":       4,
	"spatial":      4,
	"studio":       4,
}

var SupportedLayoutPresets = map[string]bool{
	"graph_3d":                    true,
	"graph_2_5d":                  true,
	"timeline_tunnel":             true,
	"swimlane_timeline":           true,
	"radial_tree":                 true,
	"layered_stack":               true,
	"pipeline_flow":               true,
	"constellation":               true,
	"city_map":                    true,
	"terrain_heatmap":             true,
	"matrix_board":                true,
	"sankey_3d":                   true,
	"radar_sphere":                true,
	"diff_split_view":             true,
	"replay_stage":                true,
	"orbit_system":                true,
	"control_room":                true,
	"document_wall":               true,
	"flow_particles":              true,
	"state_machine":               true,
	"dag":                         true,
	"galaxy":                      true,
	"ripple":                      true,
	"service_map":                 true,
	"fleet":                       true,
	"incident_timeline":           true,
	"evidence_board":              true,
	"knowledge_graph":             true,
	"decision_matrix":             true,
	"kanban":                      true,
	"gantt":                       true,
	"roadmap":                     true,
	"journey":                     true,
	"funnel":                      true,
	"radar":                       true,
	"waterfall":                   true,
	"heatmap":                     true,
	"tree":                        true,
	"river":                       true,
	"board":                       true,
	"network_boundary_map":        true,
	"permission_gate":             true,
	"step_ladder":                 true,
	"line":                        true,
	"citation_map":                true,
	"sequence_lifelines":          true,
	"class_cards":                 true,
	"activity_swimlanes":          true,
	"component_deployment":        true,
	"studio_pipeline":             true,
	"studio_topology":             true,
	"studio_sequence_walkthrough": true,
	"studio_entity_explorer":      true,
	"studio_page":                 true,
}

func normalizeInputSchemaKind(kind string) string {
	return strings.TrimSpace(strings.ToLower(kind))
}

func normalizeManifestValue(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeDataMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "js_file", "js-file":
		return "js-file"
	default:
		return strings.TrimSpace(strings.ToLower(mode))
	}
}

func validateInputSchemaFile(templateDir string, entry RegistryEntry, inputSchema string) error {
	rel, path, err := resolveTemplateInputSchemaPath(templateDir, entry, inputSchema)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema file was not found: "+rel, "Set input_schema to an existing JSON file such as schema.input.json.", 400)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema file could not be read: "+rel, "Check file permissions for "+rel+".", 400)
	}
	if !json.Valid(b) {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema file is not valid JSON: "+rel, "Fix "+rel+" so it contains a JSON object.", 400)
	}
	return nil
}

func resolveTemplateInputSchemaPath(templateDir string, entry RegistryEntry, inputSchema string) (string, string, error) {
	value := strings.TrimSpace(inputSchema)
	if value == "" {
		return "", "", metadata.NewError("template_manifest_invalid", "visual template input_schema is empty.", "Set input_schema: schema.input.json.", 400)
	}
	if filepath.IsAbs(value) {
		return "", "", metadata.NewError("template_manifest_invalid", "visual template input_schema must be relative: "+value, "Use a relative JSON path such as schema.input.json.", 400)
	}
	if containsParentPathSegment(value) {
		return "", "", metadata.NewError("template_manifest_invalid", "visual template input_schema must not contain parent traversal: "+value, "Keep input_schema inside the template directory.", 400)
	}
	clean := filepath.Clean(value)
	if clean == "." {
		return "", "", metadata.NewError("template_manifest_invalid", "visual template input_schema path is invalid.", "Set input_schema: schema.input.json.", 400)
	}
	base := TemplateBaseDir(templateDir, entry)
	path := filepath.Join(base, clean)
	rel := filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Clean(entry.Path)), clean))
	return rel, path, nil
}

func containsParentPathSegment(value string) bool {
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

func TemplateBaseDir(templateDir string, entry RegistryEntry) string {
	return filepath.Dir(filepath.Join(templateDir, filepath.Clean(entry.Path)))
}

func validateEffectsSpec(effects *EffectsSpec) error {
	if effects == nil || strings.TrimSpace(effects.Engine) == "" {
		return nil
	}
	effects.Engine = normalizeManifestValue(effects.Engine)
	if !SupportedEffectEngines[effects.Engine] {
		return metadata.NewError("template_manifest_invalid", "visual template effects.engine is not supported: "+effects.Engine, "Use three.v1 for local Three.js effects.", 400)
	}
	effects.Scene = normalizeManifestValue(effects.Scene)
	if effects.Scene == "" {
		return metadata.NewError("template_manifest_invalid", "visual template effects.scene is required when effects.engine is set.", "Set a scene-specific effect id such as runtime_event_bus_flow.", 400)
	}
	effects.Camera = normalizeManifestValue(effects.Camera)
	effects.Particles = normalizeManifestValue(effects.Particles)
	effects.Material = normalizeManifestValue(effects.Material)
	effects.Motion = normalizeManifestValue(effects.Motion)
	for i, value := range effects.Interaction {
		effects.Interaction[i] = normalizeManifestValue(value)
	}
	for i, value := range effects.Postprocess {
		effects.Postprocess[i] = normalizeManifestValue(value)
	}
	return nil
}

func normalizeVisualDesign(design *VisualDesign) {
	if design == nil {
		return
	}
	design.InitialView = normalizeManifestValue(design.InitialView)
	if design.InitialView == "" {
		design.InitialView = "overview"
	}
	if design.MaxInitialNodes <= 0 {
		design.MaxInitialNodes = 60
	}
	if design.MaxInitialEdges <= 0 {
		design.MaxInitialEdges = 120
	}
	if design.DefaultCollapseDepth < 0 {
		design.DefaultCollapseDepth = 0
	}
	for i, value := range design.GroupBy {
		design.GroupBy[i] = normalizeManifestValue(value)
	}
	for i, value := range design.Supports {
		design.Supports[i] = normalizeManifestValue(value)
	}
}

func validateAsset(templateDir, templatePath string, asset AssetSpec) error {
	if strings.TrimSpace(asset.From) == "" {
		return metadata.NewError("template_asset_missing", "visual template asset.from is empty.", "Set asset.from to a file under templates/visual.", 400)
	}
	if err := validateOfflineReference("asset source", asset.From); err != nil {
		return err
	}
	if err := validateRelativeReference("asset target", asset.To); err != nil {
		return err
	}
	rootAbs, _ := filepath.Abs(templateDir)
	currentAbs, _ := filepath.Abs(filepath.Join(templateDir, filepath.Dir(filepath.Clean(templatePath))))
	candidate := filepath.Clean(filepath.Join(currentAbs, asset.From))
	candidateAbs, _ := filepath.Abs(candidate)
	if !withinPath(rootAbs, candidateAbs) {
		return metadata.NewError("template_asset_outside_root", "visual template asset escapes template root: "+asset.From, "Keep asset.from under templates/visual, using ../_shared only inside that root.", 400)
	}
	info, err := os.Stat(candidateAbs)
	if err != nil || info.IsDir() {
		return metadata.NewError("template_asset_missing", "visual template asset was not found: "+asset.From, "Ensure every asset.from exists and is a file.", 404)
	}
	return nil
}

func validateRelativeReference(kind, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path is empty.", "Use a relative path without parent traversal.", 400)
	}
	if err := validateOfflineReference(kind, value); err != nil {
		return err
	}
	if filepath.IsAbs(value) {
		return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path must be relative: "+value, "Use relative paths so artifacts work under file:// and proxy subpaths.", 400)
	}
	clean := filepath.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path is unsafe: "+value, "Remove parent traversal from the path.", 400)
	}
	return nil
}

func validateOfflineReference(kind, value string) error {
	lower := strings.ToLower(value)
	for _, token := range []string{"http://", "https://", "//", "unpkg", "cdnjs", "jsdelivr", "fonts.googleapis.com", "fonts.gstatic.com"} {
		if strings.Contains(lower, token) {
			return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path contains forbidden remote token: "+token, "Use local relative paths only.", 400)
		}
	}
	return nil
}

func containsSlashPath(items []string, required string) bool {
	for _, item := range items {
		if filepath.ToSlash(filepath.Clean(item)) == required {
			return true
		}
	}
	return false
}

func withinPath(rootAbs, candidateAbs string) bool {
	rootAbs = filepath.Clean(rootAbs)
	candidateAbs = filepath.Clean(candidateAbs)
	if rootAbs == candidateAbs {
		return true
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
