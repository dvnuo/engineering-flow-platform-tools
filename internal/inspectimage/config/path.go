package config

import (
	"os"
	"path/filepath"
)

const EnvConfigPath = "EFP_CONFIG"
const EnvLegacyConfigPath = "INSPECT_IMAGE_CONFIG"

func ResolvePath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	if p := os.Getenv(EnvLegacyConfigPath); p != "" {
		return p, nil
	}
	return DefaultPath()
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".efp", "config.yaml"), nil
}
