# Configuration

Default config path: `~/.efp/config.yaml`

Environment override: `EFP_CONFIG`

Legacy environment overrides are still accepted for compatibility:

- Jira and Confluence: `ATLASSIAN_CONFIG`
- inspect-image: `INSPECT_IMAGE_CONFIG`

## Environment Variable References

String values in the unified YAML config can reference an environment variable
by using an exact `${NAME}` or `%NAME%` placeholder. The `${NAME}` form is
portable; the quoted `%NAME%` form is also supported for Windows-authored
configuration. This applies consistently to every tool-owned node, including
`jira`, `confluence`, `jenkins`, `nexus`, `aws`, `browser`, `mobile-auto`,
`copilot`, `inspect_image`, and `ai_platform`.

References are resolved only in memory. Config loaders retain a resolved
baseline so later saves follow field-level behavior:

- An unchanged field keeps its original placeholder even if the environment is
  rotated or unset before the save.
- Changing a referenced field replaces only that field with the new value.
- Unrelated references in the same top-level node remain placeholders.
- Named list entries, such as product instances and bookmark sources, retain
  their own references when entries are reordered or removed.

For example, `aws-auth` can read its username and password from Windows
environment variables:

```yaml
aws:
  enabled: true
  domain: HBEU
  username: "${AWS_AUTH_USERNAME}"
  password: "%AWS_AUTH_PASSWORD%"
```

Set them for the current PowerShell process before running `aws-auth`:

```powershell
$env:AWS_AUTH_USERNAME = "GB-SVC-XXX-XXX"
$env:AWS_AUTH_PASSWORD = "your-password"
aws-auth login --account 123456 --role ADFS-ReadOnly --profile saml --json
```

The referenced variable must be set to a non-empty value or config loading
fails with `config_env_missing`. For nodes read through the shared product
loader, use names such as `AWS_AUTH_USERNAME` rather than recognized `EFP_*`
config variables: any recognized `EFP_*` variable activates the separate
whole-config environment mode.

## YAML Structure

```yaml
version: 1

jira:
  default_instance: jira-main
  instances: []

confluence:
  default_instance: confluence-main
  instances: []

jenkins:
  default_instance: ci
  instances: []

nexus:
  default_instance: repo
  instances: []

splunk:
  default_instance: prod
  instances:
    - name: prod
      base_url: https://splunk-api.example.test:8089   # management REST port, not the web UI
      auth:
        type: bearer_token             # bearer_token | basic_password
        token: "${SPLUNK_TOKEN}"
      default_index: main              # prepended as index=main when a query names no index
      default_earliest: -1h            # earliest_time when --earliest is omitted
      max_results: 1000                # hard cap on results per search

appd:
  default_instance: prod
  instances:
    - name: prod
      base_url: https://appd.example.test:8090   # with or without /controller
      account: customer1                        # Controller account; qualifies bare names as name@account
      auth:
        type: api_client                        # api_client | basic_password | bearer_token
        username: efp-reader                    # API client name (api_client) or user (basic_password)
        api_key: "${APPD_CLIENT_SECRET}"        # API client secret
      verify_ssl: true
      ca_cert: ""

pgsql:
  default_instance: analytics
  instances:
    - name: analytics
      host: db.example.test
      port: 5432
      database: analytics
      username: readonly
      password: "${PGSQL_ANALYTICS_PASSWORD}"
      sslmode: verify-full
      ca_cert: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      statement_timeout_seconds: 30
      max_rows: 5000

aws:
  enabled: true
  provider: adfs-assume          # adfs-assume | saml2aws | assume-role
  domain: HBEU
  username: "${AWS_AUTH_USERNAME}"
  password: "%AWS_AUTH_PASSWORD%"
  idp_url: ""                    # saml2aws only: ADFS IdP-initiated sign-on URL
  source_profile: ""             # assume-role only: profile that already resolves credentials (default: default)
  default_account: cps-dev
  default_region: ap-east-1
  session_duration_seconds: 3600
  kubeconfig_path: ~/.efp/kube/config
  accounts:
    - name: cps-dev
      account_id: "818354133892"
      role: ADFS-ReadOnly
      regions: [ap-east-1, eu-west-1]
    - name: dcc-dev
      account_id: "334430002784"
      role: ADFS-ReadOnly
      regions: [ap-east-1]
      enabled: true

browser:
  bookmarks:
    sources:
      - name: company
        url: https://portal.example.test/agent-bookmarks.yaml
      - name: public
        url: https://static.example.test/bookmarks.json

copilot:
  provider: github_copilot_plugin
  api:
    endpoint_kind: responses
    base_url: https://api.githubcopilot.com
    timeout_seconds: 90
    use_system_proxy: true
  auth:
    method: device_code
    github_host: github.com
    github_user: ""
    github_access_token: ""
    github_access_token_expires_at: ""
    copilot_token_file: ~/.efp/tmp/copilot_token
    updated_at: ""

inspect_image:
  provider: github_copilot_plugin # github_copilot_plugin | ai_platform
  defaults:
    model: gpt-5.4-mini
    reasoning: medium
    output: text
  limits:
    max_image_bytes: 3145728
    max_images_per_call: 1
    allowed_mime_types:
      - image/jpeg
      - image/png
      - image/webp
      - image/gif
  privacy:
    store_raw_image: false
    store_raw_response: false
    redact_tokens_in_logs: true

ai_platform:
  chat:
    host: https://ai-platform.example.internal
    uri: /v1/api/v1/chat/completions
  ib2b:
    host: https://dsp.example.internal
    uri: /dsp/rest-sts/DSP_iB2B/iB2B_tokenTranslator_v2?_action=translate
  auth:
    username: ""
    password: ""
    usercase: ""
    token_file: ~/.efp/tmp/ai_platform_token
    trust_token_header: X-XXXX-E2E-Trust-Token
    tracking_prefix: EFP
    updated_at: ""
```

## Jira Instance Fields

- `name`
- `base_url`
- `api_version`: `"2"`
- `rest_path`: `"/rest/api/2"`
- `auth.type`: `basic_password | basic_api_key | bearer_token`
- `auth.username`
- `auth.password`
- `auth.api_key`
- `auth.token`
- `default_project`
- `verify_ssl`
- `ca_cert`
- `zephyr`

## Confluence Instance Fields

- `name`
- `base_url`
- `rest_path`: `"/rest/api"`
- `auth` with the same structure as Jira
- `default_space`
- `verify_ssl`
- `ca_cert`

## Jenkins Instance Fields

- `name`
- `base_url`
- `rest_path`: normally empty for Jenkins
- `auth.type`: `basic_password | basic_api_key | bearer_token`
- `auth.username`
- `auth.password`
- `auth.api_key`
- `auth.token`
- `crumb_mode`: `auto | always | never`
- `verify_ssl`
- `ca_cert`

`crumb_mode=auto` fetches `/crumbIssuer/api/json` for state-changing requests and tolerates a missing crumb issuer. Use `always` when the controller requires crumbs and you want crumb failures to be explicit. Use `never` only for controllers where CSRF crumbs are disabled or handled outside this CLI.

## Nexus Instance Fields

`nexus` is owned by the read-only `nexus` CLI (Sonatype Nexus Repository 3).

```yaml
nexus:
  default_instance: repo
  instances:
    - name: repo
      base_url: https://nexus.example.test
      rest_path: ""
      auth:
        type: basic_password
        username: ci-reader
        password: "${NEXUS_PASSWORD}"
      verify_ssl: true
      ca_cert: ""
    - name: public
      base_url: https://public-nexus.example.test
```

- `name`
- `base_url`: the Nexus Repository 3 root, for example `https://nexus.example.test`
- `rest_path`: normally empty, which means `/service/rest/v1`
- `auth.type`: `basic_password | basic_api_key | bearer_token`; omit the whole `auth` block for anonymous read access (no Authorization header is sent)
- `auth.username`: the Nexus user name, or the user token name code
- `auth.password`: the Nexus password
- `auth.api_key`: the user token passcode, sent as HTTP basic auth together with `auth.username`
- `auth.token`: a bearer token
- `verify_ssl`
- `ca_cert`

Managed runtimes inject the same fields as `EFP_NEXUS_DEFAULT_INSTANCE`, `EFP_NEXUS_INSTANCES_0_NAME`, `EFP_NEXUS_INSTANCES_0_BASE_URL`, `EFP_NEXUS_INSTANCES_0_REST_PATH`, `EFP_NEXUS_INSTANCES_0_AUTH_TYPE`, `EFP_NEXUS_INSTANCES_0_AUTH_USERNAME`, `EFP_NEXUS_INSTANCES_0_AUTH_PASSWORD`, `EFP_NEXUS_INSTANCES_0_AUTH_API_KEY`, `EFP_NEXUS_INSTANCES_0_AUTH_TOKEN`, `EFP_NEXUS_INSTANCES_0_VERIFY_SSL`, and `EFP_NEXUS_INSTANCES_0_CA_CERT`. When they are set, `nexus instance` and `nexus auth` writes require an explicit `--config <path>`.

## Splunk Instance Fields

- `name`
- `base_url`: the management REST URL, usually `https://<host>:8089` (not the web UI port)
- `auth.type`: `bearer_token | basic_password`
- `auth.token`: Splunk authentication token for `bearer_token`, sent as `Authorization: Bearer`
- `auth.username` / `auth.password`: session login for `basic_password` through `/services/auth/login`; the session key is kept in memory for one process and never written or printed
- `default_index`: prepended as `index=<name>` when a search does not constrain the index (queries starting with `|` are never rewritten)
- `default_earliest`: `earliest_time` used when `--earliest` is omitted (default `-1h`)
- `max_results`: hard cap on results one search may return (default `1000`); a `--count` above it is rejected with `invalid_args`
- `verify_ssl`
- `ca_cert`

`rest_path` is not used by `splunk`; every command addresses absolute `/services/...` paths under `base_url`. Managed runtimes inject the same fields as `EFP_SPLUNK_DEFAULT_INSTANCE`, `EFP_SPLUNK_INSTANCES_0_NAME`, `EFP_SPLUNK_INSTANCES_0_BASE_URL`, `EFP_SPLUNK_INSTANCES_0_AUTH_TYPE`, `EFP_SPLUNK_INSTANCES_0_AUTH_TOKEN` (or `_AUTH_USERNAME` / `_AUTH_PASSWORD`), `EFP_SPLUNK_INSTANCES_0_DEFAULT_INDEX`, `EFP_SPLUNK_INSTANCES_0_DEFAULT_EARLIEST`, and `EFP_SPLUNK_INSTANCES_0_MAX_RESULTS`.

## AppDynamics Instance Fields

`appd` is owned by the `appd` CLI. In managed runtimes the equivalent indexed environment settings are `EFP_APPD_DEFAULT_INSTANCE`, `EFP_APPD_INSTANCES_0_NAME`, `EFP_APPD_INSTANCES_0_BASE_URL`, `EFP_APPD_INSTANCES_0_ACCOUNT`, `EFP_APPD_INSTANCES_0_AUTH_TYPE`, `EFP_APPD_INSTANCES_0_AUTH_USERNAME`, `EFP_APPD_INSTANCES_0_AUTH_API_KEY`, and `EFP_APPD_INSTANCES_0_AUTH_PASSWORD`.

- `name`
- `base_url`: Controller URL such as `https://appd.example.test:8090`; a trailing `/controller` is accepted and stripped because the CLI builds `/controller/rest/...` paths itself
- `account`: Controller account name; a bare `auth.username` is qualified as `username@account` for both auth types
- `rest_path`: reserved, normally empty
- `auth.type`: `api_client | basic_password | bearer_token`
- `auth.username`: API client name (`api_client`) or user name (`basic_password`); may already be written as `name@account`
- `auth.api_key`: API client secret (`api_client`); an `auth.token` value is moved into `api_key` for this type
- `auth.password`: password for `basic_password`
- `auth.token`: pre-issued Controller access token for `bearer_token` (config or env only; not set by `appd auth login`)
- `verify_ssl`
- `ca_cert`

`api_client` is the OAuth client-credentials grant of AppDynamics API Clients: `appd` posts `client_id=<username>@<account>` and the secret to `/controller/api/oauth/access_token`, keeps the returned bearer token in memory for the lifetime of the process, and never writes it to disk or output. Store credentials without shell history with `appd auth login --auth-type api_client --username <api-client-name> --api-key-stdin` or `--auth-type basic_password --username <user> --password-stdin`.

## PostgreSQL Instance Fields

The `pgsql` node holds named PostgreSQL connections for the read-only `pgsql` CLI. It mirrors the `default_instance`/`instances` shape of the other products, but an entry carries connection fields instead of a base URL:

```yaml
pgsql:
  default_instance: analytics
  instances:
    - name: analytics
      host: db.example.test
      port: 5432
      database: analytics
      username: readonly
      password: "${PGSQL_ANALYTICS_PASSWORD}"
      sslmode: verify-full
      ca_cert: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      statement_timeout_seconds: 30
      max_rows: 5000
      enabled: true
```

- `name`: instance name used by `--instance`
- `host`: host name or address (required)
- `port`: `5432` when omitted
- `database`: database name (required)
- `username`: database role; give the CLI a read-only role (required)
- `password`: role password; write it with `pgsql instance add --password-stdin` or `pgsql auth login --password-stdin`, or reference an environment variable. When empty, libpq conventions (`PGPASSWORD`, `~/.pgpass`) still apply.
- `sslmode`: `disable | allow | prefer | require | verify-ca | verify-full`; `require` when omitted
- `ca_cert`: PEM CA bundle stored inline (`pgsql instance add --ca-cert-file ./ca.pem`); it is written to a `0600` temp file and passed as `sslrootcert` for the duration of each connection
- `statement_timeout_seconds`: server-side `statement_timeout` applied to every statement; `30` when omitted. `pgsql query --timeout-sec` can only lower it.
- `max_rows`: upper bound for `--limit`; `5000` when omitted
- `enabled`: `false` hides the instance from resolution (`instance_disabled`)

Managed runtimes inject the same node as `EFP_PGSQL_DEFAULT_INSTANCE`, `EFP_PGSQL_INSTANCES_0_NAME`, `EFP_PGSQL_INSTANCES_0_HOST`, `EFP_PGSQL_INSTANCES_0_PORT`, `EFP_PGSQL_INSTANCES_0_DATABASE`, `EFP_PGSQL_INSTANCES_0_USERNAME`, `EFP_PGSQL_INSTANCES_0_PASSWORD`, `EFP_PGSQL_INSTANCES_0_SSLMODE`, `EFP_PGSQL_INSTANCES_0_CA_CERT`, `EFP_PGSQL_INSTANCES_0_STATEMENT_TIMEOUT_SECONDS`, and `EFP_PGSQL_INSTANCES_0_MAX_ROWS`.

## Browser Bookmarks

`browser.bookmarks.sources` contains bookmark manifest locations, not individual bookmark entries. The optional source `description` helps agents choose a relevant collection:

```yaml
browser:
  bookmarks:
    sources:
      - name: company
        description: Internal company services.
        url: https://portal.example.test/agent-bookmarks.yaml
      - name: personal
        description: Personal search and productivity websites.
        url: ~/.efp/browser/bookmarks/personal.yaml
```

Source names are required and case-insensitively unique. Descriptions are optional. The `url` field accepts an absolute HTTP/HTTPS URL without embedded credentials, a `file://` URL, an absolute local file path, or a `~/...` path. Relative paths are rejected. The equivalent indexed environment settings are `EFP_BROWSER_BOOKMARKS_SOURCES_0_NAME`, `EFP_BROWSER_BOOKMARKS_SOURCES_0_DESCRIPTION`, and `EFP_BROWSER_BOOKMARKS_SOURCES_0_URL`.

Each source can return JSON or YAML:

```yaml
version: 1
bookmarks:
  - name: Example Search
    aliases: [search portal, web search]
    description: Search example content.
    url: https://search.example.test/
```

Each bookmark requires `name`, `description`, and an absolute HTTP/HTTPS `url`; `aliases` is optional. Unknown fields are rejected. `browser bookmark list --json` loads every configured source on each invocation, merges healthy results in source order, and does not use or write a cache. Repeat `--source <name>` to select one or more sources by case-insensitive name. The Agent opens the returned bookmark URL with the existing `browser open` command.

Manage source registrations and their optional descriptions with `browser bookmark source list/add/update/remove`. Source removal requires `--yes` and removes only the registration; it never changes the remote or local manifest. When EFP configuration is supplied through indexed environment variables, source writes require an explicit `--config <path>`.

Manage entries in configured local file sources with `browser bookmark add/update/remove --source <name>`. HTTP/HTTPS sources are read-only. For personal data, the recommended explicitly registered location is `~/.efp/browser/bookmarks/<name>.yaml`; the CLI does not scan that directory and does not implicitly load `~/.efp/bookmarks.yaml`. A first `bookmark add` creates a missing local manifest and parent directory.

## Browser Serve

`browser.serve` holds the defaults for `browser serve`, the local bridge used by the EFP Portal local browser connector:

```yaml
browser:
  serve:
    port: 8765
    allowed_origin: https://portal.example.test
```

`port` defaults to `8765` (the bridge tries `port` through `port+5` when the port is busy). `allowed_origin` is the single Portal origin echoed in `Access-Control-Allow-Origin`; it is empty by default and is written by `browser serve --register-protocol --origin <origin>`. The equivalent environment settings are `EFP_BROWSER_SERVE_PORT` and `EFP_BROWSER_SERVE_ALLOWED_ORIGIN`. Explicit `--port` and `--origin` flags override both.

## Copilot Auth

`copilot.auth` stores shared GitHub/Copilot authentication metadata for commands that use Copilot-backed APIs. The short-lived `copilot_token` is not stored in `config.yaml`; it is stored in the file named by `copilot.auth.copilot_token_file`, which defaults to `~/.efp/tmp/copilot_token`.

The token file uses YAML:

```yaml
copilot_token: ""
copilot_token_expires_at: ""
updated_at: ""
```

## inspect-image Providers

`inspect_image.provider` selects the image-inspection backend:

- `github_copilot_plugin`: uses the GitHub Copilot plugin `/responses` endpoint.
- `ai_platform`: uses the enterprise AI Platform `/chat/completions` endpoint after exchanging an iB2B JWT.

Model names are not locally restricted. `inspect_image.defaults.model` defaults to `gpt-5.4-mini`, and `--model <name>` is passed through to the selected provider.

Provider-specific endpoint settings live at the root provider nodes. `copilot.api` configures the GitHub Copilot `/responses` endpoint, while `ai_platform.chat` and `ai_platform.ib2b` configure AI Platform.

## AI Platform Auth

`ai_platform.auth.username`, `ai_platform.auth.password`, and `ai_platform.auth.usercase` are used to call the configured iB2B token translator. The translator response must include `issued_token`; inspect-image treats that token as short-lived for 30 seconds and stores it outside the main config in `ai_platform.auth.token_file`, defaulting to `~/.efp/tmp/ai_platform_token`.

The AI Platform `/chat/completions` request sends:

- `X-XXXX-E2E-Trust-Token` or the configured `trust_token_header`: the short-lived iB2B token.
- `x-correlation-id` and `x-usersession-id`: generated from `tracking_prefix` and the current timestamp.
- Body field `user`: the configured `usercase`.

The chat request supports exactly one local image and uses OpenAI-style content with a `text` item and an `image_url` item.

## TLS and CA Behavior

- `verify_ssl=false` disables certificate verification and is intended only for internal testing.
- `ca_cert` can embed PEM text for private CA trust.

## Mobile Auto

`mobile-auto` stores BrowserStack device-cloud settings under the `mobile-auto` YAML node and prefers environment credentials:

```yaml
mobile-auto:
  default_provider: browserstack
  state_dir: ~/.efp/mobile-auto
  artifacts_dir: ~/.efp/artifacts/mobile-auto
  retention_hours: 72
  defaults:
    platform: android
    network_mode: public
    idle_timeout_seconds: 300
    new_command_timeout_seconds: 300
    interactive_debugging: true
    video: true
  browserstack:
    api_base_url: https://api-cloud.browserstack.com
    appium_base_url: https://hub.browserstack.com/wd/hub
    username_env: BROWSERSTACK_USERNAME
    access_key_env: BROWSERSTACK_ACCESS_KEY
    username: ""
    access_key: ""
    verify_ssl: true
    ca_cert: ""
    http_proxy:
      proxy_host: ""
      proxy_port: 0
      proxy_user_env: ""
      proxy_pass_env: ""
      no_proxy_hosts: []
      disable_proxy_discovery: false
      force_proxy: false
    local:
      mode: managed
      binary: BrowserStackLocal
      binary_env: BROWSERSTACK_LOCAL_BINARY
      default_hold_minutes: 10
      max_hold_minutes: 30
      ready_timeout_seconds: 30
      heartbeat_seconds: 60
      force_local: false
      disable_proxy_discovery: false
      force_proxy: false
      proxy_host: ""
      proxy_port: 0
      proxy_user_env: ""
      proxy_pass_env: ""
      only_automate: false
      force: false
      include_hosts: []
      exclude_hosts: []
```

Use `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY` for credentials when possible; environment credentials take precedence over stored config values. To persist credentials into `~/.efp/config.yaml`, run:

```bash
printf '%s\n' "$BROWSERSTACK_ACCESS_KEY" | mobile-auto auth login --username "$BROWSERSTACK_USERNAME" --access-key-stdin --json
```

`MOBILE_AUTO_STATE_DIR` and `MOBILE_AUTO_ARTIFACTS_DIR` override the state and artifact roots in CI. State and artifact directories are created outside the main config with restrictive permissions where the platform supports them.

`mobile-auto.browserstack.http_proxy` controls the Go HTTP clients used for BrowserStack REST and Appium hub requests. When it is unset, the CLI can still use standard non-empty `HTTPS_PROXY`, `HTTP_PROXY`, `ALL_PROXY`, and `NO_PROXY` environment variables unless `disable_proxy_discovery` is true. `proxy_user_env` and `proxy_pass_env` name environment variables read at startup; do not store proxy credentials directly in `config.yaml`.

For enterprise networks, `mobile-auto.browserstack.local.proxy_user_env` and `proxy_pass_env` name environment variables read at tunnel startup; do not store proxy credentials directly in `config.yaml`. The Local flags are passed only for fields explicitly configured.

## AWS Auth Fields

`aws` is owned by `aws-auth`. Every scalar has an `EFP_AWS_<FIELD>` env
equivalent in managed runtimes; accounts are indexed as
`EFP_AWS_ACCOUNTS_<i>_<FIELD>` and regions as `EFP_AWS_ACCOUNTS_<i>_REGIONS_<j>`.

| Field | Meaning |
|---|---|
| `enabled` | Opt-out switch for the whole node. |
| `provider` | `adfs-assume` (default), `saml2aws`, or `assume-role`. |
| `domain`, `username`, `password` | Directory credentials used by the ADFS providers; not needed for `assume-role`. |
| `idp_url` | ADFS IdP-initiated sign-on URL; required by `saml2aws`. |
| `source_profile` | `assume-role` only: profile whose credentials assume each account role (default `default`). |
| `default_account` | Account used when `--account` is omitted. |
| `default_region` | Region used when an account lists none. |
| `session_duration_seconds` | SAML session length requested by `saml2aws` (default 3600). |
| `kubeconfig_path` | Where `eks kubeconfig` writes contexts when `KUBECONFIG` is unset (default `~/.efp/kube/config`). |
| `accounts[].name` | Account label; also the default AWS CLI profile name and the `<account>/<cluster>` context prefix. |
| `accounts[].account_id` | 12-digit AWS account id. |
| `accounts[].role` | IAM role name to assume, for example `ADFS-ReadOnly`. |
| `accounts[].role_arn` | Optional explicit role ARN; derived from `account_id` and `role` otherwise. |
| `accounts[].regions` | Regions in preference order; the first is the default for verification and EKS commands. |
| `accounts[].profile` | Optional AWS CLI profile override. |
| `accounts[].enabled` | Opt-out switch for one account. |
