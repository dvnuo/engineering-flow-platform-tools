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

`presentation` contains optional visual hints such as `shape`, `mesh`, `icon`, `model`, `color`, `arrow`, `lineStyle`, `flow`, `lane`, `laneIndex`, `depth`, and `positionHint`. Renderers may use these hints, but agents must not rely on presentation to fix incorrect semantics.

For object marks:

- `presentation.shape`: semantic shape such as service_box, database_cylinder, queue_capsule, cloud_plate, actor_card, diamond, or warning_prism.
- `presentation.mesh`: Three.js primitive hint such as box, card, cylinder, capsule, cloud, octahedron, cone, hex_prism, or sphere.
- `presentation.icon`: local icon id from `asset-registry.json`, such as generic.database, aws.lambda, aws.s3, aws.rds, aws.sqs, aws.eventbridge, aws.api_gateway, nginx, redis, mysql, kubernetes, or jenkins.
- `presentation.model`: local generated model id from `asset-registry.json`, such as nginx.logo3d, redis.logo3d, mysql.logo3d, elasticsearch.logo3d, kubernetes.logo3d, or spring.logo3d.
- `provider`, `service`, and `platform`: semantic provider fields used before falling back to kind.

Do not place remote image or model URLs in `presentation.icon` or `presentation.model`. Use the local asset registry IDs only.

For `architecture.isometric_overview`, use `renderHints.badgeMode`, `renderHints.badgeSize`, `renderHints.badgePlacement`, and `renderHints.labelIcon` to keep local icon/model badges readable. Default to `icon_and_model`, `medium`, `front`, and `true`; reserve `large` for sparse asset gallery review scenes.

For relationship marks:

- `directed`: true when direction matters.
- `presentation.arrow`: forward, reverse, or none.
- `presentation.lineStyle`: solid, dashed, or dotted.
- `presentation.flow`: true when particles should show data, event, call, or causal movement.

Do not rely on generic sphere nodes for semantic entities. Read `mark-grammar.md` before authoring large graph, flow, component, or spatial inputs.

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
- `colorBy`: semantic color policy, such as kind, provider, status, group, phase, risk, or severity.

Legacy `initial_view` is accepted for older inputs.

## renderHints

`renderHints` suggests rendering strategy:

- `density`: sparse, normal, dense.
- `routeStrategy`: straight, arc, bundled, orthogonal, timeline.
- `showLegend`: true/false.
- `showAnnotations`: true/false.
- `palette`: semantic_dark, cloud_provider, status, phase, or risk.
- `colorBy`: semantic color policy if not set in view.
- `iconMode`: billboard or none.
- `modelMode`: badge or none.
