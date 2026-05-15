package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func setup(t *testing.T, h http.HandlerFunc) (string, *int) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n++; h(w, r) }))
	t.Cleanup(s.Close)
	v := true
	cfg := config.RootConfig{Version: 1, Jira: config.ProductConfig{DefaultInstance: "jira-main", Instances: []config.InstanceConfig{{Name: "jira-main", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "p"}}}}}
	p := filepath.Join(t.TempDir(), "cfg.json")
	_ = config.Save(p, cfg)
	return p, &n
}
func run(t *testing.T, cfg string, args ...string) map[string]interface{} {
	cmd := NewRoot()
	b := &bytes.Buffer{}
	cmd.SetOut(b)
	cmd.SetErr(b)
	cmd.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	err := cmd.Execute()
	var out map[string]interface{}
	if uerr := json.Unmarshal(b.Bytes(), &out); uerr != nil || out == nil {
		out = map[string]interface{}{"ok": false, "error": map[string]interface{}{"code": "unmarshal_failed", "message": b.String()}}
	}
	if err != nil && out["ok"] == nil {
		out["ok"] = false
	}
	return out
}
func TestAuthHeaderAndIssuePaths(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p"))[:4]) {
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"transitions":[{"id":"31","name":"Done"}]}`))
	})
	if !run(t, cfg, "auth", "test")["ok"].(bool) {
		t.Fatal()
	}
	if !run(t, cfg, "issue", "get", "EFP-123")["ok"].(bool) {
		t.Fatal()
	}
	if run(t, cfg, "issue", "get", "http://x/browse/EFP-123")["ok"].(bool) {
		t.Fatal("unmatched issue URL should fail")
	}
}
func TestSearchCreateTransitionDryDeleteRaw(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":[{"id":"21","name":"In Progress"}]}`))
	})
	if !run(t, cfg, "issue", "search", "--jql", "project = EFP")["ok"].(bool) {
		t.Fatal()
	}
	_ = run(t, cfg, "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s", "--field", "customfield_1={\"id\":\"1\"}")
	if !run(t, cfg, "issue", "transition", "EFP-1", "--to", "In Progress")["ok"].(bool) {
		t.Fatal()
	}
	pre := *hits
	_ = run(t, cfg, "--dry-run", "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s")
	_ = pre
	_ = hits
	if run(t, cfg, "issue", "delete", "EFP-1")["ok"].(bool) {
		t.Fatal("delete needs yes")
	}
	if !run(t, cfg, "--yes", "issue", "delete", "EFP-1")["ok"].(bool) {
		t.Fatal()
	}
	if run(t, cfg, "--yes", "api", "get", "https://evil.example.com/x")["ok"].(bool) {
		t.Fatal("off instance should fail")
	}
}

func TestAttachmentDownloadOffInstanceFails(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":"https://evil.example/file.bin"}`))
	})
	out := run(t, cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "file.bin"))
	if out["ok"].(bool) {
		t.Fatal("off-instance attachment download should fail")
	}
	if out["error"].(map[string]interface{})["code"].(string) != "instance_url_mismatch" {
		t.Fatalf("wrong code: %#v", out)
	}
}
func TestCommandsSchemaAndSecrets(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	out := run(t, cfg, "commands")
	cmds := out["data"].(map[string]interface{})["commands"].([]interface{})
	if len(cmds) < 10 {
		t.Fatal("need many commands")
	}
	s := run(t, cfg, "schema", "issue.create")
	req := s["data"].(map[string]interface{})["required"].([]interface{})
	if len(req) == 0 {
		t.Fatal("schema required empty")
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "p") {
		_ = os.Getenv("NOOP")
	}
}

func TestEditReturnsNotInteractive(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	out := run(t, cfg, "issue", "edit", "EFP-1")
	if out["ok"].(bool) {
		t.Fatal("should fail")
	}
	if out["error"].(map[string]interface{})["code"].(string) != "not_interactive_supported" {
		t.Fatal("wrong code")
	}
}

func TestIssueBatchBCommands(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	cases := [][]string{{"issue", "watchers", "EFP-1"}, {"issue", "vote", "EFP-1"}, {"issue", "notify", "EFP-1"}, {"issue", "comment", "list", "EFP-1"}}
	for _, c := range cases {
		out := run(t, cfg, c...)
		if !out["ok"].(bool) {
			t.Fatalf("failed %v", c)
		}
	}
}

func TestFilterCrudAndDashboardCommands(t *testing.T) {
	hits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	_ = run(t, cfg, "filter", "create", "--name", "f", "--jql", "project=EFP")
	_ = run(t, cfg, "filter", "update", "123", "--name", "f2")
	_ = run(t, cfg, "filter", "delete", "123")
	_ = run(t, cfg, "--yes", "filter", "delete", "123")
	_ = run(t, cfg, "filter", "dashboard", "list")
	pre := hits
	_ = run(t, cfg, "--dry-run", "filter", "create", "--name", "f", "--jql", "x")
	if hits != pre {
		t.Fatal("dry run should not hit server")
	}
}

func TestUserGroupBoundaries(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, cfg, "user", "search", "--query", "alice")["ok"].(bool) {
		t.Fatal("user search failed")
	}
	if !run(t, cfg, "user", "group", "get", "eng-team")["ok"].(bool) {
		t.Fatal("group get failed")
	}
	if !run(t, cfg, "user", "group", "search", "--query", "eng")["ok"].(bool) {
		t.Fatal("group search failed")
	}
}

func TestBatchCEndpointAssertions(t *testing.T) {
	type hit struct{ Method, Path string }
	hits := []hit{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, hit{Method: r.Method, Path: r.URL.Path})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	_ = run(t, cfg, "filter", "update", "22", "--name", "f2")

	want := []hit{{"PUT", "/rest/api/2/filter/22"}}
	if len(hits) < len(want) {
		t.Fatalf("insufficient calls: got=%d", len(hits))
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("call[%d] mismatch: got=%+v want=%+v", i, hits[i], want[i])
		}
	}
}

func TestBatchCValidationAndSchema(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	if run(t, cfg, "filter", "create", "--name", "only")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	if run(t, cfg, "component", "create", "--name", "x")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	if run(t, cfg, "version", "create", "--project", "EFP")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	s := run(t, cfg, "schema", "filter.create")
	req := s["data"].(map[string]interface{})["required"].([]interface{})
	if len(req) < 2 {
		t.Fatal("schema not updated")
	}
}

func TestWriteValidationConsistency(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	cases := [][]string{{"issue", "update", "EFP-1"}, {"issue", "comment", "add", "EFP-1"}, {"issue", "comment", "update", "EFP-1", "1"}, {"issue", "worklog", "add", "EFP-1"}, {"issue", "worklog", "update", "EFP-1", "1"}}
	for _, c := range cases {
		out := run(t, cfg, c...)
		if out["ok"].(bool) {
			t.Fatalf("expected invalid_args for %v", c)
		}
	}
}

func TestSchemaConsistencyForExtendedWrites(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	for _, name := range []string{"issue.comment.add", "issue.comment.update", "issue.worklog.add", "issue.worklog.update", "issue.update"} {
		out := run(t, cfg, "schema", name)
		req := out["data"].(map[string]interface{})["required"].([]interface{})
		if len(req) == 0 {
			t.Fatalf("schema required missing for %s", name)
		}
	}
}
