# visual CLI instructions for VS Code GitHub Copilot

- `visual` is a terminal-invoked CLI, not a browser UI, Portal tool, runtime built-in, MCP tool, or HTTP server.
- Always use `--json` for agent workflows.
- First inspect templates with `visual template list --template-dir <templates/visual> --json`, `visual template get <template-id> --template-dir <templates/visual> --json`, and `visual template schema <template-id> --template-dir <templates/visual> --json`.
- Do not invent JSON shape; read the template schema first.
- Render to a workspace path with `visual render --template <template-id> --template-dir <templates/visual> --input <input.json> --out <workspace-output-dir> --json`.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Do not use remote assets, CDN URLs, Node/npm, or network APIs.
- Do not generate JavaScript for the artifact; generate input JSON only.
- Outputs are `file://` safe because data is embedded in `data.js` and assets use relative paths.
- Outputs are Portal proxy safe via relative paths; do not require a fixed base URL.
