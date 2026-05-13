package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const EnvConfigPath = "ATLASSIAN_CONFIG"

func DefaultPath() (string, error) {
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return "", errors.New("missing_appdata")
		}
		return filepath.Join(appdata, "atlassian", "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "atlassian", "config.json"), nil
}
