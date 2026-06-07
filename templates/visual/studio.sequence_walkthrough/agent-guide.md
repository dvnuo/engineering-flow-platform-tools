# studio.sequence_walkthrough Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use this template when the user asks for a walkthrough, explanation, tutorial, or inspectable page for a sequence of messages or events. Use `uml.sequence_3d` when a pure UML sequence diagram is enough.

## Semantic model

- hero = participants, messages, events, and phase context for the walkthrough.
- participant = actor, client, service, worker, or data system.
- message = directed call, event, return, retry, or notification.
- panel = explanation for phase, participant responsibility, failure branch, or evidence.
- control = phase step, participant focus, branch toggle, or retry inspection.

## Required construction rules

1. Set `schema` to `efp.visual.input.studio.v1`.
2. Put participants and messages under `hero.data`.
3. Give participants `kind`, `provider`, `service`, `platform`, and `presentation.icon`.
4. Give every message `from`, `to`, `directed=true`, and `presentation.arrow=forward`.
5. Add panels for overview, phase details, failure branch, and evidence.
6. Add controls for phase stepping and participant focus.
7. Set `renderHints.showLegend=true` when phase, participant, provider, or status color matters.

## Recommended fields

Use `goal`, `audience`, `assumptions`, `hero.data.participants`, `hero.data.messages`, `hero.data.events`, `navigation`, `panels[].target_refs`, `controls`, `annotations`, `view.colorBy`, `renderHints`, and `visual.narrative_steps`.

## Visual encoding rules

Use phase or participant color. Keep message labels short; put protocol details and evidence in panels. Use narrative steps that match the walkthrough order.

## Common mistakes to avoid

- Messages with no direction or arrow.
- Participants with generic marks.
- Panels that do not correspond to phases or important messages.
- Long message labels that should be panel content.
- No controls for step-through reading.

## Quality checklist before render

- Participants and messages form a coherent path.
- Panels explain at least one phase and one detail/failure branch.
- Controls can step or focus the sequence.
- Visual focus and annotations reference real participant, message, panel, or event ids.
- Legend is visible.

## Minimal good example

```json
{"schema":"efp.visual.input.studio.v1","hero":{"type":"sequence","title":"Login walkthrough","data":{"participants":[{"id":"client","label":"Client","kind":"user","presentation":{"icon":"generic.user"}}],"messages":[{"id":"client-to-api","from":"client","to":"api","label":"submit","directed":true,"presentation":{"arrow":"forward"}}]}},"panels":[{"id":"overview","type":"walkthrough","title":"Start","target_refs":["client"],"summary":"Entry point."}]}
```
