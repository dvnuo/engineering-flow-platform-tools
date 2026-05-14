package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func Load(path string) (RootConfig, error) {
	var c RootConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	c.Normalize()
	return c, nil
}

func Save(path string, c RootConfig) error {
	if path == "" {
		return errors.New("config_path_empty")
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return nil
}
