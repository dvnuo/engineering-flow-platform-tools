package mermaid

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"engineering-flow-platform-tools/internal/visual/metadata"
	"gopkg.in/yaml.v3"
)

type Node struct {
	ID    string
	Label string
	Kind  string
	Group string
}

type Edge struct {
	ID        string
	From      string
	To        string
	FromPort  string
	ToPort    string
	Label     string
	Kind      string
	Directed  bool
	Dashed    bool
	Thick     bool
	Role      string
	PathGroup string
}

type Group struct {
	ID    string
	Label string
	Kind  string
}

type Message struct {
	ID     string
	Order  int
	From   string
	To     string
	Label  string
	Kind   string
	Dashed bool
}

type Diagram struct {
	Kind        string
	Direction   string
	Title       string
	Frontmatter map[string]any
	EFP         map[string]any
	Nodes       map[string]*Node
	Groups      map[string]*Group
	Edges       []Edge
	Messages    []Message
	Lines       []string
}

func IsMermaid(raw []byte) bool {
	_, ok := parse(raw)
	return ok
}

func InferTemplateID(raw []byte) (string, bool) {
	d, ok := parse(raw)
	if !ok {
		return "", false
	}
	if template := stringFromMap(d.EFP, "template"); template != "" {
		return template, true
	}
	if template := stringFromMap(d.EFP, "template_id"); template != "" {
		return template, true
	}
	switch d.Kind {
	case "architecture-beta", "architecture":
		return "mermaid.architecture", true
	case "c4context":
		return "mermaid.c4", true
	case "sequencediagram", "sequencediagram-v2", "zenuml":
		if d.Kind == "zenuml" {
			return "mermaid.zenuml", true
		}
		return "mermaid.sequence", true
	case "classdiagram", "classdiagram-v2":
		return "mermaid.class", true
	case "erdiagram":
		return "mermaid.er", true
	case "statediagram", "statediagram-v2", "statediagram-v2-beta":
		return "mermaid.state", true
	case "timeline":
		return "mermaid.timeline", true
	case "gantt":
		return "mermaid.gantt", true
	case "journey":
		return "mermaid.journey", true
	case "gitgraph":
		return "mermaid.gitgraph", true
	case "mindmap":
		return "mermaid.mindmap", true
	case "treeview":
		return "mermaid.treeview", true
	case "sankey", "sankey-beta":
		return "mermaid.sankey", true
	case "xychart", "xychart-beta":
		return "mermaid.xy", true
	case "block", "block-beta":
		return "mermaid.block", true
	case "packet", "packet-beta":
		return "mermaid.packet", true
	case "pie":
		return "mermaid.pie", true
	case "quadrantchart":
		return "mermaid.quadrant", true
	case "kanban":
		return "mermaid.kanban", true
	case "radar", "radar-beta":
		return "mermaid.radar", true
	case "treemap", "treemap-beta":
		return "mermaid.treemap", true
	case "requirementdiagram":
		return "mermaid.requirement", true
	case "eventmodeling":
		return "mermaid.event_modeling", true
	case "venn":
		return "mermaid.venn", true
	case "ishikawa":
		return "mermaid.ishikawa", true
	case "wardley", "wardley-beta":
		return "mermaid.wardley", true
	case "flowchart", "graph":
		return "mermaid.flowchart", true
	default:
		return "mermaid.flowchart", true
	}
}

func CompileIfNeeded(kind string, raw []byte) ([]byte, error) {
	d, ok := parse(raw)
	if !ok {
		return raw, nil
	}
	data := compile(kind, d)
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, metadata.NewError("mermaid_compile_failed", "failed to compile Mermaid input: "+err.Error(), "Check the Mermaid source and EFP frontmatter.", 400)
	}
	return out, nil
}

func parse(raw []byte) (Diagram, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return Diagram{}, false
	}
	text = stripCodeFence(text)
	frontmatter, body := splitFrontmatter(text)
	bodyTitle := titleFromLines(rawLines(body))
	lines := cleanedLines(body)
	if len(lines) == 0 {
		return Diagram{}, false
	}
	first := strings.Fields(lines[0])
	if len(first) == 0 {
		return Diagram{}, false
	}
	kind := normalizeKind(first[0])
	if !knownMermaidKind(kind) {
		return Diagram{}, false
	}
	d := Diagram{
		Kind:        kind,
		Frontmatter: frontmatter,
		EFP:         objectFromMap(frontmatter, "efp"),
		Nodes:       map[string]*Node{},
		Groups:      map[string]*Group{},
	}
	d.Title = firstNonEmpty(stringFromMap(frontmatter, "title"), stringFromMap(d.EFP, "title"), bodyTitle)
	if len(first) > 1 {
		d.Direction = strings.ToUpper(first[1])
	}
	bodyLines := lines[1:]
	switch kind {
	case "architecture-beta", "architecture":
		parseArchitecture(&d, bodyLines)
	case "sequencediagram", "sequencediagram-v2", "zenuml":
		parseSequence(&d, bodyLines)
	case "classdiagram", "classdiagram-v2":
		parseClass(&d, bodyLines)
	case "statediagram", "statediagram-v2":
		parseState(&d, bodyLines)
	default:
		parseFlowLike(&d, bodyLines)
	}
	if len(d.Nodes) == 0 && len(d.Messages) == 0 {
		for i, line := range bodyLines {
			label := compact(cleanText(line), 34)
			if label == "" {
				continue
			}
			id := fmt.Sprintf("item_%02d", i+1)
			d.Nodes[id] = &Node{ID: id, Label: label, Kind: "item"}
		}
	}
	return d, true
}

func compile(kind string, d Diagram) map[string]any {
	if d.Title == "" {
		d.Title = "Mermaid Visual"
	}
	switch kind {
	case "isometric_architecture_v1":
		return mergeEFP(compileIsometric(d), d.EFP)
	case "graph_v1":
		return mergeEFP(compileGraph(d, false), d.EFP)
	case "graph_events_v1":
		return mergeEFP(compileGraph(d, true), d.EFP)
	case "uml_sequence_v1":
		return mergeEFP(compileSequence(d), d.EFP)
	case "uml_class_v1":
		return mergeEFP(compileClass(d), d.EFP)
	case "uml_state_machine_v1":
		return mergeEFP(compileState(d), d.EFP)
	case "uml_activity_v1":
		return mergeEFP(compileActivity(d), d.EFP)
	case "uml_component_deployment_v1":
		return mergeEFP(compileComponent(d), d.EFP)
	case "timeline_v1":
		return mergeEFP(compileTimeline(d), d.EFP)
	case "evidence_v1":
		return mergeEFP(compileEvidence(d), d.EFP)
	case "matrix_v1":
		return mergeEFP(compileMatrix(d), d.EFP)
	default:
		return mergeEFP(compileGraph(d, false), d.EFP)
	}
}

func compileIsometric(d Diagram) map[string]any {
	graph := BuildArchitectureSemanticGraph(d)
	layout := ArchitectureLayoutEngine(graph)
	routing := ArchitectureRoutingEngine(graph, layout)
	focusIDs := []any{}
	for i, node := range graph.Nodes {
		if i >= 3 {
			break
		}
		focusIDs = append(focusIDs, node.ID)
	}
	annotations := []any{}
	if target := firstStringFromAny(focusIDs); target != "" {
		annotations = append(annotations, map[string]any{"id": "primary-path", "target_id": target, "label": "Mermaid path", "summary": "Primary objects inferred from the Mermaid diagram.", "priority": 0.6})
	}
	return map[string]any{
		"schema":   "efp.visual.input.isometric_architecture.v1",
		"title":    d.Title,
		"subtitle": "Compiled from Mermaid " + d.Kind,
		"goal":     "Render the Mermaid architecture as an isometric scene.",
		"theme":    "architecture_light",
		"canvas": map[string]any{
			"grid":    map[string]any{"enabled": true, "step": 1, "subdivisions": 4},
			"padding": 2,
		},
		"camera": map[string]any{"mode": "orthographic_isometric", "zoom": 0.92, "theta": 0.78, "phi": 1.02, "radius": 11},
		"zones":  layout.ToVisualZones(), "entities": layout.ToVisualEntities(), "links": routing.ToVisualLinks(),
		"view": map[string]any{"colorBy": "kind", "mode": "overview"},
		"visual": map[string]any{
			"goal":              "Explain the Mermaid architecture structure and the first visible request path.",
			"initial_focus_ids": focusIDs,
			"annotations":       annotations,
			"narrative_steps": []any{
				map[string]any{"id": "overview", "title": "Architecture overview", "summary": "Start from the primary Mermaid path, then inspect supporting services and data stores.", "focus_ids": focusIDs},
			},
		},
		"renderHints": map[string]any{
			"badgeMode":            "icon_and_model",
			"badgeSize":            "medium",
			"badgePlacement":       "front",
			"labelIcon":            true,
			"layoutScale":          1.55,
			"preferExplicitRoutes": true,
			"relationLayer":        "world_ground",
			"relationRenderMode":   "ground_decal",
			"linkLabelMode":        "html_billboard",
			"showLegend":           true,
		},
		"metadata": mergeMetadata(sourceMetadata(d), map[string]any{
			"architecture_pipeline": map[string]any{
				"semantic_graph":         "BuildArchitectureSemanticGraph",
				"layout_engine":          "ArchitectureMapLayoutEngine",
				"routing_engine":         "ArchitectureRoutingEngine v2 / BusLaneRouter",
				"route_validator":        "ValidateArchitectureRoutes",
				"port_hint_violations":   routing.Metrics.PortHintViolations,
				"direction_violations":   routing.Metrics.DirectionViolations,
				"entity_intersections":   routing.Metrics.EntityIntersections,
				"route_crossing_count":   routing.Metrics.CrossingCount,
				"parallel_overlap_count": routing.Metrics.ParallelOverlapCount,
				"route_bus_lane_count":   routing.Metrics.BusLaneCount,
				"route_bundle_count":     routing.Metrics.BundleCount,
				"path_group_overlap":     routing.Metrics.PathGroupOverlap,
				"primary_route_count":    routing.Metrics.PrimaryRouteCount,
				"secondary_route_count":  routing.Metrics.SecondaryRouteCount,
				"auxiliary_route_count":  routing.Metrics.AuxiliaryRouteCount,
			},
		}),
	}
}

func nodeMapByID(nodes []Node) map[string]Node {
	out := map[string]Node{}
	for _, node := range nodes {
		out[node.ID] = node
	}
	return out
}

func architectureRanks(nodes []Node, edges []Edge) map[string]int {
	ranks := map[string]int{}
	for _, node := range nodes {
		ranks[node.ID] = 0
	}
	for pass := 0; pass < len(nodes)+len(edges)+2; pass++ {
		changed := false
		for _, edge := range edges {
			if _, ok := ranks[edge.From]; !ok {
				continue
			}
			if _, ok := ranks[edge.To]; !ok {
				continue
			}
			forward := edgeWantsForwardRank(edge)
			if forward {
				next := ranks[edge.From] + 1
				if ranks[edge.To] < next {
					ranks[edge.To] = next
					changed = true
				}
				continue
			}
			next := ranks[edge.To] + 1
			if ranks[edge.From] < next {
				ranks[edge.From] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	minRank := 0
	for _, rank := range ranks {
		if rank < minRank {
			minRank = rank
		}
	}
	if minRank != 0 {
		for id, rank := range ranks {
			ranks[id] = rank - minRank
		}
	}
	return ranks
}

func edgeWantsForwardRank(edge Edge) bool {
	from := strings.ToUpper(strings.TrimSpace(edge.FromPort))
	to := strings.ToUpper(strings.TrimSpace(edge.ToPort))
	if from == "L" && to == "R" {
		return false
	}
	return true
}

func architectureSortedGroups(d Diagram, nodes []Node, ranks map[string]int) []Group {
	groups := sortedGroups(d)
	if len(groups) == 0 {
		return groups
	}
	groupRank := map[string]int{}
	groupCount := map[string]int{}
	for _, group := range groups {
		groupRank[group.ID] = 9999
	}
	for _, node := range nodes {
		if node.Group == "" {
			continue
		}
		rank := ranks[node.ID]
		if rank < groupRank[node.Group] {
			groupRank[node.Group] = rank
		}
		groupCount[node.Group]++
	}
	for _, group := range groups {
		if groupCount[group.ID] == 0 {
			groupRank[group.ID] = 9999
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groupRank[groups[i].ID] != groupRank[groups[j].ID] {
			return groupRank[groups[i].ID] < groupRank[groups[j].ID]
		}
		return groups[i].ID < groups[j].ID
	})
	return groups
}

func architectureNodesByGroup(nodes []Node, groups []Group, ranks map[string]int) map[string][]Node {
	out := map[string][]Node{}
	defaultGroup := "main-zone"
	if len(groups) > 0 {
		defaultGroup = groups[0].ID
	}
	groupExists := map[string]bool{}
	for _, group := range groups {
		groupExists[group.ID] = true
	}
	for _, node := range nodes {
		groupID := node.Group
		if groupID == "" || !groupExists[groupID] {
			groupID = defaultGroup
		}
		node.Group = groupID
		out[groupID] = append(out[groupID], node)
	}
	for groupID := range out {
		sort.SliceStable(out[groupID], func(i, j int) bool {
			left := out[groupID][i]
			right := out[groupID][j]
			if architectureNaturalServiceOrder(left.ID, right.ID) != 0 {
				return architectureNaturalServiceOrder(left.ID, right.ID) < 0
			}
			if ranks[left.ID] != ranks[right.ID] {
				return ranks[left.ID] < ranks[right.ID]
			}
			return left.ID < right.ID
		})
	}
	return out
}

func architectureNaturalServiceOrder(leftID, rightID string) int {
	left := strings.ToLower(strings.TrimSpace(leftID))
	right := strings.ToLower(strings.TrimSpace(rightID))
	if !strings.HasPrefix(left, "service") || !strings.HasPrefix(right, "service") {
		return 0
	}
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func inferArchitectureRole(edge Edge, nodes map[string]Node) string {
	if edge.Role != "" {
		return edge.Role
	}
	if edge.Dashed {
		return "auxiliary"
	}
	group := inferArchitecturePathGroup(edge, nodes)
	from := inferEntityKind(nodes[edge.From])
	to := inferEntityKind(nodes[edge.To])
	if group == "health" || group == "observability" {
		return "auxiliary"
	}
	if group == "data" || group == "cache" || group == "storage" {
		return "secondary"
	}
	if group == "gateway" || ((from == "client" || from == "browser" || from == "mobile" || from == "cdn") && (to == "api_gateway" || to == "nginx")) || to == "api_gateway" {
		return "primary"
	}
	if group == "service" && (from == "api_gateway" || strings.Contains(strings.ToLower(edge.Label), "service")) {
		return "primary"
	}
	return "secondary"
}

func inferArchitecturePathGroup(edge Edge, nodes map[string]Node) string {
	if edge.PathGroup != "" {
		return edge.PathGroup
	}
	if inferred := pathGroupFromLabel(edge.Label); inferred != "" {
		if inferred == "data" {
			return "data"
		}
		return inferred
	}
	label := strings.ToLower(edge.Label)
	from := inferEntityKind(nodes[edge.From])
	to := inferEntityKind(nodes[edge.To])
	switch {
	case strings.Contains(label, "service"):
		return "service"
	case strings.Contains(label, "register") || strings.Contains(label, "nacos") || strings.Contains(label, "discovery"):
		return "registry"
	case strings.Contains(label, "cache") || strings.Contains(label, "redis"):
		return "cache"
	case strings.Contains(label, "storage") || strings.Contains(label, "oss") || strings.Contains(label, "file"):
		return "storage"
	case strings.Contains(label, "health") || strings.Contains(label, "admin"):
		return "health"
	case strings.Contains(label, "log") || strings.Contains(label, "metric") || strings.Contains(label, "dashboard") || strings.Contains(label, "observ"):
		return "observability"
	case to == "database":
		return "data"
	case to == "redis":
		return "cache"
	case to == "storage":
		return "storage"
	case to == "registry":
		return "registry"
	case from == "client" || from == "browser" || from == "mobile" || from == "cdn" || to == "api_gateway" || to == "nginx":
		return "gateway"
	default:
		return "main"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func compileGraph(d Diagram, withEvents bool) map[string]any {
	nodes := []any{}
	for _, node := range sortedNodes(d) {
		nodes = append(nodes, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": nonEmpty(node.Kind, "node"), "group": node.Group})
	}
	edges := []any{}
	for i, edge := range d.Edges {
		edges = append(edges, map[string]any{"from": edge.From, "to": edge.To, "label": compact(nonEmpty(edge.Label, edge.Kind, "link"), 32), "kind": nonEmpty(edge.Kind, "depends_on"), "directed": edge.Directed, "weight": i + 1})
	}
	schema := "efp.visual.input.graph.v1"
	if withEvents {
		schema = "efp.visual.input.graph_events.v1"
	}
	out := map[string]any{"schema": schema, "title": d.Title, "nodes": nodes, "edges": edges, "metadata": sourceMetadata(d)}
	if withEvents {
		events := []any{}
		for _, msg := range messagesOrEdges(d) {
			events = append(events, map[string]any{"id": msg.ID, "time": fmt.Sprintf("2026-06-03T12:%02d:00Z", msg.Order%60), "kind": "message", "node_id": msg.To, "status": "success", "summary": msg.Label})
		}
		out["events"] = events
	}
	return out
}

func compileSequence(d Diagram) map[string]any {
	participants := []any{}
	for _, node := range sortedNodes(d) {
		participants = append(participants, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": inferEntityKind(node)})
	}
	messages := []any{}
	for i, msg := range messagesOrEdges(d) {
		messages = append(messages, map[string]any{"id": msg.ID, "order": i + 1, "from": msg.From, "to": msg.To, "label": compact(msg.Label, 42), "kind": msg.Kind, "directed": true})
	}
	return map[string]any{"schema": "efp.visual.input.uml.sequence.v1", "title": d.Title, "participants": participants, "messages": messages, "metadata": sourceMetadata(d)}
}

func compileClass(d Diagram) map[string]any {
	classes := []any{}
	for _, node := range sortedNodes(d) {
		classes = append(classes, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": "class", "methods": []any{}, "attributes": []any{}})
	}
	return map[string]any{"schema": "efp.visual.input.uml.class.v1", "title": d.Title, "classes": classes, "relationships": edgeList(d), "metadata": sourceMetadata(d)}
}

func compileState(d Diagram) map[string]any {
	states := []any{}
	for _, node := range sortedNodes(d) {
		states = append(states, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": "state"})
	}
	return map[string]any{"schema": "efp.visual.input.uml.state_machine.v1", "title": d.Title, "states": states, "transitions": edgeList(d), "metadata": sourceMetadata(d)}
}

func compileActivity(d Diagram) map[string]any {
	actions := []any{}
	for _, node := range sortedNodes(d) {
		actions = append(actions, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": "action"})
	}
	return map[string]any{"schema": "efp.visual.input.uml.activity.v1", "title": d.Title, "actions": actions, "flows": edgeList(d), "metadata": sourceMetadata(d)}
}

func compileComponent(d Diagram) map[string]any {
	components := []any{}
	for _, node := range sortedNodes(d) {
		components = append(components, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "kind": inferEntityKind(node), "deployment_id": node.Group})
	}
	deployments := []any{}
	for _, group := range sortedGroups(d) {
		deployments = append(deployments, map[string]any{"id": group.ID, "label": nonEmpty(group.Label, group.ID), "kind": "node"})
	}
	return map[string]any{"schema": "efp.visual.input.uml.component_deployment.v1", "title": d.Title, "components": components, "deployments": deployments, "links": edgeList(d), "metadata": sourceMetadata(d)}
}

func compileTimeline(d Diagram) map[string]any {
	events := []any{}
	items := messagesOrEdges(d)
	if len(items) == 0 {
		for i, line := range d.Lines {
			items = append(items, Message{ID: fmt.Sprintf("event_%02d", i+1), Order: i + 1, Label: cleanText(line), Kind: "event"})
		}
	}
	for i, item := range items {
		events = append(events, map[string]any{"id": item.ID, "time": fmt.Sprintf("2026-06-03T12:%02d:00Z", i%60), "kind": nonEmpty(item.Kind, "event"), "label": compact(item.Label, 36), "status": "success", "summary": item.Label})
	}
	return map[string]any{"schema": "efp.visual.input.timeline.v1", "title": d.Title, "events": events, "metadata": sourceMetadata(d)}
}

func compileEvidence(d Diagram) map[string]any {
	nodes := sortedNodes(d)
	if len(nodes) == 0 {
		nodes = []Node{{ID: "claim_1", Label: d.Title, Kind: "claim"}}
	}
	claims, sources, links := []any{}, []any{}, []any{}
	for i, node := range nodes {
		claims = append(claims, map[string]any{"id": "claim_" + node.ID, "text": nonEmpty(node.Label, node.ID), "confidence": 0.72, "status": "supported"})
		sources = append(sources, map[string]any{"id": "source_" + node.ID, "title": nonEmpty(node.Label, node.ID), "kind": nonEmpty(node.Kind, "source"), "reliability": 0.7})
		links = append(links, map[string]any{"claim_id": "claim_" + node.ID, "source_id": "source_" + node.ID, "relation": "supports", "weight": i + 1})
	}
	return map[string]any{"schema": "efp.visual.input.evidence.v1", "title": d.Title, "claims": claims, "sources": sources, "links": links, "metadata": sourceMetadata(d)}
}

func compileMatrix(d Diagram) map[string]any {
	nodes := sortedNodes(d)
	items := []any{}
	for i, node := range nodes {
		x := float64((i%4)+1) / 5
		y := float64((i/4)+1) / float64(max(2, (len(nodes)+3)/4+1))
		items = append(items, map[string]any{"id": node.ID, "label": nonEmpty(node.Label, node.ID), "x": x, "y": y, "kind": nonEmpty(node.Kind, "item"), "status": "success"})
	}
	return map[string]any{"schema": "efp.visual.input.matrix.v1", "title": d.Title, "items": items, "metadata": sourceMetadata(d)}
}

func parseArchitecture(d *Diagram, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := regexp.MustCompile(`^group\s+([A-Za-z0-9_.:-]+)(?:\(([^)]*)\))?\s*\[([^\]]+)\]`).FindStringSubmatch(line); len(m) > 0 {
			d.Groups[m[1]] = &Group{ID: m[1], Label: cleanText(m[3]), Kind: cleanText(m[2])}
			continue
		}
		if m := regexp.MustCompile(`^service\s+([A-Za-z0-9_.:-]+)(?:\(([^)]*)\))?\s*\[([^\]]+)\](?:\s+in\s+([A-Za-z0-9_.:-]+))?`).FindStringSubmatch(line); len(m) > 0 {
			d.Nodes[m[1]] = &Node{ID: m[1], Label: cleanText(m[3]), Kind: cleanText(m[2]), Group: m[4]}
			continue
		}
		if strings.HasPrefix(line, "junction ") {
			id := strings.TrimSpace(strings.TrimPrefix(line, "junction "))
			d.Nodes[id] = &Node{ID: id, Label: id, Kind: "junction"}
			continue
		}
		if edge, ok := parseArchitectureEdge(line, len(d.Edges)+1); ok {
			d.Edges = append(d.Edges, edge)
		}
	}
}

func parseFlowLike(d *Diagram, lines []string) {
	currentGroup := ""
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "subgraph ") {
			id, label := parseSubgraph(line)
			currentGroup = id
			d.Groups[id] = &Group{ID: id, Label: label, Kind: "group"}
			continue
		}
		if lower == "end" {
			currentGroup = ""
			continue
		}
		if edge, ok := parseFlowEdge(line, len(d.Edges)+1); ok {
			addNode(d, edge.From, edge.From, "", currentGroup)
			addNode(d, edge.To, edge.To, "", currentGroup)
			d.Edges = append(d.Edges, edge)
			parseFlowEndpointInto(d, leftEndpoint(line), currentGroup)
			parseFlowEndpointInto(d, rightEndpoint(line), currentGroup)
			continue
		}
		if strings.HasPrefix(lower, "classdef ") || strings.HasPrefix(lower, "class ") || strings.HasPrefix(lower, "style ") || strings.HasPrefix(lower, "click ") {
			continue
		}
		id, label, kind := parseNodeRef(line)
		if id != "" {
			addNode(d, id, nonEmpty(label, id), kind, currentGroup)
		}
	}
}

func parseSequence(d *Diagram, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := regexp.MustCompile(`^(participant|actor)\s+([A-Za-z0-9_.:-]+)(?:\s+as\s+(.+))?$`).FindStringSubmatch(line); len(m) > 0 {
			kind := "participant"
			if m[1] == "actor" {
				kind = "actor"
			}
			d.Nodes[m[2]] = &Node{ID: m[2], Label: cleanText(nonEmpty(m[3], m[2])), Kind: kind}
			continue
		}
		if msg, ok := parseSequenceMessage(line, len(d.Messages)+1); ok {
			addNode(d, msg.From, msg.From, "participant", "")
			addNode(d, msg.To, msg.To, "participant", "")
			d.Messages = append(d.Messages, msg)
		}
	}
}

func parseClass(d *Diagram, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if edge, ok := parseClassEdge(line, len(d.Edges)+1); ok {
			addNode(d, edge.From, edge.From, "class", "")
			addNode(d, edge.To, edge.To, "class", "")
			d.Edges = append(d.Edges, edge)
			continue
		}
		if strings.HasPrefix(line, "class ") {
			id := strings.Fields(strings.TrimPrefix(line, "class "))[0]
			addNode(d, id, id, "class", "")
		}
	}
}

func parseState(d *Diagram, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if edge, ok := parseFlowEdge(line, len(d.Edges)+1); ok {
			addNode(d, edge.From, edge.From, "state", "")
			addNode(d, edge.To, edge.To, "state", "")
			d.Edges = append(d.Edges, edge)
			continue
		}
		id, label, _ := parseNodeRef(line)
		if id != "" {
			addNode(d, id, nonEmpty(label, id), "state", "")
		}
	}
}

func parseArchitectureEdge(line string, index int) (Edge, bool) {
	if !strings.ContainsAny(line, "-<") {
		return Edge{}, false
	}
	label := barLabel(line)
	cleaned := stripBarLabels(line)
	parts := regexp.MustCompile(`\s+(?:-->|<--|<-->|--|->|<-)\s+`).Split(cleaned, 2)
	if len(parts) != 2 {
		if parts = strings.Split(cleaned, "--"); len(parts) < 2 {
			return Edge{}, false
		}
	}
	from, fromPort := parseArchEndpoint(parts[0])
	to, toPort := parseArchEndpoint(parts[len(parts)-1])
	if from == "" || to == "" {
		return Edge{}, false
	}
	return Edge{ID: fmt.Sprintf("edge_%02d", index), From: from, To: to, FromPort: fromPort, ToPort: toPort, Label: cleanText(label), Kind: kindFromLabel(label), Directed: strings.Contains(line, ">") || strings.Contains(line, "<"), Role: roleFromLabel(label), PathGroup: pathGroupFromLabel(label)}, true
}

func parseFlowEdge(line string, index int) (Edge, bool) {
	label := labelBetweenBars(line)
	cleaned := stripBarLabels(line)
	re := regexp.MustCompile(`(.+?)\s*(-\.->|==>|-->|---|--|->)\s*(.+)`)
	m := re.FindStringSubmatch(cleaned)
	if len(m) == 0 {
		return Edge{}, false
	}
	fromID, fromLabel, fromKind := parseNodeRef(m[1])
	toID, toLabel, toKind := parseNodeRef(m[3])
	if fromID == "" || toID == "" {
		return Edge{}, false
	}
	_ = fromKind
	_ = toKind
	edge := Edge{ID: fmt.Sprintf("edge_%02d", index), From: fromID, To: toID, Label: cleanText(label), Kind: kindFromLabel(label), Directed: strings.Contains(m[2], ">"), Dashed: strings.Contains(m[2], "."), Thick: strings.Contains(m[2], "="), Role: roleFromLabel(label), PathGroup: pathGroupFromLabel(label)}
	if edge.Label == "" {
		edge.Label = "link"
	}
	if fromLabel != "" {
		edge.Label = firstNonEmpty(edge.Label, fromLabel)
	}
	_ = toLabel
	return edge, true
}

func parseSequenceMessage(line string, index int) (Message, bool) {
	operators := []string{"-->>", "->>", "--x", "-x", "-->", "->"}
	for _, op := range operators {
		if idx := strings.Index(line, op); idx >= 0 {
			from := sanitizeID(line[:idx])
			rest := strings.TrimSpace(line[idx+len(op):])
			label := ""
			if colon := strings.Index(rest, ":"); colon >= 0 {
				label = rest[colon+1:]
				rest = rest[:colon]
			}
			to := sanitizeID(rest)
			if from == "" || to == "" {
				return Message{}, false
			}
			return Message{ID: fmt.Sprintf("m%d", index), Order: index, From: from, To: to, Label: nonEmpty(cleanText(label), "message"), Kind: "sync", Dashed: strings.HasPrefix(op, "--")}, true
		}
	}
	return Message{}, false
}

func parseClassEdge(line string, index int) (Edge, bool) {
	re := regexp.MustCompile(`([A-Za-z0-9_.:-]+)\s+([<|*o.]?--[>|*o.]?|<\|--|--\|>)\s+([A-Za-z0-9_.:-]+)(?:\s*:\s*(.*))?`)
	m := re.FindStringSubmatch(line)
	if len(m) == 0 {
		return Edge{}, false
	}
	return Edge{ID: fmt.Sprintf("edge_%02d", index), From: m[1], To: m[3], Label: cleanText(m[4]), Kind: "relationship", Directed: strings.Contains(m[2], ">")}, true
}

func addNode(d *Diagram, id, label, kind, group string) {
	if id == "" || id == "[*]" {
		return
	}
	if existing := d.Nodes[id]; existing != nil {
		if existing.Label == id && label != "" {
			existing.Label = label
		}
		if existing.Kind == "" && kind != "" {
			existing.Kind = kind
		}
		if existing.Group == "" && group != "" {
			existing.Group = group
		}
		return
	}
	d.Nodes[id] = &Node{ID: id, Label: nonEmpty(label, id), Kind: kind, Group: group}
}

func parseNodeRef(raw string) (string, string, string) {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"` ")
	if s == "" {
		return "", "", ""
	}
	idEnd := 0
	for idEnd < len(s) {
		r := rune(s[idEnd])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == ':' {
			idEnd++
			continue
		}
		break
	}
	id := strings.TrimSpace(s[:idEnd])
	rest := strings.TrimSpace(s[idEnd:])
	label := extractBracketLabel(rest)
	kind := kindFromShape(rest)
	return sanitizeID(id), cleanText(label), kind
}

func parseFlowEndpointInto(d *Diagram, raw, group string) {
	id, label, kind := parseNodeRef(raw)
	if id != "" {
		addNode(d, id, nonEmpty(label, id), kind, group)
	}
}

func leftEndpoint(line string) string {
	for _, token := range []string{"-.->", "==>", "-->", "---", "--", "->"} {
		if i := strings.Index(line, token); i >= 0 {
			return line[:i]
		}
	}
	return line
}

func rightEndpoint(line string) string {
	for _, token := range []string{"-.->", "==>", "-->", "---", "--", "->"} {
		if i := strings.Index(line, token); i >= 0 {
			return line[i+len(token):]
		}
	}
	return line
}

func parseSubgraph(line string) (string, string) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "subgraph"))
	id, label, _ := parseNodeRef(body)
	if id == "" {
		id = sanitizeID(body)
	}
	if label == "" {
		label = body
	}
	return id, cleanText(label)
}

func stripCodeFence(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
		if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			lines = lines[:len(lines)-1]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func splitFrontmatter(text string) (map[string]any, string) {
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]any{}, text
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			var fm map[string]any
			if err := yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &fm); err != nil || fm == nil {
				fm = map[string]any{}
			}
			return fm, strings.Join(lines[i+1:], "\n")
		}
	}
	return map[string]any{}, text
}

func cleanedLines(text string) []string {
	var out []string
	for _, line := range rawLines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		if strings.HasPrefix(line, "title ") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func rawLines(text string) []string {
	return strings.Split(text, "\n")
}

func titleFromLines(lines []string) string {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "title ") {
			return strings.TrimSpace(line[6:])
		}
	}
	return ""
}

func normalizeKind(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ":")
	return value
}

func knownMermaidKind(kind string) bool {
	switch kind {
	case "architecture", "architecture-beta",
		"flowchart", "graph",
		"sequencediagram", "sequencediagram-v2", "zenuml",
		"classdiagram", "classdiagram-v2",
		"statediagram", "statediagram-v2", "statediagram-v2-beta",
		"erdiagram", "gantt", "timeline", "journey", "pie",
		"mindmap", "gitgraph", "quadrantchart", "requirementdiagram", "c4context",
		"xychart", "xychart-beta", "block", "block-beta", "packet", "packet-beta",
		"sankey", "sankey-beta", "kanban", "radar", "radar-beta", "eventmodeling", "treemap", "treemap-beta",
		"venn", "ishikawa", "wardley", "wardley-beta", "treeview":
		return true
	default:
		return false
	}
}

func objectFromMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	if obj, ok := m[key].(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func mergeEFP(data, efp map[string]any) map[string]any {
	if efp == nil {
		return data
	}
	for _, key := range []string{"camera", "canvas", "renderHints", "visual", "view", "theme", "title", "subtitle", "goal"} {
		if value, ok := efp[key]; ok {
			if dst, ok := data[key].(map[string]any); ok {
				if src, ok := value.(map[string]any); ok {
					data[key] = mergeMap(dst, src)
					continue
				}
			}
			data[key] = value
		}
	}
	return data
}

func mergeMap(dst, src map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if dstObj, ok := out[k].(map[string]any); ok {
			if srcObj, ok := v.(map[string]any); ok {
				out[k] = mergeMap(dstObj, srcObj)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func mergeMetadata(dst, src map[string]any) map[string]any {
	return mergeMap(dst, src)
}

func sortedNodes(d Diagram) []Node {
	out := make([]Node, 0, len(d.Nodes))
	for _, node := range d.Nodes {
		out = append(out, *node)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedGroups(d Diagram) []Group {
	out := make([]Group, 0, len(d.Groups))
	for _, group := range d.Groups {
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func edgeList(d Diagram) []any {
	out := []any{}
	for i, edge := range d.Edges {
		out = append(out, map[string]any{"id": nonEmpty(edge.ID, fmt.Sprintf("edge_%02d", i+1)), "from": edge.From, "to": edge.To, "label": nonEmpty(edge.Label, edge.Kind, "link"), "kind": nonEmpty(edge.Kind, "relationship"), "directed": edge.Directed})
	}
	return out
}

func messagesOrEdges(d Diagram) []Message {
	if len(d.Messages) > 0 {
		return d.Messages
	}
	out := []Message{}
	for i, edge := range d.Edges {
		out = append(out, Message{ID: nonEmpty(edge.ID, fmt.Sprintf("m%d", i+1)), Order: i + 1, From: edge.From, To: edge.To, Label: nonEmpty(edge.Label, edge.Kind, "link"), Kind: nonEmpty(edge.Kind, "message"), Dashed: edge.Dashed})
	}
	return out
}

func sourceMetadata(d Diagram) map[string]any {
	return map[string]any{"source": "mermaid", "mermaid_kind": d.Kind}
}

func inferEntityKind(node Node) string {
	value := strings.ToLower(nonEmpty(node.Kind, node.Label, node.ID))
	switch {
	case strings.Contains(value, "browser"), strings.Contains(value, "internet"), strings.Contains(value, "pc"):
		return "browser"
	case strings.Contains(value, "mobile"):
		return "mobile"
	case strings.Contains(value, "cdn"), strings.Contains(value, "cloudfront"):
		return "cdn"
	case strings.Contains(value, "database"), strings.Contains(value, "mysql"), strings.Contains(value, "postgres"), strings.Contains(value, "db"):
		return "database"
	case strings.Contains(value, "redis"), strings.Contains(value, "cache"):
		return "redis"
	case strings.Contains(value, "queue"), strings.Contains(value, "sqs"), strings.Contains(value, "kafka"):
		return "queue"
	case strings.Contains(value, "gateway"), strings.Contains(value, "api"):
		return "api_gateway"
	case strings.Contains(value, "nginx"), strings.Contains(value, "proxy"):
		return "nginx"
	case strings.Contains(value, "nacos"), strings.Contains(value, "registry"), strings.Contains(value, "discovery"):
		return "registry"
	case strings.Contains(value, "elastic"), strings.Contains(value, "log"), strings.Contains(value, "prometheus"), strings.Contains(value, "grafana"), strings.Contains(value, "observ"):
		return "observability"
	case strings.Contains(value, "admin"), strings.Contains(value, "springboot"):
		return "admin"
	case strings.Contains(value, "microservice"), strings.Contains(value, "service"), strings.Contains(value, "server"):
		return "service"
	case strings.Contains(value, "client"), strings.Contains(value, "user"), strings.Contains(value, "actor"), strings.Contains(value, "pc"), strings.Contains(value, "mobile"):
		return "client"
	case strings.Contains(value, "storage"), strings.Contains(value, "bucket"), strings.Contains(value, "oss"), strings.Contains(value, "s3"):
		return "storage"
	default:
		return nonEmpty(node.Kind, "service")
	}
}

func iconForKind(kind string) string {
	switch kind {
	case "browser", "mobile", "client":
		return "generic.user"
	case "cdn":
		return "generic.external"
	case "database":
		return "generic.database"
	case "redis":
		return "redis"
	case "queue":
		return "generic.queue"
	case "api_gateway":
		return "generic.api"
	case "nginx":
		return "nginx"
	case "storage":
		return "generic.storage"
	case "registry":
		return "generic.service"
	case "observability":
		return "generic.job"
	case "admin":
		return "generic.warning"
	default:
		return "generic.service"
	}
}

func importanceForRole(role string) float64 {
	if role == "primary" {
		return 0.9
	}
	if role == "auxiliary" {
		return 0.34
	}
	return 0.62
}

func labelPriorityForRole(role string) string {
	if role == "primary" {
		return "always"
	}
	if role == "auxiliary" {
		return "hidden"
	}
	return "important"
}

func lineStyle(edge Edge, role string) string {
	if edge.Dashed || role == "auxiliary" {
		return "dashed"
	}
	return "solid"
}

func colorForRole(role string) string {
	if role == "primary" {
		return "#111827"
	}
	if role == "auxiliary" {
		return "#475569"
	}
	return "#334155"
}

func roleFromLabel(label string) string {
	value := strings.ToLower(label)
	if strings.Contains(value, "api") || strings.Contains(value, "primary") || strings.Contains(value, "entry") {
		return "primary"
	}
	if strings.Contains(value, "health") || strings.Contains(value, "log") || strings.Contains(value, "observe") {
		return "auxiliary"
	}
	return ""
}

func pathGroupFromLabel(label string) string {
	value := strings.ToLower(label)
	for _, key := range []string{"entry", "gateway", "registry", "cache", "data", "storage", "health", "observability"} {
		if strings.Contains(value, key) {
			return key
		}
	}
	if strings.Contains(value, "api") {
		return "gateway"
	}
	return ""
}

func kindFromLabel(label string) string {
	value := strings.ToLower(strings.TrimSpace(label))
	if value == "" {
		return "depends_on"
	}
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "depends_on"
	}
	return value
}

func extractBracketLabel(rest string) string {
	for _, pair := range [][2]string{{"[", "]"}, {"(", ")"}, {"{", "}"}} {
		start := strings.Index(rest, pair[0])
		end := strings.LastIndex(rest, pair[1])
		if start >= 0 && end > start {
			return strings.Trim(rest[start+1:end], "\"` ")
		}
	}
	return ""
}

func kindFromShape(rest string) string {
	if strings.Contains(rest, "{") {
		return "decision"
	}
	if strings.Contains(rest, "((") {
		return "event"
	}
	return ""
}

func labelBetweenBars(line string) string {
	if label := barLabel(line); label != "" {
		return label
	}
	if idx := strings.Index(line, ":"); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return ""
}

func barLabel(line string) string {
	start := strings.Index(line, "|")
	if start < 0 {
		return ""
	}
	end := strings.Index(line[start+1:], "|")
	if end < 0 {
		return ""
	}
	return cleanText(line[start+1 : start+1+end])
}

func stripBarLabels(line string) string {
	re := regexp.MustCompile(`\|[^|]*\|`)
	return re.ReplaceAllString(line, " ")
}

func archEndpointID(raw string) string {
	id, _ := parseArchEndpoint(raw)
	return id
}

func parseArchEndpoint(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"` ")
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if isMermaidPort(left) {
			return sanitizeID(right), strings.ToUpper(left)
		}
		if isMermaidPort(right) {
			return sanitizeID(left), strings.ToUpper(right)
		}
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	return sanitizeID(fields[0]), ""
}

func isMermaidPort(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	return len(value) == 1 && strings.Contains("LRTB", value)
}

func cleanText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"`")
	value = strings.ReplaceAll(value, "<br/>", " ")
	value = strings.ReplaceAll(value, "<br>", " ")
	return strings.Join(strings.Fields(value), " ")
}

func compact(value string, maxLen int) string {
	value = cleanText(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen-1]) + "…"
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"`[](){}")
	if value == "" {
		return ""
	}
	value = regexp.MustCompile(`[^A-Za-z0-9_.:-]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	if _, err := strconv.Atoi(value[:1]); err == nil {
		value = "n_" + value
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmpty(values ...string) string {
	return firstNonEmpty(values...)
}

func firstStringFromAny(values []any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
