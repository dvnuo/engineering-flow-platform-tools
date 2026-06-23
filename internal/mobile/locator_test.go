package mobile

import (
	"strings"
	"testing"
)

func TestLocatorsForCandidateEscapesXPathFallback(t *testing.T) {
	locators := LocatorsForCandidate("android", Candidate{Name: `Save John's "draft"`})
	if len(locators) != 1 || locators[0].Using != "xpath" {
		t.Fatalf("unexpected locators: %#v", locators)
	}
	value := locators[0].Value
	if !strings.Contains(value, "concat(") || strings.Contains(value, `@name="Save John's "draft""`) {
		t.Fatalf("xpath fallback was not escaped: %s", value)
	}
}

func TestLocatorsForCandidateUsesCompleteAndroidFallbackChain(t *testing.T) {
	locators := LocatorsForCandidate("android", Candidate{AccessibilityID: "login-a11y", ResourceID: "com.app:id/login", Text: "Login"})
	want := []string{"accessibility id", "id", "-android uiautomator", "xpath"}
	if len(locators) != len(want) {
		t.Fatalf("locators=%#v", locators)
	}
	for i, using := range want {
		if locators[i].Using != using {
			t.Fatalf("locator[%d]=%s want %s: %#v", i, locators[i].Using, using, locators)
		}
	}
}

func TestLocatorsForCandidateUsesCompleteIOSFallbackChain(t *testing.T) {
	locators := LocatorsForCandidate("ios", Candidate{AccessibilityID: "login-a11y", Class: "XCUIElementTypeButton", Name: "Login"})
	want := []string{"accessibility id", "-ios predicate string", "-ios class chain", "xpath"}
	if len(locators) != len(want) {
		t.Fatalf("locators=%#v", locators)
	}
	for i, using := range want {
		if locators[i].Using != using {
			t.Fatalf("locator[%d]=%s want %s: %#v", i, locators[i].Using, using, locators)
		}
	}
	if !strings.Contains(locators[2].Value, "XCUIElementTypeButton") || !strings.Contains(locators[2].Value, "Login") {
		t.Fatalf("bad class chain: %s", locators[2].Value)
	}
}

func TestSelectorStringEscapesBackslashAndQuote(t *testing.T) {
	got := selectorString(`C:\Temp "draft"`)
	want := `C:\\Temp \"draft\"`
	if got != want {
		t.Fatalf("selectorString=%q want %q", got, want)
	}
}
