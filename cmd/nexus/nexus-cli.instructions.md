# Nexus CLI Instructions

Use `nexus` for read-only Sonatype Nexus Repository 3 lookups from Bash, PowerShell, or Windows cmd: repositories, component and asset search, component/asset metadata, raw read-only REST calls, and asset downloads. It is a terminal-invoked CLI binary, not a browser scraper, MCP server, or runtime built-in tool. It never uploads, deletes, or administers anything on the repository manager.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Managed runtimes: EFP_-prefixed environment variables derived from the config shape (for example `EFP_NEXUS_DEFAULT_INSTANCE`, `EFP_NEXUS_INSTANCES_0_BASE_URL`, `EFP_NEXUS_INSTANCES_0_AUTH_USERNAME`, `EFP_NEXUS_INSTANCES_0_AUTH_PASSWORD`); read-only — instance and auth writes then require an explicit `--config` path
- Node: `nexus.default_instance` and `nexus.instances`
- An instance without an `auth` block is queried anonymously (no Authorization header); `rest_path` defaults to `/service/rest/v1`

Use `--instance <name>` when multiple Nexus instances are configured.

## Discovery

```bash
nexus commands --json
nexus schema component.search --json
nexus help llm --json
```

## Common Workflows

Check access and list repositories; repository names and formats drive every later filter:

```bash
nexus auth test --json
nexus repo list --json
nexus repo get maven-releases --json
```

Find a Maven artifact and its files:

```bash
nexus component search --repository maven-releases --maven-group-id com.example --maven-artifact-id app --version 1.4.2 --json
nexus component get bWF2ZW4tcmVsZWFzZXM6ZDQ4MTE3NTQxZGNiODllYzYxM2IyMzk3MzIwMWQ3YmE --json
```

`data.items[]` entries carry `id`, `group`, `name`, `version`, and `assets[]` (each with `id`, `path`, `downloadUrl`, `checksum`, and `contentType`). Use `--maven-base-version 1.5.0-SNAPSHOT` for snapshots, and `--maven-extension jar` or `--maven-classifier sources` to narrow the files.

Find npm packages, Docker images, or anything by keyword:

```bash
nexus component search --repo-format npm --group @example --name ui-kit --json
nexus asset search --repository docker-hosted --docker-image-name payments/api --docker-image-tag 2.3.0 --json
nexus component search --keyword payments --json
```

`--repo-format` maps to the Nexus `format` filter (maven2, npm, docker, raw, pypi, nuget, helm); `--format` is the CLI output format. `asset search` takes the same filters as `component search` and returns files instead of components.

Page through results:

```bash
nexus component list --repository maven-releases --limit 100 --json
nexus component list --repository maven-releases --continuation <continuation_token> --json
nexus asset list --repository raw-hosted --all --max-pages 5 --json
```

When `data.truncated` is true, either pass `data.continuation_token` back with `--continuation`, or use `--all` (bounded by `--max-pages` and `--limit`, at most 500 items per call). `data.dropped` counts items of the last fetched page that `--limit` cut off; the continuation token skips them, so keep the default `--limit` when walking a repository without gaps.

Download an asset:

```bash
nexus asset get bWF2ZW4tcmVsZWFzZXM6MTVkYWJmZDA1MTIzYWM1MTIzNGY1NjEyMzQ1Njc4OTA --json
nexus asset download bWF2ZW4tcmVsZWFzZXM6MTVkYWJmZDA1MTIzYWM1MTIzNGY1NjEyMzQ1Njc4OTA --output ./app-1.4.2.jar --json
```

The envelope returns `path`, `bytes`, `sha1`, `sha1_verified`, `content_type`, and `name`; the file content is never printed. Downloads only follow `downloadUrl` values that belong to the instance `base_url`, otherwise the command returns `instance_url_mismatch`.

Raw API fallback (GET only; relative paths resolve under `/service/rest/v1`):

```bash
nexus api get /service/rest/v1/status/check --json
nexus api get search --query repository=maven-releases --query name=app --json
```

## Safety

`nexus` is read-only against the repository manager. The only writes are local config edits: `instance add/update/default` and `auth login`, plus the confirmation-gated `instance remove --yes` and `auth logout --yes`.

Do not print or paste credentials. Prefer stdin credential flags. A Nexus user token is stored as `--username <token-name-code>` with `--api-key-stdin` (the passcode) and sent as HTTP basic auth; omit every auth flag to register an anonymous-read instance:

```bash
nexus instance add repo --base-url https://nexus.example.test --username ci-reader --password-stdin --default --json
nexus instance add public --base-url https://public-nexus.example.test --json
```

On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes.
