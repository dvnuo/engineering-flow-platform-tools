package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/mobileauto"
	"engineering-flow-platform-tools/internal/output"
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
	if saved["session_create_started_at"] == nil || saved["remote_build_detected_at"] == nil || saved["remote_session_detected_at"] == nil || saved["running_at"] == nil {
		t.Fatalf("run progress timestamps missing: %#v", saved)
	}
	progress, _ := saved["progress_message"].(string)
	if saved["recovered_session"] != true || !strings.Contains(progress, "enrich completed") {
		t.Fatalf("run progress fields missing: %#v", saved)
	}
	if saved["appium_logs_url"] != "https://logs.example/appium" || saved["device_logs_url"] != "https://logs.example/device" || saved["video_url"] != "https://logs.example/video" {
		t.Fatalf("run links not enriched: %#v", saved)
	}
}

func TestRunStartRecoversServerErrorFromControlPlane(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/devices.json":
			_, _ = w.Write([]byte(`[{"os":"android","os_version":"14.0","device":"Pixel 8","realMobile":true}]`))
		case "/session":
			http.Error(w, "gateway timed out after BrowserStack accepted the session", http.StatusBadGateway)
		case "/app-automate/builds.json":
			_, _ = w.Write([]byte(`[{"automation_build":{"name":"build-recover","hashed_id":"build-recover-id"}}]`))
		case "/app-automate/builds/build-recover-id/sessions.json":
			_, _ = w.Write([]byte(`[{"automation_session":{"name":"session-recover","hashed_id":"session-recover-id","build_name":"build-recover","os":"android","device":"Pixel 8","browser_url":"https://dashboard.example/session-recover"}}]`))
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
	out := runMobile(t, "run", "start", "--config", path, "--app", "bs://app", "--platform", "android", "--device", "Pixel 8", "--build", "build-recover", "--name", "session-recover", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["recovery_attempted"] != true || data["recovered_session"] != true || data["state_persisted"] != true {
		t.Fatalf("expected recovered persisted session: %#v", data)
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
	if saved["status"] != "running" || saved["session_id"] != "session-recover-id" || saved["build_id"] != "build-recover-id" {
		t.Fatalf("recovered state was not running: %#v", saved)
	}
	if saved["remote_build_detected_at"] == nil || saved["remote_session_detected_at"] == nil || saved["running_at"] == nil || saved["recovered_session"] != true {
		t.Fatalf("recovered progress fields missing: %#v", saved)
	}
}

func TestRunStartFailureSettlesStartingState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/devices.json":
			_, _ = w.Write([]byte(`[{"os":"android","os_version":"14.0","device":"Pixel 8","realMobile":true}]`))
		case "/session":
			http.Error(w, "invalid desired capabilities", http.StatusBadRequest)
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
	out := runMobile(t, "run", "start", "--config", path, "--app", "bs://app", "--platform", "android", "--device", "Pixel 8", "--build", "bad-build", "--name", "bad-session", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	states := readRunStates(t, stateDir)
	if len(states) != 1 {
		t.Fatalf("expected one run state, got %d: %#v", len(states), states)
	}
	st := states[0]
	if st.Status != mobileauto.StatusFailed || st.FinishedAt == nil || st.LastErrorCode == "" || st.LastErrorMessage == "" {
		t.Fatalf("run did not settle failed with diagnostics: %#v", st)
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
	if saved["session_create_started_at"] == nil || saved["remote_session_detected_at"] == nil || saved["running_at"] == nil {
		t.Fatalf("minimal progress fields missing: %#v", saved)
	}
}

func TestRunStatusRecoversStaleStartingRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/builds.json":
			_, _ = w.Write([]byte(`[{"automation_build":{"name":"build-stale","hashed_id":"build-stale-id"}}]`))
		case "/app-automate/builds/build-stale-id/sessions.json":
			_, _ = w.Write([]byte(`[{"automation_session":{"name":"session-stale","hashed_id":"session-stale-id","build_name":"build-stale","os":"android","device":"Pixel 8","browser_url":"https://dashboard.example/session-stale"}}]`))
		case "/app-automate/sessions/session-stale-id.json":
			_, _ = w.Write([]byte(`{"automation_session":{"name":"session-stale","hashed_id":"session-stale-id","build_name":"build-stale","os":"android","device":"Pixel 8","status":"running","browser_url":"https://dashboard.example/session-stale"}}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-20 * time.Minute)
	if err := store.SaveRun(mobileauto.RunState{
		RunID:                  "run-stale",
		Provider:               "browserstack",
		Status:                 mobileauto.StatusStarting,
		ControlOwner:           "agent",
		Platform:               "android",
		Device:                 mobileauto.DeviceSelection{Name: "Pixel 8", OS: "android"},
		Network:                mobileauto.NetworkState{Mode: "public", LocalMode: "none"},
		BuildName:              "build-stale",
		SessionName:            "session-stale",
		SessionCreateStartedAt: &started,
		StartedAt:              started,
	}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "run", "status", "--config", path, "--run-id", "run-stale", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	st, err := store.LoadRun("run-stale")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != mobileauto.StatusRunning || st.SessionID != "session-stale-id" || st.BuildID != "build-stale-id" || !st.RecoveredSession {
		t.Fatalf("stale starting was not recovered: %#v", st)
	}
	if st.RemoteBuildDetectedAt == nil || st.RemoteSessionDetectedAt == nil || st.RunningAt == nil {
		t.Fatalf("recovered starting progress fields missing: %#v", st)
	}
}

func TestRunStatusSettlesStaleStartingWithoutRemoteClues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.AppiumBaseURL = "http://127.0.0.1:1"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-20 * time.Minute)
	if err := store.SaveRun(mobileauto.RunState{
		RunID:                  "run-stale-empty",
		Provider:               "browserstack",
		Status:                 mobileauto.StatusStarting,
		ControlOwner:           "agent",
		Network:                mobileauto.NetworkState{Mode: "public", LocalMode: "none"},
		SessionCreateStartedAt: &started,
		StartedAt:              started,
	}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "run", "status", "--config", path, "--run-id", "run-stale-empty", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	st, err := store.LoadRun("run-stale-empty")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != mobileauto.StatusFailed || st.FinishedAt == nil || st.LastErrorCode != "run_start_stale" {
		t.Fatalf("stale starting was not settled: %#v", st)
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

func TestRunImportExistingBrowserStackSessionProbesAndSaves(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/sessions/session-import.json":
			_, _ = w.Write([]byte(`{"automation_session":{"name":"session-import","hashed_id":"session-import","build_name":"build-import","project_name":"proj","os":"android","os_version":"14.0","device":"Pixel 8","status":"running","browser_url":"https://dashboard.example/session-import","appium_logs_url":"https://logs.example/appium","app_details":{"app_url":"bs://app-import","app_name":"Demo"}}}`))
		case "/session/session-import":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"session-import","capabilities":{"platformName":"Android"}}}`))
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
	out := runMobile(t, "run", "import", "--config", path, "--session-id", "session-import", "--build-id", "build-import-id", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["imported"] != true {
		t.Fatalf("expected imported=true: %#v", data)
	}
	run := data["run"].(map[string]any)
	runID := run["run_id"].(string)
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	st, err := store.LoadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if st.SessionID != "session-import" || st.BuildID != "build-import-id" || st.Status != mobileauto.StatusRunning {
		t.Fatalf("unexpected imported run: %#v", st)
	}
	if !st.ImportedSession || st.RecoveredSession || st.ImportProbe != "passed" || st.ImportedAt == nil {
		t.Fatalf("import markers were not saved correctly: %#v", st)
	}
	if st.App.AppURL != "bs://app-import" || st.DashboardURL == "" || st.AppiumLogsURL == "" {
		t.Fatalf("remote metadata was not copied: %#v", st)
	}
}

func TestRunImportRejectsUncontrollableSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/sessions/session-dead.json":
			_, _ = w.Write([]byte(`{"automation_session":{"name":"session-dead","hashed_id":"session-dead","build_name":"build-dead","os":"android","device":"Pixel 8","status":"running"}}`))
		case "/session/session-dead":
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
	out := runMobile(t, "run", "import", "--config", path, "--session-id", "session-dead", "--build-id", "build-dead-id", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "session_not_controllable" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed import should not write runs: %#v", entries)
	}
}

func TestRunImportDryRunDoesNotPersist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/sessions/session-dry.json":
			_, _ = w.Write([]byte(`{"automation_session":{"name":"session-dry","hashed_id":"session-dry","build_name":"build-dry","os":"android","device":"Pixel 8","status":"running"}}`))
		case "/session/session-dry":
			_, _ = w.Write([]byte(`{"value":{"sessionId":"session-dry","capabilities":{}}}`))
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
	out := runMobile(t, "run", "import", "--config", path, "--session-id", "session-dry", "--build-id", "build-dry-id", "--dry-run", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["imported"] != false || data["dry_run"] != true {
		t.Fatalf("expected dry-run import response: %#v", data)
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry-run should not write runs: %#v", entries)
	}
}

func TestSessionSearchFiltersRunningSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app-automate/builds.json":
			if r.URL.Query().Get("status") != "running" || r.URL.Query().Get("projectId") != "proj" {
				t.Fatalf("unexpected build query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[
				{"automation_build":{"name":"alpha","hashed_id":"build-alpha"}},
				{"automation_build":{"name":"beta","hashed_id":"build-beta"}}
			]`))
		case "/app-automate/builds/build-alpha/sessions.json":
			if r.URL.Query().Get("status") != "running" {
				t.Fatalf("unexpected session query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[
				{"automation_session":{"name":"target","hashed_id":"session-target","build_name":"alpha","os":"android","device":"Pixel 8","status":"running"}},
				{"automation_session":{"name":"other","hashed_id":"session-other","build_name":"alpha","os":"android","device":"Pixel 8","status":"running"}}
			]`))
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
	out := runMobile(t, "session", "search", "--config", path, "--project-id", "proj", "--build", "alpha", "--name", "target", "--platform", "android", "--status", "running", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["count"] != float64(1) || data["auth_scope"] != "current_credentials" {
		t.Fatalf("unexpected search data: %#v", data)
	}
	sessions := data["sessions"].([]any)
	got := sessions[0].(map[string]any)
	if got["build_id"] != "build-alpha" || got["build_name"] != "alpha" {
		t.Fatalf("unexpected search result: %#v", got)
	}
	session := got["session"].(map[string]any)
	if session["hashed_id"] != "session-target" {
		t.Fatalf("unexpected session result: %#v", session)
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-lost", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-lost", StartedAt: mobileTestNow()}); err != nil {
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
	if st.Status != mobileauto.StatusLost || st.FinishedAt == nil {
		t.Fatalf("run was not marked lost: %#v", st)
	}
}

func TestSwipeUsesViewportRelativeCoordinates(t *testing.T) {
	actions := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-gesture/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":1080,"height":2400}}`))
		case "/session/session-gesture/actions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			actions <- body
			_, _ = w.Write([]byte(`{"value":null}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-gesture", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-gesture", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "swipe", "--config", path, "--run-id", "run-gesture", "--direction", "up", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	body := <-actions
	steps := body["actions"].([]any)[0].(map[string]any)["actions"].([]any)
	start := steps[0].(map[string]any)
	move := steps[2].(map[string]any)
	if start["x"] != float64(540) || start["y"] != float64(1920) || move["x"] != float64(540) || move["y"] != float64(480) {
		t.Fatalf("unexpected viewport-relative swipe actions: %#v", steps)
	}
}

func TestSwipePercentFractionsMeanPercent(t *testing.T) {
	startX, startY, endX, endY, err := swipePercents(swipeCommandOptions{
		StartXPercent: 0.5,
		StartYPercent: 0.8,
		EndXPercent:   0.5,
		EndYPercent:   0.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if startX != 50 || startY != 80 || endX != 50 || endY != 20 {
		t.Fatalf("fractional percentages were not normalized: %v %v %v %v", startX, startY, endX, endY)
	}
}

func TestSwipeContainerRefUsesObservationBounds(t *testing.T) {
	actions := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-container/actions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			actions <- body
			_, _ = w.Write([]byte(`{"value":null}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path, stateDir, artifactsDir := writeMobileMockConfig(t, srv.URL)
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	obs, err := mobileauto.BuildObservationStrict("run-container", "session-container", "obs-container", `<hierarchy><node class="android.widget.ScrollView" scrollable="true" bounds="[100,200][900,1200]" /></hierarchy>`, []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveObservation("run-container", obs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-container", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-container", Platform: "android", LatestObservationID: obs.ID, StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "swipe", "--config", path, "--run-id", "run-container", "--container-ref", "obs-container:e1", "--direction", "up", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	body := <-actions
	steps := body["actions"].([]any)[0].(map[string]any)["actions"].([]any)
	start := steps[0].(map[string]any)
	move := steps[2].(map[string]any)
	if start["x"] != float64(500) || start["y"] != float64(1000) || move["x"] != float64(500) || move["y"] != float64(400) {
		t.Fatalf("unexpected container-relative swipe actions: %#v", steps)
	}
}

func TestScrollToFindsElementAfterViewportScroll(t *testing.T) {
	actions := make(chan map[string]any, 4)
	var sourceCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-scroll/source":
			if atomic.AddInt32(&sourceCalls, 1) == 1 {
				_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.TextView\" text=\"Home\" bounds=\"[0,0][100,100]\" /></hierarchy>"}`))
				return
			}
			_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.Button\" text=\"Checkout\" clickable=\"true\" bounds=\"[10,10][200,90]\" /></hierarchy>"}`))
		case "/session/session-scroll/screenshot":
			_, _ = w.Write([]byte(`{"value":"c2NyZWVu"}`))
		case "/session/session-scroll/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":1000,"height":2000}}`))
		case "/session/session-scroll/actions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			actions <- body
			_, _ = w.Write([]byte(`{"value":null}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-scroll", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-scroll", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "scroll-to", "--config", path, "--run-id", "run-scroll", "--text", "Checkout", "--max-scrolls", "2", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["found"] != true || data["scrolls"] != float64(1) || data["recommended_ref"] == "" {
		t.Fatalf("unexpected scroll-to result: %#v", data)
	}
	<-actions
	if got := atomic.LoadInt32(&sourceCalls); got != 2 {
		t.Fatalf("expected two observations, got %d", got)
	}
}

func TestScrollToEdgeBottomStopsOnRepeatedSource(t *testing.T) {
	actions := make(chan map[string]any, 2)
	var sourceCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-edge/source":
			atomic.AddInt32(&sourceCalls, 1)
			_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.TextView\" text=\"Terms\" bounds=\"[0,0][100,100]\" /></hierarchy>"}`))
		case "/session/session-edge/screenshot":
			_, _ = w.Write([]byte(`{"value":"c2NyZWVu"}`))
		case "/session/session-edge/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":1000,"height":2000}}`))
		case "/session/session-edge/actions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			actions <- body
			_, _ = w.Write([]byte(`{"value":null}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path, stateDir, artifactsDir := writeMobileMockConfig(t, srv.URL)
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-edge", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-edge", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "scroll-to", "--config", path, "--run-id", "run-edge", "--edge", "bottom", "--max-scrolls", "3", "--stable-count", "2", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["scrolls"] != float64(1) || data["stopped_reason"] != "repeated_source" || data["repeated_source"] != true {
		t.Fatalf("unexpected edge result: %#v", data)
	}
	<-actions
	if got := atomic.LoadInt32(&sourceCalls); got != 2 {
		t.Fatalf("expected two observations, got %d", got)
	}
}

func TestSwipeUntilStableMaxSwipesReturnsStructuredFailure(t *testing.T) {
	var sourceCalls int32
	var actionCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-stable/source":
			call := atomic.AddInt32(&sourceCalls, 1)
			text := "Page A"
			if call == 2 {
				text = "Page B"
			}
			if call >= 3 {
				text = "Page C"
			}
			_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.TextView\" text=\"` + text + `\" bounds=\"[0,0][100,100]\" /></hierarchy>"}`))
		case "/session/session-stable/screenshot":
			_, _ = w.Write([]byte(`{"value":"c2NyZWVu"}`))
		case "/session/session-stable/window/rect":
			_, _ = w.Write([]byte(`{"value":{"x":0,"y":0,"width":1000,"height":2000}}`))
		case "/session/session-stable/actions":
			atomic.AddInt32(&actionCalls, 1)
			_, _ = w.Write([]byte(`{"value":null}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	path, stateDir, artifactsDir := writeMobileMockConfig(t, srv.URL)
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-stable", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-stable", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "swipe", "--config", path, "--run-id", "run-stable", "--until-stable", "--max-swipes", "2", "--json")
	if out["ok"] != false {
		t.Fatalf("expected failure: %#v", out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "scroll_still_changing" || errObj["recommended_action"] != "observe" {
		t.Fatalf("unexpected error: %#v", errObj)
	}
	data := out["data"].(map[string]any)
	if data["scrolls"] != float64(2) || data["stopped_reason"] != "max_scrolls" || data["last_observation_id"] == "" {
		t.Fatalf("unexpected scroll data: %#v", data)
	}
	if got := atomic.LoadInt32(&actionCalls); got != 2 {
		t.Fatalf("expected two swipes, got %d", got)
	}
}

func TestTapWaitChangeCapturesPostObservation(t *testing.T) {
	var clicked int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-post/elements":
			_, _ = w.Write([]byte(`{"value":[{"element-6066-11e4-a52e-4f735466cecf":"element-login"}]}`))
		case "/session/session-post/element/element-login/click":
			atomic.AddInt32(&clicked, 1)
			_, _ = w.Write([]byte(`{"value":null}`))
		case "/session/session-post/source":
			_, _ = w.Write([]byte(`{"value":"<hierarchy><node class=\"android.widget.TextView\" text=\"Home\" bounds=\"[0,0][100,100]\" /></hierarchy>"}`))
		case "/session/session-post/screenshot":
			_, _ = w.Write([]byte(`{"value":"c2NyZWVu"}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	oldObs, err := mobileauto.BuildObservationStrict("run-post", "session-post", "obs-old", `<hierarchy><node class="android.widget.Button" text="Login" content-desc="Login" clickable="true" bounds="[0,0][100,100]" /></hierarchy>`, []byte("old"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveObservation("run-post", oldObs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-post", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-post", Platform: "android", LatestObservationID: oldObs.ID, ObservationVersion: 1, StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "tap", "--config", path, "--run-id", "run-post", "--ref", "obs-old:e1", "--wait-change", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	if data["wait_change_satisfied"] != true || data["post_observe"] == nil || data["observation_invalidated"] != false {
		t.Fatalf("post action observation missing: %#v", data)
	}
	if atomic.LoadInt32(&clicked) != 1 {
		t.Fatalf("expected one click")
	}
	events, err := store.LoadTimeline("run-post")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatalf("expected timeline events")
	}
}

func TestKeyboardKeycodePostsAndroidKeycode(t *testing.T) {
	keycodes := make(chan float64, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-key/appium/device/press_keycode":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			keycodes <- body["keycode"].(float64)
			_, _ = w.Write([]byte(`{"value":null}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-key", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-key", Platform: "android", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "keyboard", "keycode", "--config", path, "--run-id", "run-key", "--keycode", "66", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	if got := <-keycodes; got != 66 {
		t.Fatalf("keycode=%v", got)
	}
}

func TestWorkflowRunDryRunRedactsText(t *testing.T) {
	workflowPath := filepath.Join(t.TempDir(), "flow.yaml")
	if err := os.WriteFile(workflowPath, []byte(`name: smoke
steps:
  - action: observe
    run_id: run-1
  - action: type
    run_id: run-1
    ref: obs-1:e2
    text: super-secret
`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "workflow", "run", "--file", workflowPath, "--dry-run", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	data := out["data"].(map[string]any)
	steps := data["steps"].([]any)
	typed := steps[1].([]any)
	for _, part := range typed {
		if part == "super-secret" {
			t.Fatalf("dry-run leaked text: %#v", typed)
		}
	}
}

func TestWorkflowAssertCountOmitsMissingExpected(t *testing.T) {
	args, err := workflowStepArgs(workflowStep{Action: "assert.count", RunID: "run-1", Role: "button"})
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range args {
		if arg == "--expected" {
			t.Fatalf("missing expected should not emit --expected: %#v", args)
		}
	}
}

func TestInspectorConfigFromRunRedactsAccessKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.AppiumBaseURL = "http://127.0.0.1:4723/wd/hub"
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "secret-key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{
		RunID:       "run-inspector",
		Provider:    "browserstack",
		Status:      mobileauto.StatusRunning,
		SessionID:   "session-inspector",
		Platform:    "android",
		Device:      mobileauto.DeviceSelection{Name: "Pixel 8", OSVersion: "14.0"},
		App:         mobileauto.AppRef{AppURL: "bs://app"},
		Network:     mobileauto.NetworkState{Mode: "private-external", LocalIdentifier: "local-1"},
		BuildName:   "build-1",
		SessionName: "session-1",
		StartedAt:   mobileTestNow(),
	}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "inspector", "config", "--config", path, "--run-id", "run-inspector", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	b := mustJSON(t, out)
	if strings.Contains(string(b), "secret-key") {
		t.Fatalf("access key leaked: %s", string(b))
	}
	data := out["data"].(map[string]any)
	caps := data["capabilities"].(map[string]any)
	if caps["platformName"] != "Android" || caps["appium:app"] != "bs://app" {
		t.Fatalf("unexpected caps: %#v", caps)
	}
	bstack := caps["bstack:options"].(map[string]any)
	if bstack["localIdentifier"] != "local-1" || bstack["accessKey"] != output.Redacted {
		t.Fatalf("unexpected bstack options: %#v", bstack)
	}
}

func TestInspectorConfigSecretModeEnvExplainsCredentialVariables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = filepath.Join(t.TempDir(), "state")
	cfg.Mobile.ArtifactsDir = filepath.Join(t.TempDir(), "artifacts")
	cfg.Mobile.BrowserStack.APIBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.AppiumBaseURL = "http://127.0.0.1:4723/wd/hub"
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "secret-key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "inspector", "config", "--config", path, "--app", "bs://app", "--platform", "android", "--secret-mode", "env", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	if strings.Contains(string(mustJSON(t, out)), "secret-key") {
		t.Fatalf("access key leaked: %s", string(mustJSON(t, out)))
	}
	data := out["data"].(map[string]any)
	auth := data["auth"].(map[string]any)
	if auth["mode"] != "env" {
		t.Fatalf("unexpected auth mode: %#v", auth)
	}
	envVars := auth["env_vars"].(map[string]any)
	if envVars["key"] != "BROWSERSTACK_ACCESS_KEY" || data["username"] != "${BROWSERSTACK_USERNAME}" {
		t.Fatalf("unexpected env credential hints: %#v data=%#v", envVars, data)
	}
}

func TestInspectorAttachWarningsMarkRunLost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-lost/contexts", "/session/session-lost/context":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"value":{"error":"invalid session id","message":"Session not started or terminated"}}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-lost", Provider: "browserstack", Status: mobileauto.StatusRunning, SessionID: "session-lost", Platform: "android", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "inspector", "attach", "--config", path, "--run-id", "run-lost", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success with warnings: %#v", out)
	}
	data := out["data"].(map[string]any)
	warnings := data["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatalf("expected warnings: %#v", data)
	}
	st, err := store.LoadRun("run-lost")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != mobileauto.StatusLost || st.FinishedAt == nil {
		t.Fatalf("run was not marked lost: %#v", st)
	}
}

func TestAppLaunchPostsAppiumLifecycleEndpoint(t *testing.T) {
	called := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-app/appium/app/launch":
			called <- r.URL.Path
			_, _ = w.Write([]byte(`{"value":null}`))
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-app", Provider: "browserstack", Status: mobileauto.StatusRunning, ControlOwner: "agent", SessionID: "session-app", Platform: "android", StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "app", "launch", "--config", path, "--run-id", "run-app", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	if got := <-called; got != "/session/session-app/appium/app/launch" {
		t.Fatalf("path=%s", got)
	}
}

func TestObservationAssertionsNotExistsAndCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = "http://127.0.0.1:1"
	cfg.Mobile.BrowserStack.AppiumBaseURL = "http://127.0.0.1:1"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	obs, err := mobileauto.BuildObservationStrict("run-assert", "session-assert", "obs-assert", `<hierarchy><node class="android.widget.Button" text="Login" clickable="true" bounds="[0,0][100,100]" /></hierarchy>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveObservation("run-assert", obs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-assert", Provider: "browserstack", Status: mobileauto.StatusRunning, LatestObservationID: obs.ID, StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "assert", "not-exists", "--config", path, "--run-id", "run-assert", "--text", "Missing", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	out = runMobile(t, "assert", "count", "--config", path, "--run-id", "run-assert", "--role", "button", "--expected", "1", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
}

func TestMobileTestRunDryRunRedactsText(t *testing.T) {
	suitePath := filepath.Join(t.TempDir(), "suite.yaml")
	if err := os.WriteFile(suitePath, []byte(`name: smoke
variables:
  run: run-1
cases:
  - name: login
    tags: [smoke]
    steps:
      - action: observe
        run_id: "{{run}}"
      - action: type
        run_id: "{{run}}"
        ref: obs-1:e2
        text: super-secret
`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runMobile(t, "test", "run", "--file", suitePath, "--tag", "smoke", "--dry-run", "--json")
	if out["ok"] != true {
		t.Fatalf("expected success: %#v", out)
	}
	b := mustJSON(t, out)
	if strings.Contains(string(b), "super-secret") {
		t.Fatalf("dry-run leaked text: %s", string(b))
	}
}

func TestMobileTestRunUsesSecretsEnvAsVariableNames(t *testing.T) {
	suite := mobileTestSuite{
		SecretsEnv: map[string]string{"password_env": "MOBILE_TEST_PASSWORD"},
		Cases: []mobileTestCase{{
			Name:  "login",
			Steps: []workflowStep{{Action: "type", RunID: "run-1", TextEnv: "{{password_env}}"}},
		}},
	}
	executions := expandMobileTestCases(suite, "", nil)
	if len(executions) != 1 {
		t.Fatalf("expected execution: %#v", executions)
	}
	if executions[0].Variables["password_env"] != "MOBILE_TEST_PASSWORD" {
		t.Fatalf("secret env variable name was not merged: %#v", executions[0].Variables)
	}
	step := substituteWorkflowStep(executions[0].Case.Steps[0], executions[0].Variables)
	if step.TextEnv != "MOBILE_TEST_PASSWORD" {
		t.Fatalf("unexpected text env substitution: %#v", step)
	}
}

func TestMobileTestRunExecutesAfterAndWritesEvidenceOnFailure(t *testing.T) {
	var closed int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/session/session-after/appium/app/close":
			atomic.AddInt32(&closed, 1)
			_, _ = w.Write([]byte(`{"value":null}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	stateDir := filepath.Join(tmp, "state")
	artifactsDir := filepath.Join(tmp, "artifacts")
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
	store := mobileauto.NewStateStore(stateDir, artifactsDir)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	obs, err := mobileauto.BuildObservationStrict("run-after", "session-after", "obs-after", `<hierarchy><node class="android.widget.Button" text="Login" clickable="true" bounds="[0,0][100,100]" /></hierarchy>`, []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	obsDir := filepath.Join(store.ObservationDir("run-after"), obs.ID)
	obs.SourcePath = filepath.Join(obsDir, "source.xml")
	obs.ScreenshotPath = filepath.Join(obsDir, "screenshot.png")
	obs.CandidatesPath = filepath.Join(obsDir, "candidates.json")
	if err := store.SaveObservation("run-after", obs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(mobileauto.RunState{RunID: "run-after", Provider: "browserstack", Status: mobileauto.StatusRunning, SessionID: "session-after", Platform: "android", LatestObservationID: obs.ID, StartedAt: mobileTestNow()}); err != nil {
		t.Fatal(err)
	}
	suitePath := filepath.Join(tmp, "suite.yaml")
	if err := os.WriteFile(suitePath, []byte(`name: cleanup
after:
  - action: app.close
    run_id: run-after
cases:
  - name: cleanup
    steps:
      - action: assert.not-exists
        run_id: run-after
        text: Login
`), 0o600); err != nil {
		t.Fatal(err)
	}
	evidenceDir := filepath.Join(tmp, "evidence")
	out := runMobile(t, "test", "run", "--config", path, "--file", suitePath, "--evidence-dir", evidenceDir, "--json")
	if out["ok"] != false {
		t.Fatalf("expected failed test result: %#v", out)
	}
	if atomic.LoadInt32(&closed) != 1 {
		t.Fatalf("expected after cleanup to close app")
	}
	data := out["data"].(map[string]any)
	results := data["results"].([]any)
	result := results[0].(map[string]any)
	evidencePath := result["evidence_path"].(string)
	for _, name := range []string{"failure.json", "run-report.json", "source.xml", "screenshot.png", "candidates.json"} {
		if _, err := os.Stat(filepath.Join(evidencePath, name)); err != nil {
			t.Fatalf("missing evidence %s: %v", name, err)
		}
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

func writeMobileMockConfig(t *testing.T, baseURL string) (string, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	stateDir := filepath.Join(t.TempDir(), "state")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := config.RootConfig{}
	cfg.Normalize()
	cfg.Mobile.StateDir = stateDir
	cfg.Mobile.ArtifactsDir = artifactsDir
	cfg.Mobile.BrowserStack.APIBaseURL = baseURL
	cfg.Mobile.BrowserStack.AppiumBaseURL = baseURL
	cfg.Mobile.BrowserStack.Username = "user"
	cfg.Mobile.BrowserStack.AccessKey = "key"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path, stateDir, artifactsDir
}

func readRunStates(t *testing.T, stateDir string) []mobileauto.RunState {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(stateDir, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	store := mobileauto.NewStateStore(stateDir, "")
	out := make([]mobileauto.RunState, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		st, err := store.LoadRun(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, st)
	}
	return out
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
