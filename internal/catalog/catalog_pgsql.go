package catalog

// pgsqlCommands is the canonical command list of the pgsql binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var pgsqlCommands = []string{
	"pgsql commands", "pgsql schema <command>", "pgsql help llm", "pgsql version",
}

// pgsqlExplicit carries per-command metadata (description, risk, example,
// flags) for the pgsql binary. Every read-only command gets Risk "read".
func pgsqlExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	items := map[string]explicitMeta{
		"commands": {Description: "List pgsql commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "pgsql commands --json"},
		"schema":   {Description: "Describe one pgsql command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "pgsql schema version --json"},
		"help.llm": {Description: "Return concise agent guidance for pgsql.", Flags: common, Risk: "read", Example: "pgsql help llm --json"},
		"version":  {Description: "Print pgsql version metadata.", Flags: common, Risk: "read", Example: "pgsql version --json"},
	}
	item, ok := items[name]
	return item, ok
}
