# visual CLI Instructions for Agents

- `visual` is a terminal-invoked CLI. Always use `--json` for agent workflows.
- Installed templates default to `~/.efp/template/visual`; use `--template-dir <templates/visual>` only for workspace or release artifact catalogs.
- Do not infer templates from the file tree.
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
- Generate semantic input JSON only. Do not generate JavaScript, CSS, remote assets, CDN URLs, Node/npm runtime, or network APIs.
- Use shared authoring fields from the selected template schema and guide: `importance`, `visibility`, `labelPriority`, `summary`, `details`, `presentation`, `visual`, `view`, and `renderHints` when they improve readability.
- Fill `visual.goal`, `visual.initial_focus_ids`, `visual.hidden_detail_ids`, `visual.narrative_steps`, and `visual.annotations` with valid semantic ids when the input has more than a few objects.
- Before render, run `visual inspect-input --template <template-id> --input <input.json> --json`.
- If `inspect-input` returns warnings, revise input JSON according to each warning's `suggestion` and `auto_fix_hint` before rendering.
- Then run `visual inspect-plan --template <template-id> --input <input.json> --out <workspace-output-dir> --json` and use `visual_plan.ir`, `visual_plan.view`, `visual_plan.disclosure`, and `visual_plan.quality_loop` to confirm the first view is explainable.
- If `inspect-plan` returns `ready=false`, revise input JSON before rendering.
- Render with `visual render --template <template-id> --input <input.json> --out <workspace-output-dir> --json`.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Outputs are offline static artifacts and Portal proxy safe through relative paths.
