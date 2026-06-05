package plan

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/visual/authoring"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/mark"
	"engineering-flow-platform-tools/internal/visual/metadata"
	"engineering-flow-platform-tools/internal/visual/preview"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
)

type Options struct {
	TemplateDir string
	TemplateID  string
	InputPath   string
	OutDir      string
	Stdin       io.Reader
}

type Result struct {
	TemplateID            string                    `json:"template_id"`
	TemplateDir           string                    `json:"template_dir"`
	AgentGuideAvailable   bool                      `json:"agent_guide_available"`
	AgentGuidePath        string                    `json:"agent_guide_path,omitempty"`
	QualityRulesAvailable bool                      `json:"quality_rules_available"`
	QualityRulesPath      string                    `json:"quality_rules_path,omitempty"`
	QualityScore          int                       `json:"quality_score"`
	Ready                 bool                      `json:"ready"`
	Warnings              []preview.Warning         `json:"warnings"`
	BlockingWarnings      []string                  `json:"blocking_warnings,omitempty"`
	Recommendations       preview.Recommendations   `json:"recommendations"`
	InputSummary          visualschema.InputSummary `json:"input_summary"`
	InputInspection       preview.Summary           `json:"input_inspection"`
	VisualPlan            VisualPlan                `json:"visual_plan"`
}

type VisualPlan struct {
	Schema           string           `json:"schema"`
	TemplateID       string           `json:"template_id"`
	Renderer         string           `json:"renderer"`
	LayoutPreset     string           `json:"layout_preset"`
	InputSchemaKind  string           `json:"input_schema_kind"`
	IR               VisualIR         `json:"ir"`
	View             ViewPlan         `json:"view"`
	Labels           LabelPlan        `json:"labels"`
	Legend           LegendPlan       `json:"legend"`
	Marks            MarkPlan         `json:"marks"`
	Edges            EdgeEncodingPlan `json:"edges"`
	Colors           ColorPlan        `json:"colors"`
	Assets           AssetPlan        `json:"assets"`
	Disclosure       DisclosurePlan   `json:"disclosure"`
	Selection        SelectionPlan    `json:"selection"`
	Render           RenderPlan       `json:"render"`
	QualityLoop      []QualityAction  `json:"quality_loop"`
	AgentNextActions []WorkflowAction `json:"agent_next_actions"`
}

type VisualIR struct {
	Schema        string           `json:"schema"`
	Kind          string           `json:"kind"`
	Objects       []IRObject       `json:"objects"`
	Relationships []IRRelationship `json:"relationships,omitempty"`
	Events        []IRObject       `json:"events,omitempty"`
	Counts        map[string]int   `json:"counts"`
}

type IRObject struct {
	ID            string   `json:"id"`
	Label         string   `json:"label,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Group         string   `json:"group,omitempty"`
	Status        string   `json:"status,omitempty"`
	Importance    float64  `json:"importance,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	LabelPriority string   `json:"labelPriority,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	Source        string   `json:"source"`
	Refs          []string `json:"refs,omitempty"`
}

type IRRelationship struct {
	ID            string  `json:"id"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	Kind          string  `json:"kind,omitempty"`
	Label         string  `json:"label,omitempty"`
	Importance    float64 `json:"importance,omitempty"`
	Visibility    string  `json:"visibility,omitempty"`
	LabelPriority string  `json:"labelPriority,omitempty"`
	Summary       string  `json:"summary,omitempty"`
	Source        string  `json:"source"`
}

type ViewPlan struct {
	Mode                    string   `json:"mode"`
	LabelMode               string   `json:"labelMode"`
	FocusMode               string   `json:"focusMode"`
	CameraPreset            string   `json:"cameraPreset"`
	InitialFocusIDs         []string `json:"initial_focus_ids"`
	HiddenDetailIDs         []string `json:"hidden_detail_ids"`
	OverviewObjectIDs       []string `json:"overview_object_ids"`
	OverviewRelationshipIDs []string `json:"overview_relationship_ids,omitempty"`
	MaxInitialObjects       int      `json:"max_initial_objects"`
	MaxInitialRelationships int      `json:"max_initial_relationships"`
}

type LabelPlan struct {
	Mode         string   `json:"mode"`
	AlwaysIDs    []string `json:"always_ids,omitempty"`
	ImportantIDs []string `json:"important_ids,omitempty"`
	NormalIDs    []string `json:"normal_ids,omitempty"`
	HoverIDs     []string `json:"hover_ids,omitempty"`
	HiddenIDs    []string `json:"hidden_ids,omitempty"`
}

type LegendPlan struct {
	Show  bool         `json:"show"`
	Kind  string       `json:"kind"`
	Items []LegendItem `json:"items"`
}

type LegendItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int    `json:"count"`
	Color string `json:"color,omitempty"`
}

type MarkPlan struct {
	ShapeCounts         map[string]int `json:"shape_counts"`
	IconCounts          map[string]int `json:"icon_counts"`
	FallbackSphereCount int            `json:"fallback_sphere_count"`
}

type EdgeEncodingPlan struct {
	DirectedCount   int `json:"directed_count"`
	ArrowCount      int `json:"arrow_count"`
	UndirectedCount int `json:"undirected_count"`
}

type ColorPlan struct {
	ColorBy     string       `json:"colorBy,omitempty"`
	LegendItems []LegendItem `json:"legend_items"`
	SingleColor bool         `json:"single_color"`
}

type AssetPlan struct {
	IconsUsed    []string           `json:"icons_used"`
	MissingIcons []string           `json:"missing_icons"`
	Attributions []mark.Attribution `json:"attributions"`
}

type DisclosurePlan struct {
	Strategy       string          `json:"strategy"`
	HiddenIDs      []string        `json:"hidden_ids,omitempty"`
	ExpandableIDs  []string        `json:"expandable_ids,omitempty"`
	NarrativeSteps []NarrativeStep `json:"narrative_steps,omitempty"`
}

type NarrativeStep struct {
	ID       string   `json:"id"`
	Title    string   `json:"title,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	FocusIDs []string `json:"focus_ids,omitempty"`
}

type SelectionPlan struct {
	Enabled       bool   `json:"enabled"`
	HighlightMode string `json:"highlight_mode"`
	Inspector     bool   `json:"inspector"`
}

type RenderPlan struct {
	Command               []string `json:"command"`
	OutDir                string   `json:"out_dir,omitempty"`
	DataMode              string   `json:"data_mode"`
	Offline               bool     `json:"offline"`
	ExpectedArtifactFiles []string `json:"expected_artifact_files"`
}

type QualityAction struct {
	Code        string         `json:"code"`
	Severity    string         `json:"severity"`
	Path        string         `json:"path,omitempty"`
	Suggestion  string         `json:"suggestion"`
	AutoFixHint map[string]any `json:"auto_fix_hint,omitempty"`
}

type WorkflowAction struct {
	Step    string `json:"step"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func Inspect(opts Options) (Result, error) {
	registry, err := manifest.LoadRegistry(opts.TemplateDir)
	if err != nil {
		return Result{}, err
	}
	entry, _, ok := registry.Resolve(opts.TemplateID)
	if !ok {
		return Result{}, metadata.NewError("template_not_found", "visual template was not found: "+opts.TemplateID, "Run visual template list --json and choose one of the returned ids.", 404)
	}
	tpl, err := manifest.LoadTemplateManifest(opts.TemplateDir, entry)
	if err != nil {
		return Result{}, err
	}
	if err := manifest.ValidateTemplateManifest(opts.TemplateDir, entry, &tpl); err != nil {
		return Result{}, err
	}
	raw, err := readInput(opts.InputPath, opts.Stdin)
	if err != nil {
		return Result{}, err
	}
	guide, err := authoring.LoadGuide(opts.TemplateDir, entry, false)
	if err != nil {
		return Result{}, err
	}
	rules, rulesAvailable, rulesPath, err := authoring.LoadQualityRules(opts.TemplateDir, entry)
	if err != nil {
		return Result{}, err
	}
	parsed, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits)
	if err != nil {
		return Result{}, err
	}
	quality, summary, warnings, recommendations := preview.Analyze(opts.TemplateDir, tpl, parsed.Data, rules)
	warnings = normalizeWarnings(warnings)
	blocking := blockingWarningCodes(warnings)
	visualPlan := Build(opts.TemplateDir, tpl, parsed.Data, summary, warnings, recommendations, opts.OutDir)
	return Result{
		TemplateID:            tpl.ID,
		TemplateDir:           opts.TemplateDir,
		AgentGuideAvailable:   guide.Available,
		AgentGuidePath:        guide.GuidePath,
		QualityRulesAvailable: rulesAvailable,
		QualityRulesPath:      rulesPath,
		QualityScore:          quality,
		Ready:                 len(blocking) == 0 && quality >= 70,
		Warnings:              warnings,
		BlockingWarnings:      blocking,
		Recommendations:       recommendations,
		InputSummary:          parsed.Summary,
		InputInspection:       summary,
		VisualPlan:            visualPlan,
	}, nil
}

func Build(templateDir string, tpl manifest.TemplateManifest, data map[string]any, summary preview.Summary, warnings []preview.Warning, recommendations preview.Recommendations, outDir string) VisualPlan {
	visual := object(data, "visual")
	view := object(data, "view")
	if len(view) == 0 {
		view = object(data, "initial_view")
	}
	renderHints := object(data, "renderHints")
	ir := buildIR(tpl.InputSchemaKind, data)
	focusIDs := stringArray(firstPresent(visual["initialFocusIds"], visual["initial_focus_ids"]))
	hiddenIDs := stringArray(firstPresent(visual["hiddenDetailIds"], visual["hidden_detail_ids"]))
	labelMode := str(firstPresent(view["labelMode"], view["label_mode"], renderHints["labelMode"], renderHints["label_mode"]))
	if labelMode == "" {
		labelMode = "overview"
	}
	mode := str(view["mode"])
	if mode == "" {
		mode = recommendations.InitialView
	}
	if mode == "" {
		mode = "overview"
	}
	focusMode := str(firstPresent(view["focusMode"], view["focus_mode"]))
	if focusMode == "" {
		focusMode = "neighborhood"
	}
	cameraPreset := str(firstPresent(view["cameraPreset"], view["camera_preset"]))
	if cameraPreset == "" {
		cameraPreset = defaultCameraPreset(tpl)
	}
	labels := buildLabelPlan(labelMode, ir, hiddenIDs)
	overviewObjects, overviewRelationships := overviewIDs(ir, focusIDs, hiddenIDs, recommendations.MaxInitialNodes, recommendations.MaxInitialEdges)
	legend := buildLegendPlan(tpl, data, ir, renderHints)
	markStats := mark.Analyze(templateDir, tpl.InputSchemaKind, data)
	plan := VisualPlan{
		Schema:          "efp.visual.plan.v1",
		TemplateID:      tpl.ID,
		Renderer:        tpl.Renderer.Contract,
		LayoutPreset:    tpl.Layout.Preset,
		InputSchemaKind: tpl.InputSchemaKind,
		IR:              ir,
		View: ViewPlan{
			Mode:                    mode,
			LabelMode:               labelMode,
			FocusMode:               focusMode,
			CameraPreset:            cameraPreset,
			InitialFocusIDs:         focusIDs,
			HiddenDetailIDs:         hiddenIDs,
			OverviewObjectIDs:       overviewObjects,
			OverviewRelationshipIDs: overviewRelationships,
			MaxInitialObjects:       recommendations.MaxInitialNodes,
			MaxInitialRelationships: recommendations.MaxInitialEdges,
		},
		Labels: labels,
		Legend: legend,
		Marks: MarkPlan{
			ShapeCounts:         markStats.ShapeCounts,
			IconCounts:          markStats.IconCounts,
			FallbackSphereCount: markStats.FallbackSphereCount,
		},
		Edges: EdgeEncodingPlan{
			DirectedCount:   markStats.DirectedCount,
			ArrowCount:      markStats.ArrowCount,
			UndirectedCount: markStats.UndirectedCount,
		},
		Colors: ColorPlan{
			ColorBy:     markStats.ColorBy,
			LegendItems: legendItemsFromMark(markStats.LegendItems),
			SingleColor: markStats.SingleColor,
		},
		Assets: AssetPlan{
			IconsUsed:    markStats.IconsUsed,
			MissingIcons: markStats.MissingIcons,
			Attributions: markStats.Attributions,
		},
		Disclosure: DisclosurePlan{
			Strategy:       disclosureStrategy(tpl, summary),
			HiddenIDs:      hiddenIDs,
			ExpandableIDs:  expandableIDs(ir),
			NarrativeSteps: narrativeSteps(visual),
		},
		Selection: SelectionPlan{Enabled: true, HighlightMode: "selected_and_related", Inspector: true},
		Render: RenderPlan{
			Command:               renderCommand(tpl.ID, outDir),
			OutDir:                outDir,
			DataMode:              "js-file",
			Offline:               true,
			ExpectedArtifactFiles: []string{"index.html", "manifest.json", "manifest.js", "data.js", "assets/runtime/efp-visual-runtime.iife.js", "assets/runtime/efp-visual-renderers.iife.js", "assets/runtime/efp-visual-runtime.css"},
		},
		QualityLoop:      qualityActions(warnings),
		AgentNextActions: nextActions(tpl.ID, outDir, warnings),
	}
	return plan
}

func legendItemsFromMark(items []mark.LegendItem) []LegendItem {
	out := make([]LegendItem, 0, len(items))
	for _, item := range items {
		out = append(out, LegendItem{ID: item.ID, Label: item.Label, Count: item.Count, Color: item.Color})
	}
	return out
}

func buildIR(kind string, data map[string]any) VisualIR {
	ir := VisualIR{Schema: "efp.visual.ir.v1", Kind: kind, Counts: map[string]int{}}
	switch strings.ToLower(kind) {
	case "uml_sequence_v1":
		addObjects(&ir, data, "participants", "participant")
		addObjects(&ir, data, "phases", "phase")
		addObjects(&ir, data, "activations", "activation")
		addObjects(&ir, data, "fragments", "fragment")
		for i, msg := range objects(data, "messages") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(msg, "messages", i, firstString(msg, "from"), firstString(msg, "to")))
		}
	case "graph_v1", "graph_events_v1":
		addObjects(&ir, data, "groups", "group")
		addObjects(&ir, data, "nodes", "node")
		for i, edge := range objects(data, "edges") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(edge, "edges", i, firstString(edge, "from"), firstString(edge, "to")))
		}
		for i, event := range objects(data, "events") {
			ir.Events = append(ir.Events, objectFrom(event, "events", i, "event"))
		}
	case "timeline_v1":
		for i, event := range objects(data, "events") {
			ir.Events = append(ir.Events, objectFrom(event, "events", i, "event"))
		}
	case "evidence_v1":
		addObjects(&ir, data, "claims", "claim")
		addObjects(&ir, data, "sources", "source")
		for i, link := range objects(data, "links") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(link, "links", i, firstString(link, "claim_id", "from"), firstString(link, "source_id", "to")))
		}
	case "matrix_v1":
		addObjects(&ir, data, "items", "item")
	case "uml_class_v1":
		addObjects(&ir, data, "classes", "class")
		for i, rel := range objects(data, "relationships") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(rel, "relationships", i, firstString(rel, "from", "source"), firstString(rel, "to", "target")))
		}
	case "uml_state_machine_v1":
		addObjects(&ir, data, "states", "state")
		for i, rel := range objects(data, "transitions") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(rel, "transitions", i, firstString(rel, "from", "source"), firstString(rel, "to", "target")))
		}
	case "uml_activity_v1":
		addObjects(&ir, data, "actions", "action")
		for i, rel := range objects(data, "flows") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(rel, "flows", i, firstString(rel, "from", "source"), firstString(rel, "to", "target")))
		}
	case "uml_component_deployment_v1":
		addObjects(&ir, data, "components", "component")
		addObjects(&ir, data, "deployments", "deployment")
		for i, rel := range objects(data, "links") {
			ir.Relationships = append(ir.Relationships, relationshipFrom(rel, "links", i, firstString(rel, "from", "source"), firstString(rel, "to", "target")))
		}
	}
	ir.Counts["objects"] = len(ir.Objects)
	ir.Counts["relationships"] = len(ir.Relationships)
	ir.Counts["events"] = len(ir.Events)
	return ir
}

func addObjects(ir *VisualIR, data map[string]any, field, fallbackKind string) {
	items := objects(data, field)
	ir.Counts[field] = len(items)
	for i, obj := range items {
		ir.Objects = append(ir.Objects, objectFrom(obj, field, i, fallbackKind))
	}
}

func objectFrom(obj map[string]any, field string, index int, fallbackKind string) IRObject {
	return IRObject{
		ID:            objectID(obj, field, index),
		Label:         label(obj),
		Kind:          nonEmpty(firstString(obj, "kind"), fallbackKind),
		Group:         firstString(obj, "group", "group_id", "parent_id", "module", "package", "lane", "category"),
		Status:        firstString(obj, "status"),
		Importance:    importance(obj),
		Visibility:    normalizeVisibility(firstString(obj, "visibility")),
		LabelPriority: normalizeLabelPriority(obj),
		Summary:       firstString(obj, "summary"),
		Source:        field,
		Refs:          refs(obj),
	}
}

func relationshipFrom(obj map[string]any, field string, index int, from, to string) IRRelationship {
	id := firstString(obj, "id")
	if id == "" && from != "" && to != "" {
		id = from + "->" + to
	}
	if id == "" {
		id = field + ":" + intString(index)
	}
	return IRRelationship{
		ID:            id,
		From:          from,
		To:            to,
		Kind:          firstString(obj, "kind", "relation"),
		Label:         label(obj),
		Importance:    importance(obj),
		Visibility:    normalizeVisibility(firstString(obj, "visibility")),
		LabelPriority: normalizeLabelPriority(obj),
		Summary:       firstString(obj, "summary"),
		Source:        field,
	}
}

func buildLabelPlan(mode string, ir VisualIR, hiddenIDs []string) LabelPlan {
	hidden := set(hiddenIDs)
	plan := LabelPlan{Mode: mode}
	for _, obj := range ir.Objects {
		bucketLabelID(&plan, obj.ID, obj.LabelPriority, obj.Importance, hidden[obj.ID])
	}
	for _, rel := range ir.Relationships {
		bucketLabelID(&plan, rel.ID, rel.LabelPriority, rel.Importance, hidden[rel.ID])
	}
	return sortLabelPlan(plan)
}

func bucketLabelID(plan *LabelPlan, id, priority string, imp float64, hidden bool) {
	if id == "" {
		return
	}
	if hidden || priority == "hidden" {
		plan.HiddenIDs = append(plan.HiddenIDs, id)
		return
	}
	switch priority {
	case "always":
		plan.AlwaysIDs = append(plan.AlwaysIDs, id)
	case "important":
		plan.ImportantIDs = append(plan.ImportantIDs, id)
	case "normal":
		plan.NormalIDs = append(plan.NormalIDs, id)
	case "hover":
		plan.HoverIDs = append(plan.HoverIDs, id)
	default:
		if imp >= 0.85 {
			plan.AlwaysIDs = append(plan.AlwaysIDs, id)
		} else if imp >= 0.65 {
			plan.ImportantIDs = append(plan.ImportantIDs, id)
		} else if imp >= 0.35 {
			plan.NormalIDs = append(plan.NormalIDs, id)
		} else {
			plan.HoverIDs = append(plan.HoverIDs, id)
		}
	}
}

func overviewIDs(ir VisualIR, focusIDs, hiddenIDs []string, maxObjects, maxRelationships int) ([]string, []string) {
	focus := set(focusIDs)
	hidden := set(hiddenIDs)
	objects := append([]IRObject{}, ir.Objects...)
	sort.Slice(objects, func(i, j int) bool {
		return scoreObject(objects[i], focus, hidden) > scoreObject(objects[j], focus, hidden)
	})
	rels := append([]IRRelationship{}, ir.Relationships...)
	sort.Slice(rels, func(i, j int) bool {
		return scoreRelationship(rels[i], focus, hidden) > scoreRelationship(rels[j], focus, hidden)
	})
	if maxObjects <= 0 {
		maxObjects = 60
	}
	if maxRelationships <= 0 {
		maxRelationships = 120
	}
	objectIDs := make([]string, 0, min(len(objects), maxObjects))
	for _, obj := range objects {
		if len(objectIDs) >= maxObjects {
			break
		}
		if hidden[obj.ID] && !focus[obj.ID] {
			continue
		}
		if obj.Visibility == "hidden" {
			continue
		}
		objectIDs = append(objectIDs, obj.ID)
	}
	relIDs := make([]string, 0, min(len(rels), maxRelationships))
	for _, rel := range rels {
		if len(relIDs) >= maxRelationships {
			break
		}
		if hidden[rel.ID] && !focus[rel.ID] {
			continue
		}
		if rel.Visibility == "hidden" || rel.Visibility == "detail" && !focus[rel.ID] {
			continue
		}
		relIDs = append(relIDs, rel.ID)
	}
	return objectIDs, relIDs
}

func buildLegendPlan(tpl manifest.TemplateManifest, data map[string]any, ir VisualIR, renderHints map[string]any) LegendPlan {
	if boolValue(renderHints["showLegend"]) == false && renderHints["showLegend"] != nil {
		return LegendPlan{Show: false, Kind: "none"}
	}
	counts := map[string]int{}
	kind := "kind"
	if strings.ToLower(tpl.InputSchemaKind) == "uml_sequence_v1" {
		kind = "phase"
		for _, phase := range objects(data, "phases") {
			id := firstString(phase, "id", "label", "name")
			if id != "" {
				counts[id]++
			}
		}
		for _, rel := range ir.Relationships {
			if rel.Kind != "" {
				continue
			}
		}
	} else {
		for _, obj := range ir.Objects {
			if obj.Kind != "" {
				counts[obj.Kind]++
			}
		}
		for _, rel := range ir.Relationships {
			if rel.Kind != "" {
				counts[rel.Kind]++
			}
		}
	}
	items := make([]LegendItem, 0, len(counts))
	for id, count := range counts {
		items = append(items, LegendItem{ID: id, Label: id, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].ID < items[j].ID
		}
		return items[i].Count > items[j].Count
	})
	return LegendPlan{Show: len(items) > 0, Kind: kind, Items: items}
}

func disclosureStrategy(tpl manifest.TemplateManifest, summary preview.Summary) string {
	kind := strings.ToLower(tpl.InputSchemaKind)
	if kind == "uml_sequence_v1" {
		return "phase_narrative_with_fragments"
	}
	if strings.Contains(kind, "graph") && (summary.Groups > 0 || summary.Nodes > 20) {
		return "group_collapse_then_focus_expand"
	}
	if kind == "timeline_v1" {
		return "milestone_first_then_detail_events"
	}
	if kind == "matrix_v1" {
		return "focus_cells_then_filter_groups"
	}
	if kind == "evidence_v1" {
		return "claims_first_then_sources"
	}
	return "overview_then_detail"
}

func expandableIDs(ir VisualIR) []string {
	var out []string
	for _, obj := range ir.Objects {
		if obj.Source == "groups" || obj.Source == "fragments" || obj.Kind == "group" || obj.Kind == "fragment" {
			out = append(out, obj.ID)
		}
	}
	sort.Strings(out)
	return out
}

func narrativeSteps(visual map[string]any) []NarrativeStep {
	raw, _ := visual["narrative_steps"].([]any)
	if len(raw) == 0 {
		raw, _ = visual["narrativeSteps"].([]any)
	}
	var out []NarrativeStep
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, NarrativeStep{ID: firstString(obj, "id"), Title: firstString(obj, "title"), Summary: firstString(obj, "summary"), FocusIDs: stringArray(firstPresent(obj["focus_ids"], obj["focusIds"]))})
	}
	return out
}

func qualityActions(warnings []preview.Warning) []QualityAction {
	out := make([]QualityAction, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, QualityAction{Code: warning.Code, Severity: warning.Severity, Path: warning.Path, Suggestion: warning.Suggestion, AutoFixHint: warning.AutoFixHint})
	}
	return out
}

func nextActions(templateID, outDir string, warnings []preview.Warning) []WorkflowAction {
	var out []WorkflowAction
	if len(warnings) > 0 {
		out = append(out, WorkflowAction{Step: "revise_input", Reason: "inspect-plan found quality warnings; apply visual_plan.quality_loop suggestions before render"})
		out = append(out, WorkflowAction{Step: "rerun_inspect_plan", Command: "visual inspect-plan --template " + templateID + " --input <input.json> --json"})
	}
	cmd := "visual render --template " + templateID + " --input <input.json> --out " + nonEmpty(outDir, "<out-dir>") + " --json"
	out = append(out, WorkflowAction{Step: "render", Command: cmd})
	out = append(out, WorkflowAction{Step: "return_entrypoint", Reason: "Return data.artifact.entrypoint from render output"})
	return out
}

func renderCommand(templateID, outDir string) []string {
	return []string{"visual", "render", "--template", templateID, "--input", "<input.json>", "--out", nonEmpty(outDir, "<out-dir>"), "--json"}
}

func blockingWarningCodes(warnings []preview.Warning) []string {
	var out []string
	for _, warning := range warnings {
		if strings.ToLower(warning.Severity) == "error" {
			out = append(out, warning.Code)
		}
	}
	sort.Strings(out)
	return out
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

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		if stdin == nil {
			stdin = strings.NewReader("")
		}
		return io.ReadAll(stdin)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read input JSON: "+err.Error(), "Pass a readable JSON file path to --input.", 400)
	}
	return b, nil
}

func objects(data map[string]any, field string) []map[string]any {
	raw, _ := data[field].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func object(data map[string]any, field string) map[string]any {
	obj, _ := data[field].(map[string]any)
	if obj == nil {
		return map[string]any{}
	}
	return obj
}

func objectID(obj map[string]any, field string, index int) string {
	if id := firstString(obj, "id"); id != "" {
		return id
	}
	return field + ":" + intString(index)
}

func label(obj map[string]any) string {
	return firstString(obj, "displayName", "display_name", "label", "name", "title", "summary", "text", "id")
}

func firstString(obj map[string]any, names ...string) string {
	for _, name := range names {
		if value := str(obj[name]); value != "" {
			return value
		}
	}
	return ""
}

func str(value any) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}

func firstPresent(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringArray(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s := str(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func refs(obj map[string]any) []string {
	var out []string
	for _, name := range []string{"from", "to", "source", "target", "claim_id", "source_id", "participant_id", "node_id", "parent_id", "group_id"} {
		if s := firstString(obj, name); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func importance(obj map[string]any) float64 {
	if v, ok := number(obj["importance"]); ok {
		return normalizeNumber(v)
	}
	metrics, _ := obj["metrics"].(map[string]any)
	for _, name := range []string{"importance", "impact", "risk", "score", "weight"} {
		if v, ok := number(metrics[name]); ok {
			return normalizeNumber(v)
		}
	}
	return 0
}

func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func normalizeNumber(v float64) float64 {
	if v > 1 {
		return v / 100
	}
	return v
}

func normalizeVisibility(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "overview", "normal", "detail", "hidden":
		return value
	case "visible":
		return "overview"
	case "collapsed":
		return "detail"
	default:
		return ""
	}
}

func normalizeLabelPriority(obj map[string]any) string {
	value := firstPresent(obj["labelPriority"], obj["label_priority"])
	if n, ok := number(value); ok {
		n = normalizeNumber(n)
		switch {
		case n >= 0.85:
			return "always"
		case n >= 0.65:
			return "important"
		case n >= 0.35:
			return "normal"
		case n > 0:
			return "hover"
		default:
			return "hidden"
		}
	}
	text := strings.ToLower(str(value))
	switch text {
	case "always", "important", "normal", "hover", "hidden":
		return text
	default:
		return ""
	}
}

func boolValue(value any) bool { b, _ := value.(bool); return b }
func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
func intString(v int) string { return strconv.Itoa(v) }
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func set(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func scoreObject(obj IRObject, focus, hidden map[string]bool) float64 {
	score := obj.Importance
	if score == 0 {
		score = 0.25
	}
	if focus[obj.ID] {
		score += 2
	}
	if hidden[obj.ID] || obj.Visibility == "hidden" || obj.Visibility == "detail" {
		score -= 1
	}
	return score
}

func scoreRelationship(rel IRRelationship, focus, hidden map[string]bool) float64 {
	score := rel.Importance
	if score == 0 {
		score = 0.2
	}
	if focus[rel.ID] {
		score += 2
	}
	if hidden[rel.ID] || rel.Visibility == "hidden" || rel.Visibility == "detail" {
		score -= 1
	}
	return score
}

func defaultCameraPreset(tpl manifest.TemplateManifest) string {
	if strings.Contains(strings.ToLower(tpl.Renderer.Contract), "sequence") {
		return "left_to_right"
	}
	if strings.Contains(strings.ToLower(tpl.Category), "spatial") {
		return "orbit"
	}
	return "overview"
}

func sortLabelPlan(plan LabelPlan) LabelPlan {
	sort.Strings(plan.AlwaysIDs)
	sort.Strings(plan.ImportantIDs)
	sort.Strings(plan.NormalIDs)
	sort.Strings(plan.HoverIDs)
	sort.Strings(plan.HiddenIDs)
	return plan
}
