package auth

import (
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

type Status struct {
	AuthConfigured              bool   `json:"auth_configured"`
	GitHubHost                  string `json:"github_host"`
	GitHubUser                  string `json:"github_user,omitempty"`
	GitHubAccessTokenConfigured bool   `json:"github_access_token_configured"`
	GitHubAccessTokenValid      bool   `json:"github_access_token_valid"`
	GitHubAccessTokenExpiresAt  string `json:"github_access_token_expires_at,omitempty"`
	CopilotTokenValid           bool   `json:"copilot_token_valid"`
	CopilotTokenRefreshable     bool   `json:"copilot_token_refreshable"`
	CopilotTokenExpiresAt       string `json:"copilot_token_expires_at,omitempty"`
	TokenState                  string `json:"token_state"`
	NextAction                  string `json:"next_action,omitempty"`
}

func NeedsExchange(c config.Config) bool {
	if c.Auth.GitHubAccessToken == "" {
		return false
	}
	token := strings.TrimSpace(c.Auth.CopilotToken)
	if token == "" {
		return true
	}
	if token == strings.TrimSpace(c.Auth.GitHubAccessToken) {
		return true
	}
	if strings.HasPrefix(token, "gho_") || strings.HasPrefix(token, "ghu_") {
		return true
	}
	return false
}

func TokenValid(c config.Config, now time.Time) bool {
	if c.Auth.CopilotToken == "" {
		return false
	}
	if NeedsExchange(c) {
		return false
	}
	if c.Auth.CopilotTokenExpiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, c.Auth.CopilotTokenExpiresAt)
	if err != nil {
		return false
	}
	return now.Before(t)
}

func GitHubTokenValid(c config.Config, now time.Time) bool {
	if strings.TrimSpace(c.Auth.GitHubAccessToken) == "" {
		return false
	}
	if strings.TrimSpace(c.Auth.GitHubAccessTokenExpiresAt) == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, c.Auth.GitHubAccessTokenExpiresAt)
	if err != nil {
		return false
	}
	return now.Before(t)
}

func AuthUsable(c config.Config, now time.Time) bool {
	return TokenValid(c, now) || GitHubTokenValid(c, now)
}

func Summarize(c config.Config, now time.Time) Status {
	copilotValid := TokenValid(c, now)
	githubConfigured := strings.TrimSpace(c.Auth.GitHubAccessToken) != ""
	githubValid := GitHubTokenValid(c, now)
	tokenState := "missing"
	nextAction := "Run inspect-image auth login."
	if copilotValid {
		tokenState = "valid"
		nextAction = ""
	} else if githubValid {
		tokenState = "refreshable"
		nextAction = "Run inspect-image auth test --json to refresh the Copilot token, or retry inspect-image inspect --json."
	} else if githubConfigured {
		tokenState = "github_expired"
		nextAction = "Run inspect-image auth login."
	} else if strings.TrimSpace(c.Auth.CopilotToken) != "" {
		tokenState = "copilot_expired"
		nextAction = "Run inspect-image auth login."
	}
	return Status{
		AuthConfigured:              copilotValid || githubValid,
		GitHubHost:                  c.Auth.GitHubHost,
		GitHubUser:                  c.Auth.GitHubUser,
		GitHubAccessTokenConfigured: githubConfigured,
		GitHubAccessTokenValid:      githubValid,
		GitHubAccessTokenExpiresAt:  c.Auth.GitHubAccessTokenExpiresAt,
		CopilotTokenValid:           copilotValid,
		CopilotTokenRefreshable:     githubValid,
		CopilotTokenExpiresAt:       c.Auth.CopilotTokenExpiresAt,
		TokenState:                  tokenState,
		NextAction:                  nextAction,
	}
}
