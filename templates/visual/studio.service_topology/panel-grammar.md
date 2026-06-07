# studio.service_topology Panel Grammar

This panel grammar extends `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use service topology panels to explain responsibility, dependency paths, health, evidence, and operational risks around the hero topology.

## Semantic model

- service panel = owner, API contract, dependencies, SLO, and status.
- dependency panel = why a directed edge exists and what crosses it.
- operations panel = health, alerts, logs, and runbook references.
- risk panel = bottleneck, outage mode, or data exposure.
- assumption panel = unclear or inferred topology facts.

## Required construction rules

1. Include panels for ingress, core service, state, and async/event path when present.
2. Every panel must reference a real hero id with `target_refs`.
3. Put health and SLO numbers in `metrics[]`.
4. Put source names or runbook refs in `evidence[]`.
5. Use controls for provider, domain, status, and critical path focus.

## Recommended fields

Use `type`, `title`, `summary`, `target_refs`, `items`, `metrics`, `evidence`, `content.owner`, `content.slo`, `content.failure_mode`, and `importance`.

## Visual encoding rules

Order panels by topology reading path: ingress, orchestration, state, async flow, risk, and assumptions. Panels should describe what changes when the user focuses a path or status.

## Common mistakes to avoid

- A panel for every node with no prioritization.
- Missing evidence for risky dependencies.
- Navigation labels that do not match panel titles.
- Controls that are not tied to a target or action.

## Quality checklist before render

- Critical path objects have panels.
- Dependency panels explain edge direction.
- Controls and navigation target real ids.
- Assumptions are explicit.

## Minimal good example

```json
{"id":"orders-panel","type":"service","title":"Order Service","target_refs":["orders"],"summary":"Owns orchestration, SLO, and primary downstream calls."}
```
