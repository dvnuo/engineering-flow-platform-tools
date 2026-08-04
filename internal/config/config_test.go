package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNormalizeAuthCompatibility(t *testing.T) {
	c := RootConfig{Jira: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Username: "u", Password: "p"}}, {Auth: AuthConfig{Username: "u", APIKey: "k"}}, {Auth: AuthConfig{Token: "t"}}, {Auth: AuthConfig{Username: "u", Token: "legacy"}}}}, Jenkins: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Username: "jenkins", Token: "api-token"}}}}}
	c.Normalize()
	if c.Jira.Instances[0].Auth.Type != "basic_password" || c.Jira.Instances[1].Auth.Type != "basic_api_key" || c.Jira.Instances[2].Auth.Type != "bearer_token" || c.Jira.Instances[3].Auth.Type != "basic_api_key" {
		t.Fatalf("normalization failed")
	}
	if c.Jenkins.Instances[0].Auth.Type != "basic_api_key" || c.Jenkins.Instances[0].Auth.APIKey != "api-token" {
		t.Fatalf("jenkins normalization failed")
	}
	if c.Jira.Instances[3].Auth.APIKey != "legacy" || c.Jira.Instances[3].Auth.Token != "" {
		t.Fatalf("legacy token+username should become api_key")
	}
}

func TestRedactAuth(t *testing.T) {
	a := AuthConfig{Password: "p", APIKey: "k", Token: "t"}
	r := RedactAuth(a)
	if r.Password == "p" || r.APIKey == "k" || r.Token == "t" {
		t.Fatalf("secret leaked")
	}
}

func TestDefaultPathUsesEFPConfig(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	t.Setenv(EnvLegacyConfigPath, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, ".efp", "config.yaml") {
		t.Fatalf("path=%q", got)
	}
}

func TestLoadSharedPrecedence(t *testing.T) {
	dir := t.TempDir()
	flagPath := filepath.Join(dir, "flag.yaml")
	envPath := filepath.Join(dir, "envfile.yaml")
	if err := os.WriteFile(flagPath, []byte("jira:\n  default_instance: from-flag\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("jira:\n  default_instance: from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigPath, envPath)
	t.Setenv(EnvLegacyConfigPath, "")
	// EFP_VERSION is a recognized EFP_-prefixed var; clear it so a host value
	// cannot make the file-fallback phase look env-managed.
	t.Setenv("EFP_VERSION", "")
	t.Setenv("EFP_JIRA_DEFAULT_INSTANCE", "from-env")

	cfg, source, err := LoadShared(flagPath)
	if err != nil || cfg.Jira.DefaultInstance != "from-flag" || source != flagPath {
		t.Fatalf("flag must win: got=%q source=%q err=%v", cfg.Jira.DefaultInstance, source, err)
	}
	cfg, source, err = LoadShared("")
	if err != nil || cfg.Jira.DefaultInstance != "from-env" || source != EnvSource {
		t.Fatalf("env must beat file: got=%q source=%q err=%v", cfg.Jira.DefaultInstance, source, err)
	}
	t.Setenv("EFP_JIRA_DEFAULT_INSTANCE", "")
	cfg, source, err = LoadShared("")
	if err != nil || cfg.Jira.DefaultInstance != "from-file" || source != envPath {
		t.Fatalf("file fallback: got=%q source=%q err=%v", cfg.Jira.DefaultInstance, source, err)
	}
}

func TestLoadSharedNormalizesEnv(t *testing.T) {
	t.Setenv("EFP_VERSION", "")
	t.Setenv("EFP_JIRA_INSTANCES_0_NAME", "a")
	t.Setenv("EFP_JIRA_INSTANCES_0_AUTH_USERNAME", "u")
	t.Setenv("EFP_JIRA_INSTANCES_0_AUTH_TOKEN", "tok")
	cfg, source, err := LoadShared("")
	if err != nil {
		t.Fatal(err)
	}
	if source != EnvSource {
		t.Fatalf("source=%q", source)
	}
	if len(cfg.Jira.Instances) != 1 || cfg.Jira.Instances[0].Auth.Type != "basic_api_key" {
		t.Fatalf("env config must be normalized: %#v", cfg.Jira.Instances)
	}
}

func TestLoadResolvesPortableAndWindowsEnvReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_AWS_USERNAME", "windows-user")
	t.Setenv("TOOLS_AWS_PASSWORD", "windows-password")
	if err := os.WriteFile(path, []byte(`version: 1
aws:
  enabled: true
  domain: HBEU
  username: "${TOOLS_AWS_USERNAME}"
  password: "%TOOLS_AWS_PASSWORD%"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AWS.Username != "windows-user" || cfg.AWS.Password != "windows-password" {
		t.Fatalf("environment references were not resolved: %#v", cfg.AWS)
	}
}

func TestLoadRejectsMissingEnvReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_AWS_MISSING_PASSWORD", "")
	if err := os.WriteFile(path, []byte(`version: 1
aws:
  username: "${TOOLS_AWS_MISSING_PASSWORD}"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	var missing *MissingEnvReferenceError
	if !errors.As(err, &missing) || missing.Name != "TOOLS_AWS_MISSING_PASSWORD" {
		t.Fatalf("expected missing environment reference error, got %v", err)
	}
}

func TestSavePreservesEnvReferencesInsteadOfMaterializingSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_AWS_USERNAME", "resolved-user")
	t.Setenv("TOOLS_AWS_PASSWORD", "resolved-password")
	if err := os.WriteFile(path, []byte(`version: 1
aws:
  enabled: true
  domain: HBEU
  username: "${TOOLS_AWS_USERNAME}"
  password: "%TOOLS_AWS_PASSWORD%"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Preservation must not depend on save-time environment state. Credentials
	// may be rotated or removed after the config was loaded.
	t.Setenv("TOOLS_AWS_USERNAME", "rotated-user")
	t.Setenv("TOOLS_AWS_PASSWORD", "")
	cfg.Browser.Bookmarks.Sources = []BrowserBookmarkSource{{Name: "company", URL: "https://example.test/bookmarks.yaml"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	text := string(mustReadFile(t, path))
	for _, reference := range []string{"${TOOLS_AWS_USERNAME}", "%TOOLS_AWS_PASSWORD%"} {
		if !strings.Contains(text, reference) {
			t.Fatalf("environment reference %q was not preserved:\n%s", reference, text)
		}
	}
	for _, secret := range []string{"resolved-user", "resolved-password", "rotated-user"} {
		if strings.Contains(text, secret) {
			t.Fatalf("resolved value %q was materialized in config:\n%s", secret, text)
		}
	}
}

func TestSaveReplacesOnlyChangedReferencedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_AWS_USERNAME", "resolved-aws-user")
	t.Setenv("TOOLS_AWS_PASSWORD", "resolved-aws-password")
	t.Setenv("TOOLS_JIRA_USERNAME", "resolved-jira-user")
	if err := os.WriteFile(path, []byte(`version: 1
aws:
  enabled: true
  domain: HBEU
  username: "${TOOLS_AWS_USERNAME}"
  password: "%TOOLS_AWS_PASSWORD%"
jira:
  default_instance: main
  instances:
    - name: main
      base_url: https://jira.example.test
      auth:
        username: "${TOOLS_JIRA_USERNAME}"
        password: jira-password
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AWS.Username = "replacement-aws-user"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	text := string(mustReadFile(t, path))
	if strings.Contains(text, "${TOOLS_AWS_USERNAME}") || !strings.Contains(text, "replacement-aws-user") {
		t.Fatalf("changed AWS username reference was not replaced:\n%s", text)
	}
	if !strings.Contains(text, "%TOOLS_AWS_PASSWORD%") || strings.Contains(text, "resolved-aws-password") {
		t.Fatalf("unchanged AWS password reference was not preserved:\n%s", text)
	}
	if !strings.Contains(text, "${TOOLS_JIRA_USERNAME}") || strings.Contains(text, "resolved-jira-user") {
		t.Fatalf("unrelated Jira reference was not preserved:\n%s", text)
	}
}

func TestSaveMatchesReferencedSequenceItemsByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_JIRA_USER_A", "resolved-user-a")
	t.Setenv("TOOLS_JIRA_USER_B", "resolved-user-b")
	if err := os.WriteFile(path, []byte(`version: 1
jira:
  default_instance: a
  instances:
    - name: a
      base_url: https://a.example.test
      auth:
        username: "${TOOLS_JIRA_USER_A}"
    - name: b
      base_url: https://b.example.test
      auth:
        username: "%TOOLS_JIRA_USER_B%"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Jira.Instances[1].BaseURL = "https://new-b.example.test"
	cfg.Jira.Instances[0], cfg.Jira.Instances[1] = cfg.Jira.Instances[1], cfg.Jira.Instances[0]
	t.Setenv("TOOLS_JIRA_USER_A", "")
	t.Setenv("TOOLS_JIRA_USER_B", "rotated-user-b")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	var raw RootConfig
	if err := yaml.Unmarshal(mustReadFile(t, path), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Jira.Instances) != 2 {
		t.Fatalf("instances=%#v", raw.Jira.Instances)
	}
	first, second := raw.Jira.Instances[0], raw.Jira.Instances[1]
	if first.Name != "b" || first.BaseURL != "https://new-b.example.test" || first.Auth.Username != "%TOOLS_JIRA_USER_B%" {
		t.Fatalf("reference did not follow instance b: %#v", first)
	}
	if second.Name != "a" || second.Auth.Username != "${TOOLS_JIRA_USER_A}" {
		t.Fatalf("reference did not follow instance a: %#v", second)
	}
}

func TestSavePreservesReferenceThroughAuthTokenNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TOOLS_JENKINS_TOKEN", "resolved-jenkins-token")
	if err := os.WriteFile(path, []byte(`version: 1
jenkins:
  default_instance: ci
  instances:
    - name: ci
      base_url: https://jenkins.example.test
      auth:
        username: build-user
        token: "${TOOLS_JENKINS_TOKEN}"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jenkins.Instances[0].Auth.APIKey != "resolved-jenkins-token" || cfg.Jenkins.Instances[0].Auth.Token != "" {
		t.Fatalf("legacy token was not normalized: %#v", cfg.Jenkins.Instances[0].Auth)
	}
	cfg.Browser.Bookmarks.Sources = []BrowserBookmarkSource{{Name: "company", URL: "https://bookmarks.example.test"}}
	t.Setenv("TOOLS_JENKINS_TOKEN", "")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	var raw RootConfig
	if err := yaml.Unmarshal(mustReadFile(t, path), &raw); err != nil {
		t.Fatal(err)
	}
	auth := raw.Jenkins.Instances[0].Auth
	if auth.Token != "${TOOLS_JENKINS_TOKEN}" || auth.APIKey != "" {
		t.Fatalf("normalized auth materialized the referenced token: %#v", auth)
	}
}

func TestSaveSharedRefusesWhenEnvManaged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigPath, filepath.Join(dir, "config.yaml"))
	t.Setenv(EnvLegacyConfigPath, "")
	t.Setenv("EFP_VERSION", "")
	t.Setenv("EFP_JIRA_DEFAULT_INSTANCE", "env-managed")
	if err := SaveShared("", RootConfig{Version: 1}); !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("want ErrEnvManaged, got %v", err)
	}
	explicit := filepath.Join(dir, "explicit.yaml")
	if err := SaveShared(explicit, RootConfig{Version: 1}); err != nil {
		t.Fatalf("explicit path must stay writable under env-managed: %v", err)
	}
	if _, err := os.Stat(explicit); err != nil {
		t.Fatalf("explicit file not written: %v", err)
	}
	t.Setenv("EFP_JIRA_DEFAULT_INSTANCE", "")
	if err := SaveShared("", RootConfig{Version: 1}); err != nil {
		t.Fatalf("file save must work without env vars: %v", err)
	}
}

func TestSavePreservesOtherTopLevelNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte("version: 1\n# keep copilot node\ncopilot:\n  # keep provider comment\n  provider: github_copilot_plugin\ninspect_image:\n  defaults:\n    model: gpt-5.4\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := RootConfig{Version: 1, Jira: ProductConfig{DefaultInstance: "jira-main"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	if root["copilot"] == nil || root["inspect_image"] == nil || root["jira"] == nil {
		t.Fatalf("top-level nodes were not preserved: %s", string(b))
	}
	if !strings.Contains(string(b), "keep copilot node") || !strings.Contains(string(b), "keep provider comment") {
		t.Fatalf("comments were not preserved: %s", string(b))
	}
}

func TestSaveRewritesManagedProductNodesAsBlockStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := []byte(`jira:
    default_instance: "xxx Jira"
    instances: [{name: xxx Jira, base_url: 'https://test1-jira.systems.uk.com', api_version: "2", rest_path: /rest/api/2, auth: {type: basic_password, username: user1, password: xxx}}]
confluence:
    default_instance: "test1 Confluence"
    instances: [{name: test1 Confluence, base_url: 'https://test1-confluence.systems.uk.com', rest_path: /rest/api, auth: {type: basic_password, username: user1, password: xxx}, verify_ssl: true}]
`)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	verifySSL := true
	cfg := RootConfig{
		Version: 1,
		Jira: ProductConfig{
			DefaultInstance: "xxx Jira",
			Instances: []InstanceConfig{
				{
					Name:       "xxx Jira",
					BaseURL:    "https://test1-jira.systems.uk.com",
					APIVersion: "2",
					RESTPath:   "/rest/api/2",
					Auth:       AuthConfig{Type: "basic_password", Username: "user1", Password: "xxx"},
				},
			},
		},
		Confluence: ProductConfig{
			DefaultInstance: "test1 Confluence",
			Instances: []InstanceConfig{
				{
					Name:      "test1 Confluence",
					BaseURL:   "https://test1-confluence.systems.uk.com",
					RESTPath:  "/rest/api",
					Auth:      AuthConfig{Type: "basic_password", Username: "user1", Password: "xxx"},
					VerifySSL: &verifySSL,
				},
			},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	text := string(mustReadFile(t, path))
	for _, flowStyle := range []string{"instances: [{", "auth: {", "{name:"} {
		if strings.Contains(text, flowStyle) {
			t.Fatalf("config preserved flow-style YAML token %q:\n%s", flowStyle, text)
		}
	}
	assertProductHasNoFlowStyle(t, text, "jira")
	assertProductHasNoFlowStyle(t, text, "confluence")
}

func TestSaveWritesJenkinsNode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := RootConfig{Version: 1, Jenkins: ProductConfig{DefaultInstance: "ci", Instances: []InstanceConfig{{Name: "ci", BaseURL: "https://jenkins.example.test", CrumbMode: "auto", Auth: AuthConfig{Type: "pat", Token: "secret"}}}}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Jenkins.DefaultInstance != "ci" || len(loaded.Jenkins.Instances) != 1 || loaded.Jenkins.Instances[0].CrumbMode != "auto" {
		t.Fatalf("jenkins config not preserved: %#v", loaded.Jenkins)
	}
	redacted := RedactRoot(loaded)
	if redacted.Jenkins.Instances[0].Auth.APIKey == "secret" || redacted.Jenkins.Instances[0].Auth.Token == "secret" {
		t.Fatalf("jenkins secret leaked after redaction")
	}
}

func TestLoadAndSaveBrowserBookmarkSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := RootConfig{
		Version: 1,
		Browser: BrowserConfig{Bookmarks: BrowserBookmarksConfig{Sources: []BrowserBookmarkSource{
			{Name: "company", Description: "Company websites.", URL: "https://bookmarks.example.test/company.yaml"},
			{Name: "public", URL: "https://bookmarks.example.test/public.json"},
		}}},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Browser.Bookmarks.Sources) != 2 ||
		loaded.Browser.Bookmarks.Sources[0].Name != "company" ||
		loaded.Browser.Bookmarks.Sources[0].Description != "Company websites." ||
		loaded.Browser.Bookmarks.Sources[1].URL != "https://bookmarks.example.test/public.json" {
		t.Fatalf("browser bookmark sources not preserved: %#v", loaded.Browser.Bookmarks.Sources)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertProductHasNoFlowStyle(t *testing.T, text string, product string) {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) == 0 {
		t.Fatalf("empty YAML document for product %s", product)
	}
	node := mappingValueNode(doc.Content[0], product)
	if node == nil {
		t.Fatalf("missing YAML product node %s:\n%s", product, text)
	}
	if hasFlowStyleNode(node) {
		t.Fatalf("product node %s preserved flow-style YAML:\n%s", product, text)
	}
}

func mappingValueNode(root *yaml.Node, key string) *yaml.Node {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	return nil
}

func hasFlowStyleNode(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Style&yaml.FlowStyle != 0 {
		return true
	}
	for _, child := range node.Content {
		if hasFlowStyleNode(child) {
			return true
		}
	}
	return false
}
