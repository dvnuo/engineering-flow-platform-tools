package routing

import "math"

const epsilon = 1e-6

func Distance(a, b Vec2) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func (r Rect) Right() float64 {
	return r.X + r.W
}

func (r Rect) Bottom() float64 {
	return r.Y + r.H
}

func (r Rect) Center() Vec2 {
	return Vec2{X: r.X + r.W*0.5, Y: r.Y + r.H*0.5}
}

func (r Rect) Inflate(pad float64) Rect {
	return Rect{X: r.X - pad, Y: r.Y - pad, W: r.W + pad*2, H: r.H + pad*2}
}

func (r Rect) Contains(p Vec2) bool {
	return p.X >= r.X-epsilon && p.X <= r.Right()+epsilon && p.Y >= r.Y-epsilon && p.Y <= r.Bottom()+epsilon
}

func Bounds(points []Vec2, pad float64) Rect {
	if len(points) == 0 {
		return Rect{}
	}
	minX, maxX := points[0].X, points[0].X
	minY, maxY := points[0].Y, points[0].Y
	for _, p := range points[1:] {
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	return Rect{X: minX - pad, Y: minY - pad, W: maxX - minX + pad*2, H: maxY - minY + pad*2}
}

func SegmentIntersectsRect(a, b Vec2, rect Rect) bool {
	if rect.Contains(a) || rect.Contains(b) {
		return true
	}
	corners := []Vec2{
		{X: rect.X, Y: rect.Y},
		{X: rect.Right(), Y: rect.Y},
		{X: rect.Right(), Y: rect.Bottom()},
		{X: rect.X, Y: rect.Bottom()},
	}
	for i := range corners {
		if SegmentsIntersect(a, b, corners[i], corners[(i+1)%len(corners)]) {
			return true
		}
	}
	return false
}

func SegmentsIntersect(a, b, c, d Vec2) bool {
	orient := func(p, q, r Vec2) float64 {
		return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X)
	}
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	return o1*o2 < -epsilon && o3*o4 < -epsilon
}

func PolylineLength(points []Vec2) float64 {
	total := 0.0
	for i := 0; i < len(points)-1; i++ {
		total += Distance(points[i], points[i+1])
	}
	return total
}

func PolylineSegments(points []Vec2) []Segment {
	if len(points) < 2 {
		return nil
	}
	out := make([]Segment, 0, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		if Distance(points[i], points[i+1]) < 0.04 {
			continue
		}
		out = append(out, Segment{From: points[i], To: points[i+1], Kind: "orthogonal"})
	}
	return out
}

func SimplifyPolyline(points []Vec2) []Vec2 {
	if len(points) <= 2 {
		return append([]Vec2(nil), points...)
	}
	out := []Vec2{points[0]}
	for i := 1; i < len(points)-1; i++ {
		prev := out[len(out)-1]
		cur := points[i]
		next := points[i+1]
		if Distance(prev, cur) < 0.04 {
			continue
		}
		if nearlyCollinear(prev, cur, next) {
			continue
		}
		out = append(out, cur)
	}
	out = append(out, points[len(points)-1])
	return out
}

func SimplifyPolylineWithObstacles(points []Vec2, obstacles []RouteObstacle, ignore map[string]bool) []Vec2 {
	points = SimplifyPolyline(points)
	if len(points) <= 3 {
		return points
	}
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(points)-2 && !changed; i++ {
			for j := len(points) - 1; j >= i+2; j-- {
				if !sameAxis(points[i], points[j]) {
					continue
				}
				if segmentHitsObstacles(points[i], points[j], obstacles, ignore) {
					continue
				}
				next := make([]Vec2, 0, len(points)-(j-i-1))
				next = append(next, points[:i+1]...)
				next = append(next, points[j:]...)
				points = SimplifyPolyline(next)
				changed = true
				break
			}
		}
	}
	return points
}

func sameAxis(a, b Vec2) bool {
	return math.Abs(a.X-b.X) < 0.02 || math.Abs(a.Y-b.Y) < 0.02
}

func segmentHitsObstacles(a, b Vec2, obstacles []RouteObstacle, ignore map[string]bool) bool {
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

func nearlyCollinear(a, b, c Vec2) bool {
	abx, aby := b.X-a.X, b.Y-a.Y
	bcx, bcy := c.X-b.X, c.Y-b.Y
	return math.Abs(abx*bcy-aby*bcx) < 0.001
}

func Bends(points []Vec2) int {
	return maxInt(0, len(SimplifyPolyline(points))-2)
}

func RouteLabelAnchor(points []Vec2, offset float64) Vec2 {
	if len(points) == 0 {
		return Vec2{}
	}
	if len(points) == 1 {
		return points[0]
	}
	bestA := points[0]
	bestB := points[1]
	bestLen := Distance(bestA, bestB)
	for i := 1; i < len(points)-1; i++ {
		length := Distance(points[i], points[i+1])
		if length > bestLen {
			bestA, bestB, bestLen = points[i], points[i+1], length
		}
	}
	anchor := Vec2{X: (bestA.X + bestB.X) / 2, Y: (bestA.Y + bestB.Y) / 2}
	if bestLen > 0.01 {
		dx := bestB.X - bestA.X
		dy := bestB.Y - bestA.Y
		anchor.X += (-dy / bestLen) * (0.25 + offset*0.05)
		anchor.Y += (dx / bestLen) * (0.25 + offset*0.05)
	}
	return anchor
}

func RouteSpurs(points []Vec2) ([]Vec2, []Vec2) {
	if len(points) < 2 {
		return nil, nil
	}
	return []Vec2{points[0], points[1]}, []Vec2{points[len(points)-2], points[len(points)-1]}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
