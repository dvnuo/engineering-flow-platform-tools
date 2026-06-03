package logtool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type templateAccumulator struct {
	item Template
}

func Analyze(ctx context.Context, opts AnalyzeOptions) (AnalyzeResult, error) {
	if strings.TrimSpace(opts.Source) == "" {
		return AnalyzeResult{}, NewError("invalid_args", "--source is required.", "Pass --source <file|dir|glob>.", 400)
	}
	if strings.TrimSpace(opts.RunDir) == "" {
		return AnalyzeResult{}, NewError("invalid_args", "--run is required.", "Pass --run <run-dir>.", 400)
	}
	if opts.FormatHint == "" {
		opts.FormatHint = "auto"
	}
	switch opts.FormatHint {
	case "auto", "json", "plain":
	default:
		return AnalyzeResult{}, NewError("invalid_args", "--format-hint must be auto, json, or plain.", "Run log schema analyze --json.", 400)
	}
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = 65536
	}
	if err := EnsureRunDir(opts.RunDir); err != nil {
		return AnalyzeResult{}, err
	}
	sources, err := expandSources(opts.Source)
	if err != nil {
		return AnalyzeResult{}, err
	}
	if len(sources) == 0 {
		return AnalyzeResult{}, NewError("source_not_found", "No log files matched --source.", "Pass a file, directory, or glob matching .log, .txt, .out, .err, or .jsonl files.", 404)
	}
	entriesPath := filepath.Join(opts.RunDir, entriesFile)
	entriesOut, err := os.OpenFile(entriesPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return AnalyzeResult{}, err
	}
	defer entriesOut.Close()

	runID := runIDFromDir(opts.RunDir)
	createdAt := time.Now().UTC().Format(time.RFC3339)
	manifest := Manifest{
		Version:          1,
		RunID:            runID,
		CreatedAt:        createdAt,
		FormatHint:       opts.FormatHint,
		RedactionEnabled: true,
		ToolVersion:      opts.ToolVersion,
	}
	templates := map[string]*templateAccumulator{}
	entrySeq := 0
	sourceSeq := 0
	var minTime, maxTime string
	remainingBytes := opts.MaxBytes
	totalLimitActive := opts.MaxBytes > 0

	for _, sourcePath := range sources {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return AnalyzeResult{}, err
			}
		}
		if totalLimitActive && remainingBytes <= 0 {
			manifest.Truncated = true
			break
		}
		sourceSeq++
		sourceID := fmt.Sprintf("src_%06d", sourceSeq)
		f, err := os.Open(sourcePath)
		if err != nil {
			return AnalyzeResult{}, err
		}
		parseMax := int64(0)
		if totalLimitActive {
			parseMax = remainingBytes
		}
		parseResult, parseErr := ParseStream(f, sourcePath, ParseOptions{FormatHint: opts.FormatHint, MaxBytes: parseMax, MaxLineBytes: opts.MaxLineBytes}, func(ev ParsedEvent) error {
			entrySeq++
			tpl, variables, tplID := BuildTemplate(ev.Message)
			signal, tags := Classify(tpl, ev.Level)
			entry := Entry{
				EntryID:        fmt.Sprintf("entry_%06d", entrySeq),
				SourceID:       sourceID,
				SourcePath:     sourcePath,
				ByteStart:      ev.ByteStart,
				ByteEnd:        ev.ByteEnd,
				LineStart:      ev.LineStart,
				LineEnd:        ev.LineEnd,
				Timestamp:      ev.Timestamp,
				Level:          ev.Level,
				Service:        ev.Service,
				MessagePreview: Redact(ev.Message),
				TemplateID:     tplID,
				Variables:      variables,
				Tags:           tags,
				GoldenSignal:   signal,
			}
			if err := AppendEntry(entriesOut, entry); err != nil {
				return err
			}
			acc, ok := templates[tplID]
			if !ok {
				acc = &templateAccumulator{item: Template{
					TemplateID:            tplID,
					Template:              tpl,
					RepresentativeEntryID: entry.EntryID,
					RepresentativeText:    entry.MessagePreview,
					Levels:                map[string]int{},
					Examples:              []string{entry.EntryID},
					GoldenSignal:          signal,
					Tags:                  tags,
				}}
				templates[tplID] = acc
			}
			acc.item.Count++
			if entry.Level != "" {
				acc.item.Levels[entry.Level]++
			}
			if ev.Timestamp != "" {
				if acc.item.FirstSeen == "" || ev.Timestamp < acc.item.FirstSeen {
					acc.item.FirstSeen = ev.Timestamp
				}
				if acc.item.LastSeen == "" || ev.Timestamp > acc.item.LastSeen {
					acc.item.LastSeen = ev.Timestamp
				}
				if minTime == "" || ev.Timestamp < minTime {
					minTime = ev.Timestamp
				}
				if maxTime == "" || ev.Timestamp > maxTime {
					maxTime = ev.Timestamp
				}
			}
			if len(acc.item.Examples) < 3 && !containsString(acc.item.Examples, entry.EntryID) {
				acc.item.Examples = append(acc.item.Examples, entry.EntryID)
			}
			return nil
		})
		closeErr := f.Close()
		if parseErr != nil {
			return AnalyzeResult{}, parseErr
		}
		if closeErr != nil {
			return AnalyzeResult{}, closeErr
		}
		if totalLimitActive {
			remainingBytes -= parseResult.Bytes
		}
		sourceRef := SourceRef{SourceID: sourceID, Path: sourcePath, Bytes: parseResult.Bytes, Lines: parseResult.Lines, Truncated: parseResult.Truncated}
		manifest.Sources = append(manifest.Sources, sourceRef)
		manifest.TotalBytes += parseResult.Bytes
		manifest.TotalLines += parseResult.Lines
		if parseResult.Truncated {
			manifest.Truncated = true
			break
		}
	}
	manifest.EntriesCount = entrySeq
	manifest.TemplatesCount = len(templates)
	manifest.TimeRange = TimeRange{Start: minTime, End: maxTime}
	templateList := make([]Template, 0, len(templates))
	for _, acc := range templates {
		templateList = append(templateList, acc.item)
	}
	sortTemplates(templateList, "count")
	if err := WriteTemplates(opts.RunDir, templateList); err != nil {
		return AnalyzeResult{}, err
	}
	if err := WriteManifest(opts.RunDir, manifest); err != nil {
		return AnalyzeResult{}, err
	}
	return AnalyzeResult{
		RunID:          manifest.RunID,
		RunDir:         opts.RunDir,
		Sources:        len(manifest.Sources),
		TotalBytes:     manifest.TotalBytes,
		TotalLines:     manifest.TotalLines,
		EntriesCount:   manifest.EntriesCount,
		TemplatesCount: manifest.TemplatesCount,
		TimeRange:      manifest.TimeRange,
		Truncated:      manifest.Truncated,
	}, nil
}

func expandSources(source string) ([]string, error) {
	source = filepath.Clean(source)
	var paths []string
	if hasGlobMeta(source) {
		matches, err := filepath.Glob(source)
		if err != nil {
			return nil, NewError("invalid_args", "Source glob pattern is invalid.", "Use a file, directory, or valid glob pattern.", 400)
		}
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				paths = append(paths, absPath(match))
			}
		}
		sort.Strings(paths)
		return paths, nil
	}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewError("source_not_found", "Log source was not found.", "Pass an existing file, directory, or glob.", 404)
		}
		return nil, err
	}
	if !info.IsDir() {
		return []string{absPath(source)}, nil
	}
	err = filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isLogFile(path) {
			paths = append(paths, absPath(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func isLogFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".log", ".txt", ".out", ".err", ".jsonl":
		return true
	default:
		return false
	}
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}

func runIDFromDir(runDir string) string {
	base := filepath.Base(filepath.Clean(runDir))
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		return "run_" + time.Now().UTC().Format("20060102T150405Z")
	}
	return base
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
