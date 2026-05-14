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
	if a.Type != "" {
		return
	}
	hasUser := a.Username != ""
	hasPwd := a.Password != ""
	hasKey := a.APIKey != ""
	hasToken := a.Token != ""
	switch {
	case hasUser && hasPwd:
		a.Type = "basic_password"
	case hasUser && hasKey:
		a.Type = "basic_api_key"
	case hasUser && hasToken:
		a.Type = "basic_api_key"
		a.APIKey = a.Token
		a.Token = ""
	case hasToken:
		a.Type = "bearer_token"
	}
	a.Type = strings.TrimSpace(a.Type)
}
