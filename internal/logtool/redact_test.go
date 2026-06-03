package logtool

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	input := strings.Join([]string{
		"Authorization: Bearer abc.def",
		"Authorization: Basic basicsecret",
		"Bearer tokenvalue",
		"Cookie: sessionid=cookievalue; csrftoken=csrfvalue",
		"Set-Cookie: sid=setcookievalue",
		"X-API-Key: xapikeyvalue",
		"password=secret",
		"token: abc",
		"api_key=xyz",
		"api-key=xyz",
		"apikey=key456",
		"access_token=tok789",
		"refresh_token=ref999",
		"client_secret=sec111",
		"secret: hidden",
		"AWS_ACCESS_KEY_ID=AKIA123456789",
		"AWS_SECRET_ACCESS_KEY=awssecret",
		"AWS_SESSION_TOKEN=sessionsecret",
		"email user@example.test",
		"-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
	}, "\n")
	got := Redact(input)
	for _, leak := range []string{"abc.def", "basicsecret", "tokenvalue", "cookievalue", "csrfvalue", "setcookievalue", "xapikeyvalue", "password=secret", "api_key=xyz", "api-key=xyz", "key456", "tok789", "ref999", "sec111", "hidden", "AKIA123456789", "awssecret", "sessionsecret", "user@example.test", "BEGIN PRIVATE KEY"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in %s", leak, got)
		}
	}
	if !strings.Contains(got, "<email>") {
		t.Fatalf("email was not normalized: %s", got)
	}
}
