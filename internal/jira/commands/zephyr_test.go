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
		"jira zephyr step-result update-status <step-result-id>",
		"jira zephyr attachment upload",
		"jira zephyr execution bulk-update-status",
		"jira zephyr api delete <path>",
	} {
		if !usages[want] {
			t.Fatalf("commands missing %q", want)
		}
	}
	cases := map[string]string{
		"zephyr.zql.search":                   "jira zephyr zql search",
		"zephyr.step-result.update-status":    "jira zephyr step-result update-status <step-result-id>",
		"zephyr.attachment.upload":            "jira zephyr attachment upload",
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
