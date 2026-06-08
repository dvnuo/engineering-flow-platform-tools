package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	bcmd "engineering-flow-platform-tools/internal/browser/commands"
	"engineering-flow-platform-tools/internal/catalog"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	kcmd "engineering-flow-platform-tools/internal/jenkins/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func schemaData(t *testing.T, product, command string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	var c *cobra.Command
	switch product {
	case "jira":
		c = jcmd.NewRoot()
	case "confluence":
		c = ccmd.NewRoot()
	case "browser":
		c = bcmd.NewRoot()
	case "jenkins":
		c = kcmd.NewRoot()
	default:
		t.Fatalf("unknown product %s", product)
	}
	c.SetOut(&b)
	c.SetErr(&b)
	c.SetArgs([]string{"schema", command, "--json"})
	_ = c.Execute()
	obj := testutil.AssertOKEnvelope(t, b.Bytes())
	data, _ := obj["data"].(map[string]any)
	return data
}

func requireFlags(t *testing.T, data map[string]any, names ...string) {
	t.Helper()
	have := map[string]bool{}
	flags, _ := data["flags"].([]any)
	for _, f := range flags {
		m, _ := f.(map[string]any)
		name, _ := m["name"].(string)
		have[name] = true
	}
	for _, n := range names {
		if !have[n] {
			b, _ := json.Marshal(data)
			t.Fatalf("missing flag %s in %s", n, string(b))
		}
	}
}

func requireRequired(t *testing.T, data map[string]any, names ...string) {
	t.Helper()
	have := map[string]bool{}
	required, _ := data["required"].([]any)
	for _, r := range required {
		name, _ := r.(string)
		for _, part := range strings.Split(name, "|") {
			have[strings.TrimSpace(part)] = true
		}
	}
	for _, n := range names {
		if !have[n] {
			b, _ := json.Marshal(data)
			t.Fatalf("missing required %s in %s", n, string(b))
		}
	}
}

func schemaFlagMap(data map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	switch flags := data["flags"].(type) {
	case []any:
		for _, raw := range flags {
			flag := raw.(map[string]any)
			name, _ := flag["name"].(string)
			out[name] = flag
		}
	case []catalog.FlagSpec:
		for _, flag := range flags {
			out[flag.Name] = map[string]any{
				"name":        flag.Name,
				"type":        flag.Type,
				"description": flag.Description,
				"required":    flag.Required,
			}
		}
	}
	return out
}

func TestSchemaConcreteFlags(t *testing.T) {
	requireFlags(t, schemaData(t, "jira", "issue.create"), "project", "type", "summary", "description", "field", "json-body", "json-body-file", "dry-run")
	requireFlags(t, schemaData(t, "jira", "issue.transition"), "to", "transition-id", "comment", "field")
	mapCSV := schemaData(t, "jira", "issue.map-csv")
	requireFlags(t, mapCSV, "from-csv", "template-issue", "metadata-mode", "output", "sample-rows", "min-confidence", "include-template-defaults")
	requireRequired(t, mapCSV, "from-csv", "template-issue")
	bulkCreate := schemaData(t, "jira", "issue.bulk-create")
	requireFlags(t, bulkCreate, "from-csv", "mapping", "metadata-mode", "dry-run", "yes", "max-create", "fail-fast", "confirm-mapping", "apply-post-create-updates")
	requireRequired(t, bulkCreate, "from-csv")
	if bulkCreate["risk"].(string) != "write_requires_confirmation" {
		t.Fatalf("bulk-create risk = %s", bulkCreate["risk"].(string))
	}
	requireFlags(t, schemaData(t, "jira", "issue.link.create"), "type", "from", "to", "comment")
	requireRequired(t, schemaData(t, "jira", "issue.property.set"), "value", "value-file")
	requireFlags(t, schemaData(t, "jira", "issue.comment.add"), "body", "body-file", "body-stdin")
	requireFlags(t, schemaData(t, "jira", "api.get"), "query", "json", "instance", "config")
	requireFlags(t, schemaData(t, "jira", "zephyr.execution.update-status"), "status", "dry-run")
	requireFlags(t, schemaData(t, "jira", "zephyr.cycle.create"), "project", "project-id", "version-id", "name", "dry-run")
	requireFlags(t, schemaData(t, "jira", "zephyr.api.get"), "query")
	requireFlags(t, schemaData(t, "confluence", "page.create"), "space", "title", "parent-id", "body", "body-file", "body-stdin", "body-format", "dry-run")
	requireFlags(t, schemaData(t, "confluence", "page.update"), "id", "url", "title", "version", "minor-edit", "body", "body-file", "body-stdin")
	requireRequired(t, schemaData(t, "confluence", "page.get-by-title"), "space", "title")
	requireFlags(t, schemaData(t, "confluence", "search"), "cql", "limit", "start", "expand")
	requireFlags(t, schemaData(t, "browser", "probe"), "url", "selector", "wait", "timeout", "out", "browser", "json")
	requireFlags(t, schemaData(t, "browser", "session.start"), "name", "url", "profile", "download-dir", "port", "browser", "headless", "json")
	requireFlags(t, schemaData(t, "browser", "session.attach"), "name", "debug-addr", "debug-port", "json")
	requireRequired(t, schemaData(t, "browser", "session.attach"), "debug-port")
	requireFlags(t, schemaData(t, "browser", "session.discover"), "debug-addr", "ports", "json")
	requireFlags(t, schemaData(t, "browser", "tab.open"), "session", "url", "json")
	requireRequired(t, schemaData(t, "browser", "tab.open"), "url")
	requireFlags(t, schemaData(t, "browser", "tab.activate"), "session", "target-id", "json")
	requireRequired(t, schemaData(t, "browser", "tab.activate"), "target-id")
	requireFlags(t, schemaData(t, "browser", "page.snapshot"), "session", "target-id", "timeout", "include-html", "max-text-bytes", "max-html-bytes", "json")
	requireFlags(t, schemaData(t, "browser", "page.extract"), "selector", "limit", "include-html", "pierce", "max-html-bytes", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.extract"), "selector")
	requireFlags(t, schemaData(t, "browser", "page.extract-schema"), "file", "limit", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.extract-schema"), "file")
	requireFlags(t, schemaData(t, "browser", "page.ax"), "limit", "include-hidden", "pierce", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.click"), "selector", "ref", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.click"), "selector", "ref")
	requireFlags(t, schemaData(t, "browser", "page.type"), "selector", "ref", "text", "clear", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.type"), "selector", "ref", "text")
	requireFlags(t, schemaData(t, "browser", "page.select"), "selector", "ref", "value", "label", "index", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.select"), "selector", "ref", "value", "label", "index")
	requireFlags(t, schemaData(t, "browser", "page.check"), "selector", "ref", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.check"), "selector", "ref")
	requireFlags(t, schemaData(t, "browser", "page.uncheck"), "selector", "ref", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.uncheck"), "selector", "ref")
	requireFlags(t, schemaData(t, "browser", "page.press"), "selector", "ref", "key", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.press"), "key")
	requireFlags(t, schemaData(t, "browser", "page.upload"), "selector", "file", "clear", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.upload"), "selector", "file", "clear")
	requireFlags(t, schemaData(t, "browser", "page.wait"), "selector", "duration-ms", "url-contains", "text", "network-idle-ms", "dom-stable-ms", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.wait"), "selector", "duration-ms", "url-contains", "text", "network-idle-ms", "dom-stable-ms")
	requireFlags(t, schemaData(t, "browser", "page.screenshot"), "out", "full-page", "selector", "ref", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.eval"), "expr", "max-string-bytes", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.eval"), "expr")
	requireFlags(t, schemaData(t, "browser", "page.fetch"), "url", "max-body-bytes", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "page.fetch"), "url")
	requireFlags(t, schemaData(t, "browser", "page.console"), "level", "limit", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.errors"), "limit", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.console-clear"), "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.network"), "filter", "limit", "all", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.metrics"), "limit-resources", "filter", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.outline"), "limit", "include-hidden", "pierce", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.table"), "selector", "limit-rows", "limit-cells", "include-html", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "page.list"), "selector", "limit-items", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "assert.visible"), "selector", "ref", "not", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "assert.visible"), "selector", "ref")
	requireFlags(t, schemaData(t, "browser", "assert.text"), "contains", "selector", "ref", "not", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "assert.text"), "contains")
	requireFlags(t, schemaData(t, "browser", "assert.url"), "contains", "not", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "assert.url"), "contains")
	requireFlags(t, schemaData(t, "browser", "assert.count"), "selector", "equals", "min", "max", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "assert.count"), "selector", "equals", "min", "max")
	requireFlags(t, schemaData(t, "browser", "assert.screenshot"), "baseline", "out", "diff-out", "selector", "ref", "threshold", "full-page", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "assert.screenshot"), "baseline", "diff-out")
	requireFlags(t, schemaData(t, "browser", "workflow.run"), "file", "dry-run", "session", "target-id", "timeout", "continue-on-error", "var", "report-out", "allow-human", "yes")
	requireRequired(t, schemaData(t, "browser", "workflow.run"), "file")
	requireFlags(t, schemaData(t, "browser", "workflow.record"), "out", "duration-ms", "limit", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "workflow.record"), "out")
	requireFlags(t, schemaData(t, "browser", "form.inspect"), "selector", "limit", "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "form.fill"), "file", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "form.fill"), "file")
	requireFlags(t, schemaData(t, "browser", "frame.list"), "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "frame.snapshot"), "frame-id", "include-html", "max-text-bytes", "max-html-bytes", "session", "target-id", "timeout")
	requireRequired(t, schemaData(t, "browser", "frame.snapshot"), "frame-id")
	requireFlags(t, schemaData(t, "browser", "network.start"), "session", "target-id", "timeout", "limit", "filter")
	requireFlags(t, schemaData(t, "browser", "network.stop"), "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "network.list"), "session", "target-id", "timeout", "filter", "limit", "method", "status")
	requireFlags(t, schemaData(t, "browser", "network.wait"), "session", "target-id", "timeout", "url-contains", "method", "status", "limit")
	requireRequired(t, schemaData(t, "browser", "network.wait"), "url-contains")
	requireFlags(t, schemaData(t, "browser", "network.export"), "session", "target-id", "timeout", "out", "format", "filter", "limit")
	requireRequired(t, schemaData(t, "browser", "network.export"), "out")
	requireFlags(t, schemaData(t, "browser", "network.clear"), "session", "target-id", "timeout")
	requireFlags(t, schemaData(t, "browser", "download.list"), "session")
	requireFlags(t, schemaData(t, "browser", "download.wait"), "session", "filename-contains", "timeout")
	requireFlags(t, schemaData(t, "jenkins", "job.build-with-params"), "param", "delay", "dry-run", "json", "instance", "config")
	requireFlags(t, schemaData(t, "jenkins", "build.log-follow"), "start", "max-rounds", "wait-ms")
	requireFlags(t, schemaData(t, "jenkins", "artifact.download"), "output")
	requireRequired(t, schemaData(t, "jenkins", "api.delete"), "path", "yes")
}

func TestSchemaMatchesCobraFlags(t *testing.T) {
	roots := map[string]func() *cobra.Command{
		"jira":       jcmd.NewRoot,
		"confluence": ccmd.NewRoot,
		"browser":    bcmd.NewRoot,
		"jenkins":    kcmd.NewRoot,
	}
	for product, newRoot := range roots {
		t.Run(product, func(t *testing.T) {
			root := newRoot()
			for _, binding := range commandBindings(root) {
				data, ok := catalog.SchemaFromCobra(product, binding.name, root)
				if !ok {
					t.Fatalf("missing schema for %s", binding.name)
				}
				schemaFlags := schemaFlagMap(data)
				visitRealFlags(binding.cmd, func(flag *pflag.Flag) {
					got, ok := schemaFlags[flag.Name]
					if !ok {
						b, _ := json.Marshal(data)
						t.Fatalf("%s missing real cobra flag --%s in schema %s", binding.usage, flag.Name, string(b))
					}
					desc, _ := got["description"].(string)
					if strings.TrimSpace(desc) == "" || desc == "Command option." || desc == "Request body source." {
						t.Fatalf("%s flag --%s has unclear description %q", binding.usage, flag.Name, desc)
					}
					if got["type"].(string) != normalizePFlagType(flag.Value.Type()) {
						t.Fatalf("%s flag --%s type=%s want %s", binding.usage, flag.Name, got["type"].(string), normalizePFlagType(flag.Value.Type()))
					}
				})
			}
		})
	}
}

type commandBinding struct {
	cmd   *cobra.Command
	name  string
	usage string
}

func commandBindings(root *cobra.Command) []commandBinding {
	var out []commandBinding
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, parent []string) {
		for _, child := range cmd.Commands() {
			if !testVisibleCommand(child) {
				continue
			}
			parts := append(append([]string{}, parent...), strings.Fields(child.Use)...)
			visibleChildren := 0
			for _, grand := range child.Commands() {
				if testVisibleCommand(grand) {
					visibleChildren++
				}
			}
			usage := strings.Join(parts, " ")
			if visibleChildren == 0 || child.RunE != nil || child.Run != nil {
				out = append(out, commandBinding{cmd: child, name: dottedName(usage), usage: usage})
			}
			walk(child, parts)
		}
	}
	walk(root, []string{root.Use})
	return out
}

func testVisibleCommand(cmd *cobra.Command) bool {
	if cmd.Hidden || cmd.Name() == "completion" {
		return false
	}
	if cmd.Name() == "help" && strings.TrimSpace(cmd.Use) != "help llm" {
		return false
	}
	return true
}

func dottedName(usage string) string {
	var clean []string
	for _, part := range strings.Fields(usage)[1:] {
		if strings.HasPrefix(part, "<") || strings.HasPrefix(part, "[") {
			continue
		}
		clean = append(clean, part)
	}
	return strings.Join(clean, ".")
}

func visitRealFlags(cmd *cobra.Command, fn func(*pflag.Flag)) {
	seen := map[string]bool{}
	visit := func(flags *pflag.FlagSet) {
		if flags == nil {
			return
		}
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Name == "help" || seen[flag.Name] {
				return
			}
			seen[flag.Name] = true
			fn(flag)
		})
	}
	visit(cmd.NonInheritedFlags())
	visit(cmd.InheritedFlags())
}

func normalizePFlagType(typ string) string {
	switch typ {
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
