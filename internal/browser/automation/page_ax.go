package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const axSourceDOMARIA = "dom_aria_fallback"

var refFilePartPattern = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

type AXOptions struct {
	PageOptions
	Limit         int
	IncludeHidden bool
	Pierce        bool
}

type AXBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type AXNode struct {
	Index       int       `json:"index"`
	Ref         string    `json:"ref"`
	Role        string    `json:"role,omitempty"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Title       string    `json:"title,omitempty"`
	Disabled    bool      `json:"disabled,omitempty"`
	Checked     bool      `json:"checked,omitempty"`
	Expanded    bool      `json:"expanded,omitempty"`
	Selected    bool      `json:"selected,omitempty"`
	Pressed     bool      `json:"pressed,omitempty"`
	Level       int       `json:"level,omitempty"`
	Bounds      *AXBounds `json:"bounds,omitempty"`
	FrameID     string    `json:"frame_id,omitempty"`
	Selector    string    `json:"selector_hint,omitempty"`
	Tag         string    `json:"tag,omitempty"`
	InputType   string    `json:"input_type,omitempty"`
	Hidden      bool      `json:"hidden,omitempty"`
	Source      string    `json:"source"`
}

type AXResult struct {
	Session       string    `json:"session"`
	TargetID      string    `json:"target_id"`
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	Limit         int       `json:"limit"`
	IncludeHidden bool      `json:"include_hidden"`
	Pierce        bool      `json:"pierce,omitempty"`
	Count         int       `json:"count"`
	Source        string    `json:"source"`
	RefPath       string    `json:"ref_path,omitempty"`
	Nodes         []AXNode  `json:"nodes"`
	GeneratedAt   time.Time `json:"generated_at"`
}

type AXRefStore struct {
	Session     string       `json:"session"`
	TargetID    string       `json:"target_id"`
	URL         string       `json:"url,omitempty"`
	Title       string       `json:"title,omitempty"`
	Source      string       `json:"source"`
	GeneratedAt time.Time    `json:"generated_at"`
	Refs        []AXRefEntry `json:"refs"`
}

type AXRefEntry struct {
	Ref       string    `json:"ref"`
	Selector  string    `json:"selector_hint"`
	Role      string    `json:"role,omitempty"`
	Name      string    `json:"name,omitempty"`
	FrameID   string    `json:"frame_id,omitempty"`
	TargetID  string    `json:"target_id,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func (m *Manager) AX(ctx context.Context, opts AXOptions) (AXResult, error) {
	opts = normalizeAXOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return AXResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw struct {
		Count int      `json:"count"`
		Nodes []AXNode `json:"nodes"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(axExpression(opts), &raw, chromedp.EvalAsValue),
	); err != nil {
		return AXResult{}, mapPageError(err, "automation_failed")
	}
	now := m.now()
	nodes := sanitizeAXNodes(raw.Nodes, target.ID)
	refs := axRefsFromNodes(nodes, target.ID, now)
	refPath, err := m.saveAXRefs(session, target, finalURL, title, refs, now)
	if err != nil {
		return AXResult{}, err
	}
	return AXResult{
		Session:       session.Name,
		TargetID:      target.ID,
		URL:           RedactURL(finalURL),
		Title:         RedactString(title),
		Limit:         opts.Limit,
		IncludeHidden: opts.IncludeHidden,
		Pierce:        opts.Pierce,
		Count:         raw.Count,
		Source:        axSourceDOMARIA,
		RefPath:       refPath,
		Nodes:         nodes,
		GeneratedAt:   now,
	}, nil
}

func normalizeAXOptions(opts AXOptions) AXOptions {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 500 {
		opts.Limit = 500
	}
	return opts
}

func sanitizeAXNodes(raw []AXNode, targetID string) []AXNode {
	out := make([]AXNode, len(raw))
	for i, node := range raw {
		node.Index = i
		node.Role = strings.ToLower(TruncateBytes(RedactString(node.Role), 100))
		node.Name = TruncateBytes(RedactString(node.Name), 500)
		node.Description = TruncateBytes(RedactString(node.Description), 500)
		node.Title = TruncateBytes(RedactString(node.Title), 500)
		node.FrameID = TruncateBytes(RedactString(node.FrameID), 120)
		node.Selector = normalizeSelectorHint(node.Selector)
		node.Tag = strings.ToLower(TruncateBytes(RedactString(node.Tag), 80))
		node.InputType = strings.ToLower(TruncateBytes(RedactString(node.InputType), 80))
		node.Source = axSourceDOMARIA
		node.Ref = StableElementRef(AXRefEntry{
			Selector: node.Selector,
			Role:     node.Role,
			Name:     node.Name,
			FrameID:  node.FrameID,
			TargetID: targetID,
		}, i)
		out[i] = node
	}
	return out
}

func StableElementRef(entry AXRefEntry, index int) string {
	role := strings.ToLower(strings.TrimSpace(RedactString(entry.Role)))
	name := TruncateBytes(RedactString(entry.Name), 200)
	selector := normalizeSelectorHint(entry.Selector)
	frameID := TruncateBytes(RedactString(entry.FrameID), 120)
	targetID := TruncateBytes(RedactString(entry.TargetID), 120)
	sum := sha256.Sum256([]byte(strings.Join([]string{
		role,
		name,
		selector,
		frameID,
		targetID,
		strconv.Itoa(index),
	}, "\x00")))
	return "axref-" + strconv.Itoa(index) + "-" + hex.EncodeToString(sum[:])[:12]
}

func axRefsFromNodes(nodes []AXNode, targetID string, now time.Time) []AXRefEntry {
	refs := make([]AXRefEntry, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.Ref) == "" || strings.TrimSpace(node.Selector) == "" {
			continue
		}
		refs = append(refs, AXRefEntry{
			Ref:       node.Ref,
			Selector:  node.Selector,
			Role:      node.Role,
			Name:      node.Name,
			FrameID:   node.FrameID,
			TargetID:  targetID,
			Source:    node.Source,
			CreatedAt: now,
		})
	}
	return refs
}

func (m *Manager) saveAXRefs(session Session, target Target, finalURL, title string, refs []AXRefEntry, now time.Time) (string, error) {
	if err := m.ensureStore(); err != nil {
		return "", err
	}
	path, err := m.Store.AXRefPath(session.Name, target.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", NewError("artifact_write_failed", err.Error(), "Check permissions for browser ref artifacts.", 500)
	}
	store := AXRefStore{
		Session:     session.Name,
		TargetID:    target.ID,
		URL:         RedactURL(finalURL),
		Title:       RedactString(title),
		Source:      axSourceDOMARIA,
		GeneratedAt: now,
		Refs:        refs,
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return "", NewError("automation_failed", err.Error(), "Accessibility refs could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", NewError("artifact_write_failed", err.Error(), "Accessibility refs could not be written.", 500)
	}
	return path, nil
}

func (m *Manager) ResolveAXRef(session Session, target Target, ref string) (AXRefEntry, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return AXRefEntry{}, invalidArgs("--ref is required", "Run browser page ax --json and pass a returned ref.")
	}
	if err := m.ensureStore(); err != nil {
		return AXRefEntry{}, err
	}
	path, err := m.Store.AXRefPath(session.Name, target.ID)
	if err != nil {
		return AXRefEntry{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return AXRefEntry{}, NewError("ref_not_found", "Accessibility refs were not found for this page target.", "Run browser page ax --json again, then retry the action with a fresh ref.", 404)
	}
	var store AXRefStore
	if err := json.Unmarshal(b, &store); err != nil {
		return AXRefEntry{}, NewError("automation_failed", err.Error(), "Accessibility ref artifact is not valid JSON. Run browser page ax --json again.", 500)
	}
	for _, entry := range store.Refs {
		if entry.Ref == ref && strings.TrimSpace(entry.Selector) != "" {
			return entry, nil
		}
	}
	return AXRefEntry{}, NewError("ref_not_found", "Accessibility ref is not present for this page target.", "Refs can expire after navigation or DOM changes. Run browser page ax --json again.", 404)
}

func (s *Store) AXRefsDir(sessionName string) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	return filepath.Join(s.RootDir, "refs", sessionName), nil
}

func (s *Store) AXRefPath(sessionName, targetID string) (string, error) {
	dir, err := s.AXRefsDir(sessionName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, safeRefFilePart(targetID)+".json"), nil
}

func safeRefFilePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "target"
	}
	raw = refFilePartPattern.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "._-")
	if raw == "" {
		raw = "target"
	}
	if len(raw) > 120 {
		raw = raw[:120]
	}
	return raw
}

func axExpression(opts AXOptions) string {
	return `(function () {
  const limit = ` + strconv.Itoa(opts.Limit) + `;
  const includeHidden = ` + strconv.FormatBool(opts.IncludeHidden) + `;
  const pierce = ` + strconv.FormatBool(opts.Pierce) + `;
  const selector = [
    "h1","h2","h3","h4","h5","h6",
    "a[href]","button","input","select","textarea","label","form",
    "table","ul","ol","[role]","[contenteditable='true']","summary","details"
  ].join(",");
  const sensitiveNamePattern = /(token|secret|password|passwd|pwd|cookie|auth|authorization|credential|jwt|saml|session|access[_-]?token|refresh[_-]?token|id[_-]?token|api[_-]?key)/i;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
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
    if (el.labels) {
      for (const label of Array.from(el.labels)) labels.push(textOf(label, 500));
    }
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
  const descriptionFor = (el) => {
    if (isSensitiveElement(el)) return "";
    const ariaDescription = safeAttr(el, "aria-description", 500);
    const describedBy = attr(el, "aria-describedby");
    const parts = [];
    if (ariaDescription) parts.push(ariaDescription);
    if (describedBy && !sensitiveNamePattern.test(describedBy)) {
      for (const part of describedBy.split(/\s+/)) {
        if (sensitiveNamePattern.test(part)) continue;
        const node = document.getElementById(part);
        if (node) parts.push(textOf(node, 500));
      }
    }
    return parts.filter(Boolean).join(" ").trim().slice(0, 500);
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
      if (type === "range") return "slider";
      return "textbox";
    }
    if (tag === "textarea") return "textbox";
    if (tag === "select") return "combobox";
    if (tag === "form") return "form";
    if (tag === "table") return "table";
    if (tag === "ul" || tag === "ol") return "list";
    if (tag === "summary") return "button";
    if (tag === "details") return "group";
    if (tag === "label") return "label";
    return "";
  };
  const nameFor = (el, tag, role) => {
    if (isSensitiveElement(el)) return labelFor(el);
    const label = labelFor(el);
    const aria = safeAttr(el, "aria-label", 500);
    const title = safeAttr(el, "title", 500);
    const placeholder = safeAttr(el, "placeholder", 500);
    const alt = safeAttr(el, "alt", 500);
    if (aria) return aria;
    if (label) return label;
    if (alt) return alt;
    if (placeholder && (tag === "input" || tag === "textarea")) return placeholder;
    if (title) return title;
    if (tag === "table") {
      const caption = el.querySelector("caption");
      if (caption) return textOf(caption, 500);
    }
    if (role === "button" || role === "link" || tag.match(/^h[1-6]$/) || tag === "button" || tag === "a" || tag === "label" || tag === "summary") {
      return textOf(el, 500);
    }
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
  const stateBool = (el, attrName, propName) => {
    const raw = attr(el, attrName).toLowerCase();
    if (raw === "true") return true;
    if (raw === "false") return false;
    return Boolean(propName && el[propName]);
  };
  const nodes = Array.from(new Set(querySelectorAllPierce(document, selector, pierce, 10000)));
  const output = [];
  let count = 0;
  for (const el of nodes) {
    const hidden = isHidden(el);
    if (hidden && !includeHidden) continue;
    const tag = String(el.tagName || "").toLowerCase();
    const role = roleFor(el, tag);
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : null;
    const title = isSensitiveElement(el) ? "" : safeAttr(el, "title", 500);
    const item = {
      index: count,
      ref: "",
      role,
      name: nameFor(el, tag, role),
      description: descriptionFor(el),
      title,
      disabled: Boolean(el.disabled) || attr(el, "aria-disabled").toLowerCase() === "true",
      checked: stateBool(el, "aria-checked", "checked"),
      expanded: stateBool(el, "aria-expanded", ""),
      selected: stateBool(el, "aria-selected", "selected"),
      pressed: stateBool(el, "aria-pressed", ""),
      level: tag.match(/^h[1-6]$/) ? Number(tag.slice(1)) : Number(attr(el, "aria-level") || 0),
      bounds: rect ? {x: rect.x, y: rect.y, width: rect.width, height: rect.height} : null,
      frame_id: "",
      selector_hint: selectorFor(el),
      tag,
      input_type: tag === "input" ? (attr(el, "type").toLowerCase() || "text") : "",
      hidden,
      source: "dom_aria_fallback"
    };
    if (item.role || item.name || item.description || item.title || ["form","table","ul","ol","label","details"].includes(tag)) {
      count++;
      if (output.length < limit) output.push(item);
    }
  }
  return {count, nodes: output};
})()
` + shadowTraversalExpression()
}
