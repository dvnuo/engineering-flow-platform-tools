package confluence

import "strings"

func NormalizeBodyFormat(format string) string {
	f := strings.TrimSpace(format)
	if f == "" {
		return "storage"
	}
	return f
}

