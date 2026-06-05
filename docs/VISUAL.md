# Visual Offline Artifacts

`visual` is a terminal-invoked Go CLI for generating complete offline static visualization artifacts. It reads local templates from `~/.efp/template/visual` by default, validates input JSON, copies local assets, and writes a self-contained site to `--out`.

The built-in catalog is now a semantic catalog: 33 canonical templates across 8 categories. It intentionally does not keep legacy aliases or duplicate legacy directories. Discover templates through the CLI, not by guessing file paths.

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

## Renderer Contracts

Supported renderer contracts:

- `offline.graph.v1`
- `offline.timeline.v1`
- `offline.evidence.v1`
- `offline.matrix.v1`
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

UML semantic schema kinds:

- `uml_sequence_v1`
- `uml_class_v1`
- `uml_state_machine_v1`
- `uml_activity_v1`
- `uml_component_deployment_v1`

UML templates are not just graph templates with looser names. For example, `uml.sequence_3d` requires `participants`, ordered `messages`, optional `phases`, `activations`, and `fragments`. The runtime can then draw 3D lifelines, directional arrows, labels, and replay/phase controls.

## Agent Workflow

```bash
visual template categories --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --category uml --json
visual template get uml.sequence_3d --template-dir ./templates/visual --json
visual template schema uml.sequence_3d --template-dir ./templates/visual --json
visual inspect-input --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --json
visual render --template uml.sequence_3d --template-dir ./templates/visual --input ./templates/visual/uml.sequence_3d/examples/basic.input.json --out ./out/sequence --title "Checkout Sequence" --json
```

Agents must read `visual template schema <id> --json` before writing input JSON. Do not invent JSON shape. Do not infer templates from directories.

Every semantic input schema includes a shared `visual` object. Use it to tell the renderer how to make the first view readable instead of asking the agent to generate JavaScript:

- `visual.goal`: what the viewer should understand first
- `visual.initial_focus_ids`: semantic ids emphasized in the first view
- `visual.hidden_detail_ids`: secondary detail that can stay collapsed, muted, or hidden until search/focus
- `visual.narrative_steps`: progressive explanation beats with `focus_ids`
- `visual.annotations`: callouts attached to existing `target_id` values

Graph, timeline, matrix, evidence, and UML renderers use this guidance for focus styling, delayed detail, labels, and annotations.

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
visual template doctor --template-dir ./templates/visual --json
visual inspect-output --out ./out/sequence --json
```

`visual template doctor` reads `registry.json`, validates registry expected counts, checks for unregistered direct directories, validates every manifest/schema/example, renders every example into a temporary output directory, checks required output files, scans rendered output for offline violations, and deletes the temporary directory.

For the built-in semantic catalog, doctor must report:

- `checked_templates: 33`
- `checked_examples: 33`
- `rendered_examples: 33`
- `canonical_template_dirs: 33`
- `orphan_template_dirs: []`
- `offline: true`

Use `--dry-run` on `visual render` to preview `planned_files` without creating `--out`.

## Template Agent Guides

Visual templates now carry their own authoring contract in `templates/visual/<template-id>/agent-guide.md` and `quality.rules.json`. The global CLI instructions intentionally stay short: agents discover a template, read its schema, then read `visual template guide <template-id> --json` before writing semantic input JSON.

`visual template guide` returns the guide path, raw markdown, parsed sections, and a compact summary. `visual template get` and `visual template schema` also expose whether the guide and quality rules are available.

`inspect-input` reads the selected template schema, agent guide, and quality rules. Warnings are machine-readable and include `code`, `severity`, `path`, `suggestion`, and usually `auto_fix_hint`. Agents should revise input JSON until bad-density warnings are resolved or intentionally accepted.

The current deep renderer consumption is strongest for `uml.sequence_3d` and graph-based relationship/spatial/flow/hierarchy templates. Other templates have guides and quality checks first; more renderer-specific consumption can be added without changing the agent workflow.
