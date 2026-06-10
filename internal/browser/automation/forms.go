package automation

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"
)

type FormInspectOptions struct {
	PageOptions
	Selector string
	Limit    int
}

type FormField struct {
	Index       int      `json:"index"`
	Selector    string   `json:"selector_hint"`
	Tag         string   `json:"tag"`
	Type        string   `json:"type,omitempty"`
	Name        string   `json:"name,omitempty"`
	ID          string   `json:"id,omitempty"`
	Label       string   `json:"label,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Disabled    bool     `json:"disabled,omitempty"`
	Checked     *bool    `json:"checked,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type FormInspectResult struct {
	Session    string      `json:"session"`
	TargetID   string      `json:"target_id"`
	URL        string      `json:"url"`
	Title      string      `json:"title"`
	Selector   string      `json:"selector,omitempty"`
	Limit      int         `json:"limit"`
	Count      int         `json:"count"`
	Fields     []FormField `json:"fields"`
	Limitation string      `json:"limitation"`
}

type FormFillOptions struct {
	PageOptions
	File string
}

type FormFillDefinition struct {
	Fields map[string]any `yaml:"fields"`
}

type FormFillFieldResult struct {
	Key        string `json:"key"`
	Matched    bool   `json:"matched"`
	Selector   string `json:"selector_hint,omitempty"`
	Action     string `json:"action,omitempty"`
	ValueBytes int    `json:"value_bytes,omitempty"`
}

type FormFillResult struct {
	Session    string                `json:"session"`
	TargetID   string                `json:"target_id"`
	URL        string                `json:"url"`
	Title      string                `json:"title"`
	File       string                `json:"file"`
	FieldCount int                   `json:"field_count"`
	Matched    int                   `json:"matched"`
	Fields     []FormFillFieldResult `json:"fields"`
	Limitation string                `json:"limitation"`
}

func (m *Manager) FormInspect(ctx context.Context, opts FormInspectOptions) (FormInspectResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return FormInspectResult{}, err
	}
	defer cancel()
	var finalURL, title string
	var raw struct {
		Count  int         `json:"count"`
		Fields []FormField `json:"fields"`
	}
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(formInspectExpression(opts), &raw, chromedp.EvalAsValue),
	); err != nil {
		return FormInspectResult{}, mapPageError(err, "automation_failed")
	}
	return FormInspectResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		Selector:   normalizeSelectorHint(opts.Selector),
		Limit:      opts.Limit,
		Count:      raw.Count,
		Fields:     sanitizeFormFields(raw.Fields),
		Limitation: "Form inspection returns metadata only and never returns current field values.",
	}, nil
}

func (m *Manager) FormFill(ctx context.Context, opts FormFillOptions) (FormFillResult, error) {
	def, err := loadFormFillFile(opts.File)
	if err != nil {
		return FormFillResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return FormFillResult{}, err
	}
	defer cancel()
	var finalURL, title string
	var raw []FormFillFieldResult
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(formFillExpression(def), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return FormFillResult{}, mapPageError(err, "automation_failed")
	}
	fields := sanitizeFormFillResults(raw)
	matched := 0
	for _, field := range fields {
		if field.Matched {
			matched++
		}
	}
	return FormFillResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		File:       RedactString(opts.File),
		FieldCount: len(fields),
		Matched:    matched,
		Fields:     fields,
		Limitation: "Form fill does not echo field values; output includes value byte counts and match metadata only.",
	}, nil
}

func loadFormFillFile(path string) (FormFillDefinition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return FormFillDefinition{}, invalidArgs("--file is required", "Pass a YAML form values file.")
	}
	b, err := os.ReadFile(expandHome(path))
	if err != nil {
		return FormFillDefinition{}, NewError("form_values_read_failed", err.Error(), "Check that --file points to a readable YAML file.", 400)
	}
	var def FormFillDefinition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return FormFillDefinition{}, NewError("form_values_invalid", RedactError(err.Error()), "Use fields: {name_or_selector: value}.", 400)
	}
	if len(def.Fields) == 0 {
		return FormFillDefinition{}, invalidArgs("fields is required", "Add fields: {name_or_selector: value}.")
	}
	return def, nil
}

func formInspectExpression(opts FormInspectOptions) string {
	selector, _ := json.Marshal(strings.TrimSpace(opts.Selector))
	return `(function () {
  const rootSelector = ` + string(selector) + `;
  const limit = ` + strconv.Itoa(opts.Limit) + `;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
  const textOf = (el) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim();
  const selectorFor = (el) => {
    const tag = String(el.tagName || "").toLowerCase();
    const id = attr(el, "id");
    if (id) return tag + "#" + cssEscape(id);
    const name = attr(el, "name");
    if (name) return tag + "[name=\\"" + cssEscape(name) + "\\"]";
    let nth = 1;
    let prev = el.previousElementSibling;
    while (prev) {
      if (String(prev.tagName || "").toLowerCase() === tag) nth++;
      prev = prev.previousElementSibling;
    }
    return tag + ":nth-of-type(" + nth + ")";
  };
  const labelFor = (el) => {
    const id = attr(el, "id");
    if (id) {
      const label = document.querySelector("label[for='" + cssEscape(id) + "']");
      if (label) return textOf(label);
    }
    const parent = el.closest("label");
    if (parent) return textOf(parent);
    return attr(el, "aria-label") || attr(el, "placeholder");
  };
  const root = rootSelector ? document.querySelector(rootSelector) : document;
  const nodes = root ? Array.from(root.querySelectorAll("input,textarea,select")) : [];
  return {
    count: nodes.length,
    fields: nodes.slice(0, limit).map((el, index) => {
      const tag = String(el.tagName || "").toLowerCase();
      const type = String(el.type || tag).toLowerCase();
      const checked = (type === "checkbox" || type === "radio") ? !!el.checked : undefined;
      return {
        index,
        selector_hint: selectorFor(el),
        tag,
        type,
        name: attr(el, "name"),
        id: attr(el, "id"),
        label: labelFor(el),
        placeholder: attr(el, "placeholder"),
        required: !!el.required,
        disabled: !!el.disabled,
        checked,
        options: tag === "select" ? Array.from(el.options || []).slice(0, 50).map((option) => textOf(option)) : []
      };
    })
  };
})()`
}

func formFillExpression(def FormFillDefinition) string {
	type jsField struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	fields := make([]jsField, 0, len(def.Fields))
	for key, value := range def.Fields {
		fields = append(fields, jsField{Key: key, Value: value})
	}
	b, _ := json.Marshal(fields)
	return `(function () {
  const fields = ` + string(b) + `;
  const cssEscape = (value) => {
    if (window.CSS && CSS.escape) return CSS.escape(String(value));
    return String(value).replace(/[^a-zA-Z0-9_-]/g, "\\$&");
  };
  const attr = (el, name) => String((el && el.getAttribute(name)) || "").trim();
  const textOf = (el) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim();
  const selectorFor = (el) => {
    const tag = String(el.tagName || "").toLowerCase();
    const id = attr(el, "id");
    if (id) return tag + "#" + cssEscape(id);
    const name = attr(el, "name");
    if (name) return tag + "[name=\\"" + cssEscape(name) + "\\"]";
    return tag;
  };
  const labelFor = (el) => {
    const id = attr(el, "id");
    if (id) {
      const label = document.querySelector("label[for='" + cssEscape(id) + "']");
      if (label) return textOf(label).toLowerCase();
    }
    const parent = el.closest("label");
    if (parent) return textOf(parent).toLowerCase();
    return (attr(el, "aria-label") || attr(el, "placeholder")).toLowerCase();
  };
  const all = Array.from(document.querySelectorAll("input,textarea,select"));
  const findField = (key) => {
    const raw = String(key || "").trim();
    if (!raw) return null;
    try {
      if (/^[#.\\[]|^(input|textarea|select|form)\\b/i.test(raw)) {
        const direct = document.querySelector(raw);
        if (direct) return direct;
      }
    } catch (_) {}
    const lowered = raw.toLowerCase();
    return all.find((el) =>
      attr(el, "name").toLowerCase() === lowered ||
      attr(el, "id").toLowerCase() === lowered ||
      labelFor(el) === lowered
    ) || null;
  };
  const setValue = (el, value) => {
    const tag = String(el.tagName || "").toLowerCase();
    const type = String(el.type || tag).toLowerCase();
    if (type === "checkbox" || type === "radio") {
      el.checked = !!value;
      el.dispatchEvent(new Event("change", {bubbles: true}));
      return type === "checkbox" ? "check" : "radio";
    }
    if (tag === "select") {
      el.value = String(value == null ? "" : value);
      el.dispatchEvent(new Event("change", {bubbles: true}));
      return "select";
    }
    el.focus && el.focus();
    el.value = String(value == null ? "" : value);
    el.dispatchEvent(new Event("input", {bubbles: true}));
    el.dispatchEvent(new Event("change", {bubbles: true}));
    return "type";
  };
  return fields.map((field) => {
    const el = findField(field.key);
    const value = field.value;
    if (!el) return {key: field.key, matched: false, value_bytes: String(value == null ? "" : value).length};
    const action = setValue(el, value);
    return {key: field.key, matched: true, selector_hint: selectorFor(el), action, value_bytes: String(value == null ? "" : value).length};
  });
})()`
}

func sanitizeFormFields(raw []FormField) []FormField {
	out := make([]FormField, len(raw))
	for i, field := range raw {
		field.Index = i
		field.Selector = normalizeSelectorHint(field.Selector)
		field.Tag = strings.ToLower(TruncateBytes(RedactString(field.Tag), 80))
		field.Type = strings.ToLower(TruncateBytes(RedactString(field.Type), 80))
		field.Name = TruncateBytes(RedactString(field.Name), 200)
		field.ID = TruncateBytes(RedactString(field.ID), 200)
		field.Label = TruncateBytes(RedactString(field.Label), 500)
		field.Placeholder = TruncateBytes(RedactString(field.Placeholder), 500)
		for j, option := range field.Options {
			field.Options[j] = TruncateBytes(RedactString(option), 500)
		}
		out[i] = field
	}
	return out
}

func sanitizeFormFillResults(raw []FormFillFieldResult) []FormFillFieldResult {
	out := make([]FormFillFieldResult, len(raw))
	for i, field := range raw {
		field.Key = TruncateBytes(RedactString(field.Key), 500)
		field.Selector = normalizeSelectorHint(field.Selector)
		field.Action = RedactString(field.Action)
		out[i] = field
	}
	return out
}
