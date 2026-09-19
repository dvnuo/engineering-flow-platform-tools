# Security

- Secret redaction: `password`, `api_key`, and `token` values must not be printed.
- Output boundary redaction: every CLI envelope is redacted in `internal/output` before JSON, YAML, or table output is written, so upstream tool responses are filtered even if a command forgets command-specific sanitization.
- Artifact boundary: explicit downloads and rendered artifacts may contain raw user-requested content; command envelopes should return metadata only unless an artifact format has its own documented redaction pass.
- Config permissions: saved config files use `0600` permissions where the platform supports it.
- Config environment references: exact `${NAME}` and `%NAME%` string values are resolved only in memory across every tool-owned config node. Field-level saves preserve unchanged references and replace only explicitly changed values, including within reordered named lists. Referenced variables must be non-empty. Environment variables reduce secrets stored in YAML but are not a dedicated secret store and may be visible to processes running as the same user.
- Bearer token handling: bearer tokens are sent as Authorization headers and should not appear in logs or dry-run output.
- Basic auth risk: username/password and username/API key auth can expose long-lived credentials if copied into scripts. Prefer stdin-based login and scoped API keys.
- Off-instance URL guard: absolute URLs must belong to the selected instance base URL.
- Dry-run and `--yes`: write commands should support `--dry-run`; destructive commands require `--yes`.
- Tests: use mock servers and fake credentials only.
- Vulnerability reports: report suspected credential leaks or unsafe URL handling through the repository security reporting process.

## Mobile Auto

- BrowserStack credentials come from `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY` by default and are never printed.
- `mobile-auto type --text-env` and `--text-stdin` do not echo typed values and do not save them in run state.
- Screenshots, source XML, videos, logs, HAR/network logs, and crash logs may contain PII. Envelopes return paths, sizes, hashes, and content types rather than raw binary or large log content.
- Public mobile-auto runs must not start BrowserStack Local or set `local=true`.
- BrowserStack Local is only for private/internal network access. The CLI never auto-downloads the binary and only stops managed tunnel processes recorded in EFP state.
- The Appium plane exposes bounded routes only; it does not expose arbitrary `execute_script`, arbitrary `mobile:*`, arbitrary ADB shell, or raw BrowserStack REST pass-through commands.

## Jenkins

- Jenkins credentials live under the `jenkins` node in `~/.efp/config.yaml` and must be redacted in instance, dry-run, verbose, and error output.
- Jenkins crumbs are requested through `/crumbIssuer/api/json` for state-changing requests when `crumb_mode` is `auto` or `always`.
- Artifact downloads write binary content to local files and must not print artifact bytes into JSON envelopes.
- Raw `jenkins api` calls use the same off-instance URL guard as other instance-backed tools.

## Nexus

- Nexus credentials live under the `nexus` node in `~/.efp/config.yaml` (basic auth with a password or user token passcode, or a bearer token) and must be redacted in instance, verbose, and error output. An instance without an `auth` block sends no Authorization header; a partially filled `auth` block is a `config_error`, never a silent anonymous fallback.
- The CLI is read-only against the repository manager: it only issues GET requests and has no upload, delete, or admin commands. The only writes are local config edits, and `instance remove` and `auth logout` require `--yes`.
- Asset downloads follow the `downloadUrl` from the asset metadata only when it belongs to the instance `base_url` (`instance_url_mismatch` otherwise), stream the bytes to the local file, and return metadata (path, bytes, sha1, content type) rather than content.
- Raw `nexus api get` calls use the same off-instance URL guard as other instance-backed tools.
- Continuation tokens are opaque paging cursors, not credentials; they are returned as `continuation_token` so agents can page through results.

## Splunk

- Splunk credentials live under the `splunk` node in `~/.efp/config.yaml` (or `EFP_SPLUNK_*` variables) and are redacted in instance, dry-run, and error output; `auth login` reads tokens and passwords from stdin only.
- `basic_password` instances log in through `/services/auth/login` once per process; the returned session key is held in memory, sent only as `Authorization: Splunk <key>`, and never written to disk or printed. Authentication tokens are sent only as `Authorization: Bearer`.
- The CLI is read-only against Splunk: an SPL guard refuses `delete`, `outputlookup`, `outputcsv`, `outputtext`, `collect`, `mcollect`, `meventcollect`, `sendemail`, `sendalert`, `script`, `runshellscript`, `tscollect`, and `summaryindex` (also inside `map` searches and saved search definitions) before any job is created, saved searches are dispatched with `trigger_actions=0`, and `api` only supports GET under `/services/` or `/servicesNS/`.
- Result caps: `--count` is bounded by the instance `max_results` (default 1000), jobs are created with `max_count` equal to that cap and a 600 second TTL, jobs that outlive `--timeout-sec` are cancelled, and printed field values are truncated at `--max-field-chars` (default 2000). `--output` writes the untruncated results to a `0600` file instead of stdout.
- Error messages include Splunk's own message text with credentials and session keys redacted. Search results may contain PII from indexed events, so treat `--output` files like log data.

## AppDynamics

- `appd` is read-only: every Controller call is a GET under `/controller/rest/`, plus the OAuth token exchange that API Client credentials require; `appd api get` rejects any other path prefix and any absolute URL outside the selected instance.
- AppDynamics credentials live under the `appd` node in `~/.efp/config.yaml`: the API client secret is stored as `auth.api_key` and the basic password as `auth.password`; both are redacted in instance, dry-run, verbose, and error output.
- The OAuth access token obtained from `/controller/api/oauth/access_token` is kept in process memory only, is never written to config, disk, or output, and error snippets from the Controller are scrubbed of the secret and token before they reach an envelope.
- Snapshot properties, HTTP parameters, and event details returned by the Controller may contain sensitive request data; the shared output redaction applies to every envelope, and `snapshot list` returns summary fields only.
- Use `--api-key-stdin` or `--password-stdin` for `appd instance add` and `appd auth login`; never pass secrets as command-line values.

## Inspect Image

- `inspect-image` sends local image bytes to the configured provider endpoint: GitHub Copilot `/responses` or AI Platform `/chat/completions`.
- It accepts exactly one local image path and rejects remote URLs.
- It does not store raw images or raw responses.
- Shared config is stored in `~/.efp/config.yaml`; short-lived Copilot tokens are stored in `~/.efp/tmp/copilot_token`, and short-lived AI Platform tokens are stored in `~/.efp/tmp/ai_platform_token`. Files are written with `0600` permissions where supported.
- `github_access_token`, `copilot_token`, AI Platform passwords, iB2B `issued_token` values, trust-token headers, Authorization headers, and base64 image data must never appear in stdout, stderr, verbose output, dry-run output, or test snapshots.

## AWS Auth

- The directory password reaches the provider process through a single environment variable (`AD_PASS` for `adfs-assume`, `SAML2AWS_PASSWORD` for `saml2aws`), never through argv; `aws-auth` strips those variables from every other child process, including `aws` and `kubectl`.
- Provider stdout/stderr is redacted against the configured password before it is echoed in `auth_failed` messages.
- `status` reads `~/.aws/credentials` (or `AWS_SHARED_CREDENTIALS_FILE`) only for section names, expiry keys, and `x_principal_arn`; access key ids and secrets are never echoed.
- `assume-role` rewrites the AWS config file (or `AWS_CONFIG_FILE`) with `0600` permissions; comments in that file are not preserved.
- `eks kubeconfig` writes outside the agent workspace by default (`~/.efp/kube/config`) and only checks `kubectl auth can-i list pods`; it never reads Secrets.
