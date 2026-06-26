package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type workflowSpec struct {
	Name  string         `json:"name,omitempty" yaml:"name,omitempty"`
	Steps []workflowStep `json:"steps" yaml:"steps"`
}

type workflowStep struct {
	Action          string  `json:"action" yaml:"action"`
	RunID           string  `json:"run_id,omitempty" yaml:"run_id,omitempty"`
	Ref             string  `json:"ref,omitempty" yaml:"ref,omitempty"`
	Name            string  `json:"name,omitempty" yaml:"name,omitempty"`
	Text            string  `json:"text,omitempty" yaml:"text,omitempty"`
	TextEnv         string  `json:"text_env,omitempty" yaml:"text_env,omitempty"`
	Role            string  `json:"role,omitempty" yaml:"role,omitempty"`
	ResourceID      string  `json:"resource_id,omitempty" yaml:"resource_id,omitempty"`
	AccessibilityID string  `json:"accessibility_id,omitempty" yaml:"accessibility_id,omitempty"`
	ParentText      string  `json:"parent_text,omitempty" yaml:"parent_text,omitempty"`
	NearbyText      string  `json:"nearby_text,omitempty" yaml:"nearby_text,omitempty"`
	WithinText      string  `json:"within_text,omitempty" yaml:"within_text,omitempty"`
	Direction       string  `json:"direction,omitempty" yaml:"direction,omitempty"`
	Network         string  `json:"network,omitempty" yaml:"network,omitempty"`
	App             string  `json:"app,omitempty" yaml:"app,omitempty"`
	AppID           string  `json:"app_id,omitempty" yaml:"app_id,omitempty"`
	URL             string  `json:"url,omitempty" yaml:"url,omitempty"`
	Package         string  `json:"package,omitempty" yaml:"package,omitempty"`
	File            string  `json:"file,omitempty" yaml:"file,omitempty"`
	Platform        string  `json:"platform,omitempty" yaml:"platform,omitempty"`
	Device          string  `json:"device,omitempty" yaml:"device,omitempty"`
	Build           string  `json:"build,omitempty" yaml:"build,omitempty"`
	SessionName     string  `json:"session_name,omitempty" yaml:"session_name,omitempty"`
	Timeout         string  `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	PollInterval    string  `json:"poll_interval,omitempty" yaml:"poll_interval,omitempty"`
	Status          string  `json:"status,omitempty" yaml:"status,omitempty"`
	Contains        string  `json:"contains,omitempty" yaml:"contains,omitempty"`
	Equals          string  `json:"equals,omitempty" yaml:"equals,omitempty"`
	Expected        *int    `json:"expected,omitempty" yaml:"expected,omitempty"`
	MaxScrolls      int     `json:"max_scrolls,omitempty" yaml:"max_scrolls,omitempty"`
	Keycode         int     `json:"keycode,omitempty" yaml:"keycode,omitempty"`
	XPercent        float64 `json:"x_percent,omitempty" yaml:"x_percent,omitempty"`
	YPercent        float64 `json:"y_percent,omitempty" yaml:"y_percent,omitempty"`
	DurationMS      int     `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty"`
}

func workflowCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "workflow"}
	c.AddCommand(workflowRunCmd(o), workflowRecordCmd(o))
	return c
}

func workflowRunCmd(o *Opts) *cobra.Command {
	var filePath, reportOut string
	var dryRun bool
	c := &cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error {
		if filePath == "" {
			return print(cmd, o, output.Failure("invalid_args", "--file is required", "Pass a mobile-auto workflow YAML file.", 400))
		}
		spec, err := readWorkflowSpec(filePath)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		plan, err := workflowPlan(spec)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "name": spec.Name, "steps": redactWorkflowArgs(plan)}))
		}
		report := []map[string]any{}
		currentRunID := ""
		for i, planned := range plan {
			args := append([]string{}, planned...)
			args = fillWorkflowRunID(args, currentRunID)
			env, raw, err := executeWorkflowCommand(cmd, o, args)
			item := map[string]any{"index": i + 1, "args": redactArgs(args), "ok": env.OK}
			if raw != "" {
				item["raw_length"] = len(raw)
			}
			if env.Data != nil {
				item["data"] = env.Data
			}
			if env.Error != nil {
				item["error"] = env.Error
			}
			report = append(report, item)
			if err != nil {
				return print(cmd, o, workflowFailureEnvelope("workflow_step_failed", "workflow step failed", "Inspect report for the failed whitelisted step.", report, 500))
			}
			if !env.OK {
				return print(cmd, o, workflowFailureEnvelope("workflow_step_failed", "workflow step returned ok=false", "Inspect report for the failed whitelisted step.", report, 500))
			}
			if id := runIDFromEnvelope(env); id != "" {
				currentRunID = id
			}
		}
		data := map[string]any{"name": spec.Name, "steps": len(plan), "report": report}
		if reportOut != "" {
			path, err := writeWorkflowReport(reportOut, data)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			data["report_path"] = path
		}
		return print(cmd, o, output.Success("", data))
	}}
	c.Flags().StringVar(&filePath, "file", "", "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	c.Flags().StringVar(&reportOut, "report-out", "", "")
	return c
}

func workflowRecordCmd(o *Opts) *cobra.Command {
	var runID, outPath string
	c := &cobra.Command{Use: "record", RunE: func(cmd *cobra.Command, args []string) error {
		if runID == "" || outPath == "" {
			return print(cmd, o, output.Failure("invalid_args", "--run-id and --out are required", "Pass a run id and workflow YAML output path.", 400))
		}
		svc, err := newServices(o, false)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		events, err := svc.Store.LoadTimeline(runID)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		spec := workflowSpec{Name: "recorded-" + runID}
		for _, event := range events {
			step, ok := workflowStepFromTimeline(event)
			if !ok {
				continue
			}
			spec.Steps = append(spec.Steps, step)
		}
		if len(spec.Steps) == 0 {
			spec.Steps = append(spec.Steps, workflowStep{Action: "observe", RunID: runID})
		}
		b, err := yaml.Marshal(spec)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil && filepath.Dir(outPath) != "." {
			return renderErr(cmd, o, err)
		}
		if err := os.WriteFile(outPath, b, 0o600); err != nil {
			return renderErr(cmd, o, err)
		}
		return print(cmd, o, output.Success("", map[string]any{"run_id": runID, "out": outPath, "steps": len(spec.Steps)}))
	}}
	c.Flags().StringVar(&runID, "run-id", "", "")
	c.Flags().StringVar(&outPath, "out", "", "")
	return c
}

func readWorkflowSpec(path string) (workflowSpec, error) {
	var spec workflowSpec
	b, err := os.ReadFile(path)
	if err != nil {
		return spec, err
	}
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return spec, err
	}
	if len(spec.Steps) == 0 {
		return spec, mobileError("invalid_args", "workflow has no steps", "Add at least one whitelisted step.", 400)
	}
	return spec, nil
}

func workflowPlan(spec workflowSpec) ([][]string, error) {
	plan := make([][]string, 0, len(spec.Steps))
	for _, step := range spec.Steps {
		args, err := workflowStepArgs(step)
		if err != nil {
			return nil, err
		}
		plan = append(plan, args)
	}
	return plan, nil
}

func workflowStepArgs(step workflowStep) ([]string, error) {
	action := strings.TrimSpace(step.Action)
	switch action {
	case "run.start":
		args := []string{"run", "start"}
		args = appendFlag(args, "--app", step.App)
		args = appendFlag(args, "--file", step.File)
		args = appendFlag(args, "--platform", step.Platform)
		args = appendFlag(args, "--device", step.Device)
		args = appendFlag(args, "--network", step.Network)
		args = appendFlag(args, "--build", step.Build)
		args = appendFlag(args, "--name", step.SessionName)
		return args, nil
	case "observe":
		return appendRunID([]string{"observe"}, step.RunID), nil
	case "locate":
		args := appendRunID([]string{"locate"}, step.RunID)
		args = appendFlag(args, "--name", step.Name)
		args = appendFlag(args, "--text", step.Text)
		args = appendFlag(args, "--role", step.Role)
		args = appendFlag(args, "--resource-id", step.ResourceID)
		args = appendFlag(args, "--accessibility-id", step.AccessibilityID)
		args = appendFlag(args, "--parent-text", step.ParentText)
		args = appendFlag(args, "--nearby-text", step.NearbyText)
		args = appendFlag(args, "--within-text", step.WithinText)
		return args, nil
	case "tap", "clear", "back":
		args := appendRunID([]string{action}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		return args, nil
	case "type":
		args := appendRunID([]string{"type"}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		args = appendFlag(args, "--text", step.Text)
		args = appendFlag(args, "--text-env", step.TextEnv)
		return args, nil
	case "scroll", "swipe":
		args := appendRunID([]string{action}, step.RunID)
		args = appendFlag(args, "--direction", step.Direction)
		if step.DurationMS > 0 {
			args = append(args, "--duration-ms", intString(step.DurationMS))
		}
		return args, nil
	case "scroll-to":
		args := appendRunID([]string{"scroll-to"}, step.RunID)
		args = appendFlag(args, "--name", step.Name)
		args = appendFlag(args, "--text", step.Text)
		args = appendFlag(args, "--role", step.Role)
		args = appendFlag(args, "--direction", step.Direction)
		if step.MaxScrolls > 0 {
			args = append(args, "--max-scrolls", intString(step.MaxScrolls))
		}
		return args, nil
	case "tap-point":
		args := appendRunID([]string{"tap-point"}, step.RunID)
		if step.XPercent >= 0 {
			args = append(args, "--x-percent", floatString(step.XPercent))
		}
		if step.YPercent >= 0 {
			args = append(args, "--y-percent", floatString(step.YPercent))
		}
		return args, nil
	case "long-press", "double-tap":
		args := appendRunID([]string{action}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		if step.DurationMS > 0 {
			args = append(args, "--duration-ms", intString(step.DurationMS))
		}
		return args, nil
	case "keyboard.hide":
		return appendRunID([]string{"keyboard", "hide"}, step.RunID), nil
	case "keyboard.enter":
		return appendRunID([]string{"keyboard", "enter"}, step.RunID), nil
	case "keyboard.keycode":
		args := appendRunID([]string{"keyboard", "keycode"}, step.RunID)
		if step.Keycode > 0 {
			args = append(args, "--keycode", intString(step.Keycode))
		}
		return args, nil
	case "wait.stable":
		args := appendRunID([]string{"wait", "stable"}, step.RunID)
		args = appendFlag(args, "--timeout", step.Timeout)
		args = appendFlag(args, "--poll-interval", step.PollInterval)
		return args, nil
	case "wait.visible", "wait.gone", "wait.text", "wait.enabled":
		args := appendRunID([]string{"wait", strings.TrimPrefix(action, "wait.")}, step.RunID)
		args = appendLocatorStepFlags(args, step)
		args = appendFlag(args, "--timeout", step.Timeout)
		args = appendFlag(args, "--poll-interval", step.PollInterval)
		return args, nil
	case "assert.visible":
		args := appendRunID([]string{"assert", "visible"}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		args = appendFlag(args, "--name", step.Name)
		args = appendFlag(args, "--role", step.Role)
		return args, nil
	case "assert.not-visible":
		args := appendRunID([]string{"assert", "not-visible"}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		args = appendFlag(args, "--name", step.Name)
		args = appendFlag(args, "--role", step.Role)
		return args, nil
	case "assert.not-exists":
		args := appendRunID([]string{"assert", "not-exists"}, step.RunID)
		args = appendLocatorStepFlags(args, step)
		return args, nil
	case "assert.count":
		args := appendRunID([]string{"assert", "count"}, step.RunID)
		args = appendLocatorStepFlags(args, step)
		if step.Expected != nil {
			args = append(args, "--expected", intString(*step.Expected))
		}
		return args, nil
	case "assert.text":
		args := appendRunID([]string{"assert", "text"}, step.RunID)
		args = appendFlag(args, "--ref", step.Ref)
		args = appendFlag(args, "--name", step.Name)
		args = appendFlag(args, "--role", step.Role)
		args = appendFlag(args, "--contains", step.Contains)
		args = appendFlag(args, "--equals", step.Equals)
		return args, nil
	case "run.finish":
		args := appendRunID([]string{"run", "finish"}, step.RunID)
		args = appendFlag(args, "--status", firstNonEmpty(step.Status, "passed"))
		return args, nil
	case "app.launch", "app.close", "app.reset":
		return appendRunID([]string{"app", strings.TrimPrefix(action, "app.")}, step.RunID), nil
	case "app.activate", "app.terminate":
		args := appendRunID([]string{"app", strings.TrimPrefix(action, "app.")}, step.RunID)
		args = appendFlag(args, "--app-id", step.AppID)
		return args, nil
	case "app.deep-link":
		args := appendRunID([]string{"app", "deep-link"}, step.RunID)
		args = appendFlag(args, "--url", step.URL)
		args = appendFlag(args, "--package", step.Package)
		return args, nil
	case "permissions.accept":
		return appendRunID([]string{"permissions", "accept"}, step.RunID), nil
	case "permissions.deny":
		return appendRunID([]string{"permissions", "deny"}, step.RunID), nil
	default:
		return nil, mobileError("invalid_args", "workflow action is not whitelisted: "+action, "Use structured mobile-auto workflow actions such as observe, tap, assert.visible, or run.finish.", 400)
	}
}

func appendLocatorStepFlags(args []string, step workflowStep) []string {
	args = appendFlag(args, "--name", step.Name)
	args = appendFlag(args, "--text", step.Text)
	args = appendFlag(args, "--role", step.Role)
	args = appendFlag(args, "--resource-id", step.ResourceID)
	args = appendFlag(args, "--accessibility-id", step.AccessibilityID)
	args = appendFlag(args, "--parent-text", step.ParentText)
	args = appendFlag(args, "--nearby-text", step.NearbyText)
	args = appendFlag(args, "--within-text", step.WithinText)
	return args
}

func executeWorkflowCommand(parent *cobra.Command, o *Opts, args []string) (output.Envelope, string, error) {
	root := NewRoot()
	fullArgs := []string{"--json"}
	if o.ConfigPath != "" {
		fullArgs = append(fullArgs, "--config", o.ConfigPath)
	}
	fullArgs = append(fullArgs, args...)
	var buf bytes.Buffer
	root.SetArgs(fullArgs)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(parent.InOrStdin())
	err := root.ExecuteContext(parent.Context())
	raw := buf.String()
	var env output.Envelope
	if jsonErr := json.Unmarshal(buf.Bytes(), &env); jsonErr != nil {
		if err != nil {
			return env, raw, err
		}
		return env, raw, jsonErr
	}
	return env, raw, err
}

func workflowFailureEnvelope(code, message, hint string, report []map[string]any, status int) output.Envelope {
	env := output.Failure(code, message, hint, status)
	env.Data = map[string]any{"report": report}
	return env
}

func runIDFromEnvelope(env output.Envelope) string {
	data, ok := env.Data.(map[string]any)
	if !ok {
		return ""
	}
	if run, ok := data["run"].(map[string]any); ok {
		if id, ok := run["run_id"].(string); ok {
			return id
		}
	}
	if id, ok := data["run_id"].(string); ok {
		return id
	}
	return ""
}

func workflowStepFromTimeline(event mobile.TimelineEvent) (workflowStep, bool) {
	switch event.Type {
	case "observe":
		return workflowStep{Action: "observe", RunID: event.RunID}, true
	case "action":
		step := workflowStep{Action: workflowActionName(event.Action), RunID: event.RunID}
		if ref, ok := event.Data["ref"].(string); ok {
			step.Ref = ref
		}
		if source, ok := event.Data["text_source"].(string); ok && source != "" {
			step.TextEnv = "TODO_TEXT_ENV"
		}
		if key, ok := event.Data["keycode"].(float64); ok {
			step.Keycode = int(key)
		}
		return step, step.Action != ""
	default:
		return workflowStep{}, false
	}
}

func workflowActionName(action string) string {
	switch action {
	case "tap_point":
		return "tap-point"
	case "long_press":
		return "long-press"
	case "double_tap":
		return "double-tap"
	case "keyboard_hide":
		return "keyboard.hide"
	case "keyboard_keycode":
		return "keyboard.keycode"
	case "keyboard_enter":
		return "keyboard.enter"
	case "context_switch", "context_auto_webview":
		return ""
	default:
		return strings.ReplaceAll(action, "_", "-")
	}
}

func fillWorkflowRunID(args []string, runID string) []string {
	if runID == "" || hasFlag(args, "--run-id") || len(args) >= 2 && args[0] == "run" && args[1] == "start" {
		return args
	}
	return append(args, "--run-id", runID)
}

func appendRunID(args []string, runID string) []string {
	return appendFlag(args, "--run-id", runID)
}

func appendFlag(args []string, flag, value string) []string {
	if strings.TrimSpace(value) == "" {
		return args
	}
	return append(args, flag, value)
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func redactWorkflowArgs(plan [][]string) [][]string {
	out := make([][]string, 0, len(plan))
	for _, args := range plan {
		out = append(out, redactArgs(args))
	}
	return out
}

func redactArgs(args []string) []string {
	out := append([]string{}, args...)
	for i := 0; i < len(out)-1; i++ {
		if out[i] == "--text" {
			out[i+1] = output.Redacted
		}
	}
	return out
}

func writeWorkflowReport(path string, data map[string]any) (string, error) {
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(output.RedactValue(data), "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o600)
}

func intString(v int) string {
	return strconv.Itoa(v)
}

func floatString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func mobileError(code, message, hint string, status int) error {
	return mobile.NewError(code, message, hint, status)
}
