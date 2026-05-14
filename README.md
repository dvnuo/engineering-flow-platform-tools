# Atlassian CLI Tools

This repository provides a generic Atlassian CLI toolkit with two user-facing commands:

- `jira`
- `confluence`

## Purpose

- Build a cross-platform CLI foundation for Jira and Confluence automation.
- Keep output stable for shell, LLM, and agent integrations.
- Support multi-instance profiles with a single config file.

This CLI is not bound to any specific agent platform.

## Config

- Environment variable: `ATLASSIAN_CONFIG`
- Default path (Linux/macOS): `~/.config/atlassian/config.json`
- Default path (Windows): `%APPDATA%\\atlassian\\config.json`

Supported auth modes:

- username/password
- username/api key
- PAT/bearer token

## Output contract

Use `--json` for machine-friendly output. Top-level JSON is always an envelope:

- Success: `{"ok": true, "instance": "...", "data": {...}}`
- Error: `{"ok": false, "error": {"code": "...", "message": "...", "hint": "..."}}`

## Instance routing

- Multi-instance profiles are supported.
- Prefer `--instance` when multiple profiles exist.
- URL-based instance resolution is specified and will be implemented in business commands.

## Security principles

- Never print `password`, `api_key`, or `token`.
- Never log secrets.
- Config file write mode uses `0600`.

## Build

### Linux/macOS

```bash
bash scripts/build.sh
```

### Windows PowerShell

```powershell
./scripts/build.ps1
```

Build outputs include `jira` and `confluence` for linux, darwin, and windows targets under `dist/`.

## TLS note

When `verify_ssl=false`, TLS verification is disabled and should only be used in internal test environments.


## Command coverage

`docs/COMMAND_SPEC.md` is the source of the user-visible command contract. The Cobra command trees, `commands --json`, and `schema <command> --json` are tested against that contract.


## Confluence examples

```bash
confluence auth test --instance demo --json
confluence search --cql 'space = ENG' --instance demo --json
confluence page create --instance demo --space ENG --title "Test Page" --body "<p>Hello</p>" --dry-run --json
```


## For LLM/agent usage

- Always pass `--json` for machine-readable output.
- Use `--instance` when multiple instances are configured.
- Full Jira/Confluence URLs can auto-select an instance.
- Use `--dry-run` before write operations.
- Use `--yes` for destructive operations.
- Prefer `commands --json` and `schema <command> --json` to plan tool calls.
