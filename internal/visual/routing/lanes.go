package routing

import "strings"

func PlanBusLanes(zones []ZoneFrame, links []LinkModel) []BusLane {
	service, ok := findZone(zones, "service")
	if !ok {
		service = allZoneBounds(zones)
	}
	all := allZoneBounds(zones)
	serviceMidY := service.Y + service.H*0.5
	serviceTop := service.Y
	serviceBottom := service.Bottom()
	serviceLeft := service.X
	serviceRight := service.Right()
	defs := []struct {
		group       string
		role        string
		orientation string
		points      []Vec2
	}{
		{"gateway", "primary", "horizontal", []Vec2{{X: all.X + 0.9, Y: serviceMidY}, {X: serviceLeft + service.W*0.42, Y: serviceMidY}}},
		{"service", "primary", "horizontal", []Vec2{{X: serviceLeft - 0.7, Y: serviceMidY + 0.15}, {X: serviceRight + 0.5, Y: serviceMidY + 0.15}}},
		{"registry", "secondary", "horizontal", []Vec2{{X: serviceLeft + service.W*0.52, Y: serviceTop - 1.0}, {X: serviceRight + 3.3, Y: serviceTop - 1.0}}},
		{"data", "secondary", "horizontal", []Vec2{{X: serviceLeft + service.W*0.35, Y: serviceBottom + 1.0}, {X: serviceRight + 1.4, Y: serviceBottom + 1.0}}},
		{"cache", "secondary", "horizontal", []Vec2{{X: serviceLeft + service.W*0.55, Y: serviceBottom + 0.55}, {X: serviceRight + 2.3, Y: serviceBottom + 0.55}}},
		{"storage", "secondary", "horizontal", []Vec2{{X: serviceLeft - 1.5, Y: serviceBottom + 1.65}, {X: serviceRight + 0.7, Y: serviceBottom + 1.65}}},
		{"health", "auxiliary", "horizontal", []Vec2{{X: serviceRight + 1.0, Y: serviceBottom + 2.35}, {X: all.Right() + 1.8, Y: serviceBottom + 2.35}}},
		{"observability", "auxiliary", "horizontal", []Vec2{{X: serviceLeft - 0.7, Y: serviceBottom + 2.0}, {X: all.Right() + 1.2, Y: serviceBottom + 2.0}}},
	}
	lanes := make([]BusLane, 0, len(defs))
	for i, def := range defs {
		lanes = append(lanes, BusLane{
			ID:          "lane-" + def.group,
			PathGroup:   def.group,
			Role:        def.role,
			Orientation: def.orientation,
			Points:      def.points,
			Bounds:      Bounds(def.points, 0.4),
			Index:       i,
		})
	}
	return lanes
}

func findZone(zones []ZoneFrame, category string) (Rect, bool) {
	category = strings.ToLower(category)
	for _, zone := range zones {
		key := strings.ToLower(zone.ID + " " + zone.Label + " " + zone.Kind)
		switch category {
		case "service":
			if strings.Contains(key, "service") || strings.Contains(key, "application") || strings.Contains(key, "app") {
				return zone.Bounds, true
			}
		}
	}
	return Rect{}, false
}

func allZoneBounds(zones []ZoneFrame) Rect {
	points := []Vec2{}
	for _, zone := range zones {
		b := zone.Bounds
		points = append(points, Vec2{X: b.X, Y: b.Y}, Vec2{X: b.Right(), Y: b.Bottom()})
	}
	if len(points) == 0 {
		return Rect{X: 0, Y: 0, W: 8, H: 6}
	}
	return Bounds(points, 0)
}

func LaneByPathGroup(lanes []BusLane) map[string]BusLane {
	out := map[string]BusLane{}
	for _, lane := range lanes {
		out[lane.PathGroup] = lane
	}
	return out
}
