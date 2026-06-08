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
