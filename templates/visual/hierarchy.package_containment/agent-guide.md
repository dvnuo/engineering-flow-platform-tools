# hierarchy.package_containment Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for containment, ownership, layered architecture, package trees, or responsibility maps. Do not use it for arbitrary dependencies.

## Semantic model

- item/node = entity.
- parent/child = containment or ownership.
- group = level/domain.
- importance = initial expansion priority.

## Required construction rules

1. Parent-child must be a real hierarchy, not dependency.
2. Each item should have id, label, kind, parent_id/group, importance, and summary.
3. Hierarchy must not contain cycles.
4. Large trees must use collapsed/detail visibility.
5. Keep first-level branches to 3-5 by aggregating where needed.
6. visual.initial_focus_ids should choose key branches.
7. visual.annotations should mark ownership, risk, and responsibility boundaries.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Parent-child controls nesting and expansion. Importance controls initial expansion and labels. Group/layer controls color or banding.

## Common mistakes to avoid

- Dependency drawn as hierarchy.
- Deep tree without collapse.
- Every leaf label visible in overview.

## Quality checklist before render

- Parent references exist.
- Key branches are focus ids.
- Low-value leaves are detail/hidden.

## Minimal good example

```json
{"nodes":[{"id":"platform","label":"Platform","kind":"layer","importance":0.9},{"id":"api","label":"API","kind":"service","parent_id":"platform","importance":0.75,"summary":"Public entrypoint"}]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so the renderer can choose service boxes, database cylinders, queue capsules, cloud plates, actor cards, decision diamonds, warning prisms, and local icon billboards.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, group, phase, risk, or severity as the color policy.
