package catalog

// nexusCommands is the canonical command list of the nexus binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var nexusCommands = []string{
	"nexus instance list", "nexus instance get <name>", "nexus instance add <name>", "nexus instance update <name>", "nexus instance remove <name>", "nexus instance default [name]",
	"nexus auth login", "nexus auth logout", "nexus auth test",
	"nexus repo list", "nexus repo get <name>",
	"nexus component search", "nexus component list", "nexus component get <id>",
	"nexus asset search", "nexus asset list", "nexus asset get <id>", "nexus asset download <id>",
	"nexus api get <path>",
	"nexus commands", "nexus schema <command>", "nexus help llm", "nexus version",
}

// nexusExplicit carries per-command metadata (description, risk, example,
// flags) for the nexus binary. Every REST-backed command is read-only against
// the repository manager (Risk "read"); only the local config writers carry
// "write" or "delete".
func nexusExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	del := append([]string{"yes"}, common...)
	paging := []string{"continuation", "limit", "all", "max-pages"}
	search := append(append([]string{"repository", "repo-format", "group", "name", "version", "keyword", "sort", "direction", "maven-group-id", "maven-artifact-id", "maven-base-version", "maven-extension", "maven-classifier", "docker-image-name", "docker-image-tag"}, paging...), common...)
	list := append(append([]string{"repository"}, paging...), common...)
	items := map[string]explicitMeta{
		"instance.list":    {Description: "List configured Nexus instances with credentials redacted.", Flags: common, Risk: "read", Example: "nexus instance list --json"},
		"instance.get":     {Description: "Show one configured Nexus instance with credentials redacted.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "nexus instance get repo --json"},
		"instance.add":     {Description: "Add a Nexus instance to the shared EFP YAML config; omit the auth flags for anonymous read access.", Flags: []string{"base-url", "rest-path", "username", "auth-type", "password-stdin", "api-key-stdin", "token-stdin", "default", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url"}, Risk: "write", Example: "nexus instance add repo --base-url https://nexus.example.test --username ci-reader --password-stdin --default --json"},
		"instance.update":  {Description: "Update the base URL or REST path of a Nexus instance in the EFP config.", Flags: []string{"base-url", "rest-path", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url|rest-path"}, Risk: "write", Example: "nexus instance update repo --base-url https://nexus.example.test --json"},
		"instance.remove":  {Description: "Remove a Nexus instance from the EFP config after confirmation.", Flags: del, Required: []string{"name", "yes"}, Risk: "delete", Example: "nexus instance remove repo --yes --json"},
		"instance.default": {Description: "Read or set the default Nexus instance.", Flags: common, Risk: "write", Example: "nexus instance default repo --json"},
		"auth.login":       {Description: "Store Nexus credentials (password or user token) for the selected instance.", Flags: []string{"username", "auth-type", "password-stdin", "api-key-stdin", "token-stdin", "instance", "config", "json", "format", "verbose"}, Required: []string{"username+password-stdin|username+api-key-stdin|token-stdin"}, Risk: "write", Example: "nexus auth login --instance repo --username ci-reader --password-stdin --json"},
		"auth.logout":      {Description: "Clear stored Nexus credentials for the selected instance after confirmation; the instance then reads anonymously.", Flags: del, Required: []string{"yes"}, Risk: "delete", Example: "nexus auth logout --instance repo --yes --json"},
		"auth.test":        {Description: "Verify access to the selected Nexus instance by listing repositories and report whether credentials were used.", Flags: common, Risk: "read", Example: "nexus auth test --json"},
		"commands":         {Description: "List nexus commands with JSON-friendly metadata.", Flags: common, Risk: "read", Example: "nexus commands --json"},
		"schema":           {Description: "Describe one nexus command schema.", Flags: common, Required: []string{"command"}, Risk: "read", Example: "nexus schema component.search --json"},
		"help.llm":         {Description: "Return concise agent guidance for nexus.", Flags: common, Risk: "read", Example: "nexus help llm --json"},
		"version":          {Description: "Print nexus version metadata.", Flags: common, Risk: "read", Example: "nexus version --json"},

		"repo.list": {Description: "List Nexus repositories with their format, type, and URL.", Flags: common, Risk: "read", Example: "nexus repo list --json"},
		"repo.get":  {Description: "Show one Nexus repository by name.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "nexus repo get maven-releases --json"},

		"component.search": {Description: "Search Nexus components by repository, format, group, name, version, keyword, or Maven and Docker coordinates; results are paged by continuation token.", Flags: search, Risk: "read", Example: "nexus component search --repository maven-releases --maven-group-id com.example --maven-artifact-id app --version 1.4.2 --json"},
		"component.list":   {Description: "List the components of one Nexus repository page by page.", Flags: list, Required: []string{"repository"}, Risk: "read", Example: "nexus component list --repository npm-hosted --limit 50 --json"},
		"component.get":    {Description: "Show one Nexus component and its assets by component id.", Flags: common, Required: []string{"id"}, Risk: "read", Example: "nexus component get bWF2ZW4tcmVsZWFzZXM6ZDQ4MTE3NTQxZGNiODllYzYxM2IyMzk3MzIwMWQ3YmE --json"},

		"asset.search":   {Description: "Search Nexus assets (files) with the same filters as component search; results are paged by continuation token.", Flags: search, Risk: "read", Example: "nexus asset search --repository docker-hosted --docker-image-name payments/api --docker-image-tag 2.3.0 --json"},
		"asset.list":     {Description: "List the assets of one Nexus repository page by page.", Flags: list, Required: []string{"repository"}, Risk: "read", Example: "nexus asset list --repository raw-hosted --limit 100 --json"},
		"asset.get":      {Description: "Show one Nexus asset (download URL, path, checksums, content type) by asset id.", Flags: common, Required: []string{"id"}, Risk: "read", Example: "nexus asset get bWF2ZW4tcmVsZWFzZXM6MTVkYWJmZDA1MTIzYWM1MTIzNGY1NjEyMzQ1Njc4OTA --json"},
		"asset.download": {Description: "Download one Nexus asset to a local file and return metadata (path, bytes, sha1, content type) instead of the content.", Flags: append([]string{"output"}, common...), Required: []string{"id"}, Risk: "read", Example: "nexus asset download bWF2ZW4tcmVsZWFzZXM6MTVkYWJmZDA1MTIzYWM1MTIzNGY1NjEyMzQ1Njc4OTA --output app-1.4.2.jar --json"},

		"api.get": {Description: "Call a raw read-only Nexus REST API GET path on the selected instance; relative paths resolve under the REST API v1 prefix.", Flags: append([]string{"query"}, common...), Required: []string{"path"}, Risk: "read", Example: "nexus api get /service/rest/v1/search --query repository=maven-releases --query name=app --json"},
	}
	item, ok := items[name]
	return item, ok
}
