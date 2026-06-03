# Visual Offline Artifacts

`visual` is a terminal-invoked Go CLI for generating offline static visualization artifacts. It reads local templates from `templates/visual`, validates input JSON, copies local assets, and writes a complete site to `--out`.

It does not call Portal, MCP, Node/npm, a browser runtime, a CDN, or a network service.

## Template Directory

Template directory resolution order:

1. `--template-dir`
2. `EFP_VISUAL_TEMPLATE_DIR`
3. `visual.template_dir` from `--config`, then `EFP_CONFIG`, then `~/.efp/config.yaml`
4. `./templates/visual`
5. executable-adjacent release paths

The directory must contain `registry.json`, `_shared/**`, and template directories such as `agent.run_trace`.

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
visual template list --template-dir ./templates/visual --json
visual template get agent.run_trace --template-dir ./templates/visual --json
visual render --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --out ./out/run-trace --title "Agent Run Trace" --json
```

Successful render output includes:

- `index.html`
- `manifest.json`
- `manifest.js`
- `data.js`
- `assets/runtime/**`
- `assets/template/style.css`

The JSON response returns `data.artifact.entrypoint`.

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

Open `./out/run-trace/index.html` directly. No local server is required.

Portal/runtime proxy:

Serve the generated output directory as static files at any subpath. Because `index.html` references `manifest.js`, `data.js`, and `assets/**` with relative paths, it works under proxy paths without a fixed base URL.

## Validation And Inspection

```bash
visual validate --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --json
visual template doctor --template-dir ./templates/visual --json
visual inspect-output --out ./out/run-trace --json
```

Use `--dry-run` on `visual render` to preview `planned_files` without creating `--out`.
