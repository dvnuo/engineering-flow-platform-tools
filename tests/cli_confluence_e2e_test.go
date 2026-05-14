package tests

import (
	"bytes"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestConfluenceE2ESmoke(t *testing.T) {
	s := testutil.NewMockConfluence()
	defer s.Close()
	cfg, err := testutil.WriteConfig(testutil.JiraConfig(s.URL))
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"--config", cfg, "auth", "test", "--json"},
		{"--config", cfg, "server-info", "--json"},
		{"--config", cfg, "search", "--cql", "space = ENG", "--json"},
		{"--config", cfg, "page", "get-by-title", "--space", "ENG", "--title", "Runtime Profile", "--json"},
		{"--config", cfg, "page", "create", "--space", "ENG", "--title", "Test", "--body", "<p>Hello</p>", "--dry-run", "--json"},
	}
	for _, args := range cases {
		var b bytes.Buffer
		c := ccmd.NewRoot()
		c.SetOut(&b)
		c.SetErr(&b)
		c.SetArgs(args)
		_ = c.Execute()
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
}
