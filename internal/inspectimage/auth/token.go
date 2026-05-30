package auth

import (
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

type Status struct {
	AuthConfigured        bool   `json:"auth_configured"`
	GitHubHost            string `json:"github_host"`
	GitHubUser            string `json:"github_user,omitempty"`
	CopilotTokenValid     bool   `json:"copilot_token_valid"`
	CopilotTokenExpiresAt string `json:"copilot_token_expires_at,omitempty"`
}

func TokenValid(c config.Config, now time.Time) bool {
	if c.Auth.CopilotToken == "" {
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

func Summarize(c config.Config, now time.Time) Status {
	valid := TokenValid(c, now)
	return Status{
		AuthConfigured:        c.Auth.CopilotToken != "",
		GitHubHost:            c.Auth.GitHubHost,
		GitHubUser:            c.Auth.GitHubUser,
		CopilotTokenValid:     valid,
		CopilotTokenExpiresAt: c.Auth.CopilotTokenExpiresAt,
	}
}
