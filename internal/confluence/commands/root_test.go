package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestHelpIsAnnotatedForVisibleCommands(t *testing.T) {
	cmd := NewRoot()
	assertHelpAnnotated(t, cmd)
	help := runText(t, "", "page", "update", "--help")
	for _, want := range []string{"Update a Confluence page", "--dry-run", "Confluence page id"} {
		if !strings.Contains(help, want) {
			t.Fatalf("page update help missing %q\n%s", want, help)
		}
	}
}

func runText(t *testing.T, cfg string, args ...string) string {
	t.Helper()
	c := NewRoot()
	b := &bytes.Buffer{}
	c.SetOut(b)
	c.SetErr(b)
	fullArgs := args
	if cfg != "" {
		fullArgs = append([]string{"--config", cfg}, args...)
	}
	c.SetArgs(fullArgs)
	if err := c.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	return b.String()
}

func assertHelpAnnotated(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	if !cmd.Hidden {
		if strings.TrimSpace(cmd.Short) == "" {
			t.Fatalf("%s missing Short", cmd.CommandPath())
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Fatalf("%s missing Long", cmd.CommandPath())
		}
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if strings.TrimSpace(f.Usage) == "" {
				t.Fatalf("%s persistent flag --%s missing usage", cmd.CommandPath(), f.Name)
			}
		})
	}
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		assertHelpAnnotated(t, child)
	}
}
