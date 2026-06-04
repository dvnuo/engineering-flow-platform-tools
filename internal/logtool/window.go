package logtool

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func WindowByEntry(runDir string, entryID string, before, after int) (WindowResult, error) {
	if strings.TrimSpace(entryID) == "" {
		return WindowResult{}, NewError("invalid_args", "--entry-id is required.", "Pass --entry-id from search or entries output.", 400)
	}
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return WindowResult{}, err
	}
	var found Entry
	errStop := errors.New("found")
	err = ReadEntries(runDir, func(entry Entry) error {
		if entry.EntryID == entryID {
			found = entry
			return errStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return WindowResult{}, err
	}
	if found.EntryID == "" {
		return WindowResult{}, NewError("not_found", "Entry was not found in this run.", "Run log entries --run <run-dir> --json to list entry ids.", 404)
	}
	sourceRef, ok := sourceRefForEntry(manifest, found)
	if !ok {
		return WindowResult{}, NewError("entry_source_not_in_run", "Entry source is not part of this log run.", "Re-run log analyze or use an entry id from this run.", 409)
	}
	if found.LineStart < 1 || found.LineEnd < found.LineStart || (sourceRef.Lines > 0 && found.LineEnd > sourceRef.Lines) {
		return WindowResult{}, NewError("entry_outside_run_source_range", "Entry line range is outside the analyzed source range.", "Re-run log analyze; the run index may be stale or corrupted.", 409)
	}
	result, err := readWindow(sourceRef.Path, found.LineStart, found.LineEnd, before, after)
	if err != nil {
		return WindowResult{}, err
	}
	result.EntryID = found.EntryID
	return result, nil
}

func WindowByFileLineInRun(runDir string, path string, line int, before, after int) (WindowResult, error) {
	if strings.TrimSpace(runDir) == "" {
		return WindowResult{}, NewError("invalid_args", "--run is required.", "Pass --run <run-dir> produced by log analyze.", 400)
	}
	if strings.TrimSpace(path) == "" || line <= 0 {
		return WindowResult{}, NewError("invalid_args", "--file and --line are required.", "Pass --file <path> --line <line-number>.", 400)
	}
	manifest, err := ReadManifest(runDir)
	if err != nil {
		return WindowResult{}, err
	}
	sourceRef, ok := sourceRefInManifest(manifest, path)
	if !ok {
		return WindowResult{}, NewError("source_not_in_run", "The requested source file is not part of this log run.", "Use log analyze on that source first, or call log window with an entry_id from this run.", 403)
	}
	if sourceRef.Lines > 0 && int64(line) > sourceRef.Lines {
		return WindowResult{}, NewError("line_outside_run_source_range", "Requested line is outside the analyzed source range.", "Run log analyze again if the source log changed.", 400)
	}
	return readWindow(sourceRef.Path, int64(line), int64(line), before, after)
}

func readWindow(path string, targetStart, targetEnd int64, before, after int) (WindowResult, error) {
	if before < 0 || after < 0 || before > maxResultLimit || after > maxResultLimit {
		return WindowResult{}, NewError("invalid_args", "--before and --after must be between 0 and 200.", "Use a bounded window size.", 400)
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WindowResult{}, NewError("source_missing", "Source log file is no longer available.", "Restore the source file or re-run log analyze with available logs.", 404)
		}
		return WindowResult{}, err
	}
	defer f.Close()
	start := targetStart - int64(before)
	if start < 1 {
		start = 1
	}
	end := targetEnd + int64(after)
	reader := bufio.NewReader(f)
	lineNo := int64(0)
	var lines []WindowLine
	for {
		line, readErr := readPhysicalLine(reader, defaultMaxLinePreviewBytes)
		if line.ByteLen > 0 {
			lineNo++
			if lineNo >= start && lineNo <= end {
				text := strings.TrimRight(line.Preview, "\r\n")
				if line.Truncated {
					text += lineTruncatedMarker
				}
				lines = append(lines, WindowLine{
					Line:   lineNo,
					Text:   Redact(text),
					Target: lineNo >= targetStart && lineNo <= targetEnd,
				})
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return WindowResult{}, readErr
		}
		if lineNo > end {
			break
		}
	}
	return WindowResult{
		Source: WindowSource{Path: path, LineStart: targetStart, LineEnd: targetEnd},
		Before: before,
		After:  after,
		Lines:  lines,
	}, nil
}

func sourceRefInManifest(manifest Manifest, requested string) (SourceRef, bool) {
	requestedKeys := comparablePathKeys(requested)
	if len(requestedKeys) == 0 {
		return SourceRef{}, false
	}
	for _, source := range manifest.Sources {
		sourceKeys := comparablePathKeys(source.Path)
		for key := range requestedKeys {
			if sourceKeys[key] {
				return source, true
			}
		}
	}
	return SourceRef{}, false
}

func sourceRefForEntry(manifest Manifest, entry Entry) (SourceRef, bool) {
	if strings.TrimSpace(entry.SourceID) != "" {
		for _, source := range manifest.Sources {
			if source.SourceID == entry.SourceID {
				if strings.TrimSpace(entry.SourcePath) == "" {
					return source, true
				}
				if _, ok := sourceRefInManifest(Manifest{Sources: []SourceRef{source}}, entry.SourcePath); ok {
					return source, true
				}
				return SourceRef{}, false
			}
		}
	}
	if strings.TrimSpace(entry.SourcePath) != "" {
		return sourceRefInManifest(manifest, entry.SourcePath)
	}
	return SourceRef{}, false
}

func comparablePathKeys(path string) map[string]bool {
	keys := map[string]bool{}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return keys
	}
	addPathKey(keys, trimmed)
	cleaned := filepath.Clean(trimmed)
	addPathKey(keys, cleaned)
	if abs, err := filepath.Abs(cleaned); err == nil {
		addPathKey(keys, abs)
		if evaluated, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
			addPathKey(keys, evaluated)
		}
	}
	if evaluated, err := filepath.EvalSymlinks(cleaned); err == nil {
		addPathKey(keys, evaluated)
	}
	return keys
}

func addPathKey(keys map[string]bool, path string) {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	keys[cleaned] = true
}
