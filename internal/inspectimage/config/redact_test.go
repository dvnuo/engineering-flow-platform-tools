package config

import "testing"

func TestRedactRemovesTokens(t *testing.T) {
	cfg := Default()
	cfg.Auth.GitHubAccessToken = "ghu_secret"
	cfg.Auth.CopilotToken = "copilot_secret"
	redacted := Redact(cfg)
	if redacted.Auth.GitHubAccessToken == "ghu_secret" || redacted.Auth.CopilotToken == "copilot_secret" {
		t.Fatal("token leaked")
	}
}
