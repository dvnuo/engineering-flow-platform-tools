package automation

import (
	"context"
	"sort"
	"strings"

	"github.com/chromedp/chromedp"
)

const performanceMetricsLimitation = "Performance metrics are browser timing metadata only. The command returns navigation, paint, resource timing aggregates, DOM node counts, and long-task counts; it never returns headers, cookies, storage, request bodies, or response bodies."

type MetricsOptions struct {
	PageOptions
	LimitResources int
	Filter         string
}

type MetricsResult struct {
	Session          string                 `json:"session"`
	TargetID         string                 `json:"target_id"`
	URL              string                 `json:"url"`
	Title            string                 `json:"title"`
	Filter           string                 `json:"filter,omitempty"`
	LimitResources   int                    `json:"limit_resources"`
	Navigation       NavigationMetrics      `json:"navigation"`
	Paints           []PaintMetric          `json:"paints"`
	Resources        ResourceMetrics        `json:"resources"`
	DOM              DOMMetrics             `json:"dom"`
	LongTasks        LongTaskMetrics        `json:"long_tasks"`
	LargestResources []LargestResourceEntry `json:"largest_resources"`
	Limitation       string                 `json:"limitation"`
}

type NavigationMetrics struct {
	Type                         string  `json:"type,omitempty"`
	StartTimeMilliseconds        float64 `json:"start_time_ms,omitempty"`
	DurationMilliseconds         float64 `json:"duration_ms,omitempty"`
	DOMContentLoadedMilliseconds float64 `json:"dom_content_loaded_ms,omitempty"`
	LoadEventMilliseconds        float64 `json:"load_event_ms,omitempty"`
	ResponseStartMilliseconds    float64 `json:"response_start_ms,omitempty"`
	TransferSizeBytes            int64   `json:"transfer_size_bytes,omitempty"`
	EncodedBodySizeBytes         int64   `json:"encoded_body_size_bytes,omitempty"`
	DecodedBodySizeBytes         int64   `json:"decoded_body_size_bytes,omitempty"`
}

type PaintMetric struct {
	Name              string  `json:"name"`
	StartMilliseconds float64 `json:"start_time_ms"`
}

type ResourceMetrics struct {
	TotalCount           int            `json:"total_count"`
	MatchedCount         int            `json:"matched_count"`
	ReturnedCount        int            `json:"returned_count"`
	TransferSizeBytes    int64          `json:"transfer_size_bytes,omitempty"`
	EncodedBodySizeBytes int64          `json:"encoded_body_size_bytes,omitempty"`
	DecodedBodySizeBytes int64          `json:"decoded_body_size_bytes,omitempty"`
	DurationMilliseconds float64        `json:"duration_ms,omitempty"`
	ByResourceType       map[string]int `json:"by_resource_type,omitempty"`
}

type DOMMetrics struct {
	NodeCount int `json:"node_count"`
}

type LongTaskMetrics struct {
	Count             int     `json:"count"`
	TotalMilliseconds float64 `json:"total_ms,omitempty"`
	MaxMilliseconds   float64 `json:"max_ms,omitempty"`
}

type LargestResourceEntry struct {
	Index                int     `json:"index"`
	URL                  string  `json:"url"`
	ResourceType         string  `json:"resource_type,omitempty"`
	InitiatorType        string  `json:"initiator_type,omitempty"`
	StartMilliseconds    float64 `json:"start_time_ms,omitempty"`
	DurationMilliseconds float64 `json:"duration_ms,omitempty"`
	TransferSizeBytes    int64   `json:"transfer_size_bytes,omitempty"`
	EncodedBodySizeBytes int64   `json:"encoded_body_size_bytes,omitempty"`
	DecodedBodySizeBytes int64   `json:"decoded_body_size_bytes,omitempty"`
}

type rawMetricsSnapshot struct {
	Navigation NavigationMetrics      `json:"navigation"`
	Paints     []PaintMetric          `json:"paints"`
	Resources  []LargestResourceEntry `json:"resources"`
	DOM        DOMMetrics             `json:"dom"`
	LongTasks  LongTaskMetrics        `json:"long_tasks"`
}

func (m *Manager) Metrics(ctx context.Context, opts MetricsOptions) (MetricsResult, error) {
	opts = normalizeMetricsOptions(opts)
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return MetricsResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw rawMetricsSnapshot
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(metricsExpression(), &raw, chromedp.EvalAsValue),
	); err != nil {
		return MetricsResult{}, mapPageError(err, "automation_failed")
	}
	aggregate, largest := sanitizeMetricResources(raw.Resources, opts)
	return MetricsResult{
		Session:          session.Name,
		TargetID:         target.ID,
		URL:              RedactURL(finalURL),
		Title:            RedactString(title),
		Filter:           RedactString(opts.Filter),
		LimitResources:   opts.LimitResources,
		Navigation:       sanitizeNavigationMetrics(raw.Navigation),
		Paints:           sanitizePaintMetrics(raw.Paints),
		Resources:        aggregate,
		DOM:              DOMMetrics{NodeCount: maxInt(raw.DOM.NodeCount, 0)},
		LongTasks:        sanitizeLongTaskMetrics(raw.LongTasks),
		LargestResources: largest,
		Limitation:       performanceMetricsLimitation,
	}, nil
}

func normalizeMetricsOptions(opts MetricsOptions) MetricsOptions {
	if opts.LimitResources <= 0 {
		opts.LimitResources = 10
	}
	if opts.LimitResources > 100 {
		opts.LimitResources = 100
	}
	return opts
}

func sanitizeNavigationMetrics(raw NavigationMetrics) NavigationMetrics {
	raw.Type = strings.ToLower(TruncateBytes(RedactString(raw.Type), 80))
	raw.StartTimeMilliseconds = nonNegativeFloat(raw.StartTimeMilliseconds)
	raw.DurationMilliseconds = nonNegativeFloat(raw.DurationMilliseconds)
	raw.DOMContentLoadedMilliseconds = nonNegativeFloat(raw.DOMContentLoadedMilliseconds)
	raw.LoadEventMilliseconds = nonNegativeFloat(raw.LoadEventMilliseconds)
	raw.ResponseStartMilliseconds = nonNegativeFloat(raw.ResponseStartMilliseconds)
	raw.TransferSizeBytes = nonNegativeInt64(raw.TransferSizeBytes)
	raw.EncodedBodySizeBytes = nonNegativeInt64(raw.EncodedBodySizeBytes)
	raw.DecodedBodySizeBytes = nonNegativeInt64(raw.DecodedBodySizeBytes)
	return raw
}

func sanitizePaintMetrics(raw []PaintMetric) []PaintMetric {
	out := make([]PaintMetric, 0, len(raw))
	for _, paint := range raw {
		name := strings.ToLower(TruncateBytes(RedactString(paint.Name), 120))
		if name == "" {
			continue
		}
		out = append(out, PaintMetric{Name: name, StartMilliseconds: nonNegativeFloat(paint.StartMilliseconds)})
	}
	return out
}

func sanitizeLongTaskMetrics(raw LongTaskMetrics) LongTaskMetrics {
	if raw.Count < 0 {
		raw.Count = 0
	}
	raw.TotalMilliseconds = nonNegativeFloat(raw.TotalMilliseconds)
	raw.MaxMilliseconds = nonNegativeFloat(raw.MaxMilliseconds)
	return raw
}

func sanitizeMetricResources(raw []LargestResourceEntry, opts MetricsOptions) (ResourceMetrics, []LargestResourceEntry) {
	opts = normalizeMetricsOptions(opts)
	aggregate := ResourceMetrics{
		TotalCount:     len(raw),
		ByResourceType: map[string]int{},
	}
	matched := make([]LargestResourceEntry, 0, len(raw))
	for _, entry := range raw {
		entry.ResourceType = classifyMetricsResourceType(entry)
		entry.InitiatorType = strings.ToLower(TruncateBytes(RedactString(entry.InitiatorType), 80))
		if !metricResourceMatchesFilter(entry, opts.Filter) {
			continue
		}
		aggregate.MatchedCount++
		aggregate.TransferSizeBytes += nonNegativeInt64(entry.TransferSizeBytes)
		aggregate.EncodedBodySizeBytes += nonNegativeInt64(entry.EncodedBodySizeBytes)
		aggregate.DecodedBodySizeBytes += nonNegativeInt64(entry.DecodedBodySizeBytes)
		aggregate.DurationMilliseconds += nonNegativeFloat(entry.DurationMilliseconds)
		aggregate.ByResourceType[entry.ResourceType]++
		entry.URL = RedactURL(entry.URL)
		entry.StartMilliseconds = nonNegativeFloat(entry.StartMilliseconds)
		entry.DurationMilliseconds = nonNegativeFloat(entry.DurationMilliseconds)
		entry.TransferSizeBytes = nonNegativeInt64(entry.TransferSizeBytes)
		entry.EncodedBodySizeBytes = nonNegativeInt64(entry.EncodedBodySizeBytes)
		entry.DecodedBodySizeBytes = nonNegativeInt64(entry.DecodedBodySizeBytes)
		matched = append(matched, entry)
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransferSizeBytes == matched[j].TransferSizeBytes {
			return matched[i].DurationMilliseconds > matched[j].DurationMilliseconds
		}
		return matched[i].TransferSizeBytes > matched[j].TransferSizeBytes
	})
	limit := minInt(opts.LimitResources, len(matched))
	largest := make([]LargestResourceEntry, 0, limit)
	for i := 0; i < limit; i++ {
		entry := matched[i]
		entry.Index = i
		largest = append(largest, entry)
	}
	aggregate.ReturnedCount = len(largest)
	if len(aggregate.ByResourceType) == 0 {
		aggregate.ByResourceType = nil
	}
	return aggregate, largest
}

func metricResourceMatchesFilter(entry LargestResourceEntry, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		entry.URL,
		entry.ResourceType,
		entry.InitiatorType,
	}, "\n"))
	return strings.Contains(haystack, filter)
}

func classifyMetricsResourceType(entry LargestResourceEntry) string {
	return classifyNetworkResourceType(NetworkEntry{
		URL:           entry.URL,
		InitiatorType: entry.InitiatorType,
		ResourceType:  entry.ResourceType,
	})
}

func metricsExpression() string {
	return `(function () {
  const number = (value) => Number(value || 0);
  const nav = performance && performance.getEntriesByType ? performance.getEntriesByType("navigation")[0] : null;
  const paints = performance && performance.getEntriesByType ? performance.getEntriesByType("paint") : [];
  const resources = performance && performance.getEntriesByType ? performance.getEntriesByType("resource") : [];
  const longTasks = performance && performance.getEntriesByType ? performance.getEntriesByType("longtask") : [];
  let totalLongTask = 0;
  let maxLongTask = 0;
  for (const task of longTasks) {
    const duration = number(task.duration);
    totalLongTask += duration;
    maxLongTask = Math.max(maxLongTask, duration);
  }
  return {
    navigation: nav ? {
      type: String(nav.type || ""),
      start_time_ms: number(nav.startTime),
      duration_ms: number(nav.duration),
      dom_content_loaded_ms: number(nav.domContentLoadedEventEnd),
      load_event_ms: number(nav.loadEventEnd),
      response_start_ms: number(nav.responseStart),
      transfer_size_bytes: number(nav.transferSize),
      encoded_body_size_bytes: number(nav.encodedBodySize),
      decoded_body_size_bytes: number(nav.decodedBodySize)
    } : {},
    paints: Array.from(paints).map(entry => ({
      name: String(entry.name || ""),
      start_time_ms: number(entry.startTime)
    })),
    resources: Array.from(resources).map((entry, index) => ({
      index,
      url: String(entry.name || ""),
      resource_type: String(entry.initiatorType || ""),
      initiator_type: String(entry.initiatorType || ""),
      start_time_ms: number(entry.startTime),
      duration_ms: number(entry.duration),
      transfer_size_bytes: number(entry.transferSize),
      encoded_body_size_bytes: number(entry.encodedBodySize),
      decoded_body_size_bytes: number(entry.decodedBodySize)
    })),
    dom: {
      node_count: document && document.querySelectorAll ? document.querySelectorAll("*").length : 0
    },
    long_tasks: {
      count: longTasks.length,
      total_ms: totalLongTask,
      max_ms: maxLongTask
    }
  };
})()`
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
