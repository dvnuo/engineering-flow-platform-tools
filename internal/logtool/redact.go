package logtool

import (
	"regexp"
	"strings"
)

var redactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\bAuthorization\s*:\s*Basic\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)Authorization\s*:\s*Bearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)\bSet-Cookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)\bCookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)\bX-API-Key\s*:\s*("[^"]*"|'[^']*'|[^,\s;&}]+)`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)"?\b(password|token|api_key|api-key|apikey|access_token|refresh_token|client_secret|secret)\b"?\s*[:=]\s*("[^"]*"|'[^']*'|[^,\s;&}]+)`),
	regexp.MustCompile(`(?i)"?\bAWS_ACCESS_KEY_ID\b"?\s*[:=]\s*("[^"]*"|'[^']*'|[^,\s;&}]+)`),
	regexp.MustCompile(`(?i)"?\bAWS_SECRET_ACCESS_KEY\b"?\s*[:=]\s*("[^"]*"|'[^']*'|[^,\s;&}]+)`),
	regexp.MustCompile(`(?i)"?\bAWS_SESSION_TOKEN\b"?\s*[:=]\s*("[^"]*"|'[^']*'|[^,\s;&}]+)`),
}

var emailPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

func Redact(s string) string {
	if s == "" {
		return s
	}
	for _, re := range redactionPatterns {
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			lower := strings.ToLower(match)
			switch {
			case strings.HasPrefix(lower, "authorization"):
				if strings.Contains(lower, "basic") {
					return "Authorization: Basic [REDACTED]"
				}
				return "Authorization: Bearer [REDACTED]"
			case strings.HasPrefix(lower, "cookie"):
				return "Cookie: [REDACTED]"
			case strings.HasPrefix(lower, "set-cookie"):
				return "Set-Cookie: [REDACTED]"
			case strings.HasPrefix(lower, "x-api-key"):
				return "X-API-Key: [REDACTED]"
			case strings.HasPrefix(lower, "bearer"):
				return "Bearer [REDACTED]"
			case strings.Contains(lower, "aws_access_key_id"):
				return "AWS_ACCESS_KEY_ID=[REDACTED]"
			case strings.Contains(lower, "aws_secret_access_key"):
				return "AWS_SECRET_ACCESS_KEY=[REDACTED]"
			case strings.Contains(lower, "aws_session_token"):
				return "AWS_SESSION_TOKEN=[REDACTED]"
			case strings.Contains(lower, "private key"):
				return "[REDACTED_PRIVATE_KEY]"
			default:
				if idx := strings.IndexAny(match, ":="); idx >= 0 {
					return strings.TrimSpace(match[:idx+1]) + "[REDACTED]"
				}
				return "[REDACTED]"
			}
		})
	}
	return emailPattern.ReplaceAllString(s, "<email>")
}

func RedactError(s string) string {
	s = strings.TrimSpace(Redact(s))
	if len(s) > 1000 {
		s = s[:1000] + "...(truncated)"
	}
	if s == "" {
		return "log command failed"
	}
	return s
}
