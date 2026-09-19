package pgsql

import (
	"errors"
	"strings"
	"testing"
)

func guardCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("guard returned a non-*Error: %T %v", err, err)
	}
	return e.Code
}

func TestGuardTable(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		code string // "" when the statement must pass
		want string // substring expected in the message when rejected
	}{
		// ---- allowed ----
		{"plain select", "SELECT 1", "", ""},
		{"lowercase select with param", "select * from users where id = $1", "", ""},
		{"single trailing semicolon", "SELECT 1;", "", ""},
		{"trailing semicolon with whitespace", "  SELECT 1 ;  ", "", ""},
		{"trailing semicolon then comment", "SELECT 1; -- done", "", ""},
		{"cte select", "WITH x AS (SELECT 1) SELECT * FROM x", "", ""},
		{"recursive cte", "WITH RECURSIVE t(n) AS (VALUES (1) UNION ALL SELECT n+1 FROM t WHERE n < 100) SELECT sum(n) FROM t", "", ""},
		{"explain select", "EXPLAIN SELECT 1", "", ""},
		{"explain analyze select", "EXPLAIN ANALYZE SELECT 1", "", ""},
		{"explain option list", "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT * FROM t", "", ""},
		{"explain verbose cte", "EXPLAIN VERBOSE WITH x AS (SELECT 1) SELECT * FROM x", "", ""},
		{"show", "SHOW server_version", "", ""},
		{"table", "TABLE users", "", ""},
		{"values", "VALUES (1, 2), (3, 4)", "", ""},
		{"semicolon inside string", "SELECT 'a; DROP TABLE x' AS s", "", ""},
		{"doubled quote inside string", "SELECT 'it''s; fine'", "", ""},
		{"escape string with escaped quote", `SELECT E'a\'; DROP TABLE x' AS s`, "", ""},
		{"dollar quoted string", "SELECT $$a; DELETE FROM x$$", "", ""},
		{"tagged dollar quote", "SELECT $tag$ ; INSERT $tag$", "", ""},
		{"unicode string literal", `SELECT U&'d\0061t\+000061'`, "", ""},
		{"bit and hex strings", "SELECT B'1010', X'1F'", "", ""},
		{"forbidden word in line comment", "SELECT 1 -- INSERT INTO x", "", ""},
		{"forbidden word in block comment", "SELECT 1 /* UPDATE t SET x = 1 */", "", ""},
		{"nested block comment", "SELECT 1 /* outer /* nested */ still comment */", "", ""},
		{"semicolon inside line comment", "SELECT * FROM t WHERE id = 1 --; DROP TABLE t", "", ""},
		{"semicolon inside block comment", "SELECT 1 /*; DROP TABLE t */", "", ""},
		{"quoted identifiers that look like keywords", `SELECT "update", "delete", "into" FROM t`, "", ""},
		{"identifiers containing keywords", "SELECT update_time, deleted_at, created_by, copy_count FROM t", "", ""},
		{"blacklisted name not called", "SELECT pg_sleep_state FROM t", "", ""},
		{"quoted blacklisted name not called", `SELECT * FROM "pg_terminate_backend"`, "", ""},
		{"allowed functions", "SELECT current_setting('statement_timeout'), pg_size_pretty(pg_database_size(current_database()))", "", ""},
		{"function name inside string", "SELECT x FROM t WHERE note = 'pg_sleep(10)'", "", ""},
		{"stat activity view", "SELECT * FROM pg_stat_activity WHERE state = 'active'", "", ""},
		{"leading parenthesis union", "(SELECT 1) UNION (SELECT 2)", "", ""},
		{"aggregate with params", "SELECT count(*) FROM orders GROUP BY status HAVING count(*) > $1", "", ""},
		{"fetch first", "SELECT * FROM t ORDER BY id FETCH FIRST 10 ROWS ONLY", "", ""},
		{"casts and arrays", "SELECT $1::int + $2::int, ARRAY[1,2] @> ARRAY[1]", "", ""},
		{"trailing comment", "SELECT 1 FROM t WHERE x = 'a' -- trailing comment", "", ""},
		// ---- rejected: statement type ----
		{"update", "UPDATE t SET x = 1", CodeReadOnlyViolation, "UPDATE"},
		{"insert", "INSERT INTO t VALUES (1)", CodeReadOnlyViolation, "INSERT"},
		{"delete", "DELETE FROM t", CodeReadOnlyViolation, "DELETE"},
		{"drop", "DROP TABLE t", CodeReadOnlyViolation, "DROP"},
		{"create", "CREATE TABLE t (id int)", CodeReadOnlyViolation, "CREATE"},
		{"truncate", "TRUNCATE t", CodeReadOnlyViolation, "TRUNCATE"},
		{"alter", "ALTER TABLE t ADD COLUMN x int", CodeReadOnlyViolation, "ALTER"},
		{"grant", "GRANT SELECT ON t TO u", CodeReadOnlyViolation, "GRANT"},
		{"copy", "COPY t TO '/tmp/x'", CodeReadOnlyViolation, "COPY"},
		{"begin", "BEGIN", CodeReadOnlyViolation, "BEGIN"},
		{"set", "SET statement_timeout = 0", CodeReadOnlyViolation, "SET"},
		{"do block", "DO $$ BEGIN PERFORM 1; END $$", CodeReadOnlyViolation, "DO"},
		{"call", "CALL proc()", CodeReadOnlyViolation, "CALL"},
		{"execute", "EXECUTE stmt", CodeReadOnlyViolation, "EXECUTE"},
		{"lock", "LOCK TABLE t", CodeReadOnlyViolation, "LOCK"},
		{"vacuum", "VACUUM t", CodeReadOnlyViolation, "VACUUM"},
		{"refresh", "REFRESH MATERIALIZED VIEW mv", CodeReadOnlyViolation, "REFRESH"},
		{"merge", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN DELETE", CodeReadOnlyViolation, "MERGE"},
		{"lowercase write", "delete from t where id = 1", CodeReadOnlyViolation, "DELETE"},
		// ---- rejected: multiple statements ----
		{"two statements", "SELECT 1; SELECT 2", CodeReadOnlyViolation, "multiple statements"},
		{"select then drop", "SELECT 1; DROP TABLE t; --", CodeReadOnlyViolation, "multiple statements"},
		{"double trailing semicolon", "SELECT 1;;", CodeReadOnlyViolation, "multiple statements"},
		{"union then update", "SELECT 1 UNION SELECT 2; UPDATE t SET x = 1", CodeReadOnlyViolation, "multiple statements"},
		{"show then reset", "SHOW ALL; RESET ALL", CodeReadOnlyViolation, "multiple statements"},
		{"table then drop", "TABLE t; DROP TABLE t", CodeReadOnlyViolation, "multiple statements"},
		{"values then delete", "VALUES (1); DELETE FROM t", CodeReadOnlyViolation, "multiple statements"},
		// ---- rejected: data-modifying CTEs, SELECT INTO, locking ----
		{"cte insert", "WITH x AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM x", CodeReadOnlyViolation, "INSERT"},
		{"cte update", "WITH x AS (UPDATE t SET a = 1 RETURNING *) SELECT 1", CodeReadOnlyViolation, "UPDATE"},
		{"cte delete", "WITH x AS (DELETE FROM t RETURNING *) SELECT 1", CodeReadOnlyViolation, "DELETE"},
		{"select into", "SELECT * INTO new_table FROM t", CodeReadOnlyViolation, "INTO"},
		{"select for update", "SELECT * FROM t FOR UPDATE", CodeReadOnlyViolation, "UPDATE"},
		// ---- rejected: blacklisted functions ----
		{"terminate backend", "SELECT pg_terminate_backend(123)", CodeReadOnlyViolation, "pg_terminate_backend"},
		{"terminate backend upper case", "SELECT PG_TERMINATE_BACKEND(123)", CodeReadOnlyViolation, "pg_terminate_backend"},
		{"schema qualified cancel", "SELECT pg_catalog.pg_cancel_backend(1)", CodeReadOnlyViolation, "pg_cancel_backend"},
		{"space before paren", "SELECT pg_sleep (5)", CodeReadOnlyViolation, "pg_sleep"},
		{"comment before paren", "SELECT pg_sleep/* c */(5)", CodeReadOnlyViolation, "pg_sleep"},
		{"newline before paren", "SELECT pg_sleep\n(5)", CodeReadOnlyViolation, "pg_sleep"},
		{"quoted function name", `SELECT "pg_sleep"(1)`, CodeReadOnlyViolation, "pg_sleep"},
		{"unicode quoted function name", `SELECT U&"pg_sleep"(1)`, CodeReadOnlyViolation, "pg_sleep"},
		{"read file", "SELECT pg_read_file('/etc/passwd')", CodeReadOnlyViolation, "pg_read_file"},
		{"read binary file mixed case", "SELECT PG_Read_Binary_File('x')", CodeReadOnlyViolation, "pg_read_binary_file"},
		{"ls dir", "SELECT * FROM pg_ls_dir('.')", CodeReadOnlyViolation, "pg_ls_dir"},
		{"ls waldir", "SELECT pg_ls_waldir()", CodeReadOnlyViolation, "pg_ls_waldir"},
		{"stat file", "SELECT pg_stat_file('x')", CodeReadOnlyViolation, "pg_stat_file"},
		{"dblink", "SELECT * FROM dblink('dbname=x', 'select 1') AS t(a int)", CodeReadOnlyViolation, "dblink"},
		{"dblink exec", "SELECT dblink_exec('x', 'delete from y')", CodeReadOnlyViolation, "dblink_exec"},
		{"dblink connect", "SELECT dblink_connect('x', 'dbname=y')", CodeReadOnlyViolation, "dblink_connect"},
		{"set config", "SELECT set_config('x', 'y', false)", CodeReadOnlyViolation, "set_config"},
		{"advisory lock", "SELECT pg_advisory_lock(1)", CodeReadOnlyViolation, "pg_advisory_lock"},
		{"advisory xact lock", "SELECT pg_advisory_xact_lock(1)", CodeReadOnlyViolation, "pg_advisory_xact_lock"},
		{"try advisory lock prefix", "SELECT pg_try_advisory_lock(1)", CodeReadOnlyViolation, "pg_try_advisory_lock"},
		{"notify", "SELECT pg_notify('c', 'p')", CodeReadOnlyViolation, "pg_notify"},
		{"lo import", "SELECT lo_import('/etc/passwd')", CodeReadOnlyViolation, "lo_import"},
		{"lo export", "SELECT lo_export(1, '/tmp/x')", CodeReadOnlyViolation, "lo_export"},
		{"reload conf", "SELECT pg_reload_conf()", CodeReadOnlyViolation, "pg_reload_conf"},
		{"rotate logfile", "SELECT pg_rotate_logfile()", CodeReadOnlyViolation, "pg_rotate_logfile"},
		{"switch wal", "SELECT pg_switch_wal()", CodeReadOnlyViolation, "pg_switch_wal"},
		{"restore point", "SELECT pg_create_restore_point('x')", CodeReadOnlyViolation, "pg_create_restore_point"},
		{"promote", "SELECT pg_promote()", CodeReadOnlyViolation, "pg_promote"},
		{"sleep for", "SELECT pg_sleep_for('1 minute')", CodeReadOnlyViolation, "pg_sleep_for"},
		{"sleep until", "SELECT pg_sleep_until(now())", CodeReadOnlyViolation, "pg_sleep_until"},
		{"stat reset prefix", "SELECT pg_stat_reset()", CodeReadOnlyViolation, "pg_stat_reset"},
		{"backup start", "SELECT pg_backup_start('label')", CodeReadOnlyViolation, "pg_backup_start"},
		{"nextval", "SELECT nextval('seq')", CodeReadOnlyViolation, "nextval"},
		{"blacklisted function in lateral", "SELECT * FROM t, LATERAL pg_sleep(1)", CodeReadOnlyViolation, "pg_sleep"},
		// ---- rejected: EXPLAIN of non-read statements ----
		{"explain analyze update", "EXPLAIN ANALYZE UPDATE t SET x = 1", CodeReadOnlyViolation, "UPDATE"},
		{"explain option analyze delete", "EXPLAIN (ANALYZE) DELETE FROM t", CodeReadOnlyViolation, "DELETE"},
		{"explain update without analyze", "EXPLAIN UPDATE t SET x = 1", CodeReadOnlyViolation, "UPDATE"},
		{"explain analyze blacklisted function", "EXPLAIN ANALYZE SELECT pg_sleep(1)", CodeReadOnlyViolation, "pg_sleep"},
		{"explain of explain", "EXPLAIN EXPLAIN SELECT 1", CodeReadOnlyViolation, "EXPLAIN may only explain"},
		{"explain show", "EXPLAIN SHOW all", CodeReadOnlyViolation, "EXPLAIN may only explain"},
		{"explain nothing", "EXPLAIN", CodeReadOnlyViolation, "EXPLAIN may only explain"},
		// ---- invalid input ----
		{"empty", "", CodeInvalidArgs, "empty"},
		{"whitespace only", "   \n\t ", CodeInvalidArgs, "empty"},
		{"only a comment", "-- only a comment", CodeInvalidArgs, "empty"},
		{"only a semicolon", ";", CodeInvalidArgs, "empty"},
		{"unterminated string", "SELECT 'unterminated", CodeInvalidArgs, "unterminated string"},
		{"unterminated escape string", "SELECT E'unterminated", CodeInvalidArgs, "unterminated string"},
		{"unterminated block comment", "SELECT /* unterminated", CodeInvalidArgs, "unterminated block comment"},
		{"unterminated dollar quote", "SELECT $$unterminated", CodeInvalidArgs, "unterminated dollar"},
		{"unterminated quoted identifier", `SELECT "unterminated`, CodeInvalidArgs, "unterminated quoted identifier"},
		{"starts with number", "1 + 1", CodeReadOnlyViolation, "must start with"},
		{"starts with string", "'abc'", CodeReadOnlyViolation, "must start with"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Check(tc.sql)
			code := guardCode(t, err)
			if code != tc.code {
				t.Fatalf("Check(%q) code=%q err=%v want code=%q", tc.sql, code, err, tc.code)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Check(%q) error %q does not mention %q", tc.sql, err.Error(), tc.want)
			}
		})
	}
	if len(cases) < 40 {
		t.Fatalf("guard table has only %d cases", len(cases))
	}
}

func TestPrepareNormalizesAndClassifies(t *testing.T) {
	cases := []struct {
		sql        string
		wantSQL    string
		wantKind   string
		cursorable bool
	}{
		{"SELECT 1; -- done", "SELECT 1", "SELECT", true},
		{"  select 1 ;", "select 1", "SELECT", true},
		{"SELECT 1 /* tail */", "SELECT 1 /* tail */", "SELECT", true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", "WITH x AS (SELECT 1) SELECT * FROM x", "WITH", true},
		{"TABLE t", "TABLE t", "TABLE", true},
		{"VALUES (1)", "VALUES (1)", "VALUES", true},
		{"(SELECT 1) UNION (SELECT 2)", "(SELECT 1) UNION (SELECT 2)", "SELECT", true},
		{"EXPLAIN SELECT 1", "EXPLAIN SELECT 1", "EXPLAIN", false},
		{"SHOW server_version;", "SHOW server_version", "SHOW", false},
	}
	for _, tc := range cases {
		stmt, err := Prepare(tc.sql)
		if err != nil {
			t.Fatalf("Prepare(%q): %v", tc.sql, err)
		}
		if stmt.SQL != tc.wantSQL || stmt.Kind != tc.wantKind || stmt.Cursorable() != tc.cursorable {
			t.Fatalf("Prepare(%q) = %+v cursorable=%v want sql=%q kind=%q cursorable=%v", tc.sql, stmt, stmt.Cursorable(), tc.wantSQL, tc.wantKind, tc.cursorable)
		}
	}
}

func TestGuardErrorsCarryEnvelopeFields(t *testing.T) {
	err := Check("SELECT pg_terminate_backend(1)")
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Code != CodeReadOnlyViolation || e.Status != 403 || e.Hint == "" || !strings.Contains(e.Message, "pg_terminate_backend") {
		t.Fatalf("unexpected guard error: %+v", e)
	}
	if !strings.HasPrefix(e.Error(), CodeReadOnlyViolation+": ") {
		t.Fatalf("Error() = %q", e.Error())
	}
}

func TestForbiddenFunctionsCoverSpecList(t *testing.T) {
	spec := []string{
		"pg_terminate_backend", "pg_cancel_backend", "pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file",
		"pg_ls_logdir", "pg_ls_waldir", "lo_import", "lo_export", "dblink", "dblink_exec", "dblink_connect", "pg_sleep",
		"pg_sleep_for", "pg_sleep_until", "pg_reload_conf", "pg_rotate_logfile", "set_config", "pg_advisory_lock",
		"pg_advisory_xact_lock", "pg_notify", "pg_switch_wal", "pg_create_restore_point", "pg_promote",
	}
	listed := map[string]bool{}
	for _, name := range ForbiddenFunctions() {
		listed[name] = true
	}
	for _, name := range spec {
		if !listed[name] {
			t.Fatalf("%s missing from ForbiddenFunctions()", name)
		}
		if !isForbiddenFunction(strings.ToUpper(name)) {
			t.Fatalf("%s not matched case-insensitively", name)
		}
	}
	for _, name := range []string{"pg_sleep_state", "count", "now", "current_setting", "pg_size_pretty", "lo_get", "pg_backend_pid"} {
		if isForbiddenFunction(name) {
			t.Fatalf("%s wrongly blacklisted", name)
		}
	}
}

func TestPresetsPassGuard(t *testing.T) {
	presets := Presets()
	if len(presets) < 20 {
		t.Fatalf("expected the full preset list, got %d", len(presets))
	}
	for _, p := range presets {
		stmt, err := Prepare(p.SQL)
		if err != nil {
			t.Fatalf("preset %s rejected by guard: %v\n%s", p.Name, err, p.SQL)
		}
		if strings.HasPrefix(p.Name, "explain") {
			if stmt.Kind != "EXPLAIN" {
				t.Fatalf("preset %s kind=%s", p.Name, stmt.Kind)
			}
			continue
		}
		if !stmt.Cursorable() {
			t.Fatalf("preset %s is not cursorable (kind=%s)", p.Name, stmt.Kind)
		}
	}
}
