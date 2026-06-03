package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.Title) == "" || strings.TrimSpace(m.Description) == "" || strings.TrimSpace(m.InputSchema) == "" || strings.TrimSpace(m.InputSchemaKind) == "" || strings.TrimSpace(m.Renderer.Contract) == "" {
		return metadata.NewError("template_manifest_invalid", "visual template manifest is missing required fields.", "Set version, title, description, input_schema, input_schema_kind, and renderer.contract.", 400)
	}
	if !metadata.SupportedRenderers[m.Renderer.Contract] {
		return metadata.NewError("unsupported_renderer", "visual template renderer is not supported: "+m.Renderer.Contract, "Use offline.graph.v1, offline.timeline.v1, offline.evidence.v1, or offline.matrix.v1.", 400)
	}
	if !SupportedInputSchemaKinds[normalizeInputSchemaKind(m.InputSchemaKind)] {
		return metadata.NewError("template_manifest_invalid", "visual template input_schema_kind is not supported: "+m.InputSchemaKind, "Use graph_v1, graph_events_v1, timeline_v1, evidence_v1, or matrix_v1.", 400)
	}
	m.InputSchemaKind = normalizeInputSchemaKind(m.InputSchemaKind)
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
	for _, required := range []string{"assets/runtime/efp-visual-runtime.css", "assets/template/style.css"} {
		if !containsSlashPath(m.Styles, required) {
			return metadata.NewError("template_manifest_invalid", "visual template styles are missing "+required+".", "Include the shared runtime CSS and template CSS.", 400)
		}
	}
	return nil
}

var SupportedInputSchemaKinds = map[string]bool{
	"graph_v1":        true,
	"graph_events_v1": true,
	"timeline_v1":     true,
	"evidence_v1":     true,
	"matrix_v1":       true,
}

func normalizeInputSchemaKind(kind string) string {
	return strings.TrimSpace(strings.ToLower(kind))
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

func validateAsset(templateDir, templatePath string, asset AssetSpec) error {
	if strings.TrimSpace(asset.From) == "" {
		return metadata.NewError("template_asset_missing", "visual template asset.from is empty.", "Set asset.from to a file under templates/visual.", 400)
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
	if filepath.IsAbs(value) {
		return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path must be relative: "+value, "Use relative paths so artifacts work under file:// and proxy subpaths.", 400)
	}
	clean := filepath.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return metadata.NewError("template_asset_target_invalid", "visual template "+kind+" path is unsafe: "+value, "Remove parent traversal from the path.", 400)
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
