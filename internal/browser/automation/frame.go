package automation

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	cdpPage "github.com/chromedp/cdproto/page"
	cdpRuntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type FrameOptions struct {
	PageOptions
}

type FrameSnapshotOptions struct {
	PageOptions
	FrameID      string
	IncludeHTML  bool
	MaxTextBytes int
	MaxHTMLBytes int
}

type FrameInfo struct {
	ID         string `json:"id"`
	ParentID   string `json:"parent_id,omitempty"`
	Name       string `json:"name,omitempty"`
	URL        string `json:"url,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	Depth      int    `json:"depth"`
	ChildCount int    `json:"child_count,omitempty"`
}

type FrameListResult struct {
	Session  string      `json:"session"`
	TargetID string      `json:"target_id"`
	Count    int         `json:"count"`
	Frames   []FrameInfo `json:"frames"`
}

type FrameSnapshotResult struct {
	Session         string `json:"session"`
	TargetID        string `json:"target_id"`
	FrameID         string `json:"frame_id"`
	URL             string `json:"url"`
	Title           string `json:"title"`
	BodyTextPreview string `json:"body_text_preview,omitempty"`
	TextLength      int    `json:"text_length"`
	HTMLPreview     string `json:"html_preview,omitempty"`
	HTMLLength      int    `json:"html_length,omitempty"`
}

type rawFrameSnapshot struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	BodyText string `json:"body_text"`
	HTML     string `json:"html"`
}

func (m *Manager) FrameList(ctx context.Context, opts FrameOptions) (FrameListResult, error) {
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return FrameListResult{}, err
	}
	defer cancel()

	var tree *cdpPage.FrameTree
	if err := chromedp.Run(pageCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		tree, err = cdpPage.GetFrameTree().Do(ctx)
		return err
	})); err != nil {
		return FrameListResult{}, mapPageError(err, "automation_failed")
	}
	frames := sanitizeFrameInfos(flattenFrameTree(tree))
	return FrameListResult{
		Session:  session.Name,
		TargetID: target.ID,
		Count:    len(frames),
		Frames:   frames,
	}, nil
}

func (m *Manager) FrameSnapshot(ctx context.Context, opts FrameSnapshotOptions) (FrameSnapshotResult, error) {
	opts = normalizeFrameSnapshotOptions(opts)
	if strings.TrimSpace(opts.FrameID) == "" {
		return FrameSnapshotResult{}, invalidArgs("--frame-id is required", "Run browser frame list --json and pass a returned frame id.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return FrameSnapshotResult{}, err
	}
	defer cancel()

	var raw rawFrameSnapshot
	if err := chromedp.Run(pageCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		if err := ensureFrameExists(ctx, opts.FrameID); err != nil {
			return err
		}
		execCtx, err := cdpPage.CreateIsolatedWorld(cdp.FrameID(opts.FrameID)).WithWorldName("efp_browser_frame_snapshot").Do(ctx)
		if err != nil {
			return err
		}
		obj, exception, err := cdpRuntime.Evaluate(frameSnapshotExpression(opts.IncludeHTML)).
			WithContextID(execCtx).
			WithReturnByValue(true).
			WithAwaitPromise(true).
			Do(ctx)
		if err != nil {
			return err
		}
		if exception != nil {
			return NewError("automation_failed", "Frame snapshot evaluation failed.", "Check frame id and whether the frame is still attached.", 500)
		}
		if obj == nil || len(obj.Value) == 0 {
			return NewError("automation_failed", "Frame snapshot returned no value.", "Check frame id and whether the frame is still attached.", 500)
		}
		return json.Unmarshal(obj.Value, &raw)
	})); err != nil {
		return FrameSnapshotResult{}, mapPageError(err, "automation_failed")
	}
	result := FrameSnapshotResult{
		Session:         session.Name,
		TargetID:        target.ID,
		FrameID:         RedactString(opts.FrameID),
		URL:             RedactURL(raw.URL),
		Title:           RedactString(raw.Title),
		BodyTextPreview: TruncateBytes(RedactString(raw.BodyText), opts.MaxTextBytes),
		TextLength:      len(raw.BodyText),
	}
	if opts.IncludeHTML {
		result.HTMLLength = len(raw.HTML)
		result.HTMLPreview = TruncateBytes(RedactString(raw.HTML), opts.MaxHTMLBytes)
	}
	return result, nil
}

func normalizeFrameSnapshotOptions(opts FrameSnapshotOptions) FrameSnapshotOptions {
	if opts.MaxTextBytes <= 0 {
		opts.MaxTextBytes = 4000
	}
	if opts.MaxHTMLBytes <= 0 {
		opts.MaxHTMLBytes = 20000
	}
	return opts
}

func ensureFrameExists(ctx context.Context, frameID string) error {
	tree, err := cdpPage.GetFrameTree().Do(ctx)
	if err != nil {
		return err
	}
	if findFrameInfo(tree, frameID) == nil {
		return NewError("frame_not_found", "Frame id was not found in the selected page target.", "Run browser frame list --json and choose an attached frame id.", 404)
	}
	return nil
}

func flattenFrameTree(tree *cdpPage.FrameTree) []FrameInfo {
	var out []FrameInfo
	var walk func(*cdpPage.FrameTree, int)
	walk = func(node *cdpPage.FrameTree, depth int) {
		if node == nil || node.Frame == nil {
			return
		}
		frame := node.Frame
		out = append(out, FrameInfo{
			ID:         string(frame.ID),
			ParentID:   string(frame.ParentID),
			Name:       frame.Name,
			URL:        frame.URL,
			MimeType:   frame.MimeType,
			Depth:      depth,
			ChildCount: len(node.ChildFrames),
		})
		for _, child := range node.ChildFrames {
			walk(child, depth+1)
		}
	}
	walk(tree, 0)
	return out
}

func findFrameInfo(tree *cdpPage.FrameTree, frameID string) *FrameInfo {
	for _, frame := range flattenFrameTree(tree) {
		if frame.ID == frameID {
			return &frame
		}
	}
	return nil
}

func sanitizeFrameInfos(raw []FrameInfo) []FrameInfo {
	out := make([]FrameInfo, len(raw))
	for i, frame := range raw {
		frame.ID = TruncateBytes(RedactString(frame.ID), 160)
		frame.ParentID = TruncateBytes(RedactString(frame.ParentID), 160)
		frame.Name = TruncateBytes(RedactString(frame.Name), 500)
		frame.URL = RedactURL(frame.URL)
		frame.MimeType = strings.ToLower(TruncateBytes(RedactString(frame.MimeType), 120))
		if frame.Depth < 0 {
			frame.Depth = 0
		}
		if frame.ChildCount < 0 {
			frame.ChildCount = 0
		}
		out[i] = frame
	}
	return out
}

func frameSnapshotExpression(includeHTML bool) string {
	html := `""`
	if includeHTML {
		html = `String((document.documentElement && document.documentElement.outerHTML) || "")`
	}
	return `(function () {
  const body = document.body || document.documentElement || null;
  return {
    url: String(location.href || ""),
    title: String(document.title || ""),
    body_text: String((body && (body.innerText || body.textContent)) || ""),
    html: ` + html + `
  };
})()`
}
