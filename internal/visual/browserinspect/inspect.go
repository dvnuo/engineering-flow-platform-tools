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
	Title                      string         `json:"title,omitempty"`
	Template                   string         `json:"template,omitempty"`
	Renderer                   string         `json:"renderer,omitempty"`
	EntityLabels               int            `json:"entity_labels"`
	LinkLabels                 int            `json:"link_labels"`
	ZoneLabels                 int            `json:"zone_labels"`
	LabelIcons                 int            `json:"label_icons"`
	LabelIconsLoaded           int            `json:"label_icons_loaded"`
	BrokenLabelIcons           int            `json:"broken_label_icons"`
	VisibleEntityLabels        int            `json:"visible_entity_labels"`
	VisibleLinkLabels          int            `json:"visible_link_labels"`
	VisibleZoneLabels          int            `json:"visible_zone_labels"`
	VisibleLabelIcons          int            `json:"visible_label_icons"`
	PrimaryLinkCount           int            `json:"primary_link_count"`
	SecondaryLinkCount         int            `json:"secondary_link_count"`
	AuxiliaryLinkCount         int            `json:"auxiliary_link_count"`
	VisiblePrimaryLabels       int            `json:"visible_primary_link_label_count"`
	VisibleSecondaryLabels     int            `json:"visible_secondary_link_label_count"`
	VisibleAuxiliaryLabels     int            `json:"visible_auxiliary_link_label_count"`
	ExplicitRouteLinks         int            `json:"explicit_route_link_count"`
	HeuristicRouteLinks        int            `json:"heuristic_route_link_count"`
	PrimaryExplicitRoutes      int            `json:"primary_explicit_route_count"`
	PrimaryVisibleLabels       int            `json:"primary_visible_label_count"`
	OverviewLinkLabels         int            `json:"overview_link_label_count"`
	RelationPaletteSize        int            `json:"relation_color_palette_size"`
	RelationPalette            []string       `json:"relation_color_palette,omitempty"`
	VisibleAuxOpacityAvg       float64        `json:"visible_auxiliary_opacity_average,omitempty"`
	ZoneCountVisible           int            `json:"zone_count_visible"`
	RouteGroups                []string       `json:"route_groups,omitempty"`
	InspectorRawDefault        bool           `json:"inspector_raw_json_default"`
	SVGRelationLayer           bool           `json:"svg_relation_layer_present"`
	SVGLinkPathCount           int            `json:"svg_link_path_count"`
	SVGPrimaryPathCount        int            `json:"svg_primary_link_path_count"`
	SVGSecondaryPathCount      int            `json:"svg_secondary_link_path_count"`
	SVGAuxiliaryPathCount      int            `json:"svg_auxiliary_link_path_count"`
	VisibleSVGPathCount        int            `json:"visible_svg_link_path_count"`
	LinkPathsWithMarker        int            `json:"link_paths_with_marker_count"`
	LinkPathsWithoutMarker     int            `json:"link_paths_without_marker_count"`
	EntityLabelOverlap         int            `json:"entity_label_overlap_count"`
	LinkLabelOverlap           int            `json:"link_label_overlap_count"`
	ZoneLabelOverlap           int            `json:"zone_label_overlap_count"`
	TotalLabelOverlap          int            `json:"total_label_overlap_count"`
	LabelsOutsideStage         int            `json:"labels_outside_stage_count"`
	LabelsUnderToolbar         int            `json:"labels_under_toolbar_count"`
	LabelsUnderInspector       int            `json:"labels_under_inspector_count"`
	CameraFitIncludesLabels    bool           `json:"camera_fit_includes_labels"`
	CameraFitReservedInspector bool           `json:"camera_fit_reserved_inspector_margin"`
	CameraFitReservedToolbar   bool           `json:"camera_fit_reserved_toolbar_margin"`
	CameraFitIncludesHTML      bool           `json:"camera_fit_includes_html_labels"`
	ModelBadges                int            `json:"model_badges"`
	SvgBillboards              int            `json:"svg_billboards"`
	FallbackBadges             int            `json:"fallback_badges"`
	PresentationMode           bool           `json:"presentation_mode"`
	Controls                   int            `json:"controls"`
	Canvas                     int            `json:"canvas"`
	RuntimeDataRequested       bool           `json:"runtime_data_requested"`
	SceneComponentTree         bool           `json:"scene_component_tree_present"`
	EntityComponents           int            `json:"entity_component_count"`
	RelationComponents         int            `json:"relation_component_count"`
	HTMLLabelComponents        int            `json:"html_label_component_count"`
	LeaderLineComponents       int            `json:"leader_line_component_count"`
	GroundPathBuilder          bool           `json:"ground_path_builder_present"`
	GroundPathBuilderVersion   string         `json:"ground_path_builder_version,omitempty"`
	PathJoinStyle              string         `json:"path_join_style,omitempty"`
	PathArrowCapCount          int            `json:"path_arrow_cap_count"`
	PathArrowCapIntegrated     int            `json:"path_arrow_cap_integrated_count"`
	PathHitAreaCount           int            `json:"path_hit_area_count"`
	PathHoverHaloSupported     bool           `json:"path_hover_halo_supported"`
	PathParallelOffsetCount    int            `json:"path_parallel_offset_count"`
	PathBundleCount            int            `json:"path_bundle_count"`
	PathDashSegmentCount       int            `json:"path_dash_segment_count"`
	RoutePlanPresent           bool           `json:"route_plan_present"`
	RoutePlanVersion           string         `json:"route_plan_version,omitempty"`
	RoutePlanBackend           string         `json:"route_plan_backend,omitempty"`
	RoutePlanRouteCount        int            `json:"route_plan_route_count"`
	RoutePlanLaneCount         int            `json:"route_plan_lane_count"`
	RoutePlanObstacleCount     int            `json:"route_plan_obstacle_count"`
	RoutePlanRenderedMatch     bool           `json:"route_plan_rendered_match"`
	RoutePlanRenderedMatchCnt  int            `json:"route_plan_rendered_match_count"`
	SourceEdgeCount            int            `json:"source_edge_count"`
	DisplayRouteCount          int            `json:"display_route_count"`
	HiddenDetailRouteCount     int            `json:"hidden_detail_route_count"`
	RouteToZoneCount           int            `json:"route_to_zone_count"`
	RouteToEntityCount         int            `json:"route_to_entity_count"`
	RouteToZoneRatio           float64        `json:"route_to_zone_ratio"`
	RouteSameStyleMismatch     int            `json:"route_same_style_mismatch_count"`
	PathArrowBodyGapCount      int            `json:"path_arrow_body_gap_count"`
	PathArrowAtBendCount       int            `json:"path_arrow_at_bend_count"`
	RouteColorConsistencyScore float64        `json:"route_color_consistency_score"`
	EntityBodyRegistryCount    int            `json:"entity_body_registry_count"`
	EntityKnownBodyCount       int            `json:"entity_known_body_count"`
	EntityGenericBodyCount     int            `json:"entity_generic_body_count"`
	EntityGenericBodyRatio     float64        `json:"entity_generic_body_ratio"`
	EntitySemanticBodyScore    float64        `json:"entity_semantic_body_score"`
	EntityVisualStyleVersion   string         `json:"entity_visual_style_version,omitempty"`
	EntityVisualPaletteVersion int            `json:"entity_visual_palette_version"`
	EntityBodyShapeVariety     int            `json:"entity_body_shape_variety_count"`
	EntityContactShadows       int            `json:"entity_contact_shadow_count"`
	EntityTopHighlights        int            `json:"entity_top_highlight_count"`
	EntitySidePanels           int            `json:"entity_side_panel_count"`
	EntityIconDecals           int            `json:"entity_icon_decal_count"`
	EntityRoundedOrBeveled     int            `json:"entity_rounded_or_beveled_count"`
	EntityScreenPanels         int            `json:"entity_screen_panel_count"`
	EntitySemanticModelCount   int            `json:"entity_semantic_model_coverage_count"`
	EntitySemanticModelRatio   float64        `json:"entity_semantic_model_coverage_ratio"`
	EntityBrightnessScore      float64        `json:"entity_brightness_score"`
	EntitySaturationScore      float64        `json:"entity_saturation_score"`
	EntityChromaScore          float64        `json:"entity_chroma_score"`
	NeutralGrayMaterialRatio   float64        `json:"neutral_gray_material_ratio"`
	ZoneEntityOverflowCount    int            `json:"zone_entity_overflow_count"`
	ZoneLabelOverflowCount     int            `json:"zone_label_overflow_count"`
	ZonePaddingMinPx           int            `json:"zone_padding_min_px"`
	ModelKindCounts            map[string]int `json:"model_kind_counts,omitempty"`
	RelationOwnsPath           int            `json:"relation_components_own_path_count"`
	RelationOwnsArrow          int            `json:"relation_components_own_arrow_count"`
	RelationOwnsHit            int            `json:"relation_components_own_hit_count"`
	RelationOwnsLabel          int            `json:"relation_components_own_label_count"`
	EntityComponentsWithPorts  int            `json:"entity_components_with_ports_count"`
	RelationLayerMode          string         `json:"relation_layer_mode,omitempty"`
	RelationRenderMode         string         `json:"relation_render_mode,omitempty"`
	RelationDepthEnabled       int            `json:"relation_depth_test_enabled_count"`
	RelationDepthDisabled      int            `json:"relation_depth_test_disabled_count"`
	RouteEntityIntersections   int            `json:"route_entity_intersection_count"`
	RoutePortViolations        int            `json:"route_port_hint_violation_count"`
	RouteDirectionViolations   int            `json:"route_direction_violation_count"`
	RouteMaxLengthWorld        float64        `json:"route_max_length_world,omitempty"`
	RouteCrossSceneCount       int            `json:"route_cross_scene_count"`
	RouteCrossingCount         int            `json:"route_crossing_count"`
	RouteParallelOverlapCount  int            `json:"route_parallel_overlap_count"`
	RoutePathGroupOverlapCount int            `json:"route_path_group_overlap_count"`
	RouteBusLaneCount          int            `json:"route_bus_lane_count"`
	RouteBundleCount           int            `json:"route_bundle_count"`
	PrimaryRouteCount          int            `json:"primary_route_count"`
	SecondaryRouteCount        int            `json:"secondary_route_count"`
	AuxiliaryRouteCount        int            `json:"auxiliary_route_count"`
	RaisedBeamLooks            int            `json:"relation_looks_like_raised_beam_count"`
	WorldRelationLayer         bool           `json:"world_relation_layer_present"`
	GroundLinkMeshes           int            `json:"ground_link_mesh_count"`
	GroundLinkRibbons          int            `json:"ground_link_ribbon_count"`
	GroundLinkSegments         int            `json:"ground_link_segment_count"`
	GroundRouteRailSegments    int            `json:"ground_route_rail_segment_count"`
	GroundRouteRailJoints      int            `json:"ground_route_rail_joint_count"`
	GroundRouteRailArrows      int            `json:"ground_route_rail_arrowhead_count"`
	GroundRouteRailVisible     int            `json:"ground_route_rail_visible_count"`
	IsolatedArrowheads         int            `json:"isolated_arrowhead_count"`
	RoutesWithSegments         int            `json:"routes_with_segments_count"`
	RoutesWithoutSegments      int            `json:"routes_without_segments_count"`
	VisibleGroundLinks         int            `json:"visible_ground_link_count"`
	GroundArrowheads           int            `json:"ground_arrowhead_count"`
	VisibleGroundArrowheads    int            `json:"visible_ground_arrowhead_count"`
	GroundLinkHitAreas         int            `json:"ground_link_hit_area_count"`
	GenericLinkLabels          int            `json:"generic_link_label_count"`
	InferredLinkLabels         int            `json:"inferred_link_label_count"`
	ExplicitLinkLabels         int            `json:"explicit_link_label_count"`
	LinkLabelMode              string         `json:"link_label_mode,omitempty"`
	HTMLLinkLabels             int            `json:"html_link_label_count"`
	GroundLinkLabels           int            `json:"ground_link_label_mesh_count"`
	GroundTextureLinkLabels    int            `json:"ground_texture_link_label_count"`
	GroundLinkTextures         int            `json:"ground_link_label_texture_ready_count"`
	GroundLabelsVisible        int            `json:"ground_link_label_visible_count"`
	GroundLabelsFlipped        int            `json:"ground_link_label_flipped_count"`
	ScreenSVGVisible           bool           `json:"screen_svg_relation_layer_visible"`
	SVGDebugLayer              bool           `json:"svg_debug_relation_layer_present"`
	EntityLabelAnchors         int            `json:"entity_label_anchor_count"`
	LinkLabelAnchors           int            `json:"link_label_anchor_count"`
	ZoneLabelAnchors           int            `json:"zone_label_anchor_count"`
	WorldLeaderLines           int            `json:"world_leader_line_count"`
	OrbitSmokeEnabled          bool           `json:"orbit_smoke_enabled"`
	OrbitEntityMaxDelta        float64        `json:"orbit_entity_label_return_max_delta_px,omitempty"`
	OrbitEntityAvgDelta        float64        `json:"orbit_entity_label_return_avg_delta_px,omitempty"`
	OrbitLinkMaxDelta          float64        `json:"orbit_link_label_return_max_delta_px,omitempty"`
	OrbitLinkAvgDelta          float64        `json:"orbit_link_label_return_avg_delta_px,omitempty"`
	OrbitMissingEntities       int            `json:"orbit_missing_entity_labels_after_rotate,omitempty"`
	OrbitMissingLinks          int            `json:"orbit_missing_link_labels_after_rotate,omitempty"`
	OrbitLayerStable           bool           `json:"orbit_relation_layer_mode_stable"`
}

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type VisualSummary struct {
	Template                         string         `json:"template"`
	ScreenshotPath                   string         `json:"screenshot_path"`
	EntityLabelCount                 int            `json:"entity_label_count"`
	LabelIconCount                   int            `json:"label_icon_count"`
	LabelIconLoadedCount             int            `json:"label_icon_loaded_count"`
	BrokenLabelIconCount             int            `json:"broken_label_icon_count"`
	VisibleEntityLabelCount          int            `json:"visible_entity_label_count"`
	VisibleLinkLabelCount            int            `json:"visible_link_label_count"`
	VisibleZoneLabelCount            int            `json:"visible_zone_label_count"`
	VisibleLabelIconCount            int            `json:"visible_label_icon_count"`
	PrimaryLinkCount                 int            `json:"primary_link_count"`
	SecondaryLinkCount               int            `json:"secondary_link_count"`
	AuxiliaryLinkCount               int            `json:"auxiliary_link_count"`
	VisiblePrimaryLinkLabelCount     int            `json:"visible_primary_link_label_count"`
	VisibleSecondaryLinkLabelCount   int            `json:"visible_secondary_link_label_count"`
	VisibleAuxiliaryLinkLabelCount   int            `json:"visible_auxiliary_link_label_count"`
	ExplicitRouteLinkCount           int            `json:"explicit_route_link_count"`
	HeuristicRouteLinkCount          int            `json:"heuristic_route_link_count"`
	PrimaryExplicitRouteCount        int            `json:"primary_explicit_route_count"`
	PrimaryVisibleLabelCount         int            `json:"primary_visible_label_count"`
	OverviewLinkLabelCount           int            `json:"overview_link_label_count"`
	RelationColorPaletteSize         int            `json:"relation_color_palette_size"`
	RelationColorPalette             []string       `json:"relation_color_palette,omitempty"`
	VisibleAuxiliaryOpacityAverage   float64        `json:"visible_auxiliary_opacity_average,omitempty"`
	LinkOpacityBuckets               map[string]int `json:"link_opacity_buckets,omitempty"`
	ZoneCountVisible                 int            `json:"zone_count_visible"`
	PrimaryPathGroupsVisible         []string       `json:"primary_path_groups_visible,omitempty"`
	RouteGroups                      []string       `json:"route_groups,omitempty"`
	InspectorRawJSONDefault          bool           `json:"inspector_raw_json_default"`
	SVGRelationLayerPresent          bool           `json:"svg_relation_layer_present"`
	SVGLinkPathCount                 int            `json:"svg_link_path_count"`
	SVGPrimaryLinkPathCount          int            `json:"svg_primary_link_path_count"`
	SVGSecondaryLinkPathCount        int            `json:"svg_secondary_link_path_count"`
	SVGAuxiliaryLinkPathCount        int            `json:"svg_auxiliary_link_path_count"`
	VisibleSVGLinkPathCount          int            `json:"visible_svg_link_path_count"`
	RelationLayerBounds              *Rect          `json:"relation_layer_bbox,omitempty"`
	LinkPathsWithMarkerCount         int            `json:"link_paths_with_marker_count"`
	LinkPathsWithoutMarkerCount      int            `json:"link_paths_without_marker_count"`
	ModelBadgeCount                  int            `json:"model_badge_count"`
	SvgBillboardCount                int            `json:"svg_billboard_count"`
	FallbackBadgeCount               int            `json:"fallback_badge_count"`
	CanvasVisible                    bool           `json:"canvas_visible"`
	ControlsVisible                  bool           `json:"controls_visible"`
	ApproximateLabelOverlapCount     int            `json:"approximate_label_overlap_count"`
	EntityLabelOverlapCount          int            `json:"entity_label_overlap_count"`
	LinkLabelOverlapCount            int            `json:"link_label_overlap_count"`
	ZoneLabelOverlapCount            int            `json:"zone_label_overlap_count"`
	TotalLabelOverlapCount           int            `json:"total_label_overlap_count"`
	LabelsOutsideStageCount          int            `json:"labels_outside_stage_count"`
	LabelsUnderToolbarCount          int            `json:"labels_under_toolbar_count"`
	LabelsUnderInspectorCount        int            `json:"labels_under_inspector_count"`
	CameraFitIncludesLabels          bool           `json:"camera_fit_includes_labels"`
	CameraFitReservedInspector       bool           `json:"camera_fit_reserved_inspector_margin"`
	CameraFitReservedToolbar         bool           `json:"camera_fit_reserved_toolbar_margin"`
	CameraFitIncludesHTMLLabels      bool           `json:"camera_fit_includes_html_labels"`
	LabelLayerBounds                 *Rect          `json:"label_layer_bounds,omitempty"`
	CanvasBounds                     *Rect          `json:"canvas_bounds,omitempty"`
	ScreenshotSize                   *Rect          `json:"screenshot_size,omitempty"`
	SceneComponentTreePresent        bool           `json:"scene_component_tree_present"`
	EntityComponentCount             int            `json:"entity_component_count"`
	RelationComponentCount           int            `json:"relation_component_count"`
	HTMLLabelComponentCount          int            `json:"html_label_component_count"`
	LeaderLineComponentCount         int            `json:"leader_line_component_count"`
	GroundPathBuilderPresent         bool           `json:"ground_path_builder_present"`
	GroundPathBuilderVersion         string         `json:"ground_path_builder_version,omitempty"`
	PathJoinStyle                    string         `json:"path_join_style,omitempty"`
	PathArrowCapCount                int            `json:"path_arrow_cap_count"`
	PathArrowCapIntegratedCount      int            `json:"path_arrow_cap_integrated_count"`
	PathHitAreaCount                 int            `json:"path_hit_area_count"`
	PathHoverHaloSupported           bool           `json:"path_hover_halo_supported"`
	PathParallelOffsetCount          int            `json:"path_parallel_offset_count"`
	PathBundleCount                  int            `json:"path_bundle_count"`
	PathDashSegmentCount             int            `json:"path_dash_segment_count"`
	RoutePlanPresent                 bool           `json:"route_plan_present"`
	RoutePlanVersion                 string         `json:"route_plan_version,omitempty"`
	RoutePlanBackend                 string         `json:"route_plan_backend,omitempty"`
	RoutePlanRouteCount              int            `json:"route_plan_route_count"`
	RoutePlanLaneCount               int            `json:"route_plan_lane_count"`
	RoutePlanObstacleCount           int            `json:"route_plan_obstacle_count"`
	RoutePlanRenderedMatch           bool           `json:"route_plan_rendered_match"`
	RoutePlanRenderedMatchCount      int            `json:"route_plan_rendered_match_count"`
	SourceEdgeCount                  int            `json:"source_edge_count"`
	DisplayRouteCount                int            `json:"display_route_count"`
	HiddenDetailRouteCount           int            `json:"hidden_detail_route_count"`
	RouteToZoneCount                 int            `json:"route_to_zone_count"`
	RouteToEntityCount               int            `json:"route_to_entity_count"`
	RouteToZoneRatio                 float64        `json:"route_to_zone_ratio"`
	RouteSameStyleMismatchCount      int            `json:"route_same_style_mismatch_count"`
	PathArrowBodyGapCount            int            `json:"path_arrow_body_gap_count"`
	PathArrowAtBendCount             int            `json:"path_arrow_at_bend_count"`
	RouteColorConsistencyScore       float64        `json:"route_color_consistency_score"`
	EntityBodyRegistryCount          int            `json:"entity_body_registry_count"`
	EntityKnownBodyCount             int            `json:"entity_known_body_count"`
	EntityGenericBodyCount           int            `json:"entity_generic_body_count"`
	EntityGenericBodyRatio           float64        `json:"entity_generic_body_ratio"`
	EntitySemanticBodyScore          float64        `json:"entity_semantic_body_score"`
	EntityVisualStyleVersion         string         `json:"entity_visual_style_version,omitempty"`
	EntityVisualPaletteVersion       int            `json:"entity_visual_palette_version"`
	EntityBodyShapeVarietyCount      int            `json:"entity_body_shape_variety_count"`
	EntityContactShadowCount         int            `json:"entity_contact_shadow_count"`
	EntityTopHighlightCount          int            `json:"entity_top_highlight_count"`
	EntitySidePanelCount             int            `json:"entity_side_panel_count"`
	EntityIconDecalCount             int            `json:"entity_icon_decal_count"`
	EntityRoundedOrBeveledCount      int            `json:"entity_rounded_or_beveled_count"`
	EntityScreenPanelCount           int            `json:"entity_screen_panel_count"`
	EntitySemanticModelCoverageCount int            `json:"entity_semantic_model_coverage_count"`
	EntitySemanticModelCoverageRatio float64        `json:"entity_semantic_model_coverage_ratio"`
	EntityBrightnessScore            float64        `json:"entity_brightness_score"`
	EntitySaturationScore            float64        `json:"entity_saturation_score"`
	EntityChromaScore                float64        `json:"entity_chroma_score"`
	NeutralGrayMaterialRatio         float64        `json:"neutral_gray_material_ratio"`
	ZoneEntityOverflowCount          int            `json:"zone_entity_overflow_count"`
	ZoneLabelOverflowCount           int            `json:"zone_label_overflow_count"`
	ZonePaddingMinPx                 int            `json:"zone_padding_min_px"`
	ModelKindCounts                  map[string]int `json:"model_kind_counts,omitempty"`
	RelationComponentsOwnPathCount   int            `json:"relation_components_own_path_count"`
	RelationComponentsOwnArrowCount  int            `json:"relation_components_own_arrow_count"`
	RelationComponentsOwnHitCount    int            `json:"relation_components_own_hit_count"`
	RelationComponentsOwnLabelCount  int            `json:"relation_components_own_label_count"`
	EntityComponentsWithPortsCount   int            `json:"entity_components_with_ports_count"`
	RelationLayerMode                string         `json:"relation_layer_mode,omitempty"`
	RelationRenderMode               string         `json:"relation_render_mode,omitempty"`
	RelationDepthTestEnabledCount    int            `json:"relation_depth_test_enabled_count"`
	RelationDepthTestDisabledCount   int            `json:"relation_depth_test_disabled_count"`
	RouteEntityIntersectionCount     int            `json:"route_entity_intersection_count"`
	RoutePortHintViolationCount      int            `json:"route_port_hint_violation_count"`
	RouteDirectionViolationCount     int            `json:"route_direction_violation_count"`
	RouteMaxLengthWorld              float64        `json:"route_max_length_world,omitempty"`
	RouteCrossSceneCount             int            `json:"route_cross_scene_count"`
	RouteCrossingCount               int            `json:"route_crossing_count"`
	RouteParallelOverlapCount        int            `json:"route_parallel_overlap_count"`
	RoutePathGroupOverlapCount       int            `json:"route_path_group_overlap_count"`
	RouteBusLaneCount                int            `json:"route_bus_lane_count"`
	RouteBundleCount                 int            `json:"route_bundle_count"`
	PrimaryRouteCount                int            `json:"primary_route_count"`
	SecondaryRouteCount              int            `json:"secondary_route_count"`
	AuxiliaryRouteCount              int            `json:"auxiliary_route_count"`
	RelationLooksLikeRaisedBeam      int            `json:"relation_looks_like_raised_beam_count"`
	WorldRelationLayerPresent        bool           `json:"world_relation_layer_present"`
	GroundLinkMeshCount              int            `json:"ground_link_mesh_count"`
	GroundLinkRibbonCount            int            `json:"ground_link_ribbon_count"`
	GroundLinkSegmentCount           int            `json:"ground_link_segment_count"`
	GroundRouteRailSegmentCount      int            `json:"ground_route_rail_segment_count"`
	GroundRouteRailJointCount        int            `json:"ground_route_rail_joint_count"`
	GroundRouteRailArrowheadCount    int            `json:"ground_route_rail_arrowhead_count"`
	GroundRouteRailVisibleCount      int            `json:"ground_route_rail_visible_count"`
	IsolatedArrowheadCount           int            `json:"isolated_arrowhead_count"`
	RoutesWithSegmentsCount          int            `json:"routes_with_segments_count"`
	RoutesWithoutSegmentsCount       int            `json:"routes_without_segments_count"`
	VisibleGroundLinkCount           int            `json:"visible_ground_link_count"`
	GroundArrowheadCount             int            `json:"ground_arrowhead_count"`
	VisibleGroundArrowheadCount      int            `json:"visible_ground_arrowhead_count"`
	GroundLinkHitAreaCount           int            `json:"ground_link_hit_area_count"`
	GenericLinkLabelCount            int            `json:"generic_link_label_count"`
	InferredLinkLabelCount           int            `json:"inferred_link_label_count"`
	ExplicitLinkLabelCount           int            `json:"explicit_link_label_count"`
	LinkLabelMode                    string         `json:"link_label_mode,omitempty"`
	HTMLLinkLabelCount               int            `json:"html_link_label_count"`
	GroundLinkLabelMeshCount         int            `json:"ground_link_label_mesh_count"`
	GroundTextureLinkLabelCount      int            `json:"ground_texture_link_label_count"`
	GroundLinkLabelTextureReady      int            `json:"ground_link_label_texture_ready_count"`
	GroundLinkLabelVisibleCount      int            `json:"ground_link_label_visible_count"`
	GroundLinkLabelFlippedCount      int            `json:"ground_link_label_flipped_count"`
	ScreenSVGRelationLayerVisible    bool           `json:"screen_svg_relation_layer_visible"`
	SVGDebugRelationLayerPresent     bool           `json:"svg_debug_relation_layer_present"`
	EntityLabelAnchorCount           int            `json:"entity_label_anchor_count"`
	LinkLabelAnchorCount             int            `json:"link_label_anchor_count"`
	ZoneLabelAnchorCount             int            `json:"zone_label_anchor_count"`
	WorldLeaderLineCount             int            `json:"world_leader_line_count"`
	OrbitSmokeEnabled                bool           `json:"orbit_smoke_enabled"`
	OrbitEntityLabelReturnMaxDelta   float64        `json:"orbit_entity_label_return_max_delta_px,omitempty"`
	OrbitEntityLabelReturnAvgDelta   float64        `json:"orbit_entity_label_return_avg_delta_px,omitempty"`
	OrbitLinkLabelReturnMaxDelta     float64        `json:"orbit_link_label_return_max_delta_px,omitempty"`
	OrbitLinkLabelReturnAvgDelta     float64        `json:"orbit_link_label_return_avg_delta_px,omitempty"`
	OrbitMissingEntityLabels         int            `json:"orbit_missing_entity_labels_after_rotate,omitempty"`
	OrbitMissingLinkLabels           int            `json:"orbit_missing_link_labels_after_rotate,omitempty"`
	OrbitRelationLayerModeStable     bool           `json:"orbit_relation_layer_mode_stable"`
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
		Title:                      nodeResult.Data.Summary.Title,
		Template:                   nodeResult.Data.Summary.Template,
		Renderer:                   nodeResult.Data.Summary.Renderer,
		EntityLabels:               nodeResult.Data.Summary.EntityLabels,
		LinkLabels:                 nodeResult.Data.Summary.LinkLabels,
		ZoneLabels:                 nodeResult.Data.Summary.ZoneLabels,
		LabelIcons:                 nodeResult.Data.Summary.LabelIcons,
		LabelIconsLoaded:           nodeResult.Data.Summary.LabelIconsLoaded,
		BrokenLabelIcons:           nodeResult.Data.Summary.BrokenLabelIcons,
		VisibleEntityLabels:        nodeResult.Data.Summary.VisibleEntityLabels,
		VisibleLinkLabels:          nodeResult.Data.Summary.VisibleLinkLabels,
		VisibleZoneLabels:          nodeResult.Data.Summary.VisibleZoneLabels,
		VisibleLabelIcons:          nodeResult.Data.Summary.VisibleLabelIcons,
		PrimaryLinkCount:           nodeResult.Data.Summary.PrimaryLinkCount,
		SecondaryLinkCount:         nodeResult.Data.Summary.SecondaryLinkCount,
		AuxiliaryLinkCount:         nodeResult.Data.Summary.AuxiliaryLinkCount,
		VisiblePrimaryLabels:       nodeResult.Data.Summary.VisiblePrimaryLinkLabelCount,
		VisibleSecondaryLabels:     nodeResult.Data.Summary.VisibleSecondaryLinkLabelCount,
		VisibleAuxiliaryLabels:     nodeResult.Data.Summary.VisibleAuxiliaryLinkLabelCount,
		ExplicitRouteLinks:         nodeResult.Data.Summary.ExplicitRouteLinkCount,
		HeuristicRouteLinks:        nodeResult.Data.Summary.HeuristicRouteLinkCount,
		PrimaryExplicitRoutes:      nodeResult.Data.Summary.PrimaryExplicitRouteCount,
		PrimaryVisibleLabels:       nodeResult.Data.Summary.PrimaryVisibleLabelCount,
		OverviewLinkLabels:         nodeResult.Data.Summary.OverviewLinkLabelCount,
		RelationPaletteSize:        nodeResult.Data.Summary.RelationColorPaletteSize,
		RelationPalette:            nodeResult.Data.Summary.RelationColorPalette,
		VisibleAuxOpacityAvg:       nodeResult.Data.Summary.VisibleAuxiliaryOpacityAverage,
		ZoneCountVisible:           nodeResult.Data.Summary.ZoneCountVisible,
		RouteGroups:                nodeResult.Data.Summary.RouteGroups,
		InspectorRawDefault:        nodeResult.Data.Summary.InspectorRawJSONDefault,
		SVGRelationLayer:           nodeResult.Data.Summary.SVGRelationLayerPresent,
		SVGLinkPathCount:           nodeResult.Data.Summary.SVGLinkPathCount,
		SVGPrimaryPathCount:        nodeResult.Data.Summary.SVGPrimaryLinkPathCount,
		SVGSecondaryPathCount:      nodeResult.Data.Summary.SVGSecondaryLinkPathCount,
		SVGAuxiliaryPathCount:      nodeResult.Data.Summary.SVGAuxiliaryLinkPathCount,
		VisibleSVGPathCount:        nodeResult.Data.Summary.VisibleSVGLinkPathCount,
		LinkPathsWithMarker:        nodeResult.Data.Summary.LinkPathsWithMarkerCount,
		LinkPathsWithoutMarker:     nodeResult.Data.Summary.LinkPathsWithoutMarkerCount,
		EntityLabelOverlap:         nodeResult.Data.Summary.EntityLabelOverlapCount,
		LinkLabelOverlap:           nodeResult.Data.Summary.LinkLabelOverlapCount,
		ZoneLabelOverlap:           nodeResult.Data.Summary.ZoneLabelOverlapCount,
		TotalLabelOverlap:          nodeResult.Data.Summary.TotalLabelOverlapCount,
		LabelsOutsideStage:         nodeResult.Data.Summary.LabelsOutsideStageCount,
		LabelsUnderToolbar:         nodeResult.Data.Summary.LabelsUnderToolbarCount,
		LabelsUnderInspector:       nodeResult.Data.Summary.LabelsUnderInspectorCount,
		CameraFitIncludesLabels:    nodeResult.Data.Summary.CameraFitIncludesLabels,
		CameraFitReservedInspector: nodeResult.Data.Summary.CameraFitReservedInspector,
		CameraFitReservedToolbar:   nodeResult.Data.Summary.CameraFitReservedToolbar,
		CameraFitIncludesHTML:      nodeResult.Data.Summary.CameraFitIncludesHTMLLabels,
		ModelBadges:                nodeResult.Data.Summary.ModelBadges,
		SvgBillboards:              nodeResult.Data.Summary.SvgBillboards,
		FallbackBadges:             nodeResult.Data.Summary.FallbackBadges,
		PresentationMode:           nodeResult.Data.Summary.PresentationMode,
		Controls:                   nodeResult.Data.Summary.Controls,
		Canvas:                     nodeResult.Data.Summary.Canvas,
		RuntimeDataRequested:       hasRequest(requests, "/data.js") && hasRequest(requests, "/manifest.js"),
		SceneComponentTree:         nodeResult.Data.Summary.SceneComponentTreePresent,
		EntityComponents:           nodeResult.Data.Summary.EntityComponentCount,
		RelationComponents:         nodeResult.Data.Summary.RelationComponentCount,
		HTMLLabelComponents:        nodeResult.Data.Summary.HTMLLabelComponentCount,
		LeaderLineComponents:       nodeResult.Data.Summary.LeaderLineComponentCount,
		GroundPathBuilder:          nodeResult.Data.Summary.GroundPathBuilderPresent,
		GroundPathBuilderVersion:   nodeResult.Data.Summary.GroundPathBuilderVersion,
		PathJoinStyle:              nodeResult.Data.Summary.PathJoinStyle,
		PathArrowCapCount:          nodeResult.Data.Summary.PathArrowCapCount,
		PathArrowCapIntegrated:     nodeResult.Data.Summary.PathArrowCapIntegratedCount,
		PathHitAreaCount:           nodeResult.Data.Summary.PathHitAreaCount,
		PathHoverHaloSupported:     nodeResult.Data.Summary.PathHoverHaloSupported,
		PathParallelOffsetCount:    nodeResult.Data.Summary.PathParallelOffsetCount,
		PathBundleCount:            nodeResult.Data.Summary.PathBundleCount,
		RoutePlanPresent:           nodeResult.Data.Summary.RoutePlanPresent,
		RoutePlanVersion:           nodeResult.Data.Summary.RoutePlanVersion,
		RoutePlanBackend:           nodeResult.Data.Summary.RoutePlanBackend,
		RoutePlanRouteCount:        nodeResult.Data.Summary.RoutePlanRouteCount,
		RoutePlanLaneCount:         nodeResult.Data.Summary.RoutePlanLaneCount,
		RoutePlanObstacleCount:     nodeResult.Data.Summary.RoutePlanObstacleCount,
		RoutePlanRenderedMatch:     nodeResult.Data.Summary.RoutePlanRenderedMatch,
		RoutePlanRenderedMatchCnt:  nodeResult.Data.Summary.RoutePlanRenderedMatchCount,
		SourceEdgeCount:            nodeResult.Data.Summary.SourceEdgeCount,
		DisplayRouteCount:          nodeResult.Data.Summary.DisplayRouteCount,
		HiddenDetailRouteCount:     nodeResult.Data.Summary.HiddenDetailRouteCount,
		RouteToZoneCount:           nodeResult.Data.Summary.RouteToZoneCount,
		RouteToEntityCount:         nodeResult.Data.Summary.RouteToEntityCount,
		RouteToZoneRatio:           nodeResult.Data.Summary.RouteToZoneRatio,
		RouteSameStyleMismatch:     nodeResult.Data.Summary.RouteSameStyleMismatchCount,
		PathArrowBodyGapCount:      nodeResult.Data.Summary.PathArrowBodyGapCount,
		PathArrowAtBendCount:       nodeResult.Data.Summary.PathArrowAtBendCount,
		RouteColorConsistencyScore: nodeResult.Data.Summary.RouteColorConsistencyScore,
		EntityBodyRegistryCount:    nodeResult.Data.Summary.EntityBodyRegistryCount,
		EntityKnownBodyCount:       nodeResult.Data.Summary.EntityKnownBodyCount,
		EntityGenericBodyCount:     nodeResult.Data.Summary.EntityGenericBodyCount,
		EntityGenericBodyRatio:     nodeResult.Data.Summary.EntityGenericBodyRatio,
		EntitySemanticBodyScore:    nodeResult.Data.Summary.EntitySemanticBodyScore,
		EntityVisualStyleVersion:   nodeResult.Data.Summary.EntityVisualStyleVersion,
		EntityVisualPaletteVersion: nodeResult.Data.Summary.EntityVisualPaletteVersion,
		EntityBodyShapeVariety:     nodeResult.Data.Summary.EntityBodyShapeVarietyCount,
		EntityContactShadows:       nodeResult.Data.Summary.EntityContactShadowCount,
		EntityTopHighlights:        nodeResult.Data.Summary.EntityTopHighlightCount,
		EntitySidePanels:           nodeResult.Data.Summary.EntitySidePanelCount,
		EntityIconDecals:           nodeResult.Data.Summary.EntityIconDecalCount,
		EntityRoundedOrBeveled:     nodeResult.Data.Summary.EntityRoundedOrBeveledCount,
		EntityScreenPanels:         nodeResult.Data.Summary.EntityScreenPanelCount,
		EntitySemanticModelCount:   nodeResult.Data.Summary.EntitySemanticModelCoverageCount,
		EntitySemanticModelRatio:   nodeResult.Data.Summary.EntitySemanticModelCoverageRatio,
		EntityBrightnessScore:      nodeResult.Data.Summary.EntityBrightnessScore,
		EntitySaturationScore:      nodeResult.Data.Summary.EntitySaturationScore,
		EntityChromaScore:          nodeResult.Data.Summary.EntityChromaScore,
		NeutralGrayMaterialRatio:   nodeResult.Data.Summary.NeutralGrayMaterialRatio,
		ZoneEntityOverflowCount:    nodeResult.Data.Summary.ZoneEntityOverflowCount,
		ZoneLabelOverflowCount:     nodeResult.Data.Summary.ZoneLabelOverflowCount,
		ZonePaddingMinPx:           nodeResult.Data.Summary.ZonePaddingMinPx,
		ModelKindCounts:            nodeResult.Data.Summary.ModelKindCounts,
		RelationOwnsPath:           nodeResult.Data.Summary.RelationComponentsOwnPathCount,
		RelationOwnsArrow:          nodeResult.Data.Summary.RelationComponentsOwnArrowCount,
		RelationOwnsHit:            nodeResult.Data.Summary.RelationComponentsOwnHitCount,
		RelationOwnsLabel:          nodeResult.Data.Summary.RelationComponentsOwnLabelCount,
		EntityComponentsWithPorts:  nodeResult.Data.Summary.EntityComponentsWithPortsCount,
		RelationLayerMode:          nodeResult.Data.Summary.RelationLayerMode,
		RelationRenderMode:         nodeResult.Data.Summary.RelationRenderMode,
		RelationDepthEnabled:       nodeResult.Data.Summary.RelationDepthTestEnabledCount,
		RelationDepthDisabled:      nodeResult.Data.Summary.RelationDepthTestDisabledCount,
		RouteEntityIntersections:   nodeResult.Data.Summary.RouteEntityIntersectionCount,
		RoutePortViolations:        nodeResult.Data.Summary.RoutePortHintViolationCount,
		RouteDirectionViolations:   nodeResult.Data.Summary.RouteDirectionViolationCount,
		RouteMaxLengthWorld:        nodeResult.Data.Summary.RouteMaxLengthWorld,
		RouteCrossSceneCount:       nodeResult.Data.Summary.RouteCrossSceneCount,
		RouteCrossingCount:         nodeResult.Data.Summary.RouteCrossingCount,
		RouteParallelOverlapCount:  nodeResult.Data.Summary.RouteParallelOverlapCount,
		RoutePathGroupOverlapCount: nodeResult.Data.Summary.RoutePathGroupOverlapCount,
		RouteBusLaneCount:          nodeResult.Data.Summary.RouteBusLaneCount,
		RouteBundleCount:           nodeResult.Data.Summary.RouteBundleCount,
		PrimaryRouteCount:          nodeResult.Data.Summary.PrimaryRouteCount,
		SecondaryRouteCount:        nodeResult.Data.Summary.SecondaryRouteCount,
		AuxiliaryRouteCount:        nodeResult.Data.Summary.AuxiliaryRouteCount,
		RaisedBeamLooks:            nodeResult.Data.Summary.RelationLooksLikeRaisedBeam,
		WorldRelationLayer:         nodeResult.Data.Summary.WorldRelationLayerPresent,
		GroundLinkMeshes:           nodeResult.Data.Summary.GroundLinkMeshCount,
		GroundLinkRibbons:          nodeResult.Data.Summary.GroundLinkRibbonCount,
		GroundLinkSegments:         nodeResult.Data.Summary.GroundLinkSegmentCount,
		GroundRouteRailSegments:    nodeResult.Data.Summary.GroundRouteRailSegmentCount,
		GroundRouteRailJoints:      nodeResult.Data.Summary.GroundRouteRailJointCount,
		GroundRouteRailArrows:      nodeResult.Data.Summary.GroundRouteRailArrowheadCount,
		GroundRouteRailVisible:     nodeResult.Data.Summary.GroundRouteRailVisibleCount,
		IsolatedArrowheads:         nodeResult.Data.Summary.IsolatedArrowheadCount,
		RoutesWithSegments:         nodeResult.Data.Summary.RoutesWithSegmentsCount,
		RoutesWithoutSegments:      nodeResult.Data.Summary.RoutesWithoutSegmentsCount,
		VisibleGroundLinks:         nodeResult.Data.Summary.VisibleGroundLinkCount,
		GroundArrowheads:           nodeResult.Data.Summary.GroundArrowheadCount,
		VisibleGroundArrowheads:    nodeResult.Data.Summary.VisibleGroundArrowheadCount,
		GroundLinkHitAreas:         nodeResult.Data.Summary.GroundLinkHitAreaCount,
		GenericLinkLabels:          nodeResult.Data.Summary.GenericLinkLabelCount,
		InferredLinkLabels:         nodeResult.Data.Summary.InferredLinkLabelCount,
		ExplicitLinkLabels:         nodeResult.Data.Summary.ExplicitLinkLabelCount,
		LinkLabelMode:              nodeResult.Data.Summary.LinkLabelMode,
		HTMLLinkLabels:             nodeResult.Data.Summary.HTMLLinkLabelCount,
		GroundLinkLabels:           nodeResult.Data.Summary.GroundLinkLabelMeshCount,
		GroundTextureLinkLabels:    nodeResult.Data.Summary.GroundTextureLinkLabelCount,
		GroundLinkTextures:         nodeResult.Data.Summary.GroundLinkLabelTextureReady,
		GroundLabelsVisible:        nodeResult.Data.Summary.GroundLinkLabelVisibleCount,
		GroundLabelsFlipped:        nodeResult.Data.Summary.GroundLinkLabelFlippedCount,
		ScreenSVGVisible:           nodeResult.Data.Summary.ScreenSVGRelationLayerVisible,
		SVGDebugLayer:              nodeResult.Data.Summary.SVGDebugRelationLayerPresent,
		EntityLabelAnchors:         nodeResult.Data.Summary.EntityLabelAnchorCount,
		LinkLabelAnchors:           nodeResult.Data.Summary.LinkLabelAnchorCount,
		ZoneLabelAnchors:           nodeResult.Data.Summary.ZoneLabelAnchorCount,
		WorldLeaderLines:           nodeResult.Data.Summary.WorldLeaderLineCount,
		OrbitSmokeEnabled:          nodeResult.Data.Summary.OrbitSmokeEnabled,
		OrbitEntityMaxDelta:        nodeResult.Data.Summary.OrbitEntityLabelReturnMaxDelta,
		OrbitEntityAvgDelta:        nodeResult.Data.Summary.OrbitEntityLabelReturnAvgDelta,
		OrbitLinkMaxDelta:          nodeResult.Data.Summary.OrbitLinkLabelReturnMaxDelta,
		OrbitLinkAvgDelta:          nodeResult.Data.Summary.OrbitLinkLabelReturnAvgDelta,
		OrbitMissingEntities:       nodeResult.Data.Summary.OrbitMissingEntityLabels,
		OrbitMissingLinks:          nodeResult.Data.Summary.OrbitMissingLinkLabels,
		OrbitLayerStable:           nodeResult.Data.Summary.OrbitRelationLayerModeStable,
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
	Title                            string         `json:"title"`
	Template                         string         `json:"template"`
	Renderer                         string         `json:"renderer"`
	IsometricReady                   bool           `json:"isometricReady"`
	Stage                            bool           `json:"stage"`
	LabelLayer                       bool           `json:"labelLayer"`
	EntityLabels                     int            `json:"entityLabels"`
	LinkLabels                       int            `json:"linkLabels"`
	ZoneLabels                       int            `json:"zoneLabels"`
	LabelIcons                       int            `json:"labelIcons"`
	LabelIconsLoaded                 int            `json:"labelIconsLoaded"`
	BrokenLabelIcons                 int            `json:"brokenLabelIcons"`
	VisibleEntityLabels              int            `json:"visibleEntityLabels"`
	VisibleLinkLabels                int            `json:"visibleLinkLabels"`
	VisibleZoneLabels                int            `json:"visibleZoneLabels"`
	VisibleLabelIcons                int            `json:"visibleLabelIcons"`
	PrimaryLinkCount                 int            `json:"primaryLinkCount"`
	SecondaryLinkCount               int            `json:"secondaryLinkCount"`
	AuxiliaryLinkCount               int            `json:"auxiliaryLinkCount"`
	VisiblePrimaryLinkLabelCount     int            `json:"visiblePrimaryLinkLabelCount"`
	PrimaryVisibleLabelCount         int            `json:"primaryVisibleLabelCount"`
	VisibleSecondaryLinkLabelCount   int            `json:"visibleSecondaryLinkLabelCount"`
	VisibleAuxiliaryLinkLabelCount   int            `json:"visibleAuxiliaryLinkLabelCount"`
	ExplicitRouteLinkCount           int            `json:"explicitRouteLinkCount"`
	HeuristicRouteLinkCount          int            `json:"heuristicRouteLinkCount"`
	PrimaryExplicitRouteCount        int            `json:"primaryExplicitRouteCount"`
	OverviewLinkLabelCount           int            `json:"overviewLinkLabelCount"`
	RelationColorPaletteSize         int            `json:"relationColorPaletteSize"`
	RelationColorPalette             []string       `json:"relationColorPalette"`
	VisibleAuxiliaryOpacityAverage   float64        `json:"visibleAuxiliaryOpacityAverage"`
	LinkOpacityBuckets               map[string]int `json:"linkOpacityBuckets"`
	ZoneCountVisible                 int            `json:"zoneCountVisible"`
	PrimaryPathGroupsVisible         []string       `json:"primaryPathGroupsVisible"`
	RouteGroups                      []string       `json:"routeGroups"`
	InspectorRawJSONDefault          bool           `json:"inspectorRawJSONDefault"`
	SVGRelationLayerPresent          bool           `json:"svgRelationLayerPresent"`
	SVGLinkPathCount                 int            `json:"svgLinkPathCount"`
	SVGPrimaryLinkPathCount          int            `json:"svgPrimaryLinkPathCount"`
	SVGSecondaryLinkPathCount        int            `json:"svgSecondaryLinkPathCount"`
	SVGAuxiliaryLinkPathCount        int            `json:"svgAuxiliaryLinkPathCount"`
	VisibleSVGLinkPathCount          int            `json:"visibleSvgLinkPathCount"`
	RelationLayerBounds              *Rect          `json:"relationLayerBounds"`
	LinkPathsWithMarkerCount         int            `json:"linkPathsWithMarkerCount"`
	LinkPathsWithoutMarkerCount      int            `json:"linkPathsWithoutMarkerCount"`
	ModelBadges                      int            `json:"modelBadges"`
	SvgBillboards                    int            `json:"svgBillboards"`
	FallbackBadges                   int            `json:"fallbackBadges"`
	PresentationMode                 bool           `json:"presentationMode"`
	Controls                         int            `json:"controls"`
	ControlBar                       bool           `json:"controlBar"`
	Canvas                           int            `json:"canvas"`
	ApproximateLabelOverlapCount     int            `json:"approximateLabelOverlapCount"`
	EntityLabelOverlapCount          int            `json:"entityLabelOverlapCount"`
	LinkLabelOverlapCount            int            `json:"linkLabelOverlapCount"`
	ZoneLabelOverlapCount            int            `json:"zoneLabelOverlapCount"`
	TotalLabelOverlapCount           int            `json:"totalLabelOverlapCount"`
	LabelsOutsideStageCount          int            `json:"labelsOutsideStageCount"`
	LabelsUnderToolbarCount          int            `json:"labelsUnderToolbarCount"`
	LabelsUnderInspectorCount        int            `json:"labelsUnderInspectorCount"`
	CameraFitIncludesLabels          bool           `json:"cameraFitIncludesLabels"`
	CameraFitReservedInspector       bool           `json:"cameraFitReservedInspectorMargin"`
	CameraFitReservedToolbar         bool           `json:"cameraFitReservedToolbarMargin"`
	CameraFitIncludesHTMLLabels      bool           `json:"cameraFitIncludesHtmlLabels"`
	LabelLayerBounds                 *Rect          `json:"labelLayerBounds"`
	CanvasBounds                     *Rect          `json:"canvasBounds"`
	ScreenshotSize                   *Rect          `json:"screenshotSize"`
	SceneComponentTreePresent        bool           `json:"sceneComponentTreePresent"`
	EntityComponentCount             int            `json:"entityComponentCount"`
	RelationComponentCount           int            `json:"relationComponentCount"`
	HTMLLabelComponentCount          int            `json:"htmlLabelComponentCount"`
	LeaderLineComponentCount         int            `json:"leaderLineComponentCount"`
	GroundPathBuilderPresent         bool           `json:"groundPathBuilderPresent"`
	GroundPathBuilderVersion         string         `json:"groundPathBuilderVersion"`
	PathJoinStyle                    string         `json:"pathJoinStyle"`
	PathArrowCapCount                int            `json:"pathArrowCapCount"`
	PathArrowCapIntegratedCount      int            `json:"pathArrowCapIntegratedCount"`
	PathHitAreaCount                 int            `json:"pathHitAreaCount"`
	PathHoverHaloSupported           bool           `json:"pathHoverHaloSupported"`
	PathParallelOffsetCount          int            `json:"pathParallelOffsetCount"`
	PathBundleCount                  int            `json:"pathBundleCount"`
	PathDashSegmentCount             int            `json:"pathDashSegmentCount"`
	RoutePlanPresent                 bool           `json:"routePlanPresent"`
	RoutePlanVersion                 string         `json:"routePlanVersion"`
	RoutePlanBackend                 string         `json:"routePlanBackend"`
	RoutePlanRouteCount              int            `json:"routePlanRouteCount"`
	RoutePlanLaneCount               int            `json:"routePlanLaneCount"`
	RoutePlanObstacleCount           int            `json:"routePlanObstacleCount"`
	RoutePlanRenderedMatch           bool           `json:"routePlanRenderedMatch"`
	RoutePlanRenderedMatchCount      int            `json:"routePlanRenderedMatchCount"`
	SourceEdgeCount                  int            `json:"sourceEdgeCount"`
	DisplayRouteCount                int            `json:"displayRouteCount"`
	HiddenDetailRouteCount           int            `json:"hiddenDetailRouteCount"`
	RouteToZoneCount                 int            `json:"routeToZoneCount"`
	RouteToEntityCount               int            `json:"routeToEntityCount"`
	RouteToZoneRatio                 float64        `json:"routeToZoneRatio"`
	RouteSameStyleMismatchCount      int            `json:"routeSameStyleMismatchCount"`
	PathArrowBodyGapCount            int            `json:"pathArrowBodyGapCount"`
	PathArrowAtBendCount             int            `json:"pathArrowAtBendCount"`
	RouteColorConsistencyScore       float64        `json:"routeColorConsistencyScore"`
	EntityBodyRegistryCount          int            `json:"entityBodyRegistryCount"`
	EntityKnownBodyCount             int            `json:"entityKnownBodyCount"`
	EntityGenericBodyCount           int            `json:"entityGenericBodyCount"`
	EntityGenericBodyRatio           float64        `json:"entityGenericBodyRatio"`
	EntitySemanticBodyScore          float64        `json:"entitySemanticBodyScore"`
	EntityVisualStyleVersion         string         `json:"entityVisualStyleVersion"`
	EntityVisualPaletteVersion       int            `json:"entityVisualPaletteVersion"`
	EntityBodyShapeVarietyCount      int            `json:"entityBodyShapeVarietyCount"`
	EntityContactShadowCount         int            `json:"entityContactShadowCount"`
	EntityTopHighlightCount          int            `json:"entityTopHighlightCount"`
	EntitySidePanelCount             int            `json:"entitySidePanelCount"`
	EntityIconDecalCount             int            `json:"entityIconDecalCount"`
	EntityRoundedOrBeveledCount      int            `json:"entityRoundedOrBeveledCount"`
	EntityScreenPanelCount           int            `json:"entityScreenPanelCount"`
	EntitySemanticModelCoverageCount int            `json:"entitySemanticModelCoverageCount"`
	EntitySemanticModelCoverageRatio float64        `json:"entitySemanticModelCoverageRatio"`
	EntityBrightnessScore            float64        `json:"entityBrightnessScore"`
	EntitySaturationScore            float64        `json:"entitySaturationScore"`
	EntityChromaScore                float64        `json:"entityChromaScore"`
	NeutralGrayMaterialRatio         float64        `json:"neutralGrayMaterialRatio"`
	ZoneEntityOverflowCount          int            `json:"zoneEntityOverflowCount"`
	ZoneLabelOverflowCount           int            `json:"zoneLabelOverflowCount"`
	ZonePaddingMinPx                 int            `json:"zonePaddingMinPx"`
	ModelKindCounts                  map[string]int `json:"modelKindCounts"`
	RelationComponentsOwnPathCount   int            `json:"relationComponentsOwnPathCount"`
	RelationComponentsOwnArrowCount  int            `json:"relationComponentsOwnArrowCount"`
	RelationComponentsOwnHitCount    int            `json:"relationComponentsOwnHitCount"`
	RelationComponentsOwnLabelCount  int            `json:"relationComponentsOwnLabelCount"`
	EntityComponentsWithPortsCount   int            `json:"entityComponentsWithPortsCount"`
	RelationLayerMode                string         `json:"relationLayerMode"`
	RelationRenderMode               string         `json:"relationRenderMode"`
	RelationDepthTestEnabledCount    int            `json:"relationDepthTestEnabledCount"`
	RelationDepthTestDisabledCount   int            `json:"relationDepthTestDisabledCount"`
	RouteEntityIntersectionCount     int            `json:"routeEntityIntersectionCount"`
	RoutePortHintViolationCount      int            `json:"routePortHintViolationCount"`
	RouteDirectionViolationCount     int            `json:"routeDirectionViolationCount"`
	RouteMaxLengthWorld              float64        `json:"routeMaxLengthWorld"`
	RouteCrossSceneCount             int            `json:"routeCrossSceneCount"`
	RouteCrossingCount               int            `json:"routeCrossingCount"`
	RouteParallelOverlapCount        int            `json:"routeParallelOverlapCount"`
	RoutePathGroupOverlapCount       int            `json:"routePathGroupOverlapCount"`
	RouteBusLaneCount                int            `json:"routeBusLaneCount"`
	RouteBundleCount                 int            `json:"routeBundleCount"`
	PrimaryRouteCount                int            `json:"primaryRouteCount"`
	SecondaryRouteCount              int            `json:"secondaryRouteCount"`
	AuxiliaryRouteCount              int            `json:"auxiliaryRouteCount"`
	RelationLooksLikeRaisedBeam      int            `json:"relationLooksLikeRaisedBeamCount"`
	WorldRelationLayerPresent        bool           `json:"worldRelationLayerPresent"`
	GroundLinkMeshCount              int            `json:"groundLinkMeshCount"`
	GroundLinkRibbonCount            int            `json:"groundLinkRibbonCount"`
	GroundLinkSegmentCount           int            `json:"groundLinkSegmentCount"`
	GroundRouteRailSegmentCount      int            `json:"groundRouteRailSegmentCount"`
	GroundRouteRailJointCount        int            `json:"groundRouteRailJointCount"`
	GroundRouteRailArrowheadCount    int            `json:"groundRouteRailArrowheadCount"`
	GroundRouteRailVisibleCount      int            `json:"groundRouteRailVisibleCount"`
	IsolatedArrowheadCount           int            `json:"isolatedArrowheadCount"`
	RoutesWithSegmentsCount          int            `json:"routesWithSegmentsCount"`
	RoutesWithoutSegmentsCount       int            `json:"routesWithoutSegmentsCount"`
	VisibleGroundLinkCount           int            `json:"visibleGroundLinkCount"`
	GroundArrowheadCount             int            `json:"groundArrowheadCount"`
	VisibleGroundArrowheadCount      int            `json:"visibleGroundArrowheadCount"`
	GroundLinkHitAreaCount           int            `json:"groundLinkHitAreaCount"`
	GenericLinkLabelCount            int            `json:"genericLinkLabelCount"`
	InferredLinkLabelCount           int            `json:"inferredLinkLabelCount"`
	ExplicitLinkLabelCount           int            `json:"explicitLinkLabelCount"`
	LinkLabelMode                    string         `json:"linkLabelMode"`
	HTMLLinkLabelCount               int            `json:"htmlLinkLabelCount"`
	GroundLinkLabelMeshCount         int            `json:"groundLinkLabelMeshCount"`
	GroundTextureLinkLabelCount      int            `json:"groundTextureLinkLabelCount"`
	GroundLinkLabelTextureReady      int            `json:"groundLinkLabelTextureReadyCount"`
	GroundLinkLabelVisibleCount      int            `json:"groundLinkLabelVisibleCount"`
	GroundLinkLabelFlippedCount      int            `json:"groundLinkLabelFlippedCount"`
	ScreenSVGRelationLayerVisible    bool           `json:"screenSvgRelationLayerVisible"`
	SVGDebugRelationLayerPresent     bool           `json:"svgDebugRelationLayerPresent"`
	EntityLabelAnchorCount           int            `json:"entityLabelAnchorCount"`
	LinkLabelAnchorCount             int            `json:"linkLabelAnchorCount"`
	ZoneLabelAnchorCount             int            `json:"zoneLabelAnchorCount"`
	WorldLeaderLineCount             int            `json:"worldLeaderLineCount"`
	OrbitSmokeEnabled                bool           `json:"orbitSmokeEnabled"`
	OrbitEntityLabelReturnMaxDelta   float64        `json:"orbitEntityLabelReturnMaxDeltaPx"`
	OrbitEntityLabelReturnAvgDelta   float64        `json:"orbitEntityLabelReturnAvgDeltaPx"`
	OrbitLinkLabelReturnMaxDelta     float64        `json:"orbitLinkLabelReturnMaxDeltaPx"`
	OrbitLinkLabelReturnAvgDelta     float64        `json:"orbitLinkLabelReturnAvgDeltaPx"`
	OrbitMissingEntityLabels         int            `json:"orbitMissingEntityLabelsAfterRotate"`
	OrbitMissingLinkLabels           int            `json:"orbitMissingLinkLabelsAfterRotate"`
	OrbitRelationLayerModeStable     bool           `json:"orbitRelationLayerModeStable"`
	Ready                            bool           `json:"ready"`
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
		ControlsPresent:                 summary.PresentationMode || (summary.Controls > 0 && nodeResult.Data.Summary.ControlBar),
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
		Template:                         summary.Template,
		ScreenshotPath:                   screenshot,
		EntityLabelCount:                 summary.EntityLabels,
		LabelIconCount:                   summary.LabelIcons,
		LabelIconLoadedCount:             summary.LabelIconsLoaded,
		BrokenLabelIconCount:             summary.BrokenLabelIcons,
		VisibleEntityLabelCount:          summary.VisibleEntityLabels,
		VisibleLinkLabelCount:            summary.VisibleLinkLabels,
		VisibleZoneLabelCount:            summary.VisibleZoneLabels,
		VisibleLabelIconCount:            summary.VisibleLabelIcons,
		PrimaryLinkCount:                 summary.PrimaryLinkCount,
		SecondaryLinkCount:               summary.SecondaryLinkCount,
		AuxiliaryLinkCount:               summary.AuxiliaryLinkCount,
		VisiblePrimaryLinkLabelCount:     summary.VisiblePrimaryLabels,
		VisibleSecondaryLinkLabelCount:   summary.VisibleSecondaryLabels,
		VisibleAuxiliaryLinkLabelCount:   summary.VisibleAuxiliaryLabels,
		ExplicitRouteLinkCount:           summary.ExplicitRouteLinks,
		HeuristicRouteLinkCount:          summary.HeuristicRouteLinks,
		PrimaryExplicitRouteCount:        summary.PrimaryExplicitRoutes,
		PrimaryVisibleLabelCount:         summary.PrimaryVisibleLabels,
		OverviewLinkLabelCount:           summary.OverviewLinkLabels,
		RelationColorPaletteSize:         summary.RelationPaletteSize,
		RelationColorPalette:             summary.RelationPalette,
		VisibleAuxiliaryOpacityAverage:   summary.VisibleAuxOpacityAvg,
		LinkOpacityBuckets:               nodeResult.Data.Summary.LinkOpacityBuckets,
		ZoneCountVisible:                 summary.ZoneCountVisible,
		PrimaryPathGroupsVisible:         nodeResult.Data.Summary.PrimaryPathGroupsVisible,
		RouteGroups:                      nodeResult.Data.Summary.RouteGroups,
		InspectorRawJSONDefault:          summary.InspectorRawDefault,
		SVGRelationLayerPresent:          summary.SVGRelationLayer,
		SVGLinkPathCount:                 summary.SVGLinkPathCount,
		SVGPrimaryLinkPathCount:          summary.SVGPrimaryPathCount,
		SVGSecondaryLinkPathCount:        summary.SVGSecondaryPathCount,
		SVGAuxiliaryLinkPathCount:        summary.SVGAuxiliaryPathCount,
		VisibleSVGLinkPathCount:          summary.VisibleSVGPathCount,
		RelationLayerBounds:              nodeResult.Data.Summary.RelationLayerBounds,
		LinkPathsWithMarkerCount:         summary.LinkPathsWithMarker,
		LinkPathsWithoutMarkerCount:      summary.LinkPathsWithoutMarker,
		ModelBadgeCount:                  summary.ModelBadges,
		SvgBillboardCount:                summary.SvgBillboards,
		FallbackBadgeCount:               summary.FallbackBadges,
		CanvasVisible:                    checks.CanvasVisible,
		ControlsVisible:                  checks.ControlsPresent,
		ApproximateLabelOverlapCount:     nodeResult.Data.Summary.ApproximateLabelOverlapCount,
		EntityLabelOverlapCount:          nodeResult.Data.Summary.EntityLabelOverlapCount,
		LinkLabelOverlapCount:            nodeResult.Data.Summary.LinkLabelOverlapCount,
		ZoneLabelOverlapCount:            nodeResult.Data.Summary.ZoneLabelOverlapCount,
		TotalLabelOverlapCount:           nodeResult.Data.Summary.TotalLabelOverlapCount,
		LabelsOutsideStageCount:          nodeResult.Data.Summary.LabelsOutsideStageCount,
		LabelsUnderToolbarCount:          nodeResult.Data.Summary.LabelsUnderToolbarCount,
		LabelsUnderInspectorCount:        nodeResult.Data.Summary.LabelsUnderInspectorCount,
		CameraFitIncludesLabels:          nodeResult.Data.Summary.CameraFitIncludesLabels,
		CameraFitReservedInspector:       nodeResult.Data.Summary.CameraFitReservedInspector,
		CameraFitReservedToolbar:         nodeResult.Data.Summary.CameraFitReservedToolbar,
		CameraFitIncludesHTMLLabels:      nodeResult.Data.Summary.CameraFitIncludesHTMLLabels,
		LabelLayerBounds:                 nodeResult.Data.Summary.LabelLayerBounds,
		CanvasBounds:                     nodeResult.Data.Summary.CanvasBounds,
		ScreenshotSize:                   screenshotSize,
		SceneComponentTreePresent:        summary.SceneComponentTree,
		EntityComponentCount:             summary.EntityComponents,
		RelationComponentCount:           summary.RelationComponents,
		HTMLLabelComponentCount:          summary.HTMLLabelComponents,
		LeaderLineComponentCount:         summary.LeaderLineComponents,
		GroundPathBuilderPresent:         summary.GroundPathBuilder,
		GroundPathBuilderVersion:         summary.GroundPathBuilderVersion,
		PathJoinStyle:                    summary.PathJoinStyle,
		PathArrowCapCount:                summary.PathArrowCapCount,
		PathArrowCapIntegratedCount:      summary.PathArrowCapIntegrated,
		PathHitAreaCount:                 summary.PathHitAreaCount,
		PathHoverHaloSupported:           summary.PathHoverHaloSupported,
		PathParallelOffsetCount:          summary.PathParallelOffsetCount,
		PathBundleCount:                  summary.PathBundleCount,
		PathDashSegmentCount:             summary.PathDashSegmentCount,
		RoutePlanPresent:                 summary.RoutePlanPresent,
		RoutePlanVersion:                 summary.RoutePlanVersion,
		RoutePlanBackend:                 summary.RoutePlanBackend,
		RoutePlanRouteCount:              summary.RoutePlanRouteCount,
		RoutePlanLaneCount:               summary.RoutePlanLaneCount,
		RoutePlanObstacleCount:           summary.RoutePlanObstacleCount,
		RoutePlanRenderedMatch:           summary.RoutePlanRenderedMatch,
		RoutePlanRenderedMatchCount:      summary.RoutePlanRenderedMatchCnt,
		SourceEdgeCount:                  summary.SourceEdgeCount,
		DisplayRouteCount:                summary.DisplayRouteCount,
		HiddenDetailRouteCount:           summary.HiddenDetailRouteCount,
		RouteToZoneCount:                 summary.RouteToZoneCount,
		RouteToEntityCount:               summary.RouteToEntityCount,
		RouteToZoneRatio:                 summary.RouteToZoneRatio,
		RouteSameStyleMismatchCount:      summary.RouteSameStyleMismatch,
		PathArrowBodyGapCount:            summary.PathArrowBodyGapCount,
		PathArrowAtBendCount:             summary.PathArrowAtBendCount,
		RouteColorConsistencyScore:       summary.RouteColorConsistencyScore,
		EntityBodyRegistryCount:          summary.EntityBodyRegistryCount,
		EntityKnownBodyCount:             summary.EntityKnownBodyCount,
		EntityGenericBodyCount:           summary.EntityGenericBodyCount,
		EntityGenericBodyRatio:           summary.EntityGenericBodyRatio,
		EntitySemanticBodyScore:          summary.EntitySemanticBodyScore,
		EntityVisualStyleVersion:         summary.EntityVisualStyleVersion,
		EntityVisualPaletteVersion:       summary.EntityVisualPaletteVersion,
		EntityBodyShapeVarietyCount:      summary.EntityBodyShapeVariety,
		EntityContactShadowCount:         summary.EntityContactShadows,
		EntityTopHighlightCount:          summary.EntityTopHighlights,
		EntitySidePanelCount:             summary.EntitySidePanels,
		EntityIconDecalCount:             summary.EntityIconDecals,
		EntityRoundedOrBeveledCount:      summary.EntityRoundedOrBeveled,
		EntityScreenPanelCount:           summary.EntityScreenPanels,
		EntitySemanticModelCoverageCount: summary.EntitySemanticModelCount,
		EntitySemanticModelCoverageRatio: summary.EntitySemanticModelRatio,
		EntityBrightnessScore:            summary.EntityBrightnessScore,
		EntitySaturationScore:            summary.EntitySaturationScore,
		EntityChromaScore:                summary.EntityChromaScore,
		NeutralGrayMaterialRatio:         summary.NeutralGrayMaterialRatio,
		ZoneEntityOverflowCount:          summary.ZoneEntityOverflowCount,
		ZoneLabelOverflowCount:           summary.ZoneLabelOverflowCount,
		ZonePaddingMinPx:                 summary.ZonePaddingMinPx,
		ModelKindCounts:                  summary.ModelKindCounts,
		RelationComponentsOwnPathCount:   summary.RelationOwnsPath,
		RelationComponentsOwnArrowCount:  summary.RelationOwnsArrow,
		RelationComponentsOwnHitCount:    summary.RelationOwnsHit,
		RelationComponentsOwnLabelCount:  summary.RelationOwnsLabel,
		EntityComponentsWithPortsCount:   summary.EntityComponentsWithPorts,
		RelationLayerMode:                summary.RelationLayerMode,
		RelationRenderMode:               summary.RelationRenderMode,
		RelationDepthTestEnabledCount:    summary.RelationDepthEnabled,
		RelationDepthTestDisabledCount:   summary.RelationDepthDisabled,
		RouteEntityIntersectionCount:     summary.RouteEntityIntersections,
		RoutePortHintViolationCount:      summary.RoutePortViolations,
		RouteDirectionViolationCount:     summary.RouteDirectionViolations,
		RouteMaxLengthWorld:              summary.RouteMaxLengthWorld,
		RouteCrossSceneCount:             summary.RouteCrossSceneCount,
		RouteCrossingCount:               summary.RouteCrossingCount,
		RouteParallelOverlapCount:        summary.RouteParallelOverlapCount,
		RoutePathGroupOverlapCount:       summary.RoutePathGroupOverlapCount,
		RouteBusLaneCount:                summary.RouteBusLaneCount,
		RouteBundleCount:                 summary.RouteBundleCount,
		PrimaryRouteCount:                summary.PrimaryRouteCount,
		SecondaryRouteCount:              summary.SecondaryRouteCount,
		AuxiliaryRouteCount:              summary.AuxiliaryRouteCount,
		RelationLooksLikeRaisedBeam:      summary.RaisedBeamLooks,
		WorldRelationLayerPresent:        summary.WorldRelationLayer,
		GroundLinkMeshCount:              summary.GroundLinkMeshes,
		GroundLinkRibbonCount:            summary.GroundLinkRibbons,
		GroundLinkSegmentCount:           summary.GroundLinkSegments,
		GroundRouteRailSegmentCount:      summary.GroundRouteRailSegments,
		GroundRouteRailJointCount:        summary.GroundRouteRailJoints,
		GroundRouteRailArrowheadCount:    summary.GroundRouteRailArrows,
		GroundRouteRailVisibleCount:      summary.GroundRouteRailVisible,
		IsolatedArrowheadCount:           summary.IsolatedArrowheads,
		RoutesWithSegmentsCount:          summary.RoutesWithSegments,
		RoutesWithoutSegmentsCount:       summary.RoutesWithoutSegments,
		VisibleGroundLinkCount:           summary.VisibleGroundLinks,
		GroundArrowheadCount:             summary.GroundArrowheads,
		VisibleGroundArrowheadCount:      summary.VisibleGroundArrowheads,
		GroundLinkHitAreaCount:           summary.GroundLinkHitAreas,
		GenericLinkLabelCount:            summary.GenericLinkLabels,
		InferredLinkLabelCount:           summary.InferredLinkLabels,
		ExplicitLinkLabelCount:           summary.ExplicitLinkLabels,
		LinkLabelMode:                    summary.LinkLabelMode,
		HTMLLinkLabelCount:               summary.HTMLLinkLabels,
		GroundLinkLabelMeshCount:         summary.GroundLinkLabels,
		GroundTextureLinkLabelCount:      summary.GroundTextureLinkLabels,
		GroundLinkLabelTextureReady:      summary.GroundLinkTextures,
		GroundLinkLabelVisibleCount:      summary.GroundLabelsVisible,
		GroundLinkLabelFlippedCount:      summary.GroundLabelsFlipped,
		ScreenSVGRelationLayerVisible:    summary.ScreenSVGVisible,
		SVGDebugRelationLayerPresent:     summary.SVGDebugLayer,
		EntityLabelAnchorCount:           summary.EntityLabelAnchors,
		LinkLabelAnchorCount:             summary.LinkLabelAnchors,
		ZoneLabelAnchorCount:             summary.ZoneLabelAnchors,
		WorldLeaderLineCount:             summary.WorldLeaderLines,
		OrbitSmokeEnabled:                summary.OrbitSmokeEnabled,
		OrbitEntityLabelReturnMaxDelta:   summary.OrbitEntityMaxDelta,
		OrbitEntityLabelReturnAvgDelta:   summary.OrbitEntityAvgDelta,
		OrbitLinkLabelReturnMaxDelta:     summary.OrbitLinkMaxDelta,
		OrbitLinkLabelReturnAvgDelta:     summary.OrbitLinkAvgDelta,
		OrbitMissingEntityLabels:         summary.OrbitMissingEntities,
		OrbitMissingLinkLabels:           summary.OrbitMissingLinks,
		OrbitRelationLayerModeStable:     summary.OrbitLayerStable,
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
	if summary.LabelsUnderToolbar > 0 {
		add("browser_labels_under_toolbar", "warning", fmt.Sprintf("Some labels overlap the toolbar/header: %d.", summary.LabelsUnderToolbar), "Reserve top margin during camera fit and avoid label anchors under controls.", "reserve_toolbar_margin")
	}
	if summary.LabelsUnderInspector > 0 {
		add("browser_labels_under_inspector", "warning", fmt.Sprintf("Some labels overlap the inspector panel: %d.", summary.LabelsUnderInspector), "Reserve right-side inspector margin during camera fit.", "reserve_inspector_margin")
	}
	if !summary.CameraFitIncludesLabels && summary.EntityLabels > 0 {
		add("browser_camera_fit_ignores_labels", "warning", "The browser summary did not confirm that camera fit includes labels.", "Fit the default camera using entity, link, and zone label bounds, not only meshes.", "fit_camera_with_labels")
	}
	hasArchitectureFlowGroup := containsString(summary.RouteGroups, "entry") || containsString(summary.RouteGroups, "gateway")
	if summary.PrimaryLinkCount == 0 && hasArchitectureFlowGroup {
		add("browser_primary_path_missing", "warning", "No primary architecture path links were declared.", "Mark entry/gateway request path links as role=primary.", "declare_primary_links")
	}
	if summary.PrimaryLinkCount > 0 && summary.VisiblePrimaryLabels == 0 {
		add("browser_primary_link_labels_missing", "warning", "Primary architecture path links have no visible overview labels.", "Give at least one primary path link labelPriority=important or always.", "show_primary_path_labels")
	}
	totalDeclaredLinks := summary.PrimaryLinkCount + summary.SecondaryLinkCount + summary.AuxiliaryLinkCount
	if !summary.SceneComponentTree {
		add("browser_scene_component_tree_missing", "error", "The isometric renderer did not report a scene component tree.", "Create EntityComponent, RelationComponent, HtmlLabelComponent, and GroundPathGeometryBuilder owners for the rendered scene.", "create_scene_component_tree")
	}
	if summary.EntityLabels > 0 && summary.EntityComponents < summary.EntityLabels {
		add("browser_entity_components_missing", "error", fmt.Sprintf("Some visible entities are not owned by EntityComponent instances: components=%d labels=%d.", summary.EntityComponents, summary.EntityLabels), "Make each entity component own its body mesh, label, leader line, bbox, ports, and anchors.", "create_entity_components")
	}
	if totalDeclaredLinks > 0 && summary.RelationComponents < totalDeclaredLinks {
		add("browser_relation_components_missing", "error", fmt.Sprintf("Some relations are not owned by RelationComponent instances: components=%d links=%d.", summary.RelationComponents, totalDeclaredLinks), "Make RelationComponent own the path mesh, arrow mesh, hit area, link label, route metrics, and interaction state.", "create_relation_components")
	}
	if totalDeclaredLinks > 0 && summary.RelationOwnsPath < totalDeclaredLinks {
		add("browser_relation_component_path_missing", "error", fmt.Sprintf("Some RelationComponent instances do not own path meshes: %d/%d.", summary.RelationOwnsPath, totalDeclaredLinks), "Attach every ground route mesh to its RelationComponent.", "attach_relation_paths")
	}
	if totalDeclaredLinks > 0 && summary.RelationOwnsArrow < totalDeclaredLinks {
		add("browser_relation_component_arrow_missing", "error", fmt.Sprintf("Some RelationComponent instances do not own arrow meshes: %d/%d.", summary.RelationOwnsArrow, totalDeclaredLinks), "Attach every directed arrowhead mesh to its RelationComponent.", "attach_relation_arrows")
	}
	if totalDeclaredLinks > 0 && summary.RelationOwnsHit < totalDeclaredLinks {
		add("browser_relation_component_hit_area_missing", "warning", fmt.Sprintf("Some RelationComponent instances do not own hit areas: %d/%d.", summary.RelationOwnsHit, totalDeclaredLinks), "Attach invisible hit geometry to each RelationComponent for hover and click interactions.", "attach_relation_hit_areas")
	}
	if totalDeclaredLinks > 0 && summary.GroundPathBuilder == false {
		add("browser_ground_path_builder_missing", "error", "The runtime did not report GroundPathGeometryBuilder.", "Build ground relation path, arrow, hit area, and metrics through a single builder abstraction.", "create_ground_path_geometry_builder")
	}
	if totalDeclaredLinks > 0 && summary.GroundPathBuilder && summary.GroundPathBuilderVersion != "v6" {
		add("browser_ground_path_builder_not_v6", "warning", "GroundPathGeometryBuilder did not report v6 integrated arrow-cap metrics.", "Expose RoutePlan v2 display routes, integrated arrow caps, hit areas, hover halos, joins, dash segments, bundles, and parallel offsets through the builder.", "upgrade_ground_path_builder_v6")
	}
	if totalDeclaredLinks > 0 && !summary.RoutePlanPresent {
		add("browser_route_plan_missing", "warning", "The rendered architecture did not expose a first-class RoutePlan.", "Generate efp.routeplan.v2 during Mermaid compilation and include it in data.js.", "generate_route_plan")
	}
	if summary.RoutePlanPresent && !summary.RoutePlanRenderedMatch {
		add("browser_route_plan_render_mismatch", "warning", fmt.Sprintf("Rendered relation count does not match RoutePlan count: rendered=%d planned=%d.", summary.RoutePlanRenderedMatchCnt, summary.RoutePlanRouteCount), "Have the browser runtime render routePlan.routes directly instead of recomputing routes.", "render_route_plan_routes")
	}
	if summary.SourceEdgeCount >= 18 && summary.DisplayRouteCount > 12 {
		add("browser_display_routes_too_many", "warning", fmt.Sprintf("Overview renders too many display routes: %d display routes for %d source edges.", summary.DisplayRouteCount, summary.SourceEdgeCount), "Aggregate repeated source edges into zone-level or bundle-level display routes for the overview.", "aggregate_display_routes")
	}
	if summary.SourceEdgeCount >= 18 && summary.HiddenDetailRouteCount < 4 {
		add("browser_hidden_detail_routes_low", "warning", fmt.Sprintf("Too few source edges were hidden behind overview display routes: %d.", summary.HiddenDetailRouteCount), "Preserve source edges as hidden detail routes and render aggregated display routes in overview.", "hide_detail_routes")
	}
	if summary.SourceEdgeCount >= 18 && summary.RouteToZoneRatio < 0.65 {
		add("browser_route_to_zone_ratio_low", "warning", fmt.Sprintf("Overview relation endpoints are still too entity-level: zone ratio %.2f.", summary.RouteToZoneRatio), "Route secondary and auxiliary overview relations to zone boundaries or bundle trunks.", "route_to_zone_boundaries")
	}
	if summary.RouteSameStyleMismatch > 0 {
		add("browser_route_style_mismatch", "warning", fmt.Sprintf("Some routes have different body and arrow colors: %d.", summary.RouteSameStyleMismatch), "Use RouteStyle tokens so bodyColor and arrowColor match for each display route.", "unify_route_style_tokens")
	}
	if summary.PathArrowBodyGapCount > 0 {
		add("browser_path_arrow_body_gap", "warning", fmt.Sprintf("Some route arrow caps are separated from the path body: %d.", summary.PathArrowBodyGapCount), "Trim the path body to the arrow base and build arrowheads as integrated terminal caps.", "integrate_arrow_cap")
	}
	if summary.PathArrowAtBendCount > 0 {
		add("browser_path_arrow_at_bend", "warning", fmt.Sprintf("Some route arrows terminate on a bend: %d.", summary.PathArrowAtBendCount), "Backtrack along the final route segment so arrow caps sit on a straight terminal segment.", "move_arrow_off_bend")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityVisualStyleVersion != "" && summary.EntityVisualStyleVersion != "isometric_entity_v3" {
		add("browser_entity_visual_style_old", "warning", fmt.Sprintf("Entity visual style is %s, expected isometric_entity_v3.", summary.EntityVisualStyleVersion), "Use EntityBodySystem v3 procedural bodies for architecture entities.", "upgrade_entity_body_system_v3")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityBodyShapeVariety < 12 {
		add("browser_entity_shape_variety_low", "warning", fmt.Sprintf("Entity body shape variety is low: %d.", summary.EntityBodyShapeVariety), "Use semantic entity bodies for client, CDN, gateway, service, registry, cache, database, storage, observability, and admin.", "increase_entity_shape_variety")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityComponents > 0 && summary.EntityContactShadows < summary.EntityComponents {
		add("browser_entity_contact_shadows_missing", "warning", fmt.Sprintf("Some entities are missing contact shadows: %d/%d.", summary.EntityContactShadows, summary.EntityComponents), "Add subtle ground contact shadows to every entity body.", "add_entity_contact_shadows")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityComponents > 0 && summary.EntityTopHighlights < int(float64(summary.EntityComponents)*0.8) {
		add("browser_entity_top_highlights_missing", "warning", fmt.Sprintf("Too few entities have top highlights: %d/%d.", summary.EntityTopHighlights, summary.EntityComponents), "Add bright top slabs/highlights so isometric entities read as lively 3D objects.", "add_entity_top_highlights")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityIconDecals < 12 {
		add("browser_entity_icon_decals_missing", "warning", fmt.Sprintf("Too few body icon decals are present: %d.", summary.EntityIconDecals), "Place local icon decals or fallback glyph plates on entity body front/top surfaces.", "add_entity_icon_decals")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntitySemanticModelRatio > 0 && summary.EntitySemanticModelRatio < 0.85 {
		add("browser_entity_semantic_model_coverage_low", "warning", fmt.Sprintf("Semantic entity model coverage is low: %.2f.", summary.EntitySemanticModelRatio), "Render known architecture kinds through dedicated procedural body builders instead of generic cubes.", "increase_semantic_entity_model_coverage")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityGenericBodyCount > 0 {
		add("browser_entity_known_kind_rendered_as_cube", "warning", fmt.Sprintf("Some known architecture entities still use generic bodies: %d.", summary.EntityGenericBodyCount), "Map every known kind in the microservice golden example to an EntityBodySystem v3 builder.", "replace_generic_entity_bodies")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityBrightnessScore > 0 && summary.EntityBrightnessScore < 0.78 {
		add("browser_entity_brightness_low", "warning", fmt.Sprintf("Entity palette brightness is low: %.2f.", summary.EntityBrightnessScore), "Use the iCraft-like brighter isometric palette for architecture entities.", "apply_entity_palette_v2")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntitySaturationScore > 0 && summary.EntitySaturationScore < 0.68 {
		add("browser_entity_saturation_low", "warning", fmt.Sprintf("Entity palette saturation is low: %.2f.", summary.EntitySaturationScore), "Use saturated but controlled palette v3 colors for semantic body categories.", "apply_entity_palette_v3")
	}
	if summary.SourceEdgeCount >= 18 && summary.EntityChromaScore > 0 && summary.EntityChromaScore < 0.42 {
		add("browser_entity_chroma_low", "warning", fmt.Sprintf("Entity material chroma is low: %.2f.", summary.EntityChromaScore), "Replace pure gray body/base materials with blue-tinted iCraft neutral surfaces.", "replace_gray_entity_neutrals")
	}
	if summary.SourceEdgeCount >= 18 && summary.NeutralGrayMaterialRatio > 0.15 {
		add("browser_neutral_gray_material_ratio_high", "warning", fmt.Sprintf("Neutral gray material ratio is high: %.2f.", summary.NeutralGrayMaterialRatio), "Use blue-white neutral palette for MySQL, storage, Nacos, admin, and observability bases instead of un-hued gray.", "reduce_neutral_gray_materials")
	}
	if summary.SourceEdgeCount >= 18 && summary.ZoneEntityOverflowCount > 0 {
		add("browser_zone_entity_overflow", "warning", fmt.Sprintf("Some zones do not contain their entity footprints: %d.", summary.ZoneEntityOverflowCount), "Fit zone bounds to child entity footprints before route planning.", "fit_zone_bounds_to_entities")
	}
	if summary.SourceEdgeCount >= 18 && summary.ZoneLabelOverflowCount > 0 {
		add("browser_zone_label_overflow", "warning", fmt.Sprintf("Some zone labels overflow their fitted zone bounds: %d.", summary.ZoneLabelOverflowCount), "Reserve label padding when fitting zone bounds.", "fit_zone_bounds_to_labels")
	}
	if summary.SourceEdgeCount >= 18 && summary.ZonePaddingMinPx > 0 && summary.ZonePaddingMinPx < 12 {
		add("browser_zone_padding_low", "warning", fmt.Sprintf("Minimum zone/entity padding is too small: %dpx.", summary.ZonePaddingMinPx), "Increase zone padding for multi-entity service, storage, registry, cache, and observability areas.", "increase_zone_fit_padding")
	}
	if totalDeclaredLinks > 0 && summary.PathArrowCapCount == 0 {
		add("browser_path_arrow_cap_missing", "warning", "GroundPathGeometryBuilder did not report terminal arrow caps.", "Build arrowheads as route terminal caps owned by RelationComponent.", "build_path_arrow_caps")
	}
	if totalDeclaredLinks > 0 && summary.PathHitAreaCount == 0 {
		add("browser_path_hit_area_missing", "warning", "GroundPathGeometryBuilder did not report hit areas.", "Build a transparent route hit geometry for link hover and selection.", "build_path_hit_areas")
	}
	if summary.EntityComponents > 0 && summary.EntityComponentsWithPorts < summary.EntityComponents {
		add("browser_entity_ports_missing", "error", fmt.Sprintf("Some EntityComponent instances do not expose ports: %d/%d.", summary.EntityComponentsWithPorts, summary.EntityComponents), "Compute north/east/south/west ports from each entity footprint.", "compute_entity_ports")
	}
	if summary.EntityComponents > 0 && summary.EntityGenericBodyRatio > 0.35 {
		add("browser_entity_generic_body_ratio_high", "warning", fmt.Sprintf("Too many entities use generic bodies: %.2f (%d/%d).", summary.EntityGenericBodyRatio, summary.EntityGenericBodyCount, summary.EntityComponents), "Extend EntityBodyRegistry for common architecture kinds so entities remain recognizable without labels.", "extend_entity_body_registry")
	}
	if summary.EntityComponents >= 12 && summary.EntitySemanticBodyScore > 0 && summary.EntitySemanticBodyScore < 0.7 {
		add("browser_entity_semantic_body_score_low", "warning", fmt.Sprintf("Entity semantic body score is low: %.2f.", summary.EntitySemanticBodyScore), "Use semantic body renderers for gateway, service, registry, cache, database, storage, observability, and admin entities.", "extend_entity_body_registry_v2")
	}
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
		add("browser_ground_route_segments_missing", "error", "No world-space ground route segments were reported.", "Render each relation path segment as a ground decal strip instead of relying on screen-space SVG.", "render_ground_decal_segments")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.RelationRenderMode != "" && summary.RelationRenderMode != "ground_decal" {
		add("browser_relation_decal_missing", "error", "The default relation render mode is not ground_decal.", "Use low ground decal strips for architecture relations; do not render raised beams or screen overlays by default.", "set_ground_decal_relation_render_mode")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.RelationDepthDisabled > 0 {
		add("browser_relation_depth_overlay_enabled", "error", fmt.Sprintf("Some relation materials have depthTest disabled: %d.", summary.RelationDepthDisabled), "Keep relation decals depth-tested so entities can naturally occlude lines passing underneath.", "enable_relation_depth_test")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.RaisedBeamLooks > 0 {
		add("browser_relation_looks_like_raised_beam", "warning", fmt.Sprintf("Some relation geometries still look like raised beams: %d.", summary.RaisedBeamLooks), "Use thin ground decal strips with small height instead of thick rail boxes.", "lower_relation_decal_height")
	}
	if totalDeclaredLinks > 0 && summary.RouteEntityIntersections > 0 {
		add("browser_route_intersects_entity", "warning", fmt.Sprintf("Some relation routes cross entity footprints: %d.", summary.RouteEntityIntersections), "Route from bbox ports and insert orthogonal bends around entity footprints.", "avoid_entity_footprints")
	}
	if totalDeclaredLinks > 0 && summary.RouteCrossingCount > 8 {
		add("browser_route_crossing_high", "warning", fmt.Sprintf("Relation route crossings are high: %d.", summary.RouteCrossingCount), "Group links into pathGroup bus lanes and apply parallel offsets before rendering complex architecture maps.", "reduce_route_crossings")
	}
	if totalDeclaredLinks > 0 && summary.RouteParallelOverlapCount > 4 {
		add("browser_route_parallel_overlap", "warning", fmt.Sprintf("Some same-group routes overlap instead of using separate lanes: %d.", summary.RouteParallelOverlapCount), "Offset same pathGroup links in parallel lanes to keep bundled routes legible.", "separate_parallel_routes")
	}
	if totalDeclaredLinks >= 12 && summary.RoutePathGroupOverlapCount > 4 {
		add("browser_route_path_group_overlap", "warning", fmt.Sprintf("PathGroup route overlap remains visible: %d.", summary.RoutePathGroupOverlapCount), "Route registry/data/cache/storage/health through distinct bus lanes with parallel offsets.", "improve_pathgroup_bus_lanes")
	}
	if totalDeclaredLinks >= 12 && summary.RouteBusLaneCount == 0 {
		add("browser_route_bus_lanes_missing", "warning", "Complex architecture routes did not report bus lanes.", "Route registry/data/cache/storage/health pathGroups through stable bus lanes.", "add_pathgroup_bus_lanes")
	}
	if totalDeclaredLinks > 0 && summary.RoutePortViolations > 0 {
		add("browser_route_port_hint_violation", "warning", fmt.Sprintf("Some relation routes violate Mermaid port hints: %d.", summary.RoutePortViolations), "Respect endpoint hints such as from:R --> L:to when ranking and routing architecture diagrams.", "respect_mermaid_port_hints")
	}
	if totalDeclaredLinks > 0 && summary.RouteDirectionViolations > 0 {
		add("browser_route_direction_violation", "warning", fmt.Sprintf("Some relation routes violate expected direction: %d.", summary.RouteDirectionViolations), "Rank nodes from directed edges so R-to-L edges generally move left-to-right.", "fix_mermaid_rank_direction")
	}
	if totalDeclaredLinks > 2 && summary.PrimaryRouteCount == totalDeclaredLinks {
		add("browser_primary_links_too_many", "warning", "Every relation is styled as primary.", "Infer primary only for ingress/gateway paths; data, cache, storage, health, and logs should be secondary or auxiliary.", "fix_relation_role_inference")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundArrowheads == 0 {
		add("browser_ground_arrowheads_missing", "error", "No world-space ground arrowheads were reported.", "Render directed relation arrows as Three.js world-space meshes attached to the route end.", "render_ground_arrowheads")
	}
	if totalDeclaredLinks > 0 && summary.RelationLayerMode == "world_ground" && summary.GroundArrowheads > 0 && summary.GroundLinkRibbons == 0 {
		add("browser_link_lines_missing_but_arrows_present", "error", "World-space arrowheads were reported but no ground relation ribbons were found.", "Render continuous ground route ribbons so arrows are attached to visible link lines.", "render_ground_link_ribbons")
	}
	if summary.RoutesWithoutSegments > 0 {
		add("browser_route_segments_missing", "warning", fmt.Sprintf("Some relation routes have no ground decal segments: %d.", summary.RoutesWithoutSegments), "Skip only zero-length routes; otherwise generate at least one decal segment per relation.", "fix_routes_without_segments")
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
