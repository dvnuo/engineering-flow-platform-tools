# Visual Isometric Architecture Renderer

This renderer turns Mermaid `architecture-beta` input into an offline Three.js isometric scene. The public input remains Mermaid; the renderer pipeline uses internal semantic, layout, routing, and component stages so complex architecture maps do not depend on ad hoc drawing code.

## Pipeline

1. `Mermaid` source is parsed into a `SemanticGraph` of groups, services, and directed links.
2. `ArchitectureMapLayoutEngine` ranks entities from directed edges, then applies semantic architecture-map placement for known zone types. Rank preserves the main left-to-right flow, while zone kind places registry, cache, database, storage, observability, and admin around the service area instead of forcing every group into a single pipeline.
3. `ArchitectureRoutingEngine v2` resolves path groups, roles, bus lanes, parallel offsets, and route points.
4. `RouteValidator` reports port hint violations, direction violations, entity intersections, route crossings, and bundle metrics.
5. Runtime builds a scene component tree:
   - `ZoneComponent` for ground areas and boundaries.
   - `EntityComponent` for body, badge, label, leader line, bbox, ports, and anchors.
   - `RelationComponent` for ground path, arrow cap, hit area, route metrics, and link label.
   - `HtmlLabelComponent` for entity, link, and zone labels.
   - `LeaderLineComponent` for world-space label leaders.

## Ground Paths

`GroundPathGeometryBuilder v3` owns relation geometry. It builds ground-decal path strips, arrow terminal caps, hit areas, hover halo support, dash patterns, and parallel offsets. The default relation layer is `world_ground`; SVG relation paths are reserved for debug mode and CanvasTexture link labels are not the default.

Relation styles are role-driven:

- `primary`: main API/entry/gateway routes.
- `secondary`: data/cache/storage/registry/service routes.
- `auxiliary`: health, logs, observability, and replication routes.

Complex diagrams use `pathGroup` lanes such as `gateway`, `registry`, `data`, `cache`, `storage`, `health`, and `observability`. Same-group routes may receive parallel offsets so bundled routes remain legible. `BusLaneRouter` keeps registry routes above the service area, data/cache/storage routes below or beside it, and health/observability routes on outer lanes when possible.

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

These checks are deterministic DOM/runtime checks, not OCR or AI visual scoring. Human screenshot review is still required for final visual quality.
