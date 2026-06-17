# Command Specification

## Common Conventions

- For agent workflows, default every `jira`, `confluence`, `jenkins`, `aws-auth`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json`.
- `aws-auth login` invokes `adfs-assume` with `--profile saml` by default.
- `--json` returns the stable `ok/data/error` envelope.
- Command parsing failures return `ok=false` with `error.code=invalid_args` when `--json` is present.
- `--format table|json|yaml` selects output rendering where supported.
- `--verbose` writes non-secret diagnostics.
- Destructive commands require `--yes`.
- Write commands support `--dry-run` unless explicitly documented otherwise.
- Windows `cmd` agents should use double quotes, `where <binary>`, `dir`, `cd`, and `type` rather than Bash-only commands or single-quote quoting.

## Visual

### Basic
- visual template categories
- visual template list
- visual template list --category <category>
- visual template list --query <text>
- visual template list --renderer <renderer-contract>
- visual template list --schema-kind <input-schema-kind>
- visual template get <template-id>
- visual template schema <template-id>
- visual template guide <template-id>
- visual template doctor
- visual validate
- visual inspect-input
- visual inspect-plan
- visual inspect-render
- visual inspect-browser
- visual render
- visual inspect-output
- visual commands
- visual schema <command>
- visual help llm
- visual version

### Template Discovery

`visual template categories --json` returns category counts plus `canonical_count`, `total_count`, and `alias_count`. In the built-in public Mermaid catalog, `canonical_count=28` and `categories=[{id:"mermaid", count:28}]`.

`visual template list --json` returns canonical templates from `templates/visual/registry.json`. Use `--category`, `--query`, `--renderer`, and `--schema-kind` to narrow discovery before reading template details. The response includes normalized `filters`, `matched_count`, `canonical_count`, `total_count`, and `alias_count`.

`visual template get <template-id> --json` returns template metadata, renderer, layout, schema kind, interactions, limits, tags, `schema_file`, `example_file`, `agent_guide_available`, `agent_guide_path`, `quality_rules_available`, and `quality_rules_path`.

`visual template schema <template-id> --json` returns template metadata, `visual_design`, the Mermaid input format, `mermaid_syntax`, a `.mmd` example, the internal compiled schema, and guide summary.

Agents must not discover templates by listing `templates/visual` directories or inventing template paths. Use `template categories`, `template list`, `template get`, and `template schema` only.

The public category is `mermaid`. Internal schema kinds such as `graph_v1`, `uml_sequence_v1`, and `isometric_architecture_v1` are compiled targets and are not authored directly by users.

`isometric_architecture_v1` is an internal compiled target for Mermaid architecture/C4 diagrams. Users should author Mermaid and optional EFP frontmatter, not direct renderer IR.

### Input Inspection

`visual inspect-input --input <input.mmd> --json` validates Mermaid input and returns `quality_score`, `summary`, `warnings`, `recommendations`, and the template `visual_design`. Mermaid input can omit `--template`; the CLI infers a template from the Mermaid diagram kind. Public templates reject non-Mermaid input with `mermaid_input_required`. It does not write files. `visual preview` is a compatibility alias for the same command.

Official Mermaid diagram kinds are accepted and mapped to matching public templates: `mermaid.flowchart`, `mermaid.sequence`, `mermaid.class`, `mermaid.state`, `mermaid.er`, `mermaid.journey`, `mermaid.gantt`, `mermaid.pie`, `mermaid.quadrant`, `mermaid.requirement`, `mermaid.gitgraph`, `mermaid.c4`, `mermaid.mindmap`, `mermaid.timeline`, `mermaid.zenuml`, `mermaid.sankey`, `mermaid.xy`, `mermaid.block`, `mermaid.packet`, `mermaid.kanban`, `mermaid.architecture`, `mermaid.radar`, `mermaid.event_modeling`, `mermaid.treemap`, `mermaid.venn`, `mermaid.ishikawa`, `mermaid.wardley`, and `mermaid.treeview`.

### Visual Plan

`visual inspect-plan --input <input.mmd> --out <dir> --json` validates the same Mermaid input and compiles an agent-readable pre-render plan. The response includes `visual_plan.schema=efp.visual.plan.v1`, normalized `visual_plan.ir` objects/relationships/events, first-view budgets, label buckets, legend hints, disclosure strategy, selection behavior, quality-loop actions, and the exact render command shape.

For `isometric_architecture_v1`, `inspect-plan` also returns `visual_plan.isometric` with base plane, grid, zone/entity/link counts, positioned and auto-positioned entity counts, directed and arrow link counts, top labels, leader lines, boundary style counts, camera, and theme.

### Render Inspection

`visual inspect-render --out <dir> --json` reads a generated artifact, validates required files and offline safety, loads `manifest.json` and `data.js`, rebuilds the normalized visual plan, and returns `ready`, `render_score`, `checks`, `warnings`, `visual_plan`, and `next_actions`. Checks include `shape_diversity`, `arrows_visible`, `color_diversity`, `legend_present`, `icon_assets_present`, and `attributions_present` in addition to output, offline, runtime, plan, label, relationship, architecture isometric, artifact hook, and screenshot checks. Architecture checks include `isometric_renderer_used`, `base_plane_present`, `grid_present`, `zones_present`, `zone_boundaries_present`, `entities_present`, `entity_labels_present`, `leader_lines_present`, `directed_arrows_present`, `link_labels_present`, `orthographic_camera_planned`, `architecture_light_theme`, `no_starfield_theme`, and `no_studio_layout`. Artifact hook checks include `artifact_runtime_wired`, `artifact_isometric_runtime_hook`, `artifact_isometric_dom_hooks`, `artifact_entity_label_hooks`, `artifact_link_label_hooks`, `artifact_zone_label_hooks`, `artifact_base_plane_hook`, `artifact_grid_hook`, `artifact_leader_line_hook`, `artifact_arrow_hook`, `artifact_no_studio_runtime`, and `artifact_no_starfield_runtime`; these inspect generated `index.html`, runtime JS/CSS, `manifest.js`, and `data.js` instead of trusting only the plan. Add `--screenshot <png|jpg|gif>` when a browser screenshot is available; the command then also checks blankness, contrast, and visible content coverage with standard-library image decoding.

`visual inspect-browser --out <dir> --json` opens a rendered artifact through a temporary local HTTP server and captures browser evidence. It serves `<dir>/index.html` at `http://127.0.0.1:<port>/index.html`, launches local Chrome/Chromium through a Node.js CDP helper, waits for runtime data and renderer DOM hooks, writes `--screenshot` or `<dir>/visual-screenshot.png`, and then reuses `inspect-render --screenshot`. The response includes `server_url`, `screenshot_path`, `browser_ready`, `render_ready`, `render_score`, `visual_checks`, `visual_summary`, `dom`, `requests`, `warnings`, and `ready`. `visual_summary` reports the screenshot path, total and visible entity label/icon counts, loaded and broken label icon counts, model badge/SVG billboard/fallback counts, canvas/control visibility, approximate entity-label overlap count, bounds, and screenshot size. Checks include page/runtime load, renderer mount, screenshot write, console/network/remote request absence, isometric stage and label layer presence, entity/link/zone label hooks, label icons, model badges, SVG billboards, control bar, canvas visibility, screenshot nonblank/contrast, and expected label count.

`inspect-browser` does not use `file://`, does not make remote requests, and does not perform OCR or logo semantic recognition. It requires Chrome/Chromium and Node.js. Missing runtime returns `browser_runtime_missing`; smoke scripts may set `EFP_SKIP_BROWSER_SMOKE=1` only when that browser runtime is intentionally unavailable.

### Render Artifact Output

`visual render --json` returns `data.artifact` with these compatibility fields:

- `template_id`
- `template_version`
- `title`
- `out_dir`
- `out`
- `entrypoint`
- `relative_entrypoint`
- `offline`
- `file_url_safe`
- `http_subpath_safe`
- `files`

Rendered artifacts also copy the shared Visual Mark System into the output. `manifest.json` includes `assets.icons`, `assets.models`, `assets.attributions`, embedded `assets.mark_registry`, and embedded `assets.asset_registry`; output files include `assets/mark-registry.json`, `assets/asset-registry.json`, `assets/ATTRIBUTIONS.md`, `assets/icons/**`, and `assets/models/**`.

## Jira

### Basic
- jira instance list
- jira instance get <name>
- jira instance add <name>
- jira instance update <name>
- jira instance remove <name>
- jira instance default [name]
- jira auth login
- jira auth logout
- jira auth test
- jira myself
- jira server-info
- jira resolve-url <url>
- jira commands
- jira schema <command>
- jira help llm
- jira version

### Issue
- jira issue get <issue-or-url>
- jira issue search
- jira issue create
- jira issue update <issue-or-url>
- jira issue edit <issue-or-url>
- jira issue delete <issue-or-url>
- jira issue assign <issue-or-url>
- jira issue transitions <issue-or-url>
- jira issue transition <issue-or-url>
- jira issue changelog <issue-or-url>
- jira issue fields <issue-or-url>
- jira issue createmeta
- jira issue editmeta <issue-or-url>
- jira issue map-csv
- jira issue bulk-create
- jira issue bulk-validate
- jira issue watchers <issue-or-url>
- jira issue watch <issue-or-url>
- jira issue unwatch <issue-or-url>
- jira issue votes <issue-or-url>
- jira issue vote <issue-or-url>
- jira issue unvote <issue-or-url>
- jira issue notify <issue-or-url>

### Comment
- jira issue comment list <issue-or-url>
- jira issue comment get <issue-or-url> <comment-id>
- jira issue comment add <issue-or-url>
- jira issue comment update <issue-or-url> <comment-id>
- jira issue comment delete <issue-or-url> <comment-id>

### Zephyr
- jira zephyr doctor
- jira zephyr resolve-url <jira-url>
- jira zephyr status list
- jira zephyr util test-issue-type
- jira zephyr summary
- jira zephyr test list
- jira zephyr test get <issue-or-url>
- jira zephyr test create
- jira zephyr version list
- jira zephyr version resolve
- jira zephyr cycle list
- jira zephyr cycle resolve
- jira zephyr cycle get <cycle-id>
- jira zephyr cycle create
- jira zephyr cycle update <cycle-id>
- jira zephyr cycle delete <cycle-id>
- jira zephyr execution list
- jira zephyr execution resolve
- jira zephyr execution get <execution-id>
- jira zephyr execution create
- jira zephyr execution update-status [execution-id]
- jira zephyr execution add-tests-to-cycle
- jira zephyr execution count
- jira zephyr execution delete <execution-id>
- jira zephyr execution bulk-update-status
- jira zephyr execution export
- jira zephyr archive list
- jira zephyr archive executions
- jira zephyr archive restore
- jira zephyr archive export
- jira zephyr zql search
- jira zephyr zql clauses
- jira zephyr zql autocomplete-json
- jira zephyr zql autocomplete
- jira zephyr step-result list
- jira zephyr step-result update-status <step-result-id>
- jira zephyr attachment list
- jira zephyr attachment get <attachment-id>
- jira zephyr attachment upload
- jira zephyr attachment delete <attachment-id>
- jira zephyr folder list
- jira zephyr folder create
- jira zephyr folder update <folder-id>
- jira zephyr folder delete <folder-id>
- jira zephyr teststep list
- jira zephyr teststep get
- jira zephyr teststep create
- jira zephyr teststep update
- jira zephyr teststep delete
- jira zephyr defect list
- jira zephyr defect add
- jira zephyr customfield list
- jira zephyr customfield get <customfield-id>
- jira zephyr customfield create
- jira zephyr customfield update <customfield-id>
- jira zephyr customfield delete <customfield-id>
- jira zephyr customfield delete-bulk
- jira zephyr customfield enable <customfield-id>
- jira zephyr report coverage
- jira zephyr api catalog
- jira zephyr api describe <endpoint-id>
- jira zephyr api get <path>
- jira zephyr api post <path>
- jira zephyr api put <path>
- jira zephyr api delete <path>

### Attachment
- jira issue attachment list <issue-or-url>
- jira issue attachment upload <issue-or-url> <file>
- jira attachment get <attachment-id>
- jira attachment download <attachment-id>
- jira attachment delete <attachment-id>
- jira attachment meta

### Worklog
- jira issue worklog list <issue-or-url>
- jira issue worklog get <issue-or-url> <worklog-id>
- jira issue worklog add <issue-or-url>
- jira issue worklog update <issue-or-url> <worklog-id>
- jira issue worklog delete <issue-or-url> <worklog-id>

### Issue link / remote link / property
- jira issue link list <issue-or-url>
- jira issue link create
- jira issue link delete <link-id>
- jira issue remote-link list <issue-or-url>
- jira issue remote-link add <issue-or-url>
- jira issue remote-link delete <issue-or-url> <link-id>
- jira issue property list <issue-or-url>
- jira issue property get <issue-or-url> <key>
- jira issue property set <issue-or-url> <key>
- jira issue property delete <issue-or-url> <key>

### Project / component / version
- jira project list
- jira project get <project-key>
- jira project statuses <project-key>
- jira project roles <project-key>
- jira project role get <project-key> <role-id-or-name>
- jira project components <project-key>
- jira component get <component-id>
- jira component create
- jira component update <component-id>
- jira component delete <component-id>
- jira project versions <project-key>
- jira version get <version-id>
- jira version create
- jira version update <version-id>
- jira version delete <version-id>

### User / group
- jira user get
- jira user search
- jira user assignable
- jira group get <group-name>
- jira group members <group-name>
- jira group search

### Metadata / workflow / admin-read
- jira field list
- jira issue-type list
- jira status list
- jira priority list
- jira resolution list
- jira workflow list
- jira workflow get <name>
- jira permissions myself
- jira settings get
- jira config get

### Filter / dashboard
- jira filter list
- jira filter get <filter-id>
- jira filter search
- jira filter create
- jira filter update <filter-id>
- jira filter delete <filter-id>
- jira dashboard list
- jira dashboard get <dashboard-id>

### Raw API
- jira api get <path>
- jira api post <path>
- jira api put <path>
- jira api delete <path>

### Agile extension
- jira board list
- jira board get <board-id>
- jira sprint list <board-id>
- jira sprint get <sprint-id>
- jira sprint issues <sprint-id>
- jira backlog issues <board-id>

## Confluence

### Basic
- confluence instance list
- confluence instance get <name>
- confluence instance add <name>
- confluence instance update <name>
- confluence instance remove <name>
- confluence instance default [name]
- confluence auth login
- confluence auth logout
- confluence auth test
- confluence myself
- confluence server-info
- confluence resolve-url <url>
- confluence commands
- confluence schema <command>
- confluence help llm
- confluence version

### Search / CQL
- confluence search
- confluence cql
- confluence search content
- confluence search user

### Space
- confluence space list
- confluence space get <space-key>
- confluence space create
- confluence space update <space-key>
- confluence space delete <space-key>
- confluence space content <space-key>
- confluence space pages <space-key>
- confluence space blogs <space-key>
- confluence space labels <space-key>
- confluence space watchers <space-key>
- confluence space permission list <space-key>
- confluence space property list <space-key>
- confluence space property get <space-key> <key>
- confluence space property set <space-key> <key>
- confluence space property delete <space-key> <key>

### Page / content
- confluence page get
- confluence page get-by-title
- confluence page create
- confluence page update
- confluence page delete
- confluence page move
- confluence page children
- confluence page descendants
- confluence page ancestors
- confluence page body
- confluence page body-storage
- confluence page body-view
- confluence page version
- confluence page history
- confluence page restore
- confluence page export-html
- confluence page export-markdown

Literal page-get forms:

```text
confluence page get --id <page-id>
confluence page get --url <page-url>
```

### Generic content
- confluence content get <content-id>
- confluence content list
- confluence content create
- confluence content update <content-id>
- confluence content delete <content-id>

### Blog
- confluence blog list
- confluence blog get <blog-id-or-url>
- confluence blog create
- confluence blog update <blog-id-or-url>
- confluence blog delete <blog-id-or-url>

### Attachment
- confluence page attachment list
- confluence page attachment upload
- confluence page attachment update
- confluence attachment get <attachment-id>
- confluence attachment download <attachment-id>
- confluence attachment delete <attachment-id>

### Comment
- confluence page comment list
- confluence page comment add
- confluence comment get <comment-id>
- confluence comment update <comment-id>
- confluence comment delete <comment-id>

### Label / property
- confluence page label list
- confluence page label add
- confluence page label delete
- confluence label list
- confluence page property list
- confluence page property get
- confluence page property set
- confluence page property delete

### Restrictions / watchers
- confluence page restriction list
- confluence page restriction add
- confluence page restriction delete
- confluence page watcher list
- confluence page watch
- confluence page unwatch

### User / group
- confluence user get
- confluence user search
- confluence group list
- confluence group get <group-name>
- confluence group members <group-name>

### Long task / webhook / raw API
- confluence longtask list
- confluence longtask get <task-id>
- confluence webhook list
- confluence webhook get <webhook-id>
- confluence webhook create
- confluence webhook delete <webhook-id>
- confluence api get <path>
- confluence api post <path>
- confluence api put <path>
- confluence api delete <path>

## Jenkins

### Basic
- jenkins instance list
- jenkins instance get <name>
- jenkins instance add <name>
- jenkins instance update <name>
- jenkins instance remove <name>
- jenkins instance default [name]
- jenkins auth login
- jenkins auth logout
- jenkins auth test
- jenkins whoami
- jenkins server-info
- jenkins crumb get
- jenkins commands
- jenkins schema <command>
- jenkins help llm
- jenkins version

### Job
- jenkins job list
- jenkins job get <job>
- jenkins job config get <job>
- jenkins job config update <job>
- jenkins job create <job>
- jenkins job copy <source> <target>
- jenkins job delete <job>
- jenkins job enable <job>
- jenkins job disable <job>
- jenkins job build <job>
- jenkins job build-with-params <job>

### Queue
- jenkins queue list
- jenkins queue get <queue-id>
- jenkins queue cancel <queue-id>

### Build
- jenkins build get <job> <build>
- jenkins build status <job> <build>
- jenkins build log <job> <build>
- jenkins build log-follow <job> <build>
- jenkins build stop <job> <build>
- jenkins build artifacts <job> <build>

### Artifact
- jenkins artifact download <job> <build> <path>

### Pipeline REST API
- jenkins pipeline runs <job>
- jenkins pipeline run <job> <run-id>
- jenkins pipeline stages <job> <run-id>
- jenkins pipeline node-log <job> <run-id> <node-id>
- jenkins pipeline artifacts <job> <run-id>

### View
- jenkins view list
- jenkins view get <view>
- jenkins view create <view>
- jenkins view delete <view>
- jenkins view config get <view>
- jenkins view config update <view>

### Node / plugin
- jenkins node list
- jenkins node get <node>
- jenkins plugin list
- jenkins plugin get <plugin>

### System / raw API
- jenkins system quiet-down
- jenkins system cancel-quiet-down
- jenkins system safe-restart
- jenkins api get <path>
- jenkins api post <path>
- jenkins api put <path>
- jenkins api delete <path>

## Browser

### Basic
- browser probe
- browser session start
- browser session list
- browser session status [name]
- browser session attach
- browser session discover
- browser session stop [name]
- browser tab list
- browser tab current
- browser tab activate
- browser tab open
- browser page snapshot
- browser page extract
- browser page extract-schema
- browser page find
- browser page ax
- browser page click
- browser page type
- browser page select
- browser page check
- browser page uncheck
- browser page press
- browser page upload
- browser page wait
- browser page screenshot
- browser page eval
- browser page fetch
- browser page console
- browser page errors
- browser page console-clear
- browser page network
- browser page metrics
- browser page outline
- browser page table
- browser page table-export
- browser page list
- browser page list-export
- browser page scroll-collect
- browser page diff
- browser assert visible
- browser assert text
- browser assert url
- browser assert count
- browser assert screenshot
- browser workflow run
- browser workflow record
- browser form inspect
- browser form fill
- browser frame list
- browser frame snapshot
- browser network start
- browser network stop
- browser network list
- browser network wait
- browser network export
- browser network clear
- browser download list
- browser download wait
- browser commands
- browser schema <command>
- browser help llm
- browser version

### Persistent Workflow

Start a dedicated browser session with Chrome by default and DevTools bound to `127.0.0.1`, then select a tab and run page commands against the active target. Use `--browser edge`, `--browser chromium`, or `--browser auto` to override. Managed sessions attempt to detach the browser process from the short-lived CLI or agent command process so later agent turns can reuse the same endpoint:

```bash
browser session start --name default --url https://intranet.example.test --json
browser session discover --ports 9222,9223 --json
browser session attach --name user-demo --debug-port 9222 --json
browser tab list --session default --json
browser tab activate --session default --target-id <target-id> --json
browser page snapshot --session default --json
browser page extract --session default --selector .user-avatar --json
browser page extract-schema --session default --file schema.yaml --json
browser page find --session default --role button --name Save --json
browser page ax --session default --json
browser page outline --session default --json
browser page network --session default --filter /api/ --json
browser page metrics --session default --limit-resources 10 --json
browser assert visible --session default --selector .ready --json
browser assert screenshot --session default --baseline baseline.png --out actual.png --diff-out diff.png --json
browser page table-export --session default --selector table.results --out result/table.csv --format csv --json
browser page scroll-collect --session default --item-selector .row --out result/items.json --json
browser page diff --before before.json --after after.json --json
browser workflow run --file flow.yaml --dry-run --evidence-dir result/evidence --json
browser workflow record --session default --out flow.yaml --duration-ms 10000 --json
browser form inspect --session default --json
browser form fill --session default --file values.yaml --json
browser network start --session default --limit 500 --json
browser network list --session default --filter /api/ --json
browser network export --session default --out result/network.har-lite.json --format har-lite --json
```

### Page Actions

- `browser page ax` returns a bounded DOM/ARIA accessibility-style tree with stable short-session refs. It redacts names, descriptions, titles, frame URLs/titles, and selector hints, and stores a sanitized ref artifact for later `--ref` actions.
- `browser page click --selector <css>|--ref <ref>` clicks a visible element.
- `browser page type --selector <css>|--ref <ref> --text <text> [--clear]` types text without echoing the text in output.
- `browser page select --selector <css>|--ref <ref> (--value <value>|--label <label>|--index <n>)` selects an option and returns only selection mode/count metadata.
- `browser page check|uncheck --selector <css>|--ref <ref>` sets checkbox-like elements to checked or unchecked.
- `browser page press --key <key> [--selector <css>|--ref <ref>]` presses a key, optionally focusing a target first.
- `browser page upload --selector <css> --file <path>` attaches local regular files to an input[type=file] and returns file metadata only.
- `browser page wait --selector <css>`, `--duration-ms <n>`, `--url-contains <text>`, `--text <text>`, `--network-idle-ms <n>`, or `--dom-stable-ms <n>` waits within the command timeout.
- `browser page screenshot --out <file> [--selector <css>|--ref <ref>]` writes a page or visible-element PNG artifact and returns path/size metadata. Element screenshots require a visible selector/ref; stale refs require rerunning `browser page ax`.
- `browser page extract-schema --file <schema.yaml>` reads selector-declared fields from YAML and returns stable redacted JSON field values.
- `browser page find` locates elements by role, name, text, label, placeholder, nearby text, or selector, returning refs and fallback locator candidates.
- `browser page table-export`, `list-export`, and `scroll-collect` write redacted data collection artifacts as JSON or CSV.
- `browser page diff` compares two browser JSON envelopes or page-state JSON files and returns redacted changed paths.
- `browser page eval --expr <js>` rejects cookie, storage, header, credential, and network APIs, then redacts returned values.
- `browser page fetch --url <url-or-path>` runs a GET fetch with credentials omitted, rejects unsafe URL schemes, returns no headers, and redacts the body preview.
- `browser page console`, `browser page errors`, and `browser page console-clear` use a bounded page-side recorder for console API calls and runtime errors; messages, URLs, and stacks are redacted/truncated and object previews are not returned.
- `browser page network [--filter <text>] [--all]` returns resource timing summaries with redacted URLs and no headers or bodies.
- `browser page metrics [--limit-resources <n>] [--filter <text>]` returns browser timing metadata only: navigation, paint/resource aggregates, DOM node count, long-task count, and redacted largest resource URLs.
- `browser assert visible|text|url|count|screenshot` returns JSON-first assertion pass/fail metadata. Assertion failures use `ok=false` and `error.code=assertion_failed`; failure envelopes also include `data` with sanitized assertion details. Screenshot assertions write actual/diff PNG artifacts and return metadata only.
- Risky clicks such as submit, delete, pay, save, approve, publish, deploy, or transfer require explicit `--yes`.
- Dedicated console/network assertion commands are not included in this pass; use `browser network wait/list` and `browser page console/errors` for those checks.
- `browser workflow record --out flow.yaml --duration-ms <n>` records a bounded manual browser demonstration into a sanitized workflow skeleton. Typed text and selected values are replaced by empty variables.
- `browser workflow run --file flow.yaml [--dry-run]` parses and runs YAML workflows made only of whitelisted browser actions/assertions. Workflows support variables, CLI `--var`, conditions, `for_each`, locator fallback via `locators:`, `smart_wait`, `human.wait`, `human.confirm`, `--report-out` audit logs, and optional `--evidence-dir` bundles. It does not execute shell commands, arbitrary browser CLI strings, arbitrary JavaScript, `page eval`, or `page fetch`.
- `browser form inspect` returns form labels, names, types, selector hints, and option metadata without current values. `browser form fill --file values.yaml` fills fields from YAML and returns match metadata and value byte counts only.
- `browser session discover` and `browser session attach` operate only on explicitly supplied `127.0.0.1` DevTools ports; they do not inspect default browser profiles or export cookies.
- `browser network start|stop|list|wait|export|clear` records or exports sanitized HAR-lite metadata after `start` via a bounded page-side fetch/XHR/resource recorder. Fetch/XHR response body previews are redacted and returned by default; headers, cookies, storage, and request bodies are never returned. `network export` writes JSON/HAR-lite metadata and redacted response content previews when captured.
- `browser page extract`, `browser page outline`, and `browser page ax` accept `--pierce` to traverse open shadow roots. Closed shadow roots are not accessible.
- `browser frame list` returns the DevTools frame tree with redacted frame URLs and names.
- `browser frame snapshot --frame-id <id>` snapshots one frame through DevTools with redacted URL, title, text, and optional HTML preview.
- `browser page outline` returns a DOM-derived page outline with redacted names, labels, text, hrefs, roles, and selector hints.
- `browser page table` and `browser page list` return structured table/list data that is easier to consume than generic extraction.
- `browser download list` and `browser download wait` inspect completed files in the session download directory and return file metadata only.

### Common Browser Flags

- `--session <name>`: browser automation session name.
- `--target-id <id>`: optional DevTools page target id; defaults to the active tab.
- `--timeout <seconds>`: maximum seconds for page commands.
- `--download-dir <dir>`: dedicated session download directory for `browser session start`.
- `--json`: return the stable JSON envelope.

## Inspect Image

### Basic
- inspect-image inspect
- inspect-image auth login
- inspect-image auth status
- inspect-image auth test
- inspect-image auth logout
- inspect-image doctor
- inspect-image models
- inspect-image commands
- inspect-image schema
- inspect-image help llm
- inspect-image version

### Inspect flags
- `--image <path>`: exactly one local JPEG, PNG, WEBP, or GIF regular file.
- `--prompt <text>` or `--prompt-file <path>`: required task text.
- `--model <model>`: `gpt-5.4`, `gpt-5-mini`, or `gpt-5.4-mini`.
- `--reasoning <effort>`: `low`, `medium`, `high`, or `xhigh`.
- `--preset <preset>`: `general`, `ocr`, `ui`, `diagram`, `chart`, or `error`.
- `--out <file>`: write the full JSON envelope to a file in addition to stdout. Use this when Windows terminal stdout capture is unreliable.
- `--verbose`: write non-secret diagnostics to stderr for config load, image validation, auth checks, `/responses` request/response, output file writes, and envelope status.

Windows `cmd` agents should use double quotes and cmd-native commands:

```cmd
inspect-image.exe inspect --image "%CD%\screenshot.png" --prompt "Read the visible error" --out "%CD%\inspect-image-result.json" --json
```

Read `%CD%\inspect-image-result.json` with the file-read tool if stdout capture is unreliable. Use `type "%CD%\inspect-image-result.json"` only when no file-read tool is available.

Optional future/P1:

- inspect-image prepare


## Contract Notes

- `commands --json` returns command metadata objects.
- `schema <command> --json` returns usage, risk, arguments, flags, examples, and required fields.
- Destructive commands require `--yes`.
- Write commands support `--dry-run`.

### visual template guide

`visual template guide <template-id> --json` returns the selected template's agent authoring guide.

The JSON `data` object includes:

- `template_id`
- `requested_id`
- `canonical_id`
- `guide_path`
- `agent_guide_available`
- `raw_markdown`
- `guide` parsed by section
- `guide_summary`
- `missing_guide_sections`

If a guide is missing, the command returns `ok=true` with `agent_guide_available=false` and a warning string so agents can fall back to schema and shared guidance without crashing.

`visual inspect-input` warnings include `code`, `severity`, `path`, `message`, `suggestion`, `auto_fix_hint`, and optional `details`.

`visual inspect-plan` returns `ready`, `quality_score`, `visual_plan.schema=efp.visual.plan.v1`, normalized `visual_plan.ir`, `visual_plan.view`, `visual_plan.labels`, `visual_plan.legend`, `visual_plan.marks`, `visual_plan.edges`, `visual_plan.colors`, `visual_plan.assets`, `visual_plan.disclosure`, `visual_plan.quality_loop`, and `visual_plan.agent_next_actions` so agents can revise semantic input before rendering.
