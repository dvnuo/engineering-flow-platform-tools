# Configuration

Default config path: `~/.efp/config.yaml`

Environment override: `EFP_CONFIG`

Legacy environment overrides are still accepted for compatibility:

- Jira and Confluence: `ATLASSIAN_CONFIG`
- inspect-image: `INSPECT_IMAGE_CONFIG`

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

copilot:
  provider: github_copilot_plugin
  auth:
    method: device_code
    github_host: github.com
    github_user: ""
    github_access_token: ""
    github_access_token_expires_at: ""
    copilot_token_file: ~/.efp/tmp/copilot_token
    updated_at: ""

inspect_image:
  api:
    endpoint_kind: responses
    base_url: https://api.githubcopilot.com
    timeout_seconds: 90
    use_system_proxy: true
  defaults:
    model: gpt-5.4
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

## Copilot Auth

`copilot.auth` stores shared GitHub/Copilot authentication metadata for commands that use Copilot-backed APIs. The short-lived `copilot_token` is not stored in `config.yaml`; it is stored in the file named by `copilot.auth.copilot_token_file`, which defaults to `~/.efp/tmp/copilot_token`.

The token file uses YAML:

```yaml
copilot_token: ""
copilot_token_expires_at: ""
updated_at: ""
```

## TLS and CA Behavior

- `verify_ssl=false` disables certificate verification and is intended only for internal testing.
- `ca_cert` can embed PEM text for private CA trust.

## Mobile

`mobile` stores settings under the `mobile` YAML node and prefers environment credentials:

```yaml
mobile:
  default_provider: browserstack
  state_dir: ~/.efp/mobile
  artifacts_dir: ~/.efp/artifacts/mobile
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
    local:
      mode: managed
      binary: BrowserStackLocal
      binary_env: BROWSERSTACK_LOCAL_BINARY
      default_hold_minutes: 10
      max_hold_minutes: 30
      ready_timeout_seconds: 30
      heartbeat_seconds: 60
      force_local: false
      include_hosts: []
      exclude_hosts: []
```

Use `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY` for credentials when possible; environment credentials take precedence over stored config values. To persist credentials into `~/.efp/config.yaml`, run:

```bash
printf '%s\n' "$BROWSERSTACK_ACCESS_KEY" | mobile auth login --username "$BROWSERSTACK_USERNAME" --access-key-stdin --json
```

`MOBILE_STATE_DIR` and `MOBILE_ARTIFACTS_DIR` override the state and artifact roots in CI. State and artifact directories are created outside the main config with restrictive permissions where the platform supports them.
