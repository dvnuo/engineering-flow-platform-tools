package catalog

// appdCommands is the canonical command list of the appd binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var appdCommands = []string{
	"appd instance list", "appd instance get <name>", "appd instance add <name>", "appd instance update <name>", "appd instance remove <name>", "appd instance default [name]",
	"appd auth login", "appd auth logout", "appd auth test",
	"appd commands", "appd schema <command>", "appd help llm", "appd version",
	"appd app list", "appd app get <app>",
	"appd tier list", "appd tier get <tier>",
	"appd node list", "appd node get <node>",
	"appd bt list",
	"appd backend list",
	"appd metric browse", "appd metric get", "appd metric preset",
	"appd snapshot list", "appd snapshot get",
	"appd violation list",
	"appd event list",
	"appd api get <path>",
}

// appdExplicit carries per-command metadata (description, risk, example,
// flags) for the appd binary. Every Controller read gets Risk "read"; only
// the config-editing instance/auth commands are write or delete.
func appdExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	read := append([]string{}, append(common, "dry-run")...)
	del := append([]string{"yes"}, common...)
	timeFlags := []string{"duration-mins", "start-time", "end-time", "before-time", "after-time"}
	withApp := func(flags ...string) []string {
		return append(append(append([]string{"app"}, flags...), timeFlags...), read...)
	}
	items := map[string]explicitMeta{
		"instance.list":    {Description: "List configured AppDynamics Controller instances with credentials redacted.", Flags: common, Risk: "read", Example: "appd instance list --json"},
		"instance.get":     {Description: "Show one configured AppDynamics Controller instance with credentials redacted.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "appd instance get prod --json"},
		"instance.add":     {Description: "Add an AppDynamics Controller instance (base URL, account, and API client or basic credentials) to the shared EFP config.", Flags: []string{"base-url", "account", "rest-path", "username", "auth-type", "password-stdin", "api-key-stdin", "default", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url", "account", "username+api-key-stdin|username+password-stdin"}, Risk: "write", Example: "appd instance add prod --base-url https://appd.example.test:8090 --account customer1 --auth-type api_client --username efp-reader --api-key-stdin --default --json"},
		"instance.update":  {Description: "Update the base URL, account, or REST path of a configured AppDynamics instance.", Flags: []string{"base-url", "account", "rest-path", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url|account|rest-path"}, Risk: "write", Example: "appd instance update prod --account customer1 --json"},
		"instance.remove":  {Description: "Remove an AppDynamics instance from the EFP config after confirmation.", Flags: del, Required: []string{"name", "yes"}, Risk: "delete", Example: "appd instance remove prod --yes --json"},
		"instance.default": {Description: "Read or set the default AppDynamics instance.", Flags: common, Risk: "write", Example: "appd instance default prod --json"},
		"auth.login":       {Description: "Store AppDynamics API client (OAuth client credentials) or user@account basic credentials for the selected instance.", Flags: []string{"username", "auth-type", "password-stdin", "api-key-stdin", "instance", "config", "json", "format", "verbose"}, Required: []string{"username+api-key-stdin|username+password-stdin"}, Risk: "write", Example: "appd auth login --instance prod --auth-type api_client --username efp-reader --api-key-stdin --json"},
		"auth.logout":      {Description: "Clear AppDynamics credentials for the selected instance after confirmation.", Flags: del, Required: []string{"yes"}, Risk: "delete", Example: "appd auth logout --instance prod --yes --json"},
		"auth.test":        {Description: "Verify AppDynamics credentials by exchanging the API client for a token and listing applications.", Flags: read, Risk: "read", Example: "appd auth test --json"},
		"commands":         {Description: "List appd commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "appd commands --json"},
		"schema":           {Description: "Describe one appd command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "appd schema snapshot.list --json"},
		"help.llm":         {Description: "Return concise agent guidance for appd: triage order, time ranges, presets, and limits.", Flags: common, Risk: "read", Example: "appd help llm --json"},
		"version":          {Description: "Print appd version metadata.", Flags: common, Risk: "read", Example: "appd version --json"},

		"app.list":     {Description: "List the AppDynamics applications (id, name) visible to the credentials.", Flags: read, Risk: "read", Example: "appd app list --json"},
		"app.get":      {Description: "Fetch one AppDynamics application by name or numeric id.", Flags: read, Required: []string{"app"}, Risk: "read", Example: "appd app get ecommerce --json"},
		"tier.list":    {Description: "List the tiers of an AppDynamics application.", Flags: append([]string{"app"}, read...), Required: []string{"app"}, Risk: "read", Example: "appd tier list --app ecommerce --json"},
		"tier.get":     {Description: "Fetch one tier of an AppDynamics application by name or id.", Flags: append([]string{"app"}, read...), Required: []string{"app", "tier"}, Risk: "read", Example: "appd tier get --app ecommerce web --json"},
		"node.list":    {Description: "List the nodes of an AppDynamics application, optionally only those of one tier.", Flags: append([]string{"app", "tier"}, read...), Required: []string{"app"}, Risk: "read", Example: "appd node list --app ecommerce --tier web --json"},
		"node.get":     {Description: "Fetch one node of an AppDynamics application by name or id.", Flags: append([]string{"app"}, read...), Required: []string{"app", "node"}, Risk: "read", Example: "appd node get --app ecommerce web-node-1 --json"},
		"bt.list":      {Description: "List the business transactions of an AppDynamics application, optionally filtered client-side to one tier.", Flags: append([]string{"app", "tier"}, read...), Required: []string{"app"}, Risk: "read", Example: "appd bt list --app ecommerce --tier web --json"},
		"backend.list": {Description: "List the backends (databases, caches, remote services) detected for an AppDynamics application.", Flags: append([]string{"app"}, read...), Required: []string{"app"}, Risk: "read", Example: "appd backend list --app ecommerce --json"},

		"metric.browse": {Description: "Browse the AppDynamics metric hierarchy of an application one level at a time.", Flags: append([]string{"app", "path"}, read...), Required: []string{"app"}, Risk: "read", Example: "appd metric browse --app ecommerce --path \"Overall Application Performance\" --json"},
		"metric.get":    {Description: "Fetch metric values for one AppDynamics metric path over an explicit time range (metric-data-v2 by default).", Flags: withApp("path", "rollup", "api"), Required: []string{"app", "path"}, Risk: "read", Example: "appd metric get --app ecommerce --path \"Overall Application Performance|Average Response Time (ms)\" --duration-mins 60 --json"},
		"metric.preset": {Description: "Fetch a common AppDynamics metric (BT response time, calls, errors, tier CPU, node heap) by expanding a preset into its metric path.", Flags: withApp("preset", "tier", "bt", "node", "rollup", "api"), Required: []string{"app", "preset"}, Risk: "read", Example: "appd metric preset --app ecommerce --preset bt-response-time --tier web --bt /checkout --duration-mins 60 --json"},

		"snapshot.list": {Description: "List AppDynamics transaction snapshots in a time range with BT, tier, node, user-experience, and error filters; output is trimmed to summary fields and capped.", Flags: withApp("bt-ids", "tier-ids", "node-ids", "user-experience", "errors-only", "first-in-chain", "need-exit-calls", "need-props", "max-results"), Required: []string{"app"}, Risk: "read", Example: "appd snapshot list --app ecommerce --errors-only --duration-mins 60 --max-results 50 --json"},
		"snapshot.get":  {Description: "Fetch one AppDynamics transaction snapshot by request GUID with exit calls and properties (the call graph is not in the public REST API).", Flags: withApp("guid"), Required: []string{"app", "guid"}, Risk: "read", Example: "appd snapshot get --app ecommerce --guid 4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f --json"},

		"violation.list": {Description: "List the health-rule violations of an AppDynamics application in a time range.", Flags: withApp(), Required: []string{"app"}, Risk: "read", Example: "appd violation list --app ecommerce --duration-mins 120 --json"},
		"event.list":     {Description: "List AppDynamics events (errors, deployments, diagnostic sessions, custom) of an application by type and severity in a time range.", Flags: withApp("event-types", "severities"), Required: []string{"app", "event-types"}, Risk: "read", Example: "appd event list --app ecommerce --event-types APPLICATION_DEPLOYMENT,APPLICATION_ERROR --duration-mins 1440 --json"},
		"api.get":        {Description: "Call a raw read-only AppDynamics Controller REST path (/controller/rest/...) on the selected instance with output=JSON.", Flags: append([]string{"query"}, read...), Required: []string{"path"}, Risk: "read", Example: "appd api get /controller/rest/applications/ecommerce/tiers --json"},
	}
	item, ok := items[name]
	return item, ok
}
