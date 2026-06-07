# studio.pipeline_release Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use this template when the user asks for a release walkthrough, dashboard, status page, or explorable pipeline explanation. Use `flow.pipeline` instead when only a compact diagram is needed.

## Semantic model

- hero = release path with services, gates, jobs, events, and outcomes.
- node/item = release stage, job, service, gate, artifact, or signal.
- edge/message = directed handoff, validation, deploy, rollback, or notification.
- panel = explanation surface for status, evidence, risk, metrics, or actions.
- control = filter, focus, step, or toggle that changes what the viewer inspects.

## Required construction rules

1. Set `schema` to `efp.visual.input.studio.v1`.
2. Put the critical release path in `hero.data.nodes` and `hero.data.edges`.
3. Give every important hero object `kind`, `provider`, `service`, `platform`, and `presentation.icon`.
4. Give every directional edge `directed=true` and `presentation.arrow=forward`.
5. Add panels for overview, gates, rollout risk, and rollback or evidence.
6. Add controls for phase focus, status filter, and narrative stepping.
7. Set `renderHints.showLegend=true` when color maps to status, phase, provider, or risk.

## Recommended fields

Use `goal`, `audience`, `assumptions`, `hero.summary`, `navigation`, `panels[].target_refs`, `panels[].metrics`, `panels[].evidence`, `controls`, `annotations`, `view.colorBy`, `renderHints`, and `visual.narrative_steps`.

## Visual encoding rules

Use phase or status color. Use service, job, database, queue, warning, and decision marks instead of generic nodes. Keep the first panel as the executive release state and later panels as inspectable gate evidence.

## Common mistakes to avoid

- A Studio page with a hero graph but no panels.
- Generic stage nodes with no provider/service/platform/icon fields.
- Release handoffs without arrows.
- Every gate described in labels instead of panels.
- No controls for phase focus or rollback inspection.

## Quality checklist before render

- Panels explain current status, blocked gates, rollout risk, and rollback path.
- Navigation points to important panel ids.
- Controls name concrete target ids and actions.
- `visual.initial_focus_ids` and annotations reference existing hero or panel ids.
- Legend and semantic color policy are present.

## Minimal good example

```json
{"schema":"efp.visual.input.studio.v1","hero":{"type":"pipeline","title":"Release path","data":{"nodes":[{"id":"build","label":"Build","kind":"job","provider":"jenkins","platform":"jenkins","presentation":{"icon":"jenkins"}}]}},"panels":[{"id":"build-panel","type":"gate","title":"Build Gate","target_refs":["build"],"summary":"Build status and evidence."}]}
```
