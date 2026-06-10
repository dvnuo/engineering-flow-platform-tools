package automation

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"
)

func TestValidateActionTargetRequiresExactlyOneSelectorOrRef(t *testing.T) {
	if err := validateActionTarget(".save", "", "page.click"); err != nil {
		t.Fatalf("selector target failed: %v", err)
	}
	if err := validateActionTarget("", "axref-0-abc", "page.click"); err != nil {
		t.Fatalf("ref target failed: %v", err)
	}
	for _, tc := range []struct {
		selector string
		ref      string
	}{
		{"", ""},
		{".save", "axref-0-abc"},
	} {
		if err := validateActionTarget(tc.selector, tc.ref, "page.click"); err == nil {
			t.Fatalf("validateActionTarget(%q,%q) succeeded unexpectedly", tc.selector, tc.ref)
		}
	}
}

func TestValidateSelectOptionsRequiresTargetAndOneSelection(t *testing.T) {
	valid := []SelectOptions{
		{Selector: "select", Value: "open", Index: -1},
		{Ref: "axref-0-abc", Label: "Open", Index: -1},
		{Selector: "select", Index: 0},
	}
	for _, opts := range valid {
		if _, err := validateSelectOptions(opts); err != nil {
			t.Fatalf("validateSelectOptions(%#v) failed: %v", opts, err)
		}
	}
	invalid := []SelectOptions{
		{Value: "open", Index: -1},
		{Selector: "select", Index: -1},
		{Selector: "select", Value: "open", Label: "Open", Index: -1},
		{Selector: "select", Value: "open", Index: 0},
	}
	for _, opts := range invalid {
		if _, err := validateSelectOptions(opts); err == nil {
			t.Fatalf("validateSelectOptions(%#v) succeeded unexpectedly", opts)
		}
	}
}

func TestNormalizePressKeyMapsNamedKeys(t *testing.T) {
	cases := map[string]string{
		"Enter":     kb.Enter,
		"return":    kb.Enter,
		"Tab":       kb.Tab,
		"Escape":    kb.Escape,
		"ArrowDown": kb.ArrowDown,
		"left":      kb.ArrowLeft,
		"a":         "a",
	}
	for raw, want := range cases {
		got, err := NormalizePressKey(raw)
		if err != nil {
			t.Fatalf("NormalizePressKey(%q) failed: %v", raw, err)
		}
		if got != want {
			t.Fatalf("NormalizePressKey(%q)=%q want %q", raw, got, want)
		}
	}
	for _, raw := range []string{"", strings.Repeat("a", 100)} {
		if _, err := NormalizePressKey(raw); err == nil {
			t.Fatalf("NormalizePressKey(%q) succeeded unexpectedly", raw)
		}
	}
}

func TestPageActionResultDoesNotEchoTypedTextOrSelectedValue(t *testing.T) {
	result := pageActionResult(
		Session{Name: "default"},
		Target{ID: "page-1"},
		"type",
		"input[name='password']",
		"axref-0-abc",
		"https://intranet.test/cb?access_token=secret",
		"session=abc",
	)
	result.TextBytes = len("super-secret")
	result.SelectionMode = "value"
	result.SelectedCount = 1
	joined := result.Selector + result.Ref + result.URL + result.Title + result.SelectionMode
	for _, leaked := range []string{"super-secret", "access_token=secret", "password", "session=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("action result leaked %q in %#v", leaked, result)
		}
	}
	if result.TextBytes != len("super-secret") || result.SelectedCount != 1 {
		t.Fatalf("metadata changed: %#v", result)
	}
}
