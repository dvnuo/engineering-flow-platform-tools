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
