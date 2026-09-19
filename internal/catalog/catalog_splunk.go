package catalog

// splunkCommands is the canonical command list of the splunk binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var splunkCommands = []string{
	"splunk commands", "splunk schema <command>", "splunk help llm", "splunk version",
}

// splunkExplicit carries per-command metadata (description, risk, example,
// flags) for the splunk binary. Every read-only command gets Risk "read".
func splunkExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	items := map[string]explicitMeta{
		"commands": {Description: "List splunk commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "splunk commands --json"},
		"schema":   {Description: "Describe one splunk command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "splunk schema version --json"},
		"help.llm": {Description: "Return concise agent guidance for splunk.", Flags: common, Risk: "read", Example: "splunk help llm --json"},
		"version":  {Description: "Print splunk version metadata.", Flags: common, Risk: "read", Example: "splunk version --json"},
	}
	item, ok := items[name]
	return item, ok
}
