package splunk

import "strings"

// blockedCommands lists the SPL commands that write data, trigger actions, or
// execute code. The splunk CLI is read-only, so a search whose pipeline names
// one of them is refused before any job is created.
var blockedCommands = map[string]bool{
	"delete":         true,
	"outputlookup":   true,
	"outputcsv":      true,
	"outputtext":     true,
	"collect":        true,
	"mcollect":       true,
	"meventcollect":  true,
	"sendemail":      true,
	"sendalert":      true,
	"script":         true,
	"runshellscript": true,
	"tscollect":      true,
	"summaryindex":   true,
}

// BlockedCommands returns the sorted-stable list of SPL commands the guard
// refuses, for documentation and error hints.
func BlockedCommands() []string {
	return []string{"delete", "outputlookup", "outputcsv", "outputtext", "collect", "mcollect", "meventcollect", "sendemail", "sendalert", "script", "runshellscript", "tscollect", "summaryindex"}
}

// BlockedCommand reports the first side-effect SPL command found in query.
// The query is split on `|` outside double-quoted strings and the first word
// of every segment (lower-cased) is compared against the block list. Searches
// dispatched through `map` are inspected recursively because map runs the
// quoted search it is given. Unbalanced quotes fall back to a plain split so
// a stray quote can never hide a pipeline stage.
func BlockedCommand(query string) (string, bool) {
	for _, segment := range splitPipes(query) {
		word := firstWord(segment)
		if word == "" {
			continue
		}
		if blockedCommands[word] {
			return word, true
		}
		if word == "map" {
			for _, quoted := range quotedStrings(segment) {
				if cmd, blocked := BlockedCommand(quoted); blocked {
					return cmd, true
				}
			}
		}
	}
	return "", false
}

func splitPipes(query string) []string {
	var segments []string
	var cur strings.Builder
	inQuote := false
	escaped := false
	for _, r := range query {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case inQuote && r == '\\':
			cur.WriteRune(r)
			escaped = true
		case r == '"':
			cur.WriteRune(r)
			inQuote = !inQuote
		case r == '|' && !inQuote:
			segments = append(segments, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	segments = append(segments, cur.String())
	if inQuote {
		return strings.Split(query, "|")
	}
	return segments
}

func firstWord(segment string) string {
	for _, field := range strings.Fields(segment) {
		word := strings.ToLower(strings.Trim(field, "[]()"))
		if word != "" {
			return word
		}
	}
	return ""
}

func quotedStrings(segment string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	escaped := false
	for _, r := range segment {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case inQuote && r == '\\':
			escaped = true
		case r == '"':
			if inQuote {
				out = append(out, cur.String())
				cur.Reset()
			}
			inQuote = !inQuote
		case inQuote:
			cur.WriteRune(r)
		}
	}
	if inQuote && cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
