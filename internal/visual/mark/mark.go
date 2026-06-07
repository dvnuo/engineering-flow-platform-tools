package mark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MarkRegistry struct {
	Defaults  map[string]MarkSpec     `json:"defaults"`
	Kinds     map[string]MarkSpec     `json:"kinds"`
	Providers map[string]MarkSpec     `json:"providers"`
	Platforms map[string]MarkSpec     `json:"platforms"`
	EdgeKinds map[string]EdgeKindSpec `json:"edge_kinds"`
}

type MarkSpec struct {
	Shape        string `json:"shape"`
	Mesh         string `json:"mesh"`
	Icon         string `json:"icon"`
	IconFallback string `json:"iconFallback"`
	Color        string `json:"color"`
}

type EdgeKindSpec struct {
	Directed  bool   `json:"directed"`
	Arrow     string `json:"arrow"`
	LineStyle string `json:"lineStyle"`
	Flow      bool   `json:"flow"`
	Color     string `json:"color"`
}

type AssetRegistry struct {
	Icons        map[string]AssetEntry `json:"icons"`
	Models       map[string]AssetEntry `json:"models"`
	Attributions []Attribution         `json:"attributions"`
}

type AssetEntry struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Official      bool   `json:"official"`
	AttributionID string `json:"attribution_id"`
}

type Attribution struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	License string `json:"license,omitempty"`
	Source  string `json:"source,omitempty"`
}

type Stats struct {
	ShapeCounts         map[string]int `json:"shape_counts"`
	IconCounts          map[string]int `json:"icon_counts"`
	FallbackSphereCount int            `json:"fallback_sphere_count"`
	NodeCount           int            `json:"node_count"`
	EdgeCount           int            `json:"edge_count"`
	DirectedCount       int            `json:"directed_count"`
	ArrowCount          int            `json:"arrow_count"`
	UndirectedCount     int            `json:"undirected_count"`
	ColorBy             string         `json:"colorBy,omitempty"`
	LegendItems         []LegendItem   `json:"legend_items"`
	SingleColor         bool           `json:"single_color"`
	IconsUsed           []string       `json:"icons_used"`
	MissingIcons        []string       `json:"missing_icons"`
	Attributions        []Attribution  `json:"attributions"`
	Warnings            []Warning      `json:"warnings"`
}

type LegendItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int    `json:"count"`
	Color string `json:"color,omitempty"`
}

type Warning struct {
	Code        string         `json:"code"`
	Severity    string         `json:"severity"`
	Path        string         `json:"path,omitempty"`
	Message     string         `json:"message"`
	Suggestion  string         `json:"suggestion"`
	AutoFixHint map[string]any `json:"auto_fix_hint,omitempty"`
	Details     []string       `json:"details,omitempty"`
}

type nodeItem struct {
	Path string
	Obj  map[string]any
}

type edgeItem struct {
	Path string
	Obj  map[string]any
}

type resolvedMark struct {
	Shape        string
	Mesh         string
	Icon         string
	Color        string
	Fallback     bool
	UnknownRef   string
	ExplicitIcon string
}

func Analyze(templateDir, inputSchemaKind string, data map[string]any) Stats {
	registry := loadMarkRegistry(templateDir)
	assets := loadAssetRegistry(templateDir)
	stats := Stats{
		ShapeCounts: map[string]int{},
		IconCounts:  map[string]int{},
		ColorBy:     colorBy(data),
	}
	if stats.ColorBy == "" && strings.ToLower(inputSchemaKind) == "uml_sequence_v1" {
		stats.ColorBy = "phase"
	}
	nodes := collectNodes(inputSchemaKind, data)
	edges := collectEdges(inputSchemaKind, data)
	stats.NodeCount = len(nodes)
	stats.EdgeCount = len(edges)
	iconSeen := map[string]bool{}
	missingIconSeen := map[string]bool{}
	unknownProviderSeen := map[string]bool{}
	providerAttributionMissing := map[string]bool{}
	colorSeen := map[string]int{}
	fallbackColors := 0
	for _, node := range nodes {
		resolved := resolveNode(registry, node.Obj)
		stats.ShapeCounts[resolved.Shape]++
		if resolved.Fallback {
			stats.FallbackSphereCount++
		}
		if resolved.Icon != "" {
			stats.IconCounts[resolved.Icon]++
			iconSeen[resolved.Icon] = true
			asset, ok := assets.Icons[resolved.Icon]
			if !ok {
				missingIconSeen[resolved.Icon] = true
			} else if isProviderIcon(resolved.Icon) && strings.TrimSpace(asset.AttributionID) == "" {
				providerAttributionMissing[resolved.Icon] = true
			}
		}
		if resolved.ExplicitIcon != "" {
			if _, ok := assets.Icons[resolved.ExplicitIcon]; !ok {
				missingIconSeen[resolved.ExplicitIcon] = true
			}
		}
		if resolved.UnknownRef != "" {
			unknownProviderSeen[resolved.UnknownRef] = true
		}
		if resolved.Color == fallbackColor() {
			fallbackColors++
		}
		colorSeen[resolved.Color]++
	}
	for _, edge := range edges {
		spec := resolveEdge(registry, inputSchemaKind, edge.Obj)
		if spec.Directed {
			stats.DirectedCount++
		} else {
			stats.UndirectedCount++
		}
		if spec.Arrow != "" && spec.Arrow != "none" {
			stats.ArrowCount++
		}
		if spec.Color == fallbackColor() {
			fallbackColors++
		}
		colorSeen[spec.Color]++
		if strings.ToLower(inputSchemaKind) != "uml_sequence_v1" && needsDirection(edge.Obj) && !hasEdgeDirection(edge.Obj) {
			stats.Warnings = append(stats.Warnings, Warning{
				Code:       "edge_direction_missing",
				Severity:   "warning",
				Path:       edge.Path,
				Message:    "A directional relationship kind does not declare directed=true or presentation.arrow.",
				Suggestion: "Set directed=true and presentation.arrow=forward for calls, reads, writes, emits, subscribes, deploys, validates, blocks, depends_on, sends, or returns relationships.",
				AutoFixHint: map[string]any{
					"action": "add_edge_direction",
					"path":   edge.Path,
				},
			})
			stats.Warnings = append(stats.Warnings, Warning{
				Code:       "arrow_encoding_missing",
				Severity:   "info",
				Path:       edge.Path + ".presentation.arrow",
				Message:    "The edge direction is semantically important but no arrow encoding was provided.",
				Suggestion: "Add presentation.arrow=forward and presentation.flow=true when the relationship represents movement or causality.",
				AutoFixHint: map[string]any{
					"action": "add_arrow_encoding",
					"path":   edge.Path + ".presentation",
				},
			})
		}
	}
	if shapeQualityApplies(inputSchemaKind) && stats.NodeCount > 8 && stats.FallbackSphereCount*100 >= stats.NodeCount*80 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "generic_sphere_overuse",
			Severity:   "warning",
			Message:    "Most nodes do not provide semantic mark fields, so the renderer falls back to generic sphere nodes.",
			Suggestion: "Add kind, provider, service, platform, or presentation.shape/presentation.mesh to important nodes.",
			AutoFixHint: map[string]any{
				"action": "add_node_mark_fields",
				"fields": []string{"nodes[].kind", "nodes[].provider", "nodes[].service", "nodes[].presentation.shape"},
			},
		})
	}
	if shapeQualityApplies(inputSchemaKind) && stats.NodeCount > 0 && stats.FallbackSphereCount > 0 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "mark_shape_missing",
			Severity:   "info",
			Message:    "Some nodes have no resolvable shape and use the fallback sphere.",
			Suggestion: "Set presentation.shape or use specific kind/provider/service values for nodes that should look like services, databases, queues, actors, decisions, or warnings.",
			AutoFixHint: map[string]any{
				"action": "add_shape_or_kind",
			},
		})
	}
	if len(unknownProviderSeen) > 0 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "provider_service_unknown",
			Severity:   "warning",
			Message:    "Some provider/service/platform values do not exist in the mark registry.",
			Suggestion: "Use known provider IDs such as aws.lambda, aws.s3, aws.rds, aws.sqs, aws.eventbridge, aws.api_gateway, or platform jenkins; otherwise provide presentation.shape and presentation.icon.",
			Details:    sortedKeys(unknownProviderSeen),
			AutoFixHint: map[string]any{
				"action": "use_known_provider_service_or_shape",
			},
		})
	}
	if len(missingIconSeen) > 0 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "asset_icon_unknown",
			Severity:   "warning",
			Message:    "Some presentation.icon values are not present in the local asset registry.",
			Suggestion: "Use icon IDs from templates/visual/_shared/asset-registry.json or omit presentation.icon so the renderer can use a generic fallback.",
			Details:    sortedKeys(missingIconSeen),
			AutoFixHint: map[string]any{
				"action": "replace_unknown_icon",
			},
		})
	}
	if len(providerAttributionMissing) > 0 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "provider_icon_without_attribution",
			Severity:   "warning",
			Message:    "A provider-styled icon is used without an attribution entry in the local asset registry.",
			Suggestion: "Add an attribution_id in asset-registry.json and list the attribution in assets/ATTRIBUTIONS.md.",
			Details:    sortedKeys(providerAttributionMissing),
			AutoFixHint: map[string]any{
				"action": "add_asset_attribution",
			},
		})
	}
	if stats.NodeCount+stats.EdgeCount > 8 {
		if len(colorSeen) <= 1 || fallbackColors == stats.NodeCount+stats.EdgeCount {
			stats.SingleColor = true
			stats.Warnings = append(stats.Warnings, Warning{
				Code:       "single_color_detected",
				Severity:   "warning",
				Message:    "The visual resolves to one color family, so shape and edge meaning are harder to scan.",
				Suggestion: "Use provider/service colors, status colors, phases[].color, or view.colorBy/renderHints.colorBy with a legend.",
				AutoFixHint: map[string]any{
					"action": "add_color_encoding",
				},
			})
		}
		if stats.ColorBy == "" {
			stats.Warnings = append(stats.Warnings, Warning{
				Code:       "color_encoding_missing",
				Severity:   "info",
				Message:    "The input does not declare what color means.",
				Suggestion: "Set view.colorBy or renderHints.colorBy to kind, provider, status, group, phase, risk, or severity.",
				AutoFixHint: map[string]any{
					"action": "add_colorBy",
					"path":   "$.view.colorBy",
				},
			})
		}
	}
	stats.LegendItems = buildLegendItems(data, nodes, edges, stats.ColorBy, registry)
	if stats.ColorBy != "" && !showLegend(data) {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "legend_missing",
			Severity:   "warning",
			Message:    "The input declares colorBy but does not request a visible legend.",
			Suggestion: "Set renderHints.showLegend=true so viewers can decode the color policy.",
			AutoFixHint: map[string]any{
				"action": "add_legend",
				"path":   "$.renderHints.showLegend",
			},
		})
	} else if stats.ColorBy == "" && showLegend(data) && stats.NodeCount+stats.EdgeCount > 8 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "legend_missing",
			Severity:   "warning",
			Message:    "The input requests a legend but does not declare what color means.",
			Suggestion: "Set view.colorBy or renderHints.colorBy to kind, provider, status, group, phase, risk, or severity.",
			AutoFixHint: map[string]any{
				"action": "add_colorBy_for_legend",
				"path":   "$.view.colorBy",
			},
		})
	} else if showLegend(data) && stats.ColorBy != "" && len(stats.LegendItems) == 0 {
		stats.Warnings = append(stats.Warnings, Warning{
			Code:       "legend_missing",
			Severity:   "warning",
			Message:    "The renderer cannot build legend items for the requested colorBy field.",
			Suggestion: "Use a colorBy field that exists on nodes or edges, such as kind, provider, status, group, or phase.",
			AutoFixHint: map[string]any{
				"action": "fix_colorBy",
			},
		})
	}
	stats.IconsUsed = sortedKeys(iconSeen)
	stats.MissingIcons = sortedKeys(missingIconSeen)
	stats.Attributions = usedAttributions(assets, iconSeen)
	return stats
}

func shapeQualityApplies(kind string) bool {
	switch strings.ToLower(kind) {
	case "graph_v1", "graph_events_v1", "matrix_v1", "studio_v1", "uml_class_v1", "uml_state_machine_v1", "uml_activity_v1", "uml_component_deployment_v1":
		return true
	case "timeline_v1", "evidence_v1":
		return true
	default:
		return false
	}
}

func loadMarkRegistry(templateDir string) MarkRegistry {
	var registry MarkRegistry
	path := filepath.Join(templateDir, "_shared", "mark-registry.json")
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &registry)
	}
	if registry.Kinds == nil {
		registry.Kinds = map[string]MarkSpec{}
	}
	if registry.Providers == nil {
		registry.Providers = map[string]MarkSpec{}
	}
	if registry.Platforms == nil {
		registry.Platforms = map[string]MarkSpec{}
	}
	if registry.EdgeKinds == nil {
		registry.EdgeKinds = map[string]EdgeKindSpec{}
	}
	return registry
}

func loadAssetRegistry(templateDir string) AssetRegistry {
	var registry AssetRegistry
	path := filepath.Join(templateDir, "_shared", "asset-registry.json")
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &registry)
	}
	if registry.Icons == nil {
		registry.Icons = map[string]AssetEntry{}
	}
	if registry.Models == nil {
		registry.Models = map[string]AssetEntry{}
	}
	return registry
}

func collectNodes(kind string, data map[string]any) []nodeItem {
	switch strings.ToLower(kind) {
	case "uml_sequence_v1":
		return objectItems(data, "participants")
	case "uml_class_v1":
		return objectItems(data, "classes")
	case "uml_state_machine_v1":
		return objectItems(data, "states")
	case "uml_activity_v1":
		return objectItems(data, "actions")
	case "uml_component_deployment_v1":
		out := objectItems(data, "deployments")
		out = append(out, objectItems(data, "components")...)
		return out
	case "timeline_v1":
		return objectItems(data, "events")
	case "matrix_v1":
		return objectItems(data, "items")
	case "studio_v1":
		heroData := studioHeroData(data)
		out := objectItemsFrom(heroData, "nodes", "$.hero.data.nodes")
		out = append(out, objectItemsFrom(heroData, "items", "$.hero.data.items")...)
		out = append(out, objectItemsFrom(heroData, "events", "$.hero.data.events")...)
		out = append(out, objectItemsFrom(heroData, "participants", "$.hero.data.participants")...)
		out = append(out, objectItems(data, "panels")...)
		return out
	case "evidence_v1":
		out := objectItems(data, "claims")
		out = append(out, objectItems(data, "sources")...)
		return out
	default:
		return objectItems(data, "nodes")
	}
}

func collectEdges(kind string, data map[string]any) []edgeItem {
	switch strings.ToLower(kind) {
	case "uml_sequence_v1":
		return edgeItems(data, "messages")
	case "uml_class_v1":
		return edgeItems(data, "relationships")
	case "uml_state_machine_v1":
		return edgeItems(data, "transitions")
	case "uml_activity_v1":
		return edgeItems(data, "flows")
	case "uml_component_deployment_v1":
		return edgeItems(data, "links")
	case "evidence_v1":
		return edgeItems(data, "links")
	case "studio_v1":
		heroData := studioHeroData(data)
		out := edgeItemsFrom(heroData, "edges", "$.hero.data.edges")
		out = append(out, edgeItemsFrom(heroData, "messages", "$.hero.data.messages")...)
		return out
	default:
		return edgeItems(data, "edges")
	}
}

func objectItems(data map[string]any, field string) []nodeItem {
	return objectItemsFrom(data, field, "$."+field)
}

func objectItemsFrom(data map[string]any, field, pathPrefix string) []nodeItem {
	raw, _ := data[field].([]any)
	out := make([]nodeItem, 0, len(raw))
	for i, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, nodeItem{Path: pathPrefix + "[" + intString(i) + "]", Obj: obj})
		}
	}
	return out
}

func edgeItems(data map[string]any, field string) []edgeItem {
	return edgeItemsFrom(data, field, "$."+field)
}

func edgeItemsFrom(data map[string]any, field, pathPrefix string) []edgeItem {
	raw, _ := data[field].([]any)
	out := make([]edgeItem, 0, len(raw))
	for i, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, edgeItem{Path: pathPrefix + "[" + intString(i) + "]", Obj: obj})
		}
	}
	return out
}

func studioHeroData(data map[string]any) map[string]any {
	hero, _ := data["hero"].(map[string]any)
	if hero == nil {
		return map[string]any{}
	}
	heroData, _ := hero["data"].(map[string]any)
	if heroData == nil {
		return map[string]any{}
	}
	return heroData
}

func resolveNode(registry MarkRegistry, obj map[string]any) resolvedMark {
	presentation := object(obj, "presentation")
	spec := MarkSpec{}
	fallback := true
	unknownRef := ""
	provider := normalize(firstString(obj, "provider"))
	service := normalize(firstString(obj, "service"))
	platform := normalize(firstString(obj, "platform"))
	kind := normalize(firstString(obj, "kind", "type", "stereotype"))
	if shape := firstString(presentation, "shape", "mesh"); shape != "" {
		spec.Shape = shape
		spec.Mesh = nonEmpty(firstString(presentation, "mesh"), shape)
		fallback = false
	}
	if icon := firstString(presentation, "icon"); icon != "" {
		spec.Icon = icon
		fallback = false
	}
	if provider != "" && service != "" {
		key := provider + "." + service
		if fromRegistry, ok := registry.Providers[key]; ok {
			spec = mergeSpec(spec, fromRegistry)
			fallback = false
		} else {
			unknownRef = key
		}
	} else if provider != "" {
		if fromRegistry, ok := registry.Platforms[provider]; ok {
			spec = mergeSpec(spec, fromRegistry)
			fallback = false
		} else if fromRegistry, ok := registry.Providers[provider]; ok {
			spec = mergeSpec(spec, fromRegistry)
			fallback = false
		} else {
			unknownRef = provider
		}
	}
	if platform != "" {
		if fromRegistry, ok := registry.Platforms[platform]; ok {
			spec = mergeSpec(spec, fromRegistry)
			fallback = false
		} else if unknownRef == "" {
			unknownRef = platform
		}
	}
	if kind != "" {
		if fromRegistry, ok := registry.Kinds[kind]; ok {
			spec = mergeSpec(spec, fromRegistry)
			fallback = false
		} else if alias := kindAlias(kind); alias != "" {
			if fromRegistry, ok := registry.Kinds[alias]; ok {
				spec = mergeSpec(spec, fromRegistry)
				fallback = false
			}
		}
	}
	if color := firstString(presentation, "color"); color != "" {
		spec.Color = color
	}
	if color := firstString(obj, "color"); color != "" && spec.Color == "" {
		spec.Color = color
	}
	if spec.Shape == "" {
		spec.Shape = "sphere"
	}
	if spec.Mesh == "" {
		spec.Mesh = spec.Shape
	}
	if spec.Icon == "" {
		spec.Icon = spec.IconFallback
	}
	if spec.Color == "" {
		spec.Color = statusColor(firstString(obj, "status"))
	}
	return resolvedMark{
		Shape:        spec.Shape,
		Mesh:         spec.Mesh,
		Icon:         spec.Icon,
		Color:        normalizeColor(spec.Color),
		Fallback:     fallback && spec.Shape == "sphere",
		UnknownRef:   unknownRef,
		ExplicitIcon: firstString(presentation, "icon"),
	}
}

func resolveEdge(registry MarkRegistry, kind string, obj map[string]any) EdgeKindSpec {
	presentation := object(obj, "presentation")
	edgeKind := normalize(firstString(obj, "kind", "relation", "type"))
	spec := EdgeKindSpec{}
	if fromRegistry, ok := registry.EdgeKinds[edgeKind]; ok {
		spec = fromRegistry
	}
	if strings.ToLower(kind) == "uml_sequence_v1" {
		spec.Directed = true
		if spec.Arrow == "" {
			spec.Arrow = "forward"
		}
	}
	if boolField(obj, "directed") {
		spec.Directed = true
	}
	if arrow := firstString(presentation, "arrow"); arrow != "" {
		spec.Arrow = normalize(arrow)
		spec.Directed = spec.Arrow != "none"
	}
	if line := firstString(presentation, "lineStyle", "line_style"); line != "" {
		spec.LineStyle = normalize(line)
	}
	if flow, ok := boolValue(presentation["flow"]); ok {
		spec.Flow = flow
	}
	if color := firstString(presentation, "color"); color != "" {
		spec.Color = color
	}
	if color := firstString(obj, "color"); color != "" && spec.Color == "" {
		spec.Color = color
	}
	if needsDirection(obj) && spec.Arrow == "" {
		spec.Arrow = "forward"
		spec.Directed = true
	}
	if spec.LineStyle == "" {
		spec.LineStyle = "solid"
	}
	if spec.Color == "" {
		spec.Color = statusColor(firstString(obj, "status"))
	}
	spec.Color = normalizeColor(spec.Color)
	return spec
}

func mergeSpec(base, next MarkSpec) MarkSpec {
	if base.Shape == "" {
		base.Shape = next.Shape
	}
	if base.Mesh == "" {
		base.Mesh = next.Mesh
	}
	if base.Icon == "" {
		base.Icon = next.Icon
	}
	if base.IconFallback == "" {
		base.IconFallback = next.IconFallback
	}
	if base.Color == "" {
		base.Color = next.Color
	}
	return base
}

func buildLegendItems(data map[string]any, nodes []nodeItem, edges []edgeItem, colorBy string, registry MarkRegistry) []LegendItem {
	if colorBy == "" {
		colorBy = "kind"
	}
	counts := map[string]int{}
	colors := map[string]string{}
	collect := func(obj map[string]any) {
		value := fieldForColorBy(obj, colorBy)
		if value == "" {
			return
		}
		counts[value]++
		if colors[value] == "" {
			colors[value] = colorForLegend(value, colorBy, registry)
		}
	}
	for _, node := range nodes {
		collect(node.Obj)
	}
	for _, edge := range edges {
		collect(edge.Obj)
	}
	if strings.ToLower(colorBy) == "phase" {
		for _, phase := range objectItems(data, "phases") {
			id := firstString(phase.Obj, "id", "label", "name")
			if id == "" {
				continue
			}
			counts[id]++
			if colors[id] == "" {
				colors[id] = normalizeColor(firstString(phase.Obj, "color"))
			}
		}
	}
	out := make([]LegendItem, 0, len(counts))
	for id, count := range counts {
		out = append(out, LegendItem{ID: id, Label: id, Count: count, Color: colors[id]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].ID < out[j].ID
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func usedAttributions(assets AssetRegistry, iconSeen map[string]bool) []Attribution {
	attributionByID := map[string]Attribution{}
	for _, attr := range assets.Attributions {
		if attr.ID != "" {
			attributionByID[attr.ID] = attr
		}
	}
	used := map[string]bool{}
	for icon := range iconSeen {
		if entry, ok := assets.Icons[icon]; ok && entry.AttributionID != "" {
			used[entry.AttributionID] = true
		}
	}
	var out []Attribution
	for id := range used {
		if attr, ok := attributionByID[id]; ok {
			out = append(out, attr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func colorBy(data map[string]any) string {
	for _, obj := range []map[string]any{object(data, "view"), object(data, "renderHints")} {
		if value := firstString(obj, "colorBy", "color_by"); value != "" {
			return normalize(value)
		}
	}
	return normalize(firstString(data, "colorBy", "color_by"))
}

func showLegend(data map[string]any) bool {
	renderHints := object(data, "renderHints")
	if value, ok := boolValue(renderHints["showLegend"]); ok {
		return value
	}
	if value, ok := boolValue(renderHints["show_legend"]); ok {
		return value
	}
	return false
}

func fieldForColorBy(obj map[string]any, colorBy string) string {
	switch normalize(colorBy) {
	case "provider":
		provider := normalize(firstString(obj, "provider", "platform"))
		service := normalize(firstString(obj, "service"))
		if provider != "" && service != "" {
			return provider + "." + service
		}
		return provider
	case "service":
		return normalize(firstString(obj, "service"))
	case "status":
		return normalize(firstString(obj, "status"))
	case "group":
		return normalize(firstString(obj, "group", "group_id", "parent_id", "module", "package", "lane"))
	case "phase":
		return normalize(firstString(obj, "phase"))
	default:
		return normalize(firstString(obj, colorBy, "kind", "type"))
	}
}

func colorForLegend(value, colorBy string, registry MarkRegistry) string {
	if strings.ToLower(colorBy) == "provider" {
		if spec, ok := registry.Providers[value]; ok && spec.Color != "" {
			return normalizeColor(spec.Color)
		}
	}
	if spec, ok := registry.EdgeKinds[value]; ok && spec.Color != "" {
		return normalizeColor(spec.Color)
	}
	if spec, ok := registry.Kinds[value]; ok && spec.Color != "" {
		return normalizeColor(spec.Color)
	}
	return stablePaletteColor(value)
}

func needsDirection(obj map[string]any) bool {
	kind := normalize(firstString(obj, "kind", "relation", "type"))
	directedKinds := map[string]bool{
		"call": true, "calls": true, "sync": true, "writes": true, "reads": true,
		"emits": true, "subscribes": true, "deploys": true, "deploys_to": true,
		"validates": true, "blocks": true, "depends_on": true, "sends": true,
		"returns": true, "async": true, "event": true, "observes": true,
		"supports": true, "refutes": true, "mentions": true,
	}
	return directedKinds[kind]
}

func hasEdgeDirection(obj map[string]any) bool {
	if _, ok := obj["directed"]; ok {
		return true
	}
	presentation := object(obj, "presentation")
	return firstString(presentation, "arrow") != ""
}

func kindAlias(kind string) string {
	aliases := map[string]string{
		"db": "database", "rds": "database", "dynamodb": "database",
		"bucket": "storage", "s3": "storage",
		"event_bus": "event_stream", "stream": "event_stream", "broker": "event_stream",
		"lambda": "service", "controller": "api", "endpoint": "api",
		"build": "job", "runner": "job", "deployment": "service",
		"external_provider": "external", "client": "user",
		"gate": "decision", "branch": "decision", "error": "risk",
	}
	return aliases[kind]
}

func statusColor(status string) string {
	switch normalize(status) {
	case "success", "supported", "ok":
		return "#47c477"
	case "warning", "retry":
		return "#e5a84c"
	case "error", "failed", "refuted":
		return "#ee6b73"
	case "blocked", "busy":
		return "#a77cff"
	default:
		return fallbackColor()
	}
}

func stablePaletteColor(value string) string {
	palette := []string{"#63a9ff", "#35c2a1", "#a166ff", "#e5a84c", "#ee6b73", "#cbd5e1"}
	if value == "" {
		return fallbackColor()
	}
	hash := 0
	for _, r := range value {
		hash = (hash*31 + int(r)) & 0x7fffffff
	}
	return palette[hash%len(palette)]
}

func fallbackColor() string {
	return "#63a9ff"
}

func isProviderIcon(icon string) bool {
	return strings.HasPrefix(icon, "aws.") || icon == "jenkins"
}

func sortedKeys(values map[string]bool) []string {
	var out []string
	for key := range values {
		if strings.TrimSpace(key) != "" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func object(data map[string]any, field string) map[string]any {
	obj, _ := data[field].(map[string]any)
	if obj == nil {
		return map[string]any{}
	}
	return obj
}

func firstString(obj map[string]any, names ...string) string {
	for _, name := range names {
		if value, ok := obj[name]; ok {
			if text := stringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func boolField(obj map[string]any, name string) bool {
	value, _ := boolValue(obj[name])
	return value
}

func boolValue(value any) (bool, bool) {
	v, ok := value.(bool)
	return v, ok
}

func normalize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func normalizeColor(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallbackColor()
	}
	if strings.HasPrefix(value, "#") {
		return strings.ToLower(value)
	}
	if len(value) == 6 {
		return "#" + strings.ToLower(value)
	}
	return stablePaletteColor(value)
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	n := value
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
