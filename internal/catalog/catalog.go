package catalog

import (
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/llm"
)

var jiraCommands = []string{
	"jira instance list", "jira instance get <name>", "jira instance add <name>", "jira instance update <name>", "jira instance remove <name>", "jira instance default [name]", "jira auth login", "jira auth logout", "jira auth test", "jira myself", "jira server-info", "jira resolve-url <url>", "jira commands", "jira schema <command>", "jira help llm",
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
	"confluence instance list", "confluence instance get <name>", "confluence instance add <name>", "confluence instance update <name>", "confluence instance remove <name>", "confluence instance default [name]", "confluence auth login", "confluence auth logout", "confluence auth test", "confluence myself", "confluence server-info", "confluence resolve-url <url>", "confluence commands", "confluence schema <command>", "confluence help llm",
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
		"flags":     item.Flags,
		"examples":  item.Examples,
		"required":  required(name),
	}
}

func meta(product, usage string) llm.CommandMeta {
	return llm.CommandMeta{
		Name:        usage,
		Usage:       usage,
		Product:     product,
		Risk:        risk(usage),
		Description: description(usage),
		Examples:    []string{usage + " --json"},
		Flags:       []string{},
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
	return "Run " + strings.Join(parts, " ")
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
	case "issue.create", "jira.issue.create":
		return []string{"project", "type", "summary"}
	case "page.create", "confluence.page.create":
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

func SortedUsages(product string) []string {
	out := CommandList(product)
	sort.Strings(out)
	return out
}
