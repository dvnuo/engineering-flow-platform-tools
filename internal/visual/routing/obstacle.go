package routing

func BuildObstacles(entities []EntityFrame, clearance float64) []RouteObstacle {
	if clearance <= 0 {
		clearance = 0.2
	}
	out := make([]RouteObstacle, 0, len(entities))
	for _, entity := range entities {
		out = append(out, RouteObstacle{
			ID:       "obstacle-" + entity.ID,
			EntityID: entity.ID,
			Kind:     entity.Kind,
			Bounds:   entity.Bounds.Inflate(clearance),
			Padding:  clearance,
		})
	}
	return out
}

func SegmentIntersectsObstacles(a, b Vec2, obstacles []RouteObstacle, ignore map[string]bool) bool {
	for _, obstacle := range obstacles {
		if ignore != nil && ignore[obstacle.EntityID] {
			continue
		}
		if SegmentIntersectsRect(a, b, obstacle.Bounds) {
			return true
		}
	}
	return false
}

func PolylineIntersectsObstacles(points []Vec2, obstacles []RouteObstacle, ignore map[string]bool) int {
	count := 0
	for _, obstacle := range obstacles {
		if ignore != nil && ignore[obstacle.EntityID] {
			continue
		}
		for i := 0; i < len(points)-1; i++ {
			if SegmentIntersectsRect(points[i], points[i+1], obstacle.Bounds) {
				count++
				break
			}
		}
	}
	return count
}

func EndpointInsideEntity(points []Vec2, entities map[string]EntityFrame, fromID, toID string) int {
	if len(points) == 0 {
		return 0
	}
	count := 0
	if from, ok := entities[fromID]; ok && from.Bounds.Contains(points[0]) {
		count++
	}
	if to, ok := entities[toID]; ok && to.Bounds.Contains(points[len(points)-1]) {
		count++
	}
	return count
}
