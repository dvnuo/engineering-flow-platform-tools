# LLM/Agent Usage

- For agents, default every `jira`, `confluence`, `jenkins`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json` so output handling always uses the stable `ok/data/error` envelope.
- Only omit `--json` when intentionally reading human-oriented `--help` text or when a documented interactive human prompt requires text output.
- Use --instance when multiple instances are configured.
- Full Jira/Confluence URLs can auto-select the instance.
- Use --dry-run before write operations.
- Use --yes for destructive operations.
- Inspect error.code and error.hint before retrying.
- Command parsing failures across `jira`, `confluence`, `jenkins`, `browser`, `inspect-image`, and `visual` return a JSON `invalid_args` envelope when `--json` is present.
- On Windows `cmd`, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and commands.
- If PATH lookup is unstable, run `where <binary>` and invoke the exact `.exe` path with double quotes.
- For VS Code GitHub Copilot, copy the CLI instruction files from `cmd/browser/browser-cli.instructions.md`, `cmd/jira/jira-cli.instructions.md`, `cmd/confluence/confluence-cli.instructions.md`, `cmd/jenkins/jenkins-cli.instructions.md`, and `cmd/inspect-image/inspect-image-cli.instructions.md` into `~/.copilot/instructions/`.

## Visual Artifact Usage

- Always use `--json`.
- Use the default `~/.efp/template/visual` catalog when it is installed there; use `--template-dir` when templates are in the workspace or release artifact.
- Do not read all 195 template details up front.
- Do not infer available templates from the `templates/visual` file tree.
- Discover templates only with `visual template categories`, `visual template list`, `visual template get`, and `visual template schema`.
- Run `visual template categories --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template list --category <category> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template get <template-id> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template schema <template-id> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Backward compatibility aliases work, but they are registry aliases rather than duplicate template directories; prefer the returned `canonical_id` for new inputs and examples.
- Do not invent template paths or point inputs at alias directories.
- Before every render, run `visual template schema <id> --json`, using `--template-dir` only when the catalog is not installed at `~/.efp/template/visual`.
- Write input JSON that follows `data.json_schema` and the returned `data.template.visual_design`; do not invent the input shape and do not generate JavaScript code.
- For graph inputs larger than a small overview, include short node `label` or `name` values, include `groups` or node `parent_id`/`group_id`/`group` fields, set `initial_view.mode: "overview"`, and mark noisy low-value edges with `visibility: "detail"` or `visibility: "hidden"`.
- Before validation/render, run `visual inspect-input --template <template-id> --input <input.json> --json` and use `data.warnings`, `data.summary`, and `data.recommendations` to reduce clutter. For graph inputs, fix `missing_display_labels`, high `orphan_node_count`, low `relation_coverage`, repetitive `dominant_edge_kinds`, long labels, missing `importance`, and missing edge `visibility` before rendering.
- Validate with `visual validate --template <template-id> --input <input.json> --json`, using `--template-dir` only when the catalog is not installed at `~/.efp/template/visual`.
- Render to a new output directory with `visual render --template <template-id> --input <input.json> --out <dir> --json`.
- Return `data.artifact.entrypoint` to the user.
- Visual effects are template-declared. Do not override them with generated JavaScript; choose the right template and provide better input data.
- Do not use remote assets, CDN URLs, runtime Node/npm, generated JavaScript, or network APIs.
- Use `--dry-run` to preview planned files before writing.
- The generated `index.html` is safe for `file://` and for Portal/runtime static proxy subpaths because asset paths, including the local Three.js module bridge, are relative.

Recommended template categories:

- `agent` or `debug` for agent runs, failures, incidents, traces, and recovery.
- `codebase` for repo, code, diff, dependency, test, coverage, and migration work.
- `runtime` for infra, service, adapter, session, event, sandbox, and capability maps.
- `project` for Jira, GitHub, Confluence, requirements, releases, and reviews.
- `knowledge` for evidence, research, citation, source, and answer lineage.
- `planning` for plans, tasks, workflows, automation, approvals, and goals.
- `business` for KPI, funnel, customer, revenue, support, capacity, and ops views.
- `education` for explanations, tutorials, lifecycle, process, and tradeoff visuals.

## Jenkins Automation

- Use `jenkins` for Jenkins jobs, queues, builds, console logs, artifacts, Pipeline REST API data, nodes, plugins, views, selected controller actions, and raw Jenkins API calls.
- Jenkins instances are configured under `jenkins.instances` in `~/.efp/config.yaml`.
- Use slash job paths for folders, for example `folder/app-main`.
- Trigger simple builds with `jenkins job build <job> --json`.
- Trigger parameterized builds with `jenkins job build-with-params <job> --param NAME=value --json`.
- After triggering, inspect `data.queue_id` and run `jenkins queue get <queue-id> --json` to find the executable build number.
- Use `jenkins build status <job> <build> --json` for current state and result.
- Use `jenkins build log <job> <build> --json` for full console text, or `jenkins build log-follow <job> <build> --json` for progressive text.
- Use `jenkins build artifacts <job> <build> --json` to list artifacts, then `jenkins artifact download <job> <build> <path> --output <file> --json` to download binary content.
- Use Pipeline commands only when the Jenkins Pipeline REST API plugin is installed.
- `build stop`, `queue cancel`, `job delete`, `view delete`, `system safe-restart`, and raw `api delete` require `--yes`.
- Use `--dry-run` before Jenkins write operations.

## Browser SSO Diagnostics

- Always call `browser schema probe --json` before constructing a probe command.
- Always use `--json`.
- `browser` is a terminal-invoked CLI binary for Bash, PowerShell, or Windows cmd, not an OpenCode built-in browser tool, MCP tool, or Web UI component.
- Use `--selector` for a deterministic login-success signal.
- Use `--clean-profile` when the user wants to distinguish true OS/enterprise SSO from cached browser session.
- Read `data.files.summary`, `data.files.screenshot`, `data.files.html`, and `data.files.network`.
- Use `--save-html=false` or `--save-screenshot=false` when the user wants to avoid writing those artifacts.
- Do not ask the tool to export cookies or tokens.
- Do not treat `negotiate_401_seen` as definitive proof; it is an indicator only.
- In OpenCode runtime, this command requires a browser executable in the runtime image. If no browser is installed, expect `browser_not_found`.

## Image Inspection

- Use `inspect-image` when you need to understand a local image, screenshot, UI, diagram, chart, or visible text.
- For image analysis, recognition, screenshot understanding, UI state inspection, diagram interpretation, chart reading, visual error analysis, or visible-text extraction from an image, use `inspect-image` as the only image-analysis path.
- Do not use OCR tools as the primary path, and do not write Python/OpenCV/Tesseract/image-parser scripts to recognize or interpret image content.
- `inspect-image` is a terminal-invoked CLI binary for Bash, PowerShell, or Windows cmd, not a Portal tool, runtime built-in tool, MCP tool, or Web UI component.
- For agents, `--json` is the default way to use this CLI. Add `--json` to `inspect`, `auth status`, `auth test`, `doctor`, `models`, `commands`, `schema`, `version`, and `help llm`.
- Only omit `--json` for human-facing interactive output such as asking the user to run `inspect-image auth login` and read the device-code prompt.
- Always call `inspect-image schema inspect --json` before constructing a complex command.
- Always use `--json`.
- Use `inspect-image inspect --image <path> --prompt "<task>" --json`.
- Stdout is the primary output path. If terminal stdout capture is unreliable, use `inspect-image inspect --image <path> --prompt "<task>" --out <workspace-file> --json`; `--out` writes an additional JSON envelope copy and does not replace stdout.
- Prefer a result file inside the current workspace or next to the inspected image, then read it with the file-read tool. Use shell commands such as `type` only when no file-read tool is available.
- Use `--verbose` for non-secret diagnostics when debugging command execution; it reports config load, image validation, auth checks, `/responses` request/response, output file writes, and JSON envelope status.
- Read `data.result.answer` first.
- For OCR-like tasks, read `data.result.visible_text`.
- If `ok=false`, inspect `error.code` and `error.hint`.
- If `inspect-image auth status --json` returns `token_state=refreshable` or `copilot_token_refreshable=true`, run `inspect-image auth test --json` or retry `inspect-image inspect --json`; do not ask the user to log in again.
- If `auth_required` or `auth_expired` is not refreshable, ask the user to run `inspect-image auth login`, wait for completion, and then retry `inspect-image inspect --json`; do not fall back to OCR, Python image recognition, or guessing.
- On Windows `cmd`, use double quotes, `where`, `dir`, and `cd`; avoid Bash-only commands such as `pwd`, `command -v`, `cat`, `ls`, `cd "$PWD"`, `$PWD`, and single quotes. If capture is unreliable, use `--out "%CD%\inspect-image-result.json"` rather than shell redirection.
- For VS Code GitHub Copilot, copy `cmd/inspect-image/inspect-image-cli.instructions.md` to `~/.copilot/instructions/inspect-image-cli.instructions.md` so this guidance is available during coding sessions.

## Jira Zephyr Test Management

- If a Jira URL contains `selectedItem=com.thed.zephyr.je`, treat it as a Zephyr test-management page.
- For a project you have not checked, first run `jira zephyr doctor --project <PROJECT> --json`.
- Use Jira core commands for issues, stories, bugs, comments, attachments, and workflows.
- Use `jira zephyr` for test cycles, executions, execution status, step results, defects, attachments, ZQL, reports, and test summary context.
- A Zephyr Test Cycle is a Zephyr container for test executions, not a Jira issue. Do not send cycle ids to `jira issue ...`.
- To update "case X in cycle Y", use `jira zephyr execution update-status --cycle-id Y --issue X --status PASSED --json`; the CLI resolves the execution id.
- Prefer `jira zephyr execution resolve --cycle-id <ID> --issue <KEY> --json` before writes when the user's wording or cycle context is uncertain.
- Use `jira zephyr cycle resolve --project <PROJECT> --name '<cycle name>' --version-id -1 --json` when the user gives a cycle name instead of a cycle id.
- Use `jira zephyr status list --json` and server status aliases instead of hard-coding numeric Zephyr status ids.
- Use `jira zephyr api catalog --json` and `jira zephyr api describe <endpoint-id> --json` to discover official long-tail ZAPI endpoints before falling back to raw `jira zephyr api ...`.
- Use `--dry-run` before Zephyr write operations unless the user has explicitly approved the write.
- Zephyr delete commands and raw `jira zephyr api delete` require `--yes`; do not add it until the user has confirmed the destructive action.
- Do not browser-scrape Jira Test pages unless the API is unavailable and the user explicitly asks for UI investigation.
- For Jira Test page URLs, prefer `jira zephyr resolve-url`, `jira zephyr summary`, `jira zephyr cycle list`, and `jira zephyr execution list` instead of browser scraping.

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

## Recommended Workflow

1. Discover commands with `jira commands --json`, `confluence commands --json`, `jenkins commands --json`, `browser commands --json`, `inspect-image commands --json`, or `visual commands --json`.
2. Inspect the exact command schema before constructing arguments.
3. Prefer full Jira issue URLs or Confluence page URLs when the user provides them.
4. Add `--instance` when the URL is ambiguous across configured instances.
5. Use `--dry-run` for create, update, add, set, upload, move, restore, watch, vote, assign, and transition commands.
6. Add `--yes` only after the user has explicitly confirmed a destructive operation.
7. Parse the JSON envelope and branch on `ok`.
8. On failure, branch on `error.code` before retrying.

## Schema Checks

Use schema output to avoid guessing required flags:

```bash
jira schema issue.create --json
jira schema issue.transition --json
jira schema zephyr.zql.search --json
jira schema zephyr.execution.update-status --json
jira schema zephyr.execution.resolve --json
jira schema zephyr.cycle.resolve --json
jira schema zephyr.api.catalog --json
jira schema zephyr.execution.bulk-update-status --json
confluence schema page.create --json
confluence schema page.update --json
browser schema probe --json
jenkins schema job.build-with-params --json
jenkins schema build.status --json
jenkins schema artifact.download --json
jenkins schema api.get --json
inspect-image schema inspect --json
visual schema render --json
visual schema inspect-input --json
```

The `required` field lists mandatory arguments and flags. The `flags` field includes type and description metadata suitable for tool planning.

## URL Routing

Jira issue URLs and Confluence page URLs can select an instance automatically. If a URL belongs to a configured instance, omit `--instance` unless multiple instances share the same base URL.

If the user also supplies `--instance`, the URL must belong to that instance. Otherwise the command returns `instance_url_mismatch` and must not send credentials to the URL.

## Output Rules

All automation should request JSON:

```bash
jira issue get PROJ-123 --json
confluence page get --id 123 --json
```

Successful responses contain `ok=true` and `data`. Failed responses contain `ok=false`, `error.code`, and `error.message`; many failures also include `error.hint`.
