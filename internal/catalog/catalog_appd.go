package catalog

// appdCommands is the canonical command list of the appd binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var appdCommands = []string{
	"appd commands", "appd schema <command>", "appd help llm", "appd version",
}

// appdExplicit carries per-command metadata (description, risk, example,
// flags) for the appd binary. Every read-only command gets Risk "read".
func appdExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	items := map[string]explicitMeta{
		"commands": {Description: "List appd commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "appd commands --json"},
		"schema":   {Description: "Describe one appd command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "appd schema version --json"},
		"help.llm": {Description: "Return concise agent guidance for appd.", Flags: common, Risk: "read", Example: "appd help llm --json"},
		"version":  {Description: "Print appd version metadata.", Flags: common, Risk: "read", Example: "appd version --json"},
	}
	item, ok := items[name]
	return item, ok
}
