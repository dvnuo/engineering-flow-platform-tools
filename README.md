# Engineering Flow Platform Tools

This repository hosts cross-platform Go-based CLI tools for agent, runtime, shell, and automation workflows. Current tool binaries include:

- `jira`
- `confluence`
- `browser`

Jira and Confluence are the first tool family in this repository. Future tools may be added as separate command binaries under `cmd/<tool-name>`.

The project is designed for humans, shell scripts, and LLM/agent workflows that need stable machine-readable output.

## Design Goals

- Keep user-visible commands predictable across platforms.
- Return stable JSON envelopes with `ok`, `data`, and `error`.
- Support cross-platform builds for Go-based custom tool binaries.
- Keep commands suitable for humans, shell scripts, and LLM/agent workflows.
- Avoid printing credentials in normal, error, dry-run, or verbose output.
- Provide command metadata through `commands --json` and `schema <command> --json`.

## Current Tool Families

### Jira and Confluence

The Atlassian product integrations currently include:

- `jira`: Jira Server/Data Center automation
- `confluence`: Confluence Server/Data Center automation

Jira also includes `jira zephyr ...` commands for Zephyr Essential / Zephyr Squad test-management resources on the same Jira instance, including cycles, executions, semantic execution resolution, server status discovery, test steps, folders, attachments, defects, ZQL metadata/search, conservative summaries, and raw ZAPI catalog/access. Jira and Confluence use `ATLASSIAN_CONFIG` and `~/.config/atlassian/config.json` because they are Atlassian product integrations. This does not mean the repository is limited to those product integrations.

### Browser

`browser` is a CLI binary invoked through Bash. It opens an internal URL with Edge/Chrome/Chromium through DevTools, captures screenshot/HTML/network summary, and reports whether browser SSO appeared to complete. It uses a dedicated browser profile by default and does not export cookies or tokens.

## Quick Install

Download a release artifact for your platform, place `jira`, `confluence`, and `browser` on your `PATH`, then run:

```bash
jira version --json
confluence version --json
browser version --json
```

## Config File

- Environment override: `ATLASSIAN_CONFIG`
- Linux/macOS default: `~/.config/atlassian/config.json`
- Windows default: `%APPDATA%\atlassian\config.json`

Example multi-instance config:

```json
{
  "jira": {
    "default_instance": "local",
    "instances": [
      {
        "name": "local",
        "base_url": "https://jira.example.test",
        "rest_path": "/rest/api/2",
        "auth": {"type": "basic_api_key", "username": "user@example.test", "api_key": "redacted"}
      }
    ]
  },
  "confluence": {
    "default_instance": "docs",
    "instances": [
      {
        "name": "docs",
        "base_url": "https://confluence.example.test",
        "rest_path": "/rest/api",
        "auth": {"type": "bearer_token", "token": "redacted"}
      }
    ]
  }
}
```

Supported authentication modes:

- username/password
- username/API key
- bearer token/PAT

## Jira Examples

```bash
jira auth test --instance local --json
jira issue get PROJ-123 --instance local --json
jira issue search --jql 'project = PROJ' --limit 10 --json
jira zephyr doctor --project PROJ --json
jira zephyr summary --project PROJ --version-id -1 --json
jira zephyr cycle list --project PROJ --version-id -1 --json
jira zephyr cycle resolve --project PROJ --name "Sprint 42 Regression" --version-id -1 --json
jira zephyr execution list --cycle-id 20000 --project-id 10000 --version-id -1 --status FAIL --json
jira zephyr execution resolve --cycle-id 20000 --issue PROJ-123 --project PROJ --version-id -1 --json
jira zephyr execution update-status --cycle-id 20000 --issue PROJ-123 --status PASSED --dry-run --json
jira zephyr execution bulk-update-status --execution-ids 30000,30001 --status PASS --dry-run --json
jira zephyr status list --json
jira zephyr api catalog --json
jira zephyr cycle delete 20000 --yes --dry-run --json
jira version --json
```

## Confluence Examples

```bash
confluence auth test --instance docs --json
confluence search --cql 'space = ENG' --instance docs --json
confluence page create --instance docs --space ENG --title "Test Page" --body "<p>Hello</p>" --dry-run --json
confluence version --json
```

## Browser Examples

Windows:

```powershell
browser probe --url "https://intranet.example.test/app" --selector ".user-avatar" --wait 10 --out result --json
```

Validate whether access depends on a prior browser cookie:

```powershell
browser probe --url "https://intranet.example.test/app" --selector ".user-avatar" --clean-profile --wait 10 --out result-clean --json
```

Optional page-context API fetch:

```bash
browser probe --url "https://intranet.example.test/app" --fetch-api "/api/me" --json
```

The current OpenCode runtime image consumes prebuilt binaries copied into `runtime-tools/` by an external pipeline. A future runtime change must place `browser` on `PATH`, and probe execution inside a runtime container also requires Edge/Chrome/Chromium in that image.

## URL Instance Routing

Commands that accept Jira issue URLs or Confluence page URLs can select the matching configured instance automatically. Use `--instance` when multiple configured instances could match the same URL.

## LLM/Agent Usage

Agents should first inspect available commands:

```bash
jira commands --json
confluence commands --json
browser commands --json
```

Then inspect the exact schema before calling a command:

```bash
jira schema issue.create --json
confluence schema page.create --json
browser schema probe --json
```

Always use `--json`, inspect `error.code` and `error.hint` before retrying, run write commands with `--dry-run` first, and pass `--yes` for destructive operations.

For Zephyr, treat a Test Cycle as a Zephyr execution container rather than a Jira issue. When a user asks to update case `X` in cycle `Y`, prefer `jira zephyr execution update-status --cycle-id Y --issue X --status PASSED --json`; use `execution resolve` or `cycle resolve` first when the target is ambiguous, and use `status list` rather than hard-coding numeric status ids.

## How to recover from CLI errors

| error.code | Next action |
|---|---|
| `config_missing` | Create or pass a config file with `--config`, then run `auth test --json`. |
| `no_instance_configured` | Add an instance with `instance add`, or pass a config that contains one. |
| `instance_required` | Provide `--instance <name>` or set a default instance. |
| `ambiguous_instance` | Re-run with explicit `--instance <name>`. |
| `instance_url_mismatch` | Use a URL from the selected instance, or omit `--instance` so the URL can route automatically. |
| `auth_failed` | Refresh credentials and validate with `auth test --json`. |
| `permission_denied` | Use an account or token with the required product permission. |
| `not_found` | Verify the issue/page/content id, URL, and instance. |
| `not_supported` | Use a supported command for that server version, or try the raw `api` command. |
| `invalid_args` | Run `schema <command> --json`, then provide the required args/flags. |
| `network_error` | Retry after checking DNS, proxy, TLS, and connectivity. |
| `server_error` | Retry if transient; otherwise inspect the response and server logs. |

## Security Model

- Secrets are redacted from config display and command output.
- Config files are written with `0600` permissions where supported.
- Bearer tokens and basic auth credentials are only sent in Authorization headers.
- Absolute URLs outside the selected instance are blocked.
- `--verbose` is reserved for diagnostics and must not print credentials.
- Tests must not contain real credentials.

## Cross-Platform Build

```bash
bash scripts/build.sh
bash scripts/build.sh --snapshot
```

```powershell
./scripts/build.ps1
./scripts/build.ps1 --snapshot
```

Build outputs are placed under `dist/<goos>-<goarch>/` for linux, darwin, and windows on amd64 and arm64.

## Development And Testing

```bash
go mod tidy
go test ./...
go vet ./...
bash scripts/smoke.sh
```

On Windows:

```powershell
go test ./...
go vet ./...
./scripts/smoke.ps1
```

## Release

Tags matching `v*` trigger the release workflow. Release archives are named `engineering-flow-platform-tools_<version>_<goos>_<goarch>`.

Current archives include `jira`, `confluence`, `browser`, README, and install/config/LLM usage docs. Future archives may include more tool binaries from `cmd/<tool-name>`.
