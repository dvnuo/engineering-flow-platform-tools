package logtool

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
)

type variablePattern struct {
	re      *regexp.Regexp
	varFunc func(string) string
}

var templateWhitespace = regexp.MustCompile(`\s+`)

var variablePatterns = []variablePattern{
	{regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>]+`), func(string) string { return "<url>" }},
	{regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`), func(string) string { return "<email>" }},
	{regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), func(string) string { return "<uuid>" }},
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), func(string) string { return "<ip>" }},
	{regexp.MustCompile(`(?i)\b\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h)\b`), func(s string) string { return Redact(s) }},
	{regexp.MustCompile(`"([^"\\]|\\.)*"|'([^'\\]|\\.)*'`), func(string) string { return "<string>" }},
	{regexp.MustCompile(`(?i)\b0x[0-9a-f]+\b|\b[0-9a-f]{16,}\b`), func(string) string { return "<hex>" }},
	{regexp.MustCompile(`(?:[A-Za-z]:\\|\.{1,2}[\\/]|/)[^\s"'<>:;]+(?:[\\/][^\s"'<>:;]+)*`), func(string) string { return "<path>" }},
	{regexp.MustCompile(`\b\d{4,}\b`), func(s string) string { return Redact(s) }},
}

func BuildTemplate(message string) (string, []string, string) {
	text := Redact(message)
	var variables []string
	for _, vp := range variablePatterns {
		text = vp.re.ReplaceAllStringFunc(text, func(match string) string {
			variable := strings.TrimSpace(vp.varFunc(match))
			if variable != "" && variable != "<*>" {
				variables = append(variables, variable)
			}
			return "<*>"
		})
	}
	text = strings.TrimSpace(templateWhitespace.ReplaceAllString(text, " "))
	if text == "" {
		text = "<empty>"
	}
	sum := sha1.Sum([]byte(text))
	return text, variables, "tpl_" + hex.EncodeToString(sum[:])[:16]
}

func Classify(template, level string) (string, []string) {
	lower := strings.ToLower(template)
	signal := "information"
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "slow") || strings.Contains(lower, "latency") || strings.Contains(lower, "took too long"):
		signal = "latency"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "unavailable") || strings.Contains(lower, "down") || strings.Contains(lower, "unreachable"):
		signal = "availability"
	case strings.Contains(lower, "oom") || strings.Contains(lower, "out of memory") || strings.Contains(lower, "disk full") || strings.Contains(lower, "no space") || strings.Contains(lower, "too many open files") || strings.Contains(lower, "pool exhausted"):
		signal = "saturation"
	case isErrorLevel(level):
		signal = "error"
	}
	tags := []string{signal}
	if isErrorLevel(level) && signal != "error" {
		tags = append(tags, "error")
	}
	return signal, tags
}

func isErrorLevel(level string) bool {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "ERROR", "FATAL", "PANIC":
		return true
	default:
		return false
	}
}

func NormalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))
	if level == "WARNING" {
		return "WARN"
	}
	switch level {
	case "TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "PANIC":
		return level
	default:
		return ""
	}
}
