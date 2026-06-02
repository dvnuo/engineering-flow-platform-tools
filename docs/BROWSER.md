# Browser CLI

## Purpose

`browser` is a cross-platform Go CLI binary invoked through Bash, PowerShell, or Windows cmd. It opens an internal URL in Edge/Chrome/Chromium through DevTools and writes diagnostics that help determine whether browser-based enterprise SSO appears to complete.

## What It Verifies

- The local browser can launch for the current user or runtime environment.
- The target URL can be loaded with a dedicated probe profile.
- A provided CSS selector appears after navigation.
- Page title, final URL, screenshot, HTML, and network event summaries are available for diagnosis.
- Optional page-context `fetch` can call an API with browser credentials included.

## What It Does Not Do

- It does not read, decrypt, or export the user's default browser cookies or tokens.
- It does not reuse the default Edge/Chrome profile.
- It does not bypass MFA, Conditional Access, or enterprise browser policy.
- It does not print `Authorization`, `Cookie`, or `Set-Cookie` headers.
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

The default profile is a dedicated probe profile:

- Windows: `%LOCALAPPDATA%\browser-probe-profile`
- macOS: `~/Library/Caches/browser-probe-profile`
- Linux: `~/.cache/browser-probe-profile`
- fallback: `os.TempDir()/browser-probe-profile`

Use `--profile` to choose another dedicated profile. Use `--clean-profile` to delete the probe profile before launch. Do not point `--profile` at a real user default Edge/Chrome profile.

The tool does not read browser cookie databases, decrypt cookies, export tokens, or print request/response bodies. `--fetch-api` only records `ok`, `status`, redacted `url`, `contentType`, and a capped `bodyPreview`.

## OpenCode Runtime Handoff

This tools repo only builds the binary.

The current OpenCode runtime image consumes prebuilt binaries from `runtime-tools/`. A separate runtime repo change is required to copy `runtime-tools/browser` into `/usr/local/bin/browser`.

A separate runtime repo change is also required to install Edge/Chrome/Chromium if `browser probe` should run inside the runtime container. Without a browser executable, `browser` returns `browser_not_found`.
