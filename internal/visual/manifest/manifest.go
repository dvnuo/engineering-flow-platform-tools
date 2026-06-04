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
	Interactions []string       `json:"interactions"`
}

type OutputTemplate struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
