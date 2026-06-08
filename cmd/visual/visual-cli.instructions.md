# visual CLI Instructions for Agents

- `visual` is a terminal-invoked CLI. Always use `--json` for agent workflows.
- Installed templates default to `~/.efp/template/visual`; use `--template-dir <templates/visual>` only for workspace or release artifact catalogs.
- Do not infer templates from the file tree.
- Discover templates only through `categories`, `list`, `get`, `schema`, and `guide`; never list directories to pick a template.
- When the user asks for architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, iCraft-like, or isometric architecture, prefer Mermaid `architecture`/`architecture-beta` or `mermaid.architecture`.
- First inspect categories with `visual template categories --json`.
- Select a category, then run:
  1. `visual template list --category <category> --json`
  2. `visual template get <template-id> --json`
  3. `visual template schema <template-id> --json`
  4. `visual template guide <template-id> --json`
- Prefer Mermaid `.mmd` input for user-authored diagrams. Pure official Mermaid can be passed directly to `visual inspect-input`, `visual inspect-plan`, `visual validate`, and `visual render` without `--template`; the CLI infers the closest visual template.
- Mermaid with EFP frontmatter may add `efp.template`, `efp.camera`, `efp.canvas`, `efp.renderHints`, `efp.visual`, or `efp.view` when a higher-quality layout is needed. Keep the diagram body valid Mermaid.
- Public templates accept Mermaid `.mmd` input.
- The template guide is authoritative for semantic construction rules, recommended fields, visual encoding, common mistakes, and the quality checklist.
- Do not invent template paths or input shapes.
- Do not convert semantic templates into generic graph nodes unless the selected template is actually graph-based.
- For isometric architecture input, use Mermaid architecture/C4 syntax plus EFP frontmatter when explicit camera, routes, or render hints are needed.
- Generate Mermaid only. Do not generate JavaScript, CSS, remote assets, CDN URLs, Node/npm runtime, or network APIs.
- Use Mermaid syntax first. Use EFP frontmatter only for optional rendering hints such as `importance`, `visibility`, `labelPriority`, `summary`, `presentation`, `visual`, `view`, and `renderHints` when they improve readability.
- For Mermaid flowchart/graph-like diagrams, express objects and relationships using official Mermaid nodes, labels, classes, links, and arrows. Use EFP frontmatter only for optional mark hints such as provider/kind/icon/color when needed.
- For Mermaid architecture diagrams, use EFP frontmatter only for layout hints such as `renderHints`, `view`, `visual`, camera, route, and icon/model guidance.
- Give directional relationships Mermaid arrows. Use optional EFP frontmatter only when the renderer needs extra line style, curve, flow, route, or color hints.
- Use `view.colorBy` or `renderHints.colorBy` plus `renderHints.showLegend=true` when color means provider, kind, status, group, phase, risk, or severity.
- Use only local icon/model ids from `asset-registry.json`. Do not use external image or model URLs. AWS icon ids are local styled placeholders, and generated `*.logo3d` files are local visualization badges, not official vendor 3D models.
- For architecture diagrams, keep badge readability explicit in EFP frontmatter when needed: use `renderHints.badgeMode="icon_and_model"`, `renderHints.badgeSize="medium"`, `renderHints.badgePlacement="front"`, and `renderHints.labelIcon=true`.
- If a required logo is missing, do not invent a URL. Ask for it to be added through `scripts/assets/logo_catalog.json`, `fetch_logo_assets.mjs`, and `convert_svg_to_3d.mjs`, or use a generic fallback icon.
- In EFP frontmatter, fill `visual.goal`, `visual.initial_focus_ids`, `visual.hidden_detail_ids`, `visual.narrative_steps`, and `visual.annotations` with valid Mermaid ids when the input has more than a few objects.
- Before render, run `visual inspect-input --input <input.mmd> --json`; include `--template <template-id>` only when you intentionally override the inferred Mermaid template.
- If `inspect-input` returns warnings, revise the Mermaid according to each warning's `suggestion` and `auto_fix_hint` before rendering.
- Then run `visual inspect-plan --input <input.mmd> --out <workspace-output-dir> --json` and use `visual_plan.ir`, `visual_plan.view`, `visual_plan.marks`, `visual_plan.edges`, `visual_plan.colors`, `visual_plan.assets`, `visual_plan.disclosure`, and `visual_plan.quality_loop` to confirm the first view is explainable.
- If `inspect-plan` returns `ready=false`, revise the Mermaid before rendering.
- Render with `visual render --input <input.mmd> --out <workspace-output-dir> --json`; include `--template` only for an intentional Mermaid override.
- Then run `visual inspect-render --out <workspace-output-dir> --json`; if a screenshot is available, add `--screenshot <png|jpg|gif>`. For browser-level visual evidence, run `visual inspect-browser --out <workspace-output-dir> --json`; it serves the artifact through local HTTP, captures a screenshot, and reuses `inspect-render --screenshot`.
- Do not use `file://` for automated visual smoke. `inspect-browser` must use `http://127.0.0.1:<port>/index.html` and should report `browser_runtime_missing` if Chrome/Chromium or Node.js is unavailable.
- If `inspect-render` or `inspect-browser` returns `ready=false`, revise Mermaid or renderer assets using the warnings and render again.
- Return the generated `index.html` path from `data.artifact.entrypoint`.
- Outputs are offline static artifacts and Portal proxy safe through relative paths.
