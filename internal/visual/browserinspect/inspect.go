package browserinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"engineering-flow-platform-tools/internal/visual/metadata"
	"engineering-flow-platform-tools/internal/visual/preview"
	"engineering-flow-platform-tools/internal/visual/renderinspect"
)

type Options struct {
	TemplateDir    string
	OutDir         string
	Screenshot     string
	OfflineStrict  bool
	TimeoutSeconds int
	BrowserPath    string
	Scenario       string
	EntityID       string
	DragX          float64
	DragZ          float64
	CameraTheta    float64
	CameraPhi      float64
	CameraZoom     float64
	OrbitSmoke     bool
}

type Result struct {
	OutDir         string            `json:"out_dir"`
	ServerURL      string            `json:"server_url"`
	ScreenshotPath string            `json:"screenshot_path"`
	Browser        string            `json:"browser"`
	BrowserReady   bool              `json:"browser_ready"`
	Ready          bool              `json:"ready"`
	RenderReady    bool              `json:"render_ready"`
	RenderScore    int               `json:"render_score"`
	VisualChecks   Checks            `json:"visual_checks"`
	VisualSummary  VisualSummary     `json:"visual_summary"`
	Warnings       []preview.Warning `json:"warnings"`
	DOM            DOMSummary        `json:"dom"`
	Requests       []string          `json:"requests"`
}

type Checks struct {
	PageLoaded                      bool `json:"page_loaded"`
	RuntimeDataLoaded               bool `json:"runtime_data_loaded"`
	RendererMounted                 bool `json:"renderer_mounted"`
	ScreenshotWritten               bool `json:"screenshot_written"`
	NoConsoleErrors                 bool `json:"no_console_errors"`
	NoNetworkErrors                 bool `json:"no_network_errors"`
	NoRemoteRequests                bool `json:"no_remote_requests"`
	IsometricStagePresent           bool `json:"isometric_stage_present"`
	LabelLayerPresent               bool `json:"label_layer_present"`
	EntityLabelsPresent             bool `json:"entity_labels_present"`
	LinkLabelsPresent               bool `json:"link_labels_present"`
	ZoneLabelsPresent               bool `json:"zone_labels_present"`
	LabelIconsPresent               bool `json:"label_icons_present"`
	ModelBadgesResolved             bool `json:"model_badges_resolved"`
	SvgBillboardsResolved           bool `json:"svg_billboards_resolved"`
	NoFallbackBadgesInGoodExample   bool `json:"no_fallback_badges_in_good_example"`
	ControlsPresent                 bool `json:"controls_present"`
	CanvasVisible                   bool `json:"canvas_visible"`
	ScreenshotNonBlank              bool `json:"screenshot_non_blank"`
	ScreenshotHasEnoughContrast     bool `json:"screenshot_has_enough_contrast"`
	ScreenshotHasExpectedLabelCount bool `json:"screenshot_has_expected_label_count"`
}

type DOMSummary struct {
	Title                   string   `json:"title,omitempty"`
	Template                string   `json:"template,omitempty"`
	Renderer                string   `json:"renderer,omitempty"`
	EntityLabels            int      `json:"entity_labels"`
	LinkLabels              int      `json:"link_labels"`
	ZoneLabels              int      `json:"zone_labels"`
	LabelIcons              int      `json:"label_icons"`
	LabelIconsLoaded        int      `json:"label_icons_loaded"`
	BrokenLabelIcons        int      `json:"broken_label_icons"`
	VisibleEntityLabels     int      `json:"visible_entity_labels"`
	VisibleLinkLabels       int      `json:"visible_link_labels"`
	VisibleZoneLabels       int      `json:"visible_zone_labels"`
	VisibleLabelIcons       int      `json:"visible_label_icons"`
	PrimaryLinkCount        int      `json:"primary_link_count"`
	SecondaryLinkCount      int      `json:"secondary_link_count"`
	AuxiliaryLinkCount      int      `json:"auxiliary_link_count"`
	VisiblePrimaryLabels    int      `json:"visible_primary_link_label_count"`
	VisibleSecondaryLabels  int      `json:"visible_secondary_link_label_count"`
	VisibleAuxiliaryLabels  int      `json:"visible_auxiliary_link_label_count"`
	ExplicitRouteLinks      int      `json:"explicit_route_link_count"`
	HeuristicRouteLinks     int      `json:"heuristic_route_link_count"`
	PrimaryExplicitRoutes   int      `json:"primary_explicit_route_count"`
	PrimaryVisibleLabels    int      `json:"primary_visible_label_count"`
	OverviewLinkLabels      int      `json:"overview_link_label_count"`
	RelationPaletteSize     int      `json:"relation_color_palette_size"`
	RelationPalette         []string `json:"relation_color_palette,omitempty"`
	VisibleAuxOpacityAvg    float64  `json:"visible_auxiliary_opacity_average,omitempty"`
	ZoneCountVisible        int      `json:"zone_count_visible"`
	RouteGroups             []string `json:"route_groups,omitempty"`
	InspectorRawDefault     bool     `json:"inspector_raw_json_default"`
	SVGRelationLayer        bool     `json:"svg_relation_layer_present"`
	SVGLinkPathCount        int      `json:"svg_link_path_count"`
	SVGPrimaryPathCount     int      `json:"svg_primary_link_path_count"`
	SVGSecondaryPathCount   int      `json:"svg_secondary_link_path_count"`
	SVGAuxiliaryPathCount   int      `json:"svg_auxiliary_link_path_count"`
	VisibleSVGPathCount     int      `json:"visible_svg_link_path_count"`
	LinkPathsWithMarker     int      `json:"link_paths_with_marker_count"`
	LinkPathsWithoutMarker  int      `json:"link_paths_without_marker_count"`
	EntityLabelOverlap      int      `json:"entity_label_overlap_count"`
	LinkLabelOverlap        int      `json:"link_label_overlap_count"`
	ZoneLabelOverlap        int      `json:"zone_label_overlap_count"`
	TotalLabelOverlap       int      `json:"total_label_overlap_count"`
	LabelsOutsideStage      int      `json:"labels_outside_stage_count"`
	ModelBadges             int      `json:"model_badges"`
	SvgBillboards           int      `json:"svg_billboards"`
	FallbackBadges          int      `json:"fallback_badges"`
	Controls                int      `json:"controls"`
	Canvas                  int      `json:"canvas"`
	RuntimeDataRequested    bool     `json:"runtime_data_requested"`
	RelationLayerMode       string   `json:"relation_layer_mode,omitempty"`
	WorldRelationLayer      bool     `json:"world_relation_layer_present"`
	GroundLinkMeshes        int      `json:"ground_link_mesh_count"`
	GroundLinkRibbons       int      `json:"ground_link_ribbon_count"`
	GroundLinkSegments      int      `json:"ground_link_segment_count"`
	GroundRouteRailSegments int      `json:"ground_route_rail_segment_count"`
	GroundRouteRailJoints   int      `json:"ground_route_rail_joint_count"`
	GroundRouteRailArrows   int      `json:"ground_route_rail_arrowhead_count"`
	GroundRouteRailVisible  int      `json:"ground_route_rail_visible_count"`
	IsolatedArrowheads      int      `json:"isolated_arrowhead_count"`
	RoutesWithSegments      int      `json:"routes_with_segments_count"`
	RoutesWithoutSegments   int      `json:"routes_without_segments_count"`
	VisibleGroundLinks      int      `json:"visible_ground_link_count"`
	GroundArrowheads        int      `json:"ground_arrowhead_count"`
	VisibleGroundArrowheads int      `json:"visible_ground_arrowhead_count"`
	GroundLinkHitAreas      int      `json:"ground_link_hit_area_count"`
	GenericLinkLabels       int      `json:"generic_link_label_count"`
	InferredLinkLabels      int      `json:"inferred_link_label_count"`
	ExplicitLinkLabels      int      `json:"explicit_link_label_count"`
	LinkLabelMode           string   `json:"link_label_mode,omitempty"`
	HTMLLinkLabels          int      `json:"html_link_label_count"`
	GroundLinkLabels        int      `json:"ground_link_label_mesh_count"`
	GroundTextureLinkLabels int      `json:"ground_texture_link_label_count"`
	GroundLinkTextures      int      `json:"ground_link_label_texture_ready_count"`
	GroundLabelsVisible     int      `json:"ground_link_label_visible_count"`
	GroundLabelsFlipped     int      `json:"ground_link_label_flipped_count"`
	ScreenSVGVisible        bool     `json:"screen_svg_relation_layer_visible"`
	SVGDebugLayer           bool     `json:"svg_debug_relation_layer_present"`
	EntityLabelAnchors      int      `json:"entity_label_anchor_count"`
	LinkLabelAnchors        int      `json:"link_label_anchor_count"`
	ZoneLabelAnchors        int      `json:"zone_label_anchor_count"`
	WorldLeaderLines        int      `json:"world_leader_line_count"`
	OrbitSmokeEnabled       bool     `json:"orbit_smoke_enabled"`
	OrbitEntityMaxDelta     float64  `json:"orbit_entity_label_return_max_delta_px,omitempty"`
	OrbitEntityAvgDelta     float64  `json:"orbit_entity_label_return_avg_delta_px,omitempty"`
	OrbitLinkMaxDelta       float64  `json:"orbit_link_label_return_max_delta_px,omitempty"`
	OrbitLinkAvgDelta       float64  `json:"orbit_link_label_return_avg_delta_px,omitempty"`
	OrbitMissingEntities    int      `json:"orbit_missing_entity_labels_after_rotate,omitempty"`
	OrbitMissingLinks       int      `json:"orbit_missing_link_labels_after_rotate,omitempty"`
	OrbitLayerStable        bool     `json:"orbit_relation_layer_mode_stable"`
}

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type VisualSummary struct {
	Template                       string         `json:"template"`
	ScreenshotPath                 string         `json:"screenshot_path"`
	EntityLabelCount               int            `json:"entity_label_count"`
	LabelIconCount                 int            `json:"label_icon_count"`
	LabelIconLoadedCount           int            `json:"label_icon_loaded_count"`
	BrokenLabelIconCount           int            `json:"broken_label_icon_count"`
	VisibleEntityLabelCount        int            `json:"visible_entity_label_count"`
	VisibleLinkLabelCount          int            `json:"visible_link_label_count"`
	VisibleZoneLabelCount          int            `json:"visible_zone_label_count"`
	VisibleLabelIconCount          int            `json:"visible_label_icon_count"`
	PrimaryLinkCount               int            `json:"primary_link_count"`
	SecondaryLinkCount             int            `json:"secondary_link_count"`
	AuxiliaryLinkCount             int            `json:"auxiliary_link_count"`
	VisiblePrimaryLinkLabelCount   int            `json:"visible_primary_link_label_count"`
	VisibleSecondaryLinkLabelCount int            `json:"visible_secondary_link_label_count"`
	VisibleAuxiliaryLinkLabelCount int            `json:"visible_auxiliary_link_label_count"`
	ExplicitRouteLinkCount         int            `json:"explicit_route_link_count"`
	HeuristicRouteLinkCount        int            `json:"heuristic_route_link_count"`
	PrimaryExplicitRouteCount      int            `json:"primary_explicit_route_count"`
	PrimaryVisibleLabelCount       int            `json:"primary_visible_label_count"`
	OverviewLinkLabelCount         int            `json:"overview_link_label_count"`
	RelationColorPaletteSize       int            `json:"relation_color_palette_size"`
	RelationColorPalette           []string       `json:"relation_color_palette,omitempty"`
	VisibleAuxiliaryOpacityAverage float64        `json:"visible_auxiliary_opacity_average,omitempty"`
	LinkOpacityBuckets             map[string]int `json:"link_opacity_buckets,omitempty"`
	ZoneCountVisible               int            `json:"zone_count_visible"`
	PrimaryPathGroupsVisible       []string       `json:"primary_path_groups_visible,omitempty"`
	RouteGroups                    []string       `json:"route_groups,omitempty"`
	InspectorRawJSONDefault        bool           `json:"inspector_raw_json_default"`
	SVGRelationLayerPresent        bool           `json:"svg_relation_layer_present"`
	SVGLinkPathCount               int            `json:"svg_link_path_count"`
	SVGPrimaryLinkPathCount        int            `json:"svg_primary_link_path_count"`
	SVGSecondaryLinkPathCount      int            `json:"svg_secondary_link_path_count"`
	SVGAuxiliaryLinkPathCount      int            `json:"svg_auxiliary_link_path_count"`
	VisibleSVGLinkPathCount        int            `json:"visible_svg_link_path_count"`
	RelationLayerBounds            *Rect          `json:"relation_layer_bbox,omitempty"`
	LinkPathsWithMarkerCount       int            `json:"link_paths_with_marker_count"`
	LinkPathsWithoutMarkerCount    int            `json:"link_paths_without_marker_count"`
	ModelBadgeCount                int            `json:"model_badge_count"`
	SvgBillboardCount              int            `json:"svg_billboard_count"`
	FallbackBadgeCount             int            `json:"fallback_badge_count"`
	CanvasVisible                  bool           `json:"canvas_visible"`
	ControlsVisible                bool           `json:"controls_visible"`
	ApproximateLabelOverlapCount   int            `json:"approximate_label_overlap_count"`
	EntityLabelOverlapCount        int            `json:"entity_label_overlap_count"`
	LinkLabelOverlapCount          int            `json:"link_label_overlap_count"`
	ZoneLabelOverlapCount          int            `json:"zone_label_overlap_count"`
	TotalLabelOverlapCount         int            `json:"total_label_overlap_count"`
	LabelsOutsideStageCount        int            `json:"labels_outside_stage_count"`
	LabelLayerBounds               *Rect          `json:"label_layer_bounds,omitempty"`
	CanvasBounds                   *Rect          `json:"canvas_bounds,omitempty"`
	ScreenshotSize                 *Rect          `json:"screenshot_size,omitempty"`
	RelationLayerMode              string         `json:"relation_layer_mode,omitempty"`
	WorldRelationLayerPresent      bool           `json:"world_relation_layer_present"`
	GroundLinkMeshCount            int            `json:"ground_link_mesh_count"`
	GroundLinkRibbonCount          int            `json:"ground_link_ribbon_count"`
	GroundLinkSegmentCount         int            `json:"ground_link_segment_count"`
	GroundRouteRailSegmentCount    int            `json:"ground_route_rail_segment_count"`
	GroundRouteRailJointCount      int            `json:"ground_route_rail_joint_count"`
	GroundRouteRailArrowheadCount  int            `json:"ground_route_rail_arrowhead_count"`
	GroundRouteRailVisibleCount    int            `json:"ground_route_rail_visible_count"`
	IsolatedArrowheadCount         int            `json:"isolated_arrowhead_count"`
	RoutesWithSegmentsCount        int            `json:"routes_with_segments_count"`
	RoutesWithoutSegmentsCount     int            `json:"routes_without_segments_count"`
	VisibleGroundLinkCount         int            `json:"visible_ground_link_count"`
	GroundArrowheadCount           int            `json:"ground_arrowhead_count"`
	VisibleGroundArrowheadCount    int            `json:"visible_ground_arrowhead_count"`
	GroundLinkHitAreaCount         int            `json:"ground_link_hit_area_count"`
	GenericLinkLabelCount          int            `json:"generic_link_label_count"`
	InferredLinkLabelCount         int            `json:"inferred_link_label_count"`
	ExplicitLinkLabelCount         int            `json:"explicit_link_label_count"`
	LinkLabelMode                  string         `json:"link_label_mode,omitempty"`
	HTMLLinkLabelCount             int            `json:"html_link_label_count"`
	GroundLinkLabelMeshCount       int            `json:"ground_link_label_mesh_count"`
	GroundTextureLinkLabelCount    int            `json:"ground_texture_link_label_count"`
	GroundLinkLabelTextureReady    int            `json:"ground_link_label_texture_ready_count"`
	GroundLinkLabelVisibleCount    int            `json:"ground_link_label_visible_count"`
	GroundLinkLabelFlippedCount    int            `json:"ground_link_label_flipped_count"`
	ScreenSVGRelationLayerVisible  bool           `json:"screen_svg_relation_layer_visible"`
	SVGDebugRelationLayerPresent   bool           `json:"svg_debug_relation_layer_present"`
	EntityLabelAnchorCount         int            `json:"entity_label_anchor_count"`
	LinkLabelAnchorCount           int            `json:"link_label_anchor_count"`
	ZoneLabelAnchorCount           int            `json:"zone_label_anchor_count"`
	WorldLeaderLineCount           int            `json:"world_leader_line_count"`
	OrbitSmokeEnabled              bool           `json:"orbit_smoke_enabled"`
	OrbitEntityLabelReturnMaxDelta float64        `json:"orbit_entity_label_return_max_delta_px,omitempty"`
	OrbitEntityLabelReturnAvgDelta float64        `json:"orbit_entity_label_return_avg_delta_px,omitempty"`
	OrbitLinkLabelReturnMaxDelta   float64        `json:"orbit_link_label_return_max_delta_px,omitempty"`
	OrbitLinkLabelReturnAvgDelta   float64        `json:"orbit_link_label_return_avg_delta_px,omitempty"`
	OrbitMissingEntityLabels       int            `json:"orbit_missing_entity_labels_after_rotate,omitempty"`
	OrbitMissingLinkLabels         int            `json:"orbit_missing_link_labels_after_rotate,omitempty"`
	OrbitRelationLayerModeStable   bool           `json:"orbit_relation_layer_mode_stable"`
}

func Inspect(opts Options) (Result, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return Result{}, metadata.NewError("output_path_invalid", "visual inspect-browser requires --out.", "Pass the visual artifact output directory produced by visual render.", 400)
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return Result{}, metadata.NewError("output_path_invalid", "visual output directory could not be resolved: "+err.Error(), "Pass a valid --out directory.", 400)
	}
	if info, err := os.Stat(outDir); err != nil || !info.IsDir() {
		return Result{}, metadata.NewError("output_path_invalid", "visual output directory does not exist.", "Run visual render before inspect-browser.", 404)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		return Result{}, metadata.NewError("visual_output_invalid", "visual output directory is missing index.html.", "Run visual render again before inspect-browser.", 400)
	}
	screenshot := strings.TrimSpace(opts.Screenshot)
	if screenshot == "" {
		screenshot = filepath.Join(outDir, "visual-screenshot.png")
	} else if !filepath.IsAbs(screenshot) {
		resolved, err := filepath.Abs(screenshot)
		if err != nil {
			return Result{}, metadata.NewError("screenshot_path_invalid", "screenshot path could not be resolved: "+err.Error(), "Pass a writable --screenshot path.", 400)
		}
		screenshot = resolved
	}
	if err := os.MkdirAll(filepath.Dir(screenshot), 0o755); err != nil {
		return Result{}, metadata.NewError("screenshot_path_invalid", "screenshot directory could not be created: "+err.Error(), "Pass --screenshot inside the visual output directory.", 400)
	}

	browserPath, err := findBrowser(opts.BrowserPath)
	if err != nil {
		return Result{}, err
	}

	server, err := startServer(outDir)
	if err != nil {
		return Result{}, err
	}
	defer server.Close()

	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := server.URL + "/index.html"
	if err := os.Remove(screenshot); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, metadata.NewError("screenshot_path_invalid", "existing screenshot could not be replaced: "+err.Error(), "Choose a writable screenshot path.", 400)
	}
	requests := server.Requests()
	nodeResult, err := runBrowserSmoke(ctx, browserPath, url, screenshot, opts)
	if err != nil {
		return Result{}, err
	}
	requests = mergeRequests(requests, nodeResult.Data.Requests)
	domSummary := DOMSummary{
		Title:                   nodeResult.Data.Summary.Title,
		Template:                nodeResult.Data.Summary.Template,
		Renderer:                nodeResult.Data.Summary.Renderer,
		EntityLabels:            nodeResult.Data.Summary.EntityLabels,
		LinkLabels:              nodeResult.Data.Summary.LinkLabels,
		ZoneLabels:              nodeResult.Data.Summary.ZoneLabels,
		LabelIcons:              nodeResult.Data.Summary.LabelIcons,
		LabelIconsLoaded:        nodeResult.Data.Summary.LabelIconsLoaded,
		BrokenLabelIcons:        nodeResult.Data.Summary.BrokenLabelIcons,
		VisibleEntityLabels:     nodeResult.Data.Summary.VisibleEntityLabels,
		VisibleLinkLabels:       nodeResult.Data.Summary.VisibleLinkLabels,
		VisibleZoneLabels:       nodeResult.Data.Summary.VisibleZoneLabels,
		VisibleLabelIcons:       nodeResult.Data.Summary.VisibleLabelIcons,
		PrimaryLinkCount:        nodeResult.Data.Summary.PrimaryLinkCount,
		SecondaryLinkCount:      nodeResult.Data.Summary.SecondaryLinkCount,
		AuxiliaryLinkCount:      nodeResult.Data.Summary.AuxiliaryLinkCount,
		VisiblePrimaryLabels:    nodeResult.Data.Summary.VisiblePrimaryLinkLabelCount,
		VisibleSecondaryLabels:  nodeResult.Data.Summary.VisibleSecondaryLinkLabelCount,
		VisibleAuxiliaryLabels:  nodeResult.Data.Summary.VisibleAuxiliaryLinkLabelCount,
		ExplicitRouteLinks:      nodeResult.Data.Summary.ExplicitRouteLinkCount,
		HeuristicRouteLinks:     nodeResult.Data.Summary.HeuristicRouteLinkCount,
		PrimaryExplicitRoutes:   nodeResult.Data.Summary.PrimaryExplicitRouteCount,
		PrimaryVisibleLabels:    nodeResult.Data.Summary.PrimaryVisibleLabelCount,
		OverviewLinkLabels:      nodeResult.Data.Summary.OverviewLinkLabelCount,
		RelationPaletteSize:     nodeResult.Data.Summary.RelationColorPaletteSize,
		RelationPalette:         nodeResult.Data.Summary.RelationColorPalette,
		VisibleAuxOpacityAvg:    nodeResult.Data.Summary.VisibleAuxiliaryOpacityAverage,
		ZoneCountVisible:        nodeResult.Data.Summary.ZoneCountVisible,
		RouteGroups:             nodeResult.Data.Summary.RouteGroups,
		InspectorRawDefault:     nodeResult.Data.Summary.InspectorRawJSONDefault,
		SVGRelationLayer:        nodeResult.Data.Summary.SVGRelationLayerPresent,
		SVGLinkPathCount:        nodeResult.Data.Summary.SVGLinkPathCount,
		SVGPrimaryPathCount:     nodeResult.Data.Summary.SVGPrimaryLinkPathCount,
		SVGSecondaryPathCount:   nodeResult.Data.Summary.SVGSecondaryLinkPathCount,
		SVGAuxiliaryPathCount:   nodeResult.Data.Summary.SVGAuxiliaryLinkPathCount,
		VisibleSVGPathCount:     nodeResult.Data.Summary.VisibleSVGLinkPathCount,
		LinkPathsWithMarker:     nodeResult.Data.Summary.LinkPathsWithMarkerCount,
		LinkPathsWithoutMarker:  nodeResult.Data.Summary.LinkPathsWithoutMarkerCount,
		EntityLabelOverlap:      nodeResult.Data.Summary.EntityLabelOverlapCount,
		LinkLabelOverlap:        nodeResult.Data.Summary.LinkLabelOverlapCount,
		ZoneLabelOverlap:        nodeResult.Data.Summary.ZoneLabelOverlapCount,
		TotalLabelOverlap:       nodeResult.Data.Summary.TotalLabelOverlapCount,
		LabelsOutsideStage:      nodeResult.Data.Summary.LabelsOutsideStageCount,
		ModelBadges:             nodeResult.Data.Summary.ModelBadges,
		SvgBillboards:           nodeResult.Data.Summary.SvgBillboards,
		FallbackBadges:          nodeResult.Data.Summary.FallbackBadges,
		Controls:                nodeResult.Data.Summary.Controls,
		Canvas:                  nodeResult.Data.Summary.Canvas,
		RuntimeDataRequested:    hasRequest(requests, "/data.js") && hasRequest(requests, "/manifest.js"),
		RelationLayerMode:       nodeResult.Data.Summary.RelationLayerMode,
		WorldRelationLayer:      nodeResult.Data.Summary.WorldRelationLayerPresent,
		GroundLinkMeshes:        nodeResult.Data.Summary.GroundLinkMeshCount,
		GroundLinkRibbons:       nodeResult.Data.Summary.GroundLinkRibbonCount,
		GroundLinkSegments:      nodeResult.Data.Summary.GroundLinkSegmentCount,
		GroundRouteRailSegments: nodeResult.Data.Summary.GroundRouteRailSegmentCount,
		GroundRouteRailJoints:   nodeResult.Data.Summary.GroundRouteRailJointCount,
		GroundRouteRailArrows:   nodeResult.Data.Summary.GroundRouteRailArrowheadCount,
		GroundRouteRailVisible:  nodeResult.Data.Summary.GroundRouteRailVisibleCount,
		IsolatedArrowheads:      nodeResult.Data.Summary.IsolatedArrowheadCount,
		RoutesWithSegments:      nodeResult.Data.Summary.RoutesWithSegmentsCount,
		RoutesWithoutSegments:   nodeResult.Data.Summary.RoutesWithoutSegmentsCount,
		VisibleGroundLinks:      nodeResult.Data.Summary.VisibleGroundLinkCount,
		GroundArrowheads:        nodeResult.Data.Summary.GroundArrowheadCount,
		VisibleGroundArrowheads: nodeResult.Data.Summary.VisibleGroundArrowheadCount,
		GroundLinkHitAreas:      nodeResult.Data.Summary.GroundLinkHitAreaCount,
		GenericLinkLabels:       nodeResult.Data.Summary.GenericLinkLabelCount,
		InferredLinkLabels:      nodeResult.Data.Summary.InferredLinkLabelCount,
		ExplicitLinkLabels:      nodeResult.Data.Summary.ExplicitLinkLabelCount,
		LinkLabelMode:           nodeResult.Data.Summary.LinkLabelMode,
		HTMLLinkLabels:          nodeResult.Data.Summary.HTMLLinkLabelCount,
		GroundLinkLabels:        nodeResult.Data.Summary.GroundLinkLabelMeshCount,
		GroundTextureLinkLabels: nodeResult.Data.Summary.GroundTextureLinkLabelCount,
		GroundLinkTextures:      nodeResult.Data.Summary.GroundLinkLabelTextureReady,
		GroundLabelsVisible:     nodeResult.Data.Summary.GroundLinkLabelVisibleCount,
		GroundLabelsFlipped:     nodeResult.Data.Summary.GroundLinkLabelFlippedCount,
		ScreenSVGVisible:        nodeResult.Data.Summary.ScreenSVGRelationLayerVisible,
		SVGDebugLayer:           nodeResult.Data.Summary.SVGDebugRelationLayerPresent,
		EntityLabelAnchors:      nodeResult.Data.Summary.EntityLabelAnchorCount,
		LinkLabelAnchors:        nodeResult.Data.Summary.LinkLabelAnchorCount,
		ZoneLabelAnchors:        nodeResult.Data.Summary.ZoneLabelAnchorCount,
		WorldLeaderLines:        nodeResult.Data.Summary.WorldLeaderLineCount,
		OrbitSmokeEnabled:       nodeResult.Data.Summary.OrbitSmokeEnabled,
		OrbitEntityMaxDelta:     nodeResult.Data.Summary.OrbitEntityLabelReturnMaxDelta,
		OrbitEntityAvgDelta:     nodeResult.Data.Summary.OrbitEntityLabelReturnAvgDelta,
		OrbitLinkMaxDelta:       nodeResult.Data.Summary.OrbitLinkLabelReturnMaxDelta,
		OrbitLinkAvgDelta:       nodeResult.Data.Summary.OrbitLinkLabelReturnAvgDelta,
		OrbitMissingEntities:    nodeResult.Data.Summary.OrbitMissingEntityLabels,
		OrbitMissingLinks:       nodeResult.Data.Summary.OrbitMissingLinkLabels,
		OrbitLayerStable:        nodeResult.Data.Summary.OrbitRelationLayerModeStable,
	}
	renderResult, renderWarnings := inspectRenderedScreenshot(opts, outDir, screenshot)
	checks := buildChecks(domSummary, requests, screenshot, nodeResult, renderResult)
	warnings := append(renderWarnings, warningsForChecks(checks, domSummary)...)
	sortWarnings(warnings)
	ready := checks.AllOK() && renderResult != nil && renderResult.Ready && !hasErrorWarnings(warnings)
	return Result{
		OutDir:         outDir,
		ServerURL:      url,
		ScreenshotPath: screenshot,
		Browser:        browserPath,
		BrowserReady:   checks.PageLoaded && checks.RendererMounted,
		Ready:          ready,
		RenderReady:    renderResult != nil && renderResult.Ready,
		RenderScore:    renderScore(renderResult),
		VisualChecks:   checks,
		VisualSummary:  buildVisualSummary(domSummary, screenshot, nodeResult, renderResult, checks),
		Warnings:       warnings,
		DOM:            domSummary,
		Requests:       requests,
	}, nil
}

type browserSmokeOutput struct {
	OK   bool `json:"ok"`
	Data struct {
		Summary        browserDOMSummary `json:"summary"`
		Requests       []string          `json:"requests"`
		RemoteRequests []string          `json:"remote_requests"`
		ConsoleErrors  []string          `json:"console_errors"`
		NetworkErrors  []string          `json:"network_errors"`
		Screenshot     string            `json:"screenshot"`
	} `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Hint    string `json:"hint"`
	} `json:"error"`
}

type browserDOMSummary struct {
	Title                          string         `json:"title"`
	Template                       string         `json:"template"`
	Renderer                       string         `json:"renderer"`
	IsometricReady                 bool           `json:"isometricReady"`
	Stage                          bool           `json:"stage"`
	LabelLayer                     bool           `json:"labelLayer"`
	EntityLabels                   int            `json:"entityLabels"`
	LinkLabels                     int            `json:"linkLabels"`
	ZoneLabels                     int            `json:"zoneLabels"`
	LabelIcons                     int            `json:"labelIcons"`
	LabelIconsLoaded               int            `json:"labelIconsLoaded"`
	BrokenLabelIcons               int            `json:"brokenLabelIcons"`
	VisibleEntityLabels            int            `json:"visibleEntityLabels"`
	VisibleLinkLabels              int            `json:"visibleLinkLabels"`
	VisibleZoneLabels              int            `json:"visibleZoneLabels"`
	VisibleLabelIcons              int            `json:"visibleLabelIcons"`
	PrimaryLinkCount               int            `json:"primaryLinkCount"`
	SecondaryLinkCount             int            `json:"secondaryLinkCount"`
	AuxiliaryLinkCount             int            `json:"auxiliaryLinkCount"`
	VisiblePrimaryLinkLabelCount   int            `json:"visiblePrimaryLinkLabelCount"`
	PrimaryVisibleLabelCount       int            `json:"primaryVisibleLabelCount"`
	VisibleSecondaryLinkLabelCount int            `json:"visibleSecondaryLinkLabelCount"`
	VisibleAuxiliaryLinkLabelCount int            `json:"visibleAuxiliaryLinkLabelCount"`
	ExplicitRouteLinkCount         int            `json:"explicitRouteLinkCount"`
	HeuristicRouteLinkCount        int            `json:"heuristicRouteLinkCount"`
	PrimaryExplicitRouteCount      int            `json:"primaryExplicitRouteCount"`
	OverviewLinkLabelCount         int            `json:"overviewLinkLabelCount"`
	RelationColorPaletteSize       int            `json:"relationColorPaletteSize"`
	RelationColorPalette           []string       `json:"relationColorPalette"`
	VisibleAuxiliaryOpacityAverage float64        `json:"visibleAuxiliaryOpacityAverage"`
	LinkOpacityBuckets             map[string]int `json:"linkOpacityBuckets"`
	ZoneCountVisible               int            `json:"zoneCountVisible"`
	PrimaryPathGroupsVisible       []string       `json:"primaryPathGroupsVisible"`
	RouteGroups                    []string       `json:"routeGroups"`
	InspectorRawJSONDefault        bool           `json:"inspectorRawJSONDefault"`
	SVGRelationLayerPresent        bool           `json:"svgRelationLayerPresent"`
	SVGLinkPathCount               int            `json:"svgLinkPathCount"`
	SVGPrimaryLinkPathCount        int            `json:"svgPrimaryLinkPathCount"`
	SVGSecondaryLinkPathCount      int            `json:"svgSecondaryLinkPathCount"`
	SVGAuxiliaryLinkPathCount      int            `json:"svgAuxiliaryLinkPathCount"`
	VisibleSVGLinkPathCount        int            `json:"visibleSvgLinkPathCount"`
	RelationLayerBounds            *Rect          `json:"relationLayerBounds"`
	LinkPathsWithMarkerCount       int            `json:"linkPathsWithMarkerCount"`
	LinkPathsWithoutMarkerCount    int            `json:"linkPathsWithoutMarkerCount"`
	ModelBadges                    int            `json:"modelBadges"`
	SvgBillboards                  int            `json:"svgBillboards"`
	FallbackBadges                 int            `json:"fallbackBadges"`
	Controls                       int            `json:"controls"`
	ControlBar                     bool           `json:"controlBar"`
	Canvas                         int            `json:"canvas"`
	ApproximateLabelOverlapCount   int            `json:"approximateLabelOverlapCount"`
	EntityLabelOverlapCount        int            `json:"entityLabelOverlapCount"`
	LinkLabelOverlapCount          int            `json:"linkLabelOverlapCount"`
	ZoneLabelOverlapCount          int            `json:"zoneLabelOverlapCount"`
	TotalLabelOverlapCount         int            `json:"totalLabelOverlapCount"`
	LabelsOutsideStageCount        int            `json:"labelsOutsideStageCount"`
	LabelLayerBounds               *Rect          `json:"labelLayerBounds"`
	CanvasBounds                   *Rect          `json:"canvasBounds"`
	ScreenshotSize                 *Rect          `json:"screenshotSize"`
	RelationLayerMode              string         `json:"relationLayerMode"`
	WorldRelationLayerPresent      bool           `json:"worldRelationLayerPresent"`
	GroundLinkMeshCount            int            `json:"groundLinkMeshCount"`
	GroundLinkRibbonCount          int            `json:"groundLinkRibbonCount"`
	GroundLinkSegmentCount         int            `json:"groundLinkSegmentCount"`
	GroundRouteRailSegmentCount    int            `json:"groundRouteRailSegmentCount"`
	GroundRouteRailJointCount      int            `json:"groundRouteRailJointCount"`
	GroundRouteRailArrowheadCount  int            `json:"groundRouteRailArrowheadCount"`
	GroundRouteRailVisibleCount    int            `json:"groundRouteRailVisibleCount"`
	IsolatedArrowheadCount         int            `json:"isolatedArrowheadCount"`
	RoutesWithSegmentsCount        int            `json:"routesWithSegmentsCount"`
	RoutesWithoutSegmentsCount     int            `json:"routesWithoutSegmentsCount"`
	VisibleGroundLinkCount         int            `json:"visibleGroundLinkCount"`
	GroundArrowheadCount           int            `json:"groundArrowheadCount"`
	VisibleGroundArrowheadCount    int            `json:"visibleGroundArrowheadCount"`
	GroundLinkHitAreaCount         int            `json:"groundLinkHitAreaCount"`
	GenericLinkLabelCount          int            `json:"genericLinkLabelCount"`
	InferredLinkLabelCount         int            `json:"inferredLinkLabelCount"`
	ExplicitLinkLabelCount         int            `json:"explicitLinkLabelCount"`
	LinkLabelMode                  string         `json:"linkLabelMode"`
	HTMLLinkLabelCount             int            `json:"htmlLinkLabelCount"`
	GroundLinkLabelMeshCount       int            `json:"groundLinkLabelMeshCount"`
	GroundTextureLinkLabelCount    int            `json:"groundTextureLinkLabelCount"`
	GroundLinkLabelTextureReady    int            `json:"groundLinkLabelTextureReadyCount"`
	GroundLinkLabelVisibleCount    int            `json:"groundLinkLabelVisibleCount"`
	GroundLinkLabelFlippedCount    int            `json:"groundLinkLabelFlippedCount"`
	ScreenSVGRelationLayerVisible  bool           `json:"screenSvgRelationLayerVisible"`
	SVGDebugRelationLayerPresent   bool           `json:"svgDebugRelationLayerPresent"`
	EntityLabelAnchorCount         int            `json:"entityLabelAnchorCount"`
	LinkLabelAnchorCount           int            `json:"linkLabelAnchorCount"`
	ZoneLabelAnchorCount           int            `json:"zoneLabelAnchorCount"`
	WorldLeaderLineCount           int            `json:"worldLeaderLineCount"`
	OrbitSmokeEnabled              bool           `json:"orbitSmokeEnabled"`
	OrbitEntityLabelReturnMaxDelta float64        `json:"orbitEntityLabelReturnMaxDeltaPx"`
	OrbitEntityLabelReturnAvgDelta float64        `json:"orbitEntityLabelReturnAvgDeltaPx"`
	OrbitLinkLabelReturnMaxDelta   float64        `json:"orbitLinkLabelReturnMaxDeltaPx"`
	OrbitLinkLabelReturnAvgDelta   float64        `json:"orbitLinkLabelReturnAvgDeltaPx"`
	OrbitMissingEntityLabels       int            `json:"orbitMissingEntityLabelsAfterRotate"`
	OrbitMissingLinkLabels         int            `json:"orbitMissingLinkLabelsAfterRotate"`
	OrbitRelationLayerModeStable   bool           `json:"orbitRelationLayerModeStable"`
	Ready                          bool           `json:"ready"`
}

func (c Checks) AllOK() bool {
	return c.PageLoaded &&
		c.RuntimeDataLoaded &&
		c.RendererMounted &&
		c.ScreenshotWritten &&
		c.NoConsoleErrors &&
		c.NoNetworkErrors &&
		c.NoRemoteRequests &&
		c.IsometricStagePresent &&
		c.LabelLayerPresent &&
		c.EntityLabelsPresent &&
		c.LinkLabelsPresent &&
		c.ZoneLabelsPresent &&
		c.LabelIconsPresent &&
		c.ModelBadgesResolved &&
		c.SvgBillboardsResolved &&
		c.NoFallbackBadgesInGoodExample &&
		c.ControlsPresent &&
		c.CanvasVisible &&
		c.ScreenshotNonBlank &&
		c.ScreenshotHasEnoughContrast &&
		c.ScreenshotHasExpectedLabelCount
}

type localServer struct {
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
	requests []string
	URL      string
}

func startServer(outDir string) (*localServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, metadata.NewError("browser_server_failed", "local visual preview server could not start: "+err.Error(), "Check that local loopback networking is available.", 500)
	}
	s := &localServer{listener: ln, URL: "http://" + ln.Addr().String()}
	files := http.FileServer(http.Dir(outDir))
	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			s.requests = append(s.requests, r.URL.Path)
			s.mu.Unlock()
			w.Header().Set("Cache-Control", "no-store")
			if r.URL.Path == "/favicon.ico" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			files.ServeHTTP(w, r)
		}),
	}
	go func() {
		_ = s.server.Serve(ln)
	}()
	return s, nil
}

func (s *localServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

func (s *localServer) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.requests...)
	sort.Strings(out)
	return out
}

func findBrowser(explicit string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, explicit)
	}
	if env := strings.TrimSpace(os.Getenv("EFP_BROWSER")); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates, "google-chrome", "chromium", "chromium-browser", "chrome", "microsoft-edge", "msedge")
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		)
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", metadata.NewError("browser_runtime_missing", "Chrome or Chromium was not found for visual inspect-browser.", "Install Chrome/Chromium, set EFP_BROWSER, or pass --browser <path>. In CI, set EFP_SKIP_BROWSER_SMOKE=1 only when browser smoke is intentionally unavailable.", 501)
}

func runBrowserSmoke(ctx context.Context, browserPath, url, screenshot string, opts Options) (browserSmokeOutput, error) {
	nodePath, err := findNode()
	if err != nil {
		return browserSmokeOutput{}, err
	}
	scriptPath, err := findBrowserSmokeScript()
	if err != nil {
		return browserSmokeOutput{}, err
	}
	timeoutSeconds := opts.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 90
	}
	args := []string{
		"--url", url,
		"--browser", browserPath,
		"--screenshot", screenshot,
		"--timeout", fmt.Sprintf("%d", timeoutSeconds),
	}
	if strings.TrimSpace(opts.Scenario) != "" {
		args = append(args, "--scenario", strings.TrimSpace(opts.Scenario))
	}
	if strings.TrimSpace(opts.EntityID) != "" {
		args = append(args, "--entity", strings.TrimSpace(opts.EntityID))
	}
	if opts.DragX != 0 {
		args = append(args, "--drag-x", fmt.Sprintf("%g", opts.DragX))
	}
	if opts.DragZ != 0 {
		args = append(args, "--drag-z", fmt.Sprintf("%g", opts.DragZ))
	}
	if opts.CameraTheta != 0 {
		args = append(args, "--camera-theta", fmt.Sprintf("%g", opts.CameraTheta))
	}
	if opts.CameraPhi != 0 {
		args = append(args, "--camera-phi", fmt.Sprintf("%g", opts.CameraPhi))
	}
	if opts.CameraZoom != 0 {
		args = append(args, "--camera-zoom", fmt.Sprintf("%g", opts.CameraZoom))
	}
	if opts.OrbitSmoke {
		args = append(args, "--orbit-smoke")
	}
	cmd := exec.CommandContext(ctx, nodePath, append([]string{scriptPath}, args...)...)
	out, err := cmd.CombinedOutput()
	var parsed browserSmokeOutput
	if jsonErr := json.Unmarshal(out, &parsed); jsonErr != nil {
		if err != nil {
			return browserSmokeOutput{}, metadata.NewError("browser_page_not_ready", "browser smoke helper failed: "+err.Error(), "Run scripts/visual/browser_smoke.mjs directly for diagnostics.", 500)
		}
		return browserSmokeOutput{}, metadata.NewError("browser_page_not_ready", "browser smoke helper returned invalid JSON.", "Inspect scripts/visual/browser_smoke.mjs output.", 500)
	}
	if err != nil || !parsed.OK {
		code := parsed.Error.Code
		if code == "" {
			code = "browser_page_not_ready"
		}
		message := parsed.Error.Message
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "browser smoke helper failed."
		}
		hint := parsed.Error.Hint
		if hint == "" {
			hint = "Ensure Chrome/Chromium can run headless and the rendered output is valid."
		}
		return browserSmokeOutput{}, metadata.NewError(code, message, hint, 500)
	}
	return parsed, nil
}

func findNode() (string, error) {
	if env := strings.TrimSpace(os.Getenv("EFP_NODE")); env != "" {
		if info, err := os.Stat(env); err == nil && !info.IsDir() {
			return env, nil
		}
	}
	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}
	return "", metadata.NewError("browser_runtime_missing", "Node.js was not found for visual inspect-browser.", "Install Node.js or set EFP_NODE to a Node executable. No npm dependencies are required.", 501)
}

func findBrowserSmokeScript() (string, error) {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			candidates = append(candidates, filepath.Join(dir, "scripts", "visual", "browser_smoke.mjs"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", metadata.NewError("browser_runtime_missing", "visual browser smoke helper script was not found.", "Run inspect-browser from the repository checkout or include scripts/visual/browser_smoke.mjs with the CLI distribution.", 501)
}

func mergeRequests(local, browser []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{local, browser} {
		for _, item := range list {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func buildChecks(summary DOMSummary, requests []string, screenshotPath string, nodeResult browserSmokeOutput, renderResult *renderinspect.Result) Checks {
	info, statErr := os.Stat(screenshotPath)
	renderChecks := renderinspect.Checks{}
	if renderResult != nil {
		renderChecks = renderResult.Checks
	}
	isAssetGallery := strings.Contains(summary.Title, "Local Logo Badge Gallery")
	labelThreshold := 1
	if isAssetGallery {
		labelThreshold = 14
	}
	return Checks{
		PageLoaded:                      summary.Template != "" || summary.EntityLabels > 0 || summary.Canvas > 0,
		RuntimeDataLoaded:               summary.RuntimeDataRequested,
		RendererMounted:                 nodeResult.Data.Summary.Ready && summary.Renderer == "offline.architecture.isometric.v1",
		ScreenshotWritten:               statErr == nil && info.Size() > 0,
		NoConsoleErrors:                 len(nodeResult.Data.ConsoleErrors) == 0,
		NoNetworkErrors:                 len(nodeResult.Data.NetworkErrors) == 0,
		NoRemoteRequests:                noRemoteRequests(requests) && len(nodeResult.Data.RemoteRequests) == 0,
		IsometricStagePresent:           nodeResult.Data.Summary.Stage,
		LabelLayerPresent:               nodeResult.Data.Summary.LabelLayer,
		EntityLabelsPresent:             summary.EntityLabels > 0 && summary.VisibleEntityLabels > 0,
		LinkLabelsPresent:               summary.LinkLabels > 0,
		ZoneLabelsPresent:               summary.ZoneLabels > 0,
		LabelIconsPresent:               summary.LabelIcons >= labelThreshold && summary.LabelIconsLoaded >= labelThreshold && summary.VisibleLabelIcons >= labelThreshold && summary.BrokenLabelIcons == 0,
		ModelBadgesResolved:             !isAssetGallery || summary.ModelBadges >= labelThreshold,
		SvgBillboardsResolved:           !isAssetGallery || summary.SvgBillboards >= labelThreshold,
		NoFallbackBadgesInGoodExample:   !isAssetGallery || summary.FallbackBadges == 0,
		ControlsPresent:                 summary.Controls > 0 && nodeResult.Data.Summary.ControlBar,
		CanvasVisible:                   summary.Canvas > 0,
		ScreenshotNonBlank:              renderChecks.ScreenshotNonBlank,
		ScreenshotHasEnoughContrast:     renderChecks.ScreenshotContrast,
		ScreenshotHasExpectedLabelCount: summary.VisibleEntityLabels >= labelThreshold,
	}
}

func inspectRenderedScreenshot(opts Options, outDir, screenshot string) (*renderinspect.Result, []preview.Warning) {
	result, err := renderinspect.Inspect(renderinspect.Options{
		TemplateDir:   opts.TemplateDir,
		OutDir:        outDir,
		Screenshot:    screenshot,
		OfflineStrict: opts.OfflineStrict,
	})
	if err != nil {
		return nil, []preview.Warning{{
			Code:       "browser_inspect_render_failed",
			Severity:   "error",
			Message:    "inspect-render could not inspect the browser-generated screenshot: " + err.Error(),
			Suggestion: "Inspect the rendered artifact and screenshot path, then rerun visual inspect-browser.",
			AutoFixHint: map[string]any{
				"action": "rerun_inspect_render",
			},
		}}
	}
	return &result, nil
}

func buildVisualSummary(summary DOMSummary, screenshot string, nodeResult browserSmokeOutput, renderResult *renderinspect.Result, checks Checks) VisualSummary {
	var screenshotSize *Rect
	if renderResult != nil && renderResult.Screenshot.Provided {
		screenshotSize = &Rect{
			X:      0,
			Y:      0,
			Width:  renderResult.Screenshot.Width,
			Height: renderResult.Screenshot.Height,
		}
	} else if nodeResult.Data.Summary.ScreenshotSize != nil {
		screenshotSize = nodeResult.Data.Summary.ScreenshotSize
	}
	return VisualSummary{
		Template:                       summary.Template,
		ScreenshotPath:                 screenshot,
		EntityLabelCount:               summary.EntityLabels,
		LabelIconCount:                 summary.LabelIcons,
		LabelIconLoadedCount:           summary.LabelIconsLoaded,
		BrokenLabelIconCount:           summary.BrokenLabelIcons,
		VisibleEntityLabelCount:        summary.VisibleEntityLabels,
		VisibleLinkLabelCount:          summary.VisibleLinkLabels,
		VisibleZoneLabelCount:          summary.VisibleZoneLabels,
		VisibleLabelIconCount:          summary.VisibleLabelIcons,
		PrimaryLinkCount:               summary.PrimaryLinkCount,
		SecondaryLinkCount:             summary.SecondaryLinkCount,
		AuxiliaryLinkCount:             summary.AuxiliaryLinkCount,
		VisiblePrimaryLinkLabelCount:   summary.VisiblePrimaryLabels,
		VisibleSecondaryLinkLabelCount: summary.VisibleSecondaryLabels,
		VisibleAuxiliaryLinkLabelCount: summary.VisibleAuxiliaryLabels,
		ExplicitRouteLinkCount:         summary.ExplicitRouteLinks,
		HeuristicRouteLinkCount:        summary.HeuristicRouteLinks,
		PrimaryExplicitRouteCount:      summary.PrimaryExplicitRoutes,
		PrimaryVisibleLabelCount:       summary.PrimaryVisibleLabels,
		OverviewLinkLabelCount:         summary.OverviewLinkLabels,
		RelationColorPaletteSize:       summary.RelationPaletteSize,
		RelationColorPalette:           summary.RelationPalette,
		VisibleAuxiliaryOpacityAverage: summary.VisibleAuxOpacityAvg,
		LinkOpacityBuckets:             nodeResult.Data.Summary.LinkOpacityBuckets,
		ZoneCountVisible:               summary.ZoneCountVisible,
		PrimaryPathGroupsVisible:       nodeResult.Data.Summary.PrimaryPathGroupsVisible,
		RouteGroups:                    nodeResult.Data.Summary.RouteGroups,
		InspectorRawJSONDefault:        summary.InspectorRawDefault,
		SVGRelationLayerPresent:        summary.SVGRelationLayer,
		SVGLinkPathCount:               summary.SVGLinkPathCount,
		SVGPrimaryLinkPathCount:        summary.SVGPrimaryPathCount,
		SVGSecondaryLinkPathCount:      summary.SVGSecondaryPathCount,
		SVGAuxiliaryLinkPathCount:      summary.SVGAuxiliaryPathCount,
		VisibleSVGLinkPathCount:        summary.VisibleSVGPathCount,
		RelationLayerBounds:            nodeResult.Data.Summary.RelationLayerBounds,
		LinkPathsWithMarkerCount:       summary.LinkPathsWithMarker,
		LinkPathsWithoutMarkerCount:    summary.LinkPathsWithoutMarker,
		ModelBadgeCount:                summary.ModelBadges,
		SvgBillboardCount:              summary.SvgBillboards,
		FallbackBadgeCount:             summary.FallbackBadges,
		CanvasVisible:                  checks.CanvasVisible,
		ControlsVisible:                checks.ControlsPresent,
		ApproximateLabelOverlapCount:   nodeResult.Data.Summary.ApproximateLabelOverlapCount,
		EntityLabelOverlapCount:        nodeResult.Data.Summary.EntityLabelOverlapCount,
		LinkLabelOverlapCount:          nodeResult.Data.Summary.LinkLabelOverlapCount,
		ZoneLabelOverlapCount:          nodeResult.Data.Summary.ZoneLabelOverlapCount,
		TotalLabelOverlapCount:         nodeResult.Data.Summary.TotalLabelOverlapCount,
		LabelsOutsideStageCount:        nodeResult.Data.Summary.LabelsOutsideStageCount,
		LabelLayerBounds:               nodeResult.Data.Summary.LabelLayerBounds,
		CanvasBounds:                   nodeResult.Data.Summary.CanvasBounds,
		ScreenshotSize:                 screenshotSize,
		RelationLayerMode:              summary.RelationLayerMode,
		WorldRelationLayerPresent:      summary.WorldRelationLayer,
		GroundLinkMeshCount:            summary.GroundLinkMeshes,
		GroundLinkRibbonCount:          summary.GroundLinkRibbons,
		GroundLinkSegmentCount:         summary.GroundLinkSegments,
		GroundRouteRailSegmentCount:    summary.GroundRouteRailSegments,
		GroundRouteRailJointCount:      summary.GroundRouteRailJoints,
		GroundRouteRailArrowheadCount:  summary.GroundRouteRailArrows,
		GroundRouteRailVisibleCount:    summary.GroundRouteRailVisible,
		IsolatedArrowheadCount:         summary.IsolatedArrowheads,
		RoutesWithSegmentsCount:        summary.RoutesWithSegments,
		RoutesWithoutSegmentsCount:     summary.RoutesWithoutSegments,
		VisibleGroundLinkCount:         summary.VisibleGroundLinks,
		GroundArrowheadCount:           summary.GroundArrowheads,
		VisibleGroundArrowheadCount:    summary.VisibleGroundArrowheads,
		GroundLinkHitAreaCount:         summary.GroundLinkHitAreas,
		GenericLinkLabelCount:          summary.GenericLinkLabels,
		InferredLinkLabelCount:         summary.InferredLinkLabels,
		ExplicitLinkLabelCount:         summary.ExplicitLinkLabels,
		LinkLabelMode:                  summary.LinkLabelMode,
		HTMLLinkLabelCount:             summary.HTMLLinkLabels,
		GroundLinkLabelMeshCount:       summary.GroundLinkLabels,
		GroundTextureLinkLabelCount:    summary.GroundTextureLinkLabels,
		GroundLinkLabelTextureReady:    summary.GroundLinkTextures,
		GroundLinkLabelVisibleCount:    summary.GroundLabelsVisible,
		GroundLinkLabelFlippedCount:    summary.GroundLabelsFlipped,
		ScreenSVGRelationLayerVisible:  summary.ScreenSVGVisible,
		SVGDebugRelationLayerPresent:   summary.SVGDebugLayer,
		EntityLabelAnchorCount:         summary.EntityLabelAnchors,
		LinkLabelAnchorCount:           summary.LinkLabelAnchors,
		ZoneLabelAnchorCount:           summary.ZoneLabelAnchors,
		WorldLeaderLineCount:           summary.WorldLeaderLines,
		OrbitSmokeEnabled:              summary.OrbitSmokeEnabled,
		OrbitEntityLabelReturnMaxDelta: summary.OrbitEntityMaxDelta,
		OrbitEntityLabelReturnAvgDelta: summary.OrbitEntityAvgDelta,
		OrbitLinkLabelReturnMaxDelta:   summary.OrbitLinkMaxDelta,
		OrbitLinkLabelReturnAvgDelta:   summary.OrbitLinkAvgDelta,
		OrbitMissingEntityLabels:       summary.OrbitMissingEntities,
		OrbitMissingLinkLabels:         summary.OrbitMissingLinks,
		OrbitRelationLayerModeStable:   summary.OrbitLayerStable,
	}
}

func warningsForChecks(checks Checks, summary DOMSummary) []preview.Warning {
	var out []preview.Warning
	add := func(code, severity, message, suggestion, action string) {
		out = append(out, preview.Warning{
			Code:       code,
			Severity:   severity,
			Message:    message,
			Suggestion: suggestion,
			AutoFixHint: map[string]any{
				"action": action,
			},
		})
	}
	if !checks.PageLoaded || !checks.RuntimeDataLoaded {
		add("browser_page_not_ready", "error", "The browser did not fully load the rendered artifact.", "Check index.html, manifest.js, data.js, and runtime asset paths.", "inspect_artifact_files")
	}
	if !checks.RendererMounted {
		add("browser_renderer_not_mounted", "error", "The isometric architecture renderer did not mount in the browser DOM.", "Ensure the output was rendered with architecture.isometric_overview and the runtime registers offline.architecture.isometric.v1.", "rerender_architecture_artifact")
	}
	if !checks.EntityLabelsPresent {
		add("browser_entity_labels_missing", "error", "No architecture entity labels were found in the browser DOM.", "Ensure entities render data-entity-id labels in the label layer.", "fix_entity_label_hooks")
	}
	if !checks.LabelIconsPresent {
		if summary.BrokenLabelIcons > 0 {
			add("browser_label_icons_broken", "error", "Some label icon images were present but failed to decode in the browser.", "Regenerate the artifact from valid local SVG assets and avoid stale output directories.", "rerender_with_valid_local_icons")
		} else {
			add("browser_label_icons_missing", "warning", "Expected local label icons were not resolved in the browser DOM.", "Use local presentation.icon IDs and renderHints.labelIcon=true.", "fix_label_icons")
		}
	}
	if !checks.ModelBadgesResolved {
		add("browser_model_badges_missing", "warning", "Expected generated model badges were not resolved in the browser DOM.", "Use local presentation.model IDs and renderHints.badgeMode=icon_and_model or model.", "fix_model_badges")
	}
	if !checks.NoRemoteRequests {
		add("browser_remote_request_detected", "error", "The browser smoke found remote URL references or non-local requests.", "Vendor assets locally and reference relative asset paths only.", "remove_remote_assets")
	}
	if !checks.NoNetworkErrors {
		add("browser_network_errors", "warning", "The headless browser reported local resource loading failures.", "Inspect local asset paths, copied template assets, and browser network logs.", "inspect_local_browser_requests")
	}
	if !checks.NoConsoleErrors {
		add("browser_console_errors", "warning", "The headless browser reported JavaScript console errors.", "Open the artifact in Chrome DevTools or inspect runtime JS for thrown errors.", "inspect_console_errors")
	}
	if !checks.ScreenshotNonBlank {
		add("browser_screenshot_blank", "error", "The browser screenshot appears blank.", "Ensure the renderer mounted and waited long enough before screenshot.", "rerun_browser_screenshot")
	}
	if !checks.ScreenshotHasExpectedLabelCount {
		add("browser_label_count_low", "warning", fmt.Sprintf("The browser DOM has fewer entity labels than expected: %d.", summary.EntityLabels), "Check label visibility hooks and first-view entity count.", "fix_label_count")
	}
	if summary.TotalLabelOverlap > 5 {
		add("browser_label_overlap_high", "warning", fmt.Sprintf("Visible labels overlap too much: total=%d entity=%d link=%d zone=%d.", summary.TotalLabelOverlap, summary.EntityLabelOverlap, summary.LinkLabelOverlap, summary.ZoneLabelOverlap), "Move entities apart, add label_offset values, or reduce low-priority label visibility.", "reduce_label_overlap")
	}
	if summary.VisibleLinkLabels > 10 {
		add("browser_link_label_density_high", "warning", fmt.Sprintf("Too many link labels are visible in the overview: %d.", summary.VisibleLinkLabels), "Set secondary links to visibility=detail or lower their labelPriority.", "reduce_overview_link_labels")
	}
	if summary.VisibleEntityLabels > 18 {
		add("browser_entity_label_density_high", "warning", fmt.Sprintf("Too many entity labels are visible in the overview: %d.", summary.VisibleEntityLabels), "Prioritize important entity labels and let lower-priority entities remain selectable without first-view labels.", "reduce_overview_entity_labels")
	}
	if summary.LabelsOutsideStage > 0 {
		add("browser_labels_outside_stage", "warning", fmt.Sprintf("Some labels are outside the screenshot viewport: %d.", summary.LabelsOutsideStage), "Adjust camera zoom, label offsets, or zone bounds so labels stay inside the first screenshot.", "fit_labels_inside_stage")
	}
	hasArchitectureFlowGroup := containsString(summary.RouteGroups, "entry") || containsString(summary.RouteGroups, "gateway")
	if summary.PrimaryLinkCount == 0 && hasArchitectureFlowGroup {
		add("browser_primary_path_missing", "warning", "No primary architecture path links were declared.", "Mark entry/gateway request path links as role=primary.", "declare_primary_links")
	}
	if summary.PrimaryLinkCount > 0 && summary.VisiblePrimaryLabels == 0 {
		add("browser_primary_link_labels_missing", "warning", "Primary architecture path links have no visible overview labels.", "Give at least one primary path link labelPriority=important or always.", "show_primary_path_labels")
	}
	totalDeclaredLinks := summary.PrimaryLinkCount + summary.SecondaryLinkCount + summary.AuxiliaryLinkCount
	if hasArchitectureFlowGroup && summary.PrimaryLinkCount > 0 && summary.PrimaryExplicitRoutes < summary.PrimaryLinkCount {
		add("browser_primary_links_without_explicit_route", "warning", fmt.Sprintf("Some primary architecture path links do not use explicit route points: %d/%d explicit.", summary.PrimaryExplicitRoutes, summary.PrimaryLinkCount), "Give every primary path a route with at least two grid points so the golden example remains hand laid out.", "add_primary_explicit_routes")
	}
	if hasArchitectureFlowGroup && summary.OverviewLinkLabels > 7 {
		add("browser_overview_link_labels_too_many", "warning", fmt.Sprintf("Too many relation labels are visible in the overview: %d.", summary.OverviewLinkLabels), "Keep the golden overview to 5-7 high-value relation labels.", "reduce_golden_overview_link_labels")
	}
	if hasArchitectureFlowGroup && summary.RelationPaletteSize > 5 {
		add("browser_relation_palette_too_noisy", "warning", fmt.Sprintf("The relation layer uses too many stroke colors: %d.", summary.RelationPaletteSize), "Constrain relation colors to primary, secondary, auxiliary, and one or two muted accents.", "reduce_relation_palette")
	}
	if hasArchitectureFlowGroup && totalDeclaredLinks >= 12 && summary.ExplicitRouteLinks < 12 {
		add("browser_heuristic_routes_too_many_for_golden_example", "warning", fmt.Sprintf("The golden architecture example has too few explicit relation routes: %d explicit, %d heuristic.", summary.ExplicitRouteLinks, summary.HeuristicRouteLinks), "Add explicit route arrays to at least 12 important links; prefer explicit routes for all golden example links.", "add_golden_explicit_routes")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode != "" && summary.RelationLayerMode != "world_ground" && summary.RelationLayerMode != "svg_debug" {
		add("browser_relation_layer_screen_space_default", "error", "The default relation layer mode is not world_ground.", "Set renderHints.relationLayer to world_ground so links, arrows, and link labels are stable Three.js world objects.", "set_world_ground_relation_layer")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "svg_debug" && !summary.SVGRelationLayer {
		add("browser_svg_relation_layer_missing", "error", "The SVG debug relation overlay layer is missing.", "Render debug relation routes through .visual-isometric-relation-svg only when relationLayer=svg_debug.", "restore_svg_debug_relation_layer")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "svg_debug" && summary.SVGLinkPathCount == 0 {
		add("browser_svg_link_paths_missing", "error", "The SVG debug relation overlay has no link paths.", "Project debug link routes into SVG path elements with data-link-id hooks.", "render_svg_debug_link_paths")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode != "svg_debug" && summary.ScreenSVGVisible {
		add("browser_svg_relation_layer_visible_in_default_mode", "error", "The screen-space SVG relation layer is visible in the default mode.", "Hide SVG relation overlay by default; use Three.js world-space ground links instead.", "hide_svg_relation_layer")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundLinkMeshes == 0 {
		add("browser_ground_links_missing", "error", "No world-space ground link meshes were reported.", "Render relation paths as Three.js world-space ground ribbons/tubes.", "render_ground_links")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundLinkSegments == 0 {
		add("browser_ground_route_segments_missing", "error", "No world-space ground route segments were reported.", "Render each relation path segment as a ground ribbon mesh instead of relying on screen-space SVG.", "render_ground_route_segments")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundRouteRailSegments == 0 {
		add("browser_route_segments_missing", "error", "No raised route rail segments were reported.", "Render each relation as raised world-space route rail geometry, not just arrowheads.", "render_raised_route_rails")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundArrowheads == 0 {
		add("browser_ground_arrowheads_missing", "error", "No world-space ground arrowheads were reported.", "Render directed relation arrows as Three.js world-space meshes attached to the route end.", "render_ground_arrowheads")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundArrowheads > 0 && summary.GroundLinkRibbons == 0 {
		add("browser_link_lines_missing_but_arrows_present", "error", "World-space arrowheads were reported but no ground relation ribbons were found.", "Render continuous ground route ribbons so arrows are attached to visible link lines.", "render_ground_link_ribbons")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundRouteRailArrows > 0 && summary.GroundRouteRailSegments == 0 {
		add("browser_link_lines_missing_but_arrows_present", "error", "Raised route arrowheads were reported but raised rail segments are missing.", "Create rectangular prism rail segments and joint caps for every visible route before adding arrowheads.", "render_route_rail_segments_before_arrows")
	}
	if totalDeclaredLinks > 1 && summary.RelationLayerMode == "world_ground" && summary.GroundRouteRailSegments > summary.RoutesWithSegments && summary.GroundRouteRailJoints == 0 {
		add("browser_route_joints_missing", "warning", "Raised route rails have no reported joint caps.", "Add small joint caps at route bends so orthogonal paths read as continuous engineering arrows.", "render_route_rail_joints")
	}
	if summary.RoutesWithoutSegments > 0 {
		add("browser_route_segments_missing", "warning", fmt.Sprintf("Some relation routes have no raised rail segments: %d.", summary.RoutesWithoutSegments), "Skip only zero-length routes; otherwise generate at least one rail segment per relation.", "fix_routes_without_segments")
	}
	if summary.GenericLinkLabels > 0 {
		add("browser_generic_link_labels_visible", "warning", fmt.Sprintf("Generic visible link labels were found: %d.", summary.GenericLinkLabels), "Do not default relation labels to 'link'; use explicit Mermaid edge labels or hide unlabeled relations.", "remove_generic_link_labels")
	}
	if totalDeclaredLinks > 0 && summary.VisibleLinkLabels == 0 && summary.ExplicitLinkLabels == 0 {
		add("browser_link_label_semantics_missing", "warning", "No meaningful explicit relation labels were found.", "Use Mermaid edge labels such as -->|API| or @link directives for important relationships.", "add_meaningful_mermaid_edge_labels")
	}
	if summary.IsolatedArrowheads > 0 {
		add("browser_link_lines_missing_but_arrows_present", "error", fmt.Sprintf("Some arrowheads are isolated from route rails: %d.", summary.IsolatedArrowheads), "Ensure every visible arrowhead belongs to a relation with generated rail segments.", "connect_arrowheads_to_rails")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.LinkLabelMode != "" && summary.LinkLabelMode != "html_billboard" {
		add("browser_link_label_mode_not_html_billboard", "error", "The default link label mode is not html_billboard.", "Use world-anchored HTML billboards for readable link labels; reserve ground texture labels for debug only.", "set_html_billboard_link_labels")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.HTMLLinkLabels == 0 {
		add("browser_link_label_anchor_missing", "warning", "No world-anchored HTML link labels were reported.", "Create link label DOM cards anchored to stable world route points.", "render_html_link_labels")
	}
	if summary.GroundTextureLinkLabels > 0 && summary.LinkLabelMode != "ground_texture_debug" {
		add("browser_ground_texture_link_labels_visible_in_default_mode", "error", "Ground texture link labels are visible outside ground_texture_debug mode.", "Disable CanvasTexture ground link labels by default and use HTML billboard labels.", "disable_ground_texture_link_labels")
	}
	if summary.VisibleEntityLabels > 0 && summary.EntityLabelAnchors < summary.VisibleEntityLabels {
		add("browser_label_anchor_missing", "warning", fmt.Sprintf("Some visible entity labels do not have stable world anchors: %d anchors for %d visible labels.", summary.EntityLabelAnchors, summary.VisibleEntityLabels), "Store stable data-anchor-* metadata and avoid per-frame label reallocation.", "fix_label_anchors")
	}
	if summary.VisibleEntityLabels > 0 && summary.WorldLeaderLines < summary.VisibleEntityLabels {
		add("browser_world_leader_lines_missing", "warning", fmt.Sprintf("World-space leader lines are fewer than visible entity labels: %d/%d.", summary.WorldLeaderLines, summary.VisibleEntityLabels), "Render entity leader lines as Three.js world-space dashed segments.", "render_world_leader_lines")
	}
	if summary.OrbitSmokeEnabled {
		if !summary.OrbitLayerStable {
			add("browser_orbit_relation_layer_unstable", "error", "Orbit smoke reported an unstable relation layer mode.", "Keep relationLayerMode stable during camera orbit.", "fix_orbit_relation_layer_mode")
		}
		if summary.OrbitMissingEntities > 0 || summary.OrbitMissingLinks > 0 {
			add("browser_orbit_labels_missing", "error", fmt.Sprintf("Orbit smoke lost labels after rotate: entities=%d links=%d.", summary.OrbitMissingEntities, summary.OrbitMissingLinks), "Keep label anchors stable and do not recreate labels during orbit.", "fix_orbit_label_retention")
		}
		if summary.OrbitEntityMaxDelta > 4 {
			add("browser_orbit_entity_label_jitter_high", "warning", fmt.Sprintf("Entity labels did not return to their initial projected positions after orbit: max %.2fpx.", summary.OrbitEntityMaxDelta), "Use stable world anchors and avoid collision layout during orbit drag.", "reduce_entity_label_jitter")
		}
		if summary.OrbitLinkMaxDelta > 5 {
			add("browser_orbit_link_label_jitter_high", "warning", fmt.Sprintf("Ground link labels did not return to their initial projected positions after orbit: max %.2fpx.", summary.OrbitLinkMaxDelta), "Keep ground link label anchors and route segment selection stable.", "reduce_link_label_jitter")
		}
	}
	if summary.VisibleAuxiliaryLabels > 2 {
		add("browser_auxiliary_links_too_prominent", "warning", fmt.Sprintf("Too many auxiliary link labels are visible: %d.", summary.VisibleAuxiliaryLabels), "Hide auxiliary labels in overview unless they explain an important exception.", "hide_auxiliary_labels")
	}
	if summary.ZoneLabels >= 8 && summary.ZoneCountVisible < 8 {
		add("browser_zone_count_low", "warning", fmt.Sprintf("Only %d architecture zone labels are visible.", summary.ZoneCountVisible), "Move zone labels to top-left edges or reduce competing entity labels.", "increase_visible_zone_labels")
	}
	if summary.InspectorRawDefault {
		add("browser_inspector_raw_json_default", "warning", "The default architecture inspector is still dominated by raw JSON.", "Render a summary-first inspector and collapse raw JSON details.", "use_summary_inspector")
	}
	if summary.VisibleLinkLabels > 8 {
		add("browser_link_labels_too_many", "warning", fmt.Sprintf("Too many relation labels are visible in overview: %d.", summary.VisibleLinkLabels), "Keep overview relation labels to primary and critical secondary paths.", "reduce_svg_relation_labels")
		add("browser_route_density_high", "warning", fmt.Sprintf("Too many relationship labels are visible in overview: %d.", summary.VisibleLinkLabels), "Keep overview labels to primary and critical data paths.", "reduce_route_label_density")
	}
	return out
}

func sortWarnings(warnings []preview.Warning) {
	sort.SliceStable(warnings, func(i, j int) bool {
		return warnings[i].Code < warnings[j].Code
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasErrorWarnings(warnings []preview.Warning) bool {
	for _, w := range warnings {
		if strings.EqualFold(w.Severity, "error") {
			return true
		}
	}
	return false
}

func renderScore(result *renderinspect.Result) int {
	if result == nil {
		return 0
	}
	return result.RenderScore
}

func hasRequest(requests []string, path string) bool {
	for _, req := range requests {
		if req == path || strings.HasSuffix(req, path) {
			return true
		}
	}
	return false
}

func noRemoteRequests(requests []string) bool {
	for _, req := range requests {
		if strings.HasPrefix(req, "http://127.0.0.1:") {
			continue
		}
		if strings.HasPrefix(req, "http://") || strings.HasPrefix(req, "https://") || strings.HasPrefix(req, "//") {
			return false
		}
	}
	return true
}
