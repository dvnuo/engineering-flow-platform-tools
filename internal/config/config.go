package config

import "strings"

type RootConfig struct {
	Version    int           `json:"version" yaml:"version"`
	Jira       ProductConfig `json:"jira" yaml:"jira"`
	Confluence ProductConfig `json:"confluence" yaml:"confluence"`
	Jenkins    ProductConfig `json:"jenkins" yaml:"jenkins"`
	AWS        AWSConfig     `json:"aws" yaml:"aws"`
	Visual     VisualConfig  `json:"visual" yaml:"visual"`
	Mobile     MobileConfig  `json:"mobile" yaml:"mobile"`
}

type AWSConfig struct {
	Enabled  *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Domain   string `json:"domain,omitempty" yaml:"domain,omitempty"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type VisualConfig struct {
	TemplateDir string         `json:"template_dir" yaml:"template_dir"`
	Defaults    VisualDefaults `json:"defaults" yaml:"defaults"`
}

type VisualDefaults struct {
	OfflineStrict *bool  `json:"offline_strict,omitempty" yaml:"offline_strict,omitempty"`
	DataMode      string `json:"data_mode,omitempty" yaml:"data_mode,omitempty"`
}

type MobileConfig struct {
	DefaultProvider string             `json:"default_provider" yaml:"default_provider"`
	StateDir        string             `json:"state_dir" yaml:"state_dir"`
	ArtifactsDir    string             `json:"artifacts_dir" yaml:"artifacts_dir"`
	RetentionHours  int                `json:"retention_hours" yaml:"retention_hours"`
	Defaults        MobileDefaults     `json:"defaults" yaml:"defaults"`
	BrowserStack    MobileBrowserStack `json:"browserstack" yaml:"browserstack"`
}

type MobileDefaults struct {
	Platform                 string `json:"platform" yaml:"platform"`
	NetworkMode              string `json:"network_mode" yaml:"network_mode"`
	IdleTimeoutSeconds       int    `json:"idle_timeout_seconds" yaml:"idle_timeout_seconds"`
	NewCommandTimeoutSeconds int    `json:"new_command_timeout_seconds" yaml:"new_command_timeout_seconds"`
	InteractiveDebugging     *bool  `json:"interactive_debugging,omitempty" yaml:"interactive_debugging,omitempty"`
	Video                    *bool  `json:"video,omitempty" yaml:"video,omitempty"`
}

type MobileBrowserStack struct {
	APIBaseURL    string            `json:"api_base_url" yaml:"api_base_url"`
	AppiumBaseURL string            `json:"appium_base_url" yaml:"appium_base_url"`
	UsernameEnv   string            `json:"username_env" yaml:"username_env"`
	AccessKeyEnv  string            `json:"access_key_env" yaml:"access_key_env"`
	Username      string            `json:"username,omitempty" yaml:"username,omitempty"`
	AccessKey     string            `json:"access_key,omitempty" yaml:"access_key,omitempty"`
	VerifySSL     *bool             `json:"verify_ssl,omitempty" yaml:"verify_ssl,omitempty"`
	CACert        string            `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty"`
	Local         MobileLocalConfig `json:"local" yaml:"local"`
}

type MobileLocalConfig struct {
	Mode                  string   `json:"mode" yaml:"mode"`
	Binary                string   `json:"binary" yaml:"binary"`
	BinaryEnv             string   `json:"binary_env" yaml:"binary_env"`
	DefaultHoldMinutes    int      `json:"default_hold_minutes" yaml:"default_hold_minutes"`
	MaxHoldMinutes        int      `json:"max_hold_minutes" yaml:"max_hold_minutes"`
	ReadyTimeoutSeconds   int      `json:"ready_timeout_seconds" yaml:"ready_timeout_seconds"`
	HeartbeatSeconds      int      `json:"heartbeat_seconds" yaml:"heartbeat_seconds"`
	ForceLocal            *bool    `json:"force_local,omitempty" yaml:"force_local,omitempty"`
	DisableProxyDiscovery *bool    `json:"disable_proxy_discovery,omitempty" yaml:"disable_proxy_discovery,omitempty"`
	ForceProxy            *bool    `json:"force_proxy,omitempty" yaml:"force_proxy,omitempty"`
	ProxyHost             string   `json:"proxy_host,omitempty" yaml:"proxy_host,omitempty"`
	ProxyPort             int      `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	ProxyUserEnv          string   `json:"proxy_user_env,omitempty" yaml:"proxy_user_env,omitempty"`
	ProxyPassEnv          string   `json:"proxy_pass_env,omitempty" yaml:"proxy_pass_env,omitempty"`
	OnlyAutomate          *bool    `json:"only_automate,omitempty" yaml:"only_automate,omitempty"`
	Force                 *bool    `json:"force,omitempty" yaml:"force,omitempty"`
	IncludeHosts          []string `json:"include_hosts" yaml:"include_hosts"`
	ExcludeHosts          []string `json:"exclude_hosts" yaml:"exclude_hosts"`
}

type ProductConfig struct {
	DefaultInstance string           `json:"default_instance" yaml:"default_instance"`
	Instances       []InstanceConfig `json:"instances" yaml:"instances"`
}

type InstanceConfig struct {
	Name           string       `json:"name" yaml:"name"`
	BaseURL        string       `json:"base_url" yaml:"base_url"`
	APIVersion     string       `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	RESTPath       string       `json:"rest_path" yaml:"rest_path"`
	Auth           AuthConfig   `json:"auth" yaml:"auth"`
	DefaultProject string       `json:"default_project,omitempty" yaml:"default_project,omitempty"`
	DefaultSpace   string       `json:"default_space,omitempty" yaml:"default_space,omitempty"`
	VerifySSL      *bool        `json:"verify_ssl,omitempty" yaml:"verify_ssl,omitempty"`
	CACert         string       `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty"`
	CrumbMode      string       `json:"crumb_mode,omitempty" yaml:"crumb_mode,omitempty"`
	Zephyr         ZephyrConfig `json:"zephyr,omitempty" yaml:"zephyr,omitempty"`
}

type AuthConfig struct {
	Type     string `json:"type" yaml:"type"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Token    string `json:"token,omitempty" yaml:"token,omitempty"`
}

type ZephyrConfig struct {
	Enabled          *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	APIFamily        string         `json:"api_family,omitempty" yaml:"api_family,omitempty"`
	RESTPath         string         `json:"rest_path,omitempty" yaml:"rest_path,omitempty"`
	DefaultVersionID string         `json:"default_version_id,omitempty" yaml:"default_version_id,omitempty"`
	StatusMap        map[string]int `json:"status_map,omitempty" yaml:"status_map,omitempty"`
	StrictStatus     *bool          `json:"strict_status,omitempty" yaml:"strict_status,omitempty"`
}

func (c *RootConfig) Normalize() {
	norm := func(p *ProductConfig) {
		for i := range p.Instances {
			p.Instances[i].Auth.NormalizeType()
		}
	}
	norm(&c.Jira)
	norm(&c.Confluence)
	norm(&c.Jenkins)
	c.Mobile.Normalize()
}

func (m *MobileConfig) Normalize() {
	if strings.TrimSpace(m.DefaultProvider) == "" {
		m.DefaultProvider = "browserstack"
	}
	if strings.TrimSpace(m.StateDir) == "" {
		m.StateDir = "~/.efp/mobile"
	}
	if strings.TrimSpace(m.ArtifactsDir) == "" {
		m.ArtifactsDir = "~/.efp/artifacts/mobile"
	}
	if m.RetentionHours == 0 {
		m.RetentionHours = 72
	}
	if strings.TrimSpace(m.Defaults.Platform) == "" {
		m.Defaults.Platform = "android"
	}
	if strings.TrimSpace(m.Defaults.NetworkMode) == "" {
		m.Defaults.NetworkMode = "public"
	}
	if m.Defaults.IdleTimeoutSeconds == 0 {
		m.Defaults.IdleTimeoutSeconds = 300
	}
	if m.Defaults.NewCommandTimeoutSeconds == 0 {
		m.Defaults.NewCommandTimeoutSeconds = 300
	}
	if m.Defaults.InteractiveDebugging == nil {
		v := true
		m.Defaults.InteractiveDebugging = &v
	}
	if m.Defaults.Video == nil {
		v := true
		m.Defaults.Video = &v
	}
	if strings.TrimSpace(m.BrowserStack.APIBaseURL) == "" {
		m.BrowserStack.APIBaseURL = "https://api-cloud.browserstack.com"
	}
	if strings.TrimSpace(m.BrowserStack.AppiumBaseURL) == "" {
		m.BrowserStack.AppiumBaseURL = "https://hub.browserstack.com/wd/hub"
	}
	if strings.TrimSpace(m.BrowserStack.UsernameEnv) == "" {
		m.BrowserStack.UsernameEnv = "BROWSERSTACK_USERNAME"
	}
	if strings.TrimSpace(m.BrowserStack.AccessKeyEnv) == "" {
		m.BrowserStack.AccessKeyEnv = "BROWSERSTACK_ACCESS_KEY"
	}
	if m.BrowserStack.VerifySSL == nil {
		v := true
		m.BrowserStack.VerifySSL = &v
	}
	if strings.TrimSpace(m.BrowserStack.Local.Mode) == "" {
		m.BrowserStack.Local.Mode = "managed"
	}
	if strings.TrimSpace(m.BrowserStack.Local.Binary) == "" {
		m.BrowserStack.Local.Binary = "BrowserStackLocal"
	}
	if strings.TrimSpace(m.BrowserStack.Local.BinaryEnv) == "" {
		m.BrowserStack.Local.BinaryEnv = "BROWSERSTACK_LOCAL_BINARY"
	}
	if m.BrowserStack.Local.DefaultHoldMinutes == 0 {
		m.BrowserStack.Local.DefaultHoldMinutes = 10
	}
	if m.BrowserStack.Local.MaxHoldMinutes == 0 {
		m.BrowserStack.Local.MaxHoldMinutes = 30
	}
	if m.BrowserStack.Local.ReadyTimeoutSeconds == 0 {
		m.BrowserStack.Local.ReadyTimeoutSeconds = 30
	}
	if m.BrowserStack.Local.HeartbeatSeconds == 0 {
		m.BrowserStack.Local.HeartbeatSeconds = 60
	}
	if m.BrowserStack.Local.ForceLocal == nil {
		v := false
		m.BrowserStack.Local.ForceLocal = &v
	}
}

func (a *AuthConfig) NormalizeType() {
	a.Type = NormalizeAuthType(*a)
	if a.Type == "basic_api_key" && a.APIKey == "" && a.Token != "" {
		a.APIKey = a.Token
		if a.Username != "" {
			a.Token = ""
		}
	}
}

func NormalizeAuthType(a AuthConfig) string {
	t := strings.TrimSpace(strings.ToLower(a.Type))
	switch t {
	case "pat", "bearer", "token":
		return "bearer_token"
	case "basic_token", "api_key":
		return "basic_api_key"
	case "basic_password", "basic_api_key", "bearer_token":
		return t
	case "":
	default:
		return t
	}
	hasUser := a.Username != ""
	hasPwd := a.Password != ""
	hasKey := a.APIKey != ""
	hasToken := a.Token != ""
	switch {
	case hasUser && hasPwd:
		return "basic_password"
	case hasUser && hasKey:
		return "basic_api_key"
	case hasUser && hasToken:
		return "basic_api_key"
	case hasToken:
		return "bearer_token"
	}
	return ""
}
