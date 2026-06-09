# Visual Isometric Architecture Renderer

This renderer turns Mermaid `architecture-beta` input into an offline Three.js isometric scene. The public input remains Mermaid; the renderer pipeline uses internal semantic, layout, routing, and component stages so complex architecture maps do not depend on ad hoc drawing code.

## Pipeline

1. `Mermaid` source is parsed into a `SemanticGraph` of groups, services, and directed links.
2. `ArchitectureMapLayoutEngine` ranks entities from directed edges, then applies semantic architecture-map placement for known zone types. Rank preserves the main left-to-right flow, while zone kind places registry, cache, database, storage, observability, and admin around the service area instead of forcing every group into a single pipeline.
3. `ArchitectureRoutingEngine semantic_heuristic_v4` generates a first-class `RoutePlan` with routes, bus lanes, obstacles, bundles, spurs, label anchors, and metrics.
4. `BusLanePlanner` assigns path groups to stable lanes: gateway, service, registry, data, cache, storage, health, and observability.
5. `ObstacleAwareRouter` builds a sparse Hanan grid from ports, obstacle sides, lane coordinates, and zone-adjacent clearances, then runs A* orthogonal routing with direction-aware bend cost.
6. `ParallelNudging` separates overlapping same-group routes when that does not introduce entity intersections or port violations.
7. `RipUpAndReroute` performs two deterministic repair rounds for the worst routes using the current route occupancy as crossing/overlap cost.
8. `RouteValidator` reports port hint violations, direction violations, endpoint-inside-entity errors, entity intersections, route crossings, path-group overlaps, parallel offsets, bus lanes, and bundle metrics.
9. Runtime renders `RoutePlan.routes` directly. It only falls back to runtime heuristics when an older artifact does not include `routePlan`.
10. Runtime builds a scene component tree:
   - `ZoneComponent` for ground areas and boundaries.
   - `EntityComponent` for body, badge, label, leader line, bbox, ports, and anchors.
   - `RelationComponent` for ground path, arrow cap, hit area, route metrics, and link label.
   - `HtmlLabelComponent` for entity, link, and zone labels.
   - `LeaderLineComponent` for world-space label leaders.

## Ground Paths

`GroundPathGeometryBuilder v5` owns relation geometry. It builds ground-decal path strips, integrated arrow terminal caps, hit areas, hover halo support, dash segment metrics, bundles, and parallel offsets. The default relation layer is `world_ground`; SVG relation paths are reserved for debug mode and CanvasTexture link labels are not the default.

Relation styles are role-driven:

- `primary`: main API/entry/gateway routes.
- `secondary`: data/cache/storage/registry/service routes.
- `auxiliary`: health, logs, observability, and replication routes.

Complex diagrams use `pathGroup` lanes such as `gateway`, `registry`, `data`, `cache`, `storage`, `health`, and `observability`. Same-group routes receive bundle and parallel-offset metadata so bus-style routes remain legible. `BusLanePlanner` keeps registry routes above the service area, data/cache/storage routes below or beside it, and health/observability routes on outer lanes when possible.

`basic.mmd` is a smoke fixture for parser, ranking, simple route ports, labels, and component ownership. `microservice-golden.mmd` is the visual quality fixture for architecture-map layout, bus lanes, route density, entity catalog coverage, camera fit, and manual screenshot review.

## Entity Catalog

The runtime body registry maps common architecture kinds to procedural bodies: browser/mobile/client, CDN, Nginx/gateway/API gateway, service/microservice, registry/Nacos, Redis/cache, database/MySQL, storage/OSS/file, observability/logs/Prometheus/Grafana, and admin. Unknown kinds still render with a safe generic body, and `inspect-browser` reports known/generic body counts and ratio.

## Browser Evidence

`visual inspect-browser` exposes component and visual metrics:

- component counts for entities, relations, labels, leaders, and path builder.
- route metrics for crossings, bus lanes, bundles, port hints, direction violations, and entity intersections.
- path builder metrics for version, join style, arrow cap count, hit area count, hover halo support, and parallel offsets.
- entity body registry count, known/generic body counts, and generic body ratio.
- camera/label fit metrics including labels outside the stage or under toolbar/inspector.
- complex-map metrics including route crossings, path-group overlaps, bus lane count, bundle count, route entity intersections, and semantic body score.
- route-plan metrics including `route_plan_present`, `route_plan_route_count`, `route_plan_lane_count`, `route_plan_obstacle_count`, and whether rendered relation components match the plan.

These checks are deterministic DOM/runtime checks, not OCR or AI visual scoring. Human screenshot review is still required for final visual quality.

## Route Plan

Architecture routing is an internal deterministic stage, not a pluggable third-party backend. The renderer uses a serialized `RoutePlan` so route quality can be inspected independently from drawing code:

```json
{
  "version": "efp.routeplan.v1",
  "backend": "semantic_heuristic_v4",
  "routes": [],
  "lanes": [],
  "bundles": [],
  "obstacles": [],
  "metrics": {}
}
```

The `backend` field is a legacy JSON key that currently contains the internal route engine name, `semantic_heuristic_v4`. It is not an extension point. The browser runtime renders `routePlan.routes` and only falls back to local runtime heuristics for older artifacts that do not contain a route plan.

The internal engine uses general routing algorithms that match the needs of architecture maps:

- semantic zone placement instead of pure graph layering.
- fixed entity ports from Mermaid `R/L/T/B` hints.
- bus lanes for gateway, registry, data, cache, storage, health, and observability paths.
- source/target spurs, bundles, and parallel offsets for same-group routes.
- sparse Hanan grid visibility graph from ports, obstacles, and lane coordinates.
- A* orthogonal routing scored by length, bends, crossings, overlaps, entity intersections, port violations, lane violations, and preferred-lane rewards.
- parallel nudging accepted only when it does not worsen intersections, port violations, or overlap metrics.
- rip-up/reroute repair rounds for the highest-cost routes.
- route validation metrics surfaced in `inspect-browser`.

This keeps the CLI offline, dependency-light, and fully under project control. Third-party routing engines are intentionally not integrated because they add packaging and mapping complexity without matching the current semantic architecture-map requirements closely enough.

### Commands

Generate the internal RoutePlan:

```bash
visual route-plan \
  --input ./templates/visual/mermaid.architecture/examples/microservice-golden.mmd \
  --out ./out/routeplan.json \
  --json
```
