package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	bcmd "engineering-flow-platform-tools/internal/browser/commands"
	"engineering-flow-platform-tools/internal/clihelp"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	icmd "engineering-flow-platform-tools/internal/inspectimage/commands"
	kcmd "engineering-flow-platform-tools/internal/jenkins/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	lcmd "engineering-flow-platform-tools/internal/logtool/commands"
	"github.com/spf13/cobra"
)

func TestExecuteFallbackJSONForAllBinaries(t *testing.T) {
	for name, root := range map[string]*cobra.Command{
		"browser":       bcmd.NewRoot(),
		"confluence":    ccmd.NewRoot(),
		"inspect-image": icmd.NewRoot(),
		"jenkins":       kcmd.NewRoot(),
		"jira":          jcmd.NewRoot(),
		"log":           lcmd.NewRoot(),
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := clihelp.Execute(root, name, []string{"version", "--bad-flag", "--json"}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			var out map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
				t.Fatalf("invalid json: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
			}
			errObj := out["error"].(map[string]any)
			if errObj["code"] != "invalid_args" || !strings.Contains(errObj["hint"].(string), name+" commands --json") {
				t.Fatalf("bad fallback envelope: %#v", out)
			}
		})
	}
}

func TestExecuteBadFormatFallbackForAllBinaries(t *testing.T) {
	for name, root := range map[string]*cobra.Command{
		"browser":       bcmd.NewRoot(),
		"confluence":    ccmd.NewRoot(),
		"inspect-image": icmd.NewRoot(),
		"jenkins":       kcmd.NewRoot(),
		"jira":          jcmd.NewRoot(),
		"log":           lcmd.NewRoot(),
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := clihelp.Execute(root, name, []string{"version", "--format", "xml"}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), "invalid_args") || !strings.Contains(stdout.String(), "unknown_output_format") {
				t.Fatalf("bad fallback output: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}
