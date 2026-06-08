# Command Specification

## Common Conventions

- For agent workflows, default every `jira`, `confluence`, `jenkins`, `browser`, `inspect-image`, and `visual` command and subcommand to `--json`.
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

`visual template categories --json` returns category counts plus `canonical_count`, `total_count`, and `alias_count`. In the built-in semantic catalog, `canonical_count=34`, `total_count=34`, and `alias_count=0`.

`visual template list --json` returns canonical templates from `templates/visual/registry.json`. Use `--category`, `--query`, `--renderer`, and `--schema-kind` to narrow discovery before reading template details. The response includes normalized `filters`, `matched_count`, `canonical_count`, `total_count`, and `alias_count`.

`visual template get <template-id> --json` returns template metadata, renderer, layout, schema kind, interactions, limits, tags, `schema_file`, `example_file`, `agent_guide_available`, `agent_guide_path`, `quality_rules_available`, and `quality_rules_path`.

`visual template schema <template-id> --json` returns the template metadata, `visual_design`, full local `json_schema`, example object agents should mirror when writing JSON compatibility input, and guide summary.

Agents must not discover templates by listing `templates/visual` directories or inventing template paths. Use `template categories`, `template list`, `template get`, and `template schema` only.

Semantic categories are `uml`, `relationship`, `temporal`, `flow`, `hierarchy`, `evidence`, `matrix`, `spatial`, and `architecture`. Architecture templates use `isometric_architecture_v1` input and `offline.architecture.isometric.v1` renderer contracts. UML schema kinds are `uml_sequence_v1`, `uml_class_v1`, `uml_state_machine_v1`, `uml_activity_v1`, and `uml_component_deployment_v1`.

`isometric_architecture_v1` input uses `schema=efp.visual.input.isometric_architecture.v1`, required `title`, `zones[]`, `entities[]`, and `links[]`, plus optional `canvas.grid`, `camera`, `theme`, `controls[]`, and shared `visual`, `view`, and `renderHints` guidance. For `architecture.isometric_overview`, generate zones/entities/links rather than generic nodes/edges.

### Input Inspection

`visual inspect-input --input <input.mmd|input.json> --json` validates the input and returns `quality_score`, `summary`, `warnings`, `recommendations`, and the template `visual_design`. Mermaid input can omit `--template`; the CLI infers a template from the Mermaid diagram kind. JSON input requires `--template <template-id>`. Use it after writing Mermaid or JSON and before render. All semantic templates support shared `visual` guidance; summaries include `visual_focus_ids`, `visual_hidden_details`, `visual_narrative_steps`, `visual_annotations`, and visual reference coverage when present. Warnings include `visual_guidance_missing`, `visual_focus_missing`, `visual_annotations_missing`, and `visual_guidance_unknown_refs` so agents can revise first-view focus and annotations before rendering. Graph summaries also include relationship coverage, orphan nodes, dominant edge kinds, long labels, missing importance fields, and missing edge visibility fields. Mark warnings include `generic_sphere_overuse`, `mark_shape_missing`, `provider_service_unknown`, `asset_icon_unknown`, `edge_direction_missing`, `arrow_encoding_missing`, `single_color_detected`, `color_encoding_missing`, `legend_missing`, and `provider_icon_without_attribution`. It does not write files. `visual preview` is a compatibility alias for the same command.

Official Mermaid diagram kinds are accepted. Dedicated mappings are used when available, and unsupported semantic richness falls back to lower-quality generic visualizations rather than failing: architecture/C4 to `architecture.isometric_overview`, sequence/ZenUML to `uml.sequence_3d`, class/ER to `uml.class_structure_2_5d`, state to `uml.state_machine_3d`, timeline/gantt/journey/gitGraph to temporal templates, mindmap/treeView to hierarchy, chart/data block formats to matrix or data-flow templates, and remaining kinds to `relationship.dependency_graph`.

### Visual Plan

`visual inspect-plan --input <input.mmd|input.json> --out <dir> --json` validates the same input and compiles an agent-readable pre-render plan. Mermaid input can omit `--template`; JSON input requires it. The response includes `visual_plan.schema=efp.visual.plan.v1`, normalized `visual_plan.ir` objects/relationships/events, first-view budgets, label buckets, legend hints, disclosure strategy, selection behavior, quality-loop actions, and the exact render command shape. It also includes `visual_plan.marks.shape_counts`, `visual_plan.marks.icon_counts`, `visual_plan.marks.fallback_sphere_count`, `visual_plan.edges.directed_count`, `visual_plan.edges.arrow_count`, `visual_plan.edges.undirected_count`, `visual_plan.colors.colorBy`, `visual_plan.colors.legend_items`, `visual_plan.colors.single_color`, `visual_plan.assets.icons_used`, `visual_plan.assets.missing_icons`, and `visual_plan.assets.attributions`. It does not inspect pixels or screenshots; use it to revise dense or unclear input before rendering.

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
