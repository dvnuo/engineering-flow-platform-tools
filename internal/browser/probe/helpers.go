package probe

import (
	"net/url"
	"path/filepath"
	"strings"
)

func ArtifactPaths(outDir string, saveHTML, saveScreenshot, fetchAPI bool) ProbeFiles {
	files := ProbeFiles{
		Network: filepath.Join(outDir, "network.json"),
		Summary: filepath.Join(outDir, "summary.json"),
	}
	if saveScreenshot {
		files.Screenshot = filepath.Join(outDir, "screenshot.png")
	}
	if saveHTML {
		files.HTML = filepath.Join(outDir, "page.html")
	}
	if fetchAPI {
		files.FetchAPI = filepath.Join(outDir, "fetch_api_result.json")
	}
	return files
}

func FilterAPIEvents(events []NetworkEvent, filter string, max int) []NetworkEvent {
	if max <= 0 {
		max = 1000
	}
	filterLower := strings.ToLower(filter)
	out := make([]NetworkEvent, 0)
	for _, event := range events {
		resourceType := strings.ToLower(event.ResourceType)
		urlLower := strings.ToLower(event.URL)
		match := resourceType == "xhr" || resourceType == "fetch" ||
			strings.Contains(urlLower, "/api/") ||
			strings.Contains(urlLower, "/graphql") ||
			strings.Contains(urlLower, "/rest/")
		if filterLower != "" && strings.Contains(urlLower, filterLower) {
			match = true
		}
		if !match {
			continue
		}
		out = append(out, event)
		if len(out) >= max {
			break
		}
	}
	return out
}

func ClassifyAuthIndicators(inputURL, finalURL, title, bodyPreview string, selectorFound bool, events []NetworkEvent) AuthIndicators {
	indicators := AuthIndicators{SelectorFound: selectorFound}
	for _, event := range events {
		if u, err := url.Parse(event.URL); err == nil {
			host := strings.ToLower(u.Hostname())
			if strings.Contains(host, "login.microsoftonline.com") || strings.Contains(host, "login.windows.net") {
				indicators.MicrosoftLoginSeen = true
			}
		}
		if event.Kind == "response" {
			if event.Status == 401 {
				indicators.Negotiate401Seen = true
			}
			if event.Status >= 300 && event.Status <= 399 {
				indicators.RedirectSeen = true
			}
		}
	}
	combined := strings.ToLower(finalURL + " " + title + " " + bodyPreview)
	for _, signal := range []string{"login", "sign in", "signin", "password", "username", "mfa", "multi-factor", "authenticate"} {
		if strings.Contains(combined, signal) {
			indicators.LoginPageLikely = true
			break
		}
	}
	indicators.BusinessPageLikely = selectorFound || (!indicators.LoginPageLikely && sameHost(inputURL, finalURL) && hasOKResponse(events))
	return indicators
}

func sameHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return false
	}
	return ua.Hostname() != "" && strings.EqualFold(ua.Hostname(), ub.Hostname())
}

func hasOKResponse(events []NetworkEvent) bool {
	for _, event := range events {
		if event.Kind == "response" && event.Status == 200 {
			return true
		}
	}
	return false
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit]
}
