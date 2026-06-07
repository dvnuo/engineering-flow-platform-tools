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
	Screenshot    string
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
	Screenshot      ScreenshotInspection      `json:"screenshot,omitempty"`
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
	ShapeDiversity                     bool `json:"shape_diversity"`
	ArrowsVisible                      bool `json:"arrows_visible"`
	ColorDiversity                     bool `json:"color_diversity"`
	LegendPresent                      bool `json:"legend_present"`
	IconAssetsPresent                  bool `json:"icon_assets_present"`
	AttributionsPresent                bool `json:"attributions_present"`
	IsometricRendererUsed              bool `json:"isometric_renderer_used"`
	BasePlanePresent                   bool `json:"base_plane_present"`
	GridPresent                        bool `json:"grid_present"`
	ZonesPresent                       bool `json:"zones_present"`
	ZoneBoundariesPresent              bool `json:"zone_boundaries_present"`
	EntitiesPresent                    bool `json:"entities_present"`
	EntityLabelsPresent                bool `json:"entity_labels_present"`
	LeaderLinesPresent                 bool `json:"leader_lines_present"`
	DirectedArrowsPresent              bool `json:"directed_arrows_present"`
	LinkLabelsPresent                  bool `json:"link_labels_present"`
	OrthographicCameraPlanned          bool `json:"orthographic_camera_planned"`
	ArchitectureLightTheme             bool `json:"architecture_light_theme"`
	NoStarfieldTheme                   bool `json:"no_starfield_theme"`
	NoStudioLayout                     bool `json:"no_studio_layout"`
	ArtifactRuntimeWired               bool `json:"artifact_runtime_wired"`
	ArtifactIsometricRuntimeHook       bool `json:"artifact_isometric_runtime_hook"`
	ArtifactIsometricDOMHooks          bool `json:"artifact_isometric_dom_hooks"`
	ArtifactEntityLabelHooks           bool `json:"artifact_entity_label_hooks"`
	ArtifactLinkLabelHooks             bool `json:"artifact_link_label_hooks"`
	ArtifactZoneLabelHooks             bool `json:"artifact_zone_label_hooks"`
	ArtifactBasePlaneHook              bool `json:"artifact_base_plane_hook"`
	ArtifactGridHook                   bool `json:"artifact_grid_hook"`
	ArtifactLeaderLineHook             bool `json:"artifact_leader_line_hook"`
	ArtifactArrowHook                  bool `json:"artifact_arrow_hook"`
	ArtifactNoStudioRuntime            bool `json:"artifact_no_studio_runtime"`
	ArtifactNoStarfieldRuntime         bool `json:"artifact_no_starfield_runtime"`
	ScreenshotReadable                 bool `json:"screenshot_readable"`
	ScreenshotNonBlank                 bool `json:"screenshot_non_blank"`
	ScreenshotContrast                 bool `json:"screenshot_contrast"`
	ScreenshotCoverage                 bool `json:"screenshot_coverage"`
}

type artifactEvidence struct {
	RuntimeWired         bool
	IsometricRuntimeHook bool
	IsometricDOMHooks    bool
	EntityLabelHooks     bool
	LinkLabelHooks       bool
	ZoneLabelHooks       bool
	BasePlaneHook        bool
	GridHook             bool
	LeaderLineHook       bool
	ArrowHook            bool
	NoStudioRuntime      bool
	NoStarfieldRuntime   bool
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
	quality, summary, warnings, recommendations := preview.Analyze(opts.TemplateDir, tpl, parsed.Data, rules)
	warnings = normalizeWarnings(warnings)
	screenshot, screenshotWarnings, err := inspectScreenshot(opts.Screenshot)
	if err != nil {
		return Result{}, err
	}
	visualPlan := plan.Build(opts.TemplateDir, tpl, parsed.Data, summary, warnings, recommendations, opts.OutDir)
	evidence := inspectArtifactEvidence(opts.OutDir, tpl)
	checks := buildChecks(outputInspection, outputManifest, tpl, visualPlan, summary, screenshot, evidence)
	renderWarnings := inspectRenderWarnings(checks, outputManifest, tpl, visualPlan, summary)
	warnings = append(warnings, renderWarnings...)
	warnings = append(warnings, screenshotWarnings...)
	warnings = normalizeWarnings(warnings)
	renderScore := scoreRender(quality, warnings)
	ready := renderScore >= 70 && checks.AllCriticalOK(screenshot.Provided) && !hasErrorWarnings(warnings)
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
		Screenshot:      screenshot,
		NextActions:     nextActions(ready, warnings),
	}, nil
}

func (c Checks) AllCriticalOK(withScreenshot bool) bool {
	base := c.OutputFiles && c.ManifestJSON && c.ManifestJS && c.DataJS && c.RuntimeAssets && c.ThreeAsset && c.RendererContractMatch && c.TemplateVersionMatch && c.PlanReady && c.FirstViewObjectsWithinBudget && c.FirstViewRelationshipsWithinBudget && c.LabelsBounded && c.RelationshipsVisible
	base = base && c.IsometricRendererUsed && c.BasePlanePresent && c.GridPresent && c.ZonesPresent && c.ZoneBoundariesPresent && c.EntitiesPresent && c.EntityLabelsPresent && c.LeaderLinesPresent && c.DirectedArrowsPresent && c.LinkLabelsPresent && c.OrthographicCameraPlanned && c.ArchitectureLightTheme && c.NoStarfieldTheme && c.NoStudioLayout
	base = base && c.ArtifactRuntimeWired && c.ArtifactIsometricRuntimeHook && c.ArtifactIsometricDOMHooks && c.ArtifactEntityLabelHooks && c.ArtifactLinkLabelHooks && c.ArtifactZoneLabelHooks && c.ArtifactBasePlaneHook && c.ArtifactGridHook && c.ArtifactLeaderLineHook && c.ArtifactArrowHook && c.ArtifactNoStudioRuntime && c.ArtifactNoStarfieldRuntime
	return base && (!withScreenshot || c.ScreenshotReadable)
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

func inspectArtifactEvidence(outDir string, tpl manifest.TemplateManifest) artifactEvidence {
	evidence := artifactEvidence{
		RuntimeWired:         true,
		IsometricRuntimeHook: true,
		IsometricDOMHooks:    true,
		EntityLabelHooks:     true,
		LinkLabelHooks:       true,
		ZoneLabelHooks:       true,
		BasePlaneHook:        true,
		GridHook:             true,
		LeaderLineHook:       true,
		ArrowHook:            true,
		NoStudioRuntime:      true,
		NoStarfieldRuntime:   true,
	}
	if tpl.Renderer.Contract != "offline.architecture.isometric.v1" {
		return evidence
	}
	indexHTML := readArtifactText(filepath.Join(outDir, "index.html"))
	runtimeJS := readArtifactText(filepath.Join(outDir, "assets", "runtime", "efp-visual-renderers.iife.js"))
	runtimeCSS := readArtifactText(filepath.Join(outDir, "assets", "runtime", "efp-visual-runtime.css"))
	templateCSS := readArtifactText(filepath.Join(outDir, "assets", "templates", tpl.ID, "style.css"))
	manifestJS := readArtifactText(filepath.Join(outDir, "manifest.js"))
	dataJS := readArtifactText(filepath.Join(outDir, "data.js"))
	isometricRuntime := isometricRuntimeBlock(runtimeJS)
	combinedArtifact := strings.ToLower(indexHTML + "\n" + runtimeCSS + "\n" + templateCSS + "\n" + manifestJS + "\n" + dataJS + "\n" + isometricRuntime)

	evidence.RuntimeWired = strings.Contains(indexHTML, "assets/runtime/efp-visual-runtime.iife.js") &&
		strings.Contains(indexHTML, "assets/runtime/efp-visual-renderers.iife.js") &&
		strings.Contains(indexHTML, "assets/vendor/three/efp-three.module.min.js") &&
		strings.Contains(indexHTML, "manifest.js") &&
		strings.Contains(indexHTML, "data.js")
	evidence.IsometricRuntimeHook = strings.Contains(runtimeJS, "function renderIsometricArchitecture") &&
		strings.Contains(runtimeJS, `runtime.registerRenderer("offline.architecture.isometric.v1"`)
	evidence.IsometricDOMHooks = strings.Contains(isometricRuntime, "data-isometric-renderer") &&
		strings.Contains(runtimeCSS+templateCSS, "visual-isometric-label-layer") &&
		strings.Contains(runtimeCSS+templateCSS, "visual-isometric-stage")
	evidence.EntityLabelHooks = strings.Contains(isometricRuntime, "data-entity-label") &&
		strings.Contains(runtimeCSS+templateCSS, "visual-isometric-label")
	evidence.LinkLabelHooks = strings.Contains(isometricRuntime, "data-link-label") &&
		strings.Contains(runtimeCSS+templateCSS, "visual-isometric-link-label")
	evidence.ZoneLabelHooks = strings.Contains(isometricRuntime, "data-zone-label") &&
		strings.Contains(runtimeCSS+templateCSS, "visual-isometric-zone-label")
	evidence.BasePlaneHook = strings.Contains(isometricRuntime, "isBasePlane")
	evidence.GridHook = strings.Contains(isometricRuntime, "isIsometricGrid")
	evidence.LeaderLineHook = strings.Contains(isometricRuntime, "isLeaderLine")
	evidence.ArrowHook = strings.Contains(isometricRuntime, "createArrowHead") && strings.Contains(isometricRuntime, "isDirectedArrow")
	evidence.NoStudioRuntime = !strings.Contains(combinedArtifact, "studio-") && !strings.Contains(combinedArtifact, "studioshell") && !strings.Contains(combinedArtifact, "renderstudio")
	evidence.NoStarfieldRuntime = !strings.Contains(strings.ToLower(isometricRuntime), "visual-space-dot") &&
		!strings.Contains(strings.ToLower(isometricRuntime+"\n"+templateCSS+"\n"+dataJS), "starfield")
	return evidence
}

func readArtifactText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func isometricRuntimeBlock(runtimeJS string) string {
	start := strings.Index(runtimeJS, "function isometricArray")
	if start < 0 {
		start = strings.Index(runtimeJS, "function createIsometricShell")
	}
	if start < 0 {
		start = strings.Index(runtimeJS, "function renderIsometricArchitecture")
	}
	if start < 0 {
		return ""
	}
	block := runtimeJS[start:]
	end := strings.Index(block, `runtime.registerRenderer("offline.architecture.isometric.v1"`)
	if end < 0 {
		return block
	}
	return block[:end]
}

func buildChecks(outputInspection render.Inspection, outputManifest manifest.OutputManifest, tpl manifest.TemplateManifest, visualPlan plan.VisualPlan, summary preview.Summary, screenshot ScreenshotInspection, evidence artifactEvidence) Checks {
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
		ShapeDiversity:                     shapeDiversity(visualPlan),
		ArrowsVisible:                      arrowsVisible(visualPlan),
		ColorDiversity:                     !visualPlan.Colors.SingleColor,
		LegendPresent:                      legendPresent(visualPlan),
		IconAssetsPresent:                  len(visualPlan.Assets.MissingIcons) == 0,
		AttributionsPresent:                len(visualPlan.Assets.IconsUsed) == 0 || len(visualPlan.Assets.Attributions) > 0,
		IsometricRendererUsed:              true,
		BasePlanePresent:                   true,
		GridPresent:                        true,
		ZonesPresent:                       true,
		ZoneBoundariesPresent:              true,
		EntitiesPresent:                    true,
		EntityLabelsPresent:                true,
		LeaderLinesPresent:                 true,
		DirectedArrowsPresent:              true,
		LinkLabelsPresent:                  true,
		OrthographicCameraPlanned:          true,
		ArchitectureLightTheme:             true,
		NoStarfieldTheme:                   true,
		NoStudioLayout:                     true,
		ArtifactRuntimeWired:               true,
		ArtifactIsometricRuntimeHook:       true,
		ArtifactIsometricDOMHooks:          true,
		ArtifactEntityLabelHooks:           true,
		ArtifactLinkLabelHooks:             true,
		ArtifactZoneLabelHooks:             true,
		ArtifactBasePlaneHook:              true,
		ArtifactGridHook:                   true,
		ArtifactLeaderLineHook:             true,
		ArtifactArrowHook:                  true,
		ArtifactNoStudioRuntime:            true,
		ArtifactNoStarfieldRuntime:         true,
		ScreenshotReadable:                 !screenshot.Provided || screenshot.NonBlank && screenshot.ContrastOK && screenshot.CoverageOK,
		ScreenshotNonBlank:                 !screenshot.Provided || screenshot.NonBlank,
		ScreenshotContrast:                 !screenshot.Provided || screenshot.ContrastOK,
		ScreenshotCoverage:                 !screenshot.Provided || screenshot.CoverageOK,
	}
	if tpl.Effects.Engine == "three.v1" {
		checks.ThreeAsset = files["assets/vendor/three/efp-three.module.min.js"]
	}
	applyArchitectureChecks(&checks, tpl, visualPlan, evidence)
	return checks
}

func applyArchitectureChecks(checks *Checks, tpl manifest.TemplateManifest, visualPlan plan.VisualPlan, evidence artifactEvidence) {
	if tpl.Renderer.Contract != "offline.architecture.isometric.v1" {
		return
	}
	iso := visualPlan.Isometric
	checks.IsometricRendererUsed = strings.ToLower(tpl.InputSchemaKind) == "isometric_architecture_v1"
	checks.NoStudioLayout = !strings.Contains(strings.ToLower(visualPlan.LayoutPreset), "studio")
	if iso == nil {
		checks.BasePlanePresent = false
		checks.GridPresent = false
		checks.ZonesPresent = false
		checks.ZoneBoundariesPresent = false
		checks.EntitiesPresent = false
		checks.EntityLabelsPresent = false
		checks.LeaderLinesPresent = false
		checks.DirectedArrowsPresent = false
		checks.LinkLabelsPresent = false
		checks.OrthographicCameraPlanned = false
		checks.ArchitectureLightTheme = false
		checks.NoStarfieldTheme = false
		return
	}
	checks.BasePlanePresent = iso.BasePlane
	checks.GridPresent = iso.Grid
	checks.ZonesPresent = iso.ZoneCount > 0
	checks.ZoneBoundariesPresent = iso.ZoneCount > 0 && len(iso.BoundaryStyles) > 0
	checks.EntitiesPresent = iso.EntityCount > 0
	checks.EntityLabelsPresent = iso.EntityCount > 0 && iso.TopLabels >= iso.EntityCount
	checks.LeaderLinesPresent = iso.EntityCount > 0 && iso.LeaderLines >= iso.TopLabels
	checks.DirectedArrowsPresent = iso.DirectedLinks == 0 || iso.ArrowLinks >= iso.DirectedLinks
	checks.LinkLabelsPresent = architectureLinkLabelsPresent(visualPlan)
	checks.OrthographicCameraPlanned = iso.Camera == "orthographic_isometric"
	checks.ArchitectureLightTheme = iso.Theme == "" || strings.EqualFold(iso.Theme, "architecture_light")
	checks.NoStarfieldTheme = !strings.Contains(strings.ToLower(iso.Theme), "starfield")
	checks.ArtifactRuntimeWired = evidence.RuntimeWired
	checks.ArtifactIsometricRuntimeHook = evidence.IsometricRuntimeHook
	checks.ArtifactIsometricDOMHooks = evidence.IsometricDOMHooks
	checks.ArtifactEntityLabelHooks = evidence.EntityLabelHooks
	checks.ArtifactLinkLabelHooks = evidence.LinkLabelHooks
	checks.ArtifactZoneLabelHooks = evidence.ZoneLabelHooks
	checks.ArtifactBasePlaneHook = evidence.BasePlaneHook
	checks.ArtifactGridHook = evidence.GridHook
	checks.ArtifactLeaderLineHook = evidence.LeaderLineHook
	checks.ArtifactArrowHook = evidence.ArrowHook
	checks.ArtifactNoStudioRuntime = evidence.NoStudioRuntime
	checks.ArtifactNoStarfieldRuntime = evidence.NoStarfieldRuntime
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
	if !checks.ShapeDiversity {
		add("shape_diversity_low", "warning", "Rendered marks do not use enough shape diversity for the number of objects.", "Add kind/provider/service/presentation.shape so services, APIs, databases, queues, external systems, decisions, and risks render differently.")
	}
	if !checks.ArrowsVisible {
		add("arrows_not_visible", "warning", "Directed relationships are present but the visual plan has no arrow encodings.", "Set directed=true or presentation.arrow=forward on causal, dependency, call, data-flow, event, write, or read relationships.")
	}
	if !checks.ColorDiversity {
		add("color_diversity_low", "warning", "Rendered marks resolve to a single color policy.", "Use view.colorBy/renderHints.colorBy or explicit semantic colors with a legend.")
	}
	if !checks.LegendPresent {
		add("legend_not_present", "warning", "Color encodes semantics but no legend is available in the visual plan.", "Set renderHints.showLegend=true and choose a colorBy field that exists on the input objects.")
	}
	if !checks.IconAssetsPresent {
		add("icon_assets_missing", "warning", "Some requested icon IDs are missing from the local asset registry.", "Use local icon IDs from assets/asset-registry.json or remove unknown presentation.icon values.")
	}
	if !checks.AttributionsPresent {
		add("asset_attributions_missing", "warning", "Icons are used but no asset attribution entries are present.", "Ensure manifest.json assets.attributions and assets/ATTRIBUTIONS.md are included in the artifact.")
	}
	if tpl.Renderer.Contract == "offline.architecture.isometric.v1" {
		if !checks.IsometricRendererUsed {
			add("architecture_renderer_missing", "error", "Rendered artifact does not use the isometric architecture input contract.", "Render with architecture.isometric_overview and isometric_architecture_v1 input.")
		}
		if !checks.BasePlanePresent {
			add("architecture_base_plane_missing", "error", "Isometric architecture plan is missing the base plane.", "Set canvas.grid.enabled=true and avoid starfield themes.")
		}
		if !checks.GridPresent {
			add("architecture_grid_missing", "error", "Isometric architecture plan is missing the base grid.", "Set canvas.grid.enabled=true.")
		}
		if !checks.ZonesPresent {
			add("architecture_zones_missing", "error", "Isometric architecture plan has no zones.", "Add zones[] with id, label, and bounds.")
		}
		if !checks.ZoneBoundariesPresent {
			add("architecture_zone_boundaries_missing", "warning", "Isometric architecture plan has no zone boundaries.", "Set zone.presentation.boundary or rely on solid boundaries by defining zones.")
		}
		if !checks.EntitiesPresent {
			add("architecture_entities_missing", "error", "Isometric architecture plan has no entities.", "Add entities[] for services, APIs, databases, queues, and infrastructure objects.")
		}
		if !checks.EntityLabelsPresent {
			add("architecture_entity_labels_missing", "error", "Isometric architecture plan does not expose top labels for entities.", "Use presentation.label=top or omit label overrides so visible entities keep top labels.")
		}
		if !checks.LeaderLinesPresent {
			add("architecture_leader_lines_missing", "error", "Isometric architecture plan does not expose leader lines for entity labels.", "Set presentation.leaderLine=true or omit the field for visible entities.")
		}
		if !checks.DirectedArrowsPresent {
			add("architecture_directed_arrows_missing", "error", "Directed architecture links are missing arrow encodings.", "Set links[].presentation.arrow=forward on directed links.")
		}
		if !checks.LinkLabelsPresent {
			add("architecture_link_labels_missing", "warning", "Some architecture links are missing labels.", "Set concise links[].label values.")
		}
		if !checks.OrthographicCameraPlanned {
			add("architecture_camera_not_isometric", "warning", "Architecture plan is not using an orthographic isometric camera.", "Set camera.preset=isometric or omit it for the default orthographic isometric camera.")
		}
		if !checks.ArchitectureLightTheme {
			add("architecture_light_theme_missing", "warning", "Architecture plan is not using architecture_light theme.", "Set theme=architecture_light.")
		}
		if !checks.NoStarfieldTheme {
			add("architecture_starfield_theme", "error", "Architecture plan uses a starfield theme.", "Use theme=architecture_light for grounded architecture scenes.")
		}
		if !checks.NoStudioLayout {
			add("architecture_studio_layout_detected", "error", "Architecture plan still references a Studio layout.", "Use layout preset isometric_architecture.")
		}
		if !checks.ArtifactRuntimeWired {
			add("architecture_artifact_runtime_not_wired", "error", "The generated index.html does not wire the local runtime, Three.js vendor asset, manifest.js, and data.js.", "Run visual render again and inspect the generated index.html script tags.")
		}
		if !checks.ArtifactIsometricRuntimeHook {
			add("architecture_artifact_renderer_hook_missing", "error", "The generated runtime asset does not expose the isometric architecture renderer hook.", "Ensure efp-visual-renderers.iife.js registers offline.architecture.isometric.v1 with renderIsometricArchitecture.")
		}
		if !checks.ArtifactIsometricDOMHooks {
			add("architecture_artifact_dom_hooks_missing", "error", "The generated artifact lacks isometric DOM hooks for the stage and label layer.", "Ensure the renderer creates data-isometric-renderer and visual-isometric-label-layer elements.")
		}
		if !checks.ArtifactEntityLabelHooks {
			add("architecture_artifact_entity_label_hooks_missing", "error", "The generated runtime/style assets do not expose entity label hooks.", "Ensure entity labels use data-entity-label and visual-isometric-label.")
		}
		if !checks.ArtifactLinkLabelHooks {
			add("architecture_artifact_link_label_hooks_missing", "warning", "The generated runtime/style assets do not expose link label hooks.", "Ensure link labels use data-link-label and visual-isometric-link-label.")
		}
		if !checks.ArtifactZoneLabelHooks {
			add("architecture_artifact_zone_label_hooks_missing", "warning", "The generated runtime/style assets do not expose zone label hooks.", "Ensure zone labels use data-zone-label and visual-isometric-zone-label.")
		}
		if !checks.ArtifactBasePlaneHook {
			add("architecture_artifact_base_plane_hook_missing", "error", "The generated runtime does not expose a base-plane hook.", "Ensure the isometric renderer tags the base plane with isBasePlane.")
		}
		if !checks.ArtifactGridHook {
			add("architecture_artifact_grid_hook_missing", "error", "The generated runtime does not expose a grid hook.", "Ensure the isometric renderer tags the grid with isIsometricGrid.")
		}
		if !checks.ArtifactLeaderLineHook {
			add("architecture_artifact_leader_line_hook_missing", "error", "The generated runtime does not expose leader-line hooks.", "Ensure entity labels keep visible leader lines tagged with isLeaderLine.")
		}
		if !checks.ArtifactArrowHook {
			add("architecture_artifact_arrow_hook_missing", "error", "The generated runtime does not expose directed arrow hooks.", "Ensure architecture links use createArrowHead and tag arrow geometry with isDirectedArrow.")
		}
		if !checks.ArtifactNoStudioRuntime {
			add("architecture_artifact_studio_runtime_detected", "error", "The generated architecture artifact still contains Studio runtime or style hooks.", "Remove legacy Studio renderer/style code from shared runtime assets.")
		}
		if !checks.ArtifactNoStarfieldRuntime {
			add("architecture_artifact_starfield_runtime_detected", "error", "The generated architecture artifact still contains starfield hooks in the isometric renderer path.", "Keep architecture.isometric_overview grounded on architecture_light base plane and grid.")
		}
	}
	_ = outputManifest
	return warnings
}

func architectureLinkLabelsPresent(visualPlan plan.VisualPlan) bool {
	if strings.ToLower(visualPlan.InputSchemaKind) != "isometric_architecture_v1" {
		return true
	}
	if len(visualPlan.IR.Relationships) == 0 {
		return true
	}
	for _, rel := range visualPlan.IR.Relationships {
		if strings.TrimSpace(rel.Label) == "" {
			return false
		}
	}
	return true
}

func shapeDiversity(visualPlan plan.VisualPlan) bool {
	total := 0
	for _, count := range visualPlan.Marks.ShapeCounts {
		total += count
	}
	if total <= 6 {
		return true
	}
	if visualPlan.Marks.FallbackSphereCount*100 >= total*80 {
		return false
	}
	return len(visualPlan.Marks.ShapeCounts) >= 2
}

func arrowsVisible(visualPlan plan.VisualPlan) bool {
	if visualPlan.Edges.DirectedCount == 0 {
		return true
	}
	return visualPlan.Edges.ArrowCount > 0
}

func legendPresent(visualPlan plan.VisualPlan) bool {
	if visualPlan.Colors.ColorBy == "" {
		return true
	}
	return visualPlan.Legend.Show || len(visualPlan.Colors.LegendItems) > 0
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
