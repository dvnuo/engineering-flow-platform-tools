package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"github.com/spf13/cobra"
)

func TestPageUpdateFetchesVersionWithExpand(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/rest/api/content/123" {
			if r.URL.Query().Get("expand") != "version" {
				t.Fatalf("missing expand=version: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"version":{"number":2}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, p, "page", "update", "--id", "123", "--title", "Next")["ok"].(bool) {
		t.Fatal("page update failed")
	}
}

func TestPageExportMissingBodyDoesNotPanic(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	r := run(t, p, "page", "export-markdown", "--id", "123")
	if ok, _ := r["ok"].(bool); ok {
		t.Fatal("missing body should fail")
	}
	errObj, _ := r["error"].(map[string]any)
	if errObj["code"] != "not_found" {
		t.Fatalf("code=%v", errObj["code"])
	}
}

func TestDisplayURLResolvesPageIDByTitle(t *testing.T) {
	var sawTitleLookup bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/content" {
			sawTitleLookup = true
			if r.URL.Query().Get("spaceKey") != "ENG" || r.URL.Query().Get("title") != "Runtime Profile" || r.URL.Query().Get("type") != "page" {
				t.Fatalf("bad title query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"123"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer s.Close()
	v := true
	p := writeConfluenceConfig(t, config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
		{Name: "local", BaseURL: s.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
	}})
	out := runWithArgs(t, NewRoot(), "--config", p, "--json", "page", "get", "--url", s.URL+"/display/ENG/Runtime+Profile")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("page get failed: %#v", out)
	}
	if !sawTitleLookup {
		t.Fatal("expected title lookup")
	}
}

func TestBodyFileErrors(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":{"number":1}}`))
	})
	missing := filepath.Join(t.TempDir(), "missing.html")
	for _, args := range [][]string{
		{"page", "create", "--space", "ENG", "--title", "T", "--body-file", missing},
		{"page", "update", "--id", "123", "--version", "2", "--body-file", missing},
		{"page", "comment", "add", "--id", "123", "--body-file", missing},
	} {
		r := run(t, p, args...)
		if ok, _ := r["ok"].(bool); ok {
			t.Fatalf("expected invalid_args for %v", args)
		}
		errObj, _ := r["error"].(map[string]any)
		if errObj["code"] != "invalid_args" {
			t.Fatalf("code=%v for %v", errObj["code"], args)
		}
	}
}

func TestConfluenceResolverErrorCodes(t *testing.T) {
	v := true
	noDefault := writeConfluenceConfig(t, config.ProductConfig{Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: "https://a.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		{Name: "b", BaseURL: "https://b.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
	}})
	assertConfluenceCode(t, noDefault, []string{"search", "--cql", "space = ENG"}, "instance_required")

	empty := writeConfluenceConfig(t, config.ProductConfig{})
	assertConfluenceCode(t, empty, []string{"search", "--cql", "space = ENG"}, "no_instance_configured")

	mismatch := writeConfluenceConfig(t, config.ProductConfig{DefaultInstance: "a", Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: "https://a.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		{Name: "b", BaseURL: "https://b.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
	}})
	assertConfluenceCode(t, mismatch, []string{"--instance", "a", "page", "get", "--url", "https://b.example/pages/viewpage.action?pageId=1"}, "instance_url_mismatch")

	ambiguous := writeConfluenceConfig(t, config.ProductConfig{Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: "https://same.example/wiki", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		{Name: "b", BaseURL: "https://same.example/wiki", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
	}})
	assertConfluenceCode(t, ambiguous, []string{"page", "get", "--url", "https://same.example/wiki/pages/viewpage.action?pageId=1"}, "ambiguous_instance")
}

func runWithArgs(t *testing.T, c *cobra.Command, args ...string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	c.SetOut(&b)
	c.SetErr(&b)
	c.SetArgs(args)
	_ = c.Execute()
	out := map[string]any{}
	_ = json.Unmarshal(b.Bytes(), &out)
	return out
}

func writeConfluenceConfig(t *testing.T, product config.ProductConfig) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	b, err := json.Marshal(config.RootConfig{Version: 1, Confluence: product})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertConfluenceCode(t *testing.T, cfg string, args []string, code string) {
	t.Helper()
	full := append([]string{"--config", cfg, "--json"}, args...)
	out := runWithArgs(t, NewRoot(), full...)
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected %s for %v, got ok", code, args)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != code {
		t.Fatalf("code=%v want=%s out=%#v", errObj["code"], code, out)
	}
}
