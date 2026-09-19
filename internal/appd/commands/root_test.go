package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/testutil"
)

func runAppD(t *testing.T, cfg, stdin string, args ...string) (map[string]any, string) {
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
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, b.String())
	}
	return out, b.String()
}

func requireOK(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("command failed: %#v", out)
	}
	data, _ := out["data"].(map[string]any)
	return data
}

func requireCode(t *testing.T, out map[string]any, code string) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected error %s, got success: %#v", code, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if got, _ := errObj["code"].(string); got != code {
		t.Fatalf("expected error.code=%s got %q: %#v", code, got, out)
	}
	return errObj
}

func assertNoSecrets(t *testing.T, s string) {
	t.Helper()
	for _, secret := range testutil.Secrets {
		if strings.Contains(s, secret) {
			t.Fatalf("secret leaked: %s in %s", secret, s)
		}
	}
	if strings.Contains(strings.ToLower(s), "authorization") {
		t.Fatalf("authorization header leaked: %s", s)
	}
}

func writeCfg(t *testing.T, content string) string {
	t.Helper()
	path, err := testutil.WriteConfig(content)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func jsonCfg(t *testing.T, cfg config.RootConfig) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func listLen(t *testing.T, data map[string]any, key string) int {
	t.Helper()
	items, ok := data[key].([]any)
	if !ok {
		t.Fatalf("missing list %s in %#v", key, data)
	}
	return len(items)
}

func TestAppDAPIClientExchangesTokenAndListsApplications(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	out, raw := runAppD(t, cfg, "", "app", "list")
	data := requireOK(t, out)
	if data["count"] != float64(2) || listLen(t, data, "applications") != 2 {
		t.Fatalf("bad app list: %#v", data)
	}
	first, _ := data["applications"].([]any)[0].(map[string]any)
	if first["name"] != "ecommerce" || first["id"] != float64(1) {
		t.Fatalf("bad first application: %#v", first)
	}
	if mock.TokenHits != 1 || mock.LastGrantType != "client_credentials" || mock.LastClientID != "efp-reader@customer1" {
		t.Fatalf("bad oauth exchange: hits=%d grant=%q client_id=%q", mock.TokenHits, mock.LastGrantType, mock.LastClientID)
	}
	if !strings.HasPrefix(mock.LastContentType, "application/x-www-form-urlencoded") {
		t.Fatalf("oauth content type=%q", mock.LastContentType)
	}
	if mock.LastAuth != "Bearer "+mock.Token {
		t.Fatalf("expected bearer auth on REST call, got %q", mock.LastAuth)
	}
	if mock.LastPath != "/controller/rest/applications" || mock.LastQuery.Get("output") != "JSON" {
		t.Fatalf("bad request path=%s query=%v", mock.LastPath, mock.LastQuery)
	}
	if out["instance"] != "local" {
		t.Fatalf("missing instance in envelope: %#v", out)
	}
	assertNoSecrets(t, raw)
}

func TestAppDBasicAuthQualifiesUserWithAccount(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDBasicConfig(mock.Server.URL))
	out, raw := runAppD(t, cfg, "", "app", "list")
	requireOK(t, out)
	if mock.TokenHits != 0 {
		t.Fatalf("basic auth must not call the oauth endpoint, hits=%d", mock.TokenHits)
	}
	if !strings.HasPrefix(mock.LastAuth, "Basic ") {
		t.Fatalf("expected basic auth, got %q", mock.LastAuth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(mock.LastAuth, "Basic "))
	if err != nil || string(decoded) != "reader@customer1:secret-password-should-not-appear" {
		t.Fatalf("bad basic credentials %q err=%v", string(decoded), err)
	}
	assertNoSecrets(t, raw)
}

func TestAppDAuthTestReportsIdentityWithoutSecrets(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	out, raw := runAppD(t, cfg, "", "auth", "test")
	data := requireOK(t, out)
	if data["authenticated"] != true || data["application_count"] != float64(2) || data["auth_type"] != "api_client" || data["account"] != "customer1" || data["identity"] != "efp-reader@customer1" {
		t.Fatalf("bad auth test data: %#v", data)
	}
	assertNoSecrets(t, raw)
}

func TestAppDAuthFailedOnBadClientSecret(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, strings.Replace(testutil.AppDConfig(mock.Server.URL), "secret-api-key-should-not-appear", "wrong-secret-should-not-appear", 1))
	out, raw := runAppD(t, cfg, "", "app", "list")
	errObj := requireCode(t, out, "auth_failed")
	if errObj["status"] != float64(401) {
		t.Fatalf("expected status 401: %#v", errObj)
	}
	if !strings.Contains(errObj["hint"].(string), "efp-reader@customer1") {
		t.Fatalf("hint should name the client_id: %#v", errObj)
	}
	if strings.Contains(raw, "wrong-secret-should-not-appear") {
		t.Fatalf("client secret leaked: %s", raw)
	}
	assertNoSecrets(t, raw)
	if mock.LastClientID != "efp-reader@customer1" {
		t.Fatalf("client_id=%q", mock.LastClientID)
	}
}

func TestAppDRejectsUnsupportedAuthTypeAndMissingAccount(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	unsupported := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
		{Name: "local", BaseURL: mock.Server.URL, Account: "customer1", Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "secret-api-key-should-not-appear"}},
	}}})
	out, raw := runAppD(t, unsupported, "", "app", "list")
	errObj := requireCode(t, out, "config_error")
	if !strings.Contains(errObj["hint"].(string), "api_client") {
		t.Fatalf("hint should explain api_client: %#v", errObj)
	}
	assertNoSecrets(t, raw)

	noAccount := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
		{Name: "local", BaseURL: mock.Server.URL, Auth: config.AuthConfig{Type: "api_client", Username: "efp-reader", APIKey: "secret-api-key-should-not-appear"}},
	}}})
	out, raw = runAppD(t, noAccount, "", "app", "list")
	errObj = requireCode(t, out, "config_error")
	if !strings.Contains(errObj["message"].(string), "account") {
		t.Fatalf("message should mention the account: %#v", errObj)
	}
	assertNoSecrets(t, raw)
	if mock.Hits != 0 {
		t.Fatalf("config errors must not contact the controller, hits=%d", mock.Hits)
	}

	qualified := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
		{Name: "local", BaseURL: mock.Server.URL, Auth: config.AuthConfig{Type: "api_client", Username: "efp-reader@customer1", APIKey: "secret-api-key-should-not-appear"}},
	}}})
	out, _ = runAppD(t, qualified, "", "app", "list")
	requireOK(t, out)
	if mock.LastClientID != "efp-reader@customer1" {
		t.Fatalf("qualified client name must be used as-is, got %q", mock.LastClientID)
	}

	noCreds := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
		{Name: "local", BaseURL: mock.Server.URL, Account: "customer1"},
	}}})
	out, _ = runAppD(t, noCreds, "", "app", "list")
	requireCode(t, out, "config_error")
}

func TestAppDInstanceResolutionErrors(t *testing.T) {
	empty := jsonCfg(t, config.RootConfig{Version: 1})
	out, _ := runAppD(t, empty, "", "app", "list")
	requireCode(t, out, "no_instance_configured")

	two := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: "https://a.example.test", Account: "x", Auth: config.AuthConfig{Type: "api_client", Username: "c", APIKey: "k"}},
		{Name: "b", BaseURL: "https://b.example.test", Account: "x", Auth: config.AuthConfig{Type: "api_client", Username: "c", APIKey: "k"}},
	}}})
	out, _ = runAppD(t, two, "", "app", "list")
	errObj := requireCode(t, out, "instance_required")
	if !strings.Contains(errObj["hint"].(string), "--instance") {
		t.Fatalf("expected hint about --instance: %#v", errObj)
	}
	out, _ = runAppD(t, two, "", "--instance", "nope", "app", "list")
	requireCode(t, out, "instance_required")
}

func TestAppDAppGetByNameOrID(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	data := requireOK(t, firstOut(runAppD(t, cfg, "", "app", "get", "ECOMMERCE")))
	if data["id"] != float64(1) || data["name"] != "ecommerce" {
		t.Fatalf("bad app by name: %#v", data)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "app", "get", "2")))
	if data["name"] != "payments" {
		t.Fatalf("bad app by id: %#v", data)
	}
	out, _ := runAppD(t, cfg, "", "app", "get", "missing")
	requireCode(t, out, "not_found")
}

func firstOut(out map[string]any, _ string) map[string]any { return out }

func TestAppDTierNodeBTAndBackend(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))

	data := requireOK(t, firstOut(runAppD(t, cfg, "", "tier", "list", "--app", "ecommerce")))
	if data["count"] != float64(2) || mock.LastPath != "/controller/rest/applications/ecommerce/tiers" {
		t.Fatalf("bad tier list: %#v path=%s", data, mock.LastPath)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "tier", "get", "--app", "1", "web")))
	if data["name"] != "web" || data["id"] != float64(10) || mock.LastPath != "/controller/rest/applications/1/tiers/web" {
		t.Fatalf("bad tier get: %#v path=%s", data, mock.LastPath)
	}
	out, _ := runAppD(t, cfg, "", "tier", "get", "--app", "ecommerce", "nope")
	requireCode(t, out, "not_found")
	out, _ = runAppD(t, cfg, "", "tier", "list")
	requireCode(t, out, "invalid_args")

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "node", "list", "--app", "ecommerce")))
	if data["count"] != float64(3) || mock.LastPath != "/controller/rest/applications/ecommerce/nodes" {
		t.Fatalf("bad node list: %#v path=%s", data, mock.LastPath)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "node", "list", "--app", "ecommerce", "--tier", "web")))
	if data["count"] != float64(2) || data["tier"] != "web" || mock.LastPath != "/controller/rest/applications/ecommerce/tiers/web/nodes" {
		t.Fatalf("bad tier node list: %#v path=%s", data, mock.LastPath)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "node", "get", "--app", "ecommerce", "web-node-1")))
	if data["name"] != "web-node-1" || data["machineName"] != "ip-10-0-0-1" {
		t.Fatalf("bad node get: %#v", data)
	}
	out, _ = runAppD(t, cfg, "", "node", "get", "--app", "ecommerce", "nope")
	requireCode(t, out, "not_found")

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "bt", "list", "--app", "ecommerce")))
	if data["count"] != float64(3) || mock.LastPath != "/controller/rest/applications/ecommerce/business-transactions" {
		t.Fatalf("bad bt list: %#v", data)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "bt", "list", "--app", "ecommerce", "--tier", "WEB")))
	if data["count"] != float64(2) || data["tier_filter"] != "WEB" {
		t.Fatalf("bad bt list by tier name: %#v", data)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "bt", "list", "--app", "ecommerce", "--tier", "11")))
	if data["count"] != float64(1) {
		t.Fatalf("bad bt list by tier id: %#v", data)
	}
	bt, _ := data["business_transactions"].([]any)[0].(map[string]any)
	if bt["name"] != "/inventory/lookup" {
		t.Fatalf("bad filtered bt: %#v", bt)
	}

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "backend", "list", "--app", "ecommerce")))
	if data["count"] != float64(2) || mock.LastPath != "/controller/rest/applications/ecommerce/backends" {
		t.Fatalf("bad backend list: %#v", data)
	}

	out, _ = runAppD(t, cfg, "", "tier", "list", "--app", "Web Shop")
	requireCode(t, out, "not_found")
	if mock.LastPath != "/controller/rest/applications/Web Shop/tiers" {
		t.Fatalf("application name must be path-escaped, got %s", mock.LastPath)
	}
}

func TestAppDMetricBrowseGetAndPresets(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))

	data := requireOK(t, firstOut(runAppD(t, cfg, "", "metric", "browse", "--app", "ecommerce")))
	if data["count"] != float64(3) || mock.LastQuery.Has("metric-path") || mock.LastPath != "/controller/rest/applications/ecommerce/metrics" {
		t.Fatalf("bad root browse: %#v query=%v", data, mock.LastQuery)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "metric", "browse", "--app", "ecommerce", "--path", "Business Transaction Performance|Business Transactions|web|/checkout")))
	if mock.LastQuery.Get("metric-path") != "Business Transaction Performance|Business Transactions|web|/checkout" || data["metric_path"] != "Business Transaction Performance|Business Transactions|web|/checkout" {
		t.Fatalf("bad browse path: %#v query=%v", data, mock.LastQuery)
	}

	path := "Overall Application Performance|Average Response Time (ms)"
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "metric", "get", "--app", "ecommerce", "--path", path)))
	if mock.LastPath != "/controller/rest/applications/ecommerce/metric-data-v2" {
		t.Fatalf("default api must be v2, path=%s", mock.LastPath)
	}
	q := mock.LastQuery
	if q.Get("metric-path") != path || q.Get("time-range-type") != "BEFORE_NOW" || q.Get("duration-in-mins") != "60" || q.Get("rollup") != "false" || q.Get("output") != "JSON" {
		t.Fatalf("bad metric query: %v", q)
	}
	if data["api"] != "v2" || data["rollup"] != false || data["count"] != float64(1) || data["metric_path"] != path {
		t.Fatalf("bad metric data: %#v", data)
	}
	tr, _ := data["time_range"].(map[string]any)
	if tr["type"] != "BEFORE_NOW" || tr["duration_in_mins"] != float64(60) {
		t.Fatalf("bad time_range echo: %#v", tr)
	}
	metrics, _ := data["metrics"].([]any)
	if m, _ := metrics[0].(map[string]any); m["metricPath"] != path {
		t.Fatalf("metric path not echoed by mock: %#v", metrics)
	}

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "metric", "get", "--app", "ecommerce", "--path", path, "--api", "v1", "--rollup", "--duration-mins", "15")))
	if mock.LastPath != "/controller/rest/applications/ecommerce/metric-data" || mock.LastQuery.Get("rollup") != "true" || mock.LastQuery.Get("duration-in-mins") != "15" || data["api"] != "v1" || data["rollup"] != true {
		t.Fatalf("bad v1 rollup call: path=%s query=%v data=%#v", mock.LastPath, mock.LastQuery, data)
	}
	out, _ := runAppD(t, cfg, "", "metric", "get", "--app", "ecommerce", "--path", path, "--api", "v3")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, "", "metric", "get", "--app", "ecommerce")
	requireCode(t, out, "invalid_args")

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--preset", "bt-response-time", "--tier", "web", "--bt", "/checkout"}, "Business Transaction Performance|Business Transactions|web|/checkout|Average Response Time (ms)"},
		{[]string{"--preset", "bt-calls", "--tier", "web", "--bt", "/checkout"}, "Business Transaction Performance|Business Transactions|web|/checkout|Calls per Minute"},
		{[]string{"--preset", "BT-ERRORS", "--tier", "web", "--bt", "/checkout"}, "Business Transaction Performance|Business Transactions|web|/checkout|Errors per Minute"},
		{[]string{"--preset", "tier-cpu", "--tier", "web"}, "Application Infrastructure Performance|web|Hardware Resources|CPU|%Busy"},
		{[]string{"--preset", "node-heap", "--tier", "web", "--node", "web-node-1"}, "Application Infrastructure Performance|web|Individual Nodes|web-node-1|JVM|Memory:Heap|Used %"},
	} {
		data = requireOK(t, firstOut(runAppD(t, cfg, "", append([]string{"metric", "preset", "--app", "ecommerce"}, tc.args...)...)))
		if mock.LastQuery.Get("metric-path") != tc.want || data["metric_path"] != tc.want || data["preset"] != strings.ToLower(tc.args[1]) {
			t.Fatalf("preset %v expanded to %q (data=%#v), want %q", tc.args, mock.LastQuery.Get("metric-path"), data, tc.want)
		}
		if mock.LastPath != "/controller/rest/applications/ecommerce/metric-data-v2" || mock.LastQuery.Get("time-range-type") != "BEFORE_NOW" {
			t.Fatalf("preset must call metric-data-v2 with a time range: %s %v", mock.LastPath, mock.LastQuery)
		}
	}
	before := mock.Hits
	for _, args := range [][]string{
		{"--preset", "bt-response-time", "--tier", "web"},
		{"--preset", "tier-cpu"},
		{"--preset", "node-heap", "--tier", "web"},
		{"--preset", "nope", "--tier", "web"},
		{"--tier", "web"},
	} {
		out, _ = runAppD(t, cfg, "", append([]string{"metric", "preset", "--app", "ecommerce"}, args...)...)
		requireCode(t, out, "invalid_args")
	}
	if mock.Hits != before {
		t.Fatalf("invalid presets must not contact the controller")
	}
}

func TestAppDTimeFlagsResolveEveryRangeType(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	run := func(args ...string) map[string]any {
		t.Helper()
		return requireOK(t, firstOut(runAppD(t, cfg, "", append([]string{"violation", "list", "--app", "ecommerce"}, args...)...)))
	}
	run()
	if q := mock.LastQuery; q.Get("time-range-type") != "BEFORE_NOW" || q.Get("duration-in-mins") != "60" || q.Has("start-time") || q.Has("end-time") {
		t.Fatalf("default range: %v", q)
	}
	run("--duration-mins", "120")
	if mock.LastQuery.Get("duration-in-mins") != "120" {
		t.Fatalf("duration: %v", mock.LastQuery)
	}
	data := run("--start-time", "1700000000000", "--end-time", "1700003600000")
	if q := mock.LastQuery; q.Get("time-range-type") != "BETWEEN_TIMES" || q.Get("start-time") != "1700000000000" || q.Get("end-time") != "1700003600000" || q.Has("duration-in-mins") {
		t.Fatalf("between times: %v", q)
	}
	if tr, _ := data["time_range"].(map[string]any); tr["type"] != "BETWEEN_TIMES" || tr["start_time"] != float64(1700000000000) || tr["end_time"] != float64(1700003600000) {
		t.Fatalf("between times echo: %#v", data["time_range"])
	}
	run("--start-time", "2023-11-14T22:13:20Z", "--end-time", "2023-11-14T23:13:20+00:00")
	if q := mock.LastQuery; q.Get("start-time") != "1700000000000" || q.Get("end-time") != "1700003600000" {
		t.Fatalf("rfc3339 conversion: %v", q)
	}
	run("--start-time", "1700000000", "--end-time", "1700003600")
	if q := mock.LastQuery; q.Get("start-time") != "1700000000000" || q.Get("end-time") != "1700003600000" {
		t.Fatalf("epoch seconds scaling: %v", q)
	}
	run("--before-time", "1700003600000", "--duration-mins", "30")
	if q := mock.LastQuery; q.Get("time-range-type") != "BEFORE_TIME" || q.Get("end-time") != "1700003600000" || q.Get("duration-in-mins") != "30" || q.Has("start-time") {
		t.Fatalf("before time: %v", q)
	}
	run("--after-time", "2023-11-14T22:13:20Z")
	if q := mock.LastQuery; q.Get("time-range-type") != "AFTER_TIME" || q.Get("start-time") != "1700000000000" || q.Get("duration-in-mins") != "60" || q.Has("end-time") {
		t.Fatalf("after time: %v", q)
	}
	before := mock.Hits
	for _, args := range [][]string{
		{"--start-time", "1700000000000"},
		{"--end-time", "1700000000000"},
		{"--before-time", "1700000000000", "--after-time", "1700000000000"},
		{"--start-time", "1700000000000", "--end-time", "1700003600000", "--before-time", "1700000000000"},
		{"--start-time", "1700003600000", "--end-time", "1700000000000"},
		{"--duration-mins", "0"},
		{"--start-time", "yesterday", "--end-time", "1700003600000"},
	} {
		out, _ := runAppD(t, cfg, "", append([]string{"violation", "list", "--app", "ecommerce"}, args...)...)
		errObj := requireCode(t, out, "invalid_args")
		if !strings.Contains(errObj["hint"].(string), "--duration-mins") {
			t.Fatalf("time hint missing for %v: %#v", args, errObj)
		}
	}
	if mock.Hits != before {
		t.Fatalf("invalid time flags must not contact the controller")
	}
}

func TestAppDSnapshotListFiltersAndTrims(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))

	data := requireOK(t, firstOut(runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce")))
	if data["count_returned"] != float64(3) || data["truncated"] != false || data["max_results"] != float64(50) {
		t.Fatalf("bad snapshot list: %#v", data)
	}
	if q := mock.LastQuery; q.Get("maximum-results") != "50" || q.Get("time-range-type") != "BEFORE_NOW" || q.Has("error-occurred") || mock.LastPath != "/controller/rest/applications/ecommerce/request-snapshots" {
		t.Fatalf("bad snapshot query: %s %v", mock.LastPath, q)
	}
	first, _ := data["snapshots"].([]any)[0].(map[string]any)
	if first["requestGUID"] != "4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f" || first["userExperience"] != "ERROR" || first["errorDetails"] != "java.lang.NullPointerException" || first["URL"] != "/checkout" {
		t.Fatalf("summary fields missing: %#v", first)
	}
	for _, dropped := range []string{"callChain", "httpParameters", "transactionProperties", "businessData", "localStartTime"} {
		if _, present := first[dropped]; present {
			t.Fatalf("field %s should be trimmed from list output: %#v", dropped, first)
		}
	}
	if _, present := first["exitCalls"]; !present {
		t.Fatalf("exitCalls should be kept: %#v", first)
	}

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce", "--errors-only")))
	if data["count_returned"] != float64(1) || mock.LastQuery.Get("error-occurred") != "true" {
		t.Fatalf("errors-only: %#v %v", data, mock.LastQuery)
	}
	if filters, _ := data["filters"].(map[string]any); filters["errors_only"] != true {
		t.Fatalf("filters echo: %#v", data["filters"])
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce", "--user-experience", "error, very_slow")))
	if data["count_returned"] != float64(2) || mock.LastQuery.Get("user-experience") != "ERROR,VERY_SLOW" {
		t.Fatalf("user-experience: %#v %v", data, mock.LastQuery)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce", "--bt-ids", "202", "--tier-ids", "10", "--node-ids", "100,101")))
	if q := mock.LastQuery; q.Get("business-transaction-ids") != "202" || q.Get("application-component-ids") != "10" || q.Get("application-component-node-ids") != "100,101" || data["count_returned"] != float64(1) {
		t.Fatalf("id filters: %#v %v", data, q)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce", "--max-results", "1", "--first-in-chain", "--need-exit-calls", "--need-props")))
	if q := mock.LastQuery; q.Get("maximum-results") != "1" || q.Get("first-in-chain") != "true" || q.Get("need-exit-calls") != "true" || q.Get("need-props") != "true" {
		t.Fatalf("bool filters: %v", q)
	}
	if data["count_returned"] != float64(1) || data["truncated"] != true {
		t.Fatalf("truncation: %#v", data)
	}
	out, _ := runAppD(t, cfg, "", "snapshot", "list", "--app", "ecommerce", "--max-results", "0")
	requireCode(t, out, "invalid_args")
}

func TestAppDSnapshotGetReturnsFullSnapshotRedacted(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	guid := "4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f"
	out, raw := runAppD(t, cfg, "", "snapshot", "get", "--app", "ecommerce", "--guid", guid)
	data := requireOK(t, out)
	if q := mock.LastQuery; q.Get("guids") != guid || q.Get("need-exit-calls") != "true" || q.Get("need-props") != "true" || q.Get("time-range-type") != "BEFORE_NOW" || q.Get("duration-in-mins") != "20160" {
		t.Fatalf("snapshot get query: %v", q)
	}
	snapshot, _ := data["snapshot"].(map[string]any)
	if snapshot["requestGUID"] != guid || snapshot["callChain"] != "Component:10" {
		t.Fatalf("snapshot get should return the full object: %#v", snapshot)
	}
	if data["call_graph_available"] != false || !strings.Contains(data["note"].(string), "call graph") {
		t.Fatalf("call graph note missing: %#v", data)
	}
	if !strings.Contains(raw, "***REDACTED***") {
		t.Fatalf("http parameter secret should be redacted: %s", raw)
	}
	assertNoSecrets(t, raw)

	out, _ = runAppD(t, cfg, "", "snapshot", "get", "--app", "ecommerce", "--guid", "00000000-0000-0000-0000-000000000000", "--duration-mins", "30")
	requireCode(t, out, "not_found")
	if mock.LastQuery.Get("duration-in-mins") != "30" {
		t.Fatalf("time flags must apply to snapshot get: %v", mock.LastQuery)
	}
	out, _ = runAppD(t, cfg, "", "snapshot", "get", "--app", "ecommerce")
	requireCode(t, out, "invalid_args")
}

func TestAppDViolationAndEventList(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))

	data := requireOK(t, firstOut(runAppD(t, cfg, "", "violation", "list", "--app", "ecommerce", "--duration-mins", "120")))
	if data["count"] != float64(1) || mock.LastPath != "/controller/rest/applications/ecommerce/problems/healthrule-violations" || mock.LastQuery.Get("duration-in-mins") != "120" {
		t.Fatalf("bad violation list: %#v %s %v", data, mock.LastPath, mock.LastQuery)
	}
	violation, _ := data["violations"].([]any)[0].(map[string]any)
	if violation["severity"] != "CRITICAL" || violation["status"] != "OPEN" {
		t.Fatalf("bad violation: %#v", violation)
	}

	out, _ := runAppD(t, cfg, "", "event", "list", "--app", "ecommerce")
	requireCode(t, out, "invalid_args")

	out, raw := runAppD(t, cfg, "", "event", "list", "--app", "ecommerce", "--event-types", "application_deployment", "--duration-mins", "1440")
	data = requireOK(t, out)
	if q := mock.LastQuery; q.Get("event-types") != "APPLICATION_DEPLOYMENT" || q.Get("severities") != "ERROR,WARN,INFO" || q.Get("duration-in-mins") != "1440" || mock.LastPath != "/controller/rest/applications/ecommerce/events" {
		t.Fatalf("bad event query: %s %v", mock.LastPath, q)
	}
	if data["count"] != float64(1) || data["event_types"] != "APPLICATION_DEPLOYMENT" || data["severities"] != "ERROR,WARN,INFO" {
		t.Fatalf("bad event data: %#v", data)
	}
	event, _ := data["events"].([]any)[0].(map[string]any)
	if event["type"] != "APPLICATION_DEPLOYMENT" || event["summary"] != "Deployed build 42 by ci-bot" {
		t.Fatalf("bad event: %#v", event)
	}
	assertNoSecrets(t, raw)

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "event", "list", "--app", "ecommerce", "--event-types", "APPLICATION_ERROR,DIAGNOSTIC_SESSION", "--severities", "error")))
	if mock.LastQuery.Get("severities") != "ERROR" || data["count"] != float64(1) {
		t.Fatalf("severity filter: %#v %v", data, mock.LastQuery)
	}
}

func TestAppDAPIGetGuardsPaths(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))

	out, _ := runAppD(t, cfg, "", "api", "get", "/controller/rest/applications")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("api get failed: %#v", out)
	}
	if items, _ := out["data"].([]any); len(items) != 2 {
		t.Fatalf("expected raw application array: %#v", out["data"])
	}
	requireOK(t, firstOut(runAppD(t, cfg, "", "api", "get", "controller/rest/applications/ecommerce/tiers?foo=bar", "--query", "x=y")))
	if mock.LastPath != "/controller/rest/applications/ecommerce/tiers" || mock.LastQuery.Get("foo") != "bar" || mock.LastQuery.Get("x") != "y" || mock.LastQuery.Get("output") != "JSON" {
		t.Fatalf("bad raw query: %s %v", mock.LastPath, mock.LastQuery)
	}
	requireOK(t, firstOut(runAppD(t, cfg, "", "api", "get", mock.Server.URL+"/controller/rest/applications/ecommerce/backends")))
	if mock.LastPath != "/controller/rest/applications/ecommerce/backends" {
		t.Fatalf("absolute instance url: %s", mock.LastPath)
	}

	before := mock.Hits
	for _, tc := range []struct{ path, code string }{
		{"/controller/api/oauth/access_token", "invalid_args"},
		{"/controller/rest/../api/oauth/access_token", "invalid_args"},
		{"/api/json", "invalid_args"},
		{"https://evil.example/controller/rest/applications", "instance_url_mismatch"},
	} {
		out, raw := runAppD(t, cfg, "", "api", "get", tc.path)
		requireCode(t, out, tc.code)
		assertNoSecrets(t, raw)
	}
	out, _ = runAppD(t, cfg, "", "api", "get", "/controller/rest/applications", "--query", "novalue")
	requireCode(t, out, "invalid_args")
	if mock.Hits != before {
		t.Fatalf("guarded paths must not contact the controller")
	}
}

func TestAppDDryRunDoesNotContactController(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	out, raw := runAppD(t, cfg, "", "--dry-run", "snapshot", "list", "--app", "ecommerce", "--errors-only")
	data := requireOK(t, out)
	if data["dry_run"] != true || data["method"] != "GET" || data["path"] != "/controller/rest/applications/ecommerce/request-snapshots" || data["auth_type"] != "api_client" {
		t.Fatalf("bad dry-run data: %#v", data)
	}
	q, _ := data["query"].(map[string]any)
	if q["output"] != "JSON" || q["error-occurred"] != "true" || q["time-range-type"] != "BEFORE_NOW" {
		t.Fatalf("bad dry-run query: %#v", q)
	}
	if !strings.HasPrefix(data["url"].(string), mock.Server.URL) {
		t.Fatalf("dry-run url should be on the instance: %#v", data)
	}
	assertNoSecrets(t, raw)
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "--dry-run", "api", "get", "/controller/rest/applications", "--query", "a=b")))
	if q, _ := data["query"].(map[string]any); q["a"] != "b" {
		t.Fatalf("bad api dry-run: %#v", data)
	}
	if mock.Hits != 0 || mock.TokenHits != 0 {
		t.Fatalf("dry-run contacted the controller: hits=%d token=%d", mock.Hits, mock.TokenHits)
	}
}

func TestAppDBaseURLWithControllerSuffix(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL+"/controller/"))
	requireOK(t, firstOut(runAppD(t, cfg, "", "app", "list")))
	if mock.LastPath != "/controller/rest/applications" {
		t.Fatalf("controller suffix doubled: %s", mock.LastPath)
	}
}

func TestAppDInstanceAndAuthLifecycle(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	secret := "secret-api-key-should-not-appear\n"

	out, raw := runAppD(t, cfg, secret, "instance", "add", "prod", "--base-url", mock.Server.URL, "--account", "customer1", "--auth-type", "api_client", "--username", "efp-reader", "--api-key-stdin", "--default")
	data := requireOK(t, out)
	if data["added"] != true || data["auth_type"] != "api_client" || data["account"] != "customer1" {
		t.Fatalf("bad add: %#v", data)
	}
	assertNoSecrets(t, raw)

	out, raw = runAppD(t, cfg, "", "instance", "list")
	data = requireOK(t, out)
	if data["default_instance"] != "prod" || listLen(t, data, "instances") != 1 {
		t.Fatalf("bad instance list: %#v", data)
	}
	inst, _ := data["instances"].([]any)[0].(map[string]any)
	auth, _ := inst["auth"].(map[string]any)
	if auth["api_key"] != "***REDACTED***" || auth["username"] != "efp-reader" || inst["account"] != "customer1" {
		t.Fatalf("instance list must redact secrets: %#v", inst)
	}
	assertNoSecrets(t, raw)

	out, raw = runAppD(t, cfg, "", "instance", "get", "prod")
	data = requireOK(t, out)
	if data["base_url"] != mock.Server.URL || data["account"] != "customer1" {
		t.Fatalf("bad instance get: %#v", data)
	}
	assertNoSecrets(t, raw)
	out, _ = runAppD(t, cfg, "", "instance", "get", "nope")
	requireCode(t, out, "not_found")

	requireOK(t, firstOut(runAppD(t, cfg, "", "app", "list")))
	if mock.TokenHits != 1 || mock.LastClientID != "efp-reader@customer1" {
		t.Fatalf("stored api client not used: hits=%d client_id=%q", mock.TokenHits, mock.LastClientID)
	}

	out, _ = runAppD(t, cfg, secret, "instance", "add", "prod", "--base-url", mock.Server.URL, "--account", "customer1", "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, secret, "instance", "add", "dev", "--base-url", mock.Server.URL, "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, secret, "instance", "add", "dev", "--account", "customer1", "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, secret, "instance", "add", "dev", "--base-url", "not a url", "--account", "customer1", "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, "", "instance", "add", "dev", "--base-url", mock.Server.URL, "--account", "customer1", "--username", "efp-reader")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, secret, "instance", "add", "dev", "--base-url", mock.Server.URL, "--account", "customer1", "--auth-type", "bearer_token", "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "update", "prod", "--account", "customer2")))
	if data["updated"] != true || data["account"] != "customer2" {
		t.Fatalf("bad update: %#v", data)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "get", "prod")))
	if data["account"] != "customer2" {
		t.Fatalf("update not persisted: %#v", data)
	}
	out, _ = runAppD(t, cfg, "", "instance", "update", "prod")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, "", "instance", "update", "nope", "--account", "x")
	requireCode(t, out, "not_found")
	requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "update", "prod", "--account", "customer1")))

	data = requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "default")))
	if data["default_instance"] != "prod" {
		t.Fatalf("bad default read: %#v", data)
	}
	out, _ = runAppD(t, cfg, "", "instance", "default", "nope")
	requireCode(t, out, "not_found")
	requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "default", "prod")))

	out, raw = runAppD(t, cfg, "secret-password-should-not-appear\n", "auth", "login", "--auth-type", "basic_password", "--username", "reader", "--password-stdin")
	data = requireOK(t, out)
	if data["logged_in"] != true || data["auth_type"] != "basic_password" {
		t.Fatalf("bad login: %#v", data)
	}
	assertNoSecrets(t, raw)
	loaded, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.AppD.Instances[0].Auth; got.Type != "basic_password" || got.Username != "reader" || got.Password != "secret-password-should-not-appear" || got.APIKey != "" {
		t.Fatalf("stored auth: %+v", got)
	}
	tokenHits := mock.TokenHits
	requireOK(t, firstOut(runAppD(t, cfg, "", "app", "list")))
	if mock.TokenHits != tokenHits || !strings.HasPrefix(mock.LastAuth, "Basic ") {
		t.Fatalf("basic login not used: hits=%d auth=%q", mock.TokenHits, mock.LastAuth)
	}

	out, _ = runAppD(t, cfg, secret, "auth", "login", "--username", "reader", "--password-stdin", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	out, _ = runAppD(t, cfg, "", "auth", "login", "--username", "reader")
	requireCode(t, out, "invalid_args")

	out, _ = runAppD(t, cfg, "", "auth", "logout")
	requireCode(t, out, "invalid_args")
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "--yes", "auth", "logout")))
	if data["logged_out"] != true {
		t.Fatalf("bad logout: %#v", data)
	}
	out, _ = runAppD(t, cfg, "", "app", "list")
	requireCode(t, out, "config_error")

	out, _ = runAppD(t, cfg, "", "instance", "remove", "prod")
	requireCode(t, out, "invalid_args")
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "--yes", "instance", "remove", "prod")))
	if data["removed"] != true {
		t.Fatalf("bad remove: %#v", data)
	}
	data = requireOK(t, firstOut(runAppD(t, cfg, "", "instance", "list")))
	if listLen(t, data, "instances") != 0 || data["default_instance"] != "" {
		t.Fatalf("instance not removed: %#v", data)
	}
	out, _ = runAppD(t, cfg, "", "--yes", "instance", "remove", "prod")
	requireCode(t, out, "not_found")
}

func TestAppDAuthLoginRequiresAccountOrQualifiedName(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	cfg := jsonCfg(t, config.RootConfig{Version: 1, AppD: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{{Name: "local", BaseURL: mock.Server.URL}}}})
	out, _ := runAppD(t, cfg, "secret-api-key-should-not-appear\n", "auth", "login", "--username", "efp-reader", "--api-key-stdin")
	requireCode(t, out, "invalid_args")
	data := requireOK(t, firstOut(runAppD(t, cfg, "secret-api-key-should-not-appear\n", "auth", "login", "--username", "efp-reader@customer1", "--api-key-stdin")))
	if data["auth_type"] != "api_client" {
		t.Fatalf("api key on stdin should infer api_client: %#v", data)
	}
	requireOK(t, firstOut(runAppD(t, cfg, "", "app", "list")))
	if mock.LastClientID != "efp-reader@customer1" {
		t.Fatalf("client_id=%q", mock.LastClientID)
	}
}

func TestAppDSecretsNeverReachStdout(t *testing.T) {
	mock := testutil.NewMockAppD(t)
	apiCfg := writeCfg(t, testutil.AppDConfig(mock.Server.URL))
	basicCfg := writeCfg(t, testutil.AppDBasicConfig(mock.Server.URL))
	badCfg := writeCfg(t, strings.Replace(testutil.AppDConfig(mock.Server.URL), "secret-api-key-should-not-appear", "secret-token-should-not-appear", 1))
	guid := "4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f"
	cases := []struct {
		name string
		cfg  string
		args []string
	}{
		{"api client verbose app list", apiCfg, []string{"--verbose", "app", "list"}},
		{"basic verbose auth test", basicCfg, []string{"--verbose", "auth", "test"}},
		{"api client auth test yaml", apiCfg, []string{"--format", "yaml", "auth", "test"}},
		{"snapshot get with http params", apiCfg, []string{"snapshot", "get", "--app", "ecommerce", "--guid", guid}},
		{"snapshot list with props", basicCfg, []string{"snapshot", "list", "--app", "ecommerce", "--need-props"}},
		{"event details canary", apiCfg, []string{"event", "list", "--app", "ecommerce", "--event-types", "APPLICATION_DEPLOYMENT"}},
		{"oauth failure", badCfg, []string{"app", "list"}},
		{"instance list", apiCfg, []string{"instance", "list"}},
		{"instance get", basicCfg, []string{"instance", "get", "local"}},
		{"dry run", apiCfg, []string{"--dry-run", "metric", "get", "--app", "ecommerce", "--path", "Overall Application Performance|Average Response Time (ms)"}},
		{"off instance api", apiCfg, []string{"api", "get", "https://evil.example/controller/rest/applications"}},
		{"raw api", apiCfg, []string{"api", "get", "/controller/rest/applications/ecommerce/events?time-range-type=BEFORE_NOW&duration-in-mins=60&event-types=APPLICATION_DEPLOYMENT&severities=INFO"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := NewRoot()
			var b bytes.Buffer
			root.SetOut(&b)
			root.SetErr(&b)
			root.SetArgs(append([]string{"--config", tc.cfg, "--json"}, tc.args...))
			_ = root.Execute()
			assertNoSecrets(t, b.String())
			if b.Len() == 0 {
				t.Fatalf("no output")
			}
		})
	}
}

func TestAppDCommandsSchemaAndHelpLLM(t *testing.T) {
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
	if len(commands) < 29 {
		t.Fatalf("too few commands: %d", len(commands))
	}
	names := map[string]map[string]any{}
	for _, raw := range commands {
		m, _ := raw.(map[string]any)
		names[m["name"].(string)] = m
		if m["risk"] != "read" && !strings.HasPrefix(m["name"].(string), "instance.") && !strings.HasPrefix(m["name"].(string), "auth.") {
			t.Fatalf("controller command %s must be read risk: %#v", m["name"], m)
		}
	}
	for _, name := range []string{"app.list", "bt.list", "metric.preset", "snapshot.list", "snapshot.get", "violation.list", "event.list", "api.get", "auth.test"} {
		if _, ok := names[name]; !ok {
			t.Fatalf("missing command %s", name)
		}
	}
	if names["instance.remove"]["risk"] != "delete" || names["auth.logout"]["risk"] != "delete" || names["auth.login"]["risk"] != "write" || names["instance.add"]["risk"] != "write" {
		t.Fatalf("bad config command risks: %#v %#v", names["instance.remove"], names["auth.login"])
	}
	for _, example := range names["snapshot.list"]["examples"].([]any) {
		if strings.Contains(example.(string), "VALUE") {
			t.Fatalf("example has placeholder value: %s", example)
		}
	}

	b.Reset()
	root = NewRoot()
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"schema", "snapshot.list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	obj = testutil.AssertOKEnvelope(t, b.Bytes())
	schema, _ := obj["data"].(map[string]any)
	flagNames := map[string]bool{}
	for _, raw := range schema["flags"].([]any) {
		f, _ := raw.(map[string]any)
		flagNames[f["name"].(string)] = true
		if f["name"] == "app" && f["required"] != true {
			t.Fatalf("--app should be required in schema: %#v", f)
		}
	}
	for _, name := range []string{"app", "duration-mins", "start-time", "end-time", "before-time", "after-time", "errors-only", "user-experience", "max-results", "instance", "json"} {
		if !flagNames[name] {
			t.Fatalf("schema missing flag %s: %#v", name, flagNames)
		}
	}

	b.Reset()
	root = NewRoot()
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs([]string{"help", "llm", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	obj = testutil.AssertOKEnvelope(t, b.Bytes())
	data, _ = obj["data"].(map[string]any)
	var tips []string
	for _, raw := range data["tips"].([]any) {
		tips = append(tips, raw.(string))
	}
	joined := strings.Join(tips, "\n")
	for _, sentence := range []string{"app list", "bt list", "snapshot list --app <app> --errors-only", "violation list", "event list --app <app> --event-types APPLICATION_DEPLOYMENT", "call graph", "read-only", "data.time_range", "error.code and error.hint"} {
		if !strings.Contains(joined, sentence) {
			t.Fatalf("help llm missing %q in %s", sentence, joined)
		}
	}
	if commands, _ := data["commands"].([]any); len(commands) < 29 {
		t.Fatalf("help llm should list every command, got %d", len(commands))
	}
}
