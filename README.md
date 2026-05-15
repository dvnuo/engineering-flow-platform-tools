# Atlassian CLI Tools

Atlassian CLI Tools provides two cross-platform command line binaries for Jira and Confluence automation:

- `jira`
- `confluence`

The project is designed for humans, shell scripts, and LLM/agent workflows that need stable machine-readable output.

## Design Goals

- Keep user-visible commands predictable across platforms.
- Return stable JSON envelopes with `ok`, `data`, and `error`.
- Support multiple Jira and Confluence instances from one config file.
- Avoid printing credentials in normal, error, dry-run, or verbose output.
- Provide command metadata through `commands --json` and `schema <command> --json`.

## Quick Install

Download a release artifact for your platform, place `jira` and `confluence` on your `PATH`, then run:

```bash
jira version --json
confluence version --json
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
jira version --json
```

## Confluence Examples

```bash
confluence auth test --instance docs --json
confluence search --cql 'space = ENG' --instance docs --json
confluence page create --instance docs --space ENG --title "Test Page" --body "<p>Hello</p>" --dry-run --json
confluence version --json
```

## URL Instance Routing

Commands that accept Jira issue URLs or Confluence page URLs can select the matching configured instance automatically. Use `--instance` when multiple configured instances could match the same URL.

## LLM/Agent Usage

Agents should first inspect available commands:

```bash
jira commands --json
confluence commands --json
```

Then inspect the exact schema before calling a command:

```bash
jira schema issue.create --json
confluence schema page.create --json
```

Always use `--json`, inspect `error.code` and `error.hint` before retrying, run write commands with `--dry-run` first, and pass `--yes` for destructive operations.

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

Build outputs are placed under `dist/<os>-<arch>/` for linux, darwin, and windows on amd64 and arm64.

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

Tags matching `v*` trigger the release workflow. Release archives include the binaries plus README and install/config/LLM usage docs.
