package config

import "strings"

type RootConfig struct {
	Version    int           `json:"version" yaml:"version"`
	Jira       ProductConfig `json:"jira" yaml:"jira"`
	Confluence ProductConfig `json:"confluence" yaml:"confluence"`
}

type ProductConfig struct {
	DefaultInstance string           `json:"default_instance" yaml:"default_instance"`
	Instances       []InstanceConfig `json:"instances" yaml:"instances"`
}

type InstanceConfig struct {
	Name           string     `json:"name" yaml:"name"`
	BaseURL        string     `json:"base_url" yaml:"base_url"`
	APIVersion     string     `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	RESTPath       string     `json:"rest_path" yaml:"rest_path"`
	Auth           AuthConfig `json:"auth" yaml:"auth"`
	DefaultProject string     `json:"default_project,omitempty" yaml:"default_project,omitempty"`
	DefaultSpace   string     `json:"default_space,omitempty" yaml:"default_space,omitempty"`
	VerifySSL      *bool      `json:"verify_ssl,omitempty" yaml:"verify_ssl,omitempty"`
	CACert         string     `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty"`
}

type AuthConfig struct {
	Type     string `json:"type" yaml:"type"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Token    string `json:"token,omitempty" yaml:"token,omitempty"`
}

func (c *RootConfig) Normalize() {
	norm := func(p *ProductConfig) {
		for i := range p.Instances {
			p.Instances[i].Auth.NormalizeType()
		}
	}
	norm(&c.Jira)
	norm(&c.Confluence)
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
