package routing

import (
	"math"
	"strings"
)

func ResolvePorts(from, to EntityFrame, link LinkModel) (Port, Port) {
	fromSide := normalizeSide(link.FromPort)
	toSide := normalizeSide(link.ToPort)
	if fromSide == "" {
		fromSide = AutoPortSide(from.Center, to.Center, true)
	}
	if toSide == "" {
		toSide = AutoPortSide(to.Center, from.Center, false)
	}
	stub := StubLength(link.Role)
	return ResolvePort(from, fromSide, stub), ResolvePort(to, toSide, stub)
}

func ResolvePort(entity EntityFrame, side string, stubLength float64) Port {
	side = normalizeSide(side)
	if side == "" {
		side = "R"
	}
	b := entity.Bounds
	c := b.Center()
	point := c
	out := Vec2{}
	switch side {
	case "R":
		point = Vec2{X: b.Right(), Y: c.Y}
		out = Vec2{X: stubLength, Y: 0}
	case "L":
		point = Vec2{X: b.X, Y: c.Y}
		out = Vec2{X: -stubLength, Y: 0}
	case "T":
		point = Vec2{X: c.X, Y: b.Y}
		out = Vec2{X: 0, Y: -stubLength}
	case "B":
		point = Vec2{X: c.X, Y: b.Bottom()}
		out = Vec2{X: 0, Y: stubLength}
	}
	return Port{EntityID: entity.ID, Side: side, Point: point, Stub: Vec2{X: point.X + out.X, Y: point.Y + out.Y}}
}

func StubLength(role string) float64 {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "primary":
		return 0.45
	case "auxiliary":
		return 0.30
	default:
		return 0.35
	}
}

func AutoPortSide(from, to Vec2, source bool) string {
	dx := to.X - from.X
	dy := to.Y - from.Y
	if math.Abs(dx) >= math.Abs(dy) {
		if source {
			if dx >= 0 {
				return "R"
			}
			return "L"
		}
		if dx >= 0 {
			return "L"
		}
		return "R"
	}
	if source {
		if dy >= 0 {
			return "B"
		}
		return "T"
	}
	if dy >= 0 {
		return "T"
	}
	return "B"
}

func normalizeSide(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "R", "E", "EAST":
		return "R"
	case "L", "W", "WEST":
		return "L"
	case "T", "N", "NORTH":
		return "T"
	case "B", "S", "SOUTH":
		return "B"
	default:
		return ""
	}
}

func PortHintViolation(link LinkModel, from, to EntityFrame) bool {
	fromSide := normalizeSide(link.FromPort)
	toSide := normalizeSide(link.ToPort)
	if fromSide == "R" && toSide == "L" {
		return to.Center.X <= from.Center.X
	}
	if fromSide == "L" && toSide == "R" {
		return to.Center.X >= from.Center.X
	}
	if fromSide == "B" && toSide == "T" {
		return to.Center.Y <= from.Center.Y
	}
	if fromSide == "T" && toSide == "B" {
		return to.Center.Y >= from.Center.Y
	}
	return false
}

func UseZoneBoundaryRouting(input Input, from, to EntityFrame) bool {
	if len(input.Zones) < 6 || len(input.Links) < 10 {
		return false
	}
	if from.Group == "" || to.Group == "" || from.Group == to.Group {
		return false
	}
	return zoneFrameByID(input.Zones, from.Group).ID != "" && zoneFrameByID(input.Zones, to.Group).ID != ""
}

func ResolveZoneBoundaryPorts(zones []ZoneFrame, from, to EntityFrame, link LinkModel) (Port, Port) {
	fromZone := zoneFrameByID(zones, from.Group)
	toZone := zoneFrameByID(zones, to.Group)
	fromSide := normalizeSide(link.FromPort)
	toSide := normalizeSide(link.ToPort)
	if fromSide == "" {
		fromSide = AutoPortSide(fromZone.Bounds.Center(), toZone.Bounds.Center(), true)
	}
	if toSide == "" {
		toSide = AutoPortSide(toZone.Bounds.Center(), fromZone.Bounds.Center(), false)
	}
	stub := StubLength(link.Role)
	return ResolveRectPort(from.ID, fromZone.Bounds, fromSide, stub), ResolveRectPort(to.ID, toZone.Bounds, toSide, stub)
}

func ResolveRectPort(entityID string, bounds Rect, side string, stubLength float64) Port {
	side = normalizeSide(side)
	if side == "" {
		side = "R"
	}
	c := bounds.Center()
	point := c
	out := Vec2{}
	switch side {
	case "R":
		point = Vec2{X: bounds.Right(), Y: c.Y}
		out = Vec2{X: stubLength, Y: 0}
	case "L":
		point = Vec2{X: bounds.X, Y: c.Y}
		out = Vec2{X: -stubLength, Y: 0}
	case "T":
		point = Vec2{X: c.X, Y: bounds.Y}
		out = Vec2{X: 0, Y: -stubLength}
	case "B":
		point = Vec2{X: c.X, Y: bounds.Bottom()}
		out = Vec2{X: 0, Y: stubLength}
	}
	return Port{EntityID: entityID, Side: side, Point: point, Stub: Vec2{X: point.X + out.X, Y: point.Y + out.Y}}
}

func zoneFrameByID(zones []ZoneFrame, id string) ZoneFrame {
	for _, zone := range zones {
		if zone.ID == id {
			return zone
		}
	}
	return ZoneFrame{}
}
