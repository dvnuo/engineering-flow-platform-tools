package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	bcmd "engineering-flow-platform-tools/internal/browser/commands"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
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
}

func TestSchemaMatchesCobraFlags(t *testing.T) {
	roots := map[string]func() *cobra.Command{
		"jira":       jcmd.NewRoot,
		"confluence": ccmd.NewRoot,
		"browser":    bcmd.NewRoot,
	}
	for product, newRoot := range roots {
		t.Run(product, func(t *testing.T) {
			for _, binding := range commandBindings(newRoot()) {
				data := schemaData(t, product, binding.name)
				schemaFlags := map[string]map[string]any{}
				for _, raw := range data["flags"].([]any) {
					flag := raw.(map[string]any)
					schemaFlags[flag["name"].(string)] = flag
				}
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
