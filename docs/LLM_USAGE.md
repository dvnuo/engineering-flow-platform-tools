# LLM/Agent Usage

- Always use `--json` for machine-readable output.
- Use `--instance` when multiple instances are configured.
- Full Jira/Confluence URLs can auto-select the instance.
- Use `--dry-run` before write operations.
- Use `--yes` for destructive operations.
- On errors, inspect `error.code`, `error.message`, and `error.hint`.

## Common errors

| error.code | Meaning | Recommended next step |
|---|---|---|
| `instance_required` | No usable instance selected. | Provide `--instance` or configure default instance. |
| `ambiguous_instance` | Multiple candidates matched. | Re-run with explicit `--instance <name>`. |
| `instance_url_mismatch` | URL doesn't belong to selected instance. | Use matching URL/instance pair, or omit `--instance`. |
| `auth_failed` | Authentication failed. | Refresh credentials and run `auth test --json`. |
| `permission_denied` | Authenticated but not authorized. | Use account/token with required permissions. |
| `not_found` | Target issue/page/content was not found. | Verify identifier/URL and instance context. |
| `not_supported` | Command/path unsupported by server/version. | Try a supported operation or lower-level raw API path. |
| `invalid_args` | Required args/flags are missing/invalid. | Check `schema <command>` and fix arguments. |
| `network_error` | Connectivity/TLS/DNS timeout failure. | Retry and validate network/TLS settings. |
