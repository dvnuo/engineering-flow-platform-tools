package config

import (
	"strings"
	"testing"
)

func TestRedactRemovesTokens(t *testing.T) {
	cfg := Default()
	cfg.Auth.GitHubAccessToken = "ghu_secret"
	cfg.Auth.CopilotToken = "copilot_secret"
	redacted := Redact(cfg)
	if redacted.Auth.GitHubAccessToken == "ghu_secret" || redacted.Auth.CopilotToken == "copilot_secret" {
		t.Fatal("token leaked")
	}
}

func TestRedactStringRemovesCopilotSecrets(t *testing.T) {
	got := RedactString("Authorization: Bearer gho_SECRET tid=SECRET data:image/png;base64,abc123")
	for _, secret := range []string{"gho_SECRET", "tid=SECRET", "abc123", "Authorization"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
}
