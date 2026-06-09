package routing

const RoutePlanVersion = "efp.routeplan.v1"

type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type EntityFrame struct {
	ID     string `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Group  string `json:"group,omitempty"`
	Center Vec2   `json:"center"`
	Bounds Rect   `json:"bounds"`
	Rank   int    `json:"rank,omitempty"`
}

type ZoneFrame struct {
	ID     string `json:"id"`
	Kind   string `json:"kind,omitempty"`
	Label  string `json:"label,omitempty"`
	Bounds Rect   `json:"bounds"`
}

type Port struct {
	EntityID string `json:"entity_id"`
	Side     string `json:"side"`
	Point    Vec2   `json:"point"`
	Stub     Vec2   `json:"stub"`
}

type LinkModel struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	FromPort  string `json:"from_port,omitempty"`
	ToPort    string `json:"to_port,omitempty"`
	Label     string `json:"label,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Role      string `json:"role,omitempty"`
	PathGroup string `json:"path_group,omitempty"`
	Directed  bool   `json:"directed"`
}

type BusLane struct {
	ID          string `json:"id"`
	PathGroup   string `json:"pathGroup"`
	Role        string `json:"role"`
	Orientation string `json:"orientation"`
	Points      []Vec2 `json:"points"`
	Bounds      Rect   `json:"bounds"`
	Index       int    `json:"index"`
}

type RouteObstacle struct {
	ID       string  `json:"id"`
	EntityID string  `json:"entity_id"`
	Kind     string  `json:"kind,omitempty"`
	Bounds   Rect    `json:"bounds"`
	Padding  float64 `json:"padding"`
}

type Segment struct {
	From Vec2   `json:"from"`
	To   Vec2   `json:"to"`
	Kind string `json:"kind"`
}

type SingleRouteMetrics struct {
	Length                 float64 `json:"length"`
	BendCount              int     `json:"bend_count"`
	EntityIntersections    int     `json:"entity_intersections"`
	EndpointInsideEntities int     `json:"endpoint_inside_entities,omitempty"`
	Score                  float64 `json:"score"`
}

type Route struct {
	ID             string             `json:"id"`
	From           string             `json:"from"`
	To             string             `json:"to"`
	Role           string             `json:"role"`
	PathGroup      string             `json:"pathGroup"`
	FromPort       string             `json:"fromPort,omitempty"`
	ToPort         string             `json:"toPort,omitempty"`
	Points         []Vec2             `json:"points"`
	Segments       []Segment          `json:"segments"`
	BusLaneID      string             `json:"busLaneId,omitempty"`
	BundleID       string             `json:"bundleId,omitempty"`
	SpurStart      []Vec2             `json:"spurStart,omitempty"`
	SpurEnd        []Vec2             `json:"spurEnd,omitempty"`
	LabelAnchor    Vec2               `json:"labelAnchor"`
	LaneIndex      int                `json:"laneIndex,omitempty"`
	ParallelOffset float64            `json:"parallelOffset,omitempty"`
	Metrics        SingleRouteMetrics `json:"metrics"`
}

type RouteMetrics struct {
	PortHintViolations      int `json:"route_port_hint_violation_count"`
	DirectionViolations     int `json:"route_direction_violation_count"`
	EntityIntersections     int `json:"route_entity_intersection_count"`
	EndpointInsideEntities  int `json:"route_endpoint_inside_entity_count"`
	CrossingCount           int `json:"route_crossing_count"`
	ParallelOverlapCount    int `json:"route_parallel_overlap_count"`
	BusLaneCount            int `json:"route_bus_lane_count"`
	BundleCount             int `json:"route_bundle_count"`
	LongDetourCount         int `json:"route_long_detour_count"`
	PrimaryRouteCount       int `json:"primary_route_count"`
	SecondaryRouteCount     int `json:"secondary_route_count"`
	AuxiliaryRouteCount     int `json:"auxiliary_route_count"`
	PathGroupOverlap        int `json:"route_path_group_overlap_count"`
	ParallelOffsetCount     int `json:"route_parallel_offset_count"`
	RipUpRerouteRounds      int `json:"route_ripup_reroute_rounds,omitempty"`
	RipUpRerouteImprovement int `json:"route_ripup_reroute_improvement,omitempty"`
}

type RoutePlan struct {
	Version   string          `json:"version"`
	Backend   string          `json:"backend"`
	Routes    []Route         `json:"routes"`
	Lanes     []BusLane       `json:"lanes"`
	Bundles   []RouteBundle   `json:"bundles,omitempty"`
	Obstacles []RouteObstacle `json:"obstacles"`
	Metrics   RouteMetrics    `json:"metrics"`
}

type RouteBundle struct {
	ID        string   `json:"id"`
	PathGroup string   `json:"pathGroup"`
	RouteIDs  []string `json:"route_ids"`
}

type Input struct {
	Entities []EntityFrame `json:"entities"`
	Zones    []ZoneFrame   `json:"zones"`
	Links    []LinkModel   `json:"links"`
}

type Options struct {
	Engine       string
	Clearance    float64
	RipUpRounds  int
	UseBusLanes  bool
	UseNudging   bool
	UseRipUp     bool
	Existing     []Route
	ScoreWeights ScoreWeights
}

type ScoreWeights struct {
	Length              float64
	Bend                float64
	Crossing            float64
	Overlap             float64
	EntityIntersection  float64
	PortViolation       float64
	WrongLane           float64
	PreferredLaneReward float64
}

func DefaultOptions() Options {
	return Options{
		Engine:      "semantic_heuristic_v4",
		Clearance:   0.24,
		RipUpRounds: 2,
		UseBusLanes: true,
		UseNudging:  true,
		UseRipUp:    true,
		ScoreWeights: ScoreWeights{
			Length:              1,
			Bend:                10,
			Crossing:            40,
			Overlap:             80,
			EntityIntersection:  1000000,
			PortViolation:       1000,
			WrongLane:           25,
			PreferredLaneReward: 8,
		},
	}
}
