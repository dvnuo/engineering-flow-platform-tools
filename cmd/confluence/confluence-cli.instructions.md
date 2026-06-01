--- 
applyTo: "**" 
---

# confluence CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/confluence-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `confluence` CLI.

## What This Tool Is

`confluence` is a terminal/Bash-invoked CLI for agents that need stable JSON access to Confluence Server/Data Center resources.

Use it for pages, spaces, content, blogs, attachments, comments, labels, restrictions, users, groups, long tasks, webhooks, and raw REST calls. It is not a Portal tool, runtime built-in tool, MCP server, or browser scraper.

## Always Use JSON

Always add `--json` so results and failures use the stable envelope:

```bash
confluence page get --id <page-id> --json
```

Read these fields first:

- `ok`
- `instance`
- `data`
- `error.code`
- `error.message`
- `error.hint`

If `ok=false`, inspect `error.code`, `error.message`, and `error.hint` before retrying.

## Basic Workflow

Discover command shape before complex calls:

```bash
confluence commands --json
confluence schema page.update --json
confluence help llm --json
```

Read content:

```bash
confluence page get --id 123 --expand body.storage,version --json
confluence page get-by-title --space ENG --title "Runtime Profile" --json
confluence search --cql "space = ENG and type = page" --json
```

Write safely:

```bash
confluence page update --id 123 --title "Runtime Profile" --body-file page.html --dry-run --json
confluence page update --id 123 --title "Runtime Profile" --body-file page.html --json
```

Delete only after explicit confirmation:

```bash
confluence page delete --id 123 --yes --json
```

Use full Confluence URLs when they help select the instance:

```bash
confluence page get --url "https://confluence.example.test/display/ENG/Runtime+Profile" --json
```

## Auth And Config

The shared Atlassian config is used:

- Linux/macOS: `~/.config/atlassian/config.json`
- Windows: `%APPDATA%\atlassian\config.json`
- Override: `--config <path>` or `ATLASSIAN_CONFIG`

Use `--instance <name>` when multiple instances are configured. Auth secrets should be provided through stdin flags such as `--token-stdin`, `--password-stdin`, or `--api-key-stdin`; do not paste secrets into prompts.

## Error Recovery

Common errors:

- `config_missing`: ask the user to configure the Atlassian config file or pass `--config`.
- `instance_required`: pass `--instance <name>` or use a full Confluence URL that belongs to a configured instance.
- `instance_url_mismatch`: use a URL under the selected Confluence instance.
- `invalid_args`: call `confluence schema <command> --json` and rebuild the command.
- `auth_failed`: check credentials with `confluence auth test --json`.
- `permission_denied`: report missing Confluence permissions.
- `not_found`: verify the page, space, attachment, comment, or URL.
- `rate_limited`: wait and retry.
- `network_error`: check network, proxy, TLS, and base URL.
- `server_error`: read `error.message` for sanitized upstream details.

## Safety Rules

Always use `--dry-run` before write operations when available.

Only add `--yes` for destructive operations after the user explicitly confirms the deletion or logout.

Do not print, paste, log, or store passwords, API keys, bearer tokens, Authorization headers, or raw config auth fields. Command output redacts stored auth, but prompts and shell history are still caller responsibility.
