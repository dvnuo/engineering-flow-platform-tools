package tests

import (
	"bytes"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestResolveURLJSON(t *testing.T) {
	s := testutil.NewMockJira()
	defer s.Close()
	cfg, _ := testutil.WriteConfig(testutil.JiraConfig(s.URL))
	{
		var b bytes.Buffer
		c := jcmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfg, "resolve-url", s.URL + "/browse/EFP-123", "--json"})
		_ = c.Execute()
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
	{
		var b bytes.Buffer
		c := ccmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfg, "resolve-url", s.URL + "/wiki/spaces/ENG/pages/1", "--json"})
		_ = c.Execute()
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
}
