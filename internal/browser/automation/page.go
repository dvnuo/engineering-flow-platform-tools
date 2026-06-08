package automation

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cdpRuntime "github.com/chromedp/cdproto/runtime"
	cdpTarget "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type PageOptions struct {
	SessionName    string
	TargetID       string
	TimeoutSeconds int
}

type SnapshotOptions struct {
	PageOptions
	IncludeHTML  bool
	MaxTextBytes int
	MaxHTMLBytes int
}

type SnapshotResult struct {
	Session         string `json:"session"`
	TargetID        string `json:"target_id"`
	URL             string `json:"url"`
	Title           string `json:"title"`
	BodyTextPreview string `json:"body_text_preview,omitempty"`
	TextLength      int    `json:"text_length"`
	HTMLPreview     string `json:"html_preview,omitempty"`
	HTMLLength      int    `json:"html_length,omitempty"`
}

type ExtractOptions struct {
	PageOptions
	Selector     string
	Limit        int
	IncludeHTML  bool
	Pierce       bool
	MaxHTMLBytes int
}

type ExtractedElement struct {
	Index     int    `json:"index"`
	Text      string `json:"text,omitempty"`
	Value     string `json:"value,omitempty"`
	Href      string `json:"href,omitempty"`
	Title     string `json:"title,omitempty"`
	AriaLabel string `json:"aria_label,omitempty"`
	TagName   string `json:"tag_name,omitempty"`
	HTML      string `json:"html,omitempty"`
}

type ExtractResult struct {
	Session  string             `json:"session"`
	TargetID string             `json:"target_id"`
	Selector string             `json:"selector"`
	Pierce   bool               `json:"pierce,omitempty"`
	Count    int                `json:"count"`
	Limit    int                `json:"limit"`
	URL      string             `json:"url"`
	Title    string             `json:"title"`
	Elements []ExtractedElement `json:"elements"`
}

type ClickOptions struct {
	PageOptions
	Selector string
	Ref      string
}

type TypeOptions struct {
	PageOptions
	Selector string
	Ref      string
	Text     string
	Clear    bool
}

type WaitOptions struct {
	PageOptions
	Selector                string
	DurationMilliseconds    int
	URLContains             string
	Text                    string
	NetworkIdleMilliseconds int
	DOMStableMilliseconds   int
}

type PageActionResult struct {
	Session              string `json:"session"`
	TargetID             string `json:"target_id"`
	Action               string `json:"action"`
	Selector             string `json:"selector,omitempty"`
	Ref                  string `json:"ref,omitempty"`
	URL                  string `json:"url"`
	Title                string `json:"title"`
	TextBytes            int    `json:"text_bytes,omitempty"`
	Key                  string `json:"key,omitempty"`
	SelectionMode        string `json:"selection_mode,omitempty"`
	SelectedCount        int    `json:"selected_count,omitempty"`
	Checked              *bool  `json:"checked,omitempty"`
	DurationMilliseconds int    `json:"duration_ms,omitempty"`
}

type WaitConditionResult struct {
	Condition            string `json:"condition"`
	Satisfied            bool   `json:"satisfied"`
	WaitMilliseconds     int    `json:"wait_ms,omitempty"`
	DurationMilliseconds int    `json:"duration_ms,omitempty"`
	ResourceCount        int    `json:"resource_count,omitempty"`
	TextBytes            int    `json:"text_bytes,omitempty"`
	HTMLBytes            int    `json:"html_bytes,omitempty"`
	NodeCount            int    `json:"node_count,omitempty"`
}

type WaitResult struct {
	Session              string                `json:"session"`
	TargetID             string                `json:"target_id"`
	Action               string                `json:"action"`
	Selector             string                `json:"selector,omitempty"`
	URL                  string                `json:"url"`
	Title                string                `json:"title"`
	DurationMilliseconds int                   `json:"duration_ms,omitempty"`
	Conditions           []WaitConditionResult `json:"conditions"`
}

type ScreenshotOptions struct {
	PageOptions
	OutPath     string
	FullPage    bool
	FullPageSet bool
	Selector    string
	Ref         string
}

type ScreenshotResult struct {
	Session  string `json:"session"`
	TargetID string `json:"target_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Mode     string `json:"mode"`
	FullPage bool   `json:"full_page"`
	Selector string `json:"selector,omitempty"`
	Ref      string `json:"ref,omitempty"`
	MIMEType string `json:"mime_type"`
}

type EvalOptions struct {
	PageOptions
	Expression     string
	MaxStringBytes int
}

type EvalResult struct {
	Session        string `json:"session"`
	TargetID       string `json:"target_id"`
	URL            string `json:"url"`
	Title          string `json:"title"`
	MaxStringBytes int    `json:"max_string_bytes"`
	Value          any    `json:"value"`
}

type FetchOptions struct {
	PageOptions
	URL          string
	MaxBodyBytes int
}

type FetchResult struct {
	Session      string `json:"session"`
	TargetID     string `json:"target_id"`
	RequestedURL string `json:"requested_url"`
	URL          string `json:"url"`
	OK           bool   `json:"ok"`
	Status       int    `json:"status"`
	BodyPreview  string `json:"body_preview,omitempty"`
	BodyLength   int    `json:"body_length"`
	Truncated    bool   `json:"truncated"`
	Error        string `json:"error,omitempty"`
}

func (m *Manager) Snapshot(ctx context.Context, opts SnapshotOptions) (SnapshotResult, error) {
	if opts.MaxTextBytes <= 0 {
		opts.MaxTextBytes = 4000
	}
	if opts.MaxHTMLBytes <= 0 {
		opts.MaxHTMLBytes = 20000
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return SnapshotResult{}, err
	}
	defer cancel()

	var finalURL, title, bodyText, html string
	actions := []chromedp.Action{
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Text("body", &bodyText, chromedp.ByQuery),
	}
	if opts.IncludeHTML {
		actions = append(actions, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
	}
	if err := chromedp.Run(pageCtx, actions...); err != nil {
		return SnapshotResult{}, mapPageError(err, "automation_failed")
	}
	result := SnapshotResult{
		Session:         session.Name,
		TargetID:        target.ID,
		URL:             RedactURL(finalURL),
		Title:           RedactString(title),
		BodyTextPreview: TruncateBytes(RedactString(bodyText), opts.MaxTextBytes),
		TextLength:      len(bodyText),
	}
	if opts.IncludeHTML {
		result.HTMLLength = len(html)
		result.HTMLPreview = TruncateBytes(RedactString(html), opts.MaxHTMLBytes)
	}
	return result, nil
}

func (m *Manager) Extract(ctx context.Context, opts ExtractOptions) (ExtractResult, error) {
	if strings.TrimSpace(opts.Selector) == "" {
		return ExtractResult{}, invalidArgs("--selector is required", "Run browser schema page.extract --json.")
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.MaxHTMLBytes <= 0 {
		opts.MaxHTMLBytes = 20000
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ExtractResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count    int                `json:"count"`
		Elements []ExtractedElement `json:"elements"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(extractExpression(opts.Selector, opts.Limit, opts.IncludeHTML, opts.Pierce), &raw, chromedp.EvalAsValue),
	); err != nil {
		return ExtractResult{}, mapPageError(err, "automation_failed")
	}
	return ExtractResult{
		Session:  session.Name,
		TargetID: target.ID,
		Selector: opts.Selector,
		Pierce:   opts.Pierce,
		Count:    raw.Count,
		Limit:    opts.Limit,
		URL:      RedactURL(finalURL),
		Title:    RedactString(title),
		Elements: sanitizeExtractedElements(raw.Elements, opts.IncludeHTML, opts.MaxHTMLBytes),
	}, nil
}

func (m *Manager) Click(ctx context.Context, opts ClickOptions) (PageActionResult, error) {
	if err := validateActionTarget(opts.Selector, opts.Ref, "page.click"); err != nil {
		return PageActionResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return PageActionResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return PageActionResult{}, err
	}

	var finalURL, title string
	if err := chromedp.Run(pageCtx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return PageActionResult{}, mapPageError(err, "automation_failed")
	}
	return pageActionResult(session, target, "click", selector, ref, finalURL, title), nil
}

func (m *Manager) Type(ctx context.Context, opts TypeOptions) (PageActionResult, error) {
	if err := validateActionTarget(opts.Selector, opts.Ref, "page.type"); err != nil {
		return PageActionResult{}, err
	}
	if strings.TrimSpace(opts.Text) == "" {
		return PageActionResult{}, invalidArgs("--text is required", "Pass text to type; command output will report only byte count.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return PageActionResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return PageActionResult{}, err
	}

	actions := []chromedp.Action{
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	}
	if opts.Clear {
		actions = append(actions, chromedp.Clear(selector, chromedp.ByQuery))
	}
	var finalURL, title string
	actions = append(actions,
		chromedp.SendKeys(selector, opts.Text, chromedp.ByQuery),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	)
	if err := chromedp.Run(pageCtx, actions...); err != nil {
		return PageActionResult{}, mapPageError(err, "automation_failed")
	}
	result := pageActionResult(session, target, "type", selector, ref, finalURL, title)
	result.TextBytes = len(opts.Text)
	return result, nil
}

func (m *Manager) Wait(ctx context.Context, opts WaitOptions) (WaitResult, error) {
	if err := validateWaitOptions(opts); err != nil {
		return WaitResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return WaitResult{}, err
	}
	defer cancel()

	conditions := make([]WaitConditionResult, 0, waitConditionCount(opts))
	if strings.TrimSpace(opts.Selector) != "" {
		start := time.Now()
		if err := chromedp.Run(pageCtx, chromedp.WaitVisible(opts.Selector, chromedp.ByQuery)); err != nil {
			return WaitResult{}, mapPageError(err, "automation_failed")
		}
		conditions = append(conditions, WaitConditionResult{
			Condition:        "selector_visible",
			Satisfied:        true,
			WaitMilliseconds: elapsedMilliseconds(start),
		})
	}
	if opts.DurationMilliseconds > 0 {
		start := time.Now()
		if err := chromedp.Run(pageCtx, chromedp.Sleep(time.Duration(opts.DurationMilliseconds)*time.Millisecond)); err != nil {
			return WaitResult{}, mapPageError(err, "automation_failed")
		}
		conditions = append(conditions, WaitConditionResult{
			Condition:            "duration",
			Satisfied:            true,
			WaitMilliseconds:     elapsedMilliseconds(start),
			DurationMilliseconds: opts.DurationMilliseconds,
		})
	}
	if hasAdvancedWaitConditions(opts) {
		results, err := waitForAdvancedPageConditions(pageCtx, opts)
		if err != nil {
			return WaitResult{}, mapPageError(err, "automation_failed")
		}
		conditions = append(conditions, results...)
	}
	var finalURL, title string
	if err := chromedp.Run(pageCtx, chromedp.Location(&finalURL), chromedp.Title(&title)); err != nil {
		return WaitResult{}, mapPageError(err, "automation_failed")
	}
	return WaitResult{
		Session:              session.Name,
		TargetID:             target.ID,
		Action:               "wait",
		Selector:             RedactString(opts.Selector),
		URL:                  RedactURL(finalURL),
		Title:                RedactString(title),
		DurationMilliseconds: opts.DurationMilliseconds,
		Conditions:           conditions,
	}, nil
}

func (m *Manager) Screenshot(ctx context.Context, opts ScreenshotOptions) (ScreenshotResult, error) {
	opts, err := normalizeScreenshotOptions(opts)
	if err != nil {
		return ScreenshotResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ScreenshotResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveOptionalActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return ScreenshotResult{}, err
	}

	outPath := strings.TrimSpace(opts.OutPath)
	if outPath == "" {
		outPath, err = DefaultPageScreenshotPath(m.now())
		if err != nil {
			return ScreenshotResult{}, err
		}
	}
	outPath = filepath.Clean(expandHome(outPath))
	if outPath == "" || outPath == "." {
		return ScreenshotResult{}, invalidArgs("--out must point at a screenshot file", "Pass a writable file path such as result/page-screenshot.png.")
	}

	var finalURL, title string
	var screenshot []byte
	actions := []chromedp.Action{chromedp.Location(&finalURL), chromedp.Title(&title)}
	if selector != "" {
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Screenshot(selector, &screenshot, chromedp.ByQuery),
		)
	} else if opts.FullPage {
		actions = append(actions, chromedp.FullScreenshot(&screenshot, 100))
	} else {
		actions = append(actions, chromedp.CaptureScreenshot(&screenshot))
	}
	if err := chromedp.Run(pageCtx, actions...); err != nil {
		return ScreenshotResult{}, mapPageError(err, "automation_failed")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return ScreenshotResult{}, NewError("artifact_write_failed", err.Error(), "Check --out permissions and available disk space.", 500)
	}
	if err := os.WriteFile(outPath, screenshot, 0o600); err != nil {
		return ScreenshotResult{}, NewError("artifact_write_failed", err.Error(), "Check --out permissions and available disk space.", 500)
	}
	stat, err := os.Stat(outPath)
	if err != nil {
		return ScreenshotResult{}, NewError("artifact_write_failed", err.Error(), "Screenshot was written but metadata could not be read.", 500)
	}
	mode := "page"
	if selector != "" {
		mode = "element"
	}
	return ScreenshotResult{
		Session:  session.Name,
		TargetID: target.ID,
		URL:      RedactURL(finalURL),
		Title:    RedactString(title),
		Path:     outPath,
		Bytes:    stat.Size(),
		Mode:     mode,
		FullPage: opts.FullPage && selector == "",
		Selector: normalizeSelectorHint(selector),
		Ref:      RedactString(ref),
		MIMEType: "image/png",
	}, nil
}

func normalizeScreenshotOptions(opts ScreenshotOptions) (ScreenshotOptions, error) {
	if err := validateOptionalActionTarget(opts.Selector, opts.Ref, "page.screenshot"); err != nil {
		return opts, err
	}
	targeted := strings.TrimSpace(opts.Selector) != "" || strings.TrimSpace(opts.Ref) != ""
	if targeted {
		if opts.FullPageSet && opts.FullPage {
			return opts, invalidArgs("--full-page cannot be combined with --selector or --ref", "Element screenshots capture only the selected visible element; remove --full-page.")
		}
		opts.FullPage = false
	} else if !opts.FullPageSet {
		opts.FullPage = true
	}
	return opts, nil
}

func (m *Manager) Eval(ctx context.Context, opts EvalOptions) (EvalResult, error) {
	if err := validateEvalExpression(opts.Expression); err != nil {
		return EvalResult{}, err
	}
	if opts.MaxStringBytes <= 0 {
		opts.MaxStringBytes = 20000
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return EvalResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw any
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(evalExpression(opts.Expression), &raw, chromedp.EvalAsValue, evalAwaitPromise),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return EvalResult{}, mapPageError(err, "automation_failed")
	}
	return EvalResult{
		Session:        session.Name,
		TargetID:       target.ID,
		URL:            RedactURL(finalURL),
		Title:          RedactString(title),
		MaxStringBytes: opts.MaxStringBytes,
		Value:          SanitizeValue(raw, opts.MaxStringBytes),
	}, nil
}

func (m *Manager) Fetch(ctx context.Context, opts FetchOptions) (FetchResult, error) {
	if err := validateFetchURL(opts.URL); err != nil {
		return FetchResult{}, err
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 20000
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return FetchResult{}, err
	}
	defer cancel()

	var raw fetchEvalResult
	if err := chromedp.Run(pageCtx, chromedp.Evaluate(fetchExpression(opts.URL, opts.MaxBodyBytes), &raw, chromedp.EvalAsValue, evalAwaitPromise)); err != nil {
		return FetchResult{}, mapPageError(err, "automation_failed")
	}
	return sanitizeFetchResult(session, target, opts.URL, raw, opts.MaxBodyBytes), nil
}

func (m *Manager) attachPage(ctx context.Context, opts PageOptions) (context.Context, context.CancelFunc, Session, Target, error) {
	session, target, err := m.ResolveTarget(ctx, opts.SessionName, opts.TargetID)
	if err != nil {
		return nil, nil, Session{}, Target{}, err
	}
	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, timeout)
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(timeoutCtx, session.BrowserWebSocketURL)
	pageCtx, cancelPage := chromedp.NewContext(allocCtx, chromedp.WithTargetID(cdpTarget.ID(target.ID)))
	cancel := func() {
		cancelPage()
		cancelAlloc()
		cancelTimeout()
	}
	return pageCtx, cancel, session, target, nil
}

func PageTimeoutSeconds(seconds int) int {
	if seconds <= 0 {
		return 30
	}
	return seconds
}

func DefaultPageScreenshotPath(now time.Time) (string, error) {
	root, err := DefaultBrowserHome()
	if err != nil {
		return "", err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	name := fmt.Sprintf("page-screenshot-%s-%09d.png", now.Format("20060102-150405"), now.Nanosecond())
	return filepath.Join(root, "artifacts", name), nil
}

func pageActionResult(session Session, target Target, action, selector, ref, finalURL, title string) PageActionResult {
	return PageActionResult{
		Session:  session.Name,
		TargetID: target.ID,
		Action:   action,
		Selector: normalizeSelectorHint(selector),
		Ref:      RedactString(ref),
		URL:      RedactURL(finalURL),
		Title:    RedactString(title),
	}
}

func extractExpression(selector string, limit int, includeHTML, pierce bool) string {
	return `(function () {
  const selector = ` + strconv.Quote(selector) + `;
  const limit = ` + strconv.Itoa(limit) + `;
  const includeHTML = ` + strconv.FormatBool(includeHTML) + `;
  const pierce = ` + strconv.FormatBool(pierce) + `;
  const nodes = querySelectorAllPierce(document, selector, pierce, 10000);
  return {
    count: nodes.length,
    elements: nodes.slice(0, limit).map((el, index) => ({
      index,
      text: el.innerText || el.textContent || "",
      value: "value" in el ? String(el.value || "") : "",
      href: "href" in el ? String(el.href || "") : String(el.getAttribute("href") || ""),
      title: String(el.getAttribute("title") || ""),
      aria_label: String(el.getAttribute("aria-label") || ""),
      tag_name: String(el.tagName || "").toLowerCase(),
      html: includeHTML ? String(el.outerHTML || "") : ""
    }))
  };
})()
` + shadowTraversalExpression()
}

func validateWaitOptions(opts WaitOptions) error {
	if opts.DurationMilliseconds < 0 {
		return invalidArgs("--duration-ms must be zero or greater", "Pass a non-negative duration in milliseconds.")
	}
	if opts.NetworkIdleMilliseconds < 0 {
		return invalidArgs("--network-idle-ms must be zero or greater", "Pass a non-negative idle duration in milliseconds.")
	}
	if opts.DOMStableMilliseconds < 0 {
		return invalidArgs("--dom-stable-ms must be zero or greater", "Pass a non-negative stable duration in milliseconds.")
	}
	if waitConditionCount(opts) == 0 {
		return invalidArgs(
			"--selector, --duration-ms, --url-contains, --text, --network-idle-ms, or --dom-stable-ms is required",
			"Pass one or more bounded page wait conditions.",
		)
	}
	return nil
}

func waitConditionCount(opts WaitOptions) int {
	count := 0
	if strings.TrimSpace(opts.Selector) != "" {
		count++
	}
	if opts.DurationMilliseconds > 0 {
		count++
	}
	if strings.TrimSpace(opts.URLContains) != "" {
		count++
	}
	if strings.TrimSpace(opts.Text) != "" {
		count++
	}
	if opts.NetworkIdleMilliseconds > 0 {
		count++
	}
	if opts.DOMStableMilliseconds > 0 {
		count++
	}
	return count
}

func hasAdvancedWaitConditions(opts WaitOptions) bool {
	return strings.TrimSpace(opts.URLContains) != "" ||
		strings.TrimSpace(opts.Text) != "" ||
		opts.NetworkIdleMilliseconds > 0 ||
		opts.DOMStableMilliseconds > 0
}

type waitPollSnapshot struct {
	URLContains         bool    `json:"url_contains"`
	TextContains        bool    `json:"text_contains"`
	ResourceCount       int     `json:"resource_count"`
	ResourceFingerprint string  `json:"resource_fingerprint"`
	ResourceLatest      float64 `json:"resource_latest"`
	DOMFingerprint      string  `json:"dom_fingerprint"`
	TextBytes           int     `json:"text_bytes"`
	HTMLBytes           int     `json:"html_bytes"`
	NodeCount           int     `json:"node_count"`
}

func waitForAdvancedPageConditions(ctx context.Context, opts WaitOptions) ([]WaitConditionResult, error) {
	start := time.Now()
	needURL := strings.TrimSpace(opts.URLContains) != ""
	needText := strings.TrimSpace(opts.Text) != ""
	needNetwork := opts.NetworkIdleMilliseconds > 0
	needDOM := opts.DOMStableMilliseconds > 0
	networkTracker := newStableTracker(time.Duration(opts.NetworkIdleMilliseconds) * time.Millisecond)
	domTracker := newStableTracker(time.Duration(opts.DOMStableMilliseconds) * time.Millisecond)
	var networkOK, domOK bool

	for {
		var snap waitPollSnapshot
		if err := chromedp.Evaluate(waitSnapshotExpression(opts.URLContains, opts.Text), &snap, chromedp.EvalAsValue).Do(ctx); err != nil {
			return nil, err
		}
		now := time.Now()
		if needNetwork {
			networkKey := fmt.Sprintf("%d:%s:%.3f", snap.ResourceCount, snap.ResourceFingerprint, snap.ResourceLatest)
			networkOK, _ = networkTracker.Observe(now, networkKey)
		}
		if needDOM {
			domKey := fmt.Sprintf("%s:%d:%d:%d", snap.DOMFingerprint, snap.TextBytes, snap.HTMLBytes, snap.NodeCount)
			domOK, _ = domTracker.Observe(now, domKey)
		}
		if (!needURL || snap.URLContains) &&
			(!needText || snap.TextContains) &&
			(!needNetwork || networkOK) &&
			(!needDOM || domOK) {
			waitMS := elapsedMilliseconds(start)
			results := make([]WaitConditionResult, 0, 4)
			if needURL {
				results = append(results, WaitConditionResult{Condition: "url_contains", Satisfied: true, WaitMilliseconds: waitMS})
			}
			if needText {
				results = append(results, WaitConditionResult{Condition: "text_contains", Satisfied: true, WaitMilliseconds: waitMS, TextBytes: snap.TextBytes})
			}
			if needNetwork {
				results = append(results, WaitConditionResult{
					Condition:            "network_idle",
					Satisfied:            true,
					WaitMilliseconds:     waitMS,
					DurationMilliseconds: opts.NetworkIdleMilliseconds,
					ResourceCount:        snap.ResourceCount,
				})
			}
			if needDOM {
				results = append(results, WaitConditionResult{
					Condition:            "dom_stable",
					Satisfied:            true,
					WaitMilliseconds:     waitMS,
					DurationMilliseconds: opts.DOMStableMilliseconds,
					TextBytes:            snap.TextBytes,
					HTMLBytes:            snap.HTMLBytes,
					NodeCount:            snap.NodeCount,
				})
			}
			return results, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait conditions were not satisfied before timeout: %w", ctx.Err())
		case <-time.After(150 * time.Millisecond):
		}
	}
}

type stableTracker struct {
	duration time.Duration
	key      string
	since    time.Time
	seen     bool
	samples  int
}

func newStableTracker(duration time.Duration) *stableTracker {
	return &stableTracker{duration: duration}
}

func (t *stableTracker) Observe(now time.Time, key string) (bool, time.Duration) {
	if t == nil || t.duration <= 0 {
		return true, 0
	}
	if !t.seen || key != t.key {
		t.key = key
		t.since = now
		t.seen = true
		t.samples = 1
		return false, 0
	}
	t.samples++
	stableFor := now.Sub(t.since)
	return stableFor >= t.duration, stableFor
}

func elapsedMilliseconds(start time.Time) int {
	if start.IsZero() {
		return 0
	}
	ms := int(time.Since(start) / time.Millisecond)
	if ms < 0 {
		return 0
	}
	return ms
}

func waitSnapshotExpression(urlContains, textContains string) string {
	return `(function () {
  const urlNeedle = ` + strconv.Quote(strings.TrimSpace(urlContains)) + `;
  const textNeedle = ` + strconv.Quote(strings.TrimSpace(textContains)) + `;
  const hashString = function (value) {
    value = String(value || "");
    let hash = 2166136261;
    for (let i = 0; i < value.length; i++) {
      hash ^= value.charCodeAt(i);
      hash = Math.imul(hash, 16777619);
    }
    return String(hash >>> 0);
  };
  const body = document.body || document.documentElement || null;
  const text = body ? String(body.innerText || body.textContent || "") : "";
  const html = body ? String(body.outerHTML || "") : "";
  const resources = performance && performance.getEntriesByType ? performance.getEntriesByType("resource") : [];
  let resourceLatest = 0;
  let resourceShape = "";
  for (const entry of resources) {
    const end = Number(entry.responseEnd || (entry.startTime + entry.duration) || entry.startTime || 0);
    resourceLatest = Math.max(resourceLatest, end);
    resourceShape += [
      Math.round(Number(entry.startTime || 0)),
      Math.round(Number(entry.duration || 0)),
      Math.round(Number(entry.transferSize || 0)),
      Math.round(Number(entry.encodedBodySize || 0)),
      Math.round(Number(entry.decodedBodySize || 0))
    ].join(":") + "|";
  }
  const nodeCount = document.getElementsByTagName ? document.getElementsByTagName("*").length : 0;
  return {
    url_contains: urlNeedle === "" ? true : String(location.href || "").includes(urlNeedle),
    text_contains: textNeedle === "" ? true : text.includes(textNeedle),
    resource_count: resources.length,
    resource_fingerprint: hashString(resourceShape),
    resource_latest: resourceLatest,
    dom_fingerprint: hashString(String(text.length) + "|" + String(html.length) + "|" + String(nodeCount) + "|" + hashString(text) + "|" + hashString(html)),
    text_bytes: new TextEncoder().encode(text).length,
    html_bytes: new TextEncoder().encode(html).length,
    node_count: nodeCount
  };
})()`
}

func evalExpression(expression string) string {
	return `(async function () {
  return await (` + strings.TrimSpace(expression) + `);
})()`
}

func evalAwaitPromise(p *cdpRuntime.EvaluateParams) *cdpRuntime.EvaluateParams {
	return p.WithAwaitPromise(true)
}

func validateEvalExpression(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return invalidArgs("--expr is required", "Pass a JavaScript expression that returns a serializable value.")
	}
	if len(expression) > 10000 {
		return invalidArgs("--expr is too large", "Keep page eval expressions under 10000 bytes.")
	}
	lower := strings.ToLower(expression)
	for _, forbidden := range []string{
		"document.cookie",
		"cookiestore",
		"localstorage",
		"sessionstorage",
		"indexeddb",
		"navigator.credentials",
		"new headers",
		".headers",
		"headers[",
		"authorization",
		"set-cookie",
		"fetch(",
		"xmlhttprequest",
		"sendbeacon",
		"websocket",
	} {
		if strings.Contains(lower, forbidden) {
			return invalidArgs("--expr may not access cookies, storage, headers, credentials, or network APIs", "Use page snapshot, page extract, or page fetch for sanitized page reads.")
		}
	}
	return nil
}

type fetchEvalResult struct {
	OK          bool   `json:"ok"`
	Status      int    `json:"status"`
	URL         string `json:"url"`
	BodyPreview string `json:"body_preview"`
	BodyLength  int    `json:"body_length"`
	Truncated   bool   `json:"truncated"`
	Error       string `json:"error"`
}

func validateFetchURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return invalidArgs("--url is required", "Pass an HTTP, HTTPS, or relative URL to fetch from the current page context.")
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return invalidArgs("--url is not valid", "Pass an HTTP, HTTPS, or relative URL.")
	}
	if u.Scheme == "" {
		if strings.HasPrefix(raw, "//") || u.Host != "" {
			return invalidArgs("--url must include http or https when a host is provided", "Pass a full URL such as https://intranet.example.test/api/status.")
		}
		return nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalidArgs("--url must use http or https", "Unsafe URL schemes such as file:, data:, javascript:, chrome:, and about: are not allowed.")
	}
	if u.Host == "" {
		return invalidArgs("--url must include a host for absolute URLs", "Pass a full URL such as https://intranet.example.test/api/status.")
	}
	return nil
}

func fetchExpression(rawURL string, maxBodyBytes int) string {
	return `(async function () {
  const target = ` + strconv.Quote(strings.TrimSpace(rawURL)) + `;
  const maxBodyBytes = ` + strconv.Itoa(maxBodyBytes) + `;
  try {
    const res = await fetch(target, {
      method: "GET",
      credentials: "omit",
      cache: "no-store",
      redirect: "follow"
    });
    const body = await res.text();
    return {
      ok: res.ok,
      status: res.status,
      url: res.url,
      body_preview: body.slice(0, maxBodyBytes),
      body_length: body.length,
      truncated: body.length > maxBodyBytes,
      error: ""
    };
  } catch (err) {
    return {
      ok: false,
      status: 0,
      url: target,
      body_preview: "",
      body_length: 0,
      truncated: false,
      error: String(err)
    };
  }
})()`
}

func sanitizeFetchResult(session Session, target Target, requestedURL string, raw fetchEvalResult, maxBodyBytes int) FetchResult {
	bodyPreview := TruncateBytes(RedactString(raw.BodyPreview), maxBodyBytes)
	return FetchResult{
		Session:      session.Name,
		TargetID:     target.ID,
		RequestedURL: RedactURL(requestedURL),
		URL:          RedactURL(raw.URL),
		OK:           raw.OK,
		Status:       raw.Status,
		BodyPreview:  bodyPreview,
		BodyLength:   raw.BodyLength,
		Truncated:    raw.Truncated || len(bodyPreview) > maxBodyBytes,
		Error:        RedactError(raw.Error),
	}
}

func sanitizeExtractedElements(elements []ExtractedElement, includeHTML bool, maxHTMLBytes int) []ExtractedElement {
	out := make([]ExtractedElement, len(elements))
	for i, el := range elements {
		el.Text = RedactString(el.Text)
		el.Value = RedactString(el.Value)
		el.Href = RedactURL(el.Href)
		el.Title = RedactString(el.Title)
		el.AriaLabel = RedactString(el.AriaLabel)
		el.TagName = strings.ToLower(RedactString(el.TagName))
		if includeHTML && el.HTML != "" {
			el.HTML = TruncateBytes(RedactString(el.HTML), maxHTMLBytes)
		} else {
			el.HTML = ""
		}
		out[i] = el
	}
	return out
}

func mapPageError(err error, code string) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "deadline") || strings.Contains(msg, "timeout") {
		return NewError("timeout", err.Error(), "Increase --timeout or check whether the page is responsive.", 408)
	}
	return NewError(code, err.Error(), "Check the target tab and selector, then retry.", 500)
}
