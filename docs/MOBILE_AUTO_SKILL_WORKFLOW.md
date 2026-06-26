# Mobile Auto Skill Workflow

This document shows how an EFP skill can orchestrate `mobile-auto` from a natural-language test scenario without BrowserStack AI or MCP.

The static test suite validates command behavior, payloads, parsing, and error envelopes. Live production acceptance should still include at least one Android native public run, one private-managed Local run, one iOS native run, and a long human handoff/resume on BrowserStack devices before treating the workflow as fully production-ready.

1. Discover the CLI contract.

```bash
mobile-auto commands --json
mobile-auto schema run.start --json
mobile-auto schema observe --json
mobile-auto schema locate --json
```

2. Start the run.

Public app:

```bash
mobile-auto run start --file ./app.apk --platform android --network public --project EFP --build smoke --name login-smoke --json
```

Private app:

```bash
mobile-auto run start --file ./app.apk --platform android --network private-managed --project EFP --build smoke --name private-login --json
```

3. Observe and choose a target.

```bash
mobile-auto observe --run-id run-... --json
mobile-auto locate --run-id run-... --role button --name Login --actionable --json
```

4. Act only with latest refs.

```bash
mobile-auto tap --run-id run-... --ref obs-...:e17 --json
mobile-auto observe --run-id run-... --json
mobile-auto type --run-id run-... --ref obs-...:e21 --text-env TEST_PASSWORD --json
mobile-auto scroll-to --run-id run-... --text Dashboard --max-scrolls 4 --json
mobile-auto scroll-to --run-id run-... --edge bottom --profile fast-page-down --json
mobile-auto swipe --run-id run-... --direction up --until-stable --max-swipes 8 --json
mobile-auto long-press --run-id run-... --ref obs-...:e30 --duration-ms 900 --json
mobile-auto tap --run-id run-... --ref obs-...:e31 --wait-change --post-observe --json
mobile-auto keyboard enter --run-id run-... --json
```

5. Assert state and wait for stability when needed.

```bash
mobile-auto wait stable --run-id run-... --timeout 30s --poll-interval 1s --json
mobile-auto wait visible --run-id run-... --text Dashboard --timeout 20s --json
mobile-auto assert visible --run-id run-... --name Dashboard --json
mobile-auto assert count --run-id run-... --role button --expected 3 --json
```

6. Handoff to a human when manual inspection is needed.

```bash
mobile-auto run handoff --run-id run-... --hold-for 10m --json
mobile-auto run resume --run-id run-... --json
```

7. Finish and collect evidence.

```bash
mobile-auto artifact collect --run-id run-... --json
mobile-auto run report --run-id run-... --out report.json --json
mobile-auto inspector export --run-id run-... --out inspector-evidence --json
mobile-auto run finish --run-id run-... --status passed --collect-artifacts --json
```

Skill authors should branch on `error.code`, not message text. Recover from `stale_observation` by observing again, from `ambiguous_element` by adding stable semantic criteria, from `control_locked` by resuming after human handoff, and from `capacity_wait_timeout` by retrying later or reducing required capacity.

Prefer ref-based actions when an element is observable. Use `tap-point`, coordinate/percent `long-press`, coordinate/percent `double-tap`, or coordinate/percent `drag` only when the user intent is explicitly spatial or the UI element is not represented in the observation tree. `scroll` and `swipe` are viewport-relative, and `scroll-to` should be preferred when searching for a semantic target across a scrollable screen. Use `scroll-to --edge bottom|top` for boundary scrolling without a known target, and use `--until-stable`, `--until-visible`, or `--until-gone` when repeating a gesture until a stop condition is met. Scroll results include `scrolls`, `stopped_reason`, `repeated_source`, before/after source hashes, and final observation summaries.

Use `workflow record` to turn a run timeline into an editable YAML skeleton, and `workflow run` to replay whitelisted structured steps. Workflows do not execute arbitrary shell or raw CLI strings.

Use `test run` for CI-facing suites with case filters, tags, JSON/JUnit output, and failure evidence. Suite-level `after` steps are cleanup and run even after a before/case failure; use `secrets_env` to map suite variables to environment variable names consumed by workflow `text_env` fields without putting secret values in reports.

Use `inspector config` or `inspector attach` to hand a live BrowserStack session to Appium Inspector without recreating capabilities by hand. Inspector JSON keeps `accessKey` redacted; pass `--secret-mode env` to include safe environment-variable hints for manual Inspector setup.
