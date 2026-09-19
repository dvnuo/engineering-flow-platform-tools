# Splunk CLI Instructions

Use `splunk` for read-only Splunk Enterprise access from Bash, PowerShell, or Windows cmd: bounded SPL searches, existing search jobs, saved searches, index metadata, and raw read-only REST paths. It talks to the management REST API (usually port 8089). It is a terminal-invoked CLI binary, not a Splunk app, MCP server, or runtime built-in tool, and it never writes to Splunk.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Managed runtimes: EFP-prefixed environment variables derived from the config shape (for example `EFP_SPLUNK_DEFAULT_INSTANCE`, `EFP_SPLUNK_INSTANCES_0_BASE_URL`, `EFP_SPLUNK_INSTANCES_0_AUTH_TYPE`, `EFP_SPLUNK_INSTANCES_0_AUTH_TOKEN`, `EFP_SPLUNK_INSTANCES_0_DEFAULT_INDEX`, `EFP_SPLUNK_INSTANCES_0_MAX_RESULTS`); read-only, so config-writing commands then require an explicit `--config` path
- Node: `splunk.default_instance` and `splunk.instances` with `base_url`, `auth` (`bearer_token` token or `basic_password` username/password session login), `default_index`, `default_earliest`, `max_results`, `verify_ssl`, `ca_cert`

Use `--instance <name>` when multiple Splunk instances are configured.

## Discovery

```bash
splunk commands --json
splunk schema search.run --json
splunk help llm --json
splunk auth test --json
splunk index list --json
```

`auth test` returns the authenticated username and roles; `index list` returns index names with total event counts and the earliest/latest event times so you can pick an index before searching.

## Common Workflows

Run a bounded search (a job is created, polled until done, cancelled on timeout, then its results are read):

```bash
splunk search run --query "index=main error | head 100" --earliest -1h --latest now --json
splunk search run --query "index=main sourcetype=access_combined status=500 | stats count by host" --earliest -24h@h --json
splunk search run --query "index=main error" --earliest -1h --fields _time,host,_raw --count 200 --json
```

Quick aggregate without a job:

```bash
splunk search oneshot --query "index=main | stats count by sourcetype" --earliest -15m --json
```

Large result sets: write the untruncated JSON to a file and read it from there instead of stdout.

```bash
splunk search run --query "index=main error" --earliest -1h --count 1000 --output results.json --json
```

Existing jobs (page a finished job, or inspect a long-running one):

```bash
splunk search job get 1700000000.123 --json
splunk search job results 1700000000.123 --count 100 --offset 100 --json
splunk search job cancel 1700000000.123 --yes --json
```

Saved searches and raw REST:

```bash
splunk saved list --filter errors --json
splunk saved run "Errors last hour" --count 100 --timeout-sec 120 --json
splunk api get /services/server/info --json
splunk api get /services/saved/searches --query count=5 --json
```

Rules that keep searches cheap and safe:

- Always pass an explicit time range (`--earliest -15m`, `-1h`, `-24h@h`, plus `--latest now`). Without `--earliest` the instance `default_earliest` or `-1h` applies; never search all time.
- Start narrow: `| head 100` or `| stats count by <field>` before asking for raw events; use `--fields` to keep only the fields you need.
- `--count` is capped by the instance `max_results` (default 1000); a higher value returns `invalid_args`. Page with `--offset` or aggregate instead of raising the cap.
- Printed field values are cut at `--max-field-chars` (default 2000). `data.results_truncated` and `data.fields_truncated` tell you when more exists; re-run with `--output <file>` for the full data.
- If the query does not name an index and the instance sets `default_index`, `index=<default_index>` is prepended automatically. Queries starting with `|` (for example `| tstats`) are never rewritten.
- `wait_timeout` (408) means the job did not finish within `--timeout-sec` (default 60) and was cancelled; narrow the search or raise the timeout. `search_failed` carries Splunk's messages in `data.messages`.
- `--dry-run` shows the exact SPL and job parameters after index injection without contacting Splunk.

## Safety

The CLI is read-only. SPL containing `delete`, `outputlookup`, `outputcsv`, `outputtext`, `collect`, `mcollect`, `meventcollect`, `sendemail`, `sendalert`, `script`, `runshellscript`, `tscollect`, `summaryindex`, `dump` is refused with `spl_blocked` before any job is created; saved searches are checked the same way and dispatched with `trigger_actions=0`. `api get` only accepts paths under `/services/` or `/servicesNS/`. Use `--yes` only after explicit confirmation for `search job cancel`, `instance remove`, and `auth logout`.

Do not print or paste credentials. Prefer stdin credential flags; the session key obtained for `basic_password` instances stays in memory and is never printed:

```bash
printf '%s\n' "$SPLUNK_TOKEN" | splunk instance add prod --base-url https://splunk-api.example.test:8089 --token-stdin --default-index main --default --json
printf '%s\n' "$SPLUNK_PASSWORD" | splunk auth login --instance prod --username svc-agent --auth-type basic_password --password-stdin --json
```

`auth_failed` means the token expired or the session is invalid: store fresh credentials with `auth login` and verify with `auth test --json`. `permission_denied` means the Splunk role lacks the capability or index access.

On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes.
