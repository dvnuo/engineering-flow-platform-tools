# spatial.control_room Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template when 3D space itself carries meaning: runtime tier, ownership zone, codebase galaxy, service city, operational control room, or fleet position. Do not use 3D merely because it looks dramatic.

## Semantic model

- 3D depth must have semantics, not decoration.
- x/y/z represent layer, domain, time depth, risk, ownership boundary, or runtime tier.
- node/group/zone has explicit semantic meaning.

## Required construction rules

1. Do not use 3D only for visual novelty.
2. Explain what z/depth represents in visual.goal, legend, or annotations.
3. Large scenes must use groups/zones.
4. Important objects use importance, visibility, and labelPriority.
5. Low-value objects default to detail.
6. visual.annotations mark spatial zones, risk areas, and key paths.
7. Initial camera/view must support overview.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Position maps semantic dimensions. Color maps kind/status/zone. Size/glow maps importance. Labels should be controlled by labelPriority. Annotations explain spatial meaning.

## Common mistakes to avoid

- Random z values.
- Every label visible.
- No legend.
- No semantic spatial layers.

## Quality checklist before render

- Depth has a written meaning.
- Groups/zones are defined.
- Focus ids and annotations explain the first view.

## Minimal good example

```json
{"nodes":[{"id":"checkout-api","label":"Checkout API","kind":"service","group":"backend","importance":0.9,"presentation":{"depth":0.35},"summary":"Central transaction service"}]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so the renderer can choose service boxes, database cylinders, queue capsules, cloud plates, actor cards, decision diamonds, warning prisms, and local icon billboards.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, group, phase, risk, or severity as the color policy.
