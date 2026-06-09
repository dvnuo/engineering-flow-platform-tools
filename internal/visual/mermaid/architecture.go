package mermaid

import (
	"fmt"
	"math"
	"sort"
	"strings"
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
	LaneIndex      int
	ParallelOffset float64
	BundleID       string
}

type ArchitectureRoutingResult struct {
	Links   []ArchitectureRoutedLink
	Metrics ArchitectureRouteMetrics
}

type ArchitectureRouteMetrics struct {
	PortHintViolations   int
	DirectionViolations  int
	EntityIntersections  int
	CrossingCount        int
	ParallelOverlapCount int
	BusLaneCount         int
	BundleCount          int
	LongDetourCount      int
	PrimaryRouteCount    int
	SecondaryRouteCount  int
	AuxiliaryRouteCount  int
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
	ranks := architectureRanks(graph.Nodes, graph.Edges)
	groups := architectureSortedGroups(Diagram{Groups: groupPointers(graph.Groups)}, graph.Nodes, ranks)
	if len(groups) == 0 {
		groups = []Group{{ID: "main-zone", Label: "MAIN ZONE", Kind: "zone"}}
	}
	groupNodes := architectureNodesByGroup(graph.Nodes, groups, ranks)
	complex := len(groups) > 6 || len(graph.Nodes) > 12
	zoneW := 7.4
	zoneH := 6.3
	zoneGap := 2.8
	if complex {
		zoneW = 11.2
		zoneH = 7.4
		zoneGap = 4.4
	}
	zones := make([]ArchitectureZoneLayout, 0, len(groups))
	zoneByID := map[string]ArchitectureBounds{}
	for i, group := range groups {
		bounds := ArchitectureBounds{X: 0.5 + float64(i)*(zoneW+zoneGap), Y: 0.8, W: zoneW, H: zoneH}
		if complex {
			slot := preferredArchitectureZoneSlot(group, groupNodes[group.ID], ranks, i)
			bounds = ArchitectureBounds{
				X: 0.5 + slot.X*(zoneW+zoneGap),
				Y: 0.8 + slot.Y*(zoneH+2.7),
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

func ArchitectureRoutingEngine(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult) ArchitectureRoutingResult {
	out := make([]ArchitectureRoutedLink, 0, len(graph.Edges))
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
		if fromOK && toOK {
			route = routeArchitectureLink(from.Position, to.Position, edge, pathGroup, laneIndex, parallelOffset, layout)
		}
		out = append(out, ArchitectureRoutedLink{
			Edge:           edge,
			Role:           role,
			PathGroup:      pathGroup,
			Route:          route,
			LaneIndex:      laneIndex,
			ParallelOffset: parallelOffset,
			BundleID:       pathGroup,
		})
	}
	return ArchitectureRoutingResult{Links: out, Metrics: ValidateArchitectureRoutes(graph, layout, out)}
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
				"boundary":    "dashed",
				"fill":        "#f8fafc",
				"fillOpacity": 0.045,
				"color":       "#475569",
				"labelPoint":  map[string]any{"x": b.X + 0.45, "y": b.Y + b.H - 0.45},
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
	size := architectureEntityVisualSize(len(ids))
	for _, id := range ids {
		item := layout.Entities[id]
		kind := inferEntityKind(item.Node)
		entities = append(entities, map[string]any{
			"id":         item.Node.ID,
			"label":      nonEmpty(item.Node.Label, item.Node.ID),
			"kind":       kind,
			"zone":       item.GroupID,
			"position":   map[string]any{"x": item.Position.X, "y": item.Position.Y},
			"size":       map[string]any{"w": size.W, "d": size.D, "h": size.H},
			"importance": 0.62,
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

func architectureEntityVisualSize(count int) architectureEntitySize {
	if count > 18 {
		return architectureEntitySize{W: 1.28, D: 1.18, H: 1.12}
	}
	if count > 10 {
		return architectureEntitySize{W: 1.55, D: 1.42, H: 1.28}
	}
	return architectureEntitySize{W: 2.0, D: 2.0, H: 1.6}
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

func preferredArchitectureZoneSlot(group Group, members []Node, ranks map[string]int, fallbackIndex int) ArchitecturePoint {
	key := strings.ToLower(group.ID + " " + group.Label + " " + group.Kind)
	switch {
	case strings.Contains(key, "client"):
		return ArchitecturePoint{X: 0, Y: 1}
	case strings.Contains(key, "edge"):
		return ArchitecturePoint{X: 1, Y: 1}
	case strings.Contains(key, "gateway"):
		return ArchitecturePoint{X: 2, Y: 1}
	case strings.Contains(key, "service"), strings.Contains(key, "application"), strings.Contains(key, "app"):
		return ArchitecturePoint{X: 3, Y: 1}
	case strings.Contains(key, "registry"):
		return ArchitecturePoint{X: 4, Y: 1.1}
	case strings.Contains(key, "storage"):
		return ArchitecturePoint{X: 4.55, Y: 0}
	case strings.Contains(key, "cache"):
		return ArchitecturePoint{X: 2.7, Y: 0}
	case strings.Contains(key, "database"), strings.Contains(key, "data"):
		return ArchitecturePoint{X: 3.65, Y: 0}
	case strings.Contains(key, "observ"):
		return ArchitecturePoint{X: 5.65, Y: 0}
	case strings.Contains(key, "admin"):
		return ArchitecturePoint{X: 5.65, Y: 1.05}
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

func placeArchitectureEntityInGroup(group Group, bounds ArchitectureBounds, index, count int, complex bool) ArchitecturePoint {
	if !complex {
		return placeArchitectureEntity(bounds, index, count)
	}
	key := strings.ToLower(group.ID + " " + group.Label + " " + group.Kind)
	if strings.Contains(key, "client") {
		if count <= 1 {
			return ArchitecturePoint{X: bounds.X + bounds.W*0.5, Y: bounds.Y + bounds.H*0.58}
		}
		return ArchitecturePoint{
			X: bounds.X + bounds.W*0.30,
			Y: bounds.Y + bounds.H*(0.78-0.26*float64(index)),
		}
	}
	if strings.Contains(key, "service") || strings.Contains(key, "application") || strings.Contains(key, "app") {
		if count <= 6 {
			col := index / 3
			row := index % 3
			return ArchitecturePoint{
				X: bounds.X + bounds.W*(0.26+0.36*float64(col)),
				Y: bounds.Y + bounds.H*(0.78-0.28*float64(row)),
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
	if strings.Contains(key, "registry") || strings.Contains(key, "cache") {
		return ArchitecturePoint{
			X: bounds.X + bounds.W*(0.28+0.24*float64(index%3)),
			Y: bounds.Y + bounds.H*(0.64-0.25*float64(index/3)),
		}
	}
	if strings.Contains(key, "observ") {
		if count <= 4 {
			step := 0.68
			if count > 1 {
				step = 0.68 / float64(count-1)
			}
			return ArchitecturePoint{
				X: bounds.X + bounds.W*(0.16+float64(index)*step),
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
	case "registry":
		laneY := math.Max(from.Y, to.Y) + 1.35 + float64(laneIndex)*0.26
		exitX := from.X + math.Copysign(1.35+float64(laneIndex%3)*0.18, nonZero(dx, 1))
		route = append(route, ArchitecturePoint{X: exitX, Y: from.Y}, ArchitecturePoint{X: exitX, Y: laneY}, ArchitecturePoint{X: to.X - math.Copysign(0.72, nonZero(dx, 1)), Y: laneY})
	case "cache", "data", "storage":
		laneY := lowerArchitectureRouteLane(layout, pathGroup, laneIndex)
		exitX := lowerArchitectureExitX(from, to, pathGroup, laneIndex)
		route = append(route, ArchitecturePoint{X: exitX, Y: from.Y}, ArchitecturePoint{X: exitX, Y: laneY}, ArchitecturePoint{X: to.X, Y: laneY})
	case "health", "observability":
		laneY := lowerArchitectureRouteLane(layout, pathGroup, laneIndex)
		if strings.ToUpper(edge.FromPort) == "L" {
			laneX := math.Max(from.X, to.X) + 6.4 + float64(laneIndex)*0.32
			targetX := to.X + 1.45 + float64(laneIndex%2)*0.18
			route = append(route, ArchitecturePoint{X: laneX, Y: from.Y}, ArchitecturePoint{X: laneX, Y: laneY}, ArchitecturePoint{X: targetX, Y: laneY}, ArchitecturePoint{X: targetX, Y: to.Y})
		} else {
			exitX := lowerArchitectureExitX(from, to, pathGroup, laneIndex)
			route = append(route, ArchitecturePoint{X: exitX, Y: from.Y}, ArchitecturePoint{X: exitX, Y: laneY}, ArchitecturePoint{X: to.X, Y: laneY})
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

func lowerArchitectureExitX(from, to ArchitecturePoint, pathGroup string, laneIndex int) float64 {
	direction := 1.0
	if to.X < from.X {
		direction = -1
	}
	if pathGroup == "cache" && direction < 0 {
		return math.Min(from.X-2.6-float64(laneIndex%2)*0.22, to.X-1.45-float64(laneIndex%2)*0.18)
	}
	if pathGroup == "observability" && direction > 0 {
		return from.X + 2.05 + float64(laneIndex%3)*0.26
	}
	if (pathGroup == "data" || pathGroup == "storage") && direction > 0 {
		return math.Max(from.X+1.8+float64(laneIndex%3)*0.24, to.X-1.55-float64(laneIndex%2)*0.18)
	}
	return from.X + direction*(1.45+float64(laneIndex%3)*0.26)
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
	for _, zone := range layout.Zones {
		if zone.Bounds.Y < minY {
			minY = zone.Bounds.Y
		}
	}
	if math.IsInf(minY, 1) {
		minY = 0
	}
	base := minY + 1.05
	switch pathGroup {
	case "cache":
		return base - float64(laneIndex)*0.20
	case "data":
		return base - 0.42 - float64(laneIndex)*0.20
	case "storage":
		return base - 0.78 - float64(laneIndex)*0.20
	case "observability":
		return base - 1.08 - float64(laneIndex)*0.22
	case "health":
		return base - 1.42 - float64(laneIndex)*0.24
	default:
		return base - float64(laneIndex)*0.20
	}
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
		return to.Y >= from.Y
	}
	if fromPort == "T" && toPort == "B" {
		return to.Y <= from.Y
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
	return ArchitectureBounds{X: center.X - 1.05, Y: center.Y - 1.05, W: 2.1, H: 2.1}
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
			if linksShareEndpoint(links[i], links[j]) {
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
