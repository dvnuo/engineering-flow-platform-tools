package automation

import (
	"strings"
	"testing"
)

func TestSanitizeFrameInfosRedactsURLsAndNames(t *testing.T) {
	raw := []FrameInfo{{
		ID:         "frame-1",
		ParentID:   "parent-1",
		Name:       "session=abc",
		URL:        "https://intranet.test/frame?access_token=secret",
		MimeType:   "TEXT/HTML",
		Depth:      -1,
		ChildCount: -2,
	}}
	got := sanitizeFrameInfos(raw)
	frame := got[0]
	joined := frame.Name + frame.URL
	for _, leaked := range []string{"session=abc", "access_token=secret"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("frame leaked %q in %#v", leaked, frame)
		}
	}
	if frame.MimeType != "text/html" || frame.Depth != 0 || frame.ChildCount != 0 {
		t.Fatalf("frame not normalized: %#v", frame)
	}
}

func TestNormalizeFrameSnapshotOptionsDefaults(t *testing.T) {
	opts := normalizeFrameSnapshotOptions(FrameSnapshotOptions{})
	if opts.MaxTextBytes != 4000 || opts.MaxHTMLBytes != 20000 {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
}

func TestShadowTraversalExpressionsIncludeBoundedPierceHelper(t *testing.T) {
	extract := extractExpression("button", 10, false, true)
	outline := outlineExpression(10, false, true)
	ax := axExpression(AXOptions{Limit: 10, Pierce: true})
	for name, expr := range map[string]string{"extract": extract, "outline": outline, "ax": ax} {
		if !strings.Contains(expr, "querySelectorAllPierce") || !strings.Contains(expr, "shadowRoot") || !strings.Contains(expr, "10000") {
			t.Fatalf("%s expression missing bounded shadow traversal helper:\n%s", name, expr)
		}
	}
}
