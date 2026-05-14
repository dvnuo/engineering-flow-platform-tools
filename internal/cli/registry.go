package cli

type CommandRegistry struct {
	Product  string   `json:"product"`
	Commands []string `json:"commands"`
}

type SchemaDoc struct {
	Command string                 `json:"command"`
	Version int                    `json:"version"`
	Input   map[string]interface{} `json:"input"`
	Output  map[string]interface{} `json:"output"`
}
