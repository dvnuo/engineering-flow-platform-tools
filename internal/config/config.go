package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Auth struct {
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Token    string `json:"token,omitempty"`
}

type JiraInstance struct {
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	APIVersion     string `json:"api_version"`
	RestPath       string `json:"rest_path"`
	Auth           Auth   `json:"auth"`
	DefaultProject string `json:"default_project,omitempty"`
	VerifySSL      bool   `json:"verify_ssl"`
	CACert         string `json:"ca_cert,omitempty"`
}

type ConfluenceInstance struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	RestPath     string `json:"rest_path"`
	Auth         Auth   `json:"auth"`
	DefaultSpace string `json:"default_space,omitempty"`
	VerifySSL    bool   `json:"verify_ssl"`
	CACert       string `json:"ca_cert,omitempty"`
}

type JiraSection struct {
	DefaultInstance string         `json:"default_instance"`
	Instances       []JiraInstance `json:"instances"`
}

type ConfluenceSection struct {
	DefaultInstance string               `json:"default_instance"`
	Instances       []ConfluenceInstance `json:"instances"`
}

type Config struct {
	Version    int               `json:"version"`
	Jira       JiraSection       `json:"jira"`
	Confluence ConfluenceSection `json:"confluence"`
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return nil
}
