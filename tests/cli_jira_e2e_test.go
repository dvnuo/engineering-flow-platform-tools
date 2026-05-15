package tests

import (
	"bytes"
	"testing"

	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestJiraE2ESmoke(t *testing.T) {
	s := testutil.NewMockJira()
	defer s.Close()
	cfg, err := testutil.WriteConfig(testutil.JiraConfig(s.URL))
	if err != nil {
		t.Fatal(err)
	}

	cases := [][]string{
		{"--config", cfg, "auth", "test", "--json"},
		{"--config", cfg, "server-info", "--json"},
		{"--config", cfg, "issue", "get", "EFP-123", "--json"},
		{"--config", cfg, "issue", "search", "--jql", "project = EFP", "--json"},
		{"--config", cfg, "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "Test", "--dry-run", "--json"},
	}
	for _, args := range cases {
		var b bytes.Buffer
		c := jcmd.NewRoot()
		c.SetOut(&b)
		c.SetErr(&b)
		c.SetArgs(args)
		_ = c.Execute()
		testutil.AssertOKEnvelope(t, b.Bytes())
	}
}
