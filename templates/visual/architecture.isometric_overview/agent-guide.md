# architecture.isometric_overview Agent Guide

This template guide extends `../_shared/agent-guidance/common-visual-quality.md` and `../_shared/agent-guidance/mark-grammar.md`.

## When to use / Do not use

Use this template when the user needs a static, inspectable architecture overview: runtime topology, microservice dependencies, gateway/service/data zones, storage and observability paths, or deployment-level system boundaries.

Do not use it for Studio pages, narrative dashboards, generic graph exploration, UML sequence timing, matrix comparison, incident timelines, code-level package graphs, or unbounded dependency clouds.

## Core semantic model: zone/entity/link/label/base plane/camera

- zone = a bounded architecture area on the base plane, with `id`, `label`, and numeric `bounds`.
- entity = a typed system object such as client, nginx, service, nacos, redis, mysql, oss, admin, or elasticsearch.
- link = a directed relationship between entities, with a short label, architecture-specific kind, arrow, and route.
- label = a top HTML billboard above every visible entity, with an optional leader line back to the 3D object.
- base plane = the pale isometric surface and grid that anchors zones and routes.
- camera = orthographic isometric view with constrained orbit; the first view must make zones, entities, and arrows readable.

## Required construction rules with zones/entities/links and no Studio panels/generic graph

1. Create `zones` first. Every meaningful area must have `bounds.x`, `bounds.y`, `bounds.w`, and `bounds.h`.
2. Create `entities` with `id`, short `label`, architecture `kind`, `zone`, and explicit `position`.
3. Create `links` with `id`, `from`, `to`, short `label`, specific `kind`, `directed: true`, and `routeStyle` or explicit `route`.
4. Use `theme: architecture_light` and `canvas.grid.enabled: true`.
5. Use `visual.assumptions` for placement choices or inferred behavior.
6. Do not create `panels`, `hero`, `nav`, `story`, or Studio walkthrough content.
7. Do not use generic `nodes`/`edges`; use `entities`/`links`.
8. Do not rely on generic graph shapes or fallback spheres. Entity `kind` drives generated architecture geometry.

## Entity kind guide

Use these exact kinds where possible: `client`, `pc`, `mobile`, `cdn`, `gateway`, `nginx`, `api_gateway`, `service`, `microservice`, `registry`, `nacos`, `admin`, `database`, `mysql`, `cache`, `redis`, `storage`, `oss`, `file_storage`, `block_storage`, `queue`, `log`, `elasticsearch`, `security`, `external`.

## Link kind guide

Use these exact link kinds where possible: `api_call`, `static_resource`, `reverse_proxy`, `load_balancing`, `register`, `registering_service`, `pull_consumption`, `health_check`, `data_cache`, `data_storage`, `replication`, `distributed_file_service`, `feign_call`, `log_collection`.

## Layout guide with conventional placement and assumptions in visual.assumptions

Place client-facing zones on the left/front, edge/CDN/nginx next, API gateway before service clusters, registry/admin above or behind services, cache/database/storage on the lower or right side, and observability near the outbound log path. Keep high-traffic routes separated by explicit orthogonal `route` points when links would overlap.

If a source architecture omits exact positions, choose stable conventional placement and record it in `visual.assumptions`, for example: "Service instances are grouped by logical role because replica topology was not specified." Do not leave entities unpositioned and expect auto-layout to solve the view.

## Common mistakes

- No zones or zones without numeric bounds.
- Entities missing `kind` or `position`.
- Links missing `directed: true`, arrows, route style, or short labels.
- Long relationship labels that read like documentation sentences.
- Generic `nodes`/`edges` input copied from graph templates.
- Studio fields such as `panels`, `hero`, `navigation`, or `story`.
- Dark starfield, holographic, dust, or decorative background themes.
- Service-to-service calls crossing many entities without explicit bend points.

## Quality checklist

- `schema` is `efp.visual.input.isometric_architecture.v1`.
- `theme` is `architecture_light`.
- `canvas.grid.enabled` is true.
- Every zone has `id`, `label`, and numeric `bounds`.
- Every important entity has `id`, `label`, `kind`, `zone`, and `position`.
- Every visible entity can receive a top label and leader line.
- Every link has `directed: true`, specific `kind`, short `label`, and route style or explicit route.
- Routes avoid obvious overlaps in the first view.
- `visual.assumptions` explains inferred placement or grouped replicas.
- No `panel-grammar.md`, Studio panels, generic graph fallback, starfield, dust, or holographic effects.
