package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
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
	b, err := json.MarshalIndent(c, "", "  ")
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
