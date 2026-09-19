package pgsql

import (
	"fmt"
	"sort"
	"strings"
)

// The statement guard is a pure function over the SQL text. It is the first
// of three read-only layers (guard, READ ONLY transaction, read-only database
// role) and exists to stop side effects the transaction cannot: single
// statement only, read-style first keyword, no data-modifying keywords inside
// CTEs or SELECT INTO, and no calls to functions that terminate sessions, read
// or list server files, sleep, reconfigure, lock, notify, or reach other
// databases. Comments and string literals are lexed the way PostgreSQL lexes
// them so they cannot hide a second statement.

// Statement is a guarded statement ready for execution.
type Statement struct {
	// SQL is the input with a single trailing semicolon (and anything after
	// it, which can only be whitespace or comments) removed.
	SQL string
	// Kind is the upper-cased first keyword: SELECT, WITH, TABLE, VALUES,
	// EXPLAIN, or SHOW.
	Kind string
}

// Cursorable reports whether the statement can be wrapped in DECLARE CURSOR,
// which lets the executor fetch only limit+1 rows from the server.
func (s Statement) Cursorable() bool {
	switch s.Kind {
	case "SELECT", "WITH", "TABLE", "VALUES":
		return true
	default:
		return false
	}
}

var allowedFirstKeywords = map[string]bool{
	"SELECT": true, "WITH": true, "EXPLAIN": true, "SHOW": true, "TABLE": true, "VALUES": true,
}

// explainableKeywords are the statement types EXPLAIN may wrap. ANALYZE
// executes the statement, so only read statements are accepted, with or
// without ANALYZE.
var explainableKeywords = map[string]bool{
	"SELECT": true, "WITH": true, "TABLE": true, "VALUES": true,
}

// forbiddenKeywords are rejected anywhere outside string literals and quoted
// identifiers: they mark data-modifying CTEs, SELECT INTO, DDL, and locking.
var forbiddenKeywords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true, "CREATE": true, "ALTER": true,
	"DROP": true, "TRUNCATE": true, "GRANT": true, "REVOKE": true, "COPY": true, "LOCK": true,
	"VACUUM": true, "REFRESH": true, "INTO": true,
}

// forbiddenFunctions are matched case-insensitively as `name (` (optionally
// schema-qualified or double-quoted). Exact names come first; prefixes cover
// whole families such as dblink_* and pg_advisory_*.
var forbiddenFunctions = map[string]bool{
	"pg_terminate_backend": true, "pg_cancel_backend": true,
	"pg_read_file": true, "pg_read_binary_file": true, "pg_ls_dir": true, "pg_stat_file": true,
	"pg_ls_logdir": true, "pg_ls_waldir": true,
	"lo_import": true, "lo_export": true, "lo_unlink": true, "lo_put": true, "lo_from_bytea": true,
	"lo_create": true, "lo_creat": true, "lo_open": true, "lo_truncate": true, "lo_truncate64": true,
	"lowrite": true, "lo_write": true,
	"dblink": true, "dblink_exec": true, "dblink_connect": true,
	"pg_sleep": true, "pg_sleep_for": true, "pg_sleep_until": true,
	"pg_reload_conf": true, "pg_rotate_logfile": true, "set_config": true,
	"pg_advisory_lock": true, "pg_advisory_xact_lock": true, "pg_notify": true,
	"pg_switch_wal": true, "pg_create_restore_point": true, "pg_promote": true,
	"pg_backup_start": true, "pg_backup_stop": true, "pg_start_backup": true, "pg_stop_backup": true,
	"pg_create_physical_replication_slot": true, "pg_create_logical_replication_slot": true,
	"pg_drop_replication_slot": true, "pg_copy_physical_replication_slot": true, "pg_copy_logical_replication_slot": true,
	"pg_replication_slot_advance": true, "pg_logical_slot_get_changes": true, "pg_logical_slot_get_binary_changes": true,
	"pg_replication_origin_create": true, "pg_replication_origin_drop": true, "pg_replication_origin_session_setup": true,
	"pg_replication_origin_session_reset": true, "pg_replication_origin_advance": true,
	"pg_replication_origin_xact_setup": true, "pg_replication_origin_xact_reset": true,
	"pg_logical_emit_message": true, "pg_wal_replay_pause": true, "pg_wal_replay_resume": true,
	"pg_log_backend_memory_contexts": true, "pg_import_system_collations": true,
	"nextval": true, "setval": true,
}

var forbiddenFunctionPrefixes = []string{
	"dblink", "pg_advisory_", "pg_try_advisory_", "pg_ls_", "pg_read_", "pg_stat_reset", "pg_file_",
}

// ForbiddenFunctions returns the sorted exact-name blacklist (for docs/tests).
func ForbiddenFunctions() []string {
	out := make([]string, 0, len(forbiddenFunctions))
	for name := range forbiddenFunctions {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isForbiddenFunction(name string) bool {
	lower := strings.ToLower(name)
	if forbiddenFunctions[lower] {
		return true
	}
	for _, prefix := range forbiddenFunctionPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// Check reports whether sql is a single read-only statement that may run.
// The returned error is a *Error with code read_only_violation (or
// invalid_args for empty or unlexable input).
func Check(sql string) error {
	_, err := Prepare(sql)
	return err
}

// Prepare runs the guard and returns the normalized statement.
func Prepare(sql string) (Statement, error) {
	toks, err := tokenize(sql)
	if err != nil {
		return Statement{}, err
	}
	end := len(sql)
	if n := len(toks); n > 0 && toks[n-1].isSymbol(';') {
		end = toks[n-1].pos
		toks = toks[:n-1]
	}
	if len(toks) == 0 {
		return Statement{}, invalidArgs("statement is empty", "Pass a SELECT statement with --sql or --sql-file.")
	}
	for _, t := range toks {
		if t.isSymbol(';') {
			return Statement{}, violation("multiple statements are not allowed (';' found before the end of the statement)", "Run exactly one SELECT per pgsql query call.")
		}
	}
	kind, err := checkStatement(toks)
	if err != nil {
		return Statement{}, err
	}
	return Statement{SQL: strings.TrimSpace(sql[:end]), Kind: kind}, nil
}

func checkStatement(toks []token) (string, error) {
	j := skipOpenParens(toks, 0)
	if j >= len(toks) || toks[j].kind != tokWord {
		return "", violation("statement must start with SELECT, WITH, EXPLAIN, SHOW, TABLE, or VALUES", "Only read statements may run; rewrite the request as a SELECT.")
	}
	first := strings.ToUpper(toks[j].text)
	if !allowedFirstKeywords[first] {
		return "", violation(fmt.Sprintf("statement type %s is not allowed; only SELECT, WITH, EXPLAIN, SHOW, TABLE, and VALUES may run", first), "pgsql is read-only: rewrite the request as a SELECT, or use the application's own change process for writes.")
	}
	if first == "EXPLAIN" {
		if err := checkExplain(toks[j+1:]); err != nil {
			return "", err
		}
		return first, nil
	}
	if err := checkBody(toks[j+1:]); err != nil {
		return "", err
	}
	return first, nil
}

// checkExplain validates the tokens after EXPLAIN: an optional option list or
// legacy ANALYZE/VERBOSE flags, then a read statement that itself passes the
// body checks (ANALYZE executes it).
func checkExplain(rest []token) error {
	if err := checkBody(rest); err != nil {
		return err
	}
	k := 0
	if k < len(rest) && rest[k].isSymbol('(') {
		depth := 0
		for k < len(rest) {
			if rest[k].isSymbol('(') {
				depth++
			} else if rest[k].isSymbol(')') {
				depth--
				if depth == 0 {
					k++
					break
				}
			}
			k++
		}
		if depth != 0 {
			return invalidArgs("EXPLAIN option list is not balanced", "")
		}
	} else {
		for k < len(rest) && rest[k].kind == tokWord {
			switch strings.ToUpper(rest[k].text) {
			case "ANALYZE", "ANALYSE", "VERBOSE":
				k++
				continue
			}
			break
		}
	}
	k = skipOpenParens(rest, k)
	if k >= len(rest) || rest[k].kind != tokWord || !explainableKeywords[strings.ToUpper(rest[k].text)] {
		return violation("EXPLAIN may only explain a SELECT, WITH, TABLE, or VALUES statement", "EXPLAIN ANALYZE executes the statement, so only read statements can be explained.")
	}
	return nil
}

func checkBody(toks []token) error {
	for i, t := range toks {
		switch t.kind {
		case tokWord:
			upper := strings.ToUpper(t.text)
			if forbiddenKeywords[upper] {
				return violation(fmt.Sprintf("keyword %s is not allowed in a read-only statement", upper), "Data-modifying CTEs, SELECT INTO, DDL, locking, and COPY are rejected; quote the word if it is really a column name.")
			}
			if i+1 < len(toks) && toks[i+1].isSymbol('(') && isForbiddenFunction(t.text) {
				return violation(fmt.Sprintf("function %s is not allowed", strings.ToLower(t.text)), "Functions that terminate sessions, read server files, sleep, reconfigure, lock, notify, or reach other databases are rejected.")
			}
		case tokIdent:
			if i+1 < len(toks) && toks[i+1].isSymbol('(') && isForbiddenFunction(t.text) {
				return violation(fmt.Sprintf("function %s is not allowed", strings.ToLower(t.text)), "Functions that terminate sessions, read server files, sleep, reconfigure, lock, notify, or reach other databases are rejected.")
			}
		}
	}
	return nil
}

func skipOpenParens(toks []token, i int) int {
	for i < len(toks) && toks[i].isSymbol('(') {
		i++
	}
	return i
}

// ---- lexer -----------------------------------------------------------------

type tokenKind int

const (
	tokWord   tokenKind = iota // unquoted identifier or keyword
	tokIdent                   // double-quoted identifier (text holds the unquoted content)
	tokString                  // string literal of any flavor (content dropped)
	tokNumber                  // numeric literal
	tokParam                   // $1 style parameter
	tokSymbol                  // single punctuation byte
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func (t token) isSymbol(c byte) bool {
	return t.kind == tokSymbol && len(t.text) == 1 && t.text[0] == c
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '$'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// tokenize lexes sql the way PostgreSQL does for the constructs that matter
// to the guard: -- and nested /* */ comments (dropped), '...' strings with
// doubled quotes, E'...' strings with backslash escapes, $tag$...$tag$ dollar
// quoting, "..." identifiers, $n parameters, words, numbers, and symbols.
func tokenize(sql string) ([]token, error) {
	var toks []token
	n := len(sql)
	i := 0
	for i < n {
		c := sql[i]
		switch {
		case isSpace(c):
			i++
		case c == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			depth := 1
			i += 2
			for i < n && depth > 0 {
				if sql[i] == '/' && i+1 < n && sql[i+1] == '*' {
					depth++
					i += 2
					continue
				}
				if sql[i] == '*' && i+1 < n && sql[i+1] == '/' {
					depth--
					i += 2
					continue
				}
				i++
			}
			if depth > 0 {
				return nil, invalidArgs("unterminated block comment", "")
			}
		case c == '\'':
			end, ok := scanStandardString(sql, i+1)
			if !ok {
				return nil, invalidArgs("unterminated string literal", "")
			}
			toks = append(toks, token{kind: tokString, pos: i})
			i = end
		case (c == 'e' || c == 'E') && i+1 < n && sql[i+1] == '\'':
			end, ok := scanEscapeString(sql, i+2)
			if !ok {
				return nil, invalidArgs("unterminated string literal", "")
			}
			toks = append(toks, token{kind: tokString, pos: i})
			i = end
		case (c == 'b' || c == 'B' || c == 'x' || c == 'X' || c == 'n' || c == 'N') && i+1 < n && sql[i+1] == '\'':
			end, ok := scanStandardString(sql, i+2)
			if !ok {
				return nil, invalidArgs("unterminated string literal", "")
			}
			toks = append(toks, token{kind: tokString, pos: i})
			i = end
		case (c == 'u' || c == 'U') && i+2 < n && sql[i+1] == '&' && sql[i+2] == '\'':
			end, ok := scanStandardString(sql, i+3)
			if !ok {
				return nil, invalidArgs("unterminated string literal", "")
			}
			toks = append(toks, token{kind: tokString, pos: i})
			i = end
		case (c == 'u' || c == 'U') && i+2 < n && sql[i+1] == '&' && sql[i+2] == '"':
			text, end, ok := scanQuotedIdent(sql, i+3)
			if !ok {
				return nil, invalidArgs("unterminated quoted identifier", "")
			}
			toks = append(toks, token{kind: tokIdent, text: text, pos: i})
			i = end
		case c == '"':
			text, end, ok := scanQuotedIdent(sql, i+1)
			if !ok {
				return nil, invalidArgs("unterminated quoted identifier", "")
			}
			toks = append(toks, token{kind: tokIdent, text: text, pos: i})
			i = end
		case c == '$':
			if i+1 < n && isDigit(sql[i+1]) {
				j := i + 1
				for j < n && isDigit(sql[j]) {
					j++
				}
				toks = append(toks, token{kind: tokParam, text: sql[i:j], pos: i})
				i = j
				continue
			}
			if tag, ok := dollarTag(sql, i); ok {
				close := strings.Index(sql[i+len(tag):], tag)
				if close < 0 {
					return nil, invalidArgs("unterminated dollar-quoted string", "")
				}
				toks = append(toks, token{kind: tokString, pos: i})
				i = i + len(tag) + close + len(tag)
				continue
			}
			toks = append(toks, token{kind: tokSymbol, text: "$", pos: i})
			i++
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentChar(sql[j]) {
				j++
			}
			toks = append(toks, token{kind: tokWord, text: sql[i:j], pos: i})
			i = j
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(sql[i+1])):
			j := i + 1
			for j < n && (isDigit(sql[j]) || sql[j] == '.' || sql[j] == 'e' || sql[j] == 'E' || ((sql[j] == '+' || sql[j] == '-') && (sql[j-1] == 'e' || sql[j-1] == 'E'))) {
				j++
			}
			toks = append(toks, token{kind: tokNumber, text: sql[i:j], pos: i})
			i = j
		default:
			toks = append(toks, token{kind: tokSymbol, text: sql[i : i+1], pos: i})
			i++
		}
	}
	return toks, nil
}

// scanStandardString scans a '...' body starting after the opening quote;
// only a doubled quote continues the literal. It returns the index after the
// closing quote.
func scanStandardString(sql string, i int) (int, bool) {
	n := len(sql)
	for i < n {
		if sql[i] == '\'' {
			if i+1 < n && sql[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1, true
		}
		i++
	}
	return n, false
}

// scanEscapeString scans an E'...' body where a backslash escapes the next
// byte and a doubled quote continues the literal.
func scanEscapeString(sql string, i int) (int, bool) {
	n := len(sql)
	for i < n {
		switch sql[i] {
		case '\\':
			i += 2
		case '\'':
			if i+1 < n && sql[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1, true
		default:
			i++
		}
	}
	return n, false
}

// scanQuotedIdent scans a "..." identifier body; a doubled quote is a literal
// quote. It returns the unquoted text and the index after the closing quote.
func scanQuotedIdent(sql string, i int) (string, int, bool) {
	n := len(sql)
	var b strings.Builder
	for i < n {
		if sql[i] == '"' {
			if i+1 < n && sql[i+1] == '"' {
				b.WriteByte('"')
				i += 2
				continue
			}
			return b.String(), i + 1, true
		}
		b.WriteByte(sql[i])
		i++
	}
	return "", n, false
}

// dollarTag returns the $tag$ delimiter starting at sql[i] when it forms a
// valid dollar-quote opener ($$ or $ident$).
func dollarTag(sql string, i int) (string, bool) {
	n := len(sql)
	j := i + 1
	if j < n && sql[j] == '$' {
		return "$$", true
	}
	if j >= n || !(sql[j] == '_' || (sql[j] >= 'a' && sql[j] <= 'z') || (sql[j] >= 'A' && sql[j] <= 'Z') || sql[j] >= 0x80) {
		return "", false
	}
	for j < n && (sql[j] == '_' || (sql[j] >= 'a' && sql[j] <= 'z') || (sql[j] >= 'A' && sql[j] <= 'Z') || isDigit(sql[j]) || sql[j] >= 0x80) {
		j++
	}
	if j < n && sql[j] == '$' {
		return sql[i : j+1], true
	}
	return "", false
}
