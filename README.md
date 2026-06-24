# Engineering Flow Platform Tools

This repository hosts cross-platform Go-based CLI tools for agent, runtime, shell, and automation workflows. Current tool binaries include:

- `jira`
- `confluence`
- `jenkins`
- `aws-auth`
- `browser`
- `mobile`
- `inspect-image`
- `visual`

Jira and Confluence are the first tool family in this repository. Future tools may be added as separate command binaries under `cmd/<tool-name>`.

The project is designed for humans, shell scripts, and LLM/agent workflows that need stable machine-readable output.

## Design Goals

- Keep user-visible commands predictable across platforms.
- Return stable JSON envelopes with `ok`, `data`, and `error`.
- Support cross-platform builds for Go-based custom tool binaries.
- Keep commands suitable for humans, shell scripts, and LLM/agent workflows.
- Avoid printing credentials in normal, error, dry-run, or verbose output.
- Provide command metadata through `commands --json` and `schema <command> --json`.

## Current Tool Families

### Jira and Confluence

The Atlassian product integrations currently include:

- `jira`: Jira Server/Data Center automation
- `confluence`: Confluence Server/Data Center automation

Jira also includes `jira zephyr ...` commands for Zephyr Essential / Zephyr Squad test-management resources on the same Jira instance, including cycles, executions, semantic execution resolution, server status discovery, test steps, folders, attachments, defects, ZQL metadata/search, conservative summaries, and raw ZAPI catalog/access. Jira, Confluence, Jenkins, AWS auth, and inspect-image share `EFP_CONFIG` and `~/.efp/config.yaml`; each command owns its own YAML node so product settings do not interfere with one another.

### Jenkins

`jenkins` provides Jenkins controller automation for jobs, builds, queues, console logs, artifacts, Pipeline REST API resources, views, nodes, plugins, selected controller actions, and raw Jenkins API calls. It supports multiple Jenkins instances under the `jenkins` YAML node and handles Jenkins crumbs for state-changing requests.

### AWS Auth

`aws-auth` stores ADFS AWS auth settings under the `aws` YAML node and runs `adfs-assume` to authorize AWS credentials. Use `aws-auth auth login --password-stdin --json` to save domain, username, and password without putting the password in shell history. Use `aws-auth login --account <account-id> --role <role-name> --json` to run the authorization flow for a specific account and role.

### Browser

`browser` is a terminal-invoked CLI binary for Bash, PowerShell, or Windows cmd. It opens an internal URL with Chrome by default through DevTools, captures screenshot/HTML/network summary, and reports whether browser SSO appeared to complete. Edge/Chromium remain available with `--browser`. Persistent sessions can also inspect redacted page structure, semantic locators, accessibility-style refs, schema-based extraction, assertions, screenshot baseline checks, whitelisted workflow recording/running with locator fallback, optional workflow evidence bundles, form inspection/fill, frames, console/runtime errors, sanitized resource timing summaries, redacted fetch/XHR body previews, performance metadata, HAR-lite recorder/export metadata, tables/lists, data exports, scroll collection, page-state diffs, uploads, and download metadata. It uses dedicated browser profile and download directories by default and does not export cookies or tokens.

For VS Code GitHub Copilot, copy `cmd/browser/browser-cli.instructions.md` to `~/.copilot/instructions/browser-cli.instructions.md` so Copilot has durable guidance for browser probes.

### Mobile

`mobile` is a terminal-invoked BrowserStack App Automate real-device controller for agents. It uploads or resolves apps, lists and deterministically resolves devices, checks capacity, starts BrowserStack Local only for private-network sessions, creates remote Appium W3C sessions, observes native UI XML/screenshots into bounded element candidates, performs ref-based and viewport-relative actions with optional post-action waits, supports app lifecycle, permissions, keyboard/context/workflow/test/report helpers, generates Appium Inspector handoff/config artifacts, supports human handoff/resume, and collects artifacts without printing large logs or binary data.

Examples:

```bash
mobile doctor --json
mobile auth test --json
mobile app resolve --file ./app.apk --custom-id smoke --json
mobile run start --file ./app.apk --platform android --network public --json
mobile observe --run-id run-... --json
mobile locate --run-id run-... --role button --name Login --json
mobile tap --run-id run-... --ref obs-...:e1 --json
mobile type --run-id run-... --ref obs-...:e2 --text-env TEST_PASSWORD --json
mobile assert visible --run-id run-... --name Home --json
mobile inspector attach --run-id run-... --secret-mode env --json
mobile test run --file suite.yaml --junit-out junit.xml --json
mobile run handoff --run-id run-... --hold-for 10m --json
mobile run resume --run-id run-... --json
mobile run finish --run-id run-... --status passed --collect-artifacts --json
```

Set `BROWSERSTACK_USERNAME` and `BROWSERSTACK_ACCESS_KEY` for live BrowserStack calls. To store them in `~/.efp/config.yaml` instead, run `printf '%s\n' "$BROWSERSTACK_ACCESS_KEY" | mobile auth login --username "$BROWSERSTACK_USERNAME" --access-key-stdin --json`; environment variables still take precedence. Use `--network public` for public sessions. Use `private-managed` only when the BrowserStack device needs private/internal hosts and `BrowserStackLocal` is configured.

Current validation is strongest for Android native public sessions and mock/unit coverage. Treat `private-managed` Local, iOS native, hybrid WebView source semantics, and long human handoff/resume as beta until they are verified against live BrowserStack devices in your environment.

For Jira, Confluence, Jenkins, and AWS auth, copy `cmd/jira/jira-cli.instructions.md`, `cmd/confluence/confluence-cli.instructions.md`, `cmd/jenkins/jenkins-cli.instructions.md`, and `cmd/aws-auth/aws-auth-cli.instructions.md` into `~/.copilot/instructions/` so Copilot understands the JSON envelope, `--dry-run`, `--yes`, instance selection, AWS `saml` profile authorization, auth config, and error recovery conventions.

All CLI binaries return a stable JSON `invalid_args` envelope for command parsing failures when `--json` is present. On Windows `cmd`, use double quotes and `where <binary>` to resolve unstable PATH behavior.

### Inspect Image

`inspect-image` is a terminal-invoked CLI binary for Bash, PowerShell, or Windows cmd. It lets text-only agents inspect exactly one local image using a GitHub Copilot plugin backed vision model through `/responses`.

Examples:

```bash
inspect-image auth login
inspect-image auth test --json
inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --json
inspect-image inspect --image ./screenshot.png --prompt "Read the visible error and explain what is happening." --out ./inspect-image-result.json --json
inspect-image inspect --image ./diagram.webp --preset diagram --prompt "Explain this architecture diagram." --json
inspect-image commands --json
inspect-image schema inspect --json
inspect-image help llm
```

Supported image formats: JPEG, PNG, WEBP, GIF. Max size: 3145728 bytes.

For agents, `--json` is the default way to use `inspect-image`; human-facing interactive `auth login` prompts can omit it. Stdout is the primary output path. Use `--out <file>` only when terminal stdout capture is unreliable or you want a second JSON envelope copy, preferably inside the current workspace. Use `--verbose` for non-secret stage diagnostics on stderr.

For VS Code GitHub Copilot, copy `cmd/inspect-image/inspect-image-cli.instructions.md` to `~/.copilot/instructions/inspect-image-cli.instructions.md` so Copilot has durable guidance for when and how to invoke this CLI.

### Visual

`visual` generates complete offline static visualization artifacts from 33 semantic local templates, including dedicated UML templates. Installed templates default to `~/.efp/template/visual`; source checkouts and release archives can still pass `--template-dir ./templates/visual`. It validates input JSON, copies local template assets, and writes `index.html`, `manifest.json`, `manifest.js`, `data.js`, and `assets/**` to `--out`. It does not call Portal, MCP, Node/npm, a server, a CDN, or any remote asset.

For VS Code GitHub Copilot, copy `cmd/visual/visual-cli.instructions.md` to `~/.copilot/instructions/visual-cli.instructions.md` so Copilot uses `visual` as a terminal CLI and returns the generated `index.html` path.

## Quick Install

Download a release artifact for your platform, place `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `mobile`, `inspect-image`, and `visual` on your `PATH`, then run:

```bash
jira version --json
confluence version --json
jenkins version --json
aws-auth version --json
browser version --json
mobile version --json
inspect-image version --json
visual version --json
```

## Config File

- Environment override: `EFP_CONFIG`
- Default path: `~/.efp/config.yaml`
- Legacy overrides still work: `ATLASSIAN_CONFIG` for Jira/Confluence and `INSPECT_IMAGE_CONFIG` for inspect-image.
- Short-lived Copilot tokens are stored outside the main config at `~/.efp/tmp/copilot_token`.

Complete example:

```yaml
version: 1

jira:
  default_instance: local
  instances:
    - name: local
      base_url: https://jira.example.test
      api_version: "2"
      rest_path: /rest/api/2
      auth:
        type: basic_api_key
        username: user@example.test
        api_key: redacted
      default_project: PROJ
      verify_ssl: true
      ca_cert: ""
      zephyr:
        enabled: false
        api_family: server
        rest_path: /rest/zapi
        default_version_id: ""
        strict_status: true
        status_map:
          pass: 1
          fail: 2
          blocked: 3

confluence:
  default_instance: docs
  instances:
    - name: docs
      base_url: https://confluence.example.test
      rest_path: /rest/api
      auth:
        type: bearer_token
        token: redacted
      default_space: DOC
      verify_ssl: true
      ca_cert: ""

jenkins:
  default_instance: ci
  instances:
    - name: ci
      base_url: https://jenkins.example.test
      rest_path: ""
      crumb_mode: auto
      auth:
        type: basic_api_key
        username: user@example.test
        api_key: redacted
      verify_ssl: true
      ca_cert: ""

aws:
  enabled: true
  domain: HBEU
  username: user@example.test
  password: redacted

visual:
  template_dir: ~/.efp/template/visual
  defaults:
    offline_strict: true
    data_mode: js-file

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
  provider: github_copilot_plugin
  api:
    endpoint_kind: responses
    base_url: https://api.githubcopilot.com
    timeout_seconds: 90
    use_system_proxy: true
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
```

Supported authentication modes:

- username/password
- username/API key
- bearer token/PAT

Config node ownership:

- `jira`: Jira instances, defaults, auth, TLS, and Zephyr settings.
- `confluence`: Confluence instances, defaults, auth, and TLS settings.
- `jenkins`: Jenkins instances, defaults, auth, TLS, and crumb behavior.
- `copilot`: GitHub/Copilot authentication shared by commands that use Copilot-backed APIs.
- `inspect_image`: inspect-image API defaults, model defaults, image limits, and privacy settings.
- `aws`: AWS authorization settings used by `aws-auth login`.
- `visual`: offline artifact template directory and default render behavior.

## AWS Auth Examples

```bash
printf '%s\n' "$AWS_AD_PASSWORD" | aws-auth auth login --domain HBEU --username GB-SVC-XXX-XXX --password-stdin --json
aws-auth login --account 123456 --role ADFS-ReadOnly --profile saml --json
aws-auth --config ~/.efp/config.yaml login --account 123456 --role ADFS-ReadOnly --profile saml --json
aws-auth commands --json
aws-auth schema login --json
aws-auth help llm --json
```

`aws-auth login` invokes `adfs-assume` with `--profile saml` by default and passes the configured password through `AD_PASS` instead of command arguments.

## Jenkins Examples

```bash
jenkins auth test --instance ci --json
jenkins job list --depth 2 --json
jenkins job get folder/app-main --json
jenkins job build folder/app-main --json
jenkins job build-with-params folder/app-main --param BRANCH=main --json
jenkins queue get 123 --json
jenkins build status folder/app-main lastBuild --json
jenkins build log folder/app-main 42 --json
jenkins build log-follow folder/app-main 42 --max-rounds 3 --json
jenkins build artifacts folder/app-main 42 --json
jenkins artifact download folder/app-main 42 target/app.jar --output app.jar --json
jenkins pipeline runs folder/app-main --json
jenkins api get /api/json --query depth=1 --json
jenkins version --json
```

## Jira Examples

```bash
jira auth test --instance local --json
jira issue get PROJ-123 --instance local --json
jira issue search --jql 'project = PROJ' --limit 10 --json
jira zephyr doctor --project PROJ --json
jira zephyr summary --project PROJ --version-id -1 --json
jira zephyr version resolve --project PROJ --name "1.0" --json
jira zephyr cycle list --project PROJ --version-id -1 --json
jira zephyr cycle resolve --project PROJ --name "Sprint 42 Regression" --version-id -1 --json
jira zephyr execution list --cycle-id 20000 --project-id 10000 --version-id -1 --status FAIL --json
jira zephyr execution resolve --cycle-id 20000 --issue PROJ-123 --project PROJ --version-id -1 --json
jira zephyr execution update-status --cycle-id 20000 --issue PROJ-123 --status PASSED --dry-run --json
jira zephyr execution add-tests-to-cycle --cycle-id 20000 --project-id 10000 --version-id -1 --issues PROJ-123,PROJ-124 --folder-name Smoke --create-folder --dry-run --json
jira zephyr execution bulk-update-status --cycle-id 20000 --project-id 10000 --issues PROJ-123,PROJ-124 --status PASS --dry-run --json
jira zephyr execution bulk-update-status --execution-ids 30000,30001 --status PASS --dry-run --json
jira zephyr archive list --project-id 10000 --version-id -1 --json
jira zephyr archive executions --cycle-id 20000 --project-id 10000 --issues PROJ-123,PROJ-124 --yes --dry-run --json
jira zephyr customfield list --entity-type EXECUTION --project-id 10000 --json
jira zephyr status list --json
jira zephyr api catalog --json
jira zephyr cycle delete 20000 --yes --dry-run --json
jira version --json
```

## Confluence Examples

```bash
confluence auth test --instance docs --json
confluence search --cql 'space = ENG' --instance docs --json
confluence page create --instance docs --space ENG --title "Test Page" --body "<p>Hello</p>" --dry-run --json
confluence version --json
```

## Browser Examples

Windows:

```powershell
browser probe --url "https://intranet.example.test/app" --selector ".user-avatar" --wait 10 --out result --json
```

Validate whether access depends on a prior browser cookie:

```powershell
browser probe --url "https://intranet.example.test/app" --selector ".user-avatar" --clean-profile --wait 10 --out result-clean --json
```

Optional page-context API fetch:

```bash
browser probe --url "https://intranet.example.test/app" --fetch-api "/api/me" --json
```

Persistent browser automation session. Managed sessions attempt to detach the browser process from the short-lived CLI or agent command process, so later agent turns can reuse the same DevTools endpoint:

```bash
browser session start --name default --url "https://intranet.example.test/app" --json
browser session discover --ports 9222,9223 --json
browser session attach --name user-demo --debug-port 9222 --json
browser tab current --session default --json
browser page snapshot --session default --json
browser page extract --session default --selector ".user-avatar" --json
browser page extract-schema --session default --file schema.yaml --json
browser page find --session default --role button --name Save --json
browser page ax --session default --json
browser page outline --session default --json
browser page network --session default --filter "/api/" --json
browser page metrics --session default --limit-resources 10 --json
browser page console --session default --level error --json
browser frame list --session default --json
browser page table --session default --selector "table.results" --json
browser page list --session default --selector "nav" --json
browser page screenshot --session default --out result/page-screenshot.png --json
browser page screenshot --session default --selector ".avatar" --out result/avatar.png --json
browser page table-export --session default --selector "table.results" --out result/table.csv --format csv --json
browser page scroll-collect --session default --item-selector ".row" --out result/items.json --json
browser page diff --before before.json --after after.json --json
browser assert visible --session default --selector ".ready" --json
browser assert count --session default --selector ".result" --min 1 --json
browser assert screenshot --session default --baseline baseline.png --out actual.png --diff-out diff.png --json
browser workflow record --session default --out flow.yaml --duration-ms 10000 --json
browser workflow run --file flow.yaml --dry-run --var query=demo --report-out result/workflow-run.json --evidence-dir result/evidence --json
browser form inspect --session default --json
browser form fill --session default --file values.yaml --json
browser network start --session default --limit 500 --json
browser network list --session default --filter "/api/" --json
browser network export --session default --out result/network.har-lite.json --format har-lite --json
```

Bounded page actions:

```bash
browser page click --session default --selector "button.sign-in" --json
browser page click --session default --ref "axref-0-abcdef123456" --json
browser page type --session default --selector "input[name=q]" --text "search" --clear --json
browser page select --session default --ref "axref-1-abcdef123456" --label "Ready" --json
browser page check --session default --ref "axref-2-abcdef123456" --json
browser page press --session default --key Enter --json
browser page upload --session default --selector "input[type=file]" --file "./report.pdf" --json
browser page wait --session default --selector ".ready" --network-idle-ms 500 --json
browser page eval --session default --expr "document.title" --json
browser page fetch --session default --url "/api/me" --json
browser download wait --session default --filename-contains "report" --json
browser download list --session default --json
```

The current OpenCode runtime image consumes prebuilt binaries copied into `runtime-tools/` by an external pipeline. A future runtime change must place `browser` on `PATH`, and probe execution inside a runtime container also requires Edge/Chrome/Chromium in that image.

## Visual Examples

```bash
visual template categories --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --category mermaid --json
visual template schema mermaid.sequence --template-dir ./templates/visual --json
visual inspect-input --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --json
visual render --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --out ./out/sequence --title "Checkout Sequence" --json
visual inspect-render --template-dir ./templates/visual --out ./out/sequence --json
visual inspect-browser --template-dir ./templates/visual --out ./out/sequence --json
```

## URL Instance Routing

Commands that accept Jira issue URLs or Confluence page URLs can select the matching configured instance automatically. Use `--instance` when multiple configured instances could match the same URL.

## LLM/Agent Usage

Agents should first inspect available commands:

```bash
jira commands --json
confluence commands --json
jenkins commands --json
aws-auth commands --json
browser commands --json
inspect-image commands --json
visual commands --json
```

Then inspect the exact schema before calling a command:

```bash
jira schema issue.create --json
confluence schema page.create --json
jenkins schema job.build-with-params --json
aws-auth schema login --json
browser schema page.fetch --json
inspect-image schema inspect --json
visual schema render --json
visual schema inspect-input --json
```

For agents, default every `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json` so output handling always uses the stable `ok/data/error` envelope. Only omit `--json` when intentionally reading human-oriented `--help` text or a documented interactive human prompt. `aws-auth login` uses `adfs-assume --profile saml` by default. For `visual`, run `visual template categories --json`, `visual template list --category <category> --json`, `visual template get <template-id> --json`, and `visual template schema <template-id> --json` before render. Generate input JSON from the template schema and `visual_design`, inspect readability with `visual inspect-input`, revise low relation coverage, repetitive edge kinds, long labels, missing importance, or missing edge visibility before rendering, validate it, render to a new output directory, and return `data.artifact.entrypoint`. Inspect `error.code` and `error.hint` before retrying, run write commands with `--dry-run` first, and pass `--yes` for destructive operations.

For Zephyr, treat a Test Cycle as a Zephyr execution container rather than a Jira issue. When a user asks to update case `X` in cycle `Y`, prefer `jira zephyr execution update-status --cycle-id Y --issue X --status PASSED --json`; use `execution resolve` or `cycle resolve` first when the target is ambiguous, and use `status list` rather than hard-coding numeric status ids.

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

## Security Model

- Secrets are redacted from config display and command output.
- Config files are written with `0600` permissions where supported.
- Bearer tokens and basic auth credentials are only sent in Authorization headers.
- Absolute URLs outside the selected instance are blocked.
- `--verbose` is reserved for diagnostics and must not print credentials.
- Tests must not contain real credentials.

## Cross-Platform Build

```bash
bash scripts/build.sh
bash scripts/build.sh --snapshot
bash scripts/build.sh --os linux --arch amd64
bash scripts/build.sh --snapshot --os linux --arch amd64
```

```bat
scripts\build.bat
scripts\build.bat --snapshot
scripts\build.bat --os linux --arch amd64
scripts\build.bat --snapshot --os linux --arch amd64
```

Build outputs are placed under `dist/<goos>-<goarch>/` for linux, darwin, and windows on amd64 and arm64.

## Development And Testing

```bash
go mod tidy
go test ./...
go vet ./...
bash scripts/smoke.sh
```

On Windows:

```bat
go test ./...
go vet ./...
scripts\smoke.bat
```

## Release

Tags matching `v*` trigger the release workflow. Release archives are named `engineering-flow-platform-tools_<version>_<goos>_<goarch>`.

Current archives include `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `inspect-image`, `visual`, `templates/visual/**`, README, and docs.
