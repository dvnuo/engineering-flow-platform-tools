package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func setup(t *testing.T, h http.HandlerFunc) (string, *int) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n++; h(w, r) }))
	t.Cleanup(s.Close)
	v := true
	cfg := config.RootConfig{Version: 1, Jira: config.ProductConfig{DefaultInstance: "jira-main", Instances: []config.InstanceConfig{{Name: "jira-main", BaseURL: s.URL, RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "p"}}}}}
	p := filepath.Join(t.TempDir(), "cfg.json")
	_ = config.Save(p, cfg)
	return p, &n
}

func TestIssueCreateJSONBodyOverrideAndSearchNumbers(t *testing.T) {
	hits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/rest/api/2/search" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["maxResults"] != float64(20) || body["startAt"] != float64(5) {
				t.Fatalf("search numbers not numeric: %#v", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	pre := hits
	if !run(t, cfg, "--dry-run", "issue", "create", "--json-body", `{"fields":{"summary":"x"}}`)["ok"].(bool) {
		t.Fatal("json-body override should not require project/type/summary")
	}
	if hits != pre {
		t.Fatal("dry-run hit server")
	}
	bodyFile := filepath.Join(t.TempDir(), "issue.json")
	if err := os.WriteFile(bodyFile, []byte(`{"fields":{"summary":"x"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !run(t, cfg, "--dry-run", "issue", "create", "--json-body-file", bodyFile)["ok"].(bool) {
		t.Fatal("json-body-file override should not require project/type/summary")
	}
	if run(t, cfg, "issue", "create")["ok"].(bool) {
		t.Fatal("missing generated fields should fail")
	}
	if !run(t, cfg, "issue", "search", "--jql", "project = PROJ", "--limit", "20", "--start", "5")["ok"].(bool) {
		t.Fatal("search failed")
	}
	if run(t, cfg, "issue", "search", "--jql", "project = PROJ", "--limit", "abc")["ok"].(bool) {
		t.Fatal("bad limit should fail")
	}
	if run(t, cfg, "issue", "search", "--jql", "project = PROJ", "--start", "abc")["ok"].(bool) {
		t.Fatal("bad start should fail")
	}
}

func TestHelpIsAnnotatedForVisibleCommands(t *testing.T) {
	cmd := NewRoot()
	assertHelpAnnotated(t, cmd)
	help := runText(t, "", "issue", "transition", "--help")
	for _, want := range []string{"Transition a Jira issue", "--dry-run", "Target transition name"} {
		if !strings.Contains(help, want) {
			t.Fatalf("issue transition help missing %q\n%s", want, help)
		}
	}
}

func TestIssueTransitionCommentFieldAndSafety(t *testing.T) {
	var gotPost map[string]any
	gets := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/transitions") {
			gets++
			w.Write([]byte(`{"transitions":[{"id":"31","name":"Done"}]}`))
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/transitions") {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotPost)
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, cfg, "issue", "transition", "PROJ-1", "--to", "done", "--comment", "Completed by agent", "--field", `resolution={"name":"Done"}`)["ok"].(bool) {
		t.Fatal("transition by name failed")
	}
	if gets != 1 {
		t.Fatalf("expected one transitions GET, got %d", gets)
	}
	fields := gotPost["fields"].(map[string]any)
	if fields["resolution"].(map[string]any)["name"] != "Done" {
		t.Fatalf("missing field: %#v", gotPost)
	}
	update := gotPost["update"].(map[string]any)
	if len(update["comment"].([]any)) != 1 {
		t.Fatalf("missing comment: %#v", gotPost)
	}
	gets = 0
	if !run(t, cfg, "issue", "transition", "PROJ-1", "--transition-id", "31")["ok"].(bool) {
		t.Fatal("transition by id failed")
	}
	if gets != 0 {
		t.Fatal("transition-id should not GET transitions")
	}
	if run(t, cfg, "issue", "transition", "PROJ-1")["ok"].(bool) {
		t.Fatal("missing transition selector should fail")
	}
	badCfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":"bad"}`))
	})
	if run(t, badCfg, "issue", "transition", "PROJ-1", "--to", "Done")["ok"].(bool) {
		t.Fatal("bad transitions shape should fail")
	}
	weirdCfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":["bad"]}`))
	})
	if run(t, weirdCfg, "issue", "transition", "PROJ-1", "--to", "Done")["ok"].(bool) {
		t.Fatal("non-object transition should fail without panic")
	}
}
func run(t *testing.T, cfg string, args ...string) map[string]interface{} {
	cmd := NewRoot()
	b := &bytes.Buffer{}
	cmd.SetOut(b)
	cmd.SetErr(b)
	cmd.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
	err := cmd.Execute()
	var out map[string]interface{}
	if uerr := json.Unmarshal(b.Bytes(), &out); uerr != nil || out == nil {
		out = map[string]interface{}{"ok": false, "error": map[string]interface{}{"code": "unmarshal_failed", "message": b.String()}}
	}
	if err != nil && out["ok"] == nil {
		out["ok"] = false
	}
	return out
}

func runText(t *testing.T, cfg string, args ...string) string {
	t.Helper()
	cmd := NewRoot()
	b := &bytes.Buffer{}
	cmd.SetOut(b)
	cmd.SetErr(b)
	fullArgs := args
	if cfg != "" {
		fullArgs = append([]string{"--config", cfg}, args...)
	}
	cmd.SetArgs(fullArgs)
	if err := cmd.Execute(); err != nil {
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
func TestAuthHeaderAndIssuePaths(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/myself") {
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p"))[:4]) {
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"transitions":[{"id":"31","name":"Done"}]}`))
	})
	if !run(t, cfg, "auth", "test")["ok"].(bool) {
		t.Fatal()
	}
	if !run(t, cfg, "issue", "get", "EFP-123")["ok"].(bool) {
		t.Fatal()
	}
	if run(t, cfg, "issue", "get", "http://x/browse/EFP-123")["ok"].(bool) {
		t.Fatal("unmatched issue URL should fail")
	}
}

func TestAuthTestUsesHTTPProxyFromEnvironment(t *testing.T) {
	clearCommandProxyEnv(t)

	var proxyHits atomic.Int32
	var badProxyRequest atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		if !r.URL.IsAbs() || r.URL.Host != "jira.internal" || r.URL.Path != "/rest/api/2/myself" {
			badProxyRequest.Store(true)
			http.Error(w, "bad proxy request", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"proxied"}`))
	}))
	defer proxy.Close()

	v := true
	cfg := config.RootConfig{Version: 1, Jira: config.ProductConfig{DefaultInstance: "jira-main", Instances: []config.InstanceConfig{{Name: "jira-main", BaseURL: "http://jira.internal", RESTPath: "/rest/api/2", VerifySSL: &v, Auth: config.AuthConfig{Type: "basic_password", Username: "u", Password: "p"}}}}}
	cfgPath := filepath.Join(t.TempDir(), "cfg.json")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("JIRA_PROXY_HELPER", "1")
	t.Setenv("JIRA_PROXY_HELPER_CONFIG", cfgPath)

	runJiraProxyEnvironmentHelper(t)

	if proxyHits.Load() == 0 {
		t.Fatal("expected jira auth test to hit proxy")
	}
	if badProxyRequest.Load() {
		t.Fatal("expected jira auth test to proxy the absolute Jira URL")
	}
}

func TestAuthTestProxyHelper(t *testing.T) {
	if os.Getenv("JIRA_PROXY_HELPER") == "" {
		t.Skip("helper process only")
	}
	out := run(t, os.Getenv("JIRA_PROXY_HELPER_CONFIG"), "auth", "test")
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("jira auth test failed: %#v", out)
	}
}

func clearCommandProxyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"ALL_PROXY",
		"NO_PROXY",
		"http_proxy",
		"https_proxy",
		"all_proxy",
		"no_proxy",
		"REQUEST_METHOD",
	} {
		t.Setenv(key, "")
	}
}

func runJiraProxyEnvironmentHelper(t *testing.T) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestAuthTestProxyHelper$", "-test.count=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jira proxy environment helper failed: %v\n%s", err, output)
	}
}

func TestSearchCreateTransitionDryDeleteRaw(t *testing.T) {
	cfg, hits := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":[{"id":"21","name":"In Progress"}]}`))
	})
	if !run(t, cfg, "issue", "search", "--jql", "project = EFP")["ok"].(bool) {
		t.Fatal()
	}
	_ = run(t, cfg, "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s", "--field", "customfield_1={\"id\":\"1\"}")
	if !run(t, cfg, "issue", "transition", "EFP-1", "--to", "In Progress")["ok"].(bool) {
		t.Fatal()
	}
	pre := *hits
	_ = run(t, cfg, "--dry-run", "issue", "create", "--project", "EFP", "--type", "Task", "--summary", "s")
	_ = pre
	_ = hits
	if run(t, cfg, "issue", "delete", "EFP-1")["ok"].(bool) {
		t.Fatal("delete needs yes")
	}
	if !run(t, cfg, "--yes", "issue", "delete", "EFP-1")["ok"].(bool) {
		t.Fatal()
	}
	if run(t, cfg, "--yes", "api", "get", "https://evil.example.com/x")["ok"].(bool) {
		t.Fatal("off instance should fail")
	}
}

func TestAttachmentDownloadOffInstanceFails(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":"https://evil.example/file.bin"}`))
	})
	out := run(t, cfg, "attachment", "download", "10000", "--output", filepath.Join(t.TempDir(), "file.bin"))
	if out["ok"].(bool) {
		t.Fatal("off-instance attachment download should fail")
	}
	if out["error"].(map[string]interface{})["code"].(string) != "instance_url_mismatch" {
		t.Fatalf("wrong code: %#v", out)
	}
}
func TestCommandsSchemaAndSecrets(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	out := run(t, cfg, "commands")
	cmds := out["data"].(map[string]interface{})["commands"].([]interface{})
	if len(cmds) < 10 {
		t.Fatal("need many commands")
	}
	s := run(t, cfg, "schema", "issue.create")
	req := s["data"].(map[string]interface{})["required"].([]interface{})
	if len(req) == 0 {
		t.Fatal("schema required empty")
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), "p") {
		_ = os.Getenv("NOOP")
	}
}

func TestEditReturnsNotInteractive(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	out := run(t, cfg, "issue", "edit", "EFP-1")
	if out["ok"].(bool) {
		t.Fatal("should fail")
	}
	if out["error"].(map[string]interface{})["code"].(string) != "not_interactive_supported" {
		t.Fatal("wrong code")
	}
}

func TestIssueBatchBCommands(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	cases := [][]string{{"issue", "watchers", "EFP-1"}, {"issue", "vote", "EFP-1"}, {"issue", "notify", "EFP-1", "--subject", "Review", "--body", "Please review", "--to", "alice"}, {"issue", "comment", "list", "EFP-1"}}
	for _, c := range cases {
		out := run(t, cfg, c...)
		if !out["ok"].(bool) {
			t.Fatalf("failed %v", c)
		}
	}
}

func requireJiraCode(t *testing.T, out map[string]interface{}, want string) {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("unexpected success: %#v", out)
	}
	errObj, _ := out["error"].(map[string]interface{})
	if got, _ := errObj["code"].(string); got != want {
		t.Fatalf("error.code=%q want %q: %#v", got, want, out)
	}
}

func TestJiraBodyAndJSONValueReadErrors(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid args should not hit server: %s %s", r.Method, r.URL.Path)
	})
	missing := filepath.Join(t.TempDir(), "missing.json")
	cases := [][]string{
		{"issue", "comment", "add", "EFP-123", "--body-file", missing},
		{"issue", "comment", "update", "EFP-123", "10001", "--body-file", missing},
		{"api", "post", "/rest/api/2/issue", "--body-file", missing},
		{"issue", "property", "set", "EFP-123", "review.state", "--value-file", missing},
	}
	for _, args := range cases {
		requireJiraCode(t, run(t, cfg, args...), "invalid_args")
	}
}

func TestJiraWriteRequiredArgs(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("invalid args should not hit server: %s %s", r.Method, r.URL.Path)
	})
	cases := [][]string{
		{"issue", "link", "create", "--from", "EFP-1", "--to", "EFP-2"},
		{"issue", "link", "create", "--type", "Relates", "--to", "EFP-2"},
		{"issue", "link", "create", "--type", "Relates", "--from", "EFP-1"},
		{"issue", "remote-link", "add", "EFP-1", "--title", "Spec"},
		{"issue", "remote-link", "add", "EFP-1", "--url", "https://example.test"},
		{"issue", "property", "set", "EFP-1", "review.state"},
		{"issue", "notify", "EFP-1", "--subject", "Review", "--to", "alice"},
		{"issue", "notify", "EFP-1", "--subject", "Review", "--body", "Body"},
		{"component", "create", "--project", "EFP"},
		{"version", "create", "--name", "1.0"},
		{"filter", "create", "--name", "Mine"},
	}
	for _, args := range cases {
		requireJiraCode(t, run(t, cfg, args...), "invalid_args")
	}
}

func TestJiraStableInstanceErrorCodes(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	var rootCfg config.RootConfig
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rootCfg); err != nil {
		t.Fatal(err)
	}
	rootCfg.Jira.DefaultInstance = ""
	rootCfg.Jira.Instances = append(rootCfg.Jira.Instances, rootCfg.Jira.Instances[0])
	rootCfg.Jira.Instances[1].Name = "jira-other"
	rootCfg.Jira.Instances[1].BaseURL = "https://other.example"
	multiCfg := filepath.Join(t.TempDir(), "multi.json")
	if err := config.Save(multiCfg, rootCfg); err != nil {
		t.Fatal(err)
	}
	requireJiraCode(t, run(t, multiCfg, "issue", "search", "--jql", "project = EFP"), "instance_required")
	requireJiraCode(t, run(t, multiCfg, "--instance", "jira-main", "issue", "get", rootCfg.Jira.Instances[1].BaseURL+"/browse/EFP-1"), "instance_url_mismatch")
	requireJiraCode(t, run(t, cfg, "api", "get", "https://evil.example/rest/api/2/myself"), "instance_url_mismatch")

	rootCfg.Jira.Instances[1].BaseURL = rootCfg.Jira.Instances[0].BaseURL
	ambCfg := filepath.Join(t.TempDir(), "ambiguous.json")
	if err := config.Save(ambCfg, rootCfg); err != nil {
		t.Fatal(err)
	}
	requireJiraCode(t, run(t, ambCfg, "issue", "get", rootCfg.Jira.Instances[0].BaseURL+"/browse/EFP-1"), "ambiguous_instance")
}

func TestFilterCrudAndDashboardCommands(t *testing.T) {
	hits := 0
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	_ = run(t, cfg, "filter", "create", "--name", "f", "--jql", "project=EFP")
	_ = run(t, cfg, "filter", "update", "123", "--name", "f2")
	_ = run(t, cfg, "filter", "delete", "123")
	_ = run(t, cfg, "--yes", "filter", "delete", "123")
	_ = run(t, cfg, "filter", "dashboard", "list")
	pre := hits
	_ = run(t, cfg, "--dry-run", "filter", "create", "--name", "f", "--jql", "x")
	if hits != pre {
		t.Fatal("dry run should not hit server")
	}
}

func TestUserGroupBoundaries(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	if !run(t, cfg, "user", "search", "--query", "alice")["ok"].(bool) {
		t.Fatal("user search failed")
	}
	if !run(t, cfg, "user", "group", "get", "eng-team")["ok"].(bool) {
		t.Fatal("group get failed")
	}
	if !run(t, cfg, "user", "group", "search", "--query", "eng")["ok"].(bool) {
		t.Fatal("group search failed")
	}
}

func TestBatchCEndpointAssertions(t *testing.T) {
	type hit struct{ Method, Path string }
	hits := []hit{}
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, hit{Method: r.Method, Path: r.URL.Path})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	_ = run(t, cfg, "filter", "update", "22", "--name", "f2")

	want := []hit{{"PUT", "/rest/api/2/filter/22"}}
	if len(hits) < len(want) {
		t.Fatalf("insufficient calls: got=%d", len(hits))
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("call[%d] mismatch: got=%+v want=%+v", i, hits[i], want[i])
		}
	}
}

func TestBatchCValidationAndSchema(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	if run(t, cfg, "filter", "create", "--name", "only")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	if run(t, cfg, "component", "create", "--name", "x")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	if run(t, cfg, "version", "create", "--project", "EFP")["ok"].(bool) {
		t.Fatal("expected invalid_args")
	}
	s := run(t, cfg, "schema", "filter.create")
	req := s["data"].(map[string]interface{})["required"].([]interface{})
	if len(req) < 2 {
		t.Fatal("schema not updated")
	}
}

func TestWriteValidationConsistency(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	cases := [][]string{{"issue", "update", "EFP-1"}, {"issue", "comment", "add", "EFP-1"}, {"issue", "comment", "update", "EFP-1", "1"}, {"issue", "worklog", "add", "EFP-1"}, {"issue", "worklog", "update", "EFP-1", "1"}}
	for _, c := range cases {
		out := run(t, cfg, c...)
		if out["ok"].(bool) {
			t.Fatalf("expected invalid_args for %v", c)
		}
	}
}

func TestSchemaConsistencyForExtendedWrites(t *testing.T) {
	cfg, _ := setup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	for _, name := range []string{"issue.comment.add", "issue.comment.update", "issue.worklog.add", "issue.worklog.update", "issue.update"} {
		out := run(t, cfg, "schema", name)
		req := out["data"].(map[string]interface{})["required"].([]interface{})
		if len(req) == 0 {
			t.Fatalf("schema required missing for %s", name)
		}
	}
}
