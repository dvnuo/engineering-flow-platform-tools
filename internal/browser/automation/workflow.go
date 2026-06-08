package automation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const workflowFailureCode = "workflow_failed"

var workflowAllowedActions = map[string]bool{
	"tab.open":        true,
	"page.wait":       true,
	"page.click":      true,
	"page.type":       true,
	"page.press":      true,
	"page.select":     true,
	"page.check":      true,
	"page.uncheck":    true,
	"page.screenshot": true,
	"assert.visible":  true,
	"assert.text":     true,
	"assert.url":      true,
	"assert.count":    true,
	"network.start":   true,
	"network.wait":    true,
	"network.list":    true,
	"page.console":    true,
	"page.errors":     true,
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
}

type WorkflowDefinition struct {
	SessionName     string         `json:"session,omitempty"`
	TargetID        string         `json:"target_id,omitempty"`
	TimeoutSeconds  int            `json:"timeout,omitempty"`
	ContinueOnError bool           `json:"continue_on_error,omitempty"`
	Steps           []WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	Action                  string `json:"action"`
	Name                    string `json:"name,omitempty"`
	Selector                string `json:"selector,omitempty"`
	Ref                     string `json:"ref,omitempty"`
	Contains                string `json:"contains,omitempty"`
	URL                     string `json:"url,omitempty"`
	URLContains             string `json:"url_contains,omitempty"`
	Text                    string `json:"-"`
	Key                     string `json:"key,omitempty"`
	Value                   string `json:"-"`
	Label                   string `json:"-"`
	Out                     string `json:"out,omitempty"`
	Filter                  string `json:"filter,omitempty"`
	Method                  string `json:"method,omitempty"`
	Level                   string `json:"level,omitempty"`
	Not                     bool   `json:"not,omitempty"`
	Clear                   bool   `json:"clear,omitempty"`
	FullPage                bool   `json:"full_page,omitempty"`
	FullPageSet             bool   `json:"-"`
	Equals                  int    `json:"equals,omitempty"`
	Min                     int    `json:"min,omitempty"`
	Max                     int    `json:"max,omitempty"`
	Index                   int    `json:"index,omitempty"`
	Limit                   int    `json:"limit,omitempty"`
	Status                  int    `json:"status,omitempty"`
	DurationMilliseconds    int    `json:"duration_ms,omitempty"`
	NetworkIdleMilliseconds int    `json:"network_idle_ms,omitempty"`
	DOMStableMilliseconds   int    `json:"dom_stable_ms,omitempty"`
	HasEquals               bool   `json:"-"`
	HasMin                  bool   `json:"-"`
	HasMax                  bool   `json:"-"`
	HasIndex                bool   `json:"-"`
}

type WorkflowConfig struct {
	SessionName     string `json:"session"`
	TargetID        string `json:"target_id,omitempty"`
	TimeoutSeconds  int    `json:"timeout"`
	ContinueOnError bool   `json:"continue_on_error,omitempty"`
}

type WorkflowStepPlan struct {
	Index                   int    `json:"index"`
	Name                    string `json:"name,omitempty"`
	Action                  string `json:"action"`
	Selector                string `json:"selector,omitempty"`
	Ref                     string `json:"ref,omitempty"`
	URL                     string `json:"url,omitempty"`
	URLContainsPreview      string `json:"url_contains_preview,omitempty"`
	URLContainsBytes        int    `json:"url_contains_bytes,omitempty"`
	ContainsPreview         string `json:"contains_preview,omitempty"`
	ContainsBytes           int    `json:"contains_bytes,omitempty"`
	HasText                 bool   `json:"has_text,omitempty"`
	TextBytes               int    `json:"text_bytes,omitempty"`
	Clear                   bool   `json:"clear,omitempty"`
	Key                     string `json:"key,omitempty"`
	SelectionMode           string `json:"selection_mode,omitempty"`
	Out                     string `json:"out,omitempty"`
	Filter                  string `json:"filter,omitempty"`
	Method                  string `json:"method,omitempty"`
	Level                   string `json:"level,omitempty"`
	Not                     bool   `json:"not,omitempty"`
	FullPage                *bool  `json:"full_page,omitempty"`
	Equals                  *int   `json:"equals,omitempty"`
	Min                     *int   `json:"min,omitempty"`
	Max                     *int   `json:"max,omitempty"`
	IndexValue              *int   `json:"index_value,omitempty"`
	Limit                   int    `json:"limit,omitempty"`
	Status                  int    `json:"status,omitempty"`
	DurationMilliseconds    int    `json:"duration_ms,omitempty"`
	NetworkIdleMilliseconds int    `json:"network_idle_ms,omitempty"`
	DOMStableMilliseconds   int    `json:"dom_stable_ms,omitempty"`
}

type WorkflowStepError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Status  int    `json:"status,omitempty"`
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
			result.Steps = append(result.Steps, WorkflowStepResult{
				Index:  i,
				Name:   plan.Name,
				Action: plan.Action,
				Status: "planned",
				Pass:   true,
				Plan:   plan,
			})
		}
		result.CompletedSteps = len(result.Steps)
		result.DurationMilliseconds = elapsedMilliseconds(start)
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
		return mgr.Click(ctx, ClickOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref})
	case "page.type":
		return mgr.Type(ctx, TypeOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Text: step.Text, Clear: step.Clear})
	case "page.press":
		return mgr.Press(ctx, PressOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Key: step.Key})
	case "page.select":
		return mgr.Select(ctx, SelectOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Value: step.Value, Label: step.Label, Index: workflowSelectIndex(step)})
	case "page.check":
		return mgr.Check(ctx, CheckOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Checked: true})
	case "page.uncheck":
		return mgr.Check(ctx, CheckOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Checked: false})
	case "page.screenshot":
		return mgr.Screenshot(ctx, ScreenshotOptions{PageOptions: page, OutPath: step.Out, FullPage: workflowFullPage(step), FullPageSet: step.FullPageSet})
	case "assert.visible":
		return mgr.AssertVisible(ctx, AssertionOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	case "assert.text":
		return mgr.AssertText(ctx, AssertionOptions{PageOptions: page, Selector: step.Selector, Ref: step.Ref, Contains: step.Contains, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	case "assert.url":
		return mgr.AssertURL(ctx, AssertionOptions{PageOptions: page, Contains: step.Contains, Not: step.Not, Equals: -1, Min: -1, Max: -1})
	case "assert.count":
		return mgr.AssertCount(ctx, workflowCountAssertionOptions(page, step))
	case "network.start":
		return mgr.NetworkStart(ctx, NetworkRecorderOptions{PageOptions: page, Filter: step.Filter, Limit: step.Limit, Status: -1})
	case "network.wait":
		return mgr.NetworkWait(ctx, NetworkWaitOptions{NetworkRecorderOptions: NetworkRecorderOptions{PageOptions: page, Limit: step.Limit, Method: step.Method, Status: workflowStatus(step.Status)}, URLContains: step.URLContains})
	case "network.list":
		return mgr.NetworkList(ctx, NetworkRecorderOptions{PageOptions: page, Filter: step.Filter, Limit: step.Limit, Method: step.Method, Status: workflowStatus(step.Status)})
	case "page.console":
		return mgr.Console(ctx, ConsoleOptions{PageOptions: page, Level: step.Level, Limit: step.Limit})
	case "page.errors":
		return mgr.RuntimeErrors(ctx, ConsoleOptions{PageOptions: page, Limit: step.Limit})
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

func workflowStepError(err error) *WorkflowStepError {
	var autoErr *Error
	if errors.As(err, &autoErr) {
		return &WorkflowStepError{
			Code:    autoErr.Code,
			Message: RedactError(autoErr.Message),
			Hint:    autoErr.Hint,
			Status:  autoErr.Status,
		}
	}
	var assertErr *AssertionError
	if errors.As(err, &assertErr) && assertErr.Base != nil {
		return &WorkflowStepError{
			Code:    assertErr.Base.Code,
			Message: RedactError(assertErr.Base.Message),
			Hint:    assertErr.Base.Hint,
			Status:  assertErr.Base.Status,
		}
	}
	return &WorkflowStepError{Code: "automation_failed", Message: RedactError(err.Error()), Status: 500}
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
	return WorkflowConfig{SessionName: session, TargetID: targetID, TimeoutSeconds: timeout, ContinueOnError: continueOnError}
}

func normalizeWorkflowDefinition(def WorkflowDefinition) WorkflowDefinition {
	def.SessionName = strings.TrimSpace(def.SessionName)
	def.TargetID = strings.TrimSpace(def.TargetID)
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
	step.URL = strings.TrimSpace(step.URL)
	step.URLContains = strings.TrimSpace(step.URLContains)
	step.Key = strings.TrimSpace(step.Key)
	step.Out = strings.TrimSpace(step.Out)
	step.Filter = strings.TrimSpace(step.Filter)
	step.Method = strings.TrimSpace(step.Method)
	step.Level = strings.ToLower(strings.TrimSpace(step.Level))
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
	return step
}

func validateWorkflowStep(step WorkflowStep) error {
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
		if err := validateActionTarget(step.Selector, step.Ref, step.Action); err != nil {
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
		return validateOptionalActionTarget(step.Selector, step.Ref, step.Action)
	case "page.select":
		_, err := validateSelectOptions(SelectOptions{Selector: step.Selector, Ref: step.Ref, Value: step.Value, Label: step.Label, Index: workflowSelectIndex(step)})
		return err
	case "page.screenshot":
		return nil
	case "assert.text":
		if strings.TrimSpace(step.Contains) == "" {
			return invalidArgs("contains is required", "Pass contains for assert.text.")
		}
		return validateOptionalActionTarget(step.Selector, step.Ref, step.Action)
	case "assert.url":
		if strings.TrimSpace(step.Contains) == "" {
			return invalidArgs("contains is required", "Pass contains for assert.url.")
		}
		return nil
	case "assert.count":
		return validateCountAssertionOptions(workflowCountAssertionOptions(PageOptions{}, step))
	case "network.wait":
		if strings.TrimSpace(step.URLContains) == "" {
			return invalidArgs("url_contains is required", "Pass url_contains for network.wait.")
		}
		return nil
	case "network.start", "network.list", "page.console", "page.errors":
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
		URL:                     RedactURL(step.URL),
		URLContainsPreview:      TruncateBytes(RedactString(step.URLContains), 500),
		URLContainsBytes:        len(step.URLContains),
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
	if step.FullPageSet || step.Action == "page.screenshot" {
		value := workflowFullPage(step)
		plan.FullPage = &value
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

func normalizeWorkflowKey(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "_")
}
