package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func cfg(t *testing.T, h http.HandlerFunc) string {
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	v := true
	c := config.RootConfig{Version: 1, Jira: config.ProductConfig{DefaultInstance: "c", Instances: []config.InstanceConfig{{Name: "c", BaseURL: s.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}}}}}
	p := filepath.Join(t.TempDir(), "c.json")
	_ = config.Save(p, c)
	return p
}
func run(t *testing.T, cfg string, args ...string) map[string]any {
	c := NewRoot()
	b := &bytes.Buffer{}
	c.SetOut(b)
	c.SetErr(b)
	c.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	_ = c.Execute()
	out := map[string]any{}
	_ = json.Unmarshal(b.Bytes(), &out)
	return out
}

func TestValidationAndSchema(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) })
	if run(t, p, "page", "create", "--title", "x")["ok"].(bool) {
		t.Fatal("expected invalid")
	}
	if run(t, p, "content", "update", "1")["ok"].(bool) {
		t.Fatal("expected invalid")
	}
	s := run(t, p, "schema", "page.create")
	if len(s["data"].(map[string]any)["required"].([]any)) == 0 {
		t.Fatal("schema required missing")
	}
}

func TestEndpointMethods(t *testing.T) {
	calls := []string{}
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Write([]byte(`{"ok":true}`))
	})
	_ = run(t, p, "content", "create", "--title", "a", "--body", "b")
	_ = run(t, p, "content", "update", "1", "--title", "c")
	_ = run(t, p, "--yes", "content", "delete", "1")
	if len(calls) < 3 {
		t.Fatal("calls missing")
	}
}
