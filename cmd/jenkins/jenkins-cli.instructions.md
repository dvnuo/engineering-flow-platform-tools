# Jenkins CLI Instructions

Use `jenkins` for Jenkins controller automation from Bash, PowerShell, or Windows cmd. It is a terminal-invoked CLI binary, not a browser scraper, MCP server, or runtime built-in tool.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Node: `jenkins.default_instance` and `jenkins.instances`

Use `--instance <name>` when multiple Jenkins controllers are configured.

## Discovery

```bash
jenkins commands --json
jenkins schema job.build-with-params --json
jenkins help llm --json
```

## Common Workflows

Trigger a simple build:

```bash
jenkins job build folder/app-main --json
```

Trigger a parameterized build:

```bash
jenkins job build-with-params folder/app-main --param BRANCH=main --param ENV=stage --json
```

Inspect queue and build status:

```bash
jenkins queue get 123 --json
jenkins build status folder/app-main lastBuild --json
```

Read logs:

```bash
jenkins build log folder/app-main 42 --json
jenkins build log-follow folder/app-main 42 --max-rounds 3 --json
```

List and download artifacts:

```bash
jenkins build artifacts folder/app-main 42 --json
jenkins artifact download folder/app-main 42 target/app.jar --output app.jar --json
```

Pipeline REST API, when the Jenkins plugin is installed:

```bash
jenkins pipeline runs folder/app-main --json
jenkins pipeline stages folder/app-main 42 --json
jenkins pipeline node-log folder/app-main 42 6 --json
```

Raw API fallback:

```bash
jenkins api get /api/json --query depth=1 --json
```

## Safety

Use `--dry-run` before writes. Use `--yes` only after explicit confirmation for delete, queue cancel, build stop, safe restart, and raw `api delete`.

Do not print or paste credentials. Prefer stdin credential flags:

```bash
jenkins instance add ci --base-url https://jenkins.example.test --username user@example.test --api-key-stdin --default --json
```

On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes.
