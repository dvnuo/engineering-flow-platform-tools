package automation

import (
	"context"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

type OutlineOptions struct {
	PageOptions
	Limit         int
	IncludeHidden bool
}

type OutlineElement struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind,omitempty"`
	Role        string `json:"role,omitempty"`
	Name        string `json:"name,omitempty"`
	Text        string `json:"text,omitempty"`
	Label       string `json:"label,omitempty"`
	Href        string `json:"href,omitempty"`
	Tag         string `json:"tag,omitempty"`
	InputType   string `json:"input_type,omitempty"`
	Level       int    `json:"level,omitempty"`
	Selector    string `json:"selector_hint,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	RowCount    int    `json:"row_count,omitempty"`
	ColumnCount int    `json:"column_count,omitempty"`
	ItemCount   int    `json:"item_count,omitempty"`
	FieldCount  int    `json:"field_count,omitempty"`
}

type OutlineResult struct {
	Session       string           `json:"session"`
	TargetID      string           `json:"target_id"`
	URL           string           `json:"url"`
	Title         string           `json:"title"`
	Limit         int              `json:"limit"`
	IncludeHidden bool             `json:"include_hidden"`
	Count         int              `json:"count"`
	Elements      []OutlineElement `json:"elements"`
}

func (m *Manager) Outline(ctx context.Context, opts OutlineOptions) (OutlineResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return OutlineResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count    int              `json:"count"`
		Elements []OutlineElement `json:"elements"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(outlineExpression(opts.Limit, opts.IncludeHidden), &raw, chromedp.EvalAsValue),
	); err != nil {
		return OutlineResult{}, mapPageError(err, "automation_failed")
	}
	return OutlineResult{
		Session:       session.Name,
		TargetID:      target.ID,
		URL:           RedactURL(finalURL),
		Title:         RedactString(title),
		Limit:         opts.Limit,
		IncludeHidden: opts.IncludeHidden,
		Count:         raw.Count,
		Elements:      sanitizeOutlineElements(raw.Elements),
	}, nil
}

func sanitizeOutlineElements(raw []OutlineElement) []OutlineElement {
	out := make([]OutlineElement, len(raw))
	for i, el := range raw {
		el.Kind = strings.ToLower(RedactString(el.Kind))
		el.Role = strings.ToLower(RedactString(el.Role))
		el.Name = TruncateBytes(RedactString(el.Name), 500)
		el.Text = TruncateBytes(RedactString(el.Text), 1000)
		el.Label = TruncateBytes(RedactString(el.Label), 500)
		el.Href = RedactURL(el.Href)
		el.Tag = strings.ToLower(RedactString(el.Tag))
		el.InputType = strings.ToLower(RedactString(el.InputType))
		el.Selector = normalizeSelectorHint(el.Selector)
		out[i] = el
	}
	return out
}

func normalizeSelectorHint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if sensitiveSelectorHint(raw) {
		return "REDACTED_SELECTOR"
	}
	return TruncateBytes(RedactString(raw), 500)
}

func outlineExpression(limit int, includeHidden bool) string {
	if limit <= 0 {
		limit = 100
	}
	return `(function () {
  const limit = ` + strconv.Itoa(limit) + `;
  const includeHidden = ` + strconv.FormatBool(includeHidden) + `;
  const selector = [
    "h1","h2","h3","h4","h5","h6",
    "a[href]","button","input","select","textarea","label","form",
    "table","ul","ol","[role]","[contenteditable='true']","summary"
  ].join(",");
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const textOf = (el, max) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim().slice(0, max);
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
  const isHidden = (el) => {
    if (!el) return true;
    if (el.hidden || attr(el, "aria-hidden").toLowerCase() === "true") return true;
    const style = window.getComputedStyle ? window.getComputedStyle(el) : null;
    if (style && (style.display === "none" || style.visibility === "hidden" || style.opacity === "0")) return true;
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : {width: 1, height: 1};
    return rect.width === 0 && rect.height === 0 && textOf(el, 20) === "";
  };
  const labelFor = (el) => {
    const id = attr(el, "id");
    const labels = [];
    if (el.labels) {
      for (const label of Array.from(el.labels)) labels.push(textOf(label, 500));
    }
    if (id) {
      const byFor = document.querySelector("label[for='" + cssEscape(id) + "']");
      if (byFor) labels.push(textOf(byFor, 500));
    }
    const ariaLabelledBy = attr(el, "aria-labelledby");
    if (ariaLabelledBy) {
      for (const part of ariaLabelledBy.split(/\s+/)) {
        const node = document.getElementById(part);
        if (node) labels.push(textOf(node, 500));
      }
    }
    return labels.filter(Boolean).join(" ").trim().slice(0, 500);
  };
  const nameFor = (el, tag, role) => {
    const label = labelFor(el);
    const aria = attr(el, "aria-label");
    const title = attr(el, "title");
    const placeholder = attr(el, "placeholder");
    const alt = attr(el, "alt");
    if (aria) return aria.slice(0, 500);
    if (label) return label.slice(0, 500);
    if (alt) return alt.slice(0, 500);
    if (placeholder && (tag === "input" || tag === "textarea")) return placeholder.slice(0, 500);
    if (title) return title.slice(0, 500);
    if (tag === "table") {
      const caption = el.querySelector("caption");
      if (caption) return textOf(caption, 500);
    }
    if (role === "button" || role === "link" || tag.match(/^h[1-6]$/) || tag === "button" || tag === "a" || tag === "label" || tag === "summary") {
      return textOf(el, 500);
    }
    return "";
  };
  const roleFor = (el, tag) => {
    const role = attr(el, "role");
    if (role) return role.toLowerCase();
    if (tag.match(/^h[1-6]$/)) return "heading";
    if (tag === "a") return "link";
    if (tag === "button") return "button";
    if (tag === "input") {
      const type = attr(el, "type").toLowerCase() || "text";
      if (["button","submit","reset"].includes(type)) return "button";
      if (type === "checkbox") return "checkbox";
      if (type === "radio") return "radio";
      return "textbox";
    }
    if (tag === "textarea") return "textbox";
    if (tag === "select") return "combobox";
    if (tag === "form") return "form";
    if (tag === "table") return "table";
    if (tag === "ul" || tag === "ol") return "list";
    if (tag === "label") return "label";
    return "";
  };
  const kindFor = (tag, role) => {
    if (tag.match(/^h[1-6]$/)) return "heading";
    if (role === "link" || tag === "a") return "link";
    if (role === "button" || tag === "button") return "button";
    if (["input","select","textarea"].includes(tag)) return "field";
    if (tag === "form" || role === "form") return "form";
    if (tag === "table" || role === "table" || role === "grid") return "table";
    if (tag === "ul" || tag === "ol" || role === "list") return "list";
    if (tag === "label") return "label";
    return role || tag;
  };
  const selectorFor = (el) => {
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && node !== document.documentElement && parts.length < 6) {
      const tag = String(node.tagName || "").toLowerCase();
      if (!tag) break;
      const id = attr(node, "id");
      if (id) {
        parts.unshift(tag + "#" + cssEscape(id));
        break;
      }
      let nth = 1;
      let prev = node.previousElementSibling;
      while (prev) {
        if (String(prev.tagName || "").toLowerCase() === tag) nth++;
        prev = prev.previousElementSibling;
      }
      parts.unshift(tag + ":nth-of-type(" + nth + ")");
      node = node.parentElement;
    }
    return parts.join(" > ");
  };
  const nodes = Array.from(new Set(Array.from(document.querySelectorAll(selector))));
  const elements = [];
  let count = 0;
  for (const el of nodes) {
    const hidden = isHidden(el);
    if (hidden && !includeHidden) continue;
    const tag = String(el.tagName || "").toLowerCase();
    const role = roleFor(el, tag);
    const kind = kindFor(tag, role);
    const item = {
      index: count,
      kind,
      role,
      name: nameFor(el, tag, role),
      text: ["input","select","textarea","form"].includes(tag) ? "" : textOf(el, 1000),
      label: labelFor(el),
      href: tag === "a" ? String(el.href || attr(el, "href")) : "",
      tag,
      input_type: tag === "input" ? (attr(el, "type").toLowerCase() || "text") : "",
      level: tag.match(/^h[1-6]$/) ? Number(tag.slice(1)) : 0,
      selector_hint: selectorFor(el),
      hidden,
      row_count: tag === "table" ? el.querySelectorAll("tr").length : 0,
      column_count: tag === "table" ? Math.max(0, ...Array.from(el.querySelectorAll("tr")).map(row => row.children.length)) : 0,
      item_count: (tag === "ul" || tag === "ol" || role === "list") ? el.querySelectorAll("li,[role='listitem']").length : 0,
      field_count: tag === "form" ? el.querySelectorAll("input,select,textarea,button").length : 0
    };
    if (item.name || item.text || item.label || item.href || ["heading","link","button","field","form","table","list","label"].includes(kind)) {
      count++;
      if (elements.length < limit) elements.push(item);
    }
  }
  return {count, elements};
})()`
}
