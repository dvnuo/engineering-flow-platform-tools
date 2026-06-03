package catalog

import (
	"sort"
	"strings"

	"engineering-flow-platform-tools/internal/llm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FlagSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Repeatable  bool   `json:"repeatable,omitempty"`
	Shorthand   string `json:"shorthand,omitempty"`
	Source      string `json:"source,omitempty"`
}

type ArgumentSpec struct {
	Name        string `json:"name"`
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
	"jira issue get <issue-or-url>", "jira issue search", "jira issue create", "jira issue update <issue-or-url>", "jira issue edit <issue-or-url>", "jira issue delete <issue-or-url>", "jira issue assign <issue-or-url>", "jira issue transitions <issue-or-url>", "jira issue transition <issue-or-url>", "jira issue changelog <issue-or-url>", "jira issue fields <issue-or-url>", "jira issue createmeta", "jira issue editmeta <issue-or-url>", "jira issue map-csv", "jira issue bulk-create", "jira issue bulk-validate", "jira issue watchers <issue-or-url>", "jira issue watch <issue-or-url>", "jira issue unwatch <issue-or-url>", "jira issue votes <issue-or-url>", "jira issue vote <issue-or-url>", "jira issue unvote <issue-or-url>", "jira issue notify <issue-or-url>",
	"jira issue comment list <issue-or-url>", "jira issue comment get <issue-or-url> <comment-id>", "jira issue comment add <issue-or-url>", "jira issue comment update <issue-or-url> <comment-id>", "jira issue comment delete <issue-or-url> <comment-id>",
	"jira zephyr doctor", "jira zephyr resolve-url <jira-url>", "jira zephyr status list", "jira zephyr util test-issue-type",
	"jira zephyr summary",
	"jira zephyr test list", "jira zephyr test get <issue-or-url>", "jira zephyr test create",
	"jira zephyr cycle list", "jira zephyr cycle resolve", "jira zephyr cycle get <cycle-id>", "jira zephyr cycle create", "jira zephyr cycle update <cycle-id>", "jira zephyr cycle delete <cycle-id>",
	"jira zephyr execution list", "jira zephyr execution resolve", "jira zephyr execution get <execution-id>", "jira zephyr execution create", "jira zephyr execution update-status [execution-id]", "jira zephyr execution add-tests-to-cycle", "jira zephyr execution count", "jira zephyr execution delete <execution-id>", "jira zephyr execution bulk-update-status", "jira zephyr execution export",
	"jira zephyr zql search", "jira zephyr zql clauses", "jira zephyr zql autocomplete-json", "jira zephyr zql autocomplete", "jira zephyr step-result list", "jira zephyr step-result update-status <step-result-id>", "jira zephyr attachment list", "jira zephyr attachment get <attachment-id>", "jira zephyr attachment upload", "jira zephyr attachment delete <attachment-id>", "jira zephyr folder list", "jira zephyr folder create", "jira zephyr folder update <folder-id>", "jira zephyr folder delete <folder-id>", "jira zephyr teststep list", "jira zephyr teststep get", "jira zephyr teststep create", "jira zephyr teststep update", "jira zephyr teststep delete", "jira zephyr defect list", "jira zephyr defect add", "jira zephyr report coverage",
	"jira zephyr api catalog", "jira zephyr api describe <endpoint-id>", "jira zephyr api get <path>", "jira zephyr api post <path>", "jira zephyr api put <path>", "jira zephyr api delete <path>",
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

var jenkinsCommands = []string{
	"jenkins instance list", "jenkins instance get <name>", "jenkins instance add <name>", "jenkins instance update <name>", "jenkins instance remove <name>", "jenkins instance default [name]",
	"jenkins auth login", "jenkins auth logout", "jenkins auth test", "jenkins whoami", "jenkins server-info", "jenkins crumb get", "jenkins commands", "jenkins schema <command>", "jenkins help llm", "jenkins version",
	"jenkins job list", "jenkins job get <job>", "jenkins job config get <job>", "jenkins job config update <job>", "jenkins job create <job>", "jenkins job copy <source> <target>", "jenkins job delete <job>", "jenkins job enable <job>", "jenkins job disable <job>", "jenkins job build <job>", "jenkins job build-with-params <job>",
	"jenkins queue list", "jenkins queue get <queue-id>", "jenkins queue cancel <queue-id>",
	"jenkins build get <job> <build>", "jenkins build status <job> <build>", "jenkins build log <job> <build>", "jenkins build log-follow <job> <build>", "jenkins build stop <job> <build>", "jenkins build artifacts <job> <build>",
	"jenkins artifact download <job> <build> <path>",
	"jenkins pipeline runs <job>", "jenkins pipeline run <job> <run-id>", "jenkins pipeline stages <job> <run-id>", "jenkins pipeline node-log <job> <run-id> <node-id>", "jenkins pipeline artifacts <job> <run-id>",
	"jenkins view list", "jenkins view get <view>", "jenkins view create <view>", "jenkins view delete <view>", "jenkins view config get <view>", "jenkins view config update <view>",
	"jenkins node list", "jenkins node get <node>", "jenkins plugin list", "jenkins plugin get <plugin>",
	"jenkins system quiet-down", "jenkins system cancel-quiet-down", "jenkins system safe-restart",
	"jenkins api get <path>", "jenkins api post <path>", "jenkins api put <path>", "jenkins api delete <path>",
}

var browserCommands = []string{
	"browser probe",
	"browser commands",
	"browser schema <command>",
	"browser help llm",
	"browser version",
}

var inspectImageCommands = []string{
	"inspect-image inspect",
	"inspect-image auth login",
	"inspect-image auth status",
	"inspect-image auth test",
	"inspect-image auth logout",
	"inspect-image doctor",
	"inspect-image models",
	"inspect-image commands",
	"inspect-image schema <command>",
	"inspect-image help llm",
	"inspect-image version",
}

var logCommands = []string{
	"version",
	"commands",
	"schema <command>",
	"help llm",
	"analyze",
	"profile",
	"templates",
	"entries",
	"search",
	"window",
	"extract",
}

func Commands(product string) []llm.CommandMeta {
	var src []string
	switch product {
	case "jira":
		src = jiraCommands
	case "confluence":
		src = confluenceCommands
	case "jenkins":
		src = jenkinsCommands
	case "browser":
		src = browserCommands
	case "inspect-image":
		src = inspectImageCommands
	case "log":
		src = logCommands
	default:
		return nil
	}
	out := make([]llm.CommandMeta, 0, len(src))
	for _, usage := range src {
		if product == "log" && !strings.HasPrefix(usage, "log ") {
			usage = "log " + usage
		}
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

func CommandsFromCobra(product string, root *cobra.Command) []llm.CommandMeta {
	bindings := cobraBindings(product, root)
	out := make([]llm.CommandMeta, 0, len(bindings))
	for _, binding := range bindings {
		m := meta(product, binding.Usage)
		m.Flags = cobraFlagNames(binding.Command)
		m.Examples = examplesForCobra(m, binding.Command)
		if product == "log" {
			m.Usage = logCommandUsage(m.Name, m.Usage)
		}
		out = append(out, m)
	}
	return out
}

func SchemaFromCobra(product, name string, root *cobra.Command) (map[string]any, bool) {
	binding, ok := findCobraBinding(product, root, name)
	if !ok {
		return nil, false
	}
	item := meta(product, binding.Usage)
	item.Examples = examplesForCobra(item, binding.Command)
	args := argumentSpecs(binding.Usage)
	return map[string]any{
		"command":          item.Name,
		"usage":            binding.Usage,
		"description":      item.Description,
		"risk":             item.Risk,
		"arguments":        arguments(binding.Usage),
		"argument_details": args,
		"flags":            cobraFlagSpecs(item.Name, binding.Command, item.Required),
		"examples":         item.Examples,
		"required":         item.Required,
	}, true
}

type cobraBinding struct {
	Command *cobra.Command
	Usage   string
	Name    string
}

func cobraBindings(product string, root *cobra.Command) []cobraBinding {
	var out []cobraBinding
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, parent []string) {
		for _, child := range cmd.Commands() {
			if !visibleCommand(child) {
				continue
			}
			parts := append(append([]string{}, parent...), strings.Fields(child.Use)...)
			visibleChildren := 0
			for _, grand := range child.Commands() {
				if visibleCommand(grand) {
					visibleChildren++
				}
			}
			usage := strings.Join(parts, " ")
			if visibleChildren == 0 || child.RunE != nil || child.Run != nil {
				out = append(out, cobraBinding{Command: child, Usage: usage, Name: dotted(usage)})
			}
			walk(child, parts)
		}
	}
	walk(root, []string{product})
	return out
}

func visibleCommand(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Hidden {
		return false
	}
	if cmd.Name() == "completion" {
		return false
	}
	if cmd.Name() == "help" && strings.TrimSpace(cmd.Use) != "help llm" {
		return false
	}
	return true
}

func findCobraBinding(product string, root *cobra.Command, key string) (cobraBinding, bool) {
	key = strings.TrimSpace(key)
	for _, binding := range cobraBindings(product, root) {
		if key == binding.Name || key == binding.Usage || key == strings.TrimPrefix(binding.Usage, product+" ") {
			return binding, true
		}
	}
	return cobraBinding{}, false
}

func cobraFlagNames(cmd *cobra.Command) []string {
	var out []string
	visitCommandFlags(cmd, func(flag *pflag.Flag, source string) {
		out = append(out, flag.Name)
	})
	return out
}

func cobraFlagSpecs(command string, cmd *cobra.Command, required []string) []FlagSpec {
	req := requiredNames(required)
	var out []FlagSpec
	visitCommandFlags(cmd, func(flag *pflag.Flag, source string) {
		out = append(out, FlagSpec{
			Name:        flag.Name,
			Type:        normalizeFlagType(flag.Value.Type()),
			Required:    req[flag.Name],
			Description: flagDescriptionFor(command, flag),
			Default:     flag.DefValue,
			Repeatable:  repeatableFlag(flag.Value.Type()),
			Shorthand:   flag.Shorthand,
			Source:      source,
		})
	})
	return out
}

func visitCommandFlags(cmd *cobra.Command, fn func(*pflag.Flag, string)) {
	seen := map[string]bool{}
	visitSet := func(flags *pflag.FlagSet, source string) {
		if flags == nil {
			return
		}
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Name == "help" || seen[flag.Name] {
				return
			}
			seen[flag.Name] = true
			fn(flag, source)
		})
	}
	visitSet(cmd.NonInheritedFlags(), "command")
	visitSet(cmd.InheritedFlags(), "global")
}

func normalizeFlagType(t string) string {
	switch t {
	case "bool":
		return "bool"
	case "int", "int32", "int64":
		return "int"
	case "float32", "float64":
		return "float"
	case "stringArray", "stringSlice", "strings":
		return "string[]"
	default:
		return "string"
	}
}

func examplesForCobra(item llm.CommandMeta, cmd *cobra.Command) []string {
	examples := append([]string{}, item.Examples...)
	if len(examples) == 0 {
		examples = []string{item.Usage + " --json"}
	}
	flags := commandFlagMap(cmd)
	for i, example := range examples {
		examples[i] = completeRequiredExample(example, item.Required, flags)
	}
	return examples
}

func commandFlagMap(cmd *cobra.Command) map[string]*pflag.Flag {
	flags := map[string]*pflag.Flag{}
	visitCommandFlags(cmd, func(flag *pflag.Flag, source string) {
		flags[flag.Name] = flag
	})
	return flags
}

func completeRequiredExample(example string, required []string, flags map[string]*pflag.Flag) string {
	for _, expr := range required {
		if requiredExpressionSatisfied(example, expr, flags) {
			continue
		}
		for _, term := range strings.Split(expr, "|") {
			if term = strings.TrimSpace(term); term != "" {
				example = appendRequiredTerms(example, term, flags)
				break
			}
		}
	}
	return example
}

func requiredExpressionSatisfied(example, expr string, flags map[string]*pflag.Flag) bool {
	for _, alt := range strings.Split(expr, "|") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		ok := true
		for _, term := range strings.Split(alt, "+") {
			term = strings.TrimSpace(term)
			flag, isFlag := flags[term]
			if !isFlag {
				continue
			}
			if !exampleHasFlag(example, flag.Name) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func appendRequiredTerms(example, expr string, flags map[string]*pflag.Flag) string {
	for _, term := range strings.Split(expr, "+") {
		term = strings.TrimSpace(term)
		flag, ok := flags[term]
		if !ok || exampleHasFlag(example, flag.Name) {
			continue
		}
		example += " --" + flag.Name
		if normalizeFlagType(flag.Value.Type()) != "bool" {
			example += " " + sampleFlagValue(flag.Name)
		}
	}
	return example
}

func exampleHasFlag(example, name string) bool {
	return strings.Contains(example, "--"+name+" ") || strings.HasSuffix(example, "--"+name) || strings.Contains(example, "--"+name+"=")
}

func sampleFlagValue(name string) string {
	switch name {
	case "base-url":
		return "https://example.atlassian.test"
	case "rest-path":
		return "/rest/api"
	case "auth-type":
		return "basic_api_key"
	case "username":
		return "user@example.test"
	case "project":
		return "PROJ"
	case "project-id":
		return "10000"
	case "version-id":
		return "-1"
	case "cycle-id":
		return "20000"
	case "issue", "from":
		return "PROJ-123"
	case "to":
		return "alice"
	case "user":
		return "alice"
	case "subject":
		return "'Review requested'"
	case "body":
		return "'ok'"
	case "body-file":
		return "./body.txt"
	case "field":
		return "customfield_10000=value"
	case "json-body":
		return "'{\"fields\":{\"summary\":\"Test\"}}'"
	case "json-body-file":
		return "./body.json"
	case "time-spent":
		return "1h"
	case "started":
		return "2026-05-29T09:00:00.000+0000"
	case "value":
		return "'{\"ok\":true}'"
	case "value-file":
		return "./value.json"
	case "name":
		return "Example"
	case "jql":
		return "'project = PROJ'"
	case "query":
		return "alice"
	case "key":
		return "status"
	case "space":
		return "ENG"
	case "title":
		return "'Home'"
	case "id", "page-id", "parent-id", "attachment-id", "version":
		return "123"
	case "url":
		return "https://confluence.example.test/display/ENG/Home"
	case "operation":
		return "read"
	case "group":
		return "team"
	case "event":
		return "page_created"
	case "file":
		return "./note.txt"
	case "label":
		return "runbook"
	default:
		return "VALUE"
	}
}

func repeatableFlag(t string) bool {
	switch t {
	case "stringArray", "stringSlice", "strings":
		return true
	default:
		return false
	}
}

func requiredNames(required []string) map[string]bool {
	out := map[string]bool{}
	for _, expr := range required {
		for _, part := range strings.FieldsFunc(expr, func(r rune) bool {
			return r == '|' || r == '+' || r == ',' || r == ' '
		}) {
			part = strings.TrimSpace(part)
			if part != "" {
				out[part] = true
			}
		}
	}
	return out
}

func flagDescriptionFor(command string, flag *pflag.Flag) string {
	if strings.TrimSpace(flag.Usage) != "" {
		return strings.TrimSpace(flag.Usage)
	}
	return flagDescription(command, flag.Name)
}

func argumentSpecs(usage string) []ArgumentSpec {
	out := []ArgumentSpec{}
	for _, raw := range arguments(usage) {
		name := strings.Trim(raw, "<>[]")
		required := strings.HasPrefix(raw, "<")
		out = append(out, ArgumentSpec{Name: name, Required: required, Description: argumentDescription(name)})
	}
	return out
}

func meta(product, usage string) llm.CommandMeta {
	name := dotted(usage)
	ex := explicit[name]
	explicitFound := false
	if product == "inspect-image" {
		if local, ok := inspectImageExplicit(name); ok {
			ex = local
			explicitFound = true
		}
	}
	if product == "jenkins" {
		if local, ok := jenkinsExplicit(name); ok {
			ex = local
			explicitFound = true
		}
	}
	if product == "log" {
		if local, ok := logExplicit(name); ok {
			ex = local
			explicitFound = true
		}
	}
	r := risk(usage)
	if ex.Risk != "" {
		r = ex.Risk
	}
	flags := ex.Flags
	if len(flags) == 0 {
		flags = defaultFlags(product, r)
	}
	if product == "browser" && name != "probe" {
		flags = []string{"json", "format", "verbose"}
	}
	if product == "inspect-image" && name != "inspect" && len(ex.Flags) == 0 {
		flags = []string{"json", "format", "verbose", "config"}
	}
	desc := ex.Description
	if desc == "" {
		desc = description(usage)
	}
	example := ex.Example
	if example == "" {
		example = exampleFor(usage, r)
	}
	if product == "confluence" && strings.HasPrefix(example, "jira ") {
		example = strings.Replace(example, "jira ", "confluence ", 1)
	}
	if product == "confluence" && strings.HasPrefix(name, "api.") {
		example = strings.ReplaceAll(example, "/rest/api/2", "/rest/api")
	}
	if product == "browser" && strings.HasPrefix(example, "jira ") {
		example = strings.Replace(example, "jira ", "browser ", 1)
	}
	if product == "browser" && strings.HasPrefix(example, "confluence ") {
		example = strings.Replace(example, "confluence ", "browser ", 1)
	}
	if product == "inspect-image" {
		for _, p := range []string{"jira ", "confluence ", "browser "} {
			if strings.HasPrefix(example, p) {
				example = strings.Replace(example, p, "inspect-image ", 1)
			}
		}
	}
	req := ex.Required
	if len(req) == 0 {
		if (product == "inspect-image" || product == "jenkins" || product == "log") && explicitFound {
			req = []string{}
		} else {
			req = required(name)
		}
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

func logExplicit(name string) (explicitMeta, bool) {
	common := []string{"json", "format", "verbose"}
	items := map[string]explicitMeta{
		"version": {Description: "Print the log CLI build version, commit, and build date.",
			Flags: common, Risk: "read", Example: "log version --json"},
		"commands": {Description: "List available log commands with agent-facing metadata.",
			Flags: common, Risk: "read", Example: "log commands --json"},
		"schema": {Description: "Show argument and flag schema for a log command.",
			Flags: common, Required: []string{"command"}, Risk: "read", Example: "log schema analyze --json"},
		"help.llm": {Description: "Show log CLI guidance for agents that need bounded local log evidence.",
			Flags: common, Risk: "read", Example: "log help llm --json"},
		"analyze": {Description: "Stream local log files into a bounded analysis run directory.",
			Flags: append([]string{"source", "run", "format-hint", "max-bytes", "max-line-bytes"}, common...), Required: []string{"source", "run"}, Risk: "read", Example: "log analyze --source ./logs/app.log --run ./.log-runs/run_001 --json"},
		"profile": {Description: "Summarize sources, levels, templates, and time range for an analysis run.",
			Flags: append([]string{"run"}, common...), Required: []string{"run"}, Risk: "read", Example: "log profile --run ./.log-runs/run_001 --json"},
		"templates": {Description: "List recurring redacted log templates with counts and representative entries.",
			Flags: append([]string{"run", "only", "sort", "limit"}, common...), Required: []string{"run"}, Risk: "read", Example: "log templates --run ./.log-runs/run_001 --only non-info --sort count --json"},
		"entries": {Description: "Page through redacted indexed entries without printing the whole log.",
			Flags: append([]string{"run", "template-id", "level", "limit", "cursor"}, common...), Required: []string{"run"}, Risk: "read", Example: "log entries --run ./.log-runs/run_001 --level ERROR --limit 20 --json"},
		"search": {Description: "Search redacted entries by text or regex with bounded cursor pagination.",
			Flags: append([]string{"run", "query", "regex", "level", "template-id", "since", "until", "limit", "cursor"}, common...), Required: []string{"run", "query"}, Risk: "read", Example: "log search --run ./.log-runs/run_001 --query \"ERROR OR timeout\" --json"},
		"window": {Description: "Return redacted source lines around an entry id or file and line number.",
			Flags: append([]string{"run", "entry-id", "file", "line", "before", "after"}, common...), Required: []string{"run", "entry-id|file+line"}, Risk: "read", Example: "log window --run ./.log-runs/run_001 --entry-id entry_000001 --before 50 --after 50 --json"},
		"extract": {Description: "Extract stacktrace groups or error signatures from redacted indexed entries.",
			Flags: append([]string{"run", "kind", "limit"}, common...), Required: []string{"run", "kind"}, Risk: "read", Example: "log extract --run ./.log-runs/run_001 --kind stacktrace --json"},
	}
	item, ok := items[name]
	return item, ok
}

func logCommandUsage(name, fallback string) string {
	switch name {
	case "version":
		return "log version --json"
	case "commands":
		return "log commands --json"
	case "schema":
		return "log schema <command> --json"
	case "help.llm":
		return "log help llm"
	case "analyze":
		return "log analyze --source <path> --run <run-dir> [flags]"
	case "profile":
		return "log profile --run <run-dir> --json"
	case "templates":
		return "log templates --run <run-dir> [flags]"
	case "entries":
		return "log entries --run <run-dir> [flags]"
	case "search":
		return "log search --run <run-dir> --query <query> [flags]"
	case "window":
		return "log window --run <run-dir> (--entry-id <entry-id> OR --file <path> --line <line-number>) [flags]"
	case "extract":
		return "log extract --run <run-dir> --kind stacktrace|error-signature [flags]"
	default:
		return fallback
	}
}

func inspectImageExplicit(name string) (explicitMeta, bool) {
	items := map[string]explicitMeta{
		"inspect": {Description: "Inspect exactly one local image using a GitHub Copilot plugin backed vision model.",
			Flags: []string{"image", "prompt", "prompt-file", "model", "reasoning", "preset", "timeout", "out", "config", "json", "format", "verbose"}, Required: []string{"image", "prompt"}, Risk: "external_network_data_egress", Example: "inspect-image inspect --image ./screenshot.png --prompt \"Read the error message\" --out ./inspect-image-result.json --json"},
		"auth.login": {Description: "Authenticate inspect-image with GitHub device code flow.",
			Flags: []string{"config", "json", "format", "verbose"}, Risk: "write", Example: "inspect-image auth login"},
		"auth.status": {Description: "Show redacted inspect-image authentication status.",
			Flags: []string{"config", "json", "format", "verbose"}, Risk: "read", Example: "inspect-image auth status --json"},
		"auth.test": {Description: "Refresh and validate inspect-image authentication without printing tokens.",
			Flags: []string{"config", "json", "format", "verbose"}, Risk: "read", Example: "inspect-image auth test --json"},
		"auth.logout": {Description: "Clear inspect-image token fields after explicit confirmation.",
			Flags: []string{"yes", "config", "json", "format", "verbose"}, Required: []string{"yes"}, Risk: "delete", Example: "inspect-image auth logout --yes --json"},
		"doctor": {Description: "Check inspect-image config, auth, proxy mode, and model defaults.",
			Flags: []string{"config", "json", "format", "verbose"}, Risk: "read", Example: "inspect-image doctor --json"},
		"models": {Description: "List inspect-image allowed models and reasoning efforts.",
			Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "inspect-image models --json"},
		"commands": {Description: "List available Inspect Image commands with metadata.",
			Flags: []string{"json", "format", "verbose", "config"}, Risk: "read", Example: "inspect-image commands --json"},
		"schema": {Description: "Show inspect-image command schema.",
			Flags: []string{"json", "format", "verbose", "config"}, Required: []string{"command"}, Risk: "read", Example: "inspect-image schema inspect --json"},
		"help.llm": {Description: "Show Inspect Image usage guidance for LLM agents.",
			Flags: []string{"json", "format", "verbose", "config"}, Risk: "read", Example: "inspect-image help llm --json"},
		"version": {Description: "Print CLI version, commit, and build date.",
			Flags: []string{"json", "format", "verbose", "config"}, Risk: "read", Example: "inspect-image version --json"},
	}
	item, ok := items[name]
	return item, ok
}

func jenkinsExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	write := append(append([]string{}, common...), "dry-run")
	del := append([]string{"yes"}, common...)
	delDry := append(append([]string{"yes"}, common...), "dry-run")
	bodyWrite := append([]string{"body", "body-file", "body-stdin"}, write...)
	items := map[string]explicitMeta{
		"instance.list":    {Description: "List configured Jenkins instances with credentials redacted.", Flags: common, Risk: "read", Example: "jenkins instance list --json"},
		"instance.get":     {Description: "Show one configured Jenkins instance with credentials redacted.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "jenkins instance get ci --json"},
		"instance.add":     {Description: "Add a Jenkins instance to the shared EFP YAML config.", Flags: []string{"base-url", "rest-path", "crumb-mode", "username", "auth-type", "password-stdin", "api-key-stdin", "token-stdin", "default", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url", "username+api-key-stdin|username+password-stdin|token-stdin"}, Risk: "write", Example: "jenkins instance add ci --base-url https://jenkins.example.test --username user@example.test --api-key-stdin --default --json"},
		"instance.update":  {Description: "Update Jenkins instance URL or crumb behavior in the EFP config.", Flags: []string{"base-url", "rest-path", "crumb-mode", "config", "json", "format", "verbose"}, Required: []string{"name", "base-url|rest-path|crumb-mode"}, Risk: "write", Example: "jenkins instance update ci --crumb-mode auto --json"},
		"instance.remove":  {Description: "Remove a Jenkins instance from the EFP config after confirmation.", Flags: del, Required: []string{"name", "yes"}, Risk: "delete", Example: "jenkins instance remove ci --yes --json"},
		"instance.default": {Description: "Read or set the default Jenkins instance.", Flags: common, Risk: "write", Example: "jenkins instance default ci --json"},
		"auth.login":       {Description: "Store Jenkins credentials for the selected instance.", Flags: []string{"username", "auth-type", "password-stdin", "api-key-stdin", "token-stdin", "instance", "config", "json", "format", "verbose"}, Required: []string{"username+api-key-stdin|username+password-stdin|token-stdin"}, Risk: "write", Example: "jenkins auth login --instance ci --username user@example.test --api-key-stdin --json"},
		"auth.logout":      {Description: "Clear Jenkins credentials for the selected instance after confirmation.", Flags: del, Required: []string{"yes"}, Risk: "delete", Example: "jenkins auth logout --instance ci --yes --json"},
		"auth.test":        {Description: "Verify Jenkins credentials against the whoAmI endpoint.", Flags: common, Risk: "read", Example: "jenkins auth test --json"},
		"whoami":           {Description: "Fetch Jenkins identity and permission summary from whoAmI.", Flags: common, Risk: "read", Example: "jenkins whoami --json"},
		"server-info":      {Description: "Read Jenkins controller top-level JSON API metadata.", Flags: []string{"depth", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jenkins server-info --depth 1 --json"},
		"crumb.get":        {Description: "Fetch the Jenkins CSRF crumb for the selected instance when crumb issuer is enabled.", Flags: common, Risk: "read", Example: "jenkins crumb get --json"},
		"commands":         {Description: "List available Jenkins commands with metadata.", Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "jenkins commands --json"},
		"schema":           {Description: "Show argument and flag schema for a Jenkins command.", Flags: []string{"json", "format", "verbose"}, Required: []string{"command"}, Risk: "read", Example: "jenkins schema job.build --json"},
		"help.llm":         {Description: "Show Jenkins CLI usage guidance for LLM agents.", Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "jenkins help llm --json"},
		"version":          {Description: "Print Jenkins CLI version, commit, and build date.", Flags: common, Risk: "read", Example: "jenkins version --json"},

		"job.list":              {Description: "List Jenkins jobs from the controller root.", Flags: []string{"depth", "tree", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jenkins job list --depth 2 --json"},
		"job.get":               {Description: "Fetch Jenkins job metadata by slash folder path.", Flags: []string{"depth", "tree", "instance", "config", "json", "format", "verbose"}, Required: []string{"job"}, Risk: "read", Example: "jenkins job get folder/app-main --json"},
		"job.config.get":        {Description: "Fetch Jenkins job config.xml.", Flags: common, Required: []string{"job"}, Risk: "admin", Example: "jenkins job config get folder/app-main --json"},
		"job.config.update":     {Description: "Update Jenkins job config.xml.", Flags: bodyWrite, Required: []string{"job", "body|body-file|body-stdin"}, Risk: "write", Example: "jenkins job config update folder/app-main --body-file config.xml --dry-run --json"},
		"job.create":            {Description: "Create a Jenkins job from config.xml.", Flags: append([]string{"folder"}, bodyWrite...), Required: []string{"job", "body|body-file|body-stdin"}, Risk: "write", Example: "jenkins job create app-main --folder folder --body-file config.xml --dry-run --json"},
		"job.copy":              {Description: "Copy an existing Jenkins job to a new job name.", Flags: append([]string{"folder"}, write...), Required: []string{"source", "target"}, Risk: "write", Example: "jenkins job copy folder/template app-main --folder folder --dry-run --json"},
		"job.delete":            {Description: "Delete a Jenkins job after confirmation.", Flags: delDry, Required: []string{"job", "yes"}, Risk: "delete", Example: "jenkins job delete folder/app-main --yes --dry-run --json"},
		"job.enable":            {Description: "Enable a Jenkins job.", Flags: write, Required: []string{"job"}, Risk: "write", Example: "jenkins job enable folder/app-main --dry-run --json"},
		"job.disable":           {Description: "Disable a Jenkins job.", Flags: write, Required: []string{"job"}, Risk: "write", Example: "jenkins job disable folder/app-main --dry-run --json"},
		"job.build":             {Description: "Trigger a Jenkins job build and return the queue item location.", Flags: append([]string{"delay"}, write...), Required: []string{"job"}, Risk: "write", Example: "jenkins job build folder/app-main --delay 0sec --dry-run --json"},
		"job.build-with-params": {Description: "Trigger a Jenkins parameterized job build.", Flags: append([]string{"param", "delay"}, write...), Required: []string{"job"}, Risk: "write", Example: "jenkins job build-with-params folder/app-main --param BRANCH=main --dry-run --json"},

		"queue.list":   {Description: "List Jenkins queue items.", Flags: common, Risk: "read", Example: "jenkins queue list --json"},
		"queue.get":    {Description: "Fetch one Jenkins queue item by id.", Flags: common, Required: []string{"queue-id"}, Risk: "read", Example: "jenkins queue get 123 --json"},
		"queue.cancel": {Description: "Cancel a Jenkins queue item after confirmation.", Flags: delDry, Required: []string{"queue-id", "yes"}, Risk: "delete", Example: "jenkins queue cancel 123 --yes --dry-run --json"},

		"build.get":        {Description: "Fetch Jenkins build metadata.", Flags: []string{"tree", "depth", "instance", "config", "json", "format", "verbose"}, Required: []string{"job", "build"}, Risk: "read", Example: "jenkins build get folder/app-main 42 --json"},
		"build.status":     {Description: "Fetch compact Jenkins build status and result.", Flags: common, Required: []string{"job", "build"}, Risk: "read", Example: "jenkins build status folder/app-main lastBuild --json"},
		"build.log":        {Description: "Read Jenkins build console log or one progressive log chunk.", Flags: []string{"start", "instance", "config", "json", "format", "verbose"}, Required: []string{"job", "build"}, Risk: "read", Example: "jenkins build log folder/app-main 42 --json"},
		"build.log-follow": {Description: "Poll Jenkins progressive build log and return accumulated text.", Flags: []string{"start", "max-rounds", "wait-ms", "instance", "config", "json", "format", "verbose"}, Required: []string{"job", "build"}, Risk: "read", Example: "jenkins build log-follow folder/app-main 42 --max-rounds 3 --json"},
		"build.stop":       {Description: "Stop a Jenkins build after confirmation.", Flags: delDry, Required: []string{"job", "build", "yes"}, Risk: "write_requires_confirmation", Example: "jenkins build stop folder/app-main 42 --yes --dry-run --json"},
		"build.artifacts":  {Description: "List artifacts archived by a Jenkins build.", Flags: common, Required: []string{"job", "build"}, Risk: "read", Example: "jenkins build artifacts folder/app-main 42 --json"},
		"artifact.download": {Description: "Download a Jenkins build artifact to a local file.",
			Flags: append([]string{"output"}, write...), Required: []string{"job", "build", "path"}, Risk: "read", Example: "jenkins artifact download folder/app-main 42 target/app.jar --output app.jar --json"},

		"pipeline.runs":      {Description: "List Pipeline REST API runs for a Jenkins job.", Flags: common, Required: []string{"job"}, Risk: "read", Example: "jenkins pipeline runs folder/app-main --json"},
		"pipeline.run":       {Description: "Fetch Pipeline REST API details for one run.", Flags: common, Required: []string{"job", "run-id"}, Risk: "read", Example: "jenkins pipeline run folder/app-main 42 --json"},
		"pipeline.stages":    {Description: "Fetch Pipeline REST API stage details for one run.", Flags: common, Required: []string{"job", "run-id"}, Risk: "read", Example: "jenkins pipeline stages folder/app-main 42 --json"},
		"pipeline.node-log":  {Description: "Read Pipeline REST API node log text.", Flags: common, Required: []string{"job", "run-id", "node-id"}, Risk: "read", Example: "jenkins pipeline node-log folder/app-main 42 6 --json"},
		"pipeline.artifacts": {Description: "List Pipeline REST API artifacts for one run.", Flags: common, Required: []string{"job", "run-id"}, Risk: "read", Example: "jenkins pipeline artifacts folder/app-main 42 --json"},

		"view.list":          {Description: "List Jenkins views.", Flags: common, Risk: "read", Example: "jenkins view list --json"},
		"view.get":           {Description: "Fetch Jenkins view metadata.", Flags: common, Required: []string{"view"}, Risk: "read", Example: "jenkins view get All --json"},
		"view.create":        {Description: "Create a Jenkins view from config XML.", Flags: bodyWrite, Required: []string{"view", "body|body-file|body-stdin"}, Risk: "write", Example: "jenkins view create Release --body-file view.xml --dry-run --json"},
		"view.delete":        {Description: "Delete a Jenkins view after confirmation.", Flags: delDry, Required: []string{"view", "yes"}, Risk: "delete", Example: "jenkins view delete Release --yes --dry-run --json"},
		"view.config.get":    {Description: "Fetch Jenkins view config.xml.", Flags: common, Required: []string{"view"}, Risk: "admin", Example: "jenkins view config get All --json"},
		"view.config.update": {Description: "Update Jenkins view config.xml.", Flags: bodyWrite, Required: []string{"view", "body|body-file|body-stdin"}, Risk: "write", Example: "jenkins view config update All --body-file view.xml --dry-run --json"},
		"node.list":          {Description: "List Jenkins controller and agent nodes.", Flags: common, Risk: "read", Example: "jenkins node list --json"},
		"node.get":           {Description: "Fetch Jenkins node metadata.", Flags: common, Required: []string{"node"}, Risk: "read", Example: "jenkins node get built-in --json"},
		"plugin.list":        {Description: "List installed Jenkins plugins.", Flags: common, Risk: "read", Example: "jenkins plugin list --json"},
		"plugin.get":         {Description: "Fetch one installed Jenkins plugin by short name.", Flags: common, Required: []string{"plugin"}, Risk: "read", Example: "jenkins plugin get workflow-job --json"},

		"system.quiet-down":        {Description: "Put Jenkins into quiet-down mode.", Flags: append([]string{"reason", "block"}, write...), Risk: "admin", Example: "jenkins system quiet-down --reason Maintenance --dry-run --json"},
		"system.cancel-quiet-down": {Description: "Cancel Jenkins quiet-down mode.", Flags: write, Risk: "admin", Example: "jenkins system cancel-quiet-down --dry-run --json"},
		"system.safe-restart":      {Description: "Request a Jenkins safe restart after confirmation.", Flags: delDry, Required: []string{"yes"}, Risk: "write_requires_confirmation", Example: "jenkins system safe-restart --yes --dry-run --json"},
		"api.get":                  {Description: "Call a raw Jenkins GET API path on the selected instance.", Flags: append([]string{"query"}, common...), Required: []string{"path"}, Risk: "read", Example: "jenkins api get /api/json --query depth=1 --json"},
		"api.post":                 {Description: "Call a raw Jenkins POST API path on the selected instance.", Flags: append([]string{"query", "body", "body-file", "body-stdin", "content-type"}, write...), Required: []string{"path"}, Risk: "write", Example: "jenkins api post /quietDown --body '{}' --dry-run --json"},
		"api.put":                  {Description: "Call a raw Jenkins PUT API path on the selected instance.", Flags: append([]string{"query", "body", "body-file", "body-stdin", "content-type"}, write...), Required: []string{"path"}, Risk: "write", Example: "jenkins api put /job/app/config.xml --body-file config.xml --dry-run --json"},
		"api.delete":               {Description: "Call a raw Jenkins DELETE API path after confirmation.", Flags: delDry, Required: []string{"path", "yes"}, Risk: "delete", Example: "jenkins api delete /job/app --yes --dry-run --json"},
	}
	item, ok := items[name]
	return item, ok
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
	product := productName(parts[0])
	cmd := cleanParts(parts[1:])
	if len(cmd) == 0 {
		return "Show " + product + " command information."
	}
	actionIdx := actionIndex(cmd)
	action := cmd[actionIdx]
	resourceParts := append([]string{}, cmd[:actionIdx]...)
	resourceParts = append(resourceParts, cmd[actionIdx+1:]...)
	resource := resourceName(product, resourceParts)
	switch action {
	case "list":
		return "List " + resource + "."
	case "get":
		return "Fetch " + singular(resource) + "."
	case "search", "cql":
		return "Search " + resource + "."
	case "create", "add", "upload", "login":
		return strings.Title(action) + " " + singular(resource) + "."
	case "update", "set", "edit", "assign", "transition", "move", "restore", "watch", "unwatch", "vote", "unvote", "default":
		return strings.Title(action) + " " + singular(resource) + "."
	case "delete", "remove", "logout":
		return strings.Title(action) + " " + singular(resource) + " after explicit confirmation."
	case "download", "export-html", "export-markdown":
		return strings.Title(strings.ReplaceAll(action, "-", " ")) + " for " + singular(resource) + "."
	case "commands":
		return "List available " + product + " commands with metadata."
	case "schema":
		return "Show argument and flag schema for a " + product + " command."
	case "llm":
		return "Show " + product + " usage guidance for LLM agents."
	case "version":
		return "Print CLI version, commit, and build date."
	default:
		return strings.Title(action) + " " + resource + "."
	}
}

func defaultFlags(product, r string) []string {
	if product == "browser" {
		return []string{"json", "format", "verbose"}
	}
	if product == "inspect-image" {
		return []string{"config", "json", "format", "verbose"}
	}
	flags := []string{"instance", "config", "json", "format", "verbose"}
	switch r {
	case "write":
		flags = append(flags, "dry-run")
	case "delete":
		flags = append([]string{"yes"}, flags...)
	}
	return flags
}

func cleanParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "[") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func actionIndex(parts []string) int {
	actions := map[string]bool{
		"list": true, "get": true, "search": true, "cql": true, "create": true, "add": true,
		"upload": true, "login": true, "update": true, "set": true, "edit": true, "assign": true,
		"transition": true, "move": true, "restore": true, "watch": true, "unwatch": true,
		"vote": true, "unvote": true, "default": true, "delete": true, "remove": true, "logout": true,
		"download": true, "export": true, "export-html": true, "export-markdown": true, "commands": true, "schema": true,
		"llm": true, "version": true, "content": true, "pages": true, "blogs": true, "labels": true,
		"watchers": true, "members": true, "statuses": true, "roles": true, "components": true,
		"versions": true, "transitions": true, "changelog": true, "fields": true, "createmeta": true,
		"editmeta": true, "votes": true, "notify": true, "myself": true, "server-info": true,
		"resolve-url": true, "body": true, "body-storage": true, "body-view": true, "children": true,
		"descendants": true, "ancestors": true, "history": true, "permission": true, "settings": true,
		"config": true, "assignable": true, "issues": true, "summary": true, "count": true,
		"bulk-update-status": true, "coverage": true, "resolve": true, "catalog": true,
		"describe": true, "clauses": true, "autocomplete-json": true, "autocomplete": true,
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if actions[parts[i]] {
			return i
		}
	}
	return len(parts) - 1
}

func productName(product string) string {
	switch product {
	case "jira":
		return "Jira"
	case "confluence":
		return "Confluence"
	case "browser":
		return "Browser"
	case "inspect-image":
		return "Inspect Image"
	default:
		return product
	}
}

func resourceName(product string, parts []string) string {
	if len(parts) == 0 {
		return product + " resources"
	}
	words := append([]string{product}, parts...)
	return strings.ReplaceAll(strings.Join(words, " "), "-", " ")
}

func singular(s string) string {
	for _, suffix := range []string{" resources", " issues", " pages", " blogs", " labels", " watchers", " members", " statuses", " roles", " components", " versions", " transitions", " votes"} {
		if strings.HasSuffix(s, suffix) {
			return strings.TrimSuffix(s, "s")
		}
	}
	return s
}

func exampleFor(usage, r string) string {
	example := usage
	replacements := map[string]string{
		"<issue-or-url>":    "PROJ-123",
		"<jira-url>":        "https://jira.example.test/projects/PROJ?selectedItem=com.thed.zephyr.je%3Azephyr-tests-page#test-summary-tab",
		"<comment-id>":      "10000",
		"<attachment-id>":   "10000",
		"<worklog-id>":      "10000",
		"<link-id>":         "10000",
		"<project-key>":     "PROJ",
		"<project-id>":      "10000",
		"<issue-id>":        "10001",
		"<cycle-id>":        "20000",
		"<execution-id>":    "30000",
		"[execution-id]":    "30000",
		"<folder-id>":       "40000",
		"<endpoint-id>":     "execution.update-status",
		"<component-id>":    "10000",
		"<version-id>":      "10000",
		"<group-name>":      "team",
		"<filter-id>":       "10000",
		"<dashboard-id>":    "10000",
		"<board-id>":        "1",
		"<sprint-id>":       "1",
		"<space-key>":       "ENG",
		"<content-id>":      "123",
		"<blog-id-or-url>":  "123",
		"<task-id>":         "10000",
		"<webhook-id>":      "10000",
		"<role-id-or-name>": "10000",
		"<name>":            "local",
		"<key>":             "status",
		"<url>":             "https://example.atlassian.net/browse/PROJ-123",
		"<command>":         "issue.create",
		"<path>":            "/rest/api/2/myself",
		"<file>":            "./note.txt",
		"[name]":            "local",
	}
	if strings.HasPrefix(usage, "browser ") {
		replacements["<command>"] = "probe"
		replacements["<url>"] = "https://intranet.example.test/app"
	}
	for old, newValue := range replacements {
		example = strings.ReplaceAll(example, old, newValue)
	}
	if r == "delete" && !strings.Contains(example, "--yes") {
		example += " --yes"
	}
	return example + " --json"
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
	out := []string{}
	for _, p := range strings.Fields(usage) {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "[") {
			out = append(out, p)
		}
	}
	return out
}

func argumentDescription(name string) string {
	switch name {
	case "issue-or-url":
		return "Jira issue key or full Jira issue URL."
	case "jira-url":
		return "Full Jira URL, especially a Zephyr project or test-management URL."
	case "comment-id":
		return "Comment id."
	case "attachment-id":
		return "Attachment id."
	case "worklog-id":
		return "Jira worklog id."
	case "link-id":
		return "Jira issue link id."
	case "project-key":
		return "Jira project key."
	case "project-id":
		return "Jira numeric project id."
	case "issue-id":
		return "Jira numeric issue id."
	case "cycle-id":
		return "Zephyr test cycle id."
	case "execution-id":
		return "Zephyr execution id. Omit it only when the command supports resolving by --cycle-id plus --issue."
	case "folder-id":
		return "Zephyr folder id."
	case "endpoint-id":
		return "Zephyr API endpoint id from jira zephyr api catalog."
	case "component-id":
		return "Jira component id."
	case "version-id":
		return "Jira or Zephyr version id."
	case "group-name":
		return "Jira or Confluence group name."
	case "filter-id":
		return "Jira filter id."
	case "dashboard-id":
		return "Jira dashboard id."
	case "board-id":
		return "Jira agile board id."
	case "sprint-id":
		return "Jira sprint id."
	case "space-key":
		return "Confluence space key."
	case "content-id":
		return "Confluence content id."
	case "blog-id-or-url":
		return "Confluence blog id or full blog URL."
	case "task-id":
		return "Confluence long task id."
	case "webhook-id":
		return "Confluence webhook id."
	case "role-id-or-name":
		return "Jira project role id or role name."
	case "name":
		return "Instance or resource name."
	case "key":
		return "Property key."
	case "url":
		return "Full HTTP or HTTPS URL."
	case "command":
		return "Dotted command name such as issue.get."
	case "path":
		return "REST API path. Relative paths are resolved under the selected instance; full URLs must belong to that instance."
	case "file":
		return "Local file path."
	default:
		return "Positional command argument."
	}
}

func required(name string) []string {
	switch name {
	case "instance.add":
		return []string{"name", "base-url", "username+password-stdin|username+api-key-stdin|token-stdin"}
	case "instance.update":
		return []string{"name", "base-url|rest-path"}
	case "instance.remove":
		return []string{"name", "yes"}
	case "auth.login":
		return []string{"username+password-stdin|username+api-key-stdin|token-stdin"}
	case "issue.create":
		return []string{"project", "type", "summary"}
	case "issue.assign":
		return []string{"issue-or-url", "user"}
	case "issue.notify":
		return []string{"issue-or-url", "subject", "body|body-file|body-stdin", "to"}
	case "page.create":
		return []string{"space", "title", "body|body-file|body-stdin"}
	case "filter.create":
		return []string{"name", "jql"}
	case "filter.update":
		return []string{"filter-id", "name|jql|description"}
	case "component.create", "version.create":
		return []string{"project", "name"}
	case "component.update", "version.update":
		return []string{"name|description"}
	case "issue.update":
		return []string{"summary|description|field"}
	case "issue.comment.add", "issue.comment.update":
		return []string{"body"}
	case "issue.worklog.add":
		return []string{"time-spent"}
	case "issue.worklog.update":
		return []string{"time-spent|started|comment"}
	case "user.get":
		return []string{"username|key|user-key"}
	case "user.search", "group.search", "search.user":
		return []string{"query"}
	case "user.assignable":
		return []string{"project|issue"}
	case "space.create":
		return []string{"key", "name"}
	case "space.update":
		return []string{"space-key", "name"}
	case "space.property.set":
		return []string{"space-key", "key", "body|body-file|body-stdin"}
	case "page.move":
		return []string{"id|url", "parent-id"}
	case "page.children", "page.descendants", "page.ancestors", "page.body", "page.body-storage", "page.body-view", "page.version", "page.history", "page.watcher.list", "page.watch", "page.unwatch":
		return []string{"id|url"}
	case "page.restore":
		return []string{"id|url", "version"}
	case "page.export-html":
		return []string{"id|url"}
	case "content.create":
		return []string{"space", "title", "body|body-file|body-stdin"}
	case "content.update":
		return []string{"content-id", "title|body|body-file|body-stdin"}
	case "blog.create":
		return []string{"space", "title", "body|body-file|body-stdin"}
	case "blog.update":
		return []string{"blog-id-or-url", "title|body|body-file|body-stdin"}
	case "page.attachment.update":
		return []string{"page-id|id|url", "attachment-id", "file"}
	case "page.property.list":
		return []string{"id|url"}
	case "page.property.get", "page.property.delete":
		return []string{"id|url", "key"}
	case "page.property.set":
		return []string{"id|url", "key", "body|body-file|body-stdin"}
	case "page.restriction.list":
		return []string{"id|url"}
	case "page.restriction.add":
		return []string{"id|url", "user|group"}
	case "page.restriction.delete":
		return []string{"id|url", "operation"}
	case "webhook.create":
		return []string{"name", "url", "event"}
	case "probe":
		return []string{"url"}
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
		out = append(out, FlagSpec{Name: f, Type: flagTypeFor(command, f), Required: req[f], Description: flagDescription(command, f)})
	}
	return out
}

func flagTypeFor(command, name string) string {
	if name == "query" {
		if strings.HasPrefix(command, "api.") || strings.HasPrefix(command, "zephyr.api.") {
			return "string[]"
		}
		return "string"
	}
	return flagType(name)
}

func flagType(name string) string {
	switch name {
	case "json", "verbose", "dry-run", "yes", "body-stdin", "minor-edit", "legacy", "enable-probe", "include-template-defaults", "fail-fast", "confirm-mapping", "apply-post-create-updates", "require-selector", "clean-profile", "headless", "ignore-cert-errors", "save-html", "save-screenshot":
		return "bool"
	case "sample-rows", "max-create", "wait", "timeout", "max-network-events":
		return "int"
	case "min-confidence":
		return "float"
	case "field", "fields":
		return "string[]"
	default:
		return "string"
	}
}

func flagDescription(command, name string) string {
	switch {
	case command == "page.body" && name == "format":
		return "Confluence body representation to expand, such as storage, view, or export_view."
	case strings.HasPrefix(command, "page.restriction.") && name == "user":
		return "Confluence username to add to the page restriction; repeat for multiple users."
	case strings.HasPrefix(command, "page.restriction.") && name == "group":
		return "Confluence group name to add to the page restriction; repeat for multiple groups."
	case strings.HasPrefix(command, "page.restriction.") && name == "operation":
		return "Restriction operation: read or update."
	case command == "issue.assign" && name == "user":
		return "Jira username or account identifier to assign the issue to."
	case strings.Contains(command, "attachment") && name == "file":
		return "Local file path to upload."
	case strings.HasPrefix(command, "api.") && name == "query":
		return "Raw query parameter in key=value form; repeat for multiple parameters."
	case strings.HasPrefix(command, "zephyr.api.") && name == "query":
		return "Raw Zephyr ZAPI query parameter in key=value form; repeat for multiple parameters."
	}
	switch name {
	case "instance":
		return "Configured instance name."
	case "config":
		return "Path to config file."
	case "json":
		return "Print JSON envelope."
	case "format":
		return "Output format: table, json, or yaml."
	case "verbose":
		return "Print additional diagnostic details when available; secrets must remain redacted."
	case "dry-run":
		return "Preview write request without sending it."
	case "yes":
		return "Confirm destructive operations."
	case "base-url":
		return "Base URL for the Jira or Confluence instance, for example https://jira.example.test."
	case "rest-path":
		return "REST API root path for the instance."
	case "api-version":
		return "Jira REST API version used to build the default REST path."
	case "auth-type":
		return "Authentication type: basic_password, basic_api_key, bearer_token, or a supported alias."
	case "username":
		return "Username for basic authentication or user lookup."
	case "user-key":
		return "Confluence user key."
	case "password-stdin":
		return "Read the password from stdin."
	case "api-key-stdin":
		return "Read the API key from stdin."
	case "token-stdin":
		return "Read the bearer token or PAT from stdin."
	case "default":
		return "Make the added instance the default instance."
	case "project":
		return "Jira project key."
	case "project-id":
		return "Jira project id."
	case "version-id":
		return "Jira or Zephyr version id; -1 uses the legacy unscheduled/ad hoc version."
	case "cycle-id":
		return "Zephyr test cycle id."
	case "folder-id":
		return "Zephyr test cycle folder id."
	case "execution-id":
		return "Zephyr test execution id."
	case "execution-ids":
		return "Comma-separated Zephyr test execution ids."
	case "issue-id":
		return "Jira issue id."
	case "issue":
		return "Jira issue key."
	case "issues":
		return "Comma-separated Jira test issue keys."
	case "status":
		return "Zephyr execution status name or alias, such as PASS, PASSED, FAIL, WIP, BLOCKED, or UNEXECUTED."
	case "comment":
		return "Comment text."
	case "subject":
		return "Notification subject."
	case "to":
		return "Target transition name, issue link target, or notification recipient depending on the command."
	case "user":
		return "User identifier for this command."
	case "step-id":
		return "Zephyr test step id."
	case "step":
		return "Zephyr test step instruction text."
	case "data":
		return "Zephyr test step input data."
	case "result":
		return "Zephyr test step expected result."
	case "entity-type":
		return "Zephyr attachment entity type, such as execution."
	case "entity-id":
		return "Zephyr attachment entity id."
	case "file":
		return "File path to upload."
	case "jql":
		return "Jira JQL query."
	case "zql":
		return "Zephyr ZQL query."
	case "query":
		return "Search query text."
	case "limit":
		return "Maximum number of results."
	case "start":
		return "Start offset for paged results."
	case "offset":
		return "Start offset for paged results."
	case "field-name":
		return "Zephyr ZQL field name."
	case "field-value":
		return "Zephyr ZQL field value prefix."
	case "endpoint-id":
		return "Official Zephyr API endpoint id from jira zephyr api catalog."
	case "group":
		return "Grouping mode; currently cycle for Zephyr execution counts."
	case "description":
		return "Description text."
	case "name":
		return "Resource name for the command target, such as an instance, cycle, folder, component, version, filter, or webhook name."
	case "key":
		return "Property key or user key, depending on the command."
	case "build":
		return "Zephyr cycle build value."
	case "environment":
		return "Zephyr cycle environment value."
	case "enable-probe":
		return "Allow Zephyr doctor to probe even when zephyr.enabled=false."
	case "type":
		return "Issue type name."
	case "summary":
		return "Issue summary."
	case "field":
		return "Jira field override in key=value form; repeat for multiple fields. Values that parse as JSON are sent as JSON."
	case "json-body":
		return "Complete JSON request body. When set, field-specific create/update flags are ignored."
	case "json-body-file":
		return "Path to a file containing the complete JSON request body."
	case "space":
		return "Confluence space key."
	case "title":
		return "Page title."
	case "body":
		return "Inline request body text or JSON, depending on the command."
	case "body-file":
		return "Path to a file containing the request body."
	case "body-stdin":
		return "Read the request body from stdin."
	case "body-format":
		return "Confluence body representation for page/content/blog/comment writes, usually storage."
	case "cql":
		return "Confluence CQL query."
	case "transition-id":
		return "Jira transition id."
	case "from":
		return "Source Jira issue key for an issue link."
	case "fields":
		return "Jira fields selector. Repeat or pass comma-separated fields depending on the command."
	case "expand":
		return "Expand selector for richer response fields."
	case "from-csv":
		return "CSV file to read."
	case "template-issue":
		return "Existing Jira issue used as metadata and default template."
	case "mapping":
		return "Mapping plan JSON file."
	case "field-catalog":
		return "Local JSON field catalog file."
	case "example-issue":
		return "Local JSON example issue file."
	case "create-meta":
		return "Local JSON create metadata file."
	case "edit-meta":
		return "Local JSON edit metadata file."
	case "metadata-mode":
		return "Metadata lookup mode: auto, createmeta, or editmeta-degraded."
	case "output":
		return "Path to write JSON output."
	case "sample-rows":
		return "Number of CSV sample rows used for mapping."
	case "min-confidence":
		return "Minimum confidence required to suggest a mapping."
	case "include-template-defaults":
		return "Copy safe writable defaults from the template issue."
	case "max-create":
		return "Maximum number of issues allowed for this run."
	case "fail-fast":
		return "Stop after the first row validation or create failure."
	case "confirm-mapping":
		return "Confirm reviewed ambiguous or low-confidence mapping plan entries."
	case "apply-post-create-updates":
		return "Apply post_create_update mappings with Jira issue update after create."
	case "type-id":
		return "Jira issue type id."
	case "from-issue":
		return "Issue key or URL used to infer project and issue type."
	case "legacy":
		return "Force legacy createmeta endpoint."
	case "lead":
		return "Project lead username for a Jira component."
	case "release-date":
		return "Version release date in YYYY-MM-DD form."
	case "released":
		return "Mark the Jira version as released."
	case "favorite":
		return "Limit to favorite filters or mark the filter as favorite, depending on the command."
	case "state":
		return "Sprint state filter, such as active, future, or closed."
	case "time-spent":
		return "Jira worklog duration, such as 1h or 30m."
	case "started":
		return "Jira worklog start timestamp accepted by the Jira REST API."
	case "value":
		return "Inline JSON value for the Jira issue property."
	case "value-file":
		return "Path to a file containing the JSON value for the Jira issue property."
	case "metadata-only":
		return "Return attachment metadata without downloading content."
	case "id":
		return "Confluence page id. Use exactly one of --id or --url where both are available."
	case "url":
		if strings.HasPrefix(command, "issue.remote-link.") {
			return "External URL for the Jira remote link."
		}
		if command == "probe" {
			return "HTTP or HTTPS URL to open in Edge, Chrome, or Chromium."
		}
		if strings.HasPrefix(command, "page.") {
			return "Confluence page URL. Use exactly one of --id or --url where both are available."
		}
		return "HTTP or HTTPS URL to open or resolve. For Confluence page commands, use exactly one of --id or --url."
	case "page-id":
		return "Confluence page id for attachment commands; --id and --url are accepted aliases where available."
	case "parent-id":
		return "Confluence parent page id."
	case "position":
		return "Confluence move position relative to --parent-id, such as append, above, or below."
	case "minor-edit":
		return "Mark the Confluence update as a minor edit."
	case "version":
		return "Confluence content version number."
	case "label":
		return "Confluence label name. Repeat when the flag accepts multiple values."
	case "prefix":
		return "Confluence label prefix."
	case "event":
		return "Confluence webhook event name; repeat for multiple events."
	case "attachment-id":
		return "Confluence attachment id."
	case "text":
		return "Text term used to build a Confluence content search query."
	case "selector":
		return "CSS selector used as a deterministic login-success signal."
	case "require-selector":
		return "Fail with selector_not_found when --selector is not visible."
	case "wait":
		return "Seconds to wait after the page body is ready."
	case "timeout":
		return "Overall probe timeout in seconds."
	case "out":
		if command == "probe" {
			return "Directory for screenshot, HTML, network, and summary artifacts."
		}
		return "Path to write a JSON envelope copy."
	case "profile":
		return "Dedicated browser user-data-dir for this probe."
	case "clean-profile":
		return "Delete the dedicated probe profile before launching the browser."
	case "browser-exe":
		return "Explicit Edge/Chrome/Chromium executable path."
	case "browser":
		return "Browser family: edge, chrome, chromium, or auto."
	case "headless":
		return "Run the browser without a visible UI."
	case "ignore-cert-errors":
		return "Ignore certificate errors for internal self-signed certificate diagnostics."
	case "fetch-api":
		return "Path or URL to fetch from the loaded page context with credentials included."
	case "network-filter":
		return "Substring used to highlight matching network URLs in api_events."
	case "max-network-events":
		return "Maximum number of network events to retain."
	case "save-html":
		return "Write page.html into --out."
	case "save-screenshot":
		return "Write screenshot.png into --out."
	default:
		return "Command option."
	}
}

func FlagDescription(command, name string) string {
	return flagDescription(command, name)
}

var explicit = map[string]explicitMeta{
	"inspect":              {Description: "Inspect exactly one local image using a GitHub Copilot plugin backed vision model.", Flags: []string{"image", "prompt", "prompt-file", "model", "reasoning", "preset", "timeout", "config", "json", "format", "verbose"}, Required: []string{"image", "prompt"}, Risk: "external_network_data_egress", Example: "inspect-image inspect --image ./screenshot.png --prompt \"Read the error message\" --json"},
	"doctor":               {Description: "Check inspect-image config, auth, proxy mode, and model defaults.", Flags: []string{"config", "json", "format", "verbose"}, Risk: "read", Example: "inspect-image doctor --json"},
	"models":               {Description: "List inspect-image allowed models and reasoning efforts.", Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "inspect-image models --json"},
	"probe":                {Description: "Open an internal URL in Edge/Chrome/Chromium, capture screenshot/HTML/network summary, and report browser SSO indicators.", Flags: []string{"url", "selector", "require-selector", "wait", "timeout", "out", "profile", "clean-profile", "browser-exe", "browser", "headless", "ignore-cert-errors", "fetch-api", "network-filter", "max-network-events", "save-html", "save-screenshot", "json", "format", "verbose"}, Required: []string{"url"}, Risk: "read", Example: "browser probe --url https://intranet.example.test --selector .user-avatar --wait 10 --out result --json"},
	"version":              {Description: "Print CLI version, commit, and build date.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira version --json"},
	"auth.test":            {Description: "Verify configured credentials against the current user endpoint.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira auth test --json"},
	"server-info":          {Description: "Read server metadata from the selected instance.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira server-info --json"},
	"issue.get":            {Description: "Fetch a Jira issue by key or full issue URL.", Flags: []string{"fields", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url"}, Risk: "read", Example: "jira issue get PROJ-123 --fields '*all' --expand names,schema,editmeta --json"},
	"issue.search":         {Description: "Search Jira issues with JQL.", Flags: []string{"jql", "limit", "start", "fields", "instance", "config", "json", "format", "verbose"}, Required: []string{"jql"}, Risk: "read", Example: "jira issue search --jql \"project = PROJ\" --json"},
	"issue.create":         {Description: "Create a Jira issue.", Flags: []string{"project", "type", "summary", "description", "field", "json-body", "json-body-file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"project", "type", "summary"}, Risk: "write", Example: "jira issue create --project PROJ --type Task --summary Test --json"},
	"issue.update":         {Description: "Update fields on a Jira issue.", Flags: []string{"summary", "description", "field", "json-body", "json-body-file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"summary|description|field|json-body|json-body-file"}, Risk: "write", Example: "jira issue update PROJ-123 --summary Done --json"},
	"issue.delete":         {Description: "Delete a Jira issue after explicit confirmation.", Flags: []string{"yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "yes"}, Risk: "delete", Example: "jira issue delete PROJ-123 --yes --json"},
	"issue.transition":     {Description: "Transition a Jira issue by transition id or transition name.", Flags: []string{"transition-id", "to", "comment", "field", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"transition-id|to"}, Risk: "write", Example: "jira issue transition PROJ-123 --to Done --json"},
	"issue.createmeta":     {Description: "Fetch and normalize Jira create metadata.", Flags: []string{"project", "project-id", "type", "type-id", "from-issue", "legacy", "expand", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira issue createmeta --from-issue PROJ-123 --json"},
	"issue.map-csv":        {Description: "Build a deterministic Jira field mapping plan for a test case CSV.", Flags: []string{"from-csv", "template-issue", "project", "type", "field-catalog", "example-issue", "create-meta", "edit-meta", "metadata-mode", "output", "sample-rows", "min-confidence", "include-template-defaults", "instance", "config", "json", "format", "verbose"}, Required: []string{"from-csv", "template-issue"}, Risk: "read", Example: "jira issue map-csv --from-csv testcases.csv --template-issue PROJ-123 --output mapping-plan.json --json"},
	"issue.bulk-create":    {Description: "Validate or create Jira issues from a CSV mapping plan; actual create requires --yes and may require --confirm-mapping.", Flags: []string{"from-csv", "mapping", "template-issue", "output", "max-create", "fail-fast", "confirm-mapping", "apply-post-create-updates", "project", "type", "field-catalog", "example-issue", "create-meta", "edit-meta", "metadata-mode", "sample-rows", "min-confidence", "include-template-defaults", "yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"from-csv"}, Risk: "write_requires_confirmation", Example: "jira issue bulk-create --from-csv testcases.csv --mapping mapping-plan.json --dry-run --output dry-run.json --json"},
	"issue.bulk-validate":  {Description: "Alias for dry-run CSV bulk create validation.", Flags: []string{"from-csv", "mapping", "template-issue", "output", "max-create", "fail-fast", "confirm-mapping", "apply-post-create-updates", "project", "type", "field-catalog", "example-issue", "create-meta", "edit-meta", "metadata-mode", "sample-rows", "min-confidence", "include-template-defaults", "instance", "config", "json", "format", "verbose"}, Required: []string{"from-csv"}, Risk: "read", Example: "jira issue bulk-validate --from-csv testcases.csv --mapping mapping-plan.json --json"},
	"issue.comment.list":   {Description: "List comments on a Jira issue.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url"}, Risk: "read", Example: "jira issue comment list PROJ-123 --json"},
	"issue.comment.add":    {Description: "Add a comment to a Jira issue.", Flags: []string{"body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"body|body-file|body-stdin"}, Risk: "write", Example: "jira issue comment add PROJ-123 --body ok --json"},
	"issue.comment.update": {Description: "Update an existing Jira issue comment.", Flags: []string{"body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "comment-id", "body|body-file|body-stdin"}, Risk: "write", Example: "jira issue comment update PROJ-123 10000 --body ok --json"},
	"issue.comment.delete": {Description: "Delete a Jira issue comment after explicit confirmation.", Flags: []string{"yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url", "comment-id", "yes"}, Risk: "delete", Example: "jira issue comment delete PROJ-123 10000 --yes --json"},
	"zephyr.doctor":        {Description: "Probe Zephyr legacy ZAPI availability for a Jira project.", Flags: []string{"project", "enable-probe", "instance", "config", "json", "format", "verbose"}, Required: []string{"project"}, Risk: "read", Example: "jira zephyr doctor --project EFP --json"},
	"zephyr.resolve-url":   {Description: "Parse a Jira project URL that points at a Zephyr test-management page.", Flags: []string{"json", "format", "verbose"}, Required: []string{"jira-url"}, Risk: "read", Example: "jira zephyr resolve-url 'https://jira.example.test/projects/EFP?selectedItem=com.thed.zephyr.je%3Azephyr-tests-page#test-summary-tab' --json"},
	"zephyr.status.list":   {Description: "List configured Zephyr execution status names and legacy ids.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr status list --json"},
	"zephyr.util.test-issue-type": {Description: "Fetch Zephyr's configured Jira Test issue type metadata.",
		Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr util test-issue-type --json"},
	"zephyr.summary": {Description: "Fetch a conservative Zephyr project test summary from legacy ZAPI cycles.",
		Flags: []string{"project", "version-id", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr summary --project EFP --version-id -1 --json"},
	"zephyr.test.list": {Description: "List Jira Test issues using Jira search under the Zephyr namespace.",
		Flags: []string{"project", "jql", "limit", "start", "instance", "config", "json", "format", "verbose"}, Required: []string{"project|jql"}, Risk: "read", Example: "jira zephyr test list --project EFP --jql 'project = EFP AND issuetype = Test' --json"},
	"zephyr.test.get": {Description: "Fetch a Jira Test issue by key or URL.",
		Flags: []string{"fields", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url"}, Risk: "read", Example: "jira zephyr test get EFP-T123 --json"},
	"zephyr.test.create": {Description: "Create a Jira issue with issue type Test.",
		Flags: []string{"project", "summary", "description", "field", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"project", "summary"}, Risk: "write", Example: "jira zephyr test create --project EFP --summary 'Login rejects expired token' --dry-run --json"},
	"zephyr.cycle.list": {Description: "List Zephyr test cycles for a Jira project and version.",
		Flags: []string{"project", "project-id", "version-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"project|project-id"}, Risk: "read", Example: "jira zephyr cycle list --project EFP --version-id -1 --json"},
	"zephyr.cycle.resolve": {Description: "Resolve a Zephyr test cycle name to a deterministic cycle id.",
		Flags: []string{"name", "project", "project-id", "version-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"name"}, Risk: "read", Example: "jira zephyr cycle resolve --project EFP --name 'Sprint 42 Regression' --version-id -1 --json"},
	"zephyr.cycle.get": {Description: "Fetch a Zephyr test cycle by id.",
		Flags: []string{"project-id", "version-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"cycle-id"}, Risk: "read", Example: "jira zephyr cycle get 20000 --project-id 10000 --version-id -1 --json"},
	"zephyr.cycle.create": {Description: "Create a Zephyr test cycle.",
		Flags: []string{"project", "project-id", "version-id", "name", "description", "build", "environment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"project|project-id", "name"}, Risk: "write", Example: "jira zephyr cycle create --project EFP --version-id -1 --name 'Sprint 42 Regression' --dry-run --json"},
	"zephyr.cycle.update": {Description: "Update fields on a Zephyr test cycle.",
		Flags: []string{"name", "description", "build", "environment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"cycle-id", "name|description|build|environment"}, Risk: "write", Example: "jira zephyr cycle update 20000 --name 'Sprint 42 Regression - RC2' --dry-run --json"},
	"zephyr.cycle.delete": {Description: "Delete a Zephyr test cycle after explicit confirmation.",
		Flags: []string{"yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"cycle-id", "yes"}, Risk: "delete", Example: "jira zephyr cycle delete 20000 --yes --dry-run --json"},
	"zephyr.execution.list": {Description: "List Zephyr test executions in a cycle.",
		Flags: []string{"cycle-id", "project-id", "version-id", "status", "instance", "config", "json", "format", "verbose"}, Required: []string{"cycle-id", "project-id"}, Risk: "read", Example: "jira zephyr execution list --cycle-id 20000 --project-id 10000 --version-id -1 --status FAIL --json"},
	"zephyr.execution.resolve": {Description: "Resolve a Jira Test issue inside a Zephyr test cycle to an execution id.",
		Flags: []string{"cycle-id", "issue", "project", "project-id", "version-id", "folder-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"cycle-id", "issue"}, Risk: "read", Example: "jira zephyr execution resolve --cycle-id 20000 --issue EFP-123 --project EFP --version-id -1 --json"},
	"zephyr.execution.get": {Description: "Fetch a Zephyr test execution by id.",
		Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"execution-id"}, Risk: "read", Example: "jira zephyr execution get 30000 --json"},
	"zephyr.execution.create": {Description: "Create a Zephyr test execution for a Jira Test issue.",
		Flags: []string{"issue-id", "cycle-id", "project-id", "version-id", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-id", "cycle-id", "project-id"}, Risk: "write", Example: "jira zephyr execution create --issue-id 10001 --cycle-id 20000 --project-id 10000 --version-id -1 --dry-run --json"},
	"zephyr.execution.update-status": {Description: "Update a Zephyr execution status by execution id or by resolving --cycle-id plus --issue.",
		Flags: []string{"status", "cycle-id", "issue", "project", "project-id", "version-id", "folder-id", "comment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"status", "execution-id|cycle-id+issue"}, Risk: "write", Example: "jira zephyr execution update-status --cycle-id 20000 --issue EFP-123 --status PASSED --dry-run --json"},
	"zephyr.execution.add-tests-to-cycle": {Description: "Add Jira Test issues to a Zephyr test cycle.",
		Flags: []string{"cycle-id", "project-id", "version-id", "issues", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"cycle-id", "project-id", "issues"}, Risk: "write", Example: "jira zephyr execution add-tests-to-cycle --cycle-id 20000 --project-id 10000 --version-id -1 --issues EFP-T1,EFP-T2 --dry-run --json"},
	"zephyr.execution.count": {Description: "Count Zephyr executions grouped by cycle using conservative cycle fields.",
		Flags: []string{"project-id", "version-id", "group", "instance", "config", "json", "format", "verbose"}, Required: []string{"project-id"}, Risk: "read", Example: "jira zephyr execution count --project-id 10000 --version-id -1 --group cycle --json"},
	"zephyr.execution.delete": {Description: "Delete a Zephyr test execution after explicit confirmation.",
		Flags: []string{"yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"execution-id", "yes"}, Risk: "delete", Example: "jira zephyr execution delete 30000 --yes --dry-run --json"},
	"zephyr.execution.bulk-update-status": {Description: "Update the status of multiple Zephyr test executions.",
		Flags: []string{"execution-ids", "status", "comment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"execution-ids", "status"}, Risk: "write", Example: "jira zephyr execution bulk-update-status --execution-ids 1,2,3 --status PASS --dry-run --json"},
	"zephyr.execution.export": {Description: "Return exportable Zephyr execution query results as JSON.",
		Flags: []string{"zql", "type", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"zql"}, Risk: "read", Example: "jira zephyr execution export --zql 'executionStatus != UNEXECUTED' --type xls --json"},
	"zephyr.zql.search": {Description: "Search Zephyr executions with ZQL through legacy ZAPI.",
		Flags: []string{"query", "limit", "start", "instance", "config", "json", "format", "verbose"}, Required: []string{"query"}, Risk: "read", Example: "jira zephyr zql search --query 'executionStatus = FAIL' --limit 100 --json"},
	"zephyr.zql.clauses": {Description: "Fetch official Zephyr ZQL clause metadata.",
		Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr zql clauses --json"},
	"zephyr.zql.autocomplete-json": {Description: "Fetch official Zephyr ZQL autocomplete metadata JSON.",
		Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr zql autocomplete-json --json"},
	"zephyr.zql.autocomplete": {Description: "Fetch official Zephyr ZQL autocomplete values for a field prefix.",
		Flags: []string{"field-name", "field-value", "instance", "config", "json", "format", "verbose"}, Required: []string{"field-name"}, Risk: "read", Example: "jira zephyr zql autocomplete --field-name executionStatus --field-value PA --json"},
	"zephyr.step-result.list": {Description: "List Zephyr step results for an execution.",
		Flags: []string{"execution-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"execution-id"}, Risk: "read", Example: "jira zephyr step-result list --execution-id 30000 --json"},
	"zephyr.step-result.update-status": {Description: "Update the status of a Zephyr step result.",
		Flags: []string{"status", "comment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"step-result-id", "status"}, Risk: "write", Example: "jira zephyr step-result update-status 40000 --status PASS --dry-run --json"},
	"zephyr.attachment.list": {Description: "List Zephyr attachments for an entity.",
		Flags: []string{"entity-type", "entity-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"entity-type", "entity-id"}, Risk: "read", Example: "jira zephyr attachment list --entity-type execution --entity-id 30000 --json"},
	"zephyr.attachment.get": {Description: "Fetch Zephyr attachment metadata by id.",
		Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"attachment-id"}, Risk: "read", Example: "jira zephyr attachment get 50000 --json"},
	"zephyr.attachment.upload": {Description: "Upload a Zephyr attachment for an entity.",
		Flags: []string{"entity-type", "entity-id", "file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"entity-type", "entity-id", "file"}, Risk: "write", Example: "jira zephyr attachment upload --entity-type execution --entity-id 30000 --file ./report.png --dry-run --json"},
	"zephyr.attachment.delete": {Description: "Delete a Zephyr attachment after explicit confirmation.",
		Flags: []string{"yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"attachment-id", "yes"}, Risk: "delete", Example: "jira zephyr attachment delete 50000 --yes --dry-run --json"},
	"zephyr.folder.list": {Description: "List Zephyr folders under a test cycle.",
		Flags: []string{"cycle-id", "project-id", "version-id", "limit", "offset", "instance", "config", "json", "format", "verbose"}, Required: []string{"cycle-id", "project-id", "version-id"}, Risk: "read", Example: "jira zephyr folder list --cycle-id 20000 --project-id 10000 --version-id -1 --json"},
	"zephyr.folder.create": {Description: "Create a Zephyr folder under a test cycle.",
		Flags: []string{"cycle-id", "project-id", "version-id", "name", "description", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"cycle-id", "project-id", "version-id", "name"}, Risk: "write", Example: "jira zephyr folder create --cycle-id 20000 --project-id 10000 --version-id -1 --name Smoke --dry-run --json"},
	"zephyr.folder.update": {Description: "Update Zephyr folder metadata.",
		Flags: []string{"name", "description", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"folder-id", "name|description"}, Risk: "write", Example: "jira zephyr folder update 40000 --name 'Smoke RC2' --dry-run --json"},
	"zephyr.folder.delete": {Description: "Delete a Zephyr folder after explicit confirmation.",
		Flags: []string{"cycle-id", "project-id", "version-id", "yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"folder-id", "cycle-id", "project-id", "version-id", "yes"}, Risk: "delete", Example: "jira zephyr folder delete 40000 --cycle-id 20000 --project-id 10000 --version-id -1 --yes --dry-run --json"},
	"zephyr.teststep.list": {Description: "List Zephyr test steps for a Jira Test issue.",
		Flags: []string{"issue", "offset", "limit", "instance", "config", "json", "format", "verbose"}, Required: []string{"issue"}, Risk: "read", Example: "jira zephyr teststep list --issue EFP-123 --json"},
	"zephyr.teststep.get": {Description: "Fetch a Zephyr test step for a Jira Test issue.",
		Flags: []string{"issue", "step-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"issue", "step-id"}, Risk: "read", Example: "jira zephyr teststep get --issue EFP-123 --step-id 10 --json"},
	"zephyr.teststep.create": {Description: "Create a Zephyr test step for a Jira Test issue.",
		Flags: []string{"issue", "step", "data", "result", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue", "step"}, Risk: "write", Example: "jira zephyr teststep create --issue EFP-123 --step 'Open login page' --data 'user exists' --result 'Login page is shown' --dry-run --json"},
	"zephyr.teststep.update": {Description: "Update a Zephyr test step for a Jira Test issue.",
		Flags: []string{"issue", "step-id", "step", "data", "result", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue", "step-id", "step|data|result"}, Risk: "write", Example: "jira zephyr teststep update --issue EFP-123 --step-id 10 --step 'Open login page' --dry-run --json"},
	"zephyr.teststep.delete": {Description: "Delete a Zephyr test step after explicit confirmation.",
		Flags: []string{"issue", "step-id", "yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue", "step-id", "yes"}, Risk: "delete", Example: "jira zephyr teststep delete --issue EFP-123 --step-id 10 --yes --dry-run --json"},
	"zephyr.defect.list": {Description: "List Jira defects linked to a Zephyr execution.",
		Flags: []string{"execution-id", "instance", "config", "json", "format", "verbose"}, Required: []string{"execution-id"}, Risk: "read", Example: "jira zephyr defect list --execution-id 30000 --json"},
	"zephyr.defect.add": {Description: "Link a Jira defect issue to a Zephyr execution.",
		Flags: []string{"execution-id", "issue", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"execution-id", "issue"}, Risk: "write", Example: "jira zephyr defect add --execution-id 30000 --issue EFP-999 --dry-run --json"},
	"zephyr.report.coverage": {Description: "Build a conservative Zephyr coverage summary from cycle data.",
		Flags: []string{"project", "project-id", "version-id", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira zephyr report coverage --project EFP --version-id -1 --json"},
	"zephyr.api.get": {Description: "Call a raw Zephyr legacy ZAPI GET path on the selected Jira instance.",
		Flags: []string{"query", "instance", "config", "json", "format", "verbose"}, Required: []string{"path"}, Risk: "read", Example: "jira zephyr api get cycle --query projectId=10000 --query versionId=-1 --json"},
	"zephyr.api.catalog": {Description: "List official Zephyr Squad Server/DC ZAPI endpoint metadata without server access.",
		Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "jira zephyr api catalog --json"},
	"zephyr.api.describe": {Description: "Describe one official Zephyr Squad Server/DC ZAPI endpoint by endpoint id.",
		Flags: []string{"json", "format", "verbose"}, Required: []string{"endpoint-id"}, Risk: "read", Example: "jira zephyr api describe execution.update-status --json"},
	"zephyr.api.post": {Description: "Call a raw Zephyr legacy ZAPI POST path on the selected Jira instance.",
		Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira zephyr api post cycle --body '{}' --dry-run --json"},
	"zephyr.api.put": {Description: "Call a raw Zephyr legacy ZAPI PUT path on the selected Jira instance.",
		Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira zephyr api put execution/30000/execute --body '{\"status\":\"1\"}' --dry-run --json"},
	"zephyr.api.delete": {Description: "Call a raw Zephyr legacy ZAPI DELETE path after explicit confirmation.",
		Flags: []string{"query", "yes", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path", "yes"}, Risk: "delete", Example: "jira zephyr api delete execution/30000 --yes --dry-run --json"},
	"issue.attachment.list":     {Description: "List attachments on a Jira issue.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"issue-or-url"}, Risk: "read", Example: "jira issue attachment list PROJ-123 --json"},
	"issue.attachment.upload":   {Description: "Upload a file attachment to a Jira issue.", Flags: []string{"instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "file"}, Risk: "write", Example: "jira issue attachment upload PROJ-123 ./note.txt --json"},
	"issue.attachment.download": {Description: "Download or inspect a Jira issue attachment.", Flags: []string{"output", "instance", "config", "json", "format", "verbose", "dry-run"}, Risk: "read", Example: "jira attachment download 10000 --json"},
	"issue.link.create":         {Description: "Create a Jira issue link between two issues.", Flags: []string{"type", "from", "to", "comment", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"type", "from", "to"}, Risk: "write", Example: "jira issue link create --type Relates --from PROJ-123 --to PROJ-124 --json"},
	"issue.remote-link.add":     {Description: "Add an external remote link to a Jira issue.", Flags: []string{"url", "title", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "url", "title"}, Risk: "write", Example: "jira issue remote-link add PROJ-123 --url https://example.test/spec --title Spec --json"},
	"issue.property.set":        {Description: "Set a JSON issue property value.", Flags: []string{"value", "value-file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"issue-or-url", "key", "value|value-file"}, Risk: "write", Example: "jira issue property set PROJ-123 review.state --value '{\"ok\":true}' --json"},
	"attachment.delete":         {Description: "Delete an attachment after explicit confirmation.", Flags: []string{"yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"attachment-id", "yes"}, Risk: "delete", Example: "jira attachment delete 10000 --yes --json"},
	"project.list":              {Description: "List Jira projects.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira project list --json"},
	"project.get":               {Description: "Fetch a Jira project by key.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"project-key"}, Risk: "read", Example: "jira project get PROJ --json"},
	"field.list":                {Description: "List Jira fields.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "jira field list --json"},
	"api.get":                   {Description: "Call a raw REST GET path on the selected instance.", Flags: []string{"query", "instance", "config", "json", "format", "verbose"}, Required: []string{"path"}, Risk: "read", Example: "jira api get /rest/api/2/myself --json"},
	"api.post":                  {Description: "Call a raw REST POST path on the selected instance.", Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira api post /rest/api/2/issue --body '{}' --json"},
	"api.put":                   {Description: "Call a raw REST PUT path on the selected instance.", Flags: []string{"query", "body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"path"}, Risk: "write", Example: "jira api put /rest/api/2/issue/PROJ-123 --body '{}' --json"},
	"api.delete":                {Description: "Call a raw REST DELETE path after explicit confirmation.", Flags: []string{"query", "yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"path", "yes"}, Risk: "delete", Example: "jira api delete /rest/api/2/issue/PROJ-123 --yes --json"},
	"search":                    {Description: "Search Confluence content with CQL.", Flags: []string{"cql", "limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"cql"}, Risk: "read", Example: "confluence search --cql \"space = ENG\" --json"},
	"cql":                       {Description: "Search Confluence content with a CQL query alias.", Flags: []string{"query", "limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"query"}, Risk: "read", Example: "confluence cql --query \"space = ENG\" --json"},
	"search.content":            {Description: "Build and run a Confluence content search query.", Flags: []string{"text", "space", "type", "limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "confluence search content --space ENG --type page --json"},
	"search.user":               {Description: "Search Confluence users.", Flags: []string{"query", "instance", "config", "json", "format", "verbose"}, Required: []string{"query"}, Risk: "read", Example: "confluence search user --query alice --json"},
	"space.list":                {Description: "List Confluence spaces.", Flags: []string{"limit", "start", "type", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "confluence space list --json"},
	"space.get":                 {Description: "Fetch a Confluence space by key.", Flags: []string{"instance", "config", "json", "format", "verbose"}, Required: []string{"space-key"}, Risk: "read", Example: "confluence space get ENG --json"},
	"space.pages":               {Description: "List pages in a Confluence space.", Flags: []string{"limit", "start", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"space-key"}, Risk: "read", Example: "confluence space pages ENG --json"},
	"page.get":                  {Description: "Fetch a Confluence page by id or full URL.", Flags: []string{"id", "url", "expand", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page get --id 123 --json"},
	"page.get-by-title":         {Description: "Fetch a Confluence page by space and title.", Flags: []string{"space", "title", "expand", "limit", "instance", "config", "json", "format", "verbose"}, Required: []string{"space", "title"}, Risk: "read", Example: "confluence page get-by-title --space ENG --title Home --json"},
	"page.create":               {Description: "Create a Confluence page.", Flags: []string{"space", "title", "parent-id", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"space", "title", "body|body-file|body-stdin"}, Risk: "write", Example: "confluence page create --space ENG --title Home --body '<p>Hello</p>' --json"},
	"page.update":               {Description: "Update a Confluence page.", Flags: []string{"id", "url", "title", "version", "minor-edit", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url"}, Risk: "write", Example: "confluence page update --id 123 --title Home --json"},
	"page.delete":               {Description: "Delete a Confluence page after explicit confirmation.", Flags: []string{"id", "url", "yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url", "yes"}, Risk: "delete", Example: "confluence page delete --id 123 --yes --json"},
	"page.body":                 {Description: "Fetch a Confluence page body representation.", Flags: []string{"id", "url", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page body --id 123 --json"},
	"page.export-markdown":      {Description: "Export a Confluence page body as Markdown.", Flags: []string{"id", "url", "output", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page export-markdown --id 123 --json"},
	"page.attachment.list":      {Description: "List attachments on a Confluence page.", Flags: []string{"page-id", "id", "url", "instance", "config", "json", "format", "verbose"}, Required: []string{"page-id|id|url"}, Risk: "read", Example: "confluence page attachment list --id 123 --json"},
	"page.attachment.upload":    {Description: "Upload an attachment to a Confluence page.", Flags: []string{"page-id", "id", "url", "file", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"page-id|id|url", "file"}, Risk: "write", Example: "confluence page attachment upload --id 123 --file ./note.txt --json"},
	"page.attachment.download":  {Description: "Download or inspect a Confluence page attachment.", Flags: []string{"output", "instance", "config", "json", "format", "verbose"}, Risk: "read", Example: "confluence attachment download 10000 --json"},
	"page.comment.list":         {Description: "List comments on a Confluence page.", Flags: []string{"id", "url", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page comment list --id 123 --json"},
	"page.comment.add":          {Description: "Add a comment to a Confluence page.", Flags: []string{"id", "url", "body", "body-file", "body-stdin", "body-format", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url", "body|body-file|body-stdin"}, Risk: "write", Example: "confluence page comment add --id 123 --body ok --json"},
	"comment.update":            {Description: "Update a Confluence comment.", Flags: []string{"body", "body-file", "body-stdin", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"comment-id", "body|body-file|body-stdin"}, Risk: "write", Example: "confluence comment update 10000 --body ok --json"},
	"comment.delete":            {Description: "Delete a Confluence comment after explicit confirmation.", Flags: []string{"yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"comment-id", "yes"}, Risk: "delete", Example: "confluence comment delete 10000 --yes --json"},
	"page.label.list":           {Description: "List labels on a Confluence page.", Flags: []string{"id", "url", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url"}, Risk: "read", Example: "confluence page label list --id 123 --json"},
	"page.label.add":            {Description: "Add labels to a Confluence page.", Flags: []string{"id", "url", "label", "instance", "config", "json", "format", "verbose", "dry-run"}, Required: []string{"id|url", "label"}, Risk: "write", Example: "confluence page label add --id 123 --label runbook --json"},
	"page.label.delete":         {Description: "Remove a label from a Confluence page after explicit confirmation.", Flags: []string{"id", "url", "label", "yes", "instance", "config", "json", "format", "verbose"}, Required: []string{"id|url", "label", "yes"}, Risk: "delete", Example: "confluence page label delete --id 123 --label runbook --yes --json"},
}

func SortedUsages(product string) []string {
	out := CommandList(product)
	sort.Strings(out)
	return out
}
