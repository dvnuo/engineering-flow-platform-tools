package automation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"engineering-flow-platform-tools/internal/browser/probe"
)

var embeddedURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func RedactString(s string) string {
	if s == "" {
		return ""
	}
	s = embeddedURLPattern.ReplaceAllStringFunc(s, func(raw string) string {
		return probe.RedactURL(raw)
	})
	if looksLikeURL(s) {
		s = probe.RedactURL(s)
	}
	return probe.RedactText(s)
}

func RedactURL(raw string) string {
	return probe.RedactURL(raw)
}

func RedactError(raw string) string {
	return probe.RedactErrorMessage(raw)
}

func TruncateBytes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	for max > 0 && !utf8.ValidString(s[:max]) {
		max--
	}
	return s[:max] + "...(truncated)"
}

func truncateBytes(s string, max int) string {
	return TruncateBytes(s, max)
}

func SanitizeValue(value any, maxStringBytes int) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return TruncateBytes(RedactString(v), maxStringBytes)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = SanitizeValue(item, maxStringBytes)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = SanitizeValue(item, maxStringBytes)
		}
		return out
	case bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	default:
		return TruncateBytes(RedactString(fmt.Sprint(v)), maxStringBytes)
	}
}

func looksLikeURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && u != nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
