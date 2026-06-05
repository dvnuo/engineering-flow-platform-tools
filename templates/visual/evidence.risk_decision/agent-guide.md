# evidence.risk_decision Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for claims, sources, decisions, root cause support, and citation-backed reasoning. Do not use it for ordinary entity maps or raw logs.

## Semantic model

- claim = assertion.
- source/evidence = support material.
- link = supports, contradicts, qualifies, mentions, or depends_on relation.
- confidence = credibility.
- status = accepted, disputed, weak, or strong.

## Required construction rules

1. Claims must be judgeable assertions.
2. Sources must have id, label/title, kind, summary, and confidence/reliability.
3. Link kind/relation must be specific: supports, contradicts/refutes, qualifies, mentions, depends_on.
4. Do not treat ordinary entities as evidence.
5. Low-confidence sources should lower confidence rather than disappear.
6. visual.annotations mark key evidence, contradiction, and decision basis.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Claim/source status drives color. Confidence/reliability drives opacity/strength. Link relation drives stroke style. Annotations call out decisive or disputed evidence.

## Common mistakes to avoid

- No confidence values.
- Every link is supports.
- Claim text is a paragraph.
- Sources lack summaries.

## Quality checklist before render

- Claims are concise assertions.
- Sources are credible and summarized.
- Link relations are varied and meaningful.

## Minimal good example

```json
{"claims":[{"id":"c1","text":"The failure is caused by token expiry","confidence":0.82,"summary":"Logs and replay agree"}],"sources":[{"id":"s1","title":"Runtime log","kind":"log","reliability":0.9,"summary":"Shows expiry before retry"}],"links":[{"claim_id":"c1","source_id":"s1","relation":"supports"}]}
```

## Visual Mark System

Read `../_shared/agent-guidance/mark-grammar.md` before writing input JSON. Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, and `presentation.icon` so the renderer can choose service boxes, database cylinders, queue capsules, cloud plates, actor cards, decision diamonds, warning prisms, and local icon billboards.

For causal, dependency, call, data-flow, event, read/write, deploy, validate, block, send, or return relationships, set `directed=true` and `presentation.arrow=forward` or `reverse`. Use `presentation.lineStyle=dashed` and `presentation.flow=true` for async/event movement.

When color has meaning, set `view.colorBy` or `renderHints.colorBy` and `renderHints.showLegend=true`. Do not use random colors; choose provider, kind, status, group, phase, risk, or severity as the color policy.
