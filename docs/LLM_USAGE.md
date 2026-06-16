# LLM/Agent Usage

- For agents, default every `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json` so output handling always uses the stable `ok/data/error` envelope.
- Only omit `--json` when intentionally reading human-oriented `--help` text or when a documented interactive human prompt requires text output.
- Use --instance when multiple instances are configured.
- Full Jira/Confluence URLs can auto-select the instance.
- Use --dry-run before write operations.
- Use --yes for destructive operations.
- Inspect error.code and error.hint before retrying.
- Command parsing failures across `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `inspect-image`, and `visual` return a JSON `invalid_args` envelope when `--json` is present.
- On Windows `cmd`, use double quotes and cmd-native commands such as `where`, `dir`, `cd`, and `type`; avoid Bash-only quoting and commands.
- If PATH lookup is unstable, run `where <binary>` and invoke the exact `.exe` path with double quotes.
- For VS Code GitHub Copilot, copy the CLI instruction files from `cmd/browser/browser-cli.instructions.md`, `cmd/jira/jira-cli.instructions.md`, `cmd/confluence/confluence-cli.instructions.md`, `cmd/jenkins/jenkins-cli.instructions.md`, `cmd/aws-auth/aws-auth-cli.instructions.md`, and `cmd/inspect-image/inspect-image-cli.instructions.md` into `~/.copilot/instructions/`.

## AWS Auth

- Use `aws-auth` to store ADFS AWS auth config and run the `adfs-assume` authorization flow.
- Configure it with `printf '%s\n' "$AWS_AD_PASSWORD" | aws-auth auth login --domain HBEU --username GB-SVC-XXX-XXX --password-stdin --json`.
- Do not pass passwords as command-line flags. Use `--password-stdin`.
- Run `aws-auth auth status --json` to inspect configured state with the password redacted.
- Run `aws-auth login --account 123456 --role ADFS-ReadOnly --json` to authorize credentials for a specific account and role.
- Human interactive `aws-auth login` may omit `--json` so the CLI can prompt for a missing account or role.
- If login fails with `execution_failed`, check that `adfs-assume` is installed and on `PATH`.

## Visual Artifact Usage

- Always use `--json`.
- Use the default `~/.efp/template/visual` catalog when it is installed there; use `--template-dir` when templates are in the workspace or release artifact.
- Do not infer available templates from the `templates/visual` file tree.
- Discover templates only with `visual template categories`, `visual template list`, `visual template get`, `visual template schema`, and `visual template guide`.
- Run `visual template categories --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template list --category <category> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template get <template-id> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template schema <template-id> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- Run `visual template guide <template-id> --json`, or add `--template-dir ./templates/visual` in a source checkout.
- The built-in public catalog has 28 `mermaid.*` templates in the `mermaid` category, one per supported Mermaid Diagram Syntax family.
- Do not invent template paths.
- Author Mermaid `.mmd` input. Pure official Mermaid can be passed directly to `visual inspect-input`, `visual inspect-plan`, `visual validate`, and `visual render` without `--template`; the CLI infers the matching `mermaid.*` template.
- Use EFP frontmatter only for quality-critical layout hints such as `efp.template`, `efp.camera`, `efp.canvas`, `efp.renderHints`, `efp.visual`, and `efp.view`. Keep the Mermaid body valid Mermaid.
- Use Mermaid syntax plus optional EFP frontmatter when layout, camera, render hints, or focus guidance is needed.
- For UML sequence diagrams, use Mermaid `sequenceDiagram` or `zenuml`; the CLI maps them to `mermaid.sequence` or `mermaid.zenuml`.
- For architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, iCraft-like, or isometric architecture requests, use Mermaid `architecture-beta`, `architecture`, `C4Context`, or EFP frontmatter `efp.template: mermaid.architecture`.
- For class, state, activity, component, and sequence diagrams, use Mermaid `classDiagram`, `stateDiagram`, `flowchart`, C4/architecture syntax, `sequenceDiagram`, or `zenuml`; the CLI maps them to the corresponding `mermaid.*` template.
- For graph inputs larger than a small overview, keep Mermaid node labels short, use subgraphs when helpful, and move low-value detail into fewer visible edges or optional EFP frontmatter hints.
- For graph-like, flow, relationship, spatial, and UML diagrams, use official Mermaid arrows and relationships instead of inventing data shapes. Optional EFP frontmatter can add `kind`, `provider`, `service`, `platform`, icon/model/color hints, label priority, or route hints when the Mermaid syntax alone is not enough for the desired visual quality.
- For isometric architecture inputs, use Mermaid `architecture` / `architecture-beta`, C4, or Mermaid plus optional EFP frontmatter for bounded zones, positioned services, local icon/model ids, explicit routes, camera, grid, and label density hints.
- Use `view.colorBy` or `renderHints.colorBy` plus `renderHints.showLegend=true` whenever color carries meaning. Good defaults are `provider`, `kind`, `status`, `group`, `phase`, `risk`, and `severity`.
- Use local icon/model ids from `templates/visual/_shared/asset-registry.json`. Do not use external image/model URLs. Current AWS icon ids are local styled placeholders; generated `*.logo3d` files are local badges derived from vendored SVGs, not official vendor 3D models.
- For Mermaid architecture diagrams, set `renderHints.badgeMode="icon_and_model"`, `renderHints.badgeSize="medium"`, `renderHints.badgePlacement="front"`, and `renderHints.labelIcon=true` in EFP frontmatter when badge readability matters.
- For graph event inputs, bind each meaningful event to an existing node with `events[].node_id`; replay views should explain which object changed instead of listing detached events.
- Before validation/render, run `visual inspect-input --input <input.mmd> --json` and use `data.warnings`, `data.summary`, and `data.recommendations` to reduce clutter.
- Then run `visual inspect-plan --input <input.mmd> --out <dir> --json` and use `data.visual_plan.ir`, `data.visual_plan.view`, `data.visual_plan.marks`, `data.visual_plan.edges`, `data.visual_plan.colors`, `data.visual_plan.assets`, `data.visual_plan.disclosure`, and `data.visual_plan.quality_loop` to confirm the first view is explainable before render.
- Validate with `visual validate --input <input.mmd> --json`, using `--template-dir` only when the catalog is not installed at `~/.efp/template/visual`.
- Render to a new output directory with `visual render --input <input.mmd> --out <dir> --json`.
- Run `visual inspect-render --out <dir> --json` after render. For browser-level evidence, run `visual inspect-browser --out <dir> --json`; it serves the artifact through local `127.0.0.1`, writes a screenshot, and reuses `inspect-render --screenshot`.
- Return `data.artifact.entrypoint` to the user only after inspection passes, or return the warnings and screenshot path for review.
- Visual effects are template-declared. Do not override them with generated JavaScript; choose the right template and provide better input data.
- Do not use remote assets, CDN URLs, runtime Node/npm, generated JavaScript, or network APIs.
- Use `--dry-run` to preview planned files before writing.
- The generated `index.html` is safe for `file://` and for Portal/runtime static proxy subpaths because asset paths, including the local Three.js module bridge, are relative.

Recommended public templates:

- `mermaid.flowchart` for flowcharts, dependency graphs, process diagrams, and general directed graphs.
- `mermaid.sequence` and `mermaid.zenuml` for message flows.
- `mermaid.class`, `mermaid.er`, `mermaid.state`, `mermaid.requirement`, and `mermaid.c4` for software modeling diagrams.
- `mermaid.architecture` for architecture, topology, deployment, service maps, infrastructure maps, microservices, cloud maps, and iCraft-like isometric scenes.
- `mermaid.gantt`, `mermaid.timeline`, `mermaid.journey`, `mermaid.gitgraph`, `mermaid.pie`, `mermaid.quadrant`, `mermaid.sankey`, `mermaid.xy`, `mermaid.radar`, `mermaid.kanban`, `mermaid.mindmap`, `mermaid.block`, `mermaid.packet`, `mermaid.treemap`, `mermaid.venn`, `mermaid.ishikawa`, `mermaid.wardley`, `mermaid.treeview`, and `mermaid.event_modeling` for the matching Mermaid Diagram Syntax families.

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
- For persistent browser workflows, inspect `browser schema session.start --json`, `browser schema tab.open --json`, and the exact `browser schema page.<command> --json` or `browser schema workflow.run --json` before acting.
- Always use `--json`.
- `browser` is a terminal-invoked CLI binary for Bash, PowerShell, or Windows cmd, not an OpenCode built-in browser tool, MCP tool, or Web UI component.
- Use `browser session start` for multi-step workflows that need a dedicated browser to stay open, then use `browser tab list/current/activate/open` to select a page target.
- Use `browser session discover` and `browser session attach` only when the user explicitly provides local DevTools ports, for example a Chrome launched with `--remote-debugging-port=9222`. They do not inspect arbitrary browsers, default profiles, cookies, or tokens.
- Use `browser page snapshot`, `browser page extract`, and `browser frame snapshot` for redacted page/frame reads.
- Use `browser page extract-schema --file schema.yaml` when the agent needs stable structured JSON fields from selector-declared YAML instead of raw page text.
- Use `browser page find` before actions when CSS selectors are unknown or unstable; prefer returned refs or generated `locators:` in workflows.
- Use `browser page ax` to get accessibility-style refs before ref-based actions; rerun it after navigation or DOM changes.
- Use `browser page outline`, `table`, and `list` when an agent needs navigable page structure or structured data instead of raw text.
- Use `browser page table-export`, `list-export`, and `scroll-collect` when the user asks to collect or export visible page data. Use `browser page diff` to compare before/after JSON page-state captures.
- Use `browser form inspect` to discover form field metadata without current values, then `browser form fill --file values.yaml` to fill fields without echoing values.
- Use `--pierce` on `page extract`, `page outline`, or `page ax` only when open shadow-root traversal is needed; closed shadow roots are not accessible.
- Use `browser page network` for sanitized resource timing/API observation; it returns no headers, cookies, or bodies.
- Use `browser page metrics` for navigation, paint/resource aggregate, DOM node count, long-task count, and bounded largest-resource timing metadata. It is not a trace and returns no headers, cookies, storage, or bodies.
- Use `browser assert visible|text|url|count|screenshot` for JSON-first page state checks. Assertion failures return `ok=false`, `error.code=assertion_failed`, and sanitized details in `data`. Screenshot assertions write actual/diff PNG files and return metadata only.
- Console/network assertions are not separate assertion commands in this pass; use `browser network wait/list` and `browser page console/errors`.
- Use `browser workflow record --out flow.yaml --duration-ms 10000 --json` when the user wants to demonstrate a manual flow and let the agent convert it into a safe workflow skeleton. Typed text and selected option values become empty variables, and fallback locators are included where possible.
- Use `browser workflow run --file flow.yaml --dry-run --json` before executing YAML workflows. Workflows support top-level `vars`, CLI `--var`, `if`, `for_each`, `locators`, `smart_wait`, `human.wait`, `human.confirm`, `--report-out` audit logs, and optional `--evidence-dir` bundles. Workflows call only whitelisted browser actions/assertions and never execute shell commands, arbitrary browser CLI strings, arbitrary JavaScript, `page eval`, or `page fetch`.
- Risky clicks such as submit, delete, pay, save, approve, publish, deploy, or transfer require explicit user confirmation and `--yes`.
- Use `browser network start/list/wait/export/stop/clear` when the user will manually interact and the agent later needs sanitized HAR-lite metadata. It records only after `start`; fetch/XHR response body previews are redacted and returned by default. Network commands never return headers, cookies, storage, or request bodies.
- Use `browser page console` and `browser page errors` for redacted console/runtime diagnostics. They capture events only after recorder injection and do not return object previews.
- Use `browser frame list` before `browser frame snapshot --frame-id <id>` when frame-specific reads are needed. Frame URLs and titles are redacted.
- Use `browser page click`, `type`, `select`, `check`, `uncheck`, `press`, `upload`, `wait`, `screenshot`, `eval`, and `fetch` only as bounded actions against the active or selected tab.
- Prefer `--ref` from `browser page ax` when selectors are unstable. Selector/ref actions return metadata only and do not echo typed text or selected values.
- `browser page wait` can wait for selectors, current URL substrings, visible text, resource timing idle windows, DOM stability windows, or a bounded duration.
- `browser page screenshot` writes a PNG artifact and returns metadata rather than binary image data. Element screenshots require a visible `--selector` or fresh `--ref`; rerun `browser page ax` if a ref is stale.
- `browser page eval` rejects cookie, storage, header, credential, and network APIs; returned values are recursively redacted.
- `browser page fetch` performs GET with credentials omitted, rejects unsafe URL schemes, returns no headers, and redacts the body preview.
- `browser page upload` validates local regular files and returns file metadata only; it never prints file contents.
- `browser download list` and `browser download wait` read only path/name/size/modified metadata from the session download directory.
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

1. Discover commands with `jira commands --json`, `confluence commands --json`, `jenkins commands --json`, `aws-auth commands --json`, `browser commands --json`, `inspect-image commands --json`, or `visual commands --json`.
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
browser schema page.network --json
browser schema page.outline --json
browser schema download.wait --json
jenkins schema job.build-with-params --json
jenkins schema build.status --json
jenkins schema artifact.download --json
jenkins schema api.get --json
inspect-image schema inspect --json
visual schema render --json
visual schema inspect-input --json
visual schema inspect-plan --json
visual schema inspect-render --json
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

## Template-Level Authoring Workflow

For visual generation, use this loop:

1. Write valid Mermaid `.mmd` for the user-visible diagram. Use only official Mermaid syntax in the body.
2. Add optional `efp:` frontmatter when the visual needs a specific template, camera, route, render hint, initial focus, or annotation.
3. `visual inspect-input --input <input.mmd> --json`
4. `visual inspect-plan --input <input.mmd> --out <dir> --json`
5. Revise Mermaid/frontmatter using warning `suggestion`, `auto_fix_hint`, `visual_plan.marks`, `visual_plan.edges`, `visual_plan.colors`, `visual_plan.assets`, and `visual_plan.quality_loop`.
6. `visual render --input <input.mmd> --out <dir> --json`
7. `visual inspect-render --out <dir> --json`. For isometric architecture or visual-quality work, also run `visual inspect-browser --out <dir> --json` to generate a local HTTP browser screenshot and DOM hook report.
8. If `inspect-browser` wrote a screenshot, rerun or verify `visual inspect-render --out <dir> --screenshot <png|jpg|gif> --json`. For isometric architecture, require the artifact hook checks such as `artifact_isometric_dom_hooks`, `artifact_entity_label_hooks`, `artifact_grid_hook`, and `artifact_arrow_hook` to pass.
9. Return `data.artifact.entrypoint` to the user only when inspections report `ready=true`, or return the warnings and screenshot path with the artifact if the user wants to review a draft.
