# Visual Offline Artifacts

`visual` is a terminal-invoked Go CLI for generating offline static visualization artifacts. It reads local templates from `~/.efp/template/visual` by default, validates input JSON, copies local assets, and writes a complete site to `--out`.

It does not call Portal, MCP, Node/npm, a browser runtime, a CDN, or a network service.

The built-in catalog contains 195 canonical templates across 10 categories. See [VISUAL_TEMPLATES.md](VISUAL_TEMPLATES.md) for the full index, category guidance, backward compatibility aliases, schema kinds, layout presets, and template authoring rules.

## Template Directory

Template directory resolution order:

1. `--template-dir`
2. `EFP_VISUAL_TEMPLATE_DIR`
3. `visual.template_dir` from `--config`, then `EFP_CONFIG`, then `~/.efp/config.yaml`
4. `~/.efp/template/visual`
5. `./templates/visual`
6. executable-adjacent release paths

The directory must contain `registry.json`, `_shared/**`, and flat canonical template directories such as `agent.run_trace` and `codebase.module_dependency_graph`. Old template IDs are aliases in `registry.json`, not duplicate template directories.

Paths beginning with `~/` are expanded for `--template-dir`, `EFP_VISUAL_TEMPLATE_DIR`, and `visual.template_dir`.

## Render Contract

Supported renderer contracts:

- `offline.graph.v1`
- `offline.timeline.v1`
- `offline.evidence.v1`
- `offline.matrix.v1`

## 3D And Interaction Effects

Each canonical template declares an `effects` block that describes its intended Three.js scene: camera mode, particle system, material family, motion profile, interactions, and postprocess-style treatment. The shared renderer reads that contract at runtime, creates a local `THREE.WebGLRenderer` scene, and layers 3D meshes, lines, raycast picking, and particles behind the existing SVG/card information layer.

The generated artifact copies a vendored local Three.js bridge module to `assets/vendor/three/efp-three.module.min.js` and loads it with a relative `<script type="module">`. It does not use CDN URLs, remote modules, runtime npm, `fetch`, generated JavaScript from user input, or network access. If WebGL or module loading is unavailable, the existing SVG/HTML renderer still renders the data.

Supported input schema kinds:

- `graph_v1`
- `graph_events_v1`
- `timeline_v1`
- `evidence_v1`
- `matrix_v1`

Example:

```bash
visual template categories --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --category codebase --json
visual template get agent.run_trace --template-dir ./templates/visual --json
visual template schema agent.run_trace --template-dir ./templates/visual --json
visual render --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --out ./out/run-trace --title "Agent Run Trace" --json
```

Canonical IDs are preferred, but compatibility aliases also work. For example, `visual template get service.topology --template-dir ./templates/visual --json` resolves to `runtime.service_topology`, and `visual render --template service.topology ...` renders with the canonical template. These old IDs are registry aliases, not duplicate templates in the file tree.

Count fields distinguish registry entries from compatibility aliases:

- `canonical_count`: canonical templates in `registry.json`
- `alias_count`: compatibility aliases
- `total_count`: canonical templates plus aliases

Each template has a real `schema.input.json` referenced by `template.yaml`:

```json
{
  "schema": "efp.visual.template_input_schema.v1",
  "template_id": "agent.run_trace",
  "input_schema_kind": "graph_events_v1",
  "json_schema": {
    "$schema": "efp.visual.local.schema",
    "type": "object",
    "required": ["nodes"],
    "properties": {
      "schema": {"const": "efp.visual.input.graph_events.v1"},
      "title": {"type": "string"},
      "nodes": {"type": "array", "items": {"type": "object", "required": ["id"]}},
      "edges": {"type": "array", "items": {"type": "object", "required": ["from", "to"]}},
      "events": {"type": "array", "items": {"type": "object", "required": ["id"]}}
    }
  },
  "example": {
    "schema": "efp.visual.input.graph_events.v1",
    "title": "Agent Run Trace",
    "nodes": [{"id": "tool_1", "label": "Read files"}],
    "events": [{"id": "e1", "time": "2026-06-03T12:00:00Z", "kind": "tool_started", "node_id": "tool_1"}]
  }
}
```

`json_schema` is expanded in `visual template schema <template-id> --json`; agents should read it before writing input JSON.

Successful render output includes:

- `index.html`
- `manifest.json`
- `manifest.js`
- `data.js`
- `assets/runtime/**`
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
- no local `data.json` fetch
- no network APIs
- local module scripts are allowed only for vendored relative assets such as the Three.js bridge
- no generated JavaScript from user input
- no `go:embed` template packaging

`manifest.js` assigns `window.__EFP_VISUAL_MANIFEST__`. `data.js` assigns `window.__EFP_VISUAL_DATA__`. The runtime reads those globals and renders with local Three.js, SVG, HTML, and CSS.

## Opening Artifacts

VS Code or desktop:

```bash
visual render --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --out ./out/run-trace --json
```

Open `./out/run-trace/index.html` directly. No local server is required. The artifact is `file://` safe because `index.html` references only relative `manifest.js`, `data.js`, and `assets/**` files.

Portal/runtime proxy:

Serve the generated output directory as static files at any subpath. Portal/runtime proxy serving also depends only on relative paths, so artifacts do not require a fixed base URL.

## Validation And Inspection

```bash
visual validate --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --json
visual template doctor --template-dir ./templates/visual --json
visual inspect-output --out ./out/run-trace --json
```

`visual template doctor` reads `registry.json`, validates registry expected counts, checks for unregistered direct directories under `templates/visual`, validates every `template.yaml`, validates each `schema.input.json`, validates each `examples/basic.input.json`, renders every basic example into a temporary directory, checks required output files, scans the rendered output for offline violations, and deletes the temporary directory.

For the built-in catalog, doctor checks `registry.expected` from `templates/visual/registry.json`, the exact category counts, `canonical_template_dirs: 195`, `orphan_template_dirs: []`, template tree offline safety, non-empty template styles, rendered example output inspection, and at least 190 unique example hashes.

Use `--dry-run` on `visual render` to preview `planned_files` without creating `--out`.
