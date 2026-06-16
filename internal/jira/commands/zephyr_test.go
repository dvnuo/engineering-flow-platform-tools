package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"gopkg.in/yaml.v3"
)

func TestZephyrDoctorSuccess(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/serverInfo":
			w.Write([]byte(`{"baseUrl":"https://jira.example.test"}`))
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/moduleInfo":
			w.Write([]byte(`{"enabled":true}`))
		case "/rest/zapi/latest/systemInfo":
			w.Write([]byte(`{"version":"9.0"}`))
		case "/rest/zapi/latest/license":
			w.Write([]byte(`{"valid":true}`))
		case "/rest/zapi/latest/util/zephyrTestIssueType":
			w.Write([]byte(`{"id":"12345","name":"Test"}`))
		case "/rest/zapi/latest/util/testExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"},{"id":2,"name":"FAIL"}]`))
		case "/rest/zapi/latest/util/teststepExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"},{"id":2,"name":"FAIL"}]`))
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
	if data["module_info"] == nil || len(data["execution_statuses"].([]interface{})) != 2 || len(data["step_statuses"].([]interface{})) != 2 {
		t.Fatalf("doctor did not include official probes: %#v", data)
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
		case "/rest/zapi/latest/moduleInfo":
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
		case "/rest/zapi/latest/moduleInfo":
			w.Write([]byte(`{"enabled":true}`))
		case "/rest/zapi/latest/systemInfo":
			w.Write([]byte(`{"version":"9.0"}`))
		case "/rest/zapi/latest/license":
			w.Write([]byte(`{"valid":true}`))
		case "/rest/zapi/latest/util/zephyrTestIssueType":
			w.Write([]byte(`{"id":"12345","name":"Test"}`))
		case "/rest/zapi/latest/util/testExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"}]`))
		case "/rest/zapi/latest/util/teststepExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"}]`))
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
	if err := yaml.Unmarshal(b, &root); err != nil {
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

func TestZephyrAPIDeleteRequiresYesAndSendsDelete(t *testing.T) {
	var sawDelete bool
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/zapi/latest/execution/123" || r.Method != http.MethodDelete {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("force") != "1" {
			t.Fatalf("missing query: %s", r.URL.RawQuery)
		}
		sawDelete = true
	})
	before := *hits
	requireJiraCode(t, run(t, cfg, "zephyr", "api", "delete", "execution/123"), "invalid_args")
	if *hits != before {
		t.Fatal("api delete without --yes should not hit server")
	}
	out := run(t, cfg, "--yes", "zephyr", "api", "delete", "execution/123", "--query", "force=1")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("api delete failed: %#v", out)
	}
	if !sawDelete {
		t.Fatal("DELETE was not sent")
	}
	data := out["data"].(map[string]interface{})
	if data["deleted"] != true {
		t.Fatalf("delete output not stable: %#v", data)
	}
}

func TestZephyrSummaryDryRunAndProjectResolution(t *testing.T) {
	var requests []string
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/cycle":
			if r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" {
				t.Fatalf("bad summary query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"cycles":[{"id":"1","name":"Regression","totalExecutions":2}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "summary", "--project", "EFP", "--version-id", "-1")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("summary dry-run failed: %#v", out)
	}
	if *hits != before {
		t.Fatal("summary dry-run hit server")
	}
	out = run(t, cfg, "zephyr", "summary", "--project", "EFP", "--version-id", "-1")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("summary failed: %#v", out)
	}
	if strings.Join(requests, ",") != "GET /rest/api/2/project/EFP,GET /rest/zapi/latest/cycle" {
		t.Fatalf("bad request order: %#v", requests)
	}
	data := out["data"].(map[string]interface{})
	if data["project_id"] != "10000" || data["cycle_count"] != float64(1) {
		t.Fatalf("bad summary data: %#v", data)
	}
}

func TestZephyrZQLSearchQuery(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/zapi/latest/zql/executeSearch" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("zqlQuery") != "executionStatus = FAIL" || r.URL.Query().Get("maxRecords") != "100" || r.URL.Query().Get("offset") != "5" {
			t.Fatalf("bad zql query: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"results":[]}`))
	})
	if ok, _ := run(t, cfg, "zephyr", "zql", "search", "--query", "executionStatus = FAIL", "--limit", "100", "--start", "5")["ok"].(bool); !ok {
		t.Fatal("zql search failed")
	}
}

func TestZephyrStepResultUpdateStatusDryRun(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("step-result dry-run should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "step-result", "update-status", "555", "--status", "PASS", "--comment", "ok")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("step-result dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	body := data["body"].(map[string]interface{})
	if body["status"] != "1" || body["comment"] != "ok" {
		t.Fatalf("bad step-result body: %#v", body)
	}
	if *hits != before {
		t.Fatal("step-result dry-run hit server")
	}
}

func TestZephyrAttachmentUploadDryRunAndMultipart(t *testing.T) {
	var sawUpload bool
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/zapi/latest/attachment" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("entityId") != "12345" || r.URL.Query().Get("entityType") != "execution" {
			t.Fatalf("bad attachment query: %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			t.Fatalf("missing no-check header: %#v", r.Header)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if header.Filename != "report.png" {
			t.Fatalf("bad multipart filename: %s", header.Filename)
		}
		b, _ := io.ReadAll(file)
		if string(b) != "png" {
			t.Fatalf("bad multipart content: %q", string(b))
		}
		sawUpload = true
		w.Write([]byte(`{"uploaded":true}`))
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "attachment", "upload", "--entity-type", "execution", "--entity-id", "12345", "--file", filepath.Join(t.TempDir(), "missing.png"))
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("attachment dry-run failed: %#v", out)
	}
	if *hits != before {
		t.Fatal("attachment dry-run hit server")
	}
	filePath := filepath.Join(t.TempDir(), "report.png")
	if err := os.WriteFile(filePath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = run(t, cfg, "zephyr", "attachment", "upload", "--entity-type", "execution", "--entity-id", "12345", "--file", filePath)
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("attachment upload failed: %#v", out)
	}
	if !sawUpload {
		t.Fatal("multipart upload was not sent")
	}
}

func TestZephyrDeletesRequireYesAndDryRunPaths(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("delete validation dry-runs should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	requireJiraCode(t, run(t, cfg, "zephyr", "cycle", "delete", "20000"), "invalid_args")
	requireJiraCode(t, run(t, cfg, "zephyr", "execution", "delete", "30000"), "invalid_args")
	if *hits != before {
		t.Fatal("delete without --yes hit server")
	}
	cycle := run(t, cfg, "--yes", "--dry-run", "zephyr", "cycle", "delete", "20000")
	execution := run(t, cfg, "--yes", "--dry-run", "zephyr", "execution", "delete", "30000")
	for _, item := range []struct {
		out  map[string]interface{}
		path string
	}{
		{cycle, "/rest/zapi/latest/cycle/20000"},
		{execution, "/rest/zapi/latest/execution/30000"},
	} {
		if ok, _ := item.out["ok"].(bool); !ok {
			t.Fatalf("delete dry-run failed: %#v", item.out)
		}
		data := item.out["data"].(map[string]interface{})
		if data["method"] != "DELETE" || data["path"] != item.path {
			t.Fatalf("bad delete dry-run: %#v", data)
		}
	}
	if *hits != before {
		t.Fatal("delete dry-run hit server")
	}
}

func TestZephyrExecutionBulkUpdateStatusDryRun(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("bulk dry-run should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "execution", "bulk-update-status", "--execution-ids", "1,,2, 3", "--status", "PASS", "--comment", "ci")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("bulk dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	updates := data["updates"].([]interface{})
	if len(updates) != 3 {
		t.Fatalf("bad update count: %#v", updates)
	}
	for _, update := range updates {
		item := update.(map[string]interface{})
		body := item["body"].(map[string]interface{})
		if item["method"] != "PUT" || body["status"] != "1" || body["comment"] != "ci" {
			t.Fatalf("bad bulk dry-run item: %#v", item)
		}
	}
	if *hits != before {
		t.Fatal("bulk dry-run hit server")
	}
}

func TestZephyrExecutionListStatusDryRun(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("execution list dry-run should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "execution", "list", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--status", "FAIL")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("execution list dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	query := data["query"].(map[string]interface{})
	if query["status"] != "2" {
		t.Fatalf("status was not mapped into query: %#v", data)
	}
	requireJiraCode(t, run(t, cfg, "--dry-run", "zephyr", "execution", "list", "--cycle-id", "20000", "--project-id", "10000", "--status", "UNKNOWN"), "invalid_zephyr_status")
	if *hits != before {
		t.Fatal("execution list dry-run hit server")
	}
}

func TestZephyrExecutionResolveShapesAndErrors(t *testing.T) {
	t.Run("list issueKey", func(t *testing.T) {
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/issue/EFP-123":
				w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
			case "/rest/zapi/latest/execution":
				if r.URL.Query().Get("cycleId") != "20000" || r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" || r.URL.Query().Get("action") != "expand" {
					t.Fatalf("bad execution query: %s", r.URL.RawQuery)
				}
				w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-123","issueId":"10001","cycleId":"20000","folderId":"40000"}]}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		out := run(t, cfg, "zephyr", "execution", "resolve", "--cycle-id", "20000", "--issue", "EFP-123", "--version-id", "-1")
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("resolve failed: %#v", out)
		}
		data := out["data"].(map[string]interface{})
		if data["execution_id"] != "30000" || data["issue_key"] != "EFP-123" || data["issue_id"] != "10001" || data["folder_id"] != "40000" {
			t.Fatalf("bad resolve data: %#v", data)
		}
	})

	t.Run("map issueId", func(t *testing.T) {
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/issue/EFP-123":
				w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
			case "/rest/zapi/latest/execution":
				w.Write([]byte(`{"30000":{"issueId":"10001","cycleId":"20000","projectId":"10000"}}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		out := run(t, cfg, "zephyr", "execution", "resolve", "--cycle-id", "20000", "--issue", "EFP-123")
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("resolve failed: %#v", out)
		}
		if got := out["data"].(map[string]interface{})["execution_id"]; got != "30000" {
			t.Fatalf("execution_id=%#v", got)
		}
	})

	t.Run("zero and ambiguous", func(t *testing.T) {
		cfgZero, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/issue/EFP-123":
				w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
			case "/rest/zapi/latest/execution":
				w.Write([]byte(`{"executions":[{"id":"30001","issueKey":"EFP-999"}]}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		requireJiraCode(t, run(t, cfgZero, "zephyr", "execution", "resolve", "--cycle-id", "20000", "--issue", "EFP-123"), "zephyr_execution_not_found")

		cfgAmb, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/issue/EFP-123":
				w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
			case "/rest/zapi/latest/execution":
				w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-123","cycleId":"20000"},{"id":"30001","issueId":"10001","cycleId":"20000","folderId":"40000"}]}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		out := run(t, cfgAmb, "zephyr", "execution", "resolve", "--cycle-id", "20000", "--issue", "EFP-123")
		requireJiraCode(t, out, "ambiguous_zephyr_execution")
		if !strings.Contains(out["error"].(map[string]interface{})["hint"].(string), "30000") {
			t.Fatalf("ambiguous hint missing candidates: %#v", out)
		}
	})
}

func TestZephyrExecutionUpdateStatusSemanticDryRunAndInvalidMix(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/issue/EFP-123":
			w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/execution":
			w.Write([]byte(`{"executions":{"30000":{"issueKey":"EFP-123","issueId":"10001","cycleId":"20000"}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	out := run(t, cfg, "--dry-run", "zephyr", "execution", "update-status", "--cycle-id", "20000", "--issue", "EFP-123", "--status", "PASSED")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("semantic dry-run failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	body := data["body"].(map[string]interface{})
	if data["execution_id"] != "30000" || data["issue_key"] != "EFP-123" || data["path"] != "/rest/zapi/latest/execution/30000/execute" || body["status"] != "1" || data["target_status"] != "PASS" {
		t.Fatalf("bad semantic dry-run data: %#v", data)
	}
	direct := run(t, cfg, "--dry-run", "zephyr", "execution", "update-status", "30000", "--status", "PASSED")
	if ok, _ := direct["ok"].(bool); !ok {
		t.Fatalf("direct dry-run failed: %#v", direct)
	}
	requireJiraCode(t, run(t, cfg, "zephyr", "execution", "update-status", "30000", "--issue", "EFP-123", "--status", "PASS"), "invalid_args")
}

func TestZephyrExecutionAddTestsToCycleFolderMove(t *testing.T) {
	var addBody map[string]interface{}
	var moveBody map[string]interface{}
	executionListHits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/execution/addTestsToCycle":
			if r.Method != http.MethodPost {
				t.Fatalf("bad add method: %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&addBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"added":2}`))
		case "/rest/api/2/issue/EFP-T1":
			w.Write([]byte(`{"id":"10001","key":"EFP-T1","fields":{"project":{"id":"10000"}}}`))
		case "/rest/api/2/issue/EFP-T2":
			w.Write([]byte(`{"id":"10002","key":"EFP-T2","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/execution":
			executionListHits++
			q := r.URL.Query()
			if q.Get("cycleId") != "20000" || q.Get("projectId") != "10000" || q.Get("versionId") != "-1" || q.Get("action") != "expand" || q.Get("folderId") != "" {
				t.Fatalf("bad execution resolve query: %s", r.URL.RawQuery)
			}
			if executionListHits == 1 {
				w.Write([]byte(`{"executions":[]}`))
				return
			}
			w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-T1","issueId":"10001","cycleId":"20000","folderId":"40000"},{"id":"30002","issueKey":"EFP-T1","issueId":"10001","cycleId":"20000","folderId":"11111"},{"id":"30001","issueKey":"EFP-T2","issueId":"10002","cycleId":"20000"}]}`))
		case "/rest/zapi/latest/cycle/20000/move/executions/folder/40000":
			if r.Method != http.MethodPut {
				t.Fatalf("bad move method: %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&moveBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"moved":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	out := run(t, cfg, "zephyr", "execution", "add-tests-to-cycle", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--issues", "EFP-T1,EFP-T2", "--folder-id", "40000")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("add and move failed: %#v", out)
	}
	if addBody["cycleId"] != "20000" || addBody["projectId"] != "10000" || addBody["versionId"] != "-1" || len(addBody["issues"].([]interface{})) != 2 {
		t.Fatalf("bad add body: %#v", addBody)
	}
	ids := moveBody["ids"].([]interface{})
	if len(ids) != 2 || ids[0] != float64(30002) || ids[1] != float64(30001) {
		t.Fatalf("bad move ids: %#v", moveBody)
	}
	if executionListHits != 2 {
		t.Fatalf("execution list hits=%d", executionListHits)
	}
	data := out["data"].(map[string]interface{})
	if data["folder_id"] != "40000" || len(data["execution_ids"].([]interface{})) != 3 || len(data["moved_execution_ids"].([]interface{})) != 2 || len(data["already_in_folder_execution_ids"].([]interface{})) != 1 {
		t.Fatalf("bad add/move data: %#v", data)
	}
}

func TestZephyrExecutionAddTestsToCycleFolderDryRun(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("folder dry-run should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "--dry-run", "zephyr", "execution", "add-tests-to-cycle", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--issues", "EFP-T1,EFP-T2", "--folder-id", "40000")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("folder dry-run failed: %#v", out)
	}
	if *hits != before {
		t.Fatal("folder dry-run hit server")
	}
	data := out["data"].(map[string]interface{})
	move := data["post_add_move"].(map[string]interface{})
	body := move["body"].(map[string]interface{})
	if move["path"] != "/rest/zapi/latest/cycle/20000/move/executions/folder/40000" || body["ids"] == nil {
		t.Fatalf("bad folder dry-run data: %#v", data)
	}
	if ids, ok := body["ids"].([]interface{}); ok && len(ids) == 0 {
		t.Fatalf("dry-run should not model an empty ids move: %#v", data)
	}
}

func TestZephyrRawMoveFolderRejectsEmptyIDs(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("raw empty move should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "zephyr", "api", "put", "/rest/zapi/latest/cycle/20000/move/executions/folder/40000", "--body", `{"ids":[]}`)
	requireJiraCode(t, out, "invalid_args")
	if *hits != before {
		t.Fatal("raw empty move hit server")
	}
}

func TestZephyrExecutionAddTestsToCycleCreatesFolderByName(t *testing.T) {
	var createFolderBody map[string]interface{}
	var moveBody map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/cycle/20000/folders":
			w.Write([]byte(`{"folders":[]}`))
		case "/rest/zapi/latest/folder/create":
			if err := json.NewDecoder(r.Body).Decode(&createFolderBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"id":"40000","name":"Smoke"}`))
		case "/rest/zapi/latest/execution/addTestsToCycle":
			w.Write([]byte(`{"added":1}`))
		case "/rest/api/2/issue/EFP-T1":
			w.Write([]byte(`{"id":"10001","key":"EFP-T1","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/execution":
			w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-T1","issueId":"10001","cycleId":"20000"}]}`))
		case "/rest/zapi/latest/cycle/20000/move/executions/folder/40000":
			if err := json.NewDecoder(r.Body).Decode(&moveBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"moved":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	out := run(t, cfg, "zephyr", "execution", "add-tests-to-cycle", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--issues", "EFP-T1", "--folder-name", "Smoke", "--create-folder")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("folder-name add/move failed: %#v", out)
	}
	if createFolderBody["name"] != "Smoke" || createFolderBody["cycleId"] != "20000" {
		t.Fatalf("bad folder create body: %#v", createFolderBody)
	}
	if ids := moveBody["ids"].([]interface{}); len(ids) != 1 || ids[0] != float64(30000) {
		t.Fatalf("bad folder-name move body: %#v", moveBody)
	}
}

func TestZephyrExecutionBulkUpdateStatusByIssues(t *testing.T) {
	var updates []map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/util/testExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"}]`))
		case "/rest/zapi/latest/util/teststepExecutionStatus":
			w.Write([]byte(`[{"id":1,"name":"PASS"}]`))
		case "/rest/api/2/issue/EFP-T1":
			w.Write([]byte(`{"id":"10001","key":"EFP-T1","fields":{"project":{"id":"10000"}}}`))
		case "/rest/api/2/issue/EFP-T2":
			w.Write([]byte(`{"id":"10002","key":"EFP-T2","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/execution":
			if r.URL.Query().Get("folderId") != "40000" {
				t.Fatalf("bulk status did not scope folder: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-T1","issueId":"10001","folderId":"40000"},{"id":"30001","issueKey":"EFP-T2","issueId":"10002","folderId":"40000"}]}`))
		case "/rest/zapi/latest/execution/30000/execute", "/rest/zapi/latest/execution/30001/execute":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			updates = append(updates, body)
			w.Write([]byte(`{"updated":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	out := run(t, cfg, "zephyr", "execution", "bulk-update-status", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--folder-id", "40000", "--issues", "EFP-T1,EFP-T2", "--status", "PASS")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("bulk status by issues failed: %#v", out)
	}
	if len(updates) != 2 || updates[0]["status"] != "1" || updates[1]["status"] != "1" {
		t.Fatalf("bad bulk updates: %#v", updates)
	}
}

func TestZephyrArchiveAndRestoreByIssues(t *testing.T) {
	var archiveBody map[string]interface{}
	var restoreBody map[string]interface{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/issue/EFP-T1":
			w.Write([]byte(`{"id":"10001","key":"EFP-T1","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/execution":
			w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-T1","issueId":"10001","cycleId":"20000"}]}`))
		case "/rest/zapi/latest/execution/archive":
			if r.Method == http.MethodGet {
				w.Write([]byte(`{"executions":[{"id":"30000","issueKey":"EFP-T1","issueId":"10001","cycleId":"20000"}]}`))
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&archiveBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"jobProgressToken":"archive-token"}`))
		case "/rest/zapi/latest/execution/restore":
			if err := json.NewDecoder(r.Body).Decode(&restoreBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"jobProgressToken":"restore-token"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	archive := run(t, cfg, "--yes", "zephyr", "archive", "executions", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--issues", "EFP-T1")
	if ok, _ := archive["ok"].(bool); !ok {
		t.Fatalf("archive by issues failed: %#v", archive)
	}
	restore := run(t, cfg, "zephyr", "archive", "restore", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--issues", "EFP-T1")
	if ok, _ := restore["ok"].(bool); !ok {
		t.Fatalf("restore by issues failed: %#v", restore)
	}
	if ids := archiveBody["executions"].([]interface{}); len(ids) != 1 || ids[0] != float64(30000) {
		t.Fatalf("bad archive body: %#v", archiveBody)
	}
	if ids := restoreBody["executions"].([]interface{}); len(ids) != 1 || ids[0] != float64(30000) {
		t.Fatalf("bad restore body: %#v", restoreBody)
	}
}

func TestZephyrDynamicStatusesAndFallback(t *testing.T) {
	t.Run("status list parses server statuses", func(t *testing.T) {
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/zapi/latest/util/testExecutionStatus":
				w.Write([]byte(`[{"id":1,"name":"PASS"},{"id":9,"name":"CUSTOM"}]`))
			case "/rest/zapi/latest/util/teststepExecutionStatus":
				w.Write([]byte(`[{"id":1,"name":"PASS"},{"id":5,"name":"APPROVED"}]`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		out := run(t, cfg, "zephyr", "status", "list")
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("status list failed: %#v", out)
		}
		data := out["data"].(map[string]interface{})
		if data["source"] != "server" || len(data["execution_statuses"].([]interface{})) != 2 || len(data["step_statuses"].([]interface{})) != 2 {
			t.Fatalf("bad status list: %#v", data)
		}
	})

	t.Run("server custom status maps for writes", func(t *testing.T) {
		var body map[string]interface{}
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/zapi/latest/util/testExecutionStatus":
				w.Write([]byte(`[{"id":1,"name":"PASS"},{"id":9,"name":"CUSTOM"}]`))
			case "/rest/zapi/latest/util/teststepExecutionStatus":
				w.Write([]byte(`[{"id":1,"name":"PASS"}]`))
			case "/rest/zapi/latest/execution/30000/execute":
				if r.Method != http.MethodPut {
					t.Fatalf("bad method: %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.Write([]byte(`{"updated":true}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		if ok, _ := run(t, cfg, "zephyr", "execution", "update-status", "30000", "--status", "custom")["ok"].(bool); !ok {
			t.Fatal("custom status update failed")
		}
		if body["status"] != "9" {
			t.Fatalf("custom status not mapped: %#v", body)
		}
	})

	t.Run("write fallback when status endpoints unavailable", func(t *testing.T) {
		var body map[string]interface{}
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/zapi/latest/util/testExecutionStatus", "/rest/zapi/latest/util/teststepExecutionStatus":
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{}`))
			case "/rest/zapi/latest/execution/30000/execute":
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.Write([]byte(`{"updated":true}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		if ok, _ := run(t, cfg, "zephyr", "execution", "update-status", "30000", "--status", "PASSED")["ok"].(bool); !ok {
			t.Fatal("fallback status update failed")
		}
		if body["status"] != "1" {
			t.Fatalf("fallback status not mapped: %#v", body)
		}
	})
}

func TestZephyrCycleResolve(t *testing.T) {
	t.Run("exact and case-insensitive", func(t *testing.T) {
		cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/api/2/project/EFP":
				w.Write([]byte(`{"id":"10000","key":"EFP"}`))
			case "/rest/zapi/latest/cycle":
				w.Write([]byte(`{"cycles":[{"id":"20000","name":"Sprint 42 Regression","projectId":"10000","versionId":"-1"},{"id":"20001","name":"smoke"}]}`))
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			}
		})
		exact := run(t, cfg, "zephyr", "cycle", "resolve", "--project", "EFP", "--name", "Sprint 42 Regression", "--version-id", "-1")
		if ok, _ := exact["ok"].(bool); !ok {
			t.Fatalf("exact cycle resolve failed: %#v", exact)
		}
		if got := exact["data"].(map[string]interface{})["cycle_id"]; got != "20000" {
			t.Fatalf("cycle_id=%#v", got)
		}
		ci := run(t, cfg, "zephyr", "cycle", "resolve", "--project", "EFP", "--name", "SMOKE", "--version-id", "-1")
		if ok, _ := ci["ok"].(bool); !ok {
			t.Fatalf("case-insensitive cycle resolve failed: %#v", ci)
		}
		if got := ci["data"].(map[string]interface{})["cycle_id"]; got != "20001" {
			t.Fatalf("cycle_id=%#v", got)
		}
	})

	cfgAmb, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/cycle":
			w.Write([]byte(`{"cycles":[{"id":"20000","name":"Regression"},{"id":"20001","name":"Regression"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	requireJiraCode(t, run(t, cfgAmb, "zephyr", "cycle", "resolve", "--name", "Regression"), "ambiguous_zephyr_cycle")
}

func TestZephyrVersionListAndResolve(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/project/EFP":
			w.Write([]byte(`{"id":"10000","key":"EFP"}`))
		case "/rest/zapi/latest/util/versionBoard-list":
			if r.URL.Query().Get("projectId") != "10000" {
				t.Fatalf("bad version query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"type":"software","unreleasedVersions":[{"value":"-1","archived":false,"label":"Unscheduled"},{"value":"11700","archived":false,"label":"TestVersion1"}],"releasedVersions":[{"value":"11701","archived":false,"label":"ReleasedVersion"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	list := run(t, cfg, "zephyr", "version", "list", "--project", "EFP")
	if ok, _ := list["ok"].(bool); !ok {
		t.Fatalf("version list failed: %#v", list)
	}
	data := list["data"].(map[string]interface{})
	if data["version_count"] != float64(3) {
		t.Fatalf("bad version count: %#v", data)
	}
	resolve := run(t, cfg, "zephyr", "version", "resolve", "--project", "EFP", "--name", "testversion1")
	if ok, _ := resolve["ok"].(bool); !ok {
		t.Fatalf("version resolve failed: %#v", resolve)
	}
	if got := resolve["data"].(map[string]interface{})["version_id"]; got != "11700" {
		t.Fatalf("version_id=%#v", got)
	}
}

func TestZephyrArchiveCommands(t *testing.T) {
	var archivedBody map[string]interface{}
	var restoreBody map[string]interface{}
	var exportBody map[string]interface{}
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/execution/archive":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" || r.URL.Query().Get("cycleId") != "20000" || r.URL.Query().Get("maxRecords") != "25" {
					t.Fatalf("bad archive list query: %s", r.URL.RawQuery)
				}
				w.Write([]byte(`{"executions":[]}`))
			case http.MethodPost:
				if err := json.NewDecoder(r.Body).Decode(&archivedBody); err != nil {
					t.Fatal(err)
				}
				w.Write([]byte(`{"jobProgressToken":"archive-token"}`))
			default:
				t.Fatalf("bad archive method: %s", r.Method)
			}
		case "/rest/zapi/latest/execution/restore":
			if err := json.NewDecoder(r.Body).Decode(&restoreBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"jobProgressToken":"restore-token"}`))
		case "/rest/zapi/latest/execution/archive/export":
			if err := json.NewDecoder(r.Body).Decode(&exportBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"url":"https://jira.example.test/export.xls"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	if ok, _ := run(t, cfg, "zephyr", "archive", "list", "--project-id", "10000", "--version-id", "-1", "--cycle-id", "20000", "--limit", "25")["ok"].(bool); !ok {
		t.Fatal("archive list failed")
	}
	requireJiraCode(t, run(t, cfg, "zephyr", "archive", "executions", "--execution-ids", "30000"), "invalid_args")
	before := *hits
	dry := run(t, cfg, "--yes", "--dry-run", "zephyr", "archive", "executions", "--execution-ids", "30000,30001")
	if *hits != before {
		t.Fatal("archive dry-run hit server")
	}
	if dry["data"].(map[string]interface{})["path"] != "/rest/zapi/latest/execution/archive" {
		t.Fatalf("bad archive dry-run: %#v", dry)
	}
	if ok, _ := run(t, cfg, "--yes", "zephyr", "archive", "executions", "--execution-ids", "30000,30001")["ok"].(bool); !ok {
		t.Fatal("archive executions failed")
	}
	if ok, _ := run(t, cfg, "zephyr", "archive", "restore", "--execution-ids", "30000")["ok"].(bool); !ok {
		t.Fatal("archive restore failed")
	}
	if ok, _ := run(t, cfg, "zephyr", "archive", "export", "--type", "csv", "--start", "10")["ok"].(bool); !ok {
		t.Fatal("archive export failed")
	}
	if len(archivedBody["executions"].([]interface{})) != 2 || len(restoreBody["executions"].([]interface{})) != 1 || exportBody["exportType"] != "csv" || exportBody["startIndex"] != "10" {
		t.Fatalf("bad archive bodies: archive=%#v restore=%#v export=%#v", archivedBody, restoreBody, exportBody)
	}
}

func TestZephyrCustomFieldCommands(t *testing.T) {
	var createBody map[string]interface{}
	var updateBody map[string]interface{}
	var deleteBulkBody map[string]interface{}
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/customfield/byEntityTypeAndProject":
			if r.URL.Query().Get("entityType") != "EXECUTION" || r.URL.Query().Get("projectId") != "10000" {
				t.Fatalf("bad customfield list query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`[{"id":"3","name":"Actual Result"}]`))
		case "/rest/zapi/latest/customfield/3":
			switch r.Method {
			case http.MethodGet:
				w.Write([]byte(`{"id":"3","name":"Actual Result"}`))
			case http.MethodPut:
				if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
					t.Fatal(err)
				}
				w.Write([]byte(`{"id":"3"}`))
			case http.MethodDelete:
				w.Write([]byte(`{"message":"deleted"}`))
			default:
				t.Fatalf("bad customfield method: %s", r.Method)
			}
		case "/rest/zapi/latest/customfield/create":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"id":"3"}`))
		case "/rest/zapi/latest/customfield/delete-customfields":
			if err := json.NewDecoder(r.Body).Decode(&deleteBulkBody); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"message":"deleted"}`))
		case "/rest/zapi/latest/customfield/3/10000":
			if r.Method != http.MethodDelete || r.URL.Query().Get("enable") != "false" {
				t.Fatalf("bad customfield enable request: %s %s", r.Method, r.URL.String())
			}
			w.Write([]byte(`{"message":"disabled"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	if ok, _ := run(t, cfg, "zephyr", "customfield", "list", "--entity-type", "execution", "--project-id", "10000")["ok"].(bool); !ok {
		t.Fatal("customfield list failed")
	}
	if ok, _ := run(t, cfg, "zephyr", "customfield", "get", "3")["ok"].(bool); !ok {
		t.Fatal("customfield get failed")
	}
	if ok, _ := run(t, cfg, "zephyr", "customfield", "create", "--name", "Actual Result", "--entity-type", "execution", "--field-type", "text", "--project-id", "10000")["ok"].(bool); !ok {
		t.Fatal("customfield create failed")
	}
	if createBody["entityType"] != "EXECUTION" || createBody["fieldType"] != "TEXT" || createBody["isActive"] != true {
		t.Fatalf("bad customfield create body: %#v", createBody)
	}
	if ok, _ := run(t, cfg, "zephyr", "customfield", "update", "3", "--name", "Actual Result RC2", "--field", "description=Updated")["ok"].(bool); !ok {
		t.Fatal("customfield update failed")
	}
	if updateBody["name"] != "Actual Result RC2" || updateBody["description"] != "Updated" {
		t.Fatalf("bad customfield update body: %#v", updateBody)
	}
	before := *hits
	requireJiraCode(t, run(t, cfg, "zephyr", "customfield", "delete", "3"), "invalid_args")
	dry := run(t, cfg, "--yes", "--dry-run", "zephyr", "customfield", "delete", "3")
	if *hits != before {
		t.Fatal("customfield delete validation/dry-run hit server")
	}
	if dry["data"].(map[string]interface{})["method"] != "DELETE" {
		t.Fatalf("bad customfield delete dry-run: %#v", dry)
	}
	if ok, _ := run(t, cfg, "--yes", "zephyr", "customfield", "delete", "3")["ok"].(bool); !ok {
		t.Fatal("customfield delete failed")
	}
	if ok, _ := run(t, cfg, "--yes", "zephyr", "customfield", "delete-bulk", "--customfield-ids", "3,14")["ok"].(bool); !ok {
		t.Fatal("customfield delete-bulk failed")
	}
	if len(deleteBulkBody["customfields"].([]interface{})) != 2 {
		t.Fatalf("bad customfield delete-bulk body: %#v", deleteBulkBody)
	}
	if ok, _ := run(t, cfg, "zephyr", "customfield", "enable", "3", "--project-id", "10000", "--enabled=false")["ok"].(bool); !ok {
		t.Fatal("customfield enable failed")
	}
}

func TestZephyrAPICatalogAndDescribe(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("api catalog should not hit server: %s %s", r.Method, r.URL.Path)
	})
	before := *hits
	out := run(t, cfg, "zephyr", "api", "catalog")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("catalog failed: %#v", out)
	}
	data := out["data"].(map[string]interface{})
	groups := map[string]bool{}
	for _, item := range data["groups"].([]interface{}) {
		groups[item.(string)] = true
	}
	for _, want := range []string{"ChartResource", "ExecutionSearchResource", "ZQLFilterResource", "CycleResource", "ZNavResource", "LicenseResource", "PreferenceResource", "StepResultResource", "TraceabilityResource", "TestcaseResource", "UtilResource", "FolderResource", "ExecutionResource", "ExecutionArchiveResource", "IssuePickerResource", "AuditResource", "TeststepResource", "AttachmentResource", "CustomFieldResource", "ZAPIResource", "ZQLAutoCompleteResource", "SystemInfoResource", "FilterPickerResource"} {
		if !groups[want] {
			t.Fatalf("catalog missing group %s", want)
		}
	}
	for _, id := range []string{"execution.update-status", "execution.archive", "customfield.create", "util.version-board-list", "cycle.list", "folder.create", "teststep.list", "attachment.delete", "zql.clauses"} {
		desc := run(t, cfg, "zephyr", "api", "describe", id)
		if ok, _ := desc["ok"].(bool); !ok {
			t.Fatalf("describe %s failed: %#v", id, desc)
		}
		if desc["data"].(map[string]interface{})["id"] != id {
			t.Fatalf("bad describe %s: %#v", id, desc)
		}
	}
	if *hits != before {
		t.Fatal("api catalog/describe hit server")
	}
}

func TestZephyrZQLMetadataCommands(t *testing.T) {
	seen := map[string]bool{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/zapi/latest/zql/clauses":
			seen["clauses"] = true
			w.Write([]byte(`{"clauses":[]}`))
		case "/rest/zapi/latest/zql/autocompleteZQLJson":
			seen["autocomplete-json"] = true
			w.Write([]byte(`{"fields":[]}`))
		case "/rest/zapi/latest/zql/autocomplete":
			if r.URL.Query().Get("fieldName") != "executionStatus" || r.URL.Query().Get("fieldValue") != "PA" {
				t.Fatalf("bad autocomplete query: %s", r.URL.RawQuery)
			}
			seen["autocomplete"] = true
			w.Write([]byte(`{"values":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	for _, args := range [][]string{
		{"zephyr", "zql", "clauses"},
		{"zephyr", "zql", "autocomplete-json"},
		{"zephyr", "zql", "autocomplete", "--field-name", "executionStatus", "--field-value", "PA"},
	} {
		if ok, _ := run(t, cfg, args...)["ok"].(bool); !ok {
			t.Fatalf("zql command failed: %v", args)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("missing zql metadata requests: %#v", seen)
	}
}

func TestZephyrFolderCommands(t *testing.T) {
	var sawList bool
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/zapi/latest/cycle/20000/folders" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("projectId") != "10000" || r.URL.Query().Get("versionId") != "-1" || r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("offset") != "5" {
			t.Fatalf("bad folder list query: %s", r.URL.RawQuery)
		}
		sawList = true
		w.Write([]byte(`{"folders":[]}`))
	})
	if ok, _ := run(t, cfg, "zephyr", "folder", "list", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--limit", "10", "--offset", "5")["ok"].(bool); !ok {
		t.Fatal("folder list failed")
	}
	before := *hits
	create := run(t, cfg, "--dry-run", "zephyr", "folder", "create", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1", "--name", "Smoke", "--description", "d")
	update := run(t, cfg, "--dry-run", "zephyr", "folder", "update", "40000", "--name", "Smoke RC2")
	requireJiraCode(t, run(t, cfg, "zephyr", "folder", "delete", "40000", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1"), "invalid_args")
	del := run(t, cfg, "--yes", "--dry-run", "zephyr", "folder", "delete", "40000", "--cycle-id", "20000", "--project-id", "10000", "--version-id", "-1")
	if *hits != before {
		t.Fatal("folder dry-runs hit server")
	}
	if !sawList {
		t.Fatal("folder list was not sent")
	}
	if create["data"].(map[string]interface{})["body"].(map[string]interface{})["name"] != "Smoke" || update["data"].(map[string]interface{})["body"].(map[string]interface{})["name"] != "Smoke RC2" {
		t.Fatalf("bad folder body: create=%#v update=%#v", create, update)
	}
	if del["data"].(map[string]interface{})["method"] != "DELETE" {
		t.Fatalf("bad folder delete dry-run: %#v", del)
	}
}

func TestZephyrTeststepCRUD(t *testing.T) {
	var sawList bool
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/2/issue/EFP-123":
			w.Write([]byte(`{"id":"10001","key":"EFP-123","fields":{"project":{"id":"10000"}}}`))
		case "/rest/zapi/latest/teststep/10001":
			if r.Method != http.MethodGet || r.URL.Query().Get("offset") != "0" || r.URL.Query().Get("limit") != "50" {
				t.Fatalf("bad teststep list request: %s %s", r.Method, r.URL.String())
			}
			sawList = true
			w.Write([]byte(`{"stepBeanCollection":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	if ok, _ := run(t, cfg, "zephyr", "teststep", "list", "--issue", "EFP-123")["ok"].(bool); !ok {
		t.Fatal("teststep list failed")
	}
	before := *hits
	get := run(t, cfg, "--dry-run", "zephyr", "teststep", "get", "--issue", "EFP-123", "--step-id", "10")
	create := run(t, cfg, "--dry-run", "zephyr", "teststep", "create", "--issue", "EFP-123", "--step", "Open login page", "--data", "user exists", "--result", "Login page is shown")
	update := run(t, cfg, "--dry-run", "zephyr", "teststep", "update", "--issue", "EFP-123", "--step-id", "10", "--step", "Open login page")
	requireJiraCode(t, run(t, cfg, "zephyr", "teststep", "delete", "--issue", "EFP-123", "--step-id", "10"), "invalid_args")
	del := run(t, cfg, "--yes", "--dry-run", "zephyr", "teststep", "delete", "--issue", "EFP-123", "--step-id", "10")
	if *hits != before+4 {
		t.Fatalf("teststep dry-runs should resolve issue only, hits before=%d after=%d", before, *hits)
	}
	if !sawList {
		t.Fatal("teststep list was not sent")
	}
	if get["data"].(map[string]interface{})["path"] != "/rest/zapi/latest/teststep/10001/10" || del["data"].(map[string]interface{})["path"] != "/rest/zapi/latest/teststep/10001/10" {
		t.Fatalf("bad teststep paths: get=%#v delete=%#v", get, del)
	}
	if create["data"].(map[string]interface{})["body"].(map[string]interface{})["result"] != "Login page is shown" || update["data"].(map[string]interface{})["body"].(map[string]interface{})["step"] != "Open login page" {
		t.Fatalf("bad teststep bodies: create=%#v update=%#v", create, update)
	}
}

func TestZephyrAttachmentGetDelete(t *testing.T) {
	var sawGet bool
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/rest/zapi/latest/attachment/50000" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		sawGet = true
		w.Write([]byte(`{"id":"50000"}`))
	})
	if ok, _ := run(t, cfg, "zephyr", "attachment", "get", "50000")["ok"].(bool); !ok {
		t.Fatal("attachment get failed")
	}
	before := *hits
	requireJiraCode(t, run(t, cfg, "zephyr", "attachment", "delete", "50000"), "invalid_args")
	del := run(t, cfg, "--yes", "--dry-run", "zephyr", "attachment", "delete", "50000")
	if *hits != before {
		t.Fatal("attachment delete validation/dry-run hit server")
	}
	if !sawGet || del["data"].(map[string]interface{})["path"] != "/rest/zapi/latest/attachment/50000" {
		t.Fatalf("bad attachment behavior: get=%v delete=%#v", sawGet, del)
	}
}

func TestZephyrCommandSchemaCatalogCoverage(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	commands := run(t, cfg, "commands")
	commandData := commands["data"].(map[string]interface{})
	usages := map[string]bool{}
	for _, item := range commandData["commands"].([]interface{}) {
		cmdMeta := item.(map[string]interface{})
		usages[cmdMeta["usage"].(string)] = true
	}
	for _, want := range []string{
		"jira zephyr summary",
		"jira zephyr zql search",
		"jira zephyr zql clauses",
		"jira zephyr step-result update-status <step-result-id>",
		"jira zephyr attachment upload",
		"jira zephyr attachment delete <attachment-id>",
		"jira zephyr execution resolve",
		"jira zephyr cycle resolve",
		"jira zephyr version resolve",
		"jira zephyr archive list",
		"jira zephyr customfield list",
		"jira zephyr folder list",
		"jira zephyr teststep list",
		"jira zephyr api catalog",
		"jira zephyr execution bulk-update-status",
		"jira zephyr api delete <path>",
	} {
		if !usages[want] {
			t.Fatalf("commands missing %q", want)
		}
	}
	cases := map[string]string{
		"zephyr.zql.search":                   "jira zephyr zql search",
		"zephyr.zql.clauses":                  "jira zephyr zql clauses",
		"zephyr.step-result.update-status":    "jira zephyr step-result update-status <step-result-id>",
		"zephyr.attachment.upload":            "jira zephyr attachment upload",
		"zephyr.attachment.delete":            "jira zephyr attachment delete <attachment-id>",
		"zephyr.execution.resolve":            "jira zephyr execution resolve",
		"zephyr.cycle.resolve":                "jira zephyr cycle resolve",
		"zephyr.version.resolve":              "jira zephyr version resolve",
		"zephyr.archive.list":                 "jira zephyr archive list",
		"zephyr.customfield.list":             "jira zephyr customfield list",
		"zephyr.folder.list":                  "jira zephyr folder list",
		"zephyr.teststep.list":                "jira zephyr teststep list",
		"zephyr.api.catalog":                  "jira zephyr api catalog",
		"zephyr.execution.bulk-update-status": "jira zephyr execution bulk-update-status",
		"zephyr.api.delete":                   "jira zephyr api delete <path>",
	}
	for name, usage := range cases {
		out := run(t, cfg, "schema", name)
		if ok, _ := out["ok"].(bool); !ok {
			t.Fatalf("schema %s failed: %#v", name, out)
		}
		data := out["data"].(map[string]interface{})
		if data["usage"] != usage {
			t.Fatalf("schema %s usage=%#v want %q", name, data["usage"], usage)
		}
		if len(data["flags"].([]interface{})) == 0 {
			t.Fatalf("schema %s missing flags: %#v", name, data)
		}
	}
	apiDelete := run(t, cfg, "schema", "zephyr.api.delete")["data"].(map[string]interface{})
	if apiDelete["risk"] != "delete" {
		t.Fatalf("api delete risk not delete: %#v", apiDelete)
	}
}
