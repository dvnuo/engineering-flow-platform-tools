package routing

import (
	"math"
	"sort"
)

type HananGrid struct {
	XS    []float64
	YS    []float64
	Nodes []Vec2
	Index map[gridKey]int
}

type gridKey struct {
	X int64
	Y int64
}

func BuildHananGrid(source, target Port, entities []EntityFrame, obstacles []RouteObstacle, lanes []BusLane, clearance float64) HananGrid {
	xs := []float64{source.Point.X, source.Stub.X, target.Point.X, target.Stub.X}
	ys := []float64{source.Point.Y, source.Stub.Y, target.Point.Y, target.Stub.Y}
	for _, entity := range entities {
		xs = append(xs, entity.Bounds.X-clearance, entity.Bounds.X, entity.Bounds.Right(), entity.Bounds.Right()+clearance, entity.Center.X)
		ys = append(ys, entity.Bounds.Y-clearance, entity.Bounds.Y, entity.Bounds.Bottom(), entity.Bounds.Bottom()+clearance, entity.Center.Y)
	}
	for _, obstacle := range obstacles {
		b := obstacle.Bounds
		xs = append(xs, b.X-clearance, b.X, b.Right(), b.Right()+clearance)
		ys = append(ys, b.Y-clearance, b.Y, b.Bottom(), b.Bottom()+clearance)
	}
	for _, lane := range lanes {
		for _, p := range lane.Points {
			xs = append(xs, p.X)
			ys = append(ys, p.Y)
		}
	}
	xs = uniqueSortedFloats(xs)
	ys = uniqueSortedFloats(ys)
	nodes := []Vec2{}
	index := map[gridKey]int{}
	for _, x := range xs {
		for _, y := range ys {
			p := Vec2{X: x, Y: y}
			if pointInAnyObstacle(p, obstacles, map[string]bool{source.EntityID: true, target.EntityID: true}) {
				continue
			}
			key := keyForPoint(p)
			index[key] = len(nodes)
			nodes = append(nodes, p)
		}
	}
	return HananGrid{XS: xs, YS: ys, Nodes: nodes, Index: index}
}

func (g HananGrid) Neighbors(index int, obstacles []RouteObstacle, ignore map[string]bool) []int {
	if index < 0 || index >= len(g.Nodes) {
		return nil
	}
	p := g.Nodes[index]
	out := []int{}
	if xi, yi, ok := g.coordIndex(p); ok {
		for left := xi - 1; left >= 0; left-- {
			if id, ok := g.Index[keyForXY(g.XS[left], p.Y)]; ok {
				if !SegmentIntersectsObstacles(p, g.Nodes[id], obstacles, ignore) {
					out = append(out, id)
				}
				break
			}
		}
		for right := xi + 1; right < len(g.XS); right++ {
			if id, ok := g.Index[keyForXY(g.XS[right], p.Y)]; ok {
				if !SegmentIntersectsObstacles(p, g.Nodes[id], obstacles, ignore) {
					out = append(out, id)
				}
				break
			}
		}
		for up := yi - 1; up >= 0; up-- {
			if id, ok := g.Index[keyForXY(p.X, g.YS[up])]; ok {
				if !SegmentIntersectsObstacles(p, g.Nodes[id], obstacles, ignore) {
					out = append(out, id)
				}
				break
			}
		}
		for down := yi + 1; down < len(g.YS); down++ {
			if id, ok := g.Index[keyForXY(p.X, g.YS[down])]; ok {
				if !SegmentIntersectsObstacles(p, g.Nodes[id], obstacles, ignore) {
					out = append(out, id)
				}
				break
			}
		}
	}
	return out
}

func (g HananGrid) coordIndex(p Vec2) (int, int, bool) {
	xi := sort.SearchFloat64s(g.XS, p.X)
	yi := sort.SearchFloat64s(g.YS, p.Y)
	if xi < len(g.XS) && yi < len(g.YS) && math.Abs(g.XS[xi]-p.X) < 0.001 && math.Abs(g.YS[yi]-p.Y) < 0.001 {
		return xi, yi, true
	}
	return 0, 0, false
}

func pointInAnyObstacle(p Vec2, obstacles []RouteObstacle, ignore map[string]bool) bool {
	for _, obstacle := range obstacles {
		if ignore != nil && ignore[obstacle.EntityID] {
			continue
		}
		if obstacle.Bounds.Contains(p) {
			return true
		}
	}
	return false
}

func uniqueSortedFloats(values []float64) []float64 {
	sort.Float64s(values)
	out := []float64{}
	for _, value := range values {
		if len(out) == 0 || math.Abs(out[len(out)-1]-value) > 0.02 {
			out = append(out, math.Round(value*1000)/1000)
		}
	}
	return out
}

func keyForPoint(p Vec2) gridKey {
	return keyForXY(p.X, p.Y)
}

func keyForXY(x, y float64) gridKey {
	return gridKey{X: int64(math.Round(x * 1000)), Y: int64(math.Round(y * 1000))}
}
