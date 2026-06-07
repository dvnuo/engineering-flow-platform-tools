# studio.entity_explorer Panel Grammar

This panel grammar extends `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use entity explorer panels to explain what an entity is, why it matters, how it relates to others, and what evidence supports the page.

## Semantic model

- overview panel = scope and most important entities.
- entity panel = owner, role, status, interfaces, metrics, and evidence.
- relationship panel = dependencies, direction, risk, and consequence.
- evidence panel = sources, confidence, and freshness.
- assumption panel = inferred or incomplete facts.

## Required construction rules

1. Include overview and entity detail panels.
2. Every panel must use `target_refs` pointing to hero entities, relationships, or panels.
3. Use metrics for health, confidence, load, risk, or coverage.
4. Use evidence for sources, traces, owners, or tickets.
5. Use controls for search, kind filter, status filter, and relationship focus.

## Recommended fields

Use `type`, `title`, `summary`, `target_refs`, `items`, `metrics`, `evidence`, `content.owner`, `content.interface`, `content.risk`, and `importance`.

## Visual encoding rules

Order panels by scan path: overview, important entity, dependency or risk, evidence, assumptions. Let controls reveal lower-priority entity groups.

## Common mistakes to avoid

- A panel per raw component with no story.
- Missing owner or evidence in entity panels.
- Relationship panels that do not explain direction.
- Controls without target/action pairs.

## Quality checklist before render

- Entity panels explain role and evidence.
- Relationship panels explain direction and risk.
- Controls and navigation target real ids.
- Assumptions are visible.

## Minimal good example

```json
{"id":"api-panel","type":"entity","title":"API","target_refs":["api"],"summary":"Explains ownership, contract, and dependency evidence."}
```
