package preview

import (
	"bytes"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/visual/manifest"
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
	TemplateID      string                    `json:"template_id"`
	TemplateDir     string                    `json:"template_dir"`
	InputSummary    visualschema.InputSummary `json:"input_summary"`
	QualityScore    int                       `json:"quality_score"`
	Summary         Summary                   `json:"summary"`
	Warnings        []Warning                 `json:"warnings"`
	Recommendations Recommendations           `json:"recommendations"`
	VisualDesign    manifest.VisualDesign     `json:"visual_design"`
}

type Summary struct {
	Nodes                  int      `json:"nodes,omitempty"`
	Edges                  int      `json:"edges,omitempty"`
	Groups                 int      `json:"groups,omitempty"`
	VisibleNodes           int      `json:"visible_nodes,omitempty"`
	VisibleEdges           int      `json:"visible_edges,omitempty"`
	Events                 int      `json:"events,omitempty"`
	Participants           int      `json:"participants,omitempty"`
	Messages               int      `json:"messages,omitempty"`
	Phases                 int      `json:"phases,omitempty"`
	Activations            int      `json:"activations,omitempty"`
	Fragments              int      `json:"fragments,omitempty"`
	Classes                int      `json:"classes,omitempty"`
	Relationships          int      `json:"relationships,omitempty"`
	States                 int      `json:"states,omitempty"`
	Transitions            int      `json:"transitions,omitempty"`
	Actions                int      `json:"actions,omitempty"`
	Flows                  int      `json:"flows,omitempty"`
	Components             int      `json:"components,omitempty"`
	Deployments            int      `json:"deployments,omitempty"`
	Claims                 int      `json:"claims,omitempty"`
	Sources                int      `json:"sources,omitempty"`
	Links                  int      `json:"links,omitempty"`
	Items                  int      `json:"items,omitempty"`
	EdgeDensity            string   `json:"edge_density,omitempty"`
	LabelPressure          string   `json:"label_pressure,omitempty"`
	GroupCoverage          float64  `json:"group_coverage,omitempty"`
	RelationCoverage       float64  `json:"relation_coverage,omitempty"`
	EdgeKindCount          int      `json:"edge_kind_count,omitempty"`
	DominantEdgeKinds      []Count  `json:"dominant_edge_kinds,omitempty"`
	OrphanNodes            []string `json:"orphan_nodes,omitempty"`
	OrphanNodeCount        int      `json:"orphan_node_count,omitempty"`
	LargestGroupSize       int      `json:"largest_group_size,omitempty"`
	LargeGroups            []string `json:"large_groups,omitempty"`
	GenericGroups          []string `json:"generic_groups,omitempty"`
	MissingLabels          int      `json:"missing_labels,omitempty"`
	FallbackIDLabels       []string `json:"fallback_id_labels,omitempty"`
	LongLabels             []string `json:"long_labels,omitempty"`
	DuplicateLabels        []string `json:"duplicate_labels,omitempty"`
	EventsWithoutNodeID    int      `json:"events_without_node_id,omitempty"`
	EventsWithoutKnownNode int      `json:"events_without_known_node,omitempty"`
	EventNodeCoverage      float64  `json:"event_node_coverage,omitempty"`
	MessagesWithoutPhase   int      `json:"messages_without_phase,omitempty"`
	ParticipantFanout      []string `json:"participant_fanout,omitempty"`
	MissingImportance      int      `json:"missing_importance,omitempty"`
	MissingVisibility      int      `json:"missing_visibility,omitempty"`
	HighFanoutNodes        []string `json:"high_fanout_nodes,omitempty"`
	InitialView            string   `json:"initial_view,omitempty"`
	CollapseByDefault      bool     `json:"collapse_by_default,omitempty"`
}

type Warning struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity,omitempty"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint"`
	Details  []string `json:"details,omitempty"`
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
	parsed, err := visualschema.ValidateInput(tpl.InputSchemaKind, raw, tpl.Limits)
	if err != nil {
		return Result{}, err
	}
	quality, summary, warnings, recommendations := Analyze(tpl, parsed.Data)
	return Result{
		TemplateID:      tpl.ID,
		TemplateDir:     opts.TemplateDir,
		InputSummary:    parsed.Summary,
		QualityScore:    quality,
		Summary:         summary,
		Warnings:        warnings,
		Recommendations: recommendations,
		VisualDesign:    tpl.VisualDesign,
	}, nil
}

func Analyze(tpl manifest.TemplateManifest, data map[string]any) (int, Summary, []Warning, Recommendations) {
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
	if quality < 0 {
		quality = 0
	}
	return quality, summary, warnings, recommendations
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

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		if stdin == nil {
			stdin = bytes.NewReader(nil)
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, metadata.NewError("input_read_failed", "failed to read input JSON from stdin: "+err.Error(), "Pipe valid JSON to visual preview --input -.", 400)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, metadata.NewError("input_read_failed", "failed to read input JSON: "+err.Error(), "Pass a readable JSON file path to --input.", 400)
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
