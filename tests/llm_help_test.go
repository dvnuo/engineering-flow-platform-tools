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

func TestLLMHelpTips(t *testing.T) {
	required := []string{
		"For agents, --json is the default way to use every",
		"Always add --json",
		"Use --instance when multiple instances are configured.",
		"Full Jira/Confluence URLs can auto-select the instance.",
		"Use --dry-run before write operations.",
		"Use --yes for destructive operations.",
		"Inspect error.code and error.hint before retrying.",
		"Command parsing failures",
		"On Windows cmd",
	}
	for name, root := range map[string]*cobra.Command{"jira": jcmd.NewRoot(), "confluence": ccmd.NewRoot()} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			root.SetOut(&b)
			root.SetErr(&b)
			root.SetArgs([]string{"help", "llm", "--json"})
			_ = root.Execute()
			testutil.AssertOKEnvelope(t, b.Bytes())
			out := b.String()
			for _, sentence := range required {
				if !strings.Contains(out, sentence) {
					t.Fatalf("missing %q in %s", sentence, out)
				}
			}
			if name == "jira" {
				for _, sentence := range []string{
					"selectedItem=com.thed.zephyr.je",
					"jira zephyr doctor --project",
					"Use jira zephyr commands for test cycles, executions, execution status",
				} {
					if !strings.Contains(out, sentence) {
						t.Fatalf("missing Jira Zephyr guidance %q in %s", sentence, out)
					}
				}
			}
		})
	}
}
