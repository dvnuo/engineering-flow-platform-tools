package automation

import (
	"strings"
	"testing"
)

func TestNormalizeAXOptionsDefaultsAndCapsLimit(t *testing.T) {
	if got := normalizeAXOptions(AXOptions{}).Limit; got != 100 {
		t.Fatalf("default limit = %d", got)
	}
	if got := normalizeAXOptions(AXOptions{Limit: 1000}).Limit; got != 500 {
		t.Fatalf("capped limit = %d", got)
	}
	if got := normalizeAXOptions(AXOptions{Limit: 25}).Limit; got != 25 {
		t.Fatalf("explicit limit = %d", got)
	}
}

func TestStableElementRefIsDeterministicAndSecretFree(t *testing.T) {
	entry := AXRefEntry{
		Selector: "form > input[name='access_token']",
		Role:     "textbox",
		Name:     "token=secret",
		FrameID:  "frame-1",
		TargetID: "target-1",
	}
	got := StableElementRef(entry, 2)
	again := StableElementRef(entry, 2)
	if got != again {
		t.Fatalf("ref not deterministic: %q != %q", got, again)
	}
	for _, leaked := range []string{"secret", "access_token", "textbox", "frame-1"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("ref leaked %q: %s", leaked, got)
		}
	}
	if !strings.HasPrefix(got, "axref-2-") {
		t.Fatalf("unexpected ref format: %s", got)
	}
}

func TestSanitizeAXNodesRedactsAndAssignsRefs(t *testing.T) {
	raw := []AXNode{{
		Role:        "TEXTBOX",
		Name:        "token=secret",
		Description: "Authorization: Bearer private",
		Title:       "session=abc",
		FrameID:     "frame-secret",
		Selector:    "input[name='access_token']",
		Tag:         "INPUT",
		InputType:   "PASSWORD",
		Source:      "raw",
	}}
	got := sanitizeAXNodes(raw, "target-1")
	if len(got) != 1 {
		t.Fatalf("nodes=%#v", got)
	}
	node := got[0]
	joined := node.Name + node.Description + node.Title + node.Selector + node.Ref
	for _, leaked := range []string{"secret", "Bearer private", "session=abc", "access_token"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("AX node leaked %q in %#v", leaked, node)
		}
	}
	if node.Role != "textbox" || node.Tag != "input" || node.InputType != "password" || node.Source != axSourceDOMARIA {
		t.Fatalf("fields not normalized: %#v", node)
	}
	if node.Ref == "" {
		t.Fatalf("ref missing: %#v", node)
	}
}

func TestSafeRefFilePartNormalizesTargetID(t *testing.T) {
	got := safeRefFilePart("../target:id/with spaces")
	if strings.Contains(got, "/") || strings.Contains(got, "..") || strings.Contains(got, " ") || got == "" {
		t.Fatalf("unsafe file part: %q", got)
	}
	if len(safeRefFilePart(strings.Repeat("a", 200))) > 120 {
		t.Fatalf("file part was not capped")
	}
}
