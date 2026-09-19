package config

import (
	"strings"

	"engineering-flow-platform-tools/internal/configenv"
)

type RootConfig struct {
	Version    int           `json:"version" yaml:"version"`
	Jira       ProductConfig `json:"jira" yaml:"jira"`
	Confluence ProductConfig `json:"confluence" yaml:"confluence"`
	Jenkins    ProductConfig `json:"jenkins" yaml:"jenkins"`
	AWS        AWSConfig     `json:"aws" yaml:"aws"`
	Browser    BrowserConfig `json:"browser" yaml:"browser"`
	Mobile     MobileConfig  `json:"mobile-auto" yaml:"mobile-auto"`

	envSnapshot *configenv.Snapshot
}

type BrowserConfig struct {
	Bookmarks BrowserBookmarksConfig `json:"bookmarks" yaml:"bookmarks"`
	Serve     BrowserServeConfig     `json:"serve" yaml:"serve"`
}

// DefaultBrowserServePort is the loopback port `browser serve` listens on when
// neither --port, browser.serve.port, nor EFP_BROWSER_SERVE_PORT is set.
const DefaultBrowserServePort = 8765

// BrowserServeConfig holds defaults for `browser serve`, the local bridge used
// by the EFP Portal local browser connector. The env equivalents derived from
// the json tags are EFP_BROWSER_SERVE_PORT and EFP_BROWSER_SERVE_ALLOWED_ORIGIN.
type BrowserServeConfig struct {
	Port          int    `json:"port" yaml:"port"`
	AllowedOrigin string `json:"allowed_origin,omitempty" yaml:"allowed_origin,omitempty"`
}

type BrowserBookmarksConfig struct {
	Sources []BrowserBookmarkSource `json:"sources" yaml:"sources"`
}

type BrowserBookmarkSource struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url" yaml:"url"`
}

// AWS authorization providers accepted in AWSConfig.Provider. The env
// equivalent derived from the json tag is EFP_AWS_PROVIDER.
const (
	AWSProviderADFSAssume = "adfs-assume" // enterprise adfs-assume binary (default)
	AWSProviderSAML2AWS   = "saml2aws"    // github.com/Versent/saml2aws ADFS flow
	AWSProviderAssumeRole = "assume-role" // role chaining from an already-authenticated source profile
)

// DefaultAWSSessionDurationSeconds is the SAML session length requested when
// aws.session_duration_seconds is unset.
const DefaultAWSSessionDurationSeconds = 3600

// DefaultAWSKubeconfigPath is where `aws-auth eks kubeconfig` writes cluster
// contexts when neither --kubeconfig, KUBECONFIG, nor aws.kubeconfig_path is set.
// It deliberately lives outside the agent workspace, like EFP_CONFIG.
const DefaultAWSKubeconfigPath = "~/.efp/kube/config"

// AWSConfig is the `aws` node of the shared EFP config. Domain/username/password
// are the enterprise directory credentials the ADFS providers exchange for AWS
// credentials; Accounts is the account matrix agents may log in to. Every
// scalar has an EFP_AWS_<FIELD> env equivalent and accounts are addressed as
// EFP_AWS_ACCOUNTS_<i>_<FIELD> (regions as EFP_AWS_ACCOUNTS_<i>_REGIONS_<j>).
type AWSConfig struct {
	Enabled                *bool              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Provider               string             `json:"provider,omitempty" yaml:"provider,omitempty"`
	Domain                 string             `json:"domain,omitempty" yaml:"domain,omitempty"`
	Username               string             `json:"username,omitempty" yaml:"username,omitempty"`
	Password               string             `json:"password,omitempty" yaml:"password,omitempty"`
	IdpURL                 string             `json:"idp_url,omitempty" yaml:"idp_url,omitempty"`
	SourceProfile          string             `json:"source_profile,omitempty" yaml:"source_profile,omitempty"`
	DefaultAccount         string             `json:"default_account,omitempty" yaml:"default_account,omitempty"`
	DefaultRegion          string             `json:"default_region,omitempty" yaml:"default_region,omitempty"`
	SessionDurationSeconds int                `json:"session_duration_seconds,omitempty" yaml:"session_duration_seconds,omitempty"`
	KubeconfigPath         string             `json:"kubeconfig_path,omitempty" yaml:"kubeconfig_path,omitempty"`
	Accounts               []AWSAccountConfig `json:"accounts,omitempty" yaml:"accounts,omitempty"`
}

// AWSAccountConfig is one entry of the account matrix. Profile defaults to Name
// so every account owns its own AWS CLI profile and agents can query several
// accounts without re-authenticating in between.
type AWSAccountConfig struct {
	Name      string   `json:"name" yaml:"name"`
	AccountID string   `json:"account_id" yaml:"account_id"`
	Role      string   `json:"role,omitempty" yaml:"role,omitempty"`
	RoleARN   string   `json:"role_arn,omitempty" yaml:"role_arn,omitempty"`
	Regions   []string `json:"regions,omitempty" yaml:"regions,omitempty"`
	Profile   string   `json:"profile,omitempty" yaml:"profile,omitempty"`
	Enabled   *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// Normalize trims whitespace and drops empty regions. Defaults (provider,
// session duration, kubeconfig path, per-account profile) are resolved by the
// Effective* accessors instead of being materialized here, so a saved config
// file never gains keys the user did not write.
func (a *AWSConfig) Normalize() {
	a.Provider = strings.ToLower(strings.TrimSpace(a.Provider))
	a.Domain = strings.TrimSpace(a.Domain)
	a.Username = strings.TrimSpace(a.Username)
	a.IdpURL = strings.TrimSpace(a.IdpURL)
	a.SourceProfile = strings.TrimSpace(a.SourceProfile)
	a.DefaultAccount = strings.TrimSpace(a.DefaultAccount)
	a.DefaultRegion = strings.TrimSpace(a.DefaultRegion)
	a.KubeconfigPath = strings.TrimSpace(a.KubeconfigPath)
	for i := range a.Accounts {
		acct := &a.Accounts[i]
		acct.Name = strings.TrimSpace(acct.Name)
		acct.AccountID = strings.TrimSpace(acct.AccountID)
		acct.Role = strings.TrimSpace(acct.Role)
		acct.RoleARN = strings.TrimSpace(acct.RoleARN)
		acct.Profile = strings.TrimSpace(acct.Profile)
		regions := make([]string, 0, len(acct.Regions))
		seen := map[string]bool{}
		for _, region := range acct.Regions {
			region = strings.TrimSpace(region)
			if region == "" || seen[region] {
				continue
			}
			seen[region] = true
			regions = append(regions, region)
		}
		if len(regions) == 0 {
			acct.Regions = nil
		} else {
			acct.Regions = regions
		}
	}
}

// EffectiveProvider returns the configured provider or the adfs-assume default.
func (a AWSConfig) EffectiveProvider() string {
	if p := strings.ToLower(strings.TrimSpace(a.Provider)); p != "" {
		return p
	}
	return AWSProviderADFSAssume
}

// EffectiveSessionDurationSeconds returns the configured SAML session length or
// the default.
func (a AWSConfig) EffectiveSessionDurationSeconds() int {
	if a.SessionDurationSeconds > 0 {
		return a.SessionDurationSeconds
	}
	return DefaultAWSSessionDurationSeconds
}

// EffectiveKubeconfigPath returns aws.kubeconfig_path or the default location.
func (a AWSConfig) EffectiveKubeconfigPath() string {
	if p := strings.TrimSpace(a.KubeconfigPath); p != "" {
		return p
	}
	return DefaultAWSKubeconfigPath
}

// EnabledAccounts returns the account matrix entries that are not disabled and
// carry a name or account id.
func (a AWSConfig) EnabledAccounts() []AWSAccountConfig {
	out := make([]AWSAccountConfig, 0, len(a.Accounts))
	for _, acct := range a.Accounts {
		if !acct.IsEnabled() || (acct.Name == "" && acct.AccountID == "") {
			continue
		}
		out = append(out, acct)
	}
	return out
}

// IsEnabled reports whether the account entry may be used (enabled is opt-out).
func (acct AWSAccountConfig) IsEnabled() bool {
	return acct.Enabled == nil || *acct.Enabled
}

// EffectiveProfile returns the AWS CLI profile the account's credentials are
// written to: the explicit profile, else the account name, else the account id.
func (acct AWSAccountConfig) EffectiveProfile() string {
	if p := strings.TrimSpace(acct.Profile); p != "" {
		return p
	}
	if n := strings.TrimSpace(acct.Name); n != "" {
		return n
	}
	return strings.TrimSpace(acct.AccountID)
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
	HTTPProxy     MobileHTTPProxy   `json:"http_proxy" yaml:"http_proxy"`
	Local         MobileLocalConfig `json:"local" yaml:"local"`
}

type MobileHTTPProxy struct {
	ProxyHost             string   `json:"proxy_host,omitempty" yaml:"proxy_host,omitempty"`
	ProxyPort             int      `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	ProxyUserEnv          string   `json:"proxy_user_env,omitempty" yaml:"proxy_user_env,omitempty"`
	ProxyPassEnv          string   `json:"proxy_pass_env,omitempty" yaml:"proxy_pass_env,omitempty"`
	NoProxyHosts          []string `json:"no_proxy_hosts,omitempty" yaml:"no_proxy_hosts,omitempty"`
	DisableProxyDiscovery *bool    `json:"disable_proxy_discovery,omitempty" yaml:"disable_proxy_discovery,omitempty"`
	ForceProxy            *bool    `json:"force_proxy,omitempty" yaml:"force_proxy,omitempty"`
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
	c.AWS.Normalize()
	c.Browser.Normalize()
	c.Mobile.Normalize()
}

func (b *BrowserConfig) Normalize() {
	if b.Serve.Port == 0 {
		b.Serve.Port = DefaultBrowserServePort
	}
	b.Serve.AllowedOrigin = strings.TrimSpace(b.Serve.AllowedOrigin)
}

func (m *MobileConfig) Normalize() {
	if strings.TrimSpace(m.DefaultProvider) == "" {
		m.DefaultProvider = "browserstack"
	}
	if strings.TrimSpace(m.StateDir) == "" {
		m.StateDir = "~/.efp/mobile-auto"
	}
	if strings.TrimSpace(m.ArtifactsDir) == "" {
		m.ArtifactsDir = "~/.efp/artifacts/mobile-auto"
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
