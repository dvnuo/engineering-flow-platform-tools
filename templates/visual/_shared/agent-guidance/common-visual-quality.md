# Common Visual Quality Guidance

All visual templates share these semantic authoring fields. Agents should read this file together with the selected template's `agent-guide.md` before writing input JSON.

## importance

`importance` is a 0..1 information priority used for label level-of-detail, first-view emphasis, object size, path glow, and selection weight. It is not a business metric and it is not the same as edge weight.

## visibility

`visibility` controls progressive disclosure:

- `overview`: visible in the first view.
- `normal`: visible after ordinary filtering or normal mode.
- `detail`: hidden or dimmed until search, expansion, or detail mode.
- `hidden`: not shown initially unless explicitly focused.

## labelPriority

`labelPriority` controls label display:

- `always`: always visible.
- `important`: visible in overview and normal modes.
- `normal`: visible in normal/detail modes.
- `hover`: shown on hover, focus, or inspector.
- `hidden`: never shown as a standing label.

Legacy `label_priority` numeric values are accepted and normalized: `>=0.85` always, `>=0.65` important, `>=0.35` normal, `>0` hover, otherwise hidden.

## summary and details

`summary` is a one-sentence explanation for tooltip, inspector, focus labels, and annotations. `details` is longer explanatory text and should not be placed directly in labels.

## confidence

`confidence` is 0..1 credibility. Do not use it as visual priority; combine it with `importance` only when a low-confidence object is also important to inspect.

## sourceRefs

`sourceRefs` names evidence source ids. Use it for traceability, not as visible label text.

## presentation

`presentation` contains optional visual hints such as `color`, `lane`, `laneIndex`, `depth`, and `positionHint`. Renderers may use these hints, but agents must not rely on presentation to fix incorrect semantics.

## visual

`visual` explains user intent and narrative structure:

- `goal`: what the viewer should understand first.
- `audience`: reviewer, engineer, operator, stakeholder, learner, etc.
- `initial_focus_ids`: objects/messages/events/cells to emphasize first.
- `hidden_detail_ids`: low-value details delayed until search, expansion, or focus.
- `narrative_steps`: ordered viewing story with focus ids.
- `annotations`: callouts for critical path, risk, result, or boundary.

## view

`view` controls initial view intent:

- `mode`: overview, normal, detail, focus.
- `labelMode`: minimal, overview, normal, detail, focus.
- `focusMode`: selected, neighborhood, path, phase.
- `cameraPreset`: overview, left_to_right, top_down, timeline, orbit.

Legacy `initial_view` is accepted for older inputs.

## renderHints

`renderHints` suggests rendering strategy:

- `density`: sparse, normal, dense.
- `routeStrategy`: straight, arc, bundled, orthogonal, timeline.
- `showLegend`: true/false.
- `showAnnotations`: true/false.
