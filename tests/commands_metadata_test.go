package tests

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	bcmd "engineering-flow-platform-tools/internal/browser/commands"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	icmd "engineering-flow-platform-tools/internal/inspectimage/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
)

func TestCommandsMetadataComplete(t *testing.T) {
	placeholder := regexp.MustCompile(`<(?:issue-or-url|jira-url|comment-id|attachment-id|worklog-id|link-id|project-key|project-id|issue-id|cycle-id|execution-id|component-id|version-id|group-name|filter-id|dashboard-id|board-id|sprint-id|space-key|content-id|blog-id-or-url|task-id|webhook-id|role-id-or-name|name|key|url|command|path|file)>|\[name\]`)
	for name, root := range map[string]*cobra.Command{"jira": jcmd.NewRoot(), "confluence": ccmd.NewRoot(), "browser": bcmd.NewRoot(), "inspect-image": icmd.NewRoot()} {
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
				desc := m["description"].(string)
				if strings.HasPrefix(desc, "Run ") || strings.HasPrefix(desc, "Execute ") {
					t.Fatalf("generic description: %#v", m)
				}
				if examples, _ := m["examples"].([]any); len(examples) == 0 {
					t.Fatalf("missing examples: %#v", m)
				} else {
					for _, ex := range examples {
						s, _ := ex.(string)
						if placeholder.MatchString(s) {
							t.Fatalf("placeholder example: %#v", m)
						}
					}
				}
				if flags, _ := m["flags"].([]any); len(flags) == 0 {
					t.Fatalf("missing flags: %#v", m)
				}
			}
		})
	}
}
