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
	Edge      Edge
	Role      string
	PathGroup string
	Route     []ArchitecturePoint
}

type ArchitectureRoutingResult struct {
	Links   []ArchitectureRoutedLink
	Metrics ArchitectureRouteMetrics
}

type ArchitectureRouteMetrics struct {
	PortHintViolations  int
	DirectionViolations int
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
	zoneW := 7.4
	zoneH := 6.3
	zoneGap := 2.8
	zones := make([]ArchitectureZoneLayout, 0, len(groups))
	zoneByID := map[string]ArchitectureBounds{}
	for i, group := range groups {
		bounds := ArchitectureBounds{X: 0.5 + float64(i)*(zoneW+zoneGap), Y: 0.8, W: zoneW, H: zoneH}
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
			position := placeArchitectureEntity(bounds, i, len(members))
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
	for _, edge := range graph.Edges {
		role := inferArchitectureRole(edge, graph.NodeMap)
		pathGroup := inferArchitecturePathGroup(edge, graph.NodeMap)
		from, fromOK := layout.Entities[edge.From]
		to, toOK := layout.Entities[edge.To]
		route := []ArchitecturePoint{}
		if fromOK && toOK {
			route = routeArchitectureLink(from.Position, to.Position)
		}
		out = append(out, ArchitectureRoutedLink{Edge: edge, Role: role, PathGroup: pathGroup, Route: route})
	}
	return ArchitectureRoutingResult{Links: out, Metrics: ValidateArchitectureRoutes(graph, layout, out)}
}

func ValidateArchitectureRoutes(graph ArchitectureSemanticGraph, layout ArchitectureLayoutResult, links []ArchitectureRoutedLink) ArchitectureRouteMetrics {
	metrics := ArchitectureRouteMetrics{}
	for _, link := range links {
		from, fromOK := layout.Entities[link.Edge.From]
		to, toOK := layout.Entities[link.Edge.To]
		if !fromOK || !toOK {
			continue
		}
		if architectureDirectionViolation(link.Edge, from.Position, to.Position) {
			metrics.DirectionViolations++
			metrics.PortHintViolations++
		}
	}
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
	for _, id := range ids {
		item := layout.Entities[id]
		kind := inferEntityKind(item.Node)
		entities = append(entities, map[string]any{
			"id":         item.Node.ID,
			"label":      nonEmpty(item.Node.Label, item.Node.ID),
			"kind":       kind,
			"zone":       item.GroupID,
			"position":   map[string]any{"x": item.Position.X, "y": item.Position.Y},
			"size":       map[string]any{"w": 2.0, "d": 2.0, "h": 1.6},
			"importance": 0.62,
			"presentation": map[string]any{
				"icon": iconForKind(kind),
			},
		})
	}
	return entities
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
				"arrow":     "forward",
				"lineStyle": lineStyle(edge, routed.Role),
				"color":     colorForRole(routed.Role),
				"fromPort":  edge.FromPort,
				"toPort":    edge.ToPort,
			},
			"metadata": map[string]any{
				"mermaid_from_port": edge.FromPort,
				"mermaid_to_port":   edge.ToPort,
				"route_stage":       "ArchitectureRoutingEngine",
			},
		}
		if label := compact(edge.Label, 22); label != "" {
			linkItem["label"] = label
		}
		links = append(links, linkItem)
	}
	return links
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

func routeArchitectureLink(from, to ArchitecturePoint) []ArchitecturePoint {
	route := []ArchitecturePoint{from}
	if math.Abs(from.Y-to.Y) > 0.4 && math.Abs(from.X-to.X) > 0.4 {
		bendX := (from.X + to.X) / 2
		route = append(route, ArchitecturePoint{X: bendX, Y: from.Y}, ArchitecturePoint{X: bendX, Y: to.Y})
	}
	route = append(route, to)
	return route
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

func groupPointers(groups []Group) map[string]*Group {
	out := map[string]*Group{}
	for i := range groups {
		group := groups[i]
		out[group.ID] = &group
	}
	return out
}
