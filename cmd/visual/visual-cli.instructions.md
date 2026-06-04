# visual CLI instructions for VS Code GitHub Copilot

- `visual` is a terminal-invoked CLI, not a browser UI, Portal tool, runtime built-in, MCP tool, or HTTP server.
- Always use `--json` for agent workflows.
- Installed templates default to `~/.efp/template/visual`; use `--template-dir <templates/visual>` only when the catalog lives in a workspace or release artifact.
- First inspect categories with `visual template categories --json`; do not load all 195 template details up front and do not infer templates from the file tree.
- Select a category, then run `visual template list --category <category> --json`, `visual template get <template-id> --json`, and `visual template schema <template-id> --json`.
- Backward compatibility aliases work for `template get`, `template schema`, `validate`, and `render`, but they live only in the registry; prefer the returned `canonical_id` for new work.
- Do not invent template paths and do not point inputs at alias directories.
- Template selection strategy: agent/debug/run work uses `agent` or `debug`; repo/code/diff/test work uses `codebase`; runtime/infra/service/session work uses `runtime`; Jira/GitHub/Confluence/project work uses `project`; evidence/research/citation work uses `knowledge`; plan/task/workflow work uses `planning`; KPI/business/ops work uses `business`; explain/tutorial/process work uses `education`.
- Each template has a declared `effects` contract for its local Three.js scene, including camera, particles, material, motion, and interactions. Prefer selecting the right template and input data over generating custom JavaScript.
- Do not invent the JSON shape. Always run `visual template schema <id> --json` before writing input JSON and again before each render if the selected id changed.
- Render to a workspace path with `visual render --template <template-id> --input <input.json> --out <workspace-output-dir> --json`.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Do not use remote assets, CDN URLs, runtime Node/npm, generated JavaScript, or network APIs.
- Do not generate JavaScript for the artifact; generate input JSON only.
- Outputs are `file://` safe because data is embedded in `data.js` and assets, including the local Three.js module bridge, use relative paths.
- Outputs are Portal proxy safe via relative paths; do not require a fixed base URL.
