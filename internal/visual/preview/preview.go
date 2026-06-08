package preview

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/visual/authoring"
	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/mark"
	"engineering-flow-platform-tools/internal/visual/mermaid"
	"engineering-flow-platform-tools/internal/visual/metadata"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
)

type Options struct {
	TemplateDir string
	TemplateID  string
	InputPath   string
	Stdin       io.Reader
}

type Result struct {
	TemplateID            string                    `json:"template_id"`
	TemplateDir           string                    `json:"template_dir"`
	AgentGuideAvailable   bool                      `json:"agent_guide_available"`
	AgentGuidePath        string                    `json:"agent_guide_path,omitempty"`
	QualityRulesAvailable bool                      `json:"quality_rules_available"`
	QualityRulesPath      string                    `json:"quality_rules_path,omitempty"`
	InputSummary          visualschema.InputSummary `json:"input_summary"`
	QualityScore          int                       `json:"quality_score"`
	Summary               Summary                   `json:"summary"`
	Warnings              []Warning                 `json:"warnings"`
	Recommendations       Recommendations           `json:"recommendations"`
	VisualDesign          manifest.VisualDesign     `json:"visual_design"`
}

type Summary struct {
	Nodes                   int      `json:"nodes,omitempty"`
	Edges                   int      `json:"edges,omitempty"`
	Groups                  int      `json:"groups,omitempty"`
	Zones                   int      `json:"zones,omitempty"`
	Entities                int      `json:"entities,omitempty"`
	VisibleNodes            int      `json:"visible_nodes,omitempty"`
	VisibleEdges            int      `json:"visible_edges,omitempty"`
	Events                  int      `json:"events,omitempty"`
	Participants            int      `json:"participants,omitempty"`
	Messages                int      `json:"messages,omitempty"`
	Phases                  int      `json:"phases,omitempty"`
	Activations             int      `json:"activations,omitempty"`
	Fragments               int      `json:"fragments,omitempty"`
	Classes                 int      `json:"classes,omitempty"`
	Relationships           int      `json:"relationships,omitempty"`
	States                  int      `json:"states,omitempty"`
	Transitions             int      `json:"transitions,omitempty"`
	Actions                 int      `json:"actions,omitempty"`
	Flows                   int      `json:"flows,omitempty"`
	Components              int      `json:"components,omitempty"`
	Deployments             int      `json:"deployments,omitempty"`
	Claims                  int      `json:"claims,omitempty"`
	Sources                 int      `json:"sources,omitempty"`
	Links                   int      `json:"links,omitempty"`
	Items                   int      `json:"items,omitempty"`
	Panels                  int      `json:"panels,omitempty"`
	EdgeDensity             string   `json:"edge_density,omitempty"`
	LabelPressure           string   `json:"label_pressure,omitempty"`
	GroupCoverage           float64  `json:"group_coverage,omitempty"`
	RelationCoverage        float64  `json:"relation_coverage,omitempty"`
	EdgeKindCount           int      `json:"edge_kind_count,omitempty"`
	DominantEdgeKinds       []Count  `json:"dominant_edge_kinds,omitempty"`
	OrphanNodes             []string `json:"orphan_nodes,omitempty"`
	OrphanNodeCount         int      `json:"orphan_node_count,omitempty"`
	LargestGroupSize        int      `json:"largest_group_size,omitempty"`
	LargeGroups             []string `json:"large_groups,omitempty"`
	GenericGroups           []string `json:"generic_groups,omitempty"`
	MissingLabels           int      `json:"missing_labels,omitempty"`
	FallbackIDLabels        []string `json:"fallback_id_labels,omitempty"`
	LongLabels              []string `json:"long_labels,omitempty"`
	DuplicateLabels         []string `json:"duplicate_labels,omitempty"`
	EventsWithoutNodeID     int      `json:"events_without_node_id,omitempty"`
	EventsWithoutKnownNode  int      `json:"events_without_known_node,omitempty"`
	EventNodeCoverage       float64  `json:"event_node_coverage,omitempty"`
	MessagesWithoutPhase    int      `json:"messages_without_phase,omitempty"`
	ParticipantFanout       []string `json:"participant_fanout,omitempty"`
	MissingImportance       int      `json:"missing_importance,omitempty"`
	MissingVisibility       int      `json:"missing_visibility,omitempty"`
	VisualFocusIDs          int      `json:"visual_focus_ids,omitempty"`
	VisualHiddenDetails     int      `json:"visual_hidden_details,omitempty"`
	VisualNarrativeSteps    int      `json:"visual_narrative_steps,omitempty"`
	VisualAnnotations       int      `json:"visual_annotations,omitempty"`
	VisualReferenceCoverage float64  `json:"visual_reference_coverage,omitempty"`
	VisualUnknownRefs       []string `json:"visual_unknown_refs,omitempty"`
	HighFanoutNodes         []string `json:"high_fanout_nodes,omitempty"`
	InitialView             string   `json:"initial_view,omitempty"`
	CollapseByDefault       bool     `json:"collapse_by_default,omitempty"`
}

type Warning struct {
	Code        string         `json:"code"`
	Severity    string         `json:"severity,omitempty"`
	Path        string         `json:"path,omitempty"`
	Message     string         `json:"message"`
	Hint        string         `json:"hint,omitempty"`
	Suggestion  string         `json:"suggestion"`
	AutoFixHint map[string]any `json:"auto_fix_hint,omitempty"`
	Details     []string       `json:"details,omitempty"`
}

type Recommendations struct {
	InitialView       string   `json:"initial_view"`
	MaxInitialNodes   int      `json:"max_initial_nodes"`
	MaxInitialEdges   int      `json:"max_initial_edges"`
	GroupBy           []string `json:"group_by,omitempty"`
	CollapseByDefault bool     `json:"collapse_by_default"`
	HideEdgeTypes     []string `json:"hide_edge_types,omitempty"`
	AddFields         []string `json:"add_fields,omitempty"`
	FocusCandidates   []string `json:"focus_candidates,omitempty"`
	RewriteLabels     []string `json:"rewrite_labels,omitempty"`
	AgentGuidance     []string `json:"agent_guidance,omitempty"`
}

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func Preview(opts Options) (Result, error) {
	registry, err := manifest.LoadRegistry(opts.TemplateDir)
	if err != nil {
		return Result{}, err
	}
	raw, err := readInput(opts.InputPath, opts.Stdin)
	if err != nil {
		return Result{}, err
	}
	templateID := strings.TrimSpace(opts.TemplateID)
	if templateID == "" {
		inferred, ok := mermaid.InferTemplateID(raw)
		if !ok {
			return Result{}, metadata.NewError("template_required", "visual inspect-input requires --template for JSON input.", "Pass --template <template-id>, or pass a Mermaid .mmd input so the template can be inferred.", 400)
		}
		templateID = inferred
	}
	entry, _, ok := registry.Resolve(templateID)
	if !ok {
		return Result{}, metadata.NewError("template_not_found", "visual template was not found: "+templateID, "Run visual template list --json and choose one of the returned ids.", 404)
	}
	tpl, err := manifest.LoadTemplateManifest(opts.TemplateDir, entry)
	if err != nil {
		return Result{}, err
	}
	if err := manifest.ValidateTemplateManifest(opts.TemplateDir, entry, &tpl); err != nil {
		return Result{}, err
	}
	raw, err = mermaid.CompileIfNeeded(tpl.InputSchemaKind, raw)
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
	quality, summary, warnings, recommendations := Analyze(opts.TemplateDir, tpl, parsed.Data, rules)
	warnings = normalizeWarnings(warnings)
	return Result{
		TemplateID:            tpl.ID,
		TemplateDir:           opts.TemplateDir,
		AgentGuideAvailable:   guide.Available,
		AgentGuidePath:        guide.GuidePath,
		QualityRulesAvailable: rulesAvailable,
		QualityRulesPath:      rulesPath,
		InputSummary:          parsed.Summary,
		QualityScore:          quality,
		Summary:               summary,
		Warnings:              warnings,
		Recommendations:       recommendations,
		VisualDesign:          tpl.VisualDesign,
	}, nil
}

func Analyze(templateDir string, tpl manifest.TemplateManifest, data map[string]any, rules authoring.QualityRules) (int, Summary, []Warning, Recommendations) {
	design := tpl.VisualDesign
	if design.InitialView == "" {
		design.InitialView = "overview"
	}
	if design.MaxInitialNodes <= 0 {
		design.MaxInitialNodes = 60
	}
	if design.MaxInitialEdges <= 0 {
		design.MaxInitialEdges = 120
	}
	if len(design.GroupBy) == 0 {
		design.GroupBy = []string{"group", "module", "package"}
	}
	summary := Summary{InitialView: design.InitialView, CollapseByDefault: design.DefaultCollapseDepth > 0}
	recommendations := Recommendations{
		InitialView:       "overview",
		MaxInitialNodes:   design.MaxInitialNodes,
		MaxInitialEdges:   design.MaxInitialEdges,
		GroupBy:           design.GroupBy,
		CollapseByDefault: true,
		AgentGuidance:     design.AgentGuidance,
	}
	warnings := []Warning{}
	quality := 100
	switch strings.ToLower(tpl.InputSchemaKind) {
	case "graph_v1", "graph_events_v1":
		quality = analyzeGraph(data, design, &summary, &warnings, &recommendations)
	case "timeline_v1":
		events := len(array(data, "events"))
		summary.Events = events
		if events > design.MaxInitialNodes {
			quality -= 18
			warnings = append(warnings, Warning{Code: "timeline_too_long", Message: "Timeline has more events than the recommended initial view.", Hint: "Group events into phases and keep the initial view focused on milestones."})
		}
	case "evidence_v1":
		summary.Claims = len(array(data, "claims"))
		summary.Sources = len(array(data, "sources"))
		summary.Links = len(array(data, "links"))
		if summary.Claims+summary.Sources > design.MaxInitialNodes {
			quality -= 22
			warnings = append(warnings, Warning{Code: "evidence_board_dense", Message: "Evidence board has too many claims and sources for one initial view.", Hint: "Group sources by kind or reliability and show only decisive claims first."})
		}
	case "matrix_v1":
		summary.Items = len(array(data, "items"))
		if summary.Items > design.MaxInitialNodes {
			quality -= 20
			warnings = append(warnings, Warning{Code: "matrix_items_high", Message: "Matrix has more items than the recommended initial view.", Hint: "Use filters, categories, or high-importance items for the first view."})
		}
	case "isometric_architecture_v1":
		quality = analyzeIsometricArchitecture(data, design, &summary, &warnings, &recommendations)
	case "uml_sequence_v1":
		quality = analyzeUMLSequence(data, design, &summary, &warnings, &recommendations)
	case "uml_class_v1":
		quality = analyzeSemanticCount(data, design, &summary, &warnings, "classes", "relationships", "uml_class_dense", "Class diagram has more classes or relationships than the recommended first view.")
	case "uml_state_machine_v1":
		quality = analyzeSemanticCount(data, design, &summary, &warnings, "states", "transitions", "uml_state_machine_dense", "State machine has more states or transitions than the recommended first view.")
	case "uml_activity_v1":
		quality = analyzeSemanticCount(data, design, &summary, &warnings, "actions", "flows", "uml_activity_dense", "Activity diagram has more actions or flows than the recommended first view.")
	case "uml_component_deployment_v1":
		quality = analyzeSemanticCount(data, design, &summary, &warnings, "components", "links", "uml_component_dense", "Component deployment diagram has more components or links than the recommended first view.")
	}
	quality -= applyTemplateQualityRules(tpl, data, design, rules, &summary, &warnings, &recommendations)
	quality -= analyzeVisualGuidance(data, tpl.InputSchemaKind, design, &summary, &warnings, &recommendations)
	markStats := mark.Analyze(templateDir, tpl.InputSchemaKind, data)
	for _, warning := range markStats.Warnings {
		warnings = append(warnings, Warning{
			Code:        warning.Code,
			Severity:    warning.Severity,
			Path:        warning.Path,
			Message:     warning.Message,
			Suggestion:  warning.Suggestion,
			AutoFixHint: warning.AutoFixHint,
			Details:     warning.Details,
		})
	}
	quality -= markQualityPenalty(markStats.Warnings)
	if quality < 0 {
		quality = 0
	}
	return quality, summary, warnings, recommendations
}

func markQualityPenalty(warnings []mark.Warning) int {
	penalty := 0
	for _, warning := range warnings {
		switch strings.ToLower(warning.Severity) {
		case "error":
			penalty += 10
		case "warning":
			penalty += 4
		case "info":
			penalty += 1
		default:
			penalty += 2
		}
	}
	if penalty > 22 {
		return 22
	}
	return penalty
}

func analyzeVisualGuidance(data map[string]any, kind string, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	knownIDs := collectVisualReferenceIDs(kind, data)
	total := countSemanticItems(data)
	visual, _ := data["visual"].(map[string]any)
	if visual == nil {
		if total > maxInt(10, design.MaxInitialNodes/2) {
			*warnings = append(*warnings, Warning{Code: "visual_guidance_missing", Severity: "warning", Message: "Input does not include visual guidance for first-view focus, hidden detail, or annotations.", Hint: "Add visual.goal, visual.initial_focus_ids, visual.hidden_detail_ids, visual.narrative_steps, and visual.annotations so the renderer does not have to show everything at once."})
			recommendations.AddFields = appendUnique(recommendations.AddFields, "visual")
			recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.initial_focus_ids")
			recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.annotations")
			return 12
		}
		return 0
	}
	penalty := 0
	focusIDs, focusUnknown := visualStringRefs(visual, "initial_focus_ids", knownIDs)
	hiddenIDs, hiddenUnknown := visualStringRefs(visual, "hidden_detail_ids", knownIDs)
	annotations := objectArrayFromValue(visual["annotations"])
	steps := objectArrayFromValue(visual["narrative_steps"])
	summary.VisualFocusIDs = len(focusIDs)
	summary.VisualHiddenDetails = len(hiddenIDs)
	summary.VisualNarrativeSteps = len(steps)
	summary.VisualAnnotations = len(annotations)
	unknownRefs := append([]string{}, focusUnknown...)
	unknownRefs = append(unknownRefs, hiddenUnknown...)
	for _, step := range steps {
		_, unknown := visualStringRefs(step, "focus_ids", knownIDs)
		unknownRefs = append(unknownRefs, unknown...)
	}
	for _, annotation := range annotations {
		targetID := stringField(annotation, "target_id")
		if targetID != "" && len(knownIDs) > 0 && !knownIDs[targetID] {
			unknownRefs = appendCapped(unknownRefs, targetID, 12)
		}
	}
	if len(knownIDs) > 0 {
		knownVisualRefs := len(focusIDs) + len(hiddenIDs)
		for _, annotation := range annotations {
			if targetID := stringField(annotation, "target_id"); targetID != "" && knownIDs[targetID] {
				knownVisualRefs++
			}
		}
		summary.VisualReferenceCoverage = round2(float64(knownVisualRefs) / float64(maxInt(1, knownVisualRefs+len(unknownRefs))))
	}
	if len(unknownRefs) > 0 {
		penalty += 18
		summary.VisualUnknownRefs = duplicateFree(unknownRefs, 12)
		*warnings = append(*warnings, Warning{Code: "visual_guidance_unknown_refs", Severity: "error", Message: "Some visual guidance ids do not exist in the input.", Hint: "Use ids from the selected template schema objects when filling visual focus, hidden detail, narrative steps, and annotations.", Details: summary.VisualUnknownRefs})
	}
	if strings.TrimSpace(stringField(visual, "goal")) == "" && total > 8 {
		penalty += 6
		*warnings = append(*warnings, Warning{Code: "visual_goal_missing", Severity: "info", Message: "Visual guidance does not state what the viewer should understand first.", Hint: "Set visual.goal to one clear sentence describing the intended first impression."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.goal")
	}
	if len(focusIDs) == 0 && total > 8 {
		penalty += 10
		*warnings = append(*warnings, Warning{Code: "visual_focus_missing", Severity: "warning", Message: "Visual guidance does not name initial focus ids.", Hint: "Set visual.initial_focus_ids to the 2-5 entities, messages, events, claims, or items that should be emphasized first."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.initial_focus_ids")
	}
	if len(hiddenIDs) == 0 && total > design.MaxInitialNodes/2 {
		penalty += 6
		*warnings = append(*warnings, Warning{Code: "visual_hidden_detail_missing", Severity: "info", Message: "Visual guidance does not mark secondary detail for delayed reveal.", Hint: "Set visual.hidden_detail_ids for noisy leaf nodes, repetitive events, or low-value implementation details."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.hidden_detail_ids")
	}
	if len(annotations) == 0 && total > 8 {
		penalty += 8
		*warnings = append(*warnings, Warning{Code: "visual_annotations_missing", Severity: "warning", Message: "Visual guidance has no annotations, so the first view has no explanation anchors.", Hint: "Add 1-4 visual.annotations with target_id, label, summary, and priority."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.annotations")
	}
	if len(steps) == 0 && total > 12 {
		penalty += 6
		*warnings = append(*warnings, Warning{Code: "visual_narrative_missing", Severity: "info", Message: "Visual guidance has no narrative steps for progressive interpretation.", Hint: "Add visual.narrative_steps so the agent explains overview first and detail second."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "visual.narrative_steps")
	}
	return penalty
}

func analyzeIsometricArchitecture(data map[string]any, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	_ = design
	zones := objectArray(data, "zones")
	entities := objectArray(data, "entities")
	links := objectArray(data, "links")
	summary.Zones = len(zones)
	summary.Entities = len(entities)
	summary.Links = len(links)
	summary.Nodes = len(entities)
	summary.Edges = len(links)

	quality := 100
	penalty := 0
	add := func(code, severity, path, message, suggestion string) {
		*warnings = append(*warnings, isometricWarning(code, severity, path, message, suggestion))
		penalty += isometricWarningPenalty(severity)
	}

	zoneIDs := map[string]bool{}
	var zoneRects []isometricRect
	if len(zones) == 0 {
		add("isometric_zones_missing", "warning", "$.zones", "Isometric architecture input does not define any zones.", "Add zones[] with id, label, and bounds so the architecture scene has visible boundaries.")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "zones[]")
	}
	for i, zone := range zones {
		id := firstString(zone, "id")
		if id != "" {
			zoneIDs[id] = true
		}
		rect, ok := isometricBounds(zone)
		if !ok {
			add("isometric_zone_bounds_missing", "warning", "$.zones["+intString(i)+"].bounds", "A zone is missing numeric bounds for the isometric floor plan.", "Set zone.bounds.x/y/w/h to numbers.")
			continue
		}
		rect.ID = id
		zoneRects = append(zoneRects, rect)
	}
	if overlaps := overlappingRects(zoneRects); len(overlaps) > 0 {
		add("isometric_zone_overlap_risk", "warning", "$.zones", "Some zone bounds overlap and may make boundaries hard to read.", "Adjust zone.bounds so architecture areas have clear separation.")
		(*warnings)[len(*warnings)-1].Details = overlaps
	}

	entityIDs := map[string]bool{}
	missingPosition := 0
	fallbackRisk := []string{}
	var entityRects []isometricRect
	for i, entity := range entities {
		id := firstString(entity, "id")
		if id != "" {
			entityIDs[id] = true
		}
		path := "$.entities[" + intString(i) + "]"
		kind := firstString(entity, "kind")
		if kind == "" || isGenericKind(kind) {
			add("isometric_entity_kind_missing", "warning", path+".kind", "An architecture entity lacks a specific kind.", "Set entity.kind to service, api, database, queue, stream, worker, gateway, client, external, storage, or deployment.")
			fallbackRisk = appendCapped(fallbackRisk, path, 12)
		}
		if firstString(entity, "label", "name", "title") == "" {
			add("isometric_entity_label_missing", "warning", path+".label", "An architecture entity lacks a display label.", "Set entity.label to a short label that fits above the isometric mark.")
		}
		zone := firstString(entity, "zone")
		if zone != "" && !zoneIDs[zone] {
			add("isometric_entity_zone_unknown", "warning", path+".zone", "An entity references a zone id that does not exist.", "Set entity.zone to one of zones[].id or add the missing zone.")
		}
		pos, hasPos := isometricPosition(entity)
		if !hasPos {
			missingPosition++
			add("isometric_entity_position_missing", "warning", path+".position", "An entity is missing an explicit isometric position.", "Set entity.position.x/y so the renderer does not auto-place architecture marks.")
		}
		size := isometricSize(entity)
		if hasPos {
			entityRects = append(entityRects, isometricRect{ID: id, X: pos.X, Y: pos.Y, W: size.W, H: size.D})
		}
		if !isometricHasMark(entity) {
			fallbackRisk = appendCapped(fallbackRisk, path, 12)
		}
	}
	if overlaps := overlappingRects(entityRects); len(overlaps) > 0 {
		add("isometric_entity_overlap_risk", "warning", "$.entities", "Some positioned entities may overlap in the isometric scene.", "Move entity.position values apart or reduce entity.size.")
		(*warnings)[len(*warnings)-1].Details = overlaps
	}
	if len(entities) > 0 && missingPosition*100 > len(entities)*35 {
		add("isometric_too_many_auto_positions", "warning", "$.entities", "Too many architecture entities rely on automatic placement.", "Position important entities explicitly with entity.position.x/y.")
	}
	if len(fallbackRisk) > 0 {
		add("isometric_fallback_sphere_risk", "warning", "$.entities", "Some architecture entities may fall back to generic sphere marks.", "Add specific entity.kind plus presentation.shape, presentation.icon, provider, service, or platform.")
		(*warnings)[len(*warnings)-1].Details = duplicateFree(fallbackRisk, 12)
	}

	for i, link := range links {
		path := "$.links[" + intString(i) + "]"
		from := firstString(link, "from")
		to := firstString(link, "to")
		if !entityIDs[from] || !entityIDs[to] {
			add("isometric_link_endpoint_unknown", "warning", path, "A link references an endpoint that is not present in entities[].", "Set link.from and link.to to existing entity ids.")
		}
		if value, ok := link["directed"].(bool); !ok || !value {
			add("isometric_link_direction_missing", "warning", path+".directed", "An architecture link is not marked as directional.", "Set link.directed=true for calls, reads, writes, events, dependencies, and deployment flows.")
		}
		if !isometricLinkHasArrow(link) {
			add("isometric_link_arrow_missing", "warning", path+".presentation.arrow", "A directional architecture link does not declare a visible arrow.", "Set link.presentation.arrow=forward or reverse.")
		}
		if label := firstString(link, "label", "name", "title"); len(label) > 48 {
			add("isometric_link_label_too_long", "warning", path+".label", "A link label is too long for the isometric overview.", "Shorten link.label and move detail into summary.")
		}
	}

	if !isometricBaseGridEnabled(data) {
		add("isometric_missing_base_grid", "warning", "$.canvas.grid.enabled", "Isometric architecture input does not enable a base grid.", "Set canvas.grid.enabled=true so the scene has a base plane and scale reference.")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "canvas.grid.enabled")
	}
	if isometricGenericGraph(data, entities) {
		add("isometric_generic_graph_detected", "warning", "$", "Input looks like a generic graph rather than an isometric architecture scene.", "Use zones/entities/links with architecture-specific entity kinds instead of nodes/edges.")
	}
	if isometricStarfieldTheme(data) {
		add("isometric_starfield_theme_detected", "warning", "$.theme", "Starfield themes do not match the architecture isometric scene contract.", "Use theme=architecture_light for a grounded architecture map.")
	}

	quality -= penalty
	if quality < 0 {
		return 0
	}
	return quality
}

func isometricWarning(code, severity, path, message, suggestion string) Warning {
	if path == "" {
		path = "$"
	}
	return Warning{
		Code:       code,
		Severity:   severity,
		Path:       path,
		Message:    message,
		Suggestion: suggestion,
		Hint:       suggestion,
		AutoFixHint: map[string]any{
			"action": code,
			"path":   path,
		},
	}
}

func isometricWarningPenalty(severity string) int {
	switch strings.ToLower(severity) {
	case "error":
		return 12
	case "warning":
		return 5
	case "info":
		return 2
	default:
		return 3
	}
}

type isometricRect struct {
	ID string
	X  float64
	Y  float64
	W  float64
	H  float64
}

type isometricPoint struct {
	X float64
	Y float64
}

type isometricSizeValue struct {
	W float64
	D float64
}

func isometricBounds(zone map[string]any) (isometricRect, bool) {
	bounds, _ := zone["bounds"].(map[string]any)
	if bounds == nil {
		return isometricRect{}, false
	}
	x, okX := numericValue(bounds["x"])
	y, okY := numericValue(bounds["y"])
	w, okW := numericValue(bounds["w"])
	h, okH := numericValue(bounds["h"])
	return isometricRect{X: x, Y: y, W: w, H: h}, okX && okY && okW && okH
}

func isometricPosition(entity map[string]any) (isometricPoint, bool) {
	position, _ := entity["position"].(map[string]any)
	if position == nil {
		return isometricPoint{}, false
	}
	x, okX := numericValue(position["x"])
	y, okY := numericValue(position["y"])
	return isometricPoint{X: x, Y: y}, okX && okY
}

func isometricSize(entity map[string]any) isometricSizeValue {
	size, _ := entity["size"].(map[string]any)
	w, okW := numericValue(size["w"])
	d, okD := numericValue(size["d"])
	if !okW || w <= 0 {
		w = 3
	}
	if !okD || d <= 0 {
		d = 3
	}
	return isometricSizeValue{W: w, D: d}
}

func overlappingRects(rects []isometricRect) []string {
	var out []string
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			a, b := rects[i], rects[j]
			if a.W <= 0 || a.H <= 0 || b.W <= 0 || b.H <= 0 {
				continue
			}
			if a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y {
				label := strings.TrimSpace(a.ID + "/" + b.ID)
				if label == "/" {
					label = intString(i) + "/" + intString(j)
				}
				out = appendCapped(out, label, 12)
			}
		}
	}
	return out
}

func isometricHasMark(entity map[string]any) bool {
	if firstString(entity, "provider", "service", "platform") != "" {
		return true
	}
	if kind := firstString(entity, "kind"); kind != "" && !isGenericKind(kind) {
		return true
	}
	presentation, _ := entity["presentation"].(map[string]any)
	return firstString(presentation, "shape", "mesh", "icon") != ""
}

func isometricLinkHasArrow(link map[string]any) bool {
	presentation, _ := link["presentation"].(map[string]any)
	arrow := strings.ToLower(firstString(presentation, "arrow"))
	return arrow != "" && arrow != "none" && arrow != "false"
}

func isometricBaseGridEnabled(data map[string]any) bool {
	canvas, _ := data["canvas"].(map[string]any)
	grid, _ := canvas["grid"].(map[string]any)
	enabled, _ := grid["enabled"].(bool)
	return enabled
}

func isometricGenericGraph(data map[string]any, entities []map[string]any) bool {
	if len(objectArray(data, "nodes")) > 0 || len(objectArray(data, "edges")) > 0 {
		return true
	}
	generic := 0
	for _, entity := range entities {
		if isGenericKind(firstString(entity, "kind")) {
			generic++
		}
	}
	return len(entities) > 0 && generic*2 >= len(entities)
}

func isometricStarfieldTheme(data map[string]any) bool {
	values := []string{firstString(data, "theme", "scene_theme")}
	for _, field := range []string{"view", "renderHints"} {
		obj, _ := data[field].(map[string]any)
		values = append(values, firstString(obj, "theme", "sceneTheme", "scene_theme"))
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), "starfield") {
			return true
		}
	}
	return false
}

func analyzeUMLSequence(data map[string]any, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	participants := objectArray(data, "participants")
	messages := objectArray(data, "messages")
	phases := objectArray(data, "phases")
	activations := objectArray(data, "activations")
	fragments := objectArray(data, "fragments")
	summary.Participants = len(participants)
	summary.Messages = len(messages)
	summary.Phases = len(phases)
	summary.Activations = len(activations)
	summary.Fragments = len(fragments)
	quality := 100
	if len(participants) > 10 {
		quality -= 18
		*warnings = append(*warnings, Warning{Code: "uml_participants_high", Severity: "warning", Message: "Sequence diagram has more participants than a readable 3D overview.", Hint: "Merge low-level classes into subsystem participants or split the flow into multiple sequence scenes."})
	}
	if len(messages) > design.MaxInitialEdges {
		quality -= 20
		*warnings = append(*warnings, Warning{Code: "uml_messages_high", Severity: "warning", Message: "Sequence diagram has more messages than the recommended initial view.", Hint: "Use fragments for loops/retries and keep only important calls in the first scene."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "fragments[]")
	}
	phaseIDs := map[string]bool{}
	for _, phase := range phases {
		if id := stringField(phase, "id"); id != "" {
			phaseIDs[id] = true
		}
	}
	fanout := map[string]int{}
	longLabels := []string{}
	for _, message := range messages {
		from := stringField(message, "from")
		to := stringField(message, "to")
		fanout[from]++
		fanout[to]++
		label := displayLabelField(message)
		if len(label) > 48 {
			longLabels = appendCapped(longLabels, label, 8)
		}
		if len(phaseIDs) > 0 && stringField(message, "phase") == "" {
			summary.MessagesWithoutPhase++
		}
	}
	if summary.MessagesWithoutPhase > 0 {
		quality -= minInt(16, 5+summary.MessagesWithoutPhase)
		*warnings = append(*warnings, Warning{Code: "uml_messages_missing_phase", Severity: "warning", Message: "Some sequence messages are not assigned to a phase.", Hint: "Set messages[].phase so the renderer can color the flow and build a meaningful legend."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "messages[].phase")
	}
	if len(activations) == 0 && len(messages) >= 6 {
		quality -= 12
		*warnings = append(*warnings, Warning{Code: "uml_activations_missing", Severity: "warning", Message: "Sequence input does not declare activations, so execution spans are less clear.", Hint: "Add activations with participant_id, start_order, and end_order for important execution windows."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "activations[]")
	}
	if len(fragments) == 0 && len(messages) >= 12 {
		quality -= 8
		*warnings = append(*warnings, Warning{Code: "uml_fragments_missing", Severity: "info", Message: "Long sequence input has no loop/alt/opt fragments.", Hint: "Use fragments to summarize loops, retries, alternatives, and parallel sections instead of expanding every repeated message."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "fragments[]")
	}
	if len(longLabels) > 0 {
		quality -= 8
		summary.LongLabels = longLabels
		*warnings = append(*warnings, Warning{Code: "uml_message_labels_too_long", Severity: "warning", Message: "Some sequence message labels are too long for 3D billboards.", Hint: "Use short method/event labels and move full signatures, files, or code references into metadata.", Details: longLabels})
		recommendations.RewriteLabels = append(recommendations.RewriteLabels, longLabels...)
	}
	summary.ParticipantFanout = highFanoutNodes(fanout, 12)
	if len(summary.ParticipantFanout) > 0 {
		quality -= 6
		*warnings = append(*warnings, Warning{Code: "uml_participant_fanout_high", Severity: "info", Message: "Some participants receive many messages and may become visual bottlenecks.", Hint: "Use phases, focus scenes, or split high-fanout participants into clearer subsystem roles.", Details: summary.ParticipantFanout})
	}
	return quality
}

func analyzeSemanticCount(data map[string]any, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, primaryField, edgeField, code, message string) int {
	primary := len(array(data, primaryField))
	edges := len(array(data, edgeField))
	switch primaryField {
	case "classes":
		summary.Classes = primary
		summary.Relationships = edges
	case "states":
		summary.States = primary
		summary.Transitions = edges
	case "actions":
		summary.Actions = primary
		summary.Flows = edges
	case "components":
		summary.Components = primary
		summary.Links = edges
		summary.Deployments = len(array(data, "deployments"))
	}
	if primary > design.MaxInitialNodes || edges > design.MaxInitialEdges {
		*warnings = append(*warnings, Warning{Code: code, Severity: "warning", Message: message, Hint: "Split the diagram into focused scenes or hide low-importance details from the initial view."})
		return 78
	}
	return 100
}

func analyzeGraph(data map[string]any, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	nodes := objectArray(data, "nodes")
	edges := objectArray(data, "edges")
	groups := objectArray(data, "groups")
	events := objectArray(data, "events")
	summary.Nodes = len(nodes)
	summary.Edges = len(edges)
	summary.Groups = len(groups)
	summary.Events = len(events)
	summary.EdgeDensity = densityLabel(len(nodes), len(edges))
	summary.LabelPressure = pressureLabel(len(nodes), design.MaxInitialNodes)
	grouped := 0
	nodeIDs := map[string]bool{}
	groupLabels := map[string]string{}
	groupSizes := map[string]int{}
	degree := map[string]int{}
	edgeKinds := map[string]int{}
	edgeLabels := map[string]int{}
	labels := map[string]int{}
	missingImportance := 0
	missingVisibility := 0
	overviewEdges := 0
	for _, group := range groups {
		id := stringField(group, "id")
		if id == "" {
			continue
		}
		if label := displayLabelField(group); label != "" {
			groupLabels[id] = label
		}
	}
	for _, node := range nodes {
		id := stringField(node, "id")
		if id != "" {
			nodeIDs[id] = true
		}
		if groupKey := groupKeyField(node); groupKey != "" {
			grouped++
			groupSizes[groupKey]++
		}
		displayLabel := displayLabelField(node)
		if displayLabel == "" {
			summary.MissingLabels++
			summary.FallbackIDLabels = appendCapped(summary.FallbackIDLabels, labelField(node), 8)
		}
		label := labelField(node)
		if label != "" {
			labels[label]++
			if len(label) > 42 {
				summary.LongLabels = appendCapped(summary.LongLabels, label, 8)
			}
		}
		if !hasImportance(node) {
			missingImportance++
		}
	}
	for _, edge := range edges {
		from := stringField(edge, "from")
		to := stringField(edge, "to")
		if from != "" {
			degree[from]++
		}
		if to != "" {
			degree[to]++
		}
		if kind := stringField(edge, "kind"); kind != "" {
			edgeKinds[kind]++
		}
		if label := labelField(edge); label != "" {
			edgeLabels[label]++
		}
		if !hasImportance(edge) {
			missingImportance++
		}
		visibility := stringField(edge, "visibility")
		if visibility == "" {
			missingVisibility++
			overviewEdges++
		} else if visibility != "hidden" && visibility != "detail" {
			overviewEdges++
		}
	}
	if len(nodes) > 0 {
		summary.GroupCoverage = round2(float64(grouped) / float64(len(nodes)))
		connected := 0
		for _, node := range nodes {
			if degree[stringField(node, "id")] > 0 {
				connected++
			} else {
				summary.OrphanNodeCount++
				summary.OrphanNodes = appendCapped(summary.OrphanNodes, labelField(node), 8)
			}
		}
		summary.RelationCoverage = round2(float64(connected) / float64(len(nodes)))
	}
	for groupKey, size := range groupSizes {
		if size > summary.LargestGroupSize {
			summary.LargestGroupSize = size
		}
		groupLabel := groupLabels[groupKey]
		if groupLabel == "" {
			groupLabel = groupKey
		}
		if size >= maxInt(6, len(nodes)/4) && len(nodes) > 12 {
			summary.LargeGroups = appendCapped(summary.LargeGroups, groupLabel+" ("+intString(size)+")", 8)
		}
		if isGenericGroupLabel(groupLabel) {
			summary.GenericGroups = appendCapped(summary.GenericGroups, groupLabel, 8)
		}
	}
	if len(events) > 0 {
		knownEvents := 0
		for _, event := range events {
			nodeID := firstString(event, "node_id", "node", "target_node_id")
			if nodeID == "" {
				summary.EventsWithoutNodeID++
				continue
			}
			if nodeIDs[nodeID] {
				knownEvents++
			} else {
				summary.EventsWithoutKnownNode++
			}
		}
		summary.EventNodeCoverage = round2(float64(knownEvents) / float64(len(events)))
	}
	summary.EdgeKindCount = len(edgeKinds)
	summary.DominantEdgeKinds = topCounts(edgeKinds, 5)
	summary.DuplicateLabels = duplicateNames(labels, 8)
	summary.MissingImportance = missingImportance
	summary.MissingVisibility = missingVisibility
	highFanout := highFanoutNodes(degree, 20)
	summary.HighFanoutNodes = highFanout
	summary.VisibleNodes = initialVisibleNodes(nodes, groups, design)
	summary.VisibleEdges = minInt(overviewEdges, design.MaxInitialEdges)
	quality := 100
	if len(nodes) > design.MaxInitialNodes && len(groups) == 0 {
		quality -= 28
		*warnings = append(*warnings, Warning{Code: "missing_groups", Severity: "error", Message: "Large graph input has no groups, so the first view has to explain too many objects at once.", Hint: "Group nodes by module/package/component and render groups collapsed initially."})
	}
	if len(nodes) > design.MaxInitialNodes {
		quality -= 16
		*warnings = append(*warnings, Warning{Code: "visible_nodes_high", Severity: "warning", Message: "The graph has more nodes than the recommended initial view.", Hint: "Set visible=false for low-importance detail nodes or collapse them under groups."})
	}
	if len(edges) > design.MaxInitialEdges || summary.EdgeDensity == "high" {
		quality -= 22
		*warnings = append(*warnings, Warning{Code: "graph_density_high", Severity: "error", Message: "Edge density is high for an initial overview, so relationships will compete visually.", Hint: "Hide low-importance or detail-only edge types and keep only summary relationships visible first.", Details: countNames(summary.DominantEdgeKinds)})
		recommendations.HideEdgeTypes = lowValueEdgeKinds(edgeKinds)
	}
	if summary.GroupCoverage < 0.5 && len(nodes) > design.MaxInitialNodes {
		quality -= 14
		*warnings = append(*warnings, Warning{Code: "group_coverage_low", Severity: "warning", Message: "Most graph nodes are not assigned to a group.", Hint: "Set parent_id, group_id, group, module, or package so the renderer can collapse related nodes."})
	}
	if len(summary.LargeGroups) > 0 {
		quality -= 8
		*warnings = append(*warnings, Warning{Code: "groups_too_coarse", Severity: "warning", Message: "Some collapsed groups contain too many children to explain when expanded.", Hint: "Split coarse groups into scenario-specific subgroups or add nested parent_id/group_id levels.", Details: summary.LargeGroups})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "nodes[].parent_id")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "groups[].label")
	}
	if len(summary.GenericGroups) > 0 {
		quality -= 7
		*warnings = append(*warnings, Warning{Code: "generic_group_labels", Severity: "warning", Message: "Some groups use generic labels that do not explain their role in the visual story.", Hint: "Use scenario-specific group labels such as API Gateway, Build Scan, Approval Gate, Incident Source, or Release Check.", Details: summary.GenericGroups})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "groups[].label")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "groups[].summary")
	}
	if len(events) > 0 && summary.EventNodeCoverage < 0.8 {
		quality -= 12
		*warnings = append(*warnings, Warning{Code: "event_node_coverage_low", Severity: "warning", Message: "Many graph events are not attached to known nodes, so replay cannot explain which object changed.", Hint: "Set events[].node_id to an existing node id for every meaningful event.", Details: []string{"events_without_node_id=" + intString(summary.EventsWithoutNodeID), "events_without_known_node=" + intString(summary.EventsWithoutKnownNode)}})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "events[].node_id")
	}
	if len(highFanout) > 0 {
		quality -= 10
		*warnings = append(*warnings, Warning{Code: "high_fanout_nodes", Severity: "warning", Message: "Some nodes have very high fan-out.", Hint: "Represent high fan-out nodes as hubs or groups and hide detail edges until focus mode.", Details: highFanout})
	}
	if summary.MissingLabels > 0 {
		quality -= minInt(12, 4+summary.MissingLabels/4)
		*warnings = append(*warnings, Warning{Code: "missing_display_labels", Severity: "warning", Message: "Some graph nodes do not provide a display label or name, so the renderer falls back to technical ids.", Hint: "Add short nodes[].label or nodes[].name values and move full class names or paths into metadata.", Details: summary.FallbackIDLabels})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "nodes[].label")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "nodes[].name")
	}
	if summary.RelationCoverage > 0 && summary.RelationCoverage < 0.65 && len(nodes) > 12 {
		quality -= 14
		*warnings = append(*warnings, Warning{Code: "relation_coverage_low", Severity: "warning", Message: "Many nodes are isolated from the visible relationship graph.", Hint: "Either connect isolated nodes to a group/hub or move them out of the initial overview.", Details: summary.OrphanNodes})
	}
	if summary.OrphanNodeCount >= 3 && len(nodes) > 6 {
		quality -= minInt(18, 6+summary.OrphanNodeCount/3)
		*warnings = append(*warnings, Warning{Code: "orphan_nodes_high", Severity: "warning", Message: "Many graph nodes have no incoming or outgoing relationship.", Hint: "Add meaningful edges, assign isolated details to groups, or mark low-value isolated nodes as hidden from the initial view.", Details: summary.OrphanNodes})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "edges[].from")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "edges[].to")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "edges[].kind")
	}
	if missingVisibility > design.MaxInitialEdges/2 && len(edges) > design.MaxInitialEdges/2 {
		quality -= 10
		*warnings = append(*warnings, Warning{Code: "missing_edge_visibility", Severity: "warning", Message: "Many edges do not declare visibility, so the renderer must guess what belongs in the overview.", Hint: "Set important relationships to visibility=overview and noisy details to visibility=detail or hidden."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "edges[].visibility")
	}
	if missingImportance > (len(nodes)+len(edges))/2 && len(nodes)+len(edges) > design.MaxInitialNodes {
		quality -= 8
		*warnings = append(*warnings, Warning{Code: "missing_importance", Severity: "warning", Message: "Most nodes or edges do not declare importance, so visual emphasis is weak.", Hint: "Add importance values to modules, hubs, risks, and summary relationships."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "nodes[].importance")
		recommendations.AddFields = appendUnique(recommendations.AddFields, "edges[].importance")
	}
	if len(summary.LongLabels) > 0 {
		quality -= 8
		*warnings = append(*warnings, Warning{Code: "labels_too_long", Severity: "warning", Message: "Some labels are too long for a readable 3D overview.", Hint: "Use short display labels and move full paths or class names into metadata.", Details: summary.LongLabels})
		recommendations.RewriteLabels = append(recommendations.RewriteLabels, summary.LongLabels...)
	}
	if dominantRatio(edgeKinds, len(edges)) >= 0.65 && len(edges) > 20 {
		quality -= 12
		*warnings = append(*warnings, Warning{Code: "relation_semantics_flat", Severity: "warning", Message: "Most relationships use the same edge kind, so the graph does not explain why objects are connected.", Hint: "Use distinct edge kinds such as owns, calls, imports, emits, depends_on, tests, or deploys_to."})
	}
	if dominantRatio(edgeLabels, len(edges)) >= 0.7 && len(edges) > 20 {
		quality -= 6
		*warnings = append(*warnings, Warning{Code: "edge_labels_repetitive", Severity: "info", Message: "Most edge labels repeat the same word, which adds clutter without adding meaning.", Hint: "Omit repetitive edge labels or keep them only on selected/focus relationships."})
	}
	if _, ok := data["initial_view"].(map[string]any); !ok && len(nodes) > design.MaxInitialNodes {
		quality -= 8
		*warnings = append(*warnings, Warning{Code: "initial_view_missing", Severity: "warning", Message: "Large graph input does not define an initial_view policy.", Hint: "Add initial_view.mode=overview with max_nodes, max_edges, and collapse_groups=true."})
		recommendations.AddFields = appendUnique(recommendations.AddFields, "initial_view")
	}
	recommendations.FocusCandidates = topDegreeNodes(degree, 6)
	return quality
}

func applyTemplateQualityRules(tpl manifest.TemplateManifest, data map[string]any, design manifest.VisualDesign, rules authoring.QualityRules, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	penalty += analyzeGeneralQuality(data, rules, design, warnings, recommendations)
	switch strings.ToLower(tpl.InputSchemaKind) {
	case "uml_sequence_v1":
		penalty += analyzeUMLSequenceQuality(data, rules, warnings, recommendations)
	case "graph_v1", "graph_events_v1":
		penalty += analyzeGraphQuality(data, tpl.Category, design, warnings, recommendations)
	case "timeline_v1":
		penalty += analyzeTimelineQuality(data, warnings, recommendations)
	case "matrix_v1":
		penalty += analyzeMatrixQuality(data, warnings, recommendations)
	case "evidence_v1":
		penalty += analyzeEvidenceQuality(data, warnings, recommendations)
	}
	return penalty
}

func analyzeGeneralQuality(data map[string]any, rules authoring.QualityRules, design manifest.VisualDesign, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	maxOverview := rules.Label.MaxOverviewLabelChars
	if maxOverview <= 0 {
		maxOverview = 32
	}
	items := qualityObjects(data)
	longLabels := []string{}
	missingImportance := 0
	visibleCount := 0
	colors := map[string]int{}
	for _, item := range items {
		label := displayLabelField(item.Obj)
		if label == "" {
			label = stringField(item.Obj, "id")
		}
		if len(label) > maxOverview {
			longLabels = appendCapped(longLabels, item.Path+"="+label, 10)
			if rules.Label.RequireSummaryForLongLabel && stringField(item.Obj, "summary") == "" && stringField(item.Obj, "details") == "" {
				penalty += 2
				addWarning(warnings, Warning{Code: "summary_missing_for_long_label", Severity: "warning", Path: item.Path, Message: "A long label does not provide summary/details for hover or inspector use.", Suggestion: "Shorten the label and put the full explanation in summary or details.", AutoFixHint: map[string]any{"action": "add_summary", "path": item.Path}})
			}
		}
		if !hasImportance(item.Obj) && item.Kind != "phase" {
			missingImportance++
		}
		visibility := normalizeVisibility(firstString(item.Obj, "visibility"))
		if visibility == "" || visibility == "overview" || visibility == "normal" {
			visibleCount++
		}
		if color := firstString(item.Obj, "color"); color != "" {
			colors[color]++
		}
		if isGenericKind(stringField(item.Obj, "kind")) {
			penalty += 2
			addWarning(warnings, Warning{Code: "generic_kind", Severity: "warning", Path: item.Path + ".kind", Message: "An item uses a generic kind that does not explain its semantic role.", Suggestion: "Use a specific kind such as service, controller, event_stream, evidence, risk, milestone, validates, emits, or subscribes.", AutoFixHint: map[string]any{"action": "replace_generic_kind", "path": item.Path + ".kind"}})
		}
	}
	if len(longLabels) > 0 {
		penalty += 6
		addWarning(warnings, Warning{Code: "label_too_long", Severity: "warning", Message: "Some labels are too long for an overview label layer.", Suggestion: "Keep overview labels short and move implementation signatures, paths, and explanations into summary/details.", Details: longLabels, AutoFixHint: map[string]any{"action": "shorten_labels"}})
		recommendations.RewriteLabels = append(recommendations.RewriteLabels, longLabels...)
	}
	if missingImportance > len(items)/3 && len(items) > 10 {
		penalty += 8
		addWarning(warnings, Warning{Code: "importance_missing", Severity: "warning", Message: "Many semantic objects do not declare importance, so label and emphasis decisions are weak.", Suggestion: "Add importance 0..1 to overview-critical objects and leave low-value detail lower than 0.35.", AutoFixHint: map[string]any{"action": "add_importance"}})
	}
	if visibleCount > design.MaxInitialNodes && design.MaxInitialNodes > 0 {
		penalty += 8
		addWarning(warnings, Warning{Code: "too_many_visible_items", Severity: "warning", Message: "Too many objects are visible in the first view.", Suggestion: "Set visibility=detail/hidden for low-value objects and use visual.initial_focus_ids for the story path.", AutoFixHint: map[string]any{"action": "mark_low_value_detail"}})
	}
	if len(colors) > 12 {
		penalty += 4
		addWarning(warnings, Warning{Code: "too_many_colors", Severity: "info", Message: "The input uses many explicit colors, which can make the legend unstable.", Suggestion: "Use stable phase/kind/status palettes and reserve explicit colors for phases or major groups.", AutoFixHint: map[string]any{"action": "reduce_palette"}})
	}
	return penalty
}

func analyzeUMLSequenceQuality(data map[string]any, rules authoring.QualityRules, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	participants := objectArray(data, "participants")
	messages := objectArray(data, "messages")
	phases := objectArray(data, "phases")
	fragments := objectArray(data, "fragments")
	participantIDs := map[string]bool{}
	for i, participant := range participants {
		id := stringField(participant, "id")
		if id != "" {
			participantIDs[id] = true
		}
		path := "$.participants[" + intString(i) + "]"
		if firstString(participant, "display_name", "displayName") == "" {
			penalty += 4
			addWarning(warnings, Warning{Code: "participant_display_name_missing", Severity: "warning", Path: path, Message: "A participant does not define a display name.", Suggestion: "Set display_name/displayName to a short semantic lifeline label.", AutoFixHint: map[string]any{"action": "add_display_name", "participant_id": id}})
		}
		if stringField(participant, "subtitle") == "" {
			penalty += 2
			addWarning(warnings, Warning{Code: "participant_subtitle_missing", Severity: "info", Path: path, Message: "A participant lacks subtitle context.", Suggestion: "Add subtitle to explain the lifeline role, such as screen, API, manager, event stream, or storage.", AutoFixHint: map[string]any{"action": "add_subtitle", "participant_id": id}})
		}
		if !hasLane(participant) {
			penalty += 2
			addWarning(warnings, Warning{Code: "participant_lane_missing", Severity: "info", Path: path, Message: "A participant lacks lane_index/presentation.laneIndex.", Suggestion: "Set lane_index so related lifelines are laid out in stable left-to-right order.", AutoFixHint: map[string]any{"action": "add_lane_index", "participant_id": id}})
		}
		if !hasDepth(participant) {
			penalty += 2
			addWarning(warnings, Warning{Code: "participant_depth_missing", Severity: "info", Path: path, Message: "A participant lacks depth/presentation.depth.", Suggestion: "Set depth to encode frontend/backend/provider/runtime tier instead of random 3D placement.", AutoFixHint: map[string]any{"action": "add_depth", "participant_id": id}})
		}
		if firstString(participant, "color") == "" && !nestedStringExists(participant, "presentation", "color") {
			penalty += 2
			addWarning(warnings, Warning{Code: "participant_color_missing", Severity: "info", Path: path, Message: "A participant lacks a stable color.", Suggestion: "Set color/presentation.color for major lifelines so labels and cylinders stay recognizable.", AutoFixHint: map[string]any{"action": "add_participant_color", "participant_id": id}})
		}
	}
	phaseIDs := map[string]bool{}
	phaseColorMissing := 0
	for i, phase := range phases {
		id := firstString(phase, "id", "label", "name")
		if id != "" {
			phaseIDs[id] = true
		}
		if firstString(phase, "color") == "" {
			phaseColorMissing++
			addWarning(warnings, Warning{Code: "phase_color_missing", Severity: "warning", Path: "$.phases[" + intString(i) + "].color", Message: "A sequence phase does not define a color.", Suggestion: "Add phases[].color so the renderer can create a stable legend and phase-coded message paths.", AutoFixHint: map[string]any{"action": "add_phase_color", "phase_id": id}})
		}
	}
	penalty += minInt(18, phaseColorMissing*3)
	orders := map[string]bool{}
	pairCounts := map[string]int{}
	importanceValues := map[string]int{}
	phaseValues := map[string]int{}
	calls := map[string]int{}
	loopMessages := 0
	returnMessages := 0
	for i, message := range messages {
		path := "$.messages[" + intString(i) + "]"
		order := intStringFromAny(message["order"])
		if order != "" {
			if orders[order] {
				penalty += 8
				addWarning(warnings, Warning{Code: "duplicate_order", Severity: "error", Path: path + ".order", Message: "Two sequence messages use the same order.", Suggestion: "Assign unique message order values so vertical time placement is deterministic.", AutoFixHint: map[string]any{"action": "renumber_messages"}})
			}
			orders[order] = true
		}
		from := stringField(message, "from")
		to := stringField(message, "to")
		if !participantIDs[from] || !participantIDs[to] {
			penalty += 8
			addWarning(warnings, Warning{Code: "unknown_participant_ref", Severity: "error", Path: path, Message: "A sequence message references an unknown participant.", Suggestion: "Set from/to to participant ids declared in participants[].id.", AutoFixHint: map[string]any{"action": "fix_participant_ref", "message_id": stringField(message, "id")}})
		}
		if from != "" || to != "" {
			pairCounts[from+"->"+to]++
		}
		phase := stringField(message, "phase")
		if phase == "" {
			penalty += 3
			addWarning(warnings, Warning{Code: "message_phase_missing", Severity: "warning", Path: path + ".phase", Message: "A message does not declare a phase.", Suggestion: "Set messages[].phase to one of phases[].id so color and legend encode the story stage.", AutoFixHint: map[string]any{"action": "add_message_phase", "message_id": stringField(message, "id")}})
		} else {
			phaseValues[phase]++
			if len(phaseIDs) > 0 && !phaseIDs[phase] {
				addWarning(warnings, Warning{Code: "message_phase_unknown", Severity: "warning", Path: path + ".phase", Message: "A message phase is not declared in phases[].", Suggestion: "Add the phase to phases[] with a label and color, or change messages[].phase to an existing phase id.", AutoFixHint: map[string]any{"action": "add_phase", "phase_id": phase}})
			}
		}
		if stringField(message, "kind") == "" {
			penalty += 2
			addWarning(warnings, Warning{Code: "message_kind_missing", Severity: "warning", Path: path + ".kind", Message: "A message does not declare kind.", Suggestion: "Set kind to sync, call, return, async, self, or loop.", AutoFixHint: map[string]any{"action": "add_message_kind", "message_id": stringField(message, "id")}})
		}
		curve := normalizeCurve(stringField(message, "curve"))
		if curve == "" {
			penalty += 2
			addWarning(warnings, Warning{Code: "message_curve_missing", Severity: "warning", Path: path + ".curve", Message: "A message does not declare a curve strategy.", Suggestion: "Set curve to arc, high_arc, return, self, loop, or straight to control 3D path shape.", AutoFixHint: map[string]any{"action": "add_message_curve", "message_id": stringField(message, "id")}})
		}
		if !hasImportance(message) {
			penalty += 2
			addWarning(warnings, Warning{Code: "message_importance_missing", Severity: "warning", Path: path + ".importance", Message: "A message lacks importance.", Suggestion: "Set importance 0..1 so thickness/glow and label visibility can distinguish key calls from detail.", AutoFixHint: map[string]any{"action": "add_message_importance", "message_id": stringField(message, "id")}})
		} else {
			importanceValues[importanceBucket(message)]++
		}
		if normalizeLabelPriority(message) == "" {
			penalty += 2
			addWarning(warnings, Warning{Code: "message_label_priority_missing", Severity: "warning", Path: path + ".labelPriority", Message: "A message lacks label priority.", Suggestion: "Set labelPriority or label_priority so overview mode shows only important labels.", AutoFixHint: map[string]any{"action": "add_label_priority", "message_id": stringField(message, "id")}})
		}
		kind := strings.ToLower(stringField(message, "kind"))
		if kind == "loop" || curve == "loop" {
			loopMessages++
		}
		if kind == "return" || curve == "return" {
			returnMessages++
		} else if from != "" && to != "" {
			calls[from+"->"+to]++
		}
	}
	for pair, count := range pairCounts {
		if count > intRule(rules, "warn_messages_same_pair_over", 4) {
			penalty += 4
			addWarning(warnings, Warning{Code: "too_many_messages_same_pair", Severity: "info", Message: "Many messages repeat the same participant pair.", Suggestion: "Aggregate repetitive calls with a loop fragment or hide low-value detail messages.", Details: []string{pair + "=" + intString(count)}, AutoFixHint: map[string]any{"action": "aggregate_repeated_pair", "pair": pair}})
		}
	}
	if loopMessages > 0 && len(fragments) == 0 {
		penalty += 6
		addWarning(warnings, Warning{Code: "loop_without_fragment", Severity: "warning", Message: "Loop-like messages are present without a loop fragment.", Suggestion: "Add a fragment with kind=loop, start_order, end_order, and condition to explain repetition.", AutoFixHint: map[string]any{"action": "add_loop_fragment"}})
	}
	for i, fragment := range fragments {
		if strings.TrimSpace(firstString(fragment, "condition", "label", "summary")) == "" {
			penalty += 3
			addWarning(warnings, Warning{Code: "fragment_without_condition", Severity: "warning", Path: "$.fragments[" + intString(i) + "]", Message: "A sequence fragment lacks a condition or explanation.", Suggestion: "Add condition/summary so loop, alt, opt, or parallel regions are readable.", AutoFixHint: map[string]any{"action": "add_fragment_condition", "fragment_id": stringField(fragment, "id")}})
		}
	}
	if returnMessages > 0 && len(calls) == 0 {
		penalty += 4
		addWarning(warnings, Warning{Code: "return_without_matching_call", Severity: "info", Message: "Return messages appear without clear preceding call messages.", Suggestion: "Pair return messages with call/sync messages or lower their visibility as detail.", AutoFixHint: map[string]any{"action": "pair_return_messages"}})
	}
	if len(importanceValues) <= 1 && len(messages) > 6 {
		penalty += 6
		addWarning(warnings, Warning{Code: "all_messages_same_importance", Severity: "warning", Message: "All sequence messages have the same importance level.", Suggestion: "Use higher importance for the critical path and lower importance for repetitive or return detail.", AutoFixHint: map[string]any{"action": "vary_message_importance"}})
	}
	if len(phaseValues) <= 1 && len(messages) > 6 {
		penalty += 6
		addWarning(warnings, Warning{Code: "all_messages_same_phase", Severity: "warning", Message: "All sequence messages use one phase, so the story has no color-coded stages.", Suggestion: "Split the sequence into receive, validate, setup, running, finalize, cleanup, notify, or error phases.", AutoFixHint: map[string]any{"action": "split_message_phases"}})
	}
	return penalty
}

func analyzeGraphQuality(data map[string]any, category string, design manifest.VisualDesign, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	nodes := objectArray(data, "nodes")
	edges := objectArray(data, "edges")
	groups := objectArray(data, "groups")
	grouped := 0
	nodeImportanceMissing := 0
	edgeImportanceMissing := 0
	edgeVisibilityMissing := 0
	edgeKinds := map[string]int{}
	degree := map[string]int{}
	genericLabels := []string{}
	for i, node := range nodes {
		id := stringField(node, "id")
		if groupKeyField(node) != "" {
			grouped++
		}
		if !hasImportance(node) {
			nodeImportanceMissing++
		}
		if isGenericEntityLabel(displayLabelField(node)) {
			genericLabels = appendCapped(genericLabels, "$.nodes["+intString(i)+"]", 8)
		}
		if id != "" {
			degree[id] += 0
		}
	}
	for _, edge := range edges {
		if kind := stringField(edge, "kind"); kind != "" {
			edgeKinds[kind]++
		}
		if !hasImportance(edge) {
			edgeImportanceMissing++
		}
		if normalizeVisibility(stringField(edge, "visibility")) == "" {
			edgeVisibilityMissing++
		}
		degree[stringField(edge, "from")]++
		degree[stringField(edge, "to")]++
	}
	if len(nodes) > 0 && (len(groups) == 0 || float64(grouped)/float64(len(nodes)) < 0.5) && len(nodes) > 10 {
		penalty += 10
		addWarning(warnings, Warning{Code: "ungrouped_nodes_high", Severity: "warning", Message: "Many graph nodes are not assigned to semantic groups.", Suggestion: "Add groups[] and set nodes[].group/group_id/parent_id so the first view can collapse subsystems.", AutoFixHint: map[string]any{"action": "add_groups"}})
	}
	if nodeImportanceMissing > len(nodes)/3 && len(nodes) > 6 {
		penalty += 6
		addWarning(warnings, Warning{Code: "node_importance_missing", Severity: "warning", Message: "Many graph nodes lack importance.", Suggestion: "Add nodes[].importance so hubs and critical entities are emphasized in overview.", AutoFixHint: map[string]any{"action": "add_node_importance"}})
	}
	if edgeImportanceMissing > len(edges)/3 && len(edges) > 4 {
		penalty += 6
		addWarning(warnings, Warning{Code: "edge_importance_missing", Severity: "warning", Message: "Many graph edges lack importance.", Suggestion: "Add edges[].importance so the renderer can hide low-value relationships first.", AutoFixHint: map[string]any{"action": "add_edge_importance"}})
	}
	if edgeVisibilityMissing > len(edges)/3 && len(edges) > 4 {
		penalty += 6
		addWarning(warnings, Warning{Code: "edge_visibility_missing", Severity: "warning", Message: "Many graph edges lack visibility.", Suggestion: "Set edges[].visibility to overview, detail, or hidden to avoid rendering every relationship at once.", AutoFixHint: map[string]any{"action": "add_edge_visibility"}})
	}
	if dominantRatio(edgeKinds, len(edges)) >= 0.65 && len(edges) > 8 {
		penalty += 8
		addWarning(warnings, Warning{Code: "dominant_edge_kind", Severity: "warning", Message: "Most graph relationships use one edge kind.", Suggestion: "Use typed relationship kinds such as calls, owns, reads, writes, emits, subscribes, validates, deploys, observes, or blocks.", Details: countNames(topCounts(edgeKinds, 3)), AutoFixHint: map[string]any{"action": "split_edge_kinds"}})
	}
	if densityLabel(len(nodes), len(edges)) == "high" {
		penalty += 10
		addWarning(warnings, Warning{Code: "high_edge_density", Severity: "error", Message: "The graph has high edge density.", Suggestion: "Aggregate repeated relationships, hide detail edges, and keep only critical overview edges visible.", AutoFixHint: map[string]any{"action": "reduce_edge_density"}})
	}
	orphan := 0
	for id, d := range degree {
		if strings.TrimSpace(id) != "" && d == 0 {
			orphan++
		}
	}
	if orphan >= 3 {
		penalty += 6
		addWarning(warnings, Warning{Code: "orphan_node_count_high", Severity: "warning", Message: "Several graph nodes have no relationships.", Suggestion: "Connect isolated nodes to meaningful hubs/groups or hide them from overview.", AutoFixHint: map[string]any{"action": "connect_or_hide_orphans"}})
	}
	if len(genericLabels) > 0 {
		penalty += 4
		addWarning(warnings, Warning{Code: "generic_node_labels", Severity: "warning", Message: "Some graph nodes use generic labels.", Suggestion: "Replace labels such as Core, Policy, Review, Risk, or Step with scenario-specific names.", Details: genericLabels, AutoFixHint: map[string]any{"action": "replace_generic_labels"}})
	}
	if strings.ToLower(category) == "flow" {
		penalty += analyzeFlowQuality(data, warnings, recommendations)
	}
	return penalty
}

func analyzeTimelineQuality(data map[string]any, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	events := objectArray(data, "events")
	laneCounts := map[string]int{}
	milestones := 0
	labels := map[string]int{}
	for i, event := range events {
		path := "$.events[" + intString(i) + "]"
		if firstString(event, "time", "start", "end") == "" && intStringFromAny(event["order"]) == "" {
			penalty += 4
			addWarning(warnings, Warning{Code: "event_time_or_order_missing", Severity: "warning", Path: path, Message: "A timeline event lacks time/order placement.", Suggestion: "Add time, start/end, or order so the event has deterministic temporal placement.", AutoFixHint: map[string]any{"action": "add_event_time_or_order"}})
		}
		lane := firstString(event, "lane", "group", "category", "source")
		if lane == "" {
			penalty += 2
			addWarning(warnings, Warning{Code: "event_lane_missing", Severity: "info", Path: path, Message: "A timeline event lacks a lane.", Suggestion: "Set lane to actor, system, category, source, or phase for scanning.", AutoFixHint: map[string]any{"action": "add_event_lane"}})
		} else {
			laneCounts[lane]++
		}
		if importanceValueGo(event, 0) >= 0.75 {
			milestones++
		}
		labels[displayLabelField(event)]++
	}
	for lane, count := range laneCounts {
		if count > 12 {
			penalty += 3
			addWarning(warnings, Warning{Code: "too_many_events_same_lane", Severity: "info", Message: "Many events share one timeline lane.", Suggestion: "Split dense lanes by actor/category or aggregate repeated low-value events.", Details: []string{lane + "=" + intString(count)}, AutoFixHint: map[string]any{"action": "split_dense_lane", "lane": lane}})
		}
	}
	if milestones == 0 && len(events) > 4 {
		penalty += 6
		addWarning(warnings, Warning{Code: "milestones_missing", Severity: "warning", Message: "Timeline has no high-importance milestones.", Suggestion: "Set importance >= 0.75 on turning points, root cause, resolution, or user-visible events.", AutoFixHint: map[string]any{"action": "mark_milestones"}})
	}
	if len(duplicateNames(labels, 6)) > 0 {
		penalty += 3
		addWarning(warnings, Warning{Code: "repeated_events_not_aggregated", Severity: "info", Message: "Timeline contains repeated event labels.", Suggestion: "Aggregate repetitive events with count/summary rather than rendering each log line.", Details: duplicateNames(labels, 6), AutoFixHint: map[string]any{"action": "aggregate_repeated_events"}})
	}
	return penalty
}

func analyzeMatrixQuality(data map[string]any, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	items := objectArray(data, "items")
	if _, ok := data["scale"]; !ok {
		if _, ok := data["thresholds"]; !ok {
			if _, ok := data["legend"]; !ok && len(items) > 6 {
				penalty += 6
				addWarning(warnings, Warning{Code: "matrix_scale_missing", Severity: "warning", Message: "Matrix input does not define a scale, thresholds, or legend.", Suggestion: "Add scale/thresholds/legend so colors and values have an interpretation.", AutoFixHint: map[string]any{"action": "add_matrix_scale"}})
			}
		}
	}
	visibleLabels := 0
	for i, item := range items {
		path := "$.items[" + intString(i) + "]"
		if stringField(item, "summary") == "" {
			penalty += 1
			addWarning(warnings, Warning{Code: "cell_summary_missing", Severity: "info", Path: path, Message: "A matrix item lacks summary.", Suggestion: "Add summary so hover/inspector explains score, allocation, risk, or capability meaning.", AutoFixHint: map[string]any{"action": "add_cell_summary"}})
		}
		if normalizeLabelPriority(item) != "hidden" && normalizeVisibility(stringField(item, "visibility")) != "hidden" {
			visibleLabels++
		}
	}
	if visibleLabels > 40 {
		penalty += 4
		addWarning(warnings, Warning{Code: "too_many_visible_cell_labels", Severity: "info", Message: "Too many matrix labels may be visible at once.", Suggestion: "Use labelPriority=hover/detail for low-value cells and annotate only important cells.", AutoFixHint: map[string]any{"action": "lower_cell_label_priority"}})
	}
	if len(items) > 12 && !hasAnyGrouping(items) {
		penalty += 4
		addWarning(warnings, Warning{Code: "row_or_column_group_missing", Severity: "info", Message: "Large matrix input lacks grouping hints.", Suggestion: "Add group, row_group, column_group, or kind to organize rows/columns.", AutoFixHint: map[string]any{"action": "add_matrix_grouping"}})
	}
	return penalty
}

func analyzeEvidenceQuality(data map[string]any, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	claims := objectArray(data, "claims")
	sources := objectArray(data, "sources")
	links := objectArray(data, "links")
	for i, claim := range claims {
		if stringField(claim, "summary") == "" && len(displayLabelField(claim)) > 44 {
			penalty += 2
			addWarning(warnings, Warning{Code: "claim_summary_missing", Severity: "info", Path: "$.claims[" + intString(i) + "]", Message: "A long claim lacks summary.", Suggestion: "Make claim text concise and put explanation in summary/details.", AutoFixHint: map[string]any{"action": "add_claim_summary"}})
		}
		if !hasImportance(claim) && !hasField(claim, "confidence") {
			penalty += 2
			addWarning(warnings, Warning{Code: "confidence_missing", Severity: "warning", Path: "$.claims[" + intString(i) + "]", Message: "A claim lacks confidence.", Suggestion: "Set confidence 0..1 to encode claim strength separately from importance.", AutoFixHint: map[string]any{"action": "add_claim_confidence"}})
		}
	}
	for i, source := range sources {
		if stringField(source, "summary") == "" {
			penalty += 1
			addWarning(warnings, Warning{Code: "source_summary_missing", Severity: "info", Path: "$.sources[" + intString(i) + "]", Message: "A source lacks summary.", Suggestion: "Add source summary so evidence cards explain why the source matters.", AutoFixHint: map[string]any{"action": "add_source_summary"}})
		}
	}
	relations := map[string]int{}
	for _, link := range links {
		relations[firstString(link, "relation", "kind")]++
	}
	if dominantRatio(relations, len(links)) >= 0.9 && len(links) > 3 {
		penalty += 4
		addWarning(warnings, Warning{Code: "evidence_relation_semantics_flat", Severity: "warning", Message: "Most evidence links use one relation type.", Suggestion: "Use supports, contradicts/refutes, qualifies, mentions, or depends_on where appropriate.", AutoFixHint: map[string]any{"action": "split_evidence_relations"}})
	}
	return penalty
}

func analyzeFlowQuality(data map[string]any, warnings *[]Warning, recommendations *Recommendations) int {
	penalty := 0
	nodes := objectArray(data, "nodes")
	edges := objectArray(data, "edges")
	if len(nodes) > 9 {
		penalty += 4
		addWarning(warnings, Warning{Code: "too_many_stages", Severity: "info", Message: "Flow diagram has many stages for an overview.", Suggestion: "Keep stable stages to roughly 5-9 and hide subprocess details behind groups or detail visibility.", AutoFixHint: map[string]any{"action": "aggregate_flow_stages"}})
	}
	mainPath := false
	for i, edge := range edges {
		if !hasImportance(edge) && !hasField(edge, "weight") && !hasField(edge, "amount") {
			penalty += 2
			addWarning(warnings, Warning{Code: "flow_weight_missing", Severity: "warning", Path: "$.edges[" + intString(i) + "]", Message: "A flow edge lacks weight/amount/importance.", Suggestion: "Add weight, amount, or importance so path thickness reflects volume or priority without inventing business metrics.", AutoFixHint: map[string]any{"action": "add_flow_weight"}})
		}
		if importanceValueGo(edge, 0) >= 0.75 || normalizeVisibility(stringField(edge, "visibility")) == "overview" {
			mainPath = true
		}
		status := strings.ToLower(firstString(edge, "status", "kind"))
		if (strings.Contains(status, "error") || strings.Contains(status, "fail") || strings.Contains(status, "drop")) && stringField(edge, "summary") == "" {
			penalty += 3
			addWarning(warnings, Warning{Code: "dropoff_or_error_path_unexplained", Severity: "warning", Path: "$.edges[" + intString(i) + "]", Message: "An error/dropoff path lacks summary.", Suggestion: "Add summary explaining why this path fails, drops off, or needs attention.", AutoFixHint: map[string]any{"action": "add_error_path_summary"}})
		}
	}
	if !mainPath && len(edges) > 3 {
		penalty += 4
		addWarning(warnings, Warning{Code: "main_path_missing", Severity: "warning", Message: "Flow input does not identify a main path.", Suggestion: "Set importance >= 0.75 or visibility=overview on the core happy path edges.", AutoFixHint: map[string]any{"action": "mark_main_path"}})
	}
	for i, node := range nodes {
		label := strings.ToLower(displayLabelField(node))
		if strings.Contains(label, "func") || strings.Contains(label, "method") {
			penalty += 2
			addWarning(warnings, Warning{Code: "stage_too_granular", Severity: "info", Path: "$.nodes[" + intString(i) + "]", Message: "A flow stage looks like a low-level function or method.", Suggestion: "Use stable process stages or system boundaries, not every call.", AutoFixHint: map[string]any{"action": "aggregate_stage"}})
		}
	}
	return penalty
}

func addWarning(warnings *[]Warning, warning Warning) {
	*warnings = append(*warnings, warning)
}

func normalizeWarnings(warnings []Warning) []Warning {
	for i := range warnings {
		if strings.TrimSpace(warnings[i].Severity) == "" {
			warnings[i].Severity = "warning"
		}
		if strings.TrimSpace(warnings[i].Suggestion) == "" {
			warnings[i].Suggestion = strings.TrimSpace(warnings[i].Hint)
		}
		if strings.TrimSpace(warnings[i].Hint) == "" {
			warnings[i].Hint = warnings[i].Suggestion
		}
		if warnings[i].AutoFixHint == nil && strings.TrimSpace(warnings[i].Code) != "" {
			warnings[i].AutoFixHint = map[string]any{"action": warnings[i].Code}
		}
	}
	return warnings
}

type qualityObject struct {
	Kind string
	Path string
	Obj  map[string]any
}

func qualityObjects(data map[string]any) []qualityObject {
	fields := []string{"groups", "zones", "nodes", "entities", "edges", "events", "claims", "sources", "links", "items", "participants", "messages", "phases", "activations", "fragments", "classes", "relationships", "states", "transitions", "actions", "flows", "components", "deployments"}
	var out []qualityObject
	for _, field := range fields {
		for i, obj := range objectArray(data, field) {
			out = append(out, qualityObject{Kind: strings.TrimSuffix(field, "s"), Path: "$." + field + "[" + intString(i) + "]", Obj: obj})
		}
	}
	return out
}

func normalizeLabelPriority(obj map[string]any) string {
	for _, name := range []string{"labelPriority", "label_priority"} {
		if value, ok := obj[name]; ok {
			switch v := value.(type) {
			case string:
				vv := strings.ToLower(strings.TrimSpace(v))
				switch vv {
				case "always", "important", "normal", "hover", "hidden":
					return vv
				}
			case float64:
				return labelPriorityFromNumber(v)
			case int:
				return labelPriorityFromNumber(float64(v))
			}
		}
	}
	return ""
}

func labelPriorityFromNumber(value float64) string {
	if value > 1 {
		value = value / 100
	}
	switch {
	case value >= 0.85:
		return "always"
	case value >= 0.65:
		return "important"
	case value >= 0.35:
		return "normal"
	case value > 0:
		return "hover"
	default:
		return "hidden"
	}
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

func normalizeCurve(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "arc", "high_arc", "return", "self", "loop", "straight":
		return value
	default:
		return ""
	}
}

func importanceValueGo(obj map[string]any, fallback float64) float64 {
	if value, ok := numericValue(obj["importance"]); ok {
		if value > 1 {
			return value / 100
		}
		return value
	}
	metrics, _ := obj["metrics"].(map[string]any)
	if metrics != nil {
		for _, name := range []string{"importance", "impact", "risk", "score", "weight"} {
			if value, ok := numericValue(metrics[name]); ok {
				if value > 1 {
					return value / 100
				}
				return value
			}
		}
	}
	return fallback
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func importanceBucket(obj map[string]any) string {
	v := importanceValueGo(obj, 0)
	switch {
	case v >= 0.75:
		return "high"
	case v >= 0.35:
		return "medium"
	default:
		return "low"
	}
}

func hasLane(obj map[string]any) bool {
	if _, ok := obj["lane_index"]; ok {
		return true
	}
	if _, ok := obj["laneIndex"]; ok {
		return true
	}
	presentation, _ := obj["presentation"].(map[string]any)
	if presentation == nil {
		return false
	}
	_, ok := presentation["laneIndex"]
	return ok
}

func hasDepth(obj map[string]any) bool {
	if _, ok := obj["depth"]; ok {
		return true
	}
	presentation, _ := obj["presentation"].(map[string]any)
	if presentation == nil {
		return false
	}
	_, ok := presentation["depth"]
	return ok
}

func nestedStringExists(obj map[string]any, parent, child string) bool {
	m, _ := obj[parent].(map[string]any)
	if m == nil {
		return false
	}
	return strings.TrimSpace(stringField(m, child)) != ""
}

func intStringFromAny(value any) string {
	switch v := value.(type) {
	case float64:
		if v == float64(int(v)) {
			return intString(int(v))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return intString(v)
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func intRule(rules authoring.QualityRules, name string, fallback int) int {
	if rules.TemplateSpecific == nil {
		return fallback
	}
	if value, ok := numericValue(rules.TemplateSpecific[name]); ok && value > 0 {
		return int(value)
	}
	return fallback
}

func hasAnyGrouping(items []map[string]any) bool {
	for _, item := range items {
		if firstString(item, "group", "row_group", "column_group", "kind") != "" {
			return true
		}
	}
	return false
}

func hasField(obj map[string]any, name string) bool {
	_, ok := obj[name]
	return ok
}

func isGenericKind(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "item" || value == "node" || value == "edge" || value == "thing" || value == "object" || value == "misc" || value == "other" || value == "related_to"
}

func isGenericEntityLabel(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, " .:-_#")
	return value == "core" || value == "policy" || value == "review" || value == "risk" || value == "step" || value == "stage" || value == "system" || value == "module"
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		if stdin == nil {
			stdin = bytes.NewReader(nil)
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read visual input from stdin: "+err.Error(), "Pipe valid JSON or Mermaid to visual preview --input -.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read visual input: "+err.Error(), "Pass a readable JSON or Mermaid file path to --input.", 400)
	}
	return b, nil
}

func labelField(obj map[string]any) string {
	for _, name := range []string{"label", "name", "title", "summary", "text", "id"} {
		if value := stringField(obj, name); value != "" {
			return value
		}
	}
	return ""
}

func displayLabelField(obj map[string]any) string {
	for _, name := range []string{"label", "name", "title", "summary", "text"} {
		if value := stringField(obj, name); value != "" {
			return value
		}
	}
	return ""
}

func groupKeyField(obj map[string]any) string {
	return firstString(obj, "parent_id", "group_id", "group", "module", "package")
}

func isGenericGroupLabel(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, " .:-_#")
	for value != "" && value[0] >= '0' && value[0] <= '9' {
		value = strings.TrimSpace(value[1:])
		value = strings.Trim(value, " .:-_#")
	}
	if value == "" {
		return false
	}
	generic := map[string]bool{
		"core":      true,
		"default":   true,
		"group":     true,
		"intake":    true,
		"misc":      true,
		"other":     true,
		"phase":     true,
		"policy":    true,
		"review":    true,
		"risk":      true,
		"stage":     true,
		"step":      true,
		"task":      true,
		"ungrouped": true,
	}
	return generic[value]
}

func hasImportance(obj map[string]any) bool {
	if _, ok := obj["importance"]; ok {
		return true
	}
	metrics, _ := obj["metrics"].(map[string]any)
	if metrics == nil {
		return false
	}
	for _, name := range []string{"importance", "impact", "risk", "score", "weight"} {
		if _, ok := metrics[name]; ok {
			return true
		}
	}
	return false
}

func appendCapped(items []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	if len(items) >= limit {
		return items
	}
	return append(items, value)
}

func duplicateNames(counts map[string]int, limit int) []string {
	var names []string
	for name, count := range counts {
		if count > 1 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > limit {
		return names[:limit]
	}
	return names
}

func topCounts(counts map[string]int, limit int) []Count {
	var items []Count
	for name, count := range counts {
		if strings.TrimSpace(name) != "" {
			items = append(items, Count{Name: name, Count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func countNames(items []Count) []string {
	var out []string
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

func dominantRatio(counts map[string]int, total int) float64 {
	if total <= 0 {
		return 0
	}
	max := 0
	for _, count := range counts {
		if count > max {
			max = count
		}
	}
	return float64(max) / float64(total)
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func duplicateFree(items []string, limit int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if len(out) >= limit {
			return out
		}
	}
	return out
}

func countSemanticItems(data map[string]any) int {
	total := 0
	for _, name := range []string{"groups", "zones", "nodes", "entities", "edges", "events", "claims", "sources", "links", "items", "participants", "messages", "phases", "activations", "fragments", "classes", "relationships", "states", "transitions", "actions", "flows", "components", "deployments"} {
		total += len(array(data, name))
	}
	return total
}

func topDegreeNodes(degree map[string]int, limit int) []string {
	counts := topCounts(degree, limit)
	var out []string
	for _, item := range counts {
		if item.Count > 0 {
			out = append(out, item.Name)
		}
	}
	return out
}

func array(data map[string]any, name string) []any {
	items, _ := data[name].([]any)
	return items
}

func objectArray(data map[string]any, name string) []map[string]any {
	raw := array(data, name)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func objectArrayFromValue(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func visualStringRefs(data map[string]any, name string, knownIDs map[string]bool) ([]string, []string) {
	raw, ok := data[name].([]any)
	if !ok {
		return nil, nil
	}
	refs := []string{}
	unknown := []string{}
	for _, item := range raw {
		value, ok := item.(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		refs = append(refs, value)
		if len(knownIDs) > 0 && !knownIDs[value] {
			unknown = appendCapped(unknown, value, 12)
		}
	}
	return refs, unknown
}

func stringField(obj map[string]any, name string) string {
	value, _ := obj[name].(string)
	return strings.TrimSpace(value)
}

func firstString(obj map[string]any, names ...string) string {
	for _, name := range names {
		if value := stringField(obj, name); value != "" {
			return value
		}
	}
	return ""
}

func collectVisualReferenceIDs(kind string, data map[string]any) map[string]bool {
	fieldsByKind := map[string][]string{
		"graph_v1":                    {"groups", "nodes", "edges"},
		"graph_events_v1":             {"groups", "nodes", "edges", "events"},
		"timeline_v1":                 {"events"},
		"evidence_v1":                 {"claims", "sources", "links"},
		"matrix_v1":                   {"items"},
		"isometric_architecture_v1":   {"zones", "entities", "links"},
		"uml_sequence_v1":             {"participants", "messages", "phases", "activations", "fragments"},
		"uml_class_v1":                {"classes", "relationships"},
		"uml_state_machine_v1":        {"states", "transitions"},
		"uml_activity_v1":             {"actions", "flows"},
		"uml_component_deployment_v1": {"components", "deployments", "links"},
	}
	ids := map[string]bool{}
	source := data
	for _, field := range fieldsByKind[strings.ToLower(kind)] {
		for _, item := range objectArray(source, field) {
			for _, id := range visualIDsForObject(item) {
				ids[id] = true
			}
		}
	}
	return ids
}

func visualIDsForObject(obj map[string]any) []string {
	if id := stringField(obj, "id"); id != "" {
		return []string{id}
	}
	from := firstString(obj, "from", "source", "source_id", "sourceId", "claim_id", "claimId")
	to := firstString(obj, "to", "target", "target_id", "targetId", "source_id", "sourceId")
	if from != "" && to != "" {
		return []string{from + "->" + to}
	}
	return nil
}

func densityLabel(nodes, edges int) string {
	if nodes == 0 || edges == 0 {
		return "low"
	}
	ratio := float64(edges) / float64(nodes)
	if ratio >= 3 || edges > 250 {
		return "high"
	}
	if ratio >= 1.5 || edges > 120 {
		return "medium"
	}
	return "low"
}

func pressureLabel(nodes, maxInitial int) string {
	if maxInitial <= 0 {
		maxInitial = 60
	}
	if nodes > maxInitial*2 {
		return "high"
	}
	if nodes > maxInitial {
		return "medium"
	}
	return "low"
}

func initialVisibleNodes(nodes, groups []map[string]any, design manifest.VisualDesign) int {
	if len(groups) > 0 && design.DefaultCollapseDepth > 0 {
		visible := len(groups)
		for _, node := range nodes {
			if stringField(node, "parent_id") == "" && stringField(node, "group_id") == "" && stringField(node, "group") == "" {
				visible++
			}
		}
		return minInt(visible, design.MaxInitialNodes)
	}
	return minInt(len(nodes), design.MaxInitialNodes)
}

func highFanoutNodes(degree map[string]int, threshold int) []string {
	var out []string
	for id, count := range degree {
		if count > threshold {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func lowValueEdgeKinds(kinds map[string]int) []string {
	preferred := []string{"method_call", "calls", "imports", "low_confidence_import", "mentions", "observes"}
	var out []string
	for _, kind := range preferred {
		if kinds[kind] > 0 {
			out = append(out, kind)
		}
	}
	if len(out) > 0 {
		return out
	}
	var pairs []string
	for kind := range kinds {
		pairs = append(pairs, kind)
	}
	sort.Strings(pairs)
	if len(pairs) > 3 {
		pairs = pairs[:3]
	}
	return pairs
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intString(value int) string {
	return strconv.Itoa(value)
}
