package logtool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	defaultMaxLinePreviewBytes  int64 = 65536
	defaultMaxEventPreviewBytes int64 = 4 * 1024 * 1024
	defaultMaxEventLines        int64 = 10000
	lineTruncatedMarker               = "...(line truncated)"
	eventTruncatedMarker              = "...(event truncated)"
)

var (
	levelPattern       = regexp.MustCompile(`(?i)(^|[\s\[\]()=:,;\-])(?:trace|debug|info|warn|warning|error|fatal|panic)([\s\]\)[:,;=\-]|$)`)
	levelExtract       = regexp.MustCompile(`(?i)\b(trace|debug|info|warn|warning|error|fatal|panic)\b`)
	textTimestampRegex = []*regexp.Regexp{
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+\-]\d{2}:\d{2})`),
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`),
		regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`),
	}
)

type pendingEvent struct {
	lines        []string
	lineStart    int64
	lineEnd      int64
	byteStart    int64
	byteEnd      int64
	previewBytes int64
	truncated    bool
}

type physicalLine struct {
	Preview   string
	ByteLen   int64
	EOF       bool
	Truncated bool
	HitLimit  bool
}

func ParseStream(r io.Reader, sourcePath string, opts ParseOptions, emit func(ParsedEvent) error) (ParseResult, error) {
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = defaultMaxLinePreviewBytes
	}
	br := bufio.NewReader(r)
	var result ParseResult
	var current *pendingEvent
	lineNo := int64(0)
	offset := int64(0)
	for {
		remaining := int64(0)
		if opts.MaxBytes > 0 {
			remaining = opts.MaxBytes - result.Bytes
			if remaining <= 0 {
				result.Truncated = true
				break
			}
		}
		line, err := readPhysicalLineLimited(br, opts.MaxLineBytes, remaining)
		if err != nil && !errors.Is(err, io.EOF) {
			return result, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if line.ByteLen == 0 && line.EOF {
			break
		}
		if opts.MaxBytes > 0 && (line.HitLimit || result.Bytes+line.ByteLen > opts.MaxBytes) {
			result.Bytes += minInt64(line.ByteLen, remaining)
			result.Truncated = true
			break
		}
		lineNo++
		result.Lines = lineNo
		result.Bytes += line.ByteLen
		clean := strings.TrimRight(line.Preview, "\r\n")
		if line.Truncated {
			clean += lineTruncatedMarker
		}
		lineStartOffset := offset
		offset += line.ByteLen
		starts := isNewEventLine(clean, opts.FormatHint)
		if current == nil {
			current = &pendingEvent{lineStart: lineNo, byteStart: lineStartOffset}
		} else if starts {
			if err := emitPending(current, opts, emit); err != nil {
				return result, err
			}
			current = &pendingEvent{lineStart: lineNo, byteStart: lineStartOffset}
		}
		appendEventPreview(current, clean)
		current.lineEnd = lineNo
		current.byteEnd = offset
		if line.EOF {
			break
		}
	}
	if current != nil {
		if err := emitPending(current, opts, emit); err != nil {
			return result, err
		}
	}
	return result, nil
}

func readPhysicalLine(br *bufio.Reader, maxPreviewBytes int64) (physicalLine, error) {
	return readPhysicalLineLimited(br, maxPreviewBytes, 0)
}

func readPhysicalLineLimited(br *bufio.Reader, maxPreviewBytes, maxReadBytes int64) (physicalLine, error) {
	if maxPreviewBytes <= 0 {
		maxPreviewBytes = defaultMaxLinePreviewBytes
	}
	var out physicalLine
	var preview bytes.Buffer
	for {
		fragment, err := br.ReadSlice('\n')
		if len(fragment) > 0 {
			out.ByteLen += int64(len(fragment))
			if maxReadBytes > 0 && out.ByteLen > maxReadBytes {
				out.HitLimit = true
				out.Truncated = true
			}
			if int64(preview.Len()) < maxPreviewBytes {
				remaining := maxPreviewBytes - int64(preview.Len())
				take := int64(len(fragment))
				if take > remaining {
					take = remaining
					out.Truncated = true
				}
				if take > 0 {
					_, _ = preview.Write(fragment[:take])
				}
			} else {
				out.Truncated = true
			}
			if out.HitLimit {
				out.Preview = preview.String()
				return out, nil
			}
		}
		switch {
		case err == nil:
			out.Preview = preview.String()
			return out, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if out.ByteLen == 0 {
				return physicalLine{EOF: true}, io.EOF
			}
			out.EOF = true
			out.Preview = preview.String()
			return out, nil
		default:
			return out, err
		}
	}
}

func appendEventPreview(p *pendingEvent, line string) {
	if p.truncated {
		return
	}
	if int64(len(p.lines)) >= defaultMaxEventLines {
		appendEventTruncationMarker(p)
		return
	}
	lineBytes := int64(len([]byte(line)))
	separatorBytes := int64(0)
	if len(p.lines) > 0 {
		separatorBytes = 1
	}
	if p.previewBytes+separatorBytes+lineBytes > defaultMaxEventPreviewBytes {
		appendEventTruncationMarker(p)
		return
	}
	p.lines = append(p.lines, line)
	p.previewBytes += separatorBytes + lineBytes
}

func appendEventTruncationMarker(p *pendingEvent) {
	if p.truncated {
		return
	}
	separatorBytes := int64(0)
	if len(p.lines) > 0 {
		separatorBytes = 1
	}
	p.lines = append(p.lines, eventTruncatedMarker)
	p.previewBytes += separatorBytes + int64(len(eventTruncatedMarker))
	p.truncated = true
}

func emitPending(p *pendingEvent, opts ParseOptions, emit func(ParsedEvent) error) error {
	raw := strings.Join(p.lines, "\n")
	ts, level, service, msg := parseEventText(raw, opts.FormatHint)
	msg = Redact(msg)
	if msg == "" {
		msg = Redact(raw)
	}
	return emit(ParsedEvent{
		Raw:       Redact(raw),
		Timestamp: ts,
		Level:     level,
		Service:   service,
		Message:   msg,
		LineStart: p.lineStart,
		LineEnd:   p.lineEnd,
		ByteStart: p.byteStart,
		ByteEnd:   p.byteEnd,
	})
}

func isNewEventLine(line, hint string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.EqualFold(hint, "json") || strings.EqualFold(hint, "auto") || hint == "" {
		var obj map[string]any
		if json.Unmarshal([]byte(trimmed), &obj) == nil && len(obj) > 0 {
			return true
		}
	}
	if findTimestamp(trimmed) != "" {
		return true
	}
	return levelPattern.MatchString(trimmed)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func parseEventText(raw, hint string) (string, string, string, string) {
	first := firstLine(raw)
	trimmed := strings.TrimSpace(first)
	if strings.EqualFold(hint, "json") || strings.EqualFold(hint, "auto") || hint == "" {
		var obj map[string]any
		if json.Unmarshal([]byte(trimmed), &obj) == nil && len(obj) > 0 {
			return parseJSONEvent(obj, raw)
		}
	}
	ts := findTimestamp(raw)
	level := NormalizeLevel(extractLevel(raw))
	msg := strings.TrimSpace(raw)
	if ts != "" {
		for _, re := range textTimestampRegex {
			msg = re.ReplaceAllString(msg, " ")
		}
	}
	if level != "" {
		msg = levelExtract.ReplaceAllStringFunc(msg, func(match string) string {
			if NormalizeLevel(match) == level || (level == "WARN" && strings.EqualFold(match, "warning")) {
				level = "__removed__" + level
				return " "
			}
			return match
		})
		level = strings.TrimPrefix(level, "__removed__")
	}
	msg = strings.Trim(strings.TrimSpace(msg), "[]:- ")
	msg = strings.TrimSpace(msg)
	return ts, level, "", msg
}

func parseJSONEvent(obj map[string]any, raw string) (string, string, string, string) {
	ts := firstString(obj, "timestamp", "time", "ts", "@timestamp")
	ts = normalizeTimestamp(ts)
	level := NormalizeLevel(firstString(obj, "level", "severity", "severity_text"))
	service := firstString(obj, "service", "service.name", "logger", "component")
	msg := firstString(obj, "message", "msg", "log", "event", "error")
	if msg == "" {
		msg = strings.TrimSpace(raw)
	}
	return ts, level, service, msg
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := lookupJSONField(obj, key); ok {
			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case float64:
				if key == "ts" {
					return time.Unix(int64(v), 0).UTC().Format(time.RFC3339)
				}
			}
		}
	}
	return ""
}

func lookupJSONField(obj map[string]any, key string) (any, bool) {
	if val, ok := obj[key]; ok {
		return val, true
	}
	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		var cur any = obj
		for _, part := range parts {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			cur, ok = m[part]
			if !ok {
				return nil, false
			}
		}
		return cur, true
	}
	return nil, false
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func findTimestamp(s string) string {
	for _, re := range textTimestampRegex {
		if raw := re.FindString(s); raw != "" {
			if ts := normalizeTimestamp(raw); ts != "" {
				return ts
			}
		}
	}
	return ""
}

func normalizeTimestamp(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func extractLevel(s string) string {
	match := levelExtract.FindStringSubmatch(s)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}
