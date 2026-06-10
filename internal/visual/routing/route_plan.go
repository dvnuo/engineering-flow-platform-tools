package routing

import "strings"

func BuildRoutePlan(input Input, opts Options) RoutePlan {
	if opts.Engine == "" {
		opts = DefaultOptions()
	}
	if opts.Clearance <= 0 {
		opts.Clearance = DefaultOptions().Clearance
	}
	if opts.ScoreWeights.Length == 0 {
		opts.ScoreWeights = DefaultOptions().ScoreWeights
	}
	lanes := PlanBusLanes(input.Zones, input.Links)
	obstacles := BuildObstacles(input.Entities, opts.Clearance)
	routes := make([]Route, 0, len(input.Links))
	groupCounters := map[string]int{}
	for _, link := range input.Links {
		index := groupCounters[link.PathGroup]
		groupCounters[link.PathGroup]++
		route, ok := routeSingle(input, link, lanes, obstacles, routes, opts, index)
		if ok {
			routes = append(routes, route)
		}
	}
	parallelOffsets := 0
	if opts.UseNudging {
		before := ValidateRoutes(input, routes)
		nudged, nudgedCount := ApplyParallelNudging(routes)
		after := ValidateRoutes(input, nudged)
		if after.EntityIntersections <= before.EntityIntersections &&
			after.PortHintViolations <= before.PortHintViolations &&
			after.ParallelOverlapCount <= before.ParallelOverlapCount &&
			after.PathGroupOverlap <= before.PathGroupOverlap {
			routes, parallelOffsets = nudged, nudgedCount
		}
	}
	ripupImprovement := 0
	if opts.UseRipUp {
		routes, ripupImprovement = RipUpAndReroute(input, lanes, obstacles, routes, opts)
	}
	displayRoutes, hiddenDetailRoutes, metrics := AggregateDisplayRoutes(input, routes, lanes, obstacles, opts)
	metrics.ParallelOffsetCount += parallelOffsets
	if opts.UseRipUp {
		metrics.RipUpRerouteRounds = opts.RipUpRounds
		metrics.RipUpRerouteImprovement = ripupImprovement
	}
	return RoutePlan{
		Version:            RoutePlanVersion,
		Backend:            opts.Engine,
		SourceEdges:        append([]LinkModel(nil), input.Links...),
		DisplayRoutes:      displayRoutes,
		HiddenDetailRoutes: hiddenDetailRoutes,
		Routes:             displayRoutes,
		Lanes:              lanes,
		Bundles:            BuildBundles(displayRoutes),
		Obstacles:          obstacles,
		Metrics:            metrics,
	}
}

func routeSingle(input Input, link LinkModel, lanes []BusLane, obstacles []RouteObstacle, existing []Route, opts Options, laneIndex int) (Route, bool) {
	entities := entityMap(input.Entities)
	from, fromOK := entities[link.From]
	to, toOK := entities[link.To]
	if !fromOK || !toOK {
		return Route{}, false
	}
	source, target := ResolvePorts(from, to, link)
	lane := LaneByPathGroup(lanes)[link.PathGroup]
	if !shouldUseBusLane(link) {
		lane = BusLane{}
	}
	points, metrics := BestCandidateRoute(source, target, link, input.Entities, obstacles, lanes, lane, existing, opts)
	bestSource := source
	bestTarget := target
	if UseZoneBoundaryRouting(input, from, to) && shouldUseZoneBoundaryRoute(link) {
		zoneSource, zoneTarget := ResolveZoneBoundaryPorts(input.Zones, from, to, link)
		zonePoints, zoneMetrics := BestCandidateRoute(zoneSource, zoneTarget, link, input.Entities, obstacles, lanes, lane, existing, opts)
		zoneStrictlyClearsEntity := metrics.EntityIntersections > 0 && zoneMetrics.EntityIntersections == 0
		zoneIsClearlyBetter := link.Role != "primary" &&
			zoneMetrics.EntityIntersections <= metrics.EntityIntersections &&
			zoneMetrics.BendCount <= metrics.BendCount+2 &&
			zoneMetrics.Score < metrics.Score-opts.ScoreWeights.WrongLane
		zoneIsMapStyleBetter := link.Role != "primary" &&
			zoneMetrics.EntityIntersections == 0 &&
			zoneMetrics.BendCount <= metrics.BendCount+4 &&
			zoneMetrics.Length <= metrics.Length*1.25+6
		if zoneStrictlyClearsEntity || zoneIsClearlyBetter || zoneIsMapStyleBetter {
			points = zonePoints
			metrics = zoneMetrics
			bestSource = zoneSource
			bestTarget = zoneTarget
		}
	}
	if metrics.EntityIntersections > 0 && link.FromPort == "" && link.ToPort == "" {
		for _, fromSide := range []string{"R", "L", "T", "B"} {
			for _, toSide := range []string{"R", "L", "T", "B"} {
				altLink := link
				altLink.FromPort = fromSide
				altLink.ToPort = toSide
				altSource, altTarget := ResolvePorts(from, to, altLink)
				altPoints, altMetrics := BestCandidateRoute(altSource, altTarget, altLink, input.Entities, obstacles, lanes, lane, existing, opts)
				if altMetrics.Score < metrics.Score ||
					(altMetrics.EntityIntersections < metrics.EntityIntersections && altMetrics.BendCount <= metrics.BendCount+2) {
					points = altPoints
					metrics = altMetrics
					bestSource = altSource
					bestTarget = altTarget
				}
			}
		}
	}
	ignore := map[string]bool{bestSource.EntityID: true, bestTarget.EntityID: true}
	points = SimplifyPolylineWithObstacles(points, obstacles, ignore)
	points = EnsureMinimumVisibleRoute(points, bestSource, bestTarget, link)
	points = SimplifyPolylineWithObstacles(points, obstacles, ignore)
	metrics = ScorePolyline(points, bestSource, bestTarget, lane, existing, obstacles, opts)
	offset := ParallelOffset(laneIndex)
	route := Route{
		ID:             link.ID,
		From:           link.From,
		To:             link.To,
		Label:          link.Label,
		Role:           nonEmpty(link.Role, "secondary"),
		PathGroup:      link.PathGroup,
		FromPort:       bestSource.Side,
		ToPort:         bestTarget.Side,
		SourceEdgeIDs:  []string{link.ID},
		RouteScope:     "entity",
		FromZone:       from.Group,
		ToZone:         to.Group,
		FromEntity:     link.From,
		ToEntity:       link.To,
		TerminalMode:   "entity_port",
		Arrow:          "forward",
		Style:          RouteStyleFor(nonEmpty(link.Role, "secondary"), link.PathGroup),
		Points:         points,
		BusLaneID:      lane.ID,
		BundleID:       link.PathGroup,
		LaneIndex:      laneIndex,
		ParallelOffset: offset * 0.14,
		Metrics:        metrics,
	}
	route.Segments = PolylineSegments(route.Points)
	route.SpurStart, route.SpurEnd = RouteSpurs(route.Points)
	route.LabelAnchor = RouteLabelAnchor(route.Points, route.ParallelOffset)
	return route, true
}

func shouldUseBusLane(link LinkModel) bool {
	switch link.PathGroup {
	case "registry", "data", "cache", "storage", "health", "observability":
		return true
	case "gateway", "service":
		return link.Role == "primary"
	default:
		return link.Role == "primary"
	}
}

func shouldUseZoneBoundaryRoute(link LinkModel) bool {
	if link.Role == "primary" {
		return false
	}
	switch link.PathGroup {
	case "gateway", "service", "main", "registry", "data", "cache", "storage", "health", "observability":
		return true
	default:
		return strings.TrimSpace(link.PathGroup) != ""
	}
}

func EnsureMinimumVisibleRoute(points []Vec2, source, target Port, link LinkModel) []Vec2 {
	points = SimplifyPolyline(points)
	if len(points) < 2 || PolylineLength(points) >= minimumVisibleRouteLength(link.Role) {
		return points
	}
	start := points[0]
	end := points[len(points)-1]
	dx := end.X - start.X
	dy := end.Y - start.Y
	offset := minimumVisibleRouteDogleg(link.Role)
	if absFloat(dx) >= absFloat(dy) {
		sign := 1.0
		if end.Y >= start.Y {
			sign = -1
		}
		if absFloat(end.Y-start.Y) < 0.08 {
			if normalizeSide(source.Side) == "B" || normalizeSide(target.Side) == "B" {
				sign = 1
			}
		}
		return SimplifyPolyline([]Vec2{
			start,
			{X: start.X, Y: start.Y + offset*sign},
			{X: end.X, Y: start.Y + offset*sign},
			end,
		})
	}
	sign := 1.0
	if end.X >= start.X {
		sign = -1
	}
	if absFloat(end.X-start.X) < 0.08 {
		if normalizeSide(source.Side) == "R" || normalizeSide(target.Side) == "R" {
			sign = 1
		}
	}
	return SimplifyPolyline([]Vec2{
		start,
		{X: start.X + offset*sign, Y: start.Y},
		{X: start.X + offset*sign, Y: end.Y},
		end,
	})
}

func minimumVisibleRouteLength(role string) float64 {
	switch role {
	case "primary":
		return 0.92
	case "auxiliary":
		return 0.68
	default:
		return 0.78
	}
}

func minimumVisibleRouteDogleg(role string) float64 {
	switch role {
	case "primary":
		return 0.78
	case "auxiliary":
		return 0.52
	default:
		return 0.62
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func JoinPolylines(parts ...[]Vec2) []Vec2 {
	out := []Vec2{}
	for _, part := range parts {
		for _, point := range part {
			if len(out) == 0 || Distance(out[len(out)-1], point) > 0.035 {
				out = append(out, point)
			}
		}
	}
	return SimplifyPolyline(out)
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
