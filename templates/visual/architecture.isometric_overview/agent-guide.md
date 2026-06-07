# architecture.isometric_overview Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/mark-grammar.md`.

## When to use this template

Use this template when the user needs a static, inspectable architecture overview: runtime topology, microservice dependencies, gateway/service/data zones, storage and observability paths, or deployment-level system boundaries.

Start by authoring `zones`, `entities`, and `links` so the renderer can draw an isometric base plane, typed infrastructure objects, and directional architecture routes.

## Do not use this template

Do not use it for Studio pages, narrative dashboards, generic graph exploration, UML sequence timing, matrix comparison, incident timelines, code-level package graphs, or unbounded dependency clouds.

## Semantic model

- zone = a bounded architecture area on the base plane, with `id`, `label`, and numeric `bounds`.
- entity = a typed system object such as client, nginx, service, nacos, redis, mysql, oss, admin, or elasticsearch.
- link = a directed relationship between entities, with a short label, architecture-specific kind, arrow, and route.
- label = a top HTML billboard above every visible entity, with an optional leader line back to the 3D object.
- base plane = the pale isometric surface and grid that anchors zones and routes.
- camera = orthographic isometric view with constrained orbit; the first view must make zones, entities, and arrows readable.
- label/base plane/camera are renderer responsibilities, but the input must provide enough fields for them.

## Required construction rules

1. Create `zones` first. Every meaningful area must have `bounds.x`, `bounds.y`, `bounds.w`, and `bounds.h`.
2. Create `entities` with `id`, short `label`, architecture `kind`, `zone`, and explicit `position`.
3. Create `links` with `id`, `from`, `to`, short `label`, specific `kind`, `directed: true`, and `routeStyle` or explicit `route`.
4. Use `theme: architecture_light` and `canvas.grid.enabled: true`.
5. Use `visual.assumptions` for placement choices or inferred behavior.
6. Do not create `panels`, `hero`, `nav`, `story`, or Studio walkthrough content.
7. Do not use generic `nodes`/`edges`; use `entities`/`links`.
8. Do not rely on generic graph shapes or fallback spheres. Entity `kind` drives generated architecture geometry.

## Recommended fields

Use these exact kinds where possible: `client`, `pc`, `mobile`, `cdn`, `gateway`, `nginx`, `api_gateway`, `ingress`, `load_balancer`, `service`, `microservice`, `registry`, `nacos`, `admin`, `database`, `mysql`, `postgres`, `mongodb`, `cache`, `redis`, `storage`, `oss`, `minio`, `file_storage`, `block_storage`, `queue`, `kafka`, `rocketmq`, `rabbitmq`, `log`, `elasticsearch`, `security`, `external`, `kubernetes`, `pod`, `node`, `cluster`.

Use these exact link kinds where possible: `api_call`, `static_resource`, `reverse_proxy`, `load_balancing`, `register`, `registering_service`, `pull_consumption`, `health_check`, `data_cache`, `data_storage`, `replication`, `distributed_file_service`, `feign_call`, `log_collection`.

Prefer `visual.initial_focus_ids`, `visual.narrative_steps`, `visual.annotations`, `renderHints.colorBy`, `renderHints.showLegend`, and explicit `route` bend points for dense architecture maps.

## Visual encoding rules

- Entity `kind`, `provider`, `service`, `platform`, and `presentation` drive 3D geometry, local icon selection, and generated model badges.
- Use `presentation.icon` and `presentation.model` only with local registry IDs. Examples: `nginx` + `nginx.logo3d`, `redis` + `redis.logo3d`, `mysql` + `mysql.logo3d`, `elasticsearch` + `elasticsearch.logo3d`, `kubernetes` + `kubernetes.logo3d`, `spring` + `spring.logo3d`.
- Do not invent vendor logo URLs or remote model URLs. If a product logo is not locally registered, use a generic icon such as `generic.service`, `generic.database`, `generic.storage`, or `generic.registry`.
- Zone `bounds` and `presentation.boundary` drive floor plates and solid/dashed/dotted boundaries.
- Link `kind`, `directed`, `presentation.arrow`, `presentation.lineStyle`, and `presentation.color` drive thick arrows, line style, and route color.
- Use `theme: architecture_light`; do not choose dark starfield or decorative particle effects for this renderer.

## Layout guide with conventional placement and assumptions in visual.assumptions

Place client-facing zones on the left/front, edge/CDN/nginx next, API gateway before service clusters, registry/admin above or behind services, cache/database/storage on the lower or right side, and observability near the outbound log path. Keep high-traffic routes separated by explicit orthogonal `route` points when links would overlap.

If a source architecture omits exact positions, choose stable conventional placement and record it in `visual.assumptions`, for example: "Service instances are grouped by logical role because replica topology was not specified." Do not leave entities unpositioned and expect auto-layout to solve the view.

## Common mistakes to avoid

- No zones or zones without numeric bounds.
- Entities missing `kind` or `position`.
- Links missing `directed: true`, arrows, route style, or short labels.
- Long relationship labels that read like documentation sentences.
- Generic `nodes`/`edges` input copied from graph templates.
- Studio fields such as `panels`, `hero`, `navigation`, or `story`.
- Dark starfield, holographic, dust, or decorative background themes.
- Service-to-service calls crossing many entities without explicit bend points.

## Quality checklist before render

- `schema` is `efp.visual.input.isometric_architecture.v1`.
- `theme` is `architecture_light`.
- `canvas.grid.enabled` is true.
- Every zone has `id`, `label`, and numeric `bounds`.
- Every important entity has `id`, `label`, `kind`, `zone`, and `position`.
- Every visible entity can receive a top label and leader line.
- Important infrastructure entities use a local icon or generated model badge, never a remote URL.
- Every link has `directed: true`, specific `kind`, short `label`, and route style or explicit route.
- Routes avoid obvious overlaps in the first view.
- `visual.assumptions` explains inferred placement or grouped replicas.
- No legacy panel grammar, Studio panels, generic graph fallback, starfield, dust, or holographic effects.

## Minimal good example

```json
{
  "schema": "efp.visual.input.isometric_architecture.v1",
  "title": "Gateway To Service Path",
  "theme": "architecture_light",
  "canvas": { "grid": { "enabled": true } },
  "zones": [
    { "id": "edge", "label": "Edge", "bounds": { "x": 0, "y": 0, "w": 4, "h": 4 } },
    { "id": "service", "label": "Services", "bounds": { "x": 5, "y": 0, "w": 6, "h": 4 } }
  ],
  "entities": [
    { "id": "nginx", "label": "Nginx", "kind": "nginx", "zone": "edge", "position": { "x": 2, "y": 2 } },
    { "id": "order-service", "label": "Order Service", "kind": "microservice", "zone": "service", "position": { "x": 7, "y": 2 } }
  ],
  "links": [
    { "id": "route", "from": "nginx", "to": "order-service", "label": "routes", "kind": "reverse_proxy", "directed": true, "presentation": { "arrow": "forward" } }
  ],
  "visual": {
    "goal": "Show the first service route.",
    "initial_focus_ids": ["nginx", "order-service"],
    "narrative_steps": [{ "id": "overview", "title": "Request path", "focus_ids": ["nginx", "order-service"] }],
    "annotations": [{ "id": "route-note", "target_id": "route", "label": "Directional route" }]
  }
}
```
