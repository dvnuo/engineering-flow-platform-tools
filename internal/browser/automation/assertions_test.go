package automation

import (
	"strings"
	"testing"
)

func TestApplyNegation(t *testing.T) {
	if !applyNegation(true, false) {
		t.Fatalf("true assertion should pass")
	}
	if applyNegation(true, true) {
		t.Fatalf("negated true assertion should fail")
	}
	if !applyNegation(false, true) {
		t.Fatalf("negated false assertion should pass")
	}
}

func TestValidateCountAssertionOptions(t *testing.T) {
	valid := []AssertionOptions{
		{Selector: ".item", Equals: 1, Min: -1, Max: -1},
		{Selector: ".item", Equals: -1, Min: 1, Max: -1},
		{Selector: ".item", Equals: -1, Min: -1, Max: 5},
		{Selector: ".item", Equals: -1, Min: 1, Max: 5},
	}
	for _, opts := range valid {
		if err := validateCountAssertionOptions(opts); err != nil {
			t.Fatalf("validateCountAssertionOptions(%#v) failed: %v", opts, err)
		}
	}

	invalid := []AssertionOptions{
		{Equals: 1, Min: -1, Max: -1},
		{Selector: ".item", Ref: "axref-0-abc", Equals: 1, Min: -1, Max: -1},
		{Selector: ".item", Equals: -1, Min: -1, Max: -1},
		{Selector: ".item", Equals: 1, Min: 1, Max: -1},
		{Selector: ".item", Equals: -1, Min: 5, Max: 1},
	}
	for _, opts := range invalid {
		if err := validateCountAssertionOptions(opts); err == nil {
			t.Fatalf("validateCountAssertionOptions(%#v) succeeded unexpectedly", opts)
		}
	}
}

func TestCountAssertionPass(t *testing.T) {
	cases := []struct {
		count int
		opts  AssertionOptions
		want  bool
	}{
		{2, AssertionOptions{Equals: 2, Min: -1, Max: -1}, true},
		{3, AssertionOptions{Equals: 2, Min: -1, Max: -1}, false},
		{2, AssertionOptions{Equals: -1, Min: 1, Max: 3}, true},
		{0, AssertionOptions{Equals: -1, Min: 1, Max: 3}, false},
		{4, AssertionOptions{Equals: -1, Min: 1, Max: 3}, false},
		{4, AssertionOptions{Equals: -1, Min: -1, Max: 4}, true},
	}
	for _, tc := range cases {
		if got := countAssertionPass(tc.count, tc.opts); got != tc.want {
			t.Fatalf("countAssertionPass(%d,%#v)=%v want %v", tc.count, tc.opts, got, tc.want)
		}
	}
}

func TestAssertionResultSanitizesSelectorURLTitleAndContains(t *testing.T) {
	result := assertionBaseResult(
		Session{Name: "default"},
		Target{ID: "target-1"},
		"text",
		"input[name='password']",
		"axref-0-abc",
		"https://intranet.test/callback?access_token=secret",
		"session=abc",
		false,
	)
	result.Expected = assertionContainsExpected(`{"access_token":"secret","visible":"ok"}`)
	joined := result.Selector + result.Ref + result.URL + result.Title + result.Expected.ContainsPreview
	for _, leaked := range []string{"input[name='password']", "access_token=secret", `"access_token":"secret"`, "session=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("assertion result leaked %q in %#v", leaked, result)
		}
	}
	if result.Selector != "REDACTED_SELECTOR" {
		t.Fatalf("selector was not redacted: %#v", result)
	}
	if result.Expected.ContainsBytes == 0 {
		t.Fatalf("expected byte metadata missing: %#v", result)
	}
}

func TestAssertionFailureCarriesResultAndCode(t *testing.T) {
	result := AssertionResult{Assertion: "visible", Pass: false}
	err := assertionFailure(result)
	assertErr, ok := err.(*AssertionError)
	if !ok {
		t.Fatalf("failure error type = %T", err)
	}
	if assertErr.Base == nil || assertErr.Base.Code != assertionFailureCode {
		t.Fatalf("unexpected assertion error: %#v", assertErr)
	}
	if assertErr.Result.Assertion != "visible" || assertErr.Result.Pass {
		t.Fatalf("result not carried: %#v", assertErr.Result)
	}
	if err := assertionFailure(AssertionResult{Assertion: "visible", Pass: true}); err != nil {
		t.Fatalf("passing assertion returned error: %v", err)
	}
}
