package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const EnvConfigPath = "ATLASSIAN_CONFIG"

func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	return DefaultPath()
}

func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		if app := os.Getenv("APPDATA"); app != "" {
			return filepath.Join(app, "atlassian", "config.json"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "atlassian", "config.json"), nil
}
