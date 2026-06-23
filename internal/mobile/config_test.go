package mobile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimeConfigDefaultsAndEnvCredentials(t *testing.T) {
	t.Setenv("BROWSERSTACK_USERNAME", "user")
	t.Setenv("BROWSERSTACK_ACCESS_KEY", "key")
	t.Setenv(EnvStateDir, filepath.Join(t.TempDir(), "state"))
	t.Setenv(EnvArtifactsDir, filepath.Join(t.TempDir(), "artifacts"))
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := LoadRuntimeConfig(missing)
	if err != nil {
		t.Fatalf("LoadRuntimeConfig: %v", err)
	}
	if cfg.Mobile.BrowserStack.APIBaseURL != "https://api-cloud.browserstack.com" {
		t.Fatalf("api base=%s", cfg.Mobile.BrowserStack.APIBaseURL)
	}
	if cfg.Credentials.Username != "user" || cfg.Credentials.AccessKey != "key" {
		t.Fatalf("credentials not loaded from env: %#v", cfg.Credentials)
	}
	if !cfg.Username || !cfg.AccessKey {
		t.Fatalf("credential presence should be boolean true: username=%v access_key=%v", cfg.Username, cfg.AccessKey)
	}
	if cfg.Mobile.Defaults.NetworkMode != "public" {
		t.Fatalf("network default=%s", cfg.Mobile.Defaults.NetworkMode)
	}
}

func TestLoadRuntimeConfigRejectsOffProviderURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nmobile:\n  browserstack:\n    api_base_url: https://evil.example.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRuntimeConfig(path)
	if err == nil {
		t.Fatal("expected off-provider config error")
	}
}

func TestLoadRuntimeConfigResolvesHTTPProxyCredentials(t *testing.T) {
	t.Setenv("BROWSERSTACK_USERNAME", "user")
	t.Setenv("BROWSERSTACK_ACCESS_KEY", "key")
	t.Setenv("BS_PROXY_USER", "proxy-user")
	t.Setenv("BS_PROXY_PASS", "proxy-pass")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`version: 1
mobile:
  browserstack:
    http_proxy:
      proxy_host: proxy.internal
      proxy_port: 8080
      proxy_user_env: BS_PROXY_USER
      proxy_pass_env: BS_PROXY_PASS
      no_proxy_hosts:
        - internal.example
      disable_proxy_discovery: true
      force_proxy: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadRuntimeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPProxy.ProxyHost != "proxy.internal" || cfg.HTTPProxy.ProxyPort != 8080 {
		t.Fatalf("proxy host/port not loaded: %#v", cfg.HTTPProxy)
	}
	if cfg.HTTPProxy.ProxyUser != "proxy-user" || cfg.HTTPProxy.ProxyPass != "proxy-pass" {
		t.Fatalf("proxy credentials not resolved: %#v", cfg.HTTPProxy)
	}
	if !cfg.HTTPProxy.DisableProxyDiscovery || !cfg.HTTPProxy.ForceProxy {
		t.Fatalf("proxy booleans not loaded: %#v", cfg.HTTPProxy)
	}
}
