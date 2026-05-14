package config

import "testing"

func TestNormalizeAuthTypeCanonicalAndAliases(t *testing.T) {
	tests := []struct {
		name string
		auth AuthConfig
		want string
		key  string
	}{
		{name: "basic_password", auth: AuthConfig{Type: "basic_password", Username: "u", Password: "p"}, want: "basic_password"},
		{name: "basic_api_key", auth: AuthConfig{Type: "basic_api_key", Username: "u", APIKey: "k"}, want: "basic_api_key", key: "k"},
		{name: "bearer_token", auth: AuthConfig{Type: "bearer_token", Token: "t"}, want: "bearer_token"},
		{name: "pat alias", auth: AuthConfig{Type: "pat", Token: "t"}, want: "bearer_token"},
		{name: "basic_token alias", auth: AuthConfig{Type: "basic_token", Username: "u", Token: "t"}, want: "basic_api_key", key: "t"},
		{name: "api_key alias", auth: AuthConfig{Type: "api_key", Username: "u", APIKey: "k"}, want: "basic_api_key", key: "k"},
		{name: "username token inferred api key", auth: AuthConfig{Username: "u", Token: "t"}, want: "basic_api_key", key: "t"},
		{name: "token only inferred bearer", auth: AuthConfig{Token: "t"}, want: "bearer_token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := tt.auth
			auth.NormalizeType()
			if auth.Type != tt.want {
				t.Fatalf("type=%q want %q", auth.Type, tt.want)
			}
			if tt.key != "" && auth.APIKey != tt.key {
				t.Fatalf("api_key=%q want %q", auth.APIKey, tt.key)
			}
		})
	}
}
