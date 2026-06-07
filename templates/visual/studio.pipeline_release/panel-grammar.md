# studio.pipeline_release Panel Grammar

This panel grammar extends `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use release Studio panels to explain status, gate evidence, rollout risk, rollback readiness, and next action.

## Semantic model

- overview panel = release readiness and current phase.
- gate panel = checks, owners, evidence, and decision.
- risk panel = risk, blast radius, mitigation, and rollback.
- activity panel = recent events and narrative step.
- action panel = what an operator or reviewer should do next.

## Required construction rules

1. Include an overview panel first.
2. Include at least one gate or risk panel.
3. Every panel must use `target_refs` that point to hero nodes, edges, events, or items.
4. Put numbers in `metrics[]`, not in the title.
5. Put source names, logs, builds, or approvals in `evidence[]`.
6. Use controls for phase focus, gate filter, and rollback inspection.

## Recommended fields

Use `type`, `title`, `summary`, `target_refs`, `items`, `metrics`, `evidence`, `content.status`, `content.owner`, `content.next_action`, and `importance`.

## Visual encoding rules

Order panels by release reading path: overview, gate health, rollout risk, rollback evidence, and next action. Highlight blocked panels with status or risk metrics.

## Common mistakes to avoid

- Panels titled only "Details".
- Gate panels with no evidence.
- Rollout panels that do not state blast radius.
- Controls that do not target phase, status, or rollback ids.

## Quality checklist before render

- Every panel has a non-empty title and summary.
- Gate and risk panels name owners and evidence.
- Controls and navigation target real panel ids.
- The hero has arrows and a visible legend.

## Minimal good example

```json
{"id":"gate-panel","type":"gate","title":"Canary Gate","target_refs":["canary"],"summary":"Explains canary health and approval evidence."}
```
