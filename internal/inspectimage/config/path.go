package config

import (
	"os"
	"path/filepath"
)

const EnvConfigPath = "INSPECT_IMAGE_CONFIG"
const EnvCopilotHome = "COPILOT_HOME"

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
	if home := os.Getenv(EnvCopilotHome); home != "" {
		return filepath.Join(home, "inspect-image.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".copilot", "inspect-image.json"), nil
}
