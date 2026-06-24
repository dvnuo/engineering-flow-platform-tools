# Mobile Skill Workflow

This document shows how an EFP skill can orchestrate `mobile` from a natural-language test scenario without BrowserStack AI or MCP.

The static test suite validates command behavior, payloads, parsing, and error envelopes. Live production acceptance should still include at least one Android native public run, one private-managed Local run, one iOS native run, and a long human handoff/resume on BrowserStack devices before treating the workflow as fully production-ready.

1. Discover the CLI contract.

```bash
mobile commands --json
mobile schema run.start --json
mobile schema observe --json
mobile schema locate --json
```

2. Start the run.

Public app:

```bash
mobile run start --file ./app.apk --platform android --network public --project EFP --build smoke --name login-smoke --json
```

Private app:

```bash
mobile run start --file ./app.apk --platform android --network private-managed --project EFP --build smoke --name private-login --json
```

3. Observe and choose a target.

```bash
mobile observe --run-id run-... --json
mobile locate --run-id run-... --role button --name Login --actionable --json
```

4. Act only with latest refs.

```bash
mobile tap --run-id run-... --ref obs-...:e17 --json
mobile observe --run-id run-... --json
mobile type --run-id run-... --ref obs-...:e21 --text-env TEST_PASSWORD --json
mobile scroll-to --run-id run-... --text Dashboard --max-scrolls 4 --json
mobile long-press --run-id run-... --ref obs-...:e30 --duration-ms 900 --json
mobile tap --run-id run-... --ref obs-...:e31 --wait-change --post-observe --json
mobile keyboard enter --run-id run-... --json
```

5. Assert state and wait for stability when needed.

```bash
mobile wait stable --run-id run-... --timeout 30s --poll-interval 1s --json
mobile wait visible --run-id run-... --text Dashboard --timeout 20s --json
mobile assert visible --run-id run-... --name Dashboard --json
mobile assert count --run-id run-... --role button --expected 3 --json
```

6. Handoff to a human when manual inspection is needed.

```bash
mobile run handoff --run-id run-... --hold-for 10m --json
mobile run resume --run-id run-... --json
```

7. Finish and collect evidence.

```bash
mobile artifact collect --run-id run-... --json
mobile run report --run-id run-... --out report.json --json
mobile inspector export --run-id run-... --out inspector-evidence --json
mobile run finish --run-id run-... --status passed --collect-artifacts --json
```

Skill authors should branch on `error.code`, not message text. Recover from `stale_observation` by observing again, from `ambiguous_element` by adding stable semantic criteria, from `control_locked` by resuming after human handoff, and from `capacity_wait_timeout` by retrying later or reducing required capacity.

Prefer ref-based actions when an element is observable. Use `tap-point`, coordinate/percent `long-press`, coordinate/percent `double-tap`, or coordinate/percent `drag` only when the user intent is explicitly spatial or the UI element is not represented in the observation tree. `scroll` and `swipe` are viewport-relative, and `scroll-to` should be preferred when searching for a semantic target across a scrollable screen.

Use `workflow record` to turn a run timeline into an editable YAML skeleton, and `workflow run` to replay whitelisted structured steps. Workflows do not execute arbitrary shell or raw CLI strings.

Use `test run` for CI-facing suites with case filters, tags, JSON/JUnit output, and failure evidence. Use `inspector config` or `inspector attach` to hand a live BrowserStack session to Appium Inspector without recreating capabilities by hand.
