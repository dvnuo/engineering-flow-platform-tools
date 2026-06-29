package config

import "strings"

const (
	ProviderGitHubCopilot             = "github_copilot_plugin"
	ProviderAIPlatform                = "ai_platform"
	Provider                          = ProviderAIPlatform
	EndpointKind                      = "responses"
	DefaultBaseURL                    = "https://api.githubcopilot.com"
	DefaultTimeoutSeconds             = 90
	DefaultModel                      = "gpt-5.4-mini"
	DefaultReasoning                  = "medium"
	DefaultOutput                     = "text"
	MaxImageBytes                     = 3145728
	MaxImagesPerCall                  = 1
	DefaultAIPlatformChatURI          = "/v1/api/v1/chat/completions"
	DefaultAIPlatformIB2BURI          = "/dsp/rest-sts/DSP_iB2B/iB2B_tokenTranslator_v2?_action=translate"
	DefaultAIPlatformTokenFile        = "~/.efp/tmp/ai_platform_token"
	DefaultAIPlatformTrustTokenHeader = "X-XXXX-E2E-Trust-Token"
	DefaultAIPlatformTrackingPrefix   = "EFP"
)

var AllowedReasoning = []string{"low", "medium", "high", "xhigh"}
var AllowedMIMETypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}

type Config struct {
	Version    int              `json:"version" yaml:"version"`
	Provider   string           `json:"provider" yaml:"provider"`
	API        APIConfig        `json:"api" yaml:"api"`
	Defaults   DefaultsConfig   `json:"defaults" yaml:"defaults"`
	Limits     LimitsConfig     `json:"limits" yaml:"limits"`
	Auth       AuthConfig       `json:"auth" yaml:"auth"`
	AIPlatform AIPlatformConfig `json:"ai_platform" yaml:"ai_platform"`
	Privacy    PrivacyConfig    `json:"privacy" yaml:"privacy"`
}

type APIConfig struct {
	EndpointKind   string `json:"endpoint_kind" yaml:"endpoint_kind"`
	BaseURL        string `json:"base_url" yaml:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds" yaml:"timeout_seconds"`
	UseSystemProxy bool   `json:"use_system_proxy" yaml:"use_system_proxy"`
}

type DefaultsConfig struct {
	Model     string `json:"model" yaml:"model"`
	Reasoning string `json:"reasoning" yaml:"reasoning"`
	Output    string `json:"output" yaml:"output"`
}

type LimitsConfig struct {
	MaxImageBytes    int64    `json:"max_image_bytes" yaml:"max_image_bytes"`
	MaxImagesPerCall int      `json:"max_images_per_call" yaml:"max_images_per_call"`
	AllowedMIMETypes []string `json:"allowed_mime_types" yaml:"allowed_mime_types"`
}

type AuthConfig struct {
	Method                     string `json:"method" yaml:"method"`
	GitHubHost                 string `json:"github_host" yaml:"github_host"`
	GitHubUser                 string `json:"github_user" yaml:"github_user"`
	GitHubAccessToken          string `json:"github_access_token" yaml:"github_access_token"`
	GitHubAccessTokenExpiresAt string `json:"github_access_token_expires_at" yaml:"github_access_token_expires_at"`
	CopilotToken               string `json:"copilot_token" yaml:"copilot_token"`
	CopilotTokenExpiresAt      string `json:"copilot_token_expires_at" yaml:"copilot_token_expires_at"`
	CopilotTokenFile           string `json:"copilot_token_file" yaml:"copilot_token_file"`
	UpdatedAt                  string `json:"updated_at" yaml:"updated_at"`
}

type AIPlatformConfig struct {
	Chat AIPlatformEndpointConfig `json:"chat" yaml:"chat"`
	IB2B AIPlatformEndpointConfig `json:"ib2b" yaml:"ib2b"`
	Auth AIPlatformAuthConfig     `json:"auth" yaml:"auth"`
}

type AIPlatformEndpointConfig struct {
	Host string `json:"host" yaml:"host"`
	URI  string `json:"uri" yaml:"uri"`
}

type AIPlatformAuthConfig struct {
	Username         string `json:"username" yaml:"username"`
	Password         string `json:"password" yaml:"password"`
	Usercase         string `json:"usercase" yaml:"usercase"`
	Token            string `json:"token" yaml:"token"`
	TokenExpiresAt   string `json:"token_expires_at" yaml:"token_expires_at"`
	TokenFile        string `json:"token_file" yaml:"token_file"`
	TrustTokenHeader string `json:"trust_token_header" yaml:"trust_token_header"`
	TrackingPrefix   string `json:"tracking_prefix" yaml:"tracking_prefix"`
	UpdatedAt        string `json:"updated_at" yaml:"updated_at"`
}

type PrivacyConfig struct {
	StoreRawImage      bool `json:"store_raw_image" yaml:"store_raw_image"`
	StoreRawResponse   bool `json:"store_raw_response" yaml:"store_raw_response"`
	RedactTokensInLogs bool `json:"redact_tokens_in_logs" yaml:"redact_tokens_in_logs"`
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
		Auth:     AuthConfig{Method: "device_code", GitHubHost: "github.com", CopilotTokenFile: "~/.efp/tmp/copilot_token"},
		AIPlatform: AIPlatformConfig{
			Chat: AIPlatformEndpointConfig{URI: DefaultAIPlatformChatURI},
			IB2B: AIPlatformEndpointConfig{URI: DefaultAIPlatformIB2BURI},
			Auth: AIPlatformAuthConfig{TokenFile: DefaultAIPlatformTokenFile, TrustTokenHeader: DefaultAIPlatformTrustTokenHeader, TrackingPrefix: DefaultAIPlatformTrackingPrefix},
		},
		Privacy: PrivacyConfig{StoreRawImage: false, StoreRawResponse: false, RedactTokensInLogs: true},
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
	c.Provider = NormalizeProvider(c.Provider)
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
	if c.Auth.CopilotTokenFile == "" {
		c.Auth.CopilotTokenFile = d.Auth.CopilotTokenFile
	}
	if c.AIPlatform.Chat.URI == "" {
		c.AIPlatform.Chat.URI = d.AIPlatform.Chat.URI
	}
	if c.AIPlatform.IB2B.URI == "" {
		c.AIPlatform.IB2B.URI = d.AIPlatform.IB2B.URI
	}
	if c.AIPlatform.Auth.TokenFile == "" {
		c.AIPlatform.Auth.TokenFile = d.AIPlatform.Auth.TokenFile
	}
	if c.AIPlatform.Auth.TrustTokenHeader == "" {
		c.AIPlatform.Auth.TrustTokenHeader = d.AIPlatform.Auth.TrustTokenHeader
	}
	if c.AIPlatform.Auth.TrackingPrefix == "" {
		c.AIPlatform.Auth.TrackingPrefix = d.AIPlatform.Auth.TrackingPrefix
	}
	if !c.Privacy.RedactTokensInLogs {
		c.Privacy.RedactTokensInLogs = true
	}
}

func NormalizeProvider(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	switch normalized {
	case "":
		return Provider
	case ProviderGitHubCopilot, "github", "copilot":
		return ProviderGitHubCopilot
	case ProviderAIPlatform, "ai-platform", "ai platform":
		return ProviderAIPlatform
	default:
		return strings.TrimSpace(provider)
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
