package routing

import (
	"container/heap"
	"math"
)

type RouteContext struct {
	Existing  []Route
	Obstacles []RouteObstacle
	Ignore    map[string]bool
	Lane      BusLane
	Weights   ScoreWeights
}

type astarState struct {
	Node int
	Dir  string
}

type astarItem struct {
	State astarState
	Cost  float64
	Total float64
	Index int
}

type astarQueue []*astarItem

func (q astarQueue) Len() int { return len(q) }

func (q astarQueue) Less(i, j int) bool { return q[i].Total < q[j].Total }

func (q astarQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].Index = i
	q[j].Index = j
}

func (q *astarQueue) Push(x any) {
	item := x.(*astarItem)
	item.Index = len(*q)
	*q = append(*q, item)
}

func (q *astarQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	item.Index = -1
	*q = old[:len(old)-1]
	return item
}

func AStarOrthogonalRoute(source, target Port, entities []EntityFrame, obstacles []RouteObstacle, lanes []BusLane, lane BusLane, existing []Route, opts Options) ([]Vec2, SingleRouteMetrics, bool) {
	grid := BuildHananGrid(source, target, entities, obstacles, lanes, opts.Clearance)
	startID, startOK := grid.Index[keyForPoint(source.Stub)]
	endID, endOK := grid.Index[keyForPoint(target.Stub)]
	if !startOK || !endOK {
		return nil, SingleRouteMetrics{}, false
	}
	start := astarState{Node: startID}
	open := &astarQueue{}
	heap.Init(open)
	heap.Push(open, &astarItem{State: start, Cost: 0, Total: Distance(source.Stub, target.Stub)})
	cost := map[astarState]float64{start: 0}
	prev := map[astarState]astarState{}
	endState := astarState{}
	found := false
	ignore := map[string]bool{source.EntityID: true, target.EntityID: true}
	ctx := RouteContext{Existing: existing, Obstacles: obstacles, Ignore: ignore, Lane: lane, Weights: opts.ScoreWeights}
	for open.Len() > 0 {
		cur := heap.Pop(open).(*astarItem)
		if cur.State.Node == endID {
			endState = cur.State
			found = true
			break
		}
		for _, neighborID := range grid.Neighbors(cur.State.Node, obstacles, ignore) {
			nextDir := segmentDirection(grid.Nodes[cur.State.Node], grid.Nodes[neighborID])
			next := astarState{Node: neighborID, Dir: nextDir}
			step := astarStepCost(grid.Nodes[cur.State.Node], grid.Nodes[neighborID], cur.State.Dir, nextDir, ctx)
			nextCost := cur.Cost + step
			if old, ok := cost[next]; ok && old <= nextCost {
				continue
			}
			cost[next] = nextCost
			prev[next] = cur.State
			heap.Push(open, &astarItem{
				State: next,
				Cost:  nextCost,
				Total: nextCost + Distance(grid.Nodes[neighborID], target.Stub),
			})
		}
	}
	if !found {
		return nil, SingleRouteMetrics{}, false
	}
	points := []Vec2{}
	for state := endState; ; state = prev[state] {
		points = append(points, grid.Nodes[state.Node])
		if state == start {
			break
		}
	}
	reversePoints(points)
	points = SimplifyPolyline(points)
	metrics := ScorePolyline(points, source, target, lane, existing, obstacles, opts)
	return points, metrics, true
}

func astarStepCost(a, b Vec2, incoming, outgoing string, ctx RouteContext) float64 {
	w := ctx.Weights
	if w.Length == 0 {
		w = DefaultOptions().ScoreWeights
	}
	step := Distance(a, b) * w.Length
	if incoming != "" && outgoing != "" && incoming != outgoing {
		step += w.Bend
	}
	for _, route := range ctx.Existing {
		for _, segment := range PolylineSegments(route.Points) {
			if SegmentsIntersect(a, b, segment.From, segment.To) {
				step += w.Crossing
			}
			if segmentsOverlap(a, b, segment.From, segment.To, 0.08) {
				step += w.Overlap
			}
		}
	}
	if ctx.Lane.ID != "" && !segmentTouchesLane(a, b, ctx.Lane) {
		step += w.WrongLane
	} else if ctx.Lane.ID != "" {
		step -= w.PreferredLaneReward
	}
	return math.Max(step, 0.001)
}

func ScorePolyline(points []Vec2, source, target Port, lane BusLane, existing []Route, obstacles []RouteObstacle, opts Options) SingleRouteMetrics {
	w := opts.ScoreWeights
	if w.Length == 0 {
		w = DefaultOptions().ScoreWeights
	}
	ignore := map[string]bool{source.EntityID: true, target.EntityID: true}
	length := PolylineLength(points)
	bends := Bends(points)
	entityIntersections := PolylineIntersectsObstacles(points, obstacles, ignore)
	score := length*w.Length + float64(bends)*w.Bend + float64(entityIntersections)*w.EntityIntersection
	if bends > 5 {
		excess := bends - 5
		score += float64(excess*excess) * w.Bend * 1.8
	}
	if lane.ID != "" && routeLaneViolation(points, lane) {
		score += w.WrongLane
	}
	for _, existingRoute := range existing {
		if routesCross(points, existingRoute.Points) {
			score += w.Crossing
		}
		if routesOverlap(points, existingRoute.Points, 0.08) {
			score += w.Overlap
		}
	}
	return SingleRouteMetrics{
		Length:              length,
		BendCount:           bends,
		EntityIntersections: entityIntersections,
		Score:               score,
	}
}

func BestCandidateRoute(source, target Port, link LinkModel, entities []EntityFrame, obstacles []RouteObstacle, lanes []BusLane, lane BusLane, existing []Route, opts Options) ([]Vec2, SingleRouteMetrics) {
	candidates := [][]Vec2{}
	if math.Abs(source.Stub.X-target.Stub.X) < 0.02 || math.Abs(source.Stub.Y-target.Stub.Y) < 0.02 {
		candidates = append(candidates, []Vec2{source.Stub, target.Stub})
	}
	candidates = append(candidates,
		[]Vec2{source.Stub, {X: target.Stub.X, Y: source.Stub.Y}, target.Stub},
		[]Vec2{source.Stub, {X: source.Stub.X, Y: target.Stub.Y}, target.Stub},
	)
	if lane.ID != "" {
		candidates = append(candidates, BusLaneRoute(source.Stub, target.Stub, lane, len(existing)))
	}
	if route, _, ok := AStarOrthogonalRoute(source, target, entities, obstacles, lanes, lane, existing, opts); ok {
		candidates = append(candidates, route)
	}
	best := []Vec2{source.Stub, target.Stub}
	bestScore := math.Inf(1)
	bestMetrics := SingleRouteMetrics{}
	for _, candidate := range candidates {
		route := SimplifyPolyline(candidate)
		metrics := ScorePolyline(route, source, target, lane, existing, obstacles, opts)
		if PortHintViolation(link, EntityFrame{Center: source.Point}, EntityFrame{Center: target.Point}) {
			metrics.Score += opts.ScoreWeights.PortViolation
		}
		if metrics.Score < bestScore {
			best = route
			bestScore = metrics.Score
			bestMetrics = metrics
		}
	}
	return best, bestMetrics
}

func BusLaneRoute(from, to Vec2, lane BusLane, index int) []Vec2 {
	if len(lane.Points) < 2 {
		return []Vec2{from, to}
	}
	offset := ParallelOffset(index) * 0.18
	if lane.Orientation == "vertical" {
		x := lane.Points[0].X + offset
		return []Vec2{from, {X: x, Y: from.Y}, {X: x, Y: to.Y}, to}
	}
	y := lane.Points[0].Y + offset
	return []Vec2{from, {X: from.X, Y: y}, {X: to.X, Y: y}, to}
}

func ParallelOffset(index int) float64 {
	if index == 0 {
		return 0
	}
	magnitude := float64((index+1)/2) * 0.34
	if index%2 == 0 {
		return -magnitude
	}
	return magnitude
}

func segmentDirection(a, b Vec2) string {
	if math.Abs(a.X-b.X) >= math.Abs(a.Y-b.Y) {
		if b.X >= a.X {
			return "R"
		}
		return "L"
	}
	if b.Y >= a.Y {
		return "B"
	}
	return "T"
}

func reversePoints(points []Vec2) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

func segmentTouchesLane(a, b Vec2, lane BusLane) bool {
	if lane.ID == "" || len(lane.Points) == 0 {
		return true
	}
	if lane.Orientation == "vertical" {
		x := lane.Points[0].X
		return math.Abs(a.X-x) < 0.08 || math.Abs(b.X-x) < 0.08
	}
	y := lane.Points[0].Y
	return math.Abs(a.Y-y) < 0.08 || math.Abs(b.Y-y) < 0.08
}

func routeLaneViolation(points []Vec2, lane BusLane) bool {
	if lane.ID == "" || len(points) < 2 {
		return false
	}
	for i := 0; i < len(points)-1; i++ {
		if segmentTouchesLane(points[i], points[i+1], lane) {
			return false
		}
	}
	return true
}
