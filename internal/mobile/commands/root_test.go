package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/mobile"
)

func TestCommandsJSONIncludesRunStart(t *testing.T) {
	out := runMobile(t, "commands", "--json")
	data := out["data"].(map[string]any)
	commands := data["commands"].([]any)
	for _, raw := range commands {
		item := raw.(map[string]any)
		if item["name"] == "run.start" {
			return
		}
	}
	t.Fatalf("run.start not found in commands: %#v", commands)
}

func TestAuthLoginStoresBrowserStackCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	out := runMobileWithInput(t, "secret-key\n", "auth", "login", "--config", path, "--username", "bs-user", "--access-key-stdin", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	if strings.Contains(string(mustJSON(t, out)), "secret-key") {
		t.Fatalf("access key leaked in output: %#v", out)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mobile.BrowserStack.Username != "bs-user" || cfg.Mobile.BrowserStack.AccessKey != "secret-key" {
		t.Fatalf("credentials not saved: %#v", cfg.Mobile.BrowserStack)
	}
}

func TestAuthLogoutRequiresConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.BrowserStack.Username = "bs-user"
	cfg.Mobile.BrowserStack.AccessKey = "secret-key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "auth", "logout", "--config", path, "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	out = runMobile(t, "auth", "logout", "--config", path, "--yes", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-key") {
		t.Fatalf("access key was not removed: %s", string(b))
	}
}

func TestDoctorReportsEffectiveHTTPProxy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	forceProxy := true
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.BrowserStack.APIBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.AppiumBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.HTTPProxy.ProxyHost = "proxy.internal"
	cfg.Mobile.BrowserStack.HTTPProxy.ProxyPort = 8080
	cfg.Mobile.BrowserStack.HTTPProxy.ForceProxy = &forceProxy
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "doctor", "--config", path, "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["browserstack_api_proxy_source"] != "config" || data["browserstack_api_proxy_host"] != "proxy.internal" || data["browserstack_api_proxy_port"] != "8080" {
		t.Fatalf("unexpected api proxy fields: %#v", data)
	}
	if data["appium_proxy_source"] != "config" || data["appium_proxy_host"] != "proxy.internal" || data["appium_proxy_port"] != "8080" {
		t.Fatalf("unexpected appium proxy fields: %#v", data)
	}
}

func TestSchemaRunStartIncludesCoreFlags(t *testing.T) {
	out := runMobile(t, "schema", "run.start", "--json")
	data := out["data"].(map[string]any)
	flags := data["flags"].([]any)
	have := map[string]bool{}
	for _, raw := range flags {
		flag := raw.(map[string]any)
		have[flag["name"].(string)] = true
	}
	for _, want := range []string{"app", "file", "platform", "device", "network", "wait-capacity", "json"} {
		if !have[want] {
			t.Fatalf("missing --%s in %#v", want, flags)
		}
	}
}

func TestInvalidArgsReturnsJSON(t *testing.T) {
	out := runMobile(t, "app", "upload", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--file") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestTypeMissingTextEnvFailsBeforeServiceSetup(t *testing.T) {
	t.Setenv("MOBILE_TEST_SECRET_MISSING", "")
	out := runMobile(t, "type", "--run-id", "run-1", "--ref", "obs-1:e1", "--text-env", "MOBILE_TEXT_ENV_DOES_NOT_EXIST", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "MOBILE_TEXT_ENV_DOES_NOT_EXIST") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestRunStartInvalidTimeoutFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "run", "start", "--timeout", "soon", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--timeout") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestRunStartRecoversMissingSessionIDFromControlPlane(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/devices.json":
			_, _ = w.Write([]byte(`[{"os":"android","os_version":"14.0","device":"Pixel 8","realMobile":true}]`))
		case "/session":
			_, _ = w.Write([]byte(`{"value":{"message":"BrowserStack created the session but did not include a sessionId"}}`))
		case "/app-automate/builds.json":
			_, _ = w.Write([]byte(`[{"automation_build":{"name":"build-1","hashed_id":"build-123"}}]`))
		case "/app-automate/builds/build-123/sessions.json":
			_, _ = w.Write([]byte(`[{"automation_session":{"name":"session-1","hashed_id":"session-123","build_name":"build-1","os":"android","device":"Pixel 8","browser_url":"https://dashboard.example/session-123","appium_logs_url":"https://logs.example/appium","device_logs_url":"https://logs.example/device","video_url":"https://logs.example/video"}}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = srv.URL
	cfg.Mobile.BrowserStack.AppiumBaseURL = srv.URL
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "run", "start", "--config", path, "--app", "bs://app", "--platform", "android", "--device", "Pixel 8", "--build", "build-1", "--name", "session-1", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["recovered_session"] != true {
		t.Fatalf("expected recovered session: %#v", data)
	}
	run := data["run"].(map[string]any)
	runID := run["run_id"].(string)
	b, err := os.ReadFile(filepath.Join(stateDir, "runs", runID, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["session_id"] != "session-123" || saved["browserstack_session_id"] != "session-123" || saved["build_id"] != "build-123" {
		t.Fatalf("run ids not enriched: %#v", saved)
	}
	if saved["appium_logs_url"] != "https://logs.example/appium" || saved["device_logs_url"] != "https://logs.example/device" || saved["video_url"] != "https://logs.example/video" {
		t.Fatalf("run links not enriched: %#v", saved)
	}
}

func TestRunStartPersistsMinimalStateWhenEnrichFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/devices.json":
			_, _ = w.Write([]byte(`[{"os":"android","os_version":"14.0","device":"Pixel 8","realMobile":true}]`))
		case "/session":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"session-abc","capabilities":{"browserstack.sessionUrl":"https://dashboard.example/session-abc"}}}`))
		case "/app-automate/sessions/session-abc.json", "/app-automate/builds.json":
			http.Error(w, "temporary control-plane failure", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = srv.URL
	cfg.Mobile.BrowserStack.AppiumBaseURL = srv.URL
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "run", "start", "--config", path, "--app", "bs://app", "--platform", "android", "--device", "Pixel 8", "--build", "build-enrich", "--name", "session-enrich", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	run := data["run"].(map[string]any)
	runID := run["run_id"].(string)
	b, err := os.ReadFile(filepath.Join(stateDir, "runs", runID, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["status"] != "running" || saved["session_id"] != "session-abc" || saved["dashboard_url"] != "https://dashboard.example/session-abc" {
		t.Fatalf("minimal state was not persisted: %#v", saved)
	}
}

func TestRunRecoverAttachesExistingBrowserStackSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/sessions/session-99.json":
			_, _ = w.Write([]byte(`{"automation_session":{"name":"session-99","hashed_id":"session-99","build_name":"build-99","project_name":"proj","os":"android","os_version":"14.0","device":"Pixel 8","status":"running","browser_url":"https://dashboard.example/session-99","appium_logs_url":"https://logs.example/appium"}}`))
		case "/app-automate/builds.json":
			_, _ = w.Write([]byte(`[{"automation_build":{"name":"build-99","hashed_id":"build-99-id"}}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = srv.URL
	cfg.Mobile.BrowserStack.AppiumBaseURL = srv.URL
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "run", "recover", "--config", path, "--session-id", "session-99", "--local-identifier", "local-99", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	run := out["data"].(map[string]any)["run"].(map[string]any)
	runID := run["run_id"].(string)
	b, err := os.ReadFile(filepath.Join(stateDir, "runs", runID, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["session_id"] != "session-99" || saved["status"] != "running" || saved["build_id"] != "build-99-id" {
		t.Fatalf("unexpected recovered run: %#v", saved)
	}
	network := saved["network"].(map[string]any)
	if network["mode"] != "private-external" || network["local_identifier"] != "local-99" {
		t.Fatalf("unexpected recovered network: %#v", network)
	}
}

func TestObserveSessionLostMarksRunLost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-lost/source":
			http.Error(w, "Session not started or terminated", http.StatusNotFound)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = srv.URL
	cfg.Mobile.BrowserStack.AppiumBaseURL = srv.URL
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	store := mobile.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobile.RunState{RunID: "run-lost", Provider: "browserstack", Status: mobile.StatusRunning, ControlOwner: "agent", SessionID: "session-lost", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "observe", "--config", path, "--run-id", "run-lost", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "session_lost" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
	st, err := store.LoadRun("run-lost")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != mobile.StatusLost || st.FinishedAt == nil {
		t.Fatalf("run was not marked lost: %#v", st)
	}
}

func TestRunFinishInvalidStatusFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "run", "finish", "--run-id", "run-1", "--status", "done", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--status") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestTunnelStopMissingMetadataFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "tunnel", "stop", "--yes", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "--run-id") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func TestAppResolveInvalidAppURLFailsBeforeServiceSetup(t *testing.T) {
	out := runMobile(t, "app", "resolve", "--app-url", " app-id-without-prefix ", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "bs://") {
		t.Fatalf("unexpected error: %#v", errObj)
	}
}

func mobileTestNow() time.Time {
	return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
}

func runMobile(t *testing.T, args ...string) map[string]any {
	t.Helper()
	return runMobileWithInput(t, "", args...)
}

func runMobileWithInput(t *testing.T, input string, args ...string) map[string]any {
	t.Helper()
	cmd := NewRoot()
	var b bytes.Buffer
	cmd.SetOut(&b)
	cmd.SetErr(&b)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, b.String())
	}
	return out
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
