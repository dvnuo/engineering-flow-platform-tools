package logtool

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultSearchLimit = 20
	defaultListLimit   = 50
	maxResultLimit     = 200
)

type cursorState struct {
	Offset     int    `json:"offset"`
	Query      string `json:"query,omitempty"`
	Regex      bool   `json:"regex,omitempty"`
	Level      string `json:"level,omitempty"`
	Service    string `json:"service,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
}

func Search(runDir string, opts SearchOptions) (SearchResult, error) {
	if _, err := ReadManifest(runDir); err != nil {
		return SearchResult{}, err
	}
	limit, err := normalizeLimit(opts.Limit, defaultSearchLimit)
	if err != nil {
		return SearchResult{}, err
	}
	offset, err := applySearchCursor(&opts)
	if err != nil {
		return SearchResult{}, err
	}
	var re *regexp.Regexp
	if opts.Regex {
		re, err = regexp.Compile(opts.Query)
		if err != nil {
			return SearchResult{}, NewError("invalid_args", "Search regex is invalid.", "Fix --query or remove --regex.", 400)
		}
	}
	level := NormalizeLevel(opts.Level)
	if opts.Level != "" && level == "" {
		return SearchResult{}, NewError("invalid_args", "--level is invalid.", "Use TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.", 400)
	}
	if strings.TrimSpace(opts.Query) == "" {
		return SearchResult{}, NewError("invalid_args", "--query or --cursor is required.", "Pass --query <text>, or pass --cursor from a previous search response.", 400)
	}
	since, err := parseOptionalTime(opts.Since, "--since")
	if err != nil {
		return SearchResult{}, err
	}
	until, err := parseOptionalTime(opts.Until, "--until")
	if err != nil {
		return SearchResult{}, err
	}
	var items []SearchItem
	total := 0
	err = ReadEntries(runDir, func(entry Entry) error {
		if !entryMatchesFilters(entry, level, opts.TemplateID, since, until) {
			return nil
		}
		if opts.Service != "" && !strings.EqualFold(entry.Service, opts.Service) {
			return nil
		}
		if !queryMatches(entry, opts.Query, re) {
			return nil
		}
		total++
		if total <= offset {
			return nil
		}
		if len(items) < limit {
			items = append(items, searchItem(entry))
		}
		return nil
	})
	if err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Query: opts.Query, Matches: total, Items: items}
	if total > offset+len(items) {
		result.NextCursor = encodeSearchCursor(offset+len(items), opts)
	}
	return result, nil
}

func Entries(runDir string, opts EntryListOptions) (EntryListResult, error) {
	if _, err := ReadManifest(runDir); err != nil {
		return EntryListResult{}, err
	}
	limit, err := normalizeLimit(opts.Limit, defaultListLimit)
	if err != nil {
		return EntryListResult{}, err
	}
	offset, err := decodeCursor(opts.Cursor)
	if err != nil {
		return EntryListResult{}, err
	}
	level := NormalizeLevel(opts.Level)
	if opts.Level != "" && level == "" {
		return EntryListResult{}, NewError("invalid_args", "--level is invalid.", "Use TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.", 400)
	}
	var items []Entry
	total := 0
	err = ReadEntries(runDir, func(entry Entry) error {
		if !entryMatchesFilters(entry, level, opts.TemplateID, time.Time{}, time.Time{}) {
			return nil
		}
		total++
		if total <= offset {
			return nil
		}
		if len(items) < limit {
			items = append(items, entry)
		}
		return nil
	})
	if err != nil {
		return EntryListResult{}, err
	}
	result := EntryListResult{Items: items}
	if total > offset+len(items) {
		result.NextCursor = encodeCursor(offset + len(items))
	}
	return result, nil
}

func Profile(runDir string) (ProfileResult, error) {
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return ProfileResult{}, err
	}
	levels := map[string]int{}
	if err := ReadEntries(runDir, func(entry Entry) error {
		if entry.Level != "" {
			levels[entry.Level]++
		}
		return nil
	}); err != nil {
		return ProfileResult{}, err
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return ProfileResult{}, err
	}
	sortTemplates(templates, "count")
	if len(templates) > 10 {
		templates = templates[:10]
	}
	return ProfileResult{
		RunID:          manifest.RunID,
		Sources:        manifest.Sources,
		TotalLines:     manifest.TotalLines,
		EntriesCount:   manifest.EntriesCount,
		TemplatesCount: manifest.TemplatesCount,
		Levels:         levels,
		TopTemplates:   templates,
		TimeRange:      manifest.TimeRange,
		Truncated:      manifest.Truncated,
	}, nil
}

func Templates(runDir, only, sortBy string, limit int) (TemplatesResult, error) {
	if _, err := ReadManifest(runDir); err != nil {
		return TemplatesResult{}, err
	}
	if only == "" {
		only = "all"
	}
	if sortBy == "" {
		sortBy = "count"
	}
	switch only {
	case "all", "non-info":
	default:
		return TemplatesResult{}, NewError("invalid_args", "--only must be all or non-info.", "Run log schema templates --json.", 400)
	}
	switch sortBy {
	case "count", "first_seen", "last_seen":
	default:
		return TemplatesResult{}, NewError("invalid_args", "--sort must be count, first_seen, or last_seen.", "Run log schema templates --json.", 400)
	}
	n, err := normalizeLimit(limit, defaultListLimit)
	if err != nil {
		return TemplatesResult{}, err
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return TemplatesResult{}, err
	}
	var filtered []Template
	for _, tpl := range templates {
		if only == "non-info" && templateIsInfo(tpl) {
			continue
		}
		filtered = append(filtered, tpl)
	}
	sortTemplates(filtered, sortBy)
	if len(filtered) > n {
		filtered = filtered[:n]
	}
	return TemplatesResult{Templates: filtered}, nil
}

func sortTemplates(templates []Template, sortBy string) {
	sort.SliceStable(templates, func(i, j int) bool {
		a, b := templates[i], templates[j]
		switch sortBy {
		case "first_seen":
			if a.FirstSeen == b.FirstSeen {
				return a.TemplateID < b.TemplateID
			}
			if a.FirstSeen == "" {
				return false
			}
			if b.FirstSeen == "" {
				return true
			}
			return a.FirstSeen < b.FirstSeen
		case "last_seen":
			if a.LastSeen == b.LastSeen {
				return a.TemplateID < b.TemplateID
			}
			return a.LastSeen > b.LastSeen
		default:
			if a.Count == b.Count {
				return a.TemplateID < b.TemplateID
			}
			return a.Count > b.Count
		}
	})
}

func templateIsInfo(t Template) bool {
	if t.GoldenSignal != "" && t.GoldenSignal != "information" {
		return false
	}
	for level := range t.Levels {
		switch level {
		case "ERROR", "FATAL", "PANIC", "WARN":
			return false
		}
	}
	return true
}

func normalizeLimit(limit, def int) (int, error) {
	if limit == 0 {
		limit = def
	}
	if limit < 0 || limit > maxResultLimit {
		return 0, NewError("invalid_args", "--limit must be between 1 and 200.", "Use a bounded limit no greater than 200.", 400)
	}
	if limit == 0 {
		limit = def
	}
	return limit, nil
}

func decodeCursor(cursor string) (int, error) {
	state, err := decodeCursorState(cursor)
	return state.Offset, err
}

func decodeCursorState(cursor string) (cursorState, error) {
	if strings.TrimSpace(cursor) == "" {
		return cursorState{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorState{}, NewError("invalid_args", "--cursor is invalid.", "Use next_cursor returned by the previous response.", 400)
	}
	var state cursorState
	if err := json.Unmarshal(b, &state); err != nil || state.Offset < 0 {
		return cursorState{}, NewError("invalid_args", "--cursor is invalid.", "Use next_cursor returned by the previous response.", 400)
	}
	return state, nil
}

func encodeCursor(offset int) string {
	b, _ := json.Marshal(cursorState{Offset: offset})
	return base64.RawURLEncoding.EncodeToString(b)
}

func applySearchCursor(opts *SearchOptions) (int, error) {
	state, err := decodeCursorState(opts.Cursor)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(opts.Cursor) == "" {
		return 0, nil
	}
	if opts.Query == "" {
		opts.Query = state.Query
	}
	if !opts.Regex {
		opts.Regex = state.Regex
	}
	if opts.Level == "" {
		opts.Level = state.Level
	}
	if opts.Service == "" {
		opts.Service = state.Service
	}
	if opts.TemplateID == "" {
		opts.TemplateID = state.TemplateID
	}
	if opts.Since == "" {
		opts.Since = state.Since
	}
	if opts.Until == "" {
		opts.Until = state.Until
	}
	return state.Offset, nil
}

func encodeSearchCursor(offset int, opts SearchOptions) string {
	b, _ := json.Marshal(cursorState{
		Offset:     offset,
		Query:      opts.Query,
		Regex:      opts.Regex,
		Level:      opts.Level,
		Service:    opts.Service,
		TemplateID: opts.TemplateID,
		Since:      opts.Since,
		Until:      opts.Until,
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

func entryMatchesFilters(entry Entry, level, templateID string, since, until time.Time) bool {
	if level != "" && NormalizeLevel(entry.Level) != level {
		return false
	}
	if templateID != "" && entry.TemplateID != templateID {
		return false
	}
	if !since.IsZero() || !until.IsZero() {
		if entry.Timestamp == "" {
			return false
		}
		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return false
		}
		if !since.IsZero() && ts.Before(since) {
			return false
		}
		if !until.IsZero() && ts.After(until) {
			return false
		}
	}
	return true
}

func queryMatches(entry Entry, query string, re *regexp.Regexp) bool {
	if strings.TrimSpace(query) == "" {
		return true
	}
	haystack := entry.Level + " " + entry.Service + " " + entry.MessagePreview + " " + entry.TemplateID + " " + strings.Join(entry.Tags, " ")
	if re != nil {
		return re.MatchString(haystack)
	}
	parts := regexp.MustCompile(`(?i)\s+OR\s+`).Split(query, -1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(strings.ToLower(haystack), strings.ToLower(part)) {
			return true
		}
	}
	return false
}

func searchItem(entry Entry) SearchItem {
	return SearchItem{
		EntryID:        entry.EntryID,
		TemplateID:     entry.TemplateID,
		Timestamp:      entry.Timestamp,
		Level:          entry.Level,
		MessagePreview: entry.MessagePreview,
		Source: SourceLocation{
			Path:      entry.SourcePath,
			LineStart: entry.LineStart,
			LineEnd:   entry.LineEnd,
			ByteStart: entry.ByteStart,
			ByteEnd:   entry.ByteEnd,
		},
		EvidenceRef: entry.EntryID,
	}
}

func parseOptionalTime(value, flag string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, NewError("invalid_args", flag+" timestamp is invalid.", "Use RFC3339 such as 2026-06-03T10:00:00Z.", 400)
}
