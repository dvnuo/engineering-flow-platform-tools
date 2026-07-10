package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EnvSourceConfigJSON is the source LoadShared reports when config came from EFP_CONFIG_JSON.
const EnvSourceConfigJSON = "env:" + EnvConfigJSON

// LoadShared resolves the shared EFP config with precedence:
// explicit flag path > EFP_CONFIG_JSON env blob > EFP_CONFIG/ATLASSIAN_CONFIG file path > default path.
// An invalid EFP_CONFIG_JSON is a hard error, never a silent fallback to file sources.
func LoadShared(flagPath string) (RootConfig, string, error) {
	if flagPath != "" {
		c, err := Load(flagPath)
		return c, flagPath, err
	}
	if blob := strings.TrimSpace(os.Getenv(EnvConfigJSON)); blob != "" {
		var c RootConfig
		if err := json.Unmarshal([]byte(blob), &c); err != nil {
			return RootConfig{}, EnvSourceConfigJSON, fmt.Errorf("config_env_json_invalid: %w", err)
		}
		c.Normalize()
		return c, EnvSourceConfigJSON, nil
	}
	p, err := ResolvePath("")
	if err != nil {
		return RootConfig{}, "", err
	}
	c, err := Load(p)
	return c, p, err
}

// EnvManaged reports whether reads resolve from EFP_CONFIG_JSON instead of a
// file, i.e. no explicit path was given and the env blob is set.
func EnvManaged(flagPath string) bool {
	return flagPath == "" && strings.TrimSpace(os.Getenv(EnvConfigJSON)) != ""
}

// ErrEnvManaged is returned when a file write is refused because readers
// resolve config from EFP_CONFIG_JSON and would never see the written file.
var ErrEnvManaged = errors.New("config_env_managed: config comes from EFP_CONFIG_JSON; pass --config to write a file")

// SaveShared persists cfg to the file the shared loaders would read. When
// EFP_CONFIG_JSON manages the config and no explicit path is given, a file
// write would be invisible to every reader, so it is refused.
func SaveShared(flagPath string, c RootConfig) error {
	if EnvManaged(flagPath) {
		return ErrEnvManaged
	}
	p, err := ResolvePath(flagPath)
	if err != nil {
		return err
	}
	return Save(p, c)
}

func Load(path string) (RootConfig, error) {
	var c RootConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		if yerr := yaml.Unmarshal(b, &c); yerr != nil {
			return c, err
		}
	}
	c.Normalize()
	return c, nil
}

func Save(path string, c RootConfig) error {
	if path == "" {
		return errors.New("config_path_empty")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	doc, root := loadYAMLDocument(path)
	version := c.Version
	if version == 0 {
		version = 1
	}
	if err := setMappingValue(root, "version", version); err != nil {
		return err
	}
	if err := setMappingValue(root, "jira", c.Jira); err != nil {
		return err
	}
	if err := setMappingValue(root, "confluence", c.Confluence); err != nil {
		return err
	}
	if err := setMappingValue(root, "jenkins", c.Jenkins); err != nil {
		return err
	}
	if err := setMappingValue(root, "aws", c.AWS); err != nil {
		return err
	}
	if err := setMappingValue(root, "visual", c.Visual); err != nil {
		return err
	}
	deleteMappingValue(root, "mobile")
	if err := setMappingValue(root, "mobile-auto", c.Mobile); err != nil {
		return err
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return nil
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

func deleteMappingValue(root *yaml.Node, key string) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			return
		}
	}
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
	if old.Kind == new.Kind && old.Kind == yaml.ScalarNode && old.Style != 0 {
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
