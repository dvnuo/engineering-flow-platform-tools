package automation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseWorkflowYAMLValidatesWhitelistedActions(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
steps:
  - action: tab.open
    url: https://intranet.test/
  - action: page.wait
    selector: .ready
  - action: assert.count
    selector: .item
    min: 1
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	if len(def.Steps) != 3 || def.Steps[2].Action != "assert.count" || !def.Steps[2].HasMin {
		t.Fatalf("unexpected workflow definition: %#v", def)
	}
}

func TestParseWorkflowYAMLRejectsDisallowedActionsAndFields(t *testing.T) {
	cases := []string{
		`steps: [{action: shell}]`,
		`steps: [{action: page.eval, expr: document.cookie}]`,
		`steps: [{action: page.fetch, url: /api/me}]`,
		`steps: [{action: page.click, selector: button, command: "rm -rf /"}]`,
		`command: "browser page click --selector button"
steps: [{action: page.click, selector: button}]`,
	}
	for _, raw := range cases {
		if _, err := ParseWorkflowYAML([]byte(raw)); err == nil {
			t.Fatalf("workflow should have been rejected:\n%s", raw)
		}
	}
}

func TestRunWorkflowDryRunPlansWithoutManager(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
session: qa
timeout: 12
steps:
  - action: page.type
    selector: input[name=q]
    text: super-secret typed text
    clear: true
  - action: assert.text
    contains: access_token=secret
    not: true
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	result, err := RunWorkflow(context.Background(), nil, WorkflowRunOptions{Definition: def, DryRun: true})
	if err != nil {
		t.Fatalf("RunWorkflow dry-run failed: %v", err)
	}
	if !result.DryRun || result.Status != "planned" || !result.Pass || result.CompletedSteps != 2 {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if result.Config.SessionName != "qa" || result.Config.TimeoutSeconds != 12 {
		t.Fatalf("config not preserved: %#v", result.Config)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, leaked := range []string{"super-secret typed text", "access_token=secret"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("workflow dry-run leaked %q in %s", leaked, out)
		}
	}
	if !strings.Contains(out, `"text_bytes"`) || !strings.Contains(out, `"has_text"`) {
		t.Fatalf("typed text metadata missing: %s", out)
	}
}

func TestWorkflowRunOptionOverrides(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
session: yaml-session
target_id: yaml-target
timeout: 5
continue_on_error: true
steps:
  - action: assert.url
    contains: /ready
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	result, err := RunWorkflow(context.Background(), nil, WorkflowRunOptions{
		Definition:       def,
		DryRun:           true,
		SessionName:      "cli-session",
		TargetID:         "cli-target",
		TimeoutSeconds:   9,
		ContinueOnError:  false,
		SessionOverride:  true,
		TargetOverride:   true,
		TimeoutOverride:  true,
		ContinueOverride: true,
	})
	if err != nil {
		t.Fatalf("RunWorkflow dry-run failed: %v", err)
	}
	if result.Config.SessionName != "cli-session" || result.Config.TargetID != "cli-target" || result.Config.TimeoutSeconds != 9 || result.Config.ContinueOnError {
		t.Fatalf("overrides not applied: %#v", result.Config)
	}
}
