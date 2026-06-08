package automation

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"engineering-flow-platform-tools/internal/browser/probe"
)

const envBrowserHome = "EFP_BROWSER_HOME"

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func DefaultBrowserHome() (string, error) {
	if override := strings.TrimSpace(os.Getenv(envBrowserHome)); override != "" {
		return filepath.Clean(expandHome(override)), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", NewError("artifact_write_failed", "Could not resolve the user home directory.", "Set EFP_BROWSER_HOME to a writable directory.", 500)
	}
	return filepath.Join(home, ".efp", "browser"), nil
}

func DefaultProfileDir(name string) (string, error) {
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	root, err := DefaultBrowserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "profiles", name), nil
}

func ValidateSessionName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return invalidArgs("--name must not be empty", "Use --name default or another simple session name.")
	}
	if name == "." || name == ".." || !sessionNamePattern.MatchString(name) {
		return invalidArgs("--name may contain only letters, numbers, dot, underscore, and hyphen, and must start with a letter or number", "Use a simple session name such as default or intranet.")
	}
	return nil
}

func ValidateProfileDir(profileDir string) (string, error) {
	profileDir = filepath.Clean(expandHome(strings.TrimSpace(profileDir)))
	if profileDir == "" || profileDir == "." {
		return "", invalidArgs("--profile must point at a dedicated browser profile directory", "Use a directory under ~/.efp/browser/profiles.")
	}
	if probe.UnsafeProfileDir(profileDir) {
		return "", invalidArgs("--profile must point at a dedicated directory, not a filesystem root", "Use a directory under ~/.efp/browser/profiles.")
	}
	if probe.LooksLikeDefaultBrowserProfile(profileDir) {
		return "", invalidArgs("--profile must not point at a default Edge/Chrome/Chromium profile", "Use a dedicated profile directory under ~/.efp/browser/profiles.")
	}
	return profileDir, nil
}

func validateHTTPURL(raw, flag string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return invalidArgs(flag+" must be an http or https URL", "Pass a full URL such as https://intranet.example.test.")
	}
	return nil
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
