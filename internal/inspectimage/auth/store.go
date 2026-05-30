package auth

import (
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

func Logout(c config.Config) config.Config {
	c.Auth.GitHubAccessToken = ""
	c.Auth.GitHubAccessTokenExpiresAt = ""
	c.Auth.CopilotToken = ""
	c.Auth.CopilotTokenExpiresAt = ""
	c.Auth.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return c
}
