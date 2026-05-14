package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
)

func schemaData(t *testing.T, product, command string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	var c *cobra.Command
	if product == "jira" {
		c = jcmd.NewRoot()
	} else {
		c = ccmd.NewRoot()
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

func TestSchemaConcreteFlags(t *testing.T) {
	requireFlags(t, schemaData(t, "jira", "issue.create"), "project", "type", "summary")
	requireFlags(t, schemaData(t, "jira", "issue.transition"), "to", "transition-id")
	requireFlags(t, schemaData(t, "jira", "issue.comment.add"), "body", "body-file", "body-stdin")
	requireFlags(t, schemaData(t, "confluence", "page.create"), "space", "title", "body", "body-file", "body-stdin")
	requireFlags(t, schemaData(t, "confluence", "page.update"), "id", "url", "version", "body", "body-file", "body-stdin")
	requireFlags(t, schemaData(t, "confluence", "search"), "cql", "limit", "start", "expand")
}
