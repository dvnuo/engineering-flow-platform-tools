package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const workflowFailureCode = "workflow_failed"

var workflowTemplatePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

var workflowAllowedActions = map[string]bool{
	"tab.open":            true,
	"page.wait":           true,
	"page.click":          true,
	"page.type":           true,
	"page.press":          true,
	"page.select":         true,
	"page.check":          true,
	"page.uncheck":        true,
	"page.screenshot":     true,
	"page.extract_schema": true,
	"page.metrics":        true,
	"assert.visible":      true,
	"assert.text":         true,
	"assert.url":          true,
	"assert.count":        true,
	"assert.screenshot":   true,
	"network.start":       true,
	"network.wait":        true,
	"network.list":        true,
	"network.export":      true,
	"page.console":        true,
	"page.errors":         true,
	"form.inspect":        true,
	"form.fill":           true,
	"human.wait":          true,
	"human.confirm":       true,
}

type WorkflowRunOptions struct {
	File             string
	Definition       WorkflowDefinition
	DryRun           bool
	SessionName      string
	TargetID         string
	TimeoutSeconds   int
	ContinueOnError  bool
	SessionOverride  bool
	TargetOverride   bool
	TimeoutOverride  bool
	ContinueOverride bool
	VarOverrides     []string
	ReportOut        string
	EvidenceDir      string
	AllowHuman       bool
	AutoConfirm      bool
}

type WorkflowDefinition struct {
	SessionName     string            `json:"session,omitempty"`
	TargetID        string            `json:"target_id,omitempty"`
	TimeoutSeconds  int               `json:"timeout,omitempty"`
	ContinueOnError bool              `json:"continue_on_error,omitempty"`
	Vars            map[string]string `json:"-"`
	SmartWait       WorkflowSmartWait `json:"smart_wait,omitempty"`
	Steps           []WorkflowStep    `json:"steps"`
}

type WorkflowStep struct {
	Action                  string            `json:"action"`
	Name                    string            `json:"name,omitempty"`
	Selector                string            `json:"selector,omitempty"`
	Ref                     string            `json:"ref,omitempty"`
	Locators                []ElementLocator  `json:"locators,omitempty"`
	Contains                string            `json:"contains,omitempty"`
	URL                     string            `json:"url,omitempty"`
	URLContains             string            `json:"url_contains,omitempty"`
	File                    string            `json:"file,omitempty"`
	Baseline                string            `json:"baseline,omitempty"`
	DiffOut                 string            `json:"diff_out,omitempty"`
	Format                  string            `json:"format,omitempty"`
	Text                    string            `json:"-"`
	Key                     string            `json:"key,omitempty"`
	Value                   string            `json:"-"`
	Label                   string            `json:"-"`
	Out                     string            `json:"out,omitempty"`
	Filter                  string            `json:"filter,omitempty"`
	Method                  string            `json:"method,omitempty"`
	Level                   string            `json:"level,omitempty"`
	If                      string            `json:"if,omitempty"`
	As                      string            `json:"as,omitempty"`
	Prompt                  string            `json:"prompt,omitempty"`
	Not                     bool              `json:"not,omitempty"`
	Clear                   bool              `json:"clear,omitempty"`
	FullPage                bool              `json:"full_page,omitempty"`
	FullPageSet             bool              `json:"-"`
	ForEach                 []string          `json:"-"`
	SmartWait               WorkflowSmartWait `json:"smart_wait,omitempty"`
	Skip                    bool              `json:"-"`
	SkipReason              string            `json:"-"`
	Equals                  int               `json:"equals,omitempty"`
	Min                     int               `json:"min,omitempty"`
	Max                     int               `json:"max,omitempty"`
	Index                   int               `json:"index,omitempty"`
	Limit                   int               `json:"limit,omitempty"`
	Status                  int               `json:"status,omitempty"`
	Threshold               float64           `json:"threshold,omitempty"`
	DurationMilliseconds    int               `json:"duration_ms,omitempty"`
	NetworkIdleMilliseconds int               `json:"network_idle_ms,omitempty"`
	DOMStableMilliseconds   int               `json:"dom_stable_ms,omitempty"`
	HasEquals               bool              `json:"-"`
	HasMin                  bool              `json:"-"`
	HasMax                  bool              `json:"-"`
	HasIndex                bool              `json:"-"`
	HasThreshold            bool              `json:"-"`
}

type WorkflowSmartWait struct {
	Selector                string `json:"selector,omitempty"`
	URLContains             string `json:"url_contains,omitempty"`
	Text                    string `json:"text,omitempty"`
	DurationMilliseconds    int    `json:"duration_ms,omitempty"`
	NetworkIdleMilliseconds int    `json:"network_idle_ms,omitempty"`
	DOMStableMilliseconds   int    `json:"dom_stable_ms,omitempty"`
}

type WorkflowConfig struct {
	SessionName     string            `json:"session"`
	TargetID        string            `json:"target_id,omitempty"`
	TimeoutSeconds  int               `json:"timeout"`
	ContinueOnError bool              `json:"continue_on_error,omitempty"`
	AllowHuman      bool              `json:"allow_human,omitempty"`
	AutoConfirm     bool              `json:"auto_confirm,omitempty"`
	SmartWait       WorkflowSmartWait `json:"smart_wait,omitempty"`
}

type WorkflowStepPlan struct {
	Index                   int                `json:"index"`
	Name                    string             `json:"name,omitempty"`
	Action                  string             `json:"action"`
	Selector                string             `json:"selector,omitempty"`
	Ref                     string             `json:"ref,omitempty"`
	LocatorCount            int                `json:"locator_count,omitempty"`
	URL                     string             `json:"url,omitempty"`
	URLContainsPreview      string             `json:"url_contains_preview,omitempty"`
	URLContainsBytes        int                `json:"url_contains_bytes,omitempty"`
	File                    string             `json:"file,omitempty"`
	Baseline                string             `json:"baseline,omitempty"`
	DiffOut                 string             `json:"diff_out,omitempty"`
	Format                  string             `json:"format,omitempty"`
	ContainsPreview         string             `json:"contains_preview,omitempty"`
	ContainsBytes           int                `json:"contains_bytes,omitempty"`
	HasText                 bool               `json:"has_text,omitempty"`
	TextBytes               int                `json:"text_bytes,omitempty"`
	Clear                   bool               `json:"clear,omitempty"`
	Key                     string             `json:"key,omitempty"`
	SelectionMode           string             `json:"selection_mode,omitempty"`
	Out                     string             `json:"out,omitempty"`
	Filter                  string             `json:"filter,omitempty"`
	Method                  string             `json:"method,omitempty"`
	Level                   string             `json:"level,omitempty"`
	IfPreview               string             `json:"if_preview,omitempty"`
	ForEachCount            int                `json:"for_each_count,omitempty"`
	As                      string             `json:"as,omitempty"`
	PromptPreview           string             `json:"prompt_preview,omitempty"`
	PromptBytes             int                `json:"prompt_bytes,omitempty"`
	Skipped                 bool               `json:"skipped,omitempty"`
	SkipReason              string             `json:"skip_reason,omitempty"`
	Not                     bool               `json:"not,omitempty"`
	FullPage                *bool              `json:"full_page,omitempty"`
	Equals                  *int               `json:"equals,omitempty"`
	Min                     *int               `json:"min,omitempty"`
	Max                     *int               `json:"max,omitempty"`
	IndexValue              *int               `json:"index_value,omitempty"`
	Limit                   int                `json:"limit,omitempty"`
	Status                  int                `json:"status,omitempty"`
	Threshold               *float64           `json:"threshold,omitempty"`
	DurationMilliseconds    int                `json:"duration_ms,omitempty"`
	NetworkIdleMilliseconds int                `json:"network_idle_ms,omitempty"`
	DOMStableMilliseconds   int                `json:"dom_stable_ms,omitempty"`
	SmartWait               *WorkflowSmartWait `json:"smart_wait,omitempty"`
}

type WorkflowStepError struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Hint        string   `json:"hint,omitempty"`
	Status      int      `json:"status,omitempty"`
	NextActions []string `json:"next_actions,omitempty"`
}

type WorkflowStepResult struct {
	Index                int                `json:"index"`
	Name                 string             `json:"name,omitempty"`
	Action               string             `json:"action"`
	Status               string             `json:"status"`
	Pass                 bool               `json:"pass"`
	DurationMilliseconds int                `json:"duration_ms,omitempty"`
	Plan                 WorkflowStepPlan   `json:"plan"`
	Data                 any                `json:"data,omitempty"`
	Error                *WorkflowStepError `json:"error,omitempty"`
}

type WorkflowRunResult struct {
	DryRun               bool                 `json:"dry_run"`
	Pass                 bool                 `json:"pass"`
	Status               string               `json:"status"`
	Config               WorkflowConfig       `json:"config"`
	StepCount            int                  `json:"step_count"`
	CompletedSteps       int                  `json:"completed_steps"`
	FailedStep           int                  `json:"failed_step,omitempty"`
	DurationMilliseconds int                  `json:"duration_ms,omitempty"`
	Steps                []WorkflowStepResult `json:"steps"`
	Limitation           string               `json:"limitation"`
	ReportPath           string               `json:"report_path,omitempty"`
	ReportBytes          int64                `json:"report_bytes,omitempty"`
	EvidenceDir          string               `json:"evidence_dir,omitempty"`
	EvidenceManifest     string               `json:"evidence_manifest,omitempty"`
	EvidenceBytes        int64                `json:"evidence_bytes,omitempty"`
}

type WorkflowError struct {
	Base   *Error
	Result WorkflowRunResult
}

func (e *WorkflowError) Error() string {
	if e == nil || e.Base == nil {
		return ""
	}
	return e.Base.Error()
}

func LoadWorkflowFile(path string) (WorkflowDefinition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return WorkflowDefinition{}, invalidArgs("--file is required", "Pass a workflow YAML file.")
	}
	b, err := os.ReadFile(expandHome(path))
	if err != nil {
		return WorkflowDefinition{}, NewError("workflow_read_failed", err.Error(), "Check that --file points to a readable YAML file.", 400)
	}
	return ParseWorkflowYAML(b)
}

func ParseWorkflowYAML(b []byte) (WorkflowDefinition, error) {
	var def WorkflowDefinition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return WorkflowDefinition{}, NewError("workflow_invalid", RedactError(err.Error()), "Use a compact YAML shape such as steps: [{action: page.wait, selector: .ready}].", 400)
	}
	def = normalizeWorkflowDefinition(def)
	if err := ValidateWorkflow(def); err != nil {
		return WorkflowDefinition{}, err
	}
	return def, nil
}

func ValidateWorkflow(def WorkflowDefinition) error {
	if len(def.Steps) == 0 {
		return invalidWorkflow("steps is required", "Add one or more whitelisted workflow steps.")
	}
	for i, step := range def.Steps {
		if err := validateWorkflowStep(step); err != nil {
			return NewError("workflow_invalid", fmt.Sprintf("step %d: %s", i, err.Error()), "Use browser schema workflow.run --json and the documented action whitelist.", 400)
		}
	}
	return nil
}

func RunWorkflow(ctx context.Context, mgr *Manager, opts WorkflowRunOptions) (WorkflowRunResult, error) {
	def := normalizeWorkflowDefinition(opts.Definition)
	if len(def.Steps) == 0 && strings.TrimSpace(opts.File) != "" {
		loaded, err := LoadWorkflowFile(opts.File)
		if err != nil {
			return WorkflowRunResult{}, err
		}
		def = loaded
	}
	vars, err := workflowVars(def, opts)
	if err != nil {
		return WorkflowRunResult{}, err
	}
	def, err = expandWorkflow(def, vars)
	if err != nil {
		return WorkflowRunResult{}, err
	}
	if err := ValidateWorkflow(def); err != nil {
		return WorkflowRunResult{}, err
	}
	config := workflowConfig(def, opts)
	result := WorkflowRunResult{
		DryRun:     opts.DryRun,
		Pass:       true,
		Status:     "passed",
		Config:     config,
		StepCount:  len(def.Steps),
		Steps:      make([]WorkflowStepResult, 0, len(def.Steps)),
		Limitation: "Workflows execute only whitelisted browser CLI actions and assertions implemented by this codebase; arbitrary shell commands, browser CLI strings, page eval, and page fetch are not supported.",
	}
	start := time.Now()
	if opts.DryRun {
		result.Status = "planned"
		for i, step := range def.Steps {
			plan := workflowStepPlan(i, step)
			status := "planned"
			pass := true
			if step.Skip {
				status = "skipped"
			}
			result.Steps = append(result.Steps, WorkflowStepResult{
				Index:  i,
				Name:   plan.Name,
				Action: plan.Action,
				Status: status,
				Pass:   pass,
				Plan:   plan,
			})
		}
		result.CompletedSteps = len(result.Steps)
		result.DurationMilliseconds = elapsedMilliseconds(start)
		if err := writeWorkflowReport(opts.ReportOut, &result); err != nil {
			return WorkflowRunResult{}, err
		}
		return result, nil
	}
	if mgr == nil {
		return WorkflowRunResult{}, NewError("automation_failed", "workflow manager is required", "Run without --dry-run only through the browser workflow command.", 500)
	}
	for i, step := range def.Steps {
		stepStart := time.Now()
		plan := workflowStepPlan(i, step)
		stepResult := WorkflowStepResult{
			Index:  i,
			Name:   plan.Name,
			Action: plan.Action,
			Status: "passed",
			Pass:   true,
			Plan:   plan,
		}
		if step.Skip {
			stepResult.Status = "skipped"
			stepResult.Data = map[string]any{"reason": step.SkipReason}
			stepResult.DurationMilliseconds = elapsedMilliseconds(stepStart)
			result.Steps = append(result.Steps, stepResult)
			result.CompletedSteps = len(result.Steps)
			continue
		}
		data, err := executeWorkflowStep(ctx, mgr, config, step)
		if err != nil {
			stepResult.Pass = false
			stepResult.Status = "failed"
			stepResult.Error = workflowStepError(err)
			stepResult.Data = workflowErrorData(err)
			result.Pass = false
			result.Status = "failed"
			result.FailedStep = i
		} else {
			stepResult.Data = data
		}
		stepResult.DurationMilliseconds = elapsedMilliseconds(stepStart)
		result.Steps = append(result.Steps, stepResult)
		result.CompletedSteps = len(result.Steps)
		if err != nil && !config.ContinueOnError {
			break
		}
	}
	result.DurationMilliseconds = elapsedMilliseconds(start)
	if !opts.DryRun && strings.TrimSpace(opts.EvidenceDir) != "" {
		if err := writeWorkflowEvidenceBundle(ctx, mgr, config, opts.EvidenceDir, &result); err != nil {
			return WorkflowRunResult{}, err
		}
	}
	if err := writeWorkflowReport(opts.ReportOut, &result); err != nil {
		return WorkflowRunResult{}, err
	}
	if !result.Pass {
		return result, &WorkflowError{
			Base:   NewError(workflowFailureCode, "Browser workflow failed.", "Inspect data.steps for the first failing whitelisted step.", 412),
			Result: result,
		}
	}
	return result, nil
}

func executeWorkflowStep(ctx context.Context, mgr *Manager, config WorkflowConfig, step WorkflowStep) (any, error) {
	page := PageOptions{SessionName: config.SessionName, TargetID: config.TargetID, TimeoutSeconds: config.TimeoutSeconds}
	data, err := executeWorkflowAction(ctx, mgr, config, page, step)
	if err != nil {
		return nil, err
	}
	wait := step.SmartWait
	if !workflowSmartWaitEnabled(wait) {
		wait = config.SmartWait
	}
	if workflowSmartWaitEnabled(wait) && step.Action != "page.wait" && !strings.HasPrefix(step.Action, "human.") {
		waitResult, err := mgr.Wait(ctx, WaitOptions{
			PageOptions:             page,
			Selector:                wait.Selector,
			DurationMilliseconds:    wait.DurationMilliseconds,
			URLContains:             wait.URLContains,
			Text:                    wait.Text,
			NetworkIdleMilliseconds: wait.NetworkIdleMilliseconds,
			DOMStableMilliseconds:   wait.DOMStableMilliseconds,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"result": data, "smart_wait": waitResult}, nil
	}
	return data, nil
}

func executeWorkflowAction(ctx context.Context, mgr *Manager, config WorkflowConfig, page PageOptions, step WorkflowStep) (any, error) {
	switch step.Action {
	case "tab.open":
		return mgr.OpenTab(ctx, config.SessionName, step.URL)
	case "page.wait":
		return mgr.Wait(ctx, WaitOptions{
			PageOptions:             page,
			Selector:                step.Selector,
			DurationMilliseconds:    step.DurationMilliseconds,
			URLContains:             step.URLContains,
			Text:                    step.Text,
			NetworkIdleMilliseconds: step.NetworkIdleMilliseconds,
			DOMStableMilliseconds:   step.DOMStableMilliseconds,
		})
	case "page.click":
		return executeWorkflowClick(ctx, mgr, config, page, step)
	case "page.type":
		return executeWorkflowType(ctx, mgr, page, step)
	case "page.press":
		return executeWorkflowPress(ctx, mgr, page, step)
	case "page.select":
		return executeWorkflowSelect(ctx, mgr, page, step)
	case "page.check":
		return executeWorkflowCheck(ctx, mgr, page, step, true)
	case "page.uncheck":
		return executeWorkflowCheck(ctx, mgr, page, step, false)
	case "page.screenshot":
		return executeWorkflowScreenshot(ctx, mgr, page, step)
	case "page.extract_schema":
		return mgr.ExtractSchema(ctx, ExtractSchemaOptions{PageOptions: page, File: step.File, Limit: step.Limit})
	case "page.metrics":
		return mgr.Metrics(ctx, MetricsOptions{PageOptions: page, LimitResources: step.Limit, Filter: step.Filter})
	case "assert.visible":
		return executeWorkflowAssertVisible(ctx, mgr, page, step)
	case "assert.text":
		return executeWorkflowAssertText(ctx, mgr, page, step)
	case "assert.url":
		return mgr.AssertURL(ctx, AssertionOptions{PageOptions: page, Contains: step.Contains, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	case "assert.count":
		return mgr.AssertCount(ctx, workflowCountAssertionOptions(page, step))
	case "assert.screenshot":
		return executeWorkflowAssertScreenshot(ctx, mgr, page, step)
	case "network.start":
		return mgr.NetworkStart(ctx, NetworkRecorderOptions{PageOptions: page, Filter: step.Filter, Limit: step.Limit, Status: -1, Body: true, MaxBodyBytes: 20000})
	case "network.wait":
		return mgr.NetworkWait(ctx, NetworkWaitOptions{NetworkRecorderOptions: NetworkRecorderOptions{PageOptions: page, Limit: step.Limit, Method: step.Method, Status: workflowStatus(step.Status), Body: true, MaxBodyBytes: 20000}, URLContains: step.URLContains})
	case "network.list":
		return mgr.NetworkList(ctx, NetworkRecorderOptions{PageOptions: page, Filter: step.Filter, Limit: step.Limit, Method: step.Method, Status: workflowStatus(step.Status), Body: true, MaxBodyBytes: 20000})
	case "network.export":
		return mgr.NetworkExport(ctx, NetworkExportOptions{PageOptions: page, OutPath: step.Out, Format: step.Format, Filter: step.Filter, Limit: step.Limit})
	case "page.console":
		return mgr.Console(ctx, ConsoleOptions{PageOptions: page, Level: step.Level, Limit: step.Limit})
	case "page.errors":
		return mgr.RuntimeErrors(ctx, ConsoleOptions{PageOptions: page, Limit: step.Limit})
	case "form.inspect":
		return mgr.FormInspect(ctx, FormInspectOptions{PageOptions: page, Selector: step.Selector, Limit: step.Limit})
	case "form.fill":
		return mgr.FormFill(ctx, FormFillOptions{PageOptions: page, File: step.File})
	case "human.wait":
		if !config.AllowHuman {
			return nil, NewError("human_action_disabled", "human.wait requires --allow-human.", "Rerun with --allow-human when the workflow intentionally pauses for manual browser interaction.", 409)
		}
		start := time.Now()
		select {
		case <-ctx.Done():
			return nil, NewError("timeout", ctx.Err().Error(), "The human wait was canceled or timed out.", 408)
		case <-time.After(time.Duration(step.DurationMilliseconds) * time.Millisecond):
		}
		return map[string]any{
			"action":      "human.wait",
			"duration_ms": step.DurationMilliseconds,
			"waited_ms":   elapsedMilliseconds(start),
			"prompt":      TruncateBytes(RedactString(step.Prompt), 500),
		}, nil
	case "human.confirm":
		if !config.AllowHuman || !config.AutoConfirm {
			return nil, NewError("human_confirmation_required", "human.confirm requires --allow-human --yes.", "Inspect the browser manually, then rerun with --allow-human --yes only when the user has confirmed the step.", 409)
		}
		return map[string]any{
			"action":    "human.confirm",
			"prompt":    TruncateBytes(RedactString(step.Prompt), 500),
			"confirmed": true,
		}, nil
	default:
		return nil, invalidWorkflow("unsupported action "+step.Action, "Use only documented workflow actions.")
	}
}

func workflowErrorData(err error) any {
	var assertErr *AssertionError
	if errors.As(err, &assertErr) {
		return assertErr.Result
	}
	return nil
}

func executeWorkflowClick(ctx context.Context, mgr *Manager, config WorkflowConfig, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Click(ctx, ClickOptions{PageOptions: page, Selector: selector, Ref: ref, AllowRisky: config.AutoConfirm})
	if err == nil || len(step.Locators) == 0 || !workflowCanRetryWithLocator(err) {
		return result, err
	}
	selector, resolveErr := mgr.ResolveLocatorSelector(ctx, page, step.Locators)
	if resolveErr != nil {
		return result, err
	}
	return mgr.Click(ctx, ClickOptions{PageOptions: page, Selector: selector, AllowRisky: config.AutoConfirm})
}

func executeWorkflowType(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Type(ctx, TypeOptions{PageOptions: page, Selector: selector, Ref: ref, Text: step.Text, Clear: step.Clear})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.Type(ctx, TypeOptions{PageOptions: page, Selector: selector, Text: step.Text, Clear: step.Clear})
	}
	return result, err
}

func executeWorkflowPress(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Press(ctx, PressOptions{PageOptions: page, Selector: selector, Ref: ref, Key: step.Key})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.Press(ctx, PressOptions{PageOptions: page, Selector: selector, Key: step.Key})
	}
	return result, err
}

func executeWorkflowSelect(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Select(ctx, SelectOptions{PageOptions: page, Selector: selector, Ref: ref, Value: step.Value, Label: step.Label, Index: workflowSelectIndex(step)})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.Select(ctx, SelectOptions{PageOptions: page, Selector: selector, Value: step.Value, Label: step.Label, Index: workflowSelectIndex(step)})
	}
	return result, err
}

func executeWorkflowCheck(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep, checked bool) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Check(ctx, CheckOptions{PageOptions: page, Selector: selector, Ref: ref, Checked: checked})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.Check(ctx, CheckOptions{PageOptions: page, Selector: selector, Checked: checked})
	}
	return result, err
}

func executeWorkflowScreenshot(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowOptionalActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.Screenshot(ctx, ScreenshotOptions{PageOptions: page, OutPath: step.Out, FullPage: workflowFullPage(step), FullPageSet: step.FullPageSet, Selector: selector, Ref: ref})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.Screenshot(ctx, ScreenshotOptions{PageOptions: page, OutPath: step.Out, FullPage: workflowFullPage(step), FullPageSet: step.FullPageSet, Selector: selector})
	}
	return result, err
}

func executeWorkflowAssertVisible(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.AssertVisible(ctx, AssertionOptions{PageOptions: page, Selector: selector, Ref: ref, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.AssertVisible(ctx, AssertionOptions{PageOptions: page, Selector: selector, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	}
	return result, err
}

func executeWorkflowAssertText(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.AssertText(ctx, AssertionOptions{PageOptions: page, Selector: selector, Ref: ref, Contains: step.Contains, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.AssertText(ctx, AssertionOptions{PageOptions: page, Selector: selector, Contains: step.Contains, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	}
	return result, err
}

func executeWorkflowAssertScreenshot(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (any, error) {
	selector, ref, err := workflowOptionalActionTarget(ctx, mgr, page, step)
	if err != nil {
		return nil, err
	}
	result, err := mgr.AssertScreenshot(ctx, ScreenshotAssertionOptions{PageOptions: page, Baseline: step.Baseline, OutPath: step.Out, DiffPath: step.DiffOut, Selector: selector, Ref: ref, Threshold: workflowThreshold(step), FullPage: workflowFullPage(step), FullPageSet: step.FullPageSet})
	if selector, ok := workflowRetrySelector(ctx, mgr, page, step, err); ok {
		return mgr.AssertScreenshot(ctx, ScreenshotAssertionOptions{PageOptions: page, Baseline: step.Baseline, OutPath: step.Out, DiffPath: step.DiffOut, Selector: selector, Threshold: workflowThreshold(step), FullPage: workflowFullPage(step), FullPageSet: step.FullPageSet})
	}
	return result, err
}

func workflowRetrySelector(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep, err error) (string, bool) {
	if err == nil || len(step.Locators) == 0 || !workflowCanRetryWithLocator(err) {
		return "", false
	}
	selector, resolveErr := mgr.ResolveLocatorSelector(ctx, page, step.Locators)
	if resolveErr != nil {
		return "", false
	}
	return selector, true
}

func workflowActionTarget(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (string, string, error) {
	if strings.TrimSpace(step.Selector) != "" || strings.TrimSpace(step.Ref) != "" {
		return step.Selector, step.Ref, nil
	}
	if len(step.Locators) == 0 {
		return "", "", validateActionTarget("", "", step.Action)
	}
	selector, err := mgr.ResolveLocatorSelector(ctx, page, step.Locators)
	if err != nil {
		return "", "", err
	}
	return selector, "", nil
}

func workflowOptionalActionTarget(ctx context.Context, mgr *Manager, page PageOptions, step WorkflowStep) (string, string, error) {
	if strings.TrimSpace(step.Selector) != "" || strings.TrimSpace(step.Ref) != "" {
		return step.Selector, step.Ref, nil
	}
	if len(step.Locators) == 0 {
		return "", "", nil
	}
	selector, err := mgr.ResolveLocatorSelector(ctx, page, step.Locators)
	if err != nil {
		return "", "", err
	}
	return selector, "", nil
}

func workflowCanRetryWithLocator(err error) bool {
	var autoErr *Error
	if !errors.As(err, &autoErr) {
		return false
	}
	switch autoErr.Code {
	case "selector_not_found", "ref_not_found", "automation_failed":
		return true
	default:
		return false
	}
}

func validateWorkflowActionTarget(step WorkflowStep) error {
	if strings.TrimSpace(step.Selector) == "" && strings.TrimSpace(step.Ref) == "" && len(step.Locators) > 0 {
		return nil
	}
	return validateActionTarget(step.Selector, step.Ref, step.Action)
}

func validateWorkflowOptionalActionTarget(step WorkflowStep) error {
	return validateOptionalActionTarget(step.Selector, step.Ref, step.Action)
}

func validateWorkflowSelectOptions(step WorkflowStep) (string, error) {
	if strings.TrimSpace(step.Selector) == "" && strings.TrimSpace(step.Ref) == "" && len(step.Locators) > 0 {
		modes := 0
		mode := ""
		if strings.TrimSpace(step.Value) != "" {
			modes++
			mode = "value"
		}
		if strings.TrimSpace(step.Label) != "" {
			modes++
			mode = "label"
		}
		if workflowSelectIndex(step) >= 0 {
			modes++
			mode = "index"
		}
		if modes == 0 {
			return "", invalidArgs("value, label, or index is required", "Pass exactly one selection target for page.select.")
		}
		if modes > 1 {
			return "", invalidArgs("pass only one of value, label, or index", "Selection output reports only the selection mode and count.")
		}
		return mode, nil
	}
	return validateSelectOptions(SelectOptions{Selector: step.Selector, Ref: step.Ref, Value: step.Value, Label: step.Label, Index: workflowSelectIndex(step)})
}

type workflowEvidenceManifest struct {
	Session     string                        `json:"session"`
	TargetID    string                        `json:"target_id,omitempty"`
	Workflow    WorkflowRunResult             `json:"workflow"`
	Artifacts   map[string]string             `json:"artifacts"`
	Captures    map[string]workflowCaptureRef `json:"captures"`
	GeneratedAt time.Time                     `json:"generated_at"`
	Limitation  string                        `json:"limitation"`
}

type workflowCaptureRef struct {
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

func writeWorkflowEvidenceBundle(ctx context.Context, mgr *Manager, config WorkflowConfig, dir string, result *WorkflowRunResult) error {
	if mgr == nil || result == nil {
		return nil
	}
	dir = filepath.Clean(expandHome(strings.TrimSpace(dir)))
	if dir == "" || dir == "." {
		return invalidArgs("--evidence-dir must point at a directory", "Pass a writable directory such as result/evidence.")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Check --evidence-dir permissions.", 500)
	}
	page := PageOptions{SessionName: config.SessionName, TargetID: config.TargetID, TimeoutSeconds: config.TimeoutSeconds}
	captures := map[string]workflowCaptureRef{}
	artifacts := map[string]string{}
	writeCapture := func(name string, value any, err error) {
		if err != nil {
			captures[name] = workflowCaptureRef{Error: RedactError(err.Error())}
			return
		}
		path := filepath.Join(dir, name+".json")
		if writeErr := writeWorkflowJSONFile(path, value); writeErr != nil {
			captures[name] = workflowCaptureRef{Error: RedactError(writeErr.Error())}
			return
		}
		captures[name] = workflowCaptureRef{Path: path}
		artifacts[name] = path
	}
	snapshot, snapshotErr := mgr.Snapshot(ctx, SnapshotOptions{PageOptions: page, MaxTextBytes: 4000})
	writeCapture("final-snapshot", snapshot, snapshotErr)
	metrics, metricsErr := mgr.Metrics(ctx, MetricsOptions{PageOptions: page, LimitResources: 20})
	writeCapture("metrics", metrics, metricsErr)
	consoleResult, consoleErr := mgr.Console(ctx, ConsoleOptions{PageOptions: page, Limit: 100})
	writeCapture("console", consoleResult, consoleErr)
	errorsResult, errorsErr := mgr.RuntimeErrors(ctx, ConsoleOptions{PageOptions: page, Limit: 100})
	writeCapture("errors", errorsResult, errorsErr)
	network, networkErr := mgr.NetworkList(ctx, NetworkRecorderOptions{PageOptions: page, Limit: 1000, Status: -1, Body: true, MaxBodyBytes: 20000})
	writeCapture("network", network, networkErr)
	screenshotPath := filepath.Join(dir, "final.png")
	screenshot, screenshotErr := mgr.Screenshot(ctx, ScreenshotOptions{PageOptions: page, OutPath: screenshotPath, FullPage: true})
	writeCapture("final-screenshot", screenshot, screenshotErr)
	if screenshotErr == nil {
		artifacts["final-screenshot-png"] = screenshotPath
	}
	manifest := workflowEvidenceManifest{
		Session:     config.SessionName,
		TargetID:    config.TargetID,
		Workflow:    *result,
		Artifacts:   artifacts,
		Captures:    captures,
		GeneratedAt: mgr.now(),
		Limitation:  "Evidence bundles contain redacted browser metadata and screenshot file paths. Screenshots may still show visible page content.",
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := writeWorkflowJSONFile(manifestPath, manifest); err != nil {
		return err
	}
	stat, err := os.Stat(manifestPath)
	if err != nil {
		return NewError("artifact_write_failed", err.Error(), "Evidence manifest was written but metadata could not be read.", 500)
	}
	result.EvidenceDir = dir
	result.EvidenceManifest = manifestPath
	result.EvidenceBytes = stat.Size()
	return nil
}

func writeWorkflowJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return NewError("automation_failed", err.Error(), "Evidence artifact could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Evidence artifact could not be written.", 500)
	}
	return nil
}

func writeWorkflowReport(path string, result *WorkflowRunResult) error {
	path = strings.TrimSpace(path)
	if path == "" || result == nil {
		return nil
	}
	path = filepath.Clean(expandHome(path))
	if path == "" || path == "." {
		return invalidArgs("--report-out must point at a JSON file", "Pass a writable report path such as result/workflow-run.json.")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Check --report-out directory permissions.", 500)
	}
	result.ReportPath = path
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return NewError("automation_failed", err.Error(), "Workflow report could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Workflow report could not be written.", 500)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return NewError("artifact_write_failed", err.Error(), "Workflow report was written but metadata could not be read.", 500)
	}
	result.ReportBytes = stat.Size()
	return nil
}

func workflowStepError(err error) *WorkflowStepError {
	var autoErr *Error
	if errors.As(err, &autoErr) {
		return &WorkflowStepError{
			Code:        autoErr.Code,
			Message:     RedactError(autoErr.Message),
			Hint:        autoErr.Hint,
			Status:      autoErr.Status,
			NextActions: browserAutomationNextActions(autoErr.Code),
		}
	}
	var assertErr *AssertionError
	if errors.As(err, &assertErr) && assertErr.Base != nil {
		return &WorkflowStepError{
			Code:        assertErr.Base.Code,
			Message:     RedactError(assertErr.Base.Message),
			Hint:        assertErr.Base.Hint,
			Status:      assertErr.Base.Status,
			NextActions: browserAutomationNextActions(assertErr.Base.Code),
		}
	}
	return &WorkflowStepError{Code: "automation_failed", Message: RedactError(err.Error()), Status: 500, NextActions: browserAutomationNextActions("automation_failed")}
}

func browserAutomationNextActions(code string) []string {
	switch code {
	case "assertion_failed":
		return []string{"Run browser page ax --json if refs may be stale.", "Add or tune a page.wait step before the assertion.", "Inspect browser page screenshot metadata or page snapshot output."}
	case "selector_not_found", "target_not_found":
		return []string{"Run browser tab list --json and browser page ax --json.", "Prefer a fresh --ref from page ax when selectors are unstable.", "Add browser page wait before the action."}
	case "session_not_running", "devtools_unavailable":
		return []string{"Run browser session status --json.", "Restart with browser session start --json or attach an explicit DevTools port.", "Verify the browser was launched with remote debugging on 127.0.0.1."}
	case "human_action_disabled":
		return []string{"Rerun with --allow-human only when the workflow intentionally pauses for manual browser interaction."}
	case "human_confirmation_required":
		return []string{"Ask the user to inspect the browser.", "Rerun with --allow-human --yes only after explicit user confirmation."}
	case "risky_action_requires_confirmation":
		return []string{"Ask the user to confirm the risky browser action.", "Rerun the page click with --yes or the workflow with --yes only after confirmation.", "Prefer inserting a human.confirm step before the risky action."}
	case "workflow_invalid", "invalid_args":
		return []string{"Run browser schema workflow.run --json.", "Use browser workflow run --dry-run --json before executing."}
	default:
		return []string{"Inspect data.steps for the failing step.", "Rerun browser workflow run --dry-run --json to validate the plan.", "Use browser page snapshot/ax/console/errors for diagnosis."}
	}
}

func workflowConfig(def WorkflowDefinition, opts WorkflowRunOptions) WorkflowConfig {
	session := strings.TrimSpace(def.SessionName)
	if session == "" {
		session = DefaultSessionName
	}
	if opts.SessionOverride && strings.TrimSpace(opts.SessionName) != "" {
		session = strings.TrimSpace(opts.SessionName)
	}
	targetID := strings.TrimSpace(def.TargetID)
	if opts.TargetOverride {
		targetID = strings.TrimSpace(opts.TargetID)
	}
	timeout := def.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if opts.TimeoutOverride && opts.TimeoutSeconds > 0 {
		timeout = opts.TimeoutSeconds
	}
	continueOnError := def.ContinueOnError
	if opts.ContinueOverride {
		continueOnError = opts.ContinueOnError
	}
	return WorkflowConfig{
		SessionName:     session,
		TargetID:        targetID,
		TimeoutSeconds:  timeout,
		ContinueOnError: continueOnError,
		AllowHuman:      opts.AllowHuman,
		AutoConfirm:     opts.AutoConfirm,
		SmartWait:       def.SmartWait,
	}
}

func normalizeWorkflowDefinition(def WorkflowDefinition) WorkflowDefinition {
	def.SessionName = strings.TrimSpace(def.SessionName)
	def.TargetID = strings.TrimSpace(def.TargetID)
	if def.Vars == nil {
		def.Vars = map[string]string{}
	}
	def.SmartWait = normalizeWorkflowSmartWait(def.SmartWait)
	for i := range def.Steps {
		def.Steps[i] = normalizeWorkflowStep(def.Steps[i])
	}
	return def
}

func normalizeWorkflowStep(step WorkflowStep) WorkflowStep {
	step.Action = strings.ToLower(strings.TrimSpace(step.Action))
	step.Name = TruncateBytes(RedactString(step.Name), 200)
	step.Selector = strings.TrimSpace(step.Selector)
	step.Ref = strings.TrimSpace(step.Ref)
	step.Locators = normalizeWorkflowLocators(step.Locators)
	step.URL = strings.TrimSpace(step.URL)
	step.URLContains = strings.TrimSpace(step.URLContains)
	step.File = strings.TrimSpace(step.File)
	step.Baseline = strings.TrimSpace(step.Baseline)
	step.DiffOut = strings.TrimSpace(step.DiffOut)
	step.Format = strings.ToLower(strings.TrimSpace(step.Format))
	step.Key = strings.TrimSpace(step.Key)
	step.Out = strings.TrimSpace(step.Out)
	step.Filter = strings.TrimSpace(step.Filter)
	step.Method = strings.TrimSpace(step.Method)
	step.Level = strings.ToLower(strings.TrimSpace(step.Level))
	step.If = strings.TrimSpace(step.If)
	step.As = strings.TrimSpace(step.As)
	step.Prompt = strings.TrimSpace(step.Prompt)
	step.SmartWait = normalizeWorkflowSmartWait(step.SmartWait)
	if !step.HasEquals {
		step.Equals = -1
	}
	if !step.HasMin {
		step.Min = -1
	}
	if !step.HasMax {
		step.Max = -1
	}
	if !step.HasIndex {
		step.Index = -1
	}
	if !step.HasThreshold {
		step.Threshold = 0
	}
	return step
}

func workflowVars(def WorkflowDefinition, opts WorkflowRunOptions) (map[string]string, error) {
	vars := map[string]string{}
	for key, value := range def.Vars {
		key = normalizeWorkflowVarName(key)
		if key == "" {
			return nil, invalidWorkflow("variable names must be non-empty", "Use variable names like query or row_id.")
		}
		vars[key] = value
	}
	for _, raw := range opts.VarOverrides {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, invalidArgs("--var must be name=value", "Pass repeated --var flags such as --var query=abc.")
		}
		key = normalizeWorkflowVarName(key)
		if key == "" {
			return nil, invalidArgs("--var name must be non-empty", "Pass repeated --var flags such as --var query=abc.")
		}
		vars[key] = value
	}
	return vars, nil
}

func expandWorkflow(def WorkflowDefinition, vars map[string]string) (WorkflowDefinition, error) {
	def.SessionName = expandWorkflowString(def.SessionName, vars)
	def.TargetID = expandWorkflowString(def.TargetID, vars)
	def.SmartWait = expandWorkflowSmartWait(def.SmartWait, vars)
	expanded := make([]WorkflowStep, 0, len(def.Steps))
	for _, step := range def.Steps {
		as := normalizeWorkflowVarName(step.As)
		if as == "" {
			as = "item"
		}
		items := step.ForEach
		if len(items) == 0 {
			var err error
			next, err := expandWorkflowStep(step, vars)
			if err != nil {
				return WorkflowDefinition{}, err
			}
			expanded = append(expanded, next)
			continue
		}
		for _, item := range items {
			localVars := copyWorkflowVars(vars)
			localVars[as] = expandWorkflowString(item, vars)
			if as != "item" {
				localVars["item"] = localVars[as]
			}
			next, err := expandWorkflowStep(step, localVars)
			if err != nil {
				return WorkflowDefinition{}, err
			}
			next.ForEach = nil
			expanded = append(expanded, next)
		}
	}
	def.Steps = expanded
	return def, nil
}

func expandWorkflowStep(step WorkflowStep, vars map[string]string) (WorkflowStep, error) {
	var missing []string
	expand := func(raw string) string {
		return workflowTemplatePattern.ReplaceAllStringFunc(raw, func(match string) string {
			name := workflowTemplatePattern.FindStringSubmatch(match)[1]
			value, ok := lookupWorkflowVar(name, vars)
			if !ok {
				missing = append(missing, name)
				return ""
			}
			return value
		})
	}
	step.Action = expand(step.Action)
	step.Name = expand(step.Name)
	step.Selector = expand(step.Selector)
	step.Ref = expand(step.Ref)
	step.Locators = expandWorkflowLocators(step.Locators, vars)
	step.Contains = expand(step.Contains)
	step.URL = expand(step.URL)
	step.URLContains = expand(step.URLContains)
	step.File = expand(step.File)
	step.Baseline = expand(step.Baseline)
	step.DiffOut = expand(step.DiffOut)
	step.Format = expand(step.Format)
	step.Text = expand(step.Text)
	step.Key = expand(step.Key)
	step.Value = expand(step.Value)
	step.Label = expand(step.Label)
	step.Out = expand(step.Out)
	step.Filter = expand(step.Filter)
	step.Method = expand(step.Method)
	step.Level = expand(step.Level)
	step.If = expand(step.If)
	step.Prompt = expand(step.Prompt)
	step.SmartWait = expandWorkflowSmartWait(step.SmartWait, vars)
	if len(missing) > 0 {
		return WorkflowStep{}, invalidWorkflow("undefined workflow variable "+missing[0], "Define it under vars: or pass --var "+missing[0]+"=value.")
	}
	step = normalizeWorkflowStep(step)
	if !workflowConditionPass(step.If) {
		step.Skip = true
		step.SkipReason = "if condition evaluated false"
	}
	return step, nil
}

func copyWorkflowVars(vars map[string]string) map[string]string {
	out := make(map[string]string, len(vars))
	for key, value := range vars {
		out[key] = value
	}
	return out
}

func lookupWorkflowVar(raw string, vars map[string]string) (string, bool) {
	name := normalizeWorkflowVarName(raw)
	if strings.HasPrefix(name, "vars.") {
		name = strings.TrimPrefix(name, "vars.")
	}
	value, ok := vars[name]
	return value, ok
}

func normalizeWorkflowVarName(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), ".")
}

func workflowConditionPass(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return true
	}
	switch value {
	case "0", "false", "no", "off", "null", "nil", `""`, "''":
		return false
	default:
		return true
	}
}

func normalizeWorkflowSmartWait(wait WorkflowSmartWait) WorkflowSmartWait {
	wait.Selector = strings.TrimSpace(wait.Selector)
	wait.URLContains = strings.TrimSpace(wait.URLContains)
	wait.Text = strings.TrimSpace(wait.Text)
	if wait.DurationMilliseconds < 0 {
		wait.DurationMilliseconds = 0
	}
	if wait.NetworkIdleMilliseconds < 0 {
		wait.NetworkIdleMilliseconds = 0
	}
	if wait.DOMStableMilliseconds < 0 {
		wait.DOMStableMilliseconds = 0
	}
	return wait
}

func expandWorkflowSmartWait(wait WorkflowSmartWait, vars map[string]string) WorkflowSmartWait {
	wait.Selector = expandWorkflowString(wait.Selector, vars)
	wait.URLContains = expandWorkflowString(wait.URLContains, vars)
	wait.Text = expandWorkflowString(wait.Text, vars)
	return normalizeWorkflowSmartWait(wait)
}

func normalizeWorkflowLocators(raw []ElementLocator) []ElementLocator {
	locators := sanitizeElementLocators(raw)
	if len(locators) == 0 {
		return nil
	}
	return locators
}

func expandWorkflowLocators(raw []ElementLocator, vars map[string]string) []ElementLocator {
	if len(raw) == 0 {
		return nil
	}
	out := make([]ElementLocator, 0, len(raw))
	for _, locator := range raw {
		locator.Selector = expandWorkflowString(locator.Selector, vars)
		locator.Role = expandWorkflowString(locator.Role, vars)
		locator.Name = expandWorkflowString(locator.Name, vars)
		locator.Text = expandWorkflowString(locator.Text, vars)
		locator.Label = expandWorkflowString(locator.Label, vars)
		locator.Placeholder = expandWorkflowString(locator.Placeholder, vars)
		locator.NearText = expandWorkflowString(locator.NearText, vars)
		locator = normalizeElementLocator(locator)
		if hasElementLocator(locator) {
			out = append(out, locator)
		}
	}
	return out
}

func expandWorkflowString(raw string, vars map[string]string) string {
	return workflowTemplatePattern.ReplaceAllStringFunc(raw, func(match string) string {
		name := workflowTemplatePattern.FindStringSubmatch(match)[1]
		if value, ok := lookupWorkflowVar(name, vars); ok {
			return value
		}
		return ""
	})
}

func workflowSmartWaitEnabled(wait WorkflowSmartWait) bool {
	return strings.TrimSpace(wait.Selector) != "" ||
		strings.TrimSpace(wait.URLContains) != "" ||
		strings.TrimSpace(wait.Text) != "" ||
		wait.DurationMilliseconds > 0 ||
		wait.NetworkIdleMilliseconds > 0 ||
		wait.DOMStableMilliseconds > 0
}

func validateWorkflowStep(step WorkflowStep) error {
	if step.Skip {
		return nil
	}
	if strings.TrimSpace(step.Action) == "" {
		return invalidWorkflow("action is required", "Pass action: <allowed-action> for each step.")
	}
	if !workflowAllowedActions[step.Action] {
		return invalidWorkflow("unsupported action "+step.Action, "Workflows only support the documented browser action whitelist.")
	}
	switch step.Action {
	case "tab.open":
		return validateHTTPURL(step.URL, "url")
	case "page.wait":
		return validateWaitOptions(WaitOptions{
			Selector:                step.Selector,
			DurationMilliseconds:    step.DurationMilliseconds,
			URLContains:             step.URLContains,
			Text:                    step.Text,
			NetworkIdleMilliseconds: step.NetworkIdleMilliseconds,
			DOMStableMilliseconds:   step.DOMStableMilliseconds,
		})
	case "page.click", "page.type", "page.check", "page.uncheck", "assert.visible":
		if err := validateWorkflowActionTarget(step); err != nil {
			return err
		}
		if step.Action == "page.type" && strings.TrimSpace(step.Text) == "" {
			return invalidArgs("text is required", "Pass text for page.type; workflow output reports only byte count.")
		}
		return nil
	case "page.press":
		if _, err := NormalizePressKey(step.Key); err != nil {
			return err
		}
		return validateWorkflowOptionalActionTarget(step)
	case "page.select":
		_, err := validateWorkflowSelectOptions(step)
		return err
	case "page.screenshot":
		return nil
	case "page.extract_schema":
		if strings.TrimSpace(step.File) == "" {
			return invalidArgs("file is required", "Pass file for page.extract_schema.")
		}
		return nil
	case "page.metrics":
		return nil
	case "assert.text":
		if strings.TrimSpace(step.Contains) == "" {
			return invalidArgs("contains is required", "Pass contains for assert.text.")
		}
		return validateWorkflowOptionalActionTarget(step)
	case "assert.url":
		if strings.TrimSpace(step.Contains) == "" {
			return invalidArgs("contains is required", "Pass contains for assert.url.")
		}
		return nil
	case "assert.count":
		return validateCountAssertionOptions(workflowCountAssertionOptions(PageOptions{}, step))
	case "assert.screenshot":
		if strings.TrimSpace(step.Baseline) == "" || strings.TrimSpace(step.DiffOut) == "" {
			return invalidArgs("baseline and diff_out are required", "Pass baseline and diff_out for assert.screenshot.")
		}
		return validateWorkflowOptionalActionTarget(step)
	case "network.wait":
		if strings.TrimSpace(step.URLContains) == "" {
			return invalidArgs("url_contains is required", "Pass url_contains for network.wait.")
		}
		return nil
	case "network.start", "network.list", "page.console", "page.errors", "form.inspect":
		return nil
	case "network.export":
		if strings.TrimSpace(step.Out) == "" {
			return invalidArgs("out is required", "Pass out for network.export.")
		}
		return nil
	case "form.fill":
		if strings.TrimSpace(step.File) == "" {
			return invalidArgs("file is required", "Pass file for form.fill.")
		}
		return nil
	case "human.wait":
		if step.DurationMilliseconds <= 0 {
			return invalidArgs("duration_ms is required for human.wait", "Pass a bounded duration_ms while the user manually interacts.")
		}
		return nil
	case "human.confirm":
		return nil
	default:
		return nil
	}
}

func workflowStepPlan(index int, step WorkflowStep) WorkflowStepPlan {
	plan := WorkflowStepPlan{
		Index:                   index,
		Name:                    step.Name,
		Action:                  step.Action,
		Selector:                normalizeSelectorHint(step.Selector),
		Ref:                     RedactString(step.Ref),
		LocatorCount:            len(step.Locators),
		URL:                     RedactURL(step.URL),
		URLContainsPreview:      TruncateBytes(RedactString(step.URLContains), 500),
		URLContainsBytes:        len(step.URLContains),
		File:                    RedactString(step.File),
		Baseline:                RedactString(step.Baseline),
		DiffOut:                 RedactString(step.DiffOut),
		Format:                  RedactString(step.Format),
		ContainsPreview:         TruncateBytes(RedactString(step.Contains), 500),
		ContainsBytes:           len(step.Contains),
		HasText:                 step.Text != "",
		TextBytes:               len(step.Text),
		Clear:                   step.Clear,
		Key:                     RedactString(step.Key),
		Out:                     RedactString(step.Out),
		Filter:                  RedactString(step.Filter),
		Method:                  normalizeNetworkMethod(step.Method),
		Level:                   RedactString(step.Level),
		IfPreview:               TruncateBytes(RedactString(step.If), 500),
		ForEachCount:            len(step.ForEach),
		As:                      RedactString(step.As),
		PromptPreview:           TruncateBytes(RedactString(step.Prompt), 500),
		PromptBytes:             len(step.Prompt),
		Skipped:                 step.Skip,
		SkipReason:              step.SkipReason,
		Not:                     step.Not,
		Limit:                   step.Limit,
		Status:                  statusForOutput(step.Status),
		DurationMilliseconds:    step.DurationMilliseconds,
		NetworkIdleMilliseconds: step.NetworkIdleMilliseconds,
		DOMStableMilliseconds:   step.DOMStableMilliseconds,
	}
	if step.HasEquals {
		value := step.Equals
		plan.Equals = &value
	}
	if step.HasMin {
		value := step.Min
		plan.Min = &value
	}
	if step.HasMax {
		value := step.Max
		plan.Max = &value
	}
	if step.HasIndex {
		value := step.Index
		plan.IndexValue = &value
	}
	if step.HasThreshold {
		value := step.Threshold
		plan.Threshold = &value
	}
	if step.FullPageSet || step.Action == "page.screenshot" {
		value := workflowFullPage(step)
		plan.FullPage = &value
	}
	if workflowSmartWaitEnabled(step.SmartWait) {
		value := step.SmartWait
		value.Text = TruncateBytes(RedactString(value.Text), 500)
		value.URLContains = TruncateBytes(RedactString(value.URLContains), 500)
		plan.SmartWait = &value
	}
	if strings.TrimSpace(step.Value) != "" {
		plan.SelectionMode = "value"
	} else if strings.TrimSpace(step.Label) != "" {
		plan.SelectionMode = "label"
	} else if step.HasIndex {
		plan.SelectionMode = "index"
	}
	return plan
}

func workflowCountAssertionOptions(page PageOptions, step WorkflowStep) AssertionOptions {
	opts := AssertionOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Equals: -1, Min: -1, Max: -1}
	if step.HasEquals {
		opts.Equals = step.Equals
	}
	if step.HasMin {
		opts.Min = step.Min
	}
	if step.HasMax {
		opts.Max = step.Max
	}
	return opts
}

func workflowSelectIndex(step WorkflowStep) int {
	if step.HasIndex {
		return step.Index
	}
	return -1
}

func workflowFullPage(step WorkflowStep) bool {
	if step.FullPageSet {
		return step.FullPage
	}
	return true
}

func workflowStatus(status int) int {
	if status <= 0 {
		return -1
	}
	return status
}

func workflowThreshold(step WorkflowStep) float64 {
	if !step.HasThreshold {
		return 0
	}
	if step.Threshold < 0 {
		return 0
	}
	if step.Threshold > 1 {
		return 1
	}
	return step.Threshold
}

func invalidWorkflow(message, hint string) *Error {
	return NewError("workflow_invalid", message, hint, 400)
}

func (w *WorkflowDefinition) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("workflow must be a YAML mapping")
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := normalizeWorkflowKey(value.Content[i].Value)
		val := value.Content[i+1]
		switch key {
		case "session":
			if err := val.Decode(&w.SessionName); err != nil {
				return err
			}
		case "target_id":
			if err := val.Decode(&w.TargetID); err != nil {
				return err
			}
		case "timeout":
			if err := val.Decode(&w.TimeoutSeconds); err != nil {
				return err
			}
		case "continue_on_error":
			if err := val.Decode(&w.ContinueOnError); err != nil {
				return err
			}
		case "vars":
			if err := val.Decode(&w.Vars); err != nil {
				return err
			}
		case "smart_wait":
			if err := val.Decode(&w.SmartWait); err != nil {
				return err
			}
		case "steps":
			if err := val.Decode(&w.Steps); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported workflow field %q", value.Content[i].Value)
		}
	}
	return nil
}

func (s *WorkflowStep) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("workflow step must be a YAML mapping")
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := normalizeWorkflowKey(value.Content[i].Value)
		val := value.Content[i+1]
		switch key {
		case "action":
			if err := val.Decode(&s.Action); err != nil {
				return err
			}
		case "name":
			if err := val.Decode(&s.Name); err != nil {
				return err
			}
		case "selector":
			if err := val.Decode(&s.Selector); err != nil {
				return err
			}
		case "ref":
			if err := val.Decode(&s.Ref); err != nil {
				return err
			}
		case "locators":
			if err := val.Decode(&s.Locators); err != nil {
				return err
			}
		case "contains":
			if err := val.Decode(&s.Contains); err != nil {
				return err
			}
		case "url":
			if err := val.Decode(&s.URL); err != nil {
				return err
			}
		case "url_contains":
			if err := val.Decode(&s.URLContains); err != nil {
				return err
			}
		case "file":
			if err := val.Decode(&s.File); err != nil {
				return err
			}
		case "baseline":
			if err := val.Decode(&s.Baseline); err != nil {
				return err
			}
		case "diff_out":
			if err := val.Decode(&s.DiffOut); err != nil {
				return err
			}
		case "format":
			if err := val.Decode(&s.Format); err != nil {
				return err
			}
		case "text":
			if err := val.Decode(&s.Text); err != nil {
				return err
			}
		case "key":
			if err := val.Decode(&s.Key); err != nil {
				return err
			}
		case "value":
			if err := val.Decode(&s.Value); err != nil {
				return err
			}
		case "label":
			if err := val.Decode(&s.Label); err != nil {
				return err
			}
		case "out":
			if err := val.Decode(&s.Out); err != nil {
				return err
			}
		case "filter":
			if err := val.Decode(&s.Filter); err != nil {
				return err
			}
		case "method":
			if err := val.Decode(&s.Method); err != nil {
				return err
			}
		case "level":
			if err := val.Decode(&s.Level); err != nil {
				return err
			}
		case "if":
			if err := val.Decode(&s.If); err != nil {
				return err
			}
		case "for_each":
			values, err := decodeWorkflowStringSlice(val)
			if err != nil {
				return err
			}
			s.ForEach = values
		case "as":
			if err := val.Decode(&s.As); err != nil {
				return err
			}
		case "prompt":
			if err := val.Decode(&s.Prompt); err != nil {
				return err
			}
		case "smart_wait":
			if err := val.Decode(&s.SmartWait); err != nil {
				return err
			}
		case "not":
			if err := val.Decode(&s.Not); err != nil {
				return err
			}
		case "clear":
			if err := val.Decode(&s.Clear); err != nil {
				return err
			}
		case "full_page":
			if err := val.Decode(&s.FullPage); err != nil {
				return err
			}
			s.FullPageSet = true
		case "equals":
			if err := val.Decode(&s.Equals); err != nil {
				return err
			}
			s.HasEquals = true
		case "min":
			if err := val.Decode(&s.Min); err != nil {
				return err
			}
			s.HasMin = true
		case "max":
			if err := val.Decode(&s.Max); err != nil {
				return err
			}
			s.HasMax = true
		case "index":
			if err := val.Decode(&s.Index); err != nil {
				return err
			}
			s.HasIndex = true
		case "limit":
			if err := val.Decode(&s.Limit); err != nil {
				return err
			}
		case "status":
			if err := val.Decode(&s.Status); err != nil {
				return err
			}
		case "threshold":
			if err := val.Decode(&s.Threshold); err != nil {
				return err
			}
			s.HasThreshold = true
		case "duration_ms":
			if err := val.Decode(&s.DurationMilliseconds); err != nil {
				return err
			}
		case "network_idle_ms":
			if err := val.Decode(&s.NetworkIdleMilliseconds); err != nil {
				return err
			}
		case "dom_stable_ms":
			if err := val.Decode(&s.DOMStableMilliseconds); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported workflow step field %q", value.Content[i].Value)
		}
	}
	return nil
}

func (w *WorkflowSmartWait) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var enabled bool
		if err := value.Decode(&enabled); err != nil {
			return fmt.Errorf("smart_wait must be a mapping")
		}
		if !enabled {
			*w = WorkflowSmartWait{}
			return nil
		}
		w.NetworkIdleMilliseconds = 500
		w.DOMStableMilliseconds = 500
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("smart_wait must be a mapping")
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := normalizeWorkflowKey(value.Content[i].Value)
		val := value.Content[i+1]
		switch key {
		case "selector":
			if err := val.Decode(&w.Selector); err != nil {
				return err
			}
		case "url_contains":
			if err := val.Decode(&w.URLContains); err != nil {
				return err
			}
		case "text":
			if err := val.Decode(&w.Text); err != nil {
				return err
			}
		case "duration_ms":
			if err := val.Decode(&w.DurationMilliseconds); err != nil {
				return err
			}
		case "network_idle_ms":
			if err := val.Decode(&w.NetworkIdleMilliseconds); err != nil {
				return err
			}
		case "dom_stable_ms":
			if err := val.Decode(&w.DOMStableMilliseconds); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported smart_wait field %q", value.Content[i].Value)
		}
	}
	return nil
}

func decodeWorkflowStringSlice(value *yaml.Node) ([]string, error) {
	switch value.Kind {
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			var s string
			if err := item.Decode(&s); err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	case yaml.ScalarNode:
		var raw string
		if err := value.Decode(&raw); err != nil {
			return nil, err
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("for_each must be a string or sequence")
	}
}

func normalizeWorkflowKey(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
}
