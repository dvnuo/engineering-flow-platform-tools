# studio.sequence_walkthrough Panel Grammar

This panel grammar extends `../_shared/agent-guidance/panel-grammar.md`.

## When to use this template

Use sequence walkthrough panels to explain phases, participant responsibility, message consequences, alternatives, and evidence.

## Semantic model

- overview panel = what the viewer should understand before stepping.
- phase panel = grouped messages and expected result.
- participant panel = responsibility and state changes.
- branch panel = failure, retry, alternative, or optional path.
- evidence panel = logs, traces, test cases, or assumptions.

## Required construction rules

1. Include an overview panel and at least one phase or branch panel.
2. Every panel must reference participant, message, event, or panel ids.
3. Use `items[]` for ordered message explanations.
4. Use `evidence[]` for traces, tests, logs, or tickets.
5. Use controls for phase stepping, branch toggles, and participant focus.

## Recommended fields

Use `type`, `title`, `summary`, `target_refs`, `items`, `metrics`, `evidence`, `content.phase`, `content.expected_result`, and `importance`.

## Visual encoding rules

Panel order should match message order. Keep the selected phase visible and make branch panels explicitly optional or exceptional.

## Common mistakes to avoid

- One panel per message when phases would be clearer.
- Panels without target refs.
- Hidden failure branches.
- Missing controls for stepping.

## Quality checklist before render

- Phase panels match narrative step ids.
- Branch panels name trigger and outcome.
- Controls and navigation target real ids.
- Message arrows are visible in hero data.

## Minimal good example

```json
{"id":"phase-auth","type":"phase","title":"Authenticate","target_refs":["msg-login"],"summary":"Shows credentials exchange and token creation."}
```
