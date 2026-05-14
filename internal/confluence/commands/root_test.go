package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestPropertyAttachmentCommentFlows(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/content/9" && r.Method == "GET" {
			_, _ = w.Write([]byte(`{"version":{"number":7},"_links":{"download":"https://evil.example/file.bin"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	r1 := run(t, p, "page", "property", "list", "--id", "9")
	if ok, _ := r1["ok"].(bool); !ok {
		t.Fatal("page property list")
	}
	r2 := run(t, p, "space", "property", "list", "ENG")
	if ok, _ := r2["ok"].(bool); !ok {
		t.Fatal("space property list")
	}
	r3 := run(t, p, "attachment", "download", "9", "--output", "/tmp/a.bin")
	if ok, _ := r3["ok"].(bool); ok {
		t.Fatal("off-instance download should fail")
	}
	r4 := run(t, p, "comment", "update", "9", "--body", "<p>x</p>")
	if ok, _ := r4["ok"].(bool); !ok {
		t.Fatal("comment update")
	}
}

func TestAttachmentUploadMultipartHeader(t *testing.T) {
	gotHeader := ""
	gotCT := ""
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Atlassian-Token")
		gotCT = r.Header.Get("Content-Type")
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	tmp := filepath.Join(t.TempDir(), "a.txt")
	_ = os.WriteFile(tmp, []byte("hello"), 0644)
	r := run(t, p, "attachment", "upload", "9", tmp)
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatal("upload failed")
	}
	if gotHeader != "no-check" {
		t.Fatal("missing x-atlassian-token")
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Fatal("not multipart")
	}
}

func TestExportMarkdownAndRegistry(t *testing.T) {
	p := cfg(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"body":{"view":{"value":"<p>Hello</p>"}}}`))
	})
	r := run(t, p, "page", "export-markdown", "--id", "1")
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatal("export markdown failed")
	}
	data := r["data"].(map[string]any)
	if data["markdown"] == "" {
		t.Fatal("markdown missing")
	}
	cmds := run(t, p, "commands")["data"].(map[string]any)["commands"].([]any)
	if len(cmds) < 50 {
		t.Fatal("commands list too short")
	}
	s := run(t, p, "schema", "page.create")
	req := s["data"].(map[string]any)["required"].([]any)
	if len(req) == 0 {
		t.Fatal("schema required missing")
	}
}
