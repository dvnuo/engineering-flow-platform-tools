package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
	"engineering-flow-platform-tools/internal/testutil"
)

func TestResolveURLJSON(t *testing.T) {
	s := testutil.NewMockJira()
	defer s.Close()
	cfg, _ := testutil.WriteConfig(testutil.JiraConfig(s.URL))
	{
		var b bytes.Buffer
		c := jcmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfg, "resolve-url", s.URL + "/browse/EFP-123", "--json"})
		_ = c.Execute()
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
	{
		var b bytes.Buffer
		c := ccmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfg, "resolve-url", s.URL + "/wiki/spaces/ENG/pages/1", "--json"})
		_ = c.Execute()
		testutil.AssertJSONEnvelope(t, b.Bytes())
	}
}

func TestConfluencePageURLAutoSelectsInstance(t *testing.T) {
	v := true
	var hitB bool
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected instance a request: %s", r.URL.String())
	}))
	defer a.Close()
	bsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitB = true
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer bsrv.Close()
	cfgPath := writeJSONConfig(t, config.RootConfig{Version: 1,
		Confluence: config.ProductConfig{DefaultInstance: "a", Instances: []config.InstanceConfig{
			{Name: "a", BaseURL: a.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
			{Name: "b", BaseURL: bsrv.URL, RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		}},
	})
	var out bytes.Buffer
	c := ccmd.NewRoot()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{"--config", cfgPath, "page", "get", "--url", bsrv.URL + "/pages/viewpage.action?pageId=123", "--json"})
	_ = c.Execute()
	testutil.AssertOKEnvelope(t, out.Bytes())
	if !hitB {
		t.Fatal("expected URL-matched instance")
	}
}

func writeJSONConfig(t *testing.T, cfg config.RootConfig) string {
	t.Helper()
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

func TestExplicitInstanceURLMismatch(t *testing.T) {
	v := true
	cfgPath := writeJSONConfig(t, config.RootConfig{Version: 1,
		Jira: config.ProductConfig{DefaultInstance: "jira-a", Instances: []config.InstanceConfig{
			{Name: "jira-a", BaseURL: "https://jira-a.example", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
			{Name: "jira-b", BaseURL: "https://jira-b.example", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		}},
		Confluence: config.ProductConfig{DefaultInstance: "conf-a", Instances: []config.InstanceConfig{
			{Name: "conf-a", BaseURL: "https://conf-a.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
			{Name: "conf-b", BaseURL: "https://conf-b.example", RESTPath: "/rest/api", VerifySSL: &v, Auth: config.AuthConfig{Type: "bearer_token", Token: "t"}},
		}},
	})
	{
		var b bytes.Buffer
		c := jcmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfgPath, "--instance", "jira-a", "issue", "get", "https://jira-b.example/browse/EFP-1", "--json"})
		_ = c.Execute()
		obj := testutil.AssertJSONEnvelope(t, b.Bytes())
		if obj["ok"].(bool) || obj["error"].(map[string]any)["code"] != "instance_url_mismatch" {
			t.Fatalf("expected jira instance_url_mismatch: %s", b.String())
		}
	}
	{
		var b bytes.Buffer
		c := ccmd.NewRoot()
		c.SetOut(&b)
		c.SetArgs([]string{"--config", cfgPath, "--instance", "conf-a", "page", "get", "--url", "https://conf-b.example/pages/viewpage.action?pageId=1", "--json"})
		_ = c.Execute()
		obj := testutil.AssertJSONEnvelope(t, b.Bytes())
		if obj["ok"].(bool) || obj["error"].(map[string]any)["code"] != "instance_url_mismatch" {
			t.Fatalf("expected confluence instance_url_mismatch: %s", b.String())
		}
	}
}

func TestRawAPIOffInstanceURLMismatch(t *testing.T) {
	s := testutil.NewMockJira()
	defer s.Close()
	cfg, _ := testutil.WriteConfig(testutil.JiraConfig(s.URL))
	{
		var b bytes.Buffer
		c := jcmd.NewRoot()
		c.SetOut(&b)
		c.SetErr(&b)
		c.SetArgs([]string{"--config", cfg, "api", "get", "https://evil.example/x", "--json"})
		_ = c.Execute()
		testutil.AssertErrorCode(t, b.Bytes(), "instance_url_mismatch")
	}
	{
		var b bytes.Buffer
		c := ccmd.NewRoot()
		c.SetOut(&b)
		c.SetErr(&b)
		c.SetArgs([]string{"--config", cfg, "api", "get", "https://evil.example/x", "--json"})
		_ = c.Execute()
		testutil.AssertErrorCode(t, b.Bytes(), "instance_url_mismatch")
	}
}
