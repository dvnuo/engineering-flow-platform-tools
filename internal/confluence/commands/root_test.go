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
func TestSearchAndTitleAndDryRun(t *testing.T) {
	calls := 0
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/rest/api/search" && r.URL.Query().Get("cql") == "space = ENG" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if r.URL.Path == "/rest/api/content" && r.URL.Query().Get("type") == "page" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if r.URL.Path == "/rest/api/user/current" {
			if r.Header.Get("Authorization") == "" {
				t.Fatal("missing auth")
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"version":{"number":2},"body":{"view":{"value":"<p>Hello</p>"}}}`))
	})
	if !run(t, p, "auth", "test")["ok"].(bool) {
		t.Fatal("auth")
	}
	if !run(t, p, "search", "--cql", "space = ENG")["ok"].(bool) {
		t.Fatal("search")
	}
	if !run(t, p, "page", "get-by-title", "--space", "ENG", "--title", "Runtime Profile")["ok"].(bool) {
		t.Fatal("gbt")
	}
	r := run(t, p, "--dry-run", "page", "create", "--space", "ENG", "--title", "T", "--body", "<p>Hello</p>")
	if !r["ok"].(bool) {
		t.Fatal("dry")
	}
	if calls != 3 {
		t.Fatal("dry-run should not hit server")
	}
}
