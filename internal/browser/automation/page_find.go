package automation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func mustMarshalJSONForEval(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(b)
}

type ElementLocator struct {
	Selector    string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Role        string `json:"role,omitempty" yaml:"role,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Text        string `json:"text,omitempty" yaml:"text,omitempty"`
	Label       string `json:"label,omitempty" yaml:"label,omitempty"`
	Placeholder string `json:"placeholder,omitempty" yaml:"placeholder,omitempty"`
	NearText    string `json:"near_text,omitempty" yaml:"near_text,omitempty"`
	Nth         int    `json:"nth,omitempty" yaml:"nth,omitempty"`
}

type PageFindOptions struct {
	PageOptions
	Locator       ElementLocator
	Locators      []ElementLocator
	Limit         int
	IncludeHidden bool
}

type PageFindMatch struct {
	Index       int              `json:"index"`
	Ref         string           `json:"ref"`
	Selector    string           `json:"selector_hint"`
	Role        string           `json:"role,omitempty"`
	Name        string           `json:"name,omitempty"`
	Text        string           `json:"text,omitempty"`
	Label       string           `json:"label,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
	Tag         string           `json:"tag,omitempty"`
	InputType   string           `json:"input_type,omitempty"`
	Hidden      bool             `json:"hidden,omitempty"`
	Locators    []ElementLocator `json:"locators,omitempty"`
}

type PageFindResult struct {
	Session       string           `json:"session"`
	TargetID      string           `json:"target_id"`
	URL           string           `json:"url"`
	Title         string           `json:"title"`
	Limit         int              `json:"limit"`
	IncludeHidden bool             `json:"include_hidden,omitempty"`
	Criteria      []ElementLocator `json:"criteria"`
	Count         int              `json:"count"`
	RefPath       string           `json:"ref_path,omitempty"`
	Matches       []PageFindMatch  `json:"matches"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

func (m *Manager) Find(ctx context.Context, opts PageFindOptions) (PageFindResult, error) {
	opts = normalizePageFindOptions(opts)
	if len(opts.Locators) == 0 {
		return PageFindResult{}, invalidArgs("at least one locator criterion is required", "Pass --selector, --role, --name, --text, --label, --placeholder, or --near-text.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return PageFindResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count   int             `json:"count"`
		Matches []PageFindMatch `json:"matches"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(pageFindExpression(opts), &raw, chromedp.EvalAsValue),
	); err != nil {
		return PageFindResult{}, mapPageError(err, "automation_failed")
	}
	now := m.now()
	matches := sanitizePageFindMatches(raw.Matches, target.ID)
	refs := make([]AXRefEntry, 0, len(matches))
	for _, match := range matches {
		if strings.TrimSpace(match.Ref) == "" || strings.TrimSpace(match.Selector) == "" {
			continue
		}
		refs = append(refs, AXRefEntry{
			Ref:       match.Ref,
			Selector:  match.Selector,
			Role:      match.Role,
			Name:      match.Name,
			TargetID:  target.ID,
			Source:    "semantic_find",
			CreatedAt: now,
		})
	}
	refPath, err := m.saveAXRefs(session, target, finalURL, title, refs, now)
	if err != nil {
		return PageFindResult{}, err
	}
	return PageFindResult{
		Session:       session.Name,
		TargetID:      target.ID,
		URL:           RedactURL(finalURL),
		Title:         RedactString(title),
		Limit:         opts.Limit,
		IncludeHidden: opts.IncludeHidden,
		Criteria:      sanitizeElementLocators(opts.Locators),
		Count:         raw.Count,
		RefPath:       refPath,
		Matches:       matches,
		GeneratedAt:   now,
	}, nil
}

func (m *Manager) ResolveLocatorSelector(ctx context.Context, page PageOptions, locators []ElementLocator) (string, error) {
	result, err := m.Find(ctx, PageFindOptions{PageOptions: page, Locators: locators, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(result.Matches) == 0 || strings.TrimSpace(result.Matches[0].Selector) == "" {
		return "", NewError("selector_not_found", "No element matched the supplied semantic locator.", "Run browser page find --json to inspect candidate locators.", 404)
	}
	return result.Matches[0].Selector, nil
}

func normalizePageFindOptions(opts PageFindOptions) PageFindOptions {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if hasElementLocator(opts.Locator) {
		opts.Locators = append([]ElementLocator{opts.Locator}, opts.Locators...)
	}
	clean := make([]ElementLocator, 0, len(opts.Locators))
	for _, locator := range opts.Locators {
		locator = normalizeElementLocator(locator)
		if hasElementLocator(locator) {
			clean = append(clean, locator)
		}
	}
	opts.Locators = clean
	return opts
}

func normalizeElementLocator(locator ElementLocator) ElementLocator {
	locator.Selector = strings.TrimSpace(locator.Selector)
	locator.Role = strings.ToLower(strings.TrimSpace(locator.Role))
	locator.Name = strings.TrimSpace(locator.Name)
	locator.Text = strings.TrimSpace(locator.Text)
	locator.Label = strings.TrimSpace(locator.Label)
	locator.Placeholder = strings.TrimSpace(locator.Placeholder)
	locator.NearText = strings.TrimSpace(locator.NearText)
	if locator.Nth < 0 {
		locator.Nth = 0
	}
	return locator
}

func hasElementLocator(locator ElementLocator) bool {
	locator = normalizeElementLocator(locator)
	return locator.Selector != "" ||
		locator.Role != "" ||
		locator.Name != "" ||
		locator.Text != "" ||
		locator.Label != "" ||
		locator.Placeholder != "" ||
		locator.NearText != "" ||
		locator.Nth > 0
}

func sanitizeElementLocators(raw []ElementLocator) []ElementLocator {
	out := make([]ElementLocator, 0, len(raw))
	for _, locator := range raw {
		locator = normalizeElementLocator(locator)
		if !hasElementLocator(locator) {
			continue
		}
		locator.Selector = normalizeSelectorHint(locator.Selector)
		locator.Role = strings.ToLower(TruncateBytes(RedactString(locator.Role), 80))
		locator.Name = TruncateBytes(RedactString(locator.Name), 500)
		locator.Text = TruncateBytes(RedactString(locator.Text), 500)
		locator.Label = TruncateBytes(RedactString(locator.Label), 500)
		locator.Placeholder = TruncateBytes(RedactString(locator.Placeholder), 500)
		locator.NearText = TruncateBytes(RedactString(locator.NearText), 500)
		out = append(out, locator)
	}
	return out
}

func sanitizePageFindMatches(raw []PageFindMatch, targetID string) []PageFindMatch {
	out := make([]PageFindMatch, 0, len(raw))
	for i, match := range raw {
		match.Index = i
		match.Selector = normalizeSelectorHint(match.Selector)
		match.Role = strings.ToLower(TruncateBytes(RedactString(match.Role), 80))
		match.Name = TruncateBytes(RedactString(match.Name), 500)
		match.Text = TruncateBytes(RedactString(match.Text), 500)
		match.Label = TruncateBytes(RedactString(match.Label), 500)
		match.Placeholder = TruncateBytes(RedactString(match.Placeholder), 500)
		match.Tag = strings.ToLower(TruncateBytes(RedactString(match.Tag), 80))
		match.InputType = strings.ToLower(TruncateBytes(RedactString(match.InputType), 80))
		match.Locators = sanitizeElementLocators(match.Locators)
		match.Ref = StableElementRef(AXRefEntry{
			Selector: match.Selector,
			Role:     match.Role,
			Name:     match.Name,
			TargetID: targetID,
		}, i)
		out = append(out, match)
	}
	return out
}

func pageFindExpression(opts PageFindOptions) string {
	locatorsJSON := mustMarshalJSONForEval(opts.Locators)
	return `(function () {
  const locators = ` + locatorsJSON + `;
  const limit = ` + strconv.Itoa(opts.Limit) + `;
  const includeHidden = ` + strconv.FormatBool(opts.IncludeHidden) + `;
  const sensitiveNamePattern = /(token|secret|password|passwd|pwd|cookie|auth|authorization|credential|jwt|saml|session|access[_-]?token|refresh[_-]?token|id[_-]?token|api[_-]?key)/i;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
  const norm = (value) => String(value || "").replace(/\s+/g, " ").trim().toLowerCase();
  const textOf = (el, max) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim().slice(0, max);
  const safeAttr = (el, name, max) => {
    const value = attr(el, name);
    if (!value || sensitiveNamePattern.test(name) || sensitiveNamePattern.test(value)) return "";
    return value.slice(0, max);
  };
  const isSensitiveElement = (el) => {
    const tag = String(el.tagName || "").toLowerCase();
    const type = attr(el, "type").toLowerCase();
    if (tag === "input" && type === "password") return true;
    const fields = ["id","name","autocomplete","aria-label","placeholder","title"].map(name => attr(el, name)).join(" ");
    return sensitiveNamePattern.test(fields);
  };
  const isHidden = (el) => {
    if (!el) return true;
    if (el.hidden || attr(el, "aria-hidden").toLowerCase() === "true") return true;
    const style = window.getComputedStyle ? window.getComputedStyle(el) : null;
    if (style && (style.display === "none" || style.visibility === "hidden" || style.opacity === "0")) return true;
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : {width: 1, height: 1};
    return rect.width === 0 && rect.height === 0 && textOf(el, 20) === "";
  };
  const labelFor = (el) => {
    if (isSensitiveElement(el)) return "";
    const labels = [];
    if (el.labels) for (const label of Array.from(el.labels)) labels.push(textOf(label, 500));
    const id = attr(el, "id");
    if (id && !sensitiveNamePattern.test(id)) {
      const byFor = document.querySelector("label[for='" + cssEscape(id) + "']");
      if (byFor) labels.push(textOf(byFor, 500));
    }
    const labelledBy = attr(el, "aria-labelledby");
    if (labelledBy && !sensitiveNamePattern.test(labelledBy)) {
      for (const part of labelledBy.split(/\s+/)) {
        if (sensitiveNamePattern.test(part)) continue;
        const node = document.getElementById(part);
        if (node) labels.push(textOf(node, 500));
      }
    }
    return labels.filter(Boolean).join(" ").trim().slice(0, 500);
  };
  const roleFor = (el, tag) => {
    const role = attr(el, "role");
    if (role) return role.toLowerCase();
    if (tag.match(/^h[1-6]$/)) return "heading";
    if (tag === "a") return "link";
    if (tag === "button") return "button";
    if (tag === "input") {
      const type = attr(el, "type").toLowerCase() || "text";
      if (["button","submit","reset","image"].includes(type)) return "button";
      if (type === "checkbox") return "checkbox";
      if (type === "radio") return "radio";
      return "textbox";
    }
    if (tag === "textarea") return "textbox";
    if (tag === "select") return "combobox";
    if (tag === "form") return "form";
    if (tag === "table") return "table";
    if (tag === "ul" || tag === "ol") return "list";
    return "";
  };
  const nameFor = (el, tag, role) => {
    if (isSensitiveElement(el)) return labelFor(el);
    const aria = safeAttr(el, "aria-label", 500);
    const label = labelFor(el);
    const placeholder = safeAttr(el, "placeholder", 500);
    const title = safeAttr(el, "title", 500);
    const alt = safeAttr(el, "alt", 500);
    if (aria) return aria;
    if (label) return label;
    if (alt) return alt;
    if (placeholder && (tag === "input" || tag === "textarea")) return placeholder;
    if (title) return title;
    if (role === "button" || role === "link" || tag.match(/^h[1-6]$/) || tag === "button" || tag === "a") return textOf(el, 500);
    return "";
  };
  const selectorFor = (el) => {
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && node !== document.documentElement && parts.length < 6) {
      const tag = String(node.tagName || "").toLowerCase();
      if (!tag) break;
      const id = attr(node, "id");
      if (id && !sensitiveNamePattern.test(id)) {
        parts.unshift(tag + "#" + cssEscape(id));
        break;
      }
      const name = attr(node, "name");
      if (name && !sensitiveNamePattern.test(name) && ["input","select","textarea","button"].includes(tag)) {
        parts.unshift(tag + "[name='" + cssEscape(name) + "']");
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
  const candidateSelector = [
    "a[href]","button","input","select","textarea","label","summary","[role]",
    "h1","h2","h3","h4","h5","h6","[contenteditable='true']","table","ul","ol"
  ].join(",");
  const contains = (haystack, needle) => !needle || norm(haystack).includes(norm(needle));
  const nearText = (el) => {
    const parts = [];
    let node = el;
    for (let i = 0; node && i < 4; i++) {
      parts.push(textOf(node, 2000));
      node = node.parentElement;
    }
    return parts.join(" ");
  };
  const metadata = (el) => {
    const tag = String(el.tagName || "").toLowerCase();
    const role = roleFor(el, tag);
    const label = labelFor(el);
    const placeholder = isSensitiveElement(el) ? "" : safeAttr(el, "placeholder", 500);
    const name = nameFor(el, tag, role);
    const text = isSensitiveElement(el) ? "" : textOf(el, 500);
    return {tag, role, name, text, label, placeholder, selector: selectorFor(el), hidden: isHidden(el), input_type: tag === "input" ? (attr(el, "type").toLowerCase() || "text") : ""};
  };
  const matchesLocator = (meta, el, locator) => {
    if (locator.selector && !el.matches(locator.selector)) return false;
    if (locator.role && meta.role !== String(locator.role || "").toLowerCase()) return false;
    if (locator.name && !contains(meta.name, locator.name)) return false;
    if (locator.text && !contains(meta.text, locator.text)) return false;
    if (locator.label && !contains(meta.label, locator.label)) return false;
    if (locator.placeholder && !contains(meta.placeholder, locator.placeholder)) return false;
    if (locator.near_text && !contains(nearText(el), locator.near_text)) return false;
    return true;
  };
  const selected = [];
  for (const locator of locators) {
    let candidates = [];
    if (locator.selector) {
      try { candidates = Array.from(document.querySelectorAll(locator.selector)); } catch (_) { candidates = []; }
    } else {
      candidates = Array.from(document.querySelectorAll(candidateSelector));
    }
    const found = [];
    for (const el of candidates) {
      const meta = metadata(el);
      if (meta.hidden && !includeHidden) continue;
      if (!matchesLocator(meta, el, locator)) continue;
      found.push({el, meta});
    }
    const nth = Math.max(0, Number(locator.nth || 0) - 1);
    if (locator.nth > 0) {
      if (found[nth]) selected.push(found[nth]);
    } else {
      selected.push(...found);
    }
  }
  const seen = new Set();
  const output = [];
  for (const item of selected) {
    const selector = item.meta.selector;
    if (!selector || seen.has(selector)) continue;
    seen.add(selector);
    const locs = [{selector}];
    if (item.meta.role && item.meta.name) locs.push({role: item.meta.role, name: item.meta.name});
    if (item.meta.role && item.meta.text) locs.push({role: item.meta.role, text: item.meta.text});
    if (item.meta.label) locs.push({label: item.meta.label});
    if (item.meta.placeholder) locs.push({placeholder: item.meta.placeholder});
    output.push({
      index: output.length,
      ref: "",
      selector_hint: selector,
      role: item.meta.role,
      name: item.meta.name,
      text: item.meta.text,
      label: item.meta.label,
      placeholder: item.meta.placeholder,
      tag: item.meta.tag,
      input_type: item.meta.input_type,
      hidden: item.meta.hidden,
      locators: locs
    });
    if (output.length >= limit) break;
  }
  return {count: selected.length, matches: output};
})()`
}
