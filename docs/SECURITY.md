# Security

- Secret redaction: `password`, `api_key`, and `token` values must not be printed.
- Config permissions: saved config files use `0600` permissions where the platform supports it.
- Bearer token handling: bearer tokens are sent as Authorization headers and should not appear in logs or dry-run output.
- Basic auth risk: username/password and username/API key auth can expose long-lived credentials if copied into scripts. Prefer stdin-based login and scoped API keys.
- Off-instance URL guard: absolute URLs must belong to the selected instance base URL.
- Dry-run and `--yes`: write commands should support `--dry-run`; destructive commands require `--yes`.
- Tests: use mock servers and fake credentials only.
- Vulnerability reports: report suspected credential leaks or unsafe URL handling through the repository security reporting process.

## Inspect Image

- `inspect-image` sends local image bytes to the configured GitHub Copilot plugin `/responses` endpoint.
- It accepts exactly one local image path and rejects remote URLs.
- It does not store raw images or raw responses.
- Shared config is stored in `~/.efp/config.yaml`; short-lived Copilot tokens are stored in `~/.efp/tmp/copilot_token`. Files are written with `0600` permissions where supported.
- `github_access_token`, `copilot_token`, Authorization headers, and base64 image data must never appear in stdout, stderr, verbose output, dry-run output, or test snapshots.
