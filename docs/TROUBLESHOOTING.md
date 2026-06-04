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
| `run_not_found` | A log analysis run directory or run file is missing. | Run `log analyze --source <path> --run <run-dir> --json` or verify the run path. |
| `source_missing` | A source file recorded by `log analyze` is no longer available for `log window`. | Restore the source file or re-run `log analyze`. |
| `source_not_in_run` | `log window --file --line` requested a file outside the run manifest. | Analyze that source first or use an entry id from the run. |
| `line_outside_run_source_range` | Direct line window requested a line not recorded during analyze. | Re-run `log analyze` after append-only source changes. |
| `entry_source_not_in_run` | An indexed entry points to a source that does not match the manifest. | Re-run `log analyze`; the run index may be stale or corrupted. |
| `entry_outside_run_source_range` | An indexed entry line range exceeds the manifest source range. | Re-run `log analyze`; the run index may be stale or corrupted. |
| `log_export_exists` | A redacted evidence export output file already exists. | Pass `--overwrite` or choose a different `--output`. |
| `log_evidence_not_found` | Requested evidence id was not found in entries or templates. | Use an `entry_id` or `template_id` returned by `search`, `entries`, or `template list`. |
