# AppDynamics CLI Instructions

Use `appd` for read-only AppDynamics Controller queries from Bash, PowerShell, or Windows cmd: applications, tiers, nodes, business transactions, backends, metrics, transaction snapshots, health-rule violations, and events. It is a terminal-invoked CLI binary, not a browser scraper, MCP server, or runtime built-in tool. Every command is read-only.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Managed runtimes: EFP_-prefixed environment variables derived from the config shape (for example `EFP_APPD_DEFAULT_INSTANCE`, `EFP_APPD_INSTANCES_0_BASE_URL`, `EFP_APPD_INSTANCES_0_ACCOUNT`, `EFP_APPD_INSTANCES_0_AUTH_TYPE=api_client`, `EFP_APPD_INSTANCES_0_AUTH_USERNAME`, `EFP_APPD_INSTANCES_0_AUTH_API_KEY`); read-only — config-writing commands then require an explicit `--config` path
- Node: `appd.default_instance` and `appd.instances[]` with `base_url`, `account`, and `auth`

Credentials are an AppDynamics API Client (`auth.type: api_client`, `username` = API client name, `api_key` = client secret; the CLI exchanges them for a short-lived bearer token that stays in memory) or a basic login (`auth.type: basic_password`, `username` = user, qualified as `user@account`). `account` is the Controller account name; a bare username or client name is qualified as `name@account` automatically.

Use `--instance <name>` when multiple Controllers are configured.

## Discovery

```bash
appd commands --json
appd schema snapshot.list --json
appd help llm --json
```

## Triage Workflow

Start from the application, then narrow to the transaction, its slow or failing snapshots, open health-rule violations, and the deployments in the same window:

```bash
appd app list --json
appd bt list --app ecommerce --json
appd snapshot list --app ecommerce --errors-only --duration-mins 60 --json
appd snapshot list --app ecommerce --user-experience VERY_SLOW,STALL --bt-ids 200 --duration-mins 60 --json
appd snapshot get --app ecommerce --guid 4b9c6f2e-1d3a-4c7e-9f10-1a2b3c4d5e6f --json
appd violation list --app ecommerce --duration-mins 120 --json
appd event list --app ecommerce --event-types APPLICATION_DEPLOYMENT,APPLICATION_ERROR --duration-mins 1440 --json
```

Compare `eventTime` of `APPLICATION_DEPLOYMENT` events with Jenkins build timestamps to tell whether a regression started with a deployment. `snapshot list` returns summary fields only (`requestGUID`, `summary`, `userExperience`, `timeTakenInMilliSecs`, ids, `serverStartTime`, `exitCalls`, `errorDetails`, `URL`) capped by `--max-results`; `truncated=true` means the cap was hit. `snapshot get` returns the full snapshot with exit calls and properties; the call graph is not exposed by the public REST API, so point the user to the Controller UI for it.

## Time Ranges

Every time-ranged command (`metric get/preset`, `snapshot list/get`, `violation list`, `event list`) takes an explicit window and echoes it as `data.time_range`:

- `--duration-mins N` alone: the last N minutes (`BEFORE_NOW`, default 60)
- `--start-time <t> --end-time <t>`: `BETWEEN_TIMES`
- `--before-time <t> --duration-mins N` or `--after-time <t> --duration-mins N`: `BEFORE_TIME` / `AFTER_TIME`

Timestamps are epoch milliseconds or RFC3339 (`2026-09-19T08:00:00Z`); epoch seconds are scaled automatically. `snapshot get` defaults to the last 14 days.

## Metrics

Use presets for the common paths, and browse the tree for anything else:

```bash
appd metric preset --app ecommerce --preset bt-response-time --tier web --bt /checkout --duration-mins 60 --json
appd metric preset --app ecommerce --preset bt-errors --tier web --bt /checkout --json
appd metric preset --app ecommerce --preset tier-cpu --tier web --json
appd metric preset --app ecommerce --preset node-heap --tier web --node web-node-1 --json
appd metric browse --app ecommerce --path "Overall Application Performance" --json
appd metric get --app ecommerce --path "Overall Application Performance|Average Response Time (ms)" --duration-mins 60 --json
```

Presets: `bt-response-time`, `bt-calls`, `bt-errors` (need `--tier` and `--bt`), `tier-cpu` (needs `--tier`), `node-heap` (needs `--tier` and `--node`). Without `--rollup` the Controller returns one value per minute; add `--rollup` for a single aggregated value. `--api v1` switches to the legacy `metric-data` endpoint.

## Application Model

```bash
appd app get ecommerce --json
appd tier list --app ecommerce --json
appd node list --app ecommerce --tier web --json
appd node get --app ecommerce web-node-1 --json
appd backend list --app ecommerce --json
```

`--app`, tiers, and nodes accept names or numeric ids.

## Raw API Fallback

```bash
appd api get /controller/rest/applications/ecommerce/tiers --json
appd api get /controller/rest/applications/ecommerce/events --query time-range-type=BEFORE_NOW --query duration-in-mins=60 --query event-types=CUSTOM --query severities=INFO --json
```

Only paths under `/controller/rest/` are allowed; `output=JSON` is added automatically. Use `--dry-run` on any command to preview the request path and query without contacting the Controller.

## Safety

All commands are read-only. `auth logout` and `instance remove` require `--yes`. Do not print or paste credentials; prefer stdin credential flags:

```bash
appd instance add prod --base-url https://appd.example.test:8090 --account customer1 --auth-type api_client --username efp-reader --api-key-stdin --default --json
appd auth login --instance prod --auth-type basic_password --username reader --password-stdin --json
appd auth test --json
```

`auth_failed` usually means a wrong client secret, a disabled API client, or a missing `account`. On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes, especially around metric paths containing `|`.
