package catalog

import (
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/llm"
)

type FlagSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type explicitMeta struct {
	Description string
	Flags       []string
	Required    []string
	Risk        string
	Example     string
}

var jiraCommands = []string{
	"jira instance list", "jira instance get <name>", "jira instance add <name>", "jira instance update <name>", "jira instance remove <name>", "jira instance default [name]", "jira auth login", "jira auth logout", "jira auth test", "jira myself", "jira server-info", "jira resolve-url <url>", "jira commands", "jira schema <command>", "jira help llm", "jira version",
	"jira issue get <issue-or-url>", "jira issue search", "jira issue create", "jira issue update <issue-or-url>", "jira issue edit <issue-or-url>", "jira issue delete <issue-or-url>", "jira issue assign <issue-or-url>", "jira issue transitions <issue-or-url>", "jira issue transition <issue-or-url>", "jira issue changelog <issue-or-url>", "jira issue fields <issue-or-url>", "jira issue createmeta", "jira issue editmeta <issue-or-url>", "jira issue watchers <issue-or-url>", "jira issue watch <issue-or-url>", "jira issue unwatch <issue-or-url>", "jira issue votes <issue-or-url>", "jira issue vote <issue-or-url>", "jira issue unvote <issue-or-url>", "jira issue notify <issue-or-url>",
	"jira issue comment list <issue-or-url>", "jira issue comment get <issue-or-url> <comment-id>", "jira issue comment add <issue-or-url>", "jira issue comment update <issue-or-url> <comment-id>", "jira issue comment delete <issue-or-url> <comment-id>",
	"jira issue attachment list <issue-or-url>", "jira issue attachment upload <issue-or-url> <file>", "jira attachment get <attachment-id>", "jira attachment download <attachment-id>", "jira attachment delete <attachment-id>", "jira attachment meta",
	"jira issue worklog list <issue-or-url>", "jira issue worklog get <issue-or-url> <worklog-id>", "jira issue worklog add <issue-or-url>", "jira issue worklog update <issue-or-url> <worklog-id>", "jira issue worklog delete <issue-or-url> <worklog-id>",
	"jira issue link list <issue-or-url>", "jira issue link create", "jira issue link delete <link-id>", "jira issue remote-link list <issue-or-url>", "jira issue remote-link add <issue-or-url>", "jira issue remote-link delete <issue-or-url> <link-id>", "jira issue property list <issue-or-url>", "jira issue property get <issue-or-url> <key>", "jira issue property set <issue-or-url> <key>", "jira issue property delete <issue-or-url> <key>",
	"jira project list", "jira project get <project-key>", "jira project statuses <project-key>", "jira project roles <project-key>", "jira project role get <project-key> <role-id-or-name>", "jira project components <project-key>", "jira component get <component-id>", "jira component create", "jira component update <component-id>", "jira component delete <component-id>", "jira project versions <project-key>", "jira version get <version-id>", "jira version create", "jira version update <version-id>", "jira version delete <version-id>",
	"jira user get", "jira user search", "jira user assignable", "jira group get <group-name>", "jira group members <group-name>", "jira group search",
	"jira field list", "jira issue-type list", "jira status list", "jira priority list", "jira resolution list", "jira workflow list", "jira workflow get <name>", "jira permissions myself", "jira settings get", "jira config get",
	"jira filter list", "jira filter get <filter-id>", "jira filter search", "jira filter create", "jira filter update <filter-id>", "jira filter delete <filter-id>", "jira dashboard list", "jira dashboard get <dashboard-id>",
	"jira api get <path>", "jira api post <path>", "jira api put <path>", "jira api delete <path>",
	"jira board list", "jira board get <board-id>", "jira sprint list <board-id>", "jira sprint get <sprint-id>", "jira sprint issues <sprint-id>", "jira backlog issues <board-id>",
}

var confluenceCommands = []string{
	"confluence instance list", "confluence instance get <name>", "confluence instance add <name>", "confluence instance update <name>", "confluence instance remove <name>", "confluence instance default [name]", "confluence auth login", "confluence auth logout", "confluence auth test", "confluence myself", "confluence server-info", "confluence resolve-url <url>", "confluence commands", "confluence schema <command>", "confluence help llm", "confluence version",
	"confluence search", "confluence cql", "confluence search content", "confluence search user",
	"confluence space list", "confluence space get <space-key>", "confluence space create", "confluence space update <space-key>", "confluence space delete <space-key>", "confluence space content <space-key>", "confluence space pages <space-key>", "confluence space blogs <space-key>", "confluence space labels <space-key>", "confluence space watchers <space-key>", "confluence space permission list <space-key>", "confluence space property list <space-key>", "confluence space property get <space-key> <key>", "confluence space property set <space-key> <key>", "confluence space property delete <space-key> <key>",
	"confluence page get", "confluence page get-by-title", "confluence page create", "confluence page update", "confluence page delete", "confluence page move", "confluence page children", "confluence page descendants", "confluence page ancestors", "confluence page body", "confluence page body-storage", "confluence page body-view", "confluence page version", "confluence page history", "confluence page restore", "confluence page export-html", "confluence page export-markdown",
	"confluence content get <content-id>", "confluence content list", "confluence content create", "confluence content update <content-id>", "confluence content delete <content-id>",
	"confluence blog list", "confluence blog get <blog-id-or-url>", "confluence blog create", "confluence blog update <blog-id-or-url>", "confluence blog delete <blog-id-or-url>",
	"confluence page attachment list", "confluence page attachment upload", "confluence page attachment update", "confluence attachment get <attachment-id>", "confluence attachment download <attachment-id>", "confluence attachment delete <attachment-id>",
	"confluence page comment list", "confluence page comment add", "confluence comment get <comment-id>", "confluence comment update <comment-id>", "confluence comment delete <comment-id>",
	"confluence page label list", "confluence page label add", "confluence page label delete", "confluence label list", "confluence page property list", "confluence page property get", "confluence page property set", "confluence page property delete",
	"confluence page restriction list", "confluence page restriction add", "confluence page restriction delete", "confluence page watcher list", "confluence page watch", "confluence page unwatch",
	"confluence user get", "confluence user search", "confluence group list", "confluence group get <group-name>", "confluence group members <group-name>",
	"confluence longtask list", "confluence longtask get <task-id>", "confluence webhook list", "confluence webhook get <webhook-id>", "confluence webhook create", "confluence webhook delete <webhook-id>", "confluence api get <path>", "confluence api post <path>", "confluence api put <path>", "confluence api delete <path>",
}

func Commands(product string) []llm.CommandMeta {
	var src []string
	switch product {
	case "jira":
		src = jiraCommands
	case "confluence":
		src = confluenceCommands
	default:
		return nil
	}
	out := make([]llm.CommandMeta, 0, len(src))
	for _, usage := range src {
		out = append(out, meta(product, usage))
	}
	return out
}

func CommandList(product string) []string {
	items := Commands(product)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Usage)
	}
	return out
}

func Find(product, name string) (llm.CommandMeta, bool) {
	key := strings.TrimSpace(name)
	for _, item := range Commands(product) {
		if item.Name == key || item.Usage == key || dotted(item.Usage) == key {
			return item, true
		}
	}
	return llm.CommandMeta{}, false
}

func Schema(product, name string) map[string]any {
	item, ok := Find(product, name)
	if !ok {
		item = meta(product, product+" "+strings.ReplaceAll(name, ".", " "))
	}
	return map[string]any{
		"command":   name,
		"usage":     item.Usage,
		"risk":      item.Risk,
		"arguments": arguments(item.Usage),
		"flags":     flagSpecs(item.Name, item.Flags, item.Required),
		"examples":  item.Examples,
		"required":  item.Required,
	}
}

func meta(product, usage string) llm.CommandMeta {
	name := dotted(usage)
	ex := explicit[name]
	flags := ex.Flags
	if len(flags) == 0 {
		flags = []string{"instance", "config", "json", "format", "verbose", "dry-run", "yes"}
	}
	desc := ex.Description
	if desc == "" {
		desc = description(usage)
	}
	r := risk(usage)
	if ex.Risk != "" {
		r = ex.Risk
	}
	example := ex.Example
	if example == "" {
		example = usage + " --json"
	}
	if product == "confluence" && strings.HasPrefix(example, "jira ") {
		example = strings.Replace(example, "jira ", "confluence ", 1)
	}
	if product == "confluence" && strings.HasPrefix(name, "api.") {
		example = strings.ReplaceAll(example, "/rest/api/2", "/rest/api")
	}
	req := ex.Required
	if len(req) == 0 {
		req = required(name)
	}
	return llm.CommandMeta{
		Name:        name,
		Usage:       usage,
		Product:     product,
		Risk:        r,
		Description: desc,
		Examples:    []string{example},
		Flags:       flags,
		Required:    req,
	}
}

func risk(usage string) string {
	switch {
	case strings.Contains(usage, " delete ") || strings.Contains(usage, " remove ") || strings.Contains(usage, " logout"):
		return "delete"
	case strings.Contains(usage, " create") || strings.Contains(usage, " update") || strings.Contains(usage, " add") || strings.Contains(usage, " set") || strings.Contains(usage, " upload") || strings.Contains(usage, " move") || strings.Contains(usage, " restore") || strings.Contains(usage, " watch") || strings.Contains(usage, " unwatch") || strings.Contains(usage, " assign") || strings.Contains(usage, " transition") || strings.Contains(usage, " vote") || strings.Contains(usage, " login") || strings.Contains(usage, " default"):
		return "write"
	case strings.Contains(usage, " permission") || strings.Contains(usage, " config") || strings.Contains(usage, " settings"):
		return "admin"
	default:
		return "read"
	}
}

func description(usage string) string {
	parts := strings.Fields(usage)
	if len(parts) <= 1 {
		return usage
	}
	return "Execute " + strings.Join(parts[1:], " ") + " for " + parts[0] + "."
}

func dotted(usage string) string {
	parts := strings.Fields(usage)
	if len(parts) <= 1 {
		return usage
	}
	var clean []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "[") {
			continue
		}
		clean = append(clean, p)
	}
	return strings.Join(clean, ".")
}

func arguments(usage string) []string {
	var out []string
	for _, p := range strings.Fields(usage) {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "[") {
			out = append(out, p)
		}
	}
	return out
}

func required(name string) []string {
	switch name {
	case "issue.create":
		return []string{"project", "type", "summary"}
	case "page.create":
		return []string{"space", "title", "body"}
	case "filter.create":
		return []string{"name", "jql"}
	case "component.create", "version.create":
		return []string{"project", "name"}
	case "issue.update":
		return []string{"summary|description|field"}
	case "issue.comment.add", "issue.comment.update":
		return []string{"body"}
	case "issue.worklog.add":
		return []string{"time-spent"}
	case "issue.worklog.update":
		return []string{"time-spent|started|comment"}
	}
	return []string{}
}

func flagSpecs(command string, flags, required []string) []FlagSpec {
	req := map[string]bool{}
	for _, r := range required {
		for _, part := range strings.Split(r, "|") {
			req[strings.TrimSpace(part)] = true
		}
	}
	out := make([]FlagSpec, 0, len(flags))
	for _, f := range flags {
		out = append(out, FlagSpec{Name: f, Type: flagType(f), Required: req[f], Description: flagDescription(command, f)})
	}
	return out
}

func flagType(name string) string {
	switch name {
	case "json", "verbose", "dry-run", "yes", "body-stdin", "minor-edit":
		return "bool"
	case "field", "fields", "query":
		return "string[]"
	default:
		return "string"
	}
}

func flagDescription(command, name string) string {
	switch name {
	case "instance":
		return "Configured instance name."
	case "config":
		return "Path to config file."
	case "json":
		return "Print JSON envelope."
	case "format":
		return "Output format: table, json, or yaml."
	case "dry-run":
		return "Preview write request without sending it."
	case "yes":
		return "Confirm destructive operations."
	case "project":
		return "Jira project key."
	case "type":
		return "Issue type name."
	case "summary":
		return "Issue summary."
	case "space":
		return "Confluence space key."
	case "title":
		return "Page title."
	case "body", "body-file", "body-stdin":
		return "Request body source."
	case "cql":
		return "Confluence CQL query."
	case "transition-id":
		return "Jira transition id."
	case "to":
		return "Jira transition name."
	default:
		return "Command option."
	}
}

var explicit = map[string]explicitMeta{
	"auth.test":                 {Description: "Verify configured credentials against the current user endpoint.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira auth test --json"},
	"server-info":               {Description: "Read server metadata from the selected instance.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira server-info --json"},
	"issue.get":                 {Description: "Fetch a Jira issue by key or full issue URL.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url"}, Risk: "read", Example: "jira issue get EFP-123 --json"},
	"issue.search":              {Description: "Search Jira issues with JQL.", Flags: []string{"jql", "limit", "start", "fields", "instance", "config", "json", "format", "verbose"}, Required: []string{"jql"}, Risk: "read", Example: "jira issue search --jql \"project = EFP\" --json"},
	"issue.create":              {Description: "Create a Jira issue.", Flags: []string{"project", "type", "summary", "description", "field", "json-body", "json-body-file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"project", "type", "summary"}, Risk: "write", Example: "jira issue create --project EFP --type Task --summary Test --json"},
	"issue.update":              {Description: "Update fields on a Jira issue.", Flags: []string{"summary", "description", "field", "json-body", "json-body-file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"summary|description|field|json-body|json-body-file"}, Risk: "write", Example: "jira issue update EFP-123 --summary Done --json"},
	"issue.delete":              {Description: "Delete a Jira issue after explicit confirmation.", Flags: []string{"yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "yes"}, Risk: "delete", Example: "jira issue delete EFP-123 --yes --json"},
	"issue.transition":          {Description: "Transition a Jira issue by transition id or transition name.", Flags: []string{"transition-id", "to", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"transition-id|to"}, Risk: "write", Example: "jira issue transition EFP-123 --to Done --json"},
	"issue.comment.add":         {Description: "Add a comment to a Jira issue.", Flags: []string{"body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"body|body-file|body-stdin"}, Risk: "write", Example: "jira issue comment add EFP-123 --body ok --json"},
	"issue.attachment.download": {Description: "Download or inspect a Jira issue attachment.", Flags: []string{"output", "instance", "config", "json", "format", "verbose", "dry-run"}, Risk: "read", Example: "jira attachment download 10000 --json"},
	"project.list":              {Description: "List Jira projects.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira project list --json"},
	"project.get":               {Description: "Fetch a Jira project by key.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"project-key"}, Risk: "read", Example: "jira project get EFP --json"},
	"field.list":                {Description: "List Jira fields.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira field list --json"},
	"api.get":                   {Description: "Call a raw REST GET path on the selected instance.", Flags: []string{"query", "instance", "config", "json", "format", "verbose"}, Required: []string{"path"}, Risk: "read", Example: "jira api get /rest/api/2/myself --json"},
	"api.post":                  {Description: "Call a raw REST POST path on the selected instance.", Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira api post /rest/api/2/issue --body '{}' --json"},
	"api.put":                   {Description: "Call a raw REST PUT path on the selected instance.", Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira api put /rest/api/2/issue/EFP-123 --body '{}' --json"},
	"api.delete":                {Description: "Call a raw REST DELETE path after explicit confirmation.", Flags: []string{"query", "yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"path", "yes"}, Risk: "delete", Example: "jira api delete /rest/api/2/issue/EFP-123 --yes --json"},
	"search":                    {Description: "Search Confluence content with CQL.", Flags: []string{"cql", "limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"cql"}, Risk: "read", Example: "confluence search --cql \"space = ENG\" --json"},
	"cql":                       {Description: "Search Confluence content with a CQL query alias.", Flags: []string{"query", "limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"query"}, Risk: "read", Example: "confluence cql --query \"space = ENG\" --json"},
	"page.get":                  {Description: "Fetch a Confluence page by id or full URL.", Flags: []string{"id", "url", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page get --id 123 --json"},
	"page.get-by-title":         {Description: "Fetch a Confluence page by space and title.", Flags: []string{"space", "title", "expand", "limit", "instance", "config", "json", "format", "verbose"}, Required: []string{"space", "title"}, Risk: "read", Example: "confluence page get-by-title --space ENG --title Home --json"},
	"page.create":               {Description: "Create a Confluence page.", Flags: []string{"space", "title", "parent-id", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"space", "title", "body|body-file|body-stdin"}, Risk: "write", Example: "confluence page create --space ENG --title Home --body '<p>Hello</p>' --json"},
	"page.update":               {Description: "Update a Confluence page.", Flags: []string{"id", "url", "title", "version", "minor-edit", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url"}, Risk: "write", Example: "confluence page update --id 123 --title Home --json"},
	"page.delete":               {Description: "Delete a Confluence page after explicit confirmation.", Flags: []string{"id", "url", "yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url", "yes"}, Risk: "delete", Example: "confluence page delete --id 123 --yes --json"},
	"page.export-markdown":      {Description: "Export a Confluence page body as Markdown.", Flags: []string{"id", "url", "output", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page export-markdown --id 123 --json"},
	"page.attachment.download":  {Description: "Download or inspect a Confluence page attachment.", Flags: []string{"output", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "confluence attachment download 10000 --json"},
	"page.comment.add":          {Description: "Add a comment to a Confluence page.", Flags: []string{"id", "url", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url", "body|body-file|body-stdin"}, Risk: "write", Example: "confluence page comment add --id 123 --body ok --json"},
	"page.label.add":            {Description: "Add labels to a Confluence page.", Flags: []string{"id", "url", "label", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url", "label"}, Risk: "write", Example: "confluence page label add --id 123 --label runbook --json"},
}

func SortedUsages(product string) []string {
	out := CommandList(product)
	sort.Strings(out)
	return out
}
