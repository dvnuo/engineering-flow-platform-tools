package routing

import "math"

func ApplyParallelNudging(routes []Route) ([]Route, int) {
	out := append([]Route(nil), routes...)
	count := 0
	for group, indexes := range groupRouteIndexes(out) {
		_ = group
		if len(indexes) <= 1 {
			continue
		}
		for order, routeIndex := range indexes {
			offset := ParallelOffset(order)
			if math.Abs(offset) < 0.001 {
				continue
			}
			step := 0.28
			out[routeIndex].Points = OffsetPolyline(out[routeIndex].Points, offset*step)
			out[routeIndex].Segments = PolylineSegments(out[routeIndex].Points)
			out[routeIndex].ParallelOffset += offset * step
			out[routeIndex].LabelAnchor = RouteLabelAnchor(out[routeIndex].Points, out[routeIndex].ParallelOffset)
			count++
		}
	}
	return out, count
}

func OffsetPolyline(points []Vec2, offset float64) []Vec2 {
	if len(points) < 2 || math.Abs(offset) < 0.001 {
		return append([]Vec2(nil), points...)
	}
	out := append([]Vec2(nil), points...)
	horizontal := 0.0
	vertical := 0.0
	for i := 0; i < len(out)-1; i++ {
		dx := math.Abs(out[i+1].X - out[i].X)
		dy := math.Abs(out[i+1].Y - out[i].Y)
		if dx >= dy {
			horizontal += dx
		} else {
			vertical += dy
		}
	}
	for i := range out {
		if horizontal >= vertical {
			out[i].Y += offset
		} else {
			out[i].X += offset
		}
	}
	return SimplifyPolyline(out)
}

func routesOverlap(a, b []Vec2, tolerance float64) bool {
	for _, sa := range PolylineSegments(a) {
		for _, sb := range PolylineSegments(b) {
			if segmentsOverlap(sa.From, sa.To, sb.From, sb.To, tolerance) {
				return true
			}
		}
	}
	return false
}

func segmentsOverlap(a, b, c, d Vec2, tolerance float64) bool {
	aHorizontal := math.Abs(a.Y-b.Y) < tolerance
	cHorizontal := math.Abs(c.Y-d.Y) < tolerance
	aVertical := math.Abs(a.X-b.X) < tolerance
	cVertical := math.Abs(c.X-d.X) < tolerance
	if aHorizontal && cHorizontal && math.Abs(a.Y-c.Y) < tolerance {
		return rangesOverlap(a.X, b.X, c.X, d.X)
	}
	if aVertical && cVertical && math.Abs(a.X-c.X) < tolerance {
		return rangesOverlap(a.Y, b.Y, c.Y, d.Y)
	}
	return false
}

func rangesOverlap(a1, a2, b1, b2 float64) bool {
	if a1 > a2 {
		a1, a2 = a2, a1
	}
	if b1 > b2 {
		b1, b2 = b2, b1
	}
	return math.Min(a2, b2)-math.Max(a1, b1) > 0.12
}
