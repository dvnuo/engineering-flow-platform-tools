package automation

import (
	"context"
	"strconv"
	"strings"
	"time"

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
	Count    int                `json:"count"`
	Limit    int                `json:"limit"`
	URL      string             `json:"url"`
	Title    string             `json:"title"`
	Elements []ExtractedElement `json:"elements"`
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
		chromedp.Evaluate(extractExpression(opts.Selector, opts.Limit, opts.IncludeHTML), &raw, chromedp.EvalAsValue),
	); err != nil {
		return ExtractResult{}, mapPageError(err, "automation_failed")
	}
	return ExtractResult{
		Session:  session.Name,
		TargetID: target.ID,
		Selector: opts.Selector,
		Count:    raw.Count,
		Limit:    opts.Limit,
		URL:      RedactURL(finalURL),
		Title:    RedactString(title),
		Elements: sanitizeExtractedElements(raw.Elements, opts.IncludeHTML, opts.MaxHTMLBytes),
	}, nil
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

func extractExpression(selector string, limit int, includeHTML bool) string {
	return `(function () {
  const selector = ` + strconv.Quote(selector) + `;
  const limit = ` + strconv.Itoa(limit) + `;
  const includeHTML = ` + strconv.FormatBool(includeHTML) + `;
  const nodes = Array.from(document.querySelectorAll(selector));
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
})()`
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
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "deadline") || strings.Contains(msg, "timeout") {
		return NewError("timeout", err.Error(), "Increase --timeout or check whether the page is responsive.", 408)
	}
	return NewError(code, err.Error(), "Check the target tab and selector, then retry.", 500)
}
