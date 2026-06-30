# mobile-auto CLI Instructions

`mobile-auto` is a terminal CLI for BrowserStack App Automate real-device workflows. It is not MCP, not a model-facing function tool, and not a BrowserStack AI integration.

Always use `--json` for agents. Start complex work with:

```bash
mobile-auto commands --json
mobile-auto schema run.start --json
mobile-auto schema observe --json
```

Credentials should usually come from `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY`. If credentials must be persisted, use `mobile-auto auth login --access-key-stdin --json`; environment variables still take precedence over `~/.efp/config.yaml`.

Recommended flow:

```text
run start
observe
locate
action
observe
assert
...
run finish
```

Rules for agents:

- Prefer latest observation refs for element actions. Use coordinates only with an explicit user target or a measured viewport-relative plan.
- Use only refs returned by the latest `mobile-auto observe`.
- Re-observe after every mutating command unless you used `--post-observe`: `tap`, `tap-point`, `long-press`, `double-tap`, `drag`, `type`, `clear`, `scroll`, `scroll-to`, `swipe`, `back`, `keyboard`, or `context switch`.
- Prefer action-level waits such as `--wait-change`, `--wait-visible`, and `--wait-gone` when the next step depends on the previous action taking effect.
- Use `scroll-to --edge bottom|top` when the goal is to reach a boundary without a known target. Use `swipe` or `scroll --until-stable --max-swipes N` when repeating a gesture until the page stops changing.
- Inspect scroll results for `scrolls`, `stopped_reason`, `repeated_source`, `source_hash_before`, `source_hash_after`, `last_observation_id`, and `visible_text_after` before deciding whether another observe is needed.
- Percent flags accept either `50` or `0.5` for fifty percent. Prefer `--profile fast-page-down`, `--profile fine-scroll`, or `--profile page-up` over hand-tuned percentages when possible.
- Use `mobile-auto inspector config/attach/export` when switching from CLI automation to Appium Inspector debugging.
- Use `mobile-auto session search --status running --json` and `mobile-auto run import --session-id ... --probe --json` when an existing BrowserStack App Automate session was not started by mobile-auto but should be brought under local run state.
- Use `mobile-auto test run --file suite.yaml --junit-out junit.xml --evidence-dir evidence --json` for CI-style suite execution.
- Never act on ambiguous `locate` results.
- Use `--text-env` or `--text-stdin` for secrets.
- Public sessions must not require BrowserStack Local.
- Private sessions must ensure a tunnel before session start.
- `run handoff` transfers control to the human; mutating actions remain locked until `run resume`.
- Always call `run finish`, including after failures, and collect artifacts when diagnostics matter.

Examples:

```bash
mobile-auto run start --file ./app.apk --platform android --network public --json
mobile-auto session search --status running --json
mobile-auto run import --session-id session-... --build-id build-... --json
mobile-auto observe --run-id run-... --json
mobile-auto locate --run-id run-... --role button --name Login --json
mobile-auto tap --run-id run-... --ref obs-...:e17 --json
mobile-auto observe --run-id run-... --json
mobile-auto type --run-id run-... --ref obs-...:e21 --text-env TEST_PASSWORD --json
mobile-auto scroll-to --run-id run-... --text Checkout --max-scrolls 4 --json
mobile-auto scroll-to --run-id run-... --edge bottom --profile fast-page-down --json
mobile-auto swipe --run-id run-... --direction up --until-stable --max-swipes 8 --json
mobile-auto swipe --run-id run-... --direction up --json
mobile-auto tap --run-id run-... --ref obs-...:e30 --wait-change --post-observe --json
mobile-auto keyboard enter --run-id run-... --json
mobile-auto assert visible --run-id run-... --name Home --json
mobile-auto wait visible --run-id run-... --text Home --timeout 20s --json
mobile-auto inspector attach --run-id run-... --json
mobile-auto test run --file suite.yaml --junit-out junit.xml --json
mobile-auto run report --run-id run-... --out report.json --json
mobile-auto run finish --run-id run-... --status passed --collect-artifacts --json
```
