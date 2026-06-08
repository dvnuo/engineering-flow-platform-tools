package automation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestWorkflowVariablesLoopsConditionsSmartWaitAndReport(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
vars:
  enabled: "true"
  hidden: "false"
smart_wait:
  network_idle_ms: 500
  dom_stable_ms: 400
steps:
  - action: page.click
    selector: ".row-{{item}}"
    for_each: [a, b]
    if: "{{vars.enabled}}"
  - action: page.type
    selector: "#secret"
    text: "{{vars.secret_text}}"
    if: "{{vars.hidden}}"
  - action: human.wait
    duration_ms: 1000
    prompt: "manual check"
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	report := filepath.Join(t.TempDir(), "workflow-report.json")
	result, err := RunWorkflow(context.Background(), nil, WorkflowRunOptions{
		Definition:   def,
		DryRun:       true,
		VarOverrides: []string{"secret_text=typed-secret-value"},
		ReportOut:    report,
		AllowHuman:   true,
	})
	if err != nil {
		t.Fatalf("RunWorkflow dry-run failed: %v", err)
	}
	if result.StepCount != 4 || result.CompletedSteps != 4 {
		t.Fatalf("expected expanded steps, got %#v", result)
	}
	if !result.Steps[2].Plan.Skipped || result.Steps[2].Status != "skipped" {
		t.Fatalf("condition skip not reflected: %#v", result.Steps[2])
	}
	if result.Config.SmartWait.NetworkIdleMilliseconds != 500 || result.Config.SmartWait.DOMStableMilliseconds != 400 {
		t.Fatalf("smart_wait config missing: %#v", result.Config.SmartWait)
	}
	if result.ReportPath != report || result.ReportBytes == 0 {
		t.Fatalf("report metadata missing: %#v", result)
	}
	b, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, leaked := range []string{"typed-secret-value", "access_token"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("workflow report leaked %q in %s", leaked, out)
		}
	}
}

func TestWorkflowParsesNewAutomationActions(t *testing.T) {
	def, err := ParseWorkflowYAML([]byte(`
steps:
  - action: page.extract_schema
    file: schema.yaml
  - action: form.inspect
    selector: form
  - action: form.fill
    file: values.yaml
  - action: page.metrics
    limit: 5
  - action: network.export
    out: network.json
    format: har-lite
  - action: assert.screenshot
    baseline: baseline.png
    out: actual.png
    diff_out: diff.png
    threshold: 0.02
`))
	if err != nil {
		t.Fatalf("ParseWorkflowYAML failed: %v", err)
	}
	result, err := RunWorkflow(context.Background(), nil, WorkflowRunOptions{Definition: def, DryRun: true})
	if err != nil {
		t.Fatalf("RunWorkflow dry-run failed: %v", err)
	}
	if len(result.Steps) != 6 || result.Steps[5].Plan.Threshold == nil {
		t.Fatalf("new actions not planned: %#v", result.Steps)
	}
}
