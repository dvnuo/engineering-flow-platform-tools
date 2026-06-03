package tests

import (
	"bytes"
	"encoding/json"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	kcmd "engineering-flow-platform-tools/internal/jenkins/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	lcmd "engineering-flow-platform-tools/internal/logtool/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestJSONContractSmoke(t *testing.T) {
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
		{"log", []string{"commands", "--json"}},
		{"log", []string{"help", "llm", "--json"}},
		{"log", []string{"schema", "analyze", "--json"}},
	}
	for _, c := range checks {
		var b bytes.Buffer
		if c.root == "jira" {
			cmd := jcmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		} else if c.root == "confluence" {
			cmd := ccmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		} else if c.root == "jenkins" {
			cmd := kcmd.NewRoot()
			cmd.SetOut(&b)
			cmd.SetErr(&b)
			cmd.SetArgs(c.args)
			_ = cmd.Execute()
		} else {
			cmd := lcmd.NewRoot()
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
