package catalog

// nexusCommands is the canonical command list of the nexus binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var nexusCommands = []string{
	"nexus commands", "nexus schema <command>", "nexus help llm", "nexus version",
}

// nexusExplicit carries per-command metadata (description, risk, example,
// flags) for the nexus binary. Every read-only command gets Risk "read".
func nexusExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	items := map[string]explicitMeta{
		"commands": {Description: "List nexus commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "nexus commands --json"},
		"schema":   {Description: "Describe one nexus command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "nexus schema version --json"},
		"help.llm": {Description: "Return concise agent guidance for nexus.", Flags: common, Risk: "read", Example: "nexus help llm --json"},
		"version":  {Description: "Print nexus version metadata.", Flags: common, Risk: "read", Example: "nexus version --json"},
	}
	item, ok := items[name]
	return item, ok
}
