package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/testutil"
)

var canaries = append(append([]string{}, testutil.Secrets...), testutil.MockSplunkSessionKey)

func assertNoCanaries(t *testing.T, out string) {
	t.Helper()
	for _, secret := range canaries {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked into output: %s", secret, out)
		}
	}
}

func execSplunk(t *testing.T, stdin, cfg string, args ...string) (map[string]any, string) {
	t.Helper()
	root := NewRoot()
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	if stdin != "" {
		root.SetIn(strings.NewReader(stdin))
	}
	root.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	out := testutil.AssertJSONEnvelope(t, b.Bytes())
	assertNoCanaries(t, b.String())
	return out, b.String()
}

func runSplunk(t *testing.T, cfg string, args ...string) map[string]any {
	t.Helper()
	out, _ := execSplunk(t, "", cfg, args...)
	return out
}

func requireOK(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("command failed: %#v", out)
	}
	data, _ := out["data"].(map[string]any)
	return data
}

func requireErr(t *testing.T, out map[string]any, code string) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected error %s, got success: %#v", code, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != code {
		t.Fatalf("expected error.code=%s got %#v", code, out)
	}
	return errObj
}

func bearerConfig(t *testing.T, base string) string {
	t.Helper()
	cfg, err := testutil.WriteConfig(testutil.SplunkConfig(base))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cfg) })
	return cfg
}

func writeRootConfig(t *testing.T, cfg config.RootConfig) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func instanceConfig(t *testing.T, defaultInstance string, instances ...config.InstanceConfig) string {
	t.Helper()
	return writeRootConfig(t, config.RootConfig{Version: 1, Splunk: config.ProductConfig{DefaultInstance: defaultInstance, Instances: instances}})
}

func bearerInstance(name, base string) config.InstanceConfig {
	return config.InstanceConfig{Name: name, BaseURL: base, Auth: config.AuthConfig{Type: "bearer_token", Token: "secret-token-should-not-appear"}}
}

func sessionInstance(name, base string) config.InstanceConfig {
	return config.InstanceConfig{Name: name, BaseURL: base, Auth: config.AuthConfig{Type: "basic_password", Username: "agent", Password: "secret-password-should-not-appear"}}
}

func results(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()
	raw, _ := data["results"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, _ := item.(map[string]any)
		out = append(out, m)
	}
	return out
}

func TestSplunkBearerAuthTest(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	data := requireOK(t, runSplunk(t, cfg, "auth", "test"))
	if data["authenticated"] != true || data["username"] != "agent" || data["auth_type"] != "bearer_token" {
		t.Fatalf("bad auth test data: %#v", data)
	}
	if roles, _ := data["roles"].([]any); len(roles) != 2 {
		t.Fatalf("roles=%#v", data["roles"])
	}
	if mock.LastAuth != "Bearer secret-token-should-not-appear" || mock.LoginHits != 0 {
		t.Fatalf("expected bearer auth without login, got auth=%q logins=%d", mock.LastAuth, mock.LoginHits)
	}
	if mock.LastPath != "/services/authentication/current-context" || mock.LastQuery.Get("output_mode") != "json" {
		t.Fatalf("path=%s query=%v", mock.LastPath, mock.LastQuery)
	}
}

func TestSplunkSessionLoginPath(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := instanceConfig(t, "local", sessionInstance("local", mock.Server.URL))
	data := requireOK(t, runSplunk(t, cfg, "auth", "test"))
	if data["authenticated"] != true || data["auth_type"] != "basic_password" {
		t.Fatalf("bad data: %#v", data)
	}
	if mock.LoginHits != 1 {
		t.Fatalf("expected exactly one login, got %d", mock.LoginHits)
	}
	if mock.LastAuth != "Splunk "+testutil.MockSplunkSessionKey {
		t.Fatalf("expected session key auth, got %q", mock.LastAuth)
	}
	// One login per process: a full search flow (create, poll, results) must
	// reuse the in-memory session key.
	before := mock.LoginHits
	requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "index=main error", "--poll-ms", "10"))
	if mock.LoginHits != before+1 {
		t.Fatalf("expected one login for the whole search flow, got %d", mock.LoginHits-before)
	}
}

func TestSplunkSessionLoginFailure(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	inst := sessionInstance("local", mock.Server.URL)
	inst.Auth.Password = "secret-password-should-not-appear-wrong"
	cfg := instanceConfig(t, "local", inst)
	errObj := requireErr(t, runSplunk(t, cfg, "auth", "test"), "auth_failed")
	if !strings.Contains(errObj["message"].(string), "session login failed") || !strings.Contains(errObj["message"].(string), "Login failed") {
		t.Fatalf("message=%#v", errObj)
	}
}

func TestSplunkUnsupportedAuthType(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := instanceConfig(t, "local", config.InstanceConfig{Name: "local", BaseURL: mock.Server.URL, Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "secret-api-key-should-not-appear"}})
	errObj := requireErr(t, runSplunk(t, cfg, "auth", "test"), "config_error")
	if !strings.Contains(errObj["hint"].(string), "--token-stdin") {
		t.Fatalf("hint=%#v", errObj)
	}
	if mock.Hits != 0 {
		t.Fatalf("unsupported auth type must not contact the server")
	}
}

func TestSplunkNoInstanceConfigured(t *testing.T) {
	cfg := instanceConfig(t, "")
	requireErr(t, runSplunk(t, cfg, "auth", "test"), "no_instance_configured")
	requireErr(t, runSplunk(t, cfg, "auth", "login", "--token-stdin"), "invalid_args")
}

func TestSplunkInstanceRequiredWithSeveralInstances(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := instanceConfig(t, "", bearerInstance("a", mock.Server.URL), bearerInstance("b", mock.Server.URL))
	requireErr(t, runSplunk(t, cfg, "index", "list"), "instance_required")
	data := requireOK(t, runSplunk(t, cfg, "--instance", "b", "index", "list"))
	if data["count"] != float64(2) {
		t.Fatalf("data=%#v", data)
	}
}

func TestSplunkSearchRunPollsUntilDone(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	mock.StatusCallsUntilDone = 2
	cfg := bearerConfig(t, mock.Server.URL)
	out := runSplunk(t, cfg, "search", "run", "--query", "index=main error | head 100", "--earliest", "-30m", "--poll-ms", "10")
	if out["instance"] != "local" {
		t.Fatalf("instance=%#v", out["instance"])
	}
	data := requireOK(t, out)
	if data["sid"] != "1700000000.1" || data["dispatch_state"] != "DONE" || data["result_count"] != float64(3) || data["scan_count"] != float64(42) || data["event_count"] != float64(3) {
		t.Fatalf("bad data: %#v", data)
	}
	if data["results_truncated"] != false || data["returned"] != float64(3) || data["cap"] != float64(1000) {
		t.Fatalf("bad counters: %#v", data)
	}
	if len(results(t, data)) != 3 {
		t.Fatalf("results=%#v", data["results"])
	}
	if msgs, _ := data["messages"].([]any); len(msgs) != 1 {
		t.Fatalf("messages=%#v", data["messages"])
	}
	if len(mock.JobsCreated) != 1 {
		t.Fatalf("jobs=%d", len(mock.JobsCreated))
	}
	form := mock.JobsCreated[0]
	for key, want := range map[string]string{"search": "search index=main error | head 100", "earliest_time": "-30m", "latest_time": "now", "exec_mode": "normal", "max_count": "1000", "status_buckets": "0", "timeout": "600", "output_mode": "json"} {
		if form.Get(key) != want {
			t.Fatalf("job form %s=%q want %q (form=%v)", key, form.Get(key), want, form)
		}
	}
	if mock.LastPath != "/services/search/jobs/1700000000.1/results" || mock.LastQuery.Get("count") != "100" || mock.LastQuery.Get("offset") != "0" {
		t.Fatalf("results request path=%s query=%v", mock.LastPath, mock.LastQuery)
	}
	if mock.OutputModeMissing != 0 {
		t.Fatalf("%d requests lacked output_mode=json", mock.OutputModeMissing)
	}
}

func TestSplunkSearchRunDefaultsAndIndexInjection(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	inst := bearerInstance("local", mock.Server.URL)
	inst.DefaultIndex = "main"
	inst.DefaultEarliest = "-24h"
	cfg := instanceConfig(t, "local", inst)
	requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "error | head 5", "--poll-ms", "10"))
	form := mock.JobsCreated[0]
	if form.Get("search") != "search index=main error | head 5" || form.Get("earliest_time") != "-24h" {
		t.Fatalf("form=%v", form)
	}
	requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "index=other error", "--earliest", "-5m", "--poll-ms", "10"))
	if form := mock.JobsCreated[1]; form.Get("search") != "search index=other error" || form.Get("earliest_time") != "-5m" {
		t.Fatalf("form=%v", form)
	}
	requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "| tstats count where index=main by host", "--poll-ms", "10"))
	if form := mock.JobsCreated[2]; form.Get("search") != "| tstats count where index=main by host" {
		t.Fatalf("form=%v", form)
	}
}

func TestSplunkSearchRunFieldsAndOffset(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	data := requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "index=main error", "--count", "1", "--offset", "1", "--fields", "_time,host", "--poll-ms", "10"))
	rows := results(t, data)
	if len(rows) != 1 || rows[0]["host"] != "web-2" || rows[0]["_raw"] != nil {
		t.Fatalf("rows=%#v", rows)
	}
	if data["results_truncated"] != true || data["offset"] != float64(1) {
		t.Fatalf("data=%#v", data)
	}
	if got := mock.LastQuery["f"]; len(got) != 2 || got[0] != "_time" || got[1] != "host" {
		t.Fatalf("f=%v", got)
	}
}

func TestSplunkSearchRunWaitTimeoutCancelsJob(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	mock.NeverDone = true
	cfg := bearerConfig(t, mock.Server.URL)
	out := runSplunk(t, cfg, "search", "run", "--query", "index=main error", "--timeout-sec", "1", "--poll-ms", "20")
	errObj := requireErr(t, out, "wait_timeout")
	if errObj["status"] != float64(408) {
		t.Fatalf("status=%#v", errObj)
	}
	data, _ := out["data"].(map[string]any)
	if data["sid"] != "1700000000.1" || data["dispatch_state"] != "RUNNING" || data["cancelled"] != true || data["scan_count"] != float64(10) {
		t.Fatalf("data=%#v", data)
	}
	if len(mock.Controls) != 1 || mock.Controls[0] != "1700000000.1:cancel" {
		t.Fatalf("controls=%v", mock.Controls)
	}
}

func TestSplunkSearchRunFailedJob(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	mock.FailJob = true
	cfg := bearerConfig(t, mock.Server.URL)
	out := runSplunk(t, cfg, "search", "run", "--query", "index=main error", "--poll-ms", "10")
	errObj := requireErr(t, out, "search_failed")
	if errObj["status"] != float64(500) || !strings.Contains(errObj["message"].(string), "Unable to parse the search") {
		t.Fatalf("error=%#v", errObj)
	}
	data, _ := out["data"].(map[string]any)
	if msgs, _ := data["messages"].([]any); len(msgs) != 1 || data["dispatch_state"] != "FAILED" {
		t.Fatalf("data=%#v", data)
	}
}

func TestSplunkSearchOneshot(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	data := requireOK(t, runSplunk(t, cfg, "search", "oneshot", "--query", "index=main | stats count by host", "--earliest", "-15m"))
	if data["exec_mode"] != "oneshot" || data["result_count"] != float64(3) || data["returned"] != float64(3) || data["results_truncated"] != false {
		t.Fatalf("data=%#v", data)
	}
	form := mock.JobsCreated[0]
	if form.Get("exec_mode") != "oneshot" || form.Get("search") != "search index=main | stats count by host" || form.Get("earliest_time") != "-15m" || form.Get("max_count") != "1000" {
		t.Fatalf("form=%v", form)
	}
	if mock.Hits != 1 {
		t.Fatalf("oneshot must be a single request, got %d", mock.Hits)
	}
	paged := requireOK(t, runSplunk(t, cfg, "search", "oneshot", "--query", "index=main", "--count", "1", "--offset", "2", "--fields", "host"))
	rows := results(t, paged)
	if len(rows) != 1 || rows[0]["host"] != "web-1" || len(rows[0]) != 1 || paged["results_truncated"] != false {
		t.Fatalf("paged=%#v", paged)
	}
	if fields, _ := paged["fields"].([]any); len(fields) != 1 || fields[0] != "host" {
		t.Fatalf("fields=%#v", paged["fields"])
	}
}

func TestSplunkCountCapEnforced(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	errObj := requireErr(t, runSplunk(t, cfg, "search", "run", "--query", "index=main", "--count", "5000"), "invalid_args")
	if !strings.Contains(errObj["message"].(string), "1000") {
		t.Fatalf("message=%#v", errObj)
	}
	requireErr(t, runSplunk(t, cfg, "search", "oneshot", "--query", "index=main", "--count", "1001"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "search", "run", "--query", "index=main", "--count", "0"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "search", "run", "--query", "index=main", "--offset", "-1"), "invalid_args")
	if len(mock.JobsCreated) != 0 {
		t.Fatalf("no job may be created when the cap is exceeded: %v", mock.JobsCreated)
	}
	inst := bearerInstance("local", mock.Server.URL)
	inst.MaxResults = 50
	small := instanceConfig(t, "local", inst)
	requireErr(t, runSplunk(t, small, "search", "run", "--query", "index=main", "--count", "51"), "invalid_args")
	data := requireOK(t, runSplunk(t, small, "search", "run", "--query", "index=main", "--count", "50", "--poll-ms", "10"))
	if data["cap"] != float64(50) || mock.JobsCreated[0].Get("max_count") != "50" {
		t.Fatalf("cap=%#v form=%v", data["cap"], mock.JobsCreated[0])
	}
}

func TestSplunkGuardBlocksSideEffectSPL(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	for _, args := range [][]string{
		{"search", "run", "--query", "index=main | outputlookup dump.csv"},
		{"search", "oneshot", "--query", "index=main | head 10 | collect index=summary"},
		{"search", "run", "--query", "| delete"},
	} {
		out := runSplunk(t, cfg, args...)
		errObj := requireErr(t, out, "spl_blocked")
		if errObj["status"] != float64(400) || !strings.Contains(errObj["hint"].(string), "read-only") {
			t.Fatalf("error=%#v", errObj)
		}
		data, _ := out["data"].(map[string]any)
		if data["blocked_command"] == "" {
			t.Fatalf("data=%#v", data)
		}
	}
	if mock.Hits != 0 {
		t.Fatalf("blocked searches must not contact Splunk, hits=%d", mock.Hits)
	}
	out := runSplunk(t, cfg, "saved", "run", "Dump to lookup", "--poll-ms", "10")
	requireErr(t, out, "spl_blocked")
	data, _ := out["data"].(map[string]any)
	if data["blocked_command"] != "outputlookup" || data["saved_search"] != "Dump to lookup" {
		t.Fatalf("data=%#v", data)
	}
	if len(mock.Dispatches) != 0 {
		t.Fatalf("blocked saved search must not be dispatched: %v", mock.Dispatches)
	}
}

func TestSplunkFieldTruncation(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	long := strings.Repeat("a", 3000)
	mock.Results = []map[string]any{{"_time": "2026-09-19T10:00:00.000+08:00", "host": "web-1", "_raw": long, "tags": []any{"short", strings.Repeat("b", 500)}}}
	cfg := bearerConfig(t, mock.Server.URL)
	out, raw := execSplunk(t, "", cfg, "search", "run", "--query", "index=main", "--poll-ms", "10")
	data := requireOK(t, out)
	rows := results(t, data)
	if got := rows[0]["_raw"].(string); len(got) != 2000+len("...(truncated)") || !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("_raw len=%d", len(got))
	}
	if names, _ := data["fields_truncated"].([]any); len(names) != 1 || names[0] != "_raw" {
		t.Fatalf("fields_truncated=%#v", data["fields_truncated"])
	}
	if strings.Contains(raw, long) {
		t.Fatalf("untruncated value printed")
	}
	small := requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "index=main", "--poll-ms", "10", "--max-field-chars", "100"))
	rows = results(t, small)
	if got := rows[0]["_raw"].(string); len(got) != 100+len("...(truncated)") {
		t.Fatalf("_raw len=%d", len(got))
	}
	tags, _ := rows[0]["tags"].([]any)
	if len(tags) != 2 || tags[0] != "short" || len(tags[1].(string)) != 100+len("...(truncated)") {
		t.Fatalf("tags=%#v", tags)
	}
	if names, _ := small["fields_truncated"].([]any); len(names) != 2 || names[0] != "_raw" || names[1] != "tags" {
		t.Fatalf("fields_truncated=%#v", small["fields_truncated"])
	}
	full := requireOK(t, runSplunk(t, cfg, "search", "run", "--query", "index=main", "--poll-ms", "10", "--max-field-chars", "0"))
	if got := results(t, full)[0]["_raw"].(string); got != long {
		t.Fatalf("--max-field-chars 0 must disable truncation")
	}
}

func TestSplunkSearchRunOutputFile(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	long := strings.Repeat("z", 5000)
	mock.Results = []map[string]any{{"_time": "2026-09-19T10:00:00.000+08:00", "host": "web-1", "_raw": long}, {"_time": "2026-09-19T10:01:00.000+08:00", "host": "web-2", "_raw": "short"}}
	cfg := bearerConfig(t, mock.Server.URL)
	target := filepath.Join(t.TempDir(), "results.json")
	out, raw := execSplunk(t, "", cfg, "search", "run", "--query", "index=main", "--poll-ms", "10", "--output", target)
	data := requireOK(t, out)
	if data["path"] != target || data["results_written"] != float64(2) || data["result_count"] != float64(2) || data["sid"] != "1700000000.1" || data["dispatch_state"] != "DONE" {
		t.Fatalf("data=%#v", data)
	}
	if _, has := data["results"]; has {
		t.Fatalf("results must not be printed when --output is used")
	}
	if strings.Contains(raw, long) {
		t.Fatalf("untruncated value printed to stdout")
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if int(data["bytes"].(float64)) != len(b) {
		t.Fatalf("bytes=%v file=%d", data["bytes"], len(b))
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	rows, _ := doc["results"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["_raw"] != long || doc["sid"] != "1700000000.1" {
		t.Fatalf("file doc=%#v", doc)
	}
	requireErr(t, runSplunk(t, cfg, "search", "oneshot", "--query", "index=main", "--output", filepath.Join(t.TempDir(), "missing-dir", "x.json")), "invalid_args")
}

func TestSplunkSavedListAndRun(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	data := requireOK(t, runSplunk(t, cfg, "saved", "list"))
	items, _ := data["saved_searches"].([]any)
	if len(items) != 3 || data["count"] != float64(3) {
		t.Fatalf("data=%#v", data)
	}
	dump := items[1].(map[string]any)
	if dump["name"] != "Dump to lookup" || dump["blocked_command"] != "outputlookup" {
		t.Fatalf("dump=%#v", dump)
	}
	if _, has := items[0].(map[string]any)["blocked_command"]; has {
		t.Fatalf("safe saved search flagged: %#v", items[0])
	}
	filtered := requireOK(t, runSplunk(t, cfg, "saved", "list", "--filter", "nightly", "--count", "10"))
	if filtered["count"] != float64(1) || mock.LastQuery.Get("search") != "nightly" || mock.LastQuery.Get("count") != "10" {
		t.Fatalf("filtered=%#v query=%v", filtered, mock.LastQuery)
	}
	run := requireOK(t, runSplunk(t, cfg, "saved", "run", "Errors last hour", "--poll-ms", "10", "--count", "2", "--earliest", "-2h"))
	if run["saved_search"] != "Errors last hour" || run["search"] != "index=main error | head 100" || run["sid"] != "saved-1700000000.1" || run["dispatch_state"] != "DONE" {
		t.Fatalf("run=%#v", run)
	}
	if len(results(t, run)) != 2 || run["results_truncated"] != true {
		t.Fatalf("run=%#v", run)
	}
	form := mock.Dispatches["Errors last hour"]
	if form.Get("trigger_actions") != "0" || form.Get("dispatch.earliest_time") != "-2h" || form.Get("output_mode") != "json" {
		t.Fatalf("dispatch form=%v", form)
	}
	errObj := requireErr(t, runSplunk(t, cfg, "saved", "run", "Missing", "--poll-ms", "10"), "not_found")
	if !strings.Contains(errObj["message"].(string), "Could not find object id=Missing") {
		t.Fatalf("message=%#v", errObj)
	}
}

func TestSplunkIndexList(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	data := requireOK(t, runSplunk(t, cfg, "index", "list", "--count", "5"))
	indexes, _ := data["indexes"].([]any)
	if len(indexes) != 2 || data["count"] != float64(2) {
		t.Fatalf("data=%#v", data)
	}
	main := indexes[0].(map[string]any)
	if main["name"] != "main" || main["total_event_count"] != float64(123456) || main["disabled"] != false || main["max_time"] == nil || main["min_time"] == nil {
		t.Fatalf("main=%#v", main)
	}
	if mock.LastPath != "/services/data/indexes" || mock.LastQuery.Get("count") != "5" {
		t.Fatalf("path=%s query=%v", mock.LastPath, mock.LastQuery)
	}
}

func TestSplunkJobGetResultsCancel(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	mock.StatusCallsUntilDone = 1
	cfg := bearerConfig(t, mock.Server.URL)
	running := requireOK(t, runSplunk(t, cfg, "search", "job", "get", "1700000000.7"))
	if running["sid"] != "1700000000.7" || running["dispatch_state"] != "RUNNING" || running["is_done"] != false || running["search"] != "search index=main error" {
		t.Fatalf("running=%#v", running)
	}
	done := requireOK(t, runSplunk(t, cfg, "search", "job", "get", "1700000000.7"))
	if done["dispatch_state"] != "DONE" || done["is_done"] != true || done["result_count"] != float64(3) || done["run_duration"] != 0.5 {
		t.Fatalf("done=%#v", done)
	}
	preview := requireOK(t, runSplunk(t, cfg, "search", "job", "results", "1700000000.8"))
	if preview["preview"] != true || mock.LastPath != "/services/search/jobs/1700000000.8/results_preview" {
		t.Fatalf("preview=%#v path=%s", preview, mock.LastPath)
	}
	final := requireOK(t, runSplunk(t, cfg, "search", "job", "results", "1700000000.8", "--count", "2", "--offset", "1"))
	if final["preview"] != false || mock.LastPath != "/services/search/jobs/1700000000.8/results" || len(results(t, final)) != 2 || final["results_truncated"] != false {
		t.Fatalf("final=%#v path=%s", final, mock.LastPath)
	}
	requireErr(t, runSplunk(t, cfg, "search", "job", "cancel", "1700000000.8"), "invalid_args")
	if len(mock.Controls) != 0 {
		t.Fatalf("cancel without --yes must not reach the server")
	}
	cancelled := requireOK(t, runSplunk(t, cfg, "--yes", "search", "job", "cancel", "1700000000.8"))
	if cancelled["cancelled"] != true || len(mock.Controls) != 1 || mock.Controls[0] != "1700000000.8:cancel" {
		t.Fatalf("cancelled=%#v controls=%v", cancelled, mock.Controls)
	}
}

func TestSplunkAPIGet(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := bearerConfig(t, mock.Server.URL)
	requireErr(t, runSplunk(t, cfg, "api", "get", "/foo"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "api", "get", "/services/server/info?count=1"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "api", "get", "/services/../etc"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "api", "get", "/services/server/info", "--query", "novalue"), "invalid_args")
	if mock.Hits != 0 {
		t.Fatalf("invalid paths must not reach the server")
	}
	data := requireOK(t, runSplunk(t, cfg, "api", "get", "/services/server/info", "--query", "count=1"))
	entries, _ := data["entry"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["content"].(map[string]any)["version"] != "9.4.0" {
		t.Fatalf("data=%#v", data)
	}
	if mock.LastQuery.Get("count") != "1" || mock.LastQuery.Get("output_mode") != "json" || mock.LastMethod != "GET" {
		t.Fatalf("query=%v method=%s", mock.LastQuery, mock.LastMethod)
	}
	requireErr(t, runSplunk(t, cfg, "api", "get", "/services/does/not/exist"), "not_found")
}

func TestSplunkErrorMapping(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	inst := bearerInstance("local", mock.Server.URL)
	inst.Auth.Token = "secret-token-should-not-appear-expired"
	cfg := instanceConfig(t, "local", inst)
	errObj := requireErr(t, runSplunk(t, cfg, "auth", "test"), "auth_failed")
	if errObj["status"] != float64(401) || !strings.Contains(errObj["hint"].(string), "expired") || !strings.Contains(errObj["message"].(string), "call not properly authenticated") {
		t.Fatalf("error=%#v", errObj)
	}
	dead := testutil.NewMockSplunk(t)
	deadURL := dead.Server.URL
	dead.Server.Close()
	netCfg := instanceConfig(t, "local", bearerInstance("local", deadURL))
	requireErr(t, runSplunk(t, netCfg, "index", "list"), "network_error")
	requireErr(t, runSplunk(t, instanceConfig(t, "local", bearerInstance("local", "not a url")), "index", "list"), "config_error")
}

func TestSplunkDryRunDoesNotHitServer(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	inst := bearerInstance("local", mock.Server.URL)
	inst.DefaultIndex = "main"
	cfg := instanceConfig(t, "local", inst)
	data := requireOK(t, runSplunk(t, cfg, "--dry-run", "search", "run", "--query", "error | head 5", "--count", "5"))
	if data["dry_run"] != true || data["method"] != "POST" || data["path"] != "/services/search/jobs" {
		t.Fatalf("data=%#v", data)
	}
	form, _ := data["form"].(map[string]any)
	if form["search"] != "search index=main error | head 5" || form["exec_mode"] != "normal" || form["earliest_time"] != "-1h" {
		t.Fatalf("form=%#v", form)
	}
	saved := requireOK(t, runSplunk(t, cfg, "--dry-run", "saved", "run", "Errors last hour"))
	if saved["path"] != "/services/saved/searches/Errors%20last%20hour/dispatch" {
		t.Fatalf("saved=%#v", saved)
	}
	api := requireOK(t, runSplunk(t, cfg, "--dry-run", "api", "get", "/services/server/info", "--query", "count=1"))
	if api["method"] != "GET" || api["path"] != "/services/server/info" {
		t.Fatalf("api=%#v", api)
	}
	cancel := requireOK(t, runSplunk(t, cfg, "--dry-run", "--yes", "search", "job", "cancel", "1700000000.1"))
	if cancel["path"] != "/services/search/jobs/1700000000.1/control" {
		t.Fatalf("cancel=%#v", cancel)
	}
	if mock.Hits != 0 {
		t.Fatalf("dry-run hit the server %d times", mock.Hits)
	}
}

func TestSplunkInstanceAndAuthLifecycle(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	data := requireOK(t, func() map[string]any {
		out, _ := execSplunk(t, "secret-token-should-not-appear\n", cfg, "instance", "add", "prod", "--base-url", "https://splunk-api.example.test:8089", "--token-stdin", "--default", "--default-index", "main", "--max-results", "500")
		return out
	}())
	if data["added"] != true {
		t.Fatalf("data=%#v", data)
	}
	requireErr(t, runSplunk(t, cfg, "instance", "add", "prod", "--base-url", "https://x.example.test:8089", "--token-stdin"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "instance", "add", "noauth", "--base-url", "https://x.example.test:8089"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "instance", "add", "nourl", "--token-stdin"), "invalid_args")
	list := requireOK(t, runSplunk(t, cfg, "instance", "list"))
	instances, _ := list["instances"].([]any)
	if len(instances) != 1 || list["default_instance"] != "prod" {
		t.Fatalf("list=%#v", list)
	}
	first := instances[0].(map[string]any)
	if first["auth"].(map[string]any)["token"] != "***REDACTED***" || first["default_index"] != "main" || first["max_results"] != float64(500) {
		t.Fatalf("instance=%#v", first)
	}
	got := requireOK(t, runSplunk(t, cfg, "instance", "get", "prod"))
	if got["base_url"] != "https://splunk-api.example.test:8089" || got["auth"].(map[string]any)["type"] != "bearer_token" {
		t.Fatalf("get=%#v", got)
	}
	requireErr(t, runSplunk(t, cfg, "instance", "get", "nope"), "not_found")
	requireOK(t, runSplunk(t, cfg, "instance", "update", "prod", "--default-earliest", "-24h", "--max-results", "0"))
	updated := requireOK(t, runSplunk(t, cfg, "instance", "get", "prod"))
	if updated["default_earliest"] != "-24h" || updated["max_results"] != nil {
		t.Fatalf("updated=%#v", updated)
	}
	requireErr(t, runSplunk(t, cfg, "instance", "update", "prod"), "invalid_args")
	requireErr(t, runSplunk(t, cfg, "instance", "update", "nope", "--base-url", "https://x.example.test:8089"), "not_found")
	def := requireOK(t, runSplunk(t, cfg, "instance", "default"))
	if def["default_instance"] != "prod" {
		t.Fatalf("default=%#v", def)
	}
	requireErr(t, runSplunk(t, cfg, "instance", "default", "nope"), "not_found")

	login, _ := execSplunk(t, "secret-password-should-not-appear\n", cfg, "auth", "login", "--username", "agent", "--auth-type", "basic_password", "--password-stdin")
	if requireOK(t, login)["auth_type"] != "basic_password" {
		t.Fatalf("login=%#v", login)
	}
	loaded, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if auth := loaded.Splunk.Instances[0].Auth; auth.Type != "basic_password" || auth.Username != "agent" || auth.Password != "secret-password-should-not-appear" || auth.Token != "" {
		t.Fatalf("stored auth=%#v", config.RedactAuth(auth))
	}
	bad, _ := execSplunk(t, "secret-token-should-not-appear\n", cfg, "auth", "login", "--username", "agent", "--token-stdin")
	requireErr(t, bad, "invalid_args")
	tokenLogin, _ := execSplunk(t, "secret-token-should-not-appear\n", cfg, "auth", "login", "--token-stdin")
	requireOK(t, tokenLogin)
	loaded, _ = config.Load(cfg)
	if auth := loaded.Splunk.Instances[0].Auth; auth.Type != "bearer_token" || auth.Token != "secret-token-should-not-appear" || auth.Password != "" {
		t.Fatalf("stored auth=%#v", config.RedactAuth(auth))
	}
	requireErr(t, runSplunk(t, cfg, "auth", "logout"), "invalid_args")
	requireOK(t, runSplunk(t, cfg, "--yes", "auth", "logout"))
	loaded, _ = config.Load(cfg)
	if auth := loaded.Splunk.Instances[0].Auth; auth.Token != "" || auth.Password != "" || auth.Type != "" {
		t.Fatalf("auth not cleared: %#v", config.RedactAuth(auth))
	}
	requireErr(t, runSplunk(t, cfg, "instance", "remove", "prod"), "invalid_args")
	requireOK(t, runSplunk(t, cfg, "--yes", "instance", "remove", "prod"))
	final := requireOK(t, runSplunk(t, cfg, "instance", "list"))
	if instances, _ := final["instances"].([]any); len(instances) != 0 || final["default_instance"] != "" {
		t.Fatalf("final=%#v", final)
	}
}

func TestSplunkSecretsNeverPrintedOnErrors(t *testing.T) {
	mock := testutil.NewMockSplunk(t)
	cfg := instanceConfig(t, "local", sessionInstance("local", mock.Server.URL))
	// Session auth followed by an error response: the message must carry the
	// Splunk text but never the session key or password.
	_, raw := execSplunk(t, "", cfg, "api", "get", "/services/does/not/exist")
	if !strings.Contains(raw, "not_found") {
		t.Fatalf("out=%s", raw)
	}
	_, raw = execSplunk(t, "", cfg, "--verbose", "search", "run", "--query", "index=main", "--poll-ms", "10")
	if !strings.Contains(raw, "\"ok\": true") {
		t.Fatalf("out=%s", raw)
	}
}

func TestSplunkCommandsExposeMetadata(t *testing.T) {
	root := NewRoot()
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"commands", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	obj := testutil.AssertOKEnvelope(t, b.Bytes())
	data, _ := obj["data"].(map[string]any)
	commands, _ := data["commands"].([]any)
	if len(commands) < 20 {
		t.Fatalf("too few commands: %d", len(commands))
	}
	names := map[string]bool{}
	for _, item := range commands {
		m, _ := item.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, want := range []string{"search.run", "search.oneshot", "search.job.get", "search.job.results", "search.job.cancel", "saved.list", "saved.run", "index.list", "api.get", "auth.test", "instance.add"} {
		if !names[want] {
			t.Fatalf("missing command %s in %v", want, names)
		}
	}
	b.Reset()
	root = NewRoot()
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"schema", "search.run", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	schema := testutil.AssertOKEnvelope(t, b.Bytes())
	flags, _ := schema["data"].(map[string]any)["flags"].([]any)
	found := false
	for _, f := range flags {
		m, _ := f.(map[string]any)
		if m["name"] == "query" && m["required"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("schema search.run must mark --query required: %s", b.String())
	}
}

func TestSplunkHelpLLMTips(t *testing.T) {
	root := NewRoot()
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"help", "llm", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.AssertOKEnvelope(t, b.Bytes())
	for _, want := range []string{"explicit time range", "| head 100", "Never dump raw events beyond the cap", "--output results.json", "read-only", "spl_blocked", "default_index", "wait_timeout", "error.code and error.hint"} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("missing %q in help llm output", want)
		}
	}
}
