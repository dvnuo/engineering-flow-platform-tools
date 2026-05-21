package probe

import (
	"net/url"
	"regexp"
	"strings"
)

var sensitiveURLWords = []string{
	"token",
	"access_token",
	"id_token",
	"refresh_token",
	"code",
	"auth",
	"ticket",
	"session",
	"sig",
	"signature",
	"key",
	"password",
	"jwt",
	"saml",
}

var sensitiveAssignment = regexp.MustCompile(`(?i)([A-Za-z0-9_.~-]*(?:access_token|id_token|refresh_token|token|code|auth|ticket|session|sig|signature|key|password|jwt|saml)[A-Za-z0-9_.~-]*\s*=\s*)[^&#\s]+`)
var sensitiveHeaderValue = regexp.MustCompile(`(?i)\b(set-cookie|authorization|cookie)\s*[:=]\s*[^\r\n]+`)

func RedactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return redactAssignments(raw)
	}
	q := u.Query()
	for key := range q {
		if isSensitiveKey(key) {
			q.Set(key, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	if u.Fragment != "" {
		u.Fragment = "REDACTED"
	}
	return redactAssignments(u.String())
}

func RedactText(raw string) string {
	raw = sensitiveHeaderValue.ReplaceAllString(raw, "${1}: REDACTED")
	return redactAssignments(raw)
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, word := range sensitiveURLWords {
		if strings.Contains(key, word) {
			return true
		}
	}
	return false
}

func redactAssignments(raw string) string {
	return sensitiveAssignment.ReplaceAllString(raw, "${1}REDACTED")
}
