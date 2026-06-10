package automation

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/chromedp/chromedp"
)

type NetworkOptions struct {
	PageOptions
	Filter string
	Limit  int
	All    bool
}

type NetworkEntry struct {
	Index                 int     `json:"index"`
	URL                   string  `json:"url"`
	InitiatorType         string  `json:"initiator_type,omitempty"`
	ResourceType          string  `json:"resource_type,omitempty"`
	StartTimeMilliseconds float64 `json:"start_time_ms"`
	DurationMilliseconds  float64 `json:"duration_ms"`
	TransferSizeBytes     int64   `json:"transfer_size_bytes,omitempty"`
	EncodedBodySizeBytes  int64   `json:"encoded_body_size_bytes,omitempty"`
	DecodedBodySizeBytes  int64   `json:"decoded_body_size_bytes,omitempty"`
	APILike               bool    `json:"api_like"`
}

type NetworkResult struct {
	Session  string         `json:"session"`
	TargetID string         `json:"target_id"`
	URL      string         `json:"url"`
	Title    string         `json:"title"`
	Filter   string         `json:"filter,omitempty"`
	Limit    int            `json:"limit"`
	All      bool           `json:"all"`
	Count    int            `json:"count"`
	Entries  []NetworkEntry `json:"entries"`
}

func (m *Manager) Network(ctx context.Context, opts NetworkOptions) (NetworkResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	pageCtx, cancel, session, target, err := m.attachPage(ctx, opts.PageOptions)
	if err != nil {
		return NetworkResult{}, err
	}
	defer cancel()

	var finalURL, title string
	var raw []NetworkEntry
	if err := chromedp.Run(pageCtx,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.Evaluate(networkEntriesExpression(), &raw, chromedp.EvalAsValue),
	); err != nil {
		return NetworkResult{}, mapPageError(err, "automation_failed")
	}
	entries, count := sanitizeNetworkEntries(raw, opts)
	return NetworkResult{
		Session:  session.Name,
		TargetID: target.ID,
		URL:      RedactURL(finalURL),
		Title:    RedactString(title),
		Filter:   RedactString(opts.Filter),
		Limit:    opts.Limit,
		All:      opts.All,
		Count:    count,
		Entries:  entries,
	}, nil
}

func sanitizeNetworkEntries(raw []NetworkEntry, opts NetworkOptions) ([]NetworkEntry, int) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	out := make([]NetworkEntry, 0, minInt(limit, len(raw)))
	count := 0
	for _, entry := range raw {
		entry.InitiatorType = strings.ToLower(strings.TrimSpace(entry.InitiatorType))
		entry.ResourceType = classifyNetworkResourceType(entry)
		entry.APILike = looksLikeAPIResource(entry)
		if !opts.All && !entry.APILike {
			continue
		}
		if !networkEntryMatchesFilter(entry, opts.Filter) {
			continue
		}
		count++
		if len(out) >= limit {
			continue
		}
		entry.Index = count - 1
		entry.URL = RedactURL(entry.URL)
		entry.InitiatorType = RedactString(entry.InitiatorType)
		entry.ResourceType = RedactString(entry.ResourceType)
		out = append(out, entry)
	}
	return out, count
}

func networkEntryMatchesFilter(entry NetworkEntry, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		entry.URL,
		entry.InitiatorType,
		entry.ResourceType,
		strconv.FormatBool(entry.APILike),
	}, "\n"))
	return strings.Contains(haystack, filter)
}

func looksLikeAPIResource(entry NetworkEntry) bool {
	initiator := strings.ToLower(strings.TrimSpace(entry.InitiatorType))
	switch initiator {
	case "fetch", "xmlhttprequest", "beacon":
		return true
	}
	rawURL := strings.TrimSpace(entry.URL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil {
		return false
	}
	lowerPath := strings.ToLower(parsed.EscapedPath())
	for _, marker := range []string{"/api/", "/apis/", "/graphql", "/rest/", "/rpc/", "/ajax/", "/v1/", "/v2/", "/v3/"} {
		if strings.Contains(lowerPath, marker) {
			return true
		}
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if ext == ".json" || ext == ".ndjson" {
		return true
	}
	query := strings.ToLower(parsed.RawQuery)
	return strings.Contains(query, "format=json") || strings.Contains(query, "output=json")
}

func classifyNetworkResourceType(entry NetworkEntry) string {
	initiator := strings.ToLower(strings.TrimSpace(entry.InitiatorType))
	switch initiator {
	case "fetch", "xmlhttprequest", "beacon", "script", "css", "img", "image", "link", "iframe", "navigation":
		return initiator
	}
	parsed, err := url.Parse(strings.TrimSpace(entry.URL))
	if err != nil || parsed == nil {
		return "resource"
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".js", ".mjs":
		return "script"
	case ".css":
		return "css"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico":
		return "image"
	case ".woff", ".woff2", ".ttf", ".otf":
		return "font"
	case ".mp4", ".webm", ".mp3", ".wav":
		return "media"
	case ".json", ".ndjson":
		return "fetch"
	default:
		return "resource"
	}
}

func networkEntriesExpression() string {
	return `(function () {
  const entries = performance && performance.getEntriesByType ? performance.getEntriesByType("resource") : [];
  return entries.map((entry, index) => ({
    index,
    url: String(entry.name || ""),
    initiator_type: String(entry.initiatorType || ""),
    resource_type: String(entry.initiatorType || ""),
    start_time_ms: Number(entry.startTime || 0),
    duration_ms: Number(entry.duration || 0),
    transfer_size_bytes: Number(entry.transferSize || 0),
    encoded_body_size_bytes: Number(entry.encodedBodySize || 0),
    decoded_body_size_bytes: Number(entry.decodedBodySize || 0),
    api_like: false
  }));
})()`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
