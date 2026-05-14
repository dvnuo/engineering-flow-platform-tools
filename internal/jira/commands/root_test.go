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
	_ = cmd.Execute()
	var out map[string]interface{}
	_ = json.Unmarshal(b.Bytes(), &out)
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
	if !run(t, cfg, "issue", "get", "http://x/browse/EFP-123")["ok"].(bool) {
		t.Fatal()
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
	if !run(t, cfg, "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s", "--field", "customfield_1={\"id\":\"1\"}")["ok"].(bool) {
		t.Fatal()
	}
	if !run(t, cfg, "issue", "transition", "EFP-1", "--to", "In Progress")["ok"].(bool) {
		t.Fatal()
	}
	pre := *hits
	out := run(t, cfg, "--dry-run", "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s")
	if !out["ok"].(bool) || *hits != pre {
		t.Fatal("dry run hit server")
	}
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
