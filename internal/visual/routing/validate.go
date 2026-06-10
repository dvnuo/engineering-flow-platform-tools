package routing

import "math"

func ValidateRoutes(input Input, routes []Route) RouteMetrics {
	metrics := RouteMetrics{}
	entities := entityMap(input.Entities)
	obstacles := BuildObstacles(input.Entities, 0.14)
	for _, route := range routes {
		switch route.Role {
		case "primary":
			metrics.PrimaryRouteCount++
		case "auxiliary":
			metrics.AuxiliaryRouteCount++
		default:
			metrics.SecondaryRouteCount++
		}
		from, fromOK := entities[route.From]
		to, toOK := entities[route.To]
		if fromOK && toOK {
			if PortHintViolation(LinkModel{FromPort: route.FromPort, ToPort: route.ToPort}, from, to) {
				metrics.PortHintViolations++
				metrics.DirectionViolations++
			}
			metrics.EndpointInsideEntities += EndpointInsideEntity(route.Points, entities, route.From, route.To)
		}
		metrics.EntityIntersections += PolylineIntersectsObstacles(route.Points, obstacles, map[string]bool{route.From: true, route.To: true})
		if routeDetour(route, entities) > 2.8 {
			metrics.LongDetourCount++
		}
		if math.Abs(route.ParallelOffset) > 0.001 {
			metrics.ParallelOffsetCount++
		}
	}
	metrics.CrossingCount = CountRouteCrossings(routes)
	metrics.ParallelOverlapCount, metrics.PathGroupOverlap = CountRouteOverlaps(routes)
	bundles := BuildBundles(routes)
	metrics.BundleCount = len(bundles)
	laneSet := map[string]bool{}
	for _, route := range routes {
		if route.BusLaneID != "" {
			laneSet[route.BusLaneID] = true
		}
	}
	metrics.BusLaneCount = len(laneSet)
	return metrics
}

func CountRouteCrossings(routes []Route) int {
	count := 0
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if routes[i].Role == "auxiliary" || routes[j].Role == "auxiliary" {
				continue
			}
			if routeShareEndpoint(routes[i], routes[j]) {
				continue
			}
			if routes[i].PathGroup != "" && routes[i].PathGroup == routes[j].PathGroup {
				continue
			}
			if routesCross(routes[i].Points, routes[j].Points) {
				count++
			}
		}
	}
	return count
}

func CountRouteOverlaps(routes []Route) (sameGroup int, crossGroup int) {
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if !routesOverlap(routes[i].Points, routes[j].Points, 0.05) {
				continue
			}
			if routes[i].PathGroup != "" && routes[i].PathGroup == routes[j].PathGroup {
				if routes[i].BundleID == "" || routes[i].BundleID != routes[j].BundleID {
					sameGroup++
				}
				continue
			}
			if routes[i].Role == "auxiliary" || routes[j].Role == "auxiliary" || routeShareEndpoint(routes[i], routes[j]) {
				continue
			} else {
				crossGroup++
			}
		}
	}
	return sameGroup, crossGroup
}

func routeShareEndpoint(a, b Route) bool {
	return a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To
}

func routesCross(a, b []Vec2) bool {
	for _, sa := range PolylineSegments(a) {
		for _, sb := range PolylineSegments(b) {
			if SegmentsIntersect(sa.From, sa.To, sb.From, sb.To) {
				return true
			}
		}
	}
	return false
}

func routeDetour(route Route, entities map[string]EntityFrame) float64 {
	from, fromOK := entities[route.From]
	to, toOK := entities[route.To]
	if !fromOK || !toOK {
		return 1
	}
	direct := Distance(from.Center, to.Center)
	if direct <= 0.01 {
		return 1
	}
	return PolylineLength(route.Points) / direct
}

func entityMap(entities []EntityFrame) map[string]EntityFrame {
	out := map[string]EntityFrame{}
	for _, entity := range entities {
		out[entity.ID] = entity
	}
	return out
}
