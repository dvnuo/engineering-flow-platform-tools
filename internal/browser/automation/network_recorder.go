package automation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const networkRecorderLimitation = "Records fetch/XHR events only after browser network start injects the page-side recorder; resource timing entries are metadata-only and may not include method, status, or body. Response body previews are captured by default for fetch/XHR only, redacted, truncated, and never include headers, cookies, storage, or request bodies."

type NetworkRecorderOptions struct {
	PageOptions
	Filter       string
	Limit        int
	Method       string
	Status       int
	Body         bool
	MaxBodyBytes int
}

type NetworkWaitOptions struct {
	NetworkRecorderOptions
	URLContains string
}

type NetworkRecordEntry struct {
	Index                int     `json:"index"`
	ID                   string  `json:"id,omitempty"`
	URL                  string  `json:"url"`
	Method               string  `json:"method,omitempty"`
	Status               int     `json:"status,omitempty"`
	ResourceType         string  `json:"resource_type,omitempty"`
	InitiatorType        string  `json:"initiator_type,omitempty"`
	StartedAt            string  `json:"started_at,omitempty"`
	EndedAt              string  `json:"ended_at,omitempty"`
	DurationMilliseconds float64 `json:"duration_ms,omitempty"`
	TransferSizeBytes    int64   `json:"transfer_size_bytes,omitempty"`
	EncodedSizeBytes     int64   `json:"encoded_size_bytes,omitempty"`
	DecodedSizeBytes     int64   `json:"decoded_size_bytes,omitempty"`
	Source               string  `json:"source,omitempty"`
	Error                string  `json:"error,omitempty"`
	BodyPreview          string  `json:"body_preview,omitempty"`
	BodyLength           int     `json:"body_length,omitempty"`
	BodyTruncated        bool    `json:"body_truncated,omitempty"`
	BodyCaptured         bool    `json:"body_captured,omitempty"`
}

type rawNetworkRecordEntry struct {
	ID                   string  `json:"id"`
	URL                  string  `json:"url"`
	Method               string  `json:"method"`
	Status               int     `json:"status"`
	ResourceType         string  `json:"resource_type"`
	InitiatorType        string  `json:"initiator_type"`
	StartedAtUnixMS      float64 `json:"started_at_ms"`
	EndedAtUnixMS        float64 `json:"ended_at_ms"`
	DurationMilliseconds float64 `json:"duration_ms"`
	TransferSizeBytes    int64   `json:"transfer_size_bytes"`
	EncodedSizeBytes     int64   `json:"encoded_size_bytes"`
	DecodedSizeBytes     int64   `json:"decoded_size_bytes"`
	Source               string  `json:"source"`
	Error                string  `json:"error"`
	BodyPreview          string  `json:"body_preview"`
	BodyLength           int     `json:"body_length"`
	BodyTruncated        bool    `json:"body_truncated"`
	BodyCaptured         bool    `json:"body_captured"`
}

type rawNetworkRecorderSnapshot struct {
	Running bool                    `json:"running"`
	Limit   int                     `json:"limit"`
	Count   int                     `json:"count"`
	Entries []rawNetworkRecordEntry `json:"entries"`
}

type NetworkRecorderResult struct {
	Session      string               `json:"session"`
	TargetID     string               `json:"target_id"`
	Action       string               `json:"action"`
	URL          string               `json:"url"`
	Title        string               `json:"title"`
	Running      bool                 `json:"running"`
	Filter       string               `json:"filter,omitempty"`
	Method       string               `json:"method,omitempty"`
	Status       int                  `json:"status,omitempty"`
	Body         bool                 `json:"body"`
	MaxBodyBytes int                  `json:"max_body_bytes,omitempty"`
	Limit        int                  `json:"limit"`
	Count        int                  `json:"count"`
	Entries      []NetworkRecordEntry `json:"entries,omitempty"`
	Artifact     string               `json:"artifact_path,omitempty"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Limitation   string               `json:"limitation"`
}

type NetworkWaitResult struct {
	Session      string             `json:"session"`
	TargetID     string             `json:"target_id"`
	Action       string             `json:"action"`
	URL          string             `json:"url"`
	Title        string             `json:"title"`
	Matched      bool               `json:"matched"`
	URLContains  string             `json:"url_contains"`
	Method       string             `json:"method,omitempty"`
	Status       int                `json:"status,omitempty"`
	Body         bool               `json:"body"`
	MaxBodyBytes int                `json:"max_body_bytes,omitempty"`
	Timeout      int                `json:"timeout"`
	Entry        NetworkRecordEntry `json:"entry,omitempty"`
	Artifact     string             `json:"artifact_path,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
	Limitation   string             `json:"limitation"`
}

type NetworkRecorderArtifact struct {
	Session      string               `json:"session"`
	TargetID     string               `json:"target_id"`
	Running      bool                 `json:"running"`
	Filter       string               `json:"filter,omitempty"`
	Limit        int                  `json:"limit"`
	Body         bool                 `json:"body"`
	MaxBodyBytes int                  `json:"max_body_bytes,omitempty"`
	Count        int                  `json:"count"`
	Entries      []NetworkRecordEntry `json:"entries"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Limitation   string               `json:"limitation"`
}

func (m *Manager) NetworkStart(ctx context.Context, opts NetworkRecorderOptions) (NetworkRecorderResult, error) {
	opts = normalizeNetworkRecorderOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkRecorderResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawNetworkRecorderSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(networkRecorderStartExpression(opts), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return NetworkRecorderResult{}, mapPageError(err, "automation_failed")
	}
	return m.networkRecorderResult(session, target, "start", finalURL, title, raw, opts)
}

func (m *Manager) NetworkStop(ctx context.Context, opts NetworkRecorderOptions) (NetworkRecorderResult, error) {
	opts = normalizeNetworkRecorderOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkRecorderResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawNetworkRecorderSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(networkRecorderStopExpression(), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return NetworkRecorderResult{}, mapPageError(err, "automation_failed")
	}
	return m.networkRecorderResult(session, target, "stop", finalURL, title, raw, opts)
}

func (m *Manager) NetworkList(ctx context.Context, opts NetworkRecorderOptions) (NetworkRecorderResult, error) {
	opts = normalizeNetworkRecorderOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkRecorderResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawNetworkRecorderSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(networkRecorderCollectExpression(opts), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return NetworkRecorderResult{}, mapPageError(err, "automation_failed")
	}
	return m.networkRecorderResult(session, target, "list", finalURL, title, raw, opts)
}

func (m *Manager) NetworkClear(ctx context.Context, opts NetworkRecorderOptions) (NetworkRecorderResult, error) {
	opts = normalizeNetworkRecorderOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkRecorderResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawNetworkRecorderSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Evaluate(networkRecorderClearExpression(), &raw, chromedp.EvalAsValue),
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
	); err != nil {
		return NetworkRecorderResult{}, mapPageError(err, "automation_failed")
	}
	return m.networkRecorderResult(session, target, "clear", finalURL, title, raw, opts)
}

func (m *Manager) NetworkWait(ctx context.Context, opts NetworkWaitOptions) (NetworkWaitResult, error) {
	opts.NetworkRecorderOptions = normalizeNetworkRecorderOptions(opts.NetworkRecorderOptions)
	if strings.TrimSpace(opts.URLContains) == "" {
		return NetworkWaitResult{}, invalidArgs("--url-contains is required", "Pass a URL substring to wait for; returned URLs are redacted.")
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkWaitResult{}, err
	}
	defer cancel()

	timeout := time.Duration(PageTimeoutSeconds(opts.TimeoutSeconds)) * time.Second
	deadline := time.Now().Add(timeout)
	var finalURL, title string
	var lastRaw rawNetworkRecorderSnapshot
	for {
		if err := chromedp.Run(pageCtx,
			chromedp.Evaluate(networkRecorderCollectExpression(opts.NetworkRecorderOptions), &lastRaw, chromedp.EvalAsValue),
			chromedp.Location(&finalURL),
			chromedp.Title(&title),
		); err != nil {
			return NetworkWaitResult{}, mapPageError(err, "automation_failed")
		}
		entries, _ := sanitizeNetworkRecordEntries(lastRaw.Entries, opts.NetworkRecorderOptions)
		for _, entry := range entries {
			if networkRecordWaitMatches(entry, opts) {
				artifact, err := m.writeNetworkArtifact(session, target, lastRaw.Running, opts.NetworkRecorderOptions, entries, len(entries), m.now())
				if err != nil {
					return NetworkWaitResult{}, err
				}
				return NetworkWaitResult{
					Session:      session.Name,
					TargetID:     target.ID,
					Action:       "wait",
					URL:          RedactURL(finalURL),
					Title:        RedactString(title),
					Matched:      true,
					URLContains:  RedactString(opts.URLContains),
					Method:       normalizeNetworkMethod(opts.Method),
					Status:       statusForOutput(opts.Status),
					Body:         opts.Body,
					MaxBodyBytes: opts.MaxBodyBytes,
					Timeout:      PageTimeoutSeconds(opts.TimeoutSeconds),
					Entry:        entry,
					Artifact:     artifact,
					UpdatedAt:    m.now(),
					Limitation:   networkRecorderLimitation,
				}, nil
			}
		}
		if time.Now().After(deadline) {
			return NetworkWaitResult{}, NewError("timeout", "Network event did not match before timeout.", "Check browser network list --json or increase --timeout.", 408)
		}
		select {
		case <-ctx.Done():
			return NetworkWaitResult{}, NewError("timeout", ctx.Err().Error(), "Increase --timeout or check whether the page is responsive.", 408)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func normalizeNetworkRecorderOptions(opts NetworkRecorderOptions) NetworkRecorderOptions {
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.Limit > 5000 {
		opts.Limit = 5000
	}
	opts.Method = normalizeNetworkMethod(opts.Method)
	if opts.Status <= 0 {
		opts.Status = -1
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 20000
	}
	if opts.MaxBodyBytes > 200000 {
		opts.MaxBodyBytes = 200000
	}
	return opts
}

func (m *Manager) networkRecorderResult(session Session, target Target, action, finalURL, title string, raw rawNetworkRecorderSnapshot, opts NetworkRecorderOptions) (NetworkRecorderResult, error) {
	entries, count := sanitizeNetworkRecordEntries(raw.Entries, opts)
	now := m.now()
	artifact, err := m.writeNetworkArtifact(session, target, raw.Running, opts, entries, count, now)
	if err != nil {
		return NetworkRecorderResult{}, err
	}
	return NetworkRecorderResult{
		Session:      session.Name,
		TargetID:     target.ID,
		Action:       action,
		URL:          RedactURL(finalURL),
		Title:        RedactString(title),
		Running:      raw.Running,
		Filter:       RedactString(opts.Filter),
		Method:       opts.Method,
		Status:       statusForOutput(opts.Status),
		Body:         opts.Body,
		MaxBodyBytes: opts.MaxBodyBytes,
		Limit:        opts.Limit,
		Count:        count,
		Entries:      entries,
		Artifact:     artifact,
		UpdatedAt:    now,
		Limitation:   networkRecorderLimitation,
	}, nil
}

func sanitizeNetworkRecordEntries(raw []rawNetworkRecordEntry, opts NetworkRecorderOptions) ([]NetworkRecordEntry, int) {
	opts = normalizeNetworkRecorderOptions(opts)
	out := make([]NetworkRecordEntry, 0, minInt(opts.Limit, len(raw)))
	count := 0
	for _, entry := range raw {
		clean := sanitizeNetworkRecordEntry(entry, opts)
		if !networkRecordMatches(clean, opts) {
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

func sanitizeNetworkRecordEntry(raw rawNetworkRecordEntry, opts NetworkRecorderOptions) NetworkRecordEntry {
	entry := NetworkRecordEntry{
		ID:                   TruncateBytes(RedactString(raw.ID), 120),
		URL:                  RedactURL(raw.URL),
		Method:               normalizeNetworkMethod(raw.Method),
		Status:               normalizeStatus(raw.Status),
		ResourceType:         strings.ToLower(TruncateBytes(RedactString(raw.ResourceType), 80)),
		InitiatorType:        strings.ToLower(TruncateBytes(RedactString(raw.InitiatorType), 80)),
		StartedAt:            unixMillisString(raw.StartedAtUnixMS),
		EndedAt:              unixMillisString(raw.EndedAtUnixMS),
		DurationMilliseconds: nonNegativeFloat(raw.DurationMilliseconds),
		TransferSizeBytes:    nonNegativeInt64(raw.TransferSizeBytes),
		EncodedSizeBytes:     nonNegativeInt64(raw.EncodedSizeBytes),
		DecodedSizeBytes:     nonNegativeInt64(raw.DecodedSizeBytes),
		Source:               strings.ToLower(TruncateBytes(RedactString(raw.Source), 80)),
		Error:                TruncateBytes(RedactError(raw.Error), 500),
	}
	if opts.Body && raw.BodyCaptured {
		body := TruncateBytes(RedactString(raw.BodyPreview), opts.MaxBodyBytes)
		entry.BodyPreview = body
		entry.BodyLength = nonNegativeInt(raw.BodyLength)
		entry.BodyTruncated = raw.BodyTruncated || len(body) > opts.MaxBodyBytes
		entry.BodyCaptured = true
	}
	return entry
}

func networkRecordMatches(entry NetworkRecordEntry, opts NetworkRecorderOptions) bool {
	filter := strings.ToLower(strings.TrimSpace(opts.Filter))
	if filter != "" {
		haystack := strings.ToLower(strings.Join([]string{
			entry.URL,
			entry.Method,
			entry.ResourceType,
			entry.InitiatorType,
			entry.Source,
		}, "\n"))
		if !strings.Contains(haystack, filter) {
			return false
		}
	}
	method := normalizeNetworkMethod(opts.Method)
	if method != "" && entry.Method != method {
		return false
	}
	if opts.Status >= 0 && entry.Status != opts.Status {
		return false
	}
	return true
}

func networkRecordWaitMatches(entry NetworkRecordEntry, opts NetworkWaitOptions) bool {
	if !networkRecordMatches(entry, opts.NetworkRecorderOptions) {
		return false
	}
	return strings.Contains(strings.ToLower(entry.URL), strings.ToLower(RedactString(opts.URLContains))) ||
		strings.Contains(strings.ToLower(entry.URL), strings.ToLower(opts.URLContains))
}

func (m *Manager) writeNetworkArtifact(session Session, target Target, running bool, opts NetworkRecorderOptions, entries []NetworkRecordEntry, count int, now time.Time) (string, error) {
	if err := m.ensureStore(); err != nil {
		return "", err
	}
	path, err := m.Store.NetworkArtifactPath(session.Name, target.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", NewError("artifact_write_failed", err.Error(), "Check permissions for browser network artifacts.", 500)
	}
	artifact := NetworkRecorderArtifact{
		Session:      session.Name,
		TargetID:     target.ID,
		Running:      running,
		Filter:       RedactString(opts.Filter),
		Limit:        opts.Limit,
		Body:         opts.Body,
		MaxBodyBytes: opts.MaxBodyBytes,
		Count:        count,
		Entries:      entries,
		UpdatedAt:    now,
		Limitation:   networkRecorderLimitation,
	}
	b, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", NewError("automation_failed", err.Error(), "Network artifact could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", NewError("artifact_write_failed", err.Error(), "Network artifact could not be written.", 500)
	}
	return path, nil
}

func (s *Store) NetworkArtifactsDir(sessionName string) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	return filepath.Join(s.RootDir, "network", sessionName), nil
}

func (s *Store) NetworkArtifactPath(sessionName, targetID string) (string, error) {
	dir, err := s.NetworkArtifactsDir(sessionName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, safeRefFilePart(targetID)+".json"), nil
}

func normalizeNetworkMethod(raw string) string {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if len(method) > 20 {
		method = method[:20]
	}
	for _, r := range method {
		if (r < 'A' || r > 'Z') && r != '-' {
			return ""
		}
	}
	return method
}

func normalizeStatus(status int) int {
	if status < 0 || status > 999 {
		return 0
	}
	return status
}

func statusForOutput(status int) int {
	if status < 0 {
		return 0
	}
	return status
}

func unixMillisString(ms float64) string {
	if ms <= 0 {
		return ""
	}
	sec := int64(ms / 1000)
	nsec := int64(ms-float64(sec*1000)) * int64(time.Millisecond)
	return time.Unix(sec, nsec).UTC().Format(time.RFC3339Nano)
}

func nonNegativeFloat(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func networkRecorderStartExpression(opts NetworkRecorderOptions) string {
	return networkRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserNetworkRecorder.install(` + strconv.Itoa(opts.Limit) + `, ` + strconv.FormatBool(opts.Body) + `, ` + strconv.Itoa(opts.MaxBodyBytes) + `);
})()`
}

func networkRecorderCollectExpression(opts NetworkRecorderOptions) string {
	return networkRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserNetworkRecorder.collect(` + strconv.Itoa(opts.Limit) + `, ` + strconv.FormatBool(opts.Body) + `, ` + strconv.Itoa(opts.MaxBodyBytes) + `);
})()`
}

func networkRecorderStopExpression() string {
	return networkRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserNetworkRecorder.stop();
})()`
}

func networkRecorderClearExpression() string {
	return networkRecorderLibraryExpression() + `
(function () {
  return window.__efpBrowserNetworkRecorder.clear();
})()`
}

func networkRecorderLibraryExpression() string {
	return `(function () {
  if (window.__efpBrowserNetworkRecorder && window.__efpBrowserNetworkRecorder.version === 2) return;
  const now = () => Date.now();
  const performanceNow = () => (window.performance && performance.now) ? performance.now() : 0;
  const normalizeURL = (value) => {
    try {
      if (value && typeof Request !== "undefined" && value instanceof Request) return String(value.url || "");
    } catch (_) {}
    try {
      if (value && typeof URL !== "undefined" && value instanceof URL) return String(value.href || "");
    } catch (_) {}
    return String(value || "");
  };
  const normalizeMethod = (input, init, fallback) => {
    let method = fallback || "GET";
    try {
      if (input && typeof Request !== "undefined" && input instanceof Request && input.method) method = input.method;
    } catch (_) {}
    if (init && init.method) method = init.method;
    return String(method || "GET").toUpperCase().slice(0, 20);
  };
  const state = {
    version: 2,
    running: false,
    limit: 500,
    body: true,
    max_body_bytes: 20000,
    sequence: 0,
    entries: [],
    originals: {
      fetch: window.fetch,
      xhrOpen: window.XMLHttpRequest && window.XMLHttpRequest.prototype ? window.XMLHttpRequest.prototype.open : null,
      xhrSend: window.XMLHttpRequest && window.XMLHttpRequest.prototype ? window.XMLHttpRequest.prototype.send : null
    },
    observer: null
  };
  const push = (entry) => {
    entry.id = entry.id || ("net-" + (++state.sequence));
    state.entries.push(entry);
    if (state.entries.length > state.limit) state.entries.splice(0, state.entries.length - state.limit);
  };
  const resourceEntry = (entry) => ({
    id: "res-" + Math.round(Number(entry.startTime || 0)) + "-" + String(entry.name || "").length,
    url: String(entry.name || ""),
    method: "",
    status: 0,
    resource_type: String(entry.initiatorType || "resource"),
    initiator_type: String(entry.initiatorType || "resource"),
    started_at_ms: now() - Math.max(0, performanceNow() - Number(entry.startTime || 0)),
    ended_at_ms: now() - Math.max(0, performanceNow() - Number((entry.responseEnd || (entry.startTime + entry.duration) || entry.startTime) || 0)),
    duration_ms: Number(entry.duration || 0),
    transfer_size_bytes: Number(entry.transferSize || 0),
    encoded_size_bytes: Number(entry.encodedBodySize || 0),
    decoded_size_bytes: Number(entry.decodedBodySize || 0),
    source: "resource_timing",
    error: "",
    body_preview: "",
    body_length: 0,
    body_truncated: false,
    body_captured: false
  });
  const collectResources = () => {
    const resources = (window.performance && performance.getEntriesByType) ? performance.getEntriesByType("resource") : [];
    const existing = new Set(state.entries.filter(entry => entry.source === "resource_timing").map(entry => entry.id));
    for (const entry of resources) {
      const record = resourceEntry(entry);
      if (!existing.has(record.id)) {
        existing.add(record.id);
        push(record);
      }
    }
  };
  const attachBodyPreview = async (record, response) => {
    if (!state.body || !response || !response.clone) return;
    try {
      const clone = response.clone();
      const body = await clone.text();
      record.body_length = body.length;
      record.body_preview = body.slice(0, state.max_body_bytes);
      record.body_truncated = body.length > state.max_body_bytes;
      record.body_captured = true;
    } catch (err) {
      record.body_preview = "";
      record.body_length = 0;
      record.body_truncated = false;
      record.body_captured = false;
    }
  };
  state.install = (limit, body, maxBodyBytes) => {
    state.limit = Math.max(1, Math.min(5000, Number(limit || state.limit || 500)));
    state.body = body !== false;
    state.max_body_bytes = Math.max(0, Math.min(200000, Number(maxBodyBytes || state.max_body_bytes || 20000)));
    state.running = true;
    collectResources();
    if (!state.observer && window.PerformanceObserver) {
      try {
        state.observer = new PerformanceObserver((list) => {
          for (const entry of list.getEntries()) push(resourceEntry(entry));
        });
        state.observer.observe({entryTypes: ["resource"]});
      } catch (_) {}
    }
    if (state.originals.fetch && !state.originals.fetch.__efpBrowserNetworkWrapped) {
      const wrappedFetch = async function(input, init) {
        const startedAt = now();
        const startedPerf = performanceNow();
        const record = {
          id: "fetch-" + (++state.sequence),
          url: normalizeURL(input),
          method: normalizeMethod(input, init, "GET"),
          status: 0,
          resource_type: "fetch",
          initiator_type: "fetch",
          started_at_ms: startedAt,
          ended_at_ms: 0,
          duration_ms: 0,
          transfer_size_bytes: 0,
          encoded_size_bytes: 0,
          decoded_size_bytes: 0,
          source: "fetch",
          error: "",
          body_preview: "",
          body_length: 0,
          body_truncated: false,
          body_captured: false
        };
        try {
          const response = await state.originals.fetch.apply(this, arguments);
          record.status = Number(response && response.status || 0);
          record.ended_at_ms = now();
          record.duration_ms = performanceNow() - startedPerf;
          await attachBodyPreview(record, response);
          push(record);
          return response;
        } catch (err) {
          record.ended_at_ms = now();
          record.duration_ms = performanceNow() - startedPerf;
          record.error = String(err);
          push(record);
          throw err;
        }
      };
      wrappedFetch.__efpBrowserNetworkWrapped = true;
      window.fetch = wrappedFetch;
    }
    if (state.originals.xhrOpen && state.originals.xhrSend && window.XMLHttpRequest && window.XMLHttpRequest.prototype) {
      XMLHttpRequest.prototype.open = function(method, url) {
        this.__efpBrowserNetworkRecord = {
          id: "xhr-" + (++state.sequence),
          url: normalizeURL(url),
          method: normalizeMethod(null, {method}, "GET"),
          status: 0,
          resource_type: "xhr",
          initiator_type: "xmlhttprequest",
          started_at_ms: 0,
          ended_at_ms: 0,
          duration_ms: 0,
          transfer_size_bytes: 0,
          encoded_size_bytes: 0,
          decoded_size_bytes: 0,
          source: "xhr",
          error: "",
          body_preview: "",
          body_length: 0,
          body_truncated: false,
          body_captured: false
        };
        return state.originals.xhrOpen.apply(this, arguments);
      };
      XMLHttpRequest.prototype.send = function() {
        const record = this.__efpBrowserNetworkRecord;
        const startedPerf = performanceNow();
        if (record) record.started_at_ms = now();
        const finalize = () => {
          if (!record || record.__done) return;
          record.__done = true;
          record.status = Number(this.status || 0);
          record.ended_at_ms = now();
          record.duration_ms = performanceNow() - startedPerf;
          if (state.body) {
            try {
              const body = String(this.responseText || "");
              record.body_length = body.length;
              record.body_preview = body.slice(0, state.max_body_bytes);
              record.body_truncated = body.length > state.max_body_bytes;
              record.body_captured = true;
            } catch (_) {}
          }
          push(record);
        };
        this.addEventListener("loadend", finalize, {once: true});
        this.addEventListener("error", () => { if (record) record.error = "xhr error"; finalize(); }, {once: true});
        return state.originals.xhrSend.apply(this, arguments);
      };
    }
    return state.collect(state.limit, state.body, state.max_body_bytes);
  };
  state.collect = (limit, body, maxBodyBytes) => {
    state.limit = Math.max(1, Math.min(5000, Number(limit || state.limit || 500)));
    state.body = body !== false;
    state.max_body_bytes = Math.max(0, Math.min(200000, Number(maxBodyBytes || state.max_body_bytes || 20000)));
    collectResources();
    return {running: state.running, limit: state.limit, count: state.entries.length, entries: state.entries.slice(-state.limit)};
  };
  state.clear = () => {
    state.entries = [];
    return state.collect(state.limit, state.body, state.max_body_bytes);
  };
  state.stop = () => {
    state.running = false;
    try { if (state.observer) state.observer.disconnect(); } catch (_) {}
    state.observer = null;
    if (state.originals.fetch) window.fetch = state.originals.fetch;
    if (state.originals.xhrOpen && window.XMLHttpRequest && window.XMLHttpRequest.prototype) XMLHttpRequest.prototype.open = state.originals.xhrOpen;
    if (state.originals.xhrSend && window.XMLHttpRequest && window.XMLHttpRequest.prototype) XMLHttpRequest.prototype.send = state.originals.xhrSend;
    return state.collect(state.limit, state.body, state.max_body_bytes);
  };
  window.__efpBrowserNetworkRecorder = state;
})()`
}
