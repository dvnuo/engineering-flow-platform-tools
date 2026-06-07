# studio.entity_explorer Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use this template when the user asks for an explorable page for services, components, entities, capabilities, or ownership. Use a relationship or matrix template when the user only needs one focused visual.

## Semantic model

- hero = selected entities and their important relationships.
- node/item = service, component, capability, owner, risk, data store, or external system.
- edge/message = directed dependency, ownership, data flow, event, support, or risk relation.
- panel = inspector for one entity, relationship group, metric set, evidence set, or assumption.
- control = search, kind filter, status filter, owner focus, or relationship focus.

## Required construction rules

1. Set `schema` to `efp.visual.input.studio.v1`.
2. Put important entities in `hero.data.nodes` or `hero.data.items`.
3. Give hero objects `kind`, `provider`, `service`, `platform`, and `presentation.icon` when available.
4. Set directional relationships with `directed=true` and `presentation.arrow=forward`.
5. Add panels for overview, entity detail, dependency/risk, evidence, and assumptions.
6. Add controls for search, kind filter, status filter, and relationship focus.
7. Set `renderHints.showLegend=true` when color carries meaning.

## Recommended fields

Use `goal`, `audience`, `assumptions`, `hero.summary`, `navigation`, `panels[].target_refs`, `panels[].metrics`, `panels[].evidence`, `controls`, `annotations`, `view.colorBy`, `renderHints`, and `visual`.

## Visual encoding rules

Use kind, provider, status, or risk color. Use service, API, database, queue, actor, warning, and decision marks. Keep entity labels short and put ownership, metrics, and evidence in panels.

## Common mistakes to avoid

- Bare entities with no panels.
- Generic marks that all render as fallback objects.
- Directed dependencies without arrows.
- No controls for search or filtering.
- Missing assumptions for inferred relationships.

## Quality checklist before render

- Overview and entity detail panels exist.
- Controls support search or filtering.
- Panels reference real hero ids.
- Visual focus and annotations reference existing ids.
- Legend is visible when color has meaning.

## Minimal good example

```json
{"schema":"efp.visual.input.studio.v1","hero":{"type":"entity_explorer","title":"Service explorer","data":{"nodes":[{"id":"api","label":"API","kind":"api","provider":"aws","service":"api_gateway","platform":"aws","presentation":{"icon":"aws.api_gateway"}}]}},"panels":[{"id":"api-panel","type":"entity","title":"API","target_refs":["api"],"summary":"Entity responsibility and evidence."}]}
```
