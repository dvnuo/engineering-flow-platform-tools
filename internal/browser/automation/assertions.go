package automation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

const assertionFailureCode = "assertion_failed"

type AssertionOptions struct {
	PageOptions
	Selector string
	Ref      string
	Contains string
	Not      bool
	Equals   int
	Min      int
	Max      int
}

type AssertionExpected struct {
	ContainsPreview string `json:"contains_preview,omitempty"`
	ContainsBytes   int    `json:"contains_bytes,omitempty"`
	Equals          *int   `json:"equals,omitempty"`
	Min             *int   `json:"min,omitempty"`
	Max             *int   `json:"max,omitempty"`
}

type AssertionObserved struct {
	Visible     *bool `json:"visible,omitempty"`
	Contains    *bool `json:"contains,omitempty"`
	Count       *int  `json:"count,omitempty"`
	TextBytes   int   `json:"text_bytes,omitempty"`
	URLBytes    int   `json:"url_bytes,omitempty"`
	TitleBytes  int   `json:"title_bytes,omitempty"`
	TargetCount int   `json:"target_count,omitempty"`
}

type AssertionResult struct {
	Session   string            `json:"session"`
	TargetID  string            `json:"target_id"`
	Assertion string            `json:"assertion"`
	Pass      bool              `json:"pass"`
	Negated   bool              `json:"negated,omitempty"`
	Selector  string            `json:"selector,omitempty"`
	Ref       string            `json:"ref,omitempty"`
	URL       string            `json:"url"`
	Title     string            `json:"title"`
	Expected  AssertionExpected `json:"expected,omitempty"`
	Observed  AssertionObserved `json:"observed"`
	Message   string            `json:"message,omitempty"`
}

type AssertionError struct {
	Base   *Error
	Result AssertionResult
}

func (e *AssertionError) Error() string {
	if e == nil || e.Base == nil {
		return ""
	}
	return e.Base.Error()
}

func (m *Manager) AssertVisible(ctx context.Context, opts AssertionOptions) (AssertionResult, error) {
	if err := validateActionTarget(opts.Selector, opts.Ref, "assert.visible"); err != nil {
		return AssertionResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return AssertionResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return AssertionResult{}, err
	}

	var finalURL, title string
	var raw visibleAssertionRaw
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(visibleAssertionExpression(selector), &raw, chromedp.EvalAsValue),
	); err != nil {
		return AssertionResult{}, mapPageError(err, "automation_failed")
	}
	pass := applyNegation(raw.Visible, opts.Not)
	visible := raw.Visible
	count := raw.Count
	result := assertionBaseResult(session, target, "visible", selector, ref, finalURL, title, opts.Not)
	result.Pass = pass
	result.Observed = AssertionObserved{
		Visible:     &visible,
		Count:       &count,
		TargetCount: raw.Count,
	}
	return result, assertionFailure(result)
}

func (m *Manager) AssertText(ctx context.Context, opts AssertionOptions) (AssertionResult, error) {
	if strings.TrimSpace(opts.Contains) == "" {
		return AssertionResult{}, invalidArgs("--contains is required", "Pass the text substring to assert; output returns only redacted/truncated expectation metadata.")
	}
	if err := validateOptionalActionTarget(opts.Selector, opts.Ref, "assert.text"); err != nil {
		return AssertionResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return AssertionResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveOptionalActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return AssertionResult{}, err
	}

	var finalURL, title string
	var raw textAssertionRaw
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(textAssertionExpression(selector, opts.Contains), &raw, chromedp.EvalAsValue),
	); err != nil {
		return AssertionResult{}, mapPageError(err, "automation_failed")
	}
	pass := applyNegation(raw.Contains, opts.Not)
	contains := raw.Contains
	count := raw.Count
	result := assertionBaseResult(session, target, "text", selector, ref, finalURL, title, opts.Not)
	result.Pass = pass
	result.Expected = assertionContainsExpected(opts.Contains)
	result.Observed = AssertionObserved{
		Contains:    &contains,
		Count:       &count,
		TextBytes:   raw.TextBytes,
		TargetCount: raw.Count,
	}
	return result, assertionFailure(result)
}

func (m *Manager) AssertURL(ctx context.Context, opts AssertionOptions) (AssertionResult, error) {
	if strings.TrimSpace(opts.Contains) == "" {
		return AssertionResult{}, invalidArgs("--contains is required", "Pass the URL substring to assert; returned URLs are redacted.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return AssertionResult{}, err
	}
	defer cancel()

	var finalURL, title string
	if err := chromedp.Run(pageCtx, chromedp.Location(&finalURL), chromedp.Title(&title)); err != nil {
		return AssertionResult{}, mapPageError(err, "automation_failed")
	}
	contains := strings.Contains(finalURL, opts.Contains)
	pass := applyNegation(contains, opts.Not)
	result := assertionBaseResult(session, target, "url", "", "", finalURL, title, opts.Not)
	result.Pass = pass
	result.Expected = assertionContainsExpected(opts.Contains)
	result.Observed = AssertionObserved{
		Contains:   &contains,
		URLBytes:   len(finalURL),
		TitleBytes: len(title),
	}
	return result, assertionFailure(result)
}

func (m *Manager) AssertCount(ctx context.Context, opts AssertionOptions) (AssertionResult, error) {
	opts = normalizeCountAssertionOptions(opts)
	if err := validateCountAssertionOptions(opts); err != nil {
		return AssertionResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return AssertionResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var count int
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(countAssertionExpression(opts.Selector), &count, chromedp.EvalAsValue),
	); err != nil {
		return AssertionResult{}, mapPageError(err, "automation_failed")
	}
	pass := countAssertionPass(count, opts)
	result := assertionBaseResult(session, target, "count", opts.Selector, "", finalURL, title, false)
	result.Pass = pass
	result.Expected = countAssertionExpected(opts)
	result.Observed = AssertionObserved{Count: &count, TargetCount: count}
	return result, assertionFailure(result)
}

type visibleAssertionRaw struct {
	Visible bool `json:"visible"`
	Count   int  `json:"count"`
}

type textAssertionRaw struct {
	Contains  bool `json:"contains"`
	Count     int  `json:"count"`
	TextBytes int  `json:"text_bytes"`
}

func assertionBaseResult(session Session, target Target, assertion, selector, ref, finalURL, title string, negated bool) AssertionResult {
	return AssertionResult{
		Session:   session.Name,
		TargetID:  target.ID,
		Assertion: assertion,
		Negated:   negated,
		Selector:  normalizeSelectorHint(selector),
		Ref:       RedactString(ref),
		URL:       RedactURL(finalURL),
		Title:     RedactString(title),
	}
}

func assertionFailure(result AssertionResult) error {
	if result.Pass {
		return nil
	}
	message := fmt.Sprintf("%s assertion failed.", result.Assertion)
	result.Message = message
	return &AssertionError{
		Base: NewError(
			assertionFailureCode,
			message,
			"Inspect the returned data, rerun browser page ax/snapshot if the page changed, then adjust the assertion or wait conditions.",
			412,
		),
		Result: result,
	}
}

func assertionContainsExpected(raw string) AssertionExpected {
	return AssertionExpected{
		ContainsPreview: TruncateBytes(RedactString(raw), 500),
		ContainsBytes:   len(raw),
	}
}

func applyNegation(value, negated bool) bool {
	if negated {
		return !value
	}
	return value
}

func normalizeCountAssertionOptions(opts AssertionOptions) AssertionOptions {
	if opts.Equals < -1 {
		opts.Equals = -1
	}
	if opts.Min < -1 {
		opts.Min = -1
	}
	if opts.Max < -1 {
		opts.Max = -1
	}
	return opts
}

func validateCountAssertionOptions(opts AssertionOptions) error {
	if strings.TrimSpace(opts.Selector) == "" {
		return invalidArgs("--selector is required", "Pass the CSS selector whose element count should be asserted.")
	}
	if strings.TrimSpace(opts.Ref) != "" {
		return invalidArgs("--ref is not supported for count assertions", "Use --selector for count assertions.")
	}
	bounds := 0
	if opts.Equals >= 0 {
		bounds++
	}
	if opts.Min >= 0 {
		bounds++
	}
	if opts.Max >= 0 {
		bounds++
	}
	if bounds == 0 {
		return invalidArgs("--equals, --min, or --max is required", "Pass at least one count bound.")
	}
	if opts.Equals >= 0 && (opts.Min >= 0 || opts.Max >= 0) {
		return invalidArgs("--equals cannot be combined with --min or --max", "Use either exact count matching or min/max bounds.")
	}
	if opts.Min >= 0 && opts.Max >= 0 && opts.Min > opts.Max {
		return invalidArgs("--min must be less than or equal to --max", "Use a valid inclusive count range.")
	}
	return nil
}

func countAssertionPass(count int, opts AssertionOptions) bool {
	if opts.Equals >= 0 {
		return count == opts.Equals
	}
	if opts.Min >= 0 && count < opts.Min {
		return false
	}
	if opts.Max >= 0 && count > opts.Max {
		return false
	}
	return true
}

func countAssertionExpected(opts AssertionOptions) AssertionExpected {
	var expected AssertionExpected
	if opts.Equals >= 0 {
		value := opts.Equals
		expected.Equals = &value
	}
	if opts.Min >= 0 {
		value := opts.Min
		expected.Min = &value
	}
	if opts.Max >= 0 {
		value := opts.Max
		expected.Max = &value
	}
	return expected
}

func visibleAssertionExpression(selector string) string {
	return `(function () {
  const selector = ` + strconv.Quote(selector) + `;
  const nodes = Array.from(document.querySelectorAll(selector));
  const isVisible = (el) => {
    if (!el) return false;
    if (el.hidden || String(el.getAttribute("aria-hidden") || "").toLowerCase() === "true") return false;
    const style = window.getComputedStyle ? window.getComputedStyle(el) : null;
    if (style && (style.display === "none" || style.visibility === "hidden" || Number(style.opacity || 1) === 0)) return false;
    const rects = el.getClientRects ? el.getClientRects() : [];
    for (const rect of Array.from(rects)) {
      if (rect.width > 0 && rect.height > 0) return true;
    }
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : null;
    return Boolean(rect && rect.width > 0 && rect.height > 0);
  };
  return {visible: nodes.some(isVisible), count: nodes.length};
})()`
}

func textAssertionExpression(selector, contains string) string {
	return `(function () {
  const selector = ` + strconv.Quote(strings.TrimSpace(selector)) + `;
  const contains = ` + strconv.Quote(contains) + `;
  const nodes = selector ? Array.from(document.querySelectorAll(selector)) : [document.body || document.documentElement];
  const text = nodes.map(el => String((el && (el.innerText || el.textContent)) || "")).join("\n");
  return {contains: text.includes(contains), count: nodes.length, text_bytes: new Blob([text]).size};
})()`
}

func countAssertionExpression(selector string) string {
	return `(function () {
  const selector = ` + strconv.Quote(strings.TrimSpace(selector)) + `;
  return document.querySelectorAll(selector).length;
})()`
}
