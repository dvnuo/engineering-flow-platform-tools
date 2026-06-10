package automation

import (
	"strings"
	"testing"
)

func TestNormalizeConsoleOptionsDefaultsCapsAndValidatesLevel(t *testing.T) {
	opts, err := normalizeConsoleOptions(ConsoleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Limit != 50 || opts.Level != "" {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
	opts, err = normalizeConsoleOptions(ConsoleOptions{Limit: 1000, Level: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Limit != 500 || opts.Level != "warning" {
		t.Fatalf("unexpected normalized opts: %#v", opts)
	}
	if _, err := normalizeConsoleOptions(ConsoleOptions{Level: "trace"}); err == nil {
		t.Fatalf("invalid level succeeded")
	}
}

func TestSanitizeConsoleEntriesRedactsAndFilters(t *testing.T) {
	raw := []rawConsoleEntry{
		{
			Level:       "warn",
			Message:     `Authorization: Bearer private {"token":"secret"}`,
			Source:      "console_api",
			TimestampMS: 1717200000000,
			URL:         "https://intranet.test/app?access_token=secret",
			Line:        12,
			Column:      5,
			Stack:       "Error at https://intranet.test/app?code=abc",
		},
		{
			Level:   "info",
			Message: "ok",
			Source:  "console_api",
		},
	}
	got, count := sanitizeConsoleEntries(raw, ConsoleOptions{Level: "warning", Limit: 10})
	if count != 1 || len(got) != 1 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	entry := got[0]
	joined := entry.Message + entry.URL + entry.Stack
	for _, leaked := range []string{"Bearer private", `"token":"secret"`, "access_token=secret", "code=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("console entry leaked %q in %#v", leaked, entry)
		}
	}
	if entry.Level != "warning" || entry.Timestamp == "" || entry.Line != 12 || entry.Column != 5 {
		t.Fatalf("entry not normalized: %#v", entry)
	}
}

func TestSanitizeConsoleEntriesAppliesLimitAfterCount(t *testing.T) {
	raw := []rawConsoleEntry{
		{Level: "error", Message: "one"},
		{Level: "error", Message: "two"},
		{Level: "error", Message: "three"},
	}
	got, count := sanitizeConsoleEntries(raw, ConsoleOptions{Level: "error", Limit: 2})
	if count != 3 || len(got) != 2 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	if got[0].Index != 0 || got[1].Index != 1 {
		t.Fatalf("indexes not assigned: %#v", got)
	}
}
