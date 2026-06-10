package routing

import "sort"

func RipUpAndReroute(input Input, lanes []BusLane, obstacles []RouteObstacle, routes []Route, opts Options) ([]Route, int) {
	if opts.RipUpRounds <= 0 {
		return routes, 0
	}
	out := append([]Route(nil), routes...)
	improvements := 0
	for round := 0; round < opts.RipUpRounds; round++ {
		before := ValidateRoutes(input, out).CrossingCount + ValidateRoutes(input, out).ParallelOverlapCount + ValidateRoutes(input, out).EntityIntersections
		worst := worstRouteIndexes(input, out)
		if len(worst) == 0 {
			break
		}
		for _, idx := range worst {
			if idx < 0 || idx >= len(out) {
				continue
			}
			link := LinkModel{
				ID:        out[idx].ID,
				From:      out[idx].From,
				To:        out[idx].To,
				FromPort:  out[idx].FromPort,
				ToPort:    out[idx].ToPort,
				Role:      out[idx].Role,
				PathGroup: out[idx].PathGroup,
				Directed:  true,
			}
			existing := append([]Route(nil), out[:idx]...)
			existing = append(existing, out[idx+1:]...)
			rerouted, ok := routeSingle(input, link, lanes, obstacles, existing, opts, idx)
			if ok {
				out[idx] = rerouted
			}
		}
		after := ValidateRoutes(input, out).CrossingCount + ValidateRoutes(input, out).ParallelOverlapCount + ValidateRoutes(input, out).EntityIntersections
		if after < before {
			improvements += before - after
			continue
		}
		break
	}
	return out, improvements
}

func worstRouteIndexes(input Input, routes []Route) []int {
	type scored struct {
		Index int
		Score int
	}
	scores := []scored{}
	entities := entityMap(input.Entities)
	obstacles := BuildObstacles(input.Entities, 0.14)
	for i, route := range routes {
		score := PolylineIntersectsObstacles(route.Points, obstacles, map[string]bool{route.From: true, route.To: true}) * 1000
		if from, ok := entities[route.From]; ok {
			if to, ok := entities[route.To]; ok && PortHintViolation(LinkModel{FromPort: route.FromPort, ToPort: route.ToPort}, from, to) {
				score += 200
			}
		}
		for j, other := range routes {
			if i == j {
				continue
			}
			if routesCross(route.Points, other.Points) {
				score += 40
			}
			if route.PathGroup != "" && route.PathGroup == other.PathGroup && routesOverlap(route.Points, other.Points, 0.08) {
				score += 20
			}
		}
		if score > 0 {
			scores = append(scores, scored{Index: i, Score: score})
		}
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].Score > scores[j].Score })
	limit := 3
	if len(scores) < limit {
		limit = len(scores)
	}
	out := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scores[i].Index)
	}
	return out
}
