package config

import "testing"

func TestRedactRootMasksSecrets(t *testing.T) {
	cfg := RootConfig{Jira: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Password: "p", APIKey: "k", Token: "t"}}}}}
	r := RedactRoot(cfg)
	a := r.Jira.Instances[0].Auth
	if a.Password != "***REDACTED***" || a.APIKey != "***REDACTED***" || a.Token != "***REDACTED***" {
		t.Fatal("expected redaction")
	}
}
