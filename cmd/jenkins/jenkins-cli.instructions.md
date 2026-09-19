# Jenkins CLI Instructions

Use `jenkins` for Jenkins controller automation from Bash, PowerShell, or Windows cmd. It is a terminal-invoked CLI binary, not a browser scraper, MCP server, or runtime built-in tool.

Default every command and subcommand to `--json` so output uses the stable `ok/data/error` envelope. Inspect `error.code` and `error.hint` before retrying.

Configuration uses the shared EFP config file:

- Default: `~/.efp/config.yaml`
- Override: `--config <path>` or `EFP_CONFIG`
- Managed runtimes: EFP_-prefixed environment variables derived from the config shape (for example `EFP_JENKINS_DEFAULT_INSTANCE`, `EFP_JENKINS_INSTANCES_0_BASE_URL`, `EFP_JENKINS_INSTANCES_0_AUTH_TOKEN`); read-only — write commands then require an explicit `--config` path
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

Read a JUnit-shaped test report (covers JUnit, Cucumber-JVM's JUnit formatter, and most CI test reporters) without hand-parsing the raw Jenkins schema:

```bash
jenkins build test-report folder/app-main 42 --json
jenkins build test-report folder/app-main 42 --max-failures 5 --json
```

`test-report` returns `has_report:false` (not an error) when the build published no test report — do not treat that as a failure. Each entry in `failures[]` includes `class_name`, `name`, `error_details`, `error_stack_trace`, and `duration`. `failure_count_total` is the true failing-case count even when `failures[]` was truncated by `--max-failures` (`failures_truncated:true` signals truncation); pass `--max-failures 0` to fetch only the counts.

Wait for a build to finish instead of hand-rolling a poll loop, for example after re-triggering a build to verify a fix:

```bash
jenkins build wait folder/app-main 43 --timeout-sec 900 --json
```

`build wait` returns the terminal build state once `building` turns false, or a `wait_timeout` error (still carrying the last known state) once `--timeout-sec` elapses.

List and download artifacts:

```bash
jenkins build artifacts folder/app-main 42 --json
jenkins artifact download folder/app-main 42 target/app.jar --output app.jar --json
```

## Find a deployment

To answer "which build deployed version X to prod?" or "what did the last prod deployment run with?", chain three read commands instead of scraping console logs:

1. Locate the job when only its rough name is known. `job search` flattens nested folders and multibranch projects into slash paths and matches a case-insensitive glob against the full path and the job name (`*` also spans folder separators):

   ```bash
   jenkins job search --pattern "*deploy*" --json
   jenkins job search --pattern "deploy/*payments*" --max-depth 4 --json
   ```

   Each entry carries `path`, `url`, `class`, `buildable`, and `folder`. `unexpanded_folders` counts folders at `--max-depth` whose contents were not fetched; raise `--max-depth` (max 6) when it is non-zero and the job is still missing.

2. List the job's newest builds and filter by what a deployment ran with. Every `--param NAME=VALUE` must match a build parameter exactly; `--since` takes a look-back (`30m`, `24h`, `7d`) or an RFC3339 timestamp; `--result` and `--building` narrow further:

   ```bash
   jenkins build list deploy/payments-api --param VERSION=1.4.2 --param ENV=prod --since 30d --json
   jenkins build list deploy/payments-api --result SUCCESS --limit 5 --json
   jenkins build list deploy/payments-api --building --json
   ```

   Builds come newest-first with `number`, `url`, `result`, `building`, `timestamp_iso`, `duration_ms`, `parameters`, and `causes`. Only the newest `limit*4` builds (max 800) are scanned: `count_returned` <= `filtered_from` (builds in the scanned window that matched every filter) <= `scanned`. When `truncated` is true, older matching builds may exist beyond the window or the limit; raise `--limit`, add `--since`, or tighten the filters before concluding that nothing older matches. `oldest_scanned_timestamp_iso` shows how far back the window reached.

3. Inspect the candidate build for its parameters, why it ran, and which commits it carried:

   ```bash
   jenkins build params deploy/payments-api 42 --json
   jenkins build params deploy/payments-api 42 --max-changes 0 --json
   ```

   `causes[]` entries expose `description`, `user_id`, `user_name`, `upstream_project`, and `upstream_build`; `changes[]` entries expose `commit`, `message`, `author`, `timestamp_iso`, and `files_count`, with `changes_count_total` and `changes_truncated` describing the `--max-changes` cut. Both Pipeline (`changeSets`) and freestyle (`changeSet`) jobs are handled. Parameter values whose names look like secrets (password, token, secret, api key) are returned as `***REDACTED***`.

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

## Mobile automation build failure triage

When a Jenkins job runs a Java/Cucumber/BrowserStack mobile automation suite, correlate the failed build to its BrowserStack sessions before touching test source:

1. `jenkins build test-report folder/app-main 42 --json` to get failing scenario names and error messages directly. Fall back to `jenkins build log folder/app-main 42 --json` and search for `automate.browserstack.com` / `app-automate.browserstack.com` dashboard URLs or `sessionId` when no test report is published.
2. Correlate to BrowserStack with `mobile-auto`: prefer a shared BrowserStack build name convention (for example the Jenkinsfile sets `buildName` to `${JOB_NAME}-${BUILD_NUMBER}`) and run `mobile-auto session candidates --build "<job>-<build>" --status "" --probe=false --json` to list every session in that build regardless of pass/fail/timeout. If no build-name convention exists, parse a dashboard URL or raw session id out of the console log or an archived report and use `mobile-auto session probe --from-url <url> --probe=false --json` (or `--session-id`).
3. See `mobile-auto` instructions for pulling device/Appium/crash/network logs and video from a completed (non-live) session — that is a distinct flow from importing a still-running session for live interactive control.
4. After editing test source (locators, step definitions, waits), verify with `jenkins job build-with-params folder/app-main --param BRANCH=<fix-branch> --json` followed by `jenkins build wait folder/app-main <new-build> --timeout-sec <suite-duration> --json`, then re-check `jenkins build test-report`.

## Safety

Use `--dry-run` before writes. Use `--yes` only after explicit confirmation for delete, queue cancel, build stop, safe restart, and raw `api delete`.

Do not print or paste credentials. Prefer stdin credential flags:

```bash
jenkins instance add ci --base-url https://jenkins.example.test --username user@example.test --api-key-stdin --default --json
```

On Windows cmd, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and single quotes.
