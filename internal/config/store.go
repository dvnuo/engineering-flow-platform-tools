package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/configenv"
	"gopkg.in/yaml.v3"
)

// EnvSource is the source LoadShared reports when config was decoded from the
// EFP_-prefixed indexed environment variable convention.
const EnvSource = "env"

// LoadShared resolves the shared EFP config with precedence:
// explicit flag path > environment variables > EFP_CONFIG/ATLASSIAN_CONFIG file path > default path.
// The environment source is the flat, EFP_-prefixed indexed convention derived
// from the RootConfig json tags (e.g. EFP_JIRA_DEFAULT_INSTANCE,
// EFP_JIRA_INSTANCES_0_BASE_URL); it is active iff at least one recognized var is set.
func LoadShared(flagPath string) (RootConfig, string, error) {
	if flagPath != "" {
		c, err := Load(flagPath)
		return c, flagPath, err
	}
	if cfg, managed := LoadFromOSEnv(); managed {
		return cfg, EnvSource, nil
	}
	p, err := ResolvePath("")
	if err != nil {
		return RootConfig{}, "", err
	}
	c, err := Load(p)
	return c, p, err
}

// EnvManaged reports whether reads resolve from environment variables instead of
// a file, i.e. no explicit path was given and the env convention yields config.
func EnvManaged(flagPath string) bool {
	if flagPath != "" {
		return false
	}
	_, managed := LoadFromOSEnv()
	return managed
}

// ErrEnvManaged is returned when a file write is refused because readers
// resolve config from environment variables and would never see the written file.
var ErrEnvManaged = errors.New("config_env_managed: config comes from environment variables; pass --config to write a file")

// SaveShared persists cfg to the file the shared loaders would read. When
// environment variables manage the config and no explicit path is given, a file
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
	resolution, err := configenv.Resolve(&c, os.LookupEnv)
	if err != nil {
		return c, err
	}
	c.Normalize()
	snapshot, err := configenv.Capture(c, resolution)
	if err != nil {
		return c, err
	}
	c.envSnapshot = snapshot
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
	if err := setMappingValue(root, "version", version, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "jira", c.Jira, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "confluence", c.Confluence, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "jenkins", c.Jenkins, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "aws", c.AWS, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "browser", c.Browser, c.envSnapshot); err != nil {
		return err
	}
	if err := setMappingValue(root, "visual", c.Visual, c.envSnapshot); err != nil {
		return err
	}
	deleteMappingValue(root, "mobile")
	if err := setMappingValue(root, "mobile-auto", c.Mobile, c.envSnapshot); err != nil {
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

func setMappingValue(root *yaml.Node, key string, value any, snapshot *configenv.Snapshot) error {
	newValue, err := yamlValueNode(value)
	if err != nil {
		return err
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = configenv.MergeTopLevel(root.Content[i+1], newValue, snapshot, key)
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
