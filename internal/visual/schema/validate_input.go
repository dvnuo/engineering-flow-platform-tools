package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"engineering-flow-platform-tools/internal/visual/manifest"
	"engineering-flow-platform-tools/internal/visual/metadata"
)

type ParsedInput struct {
	Data    map[string]any `json:"data"`
	Title   string         `json:"title,omitempty"`
	Summary InputSummary   `json:"summary"`
}

type InputSummary struct {
	Schema  string `json:"schema,omitempty"`
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Nodes   int    `json:"nodes,omitempty"`
	Edges   int    `json:"edges,omitempty"`
	Events  int    `json:"events,omitempty"`
	Claims  int    `json:"claims,omitempty"`
	Sources int    `json:"sources,omitempty"`
	Links   int    `json:"links,omitempty"`
	Items   int    `json:"items,omitempty"`
}

func ValidateInput(kind string, raw []byte, limits manifest.LimitsSpec) (ParsedInput, error) {
	kind = normalizeKind(kind)
	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return ParsedInput{}, metadata.NewError("input_parse_failed", "visual input JSON could not be parsed: "+err.Error(), "Pass a valid JSON object to --input.", 400)
	}
	if data == nil {
		return ParsedInput{}, invalid("visual input root must be an object.", "Wrap the input in a JSON object with the required schema fields.")
	}
	if dec.More() {
		return ParsedInput{}, metadata.NewError("input_parse_failed", "visual input JSON contains extra tokens.", "Pass one JSON object only.", 400)
	}
	if err := validateSchemaField(kind, data); err != nil {
		return ParsedInput{}, err
	}
	summary := InputSummary{Kind: kind, Title: titleFromData(data)}
	if schemaValue, _ := data["schema"].(string); schemaValue != "" {
		summary.Schema = schemaValue
	}
	switch kind {
	case "graph_v1":
		graph, err := validateGraph(data, limits, false)
		if err != nil {
			return ParsedInput{}, err
		}
		summary.Nodes = graph.nodes
		summary.Edges = graph.edges
	case "graph_events_v1":
		graph, err := validateGraph(data, limits, true)
		if err != nil {
			return ParsedInput{}, err
		}
		summary.Nodes = graph.nodes
		summary.Edges = graph.edges
		summary.Events = graph.events
	case "timeline_v1":
		events, err := requiredArray(data, "events")
		if err != nil {
			return ParsedInput{}, err
		}
		if len(events) > limitOrDefault(limits.MaxEvents, 5000) {
			return ParsedInput{}, invalid("visual input has too many events.", "Reduce events or raise template limits.")
		}
		if err := validateEvents(events, nil); err != nil {
			return ParsedInput{}, err
		}
		summary.Events = len(events)
	case "evidence_v1":
		counts, err := validateEvidence(data)
		if err != nil {
			return ParsedInput{}, err
		}
		summary.Claims = counts.claims
		summary.Sources = counts.sources
		summary.Links = counts.links
	case "matrix_v1":
		items, err := requiredArray(data, "items")
		if err != nil {
			return ParsedInput{}, err
		}
		if len(items) > limitOrDefault(limits.MaxNodes, 1000) {
			return ParsedInput{}, invalid("visual input has too many matrix items.", "Reduce items or raise template max_nodes.")
		}
		if err := validateMatrixItems(items); err != nil {
			return ParsedInput{}, err
		}
		summary.Items = len(items)
	default:
		return ParsedInput{}, invalid("visual input schema kind is not supported: "+kind, "Use graph_v1, graph_events_v1, timeline_v1, evidence_v1, or matrix_v1.")
	}
	return ParsedInput{Data: data, Title: summary.Title, Summary: summary}, nil
}

func validateSchemaField(kind string, data map[string]any) error {
	expected := map[string]string{
		"graph_v1":        "efp.visual.input.graph.v1",
		"graph_events_v1": "efp.visual.input.graph_events.v1",
		"timeline_v1":     "efp.visual.input.timeline.v1",
		"evidence_v1":     "efp.visual.input.evidence.v1",
		"matrix_v1":       "efp.visual.input.matrix.v1",
	}
	value, ok := data["schema"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	if expected[kind] != "" && value != expected[kind] {
		return invalid("visual input schema does not match template kind.", "Use schema "+expected[kind]+" for "+kind+".")
	}
	return nil
}

type graphCounts struct {
	nodes  int
	edges  int
	events int
}

func validateGraph(data map[string]any, limits manifest.LimitsSpec, withEvents bool) (graphCounts, error) {
	nodes, err := requiredArray(data, "nodes")
	if err != nil {
		return graphCounts{}, err
	}
	if len(nodes) > limitOrDefault(limits.MaxNodes, 1000) {
		return graphCounts{}, invalid("visual input has too many graph nodes.", "Reduce nodes or raise template max_nodes.")
	}
	nodeIDs := map[string]bool{}
	for i, item := range nodes {
		obj, ok := item.(map[string]any)
		if !ok {
			return graphCounts{}, invalid(fmt.Sprintf("graph node at index %d must be an object.", i), "Each node must contain at least a non-empty string id.")
		}
		id := stringField(obj, "id")
		if id == "" {
			return graphCounts{}, invalid(fmt.Sprintf("graph node at index %d is missing id.", i), "Set node.id to a non-empty string.")
		}
		if nodeIDs[id] {
			return graphCounts{}, invalid("graph node ids must be unique.", "Rename duplicate node id "+id+".")
		}
		nodeIDs[id] = true
	}
	edges, err := optionalArray(data, "edges")
	if err != nil {
		return graphCounts{}, err
	}
	if len(edges) > limitOrDefault(limits.MaxEdges, 3000) {
		return graphCounts{}, invalid("visual input has too many graph edges.", "Reduce edges or raise template max_edges.")
	}
	for i, item := range edges {
		obj, ok := item.(map[string]any)
		if !ok {
			return graphCounts{}, invalid(fmt.Sprintf("graph edge at index %d must be an object.", i), "Each edge must contain from and to node ids.")
		}
		from := stringField(obj, "from")
		to := stringField(obj, "to")
		if from == "" || to == "" {
			return graphCounts{}, invalid(fmt.Sprintf("graph edge at index %d is missing from/to.", i), "Set edge.from and edge.to to existing node ids.")
		}
		if !nodeIDs[from] || !nodeIDs[to] {
			return graphCounts{}, invalid(fmt.Sprintf("graph edge at index %d references an unknown node.", i), "Ensure every edge.from and edge.to points to an existing node id.")
		}
	}
	var events []any
	if withEvents {
		events, err = optionalArray(data, "events")
		if err != nil {
			return graphCounts{}, err
		}
		if len(events) > limitOrDefault(limits.MaxEvents, 5000) {
			return graphCounts{}, invalid("visual input has too many events.", "Reduce events or raise template max_events.")
		}
		if err := validateEvents(events, nodeIDs); err != nil {
			return graphCounts{}, err
		}
	}
	return graphCounts{nodes: len(nodes), edges: len(edges), events: len(events)}, nil
}

func validateEvents(events []any, nodeIDs map[string]bool) error {
	eventIDs := map[string]bool{}
	for i, item := range events {
		obj, ok := item.(map[string]any)
		if !ok {
			return invalid(fmt.Sprintf("event at index %d must be an object.", i), "Each event must contain a non-empty string id.")
		}
		id := stringField(obj, "id")
		if id == "" {
			return invalid(fmt.Sprintf("event at index %d is missing id.", i), "Set event.id to a non-empty string.")
		}
		if eventIDs[id] {
			return invalid("event ids must be unique.", "Rename duplicate event id "+id+".")
		}
		eventIDs[id] = true
		nodeID := stringField(obj, "node_id")
		if nodeID != "" && nodeIDs != nil && !nodeIDs[nodeID] {
			return invalid("event references an unknown node_id: "+nodeID, "Set event.node_id to an existing node id or omit it.")
		}
	}
	return nil
}

type evidenceCounts struct {
	claims  int
	sources int
	links   int
}

func validateEvidence(data map[string]any) (evidenceCounts, error) {
	claims, err := requiredArray(data, "claims")
	if err != nil {
		return evidenceCounts{}, err
	}
	sources, err := requiredArray(data, "sources")
	if err != nil {
		return evidenceCounts{}, err
	}
	links, err := optionalArray(data, "links")
	if err != nil {
		return evidenceCounts{}, err
	}
	claimIDs := map[string]bool{}
	sourceIDs := map[string]bool{}
	for i, item := range claims {
		obj, ok := item.(map[string]any)
		if !ok {
			return evidenceCounts{}, invalid(fmt.Sprintf("claim at index %d must be an object.", i), "Each claim must contain a non-empty string id.")
		}
		id := stringField(obj, "id")
		if id == "" {
			return evidenceCounts{}, invalid(fmt.Sprintf("claim at index %d is missing id.", i), "Set claim.id to a non-empty string.")
		}
		if claimIDs[id] {
			return evidenceCounts{}, invalid("claim ids must be unique.", "Rename duplicate claim id "+id+".")
		}
		claimIDs[id] = true
		if confidence, ok := obj["confidence"]; ok && !isNumber(confidence) {
			return evidenceCounts{}, invalid("claim confidence must be numeric.", "Set confidence to a number between 0 and 1.")
		}
	}
	for i, item := range sources {
		obj, ok := item.(map[string]any)
		if !ok {
			return evidenceCounts{}, invalid(fmt.Sprintf("source at index %d must be an object.", i), "Each source must contain a non-empty string id.")
		}
		id := stringField(obj, "id")
		if id == "" {
			return evidenceCounts{}, invalid(fmt.Sprintf("source at index %d is missing id.", i), "Set source.id to a non-empty string.")
		}
		if sourceIDs[id] {
			return evidenceCounts{}, invalid("source ids must be unique.", "Rename duplicate source id "+id+".")
		}
		sourceIDs[id] = true
	}
	for i, item := range links {
		obj, ok := item.(map[string]any)
		if !ok {
			return evidenceCounts{}, invalid(fmt.Sprintf("evidence link at index %d must be an object.", i), "Each link must contain claim_id and source_id.")
		}
		claimID := stringField(obj, "claim_id")
		sourceID := stringField(obj, "source_id")
		if !claimIDs[claimID] || !sourceIDs[sourceID] {
			return evidenceCounts{}, invalid(fmt.Sprintf("evidence link at index %d references an unknown claim or source.", i), "Ensure link.claim_id and link.source_id reference existing ids.")
		}
	}
	return evidenceCounts{claims: len(claims), sources: len(sources), links: len(links)}, nil
}

func validateMatrixItems(items []any) error {
	itemIDs := map[string]bool{}
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return invalid(fmt.Sprintf("matrix item at index %d must be an object.", i), "Each item must contain a non-empty string id.")
		}
		id := stringField(obj, "id")
		if id == "" {
			return invalid(fmt.Sprintf("matrix item at index %d is missing id.", i), "Set item.id to a non-empty string.")
		}
		if itemIDs[id] {
			return invalid("matrix item ids must be unique.", "Rename duplicate item id "+id+".")
		}
		itemIDs[id] = true
		for _, axis := range []string{"x", "y"} {
			if value, ok := obj[axis]; ok && !isNumber(value) {
				return invalid("matrix item "+axis+" must be numeric.", "Set "+axis+" to a number between 0 and 1.")
			}
		}
	}
	return nil
}

func requiredArray(data map[string]any, name string) ([]any, error) {
	value, ok := data[name]
	if !ok {
		return nil, invalid("visual input is missing required array "+name+".", "Add "+name+" as a JSON array.")
	}
	arr, ok := value.([]any)
	if !ok {
		return nil, invalid("visual input field "+name+" must be an array.", "Set "+name+" to a JSON array.")
	}
	return arr, nil
}

func optionalArray(data map[string]any, name string) ([]any, error) {
	value, ok := data[name]
	if !ok || value == nil {
		return nil, nil
	}
	arr, ok := value.([]any)
	if !ok {
		return nil, invalid("visual input field "+name+" must be an array.", "Set "+name+" to a JSON array or omit it.")
	}
	return arr, nil
}

func stringField(data map[string]any, name string) string {
	value, _ := data[name].(string)
	return strings.TrimSpace(value)
}

func isNumber(value any) bool {
	switch v := value.(type) {
	case json.Number:
		_, err := v.Float64()
		return err == nil
	case float64, float32, int, int64, int32:
		return true
	default:
		return false
	}
}

func limitOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func invalid(message, hint string) error {
	return metadata.NewError("template_input_invalid", message, hint, 400)
}
