package config

import "testing"

func mapLookup(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadFromEnvSingleScalar(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_JIRA_DEFAULT_INSTANCE": "main",
	}))
	if !managed {
		t.Fatal("expected managed=true for a single recognized var")
	}
	if cfg.Jira.DefaultInstance != "main" {
		t.Fatalf("default_instance=%q", cfg.Jira.DefaultInstance)
	}
}

func TestLoadFromEnvTwoJiraInstances(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_JIRA_DEFAULT_INSTANCE":          "one",
		"EFP_JIRA_INSTANCES_0_NAME":          "one",
		"EFP_JIRA_INSTANCES_0_BASE_URL":      "https://one.example.test",
		"EFP_JIRA_INSTANCES_0_REST_PATH":     "/rest/api/2",
		"EFP_JIRA_INSTANCES_0_API_VERSION":   "2",
		"EFP_JIRA_INSTANCES_0_AUTH_TYPE":     "bearer_token",
		"EFP_JIRA_INSTANCES_0_AUTH_TOKEN":    "tok0",
		"EFP_JIRA_INSTANCES_1_NAME":          "two",
		"EFP_JIRA_INSTANCES_1_BASE_URL":      "https://two.example.test",
		"EFP_JIRA_INSTANCES_1_AUTH_TYPE":     "basic_password",
		"EFP_JIRA_INSTANCES_1_AUTH_USERNAME": "u2",
		"EFP_JIRA_INSTANCES_1_AUTH_PASSWORD": "p2",
	}))
	if !managed {
		t.Fatal("expected managed=true")
	}
	if len(cfg.Jira.Instances) != 2 {
		t.Fatalf("want 2 instances, got %d: %#v", len(cfg.Jira.Instances), cfg.Jira.Instances)
	}
	i0 := cfg.Jira.Instances[0]
	if i0.Name != "one" || i0.BaseURL != "https://one.example.test" || i0.RESTPath != "/rest/api/2" || i0.APIVersion != "2" {
		t.Fatalf("instance0 scalars: %#v", i0)
	}
	if i0.Auth.Type != "bearer_token" || i0.Auth.Token != "tok0" {
		t.Fatalf("instance0 auth: %#v", i0.Auth)
	}
	i1 := cfg.Jira.Instances[1]
	if i1.Name != "two" || i1.Auth.Type != "basic_password" || i1.Auth.Username != "u2" || i1.Auth.Password != "p2" {
		t.Fatalf("instance1: %#v", i1)
	}
}

func TestLoadFromEnvStructSliceStopsAtGap(t *testing.T) {
	cfg, _ := LoadFromEnv(mapLookup(map[string]string{
		"EFP_JIRA_INSTANCES_0_NAME": "zero",
		"EFP_JIRA_INSTANCES_2_NAME": "two",
	}))
	if len(cfg.Jira.Instances) != 1 || cfg.Jira.Instances[0].Name != "zero" {
		t.Fatalf("a gap at index 1 must stop enumeration: %#v", cfg.Jira.Instances)
	}
}

func TestLoadFromEnvBoolPointer(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{"EFP_AWS_ENABLED": "true"}))
	if !managed {
		t.Fatal("expected managed=true")
	}
	if cfg.AWS.Enabled == nil || *cfg.AWS.Enabled != true {
		t.Fatalf("aws.enabled=%v", cfg.AWS.Enabled)
	}
	cfgFalse, _ := LoadFromEnv(mapLookup(map[string]string{"EFP_AWS_ENABLED": "false"}))
	if cfgFalse.AWS.Enabled == nil || *cfgFalse.AWS.Enabled != false {
		t.Fatalf("aws.enabled(false)=%v", cfgFalse.AWS.Enabled)
	}
}

func TestLoadFromEnvIntField(t *testing.T) {
	cfg, _ := LoadFromEnv(mapLookup(map[string]string{"EFP_MOBILE_AUTO_RETENTION_HOURS": "5"}))
	if cfg.Mobile.RetentionHours != 5 {
		t.Fatalf("retention_hours=%d", cfg.Mobile.RetentionHours)
	}
}

func TestLoadFromEnvStringSlice(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_MOBILE_AUTO_BROWSERSTACK_HTTP_PROXY_NO_PROXY_HOSTS_0": "a.internal",
		"EFP_MOBILE_AUTO_BROWSERSTACK_HTTP_PROXY_NO_PROXY_HOSTS_1": "b.internal",
	}))
	if !managed {
		t.Fatal("expected managed=true")
	}
	got := cfg.Mobile.BrowserStack.HTTPProxy.NoProxyHosts
	if len(got) != 2 || got[0] != "a.internal" || got[1] != "b.internal" {
		t.Fatalf("no_proxy_hosts=%#v", got)
	}
}

func TestLoadFromEnvBrowserBookmarkSourceDescription(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_BROWSER_BOOKMARKS_SOURCES_0_NAME":        "personal",
		"EFP_BROWSER_BOOKMARKS_SOURCES_0_DESCRIPTION": "Personal websites.",
		"EFP_BROWSER_BOOKMARKS_SOURCES_0_URL":         "~/.efp/browser/bookmarks/personal.yaml",
	}))
	if !managed || len(cfg.Browser.Bookmarks.Sources) != 1 {
		t.Fatalf("bookmark sources = %#v managed=%v", cfg.Browser.Bookmarks.Sources, managed)
	}
	source := cfg.Browser.Bookmarks.Sources[0]
	if source.Name != "personal" || source.Description != "Personal websites." ||
		source.URL != "~/.efp/browser/bookmarks/personal.yaml" {
		t.Fatalf("bookmark source = %#v", source)
	}
}

func TestLoadFromEnvNotManagedWhenNoVars(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{}))
	if managed {
		t.Fatal("expected managed=false with no recognized vars")
	}
	if cfg.AWS.Enabled != nil {
		t.Fatalf("absent *bool must stay nil: %v", cfg.AWS.Enabled)
	}
}

func TestLoadFromEnvEmptyValueIsAbsent(t *testing.T) {
	// A recognized var set to empty string must be treated as absent so that
	// tests clearing vars (and empty native output) do not look env-managed.
	_, managed := LoadFromEnv(mapLookup(map[string]string{"EFP_JIRA_DEFAULT_INSTANCE": ""}))
	if managed {
		t.Fatal("an empty value must not count as env-managed")
	}
}

func TestLoadFromEnvSkipsMapField(t *testing.T) {
	// ZephyrConfig.StatusMap is a map kind and is not env-decodable; the
	// decoder must skip it and still apply the sibling scalar without crashing.
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_JIRA_INSTANCES_0_NAME":              "z",
		"EFP_JIRA_INSTANCES_0_ZEPHYR_API_FAMILY": "squad",
		"EFP_JIRA_INSTANCES_0_ZEPHYR_STATUS_MAP": "ignored",
	}))
	if !managed || len(cfg.Jira.Instances) != 1 {
		t.Fatalf("expected one instance: %#v", cfg.Jira.Instances)
	}
	if cfg.Jira.Instances[0].Zephyr.APIFamily != "squad" {
		t.Fatalf("zephyr scalar not decoded: %#v", cfg.Jira.Instances[0].Zephyr)
	}
	if cfg.Jira.Instances[0].Zephyr.StatusMap != nil {
		t.Fatalf("status_map must stay nil (map kind skipped): %#v", cfg.Jira.Instances[0].Zephyr.StatusMap)
	}
}

func TestLoadFromEnvBrowserServe(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_BROWSER_SERVE_PORT":           "8766",
		"EFP_BROWSER_SERVE_ALLOWED_ORIGIN": "https://portal.example.test",
	}))
	if !managed {
		t.Fatal("expected managed=true")
	}
	if cfg.Browser.Serve.Port != 8766 || cfg.Browser.Serve.AllowedOrigin != "https://portal.example.test" {
		t.Fatalf("browser serve env = %#v", cfg.Browser.Serve)
	}
	cfg, _ = LoadFromEnv(mapLookup(map[string]string{"EFP_BROWSER_SERVE_ALLOWED_ORIGIN": "https://portal.example.test"}))
	if cfg.Browser.Serve.Port != DefaultBrowserServePort {
		t.Fatalf("port default after env load = %d", cfg.Browser.Serve.Port)
	}
}

func TestLoadFromEnvAWSAccountMatrix(t *testing.T) {
	cfg, managed := LoadFromEnv(mapLookup(map[string]string{
		"EFP_AWS_ENABLED":                  "true",
		"EFP_AWS_PROVIDER":                 "saml2aws",
		"EFP_AWS_DEFAULT_ACCOUNT":          "cps-dev",
		"EFP_AWS_DEFAULT_REGION":           "eu-west-1",
		"EFP_AWS_SESSION_DURATION_SECONDS": "7200",
		"EFP_AWS_ACCOUNTS_0_NAME":          "cps-dev",
		"EFP_AWS_ACCOUNTS_0_ACCOUNT_ID":    "111111111111",
		"EFP_AWS_ACCOUNTS_0_ROLE":          "ADFS-ReadOnly",
		"EFP_AWS_ACCOUNTS_0_REGIONS_0":     "ap-east-1",
		"EFP_AWS_ACCOUNTS_0_REGIONS_1":     "eu-west-1",
		"EFP_AWS_ACCOUNTS_1_NAME":          "dcc-dev",
		"EFP_AWS_ACCOUNTS_1_ACCOUNT_ID":    "222222222222",
		"EFP_AWS_ACCOUNTS_1_ROLE":          "ADFS-ReadOnly",
		"EFP_AWS_ACCOUNTS_1_ENABLED":       "false",
	}))
	if !managed {
		t.Fatal("expected managed=true")
	}
	if cfg.AWS.EffectiveProvider() != "saml2aws" || cfg.AWS.DefaultAccount != "cps-dev" || cfg.AWS.DefaultRegion != "eu-west-1" || cfg.AWS.EffectiveSessionDurationSeconds() != 7200 {
		t.Fatalf("aws scalars: %#v", cfg.AWS)
	}
	if len(cfg.AWS.Accounts) != 2 {
		t.Fatalf("want 2 accounts, got %#v", cfg.AWS.Accounts)
	}
	first := cfg.AWS.Accounts[0]
	if first.Name != "cps-dev" || first.AccountID != "111111111111" || first.Role != "ADFS-ReadOnly" || first.EffectiveProfile() != "cps-dev" {
		t.Fatalf("account0: %#v", first)
	}
	if len(first.Regions) != 2 || first.Regions[0] != "ap-east-1" || first.Regions[1] != "eu-west-1" {
		t.Fatalf("account0 regions: %#v", first.Regions)
	}
	if cfg.AWS.Accounts[1].IsEnabled() {
		t.Fatalf("account1 must be disabled: %#v", cfg.AWS.Accounts[1])
	}
	if enabled := cfg.AWS.EnabledAccounts(); len(enabled) != 1 || enabled[0].Name != "cps-dev" {
		t.Fatalf("enabled accounts: %#v", enabled)
	}
}

func TestAWSConfigNormalizeTrimsAndDedupesRegions(t *testing.T) {
	cfg := RootConfig{AWS: AWSConfig{Provider: " SAML2AWS ", Accounts: []AWSAccountConfig{{Name: " cps-dev ", AccountID: " 111111111111 ", Regions: []string{" ap-east-1", "", "ap-east-1", "eu-west-1 "}}}}}
	cfg.Normalize()
	acct := cfg.AWS.Accounts[0]
	if cfg.AWS.Provider != "saml2aws" || acct.Name != "cps-dev" || acct.AccountID != "111111111111" {
		t.Fatalf("not trimmed: %#v", cfg.AWS)
	}
	if len(acct.Regions) != 2 || acct.Regions[0] != "ap-east-1" || acct.Regions[1] != "eu-west-1" {
		t.Fatalf("regions not deduped: %#v", acct.Regions)
	}
	if (AWSConfig{}).EffectiveProvider() != AWSProviderADFSAssume || (AWSConfig{}).EffectiveKubeconfigPath() != DefaultAWSKubeconfigPath {
		t.Fatal("defaults must come from the Effective accessors")
	}
}
