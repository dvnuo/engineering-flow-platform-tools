package probe

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func FindBrowser(browser, explicitPath string) (string, error) {
	if explicitPath != "" {
		if fileExists(expandHome(explicitPath)) {
			return expandHome(explicitPath), nil
		}
		return "", browserNotFound()
	}

	name := strings.ToLower(strings.TrimSpace(browser))
	if name == "" {
		name = "auto"
	}
	if name != "auto" && name != "edge" && name != "chrome" && name != "chromium" {
		return "", &ProbeError{Code: "invalid_args", Message: "--browser must be edge, chrome, chromium, or auto", Hint: "Run browser schema probe --json.", Status: 400}
	}

	for _, candidate := range browserCandidates(name) {
		if runtime.GOOS == "linux" {
			if path, err := exec.LookPath(candidate); err == nil {
				return path, nil
			}
			continue
		}
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", browserNotFound()
}

func DefaultProfileDir() string {
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "browser-probe-profile")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Caches", "browser-probe-profile")
		}
	default:
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".cache", "browser-probe-profile")
		}
	}
	return filepath.Join(os.TempDir(), "browser-probe-profile")
}

func LooksLikeDefaultBrowserProfile(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(expandHome(path))))
	for _, marker := range []string{
		"/library/application support/google/chrome",
		"/library/application support/microsoft edge",
		"/.config/google-chrome",
		"/.config/chromium",
		"/.config/microsoft-edge",
		"/appdata/local/google/chrome/user data",
		"/appdata/local/microsoft/edge/user data",
	} {
		if hasPathMarker(normalized, marker) {
			return true
		}
	}
	return false
}

func UnsafeProfileDir(path string) bool {
	clean := filepath.Clean(expandHome(path))
	if clean == "." || clean == string(os.PathSeparator) {
		return true
	}
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return false
	}
	return clean == volume || clean == volume+string(os.PathSeparator)
}

func hasPathMarker(path, marker string) bool {
	i := strings.Index(path, marker)
	if i < 0 {
		return false
	}
	end := i + len(marker)
	return end == len(path) || path[end] == '/'
}

func browserCandidates(browser string) []string {
	switch runtime.GOOS {
	case "windows":
		edge := []string{
			joinBase(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			joinBase(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
			joinBase(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		}
		chrome := []string{
			joinBase(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			joinBase(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
			joinBase(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		return filteredCandidates(browser, edge, chrome, nil)
	case "darwin":
		edge := []string{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"}
		chrome := []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
		chromium := []string{"/Applications/Chromium.app/Contents/MacOS/Chromium"}
		return filteredCandidates(browser, edge, chrome, chromium)
	default:
		edge := []string{"microsoft-edge", "microsoft-edge-stable"}
		chrome := []string{"google-chrome", "google-chrome-stable"}
		chromium := []string{"chromium", "chromium-browser"}
		return filteredCandidates(browser, edge, chrome, chromium)
	}
}

func joinBase(base string, elem ...string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	parts := append([]string{base}, elem...)
	return filepath.Join(parts...)
}

func filteredCandidates(browser string, edge, chrome, chromium []string) []string {
	switch browser {
	case "edge":
		return edge
	case "chrome":
		return chrome
	case "chromium":
		return chromium
	default:
		out := append([]string{}, edge...)
		out = append(out, chrome...)
		out = append(out, chromium...)
		return out
	}
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func expandHome(path string) string {
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

func browserNotFound() *ProbeError {
	return &ProbeError{
		Code:    "browser_not_found",
		Message: "No supported Edge/Chrome/Chromium browser was found.",
		Hint:    "Pass --browser-exe or install Microsoft Edge/Google Chrome/Chromium.",
		Status:  404,
	}
}
