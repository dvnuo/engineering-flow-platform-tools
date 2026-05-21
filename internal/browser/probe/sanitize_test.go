package probe

import (
	"reflect"
	"strings"
	"testing"
)

func TestRedactURLSensitiveQuery(t *testing.T) {
	cases := []struct {
		raw      string
		redacted []string
		kept     []string
	}{
		{raw: "https://x/cb?code=abc&state=ok", redacted: []string{"code=REDACTED"}, kept: []string{"state=ok", "https://x/cb"}},
		{raw: "https://x/?access_token=a", redacted: []string{"access_token=REDACTED"}, kept: []string{"https://x/"}},
		{raw: "https://x/?id_token=a&session=b&sig=c&key=d&password=e&jwt=f&saml=g", redacted: []string{"id_token=REDACTED", "session=REDACTED", "sig=REDACTED", "key=REDACTED", "password=REDACTED", "jwt=REDACTED", "saml=REDACTED"}},
		{raw: "invalid string containing token=abc", redacted: []string{"token=REDACTED"}},
	}
	for _, tc := range cases {
		got := RedactURL(tc.raw)
		for _, want := range tc.redacted {
			if !strings.Contains(got, want) {
				t.Fatalf("RedactURL(%q) missing %q in %q", tc.raw, want, got)
			}
		}
		for _, want := range tc.kept {
			if !strings.Contains(got, want) {
				t.Fatalf("RedactURL(%q) did not keep %q in %q", tc.raw, want, got)
			}
		}
		if strings.Contains(got, "abc") || strings.Contains(got, "access_token=a") {
			t.Fatalf("RedactURL leaked secret in %q", got)
		}
	}
}

func TestRedactURLFragment(t *testing.T) {
	got := RedactURL("https://x/#id_token=abc")
	if got != "https://x/#REDACTED" {
		t.Fatalf("fragment not redacted: %q", got)
	}
}

func TestNetworkEventHasNoSensitiveHeaderFields(t *testing.T) {
	typ := reflect.TypeOf(NetworkEvent{})
	for _, field := range []string{"Authorization", "Cookie", "SetCookie", "Set-Cookie"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("NetworkEvent has sensitive field %s", field)
		}
	}
}

func TestRedactTextRedactsSensitiveHeaders(t *testing.T) {
	got := RedactText("Authorization: Bearer secret\nCookie: a=b\nSet-Cookie: sid=secret\nok")
	for _, secret := range []string{"Bearer secret", "a=b", "sid=secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("RedactText leaked %q in %q", secret, got)
		}
	}
}

func TestRedactErrorMessageRedactsSensitiveValues(t *testing.T) {
	got := RedactErrorMessage("navigation failed for https://x/cb?code=abc&access_token=secret Authorization: Bearer private Cookie: sid=123")
	for _, leaked := range []string{"abc", "secret", "Bearer private", "sid=123"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactErrorMessage leaked %q in %q", leaked, got)
		}
	}
	for _, want := range []string{"code=REDACTED", "access_token=REDACTED", "Authorization: REDACTED", "Cookie: REDACTED"} {
		if !strings.Contains(got, want) {
			t.Fatalf("RedactErrorMessage missing %q in %q", want, got)
		}
	}
}

func TestRedactErrorMessageTruncatesLongMessages(t *testing.T) {
	got := RedactErrorMessage(strings.Repeat("x", 5000))
	if len(got) != 4000 {
		t.Fatalf("got length %d", len(got))
	}
}
