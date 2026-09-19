package pgsql

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const cursorName = "efp_pgsql_cursor"

// pgxExecutor is the real Executor: one connection per statement, a READ
// ONLY transaction with SET LOCAL timeouts, a NO SCROLL cursor so only
// limit+1 rows ever leave the server for SELECT-style statements, and an
// unconditional ROLLBACK.
type pgxExecutor struct{}

// NewExecutor returns the pgx-backed executor.
func NewExecutor() Executor { return pgxExecutor{} }

// SessionSetup returns the SET LOCAL statements issued at the start of every
// transaction.
func SessionSetup(timeout time.Duration) []string {
	secs := int(math.Ceil(timeout.Seconds()))
	if secs < 1 {
		secs = 1
	}
	return []string{
		fmt.Sprintf("SET LOCAL statement_timeout = '%ds'", secs),
		fmt.Sprintf("SET LOCAL idle_in_transaction_session_timeout = '%ds'", secs+5),
		"SET LOCAL default_transaction_read_only = on",
	}
}

// BuildDSN renders the keyword/value connection string. The password is
// never part of it (it is set on the parsed config) so no error message can
// echo it.
func BuildDSN(conn ConnParams, caPath string) string {
	port := conn.Port
	if port <= 0 {
		port = 5432
	}
	sslmode := strings.ToLower(strings.TrimSpace(conn.SSLMode))
	if sslmode == "" {
		sslmode = "require"
	}
	parts := []string{
		"host=" + dsnQuote(conn.Host),
		fmt.Sprintf("port=%d", port),
		"dbname=" + dsnQuote(conn.Database),
		"user=" + dsnQuote(conn.Username),
		"sslmode=" + dsnQuote(sslmode),
		fmt.Sprintf("connect_timeout=%d", int(ConnectTimeout.Seconds())),
		"application_name=" + ApplicationName,
	}
	if caPath != "" {
		parts = append(parts, "sslrootcert="+dsnQuote(caPath))
	}
	return strings.Join(parts, " ")
}

func dsnQuote(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}

// writeCACert stores a configured PEM bundle in a 0600 temp file for the
// duration of one connection.
func writeCACert(pem string) (string, func(), error) {
	if strings.TrimSpace(pem) == "" {
		return "", func() {}, nil
	}
	f, err := os.CreateTemp("", "efp-pgsql-ca-*.pem")
	if err != nil {
		return "", func() {}, &Error{Code: CodeConfigError, Message: "cannot write CA certificate temp file: " + err.Error(), Status: 500}
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := f.WriteString(pem); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, &Error{Code: CodeConfigError, Message: "cannot write CA certificate temp file: " + err.Error(), Status: 500}
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, &Error{Code: CodeConfigError, Message: "cannot write CA certificate temp file: " + err.Error(), Status: 500}
	}
	_ = os.Chmod(path, 0o600)
	return path, cleanup, nil
}

func (pgxExecutor) Query(ctx context.Context, conn ConnParams, sql string, args []any, limit int) (*RawResult, error) {
	if limit < 1 {
		limit = DefaultLimit
	}
	// Defense in depth: the guard runs again here so no caller can reach the
	// server with an unguarded statement.
	stmt, err := Prepare(sql)
	if err != nil {
		return nil, err
	}
	timeout := conn.EffectiveTimeout()
	caPath, cleanup, err := writeCACert(conn.CACert)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	cfg, err := pgx.ParseConfig(BuildDSN(conn, caPath))
	if err != nil {
		return nil, &Error{Code: CodeConfigError, Message: "invalid connection settings: " + sanitizeMessage(err.Error()), Status: 500}
	}
	if conn.Password != "" {
		cfg.Password = conn.Password
	}
	cfg.ConnectTimeout = ConnectTimeout
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["application_name"] = ApplicationName
	var notices []string
	cfg.OnNotice = func(_ *pgconn.PgConn, n *pgconn.Notice) {
		notices = append(notices, strings.TrimSpace(n.Severity+": "+n.Message))
	}
	ctx, cancel := context.WithTimeout(ctx, ConnectTimeout+2*timeout+10*time.Second)
	defer cancel()
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, classify(err)
	}
	defer func() {
		closeCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = c.Close(closeCtx)
	}()
	tx, err := c.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, classify(err)
	}
	defer func() {
		rbCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = tx.Rollback(rbCtx)
	}()
	for _, s := range SessionSetup(timeout) {
		if _, err := tx.Exec(ctx, s); err != nil {
			return nil, classify(err)
		}
	}
	start := time.Now()
	var rows pgx.Rows
	if stmt.Cursorable() {
		if _, err := tx.Exec(ctx, "DECLARE "+cursorName+" NO SCROLL CURSOR WITHOUT HOLD FOR "+stmt.SQL, args...); err != nil {
			return nil, classify(err)
		}
		rows, err = tx.Query(ctx, fmt.Sprintf("FETCH FORWARD %d FROM %s", limit+1, cursorName))
	} else {
		rows, err = tx.Query(ctx, stmt.SQL, args...)
	}
	if err != nil {
		return nil, classify(err)
	}
	fields := rows.FieldDescriptions()
	out := &RawResult{Columns: columnsFor(c.TypeMap(), fields), Rows: [][]any{}}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			rows.Close()
			return nil, classify(err)
		}
		out.Rows = append(out.Rows, vals)
		if len(out.Rows) > limit {
			break
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, classify(err)
	}
	out.Elapsed = time.Since(start)
	resolveUnknownTypes(ctx, tx, out.Columns, fields)
	out.Notices = notices
	return out, nil
}

func columnsFor(tm *pgtype.Map, fields []pgconn.FieldDescription) []Column {
	out := make([]Column, 0, len(fields))
	for _, fd := range fields {
		name := fmt.Sprintf("oid:%d", fd.DataTypeOID)
		if t, ok := tm.TypeForOID(fd.DataTypeOID); ok && t != nil {
			name = t.Name
		}
		out = append(out, Column{Name: fd.Name, Type: name})
	}
	return out
}

// resolveUnknownTypes replaces oid:N type names with pg_type names for
// custom types (enums, domains, composites). Failures are ignored.
func resolveUnknownTypes(ctx context.Context, tx pgx.Tx, cols []Column, fields []pgconn.FieldDescription) {
	var oids []string
	seen := map[uint32]bool{}
	for i, fd := range fields {
		if i < len(cols) && strings.HasPrefix(cols[i].Type, "oid:") && !seen[fd.DataTypeOID] {
			seen[fd.DataTypeOID] = true
			oids = append(oids, fmt.Sprintf("%d", fd.DataTypeOID))
		}
	}
	if len(oids) == 0 {
		return
	}
	rows, err := tx.Query(ctx, "SELECT oid::bigint, pg_catalog.format_type(oid, NULL) FROM pg_catalog.pg_type WHERE oid = ANY($1::oid[])", "{"+strings.Join(oids, ",")+"}")
	if err != nil {
		return
	}
	defer rows.Close()
	names := map[uint32]string{}
	for rows.Next() {
		var oid int64
		var name string
		if err := rows.Scan(&oid, &name); err != nil {
			return
		}
		names[uint32(oid)] = name
	}
	for i, fd := range fields {
		if i < len(cols) {
			if name, ok := names[fd.DataTypeOID]; ok {
				cols[i].Type = name
			}
		}
	}
}

// classify maps driver and server errors onto stable envelope codes.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var known *Error
	if errors.As(err, &known) {
		return known
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return classifyPgError(pgErr)
	}
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return &Error{Code: CodeNetworkError, Message: sanitizeMessage(err.Error()), Hint: "Check host, port, DNS, firewall, and sslmode for the instance, then retry.", Status: 502}
	}
	if errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err) {
		return &Error{Code: CodeQueryTimeout, Message: "timed out: " + sanitizeMessage(err.Error()), Hint: "Narrow the query with a WHERE clause or a smaller --limit, or raise statement_timeout_seconds on the instance.", Status: 408}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: CodeQueryTimeout, Message: "cancelled: " + sanitizeMessage(err.Error()), Status: 408}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return &Error{Code: CodeNetworkError, Message: sanitizeMessage(err.Error()), Hint: "Check host, port, DNS, firewall, and sslmode for the instance, then retry.", Status: 502}
	}
	return &Error{Code: CodeServerError, Message: sanitizeMessage(err.Error()), Status: 500}
}

func classifyPgError(pe *pgconn.PgError) *Error {
	msg := strings.TrimSpace(pe.Message)
	if d := strings.TrimSpace(pe.Detail); d != "" {
		msg += " (" + d + ")"
	}
	msg = sanitizeMessage(fmt.Sprintf("%s (SQLSTATE %s)", msg, pe.Code))
	mk := func(code string, status int, hint string) *Error {
		if hint == "" {
			hint = strings.TrimSpace(pe.Hint)
		}
		return &Error{Code: code, Message: msg, Hint: hint, Status: status, SQLState: pe.Code}
	}
	switch pe.Code {
	case "42501":
		return mk(CodePermissionDenied, 403, "The database role lacks a privilege on that object; ask for SELECT on the relation or query a view the role can read.")
	case "25006":
		return mk(CodeReadOnlyViolation, 403, "The statement tried to write inside the READ ONLY transaction; pgsql cannot modify data.")
	case "57014":
		return mk(CodeQueryTimeout, 408, "The statement hit statement_timeout; narrow it with a WHERE clause or a smaller --limit, or raise statement_timeout_seconds on the instance.")
	case "28P01", "28000":
		return mk(CodeAuthFailed, 401, "Store the password with pgsql auth login --password-stdin, and check the role's pg_hba.conf access for this host and sslmode.")
	case "3D000":
		return mk(CodeInvalidArgs, 400, "The instance database name does not exist on the server; fix it with pgsql instance update --database.")
	case "42P01", "42703":
		return mk(CodeInvalidArgs, 400, "Run pgsql schema tables --json and pgsql schema describe <table> --json to check relation and column names.")
	case "0A000":
		return mk(CodeNotSupported, 400, "")
	case "57P01", "57P02", "57P03":
		return mk(CodeNetworkError, 503, "The server is shutting down, restarting, or not accepting connections; retry later.")
	}
	class := pe.Code
	if len(class) > 2 {
		class = class[:2]
	}
	switch class {
	case "42", "22", "2B", "26", "2F", "34", "3F", "44", "0L", "0P":
		return mk(CodeInvalidArgs, 400, "Fix the statement; run pgsql schema describe <table> --json for column names and types.")
	case "08":
		return mk(CodeNetworkError, 502, "Check host, port, DNS, firewall, and sslmode for the instance, then retry.")
	case "28":
		return mk(CodeAuthFailed, 401, "")
	case "53":
		return mk(CodeServerError, 503, "The server is short on resources (connections, memory, disk); retry later or narrow the query.")
	case "40":
		return mk(CodeServerError, 500, "Transient serialization or deadlock failure; retry.")
	}
	return mk(CodeServerError, 500, "")
}

// sanitizeMessage collapses whitespace and bounds the length of a driver or
// server message; the output redaction layer additionally scrubs credentials.
func sanitizeMessage(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 600 {
		s = s[:600] + "..."
	}
	return s
}
