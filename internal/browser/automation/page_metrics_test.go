package automation

import (
	"strings"
	"testing"
)

func TestNormalizeMetricsOptionsDefaultsAndCapsLimit(t *testing.T) {
	opts := normalizeMetricsOptions(MetricsOptions{})
	if opts.LimitResources != 10 {
		t.Fatalf("default limit = %d", opts.LimitResources)
	}
	opts = normalizeMetricsOptions(MetricsOptions{LimitResources: 500})
	if opts.LimitResources != 100 {
		t.Fatalf("capped limit = %d", opts.LimitResources)
	}
	opts = normalizeMetricsOptions(MetricsOptions{LimitResources: 3})
	if opts.LimitResources != 3 {
		t.Fatalf("explicit limit = %d", opts.LimitResources)
	}
}

func TestSanitizeMetricResourcesAggregatesFiltersLimitsAndRedacts(t *testing.T) {
	raw := []LargestResourceEntry{
		{URL: "https://intranet.test/static/app.js", InitiatorType: "script", TransferSizeBytes: 100, DurationMilliseconds: 20},
		{URL: "https://intranet.test/api/me?access_token=secret", InitiatorType: "fetch", TransferSizeBytes: 300, DurationMilliseconds: 10},
		{URL: "https://intranet.test/api/orders?code=abc", InitiatorType: "xmlhttprequest", TransferSizeBytes: 200, DurationMilliseconds: 40},
	}
	aggregate, largest := sanitizeMetricResources(raw, MetricsOptions{Filter: "/api/", LimitResources: 1})
	if aggregate.TotalCount != 3 || aggregate.MatchedCount != 2 || aggregate.ReturnedCount != 1 {
		t.Fatalf("unexpected aggregate: %#v", aggregate)
	}
	if aggregate.TransferSizeBytes != 500 || aggregate.ByResourceType["fetch"] != 1 || aggregate.ByResourceType["xmlhttprequest"] != 1 {
		t.Fatalf("aggregate sizes/types wrong: %#v", aggregate)
	}
	if len(largest) != 1 || largest[0].TransferSizeBytes != 300 {
		t.Fatalf("largest resources not sorted/limited: %#v", largest)
	}
	joined := largest[0].URL
	for _, leaked := range []string{"access_token=secret", "code=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("resource URL leaked %q in %#v", leaked, largest)
		}
	}
}

func TestSanitizeMetricsClampsNegativeValues(t *testing.T) {
	nav := sanitizeNavigationMetrics(NavigationMetrics{Type: "Navigate", DurationMilliseconds: -1, TransferSizeBytes: -5})
	if nav.Type != "navigate" || nav.DurationMilliseconds != 0 || nav.TransferSizeBytes != 0 {
		t.Fatalf("navigation not sanitized: %#v", nav)
	}
	longTasks := sanitizeLongTaskMetrics(LongTaskMetrics{Count: -1, TotalMilliseconds: -2, MaxMilliseconds: -3})
	if longTasks.Count != 0 || longTasks.TotalMilliseconds != 0 || longTasks.MaxMilliseconds != 0 {
		t.Fatalf("long tasks not sanitized: %#v", longTasks)
	}
}
