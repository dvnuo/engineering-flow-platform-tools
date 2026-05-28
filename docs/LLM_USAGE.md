# LLM/Agent Usage

- Always use --json for machine-readable output.
- Use --instance when multiple instances are configured.
- Full Jira/Confluence URLs can auto-select the instance.
- Use --dry-run before write operations.
- Use --yes for destructive operations.
- Inspect error.code and error.hint before retrying.

## Browser SSO Diagnostics

- Always call `browser schema probe --json` before constructing a probe command.
- Always use `--json`.
- `browser` is a CLI binary invoked through Bash, not an OpenCode built-in browser tool, MCP tool, or Web UI component.
- Use `--selector` for a deterministic login-success signal.
- Use `--clean-profile` when the user wants to distinguish true OS/enterprise SSO from cached browser session.
- Read `data.files.summary`, `data.files.screenshot`, `data.files.html`, and `data.files.network`.
- Use `--save-html=false` or `--save-screenshot=false` when the user wants to avoid writing those artifacts.
- Do not ask the tool to export cookies or tokens.
- Do not treat `negotiate_401_seen` as definitive proof; it is an indicator only.
- In OpenCode runtime, this command requires a browser executable in the runtime image. If no browser is installed, expect `browser_not_found`.

## Jira Zephyr Test Management

- If a Jira URL contains `selectedItem=com.thed.zephyr.je`, treat it as a Zephyr test-management page.
- For a project you have not checked, first run `jira zephyr doctor --project <PROJECT> --json`.
- Use Jira core commands for issues, stories, bugs, comments, attachments, and workflows.
- Use `jira zephyr` for test cycles, executions, execution status, and test summary context.
- Use `--dry-run` before Zephyr write operations unless the user has explicitly approved the write.
- Do not browser-scrape Jira Test pages unless the API is unavailable and the user explicitly asks for UI investigation.

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

## Recommended Workflow

1. Discover commands with `jira commands --json`, `confluence commands --json`, or `browser commands --json`.
2. Inspect the exact command schema before constructing arguments.
3. Prefer full Jira issue URLs or Confluence page URLs when the user provides them.
4. Add `--instance` when the URL is ambiguous across configured instances.
5. Use `--dry-run` for create, update, add, set, upload, move, restore, watch, vote, assign, and transition commands.
6. Add `--yes` only after the user has explicitly confirmed a destructive operation.
7. Parse the JSON envelope and branch on `ok`.
8. On failure, branch on `error.code` before retrying.

## Schema Checks

Use schema output to avoid guessing required flags:

```bash
jira schema issue.create --json
jira schema issue.transition --json
confluence schema page.create --json
confluence schema page.update --json
browser schema probe --json
```

The `required` field lists mandatory arguments and flags. The `flags` field includes type and description metadata suitable for tool planning.

## URL Routing

Jira issue URLs and Confluence page URLs can select an instance automatically. If a URL belongs to a configured instance, omit `--instance` unless multiple instances share the same base URL.

If the user also supplies `--instance`, the URL must belong to that instance. Otherwise the command returns `instance_url_mismatch` and must not send credentials to the URL.

## Output Rules

All automation should request JSON:

```bash
jira issue get PROJ-123 --json
confluence page get --id 123 --json
```

Successful responses contain `ok=true` and `data`. Failed responses contain `ok=false`, `error.code`, and `error.message`; many failures also include `error.hint`.
