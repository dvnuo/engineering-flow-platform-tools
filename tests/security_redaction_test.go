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

	"engineering-flow-platform-tools/internal/config"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func assertNoSecrets(t *testing.T, s string) {
	t.Helper()
	for _, sec := range testutil.Secrets {
		if strings.Contains(s, sec) {
			t.Fatalf("secret leaked: %s", sec)
		}
	}
}

func TestNoSecretsInStdoutStderr(t *testing.T) {
	var b bytes.Buffer
	j := jcmd.NewRoot()
	j.SetOut(&b)
	j.SetErr(&b)
	j.SetArgs([]string{"auth", "test", "--json"})
	_ = j.Execute()
	assertNoSecrets(t, b.String())
	b.Reset()
	c := ccmd.NewRoot()
	c.SetOut(&b)
	c.SetErr(&b)
	c.SetArgs([]string{"auth", "test", "--json"})
	_ = c.Execute()
	assertNoSecrets(t, b.String())
}

func secretConfig(t *testing.T, base string) string {
	t.Helper()
	v := true
	cfg := config.RootConfig{Version: 1,
		Jira: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
			{Name: "local", BaseURL: base, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "secret-password-should-not-appear"}},
			{Name: "api", BaseURL: base + "/api", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "secret-api-key-should-not-appear"}},
			{Name: "token", BaseURL: base + "/token", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "secret-token-should-not-appear"}},
		}},
		Confluence: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
			{Name: "local", BaseURL: base, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "secret-password-should-not-appear"}},
			{Name: "api", BaseURL: base + "/api", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "secret-api-key-should-not-appear"}},
			{Name: "token", BaseURL: base + "/token", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "secret-token-should-not-appear"}},
		}},
	}
	p := filepath.Join(t.TempDir(), "config.json")
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSecretsRedactedAcrossSuccessFailureVerboseAndDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	cfg := secretConfig(t, srv.URL)
	cases := []struct {
		name string
		run  func(*bytes.Buffer)
	}{
		{"jira instance list", func(b *bytes.Buffer) {
			c := jcmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "instance", "list", "--json"})
			_ = c.Execute()
		}},
		{"jira instance get", func(b *bytes.Buffer) {
			c := jcmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "instance", "get", "local", "--json"})
			_ = c.Execute()
		}},
		{"jira auth test verbose", func(b *bytes.Buffer) {
			c := jcmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "--verbose", "auth", "test", "--json"})
			_ = c.Execute()
		}},
		{"jira api off instance", func(b *bytes.Buffer) {
			c := jcmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "api", "get", "https://evil.example/x", "--json"})
			_ = c.Execute()
		}},
		{"jira dry run redacts token key", func(b *bytes.Buffer) {
			c := jcmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "api", "post", "/rest/api/2/issue", "--body", `{"token":"secret-token-should-not-appear"}`, "--dry-run", "--json"})
			_ = c.Execute()
		}},
		{"confluence instance list", func(b *bytes.Buffer) {
			c := ccmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "instance", "list", "--json"})
			_ = c.Execute()
		}},
		{"confluence auth test verbose", func(b *bytes.Buffer) {
			c := ccmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "--verbose", "auth", "test", "--json"})
			_ = c.Execute()
		}},
		{"confluence api off instance", func(b *bytes.Buffer) {
			c := ccmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "api", "get", "https://evil.example/x", "--json"})
			_ = c.Execute()
		}},
		{"confluence dry run redacts token key", func(b *bytes.Buffer) {
			c := ccmd.NewRoot()
			c.SetOut(b)
			c.SetErr(b)
			c.SetArgs([]string{"--config", cfg, "api", "post", "/rest/api/content", "--body", `{"token":"secret-token-should-not-appear"}`, "--dry-run", "--json"})
			_ = c.Execute()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			tc.run(&b)
			assertNoSecrets(t, b.String())
			if strings.Contains(strings.ToLower(b.String()), "authorization") {
				t.Fatalf("authorization header leaked: %s", b.String())
			}
		})
	}
}
