# uml.sequence_3d Agent Guide

## When to use this template

Use this template when the user wants to understand ordered interactions among semantic actors, services, modules, screens, APIs, managers, or runtime components.

Use it for:
- request / response sequences
- session lifecycle
- workflow orchestration
- frontend / backend collaboration
- state transitions caused by interactions
- setup / running / finalization flows

Do not use it for:
- static class dependency maps
- ownership hierarchy
- metric comparison
- unordered knowledge graphs
- every function call in a codebase

## Semantic model

- participant = semantic lifeline
- message = ordered interaction between lifelines
- phase = color-coded stage shown in legend
- activation = key execution window on a lifeline
- fragment = loop / alt / opt / parallel region
- visual.initial_focus_ids = first-view emphasis
- visual.hidden_detail_ids = low-value details hidden in overview
- visual.annotations = key path, risk, or user-visible result callouts

## Required construction rules

1. Participants must be semantic lifelines. Do not make every function, method, file, or class a participant. Prefer screen/view, view model, controller, service, API, manager, external provider, event stream, and storage.
2. Participants should include id, label, display_name, subtitle, kind, lane_index, depth, and color.
3. Messages should include id, order, from, to, label, kind, phase, curve, importance, label_priority, and summary.
4. Phases must have distinct colors. Phase colors drive message color and the right-side legend.
5. Activations should express key execution windows, not every tiny function call.
6. Fragments should express loop / alt / opt / parallel regions.
7. visual.initial_focus_ids must include the most important lifelines and messages.
8. visual.hidden_detail_ids should hide noisy implementation messages, repeated internal updates, and non-essential return values.
9. visual.annotations should mark critical path, risk points, user-visible result, external boundary, loop/retry area, and cleanup/finalization.

## Recommended fields

Read `../_shared/agent-guidance/common-visual-quality.md`. In this template, prefer participant `display_name`, `subtitle`, `lane_index`, `depth`, `color`; phase `color`; message `curve`, `importance`, `labelPriority` or `label_priority`, `summary`; and global `visual`, `view`, `renderHints`.

## Visual encoding rules

- `lane_index` controls left-to-right lifeline order.
- `depth` controls semantic 3D tier: client, frontend, backend, provider, stream, storage.
- `phases[].color` drives the legend and message path color.
- `importance` controls message thickness, glow, and label visibility.
- `labelPriority` controls whether a message label appears in overview.
- `visual.initial_focus_ids` creates first-view emphasis.
- `visual.hidden_detail_ids` lowers or hides low-value detail in overview.
- `visual.annotations` renders callouts or inspector-visible explanation anchors.

## Common mistakes to avoid

- Do not create a participant for every method.
- Do not expand repeated calls into 20 separate messages if a loop fragment is clearer.
- Do not leave all messages with the same phase.
- Do not omit phase colors.
- Do not make labels long sentences.
- Do not set all messages to equal importance.
- Do not show all low-value return messages in overview.
- Do not use generic participants like "System" unless unavoidable.
- Do not omit visual.initial_focus_ids.
- Do not omit annotations for the key path.

## Quality checklist before render

Before render, confirm:
- participants are semantic lifelines, not low-level functions
- each message references existing participants
- message orders are unique
- phases are meaningful and have colors
- important messages have importance >= 0.75
- only important labels are visible in overview
- loop / alt / opt are represented as fragments
- visual.initial_focus_ids references valid participant/message ids
- visual.hidden_detail_ids references valid low-value details
- visual.annotations explains key path / risk / result

## Minimal good example

```json
{
  "participants": [{"id":"ui","label":"Portal UI","display_name":"Portal UI","subtitle":"screen","kind":"boundary","lane_index":0,"depth":-0.3,"color":"#63a9ff"}],
  "phases": [{"id":"request","label":"Request","color":"#63a9ff"}],
  "messages": [{"id":"m1","order":1,"from":"ui","to":"api","label":"submit()","kind":"sync","phase":"request","curve":"arc","importance":0.9,"labelPriority":"important","summary":"User action starts the request"}],
  "visual": {"goal":"Explain the critical request path first","initial_focus_ids":["ui","m1"],"annotations":[{"id":"a1","target_id":"m1","label":"Critical call"}]}
}
```
