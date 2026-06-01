--- 
applyTo: "**" 
---

# browser CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/browser-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `browser` CLI.

## What This Tool Is

`browser` is a terminal/Bash-invoked CLI for agents that need to open an internal URL in Edge, Chrome, or Chromium through DevTools and collect page diagnostics.

Use it for browser SSO checks, login-success probes, screenshots, HTML snapshots, network summaries, and page-state inspection. It is not a Portal tool, runtime built-in browser tool, MCP server, or cookie export tool.

## Always Use JSON

Always add `--json` so results and failures use the stable envelope:

```bash
browser probe --url <url> --json
```

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

## Error Recovery

Common errors:

- `invalid_args`: call `browser schema probe --json` and rebuild the command.
- `browser_not_found`: install Edge, Chrome, or Chromium, or pass `--browser-exe <path>`.
- `page_timeout`: increase `--timeout`, increase `--wait`, or verify the URL is reachable.
- `selector_not_found`: inspect `data.files.screenshot`, `data.files.html`, and `data.files.summary`, then adjust `--selector`.
- `network_error`: check proxy, DNS, certificates, and whether the browser can reach the URL.
- `server_error`: read `error.message` for the sanitized detail.

## Security Rules

`browser` does not export cookies or tokens. Do not ask it to print cookies, browser storage, or Authorization headers.

Artifacts may contain page content, visible user names, or internal URLs. Treat `page.html`, `screenshot.png`, `network.json`, and `summary.json` as potentially sensitive diagnostics.

Never paste secrets into command arguments. Prefer using a normal authenticated browser profile or enterprise SSO.
