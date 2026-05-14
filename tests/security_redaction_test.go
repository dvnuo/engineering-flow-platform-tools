package tests

import (
	"bytes"
	"strings"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func assertNoSecrets(t *testing.T, s string) {
	t.Helper()
	for _, sec := range testutil.Secrets {
		if strings.Contains(s, sec) {
			t.Fatalf("secret leaked: %s", sec)
		}
	}
}

func TestNoSecretsInStdoutStderr(t *testing.T) {
	var b bytes.Buffer
	j := jcmd.NewRoot()
	j.SetOut(&b)
	j.SetErr(&b)
	j.SetArgs([]string{"auth", "test", "--json"})
	_ = j.Execute()
	assertNoSecrets(t, b.String())
	b.Reset()
	c := ccmd.NewRoot()
	c.SetOut(&b)
	c.SetErr(&b)
	c.SetArgs([]string{"auth", "test", "--json"})
	_ = c.Execute()
	assertNoSecrets(t, b.String())
}
