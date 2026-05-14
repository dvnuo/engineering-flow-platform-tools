package config

import "testing"

func TestNormalizeAuthCompatibility(t *testing.T) {
	c := RootConfig{Jira: ProductConfig{Instances: []InstanceConfig{{Auth: AuthConfig{Username: "u", Password: "p"}}, {Auth: AuthConfig{Username: "u", APIKey: "k"}}, {Auth: AuthConfig{Token: "t"}}, {Auth: AuthConfig{Username: "u", Token: "legacy"}}}}}
	c.Normalize()
	if c.Jira.Instances[0].Auth.Type != "basic_password" || c.Jira.Instances[1].Auth.Type != "basic_api_key" || c.Jira.Instances[2].Auth.Type != "bearer_token" || c.Jira.Instances[3].Auth.Type != "basic_api_key" {
		t.Fatalf("normalization failed")
	}
	if c.Jira.Instances[3].Auth.APIKey != "legacy" || c.Jira.Instances[3].Auth.Token != "" {
		t.Fatalf("legacy token+username should become api_key")
	}
}

func TestRedactAuth(t *testing.T) {
	a := AuthConfig{Password: "p", APIKey: "k", Token: "t"}
	r := RedactAuth(a)
	if r.Password == "p" || r.APIKey == "k" || r.Token == "t" {
		t.Fatalf("secret leaked")
	}
}
