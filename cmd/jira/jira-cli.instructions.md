--- 
applyTo: "**" 
---

# jira CLI Instructions for VS Code GitHub Copilot

Copy this file into `~/.copilot/instructions/jira-cli.instructions.md` so VS Code GitHub Copilot has durable guidance for using the local `jira` CLI.

## What This Tool Is

`jira` is a terminal/Bash-invoked CLI for agents that need stable JSON access to Jira Server/Data Center resources.

Use it for issues, JQL search, transitions, comments, attachments, projects, users, groups, metadata, filters, dashboards, Agile boards and sprints, raw REST calls, and Zephyr test-management resources. It is not a Portal tool, runtime built-in tool, MCP server, or browser scraper.

## Always Use JSON

Always add `--json` so results and failures use the stable envelope:

```bash
jira issue get <issue-or-url> --json
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
jira commands --json
jira schema issue.update --json
jira help llm --json
```

Read issues:

```bash
jira issue get PROJ-123 --fields '*all' --expand names,schema,editmeta --json
jira issue search --jql "project = PROJ ORDER BY updated DESC" --json
```

Write safely:

```bash
jira issue update PROJ-123 --summary "New summary" --dry-run --json
jira issue transition PROJ-123 --to Done --comment "Completed by agent" --dry-run --json
```

Delete only after explicit confirmation:

```bash
jira issue delete PROJ-123 --yes --json
```

Use full Jira URLs when they help select the instance:

```bash
jira issue get "https://jira.example.test/browse/PROJ-123" --json
```

## Windows cmd Workflow

When Copilot is operating in Windows `cmd`, use cmd-native commands and double quotes. Do not use Bash-only commands such as `pwd`, `ls`, `cat`, or single-quote quoting.

Recommended checks:

```cmd
where jira
cd
dir
jira version --json
jira commands --json
```

Robust read command:

```cmd
jira.exe issue get "PROJ-123" --json > "%TEMP%\jira-result.json"
type "%TEMP%\jira-result.json"
```

If PATH lookup is unstable or `jira is not recognized` appears after it worked earlier, run `where jira`, then invoke the exact `.exe` path shown by `where`, wrapped in double quotes.

If command output capture is unreliable, redirect stdout to a file and read it with `type`. Keep using `--json`, then inspect `ok`, `data`, `error.code`, and `error.hint`.

## Zephyr Test Management

If a Jira URL contains `selectedItem=com.thed.zephyr.je`, treat it as a Zephyr test-management page.

For a project you have not inspected, start with:

```bash
jira zephyr doctor --project PROJ --json
jira zephyr status list --json
```

Use Zephyr commands for test cycles, executions, execution status, step results, defects, attachments, folders, ZQL, reports, and test summaries.

Important Zephyr patterns:

```bash
jira zephyr cycle resolve --project PROJ --name "Sprint 42 Regression" --version-id -1 --json
jira zephyr execution resolve --cycle-id 20000 --issue PROJ-123 --project PROJ --version-id -1 --json
jira zephyr execution update-status --cycle-id 20000 --issue PROJ-123 --status PASSED --dry-run --json
```

Do not hard-code numeric Zephyr status ids. Use `jira zephyr status list --json`.

## Auth And Config

The shared Atlassian config is used:

- Linux/macOS: `~/.config/atlassian/config.json`
- Windows: `%APPDATA%\atlassian\config.json`
- Override: `--config <path>` or `ATLASSIAN_CONFIG`

Use `--instance <name>` when multiple instances are configured. Auth secrets should be provided through stdin flags such as `--token-stdin`, `--password-stdin`, or `--api-key-stdin`; do not paste secrets into prompts.

## Error Recovery

Common errors:

- `config_missing`: ask the user to configure the Atlassian config file or pass `--config`.
- `instance_required`: pass `--instance <name>` or use a full Jira URL that belongs to a configured instance.
- `instance_url_mismatch`: use a URL under the selected Jira instance.
- `invalid_args`: call `jira schema <command> --json` and rebuild the command.
- Command parsing errors also return `invalid_args` JSON when `--json` is present.
- `auth_failed`: check credentials with `jira auth test --json`.
- `permission_denied`: report missing Jira permissions.
- `not_found`: verify the issue, project, attachment, or URL.
- `rate_limited`: wait and retry.
- `network_error`: check network, proxy, TLS, and base URL.
- `server_error`: read `error.message` for sanitized upstream details.
- `zephyr_not_detected`: run `jira zephyr doctor --project <PROJECT> --json` and verify the configured Zephyr API family.

## Safety Rules

Always use `--dry-run` before write operations when available.

Only add `--yes` for destructive operations after the user explicitly confirms the deletion or logout.

Do not browser-scrape Jira Test pages unless the API is unavailable and the user explicitly asks for UI investigation.

Do not print, paste, log, or store passwords, API keys, bearer tokens, Authorization headers, or raw config auth fields. Command output redacts stored auth, but prompts and shell history are still caller responsibility.
