package pgsql

import (
	"fmt"
	"sort"
	"strings"
)

// Presets are the fixed statements behind auth test, schema *, stat *, and
// db size. They use only catalog views and functions readable by an ordinary
// role and every one of them passes the statement guard (see
// TestPresetsPassGuard). Positional parameters are always cast explicitly
// because the CLI sends them as text.

// Preset names a fixed statement for tests and --dry-run output.
type Preset struct {
	Name string
	SQL  string
}

const relkindCase = `CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'partitioned_table' WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized_view' WHEN 'f' THEN 'foreign_table' WHEN 'i' THEN 'index' WHEN 'S' THEN 'sequence' ELSE c.relkind::text END`

// PresetAuthTest proves the connection, identity, and READ ONLY mode.
const PresetAuthTest = `SELECT current_user AS "user", current_database() AS database, version() AS server_version,
       current_setting('transaction_read_only') AS read_only, pg_catalog.pg_is_in_recovery() AS in_recovery`

// PresetSchemaTables lists relations; $1 is a schema name, or an empty
// string for every non-system schema.
const PresetSchemaTables = `SELECT n.nspname AS schema, c.relname AS name,
       ` + relkindCase + ` AS kind,
       pg_catalog.pg_get_userbyid(c.relowner) AS owner,
       CASE WHEN c.relkind IN ('r', 'm', 'p') THEN c.reltuples::bigint END AS estimated_rows,
       CASE WHEN c.relkind IN ('r', 'm', 'p') THEN pg_catalog.pg_total_relation_size(c.oid) END AS total_bytes,
       pg_catalog.obj_description(c.oid, 'pg_class') AS comment
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND ($1::text = '' OR n.nspname = $1::text)
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
ORDER BY n.nspname, c.relname`

// PresetRelation resolves one relation by (optionally schema-qualified) name.
const PresetRelation = `SELECT c.oid::bigint AS oid, n.nspname AS schema, c.relname AS name,
       ` + relkindCase + ` AS kind,
       pg_catalog.pg_get_userbyid(c.relowner) AS owner,
       CASE WHEN c.relkind IN ('r', 'm', 'p') THEN c.reltuples::bigint END AS estimated_rows,
       CASE WHEN c.relkind IN ('r', 'm', 'p') THEN pg_catalog.pg_total_relation_size(c.oid) END AS total_bytes,
       pg_catalog.obj_description(c.oid, 'pg_class') AS comment
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.oid = pg_catalog.to_regclass($1::text)`

// PresetColumns lists the columns of one relation with nullability, default,
// primary-key membership, and comment.
const PresetColumns = `SELECT a.attnum AS position, a.attname AS name,
       pg_catalog.format_type(a.atttypid, a.atttypmod) AS type,
       NOT a.attnotnull AS nullable,
       pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS "default",
       EXISTS (SELECT 1 FROM pg_catalog.pg_index i WHERE i.indrelid = a.attrelid AND i.indisprimary AND a.attnum = ANY (i.indkey)) AS primary_key,
       pg_catalog.col_description(a.attrelid, a.attnum) AS comment
FROM pg_catalog.pg_attribute a
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE a.attrelid = pg_catalog.to_regclass($1::text) AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`

// PresetIndexes lists the indexes of one relation with definition and usage.
const PresetIndexes = `SELECT ic.relname AS name, pg_catalog.pg_get_indexdef(x.indexrelid) AS definition,
       x.indisprimary AS primary_key, x.indisunique AS "unique", x.indisvalid AS valid,
       pg_catalog.pg_relation_size(x.indexrelid) AS bytes, s.idx_scan AS scans
FROM pg_catalog.pg_index x
JOIN pg_catalog.pg_class ic ON ic.oid = x.indexrelid
LEFT JOIN pg_catalog.pg_stat_user_indexes s ON s.indexrelid = x.indexrelid
WHERE x.indrelid = pg_catalog.to_regclass($1::text)
ORDER BY ic.relname`

// PresetConstraints lists the constraints of one relation.
const PresetConstraints = `SELECT con.conname AS name,
       CASE con.contype WHEN 'p' THEN 'primary_key' WHEN 'f' THEN 'foreign_key' WHEN 'u' THEN 'unique' WHEN 'c' THEN 'check' WHEN 'x' THEN 'exclusion' WHEN 't' THEN 'trigger' ELSE con.contype::text END AS type,
       pg_catalog.pg_get_constraintdef(con.oid) AS definition
FROM pg_catalog.pg_constraint con
WHERE con.conrelid = pg_catalog.to_regclass($1::text)
ORDER BY con.contype, con.conname`

// PresetStatActivity lists client sessions; $1 filters by state (an empty
// string for all) and $2 keeps only statements running at least that many
// seconds (0 for all).
const PresetStatActivity = `SELECT a.pid, a.usename AS "user", a.datname AS db, a.application_name AS app, a.client_addr::text AS client,
       a.backend_type, a.state, a.wait_event_type, a.wait_event,
       a.backend_start, a.xact_start, a.query_start, a.state_change,
       CASE WHEN a.state = 'active' THEN EXTRACT(EPOCH FROM (now() - a.query_start))::bigint END AS duration_sec,
       EXTRACT(EPOCH FROM (now() - a.xact_start))::bigint AS xact_duration_sec,
       pg_catalog.pg_blocking_pids(a.pid) AS blocked_by,
       left(a.query, 500) AS query
FROM pg_catalog.pg_stat_activity a
WHERE a.datname IS NOT NULL AND a.pid <> pg_catalog.pg_backend_pid()
  AND ($1::text = '' OR a.state = $1::text)
  AND ($2::int <= 0 OR (a.state <> 'idle' AND a.query_start IS NOT NULL AND now() - a.query_start >= make_interval(secs => $2::int)))
ORDER BY a.query_start NULLS LAST, a.pid`

// PresetStatLocks lists locks joined to their sessions; $1 = true keeps only
// waiting locks and the sessions blocking them.
const PresetStatLocks = `SELECT l.pid, a.usename AS "user", a.datname AS db, l.locktype, l.mode, l.granted,
       CASE WHEN l.relation IS NOT NULL THEN l.relation::regclass::text END AS relation,
       l.transactionid::text AS transaction_id, l.virtualxid AS virtual_xid,
       a.state, a.wait_event_type, a.wait_event,
       pg_catalog.pg_blocking_pids(l.pid) AS blocked_by,
       EXTRACT(EPOCH FROM (now() - a.query_start))::bigint AS duration_sec,
       left(a.query, 300) AS query
FROM pg_catalog.pg_locks l
LEFT JOIN pg_catalog.pg_stat_activity a ON a.pid = l.pid
WHERE (l.pid IS NULL OR l.pid <> pg_catalog.pg_backend_pid())
  AND ($1::bool = false OR NOT l.granted OR l.pid IN (SELECT unnest(pg_catalog.pg_blocking_pids(w.pid)) FROM pg_catalog.pg_locks w WHERE NOT w.granted))
ORDER BY l.granted, l.pid, l.locktype`

// PresetStatStatementsExtension checks whether pg_stat_statements is installed.
const PresetStatStatementsExtension = `SELECT e.extversion AS version, n.nspname AS schema
FROM pg_catalog.pg_extension e
JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname = 'pg_stat_statements'`

// PresetStatReplicationStatus reports recovery state and WAL positions.
const PresetStatReplicationStatus = `SELECT pg_catalog.pg_is_in_recovery() AS in_recovery,
       CASE WHEN pg_catalog.pg_is_in_recovery() THEN pg_catalog.pg_last_wal_receive_lsn()::text ELSE pg_catalog.pg_current_wal_lsn()::text END AS current_lsn,
       CASE WHEN pg_catalog.pg_is_in_recovery() THEN pg_catalog.pg_last_wal_replay_lsn()::text END AS replay_lsn,
       CASE WHEN pg_catalog.pg_is_in_recovery() THEN EXTRACT(EPOCH FROM (now() - pg_catalog.pg_last_xact_replay_timestamp()))::bigint END AS replay_delay_sec,
       (SELECT count(*) FROM pg_catalog.pg_replication_slots) AS replication_slots,
       (SELECT count(*) FROM pg_catalog.pg_stat_replication) AS replicas`

// PresetStatReplication lists streaming replicas with lag.
const PresetStatReplication = `SELECT r.pid, r.usename AS "user", r.application_name AS app, r.client_addr::text AS client,
       r.state, r.sync_state, r.sync_priority,
       r.sent_lsn::text AS sent_lsn, r.write_lsn::text AS write_lsn, r.flush_lsn::text AS flush_lsn, r.replay_lsn::text AS replay_lsn,
       pg_catalog.pg_wal_lsn_diff(CASE WHEN pg_catalog.pg_is_in_recovery() THEN pg_catalog.pg_last_wal_replay_lsn() ELSE pg_catalog.pg_current_wal_lsn() END, r.replay_lsn)::bigint AS replay_lag_bytes,
       round(EXTRACT(EPOCH FROM r.write_lag)::numeric, 3) AS write_lag_sec,
       round(EXTRACT(EPOCH FROM r.flush_lag)::numeric, 3) AS flush_lag_sec,
       round(EXTRACT(EPOCH FROM r.replay_lag)::numeric, 3) AS replay_lag_sec,
       r.backend_start
FROM pg_catalog.pg_stat_replication r
ORDER BY r.pid`

// PresetDBSize summarizes the current database.
const PresetDBSize = `SELECT current_database() AS database,
       pg_catalog.pg_database_size(current_database()) AS bytes,
       pg_catalog.pg_size_pretty(pg_catalog.pg_database_size(current_database())) AS size,
       (SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname = current_database()) AS connections,
       current_setting('max_connections')::int AS max_connections,
       version() AS server_version,
       current_setting('server_version_num')::int AS server_version_num,
       pg_catalog.pg_postmaster_start_time() AS started_at,
       EXTRACT(EPOCH FROM (now() - pg_catalog.pg_postmaster_start_time()))::bigint AS uptime_sec,
       pg_catalog.pg_is_in_recovery() AS in_recovery`

// PresetLargestRelations lists the biggest tables; $1 is the row cap.
const PresetLargestRelations = `SELECT n.nspname AS schema, c.relname AS name,
       ` + relkindCase + ` AS kind,
       pg_catalog.pg_total_relation_size(c.oid) AS total_bytes,
       pg_catalog.pg_size_pretty(pg_catalog.pg_total_relation_size(c.oid)) AS total_size,
       pg_catalog.pg_relation_size(c.oid) AS table_bytes,
       pg_catalog.pg_indexes_size(c.oid) AS index_bytes
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'm', 'p')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
ORDER BY pg_catalog.pg_total_relation_size(c.oid) DESC
LIMIT $1::int`

// statTablesSorts whitelists --sort values for stat tables.
var statTablesSorts = map[string]string{
	"n_dead_tup":          "t.n_dead_tup",
	"n_live_tup":          "t.n_live_tup",
	"dead_pct":            "dead_pct",
	"seq_scan":            "t.seq_scan",
	"seq_tup_read":        "t.seq_tup_read",
	"idx_scan":            "t.idx_scan",
	"n_tup_ins":           "t.n_tup_ins",
	"n_tup_upd":           "t.n_tup_upd",
	"n_tup_del":           "t.n_tup_del",
	"n_mod_since_analyze": "t.n_mod_since_analyze",
	"total_bytes":         "pg_catalog.pg_total_relation_size(t.relid)",
	"last_autovacuum":     "t.last_autovacuum",
}

// StatTablesSorts returns the accepted --sort values for stat tables.
func StatTablesSorts() []string { return sortedKeys(statTablesSorts) }

// StatTablesSQL builds the pg_stat_user_tables preset; $1 is a schema name
// (an empty string for all), $2 the row cap.
func StatTablesSQL(sortBy string) (string, error) {
	if sortBy == "" {
		sortBy = "n_dead_tup"
	}
	expr, ok := statTablesSorts[strings.ToLower(strings.TrimSpace(sortBy))]
	if !ok {
		return "", invalidArgs("unsupported --sort value "+sortBy, "Use one of: "+strings.Join(StatTablesSorts(), ", ")+".")
	}
	return `SELECT t.schemaname AS schema, t.relname AS name, t.n_live_tup, t.n_dead_tup,
       CASE WHEN t.n_live_tup + t.n_dead_tup > 0 THEN round(100.0 * t.n_dead_tup / (t.n_live_tup + t.n_dead_tup), 1) END AS dead_pct,
       t.seq_scan, t.seq_tup_read, t.idx_scan, t.idx_tup_fetch,
       t.n_tup_ins, t.n_tup_upd, t.n_tup_del, t.n_tup_hot_upd, t.n_mod_since_analyze,
       t.last_vacuum, t.last_autovacuum, t.last_analyze, t.last_autoanalyze, t.vacuum_count, t.autovacuum_count,
       pg_catalog.pg_total_relation_size(t.relid) AS total_bytes
FROM pg_catalog.pg_stat_user_tables t
WHERE ($1::text = '' OR t.schemaname = $1::text)
ORDER BY ` + expr + ` DESC NULLS LAST, t.schemaname, t.relname
LIMIT $2::int`, nil
}

// statSlowSorts whitelists --sort values for stat slow (modern and legacy
// pg_stat_statements column names).
var statSlowSorts = map[string][2]string{
	"total": {"s.total_exec_time", "s.total_time"},
	"mean":  {"s.mean_exec_time", "s.mean_time"},
	"max":   {"s.max_exec_time", "s.max_time"},
	"calls": {"s.calls", "s.calls"},
	"rows":  {"s.rows", "s.rows"},
}

// StatSlowSorts returns the accepted --sort values for stat slow.
func StatSlowSorts() []string {
	out := make([]string, 0, len(statSlowSorts))
	for k := range statSlowSorts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// StatSlowSQL builds the pg_stat_statements preset; legacy selects the
// PostgreSQL 12 and older column names. $1 is the row cap.
func StatSlowSQL(sortBy string, legacy bool) (string, error) {
	if sortBy == "" {
		sortBy = "total"
	}
	cols, ok := statSlowSorts[strings.ToLower(strings.TrimSpace(sortBy))]
	if !ok {
		return "", invalidArgs("unsupported --sort value "+sortBy, "Use one of: "+strings.Join(StatSlowSorts(), ", ")+".")
	}
	total, mean, max, order := "s.total_exec_time", "s.mean_exec_time", "s.max_exec_time", cols[0]
	if legacy {
		total, mean, max, order = "s.total_time", "s.mean_time", "s.max_time", cols[1]
	}
	return fmt.Sprintf(`SELECT s.queryid::text AS query_id, r.rolname AS "user", d.datname AS db, s.calls,
       round(%s::numeric, 2) AS total_ms, round(%s::numeric, 2) AS mean_ms, round(%s::numeric, 2) AS max_ms,
       s.rows, s.shared_blks_hit, s.shared_blks_read,
       left(s.query, 500) AS query
FROM pg_stat_statements s
LEFT JOIN pg_catalog.pg_roles r ON r.oid = s.userid
LEFT JOIN pg_catalog.pg_database d ON d.oid = s.dbid
ORDER BY %s DESC NULLS LAST
LIMIT $1::int`, total, mean, max, order), nil
}

// ExplainSQL wraps a guarded statement in EXPLAIN. With analyze the statement
// is executed (inside the READ ONLY transaction, then rolled back), so
// BUFFERS is added for troubleshooting value.
func ExplainSQL(statement string, analyze bool, format string) string {
	format = strings.ToUpper(strings.TrimSpace(format))
	if format == "" {
		format = "TEXT"
	}
	opts := []string{}
	if analyze {
		opts = append(opts, "ANALYZE", "BUFFERS")
	}
	opts = append(opts, "FORMAT "+format)
	return "EXPLAIN (" + strings.Join(opts, ", ") + ") " + statement
}

// Presets returns every fixed statement (including each --sort variant) so a
// test can prove they all pass the guard.
func Presets() []Preset {
	out := []Preset{
		{Name: "auth.test", SQL: PresetAuthTest},
		{Name: "schema.tables", SQL: PresetSchemaTables},
		{Name: "schema.relation", SQL: PresetRelation},
		{Name: "schema.columns", SQL: PresetColumns},
		{Name: "schema.indexes", SQL: PresetIndexes},
		{Name: "schema.constraints", SQL: PresetConstraints},
		{Name: "stat.activity", SQL: PresetStatActivity},
		{Name: "stat.locks", SQL: PresetStatLocks},
		{Name: "stat.slow.extension", SQL: PresetStatStatementsExtension},
		{Name: "stat.replication.status", SQL: PresetStatReplicationStatus},
		{Name: "stat.replication", SQL: PresetStatReplication},
		{Name: "db.size", SQL: PresetDBSize},
		{Name: "db.size.largest", SQL: PresetLargestRelations},
		{Name: "explain.text", SQL: ExplainSQL("SELECT 1", false, "text")},
		{Name: "explain.analyze.json", SQL: ExplainSQL("SELECT 1", true, "json")},
	}
	for _, s := range StatTablesSorts() {
		sql, _ := StatTablesSQL(s)
		out = append(out, Preset{Name: "stat.tables." + s, SQL: sql})
	}
	for _, s := range StatSlowSorts() {
		modern, _ := StatSlowSQL(s, false)
		legacy, _ := StatSlowSQL(s, true)
		out = append(out, Preset{Name: "stat.slow." + s, SQL: modern}, Preset{Name: "stat.slow.legacy." + s, SQL: legacy})
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
