package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type unifiedConfig struct {
	Version      int                `json:"version" yaml:"version"`
	Copilot      copilotConfig      `json:"copilot" yaml:"copilot"`
	InspectImage inspectImageConfig `json:"inspect_image" yaml:"inspect_image"`
}

type copilotConfig struct {
	Provider string     `json:"provider" yaml:"provider"`
	Auth     AuthConfig `json:"auth" yaml:"auth"`
}

type inspectImageConfig struct {
	API      APIConfig      `json:"api" yaml:"api"`
	Defaults DefaultsConfig `json:"defaults" yaml:"defaults"`
	Limits   LimitsConfig   `json:"limits" yaml:"limits"`
	Privacy  PrivacyConfig  `json:"privacy" yaml:"privacy"`
}

type copilotTokenFile struct {
	CopilotToken          string `json:"copilot_token" yaml:"copilot_token"`
	CopilotTokenExpiresAt string `json:"copilot_token_expires_at" yaml:"copilot_token_expires_at"`
	UpdatedAt             string `json:"updated_at" yaml:"updated_at"`
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	var root unifiedConfig
	var raw map[string]any
	_ = yaml.Unmarshal(b, &raw)
	_, hasInspectImage := raw["inspect_image"]
	_, hasCopilot := raw["copilot"]
	if err := yaml.Unmarshal(b, &root); err == nil && (hasInspectImage || hasCopilot) {
		c = Default()
		if root.Version != 0 {
			c.Version = root.Version
		}
		if root.Copilot.Provider != "" {
			c.Provider = root.Copilot.Provider
		}
		c.API = root.InspectImage.API
		c.Defaults = root.InspectImage.Defaults
		c.Limits = root.InspectImage.Limits
		c.Privacy = root.InspectImage.Privacy
		c.Auth = root.Copilot.Auth
		c.FillDefaults()
		if err := loadCopilotToken(&c); err != nil {
			return c, err
		}
		return c, nil
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	c.FillDefaults()
	return c, nil
}

func LoadOrDefault(path string) (Config, error) {
	c, err := Load(path)
	if err == nil {
		return c, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	return c, err
}

func Save(path string, c Config) error {
	if path == "" {
		return errors.New("config_path_empty")
	}
	c.FillDefaults()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := saveCopilotToken(c); err != nil {
		return err
	}
	doc, root := loadYAMLDocument(path)
	removeMappingKeys(root, "provider", "api", "defaults", "limits", "auth", "privacy")
	auth := c.Auth
	auth.CopilotToken = ""
	if err := setMappingValue(root, "version", c.Version); err != nil {
		return err
	}
	if err := setMappingValue(root, "copilot", copilotConfig{Provider: c.Provider, Auth: auth}); err != nil {
		return err
	}
	if err := setMappingValue(root, "inspect_image", inspectImageConfig{API: c.API, Defaults: c.Defaults, Limits: c.Limits, Privacy: c.Privacy}); err != nil {
		return err
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func PermissionOK(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().Perm()&0o077 == 0
}

func loadCopilotToken(c *Config) error {
	path, err := expandPath(c.Auth.CopilotTokenFile)
	if err != nil || path == "" {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return nil
	}
	node := doc.Content[0]
	if node.Kind == yaml.ScalarNode {
		c.Auth.CopilotToken = strings.TrimSpace(node.Value)
		return nil
	}
	var token copilotTokenFile
	if err := node.Decode(&token); err != nil {
		return err
	}
	c.Auth.CopilotToken = token.CopilotToken
	c.Auth.CopilotTokenExpiresAt = token.CopilotTokenExpiresAt
	return nil
}

func saveCopilotToken(c Config) error {
	path, err := expandPath(c.Auth.CopilotTokenFile)
	if err != nil || path == "" {
		return err
	}
	if c.Auth.CopilotToken == "" && c.Auth.CopilotTokenExpiresAt == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	token := copilotTokenFile{CopilotToken: c.Auth.CopilotToken, CopilotTokenExpiresAt: c.Auth.CopilotTokenExpiresAt, UpdatedAt: c.Auth.UpdatedAt}
	b, err := yaml.Marshal(token)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
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

func loadYAMLDocument(path string) (*yaml.Node, *yaml.Node) {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	root := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = []*yaml.Node{root}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return doc, root
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal(b, &parsed); err != nil || len(parsed.Content) == 0 || parsed.Content[0].Kind != yaml.MappingNode {
		return doc, root
	}
	return &parsed, parsed.Content[0]
}

func removeMappingKeys(root *yaml.Node, keys ...string) {
	remove := map[string]bool{}
	for _, key := range keys {
		remove[key] = true
	}
	content := root.Content[:0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if remove[root.Content[i].Value] {
			continue
		}
		content = append(content, root.Content[i], root.Content[i+1])
	}
	root.Content = content
}

func setMappingValue(root *yaml.Node, key string, value any) error {
	newValue, err := yamlValueNode(value)
	if err != nil {
		return err
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = mergeNodeComments(root.Content[i+1], newValue)
			return nil
		}
	}
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, newValue)
	return nil
}

func yamlValueNode(value any) (*yaml.Node, error) {
	b, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode}, nil
	}
	return doc.Content[0], nil
}

func mergeNodeComments(old, new *yaml.Node) *yaml.Node {
	if old == nil || new == nil {
		return new
	}
	copyComments(old, new)
	if old.Kind == new.Kind && old.Style != 0 {
		new.Style = old.Style
	}
	switch new.Kind {
	case yaml.MappingNode:
		oldValues := map[string]*yaml.Node{}
		oldKeys := map[string]*yaml.Node{}
		for i := 0; i+1 < len(old.Content); i += 2 {
			oldKeys[old.Content[i].Value] = old.Content[i]
			oldValues[old.Content[i].Value] = old.Content[i+1]
		}
		for i := 0; i+1 < len(new.Content); i += 2 {
			key := new.Content[i].Value
			if oldKey := oldKeys[key]; oldKey != nil {
				copyComments(oldKey, new.Content[i])
			}
			if oldValue := oldValues[key]; oldValue != nil {
				new.Content[i+1] = mergeNodeComments(oldValue, new.Content[i+1])
			}
		}
	case yaml.SequenceNode:
		for i := 0; i < len(new.Content) && i < len(old.Content); i++ {
			new.Content[i] = mergeNodeComments(old.Content[i], new.Content[i])
		}
	}
	return new
}

func copyComments(from, to *yaml.Node) {
	if to.HeadComment == "" {
		to.HeadComment = from.HeadComment
	}
	if to.LineComment == "" {
		to.LineComment = from.LineComment
	}
	if to.FootComment == "" {
		to.FootComment = from.FootComment
	}
}
