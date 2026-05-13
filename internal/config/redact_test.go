package config

import "testing"

func TestRedactAuth(t *testing.T) {
	a := Auth{Username: "u", Password: "p", APIKey: "k", Token: "t"}
	r := RedactAuth(a)
	if r.Password != "***REDACTED***" || r.APIKey != "***REDACTED***" || r.Token != "***REDACTED***" {
		t.Fatalf("expected secrets to be redacted")
	}
	if r.Username != "u" {
		t.Fatalf("username should remain unchanged")
	}
}
