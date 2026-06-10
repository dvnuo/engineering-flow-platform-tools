package automation

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"
)

type ExtractSchemaOptions struct {
	PageOptions
	File  string
	Limit int
}

type ExtractSchemaDefinition struct {
	Fields map[string]ExtractSchemaField `yaml:"fields" json:"fields"`
}

type ExtractSchemaField struct {
	Selector string `yaml:"selector" json:"selector"`
	Attr     string `yaml:"attr,omitempty" json:"attr,omitempty"`
	Many     bool   `yaml:"many,omitempty" json:"many,omitempty"`
	Limit    int    `yaml:"limit,omitempty" json:"limit,omitempty"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

type ExtractSchemaFieldResult struct {
	Name       string   `json:"name"`
	Selector   string   `json:"selector"`
	Attr       string   `json:"attr"`
	Many       bool     `json:"many,omitempty"`
	Required   bool     `json:"required,omitempty"`
	Count      int      `json:"count"`
	Missing    bool     `json:"missing,omitempty"`
	Value      string   `json:"value,omitempty"`
	Values     []string `json:"values,omitempty"`
	ValueBytes int      `json:"value_bytes,omitempty"`
}

type ExtractSchemaResult struct {
	Session    string                     `json:"session"`
	TargetID   string                     `json:"target_id"`
	URL        string                     `json:"url"`
	Title      string                     `json:"title"`
	File       string                     `json:"file"`
	FieldCount int                        `json:"field_count"`
	Missing    []string                   `json:"missing,omitempty"`
	Fields     []ExtractSchemaFieldResult `json:"fields"`
	Limitation string                     `json:"limitation"`
}

type extractSchemaRawField struct {
	Name   string   `json:"name"`
	Count  int      `json:"count"`
	Value  string   `json:"value"`
	Values []string `json:"values"`
}

func LoadExtractSchemaFile(path string) (ExtractSchemaDefinition, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return ExtractSchemaDefinition{}, invalidArgs("--file is required", "Pass a YAML extraction schema file.")
	}
	b, err := os.ReadFile(expandHome(path))
	if err != nil {
		return ExtractSchemaDefinition{}, NewError("schema_read_failed", err.Error(), "Check that --file points to a readable YAML file.", 400)
	}
	var def ExtractSchemaDefinition
	if err := yaml.Unmarshal(b, &def); err != nil {
		return ExtractSchemaDefinition{}, NewError("schema_invalid", RedactError(err.Error()), "Use fields: {name: {selector: h1, attr: text}}.", 400)
	}
	if len(def.Fields) == 0 {
		return ExtractSchemaDefinition{}, invalidArgs("fields is required", "Add one or more field definitions.")
	}
	for name, field := range def.Fields {
		if strings.TrimSpace(name) == "" {
			return ExtractSchemaDefinition{}, invalidArgs("field name must be non-empty", "Use stable field names such as order_id or status.")
		}
		if strings.TrimSpace(field.Selector) == "" {
			return ExtractSchemaDefinition{}, invalidArgs("field selector is required", "Each field must include a CSS selector.")
		}
	}
	return def, nil
}

func (m *Manager) ExtractSchema(ctx context.Context, opts ExtractSchemaOptions) (ExtractSchemaResult, error) {
	def, err := LoadExtractSchemaFile(opts.File)
	if err != nil {
		return ExtractSchemaResult{}, err
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ExtractSchemaResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw []extractSchemaRawField
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(extractSchemaExpression(def, opts.Limit), &raw, chromedp.EvalAsValue),
	); err != nil {
		return ExtractSchemaResult{}, mapPageError(err, "automation_failed")
	}
	fields, missing := sanitizeExtractSchemaFields(raw, def)
	return ExtractSchemaResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		File:       RedactString(opts.File),
		FieldCount: len(fields),
		Missing:    missing,
		Fields:     fields,
		Limitation: "Structured extraction reads only selector-declared page text/attribute values and returns redacted/truncated strings.",
	}, nil
}

func sanitizeExtractSchemaFields(raw []extractSchemaRawField, def ExtractSchemaDefinition) ([]ExtractSchemaFieldResult, []string) {
	rawByName := map[string]extractSchemaRawField{}
	for _, item := range raw {
		rawByName[item.Name] = item
	}
	fields := make([]ExtractSchemaFieldResult, 0, len(def.Fields))
	var missing []string
	for name, spec := range def.Fields {
		attr := normalizeExtractSchemaAttr(spec.Attr)
		item := rawByName[name]
		result := ExtractSchemaFieldResult{
			Name:     RedactString(name),
			Selector: normalizeSelectorHint(spec.Selector),
			Attr:     attr,
			Many:     spec.Many,
			Required: spec.Required,
			Count:    item.Count,
		}
		if spec.Many {
			for _, value := range item.Values {
				result.Values = append(result.Values, TruncateBytes(RedactString(value), 4000))
			}
			for _, value := range item.Values {
				result.ValueBytes += len(value)
			}
		} else {
			result.Value = TruncateBytes(RedactString(item.Value), 4000)
			result.ValueBytes = len(item.Value)
		}
		result.Missing = item.Count == 0
		if result.Missing && spec.Required {
			missing = append(missing, RedactString(name))
		}
		fields = append(fields, result)
	}
	return fields, missing
}

func extractSchemaExpression(def ExtractSchemaDefinition, defaultLimit int) string {
	type jsField struct {
		Name     string `json:"name"`
		Selector string `json:"selector"`
		Attr     string `json:"attr"`
		Many     bool   `json:"many"`
		Limit    int    `json:"limit"`
	}
	fields := make([]jsField, 0, len(def.Fields))
	for name, field := range def.Fields {
		limit := field.Limit
		if limit <= 0 {
			limit = defaultLimit
		}
		if limit > 500 {
			limit = 500
		}
		fields = append(fields, jsField{Name: name, Selector: field.Selector, Attr: normalizeExtractSchemaAttr(field.Attr), Many: field.Many, Limit: limit})
	}
	b, _ := json.Marshal(fields)
	return `(function () {
  const fields = ` + string(b) + `;
  const textOf = (el) => String((el && (el.innerText || el.textContent)) || "").replace(/\s+/g, " ").trim();
  const attrOf = (el, attr) => {
    if (!el) return "";
    attr = String(attr || "text").toLowerCase();
    if (attr === "text") return textOf(el);
    if (attr === "href" || attr === "src") return String(el.getAttribute(attr) || el[attr] || "").trim();
    if (attr === "value") return String(el.value || "").trim();
    if (attr === "html") return String(el.outerHTML || "").slice(0, 4000);
    return String(el.getAttribute(attr) || "").trim();
  };
  return fields.map((field) => {
    const nodes = Array.from(document.querySelectorAll(field.selector || ""));
    const limited = nodes.slice(0, Math.max(1, Number(field.limit || 1)));
    const values = limited.map((node) => attrOf(node, field.attr));
    return {
      name: field.name,
      count: nodes.length,
      value: values.length ? values[0] : "",
      values
    };
  });
})()`
}

func normalizeExtractSchemaAttr(raw string) string {
	attr := strings.ToLower(strings.TrimSpace(raw))
	if attr == "" {
		return "text"
	}
	if attr == "inner_text" || attr == "innertext" {
		return "text"
	}
	return attr
}
