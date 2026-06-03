# visual CLI instructions for VS Code GitHub Copilot

- `visual` is a terminal-invoked CLI, not a browser UI, Portal tool, runtime built-in, MCP tool, or HTTP server.
- Always use `--json` for agent workflows.
- First inspect categories with `visual template categories --template-dir <templates/visual> --json`; do not load all 195 template details up front.
- Select a category, then run `visual template list --template-dir <templates/visual> --category <category> --json`, `visual template get <template-id> --template-dir <templates/visual> --json`, and `visual template schema <template-id> --template-dir <templates/visual> --json`.
- Template selection strategy: agent/debug/run work uses `agent` or `debug`; repo/code/diff/test work uses `codebase`; runtime/infra/service/session work uses `runtime`; Jira/GitHub/Confluence/project work uses `project`; evidence/research/citation work uses `knowledge`; plan/task/workflow work uses `planning`; KPI/business/ops work uses `business`; explain/tutorial/process work uses `education`.
- Do not invent the JSON shape. Always run `visual template schema <id> --json` before writing input JSON.
- Render to a workspace path with `visual render --template <template-id> --template-dir <templates/visual> --input <input.json> --out <workspace-output-dir> --json`.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Do not use remote assets, CDN URLs, Node/npm, or network APIs.
- Do not generate JavaScript for the artifact; generate input JSON only.
- Outputs are `file://` safe because data is embedded in `data.js` and assets use relative paths.
- Outputs are Portal proxy safe via relative paths; do not require a fixed base URL.
