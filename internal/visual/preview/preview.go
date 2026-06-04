package preview

import (
	"bytes"
	"io"
	"os"
	"sort"
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
	Nodes             int      `json:"nodes,omitempty"`
	Edges             int      `json:"edges,omitempty"`
	Groups            int      `json:"groups,omitempty"`
	VisibleNodes      int      `json:"visible_nodes,omitempty"`
	VisibleEdges      int      `json:"visible_edges,omitempty"`
	Events            int      `json:"events,omitempty"`
	Claims            int      `json:"claims,omitempty"`
	Sources           int      `json:"sources,omitempty"`
	Links             int      `json:"links,omitempty"`
	Items             int      `json:"items,omitempty"`
	EdgeDensity       string   `json:"edge_density,omitempty"`
	LabelPressure     string   `json:"label_pressure,omitempty"`
	GroupCoverage     float64  `json:"group_coverage,omitempty"`
	HighFanoutNodes   []string `json:"high_fanout_nodes,omitempty"`
	InitialView       string   `json:"initial_view,omitempty"`
	CollapseByDefault bool     `json:"collapse_by_default,omitempty"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type Recommendations struct {
	InitialView       string   `json:"initial_view"`
	MaxInitialNodes   int      `json:"max_initial_nodes"`
	MaxInitialEdges   int      `json:"max_initial_edges"`
	GroupBy           []string `json:"group_by,omitempty"`
	CollapseByDefault bool     `json:"collapse_by_default"`
	HideEdgeTypes     []string `json:"hide_edge_types,omitempty"`
	AgentGuidance     []string `json:"agent_guidance,omitempty"`
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
	}
	if quality < 0 {
		quality = 0
	}
	return quality, summary, warnings, recommendations
}

func analyzeGraph(data map[string]any, design manifest.VisualDesign, summary *Summary, warnings *[]Warning, recommendations *Recommendations) int {
	nodes := objectArray(data, "nodes")
	edges := objectArray(data, "edges")
	groups := objectArray(data, "groups")
	summary.Nodes = len(nodes)
	summary.Edges = len(edges)
	summary.Groups = len(groups)
	summary.EdgeDensity = densityLabel(len(nodes), len(edges))
	summary.LabelPressure = pressureLabel(len(nodes), design.MaxInitialNodes)
	grouped := 0
	degree := map[string]int{}
	edgeKinds := map[string]int{}
	for _, node := range nodes {
		if firstString(node, "parent_id", "group_id", "group", "module", "package") != "" {
			grouped++
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
	}
	if len(nodes) > 0 {
		summary.GroupCoverage = round2(float64(grouped) / float64(len(nodes)))
	}
	highFanout := highFanoutNodes(degree, 20)
	summary.HighFanoutNodes = highFanout
	summary.VisibleNodes = initialVisibleNodes(nodes, groups, design)
	summary.VisibleEdges = minInt(len(edges), design.MaxInitialEdges)
	quality := 100
	if len(nodes) > design.MaxInitialNodes && len(groups) == 0 {
		quality -= 28
		*warnings = append(*warnings, Warning{Code: "missing_groups", Message: "Large graph input has no groups.", Hint: "Group nodes by module/package/component and render groups collapsed initially."})
	}
	if len(nodes) > design.MaxInitialNodes {
		quality -= 16
		*warnings = append(*warnings, Warning{Code: "visible_nodes_high", Message: "The graph has more nodes than the recommended initial view.", Hint: "Set visible=false for low-importance detail nodes or collapse them under groups."})
	}
	if len(edges) > design.MaxInitialEdges || summary.EdgeDensity == "high" {
		quality -= 22
		*warnings = append(*warnings, Warning{Code: "graph_density_high", Message: "Edge density is high for an initial overview.", Hint: "Hide low-importance or detail-only edge types and keep only summary relationships visible first."})
		recommendations.HideEdgeTypes = lowValueEdgeKinds(edgeKinds)
	}
	if summary.GroupCoverage < 0.5 && len(nodes) > design.MaxInitialNodes {
		quality -= 14
		*warnings = append(*warnings, Warning{Code: "group_coverage_low", Message: "Most graph nodes are not assigned to a group.", Hint: "Set parent_id, group_id, group, module, or package so the renderer can collapse related nodes."})
	}
	if len(highFanout) > 0 {
		quality -= 10
		*warnings = append(*warnings, Warning{Code: "high_fanout_nodes", Message: "Some nodes have very high fan-out.", Hint: "Represent high fan-out nodes as hubs or groups and hide detail edges until focus mode."})
	}
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
