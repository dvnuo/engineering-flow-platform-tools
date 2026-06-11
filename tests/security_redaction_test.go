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
	kcmd "engineering-flow-platform-tools/internal/jenkins/commands"
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
		Jenkins: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{
			{Name: "local", BaseURL: base, VerifySSL: &v, CrumbMode: "auto", Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "secret-password-should-not-appear"}},
			{Name: "api", BaseURL: base + "/api", VerifySSL: &v, CrumbMode: "auto", Auth: config.AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "secret-api-key-should-not-appear"}},
			{Name: "token", BaseURL: base + "/token", VerifySSL: &v, CrumbMode: "auto", Auth: config.AuthConfig{Type: "bearer_token", Token: "secret-token-should-not-appear"}},
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

func TestToolOutputRedactsUpstreamSuccessPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "Ada",
			"token": "secret-token-should-not-appear",
			"nested": {"api_key": "secret-api-key-should-not-appear"},
			"profile_url": "https://example.test/callback?access_token=secret-token-should-not-appear&ok=1#frag",
			"message": "Authorization: Bearer secret-password-should-not-appear"
		}`))
	}))
	defer srv.Close()

	cfg := secretConfig(t, srv.URL)
	var b bytes.Buffer
	c := jcmd.NewRoot()
	c.SetOut(&b)
	c.SetErr(&b)
	c.SetArgs([]string{"--config", cfg, "myself", "--json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("command failed: %v output=%s", err, b.String())
	}
	assertNoSecrets(t, b.String())
	if !strings.Contains(b.String(), "***REDACTED***") || !strings.Contains(b.String(), "ok=1") {
		t.Fatalf("expected redacted output with safe URL query retained: %s", b.String())
	}
}

func TestJenkinsOutputRedactsUpstreamLogsAndArtifactMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/json", "/whoAmI/api/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"mode": "NORMAL",
				"temporaryCredentials": "secret-password-should-not-appear",
				"authorizationHeader": "Bearer secret-token-should-not-appear",
				"headers": {"Set-Cookie": "sid=secret-api-key-should-not-appear"}
			}`))
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"secret-token-should-not-appear"}`))
		case "/job/app/1/consoleText", "/job/app/1/execution/node/n1/wfapi/log":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("console Authorization: Bearer secret-token-should-not-appear temporaryCredentials=secret-password-should-not-appear"))
		case "/job/app/1/artifact/secret.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("artifact body contains secret-token-should-not-appear"))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	cfg := secretConfig(t, srv.URL)
	artifactOut := filepath.Join(t.TempDir(), "artifact.txt")
	cases := []struct {
		name string
		args []string
	}{
		{"server info", []string{"--config", cfg, "server-info", "--json"}},
		{"auth verbose", []string{"--config", cfg, "--verbose", "auth", "test", "--json"}},
		{"crumb", []string{"--config", cfg, "crumb", "get", "--json"}},
		{"build log", []string{"--config", cfg, "build", "log", "app", "1", "--json"}},
		{"pipeline node log", []string{"--config", cfg, "pipeline", "node-log", "app", "1", "n1", "--json"}},
		{"dry run body", []string{"--config", cfg, "--dry-run", "api", "post", "/api/json", "--body", `{"temporaryCredentials":"secret-password-should-not-appear","authorizationHeader":"Bearer secret-token-should-not-appear"}`, "--json"}},
		{"artifact metadata", []string{"--config", cfg, "artifact", "download", "app", "1", "secret.txt", "--output", artifactOut, "--json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			c := kcmd.NewRoot()
			c.SetOut(&b)
			c.SetErr(&b)
			c.SetArgs(tc.args)
			if err := c.Execute(); err != nil {
				t.Fatalf("command failed: %v output=%s", err, b.String())
			}
			assertNoSecrets(t, b.String())
			if strings.Contains(b.String(), "Bearer secret") || strings.Contains(b.String(), "temporaryCredentials=secret") {
				t.Fatalf("credential-bearing text leaked: %s", b.String())
			}
		})
	}
}
