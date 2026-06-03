# Visual Templates

The visual CLI is an offline static site generator. It reads templates from `templates/visual`, validates one local input JSON file, copies local runtime assets, writes `manifest.js` and `data.js`, and produces a directory that can be opened through `file://` or served as static files under a Portal/runtime proxy subpath.

## Design Principles

- Templates live in git as plain files and are not embedded into the executable.
- Template output uses only relative paths and never fetches `data.json` or `manifest.json`.
- Shared renderers are reused across the catalog; templates choose schema kind and layout preset.
- Inputs use a small set of reusable contracts: `graph_v1`, `graph_events_v1`, `timeline_v1`, `evidence_v1`, and `matrix_v1`.
- Template files and generated artifacts must not reference remote assets, network APIs, module scripts, or root-relative resources.

## Categories

- `foundation`: base visual forms used by higher-level templates
- `agent`: agent execution, tool use, replay, confidence, and session state
- `codebase`: repository structure, dependencies, diffs, tests, ownership, and migration
- `runtime`: service, adapter, session, sandbox, event, secret, and deployment topology
- `debug`: incidents, traces, logs, failures, resource pressure, and recovery
- `project`: Jira, GitHub, Confluence, release, review, and stakeholder workflows
- `knowledge`: research, evidence, citation, decision, source, and answer lineage
- `planning`: plans, tasks, automation, approvals, scheduling, handoff, and goals
- `business`: product, KPI, customer, revenue, support, capacity, and ops views
- `education`: tutorial, architecture, process, lifecycle, and tradeoff explanations

## Input Schema Kinds

- `graph_v1`: nodes and edges for dependency, topology, tree, state, and flow views.
- `graph_events_v1`: graph plus ordered events for traces, replays, recoveries, and animated flows.
- `timeline_v1`: ordered events for incident, roadmap, history, cohort, and lifecycle views.
- `evidence_v1`: claims, sources, and evidence links for research and decision views.
- `matrix_v1`: positioned items for boards, dashboards, heatmaps, radars, and portfolio views.

## Layout Presets

- `citation_map`
- `city_map`
- `constellation`
- `control_room`
- `dag`
- `decision_matrix`
- `diff_split_view`
- `document_wall`
- `evidence_board`
- `fleet`
- `flow_particles`
- `funnel`
- `galaxy`
- `gantt`
- `graph_2_5d`
- `graph_3d`
- `heatmap`
- `incident_timeline`
- `journey`
- `kanban`
- `knowledge_graph`
- `layered_stack`
- `line`
- `matrix_board`
- `network_boundary_map`
- `orbit_system`
- `permission_gate`
- `pipeline_flow`
- `radar_sphere`
- `radial_tree`
- `replay_stage`
- `ripple`
- `river`
- `roadmap`
- `sankey_3d`
- `service_map`
- `state_machine`
- `step_ladder`
- `swimlane_timeline`
- `terrain_heatmap`
- `timeline_tunnel`
- `waterfall`

## Complete Index

### Foundation

- `foundation.city_map`: City Map (`graph_v1`, `city_map`)
- `foundation.constellation`: Constellation (`graph_v1`, `constellation`)
- `foundation.control_room`: Control Room (`matrix_v1`, `control_room`)
- `foundation.diff_split_view`: Diff Split View (`graph_events_v1`, `diff_split_view`)
- `foundation.document_wall`: Document Wall (`evidence_v1`, `document_wall`)
- `foundation.flow_particles`: Flow Particles (`graph_events_v1`, `flow_particles`)
- `foundation.graph_2_5d`: Graph 2 5D (`graph_v1`, `graph_2_5d`)
- `foundation.graph_3d`: Graph 3D (`graph_v1`, `graph_3d`)
- `foundation.layered_stack`: Layered Stack (`graph_v1`, `layered_stack`)
- `foundation.matrix_board`: Matrix Board (`matrix_v1`, `matrix_board`)
- `foundation.orbit_system`: Orbit System (`graph_v1`, `orbit_system`)
- `foundation.pipeline_flow`: Pipeline Flow (`graph_v1`, `pipeline_flow`)
- `foundation.radar_sphere`: Radar Sphere (`matrix_v1`, `radar_sphere`)
- `foundation.radial_tree`: Radial Tree (`graph_v1`, `radial_tree`)
- `foundation.replay_stage`: Replay Stage (`graph_events_v1`, `replay_stage`)
- `foundation.sankey_3d`: Sankey 3D (`graph_v1`, `sankey_3d`)
- `foundation.state_machine`: State Machine (`graph_v1`, `state_machine`)
- `foundation.swimlane_timeline`: Swimlane Timeline (`timeline_v1`, `swimlane_timeline`)
- `foundation.terrain_heatmap`: Terrain Heatmap (`matrix_v1`, `terrain_heatmap`)
- `foundation.timeline_tunnel`: Timeline Tunnel (`timeline_v1`, `timeline_tunnel`)

### Agent

- `agent.active_run_monitor`: Active Run Monitor (`graph_events_v1`, `fleet`)
- `agent.confidence_map`: Confidence Map (`evidence_v1`, `evidence_board`)
- `agent.context_window_map`: Context Window Map (`matrix_v1`, `matrix_board`)
- `agent.debug_blackbox`: Debug Blackbox (`graph_events_v1`, `control_room`)
- `agent.failure_recovery_tree`: Failure Recovery Tree (`graph_events_v1`, `radial_tree`)
- `agent.permission_gate_map`: Permission Gate Map (`graph_events_v1`, `layered_stack`)
- `agent.react_loop_replay`: React Loop Replay (`graph_events_v1`, `replay_stage`)
- `agent.run_replay_timeline`: Run Replay Timeline (`graph_events_v1`, `replay_stage`)
- `agent.run_trace`: Run Trace (`graph_events_v1`, `timeline_tunnel`)
- `agent.session_state_panel`: Session State Panel (`matrix_v1`, `control_room`)
- `agent.step_ladder`: Step Ladder (`graph_events_v1`, `pipeline_flow`)
- `agent.subagent_swarm`: Subagent Swarm (`graph_events_v1`, `constellation`)
- `agent.thinking_timeline`: Thinking Timeline (`graph_events_v1`, `swimlane_timeline`)
- `agent.tool_call_constellation`: Tool Call Constellation (`graph_events_v1`, `constellation`)
- `agent.tool_io_inspector`: Tool Io Inspector (`graph_events_v1`, `diff_split_view`)

### Codebase

- `codebase.api_surface_map`: API Surface Map (`graph_v1`, `layered_stack`)
- `codebase.bug_localization_map`: Bug Localization Map (`graph_events_v1`, `heatmap`)
- `codebase.call_path_tube`: Call Path Tube (`graph_v1`, `pipeline_flow`)
- `codebase.config_dependency_map`: Config Dependency Map (`graph_v1`, `radial_tree`)
- `codebase.coverage_terrain`: Coverage Terrain (`matrix_v1`, `terrain_heatmap`)
- `codebase.dead_code_detector`: Dead Code Detector (`graph_v1`, `constellation`)
- `codebase.dependency_upgrade_risk`: Dependency Upgrade Risk (`graph_v1`, `radar_sphere`)
- `codebase.diff_impact_ripple`: Diff Impact Ripple (`graph_events_v1`, `ripple`)
- `codebase.galaxy`: Galaxy (`graph_v1`, `galaxy`)
- `codebase.git_history_river`: Git History River (`timeline_v1`, `river`)
- `codebase.hotspot_city`: Hotspot City (`graph_v1`, `city_map`)
- `codebase.migration_progress_map`: Migration Progress Map (`graph_events_v1`, `pipeline_flow`)
- `codebase.module_dependency_graph`: Module Dependency Graph (`graph_v1`, `layered_stack`)
- `codebase.monorepo_workspace_map`: Monorepo Workspace Map (`graph_v1`, `city_map`)
- `codebase.ownership_map`: Ownership Map (`graph_v1`, `matrix_board`)
- `codebase.pr_diff_impact_map`: PR Diff Impact Map (`graph_events_v1`, `ripple`)
- `codebase.refactor_plan_map`: Refactor Plan Map (`graph_v1`, `diff_split_view`)
- `codebase.security_sensitive_files_map`: Security Sensitive Files Map (`graph_v1`, `network_boundary_map`)
- `codebase.symbol_graph`: Symbol Graph (`graph_v1`, `graph_2_5d`)
- `codebase.test_failure_map`: Test Failure Map (`graph_events_v1`, `layered_stack`)

### Runtime

- `runtime.adapter_proxy_map`: Adapter Proxy Map (`graph_v1`, `pipeline_flow`)
- `runtime.agent_fleet_dashboard`: Agent Fleet Dashboard (`matrix_v1`, `control_room`)
- `runtime.api_contract_map`: API Contract Map (`graph_v1`, `matrix_board`)
- `runtime.capability_matrix`: Capability Matrix (`matrix_v1`, `matrix_board`)
- `runtime.config_overlay_view`: Config Overlay View (`graph_v1`, `layered_stack`)
- `runtime.deployment_pipeline_3d`: Deployment Pipeline 3D (`graph_events_v1`, `pipeline_flow`)
- `runtime.event_bus_flow`: Event Bus Flow (`graph_events_v1`, `flow_particles`)
- `runtime.event_reconcile_loop`: Event Reconcile Loop (`graph_events_v1`, `state_machine`)
- `runtime.health_radar`: Health Radar (`matrix_v1`, `radar_sphere`)
- `runtime.k8s_pod_constellation`: K8s Pod Constellation (`graph_v1`, `constellation`)
- `runtime.mcp_server_status_map`: MCP Server Status Map (`matrix_v1`, `matrix_board`)
- `runtime.network_boundary_map`: Network Boundary Map (`graph_v1`, `layered_stack`)
- `runtime.permission_boundary_map`: Permission Boundary Map (`graph_v1`, `layered_stack`)
- `runtime.restart_map`: Restart Map (`graph_events_v1`, `radial_tree`)
- `runtime.sandbox_layer_stack`: Sandbox Layer Stack (`graph_v1`, `layered_stack`)
- `runtime.secret_flow_map`: Secret Flow Map (`graph_v1`, `network_boundary_map`)
- `runtime.service_topology`: Service Topology (`graph_v1`, `service_map`)
- `runtime.session_binding_map`: Session Binding Map (`graph_v1`, `layered_stack`)
- `runtime.tool_registry_map`: Tool Registry Map (`graph_v1`, `radial_tree`)
- `runtime.topology`: Topology (`graph_v1`, `layered_stack`)

### Debug

- `debug.alert_correlation_map`: Alert Correlation Map (`graph_v1`, `constellation`)
- `debug.before_after_compare`: Before After Compare (`graph_events_v1`, `diff_split_view`)
- `debug.blast_radius_sphere`: Blast Radius Sphere (`graph_v1`, `radial_tree`)
- `debug.ci_failure_timeline`: CI Failure Timeline (`timeline_v1`, `swimlane_timeline`)
- `debug.config_drift_map`: Config Drift Map (`matrix_v1`, `diff_split_view`)
- `debug.dependency_failure_map`: Dependency Failure Map (`graph_v1`, `service_map`)
- `debug.dependency_version_conflict`: Dependency Version Conflict (`graph_v1`, `graph_2_5d`)
- `debug.error_budget_burn`: Error Budget Burn (`timeline_v1`, `funnel`)
- `debug.error_spike_field`: Error Spike Field (`timeline_v1`, `terrain_heatmap`)
- `debug.flaky_test_radar`: Flaky Test Radar (`matrix_v1`, `radar_sphere`)
- `debug.incident_timeline`: Incident Timeline (`timeline_v1`, `incident_timeline`)
- `debug.latency_heatmap`: Latency Heatmap (`matrix_v1`, `heatmap`)
- `debug.log_cluster_map`: Log Cluster Map (`graph_v1`, `constellation`)
- `debug.queue_backlog_map`: Queue Backlog Map (`matrix_v1`, `control_room`)
- `debug.recovery_playbook_view`: Recovery Playbook View (`graph_events_v1`, `step_ladder`)
- `debug.regression_detector`: Regression Detector (`timeline_v1`, `river`)
- `debug.resource_pressure_map`: Resource Pressure Map (`matrix_v1`, `radar_sphere`)
- `debug.retry_storm_map`: Retry Storm Map (`graph_events_v1`, `flow_particles`)
- `debug.root_cause_tree`: Root Cause Tree (`graph_v1`, `radial_tree`)
- `debug.trace_waterfall_3d`: Trace Waterfall 3D (`timeline_v1`, `waterfall`)

### Project

- `project.blocker_funnel`: Blocker Funnel (`graph_v1`, `funnel`)
- `project.bug_triage_board`: Bug Triage Board (`matrix_v1`, `kanban`)
- `project.confluence_space_map`: Confluence Space Map (`graph_v1`, `radial_tree`)
- `project.cross_doc_citation_graph`: Cross Doc Citation Graph (`evidence_v1`, `citation_map`)
- `project.decision_record_wall`: Decision Record Wall (`evidence_v1`, `document_wall`)
- `project.doc_freshness_map`: Doc Freshness Map (`matrix_v1`, `heatmap`)
- `project.epic_story_map`: Epic Story Map (`graph_v1`, `layered_stack`)
- `project.issue_age_heatmap`: Issue Age Heatmap (`matrix_v1`, `heatmap`)
- `project.issue_dependency_graph`: Issue Dependency Graph (`graph_v1`, `graph_2_5d`)
- `project.pr_review_map`: PR Review Map (`graph_v1`, `graph_2_5d`)
- `project.release_readiness_dashboard`: Release Readiness Dashboard (`matrix_v1`, `control_room`)
- `project.release_train`: Release Train (`graph_events_v1`, `pipeline_flow`)
- `project.requirements_to_code_trace`: Requirements To Code Trace (`graph_v1`, `layered_stack`)
- `project.review_bottleneck_map`: Review Bottleneck Map (`matrix_v1`, `control_room`)
- `project.risk_matrix`: Risk Matrix (`matrix_v1`, `matrix_board`)
- `project.roadmap_milestone_path`: Roadmap Milestone Path (`timeline_v1`, `roadmap`)
- `project.sprint_board_3d`: Sprint Board 3D (`matrix_v1`, `kanban`)
- `project.stakeholder_network`: Stakeholder Network (`graph_v1`, `constellation`)
- `project.support_escalation_graph`: Support Escalation Graph (`graph_v1`, `layered_stack`)
- `project.velocity_flow`: Velocity Flow (`graph_events_v1`, `flow_particles`)

### Knowledge

- `knowledge.answer_lineage_view`: Answer Lineage View (`evidence_v1`, `radial_tree`)
- `knowledge.argument_map`: Argument Map (`evidence_v1`, `graph_2_5d`)
- `knowledge.citation_map`: Citation Map (`evidence_v1`, `citation_map`)
- `knowledge.claim_evidence_tree`: Claim Evidence Tree (`evidence_v1`, `radial_tree`)
- `knowledge.comparison_table_3d`: Comparison Table 3D (`matrix_v1`, `matrix_board`)
- `knowledge.concept_dependency_tree`: Concept Dependency Tree (`graph_v1`, `radial_tree`)
- `knowledge.decision_matrix`: Decision Matrix (`matrix_v1`, `decision_matrix`)
- `knowledge.design_tradeoff_radar`: Design Tradeoff Radar (`matrix_v1`, `radar_sphere`)
- `knowledge.document_cluster_map`: Document Cluster Map (`evidence_v1`, `constellation`)
- `knowledge.document_diff_map`: Document Diff Map (`evidence_v1`, `diff_split_view`)
- `knowledge.evidence_board`: Evidence Board (`evidence_v1`, `evidence_board`)
- `knowledge.faq_constellation`: Faq Constellation (`graph_v1`, `constellation`)
- `knowledge.graph`: Graph (`graph_v1`, `knowledge_graph`)
- `knowledge.research_timeline`: Research Timeline (`timeline_v1`, `timeline_tunnel`)
- `knowledge.semantic_search_landscape`: Semantic Search Landscape (`matrix_v1`, `terrain_heatmap`)
- `knowledge.source_gap_map`: Source Gap Map (`evidence_v1`, `heatmap`)
- `knowledge.source_reliability_matrix`: Source Reliability Matrix (`matrix_v1`, `matrix_board`)
- `knowledge.spec_compliance_map`: Spec Compliance Map (`graph_v1`, `matrix_board`)
- `knowledge.topic_landscape`: Topic Landscape (`matrix_v1`, `terrain_heatmap`)
- `knowledge.unknowns_map`: Unknowns Map (`matrix_v1`, `matrix_board`)

### Planning

- `planning.approval_workflow`: Approval Workflow (`graph_events_v1`, `permission_gate`)
- `planning.automation_flow`: Automation Flow (`graph_v1`, `pipeline_flow`)
- `planning.automation_run_replay`: Automation Run Replay (`graph_events_v1`, `replay_stage`)
- `planning.checklist_progress_orbit`: Checklist Progress Orbit (`graph_v1`, `orbit_system`)
- `planning.critical_path_view`: Critical Path View (`graph_v1`, `dag`)
- `planning.decision_tree`: Decision Tree (`graph_v1`, `radial_tree`)
- `planning.dependency_radar`: Dependency Radar (`graph_v1`, `radar_sphere`)
- `planning.gantt_tunnel`: Gantt Tunnel (`timeline_v1`, `gantt`)
- `planning.goal_breakdown_tree`: Goal Breakdown Tree (`graph_v1`, `radial_tree`)
- `planning.handoff_map`: Handoff Map (`graph_events_v1`, `pipeline_flow`)
- `planning.kanban_3d`: Kanban 3D (`matrix_v1`, `kanban`)
- `planning.milestone_road`: Milestone Road (`timeline_v1`, `roadmap`)
- `planning.parallel_sessions_map`: Parallel Sessions Map (`graph_events_v1`, `swimlane_timeline`)
- `planning.plan_dag`: Plan Dag (`graph_v1`, `dag`)
- `planning.queue_scheduler_view`: Queue Scheduler View (`matrix_v1`, `kanban`)
- `planning.resource_allocation_board`: Resource Allocation Board (`matrix_v1`, `matrix_board`)
- `planning.risk_burndown`: Risk Burndown (`timeline_v1`, `line`)
- `planning.runbook_execution_map`: Runbook Execution Map (`graph_events_v1`, `step_ladder`)
- `planning.task_failure_fork`: Task Failure Fork (`graph_events_v1`, `radial_tree`)
- `planning.task_swarm`: Task Swarm (`graph_events_v1`, `constellation`)

### Business

- `business.capacity_planning_map`: Capacity Planning Map (`matrix_v1`, `control_room`)
- `business.churn_signal_radar`: Churn Signal Radar (`matrix_v1`, `radar_sphere`)
- `business.cohort_timeline`: Cohort Timeline (`timeline_v1`, `swimlane_timeline`)
- `business.cost_cloud`: Cost Cloud (`graph_v1`, `constellation`)
- `business.customer_journey_map`: Customer Journey Map (`timeline_v1`, `journey`)
- `business.customer_segment_constellation`: Customer Segment Constellation (`graph_v1`, `constellation`)
- `business.experiment_result_view`: Experiment Result View (`matrix_v1`, `diff_split_view`)
- `business.feature_adoption_map`: Feature Adoption Map (`matrix_v1`, `heatmap`)
- `business.funnel_flow_3d`: Funnel Flow 3D (`graph_v1`, `funnel`)
- `business.kpi_control_room`: KPI Control Room (`matrix_v1`, `control_room`)
- `business.market_map`: Market Map (`matrix_v1`, `terrain_heatmap`)
- `business.opportunity_landscape`: Opportunity Landscape (`matrix_v1`, `terrain_heatmap`)
- `business.ops_shift_overview`: Ops Shift Overview (`matrix_v1`, `control_room`)
- `business.portfolio_matrix`: Portfolio Matrix (`matrix_v1`, `matrix_board`)
- `business.revenue_stream_map`: Revenue Stream Map (`graph_v1`, `sankey_3d`)
- `business.risk_matrix_3d`: Risk Matrix 3D (`matrix_v1`, `matrix_board`)
- `business.sales_pipeline_3d`: Sales Pipeline 3D (`graph_v1`, `pipeline_flow`)
- `business.sla_dashboard`: SLA Dashboard (`matrix_v1`, `control_room`)
- `business.support_ticket_heatmap`: Support Ticket Heatmap (`matrix_v1`, `heatmap`)
- `business.team_load_balance`: Team Load Balance (`matrix_v1`, `matrix_board`)

### Education

- `education.api_lifecycle_view`: API Lifecycle View (`timeline_v1`, `pipeline_flow`)
- `education.auth_flow_animation`: Auth Flow Animation (`graph_events_v1`, `pipeline_flow`)
- `education.cache_behavior_sim`: Cache Behavior Sim (`graph_events_v1`, `state_machine`)
- `education.compiler_pipeline_view`: Compiler Pipeline View (`graph_v1`, `pipeline_flow`)
- `education.data_lifecycle_view`: Data Lifecycle View (`graph_events_v1`, `pipeline_flow`)
- `education.database_query_plan`: Database Query Plan (`graph_v1`, `radial_tree`)
- `education.distributed_consensus_view`: Distributed Consensus View (`graph_events_v1`, `constellation`)
- `education.event_sourcing_view`: Event Sourcing View (`graph_events_v1`, `timeline_tunnel`)
- `education.exploded_architecture_view`: Exploded Architecture View (`graph_v1`, `layered_stack`)
- `education.memory_layout_view`: Memory Layout View (`graph_v1`, `layered_stack`)
- `education.migration_strategy_view`: Migration Strategy View (`graph_events_v1`, `diff_split_view`)
- `education.ml_pipeline_view`: ML Pipeline View (`graph_v1`, `pipeline_flow`)
- `education.network_packet_path`: Network Packet Path (`graph_events_v1`, `pipeline_flow`)
- `education.payment_flow_view`: Payment Flow View (`graph_events_v1`, `pipeline_flow`)
- `education.permission_model_view`: Permission Model View (`matrix_v1`, `matrix_board`)
- `education.process_animation`: Process Animation (`graph_events_v1`, `flow_particles`)
- `education.queue_processing_sim`: Queue Processing Sim (`graph_events_v1`, `flow_particles`)
- `education.security_threat_model`: Security Threat Model (`graph_v1`, `network_boundary_map`)
- `education.state_transition_tutorial`: State Transition Tutorial (`graph_v1`, `state_machine`)
- `education.tradeoff_stage`: Tradeoff Stage (`matrix_v1`, `matrix_board`)

## Choosing A Template

Use `agent` or `debug` for agent runs, incidents, traces, and recovery views. Use `codebase` for repository, diff, test, dependency, ownership, and migration work. Use `runtime` for service, infra, adapter, sandbox, capability, and session topology. Use `project` for Jira, GitHub, Confluence, release, and review workflows. Use `knowledge` for evidence, research, citation, claims, and source quality. Use `planning` for tasks, goals, workflow, schedules, and automation. Use `business` for KPI, funnel, revenue, support, capacity, and ops. Use `education` for tutorial, process, lifecycle, and explanatory visuals.

## Commands

```bash
visual template categories --template-dir ./templates/visual --json
visual template list --template-dir ./templates/visual --category codebase --json
visual template get codebase.module_dependency_graph --template-dir ./templates/visual --json
visual template schema codebase.module_dependency_graph --template-dir ./templates/visual --json
visual validate --template codebase.module_dependency_graph --template-dir ./templates/visual --input input.json --json
visual render --template codebase.module_dependency_graph --template-dir ./templates/visual --input input.json --out ./out/module-map --json
```

## Opening Output

The render result includes `relative_entrypoint: index.html`, `file_url_safe: true`, and `http_subpath_safe: true`. In VS Code, open the rendered `index.html` file directly. A Portal/runtime proxy can serve the output directory as static files under any subpath because all resources are relative.

## Adding Template 196

Create `templates/visual/<template-id>/template.yaml`, `schema.input.json`, `style.css`, and `examples/basic.input.json`. Add one registry entry with a unique id, legal category, supported schema kind, supported renderer, supported layout preset, tags, and any aliases. Reuse shared runtime assets, keep `offline.required` and `offline.forbid_network` true, use `data_mode: js-file`, and run doctor before committing.

## Doctor Acceptance

`visual template doctor --template-dir ./templates/visual --json` checks the registry, category counts, manifests, schemas, examples, rendering, output inspection, offline scanning, style file presence, external URL absence, and example uniqueness. The catalog passes when it reports 195 checked templates, 195 checked examples, 195 rendered examples, and `offline: true`.
