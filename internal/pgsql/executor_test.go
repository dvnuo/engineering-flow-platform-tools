package pgsql

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func fakeRows(n int) [][]any {
	rows := make([][]any, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, []any{int64(i + 1), "row"})
	}
	return rows
}

func TestExecuteTruncatesAtLimit(t *testing.T) {
	fake := &FakeExecutor{Handler: func(call FakeCall) (*RawResult, error) {
		return FakeResult(Cols("id", "name"), fakeRows(call.Limit+1)...), nil
	}}
	out, err := Execute(context.Background(), fake, ConnParams{}, "SELECT id, name FROM t; -- tail", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Calls) != 1 || fake.Calls[0].Limit != 3 || fake.Calls[0].SQL != "SELECT id, name FROM t" {
		t.Fatalf("unexpected executor call: %+v", fake.Calls)
	}
	if !out.RowsTruncated || out.RowCount != 3 || len(out.Rows) != 3 {
		t.Fatalf("expected truncation to 3 rows: %+v", out)
	}
	if out.Rows[2]["id"] != int64(3) || out.Rows[0]["name"] != "row" {
		t.Fatalf("rows not shaped as objects: %+v", out.Rows)
	}
	if out.ElapsedMs != 1 || out.Notices == nil || out.CellsTruncated {
		t.Fatalf("unexpected metadata: %+v", out)
	}
}

func TestExecuteExactLimitIsNotTruncated(t *testing.T) {
	fake := &FakeExecutor{Handler: func(call FakeCall) (*RawResult, error) {
		return FakeResult(Cols("id", "name"), fakeRows(call.Limit)...), nil
	}}
	out, err := Execute(context.Background(), fake, ConnParams{}, "SELECT 1", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if out.RowsTruncated || out.RowCount != 5 {
		t.Fatalf("unexpected truncation: %+v", out)
	}
}

func TestExecuteDefaultsLimit(t *testing.T) {
	fake := &FakeExecutor{}
	if _, err := Execute(context.Background(), fake, ConnParams{}, "SELECT 1", nil, 0); err != nil {
		t.Fatal(err)
	}
	if fake.Calls[0].Limit != DefaultLimit {
		t.Fatalf("limit=%d want %d", fake.Calls[0].Limit, DefaultLimit)
	}
}

func TestExecuteRejectsWritesBeforeCallingExecutor(t *testing.T) {
	fake := &FakeExecutor{}
	_, err := Execute(context.Background(), fake, ConnParams{}, "UPDATE t SET x = 1", nil, 10)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeReadOnlyViolation {
		t.Fatalf("expected read_only_violation, got %v", err)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("executor must not be called for rejected statements: %+v", fake.Calls)
	}
}

func TestExecutePropagatesExecutorErrors(t *testing.T) {
	fake := &FakeExecutor{Handler: func(FakeCall) (*RawResult, error) {
		return nil, &Error{Code: CodeQueryTimeout, Message: "boom", Status: 408}
	}}
	_, err := Execute(context.Background(), fake, ConnParams{}, "SELECT 1", nil, 10)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeQueryTimeout {
		t.Fatalf("expected query_timeout, got %v", err)
	}
}

func TestShapeNormalizesValuesAndDuplicateColumns(t *testing.T) {
	ts := time.Date(2026, 9, 19, 10, 30, 0, 0, time.UTC)
	num := pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true}
	uuid := [16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	raw := FakeResult(
		[]Column{{Name: "?column?", Type: "int4"}, {Name: "?column?", Type: "bytea"}, {Name: "ts", Type: "timestamptz"}, {Name: "n", Type: "numeric"}, {Name: "u", Type: "uuid"}, {Name: "f", Type: "float8"}, {Name: "arr", Type: "_int4"}, {Name: "iv", Type: "interval"}, {Name: "nul", Type: "text"}, {Name: "js", Type: "jsonb"}},
		[]any{int32(7), []byte("abcdef"), ts, num, uuid, math.NaN(), []any{int32(1), []byte("x")}, pgtype.Interval{Microseconds: 90 * 1e6, Valid: true}, nil, map[string]any{"k": []byte("v")}},
	)
	out := Shape(raw, 10)
	if len(out.Columns) != 10 || out.Columns[0].Name != "?column?" || out.Columns[1].Name != "?column?_2" {
		t.Fatalf("columns not de-duplicated: %+v", out.Columns)
	}
	row := out.Rows[0]
	if row["?column?"] != int32(7) {
		t.Fatalf("int kept: %#v", row["?column?"])
	}
	if b, _ := row["?column?_2"].(map[string]any); b["bytes"] != 6 {
		t.Fatalf("bytea not summarized: %#v", row["?column?_2"])
	}
	if row["ts"] != "2026-09-19T10:30:00Z" {
		t.Fatalf("time not RFC3339: %#v", row["ts"])
	}
	if row["n"] != json.Number("123.45") {
		t.Fatalf("numeric not exact json number: %#v", row["n"])
	}
	if row["u"] != "12345678-9abc-def0-1234-56789abcdef0" {
		t.Fatalf("uuid not formatted: %#v", row["u"])
	}
	if row["f"] != "NaN" {
		t.Fatalf("NaN not stringified: %#v", row["f"])
	}
	arr, _ := row["arr"].([]any)
	if len(arr) != 2 || arr[0] != int32(1) {
		t.Fatalf("array not normalized: %#v", row["arr"])
	}
	if nested, _ := arr[1].(map[string]any); nested["bytes"] != 1 {
		t.Fatalf("nested bytea not normalized: %#v", arr[1])
	}
	if s, _ := row["iv"].(string); !strings.Contains(s, "00:01:30") {
		t.Fatalf("interval not rendered as text: %#v", row["iv"])
	}
	if row["nul"] != nil {
		t.Fatalf("nil not preserved: %#v", row["nul"])
	}
	js, _ := row["js"].(map[string]any)
	if inner, _ := js["k"].(map[string]any); inner["bytes"] != 1 {
		t.Fatalf("json map not normalized: %#v", row["js"])
	}
	if _, err := json.Marshal(out.Data()); err != nil {
		t.Fatalf("output must be JSON encodable: %v", err)
	}
}

func TestNormalizeValueEdgeCases(t *testing.T) {
	if NormalizeValue(math.Inf(1)) != "Infinity" || NormalizeValue(math.Inf(-1)) != "-Infinity" || NormalizeValue(float32(1.5)) != 1.5 {
		t.Fatal("float edge cases")
	}
	if NormalizeValue(pgtype.Numeric{Valid: false}) != nil {
		t.Fatal("invalid numeric must be nil")
	}
	if NormalizeValue(pgtype.Numeric{NaN: true, Valid: true}) != "NaN" {
		t.Fatalf("numeric NaN: %#v", NormalizeValue(pgtype.Numeric{NaN: true, Valid: true}))
	}
	if NormalizeValue(net.ParseIP("10.0.0.1")) != "10.0.0.1" {
		t.Fatalf("stringer: %#v", NormalizeValue(net.ParseIP("10.0.0.1")))
	}
	var nilPtr *int
	if NormalizeValue(nilPtr) != nil {
		t.Fatal("nil pointer must be nil")
	}
	v := 3
	if NormalizeValue(&v) != 3 {
		t.Fatal("pointer must be dereferenced")
	}
	if got, _ := NormalizeValue([2]int16{1, 2}).([]any); len(got) != 2 || got[1] != int16(2) {
		t.Fatalf("array: %#v", got)
	}
	if got, _ := NormalizeValue(map[int]string{1: "a"}).(map[string]any); got["1"] != "a" {
		t.Fatalf("map: %#v", got)
	}
	if got, _ := NormalizeValue(struct{ A int }{A: 1}).(string); got != `{"A":1}` {
		t.Fatalf("struct: %#v", got)
	}
}

func TestTruncateCells(t *testing.T) {
	long := strings.Repeat("x", 50)
	out := Shape(FakeResult(Cols("a", "b", "c", "d"), []any{long, "short", []any{long}, int64(5)}), 10)
	out.TruncateCells(20)
	if !out.CellsTruncated {
		t.Fatal("expected cells_truncated")
	}
	a, _ := out.Rows[0]["a"].(string)
	if !strings.HasPrefix(a, strings.Repeat("x", 20)) || !strings.HasSuffix(a, truncatedSuffix) || len(a) != 20+len(truncatedSuffix) {
		t.Fatalf("string not truncated: %q", a)
	}
	if out.Rows[0]["b"] != "short" || out.Rows[0]["d"] != int64(5) {
		t.Fatalf("short cells changed: %+v", out.Rows[0])
	}
	c, _ := out.Rows[0]["c"].(string)
	if !strings.HasPrefix(c, `["xxx`) || !strings.HasSuffix(c, truncatedSuffix) {
		t.Fatalf("oversized array not stringified: %q", c)
	}
	untouched := Shape(FakeResult(Cols("a"), []any{long}), 10)
	untouched.TruncateCells(0)
	if untouched.CellsTruncated || untouched.Rows[0]["a"] != long {
		t.Fatal("max <= 0 must disable truncation")
	}
	multibyte := Shape(FakeResult(Cols("a"), []any{strings.Repeat("é", 30)}), 10)
	multibyte.TruncateCells(10)
	if m, _ := multibyte.Rows[0]["a"].(string); !strings.HasPrefix(m, strings.Repeat("é", 10)+truncatedSuffix) {
		t.Fatalf("rune-safe truncation failed: %q", m)
	}
}

func TestWriteFileJSONPreservesColumnOrderAndValues(t *testing.T) {
	out := Shape(FakeResult([]Column{{Name: "z", Type: "int4"}, {Name: "a", Type: "numeric"}}, []any{int64(1), pgtype.Numeric{Int: big.NewInt(25), Exp: -1, Valid: true}}, []any{int64(2), nil}), 10)
	path := filepath.Join(t.TempDir(), "nested", "out.json")
	res, err := WriteFile(out, path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != path || res.Bytes != int64(len(b)) || res.Format != "json" || res.RowCount != 2 || res.RowsTruncated {
		t.Fatalf("unexpected file output: %+v", res)
	}
	text := string(b)
	if strings.Index(text, `"z": 1`) > strings.Index(text, `"a": 2.5`) {
		t.Fatalf("column order not preserved:\n%s", text)
	}
	var decoded struct {
		Columns []Column         `json:"columns"`
		Rows    []map[string]any `json:"rows"`
		Count   int              `json:"row_count"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("file is not valid json: %v\n%s", err, text)
	}
	if len(decoded.Columns) != 2 || decoded.Count != 2 || decoded.Rows[0]["a"] != 2.5 || decoded.Rows[1]["a"] != nil {
		t.Fatalf("decoded file mismatch: %+v", decoded)
	}
}

func TestWriteFileCSV(t *testing.T) {
	out := Shape(FakeResult(Cols("name", "tags", "n"), []any{"a,b", []any{"x", "y"}, int64(3)}, []any{nil, map[string]any{"k": "v"}, json.Number("1.5")}), 10)
	path := filepath.Join(t.TempDir(), "out.CSV")
	res, err := WriteFile(out, path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Format != "csv" {
		t.Fatalf("format=%s", res.Format)
	}
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n")), "\n")
	if len(lines) != 3 || lines[0] != "name,tags,n" || lines[1] != `"a,b","[""x"",""y""]",3` || lines[2] != `,"{""k"":""v""}",1.5` {
		t.Fatalf("unexpected csv:\n%s", string(b))
	}
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	_, err := WriteFile(Shape(FakeResult(Cols("a")), 10), "  ")
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeInvalidArgs {
		t.Fatalf("expected invalid_args, got %v", err)
	}
}

func TestBuildDSNNeverContainsPassword(t *testing.T) {
	conn := ConnParams{Host: "db.example.test", Port: 6432, Database: "it's db", Username: `svc\user`, Password: "secret-password-should-not-appear", SSLMode: "verify-full"}
	dsn := BuildDSN(conn, `C:\tmp\ca.pem`)
	for _, want := range []string{"host='db.example.test'", "port=6432", `dbname='it\'s db'`, `user='svc\\user'`, "sslmode='verify-full'", "connect_timeout=10", "application_name=efp-pgsql", `sslrootcert='C:\\tmp\\ca.pem'`} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("dsn %q missing %q", dsn, want)
		}
	}
	if strings.Contains(dsn, "secret-password") || strings.Contains(dsn, "password") {
		t.Fatalf("dsn leaks the password: %q", dsn)
	}
	defaults := BuildDSN(ConnParams{Host: "h", Database: "d", Username: "u"}, "")
	if !strings.Contains(defaults, "port=5432") || !strings.Contains(defaults, "sslmode='require'") || strings.Contains(defaults, "sslrootcert") {
		t.Fatalf("defaults not applied: %q", defaults)
	}
}

func TestWriteCACertTempFile(t *testing.T) {
	path, cleanup, err := writeCACert("")
	if err != nil || path != "" {
		t.Fatalf("empty cert must not create a file: %q %v", path, err)
	}
	cleanup()
	pem := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----\n"
	path, cleanup, err = writeCACert(pem)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != pem {
		t.Fatalf("temp file content mismatch: %v %q", err, string(b))
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove %s", path)
	}
}

func TestSessionSetup(t *testing.T) {
	got := SessionSetup(30 * time.Second)
	want := []string{"SET LOCAL statement_timeout = '30s'", "SET LOCAL idle_in_transaction_session_timeout = '35s'", "SET LOCAL default_transaction_read_only = on"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v", got)
	}
	if SessionSetup(0)[0] != "SET LOCAL statement_timeout = '1s'" || SessionSetup(1500 * time.Millisecond)[0] != "SET LOCAL statement_timeout = '2s'" {
		t.Fatalf("rounding: %v %v", SessionSetup(0), SessionSetup(1500*time.Millisecond))
	}
	if (ConnParams{}).EffectiveTimeout() != DefaultStatementTimeout || (ConnParams{StatementTimeout: time.Second}).EffectiveTimeout() != time.Second {
		t.Fatal("effective timeout")
	}
}

func TestClassifyErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   string
		status int
	}{
		{"permission", &pgconn.PgError{Code: "42501", Message: "permission denied for table t"}, CodePermissionDenied, 403},
		{"read only", &pgconn.PgError{Code: "25006", Message: "cannot execute UPDATE in a read-only transaction"}, CodeReadOnlyViolation, 403},
		{"timeout", &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}, CodeQueryTimeout, 408},
		{"auth", &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}, CodeAuthFailed, 401},
		{"auth class", &pgconn.PgError{Code: "28000", Message: "no pg_hba.conf entry"}, CodeAuthFailed, 401},
		{"auth other", &pgconn.PgError{Code: "28001", Message: "invalid authorization specification"}, CodeAuthFailed, 401},
		{"missing table", &pgconn.PgError{Code: "42P01", Message: `relation "nope" does not exist`}, CodeInvalidArgs, 400},
		{"missing column", &pgconn.PgError{Code: "42703", Message: `column "nope" does not exist`}, CodeInvalidArgs, 400},
		{"syntax", &pgconn.PgError{Code: "42601", Message: "syntax error"}, CodeInvalidArgs, 400},
		{"bad param", &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type integer"}, CodeInvalidArgs, 400},
		{"missing database", &pgconn.PgError{Code: "3D000", Message: `database "x" does not exist`}, CodeInvalidArgs, 400},
		{"not supported", &pgconn.PgError{Code: "0A000", Message: "feature not supported"}, CodeNotSupported, 400},
		{"connection class", &pgconn.PgError{Code: "08006", Message: "connection failure"}, CodeNetworkError, 502},
		{"shutdown", &pgconn.PgError{Code: "57P01", Message: "terminating connection due to administrator command"}, CodeNetworkError, 503},
		{"resources", &pgconn.PgError{Code: "53300", Message: "too many connections"}, CodeServerError, 503},
		{"serialization", &pgconn.PgError{Code: "40001", Message: "could not serialize access"}, CodeServerError, 500},
		{"internal", &pgconn.PgError{Code: "XX000", Message: "internal error"}, CodeServerError, 500},
		{"short code", &pgconn.PgError{Code: "X", Message: "weird"}, CodeServerError, 500},
		{"deadline", context.DeadlineExceeded, CodeQueryTimeout, 408},
		{"cancelled", context.Canceled, CodeQueryTimeout, 408},
		{"net op error", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, CodeNetworkError, 502},
		{"dns", &net.DNSError{Err: "no such host", Name: "db.example.test"}, CodeNetworkError, 502},
		{"known error passthrough", &Error{Code: CodeConfigError, Message: "x", Status: 500}, CodeConfigError, 500},
		{"generic", errors.New("something odd"), CodeServerError, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var e *Error
			if !errors.As(classify(tc.err), &e) {
				t.Fatalf("classify returned %T", classify(tc.err))
			}
			if e.Code != tc.code || e.Status != tc.status {
				t.Fatalf("got code=%s status=%d (%s) want %s/%d", e.Code, e.Status, e.Message, tc.code, tc.status)
			}
			var pgErr *pgconn.PgError
			if errors.As(tc.err, &pgErr) {
				if e.SQLState != pgErr.Code || !strings.Contains(e.Message, "SQLSTATE "+pgErr.Code) || !strings.Contains(e.Message, pgErr.Message) {
					t.Fatalf("server error not surfaced: %+v", e)
				}
			}
		})
	}
	if classify(nil) != nil {
		t.Fatal("classify(nil) must be nil")
	}
	withDetail := classify(&pgconn.PgError{Code: "42501", Message: "denied", Detail: "needs SELECT", Hint: "ask a DBA"}).(*Error)
	if !strings.Contains(withDetail.Message, "(needs SELECT)") || withDetail.Hint == "" {
		t.Fatalf("detail/hint lost: %+v", withDetail)
	}
	longMsg := sanitizeMessage(strings.Repeat("a \n", 500))
	if len(longMsg) > 610 || strings.Contains(longMsg, "\n") {
		t.Fatalf("sanitizeMessage did not bound/collapse: %d", len(longMsg))
	}
}

func TestExplainAndSortBuilders(t *testing.T) {
	if got := ExplainSQL("SELECT 1", false, ""); got != "EXPLAIN (FORMAT TEXT) SELECT 1" {
		t.Fatalf("explain text: %q", got)
	}
	if got := ExplainSQL("SELECT 1", true, "json"); got != "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT 1" {
		t.Fatalf("explain analyze: %q", got)
	}
	if _, err := StatTablesSQL("bogus; DROP TABLE t"); err == nil {
		t.Fatal("unknown sort must be rejected")
	}
	sql, err := StatTablesSQL("")
	if err != nil || !strings.Contains(sql, "ORDER BY t.n_dead_tup DESC") {
		t.Fatalf("default sort: %v %q", err, sql)
	}
	if _, err := StatSlowSQL("evil", false); err == nil {
		t.Fatal("unknown slow sort must be rejected")
	}
	modern, _ := StatSlowSQL("mean", false)
	legacy, _ := StatSlowSQL("mean", true)
	if !strings.Contains(modern, "s.mean_exec_time") || strings.Contains(modern, "s.mean_time") {
		t.Fatalf("modern columns: %q", modern)
	}
	if !strings.Contains(legacy, "ORDER BY s.mean_time DESC") || strings.Contains(legacy, "exec_time") {
		t.Fatalf("legacy columns: %q", legacy)
	}
	if len(StatTablesSorts()) < 5 || len(StatSlowSorts()) != 5 {
		t.Fatal("sort lists")
	}
}

func TestOutputColumnHelper(t *testing.T) {
	out := Shape(FakeResult(Cols("a"), []any{"x"}, []any{"y"}), 10)
	if got := out.Column("a"); len(got) != 2 || got[1] != "y" {
		t.Fatalf("Column(): %v", got)
	}
	if empty := Shape(nil, 10); len(empty.Rows) != 0 || empty.Columns == nil || empty.Notices == nil {
		t.Fatalf("nil raw must shape to empty: %+v", empty)
	}
}
