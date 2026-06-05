# relationship.issue_dependencies Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for typed relationships among semantic entities such as services, modules, issues, requirements, sources, or knowledge objects. Do not use it for strict hierarchy, ordered sequences, or metric comparison.

## Semantic model

- node = semantic entity.
- edge = typed relationship.
- group = subsystem, owner, layer, domain, or bounded context.
- edge.visibility = overview/detail/hidden.
- node.importance = overview priority.
- edge.importance = relationship priority.

## Required construction rules

1. Do not make every function or file a node unless the user asks for code-level detail.
2. Nodes must have id, label, kind, group, importance, and summary.
3. Edges must have from, to, kind, label, importance, visibility, and summary.
4. Edge kind must be specific. Prefer calls, owns, reads, writes, emits, subscribes, validates, deploys, observes, configures, blocks.
5. Large graphs must define groups.
6. Low-value edges must use visibility=detail or visibility=hidden.
7. Initial view should be overview and avoid showing all nodes and edges.
8. visual.initial_focus_ids should include core nodes and key edges.
9. visual.annotations should mark boundaries, bottlenecks, risks, and key dependencies.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Groups collapse related nodes. Importance affects node size, edge thickness, glow, and labels. Visibility controls first-view density. Edge kind maps to color/style and legend. Annotations provide callouts for risks and boundary crossings.

## Common mistakes to avoid

- All edges are depends_on or related_to.
- No groups.
- Every edge is overview-visible.
- Labels contain full paths or long signatures.
- Nodes are low-level implementation details instead of semantic entities.

## Quality checklist before render

- Each node belongs to a meaningful group.
- Edge kinds are specific and varied.
- Low-value relationships are detail or hidden.
- Focus ids and annotation target ids reference real objects.
- Long labels have summary/details.

## Minimal good example

```json
{"nodes":[{"id":"api","label":"Order API","kind":"service","group":"backend","importance":0.9,"summary":"Owns order transaction"}],"edges":[{"id":"api->db","from":"api","to":"db","kind":"writes","visibility":"overview","importance":0.8,"summary":"Persists order"}]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so the renderer can choose service boxes, database cylinders, queue capsules, cloud plates, actor cards, decision diamonds, warning prisms, and local icon billboards.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, group, phase, risk, or severity as the color policy.
