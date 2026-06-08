package automation

import (
	"strings"
	"testing"
)

func TestSanitizeExtractedElementsRedactsStringsURLsAndHTML(t *testing.T) {
	elements := []ExtractedElement{{
		Index:     0,
		Text:      "token=secret",
		Value:     "Authorization: Bearer private",
		Href:      "https://intranet.test/cb?code=abc",
		Title:     "ok",
		AriaLabel: "session=abc",
		TagName:   "a",
		HTML:      `<a href="https://intranet.test/cb?code=abc">token=secret</a>`,
	}}
	got := sanitizeExtractedElements(elements, true, 80)
	el := got[0]
	joined := el.Text + el.Value + el.Href + el.AriaLabel + el.HTML
	for _, leaked := range []string{"secret", "Bearer private", "code=abc", "session=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("leaked %q in %#v", leaked, el)
		}
	}
	for _, want := range []string{"token=REDACTED", "Authorization: REDACTED", "code=REDACTED"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %#v", want, el)
		}
	}
}

func TestSanitizeValueRecursivelyRedactsEvalLikeResults(t *testing.T) {
	raw := map[string]any{
		"url":    "https://intranet.test/cb?access_token=secret",
		"nested": []any{"Cookie: sid=private"},
		"ok":     true,
	}
	got := SanitizeValue(raw, 200).(map[string]any)
	if strings.Contains(got["url"].(string), "secret") {
		t.Fatalf("url leaked: %#v", got)
	}
	nested := got["nested"].([]any)
	if strings.Contains(nested[0].(string), "sid=private") {
		t.Fatalf("nested value leaked: %#v", got)
	}
	if got["ok"] != true {
		t.Fatalf("bool changed: %#v", got)
	}
}
