package catalog

// pgsqlCommands is the canonical command list of the pgsql binary; it must
// stay in sync with the cobra tree and docs/COMMAND_SPEC.md (see
// tests/command_coverage_test.go).
var pgsqlCommands = []string{
	"pgsql instance list", "pgsql instance get <name>", "pgsql instance add <name>", "pgsql instance update <name>", "pgsql instance remove <name>", "pgsql instance default [name]",
	"pgsql auth login", "pgsql auth logout", "pgsql auth test",
	"pgsql query", "pgsql explain",
	"pgsql schema tables", "pgsql schema describe <table>", "pgsql schema indexes <table>",
	"pgsql stat activity", "pgsql stat locks", "pgsql stat slow", "pgsql stat replication", "pgsql stat tables",
	"pgsql db size",
	"pgsql commands", "pgsql schema <command>", "pgsql help llm", "pgsql version",
}

// pgsqlExplicit carries per-command metadata (description, risk, example,
// flags) for the pgsql binary. Every command that talks to the database is a
// read: statements run inside a READ ONLY transaction behind the statement
// guard. Config writes (instance add/update/default, auth login) are "write"
// and config removals (instance remove, auth logout) are "delete".
func pgsqlExplicit(name string) (explicitMeta, bool) {
	common := []string{"instance", "config", "json", "format", "verbose"}
	read := append([]string{"dry-run"}, common...)
	del := append([]string{"yes"}, common...)
	instanceFlags := []string{"host", "port", "database", "username", "password-stdin", "sslmode", "ca-cert-file", "statement-timeout-seconds", "max-rows", "default", "config", "json", "format", "verbose"}
	sqlFlags := []string{"sql", "sql-file", "param", "timeout-sec"}
	items := map[string]explicitMeta{
		"instance.list":    {Description: "List configured PostgreSQL instances with the password blanked.", Flags: common, Risk: "read", Example: "pgsql instance list --json"},
		"instance.get":     {Description: "Show one configured PostgreSQL instance with the password blanked.", Flags: common, Required: []string{"name"}, Risk: "read", Example: "pgsql instance get analytics --json"},
		"instance.add":     {Description: "Add a PostgreSQL instance (host, port, database, read-only role, sslmode, CA bundle, timeout, row cap) to the shared EFP YAML config.", Flags: instanceFlags, Required: []string{"name", "host", "database", "username"}, Risk: "write", Example: "printf '%s\\n' \"$PGPASSWORD\" | pgsql instance add analytics --host db.example.test --port 5432 --database analytics --username readonly --password-stdin --sslmode verify-full --default --json"},
		"instance.update":  {Description: "Update connection fields, TLS settings, limits, or the enabled flag of a configured PostgreSQL instance.", Flags: append([]string{"enabled"}, instanceFlags...), Required: []string{"name"}, Risk: "write", Example: "pgsql instance update analytics --statement-timeout-seconds 60 --max-rows 1000 --json"},
		"instance.remove":  {Description: "Remove a PostgreSQL instance from the EFP config after confirmation.", Flags: del, Required: []string{"name", "yes"}, Risk: "delete", Example: "pgsql instance remove analytics --yes --json"},
		"instance.default": {Description: "Read or set the default PostgreSQL instance.", Flags: common, Risk: "write", Example: "pgsql instance default analytics --json"},
		"auth.login":       {Description: "Store the database role password (from stdin) for the selected PostgreSQL instance.", Flags: []string{"username", "password-stdin", "instance", "config", "json", "format", "verbose"}, Required: []string{"password-stdin"}, Risk: "write", Example: "printf '%s\\n' \"$PGPASSWORD\" | pgsql auth login --instance analytics --username readonly --password-stdin --json"},
		"auth.logout":      {Description: "Clear the stored password of the selected PostgreSQL instance after confirmation.", Flags: del, Required: []string{"yes"}, Risk: "delete", Example: "pgsql auth logout --instance analytics --yes --json"},
		"auth.test": {Description: "Connect and prove identity, server version, and that the session is READ ONLY (SELECT current_user, version(), current_setting('transaction_read_only')).", Flags: read, Risk: "read", Example: "pgsql auth test --json",
			WhenToUse: "First, to confirm the instance, role, and TLS settings work before running queries."},
		"query": {Description: "One guarded read statement (SELECT, WITH, TABLE, VALUES, SHOW) evaluated inside a READ ONLY transaction with a statement timeout; rows are objects keyed by column, capped by --limit and the instance max_rows, cells by --max-cell-chars; --output writes the full result to a JSON or CSV file.",
			Flags: append(append([]string{}, sqlFlags...), "limit", "output", "max-cell-chars", "dry-run", "instance", "config", "json", "format", "verbose"), Required: []string{"sql|sql-file"}, Risk: "read",
			Example:      "pgsql query --sql \"SELECT status, count(*) AS n FROM orders WHERE created_at > now() - interval '1 day' GROUP BY status\" --limit 50 --json",
			WhenToUse:    "After schema tables/describe, for aggregates and filtered lookups the preset commands do not cover.",
			WhenNotToUse: "For writes of any kind (rejected), for full-table dumps (use aggregates or --output with a WHERE clause), or when a stat/schema preset already answers the question."},
		"explain": {Description: "Show the planner's plan for a guarded read statement; --analyze executes it inside the READ ONLY transaction (then rolls back) and adds buffers.",
			Flags: append(append([]string{}, sqlFlags...), "analyze", "plan-format", "dry-run", "instance", "config", "json", "format", "verbose"), Required: []string{"sql|sql-file"}, Risk: "read",
			Example: "pgsql explain --sql \"SELECT * FROM orders WHERE customer_id = $1\" --param 42 --analyze --json"},
		"schema.tables":    {Description: "List tables, views, materialized views, partitioned and foreign tables in a schema with owner, estimated rows, and size.", Flags: append([]string{"schema", "all-schemas", "limit"}, read...), Risk: "read", Example: "pgsql schema tables --schema public --json", WhenToUse: "Before writing any query, to learn the real relation names."},
		"schema.describe":  {Description: "Describe one relation: columns with types, nullability, defaults and primary-key membership, plus indexes and constraints.", Flags: read, Required: []string{"table"}, Risk: "read", Example: "pgsql schema describe public.orders --json"},
		"schema.indexes":   {Description: "List the indexes of one relation with definition, uniqueness, validity, size, and scan count.", Flags: read, Required: []string{"table"}, Risk: "read", Example: "pgsql schema indexes public.orders --json"},
		"stat.activity":    {Description: "List sessions from pg_stat_activity with state, wait event, running time, blocking pids (blocked_by), and a truncated query text.", Flags: append([]string{"state", "min-duration-sec", "limit"}, read...), Risk: "read", Example: "pgsql stat activity --state active --min-duration-sec 5 --json", WhenToUse: "To find long-running statements and blocking chains during an incident."},
		"stat.locks":       {Description: "List pg_locks joined to sessions; --blocked-only keeps waiting locks and the sessions holding what they wait for.", Flags: append([]string{"blocked-only", "limit"}, read...), Risk: "read", Example: "pgsql stat locks --blocked-only --json"},
		"stat.slow":        {Description: "Top statements from pg_stat_statements by total, mean, max time, calls, or rows; returns has_report=false instead of an error when the extension is missing.", Flags: append([]string{"limit", "sort"}, read...), Risk: "read", Example: "pgsql stat slow --limit 20 --sort mean --json"},
		"stat.replication": {Description: "Recovery state, WAL positions, replication slots, and streaming replicas with byte and time lag from pg_stat_replication.", Flags: read, Risk: "read", Example: "pgsql stat replication --json"},
		"stat.tables":      {Description: "Per-table statistics from pg_stat_user_tables: live and dead tuples, dead percentage, sequential and index scans, modifications, vacuum and analyze times, and size.", Flags: append([]string{"schema", "sort", "limit"}, read...), Risk: "read", Example: "pgsql stat tables --sort n_dead_tup --limit 20 --json", WhenToUse: "To spot bloat, missing indexes (high seq_scan), and stale autovacuum."},
		"db.size":          {Description: "Database size, connection usage, server version, uptime, recovery state, and the largest relations.", Flags: append([]string{"top"}, read...), Risk: "read", Example: "pgsql db size --top 10 --json"},
		"commands":         {Description: "List pgsql commands with JSON-friendly metadata.", Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "pgsql commands --json"},
		"schema":           {Description: "Describe one pgsql command schema (arguments, flags, examples).", Flags: []string{"json", "format", "verbose"}, Required: []string{"command"}, Risk: "read", Example: "pgsql schema query --json"},
		"help.llm":         {Description: "Return concise agent guidance for pgsql: discovery order, limits, PII handling, and the read-only guarantees.", Flags: []string{"json", "format", "verbose"}, Risk: "read", Example: "pgsql help llm --json"},
		"version":          {Description: "Print pgsql version metadata.", Flags: common, Risk: "read", Example: "pgsql version --json"},
	}
	item, ok := items[name]
	return item, ok
}
