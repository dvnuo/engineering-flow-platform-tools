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
