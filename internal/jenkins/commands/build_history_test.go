package commands

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/testutil"
)

const (
	testBuildSummaryTree = "number,url,result,building,timestamp,duration,displayName,actions[parameters[name,value],causes[shortDescription,userId,userName,upstreamProject,upstreamBuild]]"
	testChangeItemsTree  = "items[commitId,msg,author[fullName],timestamp,affectedPaths]"
)

func jenkinsTestSetup(t *testing.T) (*testutil.MockJenkins, string) {
	t.Helper()
	mock := testutil.NewMockJenkins(t)
	cfg, err := testutil.WriteConfig(testutil.JenkinsConfig(mock.Server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return mock, cfg
}

func lastTree(t *testing.T, mock *testutil.MockJenkins) string {
	t.Helper()
	values, err := url.ParseQuery(mock.LastQuery)
	if err != nil {
		t.Fatalf("bad query %q: %v", mock.LastQuery, err)
	}
	return values.Get("tree")
}

func requireJenkinsError(t *testing.T, out map[string]any, code string) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected %s failure, got success: %#v", code, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != code {
		t.Fatalf("error.code=%#v want %s: %#v", errObj["code"], code, out)
	}
	return errObj
}

func buildNumbers(t *testing.T, data map[string]any) []int {
	t.Helper()
	raw, _ := data["builds"].([]any)
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		b, _ := item.(map[string]any)
		n, ok := b["number"].(float64)
		if !ok {
			t.Fatalf("build without numeric number: %#v", b)
		}
		out = append(out, int(n))
	}
	return out
}

func requireBuildListCounts(t *testing.T, data map[string]any, returned, filteredFrom, scanned int, truncated bool) {
	t.Helper()
	if got := data["count_returned"].(float64); int(got) != returned {
		t.Fatalf("count_returned=%v want %d: %#v", got, returned, data)
	}
	if got := data["filtered_from"].(float64); int(got) != filteredFrom {
		t.Fatalf("filtered_from=%v want %d: %#v", got, filteredFrom, data)
	}
	if got := data["scanned"].(float64); int(got) != scanned {
		t.Fatalf("scanned=%v want %d: %#v", got, scanned, data)
	}
	if got := data["truncated"].(bool); got != truncated {
		t.Fatalf("truncated=%v want %v: %#v", got, truncated, data)
	}
	if got := len(buildNumbers(t, data)); got != returned {
		t.Fatalf("len(builds)=%d want %d", got, returned)
	}
}

func TestJenkinsBuildListReturnsNewestFirstWithinScanWindow(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api"))
	requireBuildListCounts(t, data, 20, 30, 30, true)
	if got := lastTree(t, mock); got != "allBuilds["+testBuildSummaryTree+"]{0,80}" {
		t.Fatalf("tree=%q", got)
	}
	if mock.LastPath != "/job/deploy/job/payments-api/api/json" {
		t.Fatalf("path=%q", mock.LastPath)
	}
	if data["limit"].(float64) != 20 || data["scan_window"].(float64) != 80 || data["job"] != "deploy/payments-api" {
		t.Fatalf("limit/scan_window/job=%#v", data)
	}
	numbers := buildNumbers(t, data)
	if numbers[0] != 30 || numbers[19] != 11 {
		t.Fatalf("numbers=%v", numbers)
	}
	newest := data["builds"].([]any)[0].(map[string]any)
	if newest["building"] != true || newest["result"] != nil || newest["display_name"] != "#30" {
		t.Fatalf("newest=%#v", newest)
	}
	if !strings.HasSuffix(newest["url"].(string), "/job/deploy/job/payments-api/30/") {
		t.Fatalf("url=%#v", newest["url"])
	}
	params := newest["parameters"].(map[string]any)
	if params["ENV"] != "prod" || params["VERSION"] != "1.30.0" || params["DRY_RUN"] != true {
		t.Fatalf("parameters=%#v", params)
	}
	if params["API_TOKEN"] != "***REDACTED***" {
		t.Fatalf("secret-looking parameter leaked: %#v", params)
	}
	causes := newest["causes"].([]any)
	if len(causes) != 1 || causes[0] != "Started by user Alice" {
		t.Fatalf("causes=%#v", causes)
	}
	wantISO := mock.BuildBase.Add(-5 * time.Minute).UTC().Format(time.RFC3339)
	if newest["timestamp_iso"] != wantISO {
		t.Fatalf("timestamp_iso=%#v want %s", newest["timestamp_iso"], wantISO)
	}
	if newest["duration_ms"].(float64) != 0 {
		t.Fatalf("duration_ms=%#v", newest["duration_ms"])
	}
	oldest := mock.BuildBase.Add(-5*time.Minute - 29*time.Hour).UTC().Format(time.RFC3339)
	if data["oldest_scanned_timestamp_iso"] != oldest {
		t.Fatalf("oldest_scanned_timestamp_iso=%#v want %s", data["oldest_scanned_timestamp_iso"], oldest)
	}
	finished := data["builds"].([]any)[1].(map[string]any)
	if finished["result"] != "SUCCESS" || finished["building"] != false || finished["duration_ms"].(float64) != 29000 {
		t.Fatalf("finished=%#v", finished)
	}
	if finished["causes"].([]any)[0] != "Started by upstream project \"folder/app-main\" build number 29" {
		t.Fatalf("upstream cause=%#v", finished["causes"])
	}
}

func TestJenkinsBuildListLimitControlsScanWindowAndIsCapped(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--limit", "5"))
	requireBuildListCounts(t, data, 5, 20, 20, true)
	if got := lastTree(t, mock); !strings.HasSuffix(got, "]{0,20}") {
		t.Fatalf("tree=%q", got)
	}
	if numbers := buildNumbers(t, data); numbers[0] != 30 || numbers[4] != 26 {
		t.Fatalf("numbers=%v", numbers)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--limit", "500"))
	requireBuildListCounts(t, data, 30, 30, 30, false)
	if data["limit"].(float64) != 200 || data["scan_window"].(float64) != 800 {
		t.Fatalf("limit/scan_window=%#v", data)
	}
	if got := lastTree(t, mock); !strings.HasSuffix(got, "]{0,800}") {
		t.Fatalf("tree=%q", got)
	}
}

func TestJenkinsBuildListFiltersByResultCaseInsensitively(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--result", "failure"))
	requireBuildListCounts(t, data, 5, 5, 30, false)
	if numbers := buildNumbers(t, data); numbers[0] != 25 || numbers[4] != 5 {
		t.Fatalf("numbers=%v", numbers)
	}
	for _, item := range data["builds"].([]any) {
		if item.(map[string]any)["result"] != "FAILURE" {
			t.Fatalf("unexpected result in %#v", item)
		}
	}

	// A small limit shrinks the scan window (8 builds: #30..#23), which holds
	// only one FAILURE; the exhausted window must be reported as truncated.
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--result", "FAILURE", "--limit", "2"))
	requireBuildListCounts(t, data, 1, 1, 8, true)
	if numbers := buildNumbers(t, data); numbers[0] != 25 {
		t.Fatalf("numbers=%v", numbers)
	}

	// No NOT_BUILT builds exist and the window was not exhausted: a complete,
	// empty answer.
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--result", "not_built"))
	requireBuildListCounts(t, data, 0, 0, 30, false)

	errObj := requireJenkinsError(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--result", "bogus"), "invalid_args")
	if !strings.Contains(errObj["hint"].(string), "NOT_BUILT") {
		t.Fatalf("hint=%#v", errObj["hint"])
	}
}

func TestJenkinsBuildListFiltersInProgressBuilds(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--building"))
	requireBuildListCounts(t, data, 1, 1, 30, false)
	if numbers := buildNumbers(t, data); numbers[0] != 30 {
		t.Fatalf("numbers=%v", numbers)
	}
}

func TestJenkinsBuildListRequiresEveryParamToMatchExactly(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "ENV=prod"))
	requireBuildListCounts(t, data, 15, 15, 30, false)
	for _, n := range buildNumbers(t, data) {
		if n%2 != 0 {
			t.Fatalf("odd build %d has ENV=stage, not prod", n)
		}
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "ENV=prod", "--param", "VERSION=1.12.0"))
	requireBuildListCounts(t, data, 1, 1, 30, false)
	if numbers := buildNumbers(t, data); numbers[0] != 12 {
		t.Fatalf("numbers=%v", numbers)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "ENV=prod", "--param", "VERSION=1.13.0"))
	requireBuildListCounts(t, data, 0, 0, 30, false)

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "VERSION=1.1"))
	requireBuildListCounts(t, data, 0, 0, 30, false)

	// Non-string parameter values (JSON booleans) match their textual form.
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "DRY_RUN=true"))
	requireBuildListCounts(t, data, 10, 10, 30, false)

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "NOPE=1"))
	requireBuildListCounts(t, data, 0, 0, 30, false)

	// Matching uses the raw value even though the output redacts it.
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "API_TOKEN=s3cret-12"))
	requireBuildListCounts(t, data, 1, 1, 30, false)
	params := data["builds"].([]any)[0].(map[string]any)["parameters"].(map[string]any)
	if params["API_TOKEN"] != "***REDACTED***" {
		t.Fatalf("parameters=%#v", params)
	}

	requireJenkinsError(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--param", "novalue"), "invalid_args")
}

func TestJenkinsBuildListFiltersBySince(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "24h"))
	requireBuildListCounts(t, data, 20, 24, 30, true)
	if numbers := buildNumbers(t, data); numbers[0] != 30 || numbers[19] != 11 {
		t.Fatalf("numbers=%v", numbers)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "24h", "--limit", "30"))
	requireBuildListCounts(t, data, 24, 24, 30, false)
	if numbers := buildNumbers(t, data); numbers[23] != 7 {
		t.Fatalf("numbers=%v", numbers)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "3h"))
	requireBuildListCounts(t, data, 3, 3, 30, false)

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "7d"))
	requireBuildListCounts(t, data, 20, 30, 30, true)

	rfc := time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", rfc))
	requireBuildListCounts(t, data, 3, 3, 30, false)

	exact := mock.BuildBase.Add(-5*time.Minute - 10*time.Hour).UTC().Format(time.RFC3339)
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", exact))
	requireBuildListCounts(t, data, 11, 11, 30, false)
	if numbers := buildNumbers(t, data); numbers[10] != 20 {
		t.Fatalf("since is inclusive of the build started exactly at the boundary: %v", numbers)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "2000-01-01", "--limit", "50"))
	requireBuildListCounts(t, data, 30, 30, 30, false)

	for _, bad := range []string{"nonsense", "-5h", "0d", "yesterday"} {
		requireJenkinsError(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", bad), "invalid_args")
	}
}

func TestJenkinsBuildListTruncationConsidersWhetherWindowCoversSince(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	// Window of 12 builds (#30..#19) reaches back 11h, past the 3h cutoff, so
	// exhausting it does not mean older matches were missed.
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "3h", "--limit", "3"))
	requireBuildListCounts(t, data, 3, 3, 12, false)

	// Same window, but the 20h cutoff lies beyond its oldest build: the one
	// ABORTED build inside the window fits the limit, yet #11 (ABORTED,
	// ~19h old) was never scanned, so the answer must be flagged truncated.
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "20h", "--limit", "3", "--result", "aborted"))
	requireBuildListCounts(t, data, 1, 1, 12, true)
	if numbers := buildNumbers(t, data); numbers[0] != 22 {
		t.Fatalf("numbers=%v", numbers)
	}
}

func TestJenkinsBuildListCombinesFilters(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "list", "deploy/payments-api", "--since", "24h", "--result", "success", "--param", "ENV=prod"))
	requireBuildListCounts(t, data, 6, 6, 30, false)
	want := []int{26, 24, 18, 16, 12, 8}
	got := buildNumbers(t, data)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("numbers=%v want %v", got, want)
		}
	}
}

func TestJenkinsBuildListUnknownJobIsNotFound(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	requireJenkinsError(t, runJenkins(t, cfg, "build", "list", "deploy/missing"), "not_found")
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"", time.Time{}},
		{"30m", now.Add(-30 * time.Minute)},
		{"24h", now.Add(-24 * time.Hour)},
		{"1h30m", now.Add(-90 * time.Minute)},
		{"7d", now.Add(-7 * 24 * time.Hour)},
		{"2W", now.Add(-14 * 24 * time.Hour)},
		{" 3d ", now.Add(-3 * 24 * time.Hour)},
		{"2026-09-18T10:00:00Z", time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)},
		{"2026-09-18T10:00:00+02:00", time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)},
		{"2026-09-18T10:00:00.5Z", time.Date(2026, 9, 18, 10, 0, 0, 500000000, time.UTC)},
		{"2026-09-18", time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		got, err := parseSince(tc.in, now)
		if err != nil {
			t.Fatalf("parseSince(%q) error: %v", tc.in, err)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("parseSince(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"nonsense", "-5h", "0d", "0h", "7 days", "yesterday", "24", "d"} {
		if _, err := parseSince(bad, now); err == nil {
			t.Fatalf("parseSince(%q) unexpectedly succeeded", bad)
		}
	}
}

func TestJenkinsBuildParamsNormalizesPipelineBuild(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "12"))
	if mock.LastPath != "/job/deploy/job/payments-api/12/api/json" {
		t.Fatalf("path=%q", mock.LastPath)
	}
	wantTree := testBuildSummaryTree + ",changeSets[" + testChangeItemsTree + "],changeSet[" + testChangeItemsTree + "]"
	if got := lastTree(t, mock); got != wantTree {
		t.Fatalf("tree=%q want %q", got, wantTree)
	}
	if data["number"].(float64) != 12 || data["result"] != "SUCCESS" || data["building"] != false || data["display_name"] != "#12" {
		t.Fatalf("summary=%#v", data)
	}
	if !strings.HasSuffix(data["url"].(string), "/job/deploy/job/payments-api/12/") {
		t.Fatalf("url=%#v", data["url"])
	}
	wantISO := mock.BuildBase.Add(-5*time.Minute - 18*time.Hour).UTC().Format(time.RFC3339)
	if data["timestamp_iso"] != wantISO || data["duration_ms"].(float64) != 12000 {
		t.Fatalf("timestamp_iso=%#v duration_ms=%#v", data["timestamp_iso"], data["duration_ms"])
	}
	params := data["parameters"].(map[string]any)
	if params["ENV"] != "prod" || params["VERSION"] != "1.12.0" || params["DRY_RUN"] != true || params["API_TOKEN"] != "***REDACTED***" {
		t.Fatalf("parameters=%#v", params)
	}
	causes := data["causes"].([]any)
	if len(causes) != 1 {
		t.Fatalf("causes=%#v", causes)
	}
	cause := causes[0].(map[string]any)
	if cause["description"] != "Started by user Alice" || cause["user_id"] != "alice" || cause["user_name"] != "Alice" || cause["upstream_project"] != nil || cause["upstream_build"] != nil {
		t.Fatalf("cause=%#v", cause)
	}
	if data["changes_count_total"].(float64) != 3 || data["changes_truncated"] != false {
		t.Fatalf("changes meta=%#v", data)
	}
	changes := data["changes"].([]any)
	if len(changes) != 3 {
		t.Fatalf("changes=%#v", changes)
	}
	first := changes[0].(map[string]any)
	if first["commit"] != "a1b2c3d4" || first["message"] != "Bump payments-api to 1.12.0" || first["author"] != "Alice Example" || first["files_count"].(float64) != 2 {
		t.Fatalf("first change=%#v", first)
	}
	if first["timestamp_iso"] == nil {
		t.Fatalf("first change missing timestamp_iso: %#v", first)
	}
	if third := changes[2].(map[string]any); third["commit"] != "c9d0e1f2" || third["files_count"].(float64) != 3 {
		t.Fatalf("third change=%#v", third)
	}
}

func TestJenkinsBuildParamsReportsUpstreamCauseAndLastBuild(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "13"))
	cause := data["causes"].([]any)[0].(map[string]any)
	if cause["upstream_project"] != "folder/app-main" || cause["upstream_build"].(float64) != 13 || cause["user_id"] != nil {
		t.Fatalf("cause=%#v", cause)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "lastBuild"))
	if data["number"].(float64) != 30 || data["building"] != true || data["result"] != nil {
		t.Fatalf("lastBuild=%#v", data)
	}
}

func TestJenkinsBuildParamsCapsChanges(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "12", "--max-changes", "1"))
	if len(data["changes"].([]any)) != 1 || data["changes_count_total"].(float64) != 3 || data["changes_truncated"] != true {
		t.Fatalf("max-changes 1: %#v", data)
	}
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "12", "--max-changes", "0"))
	if len(data["changes"].([]any)) != 0 || data["changes_count_total"].(float64) != 3 || data["changes_truncated"] != true {
		t.Fatalf("max-changes 0: %#v", data)
	}
	data = requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "12", "--max-changes", "-1"))
	if len(data["changes"].([]any)) != 3 || data["changes_truncated"] != false {
		t.Fatalf("negative max-changes should fall back to the default: %#v", data)
	}
}

func TestJenkinsBuildParamsHandlesFreestyleChangeSet(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "build", "params", "tools/legacy-freestyle", "7"))
	if data["number"].(float64) != 7 || data["result"] != "SUCCESS" {
		t.Fatalf("summary=%#v", data)
	}
	if params := data["parameters"].(map[string]any); params["TARGET"] != "prod" || len(params) != 1 {
		t.Fatalf("parameters=%#v", params)
	}
	cause := data["causes"].([]any)[0].(map[string]any)
	if cause["description"] != "Started by timer" || cause["user_id"] != nil || cause["upstream_project"] != nil {
		t.Fatalf("cause=%#v", cause)
	}
	if data["changes_count_total"].(float64) != 2 || data["changes_truncated"] != false {
		t.Fatalf("changes meta=%#v", data)
	}
	changes := data["changes"].([]any)
	first := changes[0].(map[string]any)
	second := changes[1].(map[string]any)
	if first["commit"] != "1001" || first["author"] != "Dan Example" || first["files_count"].(float64) != 1 || first["timestamp_iso"] == nil {
		t.Fatalf("first change=%#v", first)
	}
	if second["commit"] != "1002" || second["files_count"].(float64) != 0 || second["timestamp_iso"] != nil {
		t.Fatalf("second change=%#v", second)
	}
}

func TestJenkinsBuildParamsUnknownBuildIsNotFound(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	requireJenkinsError(t, runJenkins(t, cfg, "build", "params", "deploy/payments-api", "999"), "not_found")
	requireJenkinsError(t, runJenkins(t, cfg, "build", "params", "deploy/missing", "1"), "not_found")
}
