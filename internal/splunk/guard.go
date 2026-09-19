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
	"dump":           true,
}

// BlockedCommands returns the sorted-stable list of SPL commands the guard
// refuses, for documentation and error hints.
func BlockedCommands() []string {
	return []string{"delete", "outputlookup", "outputcsv", "outputtext", "collect", "mcollect", "meventcollect", "sendemail", "sendalert", "script", "runshellscript", "tscollect", "summaryindex", "dump"}
}

// MacroToken is returned by BlockedCommand when the query contains a
// backtick macro. A macro is expanded by Splunk, not by this guard, so its
// body could be any command at all and the query is refused rather than
// dispatched on trust.
const MacroToken = "`macro`"

// BlockedCommand reports the first side-effect SPL command found in query.
// The query is split on `|` outside double-quoted strings and the first word
// of every segment (lower-cased) is compared against the block list. Searches
// dispatched through `map` are inspected recursively because map runs the
// quoted search it is given. Unbalanced quotes fall back to a plain split so
// a stray quote can never hide a pipeline stage.
func BlockedCommand(query string) (string, bool) {
	// Splunk strips ```...``` comments before running the search, so a
	// comment placed where a command word belongs would otherwise be read as
	// the command name and hide the real one: `index=x | ```c``` delete`.
	query = stripSPLComments(query)
	if strings.ContainsRune(query, '`') {
		return MacroToken, true
	}
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

// stripSPLComments removes ```...``` inline comments. An unterminated run of
// three backticks is left in place: the trailing backticks then trip the
// macro check rather than silently swallowing the rest of the pipeline.
func stripSPLComments(query string) string {
	const marker = "```"
	var out strings.Builder
	for {
		start := strings.Index(query, marker)
		if start < 0 {
			out.WriteString(query)
			return out.String()
		}
		end := strings.Index(query[start+len(marker):], marker)
		if end < 0 {
			out.WriteString(query)
			return out.String()
		}
		out.WriteString(query[:start])
		// A comment separates tokens, exactly as whitespace would.
		out.WriteString(" ")
		query = query[start+len(marker)+end+len(marker):]
	}
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
