package automation

import (
	"strings"
	"testing"
)

func TestSanitizeOutlineElementsRedactsTextHrefAndSelector(t *testing.T) {
	raw := []OutlineElement{{
		Index:     0,
		Kind:      "Link",
		Role:      "LINK",
		Name:      "token=secret",
		Text:      "Authorization: Bearer private",
		Label:     "session=abc",
		Href:      "https://intranet.test/cb?code=abc",
		Tag:       "A",
		InputType: "TEXT",
		Selector:  `main > a#token-secret`,
	}}
	got := sanitizeOutlineElements(raw)
	el := got[0]
	joined := el.Name + el.Text + el.Label + el.Href + el.Selector
	for _, leaked := range []string{"Bearer private", "code=abc", "session=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("outline element leaked %q in %#v", leaked, el)
		}
	}
	if el.Kind != "link" || el.Role != "link" || el.Tag != "a" || el.InputType != "text" {
		t.Fatalf("expected normalized fields: %#v", el)
	}
}

func TestNormalizeSelectorHintTruncatesAndRedacts(t *testing.T) {
	raw := strings.Repeat("section > ", 100) + "#access_token=secret"
	got := normalizeSelectorHint(raw)
	if len(got) > 514 {
		t.Fatalf("selector was not truncated: len=%d", len(got))
	}
	if strings.Contains(got, "access_token=secret") {
		t.Fatalf("selector leaked token: %s", got)
	}
}
