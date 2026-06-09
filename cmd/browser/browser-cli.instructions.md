--- 
applyTo: "**" 
---

# browser CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/browser-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `browser` CLI.

## What This Tool Is

`browser` is a terminal-invoked CLI for agents that need to open an internal URL in Chrome by default through DevTools and collect page diagnostics or run bounded actions in a persistent dedicated browser session. Edge/Chromium remain available with `--browser`.

Use it for browser SSO checks, login-success probes, screenshots, HTML snapshots, network summaries, page-state inspection, semantic element finding, accessibility-style refs, schema-based extraction, assertions, screenshot baseline checks, whitelisted workflow recording/running with locator fallback, optional workflow evidence bundles, form inspection/fill, performance timing metadata, frame reads, console/runtime diagnostics, structured page outlines, table/list extraction and export, scroll collection, page-state diffs, tab selection, upload/download metadata, and bounded page actions. It is not a Portal tool, runtime built-in browser tool, MCP server, or cookie export tool.

## Always Use JSON

For agents, `--json` is the default way to use every `browser` command and subcommand. Always add `--json` so results and failures use the stable envelope:

```bash
browser probe --url <url> --json
```

Only omit `--json` when intentionally reading human-oriented `--help` text.

Read these fields first:

- `ok`
- `data.files.summary`
- `data.files.screenshot`
- `data.files.html`
- `data.files.network`
- `data.selector_found`
- `error.code`
- `error.hint`

If `ok=false`, inspect `error.code`, `error.message`, and `error.hint` before retrying.

## Basic Workflow

Discover the command shape:

```bash
browser commands --json
browser schema probe --json
browser schema session.start --json
browser schema session.attach --json
browser schema session.discover --json
browser schema page.fetch --json
browser schema page.network --json
browser schema page.extract-schema --json
browser schema page.find --json
browser schema page.outline --json
browser schema page.ax --json
browser schema page.table-export --json
browser schema page.list-export --json
browser schema page.scroll-collect --json
browser schema page.diff --json
browser schema network.start --json
browser schema network.export --json
browser schema page.metrics --json
browser schema assert.visible --json
browser schema assert.screenshot --json
browser schema workflow.run --json
browser schema workflow.record --json
browser schema form.inspect --json
browser schema form.fill --json
browser schema page.console --json
browser schema frame.list --json
browser help llm --json
```

Probe a page:

```bash
browser probe --url https://intranet.example.test --selector .user-avatar --wait 10 --out result --json
```

Require a deterministic login-success selector:

```bash
browser probe --url https://intranet.example.test --selector .user-avatar --require-selector --json
```

Distinguish true OS/enterprise SSO from a cached browser session:

```bash
browser probe --url https://intranet.example.test --clean-profile --selector .user-avatar --json
```

Fetch an API from the loaded page context:

```bash
browser probe --url https://intranet.example.test --fetch-api /api/me --network-filter /api/ --json
```

Use a persistent session for multi-step page automation. `session start` attempts to detach the managed browser from the short-lived CLI or agent command process so later chat turns can keep using the same DevTools endpoint:

```bash
browser session start --name default --url https://intranet.example.test --json
browser session discover --ports 9222,9223 --json
browser session attach --name user-demo --debug-port 9222 --json
browser tab current --session default --json
browser page snapshot --session default --json
browser page extract --session default --selector .user-avatar --json
browser page extract-schema --session default --file schema.yaml --json
browser page find --session default --role button --name Save --json
browser page ax --session default --json
browser page outline --session default --json
browser page outline --session default --pierce --json
browser page network --session default --filter /api/ --json
browser page metrics --session default --limit-resources 10 --json
browser page console --session default --level error --json
browser page errors --session default --json
browser frame list --session default --json
browser page table --session default --selector table.results --json
browser page list --session default --selector nav --json
browser page screenshot --session default --out result/page-screenshot.png --json
browser page screenshot --session default --selector .avatar --out result/avatar.png --json
browser page table-export --session default --selector table.results --out result/table.csv --format csv --json
browser page scroll-collect --session default --item-selector .row --out result/items.json --json
browser page diff --before before.json --after after.json --json
browser assert visible --session default --selector .ready --json
browser assert count --session default --selector .result --min 1 --json
browser assert screenshot --session default --baseline baseline.png --out actual.png --diff-out diff.png --json
browser workflow record --session default --out flow.yaml --duration-ms 10000 --json
browser workflow run --file flow.yaml --dry-run --var query=demo --report-out result/workflow-run.json --evidence-dir result/evidence --json
browser form inspect --session default --json
browser form fill --session default --file values.yaml --json
browser network start --session default --limit 500 --json
browser network list --session default --filter /api/ --json
browser network export --session default --out result/network.har-lite.json --format har-lite --json
```

Bounded page actions:

```bash
browser page click --session default --selector "button.sign-in" --json
browser page click --session default --ref "axref-0-abcdef123456" --json
browser page type --session default --selector "input[name=q]" --text "search" --clear --json
browser page select --session default --ref "axref-1-abcdef123456" --label "Ready" --json
browser page check --session default --ref "axref-2-abcdef123456" --json
browser page press --session default --key Enter --json
browser page upload --session default --selector "input[type=file]" --file ./report.pdf --json
browser page wait --session default --selector ".ready" --network-idle-ms 500 --dom-stable-ms 500 --json
browser page eval --session default --expr "document.title" --json
browser page fetch --session default --url /api/me --json
browser download wait --session default --filename-contains report --json
browser download list --session default --json
```

## Windows cmd Workflow

When Copilot is operating in Windows `cmd`, use cmd-native commands and double quotes. Do not use Bash-only commands such as `pwd`, `command -v`, `ls`, `cat`, `cd "$PWD"`, `$PWD`, or single-quote quoting.

Recommended checks:

```cmd
where browser
cd
dir
browser version --json
browser commands --json
browser schema probe --json
browser schema page.screenshot --json
browser schema page.wait --json
browser schema download.wait --json
```

Normal probe command:

```cmd
browser.exe probe --url "https://intranet.example.test" --selector ".user-avatar" --out "%CD%\browser-probe" --json
```

If PATH lookup is unstable or `browser is not recognized` appears after it worked earlier, run `where browser`, then invoke the exact `.exe` path shown by `where`, wrapped in double quotes.

If command output capture is unreliable, redirect the JSON envelope to a workspace file and read it with the file-read tool. Use `type` only when no file-read tool is available:

```cmd
browser.exe probe --url "https://intranet.example.test" --selector ".user-avatar" --out "%CD%\browser-probe" --json > "%CD%\browser-result.json"
```

Also inspect the artifact files under `--out`, especially `summary.json`, `network.json`, `page.html`, and `screenshot.png`.

## Error Recovery

Common errors:

- `invalid_args`: call `browser schema probe --json` and rebuild the command.
- Command parsing errors also return `invalid_args` JSON when `--json` is present.
- `browser_not_found`: install Edge, Chrome, or Chromium, or pass `--browser-exe <path>`.
- `page_timeout`: increase `--timeout`, increase `--wait`, or verify the URL is reachable.
- `selector_not_found`: inspect `data.files.screenshot`, `data.files.html`, and `data.files.summary`, then adjust `--selector`.
- `network_error`: check proxy, DNS, certificates, and whether the browser can reach the URL.
- `session_not_running`: run `browser session start --json` or restart the stored session.
- `target_not_found`: run `browser tab list --json`, then pass a current `--target-id`.
- `assertion_failed`: inspect `data` for sanitized assertion details, add a wait if needed, then retry or adjust the assertion.
- `workflow_failed`: inspect `data.steps` for the failing whitelisted step; use `--dry-run` to validate before executing.
- `server_error`: read `error.message` for the sanitized detail.

## Security Rules

`browser` does not export cookies or tokens. Do not ask it to print cookies, browser storage, or Authorization headers.

`browser session discover` and `browser session attach` require explicit `127.0.0.1` DevTools ports; they do not inspect arbitrary browsers, default profiles, cookies, or tokens. `browser page find` locates elements by role/name/text/label/placeholder/nearby text and returns refs plus fallback locators; use it before actions when selectors are unknown or unstable. `browser page ax` returns DOM/ARIA accessibility-style refs, not raw input values; rerun it after navigation or DOM changes. `browser page extract-schema` reads selector-declared YAML fields and returns redacted structured values. `browser page table-export`, `list-export`, and `scroll-collect` write redacted data artifacts; `browser page diff` compares two JSON page-state captures. `browser form inspect` returns form metadata without current values; `browser form fill` fills from YAML and returns match metadata plus value byte counts only. `browser page click/type/select/check/uncheck/press` can use either `--selector`, `--ref`, or workflow `locators`; action output returns metadata only and does not echo typed text or selected option values. Risky clicks such as submit, delete, pay, save, approve, publish, deploy, or transfer require explicit user confirmation and `--yes`. `browser assert visible/text/url/count/screenshot` returns sanitized pass/fail assertion metadata; failures use `assertion_failed`, and screenshot assertions write actual/diff PNG artifacts instead of image bytes. `browser workflow record` writes a safe workflow skeleton with typed text and selected option values replaced by variables and locator candidates. `browser workflow run` supports variables, CLI `--var`, conditions, `for_each`, `locators`, `smart_wait`, `human.wait`, `human.confirm`, `--report-out`, and optional `--evidence-dir`; it executes only whitelisted browser actions/assertions and rejects shell commands, arbitrary browser CLI strings, arbitrary JavaScript, `page eval`, and `page fetch`. Dry-run plans include typed-text byte counts, not typed text. `browser page network` returns sanitized resource timing summaries only. `browser page metrics` returns browser timing metadata only. `browser network start/list/wait/export/stop/clear` records or exports sanitized HAR-lite metadata after `start`; fetch/XHR response body previews are redacted and returned by default, while headers, cookies, storage, and request bodies are never returned. `browser page console` and `browser page errors` redact/truncate messages and stacks and do not return object previews. `browser frame list/snapshot` redact frame URLs and titles. `--pierce` traverses open shadow roots only; closed shadow roots are inaccessible. `browser page screenshot` writes a PNG artifact and returns file metadata, not image bytes; element screenshots require a visible selector/ref and stale refs require rerunning `browser page ax`. `browser page eval` rejects cookie, storage, header, credential, and network APIs before returning recursively redacted values. `browser page fetch` uses GET with credentials omitted, rejects unsafe schemes such as `file:`, `data:`, `javascript:`, `chrome:`, and `about:`, returns no headers, and redacts the body preview. `browser page upload` returns file path/name/size metadata only. `browser download list/wait` return file metadata only and do not read downloaded file contents.

Artifacts may contain page content, visible user names, or internal URLs. Treat `page.html`, `screenshot.png`, `network.json`, and `summary.json` as potentially sensitive diagnostics.

Never paste secrets into command arguments. Prefer using a normal authenticated browser profile or enterprise SSO.
