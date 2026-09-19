package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/testutil"
)

// runNexus executes the CLI with --config and --json, returning the parsed
// envelope and the raw stdout so callers can also scan for secret canaries.
func runNexus(t *testing.T, cfg, stdin string, args ...string) (map[string]any, string) {
	t.Helper()
	root := NewRoot()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader(stdin))
	fullArgs := []string{"--json"}
	if cfg != "" {
		fullArgs = append(fullArgs, "--config", cfg)
	}
	root.SetArgs(append(fullArgs, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute %v failed: %v out=%s err=%s", args, err, out.String(), errOut.String())
	}
	assertNoSecrets(t, out.String())
	assertNoSecrets(t, errOut.String())
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json for %v: %v out=%s", args, err, out.String())
	}
	return env, out.String()
}

func requireOK(t *testing.T, cfg, stdin string, args ...string) map[string]any {
	t.Helper()
	env, raw := runNexus(t, cfg, stdin, args...)
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("%v failed: %s", args, raw)
	}
	data, _ := env["data"].(map[string]any)
	return data
}

func requireCode(t *testing.T, code, cfg, stdin string, args ...string) map[string]any {
	t.Helper()
	env, raw := runNexus(t, cfg, stdin, args...)
	if ok, _ := env["ok"].(bool); ok {
		t.Fatalf("%v unexpectedly succeeded: %s", args, raw)
	}
	errObj, _ := env["error"].(map[string]any)
	if got, _ := errObj["code"].(string); got != code {
		t.Fatalf("%v error.code=%q want %q: %s", args, got, code, raw)
	}
	return errObj
}

func assertNoSecrets(t *testing.T, s string) {
	t.Helper()
	for _, secret := range testutil.Secrets {
		if strings.Contains(s, secret) {
			t.Fatalf("secret leaked: %s in %s", secret, s)
		}
	}
	if strings.Contains(strings.ToLower(s), "authorization") {
		t.Fatalf("authorization header leaked: %s", s)
	}
}

func writeNexusConfig(t *testing.T, content string) string {
	t.Helper()
	cfg, err := testutil.WriteConfig(content)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(cfg) })
	return cfg
}

func items(t *testing.T, data map[string]any) []any {
	t.Helper()
	list, _ := data["items"].([]any)
	return list
}

func TestNexusInstanceAndAuthConfigRoundTrip(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")

	added := requireOK(t, cfg, "secret-password-should-not-appear\n", "instance", "add", "repo", "--base-url", mock.Server.URL, "--username", "ci-reader", "--password-stdin", "--default")
	if added["added"] != true || added["anonymous"] != false || added["default_instance"] != "repo" {
		t.Fatalf("bad add data: %#v", added)
	}
	anon := requireOK(t, cfg, "", "instance", "add", "public", "--base-url", mock.Server.URL)
	if anon["anonymous"] != true {
		t.Fatalf("expected anonymous instance: %#v", anon)
	}
	requireCode(t, "conflict", cfg, "", "instance", "add", "repo", "--base-url", mock.Server.URL)
	requireCode(t, "invalid_args", cfg, "", "instance", "add", "broken", "--base-url", "nexus.example.test")
	requireCode(t, "invalid_args", cfg, "", "instance", "add", "broken", "--base-url", mock.Server.URL, "--username", "u")
	requireCode(t, "invalid_args", cfg, "", "instance", "add", "broken")

	list, raw := runNexus(t, cfg, "", "instance", "list")
	listData := list["data"].(map[string]any)
	instances := listData["instances"].([]any)
	if len(instances) != 2 || listData["default_instance"] != "repo" {
		t.Fatalf("bad instance list: %s", raw)
	}
	first := instances[0].(map[string]any)
	if first["rest_path"] != "/service/rest/v1" {
		t.Fatalf("expected effective rest_path in list: %#v", first)
	}
	if auth := first["auth"].(map[string]any); auth["type"] != "basic_password" || auth["username"] != "ci-reader" || auth["password"] != "***REDACTED***" {
		t.Fatalf("expected redacted basic_password auth: %#v", auth)
	}

	got := requireOK(t, cfg, "", "instance", "get", "repo")
	if got["base_url"] != mock.Server.URL || got["name"] != "repo" {
		t.Fatalf("bad instance get: %#v", got)
	}
	requireCode(t, "not_found", cfg, "", "instance", "get", "missing")

	requireCode(t, "invalid_args", cfg, "", "instance", "update", "repo")
	requireCode(t, "not_found", cfg, "", "instance", "update", "missing", "--rest-path", "/service/rest/v1")
	updated := requireOK(t, cfg, "", "instance", "update", "repo", "--rest-path", "/custom/rest/v1")
	if updated["updated"] != true {
		t.Fatalf("bad update data: %#v", updated)
	}
	if got := requireOK(t, cfg, "", "instance", "get", "repo"); got["rest_path"] != "/custom/rest/v1" {
		t.Fatalf("rest_path not updated: %#v", got)
	}

	if def := requireOK(t, cfg, "", "instance", "default"); def["default_instance"] != "repo" {
		t.Fatalf("bad default: %#v", def)
	}
	requireCode(t, "not_found", cfg, "", "instance", "default", "missing")
	requireOK(t, cfg, "", "instance", "default", "public")
	if def := requireOK(t, cfg, "", "instance", "default"); def["default_instance"] != "public" {
		t.Fatalf("default not switched: %#v", def)
	}

	requireCode(t, "invalid_args", cfg, "", "auth", "login", "--instance", "repo")
	login := requireOK(t, cfg, "secret-api-key-should-not-appear\n", "auth", "login", "--instance", "repo", "--username", "token-name", "--api-key-stdin")
	if login["logged_in"] != true || login["auth_type"] != "basic_api_key" {
		t.Fatalf("bad login data: %#v", login)
	}
	saved, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var repo config.InstanceConfig
	for _, in := range saved.Nexus.Instances {
		if in.Name == "repo" {
			repo = in
		}
	}
	if repo.Auth.Type != "basic_api_key" || repo.Auth.Username != "token-name" || repo.Auth.APIKey != "secret-api-key-should-not-appear" || repo.Auth.Password != "" {
		t.Fatalf("auth not persisted: %+v", repo.Auth)
	}
	bearer := requireOK(t, cfg, "secret-token-should-not-appear\n", "auth", "login", "--instance", "public", "--token-stdin")
	if bearer["auth_type"] != "bearer_token" {
		t.Fatalf("bad bearer login: %#v", bearer)
	}

	requireCode(t, "invalid_args", cfg, "", "auth", "logout", "--instance", "repo")
	logout := requireOK(t, cfg, "", "auth", "logout", "--instance", "repo", "--yes")
	if logout["logged_out"] != true || logout["anonymous"] != true {
		t.Fatalf("bad logout data: %#v", logout)
	}
	saved, err = config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range saved.Nexus.Instances {
		if in.Name == "repo" && (in.Auth.Type != "" || in.Auth.APIKey != "" || in.Auth.Username != "") {
			t.Fatalf("auth not cleared: %+v", in.Auth)
		}
	}

	requireCode(t, "invalid_args", cfg, "", "instance", "remove", "public")
	requireCode(t, "not_found", cfg, "", "instance", "remove", "missing", "--yes")
	if removed := requireOK(t, cfg, "", "instance", "remove", "public", "--yes"); removed["removed"] != true {
		t.Fatalf("bad remove data: %#v", removed)
	}
	list, _ = runNexus(t, cfg, "", "instance", "list")
	listData = list["data"].(map[string]any)
	if len(listData["instances"].([]any)) != 1 || listData["default_instance"] != "" {
		t.Fatalf("expected one instance and cleared default: %#v", listData)
	}
	if mock.Hits != 0 {
		t.Fatalf("config commands must not call the server, hits=%d", mock.Hits)
	}
}

func TestNexusEnvManagedConfigRefusesWrites(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	t.Setenv("EFP_NEXUS_DEFAULT_INSTANCE", "env")
	t.Setenv("EFP_NEXUS_INSTANCES_0_NAME", "env")
	t.Setenv("EFP_NEXUS_INSTANCES_0_BASE_URL", mock.Server.URL)
	t.Setenv("EFP_NEXUS_INSTANCES_0_AUTH_TYPE", "basic_password")
	t.Setenv("EFP_NEXUS_INSTANCES_0_AUTH_USERNAME", "ci-reader")
	t.Setenv("EFP_NEXUS_INSTANCES_0_AUTH_PASSWORD", "secret-password-should-not-appear")
	requireCode(t, "config_env_managed", "", "", "instance", "add", "other", "--base-url", mock.Server.URL)
	requireCode(t, "config_env_managed", "", "", "instance", "default", "env")
	requireCode(t, "config_env_managed", "", "secret-token-should-not-appear\n", "auth", "login", "--token-stdin")
	requireCode(t, "config_env_managed", "", "", "auth", "logout", "--yes")
	list := requireOK(t, "", "", "instance", "list")
	if list["default_instance"] != "env" {
		t.Fatalf("env config not read: %#v", list)
	}
	test := requireOK(t, "", "", "auth", "test")
	if test["authenticated"] != true || !strings.HasPrefix(mock.LastAuthorization, "Basic ") {
		t.Fatalf("env-managed instance did not authenticate: %#v auth=%q", test, mock.LastAuthorization)
	}
}

func TestNexusAuthTestAndRepoCommands(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))

	test := requireOK(t, cfg, "", "auth", "test")
	if test["authenticated"] != true || test["anonymous"] != false || test["repository_count"] != float64(3) || test["auth_type"] != "basic_password" || test["rest_path"] != "/service/rest/v1" {
		t.Fatalf("bad auth test data: %#v", test)
	}
	if mock.LastPath != "/service/rest/v1/repositories" || !strings.HasPrefix(mock.LastAuthorization, "Basic ") {
		t.Fatalf("auth test hit %s with auth %q", mock.LastPath, mock.LastAuthorization)
	}

	repos := requireOK(t, cfg, "", "repo", "list")
	if repos["count"] != float64(3) || len(repos["repositories"].([]any)) != 3 {
		t.Fatalf("bad repo list: %#v", repos)
	}
	repo := requireOK(t, cfg, "", "repo", "get", "maven-releases")
	if repo["name"] != "maven-releases" || repo["format"] != "maven2" || mock.LastPath != "/service/rest/v1/repositories/maven-releases" {
		t.Fatalf("bad repo get: %#v path=%s", repo, mock.LastPath)
	}
	requireCode(t, "not_found", cfg, "", "repo", "get", "missing")

	mock.Reject = true
	errObj := requireCode(t, "auth_failed", cfg, "", "auth", "test")
	if status, _ := errObj["status"].(float64); status != 401 {
		t.Fatalf("expected 401 status: %#v", errObj)
	}
}

func TestNexusAnonymousInstanceSendsNoAuthorization(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	mock.Public = true
	cfg := writeNexusConfig(t, testutil.NexusAnonymousConfig(mock.Server.URL))
	test := requireOK(t, cfg, "", "auth", "test")
	if test["authenticated"] != false || test["anonymous"] != true || test["auth_type"] != "anonymous" || test["repository_count"] != float64(3) {
		t.Fatalf("bad anonymous auth test: %#v", test)
	}
	if mock.LastAuthorization != "" {
		t.Fatalf("anonymous instance sent Authorization header")
	}
	requireOK(t, cfg, "", "repo", "list")
	if mock.LastAuthorization != "" {
		t.Fatalf("anonymous instance sent Authorization header on repo list")
	}

	closed := testutil.NewMockNexus(t)
	closedCfg := writeNexusConfig(t, testutil.NexusAnonymousConfig(closed.Server.URL))
	requireCode(t, "auth_failed", closedCfg, "", "repo", "list")
}

func TestNexusIncompleteAuthIsConfigError(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, "nexus:\n  default_instance: local\n  instances:\n    - name: local\n      base_url: "+mock.Server.URL+"\n      auth:\n        type: basic_password\n        username: ci-reader\n")
	requireCode(t, "config_error", cfg, "", "repo", "list")
	if mock.Hits != 0 {
		t.Fatalf("incomplete auth must not reach the server")
	}
}

func TestNexusInstanceResolutionErrors(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	none := writeNexusConfig(t, "jenkins:\n  default_instance: ci\n  instances: []\n")
	requireCode(t, "no_instance_configured", none, "", "repo", "list")
	requireCode(t, "no_instance_configured", none, "", "auth", "logout", "--yes")

	two := writeNexusConfig(t, "nexus:\n  instances:\n    - name: a\n      base_url: "+mock.Server.URL+"\n    - name: b\n      base_url: "+mock.Server.URL+"\n")
	requireCode(t, "instance_required", two, "", "repo", "list")
	requireCode(t, "instance_required", two, "", "repo", "list", "--instance", "missing")
	requireCode(t, "instance_required", two, "secret-token-should-not-appear\n", "auth", "login", "--token-stdin")
	mock.Public = true
	requireOK(t, two, "", "repo", "list", "--instance", "b")

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	requireCode(t, "config_missing", missing, "", "repo", "list")
	requireCode(t, "config_missing", missing, "", "instance", "list")
}

func TestNexusComponentSearchMapsFiltersAndPages(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))

	data := requireOK(t, cfg, "", "component", "search",
		"--repository", "maven-releases", "--repo-format", "maven2", "--group", "com.example", "--name", "app", "--version", "1.0.0",
		"--keyword", "app", "--sort", "version", "--direction", "desc",
		"--maven-group-id", "com.example", "--maven-artifact-id", "app", "--maven-base-version", "1.0.0", "--maven-extension", "jar", "--maven-classifier", "sources")
	if mock.LastPath != "/service/rest/v1/search" {
		t.Fatalf("search hit %s", mock.LastPath)
	}
	want := map[string]string{
		"repository": "maven-releases", "format": "maven2", "group": "com.example", "name": "app", "version": "1.0.0",
		"q": "app", "sort": "version", "direction": "desc",
		"maven.groupId": "com.example", "maven.artifactId": "app", "maven.baseVersion": "1.0.0", "maven.extension": "jar", "maven.classifier": "sources",
	}
	for k, v := range want {
		if got := mock.LastQuery.Get(k); got != v {
			t.Fatalf("query %s=%q want %q (all=%v)", k, got, v, mock.LastQuery)
		}
	}
	for _, forbidden := range []string{"repo-format", "repo_format", "keyword", "continuationToken", "limit", "all", "max-pages"} {
		if _, ok := mock.LastQuery[forbidden]; ok {
			t.Fatalf("unexpected query parameter %s: %v", forbidden, mock.LastQuery)
		}
	}
	if len(items(t, data)) != 2 || data["count_returned"] != float64(2) || data["continuation_token"] != testutil.NexusPageTwoToken || data["truncated"] != true || data["pages_fetched"] != float64(1) {
		t.Fatalf("bad first page: %#v", data)
	}

	page2 := requireOK(t, cfg, "", "component", "search", "--repository", "maven-releases", "--continuation", testutil.NexusPageTwoToken)
	if mock.LastQuery.Get("continuationToken") != testutil.NexusPageTwoToken {
		t.Fatalf("continuation token not forwarded: %v", mock.LastQuery)
	}
	if len(items(t, page2)) != 1 || page2["continuation_token"] != "" || page2["truncated"] != false {
		t.Fatalf("bad second page: %#v", page2)
	}

	before := mock.Hits
	all := requireOK(t, cfg, "", "component", "search", "--repository", "maven-releases", "--all")
	if len(items(t, all)) != 3 || all["pages_fetched"] != float64(2) || all["truncated"] != false || all["continuation_token"] != "" || mock.Hits-before != 2 {
		t.Fatalf("bad --all result: %#v hits=%d", all, mock.Hits-before)
	}

	before = mock.Hits
	capped := requireOK(t, cfg, "", "component", "search", "--repository", "maven-releases", "--all", "--limit", "2")
	if len(items(t, capped)) != 2 || capped["truncated"] != true || capped["continuation_token"] != testutil.NexusPageTwoToken || mock.Hits-before != 1 {
		t.Fatalf("bad --all --limit result: %#v hits=%d", capped, mock.Hits-before)
	}

	before = mock.Hits
	onePage := requireOK(t, cfg, "", "component", "search", "--repository", "maven-releases", "--all", "--max-pages", "1")
	if len(items(t, onePage)) != 2 || onePage["pages_fetched"] != float64(1) || onePage["truncated"] != true || mock.Hits-before != 1 {
		t.Fatalf("bad --max-pages result: %#v", onePage)
	}

	limited := requireOK(t, cfg, "", "component", "search", "--name", "app", "--limit", "1")
	if len(items(t, limited)) != 1 || limited["truncated"] != true || limited["dropped"] != float64(1) || limited["continuation_token"] != testutil.NexusPageTwoToken {
		t.Fatalf("bad --limit 1 result: %#v", limited)
	}
	if capped["dropped"] != float64(0) || all["dropped"] != float64(0) {
		t.Fatalf("dropped must be zero when no page was cut: capped=%#v all=%#v", capped, all)
	}

	before = mock.Hits
	requireCode(t, "invalid_args", cfg, "", "component", "search", "--limit", "0")
	requireCode(t, "invalid_args", cfg, "", "component", "search", "--limit", "501")
	requireCode(t, "invalid_args", cfg, "", "component", "search", "--all", "--max-pages", "0")
	if mock.Hits != before {
		t.Fatalf("invalid paging flags must not reach the server")
	}
	requireCode(t, "invalid_args", cfg, "", "component", "search", "--continuation", "bogus")
}

func TestNexusRepoFormatDoesNotCollideWithOutputFormat(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--config", cfg, "--format", "yaml", "component", "search", "--repo-format", "npm"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if mock.LastQuery.Get("format") != "npm" {
		t.Fatalf("repo format not mapped to format query: %v", mock.LastQuery)
	}
	if !strings.Contains(out.String(), "ok: true") || !strings.Contains(out.String(), "continuation_token: "+testutil.NexusPageTwoToken) {
		t.Fatalf("expected yaml output with continuation token: %s", out.String())
	}
}

func TestNexusComponentListAndGet(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))

	requireCode(t, "invalid_args", cfg, "", "component", "list")
	if mock.Hits != 0 {
		t.Fatalf("missing --repository must not reach the server")
	}
	list := requireOK(t, cfg, "", "component", "list", "--repository", "maven-releases", "--limit", "10")
	if mock.LastPath != "/service/rest/v1/components" || mock.LastQuery.Get("repository") != "maven-releases" {
		t.Fatalf("component list hit %s %v", mock.LastPath, mock.LastQuery)
	}
	if len(items(t, list)) != 2 || list["continuation_token"] != testutil.NexusPageTwoToken {
		t.Fatalf("bad component list: %#v", list)
	}
	all := requireOK(t, cfg, "", "component", "list", "--repository", "maven-releases", "--all")
	if len(items(t, all)) != 3 {
		t.Fatalf("bad component list --all: %#v", all)
	}

	got := requireOK(t, cfg, "", "component", "get", testutil.NexusComponentID)
	if got["id"] != testutil.NexusComponentID || got["name"] != "app" || len(got["assets"].([]any)) != 2 || mock.LastPath != "/service/rest/v1/components/"+testutil.NexusComponentID {
		t.Fatalf("bad component get: %#v path=%s", got, mock.LastPath)
	}
	requireCode(t, "not_found", cfg, "", "component", "get", "unknown")
}

func TestNexusAssetSearchListGet(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))

	search := requireOK(t, cfg, "", "asset", "search", "--repository", "docker-hosted", "--repo-format", "docker", "--docker-image-name", "payments/api", "--docker-image-tag", "2.3.0")
	if mock.LastPath != "/service/rest/v1/search/assets" {
		t.Fatalf("asset search hit %s", mock.LastPath)
	}
	if mock.LastQuery.Get("docker.imageName") != "payments/api" || mock.LastQuery.Get("docker.imageTag") != "2.3.0" || mock.LastQuery.Get("format") != "docker" || mock.LastQuery.Get("repository") != "docker-hosted" {
		t.Fatalf("docker filters not mapped: %v", mock.LastQuery)
	}
	if len(items(t, search)) != 2 || search["truncated"] != true {
		t.Fatalf("bad asset search: %#v", search)
	}
	first := items(t, search)[0].(map[string]any)
	if first["downloadUrl"] != mock.Server.URL+testutil.NexusArtifactPath {
		t.Fatalf("download url altered in output: %#v", first)
	}

	requireCode(t, "invalid_args", cfg, "", "asset", "list")
	list := requireOK(t, cfg, "", "asset", "list", "--repository", "maven-releases", "--all", "--max-pages", "3")
	if mock.LastPath != "/service/rest/v1/assets" || len(items(t, list)) != 3 || list["pages_fetched"] != float64(2) {
		t.Fatalf("bad asset list: %#v path=%s", list, mock.LastPath)
	}

	got := requireOK(t, cfg, "", "asset", "get", testutil.NexusAssetID)
	if got["id"] != testutil.NexusAssetID || got["path"] != "com/example/app/1.0.0/app-1.0.0.jar" {
		t.Fatalf("bad asset get: %#v", got)
	}
	if sums := got["checksum"].(map[string]any); sums["sha1"] != testutil.NexusArtifactSHA1() {
		t.Fatalf("checksum altered in output: %#v", sums)
	}
	requireCode(t, "not_found", cfg, "", "asset", "get", "unknown")
}

func TestNexusAssetDownloadWritesFileAndReturnsMetadataOnly(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))
	out := filepath.Join(t.TempDir(), "nested", "app.jar")

	env, raw := runNexus(t, cfg, "", "asset", "download", testutil.NexusAssetID, "--output", out)
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("download failed: %s", raw)
	}
	data := env["data"].(map[string]any)
	if data["path"] != out || data["bytes"] != float64(len(testutil.NexusArtifactBody)) || data["sha1"] != testutil.NexusArtifactSHA1() || data["sha1_verified"] != true {
		t.Fatalf("bad download metadata: %#v", data)
	}
	if data["content_type"] != "application/java-archive" || data["name"] != "app-1.0.0.jar" || data["asset_id"] != testutil.NexusAssetID || data["repository"] != "maven-releases" || data["download_url"] != mock.Server.URL+testutil.NexusArtifactPath {
		t.Fatalf("bad download metadata: %#v", data)
	}
	if strings.Contains(raw, testutil.NexusArtifactBody) {
		t.Fatalf("artifact bytes leaked into the envelope: %s", raw)
	}
	if b, err := os.ReadFile(out); err != nil || string(b) != testutil.NexusArtifactBody {
		t.Fatalf("downloaded file=%q err=%v", string(b), err)
	}
	if !strings.HasPrefix(mock.LastAuthorization, "Basic ") || mock.LastPath != testutil.NexusArtifactPath {
		t.Fatalf("download request path=%s auth=%q", mock.LastPath, mock.LastAuthorization)
	}
}

func TestNexusAssetDownloadGuardsAndDryRun(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))
	dir := t.TempDir()

	offsite := filepath.Join(dir, "offsite.jar")
	requireCode(t, "instance_url_mismatch", cfg, "", "asset", "download", testutil.NexusOffsiteAssetID, "--output", offsite)
	if _, err := os.Stat(offsite); !os.IsNotExist(err) {
		t.Fatalf("off-instance download must not create a file: %v", err)
	}
	if mock.LastPath != "/service/rest/v1/assets/"+testutil.NexusOffsiteAssetID {
		t.Fatalf("unexpected request after mismatch: %s", mock.LastPath)
	}

	requireCode(t, "not_found", cfg, "", "asset", "download", "unknown", "--output", filepath.Join(dir, "unknown.bin"))

	before := mock.Hits
	dry := requireOK(t, cfg, "", "--dry-run", "asset", "download", testutil.NexusAssetID, "--output", filepath.Join(dir, "dry.jar"))
	if dry["dry_run"] != true || mock.Hits != before {
		t.Fatalf("dry-run hit the server or returned bad data: %#v hits=%d", dry, mock.Hits-before)
	}
	if _, err := os.Stat(filepath.Join(dir, "dry.jar")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not create a file")
	}
}

func TestNexusAPIGet(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))

	env, _ := runNexus(t, cfg, "", "api", "get", "repositories")
	if env["ok"] != true || mock.LastPath != "/service/rest/v1/repositories" {
		t.Fatalf("relative api path resolved to %s", mock.LastPath)
	}
	if list, ok := env["data"].([]any); !ok || len(list) != 3 {
		t.Fatalf("expected raw repository list: %#v", env["data"])
	}

	env, _ = runNexus(t, cfg, "", "api", "get", "/service/rest/v1/search", "--query", "repository=maven-releases", "--query", "name=app")
	if env["ok"] != true || mock.LastPath != "/service/rest/v1/search" || mock.LastQuery.Get("repository") != "maven-releases" || mock.LastQuery.Get("name") != "app" {
		t.Fatalf("absolute-from-base api path: path=%s query=%v", mock.LastPath, mock.LastQuery)
	}
	if data := env["data"].(map[string]any); data["continuationToken"] != testutil.NexusPageTwoToken {
		t.Fatalf("raw continuationToken must survive redaction: %#v", data)
	}

	status := requireOK(t, cfg, "", "api", "get", "status")
	if status["status"] != float64(200) {
		t.Fatalf("empty body should report status: %#v", status)
	}
	check := requireOK(t, cfg, "", "api", "get", "status/check")
	if _, ok := check["File Blob Stores"]; !ok {
		t.Fatalf("bad status check data: %#v", check)
	}

	requireCode(t, "invalid_args", cfg, "", "api", "get", "repositories", "--query", "novalue")
	before := mock.Hits
	requireCode(t, "instance_url_mismatch", cfg, "", "api", "get", "https://evil.example/service/rest/v1/repositories")
	if mock.Hits != before {
		t.Fatalf("off-instance URL reached the server")
	}
	requireOK(t, cfg, "", "api", "get", mock.Server.URL+"/service/rest/v1/repositories")
	requireCode(t, "not_found", cfg, "", "api", "get", "does-not-exist")
}

func TestNexusSecretsNeverAppear(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))
	cases := [][]string{
		{"--verbose", "auth", "test"},
		{"--verbose", "repo", "list"},
		{"instance", "list"},
		{"instance", "get", "local"},
		{"--format", "yaml", "instance", "get", "local"},
		{"--format", "table", "auth", "test"},
		{"--verbose", "component", "search", "--name", "app"},
		{"--verbose", "asset", "download", testutil.NexusAssetID, "--output", filepath.Join(t.TempDir(), "a.jar")},
		{"--verbose", "api", "get", "https://evil.example/x"},
	}
	for _, args := range cases {
		root := NewRoot()
		var b bytes.Buffer
		root.SetOut(&b)
		root.SetErr(&b)
		root.SetArgs(append([]string{"--config", cfg}, args...))
		_ = root.Execute()
		assertNoSecrets(t, b.String())
	}
	mock.Reject = true
	for _, args := range [][]string{{"auth", "test"}, {"--verbose", "repo", "list"}, {"asset", "download", testutil.NexusAssetID, "--output", filepath.Join(t.TempDir(), "b.jar")}} {
		root := NewRoot()
		var b bytes.Buffer
		root.SetOut(&b)
		root.SetErr(&b)
		root.SetArgs(append([]string{"--config", cfg, "--json"}, args...))
		_ = root.Execute()
		assertNoSecrets(t, b.String())
		if !strings.Contains(b.String(), "auth_failed") {
			t.Fatalf("expected auth_failed for %v: %s", args, b.String())
		}
	}
}

func TestNexusVerboseDiagnosticsGoToStderr(t *testing.T) {
	mock := testutil.NewMockNexus(t)
	cfg := writeNexusConfig(t, testutil.NexusConfig(mock.Server.URL))
	root := NewRoot()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"--config", cfg, "--json", "--verbose", "repo", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.AssertOKEnvelope(t, out.Bytes())
	if !strings.Contains(errOut.String(), "auth=basic_password") || !strings.Contains(errOut.String(), "rest_path=/service/rest/v1") {
		t.Fatalf("expected diagnostics on stderr: %q", errOut.String())
	}
	assertNoSecrets(t, errOut.String())
}

func TestNexusCommandsSchemaAndHelpMetadata(t *testing.T) {
	commandsEnv, _ := runNexus(t, "", "", "commands")
	commands := commandsEnv["data"].(map[string]any)["commands"].([]any)
	if len(commands) < 20 {
		t.Fatalf("too few commands: %d", len(commands))
	}
	byName := map[string]map[string]any{}
	for _, raw := range commands {
		m := raw.(map[string]any)
		byName[m["name"].(string)] = m
		if risk := m["risk"].(string); strings.HasPrefix(m["name"].(string), "repo.") || strings.HasPrefix(m["name"].(string), "component.") || strings.HasPrefix(m["name"].(string), "asset.") || strings.HasPrefix(m["name"].(string), "api.") {
			if risk != "read" {
				t.Fatalf("%s must be read-only, got %s", m["name"], risk)
			}
		}
	}
	for name, risk := range map[string]string{"instance.add": "write", "instance.update": "write", "instance.default": "write", "auth.login": "write", "instance.remove": "delete", "auth.logout": "delete", "asset.download": "read", "auth.test": "read"} {
		if byName[name] == nil || byName[name]["risk"] != risk {
			t.Fatalf("%s risk=%v want %s", name, byName[name]["risk"], risk)
		}
	}
	for _, name := range []string{"instance.remove", "auth.logout"} {
		if ex := byName[name]["examples"].([]any); !strings.Contains(ex[0].(string), "--yes") {
			t.Fatalf("%s example must carry --yes: %v", name, ex)
		}
	}

	schemaEnv, _ := runNexus(t, "", "", "schema", "component.search")
	schema := schemaEnv["data"].(map[string]any)
	flags := map[string]map[string]any{}
	for _, raw := range schema["flags"].([]any) {
		f := raw.(map[string]any)
		flags[f["name"].(string)] = f
	}
	for _, name := range []string{"repository", "repo-format", "group", "name", "version", "keyword", "sort", "direction", "continuation", "limit", "all", "max-pages", "maven-group-id", "maven-artifact-id", "maven-base-version", "maven-extension", "maven-classifier", "docker-image-name", "docker-image-tag", "instance", "config", "json"} {
		f, ok := flags[name]
		if !ok {
			t.Fatalf("schema missing flag %s: %#v", name, schema)
		}
		if desc, _ := f["description"].(string); strings.TrimSpace(desc) == "" || desc == "Command option." {
			t.Fatalf("flag %s has unclear description %q", name, desc)
		}
	}
	if flags["limit"]["type"] != "int" || flags["all"]["type"] != "bool" || flags["max-pages"]["type"] != "int" || flags["repo-format"]["type"] != "string" {
		t.Fatalf("bad flag types: limit=%v all=%v max-pages=%v", flags["limit"]["type"], flags["all"]["type"], flags["max-pages"]["type"])
	}
	if schema["risk"] != "read" {
		t.Fatalf("component.search risk=%v", schema["risk"])
	}
	downloadEnv, _ := runNexus(t, "", "", "schema", "asset.download")
	download := downloadEnv["data"].(map[string]any)
	hasOutput := false
	for _, raw := range download["flags"].([]any) {
		if raw.(map[string]any)["name"] == "output" {
			hasOutput = true
		}
	}
	if !hasOutput {
		t.Fatalf("asset.download schema missing --output: %#v", download)
	}
	requireCode(t, "not_found", "", "", "schema", "asset.upload")

	helpEnv, raw := runNexus(t, "", "", "help", "llm")
	tips := helpEnv["data"].(map[string]any)["tips"].([]any)
	joined := ""
	for _, tip := range tips {
		joined += tip.(string) + "\n"
	}
	for _, want := range []string{"nexus repo list --json", "--docker-image-name", "--docker-image-tag", "continuation_token", "--all", "read-only", "--repo-format", "metadata only"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help llm missing %q: %s", want, raw)
		}
	}
}
