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
| `capacity_wait_timeout` | BrowserStack capacity did not become available before the bounded timeout. | Retry later, reduce `--required`, or inspect `mobile-auto capacity get --json`. |
| `local_binary_not_found` | `BrowserStackLocal` was required but not found. | Set `BROWSERSTACK_LOCAL_BINARY`, configure `mobile-auto.browserstack.local.binary`, or use `private-external`. |
| `local_tunnel_not_ready` | Managed BrowserStack Local did not reach the ready state before the configured timeout. | Inspect `tunnel.log`, credentials, proxy settings, and increase `mobile-auto.browserstack.local.ready_timeout_seconds` if the network is slow. |
| `local_tunnel_required` | BrowserStack reported that the app/session needs Local, but the run was not bound to a usable Local tunnel. | Use `--network private-managed`, or start BrowserStack Local externally and use `--network private-external --local-identifier <id>`. |
| `local_tunnel_connection_failed` | BrowserStack/Appium reported that the configured Local tunnel could not connect or was disconnected. | Check Local process health, identifier, proxy settings, and private host reachability, then retry. |
| `local_tunnel_ownership_mismatch` | A stored managed tunnel PID could not be proven to still belong to this CLI's BrowserStackLocal process. | Do not retry with force-kill. Inspect `tunnel.log`, stop BrowserStackLocal manually if needed, then run cleanup again. |
| `invalid_capabilities` | Appium or BrowserStack rejected the requested device/app/capability combination. | Inspect `run start` flags, device resolution, app ref, and configured Appium capabilities. |
| `browserstack_session_error` | BrowserStack returned provider-specific session creation semantics in a 200 response without a usable session id. | Inspect the returned message, BrowserStack dashboard, and Appium logs. |
| `session_recovery_not_found` / `session_recovery_ambiguous` | The CLI could not uniquely recover a missing Appium session id from BrowserStack build/session listing. | Use unique `--build` and `--name` values and inspect BrowserStack sessions for duplicates. |
| `session_lost` | The Appium session no longer exists or timed out. | Inspect BrowserStack session status and start a new run if needed. |
| `source_parse_failed` | Appium returned malformed or non-XML source for semantic observation. | Re-run `mobile-auto observe`, switch back to a native context, or use a live gated test to verify provider source compatibility. |
| `control_locked` | A run is in human handoff. | Run `mobile-auto run resume --run-id <id> --json` before agent actions. |
| `stale_observation` | A ref is missing or belongs to an older observation. | Run `mobile-auto observe --run-id <id> --json` and use a fresh ref. |
| `ambiguous_element` | A locator or semantic query matched multiple elements. | Add stable name, role, nearby text, resource id, or accessibility id. |
