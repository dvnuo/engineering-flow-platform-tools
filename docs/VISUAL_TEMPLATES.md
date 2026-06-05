# Visual Templates

The visual template catalog is a compact semantic catalog. It contains 33 canonical templates across 8 categories and intentionally does not maintain compatibility aliases. Template discovery must use `visual template categories`, `visual template list`, `visual template get`, and `visual template schema`.

## Registry Expected Counts

`templates/visual/registry.json` owns the expected catalog shape:

```json
{
  "version": 3,
  "expected": {
    "canonical_count": 33,
    "categories": {
      "uml": 5,
      "relationship": 4,
      "temporal": 4,
      "flow": 4,
      "hierarchy": 4,
      "evidence": 4,
      "matrix": 4,
      "spatial": 4
    }
  }
}
```

`visual template doctor` uses this metadata. When adding or removing templates, update `registry.expected`, the registry entries, template directories, smoke scripts, and tests together.

## Directory Consistency

Every direct directory under `templates/visual`, except `_shared`, must correspond to a canonical template entry in `registry.json`. The canonical directory is the dirname of `registry.templates[].path`.

The built-in catalog must report:

- `canonical_template_dirs: 33`
- `orphan_template_dirs: []`

Do not add unregistered examples, legacy alias directories, or ad-hoc template folders.

## Template Quality Bar

- `title` and `description` must be scenario-specific.
- `description` must not be a generic "visualize X as an offline view" sentence.
- `schema.input.json` must contain `template_id`, `input_schema_kind`, `json_schema`, and `example`.
- `schema.input.json` must expose the shared `visual` object with `goal`, `initial_focus_ids`, `hidden_detail_ids`, `narrative_steps`, and `annotations`.
- `examples/basic.input.json` must have a meaningful title.
- `examples/basic.input.json` must fill `visual` with valid semantic ids so agents learn first-view focus, delayed detail, and annotation behavior.
- `style.css` must be non-empty.
- `template.yaml` must declare `effects.engine: three.v1`, a scene id, `visual_design`, local assets, and offline settings.
- Large graph-like examples should include groups, readable labels, relationship types, metadata, and visibility/importance hints.
- UML examples must use the UML semantic schema, not generic graph nodes.

## Categories

- `uml`: structured UML diagrams with dedicated semantic schema.
- `relationship`: general relationship maps and dependency topology.
- `temporal`: time-ordered traces, timelines, replay, and history.
- `flow`: process, approval, data, and journey flow.
- `hierarchy`: layered, tree, ownership, and containment views.
- `evidence`: claims, sources, decisions, and root-cause reasoning.
- `matrix`: positioned items for capability, KPI, risk, and allocation.
- `spatial`: 3D spatial maps for large operational or codebase landscapes.

## UML Templates

`uml.sequence_3d` uses `uml_sequence_v1`:

- `participants`: lifelines with `id`, `label`, `kind`, `group`, and optional metadata/metrics.
- `messages`: ordered arrows with unique numeric `order`, `from`, `to`, `label`, `kind`, `phase`, and status.
- `phases`: stage filter metadata.
- `activations`: lifeline activity bars with `participant_id`, `start_order`, and `end_order`.
- `fragments`: `alt`, `loop`, `opt`, or `par` regions.

The sequence renderer draws 3D lifelines, activation bars, directional message lines, labels, phase filtering, replay, reset, raycast inspection, and orbit controls.

Other UML templates use semantic inputs and transform them into the shared 3D graph runtime:

- `uml.class_structure_2_5d`: `classes` plus `relationships`
- `uml.state_machine_3d`: `states` plus `transitions`
- `uml.activity_flow_3d`: `lanes`, `actions`, and `flows`
- `uml.component_deployment_3d`: `deployments`, `components`, and `links`

## Complete Index

### UML

- `uml.activity_flow_3d`: UML Activity Flow 3D (`uml_activity_v1`, `offline.uml.activity.3d.v1`)
- `uml.class_structure_2_5d`: UML Class Structure 2.5D (`uml_class_v1`, `offline.uml.class.2_5d.v1`)
- `uml.component_deployment_3d`: UML Component Deployment 3D (`uml_component_deployment_v1`, `offline.uml.component.3d.v1`)
- `uml.sequence_3d`: UML Sequence 3D (`uml_sequence_v1`, `offline.uml.sequence.3d.v1`)
- `uml.state_machine_3d`: UML State Machine 3D (`uml_state_machine_v1`, `offline.uml.state.3d.v1`)

### Relationship

- `relationship.dependency_graph`: Dependency Relationship Graph
- `relationship.issue_dependencies`: Issue Dependency Relationship Map
- `relationship.knowledge_lineage`: Knowledge Lineage Relationship Map
- `relationship.service_topology`: Service Topology Relationship Map

### Temporal

- `temporal.automation_replay`: Automation Run Replay
- `temporal.event_trace`: Agent Event Trace
- `temporal.incident_timeline`: Incident Timeline
- `temporal.release_history`: Release History Timeline

### Flow

- `flow.approval`: Approval Flow
- `flow.customer_journey`: Customer Journey Flow
- `flow.data_flow`: Runtime Data Flow
- `flow.pipeline`: Pipeline Flow

### Hierarchy

- `hierarchy.layered_architecture`: Layered Architecture Hierarchy
- `hierarchy.ownership_map`: Ownership Hierarchy Map
- `hierarchy.package_containment`: Package Containment Hierarchy
- `hierarchy.repository_tree`: Repository Workspace Tree

### Evidence

- `evidence.claim_source_board`: Claim Source Evidence Board
- `evidence.doc_freshness`: Documentation Freshness Evidence Map
- `evidence.risk_decision`: Risk Decision Evidence Matrix
- `evidence.root_cause_tree`: Root Cause Evidence Tree

### Matrix

- `matrix.capability`: Capability Matrix
- `matrix.kpi_control`: KPI Control Matrix
- `matrix.resource_allocation`: Resource Allocation Matrix
- `matrix.risk`: Project Risk Matrix

### Spatial

- `spatial.agent_fleet`: Agent Fleet Space
- `spatial.codebase_galaxy`: Codebase Galaxy
- `spatial.control_room`: Control Room Space
- `spatial.service_city`: Service City Map
