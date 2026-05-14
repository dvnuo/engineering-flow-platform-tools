package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestJSONContractSmoke(t *testing.T) {
	checks := []struct {
		root string
		args []string
	}{
		{"jira", []string{"commands", "--json"}},
		{"jira", []string{"schema", "issue.create", "--json"}},
		{"jira", []string{"resolve-url", "https://example.atlassian.net/browse/EFP-1", "--json"}},
		{"confluence", []string{"commands", "--json"}},
		{"confluence", []string{"schema", "page.create", "--json"}},
		{"confluence", []string{"resolve-url", "https://example.atlassian.net/wiki/spaces/ENG/pages/1", "--json"}},
	}
	for _, c := range checks {
		var b bytes.Buffer
		if c.root == "jira" {
			cmd := jcmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		} else {
			cmd := ccmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		}
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
