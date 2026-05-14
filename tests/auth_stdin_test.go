package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	ccmd "engineering-flow-platform-tools/internal/confluence/commands"
	jcmd "engineering-flow-platform-tools/internal/jira/commands"
)

func baseAuthConfig(t *testing.T) string {
	t.Helper()
	v := true
	cfg := config.RootConfig{Version: 1,
		Jira:       config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{{Name: "local", BaseURL: "https://jira.example", RESTPath: "/rest/api/2", VerifySSL: &v}}},
		Confluence: config.ProductConfig{DefaultInstance: "local", Instances: []config.InstanceConfig{{Name: "local", BaseURL: "https://conf.example", RESTPath: "/rest/api", VerifySSL: &v}}},
	}
	p := filepath.Join(t.TempDir(), "config.json")
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAuthLoginStdinSecrets(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		secret   string
		validate func(config.AuthConfig) bool
	}{
		{"api-key", []string{"auth", "login", "--username", "u", "--auth-type", "basic_api_key", "--api-key-stdin", "--json"}, "api-secret", func(a config.AuthConfig) bool { return a.APIKey == "api-secret" && a.Password == "" && a.Token == "" }},
		{"password", []string{"auth", "login", "--username", "u", "--auth-type", "basic_password", "--password-stdin", "--json"}, "password-secret", func(a config.AuthConfig) bool {
			return a.Password == "password-secret" && a.APIKey == "" && a.Token == ""
		}},
		{"token", []string{"auth", "login", "--auth-type", "bearer_token", "--token-stdin", "--json"}, "token-secret", func(a config.AuthConfig) bool {
			return a.Token == "token-secret" && a.Username == "" && a.Password == "" && a.APIKey == ""
		}},
	}
	for _, tc := range cases {
		t.Run("jira-"+tc.name, func(t *testing.T) {
			path := baseAuthConfig(t)
			var out bytes.Buffer
			cmd := jcmd.NewRoot()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(tc.secret + "\n"))
			cmd.SetArgs(append([]string{"--config", path}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), tc.secret) {
				t.Fatalf("secret leaked: %s", out.String())
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.validate(cfg.Jira.Instances[0].Auth) {
				t.Fatalf("bad jira auth: %+v", cfg.Jira.Instances[0].Auth)
			}
		})
		t.Run("confluence-"+tc.name, func(t *testing.T) {
			path := baseAuthConfig(t)
			var out bytes.Buffer
			cmd := ccmd.NewRoot()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(tc.secret + "\n"))
			cmd.SetArgs(append([]string{"--config", path}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), tc.secret) {
				t.Fatalf("secret leaked: %s", out.String())
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.validate(cfg.Confluence.Instances[0].Auth) {
				t.Fatalf("bad confluence auth: %+v", cfg.Confluence.Instances[0].Auth)
			}
		})
	}
}
