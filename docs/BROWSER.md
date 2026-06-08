# Browser CLI

## Purpose

`browser` is a cross-platform Go CLI binary invoked through Bash, PowerShell, or Windows cmd. It can run one-shot probes, or keep a dedicated Edge/Chrome/Chromium automation session open for tab selection, redacted page reads, bounded page actions, assertions, workflows, screenshots, network exports, and performance metadata through DevTools.

## What It Verifies

- The local browser can launch for the current user or runtime environment.
- The target URL can be loaded with a dedicated probe profile.
- A provided CSS selector appears after navigation.
- Page title, final URL, screenshot, HTML, and network event summaries are available for diagnosis.
- Optional page-context `fetch` can call an API with browser credentials included.
- A persistent session can list/open/activate tabs, snapshot/extract redacted page content, inspect page structure, produce accessibility-style refs, assert page state, run whitelisted YAML workflows, read sanitized network timing summaries, record/export sanitized HAR-lite metadata, inspect performance timing metadata, inspect console/runtime errors, inspect frames, extract tables/lists, click/type/select/check/press/upload/wait, write page or visible-element screenshot artifacts, evaluate sanitized page expressions, run sanitized GET fetches with credentials omitted, and inspect download metadata.

## What It Does Not Do

- It does not read, decrypt, or export the user's default browser cookies or tokens.
- It does not reuse the default Edge/Chrome profile.
- It does not bypass MFA, Conditional Access, or enterprise browser policy.
- It does not print `Authorization`, `Cookie`, or `Set-Cookie` headers.
- It does not return response headers, request bodies, response bodies from network observation, browser storage, or binary download bytes.
- It does not return screenshot bytes; `browser page screenshot` writes a local PNG and returns path/size metadata.
- It does not let workflows run shell commands, arbitrary browser CLI strings, arbitrary JavaScript, `page eval`, or `page fetch`; workflows call only whitelisted browser actions/assertions.
- It does not export full HAR data; `browser network export` writes HAR-lite metadata only.
- It does not return performance traces; `browser page metrics` returns browser timing metadata only.
- It does not allow `browser page eval` to access cookies, browser storage, credentials, headers, or network APIs.
- It does not expose typed text, selected option values, console object previews, raw console stacks without redaction, frame URLs/titles without redaction, or closed shadow roots.
- It does not treat `negotiate_401_seen` as proof of Kerberos or Windows Integrated Authentication success. It is only an indicator.

## Windows Manual Test

```powershell
.\dist\windows-amd64\browser.exe probe `
  --url "https://intranet.example.test/app" `
  --selector ".user-avatar" `
  --wait 10 `
  --out ".\result" `
  --json
```

To distinguish true OS/enterprise SSO from a cached browser session:

```powershell
.\dist\windows-amd64\browser.exe probe `
  --url "https://intranet.example.test/app" `
  --selector ".user-avatar" `
  --clean-profile `
  --wait 10 `
  --out ".\result-clean" `
  --json
```

If a clean profile still reaches the business page, OS/enterprise SSO is more likely working. If the non-clean profile works but the clean profile does not, access is more likely dependent on cached browser session state.

## Persistent Session Workflow

Start or reuse a dedicated browser session:

```bash
browser session start --name default --url "https://intranet.example.test/app" --json
browser session status default --json
```

Select a page target:

```bash
browser tab list --session default --json
browser tab current --session default --json
browser tab activate --session default --target-id <target-id> --json
browser tab open --session default --url "https://intranet.example.test/app" --json
```

Read redacted page state:

```bash
browser page snapshot --session default --json
browser page extract --session default --selector ".user-avatar" --json
browser page ax --session default --json
browser page outline --session default --json
browser page outline --session default --pierce --json
browser page network --session default --filter "/api/" --json
browser page metrics --session default --limit-resources 10 --json
browser page console --session default --level error --json
browser page errors --session default --json
browser frame list --session default --json
browser page table --session default --selector "table.results" --json
browser page list --session default --selector "nav" --json
```

Run bounded page actions:

```bash
browser page click --session default --selector "button.sign-in" --json
browser page click --session default --ref "axref-0-abcdef123456" --json
browser page type --session default --selector "input[name=q]" --text "search" --clear --json
browser page select --session default --ref "axref-1-abcdef123456" --label "Ready" --json
browser page check --session default --ref "axref-2-abcdef123456" --json
browser page press --session default --key Enter --json
browser page upload --session default --selector "input[type=file]" --file "./report.pdf" --json
browser page wait --session default --selector ".ready" --network-idle-ms 500 --dom-stable-ms 500 --json
browser page screenshot --session default --out "result/page-screenshot.png" --json
browser page screenshot --session default --selector ".avatar" --out "result/avatar.png" --json
browser page eval --session default --expr "document.title" --json
browser page fetch --session default --url "/api/me" --json
browser network start --session default --limit 500 --json
browser network wait --session default --url-contains "/api/" --status 200 --json
browser network list --session default --filter "/api/" --json
browser network export --session default --out "result/network.har-lite.json" --format har-lite --json
browser download wait --session default --filename-contains "report" --json
browser download list --session default --json
```

Run assertions and whitelisted workflows:

```bash
browser assert visible --session default --selector ".ready" --json
browser assert text --session default --contains "Signed in" --json
browser assert url --session default --contains "/dashboard" --json
browser assert count --session default --selector ".result" --min 1 --json
browser workflow run --file flow.yaml --dry-run --json
browser workflow run --file flow.yaml --session default --json
```

Compact workflow YAML uses an explicit action whitelist:

```yaml
steps:
  - action: page.wait
    selector: .ready
  - action: assert.visible
    selector: .ready
  - action: page.screenshot
    out: result/page.png
```

`page snapshot`, `page extract`, `page ax`, `page outline`, `page table`, `page list`, `page console`, `page errors`, `frame snapshot`, `page eval`, and `page fetch` redact URLs, sensitive assignments, sensitive JSON fields, and known secret-bearing text patterns. `page ax` is a DOM/ARIA accessibility-style fallback with stable short-session refs stored under `~/.efp/browser/refs`; rerun it after navigation or DOM changes. `page extract`, `page outline`, and `page ax` support `--pierce` for open shadow roots only. `frame snapshot` reads a selected DevTools frame by `--frame-id` and redacts frame URL/title/text. `page screenshot --selector` or `--ref` requires a visible element and returns file metadata only. `page network` reads browser resource timing entries and returns redacted URLs, initiator/resource type, timing, size counters, and an API-like marker only. `browser network start/list/wait/stop/export/clear` records or exports sanitized HAR-lite metadata after `start` via page-side fetch/XHR/resource collectors and never captures headers, cookies, storage, or bodies. `page metrics` returns navigation, paint/resource aggregate, DOM node count, long-task count, and redacted largest-resource metadata only. `browser workflow run` executes only whitelisted steps and rejects arbitrary shell, browser CLI strings, JavaScript, `page eval`, and `page fetch`. Dedicated console/network assertion commands are not included in this pass; use `network wait/list` and `page console/errors` for those checks. `page console` and `page errors` capture events only after recorder injection and redact/truncate messages and stacks. `page fetch` rejects unsafe schemes such as `file:`, `data:`, `javascript:`, `chrome:`, and `about:`, runs as GET only, omits credentials, and returns no headers.

`page wait` accepts `--selector`, `--duration-ms`, `--url-contains`, `--text`, `--network-idle-ms`, and `--dom-stable-ms`; all provided conditions must be satisfied within `--timeout`. Network-idle and DOM-stable waits use resource timing counts and DOM/text shape metadata only.

## Output Files

- `screenshot.png`: full-page screenshot when `--save-screenshot` is enabled.
- `page.html`: page outer HTML when `--save-html` is enabled.
- `network.json`: request/response summaries without headers or bodies.
- `summary.json`: stable probe result envelope data without screenshot bytes or full HTML.
- `fetch_api_result.json`: optional result for `--fetch-api`.

`network.json` contains only `kind`, `time`, `request_id`, `method`, `url`, `resource_type`, `status`, and `mime_type`. Sensitive URL query values and fragments are redacted.

To disable artifact types, pass `--save-html=false` or `--save-screenshot=false`.

## JSON Envelope Example

```json
{
  "ok": true,
  "data": {
    "input_url": "https://intranet.example.test/app",
    "final_url": "https://intranet.example.test/app",
    "title": "Internal App",
    "selector": ".user-avatar",
    "selector_found": true,
    "profile_dir": "C:\\Users\\user\\AppData\\Local\\browser-probe-profile",
    "browser_path": "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
    "out_dir": "result",
    "auth_indicators": {
      "microsoft_login_seen": true,
      "login_page_likely": false,
      "negotiate_401_seen": false,
      "redirect_seen": true,
      "selector_found": true,
      "business_page_likely": true
    },
    "api_events": [],
    "network_count": 12,
    "files": {
      "screenshot": "result\\screenshot.png",
      "html": "result\\page.html",
      "network": "result\\network.json",
      "summary": "result\\summary.json"
    }
  }
}
```

## Security Model

The one-shot `browser probe` default profile is a dedicated probe profile:

- Windows: `%LOCALAPPDATA%\browser-probe-profile`
- macOS: `~/Library/Caches/browser-probe-profile`
- Linux: `~/.cache/browser-probe-profile`
- fallback: `os.TempDir()/browser-probe-profile`

Use `--profile` to choose another dedicated profile. Use `--clean-profile` to delete the probe profile before launch. Do not point `--profile` at a real user default Edge/Chrome profile.

Persistent sessions default to `~/.efp/browser/profiles/<session-name>`, downloads default to `~/.efp/browser/downloads/<session-name>`, session metadata is stored under `~/.efp/browser/sessions`, accessibility refs under `~/.efp/browser/refs`, and network recorder artifacts under `~/.efp/browser/network`. DevTools for launched sessions is bound to `127.0.0.1`.

The tool does not read browser cookie databases, decrypt cookies, export tokens, print request/response headers, print request/response bodies, echo typed text, or read downloaded file contents. Probe `--fetch-api` records `ok`, `status`, redacted `url`, `contentType`, and a capped `bodyPreview`. Persistent `page fetch` records `ok`, `status`, redacted final URL, and a capped redacted `body_preview` with credentials omitted and no headers. Persistent `page upload` validates local regular files and returns path/name/size metadata only. Workflow dry-runs and executed step results report typed-text byte counts, not typed text.

## OpenCode Runtime Handoff

This tools repo only builds the binary.

The current OpenCode runtime image consumes prebuilt binaries from `runtime-tools/`. A separate runtime repo change is required to copy `runtime-tools/browser` into `/usr/local/bin/browser`.

A separate runtime repo change is also required to install Edge/Chrome/Chromium if `browser probe` should run inside the runtime container. Without a browser executable, `browser` returns `browser_not_found`.
