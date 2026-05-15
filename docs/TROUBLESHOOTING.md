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
