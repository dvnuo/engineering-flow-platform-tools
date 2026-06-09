package routing

import "testing"

func TestPortResolverUsesHints(t *testing.T) {
	from := EntityFrame{ID: "a", Center: Vec2{X: 0, Y: 0}, Bounds: Rect{X: -0.5, Y: -0.5, W: 1, H: 1}}
	to := EntityFrame{ID: "b", Center: Vec2{X: 4, Y: 0}, Bounds: Rect{X: 3.5, Y: -0.5, W: 1, H: 1}}
	source, target := ResolvePorts(from, to, LinkModel{FromPort: "R", ToPort: "L", Role: "primary"})
	if source.Side != "R" || target.Side != "L" {
		t.Fatalf("expected R/L ports, got %s/%s", source.Side, target.Side)
	}
	if source.Stub.X <= source.Point.X || target.Stub.X >= target.Point.X {
		t.Fatalf("expected outward stubs, got source=%+v target=%+v", source, target)
	}
}

func TestObstacleSegmentIntersection(t *testing.T) {
	rect := Rect{X: 1, Y: 1, W: 2, H: 2}
	if !SegmentIntersectsRect(Vec2{X: 0, Y: 2}, Vec2{X: 4, Y: 2}, rect) {
		t.Fatal("expected segment to intersect rect")
	}
	if SegmentIntersectsRect(Vec2{X: 0, Y: 0}, Vec2{X: 0.5, Y: 0}, rect) {
		t.Fatal("expected segment to avoid rect")
	}
}

func TestHananGridAvoidsObstacle(t *testing.T) {
	source := Port{EntityID: "a", Stub: Vec2{X: 0, Y: 0}, Point: Vec2{X: 0, Y: 0}}
	target := Port{EntityID: "b", Stub: Vec2{X: 4, Y: 0}, Point: Vec2{X: 4, Y: 0}}
	obstacles := []RouteObstacle{{EntityID: "block", Bounds: Rect{X: 1.5, Y: -0.5, W: 1, H: 1}}}
	grid := BuildHananGrid(source, target, nil, obstacles, nil, 0.3)
	for _, node := range grid.Nodes {
		if obstacles[0].Bounds.Contains(node) {
			t.Fatalf("grid node inside obstacle: %+v", node)
		}
	}
}

func TestAStarFindsOrthogonalRoute(t *testing.T) {
	entities := []EntityFrame{
		{ID: "a", Bounds: Rect{X: -0.5, Y: -0.5, W: 1, H: 1}, Center: Vec2{X: 0, Y: 0}},
		{ID: "b", Bounds: Rect{X: 4.5, Y: -0.5, W: 1, H: 1}, Center: Vec2{X: 5, Y: 0}},
		{ID: "block", Bounds: Rect{X: 2, Y: -0.5, W: 1, H: 1}, Center: Vec2{X: 2.5, Y: 0}},
	}
	source, target := ResolvePorts(entities[0], entities[1], LinkModel{FromPort: "R", ToPort: "L", Role: "primary"})
	obstacles := BuildObstacles(entities, 0.16)
	route, metrics, ok := AStarOrthogonalRoute(source, target, entities, obstacles, nil, BusLane{}, nil, DefaultOptions())
	if !ok {
		t.Fatal("expected A* route")
	}
	if metrics.EntityIntersections != 0 {
		t.Fatalf("expected route to avoid obstacle, got intersections=%d route=%+v", metrics.EntityIntersections, route)
	}
	if len(route) < 3 {
		t.Fatalf("expected obstacle detour with bends, got %+v", route)
	}
}

func TestBusLanePlannerCreatesExpectedLanes(t *testing.T) {
	lanes := PlanBusLanes([]ZoneFrame{{ID: "service", Bounds: Rect{X: 4, Y: 4, W: 6, H: 4}}}, nil)
	byGroup := LaneByPathGroup(lanes)
	for _, group := range []string{"gateway", "registry", "data", "cache", "storage", "health", "observability"} {
		if byGroup[group].ID == "" {
			t.Fatalf("missing lane for %s", group)
		}
	}
}

func TestParallelNudgingOffsetsOverlap(t *testing.T) {
	routes := []Route{
		{ID: "a", PathGroup: "data", Points: []Vec2{{X: 0, Y: 0}, {X: 4, Y: 0}}},
		{ID: "b", PathGroup: "data", Points: []Vec2{{X: 0, Y: 0}, {X: 4, Y: 0}}},
	}
	out, count := ApplyParallelNudging(routes)
	if count == 0 {
		t.Fatal("expected at least one nudge")
	}
	if !routesOverlap(routes[0].Points, routes[1].Points, 0.08) {
		t.Fatal("fixture should start overlapped")
	}
	if routesOverlap(out[0].Points, out[1].Points, 0.08) {
		t.Fatalf("expected nudged routes to reduce overlap: %+v", out)
	}
}

func TestRipUpRerouteReducesCrossing(t *testing.T) {
	input := Input{
		Entities: []EntityFrame{
			{ID: "a", Bounds: Rect{X: -0.5, Y: -0.5, W: 1, H: 1}, Center: Vec2{X: 0, Y: 0}},
			{ID: "b", Bounds: Rect{X: 3.5, Y: 3.5, W: 1, H: 1}, Center: Vec2{X: 4, Y: 4}},
			{ID: "c", Bounds: Rect{X: -0.5, Y: 3.5, W: 1, H: 1}, Center: Vec2{X: 0, Y: 4}},
			{ID: "d", Bounds: Rect{X: 3.5, Y: -0.5, W: 1, H: 1}, Center: Vec2{X: 4, Y: 0}},
		},
		Links: []LinkModel{
			{ID: "ab", From: "a", To: "b", Role: "secondary", PathGroup: "data", Directed: true},
			{ID: "cd", From: "c", To: "d", Role: "secondary", PathGroup: "cache", Directed: true},
		},
	}
	lanes := PlanBusLanes(input.Zones, input.Links)
	obstacles := BuildObstacles(input.Entities, 0.16)
	routes := []Route{
		{ID: "ab", From: "a", To: "b", Role: "secondary", PathGroup: "data", Points: []Vec2{{X: 0, Y: 0}, {X: 4, Y: 4}}},
		{ID: "cd", From: "c", To: "d", Role: "secondary", PathGroup: "cache", Points: []Vec2{{X: 0, Y: 4}, {X: 4, Y: 0}}},
	}
	before := CountRouteCrossings(routes)
	out, _ := RipUpAndReroute(input, lanes, obstacles, routes, DefaultOptions())
	after := CountRouteCrossings(out)
	if after > before {
		t.Fatalf("expected rip-up not to increase crossings: before=%d after=%d", before, after)
	}
}

func TestMicroserviceGoldenRoutePlanMetrics(t *testing.T) {
	input := Input{
		Zones: []ZoneFrame{{ID: "service", Bounds: Rect{X: 8, Y: 8, W: 7, H: 5}}},
		Entities: []EntityFrame{
			{ID: "browser", Bounds: Rect{X: 0, Y: 8, W: 1, H: 1}, Center: Vec2{X: 0.5, Y: 8.5}},
			{ID: "gateway", Bounds: Rect{X: 5, Y: 8, W: 1, H: 1}, Center: Vec2{X: 5.5, Y: 8.5}},
			{ID: "svc-a", Bounds: Rect{X: 10, Y: 8, W: 1, H: 1}, Center: Vec2{X: 10.5, Y: 8.5}},
			{ID: "svc-b", Bounds: Rect{X: 10, Y: 10, W: 1, H: 1}, Center: Vec2{X: 10.5, Y: 10.5}},
			{ID: "nacos", Bounds: Rect{X: 17, Y: 5, W: 1, H: 1}, Center: Vec2{X: 17.5, Y: 5.5}},
			{ID: "redis", Bounds: Rect{X: 17, Y: 11, W: 1, H: 1}, Center: Vec2{X: 17.5, Y: 11.5}},
			{ID: "mysql", Bounds: Rect{X: 15, Y: 14, W: 1, H: 1}, Center: Vec2{X: 15.5, Y: 14.5}},
		},
		Links: []LinkModel{
			{ID: "api", From: "browser", To: "gateway", FromPort: "R", ToPort: "L", Role: "primary", PathGroup: "gateway", Directed: true},
			{ID: "svc", From: "gateway", To: "svc-a", FromPort: "R", ToPort: "L", Role: "primary", PathGroup: "service", Directed: true},
			{ID: "reg-a", From: "svc-a", To: "nacos", Role: "secondary", PathGroup: "registry", Directed: true},
			{ID: "reg-b", From: "svc-b", To: "nacos", Role: "secondary", PathGroup: "registry", Directed: true},
			{ID: "cache", From: "svc-b", To: "redis", Role: "secondary", PathGroup: "cache", Directed: true},
			{ID: "data", From: "svc-a", To: "mysql", Role: "secondary", PathGroup: "data", Directed: true},
		},
	}
	plan := BuildRoutePlan(input, DefaultOptions())
	if plan.Version != RoutePlanVersion {
		t.Fatalf("unexpected route plan version %q", plan.Version)
	}
	if plan.Metrics.EntityIntersections != 0 {
		t.Fatalf("expected no entity intersections, got %+v", plan.Metrics)
	}
	if plan.Metrics.BusLaneCount < 3 {
		t.Fatalf("expected bus lanes, got %+v", plan.Metrics)
	}
	if len(plan.Routes) != len(input.Links) {
		t.Fatalf("expected routes for every link")
	}
}
