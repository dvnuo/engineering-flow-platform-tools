# architecture.isometric_overview Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for architecture, topology, deployment, service map, system map, infrastructure map, microservice, cloud, iCraft-like, or isometric architecture requests. It is for a spatial architecture scene, not a generic graph.

## Semantic model

- zone = bounded platform, network, runtime, cloud, team, or data area with a visible isometric boundary.
- entity = service, API, worker, queue, stream, database, storage, user, external system, gateway, deployment, or control point.
- link = directional call, read, write, event, dependency, deploy, validate, block, send, or return relationship.
- canvas.grid = base grid and ground plane hint.
- presentation.boundary = zone boundary style.
- presentation.label and presentation.leaderLine = top label and leader line hint.

## Required construction rules

1. Use `zones`, `entities`, and `links`; do not write generic `nodes` and `edges`.
2. Every zone needs `id`, `label`, and numeric `bounds.x/y/w/h`.
3. Every entity needs `id`, `label`, `kind`, `zone`, numeric `position.x/y`, and numeric `size.w/d/h`.
4. Entity `zone` values must reference existing zones.
5. Every link needs `id`, `from`, `to`, `label`, `kind`, `directed=true`, and `presentation.arrow`.
6. Link endpoints must reference existing entities.
7. Add `canvas.grid.enabled=true` for the base plane and grid.
8. Use `camera.preset=isometric` unless the user explicitly asks for another camera.
9. Use `theme=architecture_light`; do not use starfield themes.
10. Use `visual.initial_focus_ids` and `visual.annotations` with entity or link ids.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Zones render as flat bounded slabs on a base plane. Entities render as semantic architecture marks such as service boxes, API hexes, database cylinders, queue capsules, event buses, cloud plates, and actor cards. Links render as routed paths with arrows, labels, and optional route points. Labels should sit above entities with leader lines by default.

## Common mistakes to avoid

- Writing generic `nodes` and `edges` instead of zones/entities/links.
- Omitting `canvas.grid.enabled=true`.
- Leaving entities unpositioned so the renderer has to auto-place everything.
- Using `kind=node`, `kind=component`, or fallback shapes for architecture objects.
- Setting `theme=starfield` or other non-architecture scene themes.
- Omitting directed arrows on data flow, calls, events, reads, writes, and dependencies.

## Quality checklist before render

- Zones are present, bounded, and not accidentally overlapping.
- Entities are labeled, typed, positioned, sized, and attached to existing zones.
- Links are directed, have visible arrows, and use concise labels.
- No generic nodes/edges are present.
- Base plane/grid, isometric camera, architecture_light theme, top labels, and leader lines are planned.

## Minimal good example

```json
{"schema":"efp.visual.input.isometric_architecture.v1","title":"Payment Architecture","canvas":{"grid":{"enabled":true}},"camera":{"preset":"isometric"},"theme":"architecture_light","zones":[{"id":"app","label":"Application","bounds":{"x":0,"y":0,"w":12,"h":10}}],"entities":[{"id":"api","label":"Payment API","kind":"api","zone":"app","position":{"x":4,"y":4},"size":{"w":4,"d":3,"h":2}}],"links":[]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so architecture entities avoid fallback spheres.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, zone, risk, or severity as the color policy.
