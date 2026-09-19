package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	appdcmd "engineering-flow-platform-tools/internal/appd/commands"
	acmd "engineering-flow-platform-tools/internal/awsauth/commands"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	kcmd "engineering-flow-platform-tools/internal/jenkins/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	nexuscmd "engineering-flow-platform-tools/internal/nexus/commands"
	pgsqlcmd "engineering-flow-platform-tools/internal/pgsql/commands"
	splunkcmd "engineering-flow-platform-tools/internal/splunk/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
)

func TestJSONContractSmoke(t *testing.T) {
	roots := map[string]func() *cobra.Command{
		"jira":       jcmd.NewRoot,
		"confluence": ccmd.NewRoot,
		"jenkins":    kcmd.NewRoot,
		"aws-auth":   acmd.NewRoot,
		"nexus":      nexuscmd.NewRoot,
		"splunk":     splunkcmd.NewRoot,
		"appd":       appdcmd.NewRoot,
		"pgsql":      pgsqlcmd.NewRoot,
	}
	checks := []struct {
		root string
		args []string
	}{
		{"jira", []string{"commands", "--json"}},
		{"jira", []string{"help", "llm", "--json"}},
		{"jira", []string{"schema", "issue.create", "--json"}},
		{"confluence", []string{"commands", "--json"}},
		{"confluence", []string{"help", "llm", "--json"}},
		{"confluence", []string{"schema", "page.create", "--json"}},
		{"jenkins", []string{"commands", "--json"}},
		{"jenkins", []string{"help", "llm", "--json"}},
		{"jenkins", []string{"schema", "job.build", "--json"}},
		{"aws-auth", []string{"commands", "--json"}},
		{"aws-auth", []string{"help", "llm", "--json"}},
		{"aws-auth", []string{"schema", "login", "--json"}},
		{"nexus", []string{"commands", "--json"}},
		{"nexus", []string{"help", "llm", "--json"}},
		{"nexus", []string{"schema", "version", "--json"}},
		{"splunk", []string{"commands", "--json"}},
		{"splunk", []string{"help", "llm", "--json"}},
		{"splunk", []string{"schema", "version", "--json"}},
		{"appd", []string{"commands", "--json"}},
		{"appd", []string{"help", "llm", "--json"}},
		{"appd", []string{"schema", "version", "--json"}},
		{"pgsql", []string{"commands", "--json"}},
		{"pgsql", []string{"help", "llm", "--json"}},
		{"pgsql", []string{"schema", "version", "--json"}},
	}
	for _, c := range checks {
		var b bytes.Buffer
		cmd := roots[c.root]()
		cmd.SetOut(&b)
		cmd.SetErr(&b)
		cmd.SetArgs(c.args)
		_ = cmd.Execute()
		obj := testutil.AssertJSONEnvelope(t, b.Bytes())
		if _, ok := obj["ok"]; !ok {
			t.Fatal("missing ok")
		}
		var re map[string]any
		if err := json.Unmarshal(b.Bytes(), &re); err != nil {
			t.Fatalf("non pure json output: %v", err)
		}
	}
}
