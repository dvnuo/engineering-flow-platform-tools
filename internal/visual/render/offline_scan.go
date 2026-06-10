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
	".yaml": true,
	".yml":  true,
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
	"XMLHttpRequest",
	"WebSocket",
	"EventSource",
	"navigator.sendBeacon",
	"import(",
}

var (
	networkFetchPattern = regexp.MustCompile(`(?i)(^|[^a-z0-9_$])fetch\s*\(|\.fetch\s*\(`)
	rootAssetPattern    = regexp.MustCompile(`(?i)(src|href)\s*=\s*["']/`)
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
		content = strings.ReplaceAll(content, "http://www.w3.org/1999/xhtml", "")
		if containsProtocolRelativeURL(content) {
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
		if networkFetchPattern.MatchString(content) {
			violation = violationMessage(rootAbs, path, "fetch(")
			return filepath.SkipAll
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

func containsProtocolRelativeURL(content string) bool {
	for i := 0; i+1 < len(content); i++ {
		if content[i] != '/' || content[i+1] != '/' {
			continue
		}
		if precededByURLSchemeColon(content, i) {
			continue
		}
		if looksLikeURLAuthority(content, i+2) {
			return true
		}
	}
	return false
}

func precededByURLSchemeColon(content string, slash int) bool {
	runStart := slash
	for runStart > 0 && content[runStart-1] == '/' {
		runStart--
	}
	if runStart == 0 || content[runStart-1] != ':' {
		return false
	}
	start := runStart - 2
	for start >= 0 && isURLSchemeChar(content[start]) {
		start--
	}
	start++
	return start <= runStart-2 && isASCIILetter(content[start])
}

func looksLikeURLAuthority(content string, start int) bool {
	if start >= len(content) || !isURLAuthorityStart(content[start]) {
		return false
	}
	for start < len(content) && isURLAuthorityChar(content[start]) {
		start++
	}
	if start >= len(content) {
		return true
	}
	switch content[start] {
	case '/', '?', '#', '\'', '"', '`', '<', '>', ')', ']', '}', ';', ',':
		return true
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func isURLAuthorityStart(c byte) bool {
	return isASCIILetter(c) || isASCIIDigit(c) || c == '['
}

func isURLAuthorityChar(c byte) bool {
	return isASCIILetter(c) || isASCIIDigit(c) || c == '.' || c == '-' || c == '_' || c == '~' || c == '[' || c == ']'
}

func isURLSchemeChar(c byte) bool {
	return isASCIILetter(c) || isASCIIDigit(c) || c == '+' || c == '.' || c == '-'
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
