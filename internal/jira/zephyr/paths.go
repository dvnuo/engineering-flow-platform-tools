package zephyr

import (
	"strings"
)

func ZAPI(basePath, p string) string {
	base := "/" + strings.Trim(basePath, "/")
	if strings.TrimSpace(p) == "" {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(p, "/")
}

func RawPath(basePath, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", NewError("zephyr_raw_path_blocked", "raw Zephyr path is required", "Use a relative ZAPI path such as cycle.", 400)
	}
	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return "", NewError("zephyr_raw_path_blocked", "absolute external URLs are not allowed for Zephyr raw API", "Use a relative path or an absolute /rest/... Jira instance path.", 400)
	}
	if strings.HasPrefix(p, "/") {
		if !strings.HasPrefix(lower, "/rest/") && lower != "/rest" {
			return "", NewError("zephyr_raw_path_blocked", "raw Zephyr absolute paths must start with /rest/", "Use /rest/zapi/latest/... for legacy ZAPI.", 400)
		}
		return "/" + strings.TrimLeft(p, "/"), nil
	}
	if hasParentPathSegment(p) {
		return "", NewError("zephyr_raw_path_blocked", "raw Zephyr relative paths cannot contain .. segments", "Use a ZAPI-relative path such as execution/123.", 400)
	}
	return ZAPI(basePath, p), nil
}

func hasParentPathSegment(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
