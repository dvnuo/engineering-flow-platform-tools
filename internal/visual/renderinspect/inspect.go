package renderinspect

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/visual/authoring"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
	"engineering-flow-platform-tools/internal/visual/plan"
	"engineering-flow-platform-tools/internal/visual/preview"
	"engineering-flow-platform-tools/internal/visual/render"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
)

type Options struct {
	TemplateDir   string
	OutDir        string
	OfflineStrict bool
}

type Result struct {
	Artifact        render.Artifact           `json:"artifact"`
	TemplateID      string                    `json:"template_id"`
	TemplateVersion string                    `json:"template_version"`
	Title           string                    `json:"title,omitempty"`
	Renderer        string                    `json:"renderer"`
	InputSummary    visualschema.InputSummary `json:"input_summary"`
	QualityScore    int                       `json:"quality_score"`
	RenderScore     int                       `json:"render_score"`
	Ready           bool                      `json:"ready"`
	Checks          Checks                    `json:"checks"`
	Warnings        []preview.Warning         `json:"warnings"`
	VisualPlan      plan.VisualPlan           `json:"visual_plan"`
	NextActions     []Action                  `json:"next_actions"`
}

type Checks struct {
	OutputFiles                        bool `json:"output_files"`
	OfflineScan                        bool `json:"offline_scan"`
	ManifestJSON                       bool `json:"manifest_json"`
	ManifestJS                         bool `json:"manifest_js"`
	DataJS                             bool `json:"data_js"`
	RuntimeAssets                      bool `json:"runtime_assets"`
	ThreeAsset                         bool `json:"three_asset"`
	RendererContractMatch              bool `json:"renderer_contract_match"`
	TemplateVersionMatch               bool `json:"template_version_match"`
	PlanReady                          bool `json:"plan_ready"`
	FocusDeclared                      bool `json:"focus_declared"`
	FirstViewObjectsWithinBudget       bool `json:"first_view_objects_within_budget"`
	FirstViewRelationshipsWithinBudget bool `json:"first_view_relationships_within_budget"`
	LabelsBounded                      bool `json:"labels_bounded"`
	RelationshipsVisible               bool `json:"relationships_visible"`
}

type Action struct {
	Step   string `json:"step"`
	Reason string `json:"reason,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

func Inspect(opts Options) (Result, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return Result{}, metadata.NewError("output_path_invalid", "visual inspect-render requires --out.", "Pass an existing visual render output directory.", 400)
	}
	outputInspection, err := render.InspectOutput(opts.OutDir, opts.OfflineStrict)
	if err != nil {
		return Result{}, err
	}
	outputManifest, err := readOutputManifest(opts.OutDir)
	if err != nil {
		return Result{}, err
	}
	data, err := readDataJS(opts.OutDir)
	if err != nil {
		return Result{}, err
	}
	registry, err := manifest.LoadRegistry(opts.TemplateDir)
	if err != nil {
		return Result{}, err
	}
	entry, _, ok := registry.Resolve(outputManifest.Template.ID)
	if !ok {
		return Result{}, metadata.NewError("template_not_found", "visual render output references an unknown template: "+outputManifest.Template.ID, "Pass --template-dir for the template catalog used to render this artifact.", 404)
	}
	tpl, err := manifest.LoadTemplateManifest(opts.TemplateDir, entry)
	if err != nil {
		return Result{}, err
	}
	if err := manifest.ValidateTemplateManifest(opts.TemplateDir, entry, &tpl); err != nil {
		return Result{}, err
	}
	rules, _, _, err := authoring.LoadQualityRules(opts.TemplateDir, entry)
	if err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return Result{}, metadata.NewError("visual_output_invalid", "visual data.js could not be re-encoded: "+err.Error(), "Inspect the generated data.js file.", 400)
	}
	parsed, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits)
	if err != nil {
		return Result{}, err
	}
	quality, summary, warnings, recommendations := preview.Analyze(tpl, parsed.Data, rules)
	warnings = normalizeWarnings(warnings)
	visualPlan := plan.Build(tpl, parsed.Data, summary, warnings, recommendations, opts.OutDir)
	checks := buildChecks(outputInspection, outputManifest, tpl, visualPlan, summary)
	renderWarnings := inspectRenderWarnings(checks, outputManifest, tpl, visualPlan, summary)
	warnings = append(warnings, renderWarnings...)
	warnings = normalizeWarnings(warnings)
	renderScore := scoreRender(quality, warnings)
	ready := renderScore >= 70 && checks.AllCriticalOK() && !hasErrorWarnings(warnings)
	return Result{
		Artifact:        outputInspection.Artifact,
		TemplateID:      tpl.ID,
		TemplateVersion: tpl.Version,
		Title:           outputManifest.Title,
		Renderer:        tpl.Renderer.Contract,
		InputSummary:    parsed.Summary,
		QualityScore:    quality,
		RenderScore:     renderScore,
		Ready:           ready,
		Checks:          checks,
		Warnings:        warnings,
		VisualPlan:      visualPlan,
		NextActions:     nextActions(ready, warnings),
	}, nil
}

func (c Checks) AllCriticalOK() bool {
	return c.OutputFiles && c.ManifestJSON && c.ManifestJS && c.DataJS && c.RuntimeAssets && c.ThreeAsset && c.RendererContractMatch && c.TemplateVersionMatch && c.PlanReady && c.FirstViewObjectsWithinBudget && c.FirstViewRelationshipsWithinBudget && c.LabelsBounded && c.RelationshipsVisible
}

func readOutputManifest(outDir string) (manifest.OutputManifest, error) {
	var out manifest.OutputManifest
	path := filepath.Join(outDir, "manifest.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return out, metadata.NewError("visual_output_invalid", "visual output manifest.json could not be read: "+err.Error(), "Run visual render again or inspect output permissions.", 400)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, metadata.NewError("visual_output_invalid", "visual output manifest.json is invalid JSON: "+err.Error(), "Run visual render again or inspect manifest.json.", 400)
	}
	if out.Schema != "efp.visual.output.manifest.v1" || strings.TrimSpace(out.Template.ID) == "" {
		return out, metadata.NewError("visual_output_invalid", "visual output manifest.json does not contain a valid visual output manifest.", "Run visual render again and inspect manifest.json.", 400)
	}
	return out, nil
}

func readDataJS(outDir string) (map[string]any, error) {
	path := filepath.Join(outDir, "data.js")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("visual_output_invalid", "visual output data.js could not be read: "+err.Error(), "Run visual render again or inspect output permissions.", 400)
	}
	return parseJSAssignment(b, "__EFP_VISUAL_DATA__", "data.js")
}

func parseJSAssignment(b []byte, variable, file string) (map[string]any, error) {
	content := strings.TrimSpace(string(b))
	prefix := "window." + variable + " = "
	if !strings.HasPrefix(content, prefix) {
		return nil, metadata.NewError("visual_output_invalid", file+" does not assign window."+variable+".", "Run visual render again; generated data files must use local JS assignments.", 400)
	}
	content = strings.TrimSpace(strings.TrimPrefix(content, prefix))
	content = strings.TrimSuffix(content, ";")
	content = strings.TrimSpace(content)
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(content)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, metadata.NewError("visual_output_invalid", file+" contains invalid JSON assignment: "+err.Error(), "Run visual render again or inspect data.js.", 400)
	}
	if out == nil {
		return nil, metadata.NewError("visual_output_invalid", file+" does not contain a JSON object.", "Visual data must be a JSON object.", 400)
	}
	return out, nil
}

func buildChecks(outputInspection render.Inspection, outputManifest manifest.OutputManifest, tpl manifest.TemplateManifest, visualPlan plan.VisualPlan, summary preview.Summary) Checks {
	files := set(outputInspection.Artifact.Files)
	checks := Checks{
		OutputFiles:                        outputInspection.Checks.IndexHTML && outputInspection.Checks.ManifestJSON && outputInspection.Checks.ManifestJS && outputInspection.Checks.DataJS && outputInspection.Checks.RuntimeJS && outputInspection.Checks.RuntimeRenderersJS && outputInspection.Checks.RuntimeCSS,
		OfflineScan:                        outputInspection.Checks.OfflineScan,
		ManifestJSON:                       outputInspection.Checks.ManifestJSON,
		ManifestJS:                         outputInspection.Checks.ManifestJS,
		DataJS:                             outputInspection.Checks.DataJS,
		RuntimeAssets:                      files["assets/runtime/efp-visual-runtime.iife.js"] && files["assets/runtime/efp-visual-renderers.iife.js"] && files["assets/runtime/efp-visual-runtime.css"],
		ThreeAsset:                         true,
		RendererContractMatch:              outputManifest.Renderer.Contract == tpl.Renderer.Contract,
		TemplateVersionMatch:               outputManifest.Template.Version == tpl.Version,
		PlanReady:                          len(visualPlan.QualityLoop) == 0,
		FocusDeclared:                      focusDeclared(visualPlan),
		FirstViewObjectsWithinBudget:       withinBudget(len(visualPlan.View.OverviewObjectIDs), visualPlan.View.MaxInitialObjects),
		FirstViewRelationshipsWithinBudget: withinBudget(len(visualPlan.View.OverviewRelationshipIDs), visualPlan.View.MaxInitialRelationships),
		LabelsBounded:                      labelsBounded(visualPlan, summary),
		RelationshipsVisible:               relationshipsVisible(visualPlan),
	}
	if tpl.Effects.Engine == "three.v1" {
		checks.ThreeAsset = files["assets/vendor/three/efp-three.module.min.js"]
	}
	return checks
}

func inspectRenderWarnings(checks Checks, outputManifest manifest.OutputManifest, tpl manifest.TemplateManifest, visualPlan plan.VisualPlan, summary preview.Summary) []preview.Warning {
	var warnings []preview.Warning
	add := func(code, severity, message, suggestion string) {
		warnings = append(warnings, preview.Warning{Code: code, Severity: severity, Message: message, Suggestion: suggestion, AutoFixHint: map[string]any{"action": code}})
	}
	if !checks.RendererContractMatch {
		add("renderer_contract_mismatch", "error", "Rendered manifest renderer does not match the current template renderer.", "Re-render with the matching template catalog or inspect the release artifact contents.")
	}
	if !checks.TemplateVersionMatch {
		add("template_version_mismatch", "warning", "Rendered manifest template version differs from the current template version.", "Re-render if you need to inspect against the current template behavior.")
	}
	if !checks.ThreeAsset {
		add("three_asset_missing", "error", "Rendered artifact is missing the local Three.js vendor asset required by this template.", "Run visual render again and ensure templates/visual/_shared/vendor/three is present.")
	}
	if !checks.PlanReady {
		add("render_plan_not_ready", "warning", "The rendered input still has quality warnings in its visual plan.", "Run visual inspect-plan, apply visual_plan.quality_loop fixes, and render again.")
	}
	if !checks.FocusDeclared && visualPlan.IR.Counts["objects"] > 10 {
		add("render_focus_missing", "warning", "Rendered input has many objects but no declared first-view focus ids.", "Add visual.initial_focus_ids and visual.narrative_steps so the renderer does not show everything as equal priority.")
	}
	if !checks.FirstViewObjectsWithinBudget {
		add("first_view_objects_over_budget", "warning", "The planned first view contains more objects than the template budget.", "Reduce overview objects, add grouping, or mark low-value objects as hidden detail.")
	}
	if !checks.FirstViewRelationshipsWithinBudget {
		add("first_view_relationships_over_budget", "warning", "The planned first view contains more relationships than the template budget.", "Mark noisy relationships with visibility detail/hidden and keep only narrative edges in the overview.")
	}
	if !checks.LabelsBounded {
		add("render_label_pressure_high", "warning", "The rendered plan exposes too many labels for a readable first view.", "Use shorter labels, label_priority, importance, and hover/detail label modes.")
	}
	if !checks.RelationshipsVisible {
		add("render_relationships_not_visible", "warning", "The rendered plan does not expose relationships in the overview even though the template expects connected objects.", "Add or keep meaningful relationships visible so the viewer can understand why objects are connected.")
	}
	_ = outputManifest
	_ = tpl
	return warnings
}

func normalizeWarnings(warnings []preview.Warning) []preview.Warning {
	for i := range warnings {
		if warnings[i].Severity == "" {
			warnings[i].Severity = "warning"
		}
		if warnings[i].Suggestion == "" {
			warnings[i].Suggestion = warnings[i].Hint
		}
		if warnings[i].AutoFixHint == nil && warnings[i].Code != "" {
			warnings[i].AutoFixHint = map[string]any{"action": warnings[i].Code}
		}
	}
	return warnings
}

func scoreRender(quality int, warnings []preview.Warning) int {
	score := quality
	for _, warning := range warnings {
		switch strings.ToLower(warning.Severity) {
		case "error":
			score -= 35
		case "warning":
			score -= 4
		}
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func hasErrorWarnings(warnings []preview.Warning) bool {
	for _, warning := range warnings {
		if strings.ToLower(warning.Severity) == "error" {
			return true
		}
	}
	return false
}

func nextActions(ready bool, warnings []preview.Warning) []Action {
	if ready {
		return []Action{{Step: "return_entrypoint", Reason: "Rendered artifact passed structural, offline, and pre-render readability checks."}}
	}
	var codes []string
	seen := map[string]bool{}
	for _, warning := range warnings {
		if warning.Code != "" && !seen[warning.Code] {
			seen[warning.Code] = true
			codes = append(codes, warning.Code)
		}
	}
	sort.Strings(codes)
	return []Action{
		{Step: "revise_input", Reason: "inspect-render found readability or artifact warnings.", Hint: strings.Join(codes, ", ")},
		{Step: "rerun_inspect_plan", Reason: "Confirm the semantic plan is ready before rendering again."},
		{Step: "render_again", Reason: "Regenerate the offline artifact after the input is fixed."},
	}
}

func withinBudget(count, budget int) bool {
	if budget <= 0 {
		return true
	}
	return count <= budget
}

func labelsBounded(visualPlan plan.VisualPlan, summary preview.Summary) bool {
	if summary.LabelPressure == "high" {
		return false
	}
	if len(summary.LongLabels) > 0 || len(summary.DuplicateLabels) > 0 || summary.MissingLabels > 0 {
		return false
	}
	visibleLabels := len(visualPlan.Labels.AlwaysIDs) + len(visualPlan.Labels.ImportantIDs) + len(visualPlan.Labels.NormalIDs)
	budget := visualPlan.View.MaxInitialObjects
	if budget <= 0 {
		budget = 60
	}
	return visibleLabels <= budget
}

func focusDeclared(visualPlan plan.VisualPlan) bool {
	return len(visualPlan.View.InitialFocusIDs) > 0 || len(visualPlan.View.OverviewObjectIDs) <= 10
}

func relationshipsVisible(visualPlan plan.VisualPlan) bool {
	relationships := visualPlan.IR.Counts["relationships"]
	if relationships == 0 {
		return !relationshipExpected(visualPlan.InputSchemaKind)
	}
	return len(visualPlan.View.OverviewRelationshipIDs) > 0
}

func relationshipExpected(kind string) bool {
	switch strings.ToLower(kind) {
	case "matrix_v1", "timeline_v1":
		return false
	default:
		return true
	}
}

func set(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}
