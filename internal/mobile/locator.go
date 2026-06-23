package mobile

import (
	"strings"
)

type Locator struct {
	Using string `json:"using"`
	Value string `json:"value"`
}

func LocatorsForCandidate(platform string, c Candidate) []Locator {
	var out []Locator
	if c.AccessibilityID != "" {
		out = append(out, Locator{Using: "accessibility id", Value: c.AccessibilityID})
	}
	if strings.EqualFold(platform, "android") {
		if c.ResourceID != "" {
			out = append(out, Locator{Using: "id", Value: c.ResourceID})
		}
		if c.Text != "" {
			out = append(out, Locator{Using: "-android uiautomator", Value: `new UiSelector().text("` + escapeSelector(c.Text) + `")`})
		}
	} else {
		if c.Name != "" || c.Text != "" {
			name := firstNonEmpty(c.Name, c.Text)
			out = append(out, Locator{Using: "-ios predicate string", Value: `name == "` + escapeSelector(name) + `" OR label == "` + escapeSelector(name) + `"`})
		}
	}
	if len(out) == 0 && (c.Name != "" || c.Text != "") {
		out = append(out, xpathLocator(firstNonEmpty(c.Name, c.Text)))
	}
	return out
}

func xpathLocator(name string) Locator {
	literal := xpathLiteral(name)
	return Locator{Using: "xpath", Value: `//*[@text=` + literal + ` or @name=` + literal + ` or @label=` + literal + `]`}
}

func escapeSelector(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
