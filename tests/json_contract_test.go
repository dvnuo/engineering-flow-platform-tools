package tests

import (
	"bytes"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"testing"
)

func run(rootCmd func() interface {
	Execute() error
	SetArgs([]string)
	SetOut(*bytes.Buffer)
}, args ...string) []byte { return nil }

func TestJSONContractSmoke(t *testing.T) {
	checks := []struct {
		root string
		args []string
	}{
		{"jira", []string{"commands", "--json"}},
		{"jira", []string{"schema", "issue.create", "--json"}},
		{"confluence", []string{"commands", "--json"}},
		{"confluence", []string{"schema", "page.create", "--json"}},
	}
	for _, c := range checks {
		var b bytes.Buffer
		if c.root == "jira" {
			cmd := jcmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		} else {
			cmd := ccmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		}
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
}
