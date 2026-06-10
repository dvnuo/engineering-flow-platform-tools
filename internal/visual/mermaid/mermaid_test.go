package mermaid

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/visual/manifest"
	visualschema "engineering-flow-platform-tools/internal/visual/schema"
)

func TestInferTemplateID(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"architecture", "architecture-beta\n  service api(server)[API]\n", "mermaid.architecture"},
		{"flowchart", "flowchart LR\n  A --> B\n", "mermaid.flowchart"},
		{"sequence", "sequenceDiagram\n  A->>B: call\n", "mermaid.sequence"},
		{"class", "classDiagram\n  A <|-- B\n", "mermaid.class"},
		{"pie", "pie title Pets\n  \"Dogs\" : 12\n", "mermaid.pie"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := InferTemplateID([]byte(tc.src))
			if !ok || got != tc.want {
				t.Fatalf("InferTemplateID()=%q,%v want %q,true", got, ok, tc.want)
			}
		})
	}
}

func TestRoutePlanSerializesInternalEngine(t *testing.T) {
	raw := []byte(`architecture-beta
  group client(cloud)[Client Zone]
  group edge(server)[Edge Zone]
  group app(server)[Application Zone]
  group data(database)[Data Zone]
  service browser(internet)[Browser] in client
  service gateway(server)[API Gateway] in edge
  service service(server)[Order Service] in app
  service db(database)[Order Database] in data
  browser:R -->|API| L:gateway
  gateway:R -->|Service| L:service
  service:R -->|Data| L:db
`)
	compiled, err := CompileIfNeededWithOptions(context.Background(), "isometric_architecture_v1", raw, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(compiled, &data); err != nil {
		t.Fatal(err)
	}
	routePlan, ok := data["routePlan"].(map[string]any)
	if !ok {
		t.Fatal("expected routePlan in compiled architecture data")
	}
	if routePlan["backend"] != RouteEngineSemanticHeuristicV4 {
		t.Fatalf("routePlan backend=%#v want %q", routePlan["backend"], RouteEngineSemanticHeuristicV4)
	}
	routes, _ := routePlan["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("expected 3 routePlan routes, got %d", len(routes))
	}
}

func TestCompileMicroserviceGoldenArchitecture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "templates", "visual", "mermaid.architecture", "examples", "microservice-golden.mmd")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diagram, ok := parse(raw)
	if !ok {
		t.Fatal("expected microservice-golden.mmd to parse")
	}
	graph := BuildArchitectureSemanticGraph(diagram)
	if len(graph.Groups) < 10 || len(graph.Nodes) < 18 || len(graph.Edges) < 18 {
		t.Fatalf("golden example too small: groups=%d nodes=%d edges=%d", len(graph.Groups), len(graph.Nodes), len(graph.Edges))
	}
	layout := ArchitectureLayoutEngine(graph)
	zones := map[string]ArchitectureZoneLayout{}
	for _, zone := range layout.Zones {
		zones[zone.Group.ID] = zone
	}
	for _, id := range []string{"client", "edge", "gateway", "services", "registry", "cache", "data", "storage", "observability", "admin"} {
		if _, ok := zones[id]; !ok {
			t.Fatalf("expected zone %s in golden layout", id)
		}
	}
	if !(zones["client"].Bounds.X < zones["edge"].Bounds.X && zones["edge"].Bounds.X < zones["gateway"].Bounds.X && zones["gateway"].Bounds.X < zones["services"].Bounds.X) {
		t.Fatalf("expected ingress/gateway/service zones to flow left-to-right: %#v", zones)
	}
	if zones["registry"].Bounds.Y >= zones["services"].Bounds.Y {
		t.Fatalf("expected registry in the upper/right service orbit: services=%#v registry=%#v", zones["services"].Bounds, zones["registry"].Bounds)
	}
	if zones["cache"].Bounds.Y <= zones["services"].Bounds.Y || zones["data"].Bounds.Y <= zones["services"].Bounds.Y || zones["storage"].Bounds.Y <= zones["services"].Bounds.Y {
		t.Fatalf("expected cache/data/storage below service zone: services=%#v cache=%#v data=%#v storage=%#v", zones["services"].Bounds, zones["cache"].Bounds, zones["data"].Bounds, zones["storage"].Bounds)
	}
	routing := ArchitectureRoutingEngine(graph, layout)
	if len(routing.SourceEdges) != len(graph.Edges) {
		t.Fatalf("expected every source edge to be preserved: source=%d edges=%d", len(routing.SourceEdges), len(graph.Edges))
	}
	if len(routing.Links) > 12 || len(routing.Links) < 8 {
		t.Fatalf("expected overview display route aggregation: display=%d source=%d", len(routing.Links), len(graph.Edges))
	}
	if len(routing.HiddenDetailLinks) < 4 {
		t.Fatalf("expected hidden detail routes for aggregated overview: hidden=%d", len(routing.HiddenDetailLinks))
	}
	if routing.Metrics.PrimaryRouteCount > 8 {
		t.Fatalf("too many primary routes: %#v", routing.Metrics)
	}
	if routing.Metrics.SecondaryRouteCount < 4 {
		t.Fatalf("expected secondary route layer: %#v", routing.Metrics)
	}
	if routing.Metrics.BusLaneCount == 0 || routing.Metrics.BundleCount == 0 {
		t.Fatalf("expected bus lane/bundle metrics: %#v", routing.Metrics)
	}
	if routing.Metrics.BusLaneCount < 4 || routing.Metrics.BundleCount < 2 {
		t.Fatalf("expected complex pathGroup bus lane metrics: %#v", routing.Metrics)
	}
	if routing.Metrics.PortHintViolations != 0 || routing.Metrics.DirectionViolations != 0 {
		t.Fatalf("golden routes should respect port hints: %#v", routing.Metrics)
	}
	compiled, err := CompileIfNeeded("isometric_architecture_v1", raw)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(compiled, &data); err != nil {
		t.Fatal(err)
	}
	if len(data["zones"].([]any)) < 10 || len(data["entities"].([]any)) < 18 || len(data["links"].([]any)) < 8 || len(data["links"].([]any)) > 12 {
		t.Fatalf("compiled golden example lost structure: zones=%d entities=%d links=%d", len(data["zones"].([]any)), len(data["entities"].([]any)), len(data["links"].([]any)))
	}
	routePlan, ok := data["routePlan"].(map[string]any)
	if !ok {
		t.Fatalf("compiled golden example did not include routePlan")
	}
	if routePlan["version"] != "efp.routeplan.v2" {
		t.Fatalf("unexpected routePlan version: %#v", routePlan["version"])
	}
	routes, _ := routePlan["routes"].([]any)
	sourceEdges, _ := routePlan["sourceEdges"].([]any)
	displayRoutes, _ := routePlan["displayRoutes"].([]any)
	hiddenDetailRoutes, _ := routePlan["hiddenDetailRoutes"].([]any)
	lanes, _ := routePlan["lanes"].([]any)
	obstacles, _ := routePlan["obstacles"].([]any)
	if len(routes) != len(data["links"].([]any)) {
		t.Fatalf("routePlan routes should match links: routes=%d links=%d", len(routes), len(data["links"].([]any)))
	}
	if len(displayRoutes) != len(routes) {
		t.Fatalf("displayRoutes should alias rendered overview routes: display=%d routes=%d", len(displayRoutes), len(routes))
	}
	if len(sourceEdges) != len(graph.Edges) {
		t.Fatalf("sourceEdges should preserve Mermaid edges: source=%d edges=%d", len(sourceEdges), len(graph.Edges))
	}
	if len(hiddenDetailRoutes) < 4 {
		t.Fatalf("expected hidden detail routes, got %d", len(hiddenDetailRoutes))
	}
	if len(lanes) < 5 {
		t.Fatalf("expected routePlan bus lanes for complex architecture: lanes=%d", len(lanes))
	}
	if len(obstacles) < len(data["entities"].([]any)) {
		t.Fatalf("expected routePlan obstacles to cover entities: obstacles=%d entities=%d", len(obstacles), len(data["entities"].([]any)))
	}
}

func TestOfficialMermaidKindsAreAccepted(t *testing.T) {
	cases := map[string]string{
		"architecture-beta":  "mermaid.architecture",
		"architecture":       "mermaid.architecture",
		"flowchart":          "mermaid.flowchart",
		"graph":              "mermaid.flowchart",
		"sequenceDiagram":    "mermaid.sequence",
		"zenuml":             "mermaid.zenuml",
		"classDiagram":       "mermaid.class",
		"erDiagram":          "mermaid.er",
		"stateDiagram":       "mermaid.state",
		"gantt":              "mermaid.gantt",
		"timeline":           "mermaid.timeline",
		"journey":            "mermaid.journey",
		"gitGraph":           "mermaid.gitgraph",
		"mindmap":            "mermaid.mindmap",
		"treeView":           "mermaid.treeview",
		"sankey-beta":        "mermaid.sankey",
		"xychart-beta":       "mermaid.xy",
		"block-beta":         "mermaid.block",
		"packet-beta":        "mermaid.packet",
		"pie":                "mermaid.pie",
		"quadrantChart":      "mermaid.quadrant",
		"kanban":             "mermaid.kanban",
		"radar":              "mermaid.radar",
		"treemap":            "mermaid.treemap",
		"requirementDiagram": "mermaid.requirement",
		"c4Context":          "mermaid.c4",
		"eventModeling":      "mermaid.event_modeling",
		"venn":               "mermaid.venn",
		"ishikawa":           "mermaid.ishikawa",
		"wardley":            "mermaid.wardley",
	}
	for kind, want := range cases {
		t.Run(kind, func(t *testing.T) {
			raw := []byte(kind + "\n  A --> B\n")
			got, ok := InferTemplateID(raw)
			if !ok || got != want {
				t.Fatalf("InferTemplateID(%s)=%q,%v want %q,true", kind, got, ok, want)
			}
			if compiled, err := CompileIfNeeded("graph_v1", raw); err != nil || len(compiled) == 0 {
				t.Fatalf("CompileIfNeeded(%s) len=%d err=%v", kind, len(compiled), err)
			}
		})
	}
}

func TestCompileFlowchartToGraph(t *testing.T) {
	raw := []byte(`flowchart LR
  A[User Browser] -->|API| B[API Gateway]
  B -->|writes| C[(Database)]
`)
	compiled, err := CompileIfNeeded("graph_v1", raw)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(compiled, &data); err != nil {
		t.Fatal(err)
	}
	parsed, err := visualschema.ValidateInput("graph_v1", compiled, manifest.LimitsSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Summary.Nodes != 3 || parsed.Summary.Edges != 2 || data["schema"] != "efp.visual.input.graph.v1" {
		t.Fatalf("unexpected compiled graph summary/data: %#v %#v", parsed.Summary, data)
	}
}

func TestCompileSequenceDiagram(t *testing.T) {
	raw := []byte(`sequenceDiagram
  actor User
  participant Portal
  participant API
  User->>Portal: submitCheckout()
  Portal->>API: createOrder()
  API-->>Portal: orderCreated
`)
	compiled, err := CompileIfNeeded("uml_sequence_v1", raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := visualschema.ValidateInput("uml_sequence_v1", compiled, manifest.LimitsSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Summary.Participants != 3 || parsed.Summary.Messages != 3 {
		t.Fatalf("unexpected sequence summary: %#v", parsed.Summary)
	}
}

func TestCompileArchitectureWithFrontmatter(t *testing.T) {
	raw := []byte(`---
title: Mermaid Architecture
efp:
  camera:
    zoom: 1.2
  renderHints:
    presentationMode: true
---
architecture-beta
  group edge(server)[Edge Zone]
  group app(server)[App Zone]
  service nginx(server)[Nginx] in edge
  service api(server)[API Gateway] in app
  nginx:R --> L:api
`)
	compiled, err := CompileIfNeeded("isometric_architecture_v1", raw)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(compiled, &data); err != nil {
		t.Fatal(err)
	}
	parsed, err := visualschema.ValidateInput("isometric_architecture_v1", compiled, manifest.LimitsSpec{})
	if err != nil {
		t.Fatal(err)
	}
	camera := data["camera"].(map[string]any)
	renderHints := data["renderHints"].(map[string]any)
	if parsed.Summary.Zones != 2 || parsed.Summary.Entities != 2 || parsed.Summary.Links != 1 {
		t.Fatalf("unexpected architecture summary: %#v", parsed.Summary)
	}
	if camera["zoom"].(float64) != 1.2 || renderHints["presentationMode"].(bool) != true {
		t.Fatalf("frontmatter did not merge: camera=%#v renderHints=%#v", camera, renderHints)
	}
}

func TestArchitectureSemanticLayoutRoutingPipeline(t *testing.T) {
	raw := []byte(`architecture-beta
  group client(cloud)[Client Zone]
  group edge(cloud)[Edge Zone]
  group app(server)[Application Zone]
  group data(database)[Data Zone]
  service browser(internet)[Browser] in client
  service gateway(server)[API Gateway] in edge
  service service(server)[Order Service] in app
  service db(database)[Order Database] in data
  browser:R -->|API| L:gateway
  gateway:R -->|Service| L:service
  service:R -->|Data| L:db
`)
	diagram, ok := parse(raw)
	if !ok {
		t.Fatal("expected Mermaid architecture to parse")
	}
	graph := BuildArchitectureSemanticGraph(diagram)
	if len(graph.Nodes) != 4 || len(graph.Groups) != 4 || len(graph.Edges) != 3 {
		t.Fatalf("unexpected semantic graph: nodes=%d groups=%d edges=%d", len(graph.Nodes), len(graph.Groups), len(graph.Edges))
	}
	layout := ArchitectureLayoutEngine(graph)
	for _, pair := range [][2]string{{"browser", "gateway"}, {"gateway", "service"}, {"service", "db"}} {
		if layout.Ranks[pair[0]] >= layout.Ranks[pair[1]] {
			t.Fatalf("expected rank %s < %s, got %#v", pair[0], pair[1], layout.Ranks)
		}
		if layout.Entities[pair[0]].Position.X >= layout.Entities[pair[1]].Position.X {
			t.Fatalf("expected layout x %s < %s, got %#v -> %#v", pair[0], pair[1], layout.Entities[pair[0]].Position, layout.Entities[pair[1]].Position)
		}
	}
	routing := ArchitectureRoutingEngine(graph, layout)
	if len(routing.Links) != 3 {
		t.Fatalf("unexpected routing link count: %d", len(routing.Links))
	}
	if routing.Metrics.PortHintViolations != 0 || routing.Metrics.DirectionViolations != 0 {
		t.Fatalf("route validation should pass: %#v", routing.Metrics)
	}
	roles := map[string]string{}
	for _, link := range routing.Links {
		roles[link.Edge.Label] = link.Role
		if len(link.Route) < 2 {
			t.Fatalf("expected routed link to include route points: %#v", link)
		}
	}
	if roles["API"] != "primary" || roles["Service"] != "primary" || roles["Data"] != "secondary" {
		t.Fatalf("unexpected role inference: %#v", roles)
	}
}
