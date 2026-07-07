package commands

import (
	"testing"

	"engineering-flow-platform-tools/internal/testutil"
)

func TestJenkinsBuildTestReportExtractsFailures(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "test-report", "folder/app-main", "42"))
	if data["has_report"] != true {
		t.Fatalf("expected has_report=true: %#v", data)
	}
	if data["failure_count_total"].(float64) != 1 {
		t.Fatalf("failure_count_total=%#v", data["failure_count_total"])
	}
	if data["failures_truncated"] != false {
		t.Fatalf("failures_truncated=%#v", data["failures_truncated"])
	}
	failures, _ := data["failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("failures=%#v", data["failures"])
	}
	failure, _ := failures[0].(map[string]any)
	if failure["name"] != "Login button moved after redesign" {
		t.Fatalf("failure name=%#v", failure["name"])
	}
	if failure["error_details"] != "element not found: login-button" {
		t.Fatalf("failure error_details=%#v", failure["error_details"])
	}
	if failure["status"] != "FAILED" {
		t.Fatalf("failure status=%#v", failure["status"])
	}
}

func TestJenkinsBuildTestReportCapsFailuresAndFlagsTruncation(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "test-report", "folder/app-main", "42", "--max-failures", "0"))
	failures, _ := data["failures"].([]any)
	if len(failures) != 0 {
		t.Fatalf("expected zero returned failures, got %#v", failures)
	}
	if data["failure_count_total"].(float64) != 1 {
		t.Fatalf("failure_count_total=%#v", data["failure_count_total"])
	}
	if data["failures_truncated"] != true {
		t.Fatalf("expected failures_truncated=true, got %#v", data["failures_truncated"])
	}
}

func TestJenkinsBuildTestReportMissingReportIsNotAnError(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "test-report", "folder/no-report", "9"))
	if data["has_report"] != false {
		t.Fatalf("expected has_report=false: %#v", data)
	}
	if data["reason"] == "" || data["reason"] == nil {
		t.Fatalf("expected a reason for the missing report: %#v", data)
	}
}

func TestJenkinsBuildWaitReturnsImmediatelyWhenAlreadyFinished(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "wait", "folder/app-main", "42", "--interval-ms", "1"))
	if data["waited"] != true {
		t.Fatalf("waited=%#v", data["waited"])
	}
	if data["state"] != "success" {
		t.Fatalf("state=%#v", data["state"])
	}
}

func TestJenkinsBuildWaitTimesOutOnStillBuildingJob(t *testing.T) {
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	// timeout-sec has 1-second granularity; use the smallest positive
	// timeout with a fast poll interval so the test stays bounded (~1s)
	// while still exercising the real deadline/timeout code path.
	out := runJenkins(t, cfg, "build", "wait", "folder/still-building", "7", "--interval-ms", "50", "--timeout-sec", "1")
	if out["ok"] != false {
		t.Fatalf("expected timeout failure envelope: %#v", out)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != "wait_timeout" {
		t.Fatalf("error.code=%#v", errObj["code"])
	}
	data, _ := out["data"].(map[string]any)
	if data["state"] != "timeout_waiting" {
		t.Fatalf("error.data.state=%#v", data["state"])
	}
	if data["waited"] != false {
		t.Fatalf("error.data.waited=%#v", data["waited"])
	}
}
