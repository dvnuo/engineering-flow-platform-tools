package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func TestZephyrDoctorSuccess(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Write([]byte(`{"baseUrl":"https://jira.example.test"}`))
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/util/zephyrTestIssueType":
			w.Write([]byte(`{"id":"12345","name":"Test"}`))
		case "/rest/zapi/latest/cycle":
			if r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" {
				t.Fatalf("bad doctor cycle query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"cycles":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	out := run(t, cfg, "zephyr", "doctor", "--project", "EFP")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("doctor failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	if data["project_id"] != "10000" || data["api_family"] != "zapi_legacy" || data["base_path"] != "/rest/zapi/latest" {
		t.Fatalf("bad doctor data: %#v", data)
	}
}

func TestZephyrDoctorNotDetected(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Write([]byte(`{"baseUrl":"https://jira.example.test"}`))
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/util/zephyrTestIssueType":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	requireJiraCode(t, run(t, cfg, "zephyr", "doctor", "--project", "EFP"), "zephyr_not_detected")
}

func TestZephyrDisabledBlocksCommandsAndDoctorCanProbe(t *testing.T) {
	cfgPath, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Write([]byte(`{"baseUrl":"https://jira.example.test"}`))
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/util/zephyrTestIssueType":
			w.Write([]byte(`{"id":"12345","name":"Test"}`))
		case "/rest/zapi/latest/cycle":
			w.Write([]byte(`{"cycles":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	var root config.RootConfig
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	disabled := false
	root.Jira.Instances[0].Zephyr.Enabled = &disabled
	if err := config.Save(cfgPath, root); err != nil {
		t.Fatal(err)
	}
	requireJiraCode(t, run(t, cfgPath, "zephyr", "status", "list"), "zephyr_not_enabled")
	if ok, _ := run(t, cfgPath, "zephyr", "doctor", "--project", "EFP", "--enable-probe")["ok"].(bool); !ok {
		t.Fatal("doctor --enable-probe should bypass zephyr.enabled=false")
	}
}

func TestZephyrCycleListAndRawAPIGet(t *testing.T) {
	var sawCycleList bool
	var sawRaw bool
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/cycle":
			if r.Method != http.MethodGet {
				t.Fatalf("bad method: %s", r.Method)
			}
			if r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" {
				t.Fatalf("bad cycle query: %s", r.URL.RawQuery)
			}
			if strings.Contains(r.URL.RawQuery, "raw=1") {
				sawRaw = true
			} else {
				sawCycleList = true
			}
			w.Write([]byte(`{"cycles":[{"id":"1","name":"Regression"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	if ok, _ := run(t, cfg, "zephyr", "cycle", "list", "--project", "EFP", "--version-id", "-1")["ok"].(bool); !ok {
		t.Fatal("cycle list failed")
	}
	if ok, _ := run(t, cfg, "zephyr", "api", "get", "cycle", "--query", "projectId=10000", "--query", "versionId=-1", "--query", "raw=1")["ok"].(bool); !ok {
		t.Fatal("raw api get failed")
	}
	if !sawCycleList || !sawRaw {
		t.Fatalf("missing expected requests cycle=%v raw=%v", sawCycleList, sawRaw)
	}
}

func TestZephyrWriteDryRunsDoNotHitServer(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	if ok, _ := run(t, cfg, "--dry-run", "zephyr", "cycle", "create", "--project", "EFP", "--version-id", "-1", "--name", "Sprint 42 Regression")["ok"].(bool); !ok {
		t.Fatal("cycle create dry-run failed")
	}
	if *hits != before {
		t.Fatal("cycle create dry-run hit server")
	}
	out := run(t, cfg, "--dry-run", "zephyr", "execution", "update-status", "12345", "--status", "PASS")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("execution update-status dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	body := data["body"].(map[string]interface{})
	if body["status"] != "1" {
		t.Fatalf("status body not mapped to legacy id: %#v", data)
	}
	if *hits != before {
		t.Fatal("execution update-status dry-run hit server")
	}
}

func TestZephyrTestListUsesJiraSearch(t *testing.T) {
	var searchBody map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/api/2/search" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&searchBody); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(`{"issues":[{"key":"EFP-T1"}]}`))
	})
	if ok, _ := run(t, cfg, "zephyr", "test", "list", "--project", "EFP")["ok"].(bool); !ok {
		t.Fatal("test list failed")
	}
	if searchBody["jql"] != "project = EFP AND issuetype = Test ORDER BY created DESC" {
		t.Fatalf("bad test list jql: %#v", searchBody)
	}
}
