package mobile

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/httpclient"
)

const EnvStateDir = "MOBILE_STATE_DIR"
const EnvArtifactsDir = "MOBILE_ARTIFACTS_DIR"

type RuntimeConfig struct {
	Path        string                   `json:"path,omitempty"`
	Mobile      config.MobileConfig      `json:"mobile"`
	Username    bool                     `json:"username_present,omitempty"`
	AccessKey   bool                     `json:"access_key_present,omitempty"`
	Credentials Credentials              `json:"-"`
	HTTPProxy   httpclient.ProxySettings `json:"-"`
	Warnings    []string                 `json:"warnings,omitempty"`
}

type Credentials struct {
	Username  string
	AccessKey string
}

func LoadRuntimeConfig(flagPath string) (RuntimeConfig, error) {
	path, err := config.ResolvePath(flagPath)
	if err != nil {
		return RuntimeConfig{}, NewError("config_error", "could not resolve config path", "Set EFP_CONFIG or pass --config.", 400)
	}
	root, err := config.Load(path)
	warnings := []string{}
	if err != nil {
		if !os.IsNotExist(err) {
			return RuntimeConfig{}, NewError("config_error", "could not read config file", "Check EFP_CONFIG, file permissions, and YAML syntax.", 400)
		}
		root = config.RootConfig{}
		root.Normalize()
		warnings = append(warnings, "config file not found; using mobile defaults and environment credentials")
	}
	m := root.Mobile
	m.Normalize()
	if override := os.Getenv(EnvStateDir); strings.TrimSpace(override) != "" {
		m.StateDir = override
	}
	if override := os.Getenv(EnvArtifactsDir); strings.TrimSpace(override) != "" {
		m.ArtifactsDir = override
	}
	username := firstNonEmpty(os.Getenv(m.BrowserStack.UsernameEnv), m.BrowserStack.Username)
	accessKey := firstNonEmpty(os.Getenv(m.BrowserStack.AccessKeyEnv), m.BrowserStack.AccessKey)
	if err := validateProviderURL(m.BrowserStack.APIBaseURL, "api-cloud.browserstack.com"); err != nil {
		return RuntimeConfig{}, err
	}
	if err := validateProviderURL(m.BrowserStack.AppiumBaseURL, "hub.browserstack.com"); err != nil {
		return RuntimeConfig{}, err
	}
	httpProxy, err := loadHTTPProxy(m.BrowserStack.HTTPProxy)
	if err != nil {
		return RuntimeConfig{}, err
	}
	stateDir, err := ExpandPath(m.StateDir)
	if err != nil {
		return RuntimeConfig{}, err
	}
	artifactsDir, err := ExpandPath(m.ArtifactsDir)
	if err != nil {
		return RuntimeConfig{}, err
	}
	m.StateDir = stateDir
	m.ArtifactsDir = artifactsDir
	return RuntimeConfig{
		Path:        path,
		Mobile:      m,
		Username:    present(username),
		AccessKey:   present(accessKey),
		Credentials: Credentials{Username: username, AccessKey: accessKey},
		HTTPProxy:   httpProxy,
		Warnings:    warnings,
	}, nil
}

func loadHTTPProxy(cfg config.MobileHTTPProxy) (httpclient.ProxySettings, error) {
	proxy := httpclient.ProxySettings{
		ProxyHost:    cfg.ProxyHost,
		ProxyPort:    cfg.ProxyPort,
		NoProxyHosts: append([]string{}, cfg.NoProxyHosts...),
	}
	if cfg.DisableProxyDiscovery != nil {
		proxy.DisableProxyDiscovery = *cfg.DisableProxyDiscovery
	}
	if cfg.ForceProxy != nil {
		proxy.ForceProxy = *cfg.ForceProxy
	}
	if strings.TrimSpace(cfg.ProxyUserEnv) != "" {
		value, ok := os.LookupEnv(strings.TrimSpace(cfg.ProxyUserEnv))
		if !ok || strings.TrimSpace(value) == "" {
			return proxy, NewError("config_error", "BrowserStack HTTP proxy username env var is not set", "Set "+strings.TrimSpace(cfg.ProxyUserEnv)+" or remove mobile.browserstack.http_proxy.proxy_user_env.", 400)
		}
		proxy.ProxyUser = value
	}
	if strings.TrimSpace(cfg.ProxyPassEnv) != "" {
		value, ok := os.LookupEnv(strings.TrimSpace(cfg.ProxyPassEnv))
		if !ok || strings.TrimSpace(value) == "" {
			return proxy, NewError("config_error", "BrowserStack HTTP proxy password env var is not set", "Set "+strings.TrimSpace(cfg.ProxyPassEnv)+" or remove mobile.browserstack.http_proxy.proxy_pass_env.", 400)
		}
		proxy.ProxyPass = value
	}
	return proxy, nil
}

func RequireCredentials(c RuntimeConfig) error {
	if strings.TrimSpace(c.Credentials.Username) == "" || strings.TrimSpace(c.Credentials.AccessKey) == "" {
		return NewError("auth_error", "BrowserStack credentials are missing", "Set BROWSERSTACK_USERNAME and BROWSERSTACK_ACCESS_KEY or configure mobile.browserstack username/access_key.", 401)
	}
	return nil
}

func ExpandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func validateProviderURL(raw, expectedHost string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return NewError("config_error", "invalid mobile provider URL", "Use an absolute BrowserStack URL.", 400)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "https" && !isLoopback(host) {
		return NewError("config_error", "mobile provider URL must use https", "Only loopback HTTP is allowed for tests.", 400)
	}
	if host != strings.ToLower(expectedHost) && !strings.HasSuffix(host, ".browserstack.com") && !isLoopback(host) {
		return NewError("config_error", "off-provider mobile URL rejected", "Use a BrowserStack-owned host.", 400)
	}
	return nil
}

func isLoopback(host string) bool {
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func present(value string) bool {
	return strings.TrimSpace(value) != ""
}
