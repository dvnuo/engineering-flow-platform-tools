package logtool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Group(runDir string, opts GroupOptions) (GroupResult, error) {
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return GroupResult{}, err
	}
	by := strings.TrimSpace(opts.By)
	if by == "" {
		by = "template"
	}
	switch by {
	case "template", "error_signature", "level", "service", "time":
	default:
		return GroupResult{}, NewError("invalid_args", "--by must be template, error_signature, level, service, or time.", "Run log schema group --json.", 400)
	}
	limit, err := normalizeLimit(opts.Limit, defaultListLimit)
	if err != nil {
		return GroupResult{}, err
	}
	level := NormalizeLevel(opts.Level)
	if opts.Level != "" && level == "" {
		return GroupResult{}, NewError("invalid_args", "--level is invalid.", "Use TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.", 400)
	}
	var bucket time.Duration
	if by == "time" {
		bucket, err = parseBucket(opts.Bucket)
		if err != nil {
			return GroupResult{}, err
		}
	}
	templates, _ := ReadTemplates(runDir)
	templateByID := map[string]Template{}
	for _, tpl := range templates {
		templateByID[tpl.TemplateID] = tpl
	}
	acc := map[string]*GroupItem{}
	err = ReadEntries(runDir, func(entry Entry) error {
		if !entryMatchesFilters(entry, level, opts.TemplateID, time.Time{}, time.Time{}) || !queryMatches(entry, opts.Query, nil) {
			return nil
		}
		key, templateID := groupKey(entry, by, bucket, templateByID)
		if key == "" {
			return nil
		}
		if by == "error_signature" && !isErrorLevel(entry.Level) {
			return nil
		}
		item := acc[key]
		if item == nil {
			item = &GroupItem{
				Key:                   key,
				TemplateID:            templateID,
				Levels:                map[string]int{},
				Services:              map[string]int{},
				RepresentativeEntryID: entry.EntryID,
				EvidenceRef:           evidenceRefForKey(by, key, entry),
			}
			acc[key] = item
		}
		item.Count++
		if entry.Timestamp != "" {
			if item.FirstSeen == "" || entry.Timestamp < item.FirstSeen {
				item.FirstSeen = entry.Timestamp
			}
			if item.LastSeen == "" || entry.Timestamp > item.LastSeen {
				item.LastSeen = entry.Timestamp
			}
		}
		if entry.Level != "" {
			item.Levels[entry.Level]++
		}
		service := strings.TrimSpace(entry.Service)
		if service == "" {
			service = "(unknown)"
		}
		item.Services[service]++
		return nil
	})
	if err != nil {
		return GroupResult{}, err
	}
	groups := make([]GroupItem, 0, len(acc))
	for _, item := range acc {
		groups = append(groups, *item)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Count == groups[j].Count {
			return groups[i].Key < groups[j].Key
		}
		return groups[i].Count > groups[j].Count
	})
	if len(groups) > limit {
		groups = groups[:limit]
	}
	return GroupResult{RunID: manifest.RunID, GroupBy: by, Groups: groups}, nil
}

func Timeline(runDir string, opts TimelineOptions) (TimelineResult, error) {
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return TimelineResult{}, err
	}
	bucket, err := parseBucket(opts.Bucket)
	if err != nil {
		return TimelineResult{}, err
	}
	limit, err := normalizeLimit(opts.Limit, maxResultLimit)
	if err != nil {
		return TimelineResult{}, err
	}
	level := NormalizeLevel(opts.Level)
	if opts.Level != "" && level == "" {
		return TimelineResult{}, NewError("invalid_args", "--level is invalid.", "Use TRACE, DEBUG, INFO, WARN, ERROR, FATAL, or PANIC.", 400)
	}
	acc := map[time.Time]*TimelineBucket{}
	err = ReadEntries(runDir, func(entry Entry) error {
		if !entryMatchesFilters(entry, level, opts.TemplateID, time.Time{}, time.Time{}) || entry.Timestamp == "" {
			return nil
		}
		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return nil
		}
		start := ts.Truncate(bucket)
		item := acc[start]
		if item == nil {
			item = &TimelineBucket{
				Start:  start.UTC().Format(time.RFC3339),
				End:    start.Add(bucket).UTC().Format(time.RFC3339),
				Levels: map[string]int{},
			}
			acc[start] = item
		}
		item.Count++
		if entry.Level != "" {
			item.Levels[entry.Level]++
		}
		return nil
	})
	if err != nil {
		return TimelineResult{}, err
	}
	keys := make([]time.Time, 0, len(acc))
	total := 0
	for key, item := range acc {
		keys = append(keys, key)
		total += item.Count
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
	average := 0
	if len(keys) > 0 {
		average = total / len(keys)
	}
	series := make([]TimelineBucket, 0, len(keys))
	for _, key := range keys {
		item := *acc[key]
		if average > 0 && item.Count >= average*3 && item.Count > 1 {
			item.Spike = true
		}
		series = append(series, item)
	}
	if len(series) > limit {
		series = series[:limit]
	}
	return TimelineResult{RunID: manifest.RunID, Bucket: bucket.String(), Series: series}, nil
}

func Summarize(runDir string, opts SummaryOptions) (SummaryResult, error) {
	profile, err := Profile(runDir)
	if err != nil {
		return SummaryResult{}, err
	}
	templates, err := Templates(runDir, "non-info", "count", 5)
	if err != nil {
		return SummaryResult{}, err
	}
	result := SummaryResult{
		RunID:     profile.RunID,
		Focus:     Redact(opts.Focus),
		TimeRange: profile.TimeRange,
		RecommendedNextCommands: []string{
			fmt.Sprintf("log group %s --by error_signature --json", profile.RunID),
			fmt.Sprintf("log timeline %s --bucket 1m --json", profile.RunID),
		},
	}
	errorCount := profile.Levels["ERROR"] + profile.Levels["FATAL"] + profile.Levels["PANIC"]
	if len(templates.Templates) > 0 {
		top := templates.Templates[0]
		result.Headline = fmt.Sprintf("Dominant non-info template has %d occurrence(s): %s", top.Count, top.Template)
		result.Findings = append(result.Findings, SummaryFinding{
			Finding:      "The highest-volume non-info template is " + top.Template,
			Confidence:   "high",
			Count:        top.Count,
			EvidenceRefs: append([]string{}, top.Examples...),
		})
		result.RecommendedNextCommands = append(result.RecommendedNextCommands, fmt.Sprintf("log template get %s --template %s --json", profile.RunID, top.TemplateID))
	} else {
		result.Headline = "No non-info templates were found in this run."
	}
	if errorCount > 0 {
		result.Findings = append(result.Findings, SummaryFinding{
			Finding:      "The run contains ERROR/FATAL/PANIC entries.",
			Confidence:   "high",
			Count:        errorCount,
			EvidenceRefs: []string{},
		})
	}
	return result, nil
}

func ExportEvidence(runDir string, opts ExportEvidenceOptions) (ExportEvidenceResult, error) {
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return ExportEvidenceResult{}, err
	}
	opts.Evidence = strings.TrimSpace(opts.Evidence)
	if opts.Evidence == "" {
		return ExportEvidenceResult{}, NewError("invalid_args", "--evidence is required.", "Pass an entry_id or template_id from search, template, or extract output.", 400)
	}
	if strings.TrimSpace(opts.Output) == "" {
		return ExportEvidenceResult{}, NewError("invalid_args", "--output is required.", "Pass --output <file> for the redacted evidence export.", 400)
	}
	if opts.Format == "" {
		opts.Format = "json"
	}
	if opts.Format != "json" && opts.Format != "markdown" {
		return ExportEvidenceResult{}, NewError("invalid_args", "--format must be json or markdown.", "Run log schema export.evidence --json.", 400)
	}
	payload, err := evidencePayload(runDir, opts.Evidence)
	if err != nil {
		return ExportEvidenceResult{}, err
	}
	var content []byte
	if opts.Format == "markdown" {
		content = []byte(renderEvidenceMarkdown(manifest.RunID, opts.Evidence, payload))
	} else {
		content, err = json.MarshalIndent(map[string]any{"run_id": manifest.RunID, "evidence": opts.Evidence, "redacted": true, "data": payload}, "", "  ")
		if err != nil {
			return ExportEvidenceResult{}, err
		}
		content = append(content, '\n')
	}
	content = []byte(Redact(string(content)))
	result := ExportEvidenceResult{RunID: manifest.RunID, Evidence: opts.Evidence, Format: opts.Format, Output: opts.Output, DryRun: opts.DryRun, Bytes: len(content), Redacted: true}
	if opts.DryRun {
		return result, nil
	}
	if _, err := os.Stat(opts.Output); err == nil && !opts.Overwrite {
		return ExportEvidenceResult{}, NewError("log_export_exists", "Export output already exists.", "Pass --overwrite to replace the file, or choose a different --output.", 400)
	}
	if err := os.MkdirAll(filepath.Dir(opts.Output), 0o755); err != nil && filepath.Dir(opts.Output) != "." {
		return ExportEvidenceResult{}, err
	}
	if err := os.WriteFile(opts.Output, content, 0o644); err != nil {
		return ExportEvidenceResult{}, err
	}
	result.Written = true
	return result, nil
}

func parseBucket(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		value = "1m"
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, NewError("invalid_args", "--bucket is invalid.", "Use a duration such as 30s, 1m, or 5m.", 400)
	}
	return d, nil
}

func groupKey(entry Entry, by string, bucket time.Duration, templates map[string]Template) (string, string) {
	switch by {
	case "level":
		if entry.Level == "" {
			return "(unknown)", ""
		}
		return entry.Level, ""
	case "service":
		if strings.TrimSpace(entry.Service) == "" {
			return "(unknown)", ""
		}
		return entry.Service, ""
	case "time":
		if entry.Timestamp == "" {
			return "", ""
		}
		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return "", ""
		}
		return ts.Truncate(bucket).UTC().Format(time.RFC3339), ""
	case "error_signature":
		if tpl, ok := templates[entry.TemplateID]; ok && tpl.Template != "" {
			return tpl.Template, tpl.TemplateID
		}
		return entry.TemplateID, entry.TemplateID
	default:
		return entry.TemplateID, entry.TemplateID
	}
}

func evidenceRefForKey(by, key string, entry Entry) string {
	switch by {
	case "template", "error_signature":
		if entry.TemplateID != "" {
			return entry.TemplateID
		}
	}
	if entry.EntryID != "" {
		return entry.EntryID
	}
	return key
}

func evidencePayload(runDir, evidence string) (any, error) {
	var foundEntry Entry
	stop := fmt.Errorf("found")
	err := ReadEntries(runDir, func(entry Entry) error {
		if entry.EntryID == evidence {
			foundEntry = entry
			return stop
		}
		return nil
	})
	if err != nil && err != stop {
		return nil, err
	}
	if foundEntry.EntryID != "" {
		return foundEntry, nil
	}
	templates, err := ReadTemplates(runDir)
	if err != nil {
		return nil, err
	}
	for _, tpl := range templates {
		if tpl.TemplateID == evidence {
			return tpl, nil
		}
	}
	return nil, NewError("log_evidence_not_found", "Evidence was not found in this run.", "Use an entry_id or template_id returned by search, entries, template, group, or extract.", 404)
}

func renderEvidenceMarkdown(runID, evidence string, payload any) string {
	var b strings.Builder
	b.WriteString("# Log Evidence\n\n")
	b.WriteString("- Run: `")
	b.WriteString(runID)
	b.WriteString("`\n")
	b.WriteString("- Evidence: `")
	b.WriteString(evidence)
	b.WriteString("`\n")
	b.WriteString("- Redacted: true\n\n")
	switch v := payload.(type) {
	case Entry:
		b.WriteString("## Entry\n\n")
		b.WriteString("- Entry ID: `")
		b.WriteString(v.EntryID)
		b.WriteString("`\n")
		b.WriteString("- Level: `")
		b.WriteString(v.Level)
		b.WriteString("`\n")
		b.WriteString("- Source: `")
		b.WriteString(v.SourcePath)
		b.WriteString("`\n\n")
		b.WriteString("```text\n")
		b.WriteString(v.MessagePreview)
		b.WriteString("\n```\n")
	case Template:
		b.WriteString("## Template\n\n")
		b.WriteString("- Template ID: `")
		b.WriteString(v.TemplateID)
		b.WriteString("`\n")
		b.WriteString("- Count: ")
		b.WriteString(fmt.Sprintf("%d", v.Count))
		b.WriteString("\n\n")
		b.WriteString("```text\n")
		b.WriteString(v.Template)
		b.WriteString("\n```\n")
	default:
		b.WriteString("```text\n")
		b.WriteString(fmt.Sprintf("%v", payload))
		b.WriteString("\n```\n")
	}
	return Redact(b.String())
}
