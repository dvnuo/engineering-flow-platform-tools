package commands

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/jenkins"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

// buildTestReportCmd exposes Jenkins' JUnit-shaped testReport API (used by
// JUnit, Cucumber-JVM's JUnit formatter, and most CI test reporters) as a
// normalized failure list so agents do not need to know the raw schema or
// hand-roll `api get`. Builds without a published test report return
// has_report=false instead of an error, since not every job runs tests.
func buildTestReportCmd(o *Opts) *cobra.Command {
	var maxFailures int
	c := &cobra.Command{Use: "test-report <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return buildTestReport(o, cmd, args[0], args[1], maxFailures)
	}}
	c.Flags().IntVar(&maxFailures, "max-failures", 50, "Maximum number of failed test cases to return.")
	return c
}

func buildTestReport(o *Opts, cmd *cobra.Command, job, build string, maxFailures int) error {
	// Only a negative value falls back to the default; 0 is honored as
	// "report counts only, return no case bodies" for fast triage of large
	// suites.
	if maxFailures < 0 {
		maxFailures = 50
	}
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.client.Do(jenkins.Request{
		Method: http.MethodGet,
		Path:   jenkins.BuildPath(job, build) + "/testReport/api/json",
		Query:  map[string]string{"tree": "failCount,passCount,skipCount,suites[name,cases[className,name,status,age,errorDetails,errorStackTrace,duration,skipped]]"},
	})
	if err != nil {
		var httpErr *httpclient.HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusNotFound {
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{
				"has_report": false,
				"reason":     "no testReport is published for this build; the job may not archive JUnit/Cucumber JUnit-formatted results, or the build produced no test cases",
			}))
		}
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := jenkins.JSONMap(resp.Body)
	failures, totalFailing := extractJenkinsTestFailures(data, maxFailures)
	data["has_report"] = true
	data["failures"] = failures
	data["failure_count_returned"] = len(failures)
	data["failure_count_total"] = totalFailing
	data["failures_truncated"] = totalFailing > len(failures)
	return print(cmd, o, output.Success(cx.inst.Name, data))
}

func extractJenkinsTestFailures(data map[string]any, maxFailures int) ([]map[string]any, int) {
	failures := make([]map[string]any, 0, maxFailures)
	total := 0
	suites, _ := data["suites"].([]any)
	for _, rawSuite := range suites {
		suite, _ := rawSuite.(map[string]any)
		if suite == nil {
			continue
		}
		suiteName, _ := suite["name"].(string)
		cases, _ := suite["cases"].([]any)
		for _, rawCase := range cases {
			tc, _ := rawCase.(map[string]any)
			if tc == nil || !jenkinsCaseIsFailing(tc) {
				continue
			}
			total++
			if len(failures) >= maxFailures {
				continue
			}
			failures = append(failures, map[string]any{
				"suite":             suiteName,
				"class_name":        tc["className"],
				"name":              tc["name"],
				"status":            tc["status"],
				"age":               tc["age"],
				"error_details":     tc["errorDetails"],
				"error_stack_trace": tc["errorStackTrace"],
				"duration":          tc["duration"],
			})
		}
	}
	return failures, total
}

func jenkinsCaseIsFailing(tc map[string]any) bool {
	status, _ := tc["status"].(string)
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "FAILED", "REGRESSION":
		return true
	default:
		return false
	}
}

// buildWaitCmd polls a build until it stops building so a fix-and-verify
// loop (or any caller that just triggered a build) does not have to
// hand-roll a queue-then-build polling loop.
func buildWaitCmd(o *Opts) *cobra.Command {
	var intervalMS, timeoutSec int
	c := &cobra.Command{Use: "wait <job> <build>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return buildWait(o, cmd, args[0], args[1], intervalMS, timeoutSec)
	}}
	c.Flags().IntVar(&intervalMS, "interval-ms", 5000, "Polling interval in milliseconds.")
	c.Flags().IntVar(&timeoutSec, "timeout-sec", 1800, "Maximum seconds to wait for the build to finish before giving up.")
	return c
}

func buildWait(o *Opts, cmd *cobra.Command, job, build string, intervalMS, timeoutSec int) error {
	if intervalMS <= 0 {
		intervalMS = 5000
	}
	if timeoutSec <= 0 {
		timeoutSec = 1800
	}
	cx, err := loadCtx(o, job)
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for {
		resp, err := cx.client.Do(jenkins.Request{
			Method: http.MethodGet,
			Path:   jenkins.BuildPath(job, build) + "/api/json",
			Query:  map[string]string{"tree": "number,url,building,result,duration,fullDisplayName"},
		})
		if err != nil {
			return print(cmd, o, envelopeError(err, "server_error"))
		}
		data := jenkins.JSONMap(resp.Body)
		resp.Body.Close()
		building, _ := data["building"].(bool)
		if !building {
			result, _ := data["result"].(string)
			state := strings.ToLower(result)
			if state == "" {
				state = "unknown"
			}
			data["state"] = state
			data["waited"] = true
			return print(cmd, o, output.Success(cx.inst.Name, data))
		}
		if time.Now().After(deadline) {
			data["state"] = "timeout_waiting"
			data["waited"] = false
			env := output.Failure("wait_timeout", "timed out waiting for the Jenkins build to finish", "Increase --timeout-sec, or poll again with jenkins build status.", 408)
			env.Data = data
			return print(cmd, o, env)
		}
		time.Sleep(time.Duration(intervalMS) * time.Millisecond)
	}
}
