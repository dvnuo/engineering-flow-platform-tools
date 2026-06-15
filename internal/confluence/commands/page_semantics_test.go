package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestPageUpdateFetchesVersionWithExpand(t *testing.T) {
	var sawPut bool
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/rest/api/content/123" {
			if r.URL.Query().Get("expand") != "version" {
				t.Fatalf("missing expand=version: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"version":{"number":2}}`))
			return
		}
		if r.Method == "PUT" && r.URL.Path == "/rest/api/content/123" {
			sawPut = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			if payload["type"] != "page" {
				t.Fatalf("update payload type=%v want page: %#v", payload["type"], payload)
			}
			version, _ := payload["version"].(map[string]any)
			if version["number"] != float64(3) {
				t.Fatalf("update payload version=%v want 3: %#v", version["number"], payload)
			}
			if payload["title"] != "Next" {
				t.Fatalf("update payload title=%v want Next: %#v", payload["title"], payload)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, p, "page", "update", "--id", "123", "--title", "Next")["ok"].(bool) {
		t.Fatal("page update failed")
	}
	if !sawPut {
		t.Fatal("page update did not send PUT")
	}
}

func TestContentAndBlogUpdateIncludeType(t *testing.T) {
	seen := map[string]map[string]any{}
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && (r.URL.Path == "/rest/api/content/123" || r.URL.Path == "/rest/api/content/456") {
			_, _ = w.Write([]byte(`{"version":{"number":2}}`))
			return
		}
		if r.Method == "PUT" && (r.URL.Path == "/rest/api/content/123" || r.URL.Path == "/rest/api/content/456") {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			seen[r.URL.Path] = payload
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	})
	if out := run(t, p, "content", "update", "123", "--title", "Home", "--body", "<p>Hello</p>"); out["ok"] != true {
		t.Fatalf("content update failed: %#v", out)
	}
	if out := run(t, p, "blog", "update", "456", "--title", "News", "--body", "<p>Hello</p>"); out["ok"] != true {
		t.Fatalf("blog update failed: %#v", out)
	}
	if got := seen["/rest/api/content/123"]["type"]; got != "page" {
		t.Fatalf("content update type=%v want page: %#v", got, seen["/rest/api/content/123"])
	}
	if got := seen["/rest/api/content/456"]["type"]; got != "blogpost" {
		t.Fatalf("blog update type=%v want blogpost: %#v", got, seen["/rest/api/content/456"])
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

func TestPageURLRoutingAndNoStatePollution(t *testing.T) {
	var hitsA, hitsB int
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsA++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/content" && r.URL.Query().Get("title") == "Runtime Profile":
			_, _ = w.Write([]byte(`{"results":[{"id":"321"}]}`))
		case r.URL.Path == "/rest/api/content/321" && r.URL.Query().Get("expand") == "body.view":
			_, _ = w.Write([]byte(`{"body":{"view":{"value":"<p>Runtime Profile</p>"}}}`))
		default:
			_, _ = w.Write([]byte(`{"id":"123","version":{"number":2},"body":{"storage":{"value":"<p>A</p>"}}}`))
		}
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsB++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123","version":{"number":2},"body":{"storage":{"value":"<p>B</p>"}}}`))
	}))
	defer b.Close()
	v := true
	p := writeConfluenceConfig(t, config.ProductConfig{DefaultInstance: "b", Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: a.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		{Name: "b", BaseURL: b.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
	}})
	root := NewRoot()
	if out := runWithArgs(t, root, "--config", p, "--json", "page", "get", "--url", a.URL+"/spaces/ENG/pages/123/Title"); out["ok"] != true {
		t.Fatalf("space URL get failed: %#v", out)
	}
	if out := runWithArgs(t, root, "--config", p, "--json", "page", "export-markdown", "--url", a.URL+"/display/ENG/Runtime+Profile"); out["ok"] != true {
		t.Fatalf("display URL export failed: %#v", out)
	}
	if out := runWithArgs(t, root, "--config", p, "--json", "--instance", "b", "page", "get", "--id", "123"); out["ok"] != true {
		t.Fatalf("id get after URL should use explicit instance: %#v", out)
	}
	if hitsA == 0 || hitsB == 0 {
		t.Fatalf("expected both instances to be used: hitsA=%d hitsB=%d", hitsA, hitsB)
	}
}

func TestPageDisplayLookupErrorsAndOutputWrites(t *testing.T) {
	malformed := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":"bad"}`))
	})
	assertConfluenceCode(t, malformed, []string{"page", "get", "--url", confluenceBaseURLFromConfig(t, malformed) + "/display/ENG/Missing"}, "server_error")

	notFound := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	srvURL := confluenceBaseURLFromConfig(t, notFound)
	assertConfluenceCode(t, notFound, []string{"page", "get", "--url", srvURL + "/display/ENG/Missing"}, "not_found")

	okCfg := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"body":{"view":{"value":"<p>Hello</p>"}}}`))
	})
	missingDir := filepath.Join(t.TempDir(), "missing", "out.md")
	r := run(t, okCfg, "page", "export-markdown", "--id", "123", "--output", missingDir)
	if ok, _ := r["ok"].(bool); ok {
		t.Fatalf("expected failed output write: %#v", r)
	}
	errObj, _ := r["error"].(map[string]any)
	if errObj["code"] != "invalid_args" || !strings.Contains(errObj["message"].(string), "failed to write --output") {
		t.Fatalf("unexpected output write error: %#v", r)
	}
	outFile := filepath.Join(t.TempDir(), "out.md")
	if r = run(t, okCfg, "page", "export-markdown", "--id", "123", "--output", outFile); r["ok"] != true {
		t.Fatalf("expected output write success: %#v", r)
	}
}

func TestPageGetByTitleRequiredArgs(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("spaceKey") != "ENG" || r.URL.Query().Get("title") != "Runtime Profile" || r.URL.Query().Get("type") != "page" {
			t.Fatalf("bad get-by-title query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":"123"}]}`))
	})
	assertConfluenceCode(t, p, []string{"page", "get-by-title", "--title", "Runtime Profile"}, "invalid_args")
	assertConfluenceCode(t, p, []string{"page", "get-by-title", "--space", "ENG"}, "invalid_args")
	if out := run(t, p, "page", "get-by-title", "--space", "ENG", "--title", "Runtime Profile"); out["ok"] != true {
		t.Fatalf("get-by-title failed: %#v", out)
	}
}

func TestConfluenceRequiredArgsDoNotHitServer(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid args should not hit server: %s %s", r.Method, r.URL.Path)
	})
	cases := [][]string{
		{"page", "label", "delete", "--id", "123", "--yes"},
		{"page", "property", "get", "--id", "123"},
		{"page", "property", "delete", "--id", "123", "--yes"},
		{"page", "property", "delete", "--id", "123", "--key", "status"},
		{"page", "restore", "--id", "123"},
		{"page", "restore", "--id", "123", "--version", "0"},
		{"page", "move", "--id", "123"},
		{"page", "restriction", "add", "--id", "123", "--operation", "delete", "--user", "alice"},
		{"page", "restriction", "add", "--id", "123", "--operation", "read"},
		{"page", "restriction", "delete", "--id", "123", "--operation", "delete", "--yes"},
		{"page", "restriction", "delete", "--id", "123", "--operation", "read"},
		{"webhook", "create", "--url", "https://example.test", "--event", "page_created"},
		{"webhook", "create", "--name", "hook", "--event", "page_created"},
		{"webhook", "create", "--name", "hook", "--url", "https://example.test"},
	}
	for _, args := range cases {
		assertConfluenceCode(t, p, args, "invalid_args")
	}
}

func TestConfluenceRawAPIAbsoluteURLRouting(t *testing.T) {
	var hitsA, hitsB int
	var authA string
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsA++
		authA = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"instance":"a"}`))
	}))
	defer a.Close()
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsB++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"instance":"b"}`))
	}))
	defer b.Close()
	v := true
	p := writeConfluenceConfig(t, config.ProductConfig{Instances: []config.InstanceConfig{
		{Name: "a", BaseURL: a.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "token-a"}},
		{Name: "b", BaseURL: b.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "token-b"}},
	}})
	out := runWithArgs(t, NewRoot(), "--config", p, "--json", "api", "get", a.URL+"/rest/api/user/current")
	if out["ok"] != true || hitsA != 1 || authA == "" {
		t.Fatalf("absolute URL did not route to instance a: out=%#v hitsA=%d authA=%q", out, hitsA, authA)
	}
	assertConfluenceCode(t, p, []string{"--instance", "a", "api", "get", b.URL + "/rest/api/content"}, "instance_url_mismatch")
	if hitsB != 0 {
		t.Fatalf("mismatched explicit instance should not hit instance b, hits=%d", hitsB)
	}
	assertConfluenceCode(t, p, []string{"api", "get", "https://evil.example/rest/api/content"}, "instance_url_mismatch")

	empty := writeConfluenceConfig(t, config.ProductConfig{})
	assertConfluenceCode(t, empty, []string{"api", "get", "https://evil.example/rest/api/content"}, "no_instance_configured")
}

func TestConfluencePageLookupStableHTTPErrorCodes(t *testing.T) {
	for status, code := range map[int]string{
		http.StatusUnauthorized: "auth_failed",
		http.StatusForbidden:    "permission_denied",
		http.StatusNotFound:     "not_found",
	} {
		t.Run(code, func(t *testing.T) {
			p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"failed"}`))
			})
			assertConfluenceCode(t, p, []string{"page", "get", "--url", confluenceBaseURLFromConfig(t, p) + "/display/ENG/Private"}, code)
		})
	}
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

func confluenceBaseURLFromConfig(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.RootConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg.Confluence.Instances[0].BaseURL
}
