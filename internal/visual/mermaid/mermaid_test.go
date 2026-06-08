package mermaid

import (
	"encoding/json"
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
		{"architecture", "architecture-beta\n  service api(server)[API]\n", "architecture.isometric_overview"},
		{"flowchart", "flowchart LR\n  A --> B\n", "relationship.dependency_graph"},
		{"sequence", "sequenceDiagram\n  A->>B: call\n", "uml.sequence_3d"},
		{"class", "classDiagram\n  A <|-- B\n", "uml.class_structure_2_5d"},
		{"pie", "pie title Pets\n  \"Dogs\" : 12\n", "matrix.capability"},
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

func TestOfficialMermaidKindsAreAccepted(t *testing.T) {
	cases := map[string]string{
		"architecture-beta":  "architecture.isometric_overview",
		"architecture":       "architecture.isometric_overview",
		"flowchart":          "relationship.dependency_graph",
		"graph":              "relationship.dependency_graph",
		"sequenceDiagram":    "uml.sequence_3d",
		"zenuml":             "uml.sequence_3d",
		"classDiagram":       "uml.class_structure_2_5d",
		"erDiagram":          "uml.class_structure_2_5d",
		"stateDiagram":       "uml.state_machine_3d",
		"gantt":              "temporal.incident_timeline",
		"timeline":           "temporal.incident_timeline",
		"journey":            "temporal.incident_timeline",
		"gitGraph":           "temporal.incident_timeline",
		"mindmap":            "hierarchy.repository_tree",
		"treeView":           "hierarchy.repository_tree",
		"sankey-beta":        "flow.data_flow",
		"xychart-beta":       "flow.data_flow",
		"block-beta":         "flow.data_flow",
		"packet-beta":        "flow.data_flow",
		"pie":                "matrix.capability",
		"quadrantChart":      "matrix.capability",
		"kanban":             "matrix.capability",
		"radar":              "matrix.capability",
		"treemap":            "matrix.capability",
		"requirementDiagram": "relationship.dependency_graph",
		"c4Context":          "architecture.isometric_overview",
		"eventModeling":      "relationship.dependency_graph",
		"venn":               "relationship.dependency_graph",
		"ishikawa":           "relationship.dependency_graph",
		"wardley":            "relationship.dependency_graph",
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
