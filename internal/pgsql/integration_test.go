package pgsql

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// integrationConn builds ConnParams from PGSQL_TEST_DSN (a libpq keyword
// string or URL) and skips the test when it is unset.
func integrationConn(t *testing.T) ConnParams {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("PGSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("PGSQL_TEST_DSN not set; skipping real PostgreSQL integration test")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("PGSQL_TEST_DSN is not a valid connection string: %v", err)
	}
	sslmode := "prefer"
	if cfg.TLSConfig == nil {
		sslmode = "disable"
	}
	return ConnParams{Host: cfg.Host, Port: int(cfg.Port), Database: cfg.Database, Username: cfg.User, Password: cfg.Password, SSLMode: sslmode, StatementTimeout: 5 * time.Second}
}

func TestIntegrationReadOnlySession(t *testing.T) {
	conn := integrationConn(t)
	exec := NewExecutor()
	ctx := context.Background()

	out, err := Execute(ctx, exec, conn, PresetAuthTest, nil, 5)
	if err != nil {
		t.Fatalf("auth test preset failed: %v", err)
	}
	if out.RowCount != 1 || out.Rows[0]["read_only"] != "on" || out.Rows[0]["user"] == "" {
		t.Fatalf("unexpected auth test row: %+v", out.Rows)
	}

	// The guard stops writes before the server sees them.
	if _, err := Execute(ctx, exec, conn, "UPDATE pg_catalog.pg_class SET relname = relname WHERE false", nil, 5); err == nil || !strings.Contains(err.Error(), CodeReadOnlyViolation) {
		t.Fatalf("guard should reject UPDATE: %v", err)
	}
	if _, err := Execute(ctx, exec, conn, "SELECT pg_terminate_backend(1)", nil, 5); err == nil || !strings.Contains(err.Error(), CodeReadOnlyViolation) {
		t.Fatalf("guard should reject pg_terminate_backend: %v", err)
	}

	// The READ ONLY transaction is the second layer: bypass the guard's
	// statement-type check by asking the server to run a write through the
	// executor's own guard-free path (a CTE the server rejects at execution).
	_, err = exec.Query(ctx, conn, "SELECT 1", nil, 5)
	if err != nil {
		t.Fatalf("plain select through executor: %v", err)
	}
	var e *Error
	_, err = runUnguarded(ctx, conn, "UPDATE pg_catalog.pg_class SET relname = relname WHERE false")
	if !errors.As(err, &e) || (e.Code != CodeReadOnlyViolation && e.Code != CodePermissionDenied) {
		t.Fatalf("server should reject UPDATE inside READ ONLY transaction: %v", err)
	}

	// rows_truncated via limit+1 fetch.
	series, err := Execute(ctx, exec, conn, "SELECT generate_series(1, 10) AS n", nil, 3)
	if err != nil {
		t.Fatalf("generate_series: %v", err)
	}
	if !series.RowsTruncated || series.RowCount != 3 {
		t.Fatalf("expected truncation: %+v", series)
	}

	// statement_timeout maps to query_timeout (pg_sleep is blacklisted, so go
	// around the guard for this one server-side check).
	short := conn
	short.StatementTimeout = time.Second
	_, err = runUnguarded(ctx, short, "SELECT pg_sleep(3)")
	if !errors.As(err, &e) || e.Code != CodeQueryTimeout {
		t.Fatalf("expected query_timeout, got %v", err)
	}

	// Parameters are sent as text and coerced by the server.
	params, err := Execute(ctx, exec, conn, "SELECT $1::int + 1 AS n, $2::text AS s", []any{"41", "x"}, 5)
	if err != nil {
		t.Fatalf("params: %v", err)
	}
	if params.Rows[0]["n"] != int32(42) || params.Rows[0]["s"] != "x" {
		t.Fatalf("param row: %+v", params.Rows)
	}
}

// runUnguarded opens the same READ ONLY session the executor uses but skips
// the statement guard, so integration tests can prove the server-side layer.
func runUnguarded(ctx context.Context, conn ConnParams, sql string) (*RawResult, error) {
	cfg, err := pgx.ParseConfig(BuildDSN(conn, ""))
	if err != nil {
		return nil, err
	}
	cfg.Password = conn.Password
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, classify(err)
	}
	defer c.Close(ctx)
	tx, err := c.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, classify(err)
	}
	defer tx.Rollback(ctx)
	for _, s := range SessionSetup(conn.EffectiveTimeout()) {
		if _, err := tx.Exec(ctx, s); err != nil {
			return nil, classify(err)
		}
	}
	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		return nil, classify(err)
	}
	return &RawResult{}, nil
}
