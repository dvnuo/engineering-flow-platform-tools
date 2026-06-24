package commands

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type mobileTestSuite struct {
	Name          string             `json:"name,omitempty" yaml:"name,omitempty"`
	Variables     map[string]string  `json:"variables,omitempty" yaml:"variables,omitempty"`
	SecretsEnv    map[string]string  `json:"secrets_env,omitempty" yaml:"secrets_env,omitempty"`
	StopOnFailure bool               `json:"stop_on_failure,omitempty" yaml:"stop_on_failure,omitempty"`
	Before        []workflowStep     `json:"before,omitempty" yaml:"before,omitempty"`
	After         []workflowStep     `json:"after,omitempty" yaml:"after,omitempty"`
	Matrix        []mobileTestMatrix `json:"matrix,omitempty" yaml:"matrix,omitempty"`
	Cases         []mobileTestCase   `json:"cases" yaml:"cases"`
}

type mobileTestMatrix struct {
	Name      string            `json:"name,omitempty" yaml:"name,omitempty"`
	Variables map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type mobileTestCase struct {
	Name              string            `json:"name" yaml:"name"`
	Tags              []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Variables         map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	ContinueOnFailure bool              `json:"continue_on_failure,omitempty" yaml:"continue_on_failure,omitempty"`
	Steps             []workflowStep    `json:"steps" yaml:"steps"`
}

type mobileTestCaseResult struct {
	Name         string           `json:"name"`
	Matrix       string           `json:"matrix,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	Passed       bool             `json:"passed"`
	Skipped      bool             `json:"skipped,omitempty"`
	DurationMS   int64            `json:"duration_ms"`
	RunID        string           `json:"run_id,omitempty"`
	Steps        []map[string]any `json:"steps,omitempty"`
	Error        any              `json:"error,omitempty"`
	EvidencePath string           `json:"evidence_path,omitempty"`
}

func testCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.AddCommand(testRunCmd(o))
	return c
}

func testRunCmd(o *Opts) *cobra.Command {
	var filePath, caseFilter, reportOut, junitOut, evidenceDir string
	var tags []string
	var dryRun, continueOnFailure bool
	c := &cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, args []string) error {
		if filePath == "" {
			return print(cmd, o, output.Failure("invalid_args", "--file is required", "Pass a mobile test suite YAML file.", 400))
		}
		suite, err := readMobileTestSuite(filePath)
		if err != nil {
			return renderErr(cmd, o, err)
		}
		executions := expandMobileTestCases(suite, caseFilter, tags)
		if len(executions) == 0 {
			return print(cmd, o, output.Failure("not_found", "no test cases matched filters", "Check --case or --tag filters.", 404))
		}
		if dryRun {
			return print(cmd, o, output.Success("", map[string]any{"dry_run": true, "suite": suite.Name, "cases": redactTestPlan(executions)}))
		}
		results := make([]mobileTestCaseResult, 0, len(executions))
		passed := 0
		failed := 0
		stopOnFailure := suite.StopOnFailure && !continueOnFailure
		for _, execution := range executions {
			result := runMobileTestCase(cmd, o, suite, execution, evidenceDir)
			results = append(results, result)
			if result.Passed {
				passed++
			} else {
				failed++
				if stopOnFailure && !execution.Case.ContinueOnFailure {
					break
				}
			}
		}
		data := map[string]any{
			"suite":   suite.Name,
			"passed":  passed,
			"failed":  failed,
			"total":   passed + failed,
			"results": results,
		}
		if reportOut != "" {
			path, err := writeWorkflowReport(reportOut, data)
			if err != nil {
				return renderErr(cmd, o, err)
			}
			data["report_path"] = path
		}
		if junitOut != "" {
			if err := writeJUnitReport(junitOut, suite.Name, results); err != nil {
				return renderErr(cmd, o, err)
			}
			data["junit_path"] = junitOut
		}
		if failed > 0 {
			env := output.Failure("test_failed", "one or more mobile test cases failed", "Inspect results, JUnit, and evidence artifacts.", 412)
			env.Data = data
			return print(cmd, o, env)
		}
		return print(cmd, o, output.Success("", data))
	}}
	c.Flags().StringVar(&filePath, "file", "", "")
	c.Flags().StringVar(&caseFilter, "case", "", "")
	c.Flags().StringArrayVar(&tags, "tag", nil, "")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "")
	c.Flags().BoolVar(&continueOnFailure, "continue-on-failure", false, "")
	c.Flags().StringVar(&reportOut, "report-out", "", "")
	c.Flags().StringVar(&junitOut, "junit-out", "", "")
	c.Flags().StringVar(&evidenceDir, "evidence-dir", "", "")
	return c
}

type mobileTestExecution struct {
	Case      mobileTestCase
	Matrix    mobileTestMatrix
	Variables map[string]string
}

func readMobileTestSuite(path string) (mobileTestSuite, error) {
	var suite mobileTestSuite
	b, err := os.ReadFile(path)
	if err != nil {
		return suite, err
	}
	if err := yaml.Unmarshal(b, &suite); err != nil {
		return suite, err
	}
	if len(suite.Cases) == 0 {
		return suite, mobileError("invalid_args", "test suite has no cases", "Add at least one test case.", 400)
	}
	return suite, nil
}

func expandMobileTestCases(suite mobileTestSuite, caseFilter string, tags []string) []mobileTestExecution {
	matrix := suite.Matrix
	if len(matrix) == 0 {
		matrix = []mobileTestMatrix{{Name: ""}}
	}
	var out []mobileTestExecution
	for _, item := range matrix {
		for _, tc := range suite.Cases {
			if caseFilter != "" && tc.Name != caseFilter {
				continue
			}
			if len(tags) > 0 && !caseHasAnyTag(tc, tags) {
				continue
			}
			vars := map[string]string{}
			for k, v := range suite.Variables {
				vars[k] = v
			}
			for k, v := range suite.SecretsEnv {
				vars[k] = v
			}
			for k, v := range item.Variables {
				vars[k] = v
			}
			for k, v := range tc.Variables {
				vars[k] = v
			}
			out = append(out, mobileTestExecution{Case: tc, Matrix: item, Variables: vars})
		}
	}
	return out
}

func runMobileTestCase(cmd *cobra.Command, o *Opts, suite mobileTestSuite, execution mobileTestExecution, evidenceDir string) mobileTestCaseResult {
	start := time.Now()
	name := execution.Case.Name
	if execution.Matrix.Name != "" {
		name = execution.Matrix.Name + "/" + name
	}
	result := mobileTestCaseResult{Name: execution.Case.Name, Matrix: execution.Matrix.Name, Tags: execution.Case.Tags, Passed: true}
	currentRunID := ""
	stepIndex := 0
	var primaryError any

	currentRunID, failed, errValue := runMobileTestSteps(cmd, o, &result, suite.Before, execution.Variables, currentRunID, "before", true, &stepIndex)
	if failed && primaryError == nil {
		primaryError = errValue
	}
	if !failed {
		currentRunID, failed, errValue = runMobileTestSteps(cmd, o, &result, execution.Case.Steps, execution.Variables, currentRunID, "case", !execution.Case.ContinueOnFailure, &stepIndex)
		if failed && primaryError == nil {
			primaryError = errValue
		}
	}
	currentRunID, afterFailed, afterError := runMobileTestSteps(cmd, o, &result, suite.After, execution.Variables, currentRunID, "after", false, &stepIndex)
	_ = currentRunID
	if afterFailed {
		primaryError = mergeTestErrors(primaryError, afterError)
	}
	result.Passed = primaryError == nil
	result.Error = primaryError
	result.DurationMS = time.Since(start).Milliseconds()
	if !result.Passed && evidenceDir != "" {
		if path := writeTestEvidence(o, evidenceDir, name, result); path != "" {
			result.EvidencePath = path
		}
	}
	return result
}

func runMobileTestSteps(cmd *cobra.Command, o *Opts, result *mobileTestCaseResult, steps []workflowStep, vars map[string]string, currentRunID, phase string, stopOnFailure bool, stepIndex *int) (string, bool, any) {
	failed := false
	var firstError any
	for _, original := range steps {
		*stepIndex++
		step := substituteWorkflowStep(original, vars)
		args, err := workflowStepArgs(step)
		item := map[string]any{"index": *stepIndex, "phase": phase, "action": step.Action}
		if err != nil {
			item["ok"] = false
			item["error"] = err.Error()
			result.Steps = append(result.Steps, item)
			if firstError == nil {
				firstError = err.Error()
			}
			failed = true
			if stopOnFailure {
				return currentRunID, true, firstError
			}
			continue
		}
		args = fillWorkflowRunID(args, currentRunID)
		if id := runIDFromArgs(args); id != "" {
			currentRunID = id
			result.RunID = id
		}
		env, raw, err := executeWorkflowCommand(cmd, o, args)
		item["args"] = redactArgs(args)
		item["ok"] = env.OK
		if raw != "" {
			item["raw_length"] = len(raw)
		}
		if env.Data != nil {
			item["data"] = env.Data
		}
		if env.Error != nil {
			item["error"] = env.Error
		}
		result.Steps = append(result.Steps, item)
		if id := runIDFromEnvelope(env); id != "" {
			currentRunID = id
			result.RunID = id
		}
		if err != nil || !env.OK {
			failed = true
			if firstError == nil {
				if env.Error != nil {
					firstError = env.Error
				} else if err != nil {
					firstError = err.Error()
				} else {
					firstError = "workflow command returned ok=false"
				}
			}
			if stopOnFailure {
				return currentRunID, true, firstError
			}
		}
	}
	return currentRunID, failed, firstError
}

func runIDFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "--run-id" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func mergeTestErrors(primary, cleanup any) any {
	if primary == nil {
		return cleanup
	}
	if cleanup == nil {
		return primary
	}
	return map[string]any{"primary": primary, "cleanup": cleanup}
}

func writeTestEvidence(o *Opts, evidenceDir, caseName string, result mobileTestCaseResult) string {
	dir := filepath.Join(evidenceDir, mobile.SafeArtifactName(caseName))
	_ = os.MkdirAll(dir, 0o700)
	path := filepath.Join(dir, "failure.json")
	b, err := json.MarshalIndent(output.RedactValue(result), "", "  ")
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return ""
	}
	writeRunEvidence(o, dir, result.RunID)
	return dir
}

func writeRunEvidence(o *Opts, dir, runID string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	svc, err := newServices(o, false)
	if err != nil {
		return
	}
	report := map[string]any{}
	st, err := svc.Store.LoadRun(runID)
	if err != nil {
		return
	}
	report["run"] = st
	if events, err := svc.Store.LoadTimeline(runID); err == nil {
		report["timeline"] = events
	}
	if st.LatestObservationID != "" {
		if obs, err := svc.Store.LoadObservation(runID, st.LatestObservationID); err == nil {
			report["latest_observation"] = obs
			copyObservationEvidence(dir, obs)
		}
	}
	_ = writeJSONFile(filepath.Join(dir, "run-report.json"), report)
}

func copyObservationEvidence(dir string, obs mobile.Observation) {
	for name, path := range map[string]string{
		"source.xml":     obs.SourcePath,
		"screenshot.png": obs.ScreenshotPath,
		"candidates.json": obs.CandidatesPath,
	} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		_ = copyFile(path, filepath.Join(dir, name))
	}
}

func substituteWorkflowStep(step workflowStep, vars map[string]string) workflowStep {
	step.RunID = substituteVars(step.RunID, vars)
	step.Ref = substituteVars(step.Ref, vars)
	step.Name = substituteVars(step.Name, vars)
	step.Text = substituteVars(step.Text, vars)
	step.TextEnv = substituteVars(step.TextEnv, vars)
	step.Role = substituteVars(step.Role, vars)
	step.ResourceID = substituteVars(step.ResourceID, vars)
	step.AccessibilityID = substituteVars(step.AccessibilityID, vars)
	step.ParentText = substituteVars(step.ParentText, vars)
	step.NearbyText = substituteVars(step.NearbyText, vars)
	step.WithinText = substituteVars(step.WithinText, vars)
	step.Direction = substituteVars(step.Direction, vars)
	step.Network = substituteVars(step.Network, vars)
	step.App = substituteVars(step.App, vars)
	step.AppID = substituteVars(step.AppID, vars)
	step.URL = substituteVars(step.URL, vars)
	step.Package = substituteVars(step.Package, vars)
	step.File = substituteVars(step.File, vars)
	step.Platform = substituteVars(step.Platform, vars)
	step.Device = substituteVars(step.Device, vars)
	step.Build = substituteVars(step.Build, vars)
	step.SessionName = substituteVars(step.SessionName, vars)
	step.Timeout = substituteVars(step.Timeout, vars)
	step.PollInterval = substituteVars(step.PollInterval, vars)
	step.Status = substituteVars(step.Status, vars)
	step.Contains = substituteVars(step.Contains, vars)
	step.Equals = substituteVars(step.Equals, vars)
	return step
}

func substituteVars(value string, vars map[string]string) string {
	out := value
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func caseHasAnyTag(tc mobileTestCase, tags []string) bool {
	have := map[string]bool{}
	for _, tag := range tc.Tags {
		have[tag] = true
	}
	for _, tag := range tags {
		if have[tag] {
			return true
		}
	}
	return false
}

func redactTestPlan(executions []mobileTestExecution) []map[string]any {
	out := make([]map[string]any, 0, len(executions))
	for _, execution := range executions {
		steps := make([]workflowStep, 0, len(execution.Case.Steps))
		for _, step := range execution.Case.Steps {
			if step.Text != "" {
				step.Text = output.Redacted
			}
			steps = append(steps, step)
		}
		out = append(out, map[string]any{"case": execution.Case.Name, "matrix": execution.Matrix.Name, "tags": execution.Case.Tags, "steps": steps})
	}
	return out
}

type junitTestsuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Cases    []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Name    string        `xml:"name,attr"`
	Class   string        `xml:"classname,attr,omitempty"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func writeJUnitReport(path, suiteName string, results []mobileTestCaseResult) error {
	out := junitTestsuite{Name: suiteName, Tests: len(results)}
	for _, result := range results {
		tc := junitTestcase{Name: result.Name, Class: result.Matrix, Time: strconvFormatSeconds(result.DurationMS)}
		if !result.Passed {
			out.Failures++
			body, _ := json.Marshal(output.RedactValue(result.Error))
			tc.Failure = &junitFailure{Message: "mobile test failed", Body: string(body)}
		}
		out.Cases = append(out.Cases, tc)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	b, err := xml.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), b...), 0o600)
}

func strconvFormatSeconds(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64)
}
