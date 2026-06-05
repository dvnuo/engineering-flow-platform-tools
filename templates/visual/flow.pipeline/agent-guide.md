# flow.pipeline Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for process, pipeline, approval, journey, or data movement views. Do not use it for every function call or unordered knowledge graphs.

## Semantic model

- stage = process step or system boundary.
- flow = movement between stages.
- amount/weight = volume or priority.
- loss/dropoff = conversion or failure.
- phase/status = process condition.

## Required construction rules

1. Do not make every call a stage.
2. Stages are stable steps such as intake, validate, transform, persist, notify.
3. Flows must include source/target or from/to, kind, label, weight/amount, and summary.
4. If no real amount exists, use importance/weight but do not invent business metrics.
5. Aggregate repeated low-value flows.
6. Keep stages around 5-9; use detail expansion for complexity.
7. visual.annotations mark bottlenecks, dropoff, external boundaries, and failures.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Stage order should follow process order. Flow weight controls thickness. Status/kind drives color. Hidden/detail visibility controls secondary branches.

## Common mistakes to avoid

- Every function becomes a stage.
- No amount/weight/importance.
- Main path and exception path are indistinguishable.
- Every flow has the same thickness.

## Quality checklist before render

- Main path is clear.
- Error/dropoff paths have summaries.
- Repeated flows are aggregated.
- Focus ids and annotations highlight the story.

## Minimal good example

```json
{"nodes":[{"id":"validate","label":"Validate","kind":"stage","importance":0.8}],"edges":[{"id":"validate->persist","from":"validate","to":"persist","kind":"passes","weight":3,"visibility":"overview","summary":"Valid requests continue"}]}
```
