package schema

import "strings"

func normalizeKind(kind string) string {
	return strings.TrimSpace(strings.ToLower(kind))
}

func titleFromData(data map[string]any) string {
	if data == nil {
		return ""
	}
	title, _ := data["title"].(string)
	return strings.TrimSpace(title)
}
