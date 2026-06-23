# Troubleshooting

| error.code | Meaning | Next step |
|---|---|---|
| `config_missing` | Config file cannot be loaded. | Create the config or pass `--config`. |
| `no_instance_configured` | No instances exist for the product. | Add an instance. |
| `instance_required` | No default or explicit instance is available. | Pass `--instance` or set a default. |
| `ambiguous_instance` | More than one instance matches the URL. | Pass `--instance`. |
| `instance_url_mismatch` | URL is outside the selected instance. | Use a matching URL and instance. |
| `auth_failed` | Credentials were rejected. | Refresh credentials and run `auth test --json`. |
| `permission_denied` | Authenticated user lacks permission. | Use an account with the required access. |
| `not_found` | Resource does not exist or is hidden. | Verify the identifier and instance. |
| `not_supported` | Server or command does not support the operation. | Use another command or raw API path. |
| `invalid_args` | Required arguments or flags are missing or invalid. | Run `schema <command> --json`. |
| `network_error` | DNS, TLS, or connectivity failed. | Check network and TLS settings. |
| `server_error` | The server returned an unexpected failure. | Retry or inspect server logs. |
| `capacity_wait_timeout` | BrowserStack capacity did not become available before the bounded timeout. | Retry later, reduce `--required`, or inspect `mobile capacity get --json`. |
| `local_binary_not_found` | `BrowserStackLocal` was required but not found. | Set `BROWSERSTACK_LOCAL_BINARY`, configure `mobile.browserstack.local.binary`, or use `private-external`. |
| `local_tunnel_not_ready` | Managed BrowserStack Local did not reach the ready state before the configured timeout. | Inspect `tunnel.log`, credentials, proxy settings, and increase `mobile.browserstack.local.ready_timeout_seconds` if the network is slow. |
| `session_lost` | The Appium session no longer exists or timed out. | Inspect BrowserStack session status and start a new run if needed. |
| `source_parse_failed` | Appium returned malformed or non-XML source for semantic observation. | Re-run `mobile observe`, switch back to a native context, or use a live gated test to verify provider source compatibility. |
| `control_locked` | A run is in human handoff. | Run `mobile run resume --run-id <id> --json` before agent actions. |
| `stale_observation` | A ref is missing or belongs to an older observation. | Run `mobile observe --run-id <id> --json` and use a fresh ref. |
| `ambiguous_element` | A locator or semantic query matched multiple elements. | Add stable name, role, nearby text, resource id, or accessibility id. |
