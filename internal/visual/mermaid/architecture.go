package mermaid

import (
	"fmt"
	"math"
	"sort"
	"strings"

	visualrouting "engineering-flow-platform-tools/internal/visual/routing"
)

type ArchitecturePoint struct {
	X float64
	Y float64
}

type ArchitectureBounds struct {
	X float64
	Y float64
	W float64
	H float64
}

type ArchitectureSemanticGraph struct {
	Title   string
	Kind    string
	Nodes   []Node
	NodeMap map[string]Node
	Groups  []Group
	Edges   []Edge
}

type ArchitectureEntityLayout struct {
	Node     Node
	GroupID  string
	Position ArchitecturePoint
	Bounds   ArchitectureBounds
	Rank     int
}

type ArchitectureZoneLayout struct {
	Group  Group
	Bounds ArchitectureBounds
}

type ArchitectureLayoutResult struct {
	Ranks      map[string]int
	Zones      []ArchitectureZoneLayout
	Entities   map[string]ArchitectureEntityLayout
	GroupNodes map[string][]Node
}

type ArchitectureRoutedLink struct {
	Edge           Edge
	Role           string
	PathGroup      string
	Route          []ArchitecturePoint
	Segments       []ArchitectureRouteSegment
	LabelAnchor    ArchitecturePoint
	LaneIndex      int
	ParallelOffset float64
	BundleID       string
	BusLaneID      string
	SpurStart      []ArchitecturePoint
	SpurEnd        []ArchitecturePoint
	Metrics        ArchitectureSingleRouteMetrics
}

type ArchitectureRoutingResult struct {
	Backend   string
	Links     []ArchitectureRoutedLink
	Lanes     []ArchitectureBusLane
	Obstacles []ArchitectureRouteObstacle
	Metrics   ArchitectureRouteMetrics
}

type ArchitectureRoutePlan struct {
	Version   string
	Backend   string
	Routes    []ArchitectureRoutedLink
	Lanes     []ArchitectureBusLane
	Obstacles []ArchitectureRouteObstacle
	Metrics   ArchitectureRouteMetrics
}

type ArchitectureBusLane struct {
	ID          string
	PathGroup   string
	Role        string
	Orientation string
	Points      []ArchitecturePoint
	Bounds      ArchitectureBounds
	Index       int
}

type ArchitectureRouteObstacle struct {
	ID       string
	EntityID string
	Kind     string
	Bounds   ArchitectureBounds
	Padding  float64
}

type ArchitectureRouteSegment struct {
	From ArchitecturePoint
	To   ArchitecturePoint
	Kind string
}

type ArchitectureSingleRouteMetrics struct {
	Length              float64
	BendCount           int
	EntityIntersections int
	Score               float64
}

type ArchitectureRouteMetrics struct {
	PortHintViolations      int
	DirectionViolations     int
	EntityIntersections     int
	EndpointInsideEntities  int
	CrossingCount           int
	ParallelOverlapCount    int
	BusLaneCount            int
	BundleCount             int
	LongDetourCount         int
	PrimaryRouteCount       int
	SecondaryRouteCount     int
	AuxiliaryRouteCount     int
	PathGroupOverlap        int
	ParallelOffsetCount     int
	RipUpRerouteRounds      int
	RipUpRerouteImprovement int
}

func BuildArchitectureSemanticGraph(d Diagram) ArchitectureSemanticGraph {
	nodes := sortedNodes(d)
	return ArchitectureSemanticGraph{
		Title:   d.Title,
		Kind:    d.Kind,
		Nodes:   nodes,
		NodeMap: nodeMapByID(nodes),
		Groups:  sortedGroups(d),
		Edges:   append([]Edge(nil), d.Edges...),
	}
}

func ArchitectureLayoutEngine(graph ArchitectureSemanticGraph) ArchitectureLayoutResult {
	return ArchitectureMapLayoutEngine(graph)
}

func ArchitectureMapLayoutEngine(graph ArchitectureSemanticGraph) ArchitectureLayoutResult {
	ranks := architectureRanks(graph.Nodes, graph.Edges)
	groups := architectureSortedGroups(Diagram{Groups: groupPointers(graph.Groups)}, graph.Nodes, ranks)
	if len(groups) == 0 {
		groups = []Group{{ID: "main-zone", Label: "MAIN ZONE", Kind: "zone"}}
	}
	complex := len(groups) > 6 || len(graph.Nodes) > 12
	naturalGroupNodes := architectureNodesByGroup(graph.Nodes, groups, ranks)
	naturalLayout := buildArchitectureMapLayout(groups, ranks, naturalGroupNodes, complex)
	if !complex {
		return naturalLayout
	}
	if architectureHasKnownMapZones(groups) {
		return naturalLayout
	}
	barycenterGroupNodes := cloneArchitectureGroupNodes(naturalGroupNodes)
	applyArchitectureBarycenterOrdering(graph, barycenterGroupNodes, ranks)
	barycenterLayout := buildArchitectureMapLayout(groups, ranks, barycenterGroupNodes, complex)
	if architectureLayoutRouteScore(graph, barycenterLayout) < architectureLayoutRouteScore(graph, naturalLayout) {
		return barycenterLayout
	}
	return naturalLayout
}

func architectureHasKnownMapZones(groups []Group) bool {
	hits := 0
	for _, group := range groups {
		key := strings.ToLower(group.ID + " " + group.Label + " " + group.Kind)
		for _, marker := range []string{"client", "edge", "gateway", "service", "registry", "cache", "database", "data", "storage", "observ", "admin"} {
			if strings.Contains(key, marker) {
				hits++
				break
			}
		}
	}
	return hits >= 5
}

func buildArchitectureMapLayout(groups []Group, ranks map[string]int, groupNodes map[string][]Node, complex bool) ArchitectureLayoutResult {
	zoneW := 7.4
	zoneH := 6.3
	zoneGap := 2.8
	if complex {
		zoneW = 8.8
		zoneH = 6.25
		zoneGap = 4.65
	}
	zoneYGap := 2.4
	if complex {
		zoneYGap = 3.25
	}
	zones := make([]ArchitectureZoneLayout, 0, len(groups))
	zoneByID := map[string]ArchitectureBounds{}
	for i, group := range groups {
		bounds := ArchitectureBounds{X: 0.5 + float64(i)*(zoneW+zoneGap), Y: 0.8, W: zoneW, H: zoneH}
		if complex {
			slot := preferredArchitectureZoneSlot(group, groupNodes[group.ID], ranks, i)
			bounds = ArchitectureBounds{
				X: 0.5 + slot.X*(zoneW+zoneGap),
				Y: 0.8 + slot.Y*(zoneH+zoneYGap),
				W: zoneW,
				H: zoneH,
			}
		}
		zoneByID[group.ID] = bounds
		zones = append(zones, ArchitectureZoneLayout{Group: group, Bounds: bounds})
	}
	entities := map[string]ArchitectureEntityLayout{}
	for _, group := range groups {
		members := groupNodes[group.ID]
		for i, node := range members {
			groupID := node.Group
			bounds, ok := zoneByID[groupID]
			if groupID == "" || !ok {
				groupID = group.ID
				bounds = zoneByID[groupID]
			}
			position := placeArchitectureEntityInGroup(group, bounds, i, len(members), complex)
			entities[node.ID] = ArchitectureEntityLayout{
				Node:     node,
				GroupID:  groupID,
				Position: position,
				Bounds:   bounds,
				Rank:     ranks[node.ID],
			}
		}
	}
	return ArchitectureLayoutResult{Ranks: ranks, Zones: zones, Entities: entities, GroupNodes: groupNodes}
}

func cloneArchitectureGroupNodes(input map[string][]Node) map[string][]Node {
	out := map[string][]Node{}
	for groupID, nodes := range input {
		out[groupID] = append([]Node(nil), nodes...)
	}
	return out
}

func architectureLayoutRouteScore(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult) float64 {
	type projectedEdge struct {
		edge      Edge
		from      ArchitecturePoint
		to        ArchitecturePoint
		role      string
		pathGroup string
	}
	projected := make([]projectedEdge, 0, len(graph.Edges))
	score := 0.0
	for _, edge := range graph.Edges {
		from, hasFrom := layout.Entities[edge.From]
		to, hasTo := layout.Entities[edge.To]
		if !hasFrom || !hasTo {
			score += 1000
			continue
		}
		role := inferArchitectureRole(edge, graph.NodeMap)
		pathGroup := inferArchitecturePathGroup(edge, graph.NodeMap)
		length := distancePoint(from.Position, to.Position)
		score += length * 0.05
		if role == "auxiliary" {
			score += length * 0.02
		}
		if architectureDirectionViolation(edge, from.Position, to.Position) {
			score += 5000
		}
		if pathGroup != "" && architectureStraightLaneViolation(from.Position, to.Position, pathGroup, layout) {
			score += 90
		}
		if routeEntityIntersectionsCandidate([]ArchitecturePoint{from.Position, to.Position}, layout, edge.From, edge.To) > 0 {
			score += 350
		}
		projected = append(projected, projectedEdge{edge: edge, from: from.Position, to: to.Position, role: role, pathGroup: pathGroup})
	}
	for i := 0; i < len(projected); i++ {
		for j := i + 1; j < len(projected); j++ {
			left := projected[i]
			right := projected[j]
			if architectureEdgesShareEndpoint(left.edge, right.edge) {
				continue
			}
			if left.role == "auxiliary" || right.role == "auxiliary" {
				continue
			}
			if left.pathGroup != "" && left.pathGroup == right.pathGroup {
				score += 8
				continue
			}
			if segmentsIntersect(left.from, left.to, right.from, right.to) {
				score += 160
			}
		}
	}
	return score
}

func architectureEdgesShareEndpoint(a, b Edge) bool {
	return a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To
}

func architectureStraightLaneViolation(from, to ArchitecturePoint, pathGroup string, layout ArchitectureLayoutResult) bool {
	if pathGroup == "gateway" || pathGroup == "service" {
		return false
	}
	serviceBounds, ok := architectureZoneBounds(layout, "service")
	if !ok {
		return false
	}
	midY := (from.Y + to.Y) / 2
	switch pathGroup {
	case "registry":
		return midY > serviceBounds.Y+serviceBounds.H*0.48
	case "data", "cache", "storage", "health", "observability":
		return midY < serviceBounds.Y+serviceBounds.H*0.36
	default:
		return false
	}
}

func ArchitectureRoutingEngine(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult) ArchitectureRoutingResult {
	input := architectureRoutingInput(graph, layout)
	plan := visualrouting.BuildRoutePlan(input, visualrouting.DefaultOptions())
	return architectureRoutingResultFromRoutePlan(graph, plan)
}

func architectureRoutingInput(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult) visualrouting.Input {
	entities := make([]visualrouting.EntityFrame, 0, len(layout.Entities))
	ids := make([]string, 0, len(layout.Entities))
	for id := range layout.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entity := layout.Entities[id]
		footprint := entityFootprint(entity.Position)
		entities = append(entities, visualrouting.EntityFrame{
			ID:     id,
			Kind:   inferEntityKind(entity.Node),
			Group:  entity.GroupID,
			Center: visualrouting.Vec2{X: entity.Position.X, Y: entity.Position.Y},
			Bounds: visualrouting.Rect{X: footprint.X, Y: footprint.Y, W: footprint.W, H: footprint.H},
			Rank:   entity.Rank,
		})
	}
	zones := make([]visualrouting.ZoneFrame, 0, len(layout.Zones))
	for _, zone := range layout.Zones {
		zones = append(zones, visualrouting.ZoneFrame{
			ID:     zone.Group.ID,
			Kind:   zone.Group.Kind,
			Label:  zone.Group.Label,
			Bounds: visualrouting.Rect{X: zone.Bounds.X, Y: zone.Bounds.Y, W: zone.Bounds.W, H: zone.Bounds.H},
		})
	}
	links := make([]visualrouting.LinkModel, 0, len(graph.Edges))
	for i, edge := range graph.Edges {
		role := inferArchitectureRole(edge, graph.NodeMap)
		pathGroup := inferArchitecturePathGroup(edge, graph.NodeMap)
		links = append(links, visualrouting.LinkModel{
			ID:        nonEmpty(edge.ID, fmt.Sprintf("link_%02d", i+1)),
			From:      edge.From,
			To:        edge.To,
			FromPort:  edge.FromPort,
			ToPort:    edge.ToPort,
			Label:     edge.Label,
			Kind:      edge.Kind,
			Role:      role,
			PathGroup: pathGroup,
			Directed:  edge.Directed,
		})
	}
	return visualrouting.Input{Entities: entities, Zones: zones, Links: links}
}

func architectureRoutingResultFromRoutePlan(graph ArchitectureSemanticGraph, plan visualrouting.RoutePlan) ArchitectureRoutingResult {
	edgeByRouteID := map[string]Edge{}
	for i, edge := range graph.Edges {
		edgeByRouteID[nonEmpty(edge.ID, fmt.Sprintf("link_%02d", i+1))] = edge
	}
	links := make([]ArchitectureRoutedLink, 0, len(plan.Routes))
	for _, route := range plan.Routes {
		edge, ok := edgeByRouteID[route.ID]
		if !ok {
			edge = Edge{ID: route.ID, From: route.From, To: route.To, Directed: true}
		}
		edge.FromPort = route.FromPort
		edge.ToPort = route.ToPort
		links = append(links, ArchitectureRoutedLink{
			Edge:           edge,
			Role:           route.Role,
			PathGroup:      route.PathGroup,
			Route:          architecturePointsFromRouting(route.Points),
			Segments:       architectureSegmentsFromRouting(route.Segments),
			LabelAnchor:    architecturePointFromRouting(route.LabelAnchor),
			LaneIndex:      route.LaneIndex,
			ParallelOffset: route.ParallelOffset,
			BundleID:       route.BundleID,
			BusLaneID:      route.BusLaneID,
			SpurStart:      architecturePointsFromRouting(route.SpurStart),
			SpurEnd:        architecturePointsFromRouting(route.SpurEnd),
			Metrics: ArchitectureSingleRouteMetrics{
				Length:              route.Metrics.Length,
				BendCount:           route.Metrics.BendCount,
				EntityIntersections: route.Metrics.EntityIntersections,
				Score:               route.Metrics.Score,
			},
		})
	}
	return ArchitectureRoutingResult{
		Backend:   plan.Backend,
		Links:     links,
		Lanes:     architectureLanesFromRouting(plan.Lanes),
		Obstacles: architectureObstaclesFromRouting(plan.Obstacles),
		Metrics:   architectureMetricsFromRouting(plan.Metrics),
	}
}

func architecturePointFromRouting(point visualrouting.Vec2) ArchitecturePoint {
	return ArchitecturePoint{X: point.X, Y: point.Y}
}

func architecturePointsFromRouting(points []visualrouting.Vec2) []ArchitecturePoint {
	out := make([]ArchitecturePoint, 0, len(points))
	for _, point := range points {
		out = append(out, architecturePointFromRouting(point))
	}
	return out
}

func architectureSegmentsFromRouting(segments []visualrouting.Segment) []ArchitectureRouteSegment {
	out := make([]ArchitectureRouteSegment, 0, len(segments))
	for _, segment := range segments {
		out = append(out, ArchitectureRouteSegment{
			From: architecturePointFromRouting(segment.From),
			To:   architecturePointFromRouting(segment.To),
			Kind: segment.Kind,
		})
	}
	return out
}

func architectureLanesFromRouting(lanes []visualrouting.BusLane) []ArchitectureBusLane {
	out := make([]ArchitectureBusLane, 0, len(lanes))
	for _, lane := range lanes {
		out = append(out, ArchitectureBusLane{
			ID:          lane.ID,
			PathGroup:   lane.PathGroup,
			Role:        lane.Role,
			Orientation: lane.Orientation,
			Points:      architecturePointsFromRouting(lane.Points),
			Bounds:      ArchitectureBounds{X: lane.Bounds.X, Y: lane.Bounds.Y, W: lane.Bounds.W, H: lane.Bounds.H},
			Index:       lane.Index,
		})
	}
	return out
}

func architectureObstaclesFromRouting(obstacles []visualrouting.RouteObstacle) []ArchitectureRouteObstacle {
	out := make([]ArchitectureRouteObstacle, 0, len(obstacles))
	for _, obstacle := range obstacles {
		out = append(out, ArchitectureRouteObstacle{
			ID:       obstacle.ID,
			EntityID: obstacle.EntityID,
			Kind:     obstacle.Kind,
			Bounds:   ArchitectureBounds{X: obstacle.Bounds.X, Y: obstacle.Bounds.Y, W: obstacle.Bounds.W, H: obstacle.Bounds.H},
			Padding:  obstacle.Padding,
		})
	}
	return out
}

func architectureMetricsFromRouting(metrics visualrouting.RouteMetrics) ArchitectureRouteMetrics {
	return ArchitectureRouteMetrics{
		PortHintViolations:      metrics.PortHintViolations,
		DirectionViolations:     metrics.DirectionViolations,
		EntityIntersections:     metrics.EntityIntersections,
		EndpointInsideEntities:  metrics.EndpointInsideEntities,
		CrossingCount:           metrics.CrossingCount,
		ParallelOverlapCount:    metrics.ParallelOverlapCount,
		BusLaneCount:            metrics.BusLaneCount,
		BundleCount:             metrics.BundleCount,
		LongDetourCount:         metrics.LongDetourCount,
		PrimaryRouteCount:       metrics.PrimaryRouteCount,
		SecondaryRouteCount:     metrics.SecondaryRouteCount,
		AuxiliaryRouteCount:     metrics.AuxiliaryRouteCount,
		PathGroupOverlap:        metrics.PathGroupOverlap,
		ParallelOffsetCount:     metrics.ParallelOffsetCount,
		RipUpRerouteRounds:      metrics.RipUpRerouteRounds,
		RipUpRerouteImprovement: metrics.RipUpRerouteImprovement,
	}
}

func architectureSemanticHeuristicRouting(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult) ArchitectureRoutingResult {
	out := make([]ArchitectureRoutedLink, 0, len(graph.Edges))
	lanes := BusLanePlanner(layout, graph.Edges)
	laneByGroup := map[string]ArchitectureBusLane{}
	for _, lane := range lanes {
		laneByGroup[lane.PathGroup] = lane
	}
	obstacles := architectureRouteObstacles(layout)
	groupCounters := map[string]int{}
	for _, edge := range graph.Edges {
		role := inferArchitectureRole(edge, graph.NodeMap)
		pathGroup := inferArchitecturePathGroup(edge, graph.NodeMap)
		laneIndex := groupCounters[pathGroup]
		groupCounters[pathGroup]++
		parallelOffset := architectureParallelOffset(laneIndex)
		from, fromOK := layout.Entities[edge.From]
		to, toOK := layout.Entities[edge.To]
		route := []ArchitecturePoint{}
		fromPort := ArchitecturePoint{}
		toPort := ArchitecturePoint{}
		if fromOK && toOK {
			fromPort, toPort = architectureRoutePorts(from, to, edge, role)
			route = obstacleAwareArchitectureRoute(fromPort, toPort, edge, pathGroup, laneIndex, parallelOffset, layout, laneByGroup[pathGroup])
		}
		segments := architectureRouteSegments(route)
		labelAnchor := architectureRouteLabelAnchor(route, parallelOffset)
		metrics := scoreArchitectureRoute(route, edge, pathGroup, layout)
		spurStart, spurEnd := architectureRouteSpurs(route)
		laneID := ""
		if lane, ok := laneByGroup[pathGroup]; ok {
			laneID = lane.ID
		}
		out = append(out, ArchitectureRoutedLink{
			Edge:           edge,
			Role:           role,
			PathGroup:      pathGroup,
			Route:          route,
			Segments:       segments,
			LabelAnchor:    labelAnchor,
			LaneIndex:      laneIndex,
			ParallelOffset: parallelOffset,
			BundleID:       pathGroup,
			BusLaneID:      laneID,
			SpurStart:      spurStart,
			SpurEnd:        spurEnd,
			Metrics:        metrics,
		})
	}
	return ArchitectureRoutingResult{Backend: RouteEngineSemanticHeuristicV4, Links: out, Lanes: lanes, Obstacles: obstacles, Metrics: ValidateArchitectureRoutes(graph, layout, out)}
}

func ValidateArchitectureRoutes(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult, links []ArchitectureRoutedLink) ArchitectureRouteMetrics {
	metrics := ArchitectureRouteMetrics{}
	for _, link := range links {
		switch link.Role {
		case "primary":
			metrics.PrimaryRouteCount++
		case "auxiliary":
			metrics.AuxiliaryRouteCount++
		default:
			metrics.SecondaryRouteCount++
		}
		from, fromOK := layout.Entities[link.Edge.From]
		to, toOK := layout.Entities[link.Edge.To]
		if !fromOK || !toOK {
			continue
		}
		if architectureDirectionViolation(link.Edge, from.Position, to.Position) {
			metrics.DirectionViolations++
			metrics.PortHintViolations++
		}
		if routeDetourRatio(link.Route, from.Position, to.Position) > 2.6 {
			metrics.LongDetourCount++
		}
		metrics.EntityIntersections += routeEntityIntersections(link, layout)
	}
	metrics.CrossingCount = routeCrossings(links)
	metrics.ParallelOverlapCount = routeParallelOverlaps(links)
	metrics.BusLaneCount, metrics.BundleCount = routeBusLaneMetrics(links)
	metrics.PathGroupOverlap = metrics.ParallelOverlapCount
	return metrics
}

func (layout ArchitectureLayoutResult) ToVisualZones() []any {
	zones := make([]any, 0, len(layout.Zones))
	for _, zone := range layout.Zones {
		b := zone.Bounds
		bounds := map[string]any{"x": b.X, "y": b.Y, "w": b.W, "h": b.H}
		zones = append(zones, map[string]any{
			"id":         zone.Group.ID,
			"label":      strings.ToUpper(nonEmpty(zone.Group.Label, zone.Group.ID)),
			"bounds":     bounds,
			"style":      "dashed",
			"importance": 0.7,
			"presentation": map[string]any{
				"boundary":      "dashed",
				"fill":          "#ffffff",
				"fillOpacity":   0.014,
				"color":         "#111827",
				"boundaryColor": "#111827",
				"cornerRadius":  0.78,
				"labelPoint":    map[string]any{"x": b.X + 0.74, "y": b.Y + b.H - 0.62},
			},
		})
	}
	return zones
}

func (layout ArchitectureLayoutResult) ToVisualEntities() []any {
	ids := make([]string, 0, len(layout.Entities))
	for id := range layout.Entities {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		left := layout.Entities[ids[i]]
		right := layout.Entities[ids[j]]
		if left.Rank != right.Rank {
			return left.Rank < right.Rank
		}
		return ids[i] < ids[j]
	})
	entities := make([]any, 0, len(ids))
	for _, id := range ids {
		item := layout.Entities[id]
		kind := inferEntityKind(item.Node)
		size := architectureEntityVisualSize(kind, len(ids))
		entities = append(entities, map[string]any{
			"id":            item.Node.ID,
			"label":         nonEmpty(item.Node.Label, item.Node.ID),
			"kind":          kind,
			"zone":          item.GroupID,
			"position":      map[string]any{"x": item.Position.X, "y": item.Position.Y},
			"size":          map[string]any{"w": size.W, "d": size.D, "h": size.H},
			"importance":    0.74,
			"labelPriority": "important",
			"presentation": map[string]any{
				"icon": iconForKind(kind),
			},
		})
	}
	return entities
}

type architectureEntitySize struct {
	W float64
	D float64
	H float64
}

func architectureEntityVisualSize(kind string, count int) architectureEntitySize {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if count > 18 {
		switch kind {
		case "api_gateway", "gateway", "nginx":
			return architectureEntitySize{W: 1.04, D: 0.78, H: 0.78}
		case "service", "microservice", "api":
			return architectureEntitySize{W: 0.92, D: 0.82, H: 0.76}
		case "registry", "nacos":
			return architectureEntitySize{W: 0.82, D: 0.78, H: 0.62}
		case "redis", "cache":
			return architectureEntitySize{W: 0.88, D: 0.82, H: 0.72}
		case "database", "mysql", "postgres", "mongodb":
			return architectureEntitySize{W: 0.9, D: 0.82, H: 0.72}
		case "storage", "oss", "file_storage", "block_storage":
			return architectureEntitySize{W: 0.88, D: 0.8, H: 0.7}
		case "cdn", "browser", "mobile", "client", "pc":
			return architectureEntitySize{W: 0.9, D: 0.8, H: 0.72}
		case "admin", "observability", "log", "logs", "elasticsearch", "prometheus", "grafana":
			return architectureEntitySize{W: 0.84, D: 0.76, H: 0.68}
		default:
			return architectureEntitySize{W: 0.9, D: 0.8, H: 0.72}
		}
	}
	if count > 10 {
		return architectureEntitySize{W: 1.18, D: 1.04, H: 0.92}
	}
	return architectureEntitySize{W: 1.55, D: 1.42, H: 1.18}
}

func (routing ArchitectureRoutingResult) ToVisualLinks() []any {
	links := make([]any, 0, len(routing.Links))
	for i, routed := range routing.Links {
		edge := routed.Edge
		route := make([]any, 0, len(routed.Route))
		for _, point := range routed.Route {
			route = append(route, map[string]any{"x": point.X, "y": point.Y})
		}
		linkItem := map[string]any{
			"id":            nonEmpty(edge.ID, fmt.Sprintf("link_%02d", i+1)),
			"from":          edge.From,
			"to":            edge.To,
			"from_port":     edge.FromPort,
			"to_port":       edge.ToPort,
			"kind":          nonEmpty(edge.Kind, "depends_on"),
			"directed":      edge.Directed,
			"role":          routed.Role,
			"pathGroup":     routed.PathGroup,
			"routeStyle":    "orthogonal",
			"route":         route,
			"importance":    importanceForRole(routed.Role),
			"labelPriority": labelPriorityForRole(routed.Role),
			"presentation": map[string]any{
				"arrow":          "forward",
				"lineStyle":      lineStyle(edge, routed.Role),
				"color":          colorForRole(routed.Role),
				"fromPort":       edge.FromPort,
				"toPort":         edge.ToPort,
				"laneIndex":      routed.LaneIndex,
				"parallelOffset": routed.ParallelOffset,
			},
			"metadata": map[string]any{
				"mermaid_from_port": edge.FromPort,
				"mermaid_to_port":   edge.ToPort,
				"route_stage":       "ArchitectureRoutingEngine",
				"lane_index":        routed.LaneIndex,
				"parallel_offset":   routed.ParallelOffset,
				"bundle_id":         routed.BundleID,
			},
		}
		if label := compact(edge.Label, 22); label != "" {
			linkItem["label"] = label
		}
		links = append(links, linkItem)
	}
	return links
}

func (routing ArchitectureRoutingResult) ToRoutePlan() ArchitectureRoutePlan {
	engine := routing.Backend
	if engine == "" {
		engine = RouteEngineSemanticHeuristicV4
	}
	return ArchitectureRoutePlan{
		Version:   "efp.routeplan.v1",
		Backend:   engine,
		Routes:    append([]ArchitectureRoutedLink(nil), routing.Links...),
		Lanes:     append([]ArchitectureBusLane(nil), routing.Lanes...),
		Obstacles: append([]ArchitectureRouteObstacle(nil), routing.Obstacles...),
		Metrics:   routing.Metrics,
	}
}

func (routing ArchitectureRoutingResult) ToVisualRoutePlan() map[string]any {
	return routing.ToRoutePlan().ToVisualRoutePlan()
}

func (plan ArchitectureRoutePlan) ToVisualRoutePlan() map[string]any {
	routes := make([]any, 0, len(plan.Routes))
	for i, routed := range plan.Routes {
		edge := routed.Edge
		routeID := nonEmpty(edge.ID, fmt.Sprintf("link_%02d", i+1))
		segments := make([]any, 0, len(routed.Segments))
		for _, segment := range routed.Segments {
			segments = append(segments, map[string]any{
				"from": pointMap(segment.From),
				"to":   pointMap(segment.To),
				"kind": segment.Kind,
			})
		}
		routes = append(routes, map[string]any{
			"id":             routeID,
			"from":           edge.From,
			"to":             edge.To,
			"role":           routed.Role,
			"pathGroup":      routed.PathGroup,
			"fromPort":       edge.FromPort,
			"toPort":         edge.ToPort,
			"points":         pointsMap(routed.Route),
			"segments":       segments,
			"busLaneId":      routed.BusLaneID,
			"bundleId":       routed.BundleID,
			"spurStart":      pointsMap(routed.SpurStart),
			"spurEnd":        pointsMap(routed.SpurEnd),
			"labelAnchor":    pointMap(routed.LabelAnchor),
			"laneIndex":      routed.LaneIndex,
			"parallelOffset": routed.ParallelOffset,
			"metrics": map[string]any{
				"length":               rounded(routed.Metrics.Length),
				"bend_count":           routed.Metrics.BendCount,
				"entity_intersections": routed.Metrics.EntityIntersections,
				"score":                rounded(routed.Metrics.Score),
			},
		})
	}
	lanes := make([]any, 0, len(plan.Lanes))
	for _, lane := range plan.Lanes {
		lanes = append(lanes, map[string]any{
			"id":          lane.ID,
			"pathGroup":   lane.PathGroup,
			"role":        lane.Role,
			"orientation": lane.Orientation,
			"points":      pointsMap(lane.Points),
			"bounds":      boundsMap(lane.Bounds),
			"index":       lane.Index,
		})
	}
	obstacles := make([]any, 0, len(plan.Obstacles))
	for _, obstacle := range plan.Obstacles {
		obstacles = append(obstacles, map[string]any{
			"id":        obstacle.ID,
			"entity_id": obstacle.EntityID,
			"kind":      obstacle.Kind,
			"bounds":    boundsMap(obstacle.Bounds),
			"padding":   obstacle.Padding,
		})
	}
	bundlesByID := map[string][]string{}
	for i, routed := range plan.Routes {
		if routed.BundleID == "" {
			continue
		}
		routeID := nonEmpty(routed.Edge.ID, fmt.Sprintf("link_%02d", i+1))
		bundlesByID[routed.BundleID] = append(bundlesByID[routed.BundleID], routeID)
	}
	bundleIDs := make([]string, 0, len(bundlesByID))
	for id := range bundlesByID {
		bundleIDs = append(bundleIDs, id)
	}
	sort.Strings(bundleIDs)
	bundles := make([]any, 0, len(bundleIDs))
	for _, id := range bundleIDs {
		bundles = append(bundles, map[string]any{
			"id":        id,
			"pathGroup": id,
			"route_ids": bundlesByID[id],
		})
	}
	backend := plan.Backend
	if backend == "" {
		backend = RouteEngineSemanticHeuristicV4
	}
	return map[string]any{
		"version":   "efp.routeplan.v1",
		"backend":   backend,
		"routes":    routes,
		"lanes":     lanes,
		"bundles":   bundles,
		"obstacles": obstacles,
		"metrics": map[string]any{
			"route_port_hint_violation_count":    plan.Metrics.PortHintViolations,
			"route_direction_violation_count":    plan.Metrics.DirectionViolations,
			"route_entity_intersection_count":    plan.Metrics.EntityIntersections,
			"route_endpoint_inside_entity_count": plan.Metrics.EndpointInsideEntities,
			"route_crossing_count":               plan.Metrics.CrossingCount,
			"route_parallel_overlap_count":       plan.Metrics.ParallelOverlapCount,
			"route_bus_lane_count":               plan.Metrics.BusLaneCount,
			"route_bundle_count":                 plan.Metrics.BundleCount,
			"route_long_detour_count":            plan.Metrics.LongDetourCount,
			"route_path_group_overlap_count":     plan.Metrics.PathGroupOverlap,
			"route_parallel_offset_count":        plan.Metrics.ParallelOffsetCount,
			"route_ripup_reroute_rounds":         plan.Metrics.RipUpRerouteRounds,
			"route_ripup_reroute_improvement":    plan.Metrics.RipUpRerouteImprovement,
			"primary_route_count":                plan.Metrics.PrimaryRouteCount,
			"secondary_route_count":              plan.Metrics.SecondaryRouteCount,
			"auxiliary_route_count":              plan.Metrics.AuxiliaryRouteCount,
		},
	}
}

func pointsMap(points []ArchitecturePoint) []any {
	out := make([]any, 0, len(points))
	for _, point := range points {
		out = append(out, pointMap(point))
	}
	return out
}

func pointMap(point ArchitecturePoint) map[string]any {
	return map[string]any{"x": rounded(point.X), "y": rounded(point.Y)}
}

func boundsMap(bounds ArchitectureBounds) map[string]any {
	return map[string]any{"x": rounded(bounds.X), "y": rounded(bounds.Y), "w": rounded(bounds.W), "h": rounded(bounds.H)}
}

func rounded(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func preferredArchitectureZoneSlot(group Group, members []Node, ranks map[string]int, fallbackIndex int) ArchitecturePoint {
	key := strings.ToLower(group.ID + " " + group.Label + " " + group.Kind)
	switch {
	case strings.Contains(key, "client"):
		return ArchitecturePoint{X: 0.05, Y: 0.08}
	case strings.Contains(key, "edge"):
		return ArchitecturePoint{X: 0.92, Y: 0.83}
	case strings.Contains(key, "gateway"):
		return ArchitecturePoint{X: 1.82, Y: 1.08}
	case strings.Contains(key, "service"), strings.Contains(key, "application"), strings.Contains(key, "app"):
		return ArchitecturePoint{X: 2.92, Y: 1.35}
	case strings.Contains(key, "registry"):
		return ArchitecturePoint{X: 4.05, Y: 0.22}
	case strings.Contains(key, "storage"):
		return ArchitecturePoint{X: 2.92, Y: 3.18}
	case strings.Contains(key, "cache"):
		return ArchitecturePoint{X: 0.52, Y: 2.00}
	case strings.Contains(key, "database"), strings.Contains(key, "data"):
		return ArchitecturePoint{X: 2.00, Y: 2.58}
	case strings.Contains(key, "observ"):
		return ArchitecturePoint{X: 3.75, Y: 3.88}
	case strings.Contains(key, "admin"):
		return ArchitecturePoint{X: 4.92, Y: 1.30}
	}
	minRank := 0
	if len(members) > 0 {
		minRank = ranks[members[0].ID]
		for _, member := range members {
			if ranks[member.ID] < minRank {
				minRank = ranks[member.ID]
			}
		}
	} else {
		minRank = fallbackIndex
	}
	return ArchitecturePoint{X: float64(minRank), Y: float64(fallbackIndex % 2)}
}

func applyArchitectureBarycenterOrdering(graph ArchitectureSemanticGraph, groupNodes map[string][]Node, ranks map[string]int) {
	if len(graph.Edges) == 0 {
		return
	}
	nodeGroup := map[string]string{}
	for groupID, nodes := range groupNodes {
		for _, node := range nodes {
			nodeGroup[node.ID] = groupID
		}
	}
	groupSlots := map[string]ArchitecturePoint{}
	for _, group := range graph.Groups {
		groupSlots[group.ID] = preferredArchitectureZoneSlot(group, groupNodes[group.ID], ranks, 0)
	}
	scores := map[string]float64{}
	for groupID, nodes := range groupNodes {
		for index, node := range nodes {
			scores[node.ID] = float64(ranks[node.ID])*0.18 + float64(index)*0.015
		}
		_ = groupID
	}
	for pass := 0; pass < 4; pass++ {
		nextScores := map[string]float64{}
		for id, score := range scores {
			nextScores[id] = score
		}
		for _, edge := range graph.Edges {
			group := inferArchitecturePathGroup(edge, graph.NodeMap)
			weight := architectureBarycenterPathWeight(group)
			bias := architectureBarycenterPathBias(group)
			if edge.From != "" {
				targetSlot := groupSlots[nodeGroup[edge.To]]
				nextScores[edge.From] += (bias + targetSlot.Y*0.16) * weight
				if group == "gateway" || group == "service" {
					nextScores[edge.From] -= 0.16
				}
			}
			if edge.To != "" {
				sourceSlot := groupSlots[nodeGroup[edge.From]]
				nextScores[edge.To] += (bias + sourceSlot.Y*0.10) * weight * 0.74
				if group == "gateway" || group == "service" {
					nextScores[edge.To] -= 1.65
				}
			}
		}
		scores = nextScores
	}
	for groupID := range groupNodes {
		sort.SliceStable(groupNodes[groupID], func(i, j int) bool {
			left := groupNodes[groupID][i]
			right := groupNodes[groupID][j]
			if math.Abs(scores[left.ID]-scores[right.ID]) > 0.001 {
				return scores[left.ID] < scores[right.ID]
			}
			if ranks[left.ID] != ranks[right.ID] {
				return ranks[left.ID] < ranks[right.ID]
			}
			return left.ID < right.ID
		})
	}
	enforceArchitecturePortOrder(graph, groupNodes, nodeGroup)
}

func architectureBarycenterPathBias(pathGroup string) float64 {
	switch pathGroup {
	case "gateway", "service":
		return 0.12
	case "registry":
		return 0.20
	case "cache":
		return 0.54
	case "data":
		return 0.62
	case "storage":
		return 0.78
	case "health", "observability":
		return 0.86
	default:
		return 0.48
	}
}

func architectureBarycenterPathWeight(pathGroup string) float64 {
	switch pathGroup {
	case "gateway", "service":
		return 2.0
	case "registry", "cache", "data", "storage":
		return 1.0
	case "health", "observability":
		return 0.22
	default:
		return 0.82
	}
}

func enforceArchitecturePortOrder(graph ArchitectureSemanticGraph, groupNodes map[string][]Node, nodeGroup map[string]string) {
	indexOf := func(nodes []Node, id string) int {
		for i, node := range nodes {
			if node.ID == id {
				return i
			}
		}
		return -1
	}
	moveBefore := func(nodes []Node, fromIndex, toIndex int) []Node {
		if fromIndex < 0 || toIndex < 0 || fromIndex < toIndex {
			return nodes
		}
		item := nodes[fromIndex]
		copy(nodes[toIndex+1:fromIndex+1], nodes[toIndex:fromIndex])
		nodes[toIndex] = item
		return nodes
	}
	moveAfter := func(nodes []Node, fromIndex, toIndex int) []Node {
		if fromIndex < 0 || toIndex < 0 || fromIndex > toIndex {
			return nodes
		}
		item := nodes[fromIndex]
		copy(nodes[fromIndex:toIndex], nodes[fromIndex+1:toIndex+1])
		nodes[toIndex] = item
		return nodes
	}
	for pass := 0; pass < 4; pass++ {
		changed := false
		for _, edge := range graph.Edges {
			groupID := nodeGroup[edge.From]
			if groupID == "" || groupID != nodeGroup[edge.To] {
				continue
			}
			nodes := groupNodes[groupID]
			fromIndex := indexOf(nodes, edge.From)
			toIndex := indexOf(nodes, edge.To)
			fromPort := strings.ToUpper(strings.TrimSpace(edge.FromPort))
			toPort := strings.ToUpper(strings.TrimSpace(edge.ToPort))
			next := nodes
			if fromPort == "R" && toPort == "L" && fromIndex > toIndex {
				next = moveBefore(nodes, fromIndex, toIndex)
				changed = true
			} else if fromPort == "L" && toPort == "R" && fromIndex < toIndex {
				next = moveAfter(nodes, fromIndex, toIndex)
				changed = true
			}
			groupNodes[groupID] = next
		}
		if !changed {
			break
		}
	}
}

func placeArchitectureEntityInGroup(group Group, bounds ArchitectureBounds, index, count int, complex bool) ArchitecturePoint {
	if !complex {
		return placeArchitectureEntity(bounds, index, count)
	}
	key := strings.ToLower(group.ID + " " + group.Label + " " + group.Kind)
	if strings.Contains(key, "client") {
		if count <= 1 {
			return ArchitecturePoint{X: bounds.X + bounds.W*0.5, Y: bounds.Y + bounds.H*0.58}
		}
		clientPositions := []ArchitecturePoint{
			{X: bounds.X + bounds.W*0.22, Y: bounds.Y + bounds.H*0.28},
			{X: bounds.X + bounds.W*0.24, Y: bounds.Y + bounds.H*0.62},
			{X: bounds.X + bounds.W*0.50, Y: bounds.Y + bounds.H*0.44},
			{X: bounds.X + bounds.W*0.72, Y: bounds.Y + bounds.H*0.24},
			{X: bounds.X + bounds.W*0.72, Y: bounds.Y + bounds.H*0.68},
		}
		if index < len(clientPositions) {
			return clientPositions[index]
		}
		return ArchitecturePoint{
			X: bounds.X + bounds.W*(0.23+0.27*float64(index%2)),
			Y: bounds.Y + bounds.H*(0.73-0.22*float64(index/2)),
		}
	}
	if strings.Contains(key, "service") || strings.Contains(key, "application") || strings.Contains(key, "app") {
		if count <= 6 {
			cols := 3
			col := index % cols
			row := index / cols
			return ArchitecturePoint{
				X: bounds.X + bounds.W*(0.20+0.30*float64(col)),
				Y: bounds.Y + bounds.H*(0.38+0.34*float64(row)),
			}
		}
		cols := 3
		col := index % cols
		row := index / cols
		return ArchitecturePoint{
			X: bounds.X + bounds.W*0.22 + float64(col)*bounds.W*0.29,
			Y: bounds.Y + bounds.H*0.70 - float64(row)*bounds.H*0.30,
		}
	}
	if strings.Contains(key, "registry") {
		if count == 3 {
			positions := []ArchitecturePoint{
				{X: bounds.X + bounds.W*0.25, Y: bounds.Y + bounds.H*0.58},
				{X: bounds.X + bounds.W*0.72, Y: bounds.Y + bounds.H*0.58},
				{X: bounds.X + bounds.W*0.50, Y: bounds.Y + bounds.H*0.82},
			}
			return positions[index%len(positions)]
		}
		return ArchitecturePoint{
			X: bounds.X + bounds.W*(0.23+0.31*float64(index%3)),
			Y: bounds.Y + bounds.H*(0.66-0.28*float64(index/3)),
		}
	}
	if strings.Contains(key, "cache") {
		if count == 3 {
			positions := []ArchitecturePoint{
				{X: bounds.X + bounds.W*0.24, Y: bounds.Y + bounds.H*0.52},
				{X: bounds.X + bounds.W*0.50, Y: bounds.Y + bounds.H*0.52},
				{X: bounds.X + bounds.W*0.76, Y: bounds.Y + bounds.H*0.52},
			}
			return positions[index%len(positions)]
		}
		return ArchitecturePoint{
			X: bounds.X + bounds.W*(0.22+0.30*float64(index%3)),
			Y: bounds.Y + bounds.H*(0.67-0.30*float64(index/3)),
		}
	}
	if strings.Contains(key, "observ") {
		if count <= 4 {
			step := 0.84
			if count > 1 {
				step = 0.84 / float64(count-1)
			}
			return ArchitecturePoint{
				X: bounds.X + bounds.W*(0.08+float64(index)*step),
				Y: bounds.Y + bounds.H*0.52,
			}
		}
		return ArchitecturePoint{
			X: bounds.X + bounds.W*(0.28+0.25*float64(index%2)),
			Y: bounds.Y + bounds.H*(0.66-0.30*float64(index/2)),
		}
	}
	return placeArchitectureEntity(bounds, index, count)
}

func placeArchitectureEntity(bounds ArchitectureBounds, index, count int) ArchitecturePoint {
	colCount := int(math.Ceil(math.Sqrt(float64(count))))
	if colCount < 1 {
		colCount = 1
	}
	col := index % colCount
	row := index / colCount
	rowCount := maxInt(2, int(math.Ceil(float64(count)/float64(colCount)))+1)
	cellW := bounds.W / float64(colCount+1)
	cellH := bounds.H / float64(rowCount)
	x := bounds.X + cellW*float64(col+1)
	y := bounds.Y + bounds.H*0.58 - cellH*float64(row)
	if count == 1 {
		x = bounds.X + bounds.W*0.5
		y = bounds.Y + bounds.H*0.54
	}
	return ArchitecturePoint{X: x, Y: y}
}

func BusLanePlanner(layout ArchitectureLayoutResult, edges []Edge) []ArchitectureBusLane {
	serviceBounds, hasService := architectureZoneBounds(layout, "service")
	if !hasService {
		serviceBounds = architectureAllZoneBounds(layout)
	}
	all := architectureAllZoneBounds(layout)
	serviceMidY := serviceBounds.Y + serviceBounds.H*0.5
	serviceTop := serviceBounds.Y
	serviceBottom := serviceBounds.Y + serviceBounds.H
	serviceLeft := serviceBounds.X
	serviceRight := serviceBounds.X + serviceBounds.W
	defs := []struct {
		group       string
		role        string
		orientation string
		points      []ArchitecturePoint
	}{
		{"gateway", "primary", "horizontal", []ArchitecturePoint{{X: all.X + 0.9, Y: serviceMidY}, {X: serviceLeft + serviceBounds.W*0.42, Y: serviceMidY}}},
		{"service", "primary", "horizontal", []ArchitecturePoint{{X: serviceLeft - 0.7, Y: serviceMidY + 0.15}, {X: serviceRight + 0.5, Y: serviceMidY + 0.15}}},
		{"registry", "secondary", "horizontal", []ArchitecturePoint{{X: serviceLeft + serviceBounds.W*0.52, Y: serviceTop - 1.0}, {X: serviceRight + 3.3, Y: serviceTop - 1.0}}},
		{"data", "secondary", "horizontal", []ArchitecturePoint{{X: serviceLeft + serviceBounds.W*0.35, Y: serviceBottom + 1.0}, {X: serviceRight + 1.4, Y: serviceBottom + 1.0}}},
		{"cache", "secondary", "horizontal", []ArchitecturePoint{{X: serviceLeft + serviceBounds.W*0.55, Y: serviceBottom + 0.55}, {X: serviceRight + 2.3, Y: serviceBottom + 0.55}}},
		{"storage", "secondary", "horizontal", []ArchitecturePoint{{X: serviceLeft - 1.5, Y: serviceBottom + 1.65}, {X: serviceRight + 0.7, Y: serviceBottom + 1.65}}},
		{"health", "auxiliary", "horizontal", []ArchitecturePoint{{X: serviceRight + 1.0, Y: serviceBottom + 2.35}, {X: all.X + all.W + 1.8, Y: serviceBottom + 2.35}}},
		{"observability", "auxiliary", "horizontal", []ArchitecturePoint{{X: serviceLeft - 0.7, Y: serviceBottom + 2.0}, {X: all.X + all.W + 1.2, Y: serviceBottom + 2.0}}},
	}
	used := map[string]bool{}
	for _, edge := range edges {
		group := inferArchitecturePathGroup(edge, map[string]Node{})
		used[group] = true
	}
	lanes := make([]ArchitectureBusLane, 0, len(defs))
	for i, def := range defs {
		points := def.points
		if len(points) == 0 {
			continue
		}
		bounds := boundsForArchitecturePoints(points, 0.4)
		lanes = append(lanes, ArchitectureBusLane{
			ID:          "lane-" + def.group,
			PathGroup:   def.group,
			Role:        def.role,
			Orientation: def.orientation,
			Points:      points,
			Bounds:      bounds,
			Index:       i,
		})
		if !used[def.group] {
			continue
		}
	}
	return lanes
}

func architectureRouteObstacles(layout ArchitectureLayoutResult) []ArchitectureRouteObstacle {
	ids := make([]string, 0, len(layout.Entities))
	for id := range layout.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ArchitectureRouteObstacle, 0, len(ids))
	for _, id := range ids {
		entity := layout.Entities[id]
		padding := 0.20
		bounds := inflateArchitectureBounds(entityFootprint(entity.Position), padding)
		out = append(out, ArchitectureRouteObstacle{
			ID:       "obstacle-" + id,
			EntityID: id,
			Kind:     inferEntityKind(entity.Node),
			Bounds:   bounds,
			Padding:  padding,
		})
	}
	return out
}

func architectureRoutePorts(from, to ArchitectureEntityLayout, edge Edge, role string) (ArchitecturePoint, ArchitecturePoint) {
	padding := 0.28
	switch role {
	case "primary":
		padding = 0.35
	case "auxiliary":
		padding = 0.22
	}
	fromSide := strings.ToUpper(strings.TrimSpace(edge.FromPort))
	toSide := strings.ToUpper(strings.TrimSpace(edge.ToPort))
	if fromSide == "" {
		fromSide = automaticArchitecturePortSide(from.Position, to.Position, true)
	}
	if toSide == "" {
		toSide = automaticArchitecturePortSide(to.Position, from.Position, false)
	}
	return architecturePortPoint(entityFootprint(from.Position), fromSide, padding), architecturePortPoint(entityFootprint(to.Position), toSide, padding)
}

func automaticArchitecturePortSide(from, to ArchitecturePoint, source bool) string {
	dx := to.X - from.X
	dy := to.Y - from.Y
	if math.Abs(dx) >= math.Abs(dy) {
		if source {
			if dx >= 0 {
				return "R"
			}
			return "L"
		}
		if dx >= 0 {
			return "L"
		}
		return "R"
	}
	if source {
		if dy >= 0 {
			return "B"
		}
		return "T"
	}
	if dy >= 0 {
		return "T"
	}
	return "B"
}

func architecturePortPoint(bounds ArchitectureBounds, side string, padding float64) ArchitecturePoint {
	cx := bounds.X + bounds.W*0.5
	cy := bounds.Y + bounds.H*0.5
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "R", "E", "EAST":
		return ArchitecturePoint{X: bounds.X + bounds.W + padding, Y: cy}
	case "L", "W", "WEST":
		return ArchitecturePoint{X: bounds.X - padding, Y: cy}
	case "T", "N", "NORTH":
		return ArchitecturePoint{X: cx, Y: bounds.Y - padding}
	case "B", "S", "SOUTH":
		return ArchitecturePoint{X: cx, Y: bounds.Y + bounds.H + padding}
	default:
		return ArchitecturePoint{X: cx, Y: cy}
	}
}

func obstacleAwareArchitectureRoute(from, to ArchitecturePoint, edge Edge, pathGroup string, laneIndex int, parallelOffset float64, layout ArchitectureLayoutResult, lane ArchitectureBusLane) []ArchitecturePoint {
	candidates := [][]ArchitecturePoint{
		{from, to},
		{from, {X: to.X, Y: from.Y}, to},
		{from, {X: from.X, Y: to.Y}, to},
		routeArchitectureLink(from, to, edge, pathGroup, laneIndex, parallelOffset, layout),
	}
	if lane.ID != "" {
		candidates = append(candidates, architectureBusLaneRoute(from, to, lane, laneIndex, parallelOffset))
	}
	best := []ArchitecturePoint{from, to}
	bestScore := math.Inf(1)
	for _, candidate := range candidates {
		route := simplifyArchitectureRoute(candidate)
		score := scoreArchitectureRoute(route, edge, pathGroup, layout).Score
		if score < bestScore {
			bestScore = score
			best = route
		}
	}
	if len(best) >= 2 && distancePoint(best[0], best[len(best)-1]) < 0.75 {
		lift := 0.62 + math.Abs(parallelOffset)*0.12
		best = simplifyArchitectureRoute([]ArchitecturePoint{from, {X: from.X, Y: from.Y + lift}, {X: to.X, Y: from.Y + lift}, to})
	}
	return best
}

func architectureBusLaneRoute(from, to ArchitecturePoint, lane ArchitectureBusLane, laneIndex int, parallelOffset float64) []ArchitecturePoint {
	if len(lane.Points) < 2 {
		return []ArchitecturePoint{from, to}
	}
	offset := parallelOffset * 0.18
	if lane.Orientation == "vertical" {
		x := lane.Points[0].X + offset
		return []ArchitecturePoint{from, {X: x, Y: from.Y}, {X: x, Y: to.Y}, to}
	}
	y := lane.Points[0].Y + offset
	if lane.PathGroup == "gateway" || lane.PathGroup == "service" {
		if math.Abs(from.Y-to.Y) <= 0.9 && to.X > from.X {
			return []ArchitecturePoint{from, to}
		}
	}
	return []ArchitecturePoint{from, {X: from.X, Y: y}, {X: to.X, Y: y}, to}
}

func routeArchitectureLink(from, to ArchitecturePoint, edge Edge, pathGroup string, laneIndex int, parallelOffset float64, layout ArchitectureLayoutResult) []ArchitecturePoint {
	route := []ArchitecturePoint{from}
	dx := to.X - from.X
	dy := to.Y - from.Y
	if math.Abs(dx) < 0.05 && math.Abs(dy) < 0.05 {
		return []ArchitecturePoint{from, to}
	}
	if architectureRouteCanBeDirect(from, to, edge, pathGroup) {
		return []ArchitecturePoint{from, to}
	}
	switch pathGroup {
	case "gateway", "service":
		if pathGroup == "service" {
			approachX := from.X + math.Max(1.4, (to.X-from.X)*0.38)
			laneY := to.Y
			if serviceBounds, ok := architectureZoneBounds(layout, "service"); ok {
				approachX = math.Min(approachX, serviceBounds.X-0.72-float64(laneIndex%3)*0.18)
				if to.Y <= serviceBounds.Y+serviceBounds.H*0.55 {
					laneY = to.Y - 1.02 - float64(laneIndex%2)*0.18
				} else {
					laneY = to.Y + 1.02 + float64(laneIndex%2)*0.18
				}
			}
			route = append(route,
				ArchitecturePoint{X: approachX, Y: from.Y},
				ArchitecturePoint{X: approachX, Y: laneY},
				ArchitecturePoint{X: to.X, Y: laneY},
			)
			break
		}
		if math.Abs(dy) <= 0.75 && to.X > from.X {
			route = append(route, to)
			return simplifyArchitectureRoute(route)
		}
		laneY := architectureGatewayLaneY(from, to, laneIndex)
		if pathGroup == "service" {
			laneY = to.Y
		}
		bendX := from.X + math.Max(1.4, (to.X-from.X)*0.45)
		route = append(route, ArchitecturePoint{X: bendX, Y: from.Y + parallelOffset*0.22}, ArchitecturePoint{X: bendX, Y: laneY}, ArchitecturePoint{X: to.X - math.Copysign(0.55, nonZero(dx, 1)), Y: laneY})
	case "registry":
		laneY := architectureUpperRouteLane(layout, pathGroup, laneIndex)
		exitX := architectureBusExitX(from, to, pathGroup, laneIndex, layout)
		approachX := to.X - math.Copysign(0.72, nonZero(dx, 1))
		route = append(route,
			ArchitecturePoint{X: exitX, Y: from.Y},
			ArchitecturePoint{X: exitX, Y: laneY},
			ArchitecturePoint{X: approachX, Y: laneY},
			ArchitecturePoint{X: approachX, Y: to.Y},
		)
	case "cache", "data", "storage":
		laneY := lowerArchitectureRouteLaneFor(from, to, pathGroup, laneIndex)
		exitX := architectureBusExitX(from, to, pathGroup, laneIndex, layout)
		approachX := to.X
		route = append(route,
			ArchitecturePoint{X: exitX, Y: from.Y},
			ArchitecturePoint{X: exitX, Y: laneY},
			ArchitecturePoint{X: approachX, Y: laneY},
			ArchitecturePoint{X: approachX, Y: to.Y},
		)
	case "health", "observability":
		laneY := lowerArchitectureRouteLaneFor(from, to, pathGroup, laneIndex)
		if strings.ToUpper(edge.FromPort) == "L" {
			laneX := math.Max(from.X, to.X) + 3.8 + float64(laneIndex)*0.36
			targetX := to.X + 1.15 + float64(laneIndex%2)*0.18
			route = append(route, ArchitecturePoint{X: laneX, Y: from.Y}, ArchitecturePoint{X: laneX, Y: laneY}, ArchitecturePoint{X: targetX, Y: laneY}, ArchitecturePoint{X: targetX, Y: to.Y})
		} else {
			exitX := architectureBusExitX(from, to, pathGroup, laneIndex, layout)
			route = append(route,
				ArchitecturePoint{X: exitX, Y: from.Y},
				ArchitecturePoint{X: exitX, Y: laneY},
				ArchitecturePoint{X: to.X, Y: laneY},
				ArchitecturePoint{X: to.X, Y: to.Y},
			)
		}
	default:
		if math.Abs(dy) > 0.55 && math.Abs(dx) > 0.55 {
			bendX := (from.X + to.X) / 2
			route = append(route, ArchitecturePoint{X: bendX, Y: from.Y + parallelOffset}, ArchitecturePoint{X: bendX, Y: to.Y + parallelOffset})
		}
	}
	route = append(route, to)
	return simplifyArchitectureRoute(route)
}

func architectureGatewayLaneY(from, to ArchitecturePoint, laneIndex int) float64 {
	return (from.Y+to.Y)/2 + architectureParallelOffset(laneIndex)*0.25
}

func architectureBusExitX(from, to ArchitecturePoint, pathGroup string, laneIndex int, layout ArchitectureLayoutResult) float64 {
	direction := 1.0
	if to.X < from.X {
		direction = -1
	}
	if serviceBounds, ok := architectureZoneBounds(layout, "service"); ok {
		right := serviceBounds.X + serviceBounds.W + 0.55 + float64(laneIndex%4)*0.18
		left := serviceBounds.X - 0.55 - float64(laneIndex%4)*0.18
		switch pathGroup {
		case "registry", "cache":
			return math.Max(right, from.X+0.95+float64(laneIndex%3)*0.16)
		case "data":
			return from.X + 0.95 + float64(laneIndex%3)*0.18
		case "observability":
			return math.Max(from.X+1.4+float64(laneIndex%3)*0.18, to.X-1.2-float64(laneIndex%2)*0.15)
		case "storage":
			return math.Min(left, from.X-0.95-float64(laneIndex%3)*0.16)
		case "health":
			return math.Max(right+1.8, from.X+1.2+float64(laneIndex%3)*0.22)
		}
	}
	if pathGroup == "cache" && direction < 0 {
		return math.Min(from.X-2.2-float64(laneIndex%2)*0.22, to.X-1.35-float64(laneIndex%2)*0.18)
	}
	if pathGroup == "observability" && direction > 0 {
		return from.X + 1.85 + float64(laneIndex%3)*0.28
	}
	if (pathGroup == "data" || pathGroup == "storage") && direction > 0 {
		return math.Max(from.X+1.55+float64(laneIndex%3)*0.26, to.X-1.25-float64(laneIndex%2)*0.18)
	}
	if pathGroup == "registry" && direction > 0 {
		return from.X + 1.15 + float64(laneIndex%3)*0.22
	}
	return from.X + direction*(1.25+float64(laneIndex%3)*0.26)
}

func architectureZoneBounds(layout ArchitectureLayoutResult, category string) (ArchitectureBounds, bool) {
	for _, zone := range layout.Zones {
		key := strings.ToLower(zone.Group.ID + " " + zone.Group.Label + " " + zone.Group.Kind)
		switch category {
		case "service":
			if strings.Contains(key, "service") || strings.Contains(key, "application") || strings.Contains(key, "app") {
				return zone.Bounds, true
			}
		}
	}
	return ArchitectureBounds{}, false
}

func nonZero(value, fallback float64) float64 {
	if math.Abs(value) < 0.001 {
		return fallback
	}
	return value
}

func architectureRouteCanBeDirect(from, to ArchitecturePoint, edge Edge, pathGroup string) bool {
	if pathGroup == "registry" || pathGroup == "cache" || pathGroup == "data" || pathGroup == "storage" || pathGroup == "health" || pathGroup == "observability" {
		return false
	}
	if strings.ToUpper(edge.FromPort) == "R" && strings.ToUpper(edge.ToPort) == "L" && to.X > from.X && math.Abs(to.Y-from.Y) < 0.35 {
		return true
	}
	if strings.ToUpper(edge.FromPort) == "B" && strings.ToUpper(edge.ToPort) == "T" && to.Y < from.Y && math.Abs(to.X-from.X) < 0.7 {
		return true
	}
	return math.Abs(from.Y-to.Y) <= 0.4 || math.Abs(from.X-to.X) <= 0.4
}

func lowerArchitectureRouteLane(layout ArchitectureLayoutResult, pathGroup string, laneIndex int) float64 {
	minY := math.Inf(1)
	serviceY := math.Inf(1)
	for _, zone := range layout.Zones {
		key := strings.ToLower(zone.Group.ID + " " + zone.Group.Label)
		if strings.Contains(key, "service") || strings.Contains(key, "application") || strings.Contains(key, "app") {
			serviceY = zone.Bounds.Y
		}
		if zone.Bounds.Y < minY {
			minY = zone.Bounds.Y
		}
	}
	if math.IsInf(minY, 1) {
		minY = 0
	}
	if math.IsInf(serviceY, 1) {
		serviceY = minY + 4.0
	}
	base := math.Min(serviceY-1.1, minY+1.1)
	switch pathGroup {
	case "cache":
		return base - 0.15 - float64(laneIndex)*0.24
	case "data":
		return base - 0.52 - float64(laneIndex)*0.24
	case "storage":
		return base - 0.90 - float64(laneIndex)*0.24
	case "observability":
		return base - 1.26 - float64(laneIndex)*0.26
	case "health":
		return base - 1.68 - float64(laneIndex)*0.28
	default:
		return base - float64(laneIndex)*0.20
	}
}

func lowerArchitectureRouteLaneFor(from, to ArchitecturePoint, pathGroup string, laneIndex int) float64 {
	base := math.Max(from.Y, to.Y) + 0.72
	switch pathGroup {
	case "cache":
		return base + 0.05 + float64(laneIndex)*0.24
	case "data":
		return base + 0.36 + float64(laneIndex)*0.24
	case "storage":
		return base + 0.70 + float64(laneIndex)*0.25
	case "observability":
		return base + 1.02 + float64(laneIndex)*0.26
	case "health":
		return base + 1.34 + float64(laneIndex)*0.28
	default:
		return base + float64(laneIndex)*0.20
	}
}

func architectureUpperRouteLane(layout ArchitectureLayoutResult, pathGroup string, laneIndex int) float64 {
	minY := math.Inf(1)
	for _, zone := range layout.Zones {
		key := strings.ToLower(zone.Group.ID + " " + zone.Group.Label)
		if strings.Contains(key, "service") || strings.Contains(key, "registry") {
			if zone.Bounds.Y < minY {
				minY = zone.Bounds.Y
			}
		}
	}
	if math.IsInf(minY, 1) {
		minY = 0
	}
	return minY - 0.62 - float64(laneIndex)*0.24
}

func architectureParallelOffset(index int) float64 {
	if index == 0 {
		return 0
	}
	magnitude := float64((index+1)/2) * 0.34
	if index%2 == 0 {
		return -magnitude
	}
	return magnitude
}

func simplifyArchitectureRoute(route []ArchitecturePoint) []ArchitecturePoint {
	if len(route) <= 2 {
		return route
	}
	out := []ArchitecturePoint{route[0]}
	for i := 1; i < len(route)-1; i++ {
		prev := out[len(out)-1]
		cur := route[i]
		next := route[i+1]
		if distancePoint(prev, cur) < 0.05 {
			continue
		}
		if nearlyCollinear(prev, cur, next) {
			continue
		}
		out = append(out, cur)
	}
	out = append(out, route[len(route)-1])
	return out
}

func architectureRouteSegments(route []ArchitecturePoint) []ArchitectureRouteSegment {
	if len(route) < 2 {
		return nil
	}
	segments := make([]ArchitectureRouteSegment, 0, len(route)-1)
	for i := 0; i < len(route)-1; i++ {
		if distancePoint(route[i], route[i+1]) < 0.05 {
			continue
		}
		segments = append(segments, ArchitectureRouteSegment{From: route[i], To: route[i+1], Kind: "orthogonal"})
	}
	return segments
}

func architectureRouteLabelAnchor(route []ArchitecturePoint, offset float64) ArchitecturePoint {
	if len(route) == 0 {
		return ArchitecturePoint{}
	}
	if len(route) == 1 {
		return route[0]
	}
	bestStart := route[0]
	bestEnd := route[1]
	bestLen := distancePoint(bestStart, bestEnd)
	for i := 1; i < len(route)-1; i++ {
		length := distancePoint(route[i], route[i+1])
		if length > bestLen {
			bestLen = length
			bestStart = route[i]
			bestEnd = route[i+1]
		}
	}
	anchor := ArchitecturePoint{X: (bestStart.X + bestEnd.X) / 2, Y: (bestStart.Y + bestEnd.Y) / 2}
	dx := bestEnd.X - bestStart.X
	dy := bestEnd.Y - bestStart.Y
	length := math.Hypot(dx, dy)
	if length > 0.01 {
		anchor.X += (-dy / length) * (0.25 + offset*0.05)
		anchor.Y += (dx / length) * (0.25 + offset*0.05)
	}
	return anchor
}

func architectureRouteSpurs(route []ArchitecturePoint) ([]ArchitecturePoint, []ArchitecturePoint) {
	if len(route) < 2 {
		return nil, nil
	}
	start := []ArchitecturePoint{route[0], route[1]}
	end := []ArchitecturePoint{route[len(route)-2], route[len(route)-1]}
	return start, end
}

func scoreArchitectureRoute(route []ArchitecturePoint, edge Edge, pathGroup string, layout ArchitectureLayoutResult) ArchitectureSingleRouteMetrics {
	intersections := routeEntityIntersectionsCandidate(route, layout, edge.From, edge.To)
	bends := maxInt(0, len(route)-2)
	length := routeLength2D(route)
	diagonals := routeDiagonalSegments(route)
	score := length + float64(bends)*6 + float64(diagonals)*80 + float64(intersections)*10000
	if len(route) >= 2 && architectureDirectionViolation(edge, route[0], route[len(route)-1]) {
		score += 1000
	}
	if pathGroup != "" && architectureRouteLaneViolation(route, pathGroup, layout) {
		score += 50
	}
	if len(route) >= 2 && routeDetourRatio(route, route[0], route[len(route)-1]) > 2.8 {
		score += 10
	}
	return ArchitectureSingleRouteMetrics{Length: length, BendCount: bends, EntityIntersections: intersections, Score: score}
}

func routeDiagonalSegments(route []ArchitecturePoint) int {
	count := 0
	for i := 0; i < len(route)-1; i++ {
		dx := math.Abs(route[i].X - route[i+1].X)
		dy := math.Abs(route[i].Y - route[i+1].Y)
		if dx > 0.05 && dy > 0.05 {
			count++
		}
	}
	return count
}

func architectureRouteLaneViolation(route []ArchitecturePoint, pathGroup string, layout ArchitectureLayoutResult) bool {
	if len(route) < 2 || pathGroup == "gateway" || pathGroup == "service" {
		return false
	}
	serviceBounds, ok := architectureZoneBounds(layout, "service")
	if !ok {
		return false
	}
	midY := routeMidY(route)
	switch pathGroup {
	case "registry":
		return midY > serviceBounds.Y+serviceBounds.H*0.45
	case "data", "cache", "storage", "health", "observability":
		return midY < serviceBounds.Y+serviceBounds.H*0.35
	default:
		return false
	}
}

func routeEntityIntersectionsCandidate(route []ArchitecturePoint, layout ArchitectureLayoutResult, fromID, toID string) int {
	count := 0
	for id, entity := range layout.Entities {
		if id == fromID || id == toID {
			continue
		}
		footprint := inflateArchitectureBounds(entityFootprint(entity.Position), 0.08)
		for i := 0; i < len(route)-1; i++ {
			if segmentIntersectsBounds(route[i], route[i+1], footprint) {
				count++
				break
			}
		}
	}
	return count
}

func boundsForArchitecturePoints(points []ArchitecturePoint, pad float64) ArchitectureBounds {
	if len(points) == 0 {
		return ArchitectureBounds{}
	}
	minX, maxX := points[0].X, points[0].X
	minY, maxY := points[0].Y, points[0].Y
	for _, point := range points[1:] {
		minX = math.Min(minX, point.X)
		maxX = math.Max(maxX, point.X)
		minY = math.Min(minY, point.Y)
		maxY = math.Max(maxY, point.Y)
	}
	return ArchitectureBounds{X: minX - pad, Y: minY - pad, W: maxX - minX + pad*2, H: maxY - minY + pad*2}
}

func inflateArchitectureBounds(bounds ArchitectureBounds, pad float64) ArchitectureBounds {
	return ArchitectureBounds{X: bounds.X - pad, Y: bounds.Y - pad, W: bounds.W + pad*2, H: bounds.H + pad*2}
}

func architectureAllZoneBounds(layout ArchitectureLayoutResult) ArchitectureBounds {
	points := []ArchitecturePoint{}
	for _, zone := range layout.Zones {
		b := zone.Bounds
		points = append(points,
			ArchitecturePoint{X: b.X, Y: b.Y},
			ArchitecturePoint{X: b.X + b.W, Y: b.Y + b.H},
		)
	}
	if len(points) == 0 {
		return ArchitectureBounds{X: 0, Y: 0, W: 8, H: 6}
	}
	return boundsForArchitecturePoints(points, 0)
}

func architectureDirectionViolation(edge Edge, from, to ArchitecturePoint) bool {
	fromPort := strings.ToUpper(strings.TrimSpace(edge.FromPort))
	toPort := strings.ToUpper(strings.TrimSpace(edge.ToPort))
	if fromPort == "R" && toPort == "L" {
		return to.X <= from.X
	}
	if fromPort == "L" && toPort == "R" {
		return to.X >= from.X
	}
	if fromPort == "B" && toPort == "T" {
		return to.Y <= from.Y
	}
	if fromPort == "T" && toPort == "B" {
		return to.Y >= from.Y
	}
	return false
}

func routeDetourRatio(route []ArchitecturePoint, from, to ArchitecturePoint) float64 {
	direct := distancePoint(from, to)
	if direct <= 0.01 {
		return 1
	}
	return routeLength2D(route) / direct
}

func routeLength2D(route []ArchitecturePoint) float64 {
	total := 0.0
	for i := 0; i < len(route)-1; i++ {
		total += distancePoint(route[i], route[i+1])
	}
	return total
}

func distancePoint(a, b ArchitecturePoint) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func nearlyCollinear(a, b, c ArchitecturePoint) bool {
	abx, aby := b.X-a.X, b.Y-a.Y
	bcx, bcy := c.X-b.X, c.Y-b.Y
	cross := math.Abs(abx*bcy - aby*bcx)
	return cross < 0.001
}

func routeEntityIntersections(link ArchitectureRoutedLink, layout ArchitectureLayoutResult) int {
	count := 0
	for id, entity := range layout.Entities {
		if id == link.Edge.From || id == link.Edge.To {
			continue
		}
		footprint := entityFootprint(entity.Position)
		for i := 0; i < len(link.Route)-1; i++ {
			if segmentIntersectsBounds(link.Route[i], link.Route[i+1], footprint) {
				count++
				break
			}
		}
	}
	return count
}

func entityFootprint(center ArchitecturePoint) ArchitectureBounds {
	return ArchitectureBounds{X: center.X - 0.62, Y: center.Y - 0.62, W: 1.24, H: 1.24}
}

func segmentIntersectsBounds(a, b ArchitecturePoint, box ArchitectureBounds) bool {
	if pointInBounds(a, box) || pointInBounds(b, box) {
		return true
	}
	corners := []ArchitecturePoint{
		{X: box.X, Y: box.Y},
		{X: box.X + box.W, Y: box.Y},
		{X: box.X + box.W, Y: box.Y + box.H},
		{X: box.X, Y: box.Y + box.H},
	}
	for i := 0; i < len(corners); i++ {
		if segmentsIntersect(a, b, corners[i], corners[(i+1)%len(corners)]) {
			return true
		}
	}
	return false
}

func pointInBounds(p ArchitecturePoint, box ArchitectureBounds) bool {
	return p.X >= box.X && p.X <= box.X+box.W && p.Y >= box.Y && p.Y <= box.Y+box.H
}

func routeCrossings(links []ArchitectureRoutedLink) int {
	count := 0
	for i := 0; i < len(links); i++ {
		for j := i + 1; j < len(links); j++ {
			if links[i].Role == "auxiliary" || links[j].Role == "auxiliary" {
				continue
			}
			if linksShareEndpoint(links[i], links[j]) {
				continue
			}
			if links[i].PathGroup != "" && links[i].PathGroup == links[j].PathGroup {
				continue
			}
			if routesIntersect(links[i].Route, links[j].Route) {
				count++
			}
		}
	}
	return count
}

func linksShareEndpoint(a, b ArchitectureRoutedLink) bool {
	return a.Edge.From == b.Edge.From || a.Edge.From == b.Edge.To || a.Edge.To == b.Edge.From || a.Edge.To == b.Edge.To
}

func routesIntersect(a, b []ArchitecturePoint) bool {
	for i := 0; i < len(a)-1; i++ {
		for j := 0; j < len(b)-1; j++ {
			if segmentsIntersect(a[i], a[i+1], b[j], b[j+1]) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d ArchitecturePoint) bool {
	orient := func(p, q, r ArchitecturePoint) float64 {
		return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X)
	}
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	return o1*o2 < 0 && o3*o4 < 0
}

func routeParallelOverlaps(links []ArchitectureRoutedLink) int {
	count := 0
	for i := 0; i < len(links); i++ {
		for j := i + 1; j < len(links); j++ {
			if links[i].PathGroup != links[j].PathGroup || links[i].PathGroup == "" {
				continue
			}
			if math.Abs(routeMidY(links[i].Route)-routeMidY(links[j].Route)) < 0.12 && math.Abs(routeMidX(links[i].Route)-routeMidX(links[j].Route)) < 1.1 {
				count++
			}
		}
	}
	return count
}

func routeMidX(route []ArchitecturePoint) float64 {
	if len(route) == 0 {
		return 0
	}
	return route[len(route)/2].X
}

func routeMidY(route []ArchitecturePoint) float64 {
	if len(route) == 0 {
		return 0
	}
	return route[len(route)/2].Y
}

func routeBusLaneMetrics(links []ArchitectureRoutedLink) (int, int) {
	counts := map[string]int{}
	for _, link := range links {
		if link.PathGroup != "" {
			counts[link.PathGroup]++
		}
	}
	lanes := 0
	for _, count := range counts {
		if count > 1 {
			lanes++
		}
	}
	return lanes, lanes
}

func groupPointers(groups []Group) map[string]*Group {
	out := map[string]*Group{}
	for i := range groups {
		group := groups[i]
		out[group.ID] = &group
	}
	return out
}
