# Visual Offline Artifacts

`visual` is a terminal-invoked Go CLI for generating offline static visualization artifacts. It reads local templates from `templates/visual`, validates input JSON, copies local assets, and writes a complete site to `--out`.

It does not call Portal, MCP, Node/npm, a browser runtime, a CDN, or a network service.

The built-in catalog contains 195 canonical templates across 10 categories. See [VISUAL_TEMPLATES.md](VISUAL_TEMPLATES.md) for the full index, category guidance, backward compatibility aliases, schema kinds, layout presets, and template authoring rules.

## Template Directory

Template directory resolution order:

1. `--template-dir`
2. `EFP_VISUAL_TEMPLATE_DIR`
3. `visual.template_dir` from `--config`, then `EFP_CONFIG`, then `~/.efp/config.yaml`
4. `./templates/visual`
5. executable-adjacent release paths

The directory must contain `registry.json`, `_shared/**`, and flat template directories such as `agent.run_trace` and `codebase.module_dependency_graph`.

## Render Contract

Supported renderer contracts:

- `offline.graph.v1`
- `offline.timeline.v1`
- `offline.evidence.v1`
- `offline.matrix.v1`

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

Canonical IDs are preferred, but compatibility aliases also work. For example, `visual template get service.topology --template-dir ./templates/visual --json` resolves to `runtime.service_topology`, and `visual render --template service.topology ...` renders with the canonical template.

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
- no module scripts
- no generated JavaScript from user input
- no `go:embed` template packaging

`manifest.js` assigns `window.__EFP_VISUAL_MANIFEST__`. `data.js` assigns `window.__EFP_VISUAL_DATA__`. The runtime reads those globals and renders with local SVG/HTML/CSS.

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

`visual template doctor` reads `registry.json`, validates every `template.yaml`, validates each `schema.input.json`, validates each `examples/basic.input.json`, renders every basic example into a temporary directory, checks required output files, scans the rendered output for offline violations, and deletes the temporary directory.

For the built-in catalog, doctor checks `registry.expected` from `templates/visual/registry.json`, the exact category counts, template tree offline safety, non-empty template styles, rendered example output inspection, and at least 190 unique example hashes.

Use `--dry-run` on `visual render` to preview `planned_files` without creating `--out`.
