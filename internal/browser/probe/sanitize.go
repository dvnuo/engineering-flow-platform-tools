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
	raw = redactSensitiveHeaderValues(raw)
	return redactAssignments(raw)
}

func RedactErrorMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = RedactText(raw)
	raw = redactAssignments(raw)
	if len(raw) > 4000 {
		return raw[:4000]
	}
	return raw
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

func redactSensitiveHeaderValues(raw string) string {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		headerStart, valueStart := nextSensitiveHeader(raw, i)
		if headerStart < 0 {
			b.WriteString(raw[i:])
			break
		}
		b.WriteString(raw[i:valueStart])
		valueEnd := endOfHeaderValue(raw, valueStart)
		trimEnd := valueEnd
		for trimEnd > valueStart && isInlineSpace(raw[trimEnd-1]) {
			trimEnd--
		}
		b.WriteString("REDACTED")
		b.WriteString(raw[trimEnd:valueEnd])
		i = valueEnd
	}
	return b.String()
}

func nextSensitiveHeader(raw string, start int) (int, int) {
	lower := strings.ToLower(raw)
	headers := []string{"set-cookie", "authorization", "cookie"}
	bestStart := -1
	bestValueStart := -1
	for i := start; i < len(raw); i++ {
		for _, header := range headers {
			if valueStart, ok := sensitiveHeaderValueStart(lower, i, header); ok {
				if bestStart == -1 || i < bestStart {
					bestStart = i
					bestValueStart = valueStart
				}
			}
		}
	}
	return bestStart, bestValueStart
}

func sensitiveHeaderValueStart(lower string, start int, header string) (int, bool) {
	if start > 0 && isHeaderNameByte(lower[start-1]) {
		return 0, false
	}
	if !strings.HasPrefix(lower[start:], header) {
		return 0, false
	}
	i := start + len(header)
	for i < len(lower) && lower[i] == ' ' {
		i++
	}
	if i >= len(lower) || (lower[i] != ':' && lower[i] != '=') {
		return 0, false
	}
	i++
	for i < len(lower) && lower[i] == ' ' {
		i++
	}
	return i, true
}

func endOfHeaderValue(raw string, valueStart int) int {
	lineEnd := len(raw)
	for i := valueStart; i < len(raw); i++ {
		if raw[i] == '\n' || raw[i] == '\r' {
			lineEnd = i
			break
		}
	}
	nextHeader, _ := nextSensitiveHeader(raw, valueStart)
	if nextHeader >= 0 && nextHeader < lineEnd {
		return nextHeader
	}
	return lineEnd
}

func isHeaderNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

func isInlineSpace(b byte) bool {
	return b == ' ' || b == '\t'
}
