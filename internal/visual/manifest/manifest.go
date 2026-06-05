package manifest

type TemplateManifest struct {
	ID              string       `yaml:"id" json:"id"`
	Version         string       `yaml:"version" json:"version"`
	Category        string       `yaml:"category" json:"category"`
	Title           string       `yaml:"title" json:"title"`
	Description     string       `yaml:"description" json:"description"`
	InputSchema     string       `yaml:"input_schema" json:"input_schema"`
	InputSchemaKind string       `yaml:"input_schema_kind" json:"input_schema_kind"`
	Renderer        RendererSpec `yaml:"renderer" json:"renderer"`
	Layout          LayoutSpec   `yaml:"layout" json:"layout"`
	Effects         EffectsSpec  `yaml:"effects,omitempty" json:"effects,omitempty"`
	VisualDesign    VisualDesign `yaml:"visual_design,omitempty" json:"visual_design,omitempty"`
	Offline         OfflineSpec  `yaml:"offline" json:"offline"`
	Assets          []AssetSpec  `yaml:"assets" json:"assets"`
	Styles          []string     `yaml:"styles" json:"styles"`
	Scripts         []string     `yaml:"scripts" json:"scripts"`
	Interactions    []string     `yaml:"interactions" json:"interactions"`
	Limits          LimitsSpec   `yaml:"limits" json:"limits"`
	Tags            []string     `yaml:"tags" json:"tags"`
}

type RendererSpec struct {
	Contract string `yaml:"contract" json:"contract"`
}

type LayoutSpec struct {
	Preset  string `yaml:"preset" json:"preset"`
	GroupBy string `yaml:"group_by,omitempty" json:"group_by,omitempty"`
}

type EffectsSpec struct {
	Engine      string   `yaml:"engine,omitempty" json:"engine,omitempty"`
	Scene       string   `yaml:"scene,omitempty" json:"scene,omitempty"`
	Camera      string   `yaml:"camera,omitempty" json:"camera,omitempty"`
	Particles   string   `yaml:"particles,omitempty" json:"particles,omitempty"`
	Material    string   `yaml:"material,omitempty" json:"material,omitempty"`
	Motion      string   `yaml:"motion,omitempty" json:"motion,omitempty"`
	Interaction []string `yaml:"interaction,omitempty" json:"interaction,omitempty"`
	Postprocess []string `yaml:"postprocess,omitempty" json:"postprocess,omitempty"`
}

type VisualDesign struct {
	InitialView          string   `yaml:"initial_view,omitempty" json:"initial_view,omitempty"`
	MaxInitialNodes      int      `yaml:"max_initial_nodes,omitempty" json:"max_initial_nodes,omitempty"`
	MaxInitialEdges      int      `yaml:"max_initial_edges,omitempty" json:"max_initial_edges,omitempty"`
	DefaultCollapseDepth int      `yaml:"default_collapse_depth,omitempty" json:"default_collapse_depth,omitempty"`
	GroupBy              []string `yaml:"group_by,omitempty" json:"group_by,omitempty"`
	Supports             []string `yaml:"supports,omitempty" json:"supports,omitempty"`
	AgentGuidance        []string `yaml:"agent_guidance,omitempty" json:"agent_guidance,omitempty"`
}

type OfflineSpec struct {
	Required      bool   `yaml:"required" json:"required"`
	ForbidNetwork bool   `yaml:"forbid_network" json:"forbid_network"`
	DataMode      string `yaml:"data_mode" json:"data_mode"`
}

type AssetSpec struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

type LimitsSpec struct {
	MaxNodes  int `yaml:"max_nodes" json:"max_nodes"`
	MaxEdges  int `yaml:"max_edges" json:"max_edges"`
	MaxEvents int `yaml:"max_events" json:"max_events"`
	MaxItems  int `yaml:"max_items" json:"max_items"`
}

type OutputManifest struct {
	Schema       string         `json:"schema"`
	Template     OutputTemplate `json:"template"`
	Renderer     RendererSpec   `json:"renderer"`
	Title        string         `json:"title"`
	CreatedAt    string         `json:"created_at"`
	Offline      bool           `json:"offline"`
	Entrypoint   string         `json:"entrypoint"`
	Layout       LayoutSpec     `json:"layout"`
	Effects      EffectsSpec    `json:"effects,omitempty"`
	VisualDesign VisualDesign   `json:"visual_design,omitempty"`
	Interactions []string       `json:"interactions"`
	Assets       OutputAssets   `json:"assets,omitempty"`
}

type OutputTemplate struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type OutputAssets struct {
	Icons         []string           `json:"icons,omitempty"`
	Models        []string           `json:"models,omitempty"`
	Attributions  []AssetAttribution `json:"attributions,omitempty"`
	AssetRegistry map[string]any     `json:"asset_registry,omitempty"`
	MarkRegistry  map[string]any     `json:"mark_registry,omitempty"`
}

type AssetAttribution struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	License string `json:"license,omitempty"`
	Source  string `json:"source,omitempty"`
}
