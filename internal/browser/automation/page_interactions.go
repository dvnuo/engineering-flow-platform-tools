package automation

import (
	"context"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type SelectOptions struct {
	PageOptions
	Selector string
	Ref      string
	Value    string
	Label    string
	Index    int
}

type CheckOptions struct {
	PageOptions
	Selector string
	Ref      string
	Checked  bool
}

type PressOptions struct {
	PageOptions
	Selector string
	Ref      string
	Key      string
}

type selectActionRawResult struct {
	SelectedCount int `json:"selected_count"`
}

type checkActionRawResult struct {
	Checked bool `json:"checked"`
}

func (m *Manager) Select(ctx context.Context, opts SelectOptions) (PageActionResult, error) {
	mode, err := validateSelectOptions(opts)
	if err != nil {
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
	var raw selectActionRawResult
	if err := chromedp.Run(pageCtx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Evaluate(selectActionExpression(selector, opts, mode), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return PageActionResult{}, mapPageError(err, "automation_failed")
	}
	result := pageActionResult(session, target, "select", selector, ref, finalURL, title)
	result.SelectionMode = mode
	result.SelectedCount = raw.SelectedCount
	return result, nil
}

func (m *Manager) Check(ctx context.Context, opts CheckOptions) (PageActionResult, error) {
	if err := validateActionTarget(opts.Selector, opts.Ref, "page.check"); err != nil {
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
	var raw checkActionRawResult
	if err := chromedp.Run(pageCtx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Evaluate(checkActionExpression(selector, opts.Checked), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return PageActionResult{}, mapPageError(err, "automation_failed")
	}
	result := pageActionResult(session, target, checkedActionName(opts.Checked), selector, ref, finalURL, title)
	result.Checked = &raw.Checked
	return result, nil
}

func (m *Manager) Press(ctx context.Context, opts PressOptions) (PageActionResult, error) {
	key, err := NormalizePressKey(opts.Key)
	if err != nil {
		return PageActionResult{}, err
	}
	if err := validateOptionalActionTarget(opts.Selector, opts.Ref, "page.press"); err != nil {
		return PageActionResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return PageActionResult{}, err
	}
	defer cancel()
	selector, ref, err := m.resolveOptionalActionSelector(session, target, opts.Selector, opts.Ref)
	if err != nil {
		return PageActionResult{}, err
	}

	var actions []chromedp.Action
	if selector != "" {
		actions = append(actions,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Focus(selector, chromedp.ByQuery),
		)
	}
	var finalURL, title string
	actions = append(actions,
		chromedp.KeyEvent(key),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	)
	if err := chromedp.Run(pageCtx, actions...); err != nil {
		return PageActionResult{}, mapPageError(err, "automation_failed")
	}
	result := pageActionResult(session, target, "press", selector, ref, finalURL, title)
	result.Key = RedactString(opts.Key)
	return result, nil
}

func validateActionTarget(selector, ref, command string) error {
	selector = strings.TrimSpace(selector)
	ref = strings.TrimSpace(ref)
	if selector == "" && ref == "" {
		return invalidArgs("--selector or --ref is required", "Run browser schema "+command+" --json.")
	}
	if selector != "" && ref != "" {
		return invalidArgs("pass only one of --selector or --ref", "Use --selector for a CSS selector or --ref from browser page ax, not both.")
	}
	return nil
}

func validateOptionalActionTarget(selector, ref, command string) error {
	selector = strings.TrimSpace(selector)
	ref = strings.TrimSpace(ref)
	if selector != "" && ref != "" {
		return invalidArgs("pass only one of --selector or --ref", "Use --selector for a CSS selector or --ref from browser page ax, not both.")
	}
	return nil
}

func validateSelectOptions(opts SelectOptions) (string, error) {
	if err := validateActionTarget(opts.Selector, opts.Ref, "page.select"); err != nil {
		return "", err
	}
	modes := 0
	mode := ""
	if strings.TrimSpace(opts.Value) != "" {
		modes++
		mode = "value"
	}
	if strings.TrimSpace(opts.Label) != "" {
		modes++
		mode = "label"
	}
	if opts.Index >= 0 {
		modes++
		mode = "index"
	}
	if modes == 0 {
		return "", invalidArgs("--value, --label, or --index is required", "Pass exactly one selection target.")
	}
	if modes > 1 {
		return "", invalidArgs("pass only one of --value, --label, or --index", "Selection output reports only the selection mode and count.")
	}
	return mode, nil
}

func (m *Manager) resolveActionSelector(session Session, target Target, selector, ref string) (string, string, error) {
	selector = strings.TrimSpace(selector)
	ref = strings.TrimSpace(ref)
	if selector != "" {
		return selector, "", nil
	}
	entry, err := m.ResolveAXRef(session, target, ref)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(entry.FrameID) != "" {
		return "", "", NewError("frame_action_unsupported", "Ref points to a frame-specific node.", "This command currently supports refs in the selected page execution context. Run browser frame snapshot for frame reads.", 400)
	}
	return entry.Selector, entry.Ref, nil
}

func (m *Manager) resolveOptionalActionSelector(session Session, target Target, selector, ref string) (string, string, error) {
	selector = strings.TrimSpace(selector)
	ref = strings.TrimSpace(ref)
	if selector != "" {
		return selector, "", nil
	}
	if ref == "" {
		return "", "", nil
	}
	return m.resolveActionSelector(session, target, "", ref)
}

func NormalizePressKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", invalidArgs("--key is required", "Pass a key such as Enter, Tab, Escape, ArrowDown, or a single printable character.")
	}
	if len(key) > 80 {
		return "", invalidArgs("--key is too long", "Pass a key name or a short printable key sequence.")
	}
	switch strings.ToLower(key) {
	case "enter", "return":
		return kb.Enter, nil
	case "tab":
		return kb.Tab, nil
	case "escape", "esc":
		return kb.Escape, nil
	case "backspace":
		return kb.Backspace, nil
	case "delete":
		return kb.Delete, nil
	case "arrowdown", "down":
		return kb.ArrowDown, nil
	case "arrowleft", "left":
		return kb.ArrowLeft, nil
	case "arrowright", "right":
		return kb.ArrowRight, nil
	case "arrowup", "up":
		return kb.ArrowUp, nil
	case "home":
		return kb.Home, nil
	case "end":
		return kb.End, nil
	case "pagedown":
		return kb.PageDown, nil
	case "pageup":
		return kb.PageUp, nil
	default:
		return key, nil
	}
}

func checkedActionName(checked bool) string {
	if checked {
		return "check"
	}
	return "uncheck"
}

func selectActionExpression(selector string, opts SelectOptions, mode string) string {
	return `(function () {
  const selector = ` + strconv.Quote(selector) + `;
  const mode = ` + strconv.Quote(mode) + `;
  const value = ` + strconv.Quote(opts.Value) + `;
  const label = ` + strconv.Quote(opts.Label) + `;
  const index = ` + strconv.Itoa(opts.Index) + `;
  const el = document.querySelector(selector);
  if (!el) throw new Error("selector_not_found");
  if (String(el.tagName || "").toLowerCase() !== "select") throw new Error("element_is_not_select");
  const options = Array.from(el.options || []);
  if (mode === "value") {
    let matched = false;
    for (const option of options) {
      const ok = String(option.value) === value;
      option.selected = ok;
      matched = matched || ok;
      if (ok && !el.multiple) break;
    }
    if (!matched) throw new Error("select_value_not_found");
  } else if (mode === "label") {
    let matched = false;
    for (const option of options) {
      const text = String(option.label || option.text || "").trim();
      const ok = text === label;
      option.selected = ok;
      matched = matched || ok;
      if (ok && !el.multiple) break;
    }
    if (!matched) throw new Error("select_label_not_found");
  } else if (mode === "index") {
    if (index < 0 || index >= options.length) throw new Error("select_index_not_found");
    if (!el.multiple) {
      el.selectedIndex = index;
    } else {
      options[index].selected = true;
    }
  }
  el.dispatchEvent(new Event("input", {bubbles: true}));
  el.dispatchEvent(new Event("change", {bubbles: true}));
  return {selected_count: options.filter(option => option.selected).length};
})()`
}

func checkActionExpression(selector string, checked bool) string {
	return `(function () {
  const selector = ` + strconv.Quote(selector) + `;
  const desired = ` + strconv.FormatBool(checked) + `;
  const el = document.querySelector(selector);
  if (!el) throw new Error("selector_not_found");
  const tag = String(el.tagName || "").toLowerCase();
  const type = String(el.getAttribute("type") || "").toLowerCase();
  const role = String(el.getAttribute("role") || "").toLowerCase();
  if (tag === "input" && (type === "checkbox" || type === "radio")) {
    el.checked = desired;
    el.dispatchEvent(new Event("input", {bubbles: true}));
    el.dispatchEvent(new Event("change", {bubbles: true}));
    return {checked: Boolean(el.checked)};
  }
  if (role === "checkbox" || role === "switch" || role === "menuitemcheckbox") {
    const current = String(el.getAttribute("aria-checked") || "false").toLowerCase() === "true";
    if (current !== desired) el.click();
    el.setAttribute("aria-checked", desired ? "true" : "false");
    el.dispatchEvent(new Event("change", {bubbles: true}));
    return {checked: desired};
  }
  throw new Error("element_is_not_checkable");
})()`
}
