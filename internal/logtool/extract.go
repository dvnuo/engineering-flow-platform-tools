package logtool

import (
	"regexp"
	"sort"
	"strings"
)

var stacktraceMarkers = []*regexp.Regexp{
	regexp.MustCompile(`(?m)\bException\b`),
	regexp.MustCompile(`(?m)\bTraceback\b`),
	regexp.MustCompile(`(?m)\bpanic:`),
	regexp.MustCompile(`(?m)\bgoroutine\s+\d+`),
	regexp.MustCompile(`(?m)^\s+at\s+[A-Za-z0-9_.$/]+\(`),
}

func Extract(runDir string, kind string, limit int) (ExtractResult, error) {
	if _, err := ReadManifest(runDir); err != nil {
		return ExtractResult{}, err
	}
	n, err := normalizeLimit(limit, defaultSearchLimit)
	if err != nil {
		return ExtractResult{}, err
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return ExtractResult{}, err
	}
	templateByID := map[string]Template{}
	for _, tpl := range templates {
		templateByID[tpl.TemplateID] = tpl
	}
	switch kind {
	case "stacktrace":
		return extractStacktraces(runDir, templateByID, n)
	case "error-signature":
		return extractErrorSignatures(runDir, templateByID, n)
	default:
		return ExtractResult{}, NewError("invalid_args", "--kind must be stacktrace or error-signature.", "Run log schema extract --json.", 400)
	}
}

func extractStacktraces(runDir string, templateByID map[string]Template, limit int) (ExtractResult, error) {
	acc := map[string]*ExtractItem{}
	err := ReadEntries(runDir, func(entry Entry) error {
		if !looksLikeStacktrace(entry.MessagePreview) {
			return nil
		}
		item := acc[entry.TemplateID]
		if item == nil {
			tpl := templateByID[entry.TemplateID]
			item = &ExtractItem{
				TemplateID:            entry.TemplateID,
				Template:              tpl.Template,
				RepresentativeEntryID: entry.EntryID,
				RepresentativeText:    entry.MessagePreview,
				Levels:                map[string]int{},
				EvidenceRefs:          []string{entry.EntryID},
			}
			acc[entry.TemplateID] = item
		}
		item.Count++
		if entry.Level != "" {
			item.Levels[entry.Level]++
		}
		if len(item.EvidenceRefs) < 5 && !containsString(item.EvidenceRefs, entry.EntryID) {
			item.EvidenceRefs = append(item.EvidenceRefs, entry.EntryID)
		}
		return nil
	})
	if err != nil {
		return ExtractResult{}, err
	}
	items := extractItems(acc)
	if len(items) > limit {
		items = items[:limit]
	}
	return ExtractResult{Kind: "stacktrace", Items: items}, nil
}

func extractErrorSignatures(runDir string, templateByID map[string]Template, limit int) (ExtractResult, error) {
	acc := map[string]*ExtractItem{}
	err := ReadEntries(runDir, func(entry Entry) error {
		if !isErrorLevel(entry.Level) {
			return nil
		}
		item := acc[entry.TemplateID]
		if item == nil {
			tpl := templateByID[entry.TemplateID]
			item = &ExtractItem{
				TemplateID:            entry.TemplateID,
				Template:              tpl.Template,
				RepresentativeEntryID: entry.EntryID,
				RepresentativeText:    entry.MessagePreview,
				Levels:                map[string]int{},
				EvidenceRefs:          []string{entry.EntryID},
			}
			acc[entry.TemplateID] = item
		}
		item.Count++
		if entry.Level != "" {
			item.Levels[entry.Level]++
		}
		if len(item.EvidenceRefs) < 5 && !containsString(item.EvidenceRefs, entry.EntryID) {
			item.EvidenceRefs = append(item.EvidenceRefs, entry.EntryID)
		}
		return nil
	})
	if err != nil {
		return ExtractResult{}, err
	}
	items := extractItems(acc)
	if len(items) > limit {
		items = items[:limit]
	}
	return ExtractResult{Kind: "error-signature", Items: items}, nil
}

func looksLikeStacktrace(text string) bool {
	if !strings.Contains(text, "\n") {
		return false
	}
	for _, marker := range stacktraceMarkers {
		if marker.MatchString(text) {
			return true
		}
	}
	return false
}

func extractItems(acc map[string]*ExtractItem) []ExtractItem {
	items := make([]ExtractItem, 0, len(acc))
	for _, item := range acc {
		item.RepresentativeText = Redact(item.RepresentativeText)
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].TemplateID < items[j].TemplateID
		}
		return items[i].Count > items[j].Count
	})
	return items
}
