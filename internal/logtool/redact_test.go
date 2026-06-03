package logtool

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer abc.def",
		"Bearer tokenvalue",
		"password=secret",
		"token: abc",
		"api_key=xyz",
		"api-key=xyz",
		"secret: hidden",
		"AWS_ACCESS_KEY_ID=AKIA123456789",
		"AWS_SECRET_ACCESS_KEY=awssecret",
		"email user@example.test",
		"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
	}, "\n")
	got := Redact(input)
	for _, leak := range []string{"abc.def", "tokenvalue", "password=secret", "api_key=xyz", "api-key=xyz", "hidden", "AKIA123456789", "awssecret", "user@example.test", "BEGIN PRIVATE KEY"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in %s", leak, got)
		}
	}
	if !strings.Contains(got, "<email>") {
		t.Fatalf("email was not normalized: %s", got)
	}
}
