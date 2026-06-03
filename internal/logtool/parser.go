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
	lines     []string
	lineStart int64
	lineEnd   int64
	byteStart int64
	byteEnd   int64
}

func ParseStream(r io.Reader, sourcePath string, opts ParseOptions, emit func(ParsedEvent) error) (ParseResult, error) {
	if opts.MaxLineBytes <= 0 {
		opts.MaxLineBytes = 65536
	}
	br := bufio.NewReader(r)
	var result ParseResult
	var current *pendingEvent
	lineNo := int64(0)
	offset := int64(0)
	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return result, err
		}
		if line == "" && errors.Is(err, io.EOF) {
			break
		}
		lineBytes := int64(len([]byte(line)))
		if opts.MaxBytes > 0 && result.Bytes+lineBytes > opts.MaxBytes {
			result.Truncated = true
			break
		}
		lineNo++
		result.Lines = lineNo
		result.Bytes += lineBytes
		clean := strings.TrimRight(line, "\r\n")
		lineStartOffset := offset
		offset += lineBytes
		starts := isNewEventLine(clean, opts.FormatHint)
		if current == nil {
			current = &pendingEvent{lineStart: lineNo, byteStart: lineStartOffset}
		} else if starts {
			if err := emitPending(current, opts, emit); err != nil {
				return result, err
			}
			current = &pendingEvent{lineStart: lineNo, byteStart: lineStartOffset}
		}
		current.lines = append(current.lines, truncateForPreview(clean, opts.MaxLineBytes))
		current.lineEnd = lineNo
		current.byteEnd = offset
		if errors.Is(err, io.EOF) {
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

func truncateForPreview(s string, max int64) string {
	if max <= 0 {
		max = 65536
	}
	if int64(len([]byte(s))) <= max {
		return s
	}
	b := []byte(s)
	if int64(len(b)) > max {
		b = b[:max]
	}
	b = bytes.TrimRight(b, "\x00")
	return string(b) + "...(line truncated)"
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
