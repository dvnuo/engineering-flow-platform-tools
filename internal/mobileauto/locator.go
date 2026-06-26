package mobileauto

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
			out = append(out, Locator{Using: "-android uiautomator", Value: `new UiSelector().text("` + selectorString(c.Text) + `")`})
		}
	} else {
		if c.Name != "" || c.Text != "" {
			name := firstNonEmpty(c.Name, c.Text)
			quoted := selectorString(name)
			out = append(out, Locator{Using: "-ios predicate string", Value: `name == "` + quoted + `" OR label == "` + quoted + `"`})
			out = append(out, Locator{Using: "-ios class chain", Value: iosClassChain(c, quoted)})
		}
	}
	if c.Name != "" || c.Text != "" {
		out = append(out, xpathLocator(firstNonEmpty(c.Name, c.Text)))
	}
	return out
}

func iosClassChain(c Candidate, quotedName string) string {
	className := "*"
	if strings.HasPrefix(c.Class, "XCUIElementType") {
		className = c.Class
	}
	return "**/" + className + "[`name == \"" + quotedName + "\" OR label == \"" + quotedName + "\"`]"
}

func xpathLocator(name string) Locator {
	literal := xpathLiteral(name)
	return Locator{Using: "xpath", Value: `//*[@text=` + literal + ` or @name=` + literal + ` or @label=` + literal + `]`}
}

func selectorString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
