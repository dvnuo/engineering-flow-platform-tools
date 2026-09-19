package catalog

// splunkCommands is the canonical command list of the splunk binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var splunkCommands = []string{
	"splunk instance list", "splunk instance get <name>", "splunk instance add <name>", "splunk instance update <name>", "splunk instance remove <name>", "splunk instance default [name]",
	"splunk auth login", "splunk auth logout", "splunk auth test", "splunk commands", "splunk schema <command>", "splunk help llm", "splunk version",
	"splunk search run", "splunk search oneshot", "splunk search job get <sid>", "splunk search job results <sid>", "splunk search job cancel <sid>",
	"splunk saved list", "splunk saved run <name>", "splunk index list",
	"splunk api get <path>",
}

// splunkExplicit carries per-command metadata (description, risk, example,
// flags) for the splunk binary. Every command that only reads from Splunk
// gets Risk "read"; config writes are "write", config removals "delete", and
// search job cancel is the single service-affecting "write" that needs --yes.
func splunkExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	write := append(append([]string{}, common...), "dry-run")
	del := append([]string{"yes"}, common...)
	resultFlags := []string{"count", "offset", "fields", "max-field-chars", "output"}
	searchFlags := append(append(append([]string{"query", "earliest", "latest"}, resultFlags...), "timeout-sec", "poll-ms"), write...)
	oneshotFlags := append(append(append([]string{"query", "earliest", "latest"}, resultFlags...), "timeout-sec"), write...)
	savedRunFlags := append(append(append([]string{"earliest", "latest"}, resultFlags...), "timeout-sec", "poll-ms"), write...)
	instanceFields := []string{"base-url", "default-index", "default-earliest", "max-results"}
	authFlags := []string{"username", "auth-type", "password-stdin", "token-stdin"}
	items := map[string]explicitMeta{
		"instance.list":    {Description: "List configured Splunk instances with credentials redacted.", Flags: common, Risk: "read", Example: "splunk instance list --json"},
		"instance.get":     {Description: "Show one configured Splunk instance with credentials redacted.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "splunk instance get prod --json"},
		"instance.add":     {Description: "Add a Splunk instance (management REST URL, credentials, default index, default earliest time, result cap) to the shared EFP config.", Flags: append(append(append(append([]string{}, instanceFields...), authFlags...), "default"), common...), Required: []string{"name", "base-url", "token-stdin|username+password-stdin"}, Risk: "write", Example: "splunk instance add prod --base-url https://splunk-api.example.test:8089 --token-stdin --default --json"},
		"instance.update":  {Description: "Update a Splunk instance's base URL, default index, default earliest time, or result cap in the EFP config.", Flags: append(append([]string{}, instanceFields...), common...), Required: []string{"name", "base-url|default-index|default-earliest|max-results"}, Risk: "write", Example: "splunk instance update prod --default-index main --max-results 500 --json"},
		"instance.remove":  {Description: "Remove a Splunk instance from the EFP config after confirmation.", Flags: del, Required: []string{"name", "yes"}, Risk: "delete", Example: "splunk instance remove prod --yes --json"},
		"instance.default": {Description: "Read or set the default Splunk instance.", Flags: common, Risk: "write", Example: "splunk instance default prod --json"},
		"auth.login":       {Description: "Store a Splunk authentication token or session-login username/password for the selected instance.", Flags: append(append([]string{}, authFlags...), common...), Required: []string{"token-stdin|username+password-stdin"}, Risk: "write", Example: "splunk auth login --instance prod --token-stdin --json"},
		"auth.logout":      {Description: "Clear Splunk credentials for the selected instance after confirmation.", Flags: del, Required: []string{"yes"}, Risk: "delete", Example: "splunk auth logout --instance prod --yes --json"},
		"auth.test":        {Description: "Verify Splunk credentials against authentication/current-context and return the username and roles.", Flags: common, Risk: "read", Example: "splunk auth test --json"},
		"commands":         {Description: "List splunk commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "splunk commands --json"},
		"schema":           {Description: "Describe one splunk command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "splunk schema search.run --json"},
		"help.llm":         {Description: "Return concise agent guidance for splunk: time ranges, result caps, --output, and the read-only SPL guard.", Flags: common, Risk: "read", Example: "splunk help llm --json"},
		"version":          {Description: "Print splunk version metadata.", Flags: common, Risk: "read", Example: "splunk version --json"},

		"search.run":         {Description: "Create a bounded search job, poll it until it finishes or --timeout-sec elapses (then cancel it), and return capped, truncation-aware results.", Flags: searchFlags, Required: []string{"query"}, Risk: "read", Example: "splunk search run --query \"index=main error | head 100\" --earliest -1h --count 100 --json"},
		"search.oneshot":     {Description: "Submit a oneshot search and return its results in a single call without polling; best for quick aggregate queries.", Flags: oneshotFlags, Required: []string{"query"}, Risk: "read", Example: "splunk search oneshot --query \"index=main | stats count by host\" --earliest -15m --json"},
		"search.job.get":     {Description: "Read the status of an existing search job: dispatch state, progress, event, result, and scan counts, and messages.", Flags: common, Required: []string{"sid"}, Risk: "read", Example: "splunk search job get 1700000000.123 --json"},
		"search.job.results": {Description: "Page the results of a finished search job (or a preview while it is running) with the same caps and truncation as search run.", Flags: append(append([]string{}, resultFlags...), common...), Required: []string{"sid"}, Risk: "read", Example: "splunk search job results 1700000000.123 --count 100 --offset 0 --json"},
		"search.job.cancel":  {Description: "Cancel a running search job after confirmation.", Flags: append([]string{"yes"}, write...), Required: []string{"sid", "yes"}, Risk: "write", Example: "splunk search job cancel 1700000000.123 --yes --json"},

		"saved.list": {Description: "List saved searches with their SPL, schedule, and a blocked_command marker when the definition is not read-only.", Flags: append([]string{"count", "offset", "filter"}, common...), Risk: "read", Example: "splunk saved list --filter errors --count 100 --json"},
		"saved.run":  {Description: "Dispatch a saved search whose definition passes the read-only guard, wait for it, and return capped results.", Flags: savedRunFlags, Required: []string{"name"}, Risk: "read", Example: "splunk saved run \"Errors last hour\" --count 100 --timeout-sec 120 --json"},
		"index.list": {Description: "List indexes with total event counts and the earliest/latest event times.", Flags: append([]string{"count"}, common...), Risk: "read", Example: "splunk index list --count 200 --json"},
		"api.get":    {Description: "Call a raw read-only Splunk REST path under /services/ or /servicesNS/ with output_mode=json.", Flags: append(append([]string{"query"}, common...), "dry-run"), Required: []string{"path"}, Risk: "read", Example: "splunk api get /services/server/info --query count=1 --json"},
	}
	item, ok := items[name]
	return item, ok
}
