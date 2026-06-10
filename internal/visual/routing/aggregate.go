package routing

import (
	"fmt"
	"sort"
	"strings"
)

type displayRouteGroup struct {
	Key    string
	Routes []Route
	Links  []LinkModel
}

func AggregateDisplayRoutes(input Input, sourceRoutes []Route, lanes []BusLane, obstacles []RouteObstacle, opts Options) ([]Route, []Route, RouteMetrics) {
	if len(sourceRoutes) == 0 {
		return nil, nil, RouteMetrics{RouteColorConsistency: 1}
	}
	complex := len(input.Zones) >= 6 || len(input.Entities) >= 12 || len(sourceRoutes) >= 10
	if !complex {
		out := cloneRoutes(sourceRoutes)
		for i := range out {
			applyDisplayRouteDefaults(&out[i], input, []string{out[i].ID}, "entity", "entity_port")
		}
		metrics := displayRouteMetrics(input, out, nil)
		return out, nil, metrics
	}

	linkByID := linkMap(input.Links)
	groups := map[string]*displayRouteGroup{}
	order := []string{}
	hidden := []Route{}
	for _, route := range sourceRoutes {
		link := linkByID[route.ID]
		if link.ID == "" {
			link = LinkModel{ID: route.ID, From: route.From, To: route.To, Label: route.Label, Role: route.Role, PathGroup: route.PathGroup, Directed: true}
		}
		fromZone := entityZone(input, route.From)
		toZone := entityZone(input, route.To)
		if fromZone != "" && toZone != "" && fromZone == toZone {
			hidden = append(hidden, route)
			continue
		}
		scope := displayScope(route)
		key := routeGroupKey(route, fromZone, toZone, scope)
		if _, ok := groups[key]; !ok {
			groups[key] = &displayRouteGroup{Key: key}
			order = append(order, key)
		}
		groups[key].Routes = append(groups[key].Routes, route)
		groups[key].Links = append(groups[key].Links, link)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return routeGroupSortKey(groups[order[i]]) < routeGroupSortKey(groups[order[j]])
	})

	display := []Route{}
	for _, key := range order {
		group := groups[key]
		if len(group.Routes) == 0 {
			continue
		}
		route := buildDisplayRoute(input, group, lanes, obstacles, display, opts)
		display = append(display, route)
		if len(group.Routes) > 1 || route.RouteScope != "entity" {
			for _, source := range group.Routes {
				hidden = appendHiddenRoute(hidden, source)
			}
		}
	}
	metrics := displayRouteMetrics(input, display, hidden)
	return display, hidden, metrics
}

func buildDisplayRoute(input Input, group *displayRouteGroup, lanes []BusLane, obstacles []RouteObstacle, existing []Route, opts Options) Route {
	source := chooseRepresentativeRoute(group.Routes)
	link := chooseRepresentativeLink(group.Links, source)
	sourceIDs := sourceRouteIDs(group.Routes)
	route := source
	route.ID = displayRouteID(group, source)
	route.SourceEdgeIDs = sourceIDs
	route.DetailRouteIDs = sourceIDs
	route.FromEntity = source.From
	route.ToEntity = source.To
	route.FromZone = entityZone(input, source.From)
	route.ToZone = entityZone(input, source.To)
	route.RouteScope = displayScope(source)
	route.TerminalMode = "entity_port"
	if route.RouteScope != "entity" {
		route.TerminalMode = "zone_boundary"
		if route.RouteScope == "bundle" {
			route.TerminalMode = "bundle_spur"
		}
		displayLink := link
		displayLink.ID = route.ID
		displayLink.Label = displayRouteLabel(group.Links, source.PathGroup)
		displayLink.Role = source.Role
		displayLink.PathGroup = source.PathGroup
		displayLink.Directed = true
		if route.FromZone != "" && route.ToZone != "" {
			zoneRoute, ok := routeSingle(input, displayLink, lanes, obstacles, existing, opts, 0)
			if ok {
				route.Points = zoneRoute.Points
				route.Segments = zoneRoute.Segments
				route.LabelAnchor = zoneRoute.LabelAnchor
				route.FromPort = zoneRoute.FromPort
				route.ToPort = zoneRoute.ToPort
				route.BusLaneID = zoneRoute.BusLaneID
				route.BundleID = zoneRoute.BundleID
				route.Metrics = zoneRoute.Metrics
			}
		}
	}
	route.Label = displayRouteLabel(group.Links, route.PathGroup)
	route.Arrow = "forward"
	route.Style = RouteStyleFor(route.Role, route.PathGroup)
	route.BundleID = nonEmpty(route.BundleID, route.PathGroup)
	route.Segments = PolylineSegments(route.Points)
	route.SpurStart, route.SpurEnd = RouteSpurs(route.Points)
	route.LabelAnchor = RouteLabelAnchor(route.Points, route.ParallelOffset)
	applyDisplayRouteDefaults(&route, input, sourceIDs, route.RouteScope, route.TerminalMode)
	return route
}

func displayScope(route Route) string {
	if route.Role == "primary" && route.PathGroup == "gateway" && strings.Contains(strings.ToLower(route.FromZone), "client") && strings.Contains(strings.ToLower(route.ToZone), "edge") {
		return "zone"
	}
	if route.Role == "primary" && (route.PathGroup == "gateway" || route.PathGroup == "service") {
		return "entity"
	}
	if route.Role == "auxiliary" {
		return "zone"
	}
	switch route.PathGroup {
	case "registry", "cache", "data", "storage", "observability", "health":
		return "bundle"
	default:
		return "zone"
	}
}

func routeGroupKey(route Route, fromZone, toZone, scope string) string {
	if scope == "entity" {
		return strings.Join([]string{"entity", route.From, route.To, route.PathGroup, route.Role}, "|")
	}
	return strings.Join([]string{scope, fromZone, toZone, route.PathGroup, route.Role}, "|")
}

func routeGroupSortKey(group *displayRouteGroup) string {
	if group == nil || len(group.Routes) == 0 {
		return "z"
	}
	route := group.Routes[0]
	prefix := map[string]string{
		"gateway":       "01",
		"service":       "02",
		"registry":      "03",
		"cache":         "04",
		"data":          "05",
		"storage":       "06",
		"observability": "07",
		"health":        "08",
	}[route.PathGroup]
	if prefix == "" {
		prefix = "20"
	}
	return prefix + "|" + group.Key
}

func chooseRepresentativeRoute(routes []Route) Route {
	if len(routes) == 0 {
		return Route{}
	}
	best := routes[0]
	for _, route := range routes[1:] {
		if route.Role == "primary" && best.Role != "primary" {
			best = route
			continue
		}
		if PolylineLength(route.Points) > PolylineLength(best.Points) && best.Role != "primary" {
			best = route
		}
	}
	return best
}

func chooseRepresentativeLink(links []LinkModel, fallback Route) LinkModel {
	if len(links) == 0 {
		return LinkModel{ID: fallback.ID, From: fallback.From, To: fallback.To, Label: fallback.Label, Role: fallback.Role, PathGroup: fallback.PathGroup, Directed: true}
	}
	for _, link := range links {
		if strings.TrimSpace(link.Label) != "" {
			return link
		}
	}
	return links[0]
}

func displayRouteID(group *displayRouteGroup, fallback Route) string {
	if group == nil || len(group.Routes) <= 1 {
		return fallback.ID
	}
	route := group.Routes[0]
	fromZone := sanitizeRouteID(entityZoneFromRoutes(group.Routes, true))
	toZone := sanitizeRouteID(entityZoneFromRoutes(group.Routes, false))
	return fmt.Sprintf("display_%s_%s_%s_%s", sanitizeRouteID(route.PathGroup), sanitizeRouteID(fromZone), sanitizeRouteID(toZone), sanitizeRouteID(route.Role))
}

func entityZoneFromRoutes(routes []Route, from bool) string {
	if len(routes) == 0 {
		return ""
	}
	if from {
		if routes[0].FromZone != "" {
			return routes[0].FromZone
		}
		return routes[0].From
	}
	if routes[0].ToZone != "" {
		return routes[0].ToZone
	}
	return routes[0].To
}

func sanitizeRouteID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "route"
	}
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_", ":", "_")
	return replacer.Replace(value)
}

func sourceRouteIDs(routes []Route) []string {
	out := make([]string, 0, len(routes))
	seen := map[string]bool{}
	for _, route := range routes {
		id := strings.TrimSpace(route.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func displayRouteLabel(links []LinkModel, pathGroup string) string {
	for _, link := range links {
		label := strings.TrimSpace(link.Label)
		if label != "" && !isGenericLinkLabel(label) {
			return compactRouteLabel(label)
		}
	}
	switch pathGroup {
	case "gateway":
		return "API"
	case "service":
		return "Service"
	case "registry":
		return "Register"
	case "cache":
		return "Cache"
	case "data":
		return "Data"
	case "storage":
		return "Storage"
	case "observability":
		return "Logs"
	case "health":
		return "Health"
	default:
		return ""
	}
}

func compactRouteLabel(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 22 {
		return value
	}
	return strings.TrimSpace(value[:19]) + "..."
}

func isGenericLinkLabel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "link", "relationship", "depends_on", "depends":
		return true
	default:
		return false
	}
}

func RouteStyleFor(role, pathGroup string) RouteStyle {
	role = strings.ToLower(strings.TrimSpace(role))
	pathGroup = strings.ToLower(strings.TrimSpace(pathGroup))
	style := RouteStyle{
		Color:      "#475569",
		BodyColor:  "#475569",
		ArrowColor: "#475569",
		LabelColor: "#111827",
		Width:      0.007,
		Opacity:    0.58,
		CapStyle:   "integrated_arrow",
		JoinStyle:  "bevel",
	}
	switch role {
	case "primary":
		style.Color = "#111827"
		style.BodyColor = "#111827"
		style.ArrowColor = "#111827"
		style.Width = 0.014
		style.Opacity = 0.92
	case "auxiliary":
		style.Color = "#94a3b8"
		style.BodyColor = "#94a3b8"
		style.ArrowColor = "#94a3b8"
		style.Width = 0.0045
		style.Opacity = 0.36
		style.DashPattern = []float64{0.55, 0.30}
	}
	style.AccentColor = map[string]string{
		"gateway":       "#111827",
		"service":       "#2563eb",
		"registry":      "#14b8a6",
		"cache":         "#ef4444",
		"data":          "#2563eb",
		"storage":       "#16a34a",
		"observability": "#f97316",
		"health":        "#64748b",
	}[pathGroup]
	if style.AccentColor == "" {
		style.AccentColor = style.Color
	}
	return style
}

func applyDisplayRouteDefaults(route *Route, input Input, sourceIDs []string, scope, terminalMode string) {
	if route == nil {
		return
	}
	if route.SourceEdgeIDs == nil {
		route.SourceEdgeIDs = append([]string(nil), sourceIDs...)
	}
	if route.FromEntity == "" {
		route.FromEntity = route.From
	}
	if route.ToEntity == "" {
		route.ToEntity = route.To
	}
	if route.FromZone == "" {
		route.FromZone = entityZone(input, route.From)
	}
	if route.ToZone == "" {
		route.ToZone = entityZone(input, route.To)
	}
	if route.RouteScope == "" {
		route.RouteScope = scope
	}
	if route.TerminalMode == "" {
		route.TerminalMode = terminalMode
	}
	if route.Arrow == "" {
		route.Arrow = "forward"
	}
	if route.Label == "" {
		route.Label = route.PathGroup
	}
	if route.Style.Color == "" {
		route.Style = RouteStyleFor(route.Role, route.PathGroup)
	}
}

func displayRouteMetrics(input Input, display, hidden []Route) RouteMetrics {
	metrics := ValidateRoutes(input, display)
	metrics.SourceEdgeCount = len(input.Links)
	metrics.DisplayRouteCount = len(display)
	metrics.HiddenDetailRouteCount = len(hidden)
	for _, route := range display {
		if route.RouteScope == "zone" || route.RouteScope == "bundle" || route.TerminalMode == "zone_boundary" || route.TerminalMode == "bundle_spur" {
			metrics.RouteToZoneCount++
		} else {
			metrics.RouteToEntityCount++
		}
		if route.Label != "" {
			metrics.VisibleLinkLabelCount++
		}
		if !sameRouteStyleColor(route.Style) {
			metrics.RouteSameStyleMismatch++
		}
	}
	if len(display) > 0 {
		metrics.RouteColorConsistency = float64(len(display)-metrics.RouteSameStyleMismatch) / float64(len(display))
	}
	if metrics.RouteColorConsistency == 0 && metrics.RouteSameStyleMismatch == 0 {
		metrics.RouteColorConsistency = 1
	}
	return metrics
}

func sameRouteStyleColor(style RouteStyle) bool {
	color := strings.ToLower(strings.TrimSpace(style.Color))
	body := strings.ToLower(strings.TrimSpace(style.BodyColor))
	arrow := strings.ToLower(strings.TrimSpace(style.ArrowColor))
	if body == "" {
		body = color
	}
	if arrow == "" {
		arrow = color
	}
	return body == arrow
}

func linkMap(links []LinkModel) map[string]LinkModel {
	out := map[string]LinkModel{}
	for _, link := range links {
		out[link.ID] = link
	}
	return out
}

func entityZone(input Input, entityID string) string {
	for _, entity := range input.Entities {
		if entity.ID == entityID {
			return entity.Group
		}
	}
	return ""
}

func cloneRoutes(routes []Route) []Route {
	out := make([]Route, len(routes))
	copy(out, routes)
	return out
}

func appendHiddenRoute(hidden []Route, route Route) []Route {
	for _, item := range hidden {
		if item.ID == route.ID {
			return hidden
		}
	}
	return append(hidden, route)
}
