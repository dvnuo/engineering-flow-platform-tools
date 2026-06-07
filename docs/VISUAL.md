# Visual Offline Artifacts

`visual` is a terminal-invoked Go CLI for generating complete offline static visualization artifacts. It reads local templates from `~/.efp/template/visual` by default, validates input JSON, copies local assets, and writes a self-contained site to `--out`.

The built-in catalog is now a semantic catalog: 34 canonical templates across 9 categories. It intentionally does not keep legacy aliases or duplicate legacy directories. Discover templates through the CLI, not by guessing file paths.

## Template Directory

Template directory resolution order:

1. `--template-dir`
2. `EFP_VISUAL_TEMPLATE_DIR`
3. `visual.template_dir` from `--config`, then `EFP_CONFIG`, then `~/.efp/config.yaml`
4. `~/.efp/template/visual`
5. `./templates/visual`
6. executable-adjacent release paths

The directory must contain `registry.json`, `_shared/**`, and one direct directory per canonical template. Every direct directory except `_shared` must be registered.

## Categories

- `uml`: UML sequence, class, state machine, activity, and component/deployment diagrams.
- `relationship`: dependency, topology, lineage, and issue relationship maps.
- `temporal`: timelines, event traces, automation replay, and release history.
- `flow`: pipeline, approval, data flow, and customer journey views.
- `hierarchy`: layered architecture, repository trees, ownership, and containment.
- `evidence`: claim/source boards, root-cause evidence, risk decisions, and documentation freshness.
- `matrix`: capability, KPI, risk, and resource allocation matrices.
- `spatial`: service cities, codebase galaxies, agent fleets, and control-room spaces.
- `architecture`: isometric architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, and iCraft-like scenes.

## Renderer Contracts

Supported renderer contracts:

- `offline.graph.v1`
- `offline.timeline.v1`
- `offline.evidence.v1`
- `offline.matrix.v1`
- `offline.architecture.isometric.v1`
- `offline.uml.sequence.3d.v1`
- `offline.uml.class.2_5d.v1`
- `offline.uml.state.3d.v1`
- `offline.uml.activity.3d.v1`
- `offline.uml.component.3d.v1`

## Input Schema Kinds

Reusable visual schema kinds:

- `graph_v1`
- `graph_events_v1`
- `timeline_v1`
- `evidence_v1`
- `matrix_v1`
- `isometric_architecture_v1`

UML semantic schema kinds:

- `uml_sequence_v1`
- `uml_class_v1`
- `uml_state_machine_v1`
- `uml_activity_v1`
- `uml_component_deployment_v1`

UML templates are not just graph templates with looser names. For example, `uml.sequence_3d` requires `participants`, ordered `messages`, optional `phases`, `activations`, and `fragments`. The runtime can then draw 3D lifelines, directional arrows, labels, and replay/phase controls.

Architecture templates use `isometric_architecture_v1` with `schema=efp.visual.input.isometric_architecture.v1`. The input organizes an isometric architecture scene around `zones`, `entities`, `links`, optional `canvas.grid`, `camera`, `theme`, `controls`, `view`, `renderHints`, and `visual`. `architecture.isometric_overview` requires zones/entities/links and must not be authored as generic nodes/edges.

## Agent Workflow

```bash
visual template categories --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --category uml --json
visual template get uml.sequence_3d --template-dir ./templates/visual --json
visual template schema uml.sequence_3d --template-dir ./templates/visual --json
visual template guide uml.sequence_3d --template-dir ./templates/visual --json
visual inspect-input --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --json
visual inspect-plan --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/game-session-flow.input.json --out ./out/sequence --json
visual render --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/game-session-flow.input.json --out ./out/sequence --title "Checkout Sequence" --json
visual inspect-render --template-dir ./templates/visual --out ./out/sequence --json
visual inspect-browser --template-dir ./templates/visual --out ./out/sequence --json
```

Agents must read `visual template schema <id> --json` before writing input JSON. Do not invent JSON shape. Do not infer templates from directories.

Every semantic input schema includes a shared `visual` object. Use it to tell the renderer how to make the first view readable instead of asking the agent to generate JavaScript:

- `visual.goal`: what the viewer should understand first
- `visual.initial_focus_ids`: semantic ids emphasized in the first view
- `visual.hidden_detail_ids`: secondary detail that can stay collapsed, muted, or hidden until search/focus
- `visual.narrative_steps`: progressive explanation beats with `focus_ids`
- `visual.annotations`: callouts attached to existing `target_id` values

Graph, timeline, matrix, evidence, UML, and architecture renderers use this guidance for focus styling, delayed detail, labels, and annotations.

## Isometric Architecture Layer

The isometric architecture layer is a scene contract over the semantic Diagram/Mark layer. It does not replace graph, UML, timeline, evidence, matrix, or mark authoring. Instead, it maps systems into a grounded architecture scene:

- zones define bounded infrastructure, runtime, data, network, or platform areas
- entities define services, APIs, workers, queues, streams, databases, storage, users, gateways, ingress/load balancers, Kubernetes objects, and external systems
- links define directed calls, reads, writes, events, dependencies, deploys, validates, blocks, sends, and returns
- canvas.grid defines the base plane and ground reference
- camera defaults to orthographic isometric
- theme defaults to architecture_light and should not be starfield
- top labels and leader lines keep entities readable in the first view

Use `architecture.isometric_overview` when the user asks for architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, iCraft-like, or isometric architecture. Use the underlying diagram categories when the user only needs a focused non-isometric visual.

## Visual Design Guidance

Each template declares `visual_design` in `template.yaml`. The schema command returns this guidance so agents can create readable inputs:

- `initial_view`: first screen intent, usually `overview`
- `max_initial_nodes` and `max_initial_edges`: first-view budget
- `default_collapse_depth`: whether grouped graph data starts collapsed
- `group_by`: preferred grouping keys
- `supports`: expected interactions
- `agent_guidance`: template-specific input authoring rules

For large graph-like inputs, use short display labels, put full names and paths in `metadata`, add groups or `parent_id`/`group_id`/`group`, keep overview relationships visible, and mark noisy detail edges with `visibility: "detail"` or `visibility: "hidden"`.

For UML sequence inputs, provide participants as semantic lifelines, use unique numeric `messages[].order`, add concise message labels, define phases when a flow has stages, and use fragments for `alt`, `loop`, `opt`, or `par` regions. For a stronger 3D sequence scene, also provide participant `display_name`, `subtitle`, `lane_index`, `depth`, and `color`; provide message `curve`, `importance`, `label_priority`, `depth`, and `summary`; then use `visual.initial_focus_ids` and `visual.annotations` to explain the high-value paths.

## Visual Mark System

Graph-like, matrix, UML component/activity/state, and sequence renderers use the shared Visual Mark System. The agent-facing grammar lives in `templates/visual/_shared/agent-guidance/mark-grammar.md`; runtime defaults live in `_shared/mark-registry.json`; local icon/model metadata lives in `_shared/asset-registry.json`.

Object marks are resolved from `presentation.mesh` or `presentation.shape`, then `presentation.model`, then `presentation.icon`, then `provider + service`, `platform`, `kind`, `group`, and finally fallback. Use these fields so a service can render as a box, an API as a hex service, a database or storage object as a cylinder, a queue or stream as a capsule, a user as an actor card, an external system as a cloud plate, and a decision or risk as a diamond or warning prism. `presentation.model` references local generated GLB badge IDs such as `nginx.logo3d`, `redis.logo3d`, `mysql.logo3d`, `elasticsearch.logo3d`, `kubernetes.logo3d`, or `spring.logo3d`.

Relationship marks are resolved from `edge.kind`, `directed`, and `edge.presentation`. Directional relationship kinds such as `calls`, `writes`, `reads`, `emits`, `subscribes`, `deploys`, `validates`, `blocks`, `depends_on`, `sends`, and `returns` render with visible arrows by default. Set `presentation.arrow`, `presentation.lineStyle`, `presentation.curve`, `presentation.flow`, and `presentation.color` when direction or motion carries meaning.

Color and legend are semantic, not random. Use `view.colorBy` or `renderHints.colorBy` with values such as `provider`, `kind`, `status`, `group`, `phase`, `risk`, or `severity`, and set `renderHints.showLegend=true` when color has meaning. `inspect-input` warns on `generic_sphere_overuse`, `mark_shape_missing`, `provider_service_unknown`, `asset_icon_unknown`, `asset_model_missing`, `asset_remote_url_forbidden`, `asset_registry_path_missing`, `asset_model_too_large`, `asset_vendor_attribution_missing`, `edge_direction_missing`, `arrow_encoding_missing`, `single_color_detected`, `color_encoding_missing`, `legend_missing`, and `provider_icon_without_attribution`.

The bundled AWS icon ids are local styled placeholders for offline visualization. Simple Icons based technology logos and generated `*.logo3d` badges are vendored local assets with attribution metadata. They are not official vendor 3D models. If official assets are vendored later, update `_shared/assets/ATTRIBUTIONS.md`, `_shared/assets/attributions/ASSETS.md`, `_shared/asset-registry.json`, tests, and release notes together.

The build-time asset pipeline lives under `scripts/assets/`: `logo_catalog.json` is the allowlist, `fetch_logo_assets.mjs` vendors SVGs, `convert_svg_to_3d.mjs` creates local GLB badges, `vecto3d_adapter.mjs` probes an optional vecto3d checkout/command, `optimize_generated_models.mjs` records model size, and `validate_asset_registry.mjs` checks local paths and attribution metadata. Runtime render never downloads assets.

For `architecture.isometric_overview`, badge readability is controlled by `renderHints.badgeMode`, `renderHints.badgeSize`, `renderHints.badgePlacement`, and `renderHints.labelIcon`. Use `icon_and_model`, `medium`, `front`, and `true` for normal architecture diagrams. Use `badgeSize=large` mainly for sparse reviews such as `templates/visual/architecture.isometric_overview/examples/asset-gallery.input.json`, which is the development gallery for checking whether vendored SVG icons and generated `*.logo3d` badges are visually recognizable.

## Visual Plan

`visual inspect-plan` is the pre-render planning step for agents. It validates input, runs the same quality rules as `inspect-input`, and returns `visual_plan.schema=efp.visual.plan.v1`. The plan contains a normalized `visual_plan.ir` with objects, relationships, events, and counts; a first-view budget with focus ids and hidden detail ids; label buckets; legend hints; disclosure strategy; selection behavior; mark statistics, edge direction counts, color/legend analysis, asset usage, quality-loop actions, and render command hints.

Use `inspect-plan` after fixing `inspect-input` warnings and before `visual render`. It does not analyze screenshots or rendered pixels; it tells the agent whether the semantic input is likely to produce a readable first view.

`visual inspect-render` runs after `visual render`. It checks required output files, offline safety, manifest/data consistency, local Three.js asset presence, shape diversity, visible arrows, color diversity, legend presence, local icon/model assets, attributions, and the rebuilt visual plan so agents can catch a rendered artifact that is technically valid but still hard to read. For `architecture.isometric_overview`, it also inspects generated artifact hooks in `index.html`, runtime JS/CSS, `manifest.js`, and `data.js`: runtime wiring, isometric renderer registration, label-layer hooks, entity/link/zone label hooks, base-plane/grid/leader-line/arrow hooks, generated model badge hooks, local asset registry/icons/models, remote asset URL absence, and absence of Studio/starfield hooks in the isometric path. If a screenshot is available, pass `--screenshot <png|jpg|gif>` to add blankness, contrast, and visible coverage checks.

`visual inspect-browser` is the browser-level smoke step for rendered artifacts. It serves `--out` through a temporary `http://127.0.0.1:<port>/index.html` server, launches local Chrome/Chromium headlessly, waits for the runtime data and renderer DOM hooks, writes a screenshot to `--screenshot` or `<out>/visual-screenshot.png`, and then reuses `inspect-render --screenshot` checks. It does not use `file://`, does not contact remote URLs, and does not require npm packages. The checks are deliberately mechanical: DOM hooks for labels/icons/model badges/control bars, local request safety, console/network errors, and screenshot nonblank/contrast/coverage. It is not OCR and does not judge whether a vendor logo is semantically recognizable.

Example for the architecture badge gallery:

```bash
visual render --template architecture.isometric_overview --template-dir ./templates/visual --input ./templates/visual/architecture.isometric_overview/examples/asset-gallery.input.json --out ./out/isometric-asset-gallery --json
visual inspect-browser --template-dir ./templates/visual --out ./out/isometric-asset-gallery --screenshot ./out/isometric-asset-gallery/screenshot.png --json
visual inspect-render --template-dir ./templates/visual --out ./out/isometric-asset-gallery --screenshot ./out/isometric-asset-gallery/screenshot.png --json
```

`inspect-browser` requires a Chrome or Chromium executable and Node.js. It returns `browser_runtime_missing` when either runtime is unavailable; CI smoke may set `EFP_SKIP_BROWSER_SMOKE=1` only when browser smoke is intentionally skipped.

## Render Output Contract

Successful render output includes:

- `index.html`
- `manifest.json`
- `manifest.js`
- `data.js`
- `assets/runtime/efp-visual-runtime.iife.js`
- `assets/runtime/efp-visual-renderers.iife.js`
- `assets/runtime/efp-visual-runtime.css`
- `assets/vendor/three/efp-three.module.min.js` when the renderer uses Three.js
- `assets/templates/<template-id>/style.css`
- `assets/agent-guidance/mark-grammar.md`
- `assets/mark-registry.json`
- `assets/asset-registry.json`
- `assets/ATTRIBUTIONS.md`
- `assets/icons/**`
- `assets/models/**`
- `assets/attributions/**`
- `assets/manifests/**`

The JSON response returns `data.artifact`:

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

`manifest.json` also includes `assets.icons`, `assets.models`, `assets.attributions`, and embedded `assets.mark_registry` / `assets.asset_registry` objects so Portal/runtime and `inspect-render` can explain which local marks and assets were used.

## Offline Contract

Artifacts must be fully offline:

- all asset links are relative paths
- no CDN or remote URL
- no runtime `fetch`, `XMLHttpRequest`, `WebSocket`, `EventSource`, or beacon APIs
- no generated JavaScript from user input
- no Node/npm requirement
- no `go:embed` template packaging

`manifest.js` assigns `window.__EFP_VISUAL_MANIFEST__`. `data.js` assigns `window.__EFP_VISUAL_DATA__`. The runtime reads those globals and renders with local Three.js, SVG, HTML, and CSS.

## Validation And Inspection

```bash
visual validate --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --json
visual inspect-input --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --json
visual inspect-plan --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/game-session-flow.input.json --out ./out/sequence --json
visual inspect-render --template-dir ./templates/visual --out ./out/sequence --json
visual inspect-browser --template-dir ./templates/visual --out ./out/sequence --json
visual template doctor --template-dir ./templates/visual --json
visual inspect-output --out ./out/sequence --json
```

`visual template doctor` reads `registry.json`, validates registry expected counts, checks for unregistered direct directories, validates every manifest/schema/example, renders every example into a temporary output directory, checks required output files, scans rendered output for offline violations, and deletes the temporary directory.

For the built-in semantic catalog, doctor must report:

- `checked_templates: 34`
- `checked_examples: 34`
- `rendered_examples: 34`
- `canonical_template_dirs: 34`
- `orphan_template_dirs: []`
- `offline: true`

Use `--dry-run` on `visual render` to preview `planned_files` without creating `--out`.

## Template Agent Guides

Visual templates carry their own authoring contract in `templates/visual/<template-id>/agent-guide.md` and `quality.rules.json`. The global CLI instructions intentionally stay short: agents discover a template, read its schema, then read `visual template guide <template-id> --json` before writing semantic input JSON.

`visual template guide` returns the guide path, raw markdown, parsed sections, and a compact summary. `visual template get` and `visual template schema` also expose whether the guide and quality rules are available.

`inspect-input` reads the selected template schema, agent guide, and quality rules. Warnings are machine-readable and include `code`, `severity`, `path`, `suggestion`, and usually `auto_fix_hint`. Agents should revise input JSON until bad-density warnings are resolved or intentionally accepted.

The current deep mark consumption is strongest for graph-based relationship/spatial/flow/hierarchy templates, matrix templates, UML component/activity/state templates, and `uml.sequence_3d` message arrows. Evidence and timeline templates use shared guidance, color policy, and inspection first; more renderer-specific shape/icon consumption can be added without changing the agent workflow.
