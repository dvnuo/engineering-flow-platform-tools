# PostgreSQL CLI Instructions

Use `pgsql` for read-only PostgreSQL access from Bash, PowerShell, or Windows cmd. It is a terminal-invoked CLI binary, not a psql wrapper, MCP server, or runtime built-in tool. It cannot modify data: every statement runs inside a `READ ONLY` transaction with a statement timeout behind a statement guard, and the database role configured for the instance should itself be read-only.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Managed runtimes: EFP_-prefixed environment variables derived from the config shape (`EFP_PGSQL_DEFAULT_INSTANCE`, `EFP_PGSQL_INSTANCES_0_NAME`, `EFP_PGSQL_INSTANCES_0_HOST`, `EFP_PGSQL_INSTANCES_0_PORT`, `EFP_PGSQL_INSTANCES_0_DATABASE`, `EFP_PGSQL_INSTANCES_0_USERNAME`, `EFP_PGSQL_INSTANCES_0_PASSWORD`, `EFP_PGSQL_INSTANCES_0_SSLMODE`, `EFP_PGSQL_INSTANCES_0_CA_CERT`, `EFP_PGSQL_INSTANCES_0_STATEMENT_TIMEOUT_SECONDS`, `EFP_PGSQL_INSTANCES_0_MAX_ROWS`); read-only — `instance add/update/remove/default` and `auth login/logout` then require an explicit `--config` path
- Node: `pgsql.default_instance` and `pgsql.instances` (`name`, `host`, `port`, `database`, `username`, `password`, `sslmode`, `ca_cert`, `statement_timeout_seconds`, `max_rows`, `enabled`)

Use `--instance <name>` when multiple databases are configured.

## Discovery

```bash
pgsql commands --json
pgsql schema query --json
pgsql help llm --json
pgsql auth test --json
```

`auth test` returns `{authenticated, user, database, server_version, read_only, in_recovery}`; `read_only` must be `true`.

## Common Workflows

Learn the schema before writing SQL (never guess table or column names):

```bash
pgsql schema tables --json
pgsql schema tables --all-schemas --json
pgsql schema describe public.orders --json
pgsql schema indexes public.orders --json
```

Run a bounded read query. Always pass `--limit`; prefer aggregates and `WHERE` clauses over `SELECT *`:

```bash
pgsql query --sql "SELECT status, count(*) AS n FROM orders WHERE created_at > now() - interval '1 day' GROUP BY status" --limit 50 --json
pgsql query --sql "SELECT id, status, updated_at FROM orders WHERE customer_id = \$1 ORDER BY updated_at DESC" --param 42 --limit 20 --json
pgsql query --sql-file ./report.sql --limit 500 --output ./report.csv --json
pgsql query --sql "SELECT 1" --dry-run --json
```

`data.rows` is a list of objects keyed by column name; `columns[]` carries names and PostgreSQL types. `rows_truncated:true` means more rows matched than `--limit` (capped by the instance `max_rows`); `cells_truncated:true` means a cell was cut at `--max-cell-chars` (default 2000). Parameters are sent as text, so cast them in SQL (`$1::int`, `$1::timestamptz`). `--output file.csv` (or `.json`) writes the full result to disk and returns only `{path, bytes, format, row_count, rows_truncated}`. `--dry-run` shows the guarded statement and connection target without connecting.

Explain a plan:

```bash
pgsql explain --sql "SELECT * FROM orders WHERE customer_id = \$1" --param 42 --json
pgsql explain --sql "SELECT * FROM orders WHERE customer_id = \$1" --param 42 --analyze --plan-format json --json
```

Troubleshoot activity, locks, slow statements, replication, and table health:

```bash
pgsql stat activity --state active --min-duration-sec 5 --json
pgsql stat locks --blocked-only --json
pgsql stat slow --limit 20 --sort mean --json
pgsql stat replication --json
pgsql stat tables --sort n_dead_tup --limit 20 --json
pgsql db size --top 10 --json
```

In `stat activity`, `blocked_by` lists the pids blocking a session (from `pg_blocking_pids`); follow the chain to the root blocker and report it, do not terminate it (`pg_terminate_backend` is rejected). `stat slow` returns `has_report:false` (not an error) when `pg_stat_statements` is not installed.

## Errors

| error.code | Meaning | Next action |
|---|---|---|
| `read_only_violation` | The guard or the server refused a write, a second statement, or a side-effecting function. | Rewrite as a single SELECT; writes are out of scope for this tool. |
| `query_timeout` | `statement_timeout` (instance default 30s) was hit. | Add a WHERE clause, reduce `--limit`, or ask for a higher `statement_timeout_seconds`. |
| `permission_denied` | The role lacks SELECT on the relation. | Use a view or table the role can read, or ask for the grant. |
| `invalid_args` | Unknown relation/column, syntax error, or bad parameter (server message included). | Run `schema tables` / `schema describe` and fix the statement. |
| `auth_failed` | Password or pg_hba rejection. | `pgsql auth login --password-stdin`, check sslmode. |
| `network_error` | Host unreachable, TLS failure, or server not accepting connections. | Check host/port/sslmode; retry later. |
| `no_instance_configured` / `instance_required` | No usable instance or an ambiguous choice. | Configure one or pass `--instance`. |

## Safety

- Results may contain personal data. Summarize and aggregate; do not paste raw rows into tickets, chat, or other systems, and do not use `--output` to copy tables around.
- The guard rejects `INSERT/UPDATE/DELETE/MERGE`, DDL, `COPY`, `LOCK`, data-modifying CTEs, `SELECT INTO`, multiple statements, and functions such as `pg_terminate_backend`, `pg_cancel_backend`, `pg_read_file`, `pg_ls_dir`, `lo_import`, `dblink*`, `pg_sleep*`, `pg_reload_conf`, `set_config`, `pg_advisory_*`, `pg_notify`. Do not try to work around it; the transaction is `READ ONLY` and the role should be too.
- Use `--yes` only for `instance remove` and `auth logout` after explicit confirmation.
- Do not print or paste credentials. Store the password from stdin:

```bash
printf '%s\n' "$PGPASSWORD" | pgsql instance add analytics --host db.example.test --database analytics --username readonly --password-stdin --sslmode verify-full --ca-cert-file ./ca.pem --default --json
printf '%s\n' "$PGPASSWORD" | pgsql auth login --instance analytics --password-stdin --json
```

On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes. Put SQL in a file and use `--sql-file` when quoting gets awkward.
