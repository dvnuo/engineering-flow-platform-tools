package logtool

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

func WindowByEntry(runDir string, entryID string, before, after int) (WindowResult, error) {
	if strings.TrimSpace(entryID) == "" {
		return WindowResult{}, NewError("invalid_args", "--entry-id is required.", "Pass --entry-id from search or entries output.", 400)
	}
	if _, err := ReadManifest(runDir); err != nil {
		return WindowResult{}, err
	}
	var found Entry
	errStop := errors.New("found")
	err := ReadEntries(runDir, func(entry Entry) error {
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
	result, err := readWindow(found.SourcePath, found.LineStart, found.LineEnd, before, after)
	if err != nil {
		return WindowResult{}, err
	}
	result.EntryID = found.EntryID
	return result, nil
}

func WindowByFileLine(path string, line int, before, after int) (WindowResult, error) {
	if strings.TrimSpace(path) == "" || line <= 0 {
		return WindowResult{}, NewError("invalid_args", "--file and --line are required.", "Pass --file <path> --line <line-number>.", 400)
	}
	return readWindow(path, int64(line), int64(line), before, after)
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
		line, readErr := reader.ReadString('\n')
		if line != "" {
			lineNo++
			if lineNo >= start && lineNo <= end {
				text := strings.TrimRight(line, "\r\n")
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
