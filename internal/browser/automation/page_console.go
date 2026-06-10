package automation

import (
	"context"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

const consoleRecorderLimitation = "Console diagnostics are captured after browser page console, page errors, or page console-clear injects a page-side recorder. Earlier console events from before injection may not be available."

type ConsoleOptions struct {
	PageOptions
	Level string
	Limit int
}

type ConsoleEntry struct {
	Index     int    `json:"index"`
	Level     string `json:"level"`
	Message   string `json:"message,omitempty"`
	Source    string `json:"source,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	URL       string `json:"url,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	Stack     string `json:"stack,omitempty"`
}

type rawConsoleEntry struct {
	Level       string  `json:"level"`
	Message     string  `json:"message"`
	Source      string  `json:"source"`
	TimestampMS float64 `json:"timestamp_ms"`
	URL         string  `json:"url"`
	Line        int     `json:"line"`
	Column      int     `json:"column"`
	Stack       string  `json:"stack"`
}

type rawConsoleSnapshot struct {
	Count   int               `json:"count"`
	Entries []rawConsoleEntry `json:"entries"`
}

type ConsoleResult struct {
	Session    string         `json:"session"`
	TargetID   string         `json:"target_id"`
	URL        string         `json:"url"`
	Title      string         `json:"title"`
	Level      string         `json:"level,omitempty"`
	Limit      int            `json:"limit"`
	Count      int            `json:"count"`
	Entries    []ConsoleEntry `json:"entries"`
	Limitation string         `json:"limitation"`
}

func (m *Manager) Console(ctx context.Context, opts ConsoleOptions) (ConsoleResult, error) {
	opts, err := normalizeConsoleOptions(opts)
	if err != nil {
		return ConsoleResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ConsoleResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawConsoleSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(consoleRecorderCollectExpression(opts.Limit), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return ConsoleResult{}, mapPageError(err, "automation_failed")
	}
	entries, count := sanitizeConsoleEntries(raw.Entries, opts)
	return ConsoleResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		Level:      opts.Level,
		Limit:      opts.Limit,
		Count:      count,
		Entries:    entries,
		Limitation: consoleRecorderLimitation,
	}, nil
}

func (m *Manager) RuntimeErrors(ctx context.Context, opts ConsoleOptions) (ConsoleResult, error) {
	opts.Level = "error"
	result, err := m.Console(ctx, opts)
	if err != nil {
		return ConsoleResult{}, err
	}
	filtered := make([]ConsoleEntry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		if entry.Level == "error" || entry.Source == "runtime_exception" || entry.Source == "unhandled_rejection" {
			filtered = append(filtered, entry)
		}
	}
	result.Entries = filtered
	result.Count = len(filtered)
	return result, nil
}

func (m *Manager) ConsoleClear(ctx context.Context, opts ConsoleOptions) (ConsoleResult, error) {
	opts, err := normalizeConsoleOptions(opts)
	if err != nil {
		return ConsoleResult{}, err
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return ConsoleResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawConsoleSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(consoleRecorderClearExpression(), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return ConsoleResult{}, mapPageError(err, "automation_failed")
	}
	return ConsoleResult{
		Session:    session.Name,
		TargetID:   target.ID,
		URL:        RedactURL(finalURL),
		Title:      RedactString(title),
		Limit:      opts.Limit,
		Count:      0,
		Entries:    []ConsoleEntry{},
		Limitation: consoleRecorderLimitation,
	}, nil
}

func normalizeConsoleOptions(opts ConsoleOptions) (ConsoleOptions, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 500 {
		opts.Limit = 500
	}
	opts.Level = normalizeConsoleLevel(opts.Level)
	if opts.Level == "invalid" {
		return ConsoleOptions{}, invalidArgs("--level must be error, warning, info, log, or debug", "Pass a supported console level or omit --level.")
	}
	return opts, nil
}

func sanitizeConsoleEntries(raw []rawConsoleEntry, opts ConsoleOptions) ([]ConsoleEntry, int) {
	opts, _ = normalizeConsoleOptions(opts)
	out := make([]ConsoleEntry, 0, minInt(opts.Limit, len(raw)))
	count := 0
	for _, entry := range raw {
		clean := sanitizeConsoleEntry(entry)
		if opts.Level != "" && clean.Level != opts.Level {
			continue
		}
		count++
		if len(out) >= opts.Limit {
			continue
		}
		clean.Index = count - 1
		out = append(out, clean)
	}
	return out, count
}

func sanitizeConsoleEntry(raw rawConsoleEntry) ConsoleEntry {
	return ConsoleEntry{
		Level:     normalizeConsoleLevel(raw.Level),
		Message:   TruncateBytes(RedactString(raw.Message), 1000),
		Source:    strings.ToLower(TruncateBytes(RedactString(raw.Source), 80)),
		Timestamp: unixMillisString(raw.TimestampMS),
		URL:       RedactURL(raw.URL),
		Line:      nonNegativeInt(raw.Line),
		Column:    nonNegativeInt(raw.Column),
		Stack:     TruncateBytes(RedactString(raw.Stack), 4000),
	}
}

func normalizeConsoleLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "warn", "warning":
		return "warning"
	case "error":
		return "error"
	case "info":
		return "info"
	case "log":
		return "log"
	case "debug":
		return "debug"
	default:
		return "invalid"
	}
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func consoleRecorderCollectExpression(limit int) string {
	return consoleRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserConsoleRecorder.collect(` + strconv.Itoa(limit) + `);
})()`
}

func consoleRecorderClearExpression() string {
	return consoleRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserConsoleRecorder.clear();
})()`
}

func consoleRecorderLibraryExpression() string {
	return `(function () {
  if (window.__efpBrowserConsoleRecorder && window.__efpBrowserConsoleRecorder.version === 1) return;
  const state = {
    version: 1,
    limit: 500,
    entries: [],
    sequence: 0,
    originals: {
      log: console.log,
      info: console.info,
      warn: console.warn,
      error: console.error,
      debug: console.debug
    }
  };
  const levelMap = {warn: "warning"};
  const safeString = (value) => {
    try {
      if (value === null || value === undefined) return String(value);
      if (typeof value === "string") return value;
      if (typeof value === "number" || typeof value === "boolean" || typeof value === "bigint") return String(value);
      if (value instanceof Error) return String(value.name || "Error") + ": " + String(value.message || "");
      const tag = Object.prototype.toString.call(value);
      return tag || "[object]";
    } catch (_) {
      return "[unserializable]";
    }
  };
  const push = (entry) => {
    entry.timestamp_ms = entry.timestamp_ms || Date.now();
    state.entries.push(entry);
    if (state.entries.length > state.limit) state.entries.splice(0, state.entries.length - state.limit);
  };
  const wrap = (level) => {
    const original = state.originals[level];
    if (typeof original !== "function" || original.__efpBrowserConsoleWrapped) return;
    const wrapped = function() {
      const args = Array.from(arguments || []);
      push({
        level: levelMap[level] || level,
        message: args.map(safeString).join(" "),
        source: "console_api",
        timestamp_ms: Date.now(),
        url: String(location.href || ""),
        line: 0,
        column: 0,
        stack: ""
      });
      return original.apply(this, arguments);
    };
    wrapped.__efpBrowserConsoleWrapped = true;
    console[level] = wrapped;
  };
  ["log","info","warn","error","debug"].forEach(wrap);
  window.addEventListener("error", function(event) {
    const err = event.error;
    push({
      level: "error",
      message: String(event.message || (err && err.message) || "runtime error"),
      source: "runtime_exception",
      timestamp_ms: Date.now(),
      url: String(event.filename || location.href || ""),
      line: Number(event.lineno || 0),
      column: Number(event.colno || 0),
      stack: err && err.stack ? String(err.stack) : ""
    });
  });
  window.addEventListener("unhandledrejection", function(event) {
    const reason = event.reason;
    push({
      level: "error",
      message: safeString(reason || "unhandled rejection"),
      source: "unhandled_rejection",
      timestamp_ms: Date.now(),
      url: String(location.href || ""),
      line: 0,
      column: 0,
      stack: reason && reason.stack ? String(reason.stack) : ""
    });
  });
  state.collect = (limit) => {
    state.limit = Math.max(1, Math.min(500, Number(limit || state.limit || 50)));
    return {count: state.entries.length, entries: state.entries.slice(-state.limit)};
  };
  state.clear = () => {
    state.entries = [];
    return state.collect(state.limit);
  };
  window.__efpBrowserConsoleRecorder = state;
})()`
}
