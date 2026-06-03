package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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
	if err := setMappingValue(root, "visual", c.Visual); err != nil {
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
