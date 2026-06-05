# temporal.incident_timeline Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md`.

## When to use this template

Use this template for meaningful time-ordered changes, milestones, incidents, replays, releases, and event traces. Do not use it for unordered dependencies or static ownership hierarchy.

## Semantic model

- event = meaningful change or milestone.
- time/order = temporal placement.
- lane = actor, system, category, source, or phase.
- duration/start/end = interval when applicable.
- importance = milestone priority.

## Required construction rules

1. Events must be meaningful time events, not arbitrary log lines.
2. Each event should include id, label, time or order, kind, lane, importance, and summary.
3. Use duration/start/end for intervals rather than fake point events.
4. Aggregate repeated events with count/summary.
5. Lanes should represent actor, system, category, or phase.
6. Milestones use importance >= 0.75.
7. Low-value events use visibility=detail.
8. visual.annotations mark turning points, anomalies, root cause, and outcome.

## Recommended fields

Use `importance`, `visibility`, `labelPriority`, `summary`, `details`, `sourceRefs`, `presentation`, `visual`, `view`, and `renderHints` from `../_shared/agent-guidance/common-visual-quality.md`.

## Visual encoding rules

Time/order drives position. Lanes separate actors/categories. Importance controls label and marker weight. Status/kind drives color. Annotations identify inflection points.

## Common mistakes to avoid

- Expanding every log line.
- No lane.
- No milestone.
- Long labels.
- All events have equal importance.

## Quality checklist before render

- Events are sorted and meaningful.
- Repetition is aggregated.
- Milestones are marked.
- visual focus and annotations are present.

## Minimal good example

```json
{"events":[{"id":"deploy","time":"2026-06-03T12:00:00Z","lane":"release","label":"Deploy started","importance":0.8,"summary":"Release moved to production"}]}
```
