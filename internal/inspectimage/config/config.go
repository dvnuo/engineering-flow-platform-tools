package config

const (
	Provider              = "github_copilot_plugin"
	EndpointKind          = "responses"
	DefaultBaseURL        = "https://api.githubcopilot.com"
	DefaultTimeoutSeconds = 90
	DefaultModel          = "gpt-5.4"
	DefaultReasoning      = "medium"
	DefaultOutput         = "text"
	MaxImageBytes         = 3145728
	MaxImagesPerCall      = 1
)

var AllowedModels = []string{"gpt-5.4", "gpt-5-mini", "gpt-5.4-mini"}
var AllowedReasoning = []string{"low", "medium", "high", "xhigh"}
var AllowedMIMETypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}

type Config struct {
	Version  int            `json:"version"`
	Provider string         `json:"provider"`
	API      APIConfig      `json:"api"`
	Defaults DefaultsConfig `json:"defaults"`
	Limits   LimitsConfig   `json:"limits"`
	Auth     AuthConfig     `json:"auth"`
	Privacy  PrivacyConfig  `json:"privacy"`
}

type APIConfig struct {
	EndpointKind   string `json:"endpoint_kind"`
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	UseSystemProxy bool   `json:"use_system_proxy"`
}

type DefaultsConfig struct {
	Model     string `json:"model"`
	Reasoning string `json:"reasoning"`
	Output    string `json:"output"`
}

type LimitsConfig struct {
	MaxImageBytes    int64    `json:"max_image_bytes"`
	MaxImagesPerCall int      `json:"max_images_per_call"`
	AllowedMIMETypes []string `json:"allowed_mime_types"`
}

type AuthConfig struct {
	Method                     string `json:"method"`
	GitHubHost                 string `json:"github_host"`
	GitHubUser                 string `json:"github_user"`
	GitHubAccessToken          string `json:"github_access_token"`
	GitHubAccessTokenExpiresAt string `json:"github_access_token_expires_at"`
	CopilotToken               string `json:"copilot_token"`
	CopilotTokenExpiresAt      string `json:"copilot_token_expires_at"`
	UpdatedAt                  string `json:"updated_at"`
}

type PrivacyConfig struct {
	StoreRawImage      bool `json:"store_raw_image"`
	StoreRawResponse   bool `json:"store_raw_response"`
	RedactTokensInLogs bool `json:"redact_tokens_in_logs"`
}

func Default() Config {
	return Config{
		Version:  1,
		Provider: Provider,
		API: APIConfig{
			EndpointKind:   EndpointKind,
			BaseURL:        DefaultBaseURL,
			TimeoutSeconds: DefaultTimeoutSeconds,
			UseSystemProxy: true,
		},
		Defaults: DefaultsConfig{Model: DefaultModel, Reasoning: DefaultReasoning, Output: DefaultOutput},
		Limits:   LimitsConfig{MaxImageBytes: MaxImageBytes, MaxImagesPerCall: MaxImagesPerCall, AllowedMIMETypes: append([]string{}, AllowedMIMETypes...)},
		Auth:     AuthConfig{Method: "device_code", GitHubHost: "github.com"},
		Privacy:  PrivacyConfig{StoreRawImage: false, StoreRawResponse: false, RedactTokensInLogs: true},
	}
}

func (c *Config) FillDefaults() {
	d := Default()
	if c.Version == 0 {
		c.Version = d.Version
	}
	if c.Provider == "" {
		c.Provider = d.Provider
	}
	if c.API.EndpointKind == "" {
		c.API.EndpointKind = d.API.EndpointKind
	}
	if c.API.BaseURL == "" {
		c.API.BaseURL = d.API.BaseURL
	}
	if c.API.TimeoutSeconds == 0 {
		c.API.TimeoutSeconds = d.API.TimeoutSeconds
	}
	if c.Defaults.Model == "" {
		c.Defaults.Model = d.Defaults.Model
	}
	if c.Defaults.Reasoning == "" {
		c.Defaults.Reasoning = d.Defaults.Reasoning
	}
	if c.Defaults.Output == "" {
		c.Defaults.Output = d.Defaults.Output
	}
	if c.Limits.MaxImageBytes == 0 {
		c.Limits.MaxImageBytes = d.Limits.MaxImageBytes
	}
	if c.Limits.MaxImagesPerCall == 0 {
		c.Limits.MaxImagesPerCall = d.Limits.MaxImagesPerCall
	}
	if len(c.Limits.AllowedMIMETypes) == 0 {
		c.Limits.AllowedMIMETypes = d.Limits.AllowedMIMETypes
	}
	if c.Auth.Method == "" {
		c.Auth.Method = d.Auth.Method
	}
	if c.Auth.GitHubHost == "" {
		c.Auth.GitHubHost = d.Auth.GitHubHost
	}
	if !c.Privacy.RedactTokensInLogs {
		c.Privacy.RedactTokensInLogs = true
	}
}

func StringAllowed(v string, allowed []string) bool {
	for _, item := range allowed {
		if v == item {
			return true
		}
	}
	return false
}
