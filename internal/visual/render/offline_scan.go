package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

var scannedExtensions = map[string]bool{
	".html": true,
	".js":   true,
	".css":  true,
	".svg":  true,
	".json": true,
}

var offlineTokens = []string{
	"https://",
	"http://",
	"unpkg",
	"cdnjs",
	"jsdelivr",
	"fonts.googleapis.com",
	"fonts.gstatic.com",
	"@import",
	"fetch(",
	"XMLHttpRequest",
	"WebSocket",
	"EventSource",
	"navigator.sendBeacon",
	"import(",
}

var (
	protocolRelativePattern = regexp.MustCompile(`(?i)(src|href)\s*=\s*["']//|url\(\s*["']?//`)
	moduleScriptPattern     = regexp.MustCompile(`(?i)<script\s+[^>]*type\s*=\s*["']module["']`)
	rootAssetPattern        = regexp.MustCompile(`(?i)(src|href)\s*=\s*["']/`)
)

func ScanOffline(dir string) error {
	rootAbs, err := filepath.Abs(dir)
	if err != nil {
		return metadata.NewError("offline_violation", "failed to resolve scan directory: "+err.Error(), "Pass a valid output directory.", 400)
	}
	info, err := os.Stat(rootAbs)
	if err != nil || !info.IsDir() {
		return metadata.NewError("offline_violation", "offline scan directory is missing: "+dir, "Generate or pass an existing visual artifact directory.", 404)
	}
	var violation string
	walkErr := filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !scannedExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(b), "http://www.w3.org/2000/svg", "")
		content = strings.ReplaceAll(content, "http://www.w3.org/1999/xlink", "")
		if moduleScriptPattern.MatchString(content) {
			violation = violationMessage(rootAbs, path, `<script type="module"`)
			return filepath.SkipAll
		}
		if protocolRelativePattern.MatchString(content) {
			violation = violationMessage(rootAbs, path, "//")
			return filepath.SkipAll
		}
		if rootAssetPattern.MatchString(content) {
			violation = violationMessage(rootAbs, path, `src="/ or href="/`)
			return filepath.SkipAll
		}
		lower := strings.ToLower(content)
		for _, token := range offlineTokens {
			if strings.Contains(lower, strings.ToLower(token)) {
				violation = violationMessage(rootAbs, path, token)
				return filepath.SkipAll
			}
		}
		return nil
	})
	if walkErr != nil {
		return metadata.NewError("offline_violation", "offline scan failed: "+walkErr.Error(), "Inspect generated artifact files and retry.", 500)
	}
	if violation != "" {
		return metadata.NewError(
			"offline_violation",
			violation,
			"Remove remote/network dependency; visual artifacts must be fully offline.",
			400,
		)
	}
	return nil
}

func violationMessage(rootAbs, path, token string) string {
	rel, err := filepath.Rel(rootAbs, path)
	if err != nil {
		rel = path
	}
	return "offline violation in " + filepath.ToSlash(rel) + ": " + token
}
