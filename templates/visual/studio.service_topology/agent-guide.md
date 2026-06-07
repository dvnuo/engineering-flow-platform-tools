# studio.service_topology Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use this template when the user asks for an explorable service topology, operational dashboard, dependency page, or walkthrough of how services interact. Use `relationship.service_topology` when a diagram alone is enough.

## Semantic model

- hero = topology of services, gateways, queues, databases, users, and external systems.
- node/item = service, data store, queue, stream, gateway, actor, or operation surface.
- edge/message = directed call, read/write, event, subscription, deploy, or observability relationship.
- panel = operational explanation for service responsibility, dependencies, health, or evidence.
- control = domain, provider, status, risk, or critical path exploration action.

## Required construction rules

1. Set `schema` to `efp.visual.input.studio.v1`.
2. Put important topology objects in `hero.data.nodes` and dependencies in `hero.data.edges`.
3. Give hero objects `kind`, `provider`, `service`, `platform`, and `presentation.icon`.
4. Set `directed=true` and `presentation.arrow=forward` on directional dependencies.
5. Add panels for ingress, core service, state, async path, and operational risk.
6. Add controls for domain filter, provider filter, and critical path focus.
7. Set `renderHints.showLegend=true` when color carries provider, kind, status, or risk.

## Recommended fields

Use `goal`, `audience`, `assumptions`, `hero.summary`, `navigation`, `panels[].target_refs`, `panels[].metrics`, `panels[].evidence`, `controls`, `annotations`, `view.colorBy`, `renderHints`, and `visual`.

## Visual encoding rules

Use provider or domain color. Use API, service, database, queue, stream, cloud, actor, warning, and decision marks. Keep topology labels short and put SLOs, owners, and evidence in panels.

## Common mistakes to avoid

- Generic service nodes without local icons or provider fields.
- Missing arrows on calls, reads, writes, emits, or subscribes edges.
- A topology page with no inspector panels.
- Panels that repeat the graph instead of explaining ownership or risk.
- No legend for provider or domain color.

## Quality checklist before render

- Panels cover ingress, core service, state, async flow, and risk.
- Controls allow critical path and status exploration.
- Navigation targets real panel ids.
- Visual focus and annotations reference existing semantic ids.
- Legend is visible.

## Minimal good example

```json
{"schema":"efp.visual.input.studio.v1","hero":{"type":"topology","title":"Order topology","data":{"nodes":[{"id":"api","label":"Order API","kind":"api","provider":"aws","service":"api_gateway","platform":"aws","presentation":{"icon":"aws.api_gateway"}}]}},"panels":[{"id":"api-panel","type":"inspector","title":"Order API","target_refs":["api"],"summary":"Ingress responsibility and dependencies."}]}
```
