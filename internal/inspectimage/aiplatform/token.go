package aiplatform

import (
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/inspectimage/config"
)

func TokenValid(c config.Config, now time.Time) bool {
	if strings.TrimSpace(c.AIPlatform.Auth.Token) == "" {
		return false
	}
	if strings.TrimSpace(c.AIPlatform.Auth.TokenExpiresAt) == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, c.AIPlatform.Auth.TokenExpiresAt)
	if err != nil {
		return false
	}
	return now.Before(t.Add(-5 * time.Second))
}

func CredentialsConfigured(c config.Config) bool {
	return strings.TrimSpace(c.AIPlatform.Auth.Username) != "" &&
		strings.TrimSpace(c.AIPlatform.Auth.Password) != "" &&
		strings.TrimSpace(c.AIPlatform.Auth.Usercase) != ""
}

func EndpointsConfigured(c config.Config) bool {
	return strings.TrimSpace(c.AIPlatform.Chat.Host) != "" &&
		strings.TrimSpace(c.AIPlatform.Chat.URI) != "" &&
		strings.TrimSpace(c.AIPlatform.IB2B.Host) != "" &&
		strings.TrimSpace(c.AIPlatform.IB2B.URI) != ""
}

func AuthUsable(c config.Config, now time.Time) bool {
	return TokenValid(c, now) || (CredentialsConfigured(c) && EndpointsConfigured(c))
}

func Summarize(c config.Config, now time.Time) Status {
	tokenValid := TokenValid(c, now)
	credentials := CredentialsConfigured(c)
	endpoints := EndpointsConfigured(c)
	refreshable := credentials && endpoints
	tokenState := "missing"
	nextAction := "Configure ai_platform.auth.username, password, usercase, chat host/uri, and ib2b host/uri."
	if tokenValid {
		tokenState = "valid"
		nextAction = ""
	} else if refreshable {
		tokenState = "refreshable"
		nextAction = "Run inspect-image auth test --json to refresh the AI Platform token, or retry inspect-image inspect --json."
	} else if strings.TrimSpace(c.AIPlatform.Auth.Token) != "" {
		tokenState = "expired"
	}
	return Status{
		Provider:               config.ProviderAIPlatform,
		AuthConfigured:         tokenValid || refreshable,
		UsernameConfigured:     strings.TrimSpace(c.AIPlatform.Auth.Username) != "",
		PasswordConfigured:     strings.TrimSpace(c.AIPlatform.Auth.Password) != "",
		UsercaseConfigured:     strings.TrimSpace(c.AIPlatform.Auth.Usercase) != "",
		EndpointsConfigured:    endpoints,
		TokenValid:             tokenValid,
		TokenRefreshable:       refreshable,
		TokenExpiresAt:         c.AIPlatform.Auth.TokenExpiresAt,
		TokenState:             tokenState,
		NextAction:             nextAction,
		ChatEndpointConfigured: strings.TrimSpace(c.AIPlatform.Chat.Host) != "" && strings.TrimSpace(c.AIPlatform.Chat.URI) != "",
		IB2BEndpointConfigured: strings.TrimSpace(c.AIPlatform.IB2B.Host) != "" && strings.TrimSpace(c.AIPlatform.IB2B.URI) != "",
	}
}

func Logout(c config.Config) config.Config {
	c.AIPlatform.Auth.Username = ""
	c.AIPlatform.Auth.Password = ""
	c.AIPlatform.Auth.Usercase = ""
	c.AIPlatform.Auth.Token = ""
	c.AIPlatform.Auth.TokenExpiresAt = ""
	c.AIPlatform.Auth.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return c
}
