package automation

import (
	"strconv"
	"strings"
	"testing"
	"time"
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
		"url":          "https://intranet.test/cb?access_token=secret",
		"nested":       []any{"Cookie: sid=private"},
		"token":        "bare-secret",
		"headers":      map[string]any{"Authorization": "Bearer private"},
		"localStorage": map[string]any{"theme": "dark"},
		"ok":           true,
	}
	got := SanitizeValue(raw, 200).(map[string]any)
	if strings.Contains(got["url"].(string), "secret") {
		t.Fatalf("url leaked: %#v", got)
	}
	nested := got["nested"].([]any)
	if strings.Contains(nested[0].(string), "sid=private") {
		t.Fatalf("nested value leaked: %#v", got)
	}
	for _, key := range []string{"token", "headers", "localStorage"} {
		if got[key] != "REDACTED" {
			t.Fatalf("%s was not redacted: %#v", key, got)
		}
	}
	if got["ok"] != true {
		t.Fatalf("bool changed: %#v", got)
	}
}

func TestValidateFetchURLRejectsUnsafeSchemes(t *testing.T) {
	for _, raw := range []string{
		"https://intranet.test/api",
		"http://intranet.test/api",
		"/api/status",
		"api/status",
		"?q=1",
	} {
		if err := validateFetchURL(raw); err != nil {
			t.Fatalf("validateFetchURL(%q) unexpected error: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"",
		"javascript:alert(1)",
		"data:text/plain,hello",
		"file:///etc/passwd",
		"chrome://version",
		"about:blank",
		"//intranet.test/api",
	} {
		if err := validateFetchURL(raw); err == nil {
			t.Fatalf("validateFetchURL(%q) succeeded unexpectedly", raw)
		}
	}
}

func TestValidateEvalExpressionRejectsSecretAndNetworkAPIs(t *testing.T) {
	for _, expr := range []string{
		"document.title",
		"document.querySelector('h1')?.innerText",
		"(() => ({count: document.links.length}))()",
	} {
		if err := validateEvalExpression(expr); err != nil {
			t.Fatalf("validateEvalExpression(%q) unexpected error: %v", expr, err)
		}
	}
	for _, expr := range []string{
		"document.cookie",
		"localStorage.getItem('token')",
		"sessionStorage",
		"fetch('/api')",
		"new XMLHttpRequest()",
		"navigator.credentials.get()",
		"response.headers",
	} {
		if err := validateEvalExpression(expr); err == nil {
			t.Fatalf("validateEvalExpression(%q) succeeded unexpectedly", expr)
		}
	}
}

func TestDefaultPageScreenshotPathUsesBrowserHome(t *testing.T) {
	t.Setenv(envBrowserHome, t.TempDir())
	now := time.Date(2026, 6, 8, 1, 2, 3, 4, time.UTC)
	got, err := DefaultPageScreenshotPath(now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "artifacts") || !strings.HasSuffix(got, "page-screenshot-20260608-010203-000000004.png") {
		t.Fatalf("unexpected screenshot path: %s", got)
	}
}

func TestValidateWaitOptionsAcceptsAdvancedConditions(t *testing.T) {
	valid := []WaitOptions{
		{Selector: ".ready"},
		{DurationMilliseconds: 1},
		{URLContains: "/dashboard"},
		{Text: "Welcome"},
		{NetworkIdleMilliseconds: 250},
		{DOMStableMilliseconds: 250},
		{Selector: ".ready", Text: "Done", NetworkIdleMilliseconds: 100},
	}
	for _, opts := range valid {
		if err := validateWaitOptions(opts); err != nil {
			t.Fatalf("validateWaitOptions(%#v) unexpected error: %v", opts, err)
		}
	}
}

func TestValidateWaitOptionsRejectsMissingAndNegativeConditions(t *testing.T) {
	invalid := []WaitOptions{
		{},
		{DurationMilliseconds: -1},
		{NetworkIdleMilliseconds: -1},
		{DOMStableMilliseconds: -1},
	}
	for _, opts := range invalid {
		if err := validateWaitOptions(opts); err == nil {
			t.Fatalf("validateWaitOptions(%#v) succeeded unexpectedly", opts)
		}
	}
}

func TestStableTrackerRequiresUnchangedKeyForDuration(t *testing.T) {
	tracker := newStableTracker(500 * time.Millisecond)
	start := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
	if ok, stableFor := tracker.Observe(start, "a"); ok || stableFor != 0 {
		t.Fatalf("first sample should not be stable: ok=%v stableFor=%v", ok, stableFor)
	}
	if ok, stableFor := tracker.Observe(start.Add(400*time.Millisecond), "a"); ok || stableFor != 400*time.Millisecond {
		t.Fatalf("early unchanged sample = ok=%v stableFor=%v", ok, stableFor)
	}
	if ok, stableFor := tracker.Observe(start.Add(450*time.Millisecond), "b"); ok || stableFor != 0 {
		t.Fatalf("changed sample should reset: ok=%v stableFor=%v", ok, stableFor)
	}
	if ok, stableFor := tracker.Observe(start.Add(950*time.Millisecond), "b"); !ok || stableFor != 500*time.Millisecond {
		t.Fatalf("stable sample = ok=%v stableFor=%v", ok, stableFor)
	}
}

func TestWaitSnapshotExpressionDoesNotExposeNeedlesAsBareCode(t *testing.T) {
	expr := waitSnapshotExpression(`"; document.cookie; "`, `</script><script>`)
	if !strings.Contains(expr, strconv.Quote(`"; document.cookie; "`)) {
		t.Fatalf("url needle was not quoted safely: %s", expr)
	}
	if !strings.Contains(expr, strconv.Quote(`</script><script>`)) {
		t.Fatalf("text needle was not quoted safely: %s", expr)
	}
}

func TestSanitizeFetchResultRedactsBodyAndURLs(t *testing.T) {
	result := sanitizeFetchResult(
		Session{Name: "default"},
		Target{ID: "page-1"},
		"https://intranet.test/api?access_token=secret",
		fetchEvalResult{
			OK:          true,
			Status:      200,
			URL:         "https://intranet.test/api?code=abc",
			BodyPreview: `{"token":"secret","next":"https://intranet.test/cb?code=abc"}`,
			BodyLength:  64,
		},
		200,
	)
	joined := result.RequestedURL + result.URL + result.BodyPreview
	for _, leaked := range []string{"access_token=secret", "code=abc", `"token":"secret"`} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("fetch result leaked %q: %#v", leaked, result)
		}
	}
}
