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
	c := config.RootConfig{Version: 1, Confluence: config.ProductConfig{DefaultInstance: "c", Instances: []config.InstanceConfig{{Name: "c", BaseURL: s.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}}}}}
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

func TestCoreAndSafety(t *testing.T) {
	calls := 0
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/rest/api/user/current" && r.Header.Get("Authorization") == "" {
			t.Fatal("missing auth")
		}
		if r.URL.Path == "/rest/api/search" && r.URL.Query().Get("cql") == "" {
			t.Fatal("missing cql")
		}
		if r.Method == "GET" && r.URL.Path == "/rest/api/content/123" {
			_, _ = w.Write([]byte(`{"version":{"number":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"body":{"view":{"value":"<p>Hello</p>"}}}`))
	})
	if !run(t, p, "auth", "test")["ok"].(bool) {
		t.Fatal("auth")
	}
	if !run(t, p, "search", "--cql", "space = ENG")["ok"].(bool) {
		t.Fatal("search")
	}
	if run(t, p, "page", "delete", "--id", "1")["ok"].(bool) {
		t.Fatal("delete should require yes")
	}
	before := calls
	if !run(t, p, "--dry-run", "page", "create", "--space", "ENG", "--title", "T", "--body", "<p>x</p>")["ok"].(bool) {
		t.Fatal("dry")
	}
	if calls != before {
		t.Fatal("dry-run hit server")
	}
	if !run(t, p, "page", "update", "--id", "123", "--title", "N")["ok"].(bool) {
		t.Fatal("update")
	}
	if run(t, p, "api", "get", "https://evil.example/rest/api/content/1")["ok"].(bool) {
		t.Fatal("off instance should fail")
	}
}
