# visual CLI Instructions for Agents

- `visual` is a terminal-invoked CLI. Always use `--json` for agent workflows.
- Installed templates default to `~/.efp/template/visual`; use `--template-dir <templates/visual>` only for workspace or release artifact catalogs.
- Do not infer templates from the file tree.
- Discover templates only through `categories`, `list`, `get`, `schema`, and `guide`; never list directories to pick a template.
- When the user asks for architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, iCraft-like, or isometric architecture, prefer the `architecture` category first.
- First inspect categories with `visual template categories --json`.
- Select a category, then run:
  1. `visual template list --category <category> --json`
  2. `visual template get <template-id> --json`
  3. `visual template schema <template-id> --json`
  4. `visual template guide <template-id> --json`
- Do not write input JSON until you have read the selected template's guide.
- The template guide is authoritative for semantic construction rules, recommended fields, visual encoding, common mistakes, and the quality checklist.
- Do not invent template paths or input shapes.
- Do not convert semantic templates into generic graph nodes unless the selected template is actually graph-based.
- For `architecture.isometric_overview`, generate `zones[]`, `entities[]`, and `links[]`; do not generate generic `nodes[]` / `edges[]`.
- For isometric architecture inputs, define `canvas.grid.enabled=true`, bounded zones, positioned/sized entities with `kind`, and directed links with visible arrows.
- Generate semantic input JSON only. Do not generate JavaScript, CSS, remote assets, CDN URLs, Node/npm runtime, or network APIs.
- Use shared authoring fields from the selected template schema and guide: `importance`, `visibility`, `labelPriority`, `summary`, `details`, `presentation`, `visual`, `view`, and `renderHints` when they improve readability.
- Use the Visual Mark System for graph-like objects and relationships. Give nodes `kind`, `provider`, `service`, `platform`, or `presentation.shape` / `presentation.mesh` / `presentation.icon` so the renderer can choose boxes, cylinders, capsules, cloud plates, actor cards, diamonds, warnings, and icons instead of fallback spheres.
- Give directional relationships `directed=true` and `presentation.arrow`; use `presentation.lineStyle`, `presentation.curve`, and `presentation.flow` for data movement, events, calls, reads, writes, dependencies, returns, and blocking edges.
- Use `view.colorBy` or `renderHints.colorBy` plus `renderHints.showLegend=true` when color means provider, kind, status, group, phase, risk, or severity.
- Use only local icon ids from `asset-registry.json`. Do not use external image URLs. AWS and Jenkins icon ids in the bundled catalog are local styled placeholders, not official vendor logos.
- Fill `visual.goal`, `visual.initial_focus_ids`, `visual.hidden_detail_ids`, `visual.narrative_steps`, and `visual.annotations` with valid semantic ids when the input has more than a few objects.
- Before render, run `visual inspect-input --template <template-id> --input <input.json> --json`.
- If `inspect-input` returns warnings, revise input JSON according to each warning's `suggestion` and `auto_fix_hint` before rendering.
- Then run `visual inspect-plan --template <template-id> --input <input.json> --out <workspace-output-dir> --json` and use `visual_plan.ir`, `visual_plan.view`, `visual_plan.marks`, `visual_plan.edges`, `visual_plan.colors`, `visual_plan.assets`, `visual_plan.disclosure`, and `visual_plan.quality_loop` to confirm the first view is explainable.
- If `inspect-plan` returns `ready=false`, revise input JSON before rendering.
- Render with `visual render --template <template-id> --input <input.json> --out <workspace-output-dir> --json`.
- Then run `visual inspect-render --out <workspace-output-dir> --json`; if a screenshot is available, add `--screenshot <png|jpg|gif>`. If it returns `ready=false`, revise input JSON using the warnings and render again.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Outputs are offline static artifacts and Portal proxy safe through relative paths.
