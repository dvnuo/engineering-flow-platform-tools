# Visual Templates

The public visual template catalog is a Mermaid catalog. It contains 28 canonical `mermaid.*` templates in one `mermaid` category, matching Mermaid Diagram Syntax families. Users and agents author Mermaid `.mmd`; the CLI compiles Mermaid into internal renderer IR before rendering.

## Registry Expected Counts

`templates/visual/registry.json` owns the public catalog shape:

```json
{
  "version": 4,
  "expected": {
    "canonical_count": 28,
    "categories": {
      "mermaid": 28
    }
  }
}
```

`visual template doctor` uses this metadata and renders every public `examples/basic.mmd`.

## Public Template Index

- `mermaid.flowchart`
- `mermaid.sequence`
- `mermaid.class`
- `mermaid.state`
- `mermaid.er`
- `mermaid.journey`
- `mermaid.gantt`
- `mermaid.pie`
- `mermaid.quadrant`
- `mermaid.requirement`
- `mermaid.gitgraph`
- `mermaid.c4`
- `mermaid.mindmap`
- `mermaid.timeline`
- `mermaid.zenuml`
- `mermaid.sankey`
- `mermaid.xy`
- `mermaid.block`
- `mermaid.packet`
- `mermaid.kanban`
- `mermaid.architecture`
- `mermaid.radar`
- `mermaid.event_modeling`
- `mermaid.treemap`
- `mermaid.venn`
- `mermaid.ishikawa`
- `mermaid.wardley`
- `mermaid.treeview`

`graph` is accepted as a Mermaid flowchart alias and maps to `mermaid.flowchart`. Beta names such as `architecture-beta`, `sankey-beta`, `xychart-beta`, `block-beta`, `packet-beta`, `radar-beta`, `treemap-beta`, and `wardley-beta` are accepted where Mermaid uses them.

## Authoring Contract

- Public examples are Mermaid files at `examples/basic.mmd`.
- `template schema <id>` returns `input_format: "mermaid"`, `mermaid_syntax`, the `.mmd` example, and the internal compiled schema for renderer diagnostics.
- Public templates reject non-Mermaid input with `mermaid_input_required`.
- EFP frontmatter may be used for layout hints while the body remains official Mermaid.
- Do not infer templates from directories; use `visual template categories`, `visual template list`, `visual template get`, `visual template schema`, and `visual template guide`.

## Internal Renderer Targets

Older semantic template directories remain registered as `internal` renderer targets. They are implementation assets for Mermaid compilation, testing, and offline rendering. They are not public authoring templates and should not appear in `visual template list`.

## Doctor Expectations

For the built-in public catalog, doctor must report:

- `canonical_templates: 28`
- `checked_templates: 28`
- `checked_examples: 28`
- `rendered_examples: 28`
- `orphan_template_dirs: []`
- `offline: true`

## Visual Mark System

The shared mark files remain part of the renderer contract:

- `_shared/agent-guidance/mark-grammar.md`
- `_shared/mark-registry.json`
- `_shared/asset-registry.json`
- `_shared/assets/icons/**`
- `_shared/assets/models/**`
- `_shared/assets/ATTRIBUTIONS.md`

Mermaid syntax is the input; mark and asset registries are renderer-side support for richer shapes, arrows, colors, icons, badges, and offline attribution.
