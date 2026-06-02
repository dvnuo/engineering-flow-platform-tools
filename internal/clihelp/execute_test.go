package clihelp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecutePrintsJSONForCobraParseError(t *testing.T) {
	root := &cobra.Command{Use: "tool", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(&cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error { return nil }})
	var stdout, stderr bytes.Buffer
	code := Execute(root, "tool", []string{"run", "--unknown", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, stdout.String())
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "unknown flag") {
		t.Fatalf("bad error envelope: %#v", out)
	}
}

func TestExecuteFallsBackForUnknownFormatAndRedactsSecrets(t *testing.T) {
	root := &cobra.Command{Use: "tool", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("format", "table", "")
	root.AddCommand(&cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error {
		return outputFormatErrorForTest()
	}})
	var stdout, stderr bytes.Buffer
	code := Execute(root, "tool", []string{"run", "--format", "xml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "invalid_args") || !strings.Contains(out, "unknown_output_format") {
		t.Fatalf("bad fallback output: %s", out)
	}
	if strings.Contains(out, "gho_SECRET") || strings.Contains(out, "Authorization") || strings.Contains(out, "data:image/png;base64,abc") {
		t.Fatalf("secret leaked: %s", out)
	}
}

func outputFormatErrorForTest() error {
	return &testErr{s: `unknown_output_format Authorization: Bearer gho_SECRET data:image/png;base64,abc`}
}

type testErr struct{ s string }

func (e *testErr) Error() string { return e.s }
