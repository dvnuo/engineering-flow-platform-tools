package automation

import (
	"strings"
	"testing"
)

func TestSanitizeNetworkEntriesDefaultsToAPILikeAndRedactsURLs(t *testing.T) {
	raw := []NetworkEntry{
		{URL: "https://intranet.test/static/app.js", InitiatorType: "script", DurationMilliseconds: 10},
		{URL: "https://intranet.test/api/me?access_token=secret", InitiatorType: "fetch", TransferSizeBytes: 123},
		{URL: "https://intranet.test/graphql?code=abc", InitiatorType: "other"},
	}
	got, count := sanitizeNetworkEntries(raw, NetworkOptions{Limit: 10})
	if count != 2 || len(got) != 2 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	joined := got[0].URL + got[1].URL
	for _, leaked := range []string{"access_token=secret", "code=abc"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("network URL leaked %q in %#v", leaked, got)
		}
	}
	for _, entry := range got {
		if !entry.APILike {
			t.Fatalf("entry should be API-like: %#v", entry)
		}
	}
}

func TestSanitizeNetworkEntriesAllIncludesStaticResourcesAndLimit(t *testing.T) {
	raw := []NetworkEntry{
		{URL: "https://intranet.test/static/app.js", InitiatorType: "script"},
		{URL: "https://intranet.test/static/app.css", InitiatorType: "css"},
		{URL: "https://intranet.test/api/me", InitiatorType: "fetch"},
	}
	got, count := sanitizeNetworkEntries(raw, NetworkOptions{Limit: 2, All: true})
	if count != 3 || len(got) != 2 {
		t.Fatalf("count=%d entries=%#v", count, got)
	}
	if got[0].ResourceType != "script" || got[1].ResourceType != "css" {
		t.Fatalf("resource types were not preserved: %#v", got)
	}
}

func TestNetworkEntryMatchesFilterUsesURLTypeAndAPIMarker(t *testing.T) {
	entry := NetworkEntry{
		URL:           "https://intranet.test/api/me",
		InitiatorType: "fetch",
		ResourceType:  "fetch",
		APILike:       true,
	}
	for _, filter := range []string{"api/me", "FETCH", "true"} {
		if !networkEntryMatchesFilter(entry, filter) {
			t.Fatalf("filter %q did not match %#v", filter, entry)
		}
	}
	if networkEntryMatchesFilter(entry, "stylesheet") {
		t.Fatalf("unexpected stylesheet match")
	}
}
