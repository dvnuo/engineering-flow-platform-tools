package commands

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/pgsql"
	"engineering-flow-platform-tools/internal/testutil"
)

const testSecret = "secret-password-should-not-appear"

func writePgsqlConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func defaultConfig(t *testing.T) string {
	t.Helper()
	return writePgsqlConfig(t, "version: 1\npgsql:\n  default_instance: local\n  instances:\n    - name: local\n      host: db.example.test\n      port: 5433\n      database: app\n      username: readonly\n      password: "+testSecret+"\n      sslmode: verify-full\n      statement_timeout_seconds: 7\n      max_rows: 10\n    - name: other\n      host: other.example.test\n      database: other\n      username: ro\n")
}

func runCLI(t *testing.T, fake *pgsql.FakeExecutor, cfg, stdin string, args ...string) (map[string]any, string) {
	t.Helper()
	root := NewRootWithExecutor(fake)
	var b bytes.Buffer
	root.SetOut(&b)
	root.SetErr(&b)
	if stdin != "" {
		root.SetIn(strings.NewReader(stdin))
	}
	full := []string{"--json"}
	if cfg != "" {
		full = append(full, "--config", cfg)
	}
	root.SetArgs(append(full, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v out=%s", err, b.String())
	}
	var out map[string]any
	if err := json.Unmarshal(b.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v out=%s", err, b.String())
	}
	if strings.Contains(b.String(), testSecret) {
		t.Fatalf("secret leaked in output of %v: %s", args, b.String())
	}
	return out, b.String()
}

func requireOK(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("command failed: %#v", out)
	}
	data, _ := out["data"].(map[string]any)
	return data
}

func requireErr(t *testing.T, out map[string]any, code string) map[string]any {
	t.Helper()
	if ok, _ := out["ok"].(bool); ok {
		t.Fatalf("expected error %s, got success: %#v", code, out)
	}
	errObj, _ := out["error"].(map[string]any)
	if errObj["code"] != code {
		t.Fatalf("expected error code %s, got %#v", code, errObj)
	}
	return errObj
}

func authRow() *pgsql.RawResult {
	return pgsql.FakeResult(pgsql.Cols("user", "database", "server_version", "read_only", "in_recovery"), []any{"readonly", "app", "PostgreSQL 16.3", "on", false})
}

func TestInstanceConfigRoundTrip(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-really-der")}), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &pgsql.FakeExecutor{}

	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "add", "analytics", "--host", "db.example.test")), "invalid_args")
	data := requireOK(t, first(runCLI(t, fake, cfg, testSecret+"\n", "instance", "add", "analytics", "--host", "db.example.test", "--port", "5433", "--database", "analytics", "--username", "readonly", "--password-stdin", "--sslmode", "verify-full", "--ca-cert-file", caPath, "--statement-timeout-seconds", "45", "--max-rows", "500", "--default")))
	if data["added"] != true || data["default"] != true || data["auth_configured"] != true || data["ca_cert_configured"] != true || data["port"] != float64(5433) || data["sslmode"] != "verify-full" {
		t.Fatalf("unexpected add data: %#v", data)
	}
	loaded, err := config.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Pgsql.DefaultInstance != "analytics" || len(loaded.Pgsql.Instances) != 1 {
		t.Fatalf("config not saved: %+v", loaded.Pgsql)
	}
	in := loaded.Pgsql.Instances[0]
	if in.Host != "db.example.test" || in.Port != 5433 || in.Database != "analytics" || in.Username != "readonly" || in.Password != testSecret || in.SSLMode != "verify-full" || !strings.Contains(in.CACert, "BEGIN CERTIFICATE") || in.StatementTimeoutSeconds != 45 || in.MaxRows != 500 {
		t.Fatalf("saved instance mismatch: %+v", in)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "add", "analytics", "--host", "x", "--database", "y", "--username", "z")), "invalid_args")

	list := requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "list")))
	instances, _ := list["instances"].([]any)
	if len(instances) != 1 || list["default_instance"] != "analytics" {
		t.Fatalf("unexpected list: %#v", list)
	}
	get := requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "get", "analytics")))
	if get["name"] != "analytics" || get["default"] != true || get["statement_timeout_seconds"] != float64(45) || get["max_rows"] != float64(500) {
		t.Fatalf("unexpected get: %#v", get)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "get", "nope")), "not_found")

	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--sslmode", "bogus")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--port", "70000")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--max-rows", "0")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--ca-cert-file", filepath.Join(t.TempDir(), "missing.pem"))), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "update", "nope", "--port", "5432")), "not_found")
	upd := requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--port", "6432", "--sslmode", "require", "--enabled=false")))
	if upd["updated"] != true || upd["port"] != float64(6432) || upd["sslmode"] != "require" || upd["enabled"] != false || upd["auth_configured"] != true {
		t.Fatalf("unexpected update: %#v", upd)
	}
	loaded, _ = config.Load(cfg)
	if loaded.Pgsql.Instances[0].Password != testSecret || loaded.Pgsql.Instances[0].Port != 6432 || loaded.Pgsql.Instances[0].IsEnabled() {
		t.Fatalf("update lost fields: %+v", loaded.Pgsql.Instances[0])
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "auth", "test")), "instance_disabled")
	requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "update", "analytics", "--enabled=true")))

	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "default", "nope")), "not_found")
	requireOK(t, first(runCLI(t, fake, cfg, testSecret+"\n", "instance", "add", "second", "--host", "h2", "--database", "d2", "--username", "u2", "--password-stdin")))
	def := requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "default", "second")))
	if def["default_instance"] != "second" {
		t.Fatalf("default not set: %#v", def)
	}
	def = requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "default")))
	if def["default_instance"] != "second" {
		t.Fatalf("default not read: %#v", def)
	}

	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "remove", "second")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "instance", "remove", "nope", "--yes")), "not_found")
	rm := requireOK(t, first(runCLI(t, fake, cfg, "", "instance", "remove", "second", "--yes")))
	if rm["removed"] != true {
		t.Fatalf("unexpected remove: %#v", rm)
	}
	loaded, _ = config.Load(cfg)
	if len(loaded.Pgsql.Instances) != 1 || loaded.Pgsql.DefaultInstance != "" {
		t.Fatalf("remove did not clear default: %+v", loaded.Pgsql)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("instance commands must not touch the database: %+v", fake.Calls)
	}
}

func first(out map[string]any, _ string) map[string]any { return out }

func TestAuthLoginLogoutAndTest(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) { return authRow(), nil }}

	requireErr(t, first(runCLI(t, fake, cfg, "", "auth", "login")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "\n", "auth", "login", "--password-stdin")), "invalid_args")
	login := requireOK(t, first(runCLI(t, fake, cfg, "new-"+testSecret+"\r\n", "auth", "login", "--instance", "other", "--username", "svc", "--password-stdin")))
	if login["logged_in"] != true || login["username"] != "svc" {
		t.Fatalf("unexpected login: %#v", login)
	}
	loaded, _ := config.Load(cfg)
	if loaded.Pgsql.Instances[1].Password != "new-"+testSecret || loaded.Pgsql.Instances[1].Username != "svc" || loaded.Pgsql.Instances[0].Password != testSecret {
		t.Fatalf("login did not store the password on the right instance: %+v", loaded.Pgsql.Instances)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "auth", "logout")), "invalid_args")
	logout := requireOK(t, first(runCLI(t, fake, cfg, "", "auth", "logout", "--instance", "other", "--yes")))
	if logout["logged_out"] != true {
		t.Fatalf("unexpected logout: %#v", logout)
	}
	loaded, _ = config.Load(cfg)
	if loaded.Pgsql.Instances[1].Password != "" || loaded.Pgsql.Instances[1].Username != "svc" {
		t.Fatalf("logout must clear only the password: %+v", loaded.Pgsql.Instances[1])
	}

	out, _ := runCLI(t, fake, cfg, "", "auth", "test", "--verbose")
	data := requireOK(t, out)
	if data["authenticated"] != true || data["user"] != "readonly" || data["read_only"] != true || data["server_version"] != "PostgreSQL 16.3" || data["database"] != "app" || out["instance"] != "local" {
		t.Fatalf("unexpected auth test: %#v", data)
	}
	conn, _ := data["connection"].(map[string]any)
	if conn["host"] != "db.example.test" || conn["port"] != float64(5433) || conn["username"] != "readonly" || conn["sslmode"] != "verify-full" || conn["statement_timeout_seconds"] != float64(7) || conn["max_rows"] != float64(10) {
		t.Fatalf("unexpected connection description: %#v", conn)
	}
	if _, present := conn["password"]; present {
		t.Fatalf("connection description must not carry the password: %#v", conn)
	}
	call := fake.Calls[len(fake.Calls)-1]
	if call.SQL != pgsql.PresetAuthTest || call.Conn.Host != "db.example.test" || call.Conn.Port != 5433 || call.Conn.Database != "app" || call.Conn.Username != "readonly" || call.Conn.Password != testSecret || call.Conn.SSLMode != "verify-full" || call.Conn.StatementTimeout != 7*time.Second {
		t.Fatalf("executor received wrong connection params: %+v", call)
	}
	dry := requireOK(t, first(runCLI(t, fake, cfg, "", "auth", "test", "--dry-run")))
	if dry["dry_run"] != true || dry["sql"] != pgsql.PresetAuthTest {
		t.Fatalf("unexpected dry run: %#v", dry)
	}
}

func TestInstanceResolution(t *testing.T) {
	fake := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) { return authRow(), nil }}
	none := writePgsqlConfig(t, "version: 1\njenkins:\n  instances: []\n")
	requireErr(t, first(runCLI(t, fake, none, "", "auth", "test")), "no_instance_configured")
	twoNoDefault := writePgsqlConfig(t, "pgsql:\n  instances:\n    - name: a\n      host: h\n      database: d\n      username: u\n    - name: b\n      host: h\n      database: d\n      username: u\n")
	requireErr(t, first(runCLI(t, fake, twoNoDefault, "", "auth", "test")), "instance_required")
	requireErr(t, first(runCLI(t, fake, twoNoDefault, "", "auth", "test", "--instance", "missing")), "instance_required")
	out, _ := runCLI(t, fake, twoNoDefault, "", "auth", "test", "--instance", "b")
	if requireOK(t, out); out["instance"] != "b" {
		t.Fatalf("explicit instance not honored: %#v", out)
	}
	single := writePgsqlConfig(t, "pgsql:\n  instances:\n    - name: only\n      host: h\n      database: d\n      username: u\n")
	out, _ = runCLI(t, fake, single, "", "auth", "test")
	if requireOK(t, out); out["instance"] != "only" {
		t.Fatalf("single instance not auto-selected: %#v", out)
	}
	incomplete := writePgsqlConfig(t, "pgsql:\n  instances:\n    - name: broken\n      host: h\n")
	errObj := requireErr(t, first(runCLI(t, fake, incomplete, "", "auth", "test")), "config_error")
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "database") || !strings.Contains(msg, "username") {
		t.Fatalf("missing fields not reported: %#v", errObj)
	}
	requireErr(t, first(runCLI(t, fake, filepath.Join(t.TempDir(), "absent.yaml"), "", "auth", "test")), "config_missing")
	requireErr(t, first(runCLI(t, fake, filepath.Join(t.TempDir(), "absent.yaml"), "", "instance", "list")), "config_missing")
}

func TestQueryAgainstFake(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		rows := make([][]any, 0, call.Limit+1)
		for i := 0; i < call.Limit+1; i++ {
			rows = append(rows, []any{int64(i + 1), strings.Repeat("x", 30), []byte("blob")})
		}
		return pgsql.FakeResult([]pgsql.Column{{Name: "id", Type: "int8"}, {Name: "note", Type: "text"}, {Name: "raw", Type: "bytea"}}, rows...), nil
	}}
	out, _ := runCLI(t, fake, cfg, "", "query", "--sql", "SELECT id, note, raw FROM t WHERE id > $1; ", "--param", "5", "--limit", "50", "--timeout-sec", "3", "--max-cell-chars", "5")
	data := requireOK(t, out)
	if data["limit"] != float64(10) || data["limit_capped"] != true || data["row_count"] != float64(10) || data["rows_truncated"] != true || data["cells_truncated"] != true || data["statement_timeout_seconds"] != float64(3) {
		t.Fatalf("unexpected query data: %#v", data)
	}
	call := fake.Calls[0]
	if call.SQL != "SELECT id, note, raw FROM t WHERE id > $1" || call.Limit != 10 || len(call.Args) != 1 || call.Args[0] != "5" || call.Conn.StatementTimeout != 3*time.Second {
		t.Fatalf("unexpected executor call: %+v", call)
	}
	rows, _ := data["rows"].([]any)
	firstRow, _ := rows[0].(map[string]any)
	if note, _ := firstRow["note"].(string); !strings.HasPrefix(note, "xxxxx...") {
		t.Fatalf("cell not truncated: %#v", firstRow)
	}
	if raw, _ := firstRow["raw"].(map[string]any); raw["bytes"] != float64(4) {
		t.Fatalf("bytea not summarized: %#v", firstRow)
	}
	columns, _ := data["columns"].([]any)
	if len(columns) != 3 {
		t.Fatalf("columns missing: %#v", data["columns"])
	}

	out, _ = runCLI(t, fake, cfg, "", "query", "--sql", "SELECT 1", "--limit", "2", "--timeout-sec", "100")
	if requireOK(t, out); fake.Calls[1].Conn.StatementTimeout != 7*time.Second || fake.Calls[1].Limit != 2 {
		t.Fatalf("timeout must be capped by the instance: %+v", fake.Calls[1])
	}

	sqlFile := filepath.Join(t.TempDir(), "q.sql")
	if err := os.WriteFile(sqlFile, []byte("-- header\nSELECT 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	requireOK(t, first(runCLI(t, fake, cfg, "", "query", "--sql-file", sqlFile, "--limit", "1")))
	if fake.Calls[2].SQL != "-- header\nSELECT 2" {
		t.Fatalf("sql-file statement not normalized: %q", fake.Calls[2].SQL)
	}

	requireErr(t, first(runCLI(t, fake, cfg, "", "query", "--sql", "SELECT 1", "--sql-file", sqlFile)), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "query")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "query", "--sql-file", filepath.Join(t.TempDir(), "missing.sql"))), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "query", "--sql", "SELECT 1", "--limit", "0")), "invalid_args")
	if len(fake.Calls) != 3 {
		t.Fatalf("invalid invocations must not reach the executor: %d", len(fake.Calls))
	}
}

func TestQueryOutputFile(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		return pgsql.FakeResult(pgsql.Cols("id", "note"), []any{int64(1), strings.Repeat("y", 100)}, []any{int64(2), "b"}), nil
	}}
	csvPath := filepath.Join(t.TempDir(), "out.csv")
	data := requireOK(t, first(runCLI(t, fake, cfg, "", "query", "--sql", "SELECT id, note FROM t", "--limit", "5", "--output", csvPath, "--max-cell-chars", "3")))
	if data["path"] != csvPath || data["format"] != "csv" || data["row_count"] != float64(2) || data["rows_truncated"] != false || data["bytes"] == float64(0) {
		t.Fatalf("unexpected file data: %#v", data)
	}
	if _, present := data["rows"]; present {
		t.Fatalf("rows must not be returned with --output: %#v", data)
	}
	b, err := os.ReadFile(csvPath)
	if err != nil || !strings.HasPrefix(string(b), "id,note") || !strings.Contains(string(b), strings.Repeat("y", 100)) {
		t.Fatalf("csv not written in full: %v %q", err, string(b))
	}
	jsonPath := filepath.Join(t.TempDir(), "out.json")
	data = requireOK(t, first(runCLI(t, fake, cfg, "", "query", "--sql", "SELECT id, note FROM t", "--output", jsonPath)))
	if data["format"] != "json" {
		t.Fatalf("unexpected format: %#v", data)
	}
	var decoded map[string]any
	b, _ = os.ReadFile(jsonPath)
	if err := json.Unmarshal(b, &decoded); err != nil || decoded["row_count"] != float64(2) {
		t.Fatalf("json file invalid: %v %s", err, string(b))
	}
}

func TestQueryDryRunAndGuard(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{}
	out, _ := runCLI(t, fake, cfg, "", "query", "--sql", "SELECT 1; -- x", "--param", "a", "--dry-run")
	data := requireOK(t, out)
	if data["dry_run"] != true || data["sql"] != "SELECT 1" || data["kind"] != "SELECT" || data["limit"] != float64(10) || data["limit_capped"] != true {
		t.Fatalf("unexpected dry run: %#v", data)
	}
	conn, _ := data["connection"].(map[string]any)
	if conn["host"] != "db.example.test" || conn["database"] != "app" {
		t.Fatalf("dry run connection: %#v", conn)
	}
	if params, _ := data["params"].([]any); len(params) != 1 || params[0] != "a" {
		t.Fatalf("dry run params: %#v", data["params"])
	}
	for _, sql := range []string{"UPDATE t SET x = 1", "SELECT 1; DROP TABLE t", "SELECT pg_terminate_backend(1)", "WITH x AS (DELETE FROM t RETURNING 1) SELECT * FROM x", "SELECT * INTO backup FROM t"} {
		errObj := requireErr(t, first(runCLI(t, fake, cfg, "", "query", "--sql", sql)), "read_only_violation")
		if hint, _ := errObj["hint"].(string); hint == "" {
			t.Fatalf("guard errors carry a hint: %#v", errObj)
		}
	}
	requireErr(t, first(runCLI(t, fake, filepath.Join(t.TempDir(), "absent.yaml"), "", "query", "--sql", "DELETE FROM t")), "read_only_violation")
	requireErr(t, first(runCLI(t, fake, cfg, "", "query", "--sql", "   ")), "invalid_args")
	if len(fake.Calls) != 0 {
		t.Fatalf("dry-run and rejected statements must not reach the executor: %+v", fake.Calls)
	}
	stat := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "activity", "--dry-run")))
	if statements, _ := stat["statements"].([]any); len(statements) != 1 || stat["sql"] != pgsql.PresetStatActivity {
		t.Fatalf("stat dry run: %#v", stat)
	}
	describe := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "describe", "orders", "--dry-run")))
	if statements, _ := describe["statements"].([]any); len(statements) != 4 {
		t.Fatalf("describe dry run: %#v", describe)
	}
}

func TestErrorCodeMapping(t *testing.T) {
	cfg := defaultConfig(t)
	cases := []struct {
		err    *pgsql.Error
		status float64
	}{
		{&pgsql.Error{Code: pgsql.CodePermissionDenied, Message: "permission denied for table t (SQLSTATE 42501)", Hint: "ask", Status: 403, SQLState: "42501"}, 403},
		{&pgsql.Error{Code: pgsql.CodeReadOnlyViolation, Message: "cannot execute UPDATE in a read-only transaction (SQLSTATE 25006)", Status: 403}, 403},
		{&pgsql.Error{Code: pgsql.CodeQueryTimeout, Message: "canceling statement due to statement timeout (SQLSTATE 57014)", Status: 408}, 408},
		{&pgsql.Error{Code: pgsql.CodeAuthFailed, Message: "password authentication failed for user \"readonly\" (SQLSTATE 28P01)", Status: 401}, 401},
		{&pgsql.Error{Code: pgsql.CodeNetworkError, Message: "failed to connect to `user=readonly database=app`: dial error", Status: 502}, 502},
		{&pgsql.Error{Code: pgsql.CodeInvalidArgs, Message: "relation \"nope\" does not exist (SQLSTATE 42P01)", Status: 400}, 400},
		{&pgsql.Error{Code: pgsql.CodeNotSupported, Message: "nope (SQLSTATE 0A000)", Status: 400}, 400},
		{&pgsql.Error{Code: pgsql.CodeServerError, Message: "boom"}, 500},
	}
	for _, tc := range cases {
		fake := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) { return nil, tc.err }}
		out, _ := runCLI(t, fake, cfg, "", "query", "--sql", "SELECT 1")
		errObj := requireErr(t, out, tc.err.Code)
		if errObj["status"] != tc.status || !strings.Contains(errObj["message"].(string), strings.SplitN(tc.err.Message, " (", 2)[0]) {
			t.Fatalf("unexpected mapping for %s: %#v", tc.err.Code, errObj)
		}
		if tc.err.Hint != "" && errObj["hint"] != tc.err.Hint {
			t.Fatalf("hint lost: %#v", errObj)
		}
	}
	leaky := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) {
		return nil, &pgsql.Error{Code: pgsql.CodeNetworkError, Message: "failed with password=" + testSecret, Status: 502}
	}}
	runCLI(t, leaky, cfg, "", "auth", "test")
}

func TestSchemaCommands(t *testing.T) {
	cfg := defaultConfig(t)
	relation := pgsql.FakeResult(pgsql.Cols("oid", "schema", "name", "kind"), []any{int64(16400), "public", "orders", "table"})
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		switch call.SQL {
		case pgsql.PresetSchemaTables:
			return pgsql.FakeResult(pgsql.Cols("schema", "name", "kind"), []any{"public", "orders", "table"}), nil
		case pgsql.PresetRelation:
			if call.Args[0] == "missing" {
				return pgsql.FakeResult(pgsql.Cols("oid")), nil
			}
			return relation, nil
		case pgsql.PresetColumns:
			return pgsql.FakeResult([]pgsql.Column{{Name: "position", Type: "int2"}, {Name: "name", Type: "name"}, {Name: "type", Type: "text"}, {Name: "nullable", Type: "bool"}, {Name: "primary_key", Type: "bool"}}, []any{int16(1), "id", "bigint", false, true}, []any{int16(2), "status", "text", true, false}), nil
		case pgsql.PresetIndexes:
			return pgsql.FakeResult(pgsql.Cols("name", "definition"), []any{"orders_pkey", "CREATE UNIQUE INDEX orders_pkey ON public.orders USING btree (id)"}), nil
		case pgsql.PresetConstraints:
			return pgsql.FakeResult(pgsql.Cols("name", "type", "definition"), []any{"orders_pkey", "primary_key", "PRIMARY KEY (id)"}), nil
		}
		return nil, &pgsql.Error{Code: pgsql.CodeServerError, Message: "unexpected sql " + call.SQL, Status: 500}
	}}
	tables := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "tables")))
	if tables["schema"] != "public" || tables["row_count"] != float64(1) || fake.Calls[0].Args[0] != "public" || fake.Calls[0].Limit != 500 {
		t.Fatalf("unexpected schema tables: %#v %+v", tables, fake.Calls[0])
	}
	all := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "tables", "--all-schemas", "--limit", "7")))
	if all["schema"] != "*" || fake.Calls[1].Args[0] != "" || fake.Calls[1].Limit != 7 {
		t.Fatalf("all-schemas not applied: %+v", fake.Calls[1])
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "schema", "tables", "--limit", "0")), "invalid_args")

	describe := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "describe", "public.orders")))
	table, _ := describe["table"].(map[string]any)
	if table["name"] != "orders" || describe["column_count"] != float64(2) {
		t.Fatalf("unexpected describe: %#v", describe)
	}
	if pk, _ := describe["primary_key"].([]any); len(pk) != 1 || pk[0] != "id" {
		t.Fatalf("primary key not derived: %#v", describe["primary_key"])
	}
	if idx, _ := describe["indexes"].([]any); len(idx) != 1 {
		t.Fatalf("indexes missing: %#v", describe)
	}
	if cons, _ := describe["constraints"].([]any); len(cons) != 1 {
		t.Fatalf("constraints missing: %#v", describe)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "schema", "describe", "missing")), "not_found")
	requireErr(t, first(runCLI(t, fake, cfg, "", "schema", "describe", " ")), "invalid_args")

	indexes := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "indexes", "orders")))
	if rows, _ := indexes["indexes"].([]any); len(rows) != 1 || indexes["row_count"] != float64(1) {
		t.Fatalf("unexpected indexes: %#v", indexes)
	}
	if table, _ := indexes["table"].(map[string]any); table["schema"] != "public" {
		t.Fatalf("indexes table missing: %#v", indexes)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "schema", "indexes", "missing")), "not_found")

	// `schema <command>` still describes pgsql commands.
	meta := requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "query")))
	if meta["command"] != "query" {
		t.Fatalf("schema query should describe the command: %#v", meta)
	}
	meta = requireOK(t, first(runCLI(t, fake, cfg, "", "schema", "schema.describe")))
	if meta["usage"] != "pgsql schema describe <table>" {
		t.Fatalf("schema schema.describe: %#v", meta)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "schema", "nope")), "not_found")
}

func TestStatCommands(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		return pgsql.FakeResult(pgsql.Cols("pid", "state"), []any{int32(42), "active"}), nil
	}}
	activity := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "activity", "--state", "active", "--min-duration-sec", "5", "--limit", "9")))
	if activity["row_count"] != float64(1) || fake.Calls[0].SQL != pgsql.PresetStatActivity || fake.Calls[0].Args[0] != "active" || fake.Calls[0].Args[1] != "5" || fake.Calls[0].Limit != 9 {
		t.Fatalf("unexpected activity call: %#v %+v", activity, fake.Calls[0])
	}
	if filters, _ := activity["filters"].(map[string]any); filters["state"] != "active" || filters["min_duration_sec"] != float64(5) {
		t.Fatalf("filters missing: %#v", activity)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "stat", "activity", "--min-duration-sec", "-1")), "invalid_args")

	locks := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "locks", "--blocked-only")))
	if locks["blocked_only"] != true || fake.Calls[1].SQL != pgsql.PresetStatLocks || fake.Calls[1].Args[0] != "true" {
		t.Fatalf("unexpected locks call: %+v", fake.Calls[1])
	}
	requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "locks")))
	if fake.Calls[2].Args[0] != "false" {
		t.Fatalf("blocked-only default: %+v", fake.Calls[2])
	}

	repl := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "replication")))
	if fake.Calls[3].SQL != pgsql.PresetStatReplicationStatus || fake.Calls[4].SQL != pgsql.PresetStatReplication || repl["pid"] != float64(42) || repl["row_count"] != float64(1) {
		t.Fatalf("unexpected replication: %#v", repl)
	}

	tables := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "tables", "--schema", "app", "--sort", "seq_scan", "--limit", "5")))
	if tables["sort"] != "seq_scan" || tables["schema"] != "app" || fake.Calls[5].Args[0] != "app" || fake.Calls[5].Args[1] != "5" || !strings.Contains(fake.Calls[5].SQL, "ORDER BY t.seq_scan DESC") {
		t.Fatalf("unexpected stat tables call: %#v %+v", tables, fake.Calls[5])
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "stat", "tables", "--sort", "evil")), "invalid_args")
	if len(fake.Calls) != 6 {
		t.Fatalf("invalid sort must not reach the executor: %d", len(fake.Calls))
	}
}

func TestStatSlow(t *testing.T) {
	cfg := defaultConfig(t)
	var extension *pgsql.RawResult
	var firstErr *pgsql.Error
	fake := &pgsql.FakeExecutor{}
	fake.Handler = func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		if call.SQL == pgsql.PresetStatStatementsExtension {
			return extension, nil
		}
		if firstErr != nil && !strings.Contains(call.SQL, "total_time") {
			return nil, firstErr
		}
		return pgsql.FakeResult(pgsql.Cols("query_id", "calls"), []any{"1", int64(3)}), nil
	}

	extension = pgsql.FakeResult(pgsql.Cols("version", "schema"))
	missing := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "slow")))
	if missing["has_report"] != false || missing["reason"] == "" || len(fake.Calls) != 1 {
		t.Fatalf("unexpected has_report=false result: %#v calls=%d", missing, len(fake.Calls))
	}

	extension = pgsql.FakeResult(pgsql.Cols("version", "schema"), []any{"1.10", "public"})
	fake.Calls = nil
	report := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "slow", "--limit", "3", "--sort", "mean")))
	if report["has_report"] != true || report["extension_version"] != "1.10" || report["row_count"] != float64(1) || report["sort"] != "mean" || len(fake.Calls) != 2 || fake.Calls[1].Args[0] != "3" || !strings.Contains(fake.Calls[1].SQL, "mean_exec_time") {
		t.Fatalf("unexpected report: %#v %+v", report, fake.Calls)
	}

	firstErr = &pgsql.Error{Code: pgsql.CodeInvalidArgs, Message: "column s.total_exec_time does not exist", Status: 400, SQLState: "42703"}
	fake.Calls = nil
	legacy := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "slow")))
	if legacy["has_report"] != true || len(fake.Calls) != 3 || !strings.Contains(fake.Calls[2].SQL, "s.total_time") {
		t.Fatalf("legacy fallback not used: %#v %+v", legacy, fake.Calls)
	}

	firstErr = &pgsql.Error{Code: pgsql.CodeInvalidArgs, Message: "relation pg_stat_statements does not exist", Status: 400, SQLState: "42P01"}
	fake.Calls = nil
	hidden := requireOK(t, first(runCLI(t, fake, cfg, "", "stat", "slow")))
	if hidden["has_report"] != false || !strings.Contains(hidden["reason"].(string), "public") {
		t.Fatalf("hidden view not reported: %#v", hidden)
	}

	firstErr = &pgsql.Error{Code: pgsql.CodePermissionDenied, Message: "denied", Status: 403, SQLState: "42501"}
	requireErr(t, first(runCLI(t, fake, cfg, "", "stat", "slow")), "permission_denied")
	requireErr(t, first(runCLI(t, fake, cfg, "", "stat", "slow", "--sort", "bogus")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "stat", "slow", "--limit", "0")), "invalid_args")
}

func TestDBSize(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		if call.SQL == pgsql.PresetDBSize {
			return pgsql.FakeResult(pgsql.Cols("database", "bytes", "size"), []any{"app", int64(1024), "1024 bytes"}), nil
		}
		return pgsql.FakeResult(pgsql.Cols("schema", "name", "total_bytes"), []any{"public", "orders", int64(512)}), nil
	}}
	data := requireOK(t, first(runCLI(t, fake, cfg, "", "db", "size", "--top", "3")))
	if data["database"] != "app" || data["bytes"] != float64(1024) || len(fake.Calls) != 2 || fake.Calls[1].Args[0] != "3" || fake.Calls[1].Limit != 3 {
		t.Fatalf("unexpected db size: %#v %+v", data, fake.Calls)
	}
	if largest, _ := data["largest_relations"].([]any); len(largest) != 1 {
		t.Fatalf("largest relations missing: %#v", data)
	}
	fake.Calls = nil
	data = requireOK(t, first(runCLI(t, fake, cfg, "", "db", "size", "--top", "0")))
	if largest, _ := data["largest_relations"].([]any); len(largest) != 0 || len(fake.Calls) != 1 {
		t.Fatalf("--top 0 must skip the second query: %#v %d", data, len(fake.Calls))
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "db", "size", "--top", "-1")), "invalid_args")
}

func TestExplain(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(call pgsql.FakeCall) (*pgsql.RawResult, error) {
		if strings.Contains(call.SQL, "FORMAT JSON") {
			return pgsql.FakeResult([]pgsql.Column{{Name: "QUERY PLAN", Type: "json"}}, []any{[]any{map[string]any{"Plan": map[string]any{"Node Type": "Seq Scan"}}}}), nil
		}
		return pgsql.FakeResult(pgsql.Cols("QUERY PLAN"), []any{"Seq Scan on t  (cost=0.00..1.00 rows=1 width=4)"}, []any{"  Filter: (id = 1)"}), nil
	}}
	text := requireOK(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "SELECT * FROM t WHERE id = $1;", "--param", "1")))
	if text["plan_format"] != "text" || text["analyze"] != false || text["plan"] != "Seq Scan on t  (cost=0.00..1.00 rows=1 width=4)\n  Filter: (id = 1)" {
		t.Fatalf("unexpected text plan: %#v", text)
	}
	if lines, _ := text["plan_lines"].([]any); len(lines) != 2 {
		t.Fatalf("plan lines: %#v", text)
	}
	if fake.Calls[0].SQL != "EXPLAIN (FORMAT TEXT) SELECT * FROM t WHERE id = $1" || fake.Calls[0].Args[0] != "1" {
		t.Fatalf("unexpected explain call: %+v", fake.Calls[0])
	}
	js := requireOK(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "SELECT 1", "--analyze", "--plan-format", "json")))
	if js["analyze"] != true || js["executed"] != true || fake.Calls[1].SQL != "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT 1" {
		t.Fatalf("unexpected analyze call: %#v %+v", js, fake.Calls[1])
	}
	if plan, _ := js["plan"].([]any); len(plan) != 1 {
		t.Fatalf("json plan not returned as structure: %#v", js["plan"])
	}
	dry := requireOK(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "SELECT 1", "--dry-run")))
	if dry["sql"] != "EXPLAIN (FORMAT TEXT) SELECT 1" || dry["dry_run"] != true {
		t.Fatalf("unexpected explain dry run: %#v", dry)
	}
	requireErr(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "UPDATE t SET x = 1")), "read_only_violation")
	requireErr(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "EXPLAIN SELECT 1")), "read_only_violation")
	requireErr(t, first(runCLI(t, fake, cfg, "", "explain", "--sql", "SELECT 1", "--plan-format", "xml")), "invalid_args")
	requireErr(t, first(runCLI(t, fake, cfg, "", "explain")), "invalid_args")
	if len(fake.Calls) != 2 {
		t.Fatalf("rejected explains must not reach the executor: %d", len(fake.Calls))
	}
}

func TestEnvManagedConfig(t *testing.T) {
	t.Setenv("EFP_PGSQL_DEFAULT_INSTANCE", "envdb")
	t.Setenv("EFP_PGSQL_INSTANCES_0_NAME", "envdb")
	t.Setenv("EFP_PGSQL_INSTANCES_0_HOST", "env.example.test")
	t.Setenv("EFP_PGSQL_INSTANCES_0_DATABASE", "envapp")
	t.Setenv("EFP_PGSQL_INSTANCES_0_USERNAME", "envuser")
	t.Setenv("EFP_PGSQL_INSTANCES_0_PASSWORD", testSecret)
	t.Setenv("EFP_PGSQL_INSTANCES_0_SSLMODE", "require")
	fake := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) { return authRow(), nil }}
	out, _ := runCLI(t, fake, "", "", "auth", "test")
	requireOK(t, out)
	if out["instance"] != "envdb" || fake.Calls[0].Conn.Host != "env.example.test" || fake.Calls[0].Conn.Database != "envapp" || fake.Calls[0].Conn.Username != "envuser" || fake.Calls[0].Conn.Password != testSecret || fake.Calls[0].Conn.SSLMode != "require" || fake.Calls[0].Conn.Port != 5432 {
		t.Fatalf("env config not applied: %+v", fake.Calls[0])
	}
	requireErr(t, first(runCLI(t, fake, "", testSecret+"\n", "instance", "add", "x", "--host", "h", "--database", "d", "--username", "u", "--password-stdin")), "config_env_managed")
	requireErr(t, first(runCLI(t, fake, "", testSecret+"\n", "auth", "login", "--password-stdin")), "config_env_managed")
	list := requireOK(t, first(runCLI(t, fake, "", "", "instance", "list")))
	if instances, _ := list["instances"].([]any); len(instances) != 1 {
		t.Fatalf("env instances not listed: %#v", list)
	}
}

func TestCommandsSchemaHelpAndVersion(t *testing.T) {
	fake := &pgsql.FakeExecutor{}
	out, raw := runCLI(t, fake, "", "", "commands")
	obj := testutil.AssertOKEnvelope(t, []byte(raw))
	data, _ := obj["data"].(map[string]any)
	commands, _ := data["commands"].([]any)
	if len(commands) != 24 {
		t.Fatalf("expected 24 commands, got %d: %#v", len(commands), out)
	}
	risks := map[string]string{}
	for _, item := range commands {
		m, _ := item.(map[string]any)
		risks[m["name"].(string)] = m["risk"].(string)
	}
	for name, want := range map[string]string{"query": "read", "explain": "read", "stat.activity": "read", "db.size": "read", "instance.add": "write", "instance.update": "write", "instance.default": "write", "auth.login": "write", "instance.remove": "delete", "auth.logout": "delete"} {
		if risks[name] != want {
			t.Fatalf("%s risk=%s want %s", name, risks[name], want)
		}
	}
	help := requireOK(t, first(runCLI(t, fake, "", "", "help", "llm")))
	tips, _ := help["tips"].([]any)
	joined := ""
	for _, tip := range tips {
		joined += tip.(string) + "\n"
	}
	for _, want := range []string{"schema tables", "--limit", "SELECT *", "PII", "READ ONLY", "read-only database role", "--dry-run"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("help llm tips missing %q:\n%s", want, joined)
		}
	}
	schema := requireOK(t, first(runCLI(t, fake, "", "", "schema", "query")))
	flags, _ := schema["flags"].([]any)
	names := map[string]bool{}
	for _, f := range flags {
		m, _ := f.(map[string]any)
		names[m["name"].(string)] = true
	}
	for _, want := range []string{"sql", "sql-file", "param", "limit", "timeout-sec", "output", "max-cell-chars", "instance", "dry-run"} {
		if !names[want] {
			t.Fatalf("schema query missing flag %s: %#v", want, names)
		}
	}
	ver := requireOK(t, first(runCLI(t, fake, "", "", "version")))
	if _, ok := ver["version"]; !ok {
		t.Fatalf("version missing: %#v", ver)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("metadata commands must not touch the database")
	}
}

func TestPresetsPassGuardFromCommands(t *testing.T) {
	for _, p := range pgsql.Presets() {
		if err := pgsql.Check(p.SQL); err != nil {
			t.Fatalf("preset %s rejected: %v", p.Name, err)
		}
	}
}

func TestTableAndYAMLFormats(t *testing.T) {
	cfg := defaultConfig(t)
	fake := &pgsql.FakeExecutor{Handler: func(pgsql.FakeCall) (*pgsql.RawResult, error) { return authRow(), nil }}
	for _, format := range []string{"table", "yaml"} {
		root := NewRootWithExecutor(fake)
		var b bytes.Buffer
		root.SetOut(&b)
		root.SetErr(&b)
		root.SetArgs([]string{"--config", cfg, "--format", format, "auth", "test"})
		if err := root.Execute(); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if strings.Contains(b.String(), testSecret) || !strings.Contains(b.String(), "ok") {
			t.Fatalf("%s output unexpected: %s", format, b.String())
		}
		if format == "yaml" && !strings.Contains(b.String(), "user: readonly") {
			t.Fatalf("yaml output unexpected: %s", b.String())
		}
	}
}
