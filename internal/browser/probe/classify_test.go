package probe

import (
	"path/filepath"
	"testing"
)

func TestClassifyAuthIndicators(t *testing.T) {
	events := []NetworkEvent{
		{Kind: "request", URL: "https://login.microsoftonline.com/common/oauth2", ResourceType: "Document"},
		{Kind: "response", URL: "https://intranet.test/app", Status: 302},
		{Kind: "response", URL: "https://intranet.test/app", Status: 401},
	}
	got := ClassifyAuthIndicators("https://intranet.test/app", "https://intranet.test/app", "Home", "signed in", true, events)
	if !got.MicrosoftLoginSeen || !got.RedirectSeen || !got.Negotiate401Seen || !got.SelectorFound || !got.BusinessPageLikely {
		t.Fatalf("unexpected indicators: %#v", got)
	}
}

func TestFilterAPIEvents(t *testing.T) {
	events := []NetworkEvent{
		{Kind: "request", URL: "https://x/assets/app.js", ResourceType: "Script"},
		{Kind: "request", URL: "https://x/api/me", ResourceType: "Fetch"},
		{Kind: "response", URL: "https://x/graphql", ResourceType: "XHR", Status: 200},
		{Kind: "response", URL: "https://x/feature", ResourceType: "Document", Status: 200},
	}
	got := FilterAPIEvents(events, "feature", 10)
	if len(got) != 3 {
		t.Fatalf("got %d api events: %#v", len(got), got)
	}
	limited := FilterAPIEvents(events, "", 1)
	if len(limited) != 1 {
		t.Fatalf("limit not applied: %#v", limited)
	}
}

func TestArtifactPaths(t *testing.T) {
	files := ArtifactPaths("result", true, true, true)
	if files.Screenshot != filepath.Join("result", "screenshot.png") ||
		files.HTML != filepath.Join("result", "page.html") ||
		files.Network != filepath.Join("result", "network.json") ||
		files.Summary != filepath.Join("result", "summary.json") ||
		files.FetchAPI != filepath.Join("result", "fetch_api_result.json") {
		t.Fatalf("unexpected files: %#v", files)
	}
}
