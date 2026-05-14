package tests

import (
	"bytes"
	"strings"
	"testing"

	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
)

func TestCommandsMetadataComplete(t *testing.T) {
	for name, root := range map[string]*cobra.Command{"jira": jcmd.NewRoot(), "confluence": ccmd.NewRoot()} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			root.SetOut(&b)
			root.SetErr(&b)
			root.SetArgs([]string{"commands", "--json"})
			_ = root.Execute()
			obj := testutil.AssertOKEnvelope(t, b.Bytes())
			data, _ := obj["data"].(map[string]any)
			commands, _ := data["commands"].([]any)
			if len(commands) == 0 {
				t.Fatal("missing commands")
			}
			for _, item := range commands {
				m, _ := item.(map[string]any)
				for _, k := range []string{"name", "usage", "risk", "description"} {
					if strings.TrimSpace(m[k].(string)) == "" {
						t.Fatalf("missing %s in %#v", k, m)
					}
				}
				if strings.HasPrefix(m["description"].(string), "Run ") {
					t.Fatalf("generic description: %#v", m)
				}
				if examples, _ := m["examples"].([]any); len(examples) == 0 {
					t.Fatalf("missing examples: %#v", m)
				}
				if flags, _ := m["flags"].([]any); len(flags) == 0 {
					t.Fatalf("missing flags: %#v", m)
				}
			}
		})
	}
}
