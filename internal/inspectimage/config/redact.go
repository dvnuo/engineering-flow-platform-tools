package config

import "strings"

const Redacted = "***REDACTED***"

func Redact(c Config) Config {
	c.Auth.GitHubAccessToken = redact(c.Auth.GitHubAccessToken)
	c.Auth.CopilotToken = redact(c.Auth.CopilotToken)
	return c
}

func RedactString(s string) string {
	out := s
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
