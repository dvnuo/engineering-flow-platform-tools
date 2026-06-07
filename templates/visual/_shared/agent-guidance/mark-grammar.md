# Visual Mark Grammar

Visual inputs should describe what each object and relationship means, not only where it appears. The renderer uses these semantic fields to choose shape, icon, generated model badge, mesh, color, arrow style, and legend entries.

## Objects

Use `kind`, `provider`, `service`, `platform`, and `presentation` to encode the visual mark.

Priority:

1. `presentation.mesh` or `presentation.shape`
2. `presentation.model`
3. `presentation.icon`
4. `provider + service`
5. `platform`
6. `kind`
7. `group`
8. fallback

Recommended object fields:

```json
{
  "id": "orders-api",
  "label": "Orders API",
  "kind": "api",
  "provider": "aws",
  "service": "api_gateway",
  "platform": "aws",
  "presentation": {
    "shape": "hex_service",
    "mesh": "hex_prism",
    "icon": "aws.api_gateway",
    "model": "nginx.logo3d",
    "color": "#ff9900",
    "depth": 0.4,
    "lane": "backend"
  }
}
```

Do not rely on generic sphere nodes for semantic entities. Use `kind`, `provider`, `service`, `platform`, `presentation.shape`, `presentation.icon`, or `presentation.model` so service boxes, databases, queues, actors, decisions, warnings, and external systems render differently.

`presentation.icon` and `presentation.model` must be local IDs from `asset-registry.json`. Never put external image/model URLs in visual input. If a logo is not in the registry, use a generic fallback ID or add it through the build-time asset pipeline first. Generated `*.logo3d` models are lightweight local GLB badges derived from local SVG source icons; they are not official vendor 3D models.

Legacy fields remain supported:

- `color` maps to `presentation.color`.
- `depth` maps to `presentation.depth`.
- `lane_index` maps to `presentation.laneIndex`.
- `label_priority` maps to `labelPriority`.

## Relationships

Use `kind`, `directed`, and `presentation` to encode direction and line style.

Recommended edge fields:

```json
{
  "from": "orders-api",
  "to": "orders-db",
  "kind": "writes",
  "directed": true,
  "presentation": {
    "arrow": "forward",
    "lineStyle": "solid",
    "curve": "arc",
    "flow": true,
    "color": "#35c2a1"
  }
}
```

Directed defaults are inferred for causal or data movement relationships such as `calls`, `writes`, `reads`, `emits`, `subscribes`, `deploys`, `validates`, `blocks`, `depends_on`, `sends`, and `returns`, but agents should still set `directed` and `presentation.arrow` when direction matters.

Legacy edge fields remain supported:

- `curve` maps to `presentation.curve`.
- `color` maps to `presentation.color`.

## Sequence Messages

Sequence messages are directed by default. Still provide `kind`, `phase`, and optional `presentation` to improve arrow and flow encoding:

```json
{
  "id": "m1",
  "from": "browser",
  "to": "api",
  "kind": "sync",
  "phase": "request",
  "directed": true,
  "presentation": {
    "arrow": "forward",
    "lineStyle": "solid",
    "flow": true
  }
}
```

## Color And Legend

Use `view.colorBy` or `renderHints.colorBy` to explain color meaning. Use `renderHints.showLegend=true` when color encodes kind, provider, status, group, phase, risk, or severity.

Color priority:

1. `presentation.color`
2. `phases[].color` for sequence messages
3. provider/service color from the mark registry
4. kind palette
5. group palette
6. status palette
7. fallback palette

Recommended:

```json
{
  "view": {
    "colorBy": "provider"
  },
  "renderHints": {
    "palette": "cloud_provider",
    "showLegend": true,
    "iconMode": "billboard"
  }
}
```

If all marks resolve to the same fallback color, `visual inspect-input` reports `single_color_detected`. If `colorBy` is used without a usable legend, it reports `legend_missing`. If local asset IDs are wrong or remote URLs are used, it reports `asset_icon_unknown`, `asset_model_missing`, `asset_remote_url_forbidden`, `asset_registry_path_missing`, or `asset_vendor_attribution_missing`.
