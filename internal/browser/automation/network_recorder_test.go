package automation

import (
	"strings"
	"testing"
)

func TestNetworkArtifactPathUsesSafeTargetFile(t *testing.T) {
	store := NewStore(t.TempDir())
	path, err := store.NetworkArtifactPath("default", "../target:id with spaces")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "network") || strings.Contains(path, "..") || strings.Contains(path, " ") || !strings.HasSuffix(path, ".json") {
		t.Fatalf("unsafe network artifact path: %s", path)
	}
}

func TestSanitizeNetworkRecordEntriesRedactsAndFilters(t *testing.T) {
	raw := []rawNetworkRecordEntry{
		{
			ID:                   "fetch-1",
			URL:                  "https://intranet.test/api/me?access_token=secret",
			Method:               "post",
			Status:               200,
			ResourceType:         "fetch",
			InitiatorType:        "fetch",
			StartedAtUnixMS:      1717200000000,
			EndedAtUnixMS:        1717200000200,
			DurationMilliseconds: 200,
			TransferSizeBytes:    123,
			Source:               "fetch",
			BodyPreview:          `{"token":"secret","name":"Ada"}`,
			BodyLength:           len(`{"token":"secret","name":"Ada"}`),
			BodyCaptured:         true,
		},
		{
			ID:            "res-1",
			URL:           "https://intranet.test/static/app.js",
			Method:        "",
			Status:        0,
			ResourceType:  "script",
			InitiatorType: "script",
			Source:        "resource_timing",
		},
	}
	got, count := sanitizeNetworkRecordEntries(raw, NetworkRecorderOptions{Filter: "/api/", Method: "POST", Status: 200, Limit: 10, Body: true, MaxBodyBytes: 20000})
	if count != 1 || len(got) != 1 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	entry := got[0]
	if entry.Method != "POST" || entry.Status != 200 || entry.StartedAt == "" || entry.EndedAt == "" {
		t.Fatalf("entry not normalized: %#v", entry)
	}
	if strings.Contains(entry.URL, "access_token=secret") {
		t.Fatalf("network URL leaked token: %#v", entry)
	}
	if !entry.BodyCaptured || strings.Contains(entry.BodyPreview, "secret") || !strings.Contains(entry.BodyPreview, "Ada") {
		t.Fatalf("network body preview was not redacted: %#v", entry)
	}
}

func TestSanitizeNetworkRecordEntriesAppliesLimitAfterCount(t *testing.T) {
	raw := []rawNetworkRecordEntry{
		{URL: "https://intranet.test/api/1", Method: "GET", Status: 200},
		{URL: "https://intranet.test/api/2", Method: "GET", Status: 201},
		{URL: "https://intranet.test/api/3", Method: "GET", Status: 202},
	}
	got, count := sanitizeNetworkRecordEntries(raw, NetworkRecorderOptions{Limit: 2})
	if count != 3 || len(got) != 2 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	if got[0].Index != 0 || got[1].Index != 1 {
		t.Fatalf("indexes not assigned: %#v", got)
	}
}

func TestNetworkRecordWaitMatchesURLMethodAndStatus(t *testing.T) {
	entry := NetworkRecordEntry{
		URL:    RedactURL("https://intranet.test/api/me?code=abc"),
		Method: "GET",
		Status: 200,
	}
	if !networkRecordWaitMatches(entry, NetworkWaitOptions{
		NetworkRecorderOptions: NetworkRecorderOptions{Method: "get", Status: 200},
		URLContains:            "/api/me",
	}) {
		t.Fatalf("expected wait match")
	}
	if networkRecordWaitMatches(entry, NetworkWaitOptions{
		NetworkRecorderOptions: NetworkRecorderOptions{Method: "POST", Status: 200},
		URLContains:            "/api/me",
	}) {
		t.Fatalf("unexpected method match")
	}
	if networkRecordWaitMatches(entry, NetworkWaitOptions{
		NetworkRecorderOptions: NetworkRecorderOptions{Method: "GET", Status: 404},
		URLContains:            "/api/me",
	}) {
		t.Fatalf("unexpected status match")
	}
}

func TestNormalizeNetworkRecorderOptionsCapsLimitAndMethod(t *testing.T) {
	opts := normalizeNetworkRecorderOptions(NetworkRecorderOptions{Limit: 10000, Method: "post", Status: -1})
	if opts.Limit != 5000 || opts.Method != "POST" || opts.Status != -1 {
		t.Fatalf("unexpected normalized opts: %#v", opts)
	}
	opts = normalizeNetworkRecorderOptions(NetworkRecorderOptions{})
	if opts.Limit != 500 || opts.Status != -1 || opts.MaxBodyBytes != 20000 {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
}
