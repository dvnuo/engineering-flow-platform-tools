package automation

import "testing"

func TestNormalizeScreenshotOptionsDefaultsAndElementMode(t *testing.T) {
	opts, err := normalizeScreenshotOptions(ScreenshotOptions{})
	if err != nil {
		t.Fatalf("normalize page screenshot failed: %v", err)
	}
	if !opts.FullPage {
		t.Fatalf("page screenshot should default to full page: %#v", opts)
	}

	opts, err = normalizeScreenshotOptions(ScreenshotOptions{Selector: ".avatar", FullPage: true})
	if err != nil {
		t.Fatalf("element screenshot with default full-page flag failed: %v", err)
	}
	if opts.FullPage {
		t.Fatalf("element screenshot should normalize to non-full-page: %#v", opts)
	}

	if _, err := normalizeScreenshotOptions(ScreenshotOptions{Selector: ".avatar", FullPage: true, FullPageSet: true}); err == nil {
		t.Fatalf("explicit full-page element screenshot should fail")
	}
	if _, err := normalizeScreenshotOptions(ScreenshotOptions{Selector: ".avatar", Ref: "axref-0-abc"}); err == nil {
		t.Fatalf("selector and ref together should fail")
	}
}
