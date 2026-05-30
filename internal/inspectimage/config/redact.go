package config

import (
	"regexp"
	"strings"
)

const Redacted = "***REDACTED***"

func Redact(c Config) Config {
	c.Auth.GitHubAccessToken = redact(c.Auth.GitHubAccessToken)
	c.Auth.CopilotToken = redact(c.Auth.CopilotToken)
	return c
}

func RedactString(s string) string {
	out := s
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`gh[uo]_[A-Za-z0-9._~+/=-]+`),
		regexp.MustCompile(`tid=[^;&,\s"']+`),
		regexp.MustCompile(`(?i)authorization\s*[:=]\s*bearer\s+[^;&,\s"']+`),
		regexp.MustCompile(`(?i)\bbearer\s+[^;&,\s"']+`),
		regexp.MustCompile(`(?i)(data:image/[a-z0-9.+-]+;base64,)[A-Za-z0-9+/=_-]+`),
	} {
		out = re.ReplaceAllString(out, Redacted)
	}
	for _, marker := range []string{"github_access_token", "copilot_token", "Authorization", "Bearer "} {
		if strings.Contains(out, marker) {
			out = strings.ReplaceAll(out, marker, Redacted)
		}
	}
	return out
}

func redact(v string) string {
	if v == "" {
		return ""
	}
	return Redacted
}
