# mobile CLI Instructions

`mobile` is a terminal CLI for BrowserStack App Automate real-device workflows. It is not MCP, not a model-facing function tool, and not a BrowserStack AI integration.

Always use `--json` for agents. Start complex work with:

```bash
mobile commands --json
mobile schema run.start --json
mobile schema observe --json
```

Credentials should usually come from `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY`. If credentials must be persisted, use `mobile auth login --access-key-stdin --json`; environment variables still take precedence over `~/.efp/config.yaml`.

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

- Never invent refs, selectors, XPath, resource IDs, accessibility IDs, or coordinates.
- Use only refs returned by the latest `mobile observe`.
- Re-observe after every mutating command: `tap`, `type`, `clear`, `scroll`, `swipe`, `back`, or `context switch`.
- Never act on ambiguous `locate` results.
- Use `--text-env` or `--text-stdin` for secrets.
- Public sessions must not require BrowserStack Local.
- Private sessions must ensure a tunnel before session start.
- `run handoff` transfers control to the human; mutating actions remain locked until `run resume`.
- Always call `run finish`, including after failures, and collect artifacts when diagnostics matter.

Examples:

```bash
mobile run start --file ./app.apk --platform android --network public --json
mobile observe --run-id run-... --json
mobile locate --run-id run-... --role button --name Login --json
mobile tap --run-id run-... --ref obs-...:e17 --json
mobile observe --run-id run-... --json
mobile type --run-id run-... --ref obs-...:e21 --text-env TEST_PASSWORD --json
mobile assert visible --run-id run-... --name Home --json
mobile run finish --run-id run-... --status passed --collect-artifacts --json
```
