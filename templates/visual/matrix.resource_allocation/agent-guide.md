# matrix.resource_allocation Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for capability, KPI, risk, allocation, heatmap, score, or comparison views. Do not use it for strict sequences or dependency paths.

## Semantic model

- row = one semantic dimension.
- column = another semantic dimension.
- cell/item = relationship, score, allocation, risk, or capability.
- value/status = metric or qualitative state.
- threshold/scale = interpretation rule.

## Required construction rules

1. Rows and columns must be distinct semantic dimensions.
2. Each item/cell should include row/column or x/y, value/status, kind, importance, and summary.
3. If no real value exists, do not invent precise numbers; use status or qualitative score.
4. Provide legend, scale, or thresholds.
5. Important cells use importance or annotations.
6. Aggregate sparse rows/columns when empty cells dominate.
7. visual.annotations mark extremes, risks, gaps, and priorities.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Position maps row/column or x/y. Color maps status/threshold. Size/emphasis maps importance. Labels should be hover/detail unless critical.

## Common mistakes to avoid

- Matrix used for sequence flow.
- No scale or legend.
- Every cell label visible.
- Too many rows/columns without grouping.

## Quality checklist before render

- Dimensions are clear.
- Scale/thresholds/legend exist.
- Important cells are annotated.

## Minimal good example

```json
{"items":[{"id":"auth-api","label":"Auth API","x":0.2,"y":0.8,"kind":"capability","status":"warning","importance":0.8,"summary":"Needs token refresh hardening"}]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so the renderer can choose service boxes, database cylinders, queue capsules, cloud plates, actor cards, decision diamonds, warning prisms, and local icon billboards.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, group, phase, risk, or severity as the color policy.
