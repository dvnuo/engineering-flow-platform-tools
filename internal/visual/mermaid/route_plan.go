package mermaid

import (
	"context"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

const RouteEngineSemanticHeuristicV4 = "semantic_heuristic_v4"

type RouteInput struct {
	Graph  ArchitectureSemanticGraph `json:"graph"`
	Layout ArchitectureLayoutResult  `json:"layout"`
}

type CompileOptions struct{}

func BuildArchitectureRoutePlan(ctx context.Context, input RouteInput) (ArchitectureRoutePlan, error) {
	_ = ctx
	routing := ArchitectureRoutingEngine(input.Graph, input.Layout)
	return routing.ToRoutePlan(), nil
}

func GenerateRoutePlan(ctx context.Context, raw []byte) (map[string]any, error) {
	diagram, ok := parse(raw)
	if !ok {
		return nil, metadata.NewError("mermaid_input_required", "visual route-plan accepts Mermaid architecture input.", "Pass a Mermaid architecture-beta file.", 400)
	}
	if diagram.Kind != "architecture-beta" && diagram.Kind != "architecture" {
		return nil, metadata.NewError("route_plan_unsupported_mermaid", "route-plan currently supports Mermaid architecture diagrams.", "Pass architecture-beta input or render the diagram normally.", 400)
	}
	graph := BuildArchitectureSemanticGraph(diagram)
	layout := ArchitectureLayoutEngine(graph)
	plan, err := BuildArchitectureRoutePlan(ctx, RouteInput{Graph: graph, Layout: layout})
	if err != nil {
		return nil, err
	}
	return plan.ToVisualRoutePlan(), nil
}
