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
