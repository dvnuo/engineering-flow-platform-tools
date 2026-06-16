package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/catalog"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
	"github.com/spf13/cobra"
)

type semanticMock struct {
	server *httptest.Server
	hits   int
}

func newSemanticMock(t *testing.T) *semanticMock {
	t.Helper()
	m := &semanticMock{}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.hits++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"auth required","errorMessages":["auth required"]}`))
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/transitions"):
			_, _ = w.Write([]byte(`{"transitions":[{"id":"31","name":"Done"}]}`))
		case strings.Contains(r.URL.Path, "/content/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"123","version":{"number":2},"body":{"storage":{"value":"<p>Hello</p>"}}}`))
		case strings.Contains(r.URL.Path, "/attachment/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"10000","content":"` + m.server.URL + `/download.bin"}`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"ok":true,"id":"123","key":"PROJ-123","results":[],"values":[],"issues":[]}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"id":"123","key":"PROJ-123"}`))
		}
	}))
	t.Cleanup(m.server.Close)
	return m
}

func semanticConfig(t *testing.T, baseURL string) string {
	t.Helper()
	cfg, err := testutil.WriteConfig(testutil.JiraConfig(baseURL))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func runSemantic(t *testing.T, product, cfg string, args ...string) map[string]any {
	t.Helper()
	var root *cobra.Command
	switch product {
	case "jira":
		root = jcmd.NewRoot()
	case "confluence":
		root = ccmd.NewRoot()
	default:
		t.Fatalf("unknown product %q", product)
	}
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	root.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	_ = root.Execute()
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("%s %v did not emit JSON envelope: %q", product, args, b.String())
	}
	return out
}

func requireOK(t *testing.T, product, cfg string, args ...string) {
	t.Helper()
	out := runSemantic(t, product, cfg, args...)
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("%s %v failed: %#v", product, args, out)
	}
}

func requireErrorCode(t *testing.T, code string, product, cfg string, args ...string) {
	t.Helper()
	out := runSemantic(t, product, cfg, args...)
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("%s %v unexpectedly succeeded: %#v", product, args, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if got, _ := errObj["code"].(string); got != code {
		t.Fatalf("%s %v error.code=%q want %q: %#v", product, args, got, code, out)
	}
}

func requireDryRunNoHit(t *testing.T, m *semanticMock, product, cfg string, args ...string) {
	t.Helper()
	before := m.hits
	requireOK(t, product, cfg, append([]string{"--dry-run"}, args...)...)
	if m.hits != before {
		t.Fatalf("%s dry-run %v hit mock server: before=%d after=%d", product, args, before, m.hits)
	}
}

func TestCommandSemanticsMatrixJiraFamilies(t *testing.T) {
	m := newSemanticMock(t)
	cfg := semanticConfig(t, m.server.URL)
	upload := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(upload, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	readCases := [][]string{
		{"issue", "get", "PROJ-123"},
		{"issue", "comment", "list", "PROJ-123"},
		{"issue", "attachment", "list", "PROJ-123"},
		{"attachment", "meta"},
		{"issue", "worklog", "list", "PROJ-123"},
		{"issue", "link", "list", "PROJ-123"},
		{"issue", "remote-link", "list", "PROJ-123"},
		{"issue", "property", "list", "PROJ-123"},
		{"project", "list"},
		{"component", "get", "10000"},
		{"version", "get", "10000"},
		{"user", "search", "--query", "alice"},
		{"group", "search", "--query", "team"},
		{"field", "list"},
		{"issue-type", "list"},
		{"status", "list"},
		{"priority", "list"},
		{"resolution", "list"},
		{"workflow", "list"},
		{"permissions", "myself"},
		{"settings", "get"},
		{"config", "get"},
		{"filter", "list"},
		{"dashboard", "list"},
		{"api", "get", "/rest/api/2/myself"},
		{"board", "list"},
		{"sprint", "list", "1"},
		{"backlog", "issues", "1"},
		{"zephyr", "resolve-url", "https://jira.example.test/projects/PROJ?selectedItem=com.thed.zephyr.je%3Azephyr-tests-page#test-summary-tab"},
		{"zephyr", "doctor", "--project", "PROJ"},
		{"zephyr", "status", "list"},
		{"zephyr", "util", "test-issue-type"},
		{"zephyr", "test", "list", "--project", "PROJ"},
		{"zephyr", "test", "get", "PROJ-T1"},
		{"zephyr", "version", "list", "--project", "PROJ"},
		{"zephyr", "cycle", "list", "--project", "PROJ"},
		{"zephyr", "execution", "list", "--cycle-id", "1", "--project-id", "123"},
		{"zephyr", "execution", "get", "1"},
		{"zephyr", "archive", "list", "--project-id", "123"},
		{"zephyr", "customfield", "list", "--entity-type", "EXECUTION"},
		{"zephyr", "api", "get", "cycle", "--query", "projectId=123", "--query", "versionId=-1"},
	}
	for _, args := range readCases {
		requireOK(t, "jira", cfg, args...)
	}

	writeDryRuns := [][]string{
		{"issue", "create", "--project", "PROJ", "--type", "Task", "--summary", "Test"},
		{"issue", "update", "PROJ-123", "--summary", "Done"},
		{"issue", "assign", "PROJ-123", "--user", "alice"},
		{"issue", "transition", "PROJ-123", "--transition-id", "31"},
		{"issue", "comment", "add", "PROJ-123", "--body", "ok"},
		{"issue", "attachment", "upload", "PROJ-123", upload},
		{"issue", "worklog", "add", "PROJ-123", "--time-spent", "1h"},
		{"issue", "link", "create", "--type", "Relates", "--from", "PROJ-123", "--to", "PROJ-124"},
		{"issue", "remote-link", "add", "PROJ-123", "--url", "https://example.com", "--title", "Spec"},
		{"issue", "property", "set", "PROJ-123", "status", "--value", `{"ok":true}`},
		{"component", "create", "--project", "PROJ", "--name", "API"},
		{"version", "create", "--project", "PROJ", "--name", "1.0"},
		{"filter", "create", "--name", "Mine", "--jql", "project = PROJ"},
		{"api", "post", "/rest/api/2/issue", "--body", `{"fields":{}}`},
		{"zephyr", "test", "create", "--project", "PROJ", "--summary", "Login rejects expired token"},
		{"zephyr", "cycle", "create", "--project", "PROJ", "--name", "Regression"},
		{"zephyr", "cycle", "update", "1", "--name", "Regression RC2"},
		{"zephyr", "execution", "create", "--issue-id", "10001", "--cycle-id", "1", "--project-id", "123"},
		{"zephyr", "execution", "update-status", "1", "--status", "PASS"},
		{"zephyr", "execution", "add-tests-to-cycle", "--cycle-id", "1", "--project-id", "123", "--issues", "PROJ-T1,PROJ-T2", "--folder-id", "456"},
		{"zephyr", "archive", "executions", "--execution-ids", "1,2", "--yes"},
		{"zephyr", "archive", "restore", "--execution-ids", "1,2"},
		{"zephyr", "customfield", "create", "--name", "Actual Result", "--entity-type", "EXECUTION", "--field-type", "TEXT"},
		{"zephyr", "customfield", "update", "3", "--name", "Actual Result"},
		{"zephyr", "api", "post", "cycle", "--body", `{}`},
		{"zephyr", "api", "put", "execution/1/execute", "--body", `{"status":"1"}`},
	}
	for _, args := range writeDryRuns {
		requireDryRunNoHit(t, m, "jira", cfg, args...)
	}

	invalidCases := [][]string{
		{"issue", "create"},
		{"issue", "search", "--jql", "project = PROJ", "--limit", "bad"},
		{"issue", "transition", "PROJ-123"},
		{"issue", "comment", "add", "PROJ-123"},
		{"issue", "worklog", "add", "PROJ-123"},
		{"component", "create", "--project", "PROJ"},
		{"filter", "create", "--name", "Mine"},
		{"zephyr", "cycle", "create", "--project", "PROJ"},
	}
	for _, args := range invalidCases {
		requireErrorCode(t, "invalid_args", "jira", cfg, args...)
	}
	requireErrorCode(t, "invalid_zephyr_status", "jira", cfg, "zephyr", "execution", "update-status", "1", "--status", "unknown")
	requireErrorCode(t, "instance_url_mismatch", "jira", cfg, "api", "get", "https://evil.example/rest/api/2/myself")
	requireErrorCode(t, "zephyr_raw_path_blocked", "jira", cfg, "zephyr", "api", "get", "https://evil.example/rest/zapi/latest/cycle")
}

func TestCommandSemanticsMatrixConfluenceFamilies(t *testing.T) {
	m := newSemanticMock(t)
	cfg := semanticConfig(t, m.server.URL)
	upload := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(upload, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	readCases := [][]string{
		{"search", "--cql", "space = ENG"},
		{"cql", "--query", "space = ENG"},
		{"search", "content", "--space", "ENG", "--type", "page"},
		{"search", "user", "--query", "alice"},
		{"space", "list"},
		{"space", "get", "ENG"},
		{"space", "content", "ENG"},
		{"space", "pages", "ENG"},
		{"space", "blogs", "ENG"},
		{"space", "labels", "ENG"},
		{"space", "permission", "list", "ENG"},
		{"space", "property", "list", "ENG"},
		{"page", "get", "--id", "123"},
		{"page", "get-by-title", "--space", "ENG", "--title", "Home"},
		{"page", "children", "--id", "123"},
		{"page", "descendants", "--id", "123"},
		{"page", "ancestors", "--id", "123"},
		{"page", "body", "--id", "123"},
		{"page", "version", "--id", "123"},
		{"page", "history", "--id", "123"},
		{"content", "list"},
		{"content", "get", "123"},
		{"blog", "list", "--space", "ENG"},
		{"blog", "get", "123"},
		{"page", "attachment", "list", "--id", "123"},
		{"attachment", "get", "10000"},
		{"page", "comment", "list", "--id", "123"},
		{"comment", "get", "10000"},
		{"page", "label", "list", "--id", "123"},
		{"label", "list"},
		{"page", "property", "list", "--id", "123"},
		{"page", "restriction", "list", "--id", "123"},
		{"page", "watcher", "list", "--id", "123"},
		{"user", "search", "--query", "alice"},
		{"group", "list"},
		{"longtask", "list"},
		{"webhook", "list"},
		{"api", "get", "/rest/api/content"},
	}
	for _, args := range readCases {
		requireOK(t, "confluence", cfg, args...)
	}

	writeDryRuns := [][]string{
		{"space", "create", "--key", "ENG", "--name", "Engineering"},
		{"space", "update", "ENG", "--name", "Engineering"},
		{"space", "property", "set", "ENG", "status", "--body", "ok"},
		{"page", "create", "--space", "ENG", "--title", "Home", "--body", "<p>Hello</p>"},
		{"page", "update", "--id", "123", "--title", "Home"},
		{"page", "move", "--id", "123", "--parent-id", "456"},
		{"page", "restore", "--id", "123", "--version", "2"},
		{"content", "create", "--space", "ENG", "--title", "Home", "--body", "<p>Hello</p>"},
		{"content", "update", "123", "--title", "Home", "--body", "<p>Hello</p>"},
		{"blog", "create", "--space", "ENG", "--title", "Update", "--body", "<p>Hello</p>"},
		{"blog", "update", "123", "--title", "Update", "--body", "<p>Hello</p>"},
		{"page", "attachment", "upload", "--id", "123", "--file", upload},
		{"page", "attachment", "update", "--id", "123", "--attachment-id", "10000", "--file", upload},
		{"page", "comment", "add", "--id", "123", "--body", "ok"},
		{"comment", "update", "10000", "--body", "ok"},
		{"page", "label", "add", "--id", "123", "--label", "runbook"},
		{"page", "property", "set", "--id", "123", "--key", "status", "--body", "ok"},
		{"page", "restriction", "add", "--id", "123", "--operation", "read", "--user", "alice"},
		{"page", "watch", "--id", "123"},
		{"page", "unwatch", "--id", "123"},
		{"webhook", "create", "--name", "hook", "--url", "https://example.com", "--event", "page_created"},
		{"api", "post", "/rest/api/content", "--body", `{"type":"page"}`},
	}
	for _, args := range writeDryRuns {
		requireDryRunNoHit(t, m, "confluence", cfg, args...)
	}

	invalidCases := [][]string{
		{"search"},
		{"cql"},
		{"search", "user"},
		{"page", "create", "--space", "ENG"},
		{"page", "comment", "add", "--id", "123"},
		{"page", "label", "add", "--id", "123"},
		{"page", "property", "set", "--id", "123"},
		{"page", "restriction", "list"},
	}
	for _, args := range invalidCases {
		requireErrorCode(t, "invalid_args", "confluence", cfg, args...)
	}
	requireErrorCode(t, "instance_url_mismatch", "confluence", cfg, "api", "get", "https://evil.example/rest/api/content/123")
}

func TestDeleteRemoveLogoutRequireYesFromCatalog(t *testing.T) {
	m := newSemanticMock(t)
	cfg := semanticConfig(t, m.server.URL)
	for _, product := range []string{"jira", "confluence"} {
		for _, cmd := range catalog.Commands(product) {
			if cmd.Risk != "delete" {
				continue
			}
			if len(cmd.Examples) == 0 || !strings.Contains(cmd.Examples[0], "--yes") {
				t.Fatalf("%s %s delete metadata missing --yes example: %#v", product, cmd.Name, cmd)
			}
			args := strings.Fields(cmd.Examples[0])
			if len(args) == 0 || args[0] != product {
				t.Fatalf("%s %s bad example: %q", product, cmd.Name, cmd.Examples[0])
			}
			args = args[1:]
			filtered := args[:0]
			for _, arg := range args {
				if arg != "--yes" {
					filtered = append(filtered, arg)
				}
			}
			requireErrorCode(t, "invalid_args", product, cfg, filtered...)
		}
	}
}
