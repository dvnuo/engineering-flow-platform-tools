package automation

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"
)

type WorkflowRecordOptions struct {
	PageOptions
	OutPath              string
	DurationMilliseconds int
	Limit                int
}

type WorkflowRecordedEvent struct {
	Index     int              `json:"index"`
	Action    string           `json:"action"`
	Selector  string           `json:"selector,omitempty"`
	Key       string           `json:"key,omitempty"`
	InputKind string           `json:"input_kind,omitempty"`
	TextBytes int              `json:"text_bytes,omitempty"`
	Locators  []ElementLocator `json:"locators,omitempty"`
	At        time.Time        `json:"at"`
}

type WorkflowRecordResult struct {
	Session     string                  `json:"session"`
	TargetID    string                  `json:"target_id"`
	URL         string                  `json:"url"`
	Title       string                  `json:"title"`
	Path        string                  `json:"path"`
	Bytes       int64                   `json:"bytes"`
	DurationMS  int                     `json:"duration_ms"`
	Limit       int                     `json:"limit"`
	EventCount  int                     `json:"event_count"`
	StepCount   int                     `json:"step_count"`
	Events      []WorkflowRecordedEvent `json:"events"`
	Limitation  string                  `json:"limitation"`
	GeneratedAt time.Time               `json:"generated_at"`
}

type workflowRecordRawEvent struct {
	Action    string           `json:"action"`
	Selector  string           `json:"selector"`
	Key       string           `json:"key"`
	InputKind string           `json:"input_kind"`
	TextBytes int              `json:"text_bytes"`
	Locators  []ElementLocator `json:"locators"`
}

type workflowRecordFile struct {
	Session string                   `yaml:"session,omitempty"`
	Vars    map[string]string        `yaml:"vars,omitempty"`
	Steps   []workflowRecordFileStep `yaml:"steps"`
}

type workflowRecordFileStep struct {
	Action   string           `yaml:"action"`
	Name     string           `yaml:"name,omitempty"`
	Selector string           `yaml:"selector,omitempty"`
	Locators []ElementLocator `yaml:"locators,omitempty"`
	Text     string           `yaml:"text,omitempty"`
	Label    string           `yaml:"label,omitempty"`
	Key      string           `yaml:"key,omitempty"`
	Clear    bool             `yaml:"clear,omitempty"`
}

func (m *Manager) RecordWorkflow(ctx context.Context, opts WorkflowRecordOptions) (WorkflowRecordResult, error) {
	opts = normalizeWorkflowRecordOptions(opts)
	if strings.TrimSpace(opts.OutPath) == "" {
		return WorkflowRecordResult{}, invalidArgs("--out is required", "Pass the workflow YAML path to write, such as flow.yaml.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return WorkflowRecordResult{}, err
	}
	defer cancel()

	var finalURL, title string
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(workflowRecordInstallExpression(opts.Limit), nil, chromedp.EvalAsValue),
		chromedp.Sleep(time.Duration(opts.DurationMilliseconds)*time.Millisecond),
	); err != nil {
		return WorkflowRecordResult{}, mapPageError(err, "automation_failed")
	}
	var raw []workflowRecordRawEvent
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(`(window.__efpWorkflowRecorder && window.__efpWorkflowRecorder.events) || []`, &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return WorkflowRecordResult{}, mapPageError(err, "automation_failed")
	}

	now := m.now()
	events, file := workflowRecordFileFromEvents(raw, session.Name, now)
	outPath := filepath.Clean(expandHome(opts.OutPath))
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return WorkflowRecordResult{}, NewError("artifact_write_failed", err.Error(), "Check --out directory permissions.", 500)
	}
	b, err := yaml.Marshal(file)
	if err != nil {
		return WorkflowRecordResult{}, NewError("automation_failed", err.Error(), "Recorded workflow could not be encoded.", 500)
	}
	if err := os.WriteFile(outPath, b, 0o600); err != nil {
		return WorkflowRecordResult{}, NewError("artifact_write_failed", err.Error(), "Recorded workflow could not be written.", 500)
	}
	stat, err := os.Stat(outPath)
	if err != nil {
		return WorkflowRecordResult{}, NewError("artifact_write_failed", err.Error(), "Recorded workflow was written but metadata could not be read.", 500)
	}
	return WorkflowRecordResult{
		Session:     session.Name,
		TargetID:    target.ID,
		URL:         RedactURL(finalURL),
		Title:       RedactString(title),
		Path:        outPath,
		Bytes:       stat.Size(),
		DurationMS:  opts.DurationMilliseconds,
		Limit:       opts.Limit,
		EventCount:  len(events),
		StepCount:   len(file.Steps),
		Events:      events,
		Limitation:  "Recording writes a safe workflow skeleton only. Typed text and selected option values are replaced with empty variables and are not returned.",
		GeneratedAt: now,
	}, nil
}

func normalizeWorkflowRecordOptions(opts WorkflowRecordOptions) WorkflowRecordOptions {
	if opts.DurationMilliseconds <= 0 {
		opts.DurationMilliseconds = 10000
	}
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	return opts
}

func workflowRecordFileFromEvents(raw []workflowRecordRawEvent, session string, now time.Time) ([]WorkflowRecordedEvent, workflowRecordFile) {
	vars := map[string]string{}
	steps := make([]workflowRecordFileStep, 0, len(raw))
	events := make([]WorkflowRecordedEvent, 0, len(raw))
	textIndex := 0
	selectIndex := 0
	for _, item := range raw {
		action := strings.TrimSpace(item.Action)
		selector := normalizeSelectorHint(item.Selector)
		if action == "" || selector == "" {
			continue
		}
		event := WorkflowRecordedEvent{
			Index:     len(events),
			Action:    action,
			Selector:  selector,
			Key:       RedactString(item.Key),
			InputKind: RedactString(item.InputKind),
			TextBytes: item.TextBytes,
			Locators:  sanitizeElementLocators(item.Locators),
			At:        now,
		}
		step := workflowRecordFileStep{Action: action, Selector: selector, Locators: workflowRecordLocators(selector, item.Locators)}
		switch action {
		case "page.type":
			textIndex++
			name := "recorded_text_" + strconv.Itoa(textIndex)
			vars[name] = ""
			step.Text = "{{vars." + name + "}}"
			step.Clear = true
			step.Name = "fill " + name
		case "page.select":
			selectIndex++
			name := "recorded_select_" + strconv.Itoa(selectIndex)
			vars[name] = ""
			step.Label = "{{vars." + name + "}}"
			step.Name = "select " + name
		case "page.press":
			step.Key = RedactString(item.Key)
		}
		events = append(events, event)
		steps = append(steps, step)
	}
	file := workflowRecordFile{Session: session, Steps: steps}
	if len(vars) > 0 {
		file.Vars = vars
	}
	return events, file
}

func workflowRecordLocators(selector string, raw []ElementLocator) []ElementLocator {
	locators := sanitizeElementLocators(raw)
	if strings.TrimSpace(selector) != "" {
		locators = append([]ElementLocator{{Selector: selector}}, locators...)
	}
	out := make([]ElementLocator, 0, len(locators))
	seen := map[string]bool{}
	for _, locator := range locators {
		key := strings.Join([]string{
			locator.Selector,
			locator.Role,
			locator.Name,
			locator.Text,
			locator.Label,
			locator.Placeholder,
			locator.NearText,
			strconv.Itoa(locator.Nth),
		}, "\x00")
		if seen[key] || !hasElementLocator(locator) {
			continue
		}
		seen[key] = true
		out = append(out, locator)
	}
	return out
}

func workflowRecordInstallExpression(limit int) string {
	return `(function () {
  const limit = ` + strconv.Itoa(limit) + `;
  const root = window.__efpWorkflowRecorder = {events: []};
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
  const labelFor = (el) => {
    if (isSensitiveElement(el)) return "";
    const labels = [];
    if (el.labels) for (const label of Array.from(el.labels)) labels.push(textOf(label, 500));
    const id = attr(el, "id");
    if (id && !sensitiveNamePattern.test(id)) {
      const byFor = document.querySelector("label[for='" + cssEscape(id) + "']");
      if (byFor) labels.push(textOf(byFor, 500));
    }
    return labels.filter(Boolean).join(" ").trim().slice(0, 500);
  };
  const roleFor = (el, tag) => {
    const role = attr(el, "role");
    if (role) return role.toLowerCase();
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
    return "";
  };
  const nameFor = (el, tag, role) => {
    if (isSensitiveElement(el)) return labelFor(el);
    const aria = safeAttr(el, "aria-label", 500);
    const label = labelFor(el);
    const placeholder = safeAttr(el, "placeholder", 500);
    const title = safeAttr(el, "title", 500);
    if (aria) return aria;
    if (label) return label;
    if (placeholder && (tag === "input" || tag === "textarea")) return placeholder;
    if (title) return title;
    if (role === "button" || role === "link" || tag === "button" || tag === "a") return textOf(el, 500);
    return "";
  };
  const selectorFor = (el) => {
    if (!el || el.nodeType !== 1) return "";
    const id = attr(el, "id");
    const tag = String(el.tagName || "").toLowerCase();
    if (id) return tag + "#" + cssEscape(id);
    const name = attr(el, "name");
    if (name && ["input","textarea","select","button"].includes(tag)) return tag + "[name=\\"" + cssEscape(name) + "\\"]";
    const parts = [];
    let node = el;
    while (node && node.nodeType === 1 && node !== document.documentElement && parts.length < 6) {
      const t = String(node.tagName || "").toLowerCase();
      const nodeId = attr(node, "id");
      if (nodeId) {
        parts.unshift(t + "#" + cssEscape(nodeId));
        break;
      }
      let nth = 1;
      let prev = node.previousElementSibling;
      while (prev) {
        if (String(prev.tagName || "").toLowerCase() === t) nth++;
        prev = prev.previousElementSibling;
      }
      parts.unshift(t + ":nth-of-type(" + nth + ")");
      node = node.parentElement;
    }
    return parts.join(" > ");
  };
  const push = (event) => {
    if (root.events.length >= limit) return;
    const target = event.target_element;
    delete event.target_element;
    if (target && target.nodeType === 1) {
      const tag = String(target.tagName || "").toLowerCase();
      const role = roleFor(target, tag);
      const name = nameFor(target, tag, role);
      const label = labelFor(target);
      const placeholder = isSensitiveElement(target) ? "" : safeAttr(target, "placeholder", 500);
      const text = (tag === "input" || tag === "textarea" || tag === "select" || isSensitiveElement(target)) ? "" : textOf(target, 500);
      const locators = [{selector: event.selector}];
      if (role && name) locators.push({role, name});
      if (role && text) locators.push({role, text});
      if (label) locators.push({label});
      if (placeholder) locators.push({placeholder});
      event.locators = locators;
    }
    root.events.push(event);
  };
  document.addEventListener("click", (event) => {
    const target = event.target && event.target.closest ? event.target.closest("button,a,input,select,textarea,[role=button],[role=checkbox],[role=radio]") : event.target;
    if (!target) return;
    const tag = String(target.tagName || "").toLowerCase();
    const type = String(target.type || "").toLowerCase();
    if (tag === "input" && ["text","search","email","password","tel","url","number"].includes(type)) return;
    if (tag === "input" && ["checkbox","radio"].includes(type)) {
      push({action: target.checked ? "page.check" : "page.uncheck", selector: selectorFor(target), input_kind: type, target_element: target});
      return;
    }
    push({action: "page.click", selector: selectorFor(target), input_kind: tag || type, target_element: target});
  }, true);
  document.addEventListener("change", (event) => {
    const target = event.target;
    const tag = String(target && target.tagName || "").toLowerCase();
    const type = String(target && target.type || "").toLowerCase();
    if (!target) return;
    if (tag === "input" && ["checkbox","radio"].includes(type)) {
      push({action: target.checked ? "page.check" : "page.uncheck", selector: selectorFor(target), input_kind: type, target_element: target});
      return;
    }
    if (tag === "select") {
      push({action: "page.select", selector: selectorFor(target), input_kind: "select", text_bytes: String(target.value || "").length, target_element: target});
      return;
    }
    if (tag === "input" || tag === "textarea") {
      push({action: "page.type", selector: selectorFor(target), input_kind: type || tag, text_bytes: String(target.value || "").length, target_element: target});
    }
  }, true);
  document.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === "Escape" || event.key === "Tab") {
      push({action: "page.press", selector: selectorFor(event.target), key: event.key, input_kind: "key", target_element: event.target});
    }
  }, true);
  return {installed: true, limit};
})()`
}
