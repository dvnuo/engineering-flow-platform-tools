(function () {
  "use strict";

  var runtime = window.EFPVisualRuntime;
  var svgNS = ["h", "ttp:", "/", "/", "www.w3.org/2000/svg"].join("");

  function el(tag, className, value) {
    return runtime.element(tag, className, value);
  }

  function svg(tag, attrs) {
    var node = document.createElementNS(svgNS, tag);
    Object.keys(attrs || {}).forEach(function (key) {
      node.setAttribute(key, attrs[key]);
    });
    return node;
  }

  function appShell(container, manifest) {
    container.textContent = "";
    var app = el("div", "visual-app");
    var header = el("header", "visual-header");
    header.appendChild(el("h1", "visual-title", manifest.title || "Visual Artifact"));
    header.appendChild(el("div", "visual-subtitle", (manifest.template && manifest.template.id ? manifest.template.id : "visual") + " · " + (manifest.renderer && manifest.renderer.contract ? manifest.renderer.contract : "")));
    var toolbar = el("div", "visual-toolbar");
    var content = el("main", "visual-content");
    var preset = normalizePreset(manifest && manifest.layout && manifest.layout.preset);
    var stage = el("section", "visual-stage " + presetClass(preset));
    stage.setAttribute("data-preset", preset);
    var inspectorBox = el("aside", "visual-inspector");
    content.appendChild(stage);
    content.appendChild(inspectorBox);
    app.appendChild(header);
    app.appendChild(toolbar);
    app.appendChild(content);
    container.appendChild(app);
    return { app: app, toolbar: toolbar, stage: stage, inspector: runtime.createInspector(inspectorBox) };
  }

  function presetClass(preset) {
    preset = normalizePreset(preset);
    return "preset-" + preset.replace(/_/g, "-");
  }

  function normalizePreset(preset) {
    preset = String(preset || "constellation").toLowerCase();
    var aliases = {
      dag: "layered_stack",
      tree: "radial_tree",
      galaxy: "constellation",
      service_map: "layered_stack",
      fleet: "control_room",
      incident_timeline: "swimlane_timeline",
      evidence_board: "document_wall",
      knowledge_graph: "constellation",
      decision_matrix: "matrix_board",
      kanban: "matrix_board",
      gantt: "timeline_tunnel",
      roadmap: "timeline_tunnel",
      journey: "swimlane_timeline",
      funnel: "pipeline_flow",
      radar: "radar_sphere",
      waterfall: "swimlane_timeline",
      heatmap: "terrain_heatmap",
      river: "timeline_tunnel",
      board: "matrix_board",
      network_boundary_map: "layered_stack",
      permission_gate: "pipeline_flow",
      step_ladder: "pipeline_flow",
      line: "timeline_tunnel",
      citation_map: "document_wall",
      sequence: "sequence_lifelines",
      sequence_3d: "sequence_lifelines",
      class_diagram: "class_cards",
      activity: "activity_swimlanes",
      component: "component_deployment"
    };
    return aliases[preset] || preset;
  }

  function profileForPreset(preset) {
    preset = normalizePreset(preset);
    var profile = {
      key: "depth",
      particles: true,
      grid: true,
      radar: false,
      tunnel: false,
      heat: false,
      city: false,
      document: false,
      matrix: false
    };
    if (preset === "timeline_tunnel" || preset === "swimlane_timeline" || preset === "replay_stage") {
      profile.key = "tunnel";
      profile.tunnel = true;
    } else if (preset === "flow_particles" || preset === "pipeline_flow" || preset === "sankey_3d") {
      profile.key = "particles";
      profile.particles = true;
      profile.tunnel = true;
    } else if (preset === "radar_sphere" || preset === "orbit_system" || preset === "ripple") {
      profile.key = "radar";
      profile.radar = true;
    } else if (preset === "terrain_heatmap" || preset === "heatmap") {
      profile.key = "terrain";
      profile.heat = true;
      profile.particles = false;
    } else if (preset === "city_map") {
      profile.key = "city";
      profile.city = true;
    } else if (preset === "document_wall" || preset === "citation_map" || preset === "evidence_board") {
      profile.key = "documents";
      profile.document = true;
    } else if (preset === "matrix_board" || preset === "control_room") {
      profile.key = "matrix";
      profile.matrix = true;
    } else if (preset === "sequence_lifelines" || preset === "class_cards" || preset === "activity_swimlanes" || preset === "component_deployment") {
      profile.key = "space";
    } else if (preset === "graph_3d" || preset === "graph_2_5d" || preset === "constellation") {
      profile.key = "space";
    }
    return profile;
  }

  function decorateStage(stage, manifest, data, preset) {
    var profile = profileForPreset(preset);
    stage.classList.add("visual-effect-" + profile.key);
    attachStageInteraction(stage);
    var layer = el("div", "visual-scene-layer visual-scene-" + profile.key);
    layer.setAttribute("aria-hidden", "true");
    if (profile.grid || profile.tunnel || profile.heat || profile.city || profile.matrix) {
      layer.appendChild(el("div", "visual-depth-grid"));
    }
    if (profile.tunnel) {
      layer.appendChild(el("div", "visual-tunnel-rings"));
    }
    if (profile.radar) {
      layer.appendChild(el("div", "visual-radar-sweep"));
    }
    if (profile.heat) {
      layer.appendChild(el("div", "visual-heat-field"));
    }
    var dots = Math.min(42, 16 + countDataItems(data));
    for (var i = 0; i < dots; i += 1) {
      var dot = el("i", "visual-space-dot");
      dot.style.left = ((i * 37) % 100) + "%";
      dot.style.top = ((i * 53 + 17) % 100) + "%";
      dot.style.animationDelay = ((i % 11) * -0.37) + "s";
      dot.style.opacity = String(0.22 + (i % 5) * 0.12);
      layer.appendChild(dot);
    }
    stage.appendChild(layer);
    return profile;
  }

  function countDataItems(data) {
    return ["nodes", "edges", "events", "claims", "sources", "links", "items", "participants", "messages", "classes", "relationships", "states", "transitions", "actions", "flows", "components", "deployments"].reduce(function (sum, key) {
      return sum + (Array.isArray(data && data[key]) ? data[key].length : 0);
    }, 0);
  }

  function attachStageInteraction(stage) {
    if (stage.__efpVisualInteraction) {
      return;
    }
    stage.__efpVisualInteraction = true;
    stage.style.setProperty("--tilt-x", "0deg");
    stage.style.setProperty("--tilt-y", "0deg");
    stage.addEventListener("pointermove", function (event) {
      var rect = stage.getBoundingClientRect();
      var nx = rect.width ? (event.clientX - rect.left) / rect.width - 0.5 : 0;
      var ny = rect.height ? (event.clientY - rect.top) / rect.height - 0.5 : 0;
      stage.style.setProperty("--tilt-x", (ny * -5).toFixed(2) + "deg");
      stage.style.setProperty("--tilt-y", (nx * 7).toFixed(2) + "deg");
      stage.style.setProperty("--pointer-x", ((nx + 0.5) * 100).toFixed(1) + "%");
      stage.style.setProperty("--pointer-y", ((ny + 0.5) * 100).toFixed(1) + "%");
    });
    stage.addEventListener("pointerleave", function () {
      stage.style.setProperty("--tilt-x", "0deg");
      stage.style.setProperty("--tilt-y", "0deg");
    });
  }

  function edgePath(from, to, preset, index) {
    var dx = to.x - from.x;
    var dy = to.y - from.y;
    var lift = normalizePreset(preset) === "graph_3d" || normalizePreset(preset) === "graph_2_5d" ? 42 : 0;
    var bend = Math.min(120, Math.max(24, Math.sqrt(dx * dx + dy * dy) * 0.18));
    var mx = (from.x + to.x) / 2;
    var my = (from.y + to.y) / 2 - bend - lift + ((index % 3) - 1) * 12;
    return "M " + from.x.toFixed(1) + " " + from.y.toFixed(1) + " Q " + mx.toFixed(1) + " " + my.toFixed(1) + " " + to.x.toFixed(1) + " " + to.y.toFixed(1);
  }

  function nodeDepth(node, index, preset) {
    var metric = node && node.metrics ? Number(node.metrics.depth || node.metrics.risk || node.metrics.impact || node.metrics.score) : NaN;
    if (Number.isFinite(metric)) {
      return Math.max(0, Math.min(1, metric > 1 ? metric / 100 : metric));
    }
    if (normalizePreset(preset) === "city_map") {
      return (index % 6) / 5;
    }
    return 0.25 + ((index * 37) % 70) / 100;
  }

  function addFlowParticle(canvas, path, index, status) {
    var particle = svg("circle", { class: "visual-particle", r: 3.5, fill: nodeColor(status) });
    var motion = svg("animateMotion", {
      dur: (2.4 + (index % 5) * 0.28).toFixed(2) + "s",
      repeatCount: "indefinite",
      path: path,
      begin: ((index % 7) * 0.18).toFixed(2) + "s"
    });
    particle.appendChild(motion);
    canvas.appendChild(particle);
  }

  function safeClass(value) {
    return String(value || "default").toLowerCase().replace(/[^a-z0-9_-]+/g, "-");
  }

  function hexColor(status) {
    return parseInt(nodeColor(status).slice(1), 16);
  }

  function effectSpec(manifest) {
    return manifest && manifest.effects ? manifest.effects : {};
  }

  function visualDesign(manifest) {
    var design = manifest && manifest.visual_design ? manifest.visual_design : {};
    return {
      initialView: String(design.initial_view || "overview"),
      maxInitialNodes: positiveInt(design.max_initial_nodes, 60),
      maxInitialEdges: positiveInt(design.max_initial_edges, 120),
      defaultCollapseDepth: Math.max(0, positiveInt(design.default_collapse_depth, 0)),
      groupBy: Array.isArray(design.group_by) && design.group_by.length ? design.group_by : ["group", "module", "package"],
      supports: Array.isArray(design.supports) ? design.supports : [],
      agentGuidance: Array.isArray(design.agent_guidance) ? design.agent_guidance : []
    };
  }

  function positiveInt(value, fallback) {
    var n = Number(value);
    return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
  }

  function numberValue(value, fallback) {
    var n = Number(value);
    return Number.isFinite(n) ? n : fallback;
  }

  function importanceValue(item, fallback) {
    if (!item) {
      return fallback || 0;
    }
    var direct = Number(item.importance);
    if (Number.isFinite(direct)) {
      return direct > 1 ? direct / 100 : direct;
    }
    var metrics = item.metrics || {};
    var metric = Number(metrics.importance || metrics.impact || metrics.risk || metrics.score || metrics.weight);
    if (Number.isFinite(metric)) {
      return metric > 1 ? metric / 100 : metric;
    }
    return fallback || 0;
  }

  function itemLabel(item) {
    return String((item && (item.displayName || item.display_name || item.label || item.name || item.title || item.summary || item.text || item.id)) || "");
  }

  function itemMatchesQuery(item, query) {
    if (!query) {
      return true;
    }
    var text = [
      itemLabel(item),
      item && item.kind,
      item && item.status,
      item && item.id,
      item && item.summary
    ].filter(Boolean).join(" ").toLowerCase();
    return text.indexOf(query) >= 0;
  }

  function stringArray(value) {
    if (!Array.isArray(value)) {
      return [];
    }
    return value.map(function (item) {
      return String(item || "").trim();
    }).filter(Boolean);
  }

  function idSet(items) {
    var out = {};
    stringArray(items).forEach(function (id) {
      out[id] = true;
    });
    return out;
  }

  function readVisualHints(data) {
    var visual = data && data.visual && typeof data.visual === "object" ? data.visual : {};
    var view = data && data.view && typeof data.view === "object" ? data.view : (data && data.initial_view && typeof data.initial_view === "object" ? data.initial_view : {});
    var renderHints = data && data.renderHints && typeof data.renderHints === "object" ? data.renderHints : {};
    return {
      goal: String(visual.goal || ""),
      audience: String(visual.audience || ""),
      initialFocusIDs: stringArray(visual.initialFocusIds || visual.initial_focus_ids),
      hiddenDetailIDs: stringArray(visual.hiddenDetailIds || visual.hidden_detail_ids),
      emphasis: stringArray(visual.emphasis),
      narrativeSteps: Array.isArray(visual.narrativeSteps) ? visual.narrativeSteps : (Array.isArray(visual.narrative_steps) ? visual.narrative_steps : []),
      annotations: Array.isArray(visual.annotations) ? visual.annotations : [],
      view: view,
      renderHints: renderHints,
      labelMode: String(view.labelMode || view.label_mode || renderHints.labelMode || renderHints.label_mode || "overview"),
      showLegend: renderHints.showLegend !== false,
      showAnnotations: renderHints.showAnnotations !== false
    };
  }

  function normalizeLabelPriorityValue(value) {
    if (typeof value === "number" && Number.isFinite(value)) {
      var n = value > 1 ? value / 100 : value;
      if (n >= 0.85) return "always";
      if (n >= 0.65) return "important";
      if (n >= 0.35) return "normal";
      if (n > 0) return "hover";
      return "hidden";
    }
    var text = String(value || "").toLowerCase().trim();
    if (text === "always" || text === "important" || text === "normal" || text === "hover" || text === "hidden") {
      return text;
    }
    return "";
  }

  function normalizeVisibilityValue(value) {
    var text = String(value || "").toLowerCase().trim();
    if (text === "overview" || text === "normal" || text === "detail" || text === "hidden") return text;
    if (text === "visible") return "overview";
    if (text === "collapsed") return "detail";
    return "";
  }

  function normalizeVisualQualityFields(item) {
    item = item || {};
    var presentation = item.presentation && typeof item.presentation === "object" ? item.presentation : {};
    return {
      id: String(item.id || ""),
      label: itemLabel(item),
      displayName: String(item.displayName || item.display_name || item.label || item.name || item.id || ""),
      labelPriority: normalizeLabelPriorityValue(item.labelPriority !== undefined ? item.labelPriority : item.label_priority),
      visibility: normalizeVisibilityValue(item.visibility),
      importance: importanceValue(item, 0),
      laneIndex: item.laneIndex !== undefined ? item.laneIndex : (item.lane_index !== undefined ? item.lane_index : presentation.laneIndex),
      depth: item.depth !== undefined ? item.depth : presentation.depth,
      color: item.color || presentation.color || "",
      summary: String(item.summary || ""),
      details: String(item.details || "")
    };
  }

  function presentationOf(item) {
    item = item || {};
    return item.presentation && typeof item.presentation === "object" ? item.presentation : {};
  }

  function markRegistryFromManifest(manifest) {
    var assets = manifest && manifest.assets && typeof manifest.assets === "object" ? manifest.assets : {};
    var registry = assets.mark_registry && typeof assets.mark_registry === "object" ? assets.mark_registry : {};
    return {
      defaults: registry.defaults || { unknown: { shape: "sphere", mesh: "sphere", color: "#63a9ff", iconFallback: "generic.service" } },
      kinds: registry.kinds || {},
      providers: registry.providers || {},
      platforms: registry.platforms || {},
      edgeKinds: registry.edge_kinds || {},
      palettes: registry.palettes || {}
    };
  }

  function assetRegistryFromManifest(manifest) {
    var assets = manifest && manifest.assets && typeof manifest.assets === "object" ? manifest.assets : {};
    var registry = assets.asset_registry && typeof assets.asset_registry === "object" ? assets.asset_registry : {};
    return { icons: registry.icons || {}, models: registry.models || {}, attributions: registry.attributions || [] };
  }

  function createMarkContext(manifest, data) {
    return {
      manifest: manifest || {},
      data: data || {},
      renderHints: data && data.renderHints && typeof data.renderHints === "object" ? data.renderHints : {},
      markRegistry: markRegistryFromManifest(manifest),
      assetRegistry: assetRegistryFromManifest(manifest)
    };
  }

  function normalizeMarkKey(value) {
    return String(value || "").trim().toLowerCase().replace(/-/g, "_");
  }

  function mergeMarkSpec(base, next) {
    base = Object.assign({}, base || {});
    next = next || {};
    ["shape", "mesh", "icon", "iconFallback", "model", "modelFallback", "color"].forEach(function (key) {
      if (!base[key] && next[key]) {
        base[key] = next[key];
      }
    });
    return base;
  }

  function kindAlias(kind) {
    var aliases = {
      db: "database",
      rds: "database",
      dynamodb: "database",
      bucket: "storage",
      s3: "storage",
      event: "event_stream",
      event_bus: "event_stream",
      stream: "event_stream",
      broker: "event_stream",
      lambda: "service",
      controller: "api",
      endpoint: "api",
      deployment: "service",
      build: "job",
      runner: "job",
      external_provider: "external",
      client: "user",
      gate: "decision",
      branch: "decision",
      error: "risk",
      class: "class",
      module: "module",
      component: "component"
    };
    return aliases[kind] || "";
  }

  function providerServiceKey(item) {
    var provider = normalizeMarkKey(item && item.provider);
    var service = normalizeMarkKey(item && item.service);
    if (provider && service) {
      return provider + "." + service;
    }
    return provider;
  }

  function resolveMarkSpec(item, context) {
    var source = item && item.payload && typeof item.payload === "object" ? item.payload : item;
    source = source || {};
    var presentation = presentationOf(source);
    var registry = context && context.markRegistry ? context.markRegistry : markRegistryFromManifest({});
    var spec = {};
    var explicitShape = presentation.mesh || presentation.shape;
    if (explicitShape) {
      spec.shape = presentation.shape || presentation.mesh;
      spec.mesh = presentation.mesh || presentation.shape;
    }
    if (presentation.icon) {
      spec.icon = presentation.icon;
    }
    if (presentation.model) {
      spec.model = presentation.model;
    }
    var providerKey = providerServiceKey(source);
    if (providerKey && registry.providers[providerKey]) {
      spec = mergeMarkSpec(spec, registry.providers[providerKey]);
    }
    var platform = normalizeMarkKey(source.platform);
    if (platform && registry.platforms[platform]) {
      spec = mergeMarkSpec(spec, registry.platforms[platform]);
    }
    var provider = normalizeMarkKey(source.provider);
    if (provider && registry.platforms[provider]) {
      spec = mergeMarkSpec(spec, registry.platforms[provider]);
    }
    var kind = normalizeMarkKey(source.kind || source.type || source.stereotype || item && item.kind);
    if (kind && registry.kinds[kind]) {
      spec = mergeMarkSpec(spec, registry.kinds[kind]);
    } else if (kindAlias(kind) && registry.kinds[kindAlias(kind)]) {
      spec = mergeMarkSpec(spec, registry.kinds[kindAlias(kind)]);
    }
    if (source.__group && !explicitShape) {
      spec = mergeMarkSpec(spec, { shape: "group_hull", mesh: "box", iconFallback: "generic.service", color: "#63a9ff" });
    }
    if (!spec.shape) {
      var unknown = registry.defaults && registry.defaults.unknown ? registry.defaults.unknown : {};
      spec = mergeMarkSpec(spec, unknown);
    }
    spec.shape = spec.shape || "sphere";
    spec.mesh = spec.mesh || spec.shape || "sphere";
    spec.icon = presentation.icon || spec.icon || spec.iconFallback || "";
    spec.model = presentation.model || spec.model || spec.modelFallback || "";
    spec.color = normalizeColorString(presentation.color || source.color || spec.color || resolveColorSpec(source, context).color);
    spec.depth = presentation.depth !== undefined ? presentation.depth : source.depth;
    spec.lane = presentation.lane || source.lane || source.group || source.module || "";
    return spec;
  }

  function normalizeColorString(value) {
    var text = String(value || "").trim();
    if (!text) {
      return "#63a9ff";
    }
    if (text.charAt(0) !== "#") {
      return colorStringFromHex(phaseColor(text, 0));
    }
    return text;
  }

  function resolveColorSpec(item, context) {
    item = item || {};
    var presentation = presentationOf(item);
    if (presentation.color || item.color) {
      return { color: normalizeColorString(presentation.color || item.color), source: "presentation" };
    }
    var registry = context && context.markRegistry ? context.markRegistry : markRegistryFromManifest({});
    var providerKey = providerServiceKey(item);
    if (providerKey && registry.providers[providerKey] && registry.providers[providerKey].color) {
      return { color: registry.providers[providerKey].color, source: "provider" };
    }
    var kind = normalizeMarkKey(item.kind || item.type || item.stereotype);
    if (kind && registry.kinds[kind] && registry.kinds[kind].color) {
      return { color: registry.kinds[kind].color, source: "kind" };
    }
    if (kindAlias(kind) && registry.kinds[kindAlias(kind)] && registry.kinds[kindAlias(kind)].color) {
      return { color: registry.kinds[kindAlias(kind)].color, source: "kind" };
    }
    return { color: nodeColor(item.status), source: item.status ? "status" : "fallback" };
  }

  function createMarkGeometry(THREE, spec, item) {
    var mesh = normalizeMarkKey(spec && spec.mesh);
    if (mesh === "box" || mesh === "service_box") {
      return new THREE.BoxGeometry(0.52, item && item.__group ? 0.44 : 0.34, 0.34);
    }
    if (mesh === "card") {
      return new THREE.BoxGeometry(0.66, 0.38, 0.09);
    }
    if (mesh === "hex_prism") {
      return new THREE.CylinderGeometry(0.29, 0.29, 0.35, 6);
    }
    if (mesh === "cylinder") {
      return new THREE.CylinderGeometry(0.24, 0.24, 0.42, 28);
    }
    if (mesh === "capsule") {
      if (THREE.CapsuleGeometry) {
        return new THREE.CapsuleGeometry(0.19, 0.32, 8, 18);
      }
      return new THREE.CylinderGeometry(0.19, 0.19, 0.52, 20);
    }
    if (mesh === "cloud") {
      return new THREE.DodecahedronGeometry(0.3, 1);
    }
    if (mesh === "octahedron" || mesh === "diamond") {
      return new THREE.OctahedronGeometry(0.32, 0);
    }
    if (mesh === "cone" || mesh === "warning_prism") {
      return new THREE.ConeGeometry(0.28, 0.5, 5);
    }
    return new THREE.IcosahedronGeometry(0.22, 2);
  }

  function createMarkMesh(spec, item, THREE, effects) {
    var geometry = createMarkGeometry(THREE, spec, item);
    var material = createThreeMaterial(THREE, item, effects || {}, spec);
    var mesh = new THREE.Mesh(geometry, material);
    if (normalizeMarkKey(spec.mesh) === "cloud") {
      mesh.scale.set(1.15, 0.72, 0.88);
    } else if (normalizeMarkKey(spec.mesh) === "card") {
      mesh.scale.set(1.08, 1, 1);
    }
    return mesh;
  }

  function iconPathFor(spec, context) {
    if (!spec || !spec.icon) {
      return "";
    }
    var registry = context && context.assetRegistry ? context.assetRegistry : assetRegistryFromManifest({});
    var icon = registry.icons && registry.icons[spec.icon];
    return icon && icon.path ? String(icon.path) : "";
  }

  function modelPathFor(spec, context) {
    if (!spec || !spec.model) {
      return "";
    }
    var registry = context && context.assetRegistry ? context.assetRegistry : assetRegistryFromManifest({});
    var model = registry.models && registry.models[spec.model];
    return model && model.path ? String(model.path) : "";
  }

  function badgeSettings(context) {
    var hints = context && context.renderHints && typeof context.renderHints === "object" ? context.renderHints : {};
    var mode = normalizeMarkKey(hints.badgeMode || hints.badge_mode || "icon_and_model");
    if (["icon_and_model", "icon", "model", "none"].indexOf(mode) < 0) mode = "icon_and_model";
    var size = normalizeMarkKey(hints.badgeSize || hints.badge_size || "medium");
    if (["small", "medium", "large"].indexOf(size) < 0) size = "medium";
    var placement = normalizeMarkKey(hints.badgePlacement || hints.badge_placement || "front");
    if (["front", "top", "side"].indexOf(placement) < 0) placement = "front";
    return {
      mode: mode,
      size: size,
      placement: placement,
      labelIcon: hints.labelIcon !== false && hints.label_icon !== false
    };
  }

  function badgeScale(settings) {
    if (!settings) return 1;
    if (settings.size === "small") return 0.78;
    if (settings.size === "large") return 1.28;
    return 1;
  }

  function badgeInitials(spec, item) {
    var text = String(spec && spec.model || spec && spec.icon || itemLabel(item) || item && item.kind || "APP");
    text = text.replace(/\.logo3d$/i, "").replace(/^generic\./i, "").replace(/^simple-icons\./i, "");
    text = text.split(/[._:/-]+/).filter(Boolean).slice(-1)[0] || text;
    text = text.replace(/[^A-Za-z0-9]+/g, "").toUpperCase();
    if (text.length > 7) text = text.slice(0, 7);
    return text || "APP";
  }

  function createBadgeTexture(THREE, text, color) {
    if (!THREE.CanvasTexture || typeof document === "undefined") return null;
    var canvas = document.createElement("canvas");
    canvas.width = 384;
    canvas.height = 128;
    var ctx = canvas.getContext("2d");
    if (!ctx) return null;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = "rgba(255,255,255,0.96)";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.strokeStyle = color || "#334155";
    ctx.lineWidth = 12;
    ctx.strokeRect(6, 6, canvas.width - 12, canvas.height - 12);
    ctx.fillStyle = color || "#334155";
    ctx.font = "800 58px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(text, canvas.width / 2, canvas.height / 2 + 2);
    var texture = new THREE.CanvasTexture(canvas);
    if (THREE.SRGBColorSpace) {
      texture.colorSpace = THREE.SRGBColorSpace;
    }
    texture.needsUpdate = true;
    return texture;
  }

  function badgePosition(settings, size, kind) {
    var top = kind === "icon" ? 0.12 : 0;
    if (settings && settings.placement === "top") {
      return { x: 0, y: size.h * (kind === "icon" ? 0.68 : 0.58), z: kind === "icon" ? size.d * 0.08 : size.d * 0.04 };
    }
    if (settings && settings.placement === "side") {
      return { x: size.w * 0.34, y: size.h * (kind === "icon" ? 0.34 : 0.3), z: size.d * (kind === "icon" ? 0.1 : 0.04) };
    }
    return { x: 0, y: size.h * (kind === "icon" ? 0.48 + top : 0.34), z: size.d * (kind === "icon" ? 0.43 : 0.34) };
  }

  function createInlineIcon(spec, context, className) {
    var path = iconPathFor(spec, context);
    if (!path) {
      return null;
    }
    var icon = el("img", className || "visual-inline-icon");
    icon.src = path;
    icon.alt = "";
    icon.setAttribute("aria-hidden", "true");
    return icon;
  }

  function appendSvgIcon(parent, spec, context, x, y, size) {
    var path = iconPathFor(spec, context);
    if (!path) {
      return null;
    }
    var icon = svg("image", { class: "visual-svg-icon", x: x, y: y, width: size, height: size, href: path });
    parent.appendChild(icon);
    return icon;
  }

  function createIconBillboard(spec, item, THREE, context, size) {
    var settings = badgeSettings(context);
    if (settings.mode === "none" || settings.mode === "model") {
      return null;
    }
    var path = iconPathFor(spec, context);
    if (!path || !THREE.TextureLoader) {
      return null;
    }
    var texture = new THREE.TextureLoader().load(path);
    if (THREE.SRGBColorSpace) {
      texture.colorSpace = THREE.SRGBColorSpace;
    }
    var material = new THREE.MeshBasicMaterial({
      map: texture,
      transparent: true,
      opacity: 0.98,
      depthTest: false,
      depthWrite: false
    });
    var group = new THREE.Group();
    var factor = badgeScale(settings);
    var iconSize = Math.max(0.26, Math.min(0.46, (size ? Math.min(size.w, size.h) : 0.7) * 0.34 * factor));
    var backing = new THREE.Mesh(new THREE.PlaneGeometry(iconSize * 1.22, iconSize * 1.22), new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 0.9,
      depthTest: false,
      depthWrite: false
    }));
    backing.position.set(0, 0, -0.004);
    backing.renderOrder = 7;
    group.add(backing);
    var plane = new THREE.Mesh(new THREE.PlaneGeometry(iconSize, iconSize), material);
    plane.position.set(0, 0, 0);
    plane.renderOrder = 6;
    plane.userData = { icon: spec.icon, label: itemLabel(item), payload: item };
    group.add(plane);
    var pos = badgePosition(settings, size || { w: 0.9, h: 0.7, d: 0.7 }, "icon");
    group.position.set(pos.x, pos.y, pos.z);
    group.renderOrder = 8;
    group.userData = { isIconBillboard: true, icon: spec.icon, label: itemLabel(item), payload: item };
    return group;
  }

  function createModelBadge(THREE, spec, item, size, context) {
    var settings = badgeSettings(context);
    if (settings.mode === "none" || settings.mode === "icon") {
      return null;
    }
    var path = modelPathFor(spec, context);
    if (!path) {
      return null;
    }
    var group = new THREE.Group();
    var factor = badgeScale(settings);
    var color = colorValue(spec.color, 0x64748b);
    var baseMaterial = new THREE.MeshStandardMaterial({
      color: color,
      roughness: 0.44,
      metalness: 0.18,
      emissive: color,
      emissiveIntensity: 0.04
    });
    var plateW = size.w * 0.56 * factor;
    var plateH = Math.max(size.h * 0.1, 0.07) * factor;
    var plateD = size.d * 0.18 * factor;
    var plate = new THREE.Mesh(new THREE.BoxGeometry(plateW, plateH, plateD), baseMaterial);
    var pos = badgePosition(settings, size, "model");
    plate.position.set(0, 0, 0);
    plate.userData = { isGeneratedModelBadge: true, model: spec.model, modelPath: path, payload: item };
    group.add(plate);
    var shine = new THREE.Mesh(new THREE.BoxGeometry(plateW * 0.7, Math.max(plateH * 0.22, 0.012), plateD * 0.18), new THREE.MeshBasicMaterial({
      color: 0xffffff,
      transparent: true,
      opacity: 0.54
    }));
    shine.position.set(0, plateH * 0.12, plateD * 0.58);
    shine.userData = plate.userData;
    group.add(shine);
    var texture = createBadgeTexture(THREE, badgeInitials(spec, item), spec.color || "#334155");
    if (texture) {
      var textPlane = new THREE.Mesh(new THREE.PlaneGeometry(plateW * 0.88, plateH * 1.8), new THREE.MeshBasicMaterial({
        map: texture,
        transparent: true,
        depthTest: false,
        depthWrite: false
      }));
      textPlane.position.set(0, plateH * 0.08, plateD * 0.68);
      textPlane.renderOrder = 9;
      textPlane.userData = Object.assign({ isGeneratedModelBadgeLabel: true }, plate.userData);
      group.add(textPlane);
    }
    group.position.set(pos.x, pos.y, pos.z);
    group.userData = { isGeneratedModelBadge: true, model: spec.model, modelPath: path, payload: item };
    return group;
  }

  function createLabelEngine(options) {
    options = options || {};
    var mode = String(options.mode || "overview").toLowerCase();
    var focusIDs = options.focusIDs || {};
    return {
      shouldShow: function (item, fallback) {
        var q = normalizeVisualQualityFields(item);
        if (focusIDs[q.id]) return true;
        if (q.visibility === "hidden") return false;
        if (q.labelPriority === "hidden") return false;
        if (mode === "minimal") return q.labelPriority === "always" || q.importance >= 0.85 || !!fallback;
        if (mode === "overview") return q.labelPriority === "always" || q.labelPriority === "important" || q.importance >= 0.65 || !!fallback;
        if (mode === "normal") return q.labelPriority !== "hover" || q.importance >= 0.55 || !!fallback;
        if (mode === "detail") return q.visibility !== "hidden";
        if (mode === "focus") return focusIDs[q.id] || !!fallback;
        return !!fallback;
      },
      text: function (item, maxLength) {
        var text = normalizeVisualQualityFields(item).label;
        maxLength = maxLength || 42;
        return text.length > maxLength ? text.slice(0, Math.max(8, maxLength - 1)) + "…" : text;
      },
      payload: function (item) {
        var q = normalizeVisualQualityFields(item);
        return { id: q.id, label: q.label, summary: q.summary, details: q.details, importance: q.importance, visibility: q.visibility, labelPriority: q.labelPriority, payload: item };
      }
    };
  }

  function createLegendOverlay(container, title, items, onToggle) {
    if (!container || !items || !items.length) {
      return null;
    }
    var legend = el("div", "visual-legend-overlay");
    legend.appendChild(el("div", "visual-legend-title", title || "Legend"));
    items.forEach(function (item) {
      var id = String(item.id || item.value || item.label || "");
      if (!id) return;
      var button = document.createElement("button");
      button.type = "button";
      button.className = "visual-legend-item";
      var swatch = el("span", "visual-legend-swatch");
      swatch.style.backgroundColor = item.color || "rgba(99, 169, 255, 0.85)";
      button.appendChild(swatch);
      button.appendChild(el("span", "", item.label || id));
      button.addEventListener("click", function () {
        button.classList.toggle("visual-legend-active");
        if (onToggle) onToggle(id, button.classList.contains("visual-legend-active"));
      });
      legend.appendChild(button);
    });
    container.appendChild(legend);
    return legend;
  }

  function explicitColorByFromVisual(visual) {
    var view = visual && visual.view ? visual.view : {};
    var hints = visual && visual.renderHints ? visual.renderHints : {};
    return normalizeMarkKey(view.colorBy || view.color_by || hints.colorBy || hints.color_by || "");
  }

  function colorByFromVisual(visual) {
    return explicitColorByFromVisual(visual) || "kind";
  }

  function explicitlyRequestsLegend(visual) {
    var hints = visual && visual.renderHints ? visual.renderHints : {};
    return hints.showLegend === true || hints.show_legend === true;
  }

  function hasDeclaredColorBy(visual) {
    var view = visual && visual.view ? visual.view : {};
    var hints = visual && visual.renderHints ? visual.renderHints : {};
    return !!(view.colorBy || view.color_by || hints.colorBy || hints.color_by);
  }

  function shouldShowSemanticLegend(visual) {
    var hints = visual && visual.renderHints ? visual.renderHints : {};
    if (hints.showLegend === false || hints.show_legend === false) {
      return false;
    }
    return hasDeclaredColorBy(visual) || explicitlyRequestsLegend(visual);
  }

  function legendValueForItem(item, colorBy) {
    item = item || {};
    if (colorBy === "provider") {
      var key = providerServiceKey(item);
      return key || normalizeMarkKey(item.platform);
    }
    if (colorBy === "group") {
      return normalizeMarkKey(item.group || item.group_id || item.parent_id || item.module || item.package || item.lane);
    }
    if (colorBy === "service") {
      return normalizeMarkKey(item.service);
    }
    if (colorBy === "status") {
      return normalizeMarkKey(item.status);
    }
    if (colorBy === "phase") {
      return normalizeMarkKey(item.phase);
    }
    return normalizeMarkKey(item[colorBy] || item.kind || item.type);
  }

  function buildLegendItems(data, state, context) {
    var colorBy = colorByFromVisual(state.visual || {});
    var counts = {};
    var colors = {};
    function add(item, isEdge) {
      var value = legendValueForItem(item, colorBy);
      if (!value) {
        return;
      }
      counts[value] = (counts[value] || 0) + 1;
      if (!colors[value]) {
        if (isEdge && (colorBy === "relation" || colorBy === "kind" || colorBy === "type")) {
          colors[value] = resolveEdgeSpec(item, context).color;
        } else if (colorBy === "provider" || colorBy === "service" || colorBy === "kind") {
          colors[value] = resolveMarkSpec(item, context).color;
        } else {
          colors[value] = resolveColorSpec(item, context).color;
        }
      }
    }
    (state.rawNodes || []).forEach(function (item) { add(item, false); });
    (state.rawEdges || []).forEach(function (item) { add(item, true); });
    var items = Object.keys(counts).sort(function (a, b) {
      if (counts[a] === counts[b]) return a.localeCompare(b);
      return counts[b] - counts[a];
    }).slice(0, 14).map(function (id) {
      return { id: id, label: id, count: counts[id], color: colors[id] || colorStringFromHex(phaseColor(id, 0)) };
    });
    return { title: colorBy === "provider" ? "Providers" : colorBy.charAt(0).toUpperCase() + colorBy.slice(1), items: items };
  }

  function colorSpecForPolicy(item, context, colorBy, markSpec) {
    var colorSpec = resolveColorSpec(item, context);
    if (colorBy === "status") {
      return { color: nodeColor(item && item.status), source: "status" };
    }
    if (colorBy === "provider" || colorBy === "service" || colorBy === "platform" || colorBy === "kind") {
      return { color: markSpec && markSpec.color || colorSpec.color, source: colorBy };
    }
    return colorSpec;
  }

  function createSelectionStore() {
    return { selectedID: "", highlightIDs: {}, dimIDs: {}, focusIDs: {} };
  }

  function applyFocusState(entries, selectedID, relatedIDs) {
    relatedIDs = relatedIDs || {};
    (entries || []).forEach(function (entry) {
      var id = entry && (entry.id || (entry.node && entry.node.id) || (entry.edge && entry.edge.id));
      var element = entry && (entry.element || entry.node || entry.mesh);
      if (!element || !element.classList) return;
      var active = id === selectedID || relatedIDs[id];
      element.classList.toggle("visual-focused", !!active);
      element.classList.toggle("visual-dimmed", !!selectedID && !active);
    });
  }

  function applyLabelMode(labels, engine) {
    (labels || []).forEach(function (entry) {
      if (!entry || !entry.element) return;
      entry.element.hidden = !engine.shouldShow(entry.node || entry.edge || entry.payload, false);
    });
  }

  function visualAnnotationsFor(hints, targetID) {
    if (!hints || !targetID) {
      return [];
    }
    return hints.annotations.filter(function (annotation) {
      return annotation && String(annotation.targetId || annotation.target_id || "") === String(targetID);
    }).sort(function (a, b) {
      return importanceValue(b, Number(b.priority || 0.5)) - importanceValue(a, Number(a.priority || 0.5));
    });
  }

  function visualAnnotationText(annotation) {
    if (!annotation) {
      return "";
    }
    return String(annotation.label || annotation.summary || annotation.id || "").trim();
  }

  function groupIDForNode(node, design) {
    if (!node) {
      return "";
    }
    var names = ["parent_id", "group_id", "group"].concat(design && design.groupBy ? design.groupBy : []);
    for (var i = 0; i < names.length; i += 1) {
      var name = names[i];
      if (node[name] !== undefined && node[name] !== null && String(node[name]).trim()) {
        return String(node[name]).trim();
      }
    }
    return "";
  }

  function buildGroupNode(id, children, explicit) {
    explicit = explicit || {};
    var status = explicit.status;
    if (!status) {
      status = children.some(function (node) { return String(node.status || "").toLowerCase() === "error" || String(node.status || "").toLowerCase() === "failed"; }) ? "warning" : "ok";
    }
    return {
      id: id,
      label: explicit.label || explicit.name || explicit.title || id,
      kind: explicit.kind || "group",
      status: status,
      metadata: explicit.metadata || {},
      metrics: explicit.metrics || {},
      importance: explicit.importance !== undefined ? explicit.importance : Math.max(0.35, Math.min(1, children.length / 18)),
      __group: true,
      child_count: children.length,
      children: children
    };
  }

  function buildGraphState(data, manifest) {
    var design = visualDesign(manifest);
    var visual = readVisualHints(data);
    var visualFocus = idSet(visual.initialFocusIDs);
    var rawNodes = Array.isArray(data && data.nodes) ? data.nodes.slice() : [];
    var rawEdges = Array.isArray(data && data.edges) ? data.edges.slice() : [];
    var explicitGroups = Array.isArray(data && data.groups) ? data.groups : [];
    var groups = {};
    var groupOrder = [];
    explicitGroups.forEach(function (group) {
      if (!group || !group.id) {
        return;
      }
      var id = String(group.id);
      groups[id] = buildGroupNode(id, [], group);
      groupOrder.push(id);
    });
    var parentByNode = {};
    rawNodes.forEach(function (node) {
      var groupID = groupIDForNode(node, design);
      if (!groupID) {
        return;
      }
      parentByNode[node.id] = groupID;
      if (!groups[groupID]) {
        groups[groupID] = buildGroupNode(groupID, [], { id: groupID, label: groupID });
        groupOrder.push(groupID);
      }
      groups[groupID].children.push(node);
    });
    groupOrder.forEach(function (id) {
      groups[id] = buildGroupNode(id, groups[id].children || [], groups[id]);
    });
    var collapsed = {};
    groupOrder.forEach(function (id) {
      var group = groups[id];
      collapsed[id] = group.collapsed !== undefined ? !!group.collapsed : design.defaultCollapseDepth > 0;
      if ((group.children || []).some(function (child) { return child && visualFocus[child.id]; })) {
        collapsed[id] = false;
      }
    });
    return {
      design: design,
      visual: visual,
      visualFocus: visualFocus,
      visualHidden: idSet(visual.hiddenDetailIDs),
      rawNodes: rawNodes,
      rawEdges: rawEdges,
      groups: groups,
      groupOrder: groupOrder,
      parentByNode: parentByNode,
      collapsed: collapsed
    };
  }

  function compareImportance(a, b) {
    var ai = importanceValue(a, 0);
    var bi = importanceValue(b, 0);
    if (ai === bi) {
      return itemLabel(a).localeCompare(itemLabel(b));
    }
    return bi - ai;
  }

  function visibleGraph(state, filters) {
    filters = filters || {};
    var query = String(filters.query || "").toLowerCase();
    var focusIDs = state.visualFocus || {};
    var hiddenIDs = state.visualHidden || {};
    var visibleIDs = {};
    var visibleNodes = [];
    var groupMatches = {};
    var hasGroups = state.groupOrder.length > 0;

    function passesNode(node) {
      if (!query && hiddenIDs[node.id] && !focusIDs[node.id]) {
        return false;
      }
      return (!filters.status || node.status === filters.status) && (!filters.kind || node.kind === filters.kind) && itemMatchesQuery(node, query);
    }

    if (hasGroups) {
      state.groupOrder.forEach(function (groupID) {
        var group = state.groups[groupID];
        var children = (group.children || []).filter(function (child) {
          return (!filters.status || child.status === filters.status) && (!filters.kind || child.kind === filters.kind) && itemMatchesQuery(child, query);
        });
        groupMatches[groupID] = children;
        if (query && children.length > 0) {
          state.collapsed[groupID] = false;
        }
        if (!query && state.collapsed[groupID]) {
          visibleNodes.push(group);
          visibleIDs[group.id] = true;
        } else if (children.length > 0 || itemMatchesQuery(group, query)) {
          if (state.collapsed[groupID]) {
            visibleNodes.push(group);
            visibleIDs[group.id] = true;
          } else {
            children.sort(compareImportance).forEach(function (child) {
              visibleNodes.push(child);
              visibleIDs[child.id] = true;
            });
          }
        }
      });
      state.rawNodes.forEach(function (node) {
        if (state.parentByNode[node.id]) {
          return;
        }
        if (passesNode(node)) {
          visibleNodes.push(node);
          visibleIDs[node.id] = true;
        }
      });
    } else {
      state.rawNodes.filter(passesNode).sort(compareImportance).forEach(function (node) {
        if (visibleNodes.length < state.design.maxInitialNodes || query || node.visible === true || importanceValue(node, 0) >= 0.65) {
          visibleNodes.push(node);
          visibleIDs[node.id] = true;
        }
      });
    }

    var maxNodes = query ? Math.max(state.design.maxInitialNodes, visibleNodes.length) : state.design.maxInitialNodes;
    if (visibleNodes.length > maxNodes) {
      visibleNodes = visibleNodes.slice().sort(function (a, b) {
        var af = focusIDs[a.id] ? 1 : 0;
        var bf = focusIDs[b.id] ? 1 : 0;
        if (af !== bf) {
          return bf - af;
        }
        return compareImportance(a, b);
      }).slice(0, maxNodes);
      visibleIDs = {};
      visibleNodes.forEach(function (node) {
        visibleIDs[node.id] = true;
      });
    }

    var edgeKey = {};
    var visibleEdges = [];
    state.rawEdges.forEach(function (edge) {
      if (filters.edgeKind && edge.kind !== filters.edgeKind) {
        return;
      }
      if (!query && edge.id && hiddenIDs[edge.id]) {
        return;
      }
      if (edge.visibility === "hidden") {
        return;
      }
      if (edge.visibility === "detail" && !query) {
        return;
      }
      var from = state.parentByNode[edge.from] && state.collapsed[state.parentByNode[edge.from]] ? state.parentByNode[edge.from] : edge.from;
      var to = state.parentByNode[edge.to] && state.collapsed[state.parentByNode[edge.to]] ? state.parentByNode[edge.to] : edge.to;
      if (from === to || !visibleIDs[from] || !visibleIDs[to]) {
        return;
      }
      var key = from + "->" + to + ":" + (edge.kind || "");
      if (edgeKey[key]) {
        edgeKey[key].weight = numberValue(edgeKey[key].weight, 1) + numberValue(edge.weight || edge.value, 1);
        edgeKey[key].aggregated = true;
        return;
      }
      var copy = Object.assign({}, edge, { from: from, to: to });
      if (from !== edge.from || to !== edge.to) {
        copy.aggregated = true;
        copy.label = copy.label || edge.kind || "grouped";
      }
      edgeKey[key] = copy;
      visibleEdges.push(copy);
    });
    if (!query && visibleEdges.length > state.design.maxInitialEdges) {
      visibleEdges = visibleEdges.sort(function (a, b) { return importanceValue(b, numberValue(b.weight || b.value, 0)) - importanceValue(a, numberValue(a.weight || a.value, 0)); }).slice(0, state.design.maxInitialEdges);
    }
    return { nodes: visibleNodes, edges: visibleEdges, ids: visibleIDs };
  }

  function collectThreeItems(data, manifest) {
    var out = [];
    var nodes = Array.isArray(data && data.nodes) ? data.nodes : [];
    var groups = Array.isArray(data && data.groups) ? data.groups : [];
    var events = Array.isArray(data && data.events) ? data.events : [];
    var claims = Array.isArray(data && data.claims) ? data.claims : [];
    var sources = Array.isArray(data && data.sources) ? data.sources : [];
    var items = Array.isArray(data && data.items) ? data.items : [];
    var design = visualDesign(manifest);
    var visual = readVisualHints(data);
    var hiddenIDs = idSet(visual.hiddenDetailIDs);
    var focusIDs = idSet(visual.initialFocusIDs);
    function notHidden(item) {
      return !item || !item.id || !hiddenIDs[item.id] || focusIDs[item.id];
    }
    nodes = nodes.filter(notHidden);
    events = events.filter(notHidden);
    claims = claims.filter(notHidden);
    sources = sources.filter(notHidden);
    items = items.filter(notHidden);
    if (groups.length && nodes.length > design.maxInitialNodes) {
      groups.slice().sort(compareImportance).forEach(function (item) {
        out.push({ type: "group", id: item.id, label: item.label || item.name || item.title || item.id, status: item.status, kind: item.kind || "group", payload: item });
      });
      nodes.slice().sort(compareImportance).slice(0, Math.max(0, design.maxInitialNodes - out.length)).forEach(function (item) {
        out.push({ type: "node", id: item.id, label: item.label || item.name || item.title || item.id, status: item.status, kind: item.kind, payload: item });
      });
      return out;
    }
    if (nodes.length > design.maxInitialNodes) {
      nodes = nodes.slice().sort(function (a, b) {
        var af = focusIDs[a.id] ? 1 : 0;
        var bf = focusIDs[b.id] ? 1 : 0;
        if (af !== bf) {
          return bf - af;
        }
        return compareImportance(a, b);
      }).slice(0, design.maxInitialNodes);
    }
    nodes.forEach(function (item) {
      out.push({ type: "node", id: item.id, label: item.label || item.name || item.title || item.id, status: item.status, kind: item.kind, payload: item });
    });
    if (!nodes.length) {
      events.forEach(function (item) {
        out.push({ type: "event", id: item.id, label: item.label || item.name || item.summary || item.id, status: item.status, kind: item.kind, payload: item });
      });
    }
    claims.forEach(function (item) {
      out.push({ type: "claim", id: item.id, label: item.label || item.name || item.text || item.id, status: item.status, kind: "claim", payload: item });
    });
    sources.forEach(function (item) {
      out.push({ type: "source", id: item.id, label: item.label || item.name || item.title || item.id, status: item.status, kind: item.kind || "source", payload: item });
    });
    items.forEach(function (item) {
      out.push({ type: "item", id: item.id, label: item.label || item.name || item.title || item.id, status: item.status, kind: item.kind, x: item.x, y: item.y, payload: item });
    });
    return out;
  }

  function threePosition(THREE, item, index, count, effects, preset) {
    var scene = safeClass(effects.scene);
    var motion = safeClass(effects.motion);
    var material = safeClass(effects.material);
    var angle = (Math.PI * 2 * index) / Math.max(1, count);
    var radius = 2.2 + (index % 4) * 0.34;
    var x = Math.cos(angle) * radius;
    var y = Math.sin(index * 1.17) * 0.7;
    var z = Math.sin(angle) * radius;
    if (typeof item.x === "number" && typeof item.y === "number") {
      x = (item.x - 0.5) * 5.4;
      z = (0.5 - item.y) * 4.4;
      y = 0.18 + (index % 5) * 0.16;
    } else if (motion.indexOf("flow") >= 0 || motion.indexOf("timeline") >= 0 || scene.indexOf("timeline") >= 0 || preset.indexOf("timeline") >= 0 || preset.indexOf("pipeline") >= 0) {
      x = -3.2 + index * (6.4 / Math.max(1, count - 1));
      y = Math.sin(index * 0.82) * 0.52;
      z = (index % 3 - 1) * 0.86;
    } else if (material.indexOf("height") >= 0 || material.indexOf("city") >= 0 || preset.indexOf("city") >= 0 || preset.indexOf("heat") >= 0) {
      var cols = Math.max(3, Math.ceil(Math.sqrt(count)));
      x = (index % cols - (cols - 1) / 2) * 0.9;
      z = (Math.floor(index / cols) - (Math.ceil(count / cols) - 1) / 2) * 0.9;
      y = 0.16 + (index % 7) * 0.18;
    } else if (item.type === "claim" || item.type === "source" || scene.indexOf("evidence") >= 0 || scene.indexOf("lineage") >= 0) {
      x = item.type === "source" ? -2.4 : 2.3;
      y = 1.5 - (index % 6) * 0.58;
      z = (index % 3 - 1) * 0.7;
    } else if (scene.indexOf("radar") >= 0 || scene.indexOf("orbit") >= 0) {
      radius = 1.1 + (index % 5) * 0.54;
      x = Math.cos(angle) * radius;
      y = Math.sin(index * 0.9) * 0.5;
      z = Math.sin(angle) * radius;
    }
    return new THREE.Vector3(x, y, z);
  }

  function createThreeMaterial(THREE, item, effects, markSpec) {
    var color = colorValue(markSpec && markSpec.color, hexColor(item.status));
    var material = safeClass(effects.material);
    var params = {
      color: color,
      emissive: color,
      emissiveIntensity: material.indexOf("emissive") >= 0 || material.indexOf("holographic") >= 0 ? 0.28 : 0.14,
      metalness: material.indexOf("glass") >= 0 || material.indexOf("holographic") >= 0 ? 0.35 : 0.16,
      roughness: material.indexOf("glass") >= 0 ? 0.18 : 0.48
    };
    if (material.indexOf("glass") >= 0 || material.indexOf("holographic") >= 0) {
      params.transparent = true;
      params.opacity = 0.78;
    }
    if ((material.indexOf("glass") >= 0 || material.indexOf("physical") >= 0) && THREE.MeshPhysicalMaterial) {
      params.clearcoat = 0.65;
      params.iridescence = material.indexOf("holographic") >= 0 ? 0.45 : 0.12;
      return new THREE.MeshPhysicalMaterial(params);
    }
    return new THREE.MeshStandardMaterial(params);
  }

  function createThreeScene(stage, manifest, data, preset, profile, inspector) {
    var effects = effectSpec(manifest);
    if (effects.engine !== "three.v1") {
      return null;
    }
    var THREE = window.THREE;
    if (!THREE || !THREE.WebGLRenderer) {
      stage.classList.add("visual-three-missing");
      return null;
    }
    try {
      var layer = el("div", "visual-three-layer visual-three-scene-" + safeClass(effects.scene));
      layer.setAttribute("aria-hidden", "true");
      stage.insertBefore(layer, stage.firstChild);
      var width = Math.max(720, stage.clientWidth || 900);
      var height = Math.max(520, stage.clientHeight || 620);
      var renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
      renderer.setClearColor(0x000000, 0);
      renderer.setPixelRatio(Math.min(2, window.devicePixelRatio || 1));
      renderer.setSize(width, height, false);
      if (THREE.SRGBColorSpace) {
        renderer.outputColorSpace = THREE.SRGBColorSpace;
      }
      layer.appendChild(renderer.domElement);

      var scene = new THREE.Scene();
      var camera = new THREE.PerspectiveCamera(42, width / height, 0.1, 120);
      var cameraMode = safeClass(effects.camera);
      if (cameraMode.indexOf("tunnel") >= 0 || cameraMode.indexOf("dolly") >= 0) {
        camera.position.set(0, 1.2, 7.2);
      } else if (cameraMode.indexOf("terrain") >= 0 || cameraMode.indexOf("isometric") >= 0) {
        camera.position.set(4.5, 5.2, 6.2);
      } else {
        camera.position.set(0.4, 2.8, 6.8);
      }
      camera.lookAt(0, 0, 0);
      scene.add(new THREE.AmbientLight(0xffffff, 0.72));
      var light = new THREE.DirectionalLight(0x8fdcff, 1.25);
      light.position.set(4, 6, 5);
      scene.add(light);

      var root = new THREE.Group();
      var particleRoot = new THREE.Group();
      scene.add(root);
      scene.add(particleRoot);
      var items = collectThreeItems(data, manifest);
      var markContext = createMarkContext(manifest, data);
      var objects = [];
      var positions = {};
      items.forEach(function (item, index) {
        var pos = threePosition(THREE, item, index, items.length, effects, preset);
        positions[item.id] = pos;
        var markSpec = resolveMarkSpec(item.payload || item, markContext);
        var mesh = createMarkMesh(markSpec, item.payload || item, THREE, effects);
        mesh.position.copy(pos);
        if (safeClass(markSpec.mesh).indexOf("box") >= 0) {
          var lift = item.payload && item.payload.metrics ? Number(item.payload.metrics.risk || item.payload.metrics.impact || item.payload.metrics.score || item.payload.metrics.value) : NaN;
          if (!Number.isFinite(lift)) {
            lift = index % 6;
          }
          mesh.scale.y = 0.75 + Math.min(2.8, Math.max(0, lift > 1 ? lift / 35 : lift * 0.35));
          mesh.position.y += mesh.scale.y * 0.08;
        }
        mesh.userData = { label: item.label, payload: item.payload || item };
        var icon = createIconBillboard(markSpec, item.payload || item, THREE, markContext);
        if (icon) {
          mesh.add(icon);
        }
        root.add(mesh);
        objects.push(mesh);
      });

      var edges = Array.isArray(data && data.edges) ? data.edges : [];
      edges.forEach(function (edge) {
        var from = positions[edge.from];
        var to = positions[edge.to];
        if (!from || !to) {
          return;
        }
        var lineGeo = new THREE.BufferGeometry();
        lineGeo.setFromPoints([from, to]);
        var line = new THREE.Line(lineGeo, new THREE.LineBasicMaterial({
          color: hexColor(edge.status),
          transparent: true,
          opacity: 0.62
        }));
        root.add(line);
      });

      var particleCount = Math.min(520, 96 + Math.max(items.length, edges.length) * 18);
      var particlePositions = new Float32Array(particleCount * 3);
      for (var i = 0; i < particleCount; i += 1) {
        var a = (Math.PI * 2 * i) / particleCount;
        var r = 2.2 + (i % 11) * 0.18;
        particlePositions[i * 3] = Math.cos(a) * r;
        particlePositions[i * 3 + 1] = Math.sin(i * 0.47) * 1.2;
        particlePositions[i * 3 + 2] = Math.sin(a) * r;
      }
      var particleGeometry = new THREE.BufferGeometry();
      particleGeometry.setAttribute("position", new THREE.Float32BufferAttribute(particlePositions, 3));
      var particleMaterial = new THREE.PointsMaterial({
        color: profile && profile.radar ? 0x35c2a1 : 0x63a9ff,
        size: safeClass(effects.particles).indexOf("dust") >= 0 ? 0.026 : 0.04,
        transparent: true,
        opacity: 0.5,
        blending: THREE.AdditiveBlending
      });
      particleRoot.add(new THREE.Points(particleGeometry, particleMaterial));

      var targetTiltX = 0;
      var targetTiltY = 0;
      var pointer = new THREE.Vector2();
      var raycaster = new THREE.Raycaster();
      stage.addEventListener("pointermove", function (event) {
        var rect = stage.getBoundingClientRect();
        targetTiltY = rect.width ? ((event.clientX - rect.left) / rect.width - 0.5) * 0.32 : 0;
        targetTiltX = rect.height ? ((event.clientY - rect.top) / rect.height - 0.5) * -0.22 : 0;
      });
      stage.addEventListener("click", function (event) {
        if (!objects.length || !inspector) {
          return;
        }
        var rect = renderer.domElement.getBoundingClientRect();
        pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
        raycaster.setFromCamera(pointer, camera);
        var hits = raycaster.intersectObjects(objects, false);
        if (hits.length && hits[0].object && hits[0].object.userData) {
          inspector.show(hits[0].object.userData.label, hits[0].object.userData.payload);
        }
      });

      function resize() {
        width = Math.max(720, stage.clientWidth || width);
        height = Math.max(520, stage.clientHeight || height);
        camera.aspect = width / height;
        camera.updateProjectionMatrix();
        renderer.setSize(width, height, false);
      }
      var observer = typeof ResizeObserver !== "undefined" ? new ResizeObserver(resize) : null;
      if (observer) {
        observer.observe(stage);
      }
      var start = Date.now();
      function animate() {
        if (!document.body.contains(stage)) {
          if (observer) {
            observer.disconnect();
          }
          renderer.dispose();
          return;
        }
        var t = (Date.now() - start) / 1000;
        root.rotation.x += (targetTiltX - root.rotation.x) * 0.045;
        root.rotation.y += (targetTiltY + t * 0.065 - root.rotation.y) * 0.045;
        particleRoot.rotation.y = t * (safeClass(effects.motion).indexOf("scan") >= 0 ? 0.22 : 0.12);
        particleRoot.rotation.x = Math.sin(t * 0.42) * 0.08;
        renderer.render(scene, camera);
        window.requestAnimationFrame(animate);
      }
      animate();
      stage.classList.add("visual-three-active");
      return { scene: scene, renderer: renderer, camera: camera };
    } catch (err) {
      stage.classList.add("visual-three-fallback");
      stage.setAttribute("data-three-error", err && err.message ? err.message : String(err));
      return null;
    }
  }

  function isPrimaryThreeGraph(manifest, preset, profile) {
    var effects = effectSpec(manifest);
    return effects.engine === "three.v1";
  }

  function graphNodeGroupKey(node) {
    if (!node) {
      return "";
    }
    return String(node.parent_id || node.group_id || node.group || node.module || node.package || node.kind || "").trim();
  }

  function layoutGraphNodes3D(THREE, nodes, preset) {
    var positions = {};
    var count = Math.max(1, nodes.length);
    var groupIndex = {};
    var groupCounts = {};
    var groups = [];
    nodes.forEach(function (node) {
      var key = graphNodeGroupKey(node) || "ungrouped";
      if (!groupIndex[key]) {
        groupIndex[key] = groups.length + 1;
        groups.push(key);
      }
      groupCounts[key] = 0;
    });
    nodes.forEach(function (node, index) {
      var key = graphNodeGroupKey(node) || "ungrouped";
      var g = groupIndex[key] - 1;
      var local = groupCounts[key] || 0;
      groupCounts[key] = local + 1;
      var angle = (Math.PI * 2 * index) / count;
      var x;
      var y;
      var z;
      if (preset === "timeline_tunnel" || preset === "swimlane_timeline" || preset === "replay_stage" || preset === "pipeline_flow" || preset === "flow_particles" || preset === "step_ladder") {
        x = -3.1 + index * (6.2 / Math.max(1, count - 1));
        y = node.__group ? 0.36 + (index % 2) * 0.2 : Math.sin(index * 0.74) * 0.42;
        z = ((index % 3) - 1) * 0.78;
      } else if (preset === "layered_stack" || preset === "state_machine" || preset === "dag" || preset === "permission_gate" || preset === "network_boundary_map" || preset === "service_map") {
        var cols = Math.max(2, Math.ceil(Math.sqrt(count)));
        x = (index % cols - (cols - 1) / 2) * 1.15;
        y = node.__group ? 0.5 : 0.08 + (g % 4) * 0.18;
        z = (Math.floor(index / cols) - (Math.ceil(count / cols) - 1) / 2) * 1.08;
      } else if (preset === "city_map" || preset === "heatmap") {
        var cityCols = Math.max(3, Math.ceil(Math.sqrt(count)));
        x = (index % cityCols - (cityCols - 1) / 2) * 0.92;
        y = 0.18 + importanceValue(node, 0.35) * 0.88;
        z = (Math.floor(index / cityCols) - (Math.ceil(count / cityCols) - 1) / 2) * 0.92;
      } else if (node.__group || preset === "orbit_system") {
        var radius = 1.85 + (index % 4) * 0.38;
        x = Math.cos(angle) * radius;
        y = Math.sin(index * 0.73) * 0.82;
        z = Math.sin(angle) * radius;
      } else if (groups.length > 1) {
        var groupAngle = (Math.PI * 2 * g) / groups.length;
        var localAngle = groupAngle + (local - 1) * 0.42;
        var clusterRadius = 2.2 + (g % 3) * 0.26;
        var localRadius = 0.32 + Math.sqrt(local + 1) * 0.18;
        x = Math.cos(groupAngle) * clusterRadius + Math.cos(localAngle) * localRadius;
        y = ((local % 7) - 3) * 0.2;
        z = Math.sin(groupAngle) * clusterRadius + Math.sin(localAngle) * localRadius;
      } else {
        var golden = Math.PI * (3 - Math.sqrt(5));
        y = 1 - (index / Math.max(1, count - 1)) * 2;
        var sphereRadius = Math.sqrt(Math.max(0, 1 - y * y));
        var theta = index * golden;
        x = Math.cos(theta) * sphereRadius * 2.8;
        z = Math.sin(theta) * sphereRadius * 2.8;
        y *= 1.28;
      }
      positions[node.id] = new THREE.Vector3(x, y, z);
    });
    return positions;
  }

  function disposeThreeObject(object) {
    if (!object) {
      return;
    }
    if (object.children) {
      object.children.slice().forEach(disposeThreeObject);
    }
    if (object.geometry && object.geometry.dispose) {
      object.geometry.dispose();
    }
    if (object.material) {
      if (Array.isArray(object.material)) {
        object.material.forEach(function (material) {
          if (material && material.dispose) {
            material.dispose();
          }
        });
      } else if (object.material.dispose) {
        object.material.dispose();
      }
    }
  }

  function addThreeGrid(THREE, root, size, divisions, y, color) {
    if (THREE.GridHelper) {
      var helper = new THREE.GridHelper(size, divisions, color || 0x2d4254, 0x172434);
      helper.position.y = y || 0;
      root.add(helper);
      return helper;
    }
    var group = new THREE.Group();
    var material = new THREE.LineBasicMaterial({
      color: color || 0x2d4254,
      transparent: true,
      opacity: 0.42
    });
    var half = size / 2;
    var step = size / Math.max(1, divisions);
    for (var i = 0; i <= divisions; i += 1) {
      var p = -half + i * step;
      var gx = new THREE.BufferGeometry();
      gx.setFromPoints([new THREE.Vector3(-half, 0, p), new THREE.Vector3(half, 0, p)]);
      group.add(new THREE.Line(gx, material));
      var gz = new THREE.BufferGeometry();
      gz.setFromPoints([new THREE.Vector3(p, 0, -half), new THREE.Vector3(p, 0, half)]);
      group.add(new THREE.Line(gz, material));
    }
    group.position.y = y || 0;
    root.add(group);
    return group;
  }

  function edgePresentation(edge) {
    return edge && edge.presentation && typeof edge.presentation === "object" ? edge.presentation : {};
  }

  function directedEdgeKind(kind) {
    kind = normalizeMarkKey(kind);
    return {
      call: true,
      calls: true,
      sync: true,
      async: true,
      write: true,
      writes: true,
      read: true,
      reads: true,
      emit: true,
      emits: true,
      publish: true,
      publishes: true,
      subscribe: true,
      subscribes: true,
      deploy: true,
      event: true,
      deploys: true,
      deploys_to: true,
      validate: true,
      validates: true,
      block: true,
      blocks: true,
      depend: true,
      depends: true,
      depends_on: true,
      dependency: true,
      send: true,
      sends: true,
      return: true,
      returns: true,
      message: true,
      flow: true,
      transition: true,
      observes: true,
      supports: true,
      refutes: true,
      mentions: true
    }[kind] === true;
  }

  function resolveEdgeSpec(edge, context) {
    edge = edge || {};
    var presentation = edgePresentation(edge);
    var registry = context && context.markRegistry ? context.markRegistry : markRegistryFromManifest({});
    var kind = normalizeMarkKey(edge.kind || edge.relation || edge.type);
    var base = registry.edgeKinds && registry.edgeKinds[kind] ? Object.assign({}, registry.edgeKinds[kind]) : {};
    var directed = edge.directed === true || directedEdgeKind(kind) || !!presentation.arrow;
    var arrow = normalizeMarkKey(presentation.arrow || base.arrow || (directed ? "forward" : "none"));
    if (arrow === "none") {
      directed = false;
    }
    var lineStyle = normalizeMarkKey(presentation.lineStyle || presentation.line_style || base.lineStyle || base.line_style || (kind === "async" || kind === "emits" || kind === "subscribes" || kind === "observes" ? "dashed" : "solid"));
    var color = normalizeColorString(presentation.color || edge.color || base.color || nodeColor(edge.status));
    return {
      directed: directed,
      arrow: arrow,
      lineStyle: lineStyle,
      curve: normalizeMarkKey(presentation.curve || edge.curve || base.curve || (lineStyle === "dashed" ? "arc" : "straight")),
      flow: presentation.flow !== undefined ? !!presentation.flow : !!base.flow || kind === "emits" || kind === "subscribes" || kind === "writes" || kind === "reads" || kind === "calls",
      color: color,
      opacity: edge.aggregated ? 0.86 : 0.68
    };
  }

  function edgeCurveFor(THREE, from, to, edge, edgeSpec, index) {
    var a = from.clone();
    var b = to.clone();
    var mid = a.clone().lerp(b, 0.5);
    if (edgeSpec.curve === "arc" || edgeSpec.curve === "high_arc" || edgeSpec.flow) {
      var direction = b.clone().sub(a);
      var lift = edgeSpec.curve === "high_arc" ? 0.55 : 0.28;
      mid.y += lift + importanceValue(edge, 0) * 0.22;
      mid.z += ((index || 0) % 3 - 1) * 0.13;
      if (direction.length() > 0.01) {
        var side = new THREE.Vector3(-direction.z, 0, direction.x).normalize().multiplyScalar(((index || 0) % 2 ? 1 : -1) * 0.08);
        mid.add(side);
      }
      return THREE.CatmullRomCurve3 ? new THREE.CatmullRomCurve3([a, mid, b]) : { getPoint: function (t) { return t < 0.5 ? a.clone().lerp(mid, t * 2) : mid.clone().lerp(b, (t - 0.5) * 2); } };
    }
    return { getPoint: function (t) { return a.clone().lerp(b, t); } };
  }

  function curvePoints(curve, segments) {
    if (curve.getPoints) {
      return curve.getPoints(segments || 16);
    }
    var out = [];
    for (var i = 0; i <= (segments || 16); i += 1) {
      out.push(curve.getPoint(i / (segments || 16)));
    }
    return out;
  }

  function createEdgeTube(THREE, curve, edgeSpec, radius) {
    var color = colorValue(edgeSpec.color, 0x63a9ff);
    var geometry = THREE.TubeGeometry ? new THREE.TubeGeometry(curve, 18, radius || 0.012, 8, false) : new THREE.BufferGeometry().setFromPoints(curvePoints(curve, 18));
    var blendMode = edgeSpec.lightBackground ? THREE.NormalBlending : THREE.AdditiveBlending;
    var material = THREE.TubeGeometry ? new THREE.MeshBasicMaterial({
      color: color,
      transparent: true,
      opacity: 0,
      blending: blendMode
    }) : new THREE.LineBasicMaterial({
      color: color,
      transparent: true,
      opacity: 0,
      blending: blendMode
    });
    material.depthTest = false;
    material.depthWrite = false;
    var object = THREE.TubeGeometry ? new THREE.Mesh(geometry, material) : new THREE.Line(geometry, material);
    object.renderOrder = 1;
    object.userData.isEdgeTube = true;
    object.userData.baseOpacity = edgeSpec.opacity;
    object.userData.targetOpacity = edgeSpec.opacity;
    return object;
  }

  function createArrowHead(THREE, curve, edgeSpec) {
    if (!edgeSpec.directed || edgeSpec.arrow === "none") {
      return null;
    }
    var t = edgeSpec.arrow === "reverse" ? 0.06 : 0.94;
    var tail = curve.getPoint(edgeSpec.arrow === "reverse" ? 0.14 : 0.86);
    var tip = curve.getPoint(t);
    var direction = tip.clone().sub(tail).normalize();
    if (direction.length() < 0.001) {
      direction.set(0, 1, 0);
    }
    if (edgeSpec.lightBackground && THREE.BufferGeometry) {
      var side = new THREE.Vector3(-direction.z, 0, direction.x).normalize();
      if (side.length() < 0.001) {
        side.set(1, 0, 0);
      }
      var back = tip.clone().sub(direction.clone().multiplyScalar(0.24));
      var yLift = new THREE.Vector3(0, 0.018, 0);
      var geometryFlat = new THREE.BufferGeometry().setFromPoints([
        tip.clone().add(yLift),
        back.clone().add(side.clone().multiplyScalar(0.105)).add(yLift),
        back.clone().add(side.clone().multiplyScalar(-0.105)).add(yLift)
      ]);
      if (geometryFlat.computeVertexNormals) {
        geometryFlat.computeVertexNormals();
      }
      var materialFlat = new THREE.MeshBasicMaterial({
        color: colorValue(edgeSpec.color, 0x111827),
        transparent: true,
        opacity: 0,
        blending: THREE.NormalBlending,
        side: THREE.DoubleSide || 2
      });
      materialFlat.depthTest = false;
      materialFlat.depthWrite = false;
      var flatArrow = new THREE.Mesh(geometryFlat, materialFlat);
      flatArrow.renderOrder = 4;
      flatArrow.userData.baseOpacity = Math.min(1, edgeSpec.opacity + 0.1);
      flatArrow.userData.targetOpacity = flatArrow.userData.baseOpacity;
      return flatArrow;
    }
    var geometry = THREE.ConeGeometry ? new THREE.ConeGeometry(0.075, 0.24, 18) : new THREE.IcosahedronGeometry(0.1, 1);
    var material = new THREE.MeshBasicMaterial({
      color: colorValue(edgeSpec.color, 0x63a9ff),
      transparent: true,
      opacity: 0,
      blending: edgeSpec.lightBackground ? THREE.NormalBlending : THREE.AdditiveBlending
    });
    material.depthTest = false;
    material.depthWrite = false;
    var cone = new THREE.Mesh(geometry, material);
    cone.position.copy(tip);
    cone.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), direction);
    cone.renderOrder = 3;
    cone.userData.baseOpacity = Math.min(1, edgeSpec.opacity + 0.1);
    cone.userData.targetOpacity = cone.userData.baseOpacity;
    return cone;
  }

  function createFlowParticles(THREE, curve, edgeSpec, count) {
    if (!edgeSpec.flow && edgeSpec.lineStyle !== "dashed") {
      return [];
    }
    var particles = [];
    var material = new THREE.MeshBasicMaterial({
      color: colorValue(edgeSpec.color, 0x63a9ff),
      transparent: true,
      opacity: 0,
      blending: edgeSpec.lightBackground ? THREE.NormalBlending : THREE.AdditiveBlending
    });
    material.depthTest = false;
    material.depthWrite = false;
    var geometry = new THREE.SphereGeometry(edgeSpec.lineStyle === "dashed" ? 0.032 : 0.04, 10, 8);
    for (var i = 0; i < Math.max(1, count || 1); i += 1) {
      var marker = new THREE.Mesh(geometry, material.clone());
      marker.position.copy(curve.getPoint(i / Math.max(1, count || 1)));
      marker.renderOrder = 2;
      marker.userData = {
        curve: curve,
        phase: i / Math.max(1, count || 1),
        speed: edgeSpec.flow ? 0.18 + i * 0.025 : 0.08,
        baseOpacity: edgeSpec.lineStyle === "dashed" ? 0.58 : 0.82,
        targetOpacity: edgeSpec.lineStyle === "dashed" ? 0.58 : 0.82,
        baseScale: 1,
        targetScale: 1
      };
      particles.push(marker);
    }
    return particles;
  }

  function createDirectedEdge(THREE, endpoints, edge, edgeSpec, index) {
    var curve = edgeCurveFor(THREE, endpoints.from.position, endpoints.to.position, edge, edgeSpec, index);
    var tube = createEdgeTube(THREE, curve, edgeSpec, edge.aggregated ? 0.02 : 0.012);
    var arrow = createArrowHead(THREE, curve, edgeSpec);
    var markers = createFlowParticles(THREE, curve, edgeSpec, edgeSpec.flow ? 2 : edgeSpec.lineStyle === "dashed" ? 3 : 1);
    return { line: tube, arrow: arrow, markers: markers, curve: curve, edgeSpec: edgeSpec };
  }

  function createThreeGraphScene(stage, manifest, state, preset, profile, inspector, onGraphChange) {
    var effects = effectSpec(manifest);
    if (effects.engine !== "three.v1") {
      return null;
    }
    var THREE = window.THREE;
    if (!THREE || !THREE.WebGLRenderer) {
      stage.classList.add("visual-three-missing");
      return null;
    }
    try {
      stage.classList.add("visual-three-primary");
      var layer = el("div", "visual-three-layer visual-three-primary-layer visual-three-scene-" + safeClass(effects.scene));
      layer.setAttribute("role", "application");
      layer.setAttribute("aria-label", "Interactive 3D graph. Drag to orbit, use the mouse wheel to zoom, click a node to inspect it.");
      stage.appendChild(layer);
      var width = Math.max(720, stage.clientWidth || 900);
      var height = Math.max(520, stage.clientHeight || 620);
      var renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
      renderer.setClearColor(0x000000, 0);
      renderer.setPixelRatio(Math.min(2, window.devicePixelRatio || 1));
      renderer.setSize(width, height, false);
      if (THREE.SRGBColorSpace) {
        renderer.outputColorSpace = THREE.SRGBColorSpace;
      }
      layer.appendChild(renderer.domElement);
      var labelLayer = el("div", "visual-three-label-layer");
      layer.appendChild(labelLayer);

      var scene = new THREE.Scene();
      var camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 140);
      var orbit = {
        theta: 0.24,
        phi: 1.16,
        radius: 7.4,
        target: new THREE.Vector3(0, 0, 0)
      };
      scene.add(new THREE.AmbientLight(0xffffff, 0.52));
      var keyLight = new THREE.DirectionalLight(0x9bd7ff, 1.45);
      keyLight.position.set(4.5, 5.8, 5.4);
      scene.add(keyLight);
      var fillLight = new THREE.DirectionalLight(0x38d6aa, 0.72);
      fillLight.position.set(-4.2, -1.4, -3.5);
      scene.add(fillLight);

      var root = new THREE.Group();
      var edgeRoot = new THREE.Group();
      var nodeRoot = new THREE.Group();
      root.add(edgeRoot);
      root.add(nodeRoot);
      scene.add(root);
      var particleRoot = new THREE.Group();
      scene.add(particleRoot);

      var objects = [];
      var nodeMap = {};
      var edgeItems = [];
      var labels = [];
      var edgeLabels = [];
      var annotationLabels = [];
      var selectedID = "";
      var hoverID = "";
      var currentModel = { nodes: [], edges: [] };
      var currentFilters = {};
      var markContext = createMarkContext(manifest, state.rawNodes && state.rawNodes.length ? { nodes: state.rawNodes, edges: state.rawEdges } : {});
      var labelEngine = createLabelEngine({ mode: state.visual.labelMode, focusIDs: state.visualFocus });
      var selectionStore = createSelectionStore();
      var pointer = new THREE.Vector2();
      var raycaster = new THREE.Raycaster();
      var dragging = false;
      var moved = false;
      var dragMode = "idle";
      var draggedMesh = null;
      var pendingMesh = null;
      var lastExpandOrigin = null;
      var dragPlane = { normal: new THREE.Vector3(), point: new THREE.Vector3() };
      var dragPointerPoint = new THREE.Vector3();
      var dragOffset = new THREE.Vector3();
      var autoRotateGraph = true;
      var lastX = 0;
      var lastY = 0;
      var startX = 0;
      var startY = 0;
      var start = Date.now();

      function updateCamera() {
        orbit.phi = Math.max(0.24, Math.min(Math.PI - 0.24, orbit.phi));
        orbit.radius = Math.max(2.2, Math.min(22, orbit.radius));
        var sinPhi = Math.sin(orbit.phi);
        camera.position.set(
          orbit.target.x + orbit.radius * sinPhi * Math.sin(orbit.theta),
          orbit.target.y + orbit.radius * Math.cos(orbit.phi),
          orbit.target.z + orbit.radius * sinPhi * Math.cos(orbit.theta)
        );
        camera.lookAt(orbit.target);
      }

      function clearGraph() {
        objects = [];
        nodeMap = {};
        edgeItems = [];
        labels = [];
        edgeLabels = [];
        annotationLabels = [];
        while (labelLayer.firstChild) {
          labelLayer.removeChild(labelLayer.firstChild);
        }
        nodeRoot.children.slice().forEach(function (child) {
          nodeRoot.remove(child);
          disposeThreeObject(child);
        });
        edgeRoot.children.slice().forEach(function (child) {
          edgeRoot.remove(child);
          disposeThreeObject(child);
        });
      }

      function snapshotNodePositions() {
        var out = {};
        Object.keys(nodeMap).forEach(function (id) {
          out[id] = nodeMap[id].mesh.position.clone();
        });
        return out;
      }

      function groupCountText(node) {
        var count = Number(node && node.child_count);
        if (!Number.isFinite(count) || count <= 0) {
          return "";
        }
        return count === 1 ? "1 item hidden" : count + " items hidden";
      }

      function displayNodeLabel(node) {
        var label = itemLabel(node);
        if (node && node.__group) {
          var count = groupCountText(node);
          return count ? label + " · " + count : label;
        }
        return label;
      }

      function addLabel(node, mesh) {
        var label = el("div", "visual-three-label" + (node.__group ? " visual-three-group-label" : "") + (state.visualFocus[node.id] ? " visual-three-focus-label" : ""), labelEngine.text(node, node.__group ? 54 : 38));
        label.hidden = !labelEngine.shouldShow(node, node.__group);
        labelLayer.appendChild(label);
        labels.push({ element: label, mesh: mesh, node: node });
      }

      function addVisualAnnotation(annotation, mesh) {
        var text = visualAnnotationText(annotation);
        if (!text || !mesh) {
          return;
        }
        var label = el("div", "visual-three-annotation-label", text);
        if (annotation.summary) {
          label.setAttribute("title", String(annotation.summary));
        }
        labelLayer.appendChild(label);
        annotationLabels.push({ element: label, mesh: mesh, annotation: annotation });
      }

      function addEdgeLabel(edge) {
        var text = edge.label || edge.kind || "";
        if (!text) {
          return;
        }
        var label = el("div", "visual-three-edge-label", text);
        label.hidden = true;
        labelLayer.appendChild(label);
        edgeLabels.push({
          element: label,
          edge: edge,
          position: new THREE.Vector3()
        });
      }

      function startPositionForNode(node, target, previousPositions) {
        if (previousPositions[node.id]) {
          return previousPositions[node.id].clone();
        }
        if (node.__group && node.children && node.children.length) {
          var sum = new THREE.Vector3();
          var hits = 0;
          node.children.forEach(function (child) {
            if (child && previousPositions[child.id]) {
              sum.add(previousPositions[child.id]);
              hits += 1;
            }
          });
          if (hits > 0) {
            return sum.multiplyScalar(1 / hits);
          }
        }
        var parentID = state.parentByNode[node.id] || "";
        if (parentID && previousPositions[parentID]) {
          return previousPositions[parentID].clone();
        }
        if (parentID && lastExpandOrigin && lastExpandOrigin.id === parentID) {
          return lastExpandOrigin.position.clone();
        }
        return target.clone().multiplyScalar(0.16);
      }

      function anchorExpandedLayout(positions) {
        if (!lastExpandOrigin || !lastExpandOrigin.id || !lastExpandOrigin.position) {
          return;
        }
        var anchor = positions[lastExpandOrigin.id] ? positions[lastExpandOrigin.id].clone() : null;
        if (!anchor) {
          var group = state.groups[lastExpandOrigin.id];
          var sum = new THREE.Vector3();
          var hits = 0;
          (group && group.children ? group.children : []).forEach(function (child) {
            if (child && positions[child.id]) {
              sum.add(positions[child.id]);
              hits += 1;
            }
          });
          if (hits > 0) {
            anchor = sum.multiplyScalar(1 / hits);
          }
        }
        if (!anchor) {
          return;
        }
        var shift = lastExpandOrigin.position.clone().sub(anchor);
        Object.keys(positions).forEach(function (id) {
          positions[id].add(shift);
        });
      }

      function endpointMeshes(edge) {
        var from = nodeMap[edge.from] && nodeMap[edge.from].mesh;
        var to = nodeMap[edge.to] && nodeMap[edge.to].mesh;
        if (!from || !to) {
          return null;
        }
        return { from: from, to: to };
      }

      function updateEdgeGeometry(item) {
        var endpoints = endpointMeshes(item.edge);
        if (!endpoints) {
          return;
        }
        var edgeSpec = item.edgeSpec || resolveEdgeSpec(item.edge, markContext);
        var curve = edgeCurveFor(THREE, endpoints.from.position, endpoints.to.position, item.edge, edgeSpec, item.index || 0);
        item.curve = curve;
        if (item.line && item.line.geometry) {
          if (item.line.geometry.dispose) {
            item.line.geometry.dispose();
          }
          item.line.geometry = THREE.TubeGeometry ? new THREE.TubeGeometry(curve, 18, item.edge && item.edge.aggregated ? 0.02 : 0.012, 8, false) : new THREE.BufferGeometry().setFromPoints(curvePoints(curve, 18));
          if (item.line.geometry.computeBoundingSphere) {
            item.line.geometry.computeBoundingSphere();
          }
        }
        if (item.arrow) {
          var tip = curve.getPoint(edgeSpec.arrow === "reverse" ? 0.06 : 0.94);
          var tail = curve.getPoint(edgeSpec.arrow === "reverse" ? 0.14 : 0.86);
          var direction = tip.clone().sub(tail).normalize();
          item.arrow.position.copy(tip);
          item.arrow.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), direction.length() > 0.001 ? direction : new THREE.Vector3(0, 1, 0));
        }
        (item.markers || []).forEach(function (marker) {
          marker.userData.curve = curve;
        });
      }

      function updateEdgeLabelPosition(item) {
        var endpoints = endpointMeshes(item.edge);
        if (!endpoints) {
          return false;
        }
        item.position.copy(endpoints.from.position).add(endpoints.to.position).multiplyScalar(0.5);
        return true;
      }

      function setPointerFromEvent(event) {
        var rect = renderer.domElement.getBoundingClientRect();
        pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
      }

      function pointerLocalPoint(event, plane, target) {
        setPointerFromEvent(event);
        raycaster.setFromCamera(pointer, camera);
        var ray = raycaster.ray;
        if (!ray || !ray.origin || !ray.direction) {
          return null;
        }
        var denominator = plane.normal.dot(ray.direction);
        if (Math.abs(denominator) < 0.000001) {
          return null;
        }
        var distance = plane.point.clone().sub(ray.origin).dot(plane.normal) / denominator;
        if (!Number.isFinite(distance) || distance < 0) {
          return null;
        }
        target.copy(ray.origin).add(ray.direction.clone().multiplyScalar(distance));
        var parent = draggedMesh && draggedMesh.parent ? draggedMesh.parent : nodeRoot;
        parent.worldToLocal(target);
        return target;
      }

      function freezeCameraMotion() {
        autoRotateGraph = false;
      }

      function beginNodeDrag(mesh, event) {
        if (!mesh || !mesh.userData) {
          return false;
        }
        freezeCameraMotion();
        camera.updateMatrixWorld(true);
        mesh.parent.updateMatrixWorld(true);
        var normal = new THREE.Vector3();
        var worldPosition = mesh.position.clone();
        mesh.parent.localToWorld(worldPosition);
        camera.getWorldDirection(normal);
        dragPlane.normal.copy(normal);
        dragPlane.point.copy(worldPosition);
        draggedMesh = mesh;
        if (!pointerLocalPoint(event, dragPlane, dragPointerPoint)) {
          draggedMesh = null;
          return false;
        }
        dragOffset.copy(mesh.position).sub(dragPointerPoint);
        if (!mesh.userData.targetPosition) {
          mesh.userData.targetPosition = mesh.position.clone();
        }
        renderer.domElement.style.cursor = "grabbing";
        return true;
      }

      function dragNodeToPointer(mesh, event) {
        if (!mesh || !mesh.userData) {
          return;
        }
        if (!pointerLocalPoint(event, dragPlane, dragPointerPoint)) {
          return;
        }
        var target = dragPointerPoint.clone().add(dragOffset);
        if (!mesh.userData.targetPosition) {
          mesh.userData.targetPosition = mesh.position.clone();
        }
        var previous = mesh.userData.targetPosition.clone();
        var delta = target.clone().sub(previous);
        mesh.userData.targetPosition.copy(target);
        mesh.position.copy(target);
        var draggedID = mesh.userData.id;
        edgeItems.forEach(function (item) {
          var neighborID = "";
          if (item.edge.from === draggedID) {
            neighborID = item.edge.to;
          } else if (item.edge.to === draggedID) {
            neighborID = item.edge.from;
          }
          if (!neighborID || !nodeMap[neighborID]) {
            return;
          }
          var neighborMesh = nodeMap[neighborID].mesh;
          var nudge = delta.clone().multiplyScalar(0.18);
          if (!neighborMesh.userData.targetPosition) {
            neighborMesh.userData.targetPosition = neighborMesh.position.clone();
          }
          neighborMesh.userData.targetPosition.add(nudge);
          neighborMesh.position.add(nudge.clone().multiplyScalar(0.22));
        });
      }

      function buildParticles(total) {
        particleRoot.children.slice().forEach(function (child) {
          particleRoot.remove(child);
          disposeThreeObject(child);
        });
        var particleCount = Math.min(780, 160 + total * 10);
        var particlePositions = new Float32Array(particleCount * 3);
        for (var i = 0; i < particleCount; i += 1) {
          var a = (Math.PI * 2 * i) / particleCount;
          var r = 3.2 + (i % 13) * 0.18;
          particlePositions[i * 3] = Math.cos(a) * r;
          particlePositions[i * 3 + 1] = Math.sin(i * 0.41) * 1.55;
          particlePositions[i * 3 + 2] = Math.sin(a) * r;
        }
        var particleGeometry = new THREE.BufferGeometry();
        particleGeometry.setAttribute("position", new THREE.Float32BufferAttribute(particlePositions, 3));
        particleRoot.add(new THREE.Points(particleGeometry, new THREE.PointsMaterial({
          color: profile && profile.radar ? 0x35c2a1 : 0x63a9ff,
          size: 0.028,
          transparent: true,
          opacity: 0.44,
          blending: THREE.AdditiveBlending
        })));
      }

      function applyFocus(id) {
        selectedID = id || "";
        var neighborIDs = {};
        edgeItems.forEach(function (item) {
          var active = id && (item.edge.from === id || item.edge.to === id);
          item.line.userData.targetOpacity = active ? 0.98 : id ? 0.2 : item.line.userData.baseOpacity;
          if (item.arrow) {
            item.arrow.userData.targetOpacity = active ? 1 : id ? 0.16 : item.arrow.userData.baseOpacity;
          }
          (item.markers || []).forEach(function (marker) {
            marker.userData.targetOpacity = active ? 0.92 : id ? 0.12 : marker.userData.baseOpacity;
            marker.userData.targetScale = active ? marker.userData.baseScale * 1.35 : marker.userData.baseScale;
          });
          if (active) {
            neighborIDs[item.edge.from] = true;
            neighborIDs[item.edge.to] = true;
          }
        });
        Object.keys(nodeMap).forEach(function (key) {
          var mesh = nodeMap[key].mesh;
          var base = mesh.userData.baseScale || 1;
          var active = key === id;
          var neighbor = neighborIDs[key];
          mesh.userData.targetScale = base * (active ? 1.38 : neighbor ? 1.14 : 1);
          mesh.material.transparent = true;
          mesh.userData.targetOpacity = !id || active || neighbor ? mesh.userData.baseOpacity : 0.34;
          mesh.userData.targetEmissive = active ? 0.68 : neighbor ? 0.34 : mesh.userData.baseEmissive;
        });
      }

      function rebuild(model, filters) {
        currentModel = model || visibleGraph(state, filters || currentFilters);
        currentFilters = filters || currentFilters || {};
        var previousPositions = snapshotNodePositions();
        clearGraph();
        var positions = layoutGraphNodes3D(THREE, currentModel.nodes, preset);
        anchorExpandedLayout(positions);
        currentModel.nodes.forEach(function (node) {
          var markSpec = resolveMarkSpec(node, markContext);
          if (node.__group && !node.presentation) {
            markSpec.shape = "group_hull";
            markSpec.mesh = "box";
          }
          if (preset === "graph_2_5d" && markSpec.mesh === "sphere") {
            markSpec.mesh = "card";
            markSpec.shape = "node_card";
          }
          var mesh = createMarkMesh(markSpec, node, THREE, effects);
          var material = mesh.material;
          var visualFocus = !!state.visualFocus[node.id];
          if (visualFocus && material.emissiveIntensity !== undefined) {
            material.emissiveIntensity = Math.max(material.emissiveIntensity || 0, 0.46);
          }
          material.transparent = true;
          material.opacity = node.__group || visualFocus ? 0.92 : 0.82;
          var targetPosition = (positions[node.id] || new THREE.Vector3(0, 0, 0)).clone();
          mesh.position.copy(startPositionForNode(node, targetPosition, previousPositions));
          var scale = node.__group ? 1 + Math.min(1.7, (node.child_count || 1) / 18) : 0.72 + importanceValue(node, 0.35) * 0.7;
          if (visualFocus) {
            scale *= 1.22;
          }
          mesh.scale.setScalar(0.04);
          mesh.userData = {
            id: node.id,
            label: itemLabel(node),
            node: node,
            baseScale: scale,
            baseOpacity: material.opacity,
            baseEmissive: material.emissiveIntensity || 0.14,
            targetScale: scale,
            targetOpacity: material.opacity,
            targetEmissive: material.emissiveIntensity || 0.14,
            targetPosition: targetPosition
          };
          var icon = createIconBillboard(markSpec, node, THREE, markContext);
          if (icon) {
            mesh.add(icon);
          }
          material.opacity = 0;
          nodeRoot.add(mesh);
          objects.push(mesh);
          nodeMap[node.id] = { mesh: mesh, node: node };
          if (node.__group || visualFocus || currentModel.nodes.length <= 28 || importanceValue(node, 0) >= 0.72) {
            addLabel(node, mesh);
          }
        });
        (state.visual.annotations || []).forEach(function (annotation) {
          var target = nodeMap[String(annotation.target_id || "")];
          if (target && target.mesh) {
            addVisualAnnotation(annotation, target.mesh);
          }
        });
        currentModel.edges.forEach(function (edge, edgeIndex) {
          var endpoints = endpointMeshes(edge);
          if (!endpoints) {
            return;
          }
          var edgeSpec = resolveEdgeSpec(edge, markContext);
          var visualEdge = createDirectedEdge(THREE, endpoints, edge, edgeSpec, edgeIndex);
          edgeRoot.add(visualEdge.line);
          if (visualEdge.arrow) {
            edgeRoot.add(visualEdge.arrow);
          }
          (visualEdge.markers || []).forEach(function (marker) {
            edgeRoot.add(marker);
          });
          addEdgeLabel(edge);
          edgeItems.push({ line: visualEdge.line, arrow: visualEdge.arrow, edge: edge, markers: visualEdge.markers, curve: visualEdge.curve, edgeSpec: edgeSpec, index: edgeIndex });
        });
        buildParticles(currentModel.nodes.length + currentModel.edges.length);
        applyFocus(selectedID && nodeMap[selectedID] ? selectedID : "");
        lastExpandOrigin = null;
      }

      function raycast(event) {
        if (!objects.length) {
          return [];
        }
        setPointerFromEvent(event);
        raycaster.setFromCamera(pointer, camera);
        return raycaster.intersectObjects(objects, false);
      }

      function selectNode(node) {
        if (!node) {
          return;
        }
        freezeCameraMotion();
        selectedID = node.id;
        if (node.__group) {
          var groupMesh = nodeMap[node.id] && nodeMap[node.id].mesh;
          lastExpandOrigin = {
            id: node.id,
            position: groupMesh ? groupMesh.position.clone() : new THREE.Vector3()
          };
          state.collapsed[node.id] = !state.collapsed[node.id];
          inspector.show(itemLabel(node), {
            id: node.id,
            label: itemLabel(node),
            collapsed: state.collapsed[node.id],
            child_count: node.child_count,
            children: (node.children || []).map(function (child) { return child.id; })
          });
          if (onGraphChange) {
            onGraphChange();
          } else {
            rebuild(null, currentFilters);
          }
          return;
        }
        applyFocus(node.id);
        inspector.show(itemLabel(node), node);
      }

      renderer.domElement.addEventListener("contextmenu", function (event) {
        event.preventDefault();
      });
      renderer.domElement.addEventListener("pointerdown", function (event) {
        freezeCameraMotion();
        dragging = true;
        moved = false;
        draggedMesh = null;
        pendingMesh = null;
        dragMode = event.shiftKey || event.button === 2 ? "pendingPan" : "pendingOrbit";
        if (!event.shiftKey && event.button !== 2) {
          var hits = raycast(event);
          if (hits.length && hits[0].object.userData) {
            pendingMesh = hits[0].object;
            dragMode = "pendingNode";
            hoverID = pendingMesh.userData.id || "";
            renderer.domElement.style.cursor = "pointer";
          }
        }
        startX = event.clientX;
        startY = event.clientY;
        lastX = event.clientX;
        lastY = event.clientY;
        if (renderer.domElement.setPointerCapture) {
          renderer.domElement.setPointerCapture(event.pointerId);
        }
        event.preventDefault();
        event.stopPropagation();
      });
      renderer.domElement.addEventListener("pointermove", function (event) {
        if (dragging) {
          var dx = event.clientX - lastX;
          var dy = event.clientY - lastY;
          var totalMove = Math.abs(event.clientX - startX) + Math.abs(event.clientY - startY);
          if (!moved && totalMove <= 7) {
            event.preventDefault();
            event.stopPropagation();
            return;
          }
          if (!moved) {
            moved = true;
            if (dragMode === "pendingNode" && pendingMesh && beginNodeDrag(pendingMesh, event)) {
              dragMode = "node";
              applyFocus(pendingMesh.userData.id);
            } else if (dragMode === "pendingNode") {
              dragMode = "idle";
            } else if (dragMode === "pendingPan") {
              freezeCameraMotion();
              dragMode = "pan";
            } else {
              freezeCameraMotion();
              dragMode = "orbit";
            }
          }
          if (dragMode === "node" && draggedMesh) {
            dragNodeToPointer(draggedMesh, event);
          } else if (dragMode === "pan") {
            orbit.target.x -= dx * orbit.radius * 0.0018;
            orbit.target.y += dy * orbit.radius * 0.0018;
          } else if (dragMode === "orbit") {
            orbit.theta -= dx * 0.006;
            orbit.phi -= dy * 0.005;
          }
          lastX = event.clientX;
          lastY = event.clientY;
          if (dragMode !== "node") {
            updateCamera();
          }
          event.preventDefault();
          event.stopPropagation();
          return;
        }
        var hits = raycast(event);
        hoverID = hits.length && hits[0].object.userData ? hits[0].object.userData.id : "";
        renderer.domElement.style.cursor = hoverID ? "pointer" : "grab";
      });
      renderer.domElement.addEventListener("pointerup", function (event) {
        var releasedMesh = draggedMesh || pendingMesh;
        dragging = false;
        draggedMesh = null;
        pendingMesh = null;
        var releasedMode = dragMode;
        dragMode = "idle";
        if (renderer.domElement.releasePointerCapture) {
          renderer.domElement.releasePointerCapture(event.pointerId);
        }
        if (!moved && releasedMesh && releasedMesh.userData) {
          selectNode(releasedMesh.userData.node);
        } else if (moved && releasedMode === "node" && releasedMesh && releasedMesh.userData) {
          applyFocus(releasedMesh.userData.id);
          inspector.show(itemLabel(releasedMesh.userData.node), releasedMesh.userData.node);
        } else if (!moved) {
          var hits = raycast(event);
          if (hits.length && hits[0].object.userData) {
            selectNode(hits[0].object.userData.node);
          }
        }
        renderer.domElement.style.cursor = hoverID ? "pointer" : "grab";
        event.preventDefault();
        event.stopPropagation();
      });
      renderer.domElement.addEventListener("wheel", function (event) {
        freezeCameraMotion();
        orbit.radius *= event.deltaY > 0 ? 1.08 : 0.92;
        updateCamera();
        event.preventDefault();
        event.stopPropagation();
      }, { passive: false });
      renderer.domElement.addEventListener("dblclick", function (event) {
        freezeCameraMotion();
        orbit.theta = 0.24;
        orbit.phi = 1.16;
        orbit.radius = 7.4;
        orbit.target.set(0, 0, 0);
        selectedID = "";
        applyFocus("");
        updateCamera();
        event.preventDefault();
      });

      function resize() {
        width = Math.max(720, stage.clientWidth || width);
        height = Math.max(520, stage.clientHeight || height);
        camera.aspect = width / height;
        camera.updateProjectionMatrix();
        renderer.setSize(width, height, false);
      }
      var observer = typeof ResizeObserver !== "undefined" ? new ResizeObserver(resize) : null;
      if (observer) {
        observer.observe(stage);
      }

      function updateLabels() {
        root.updateMatrixWorld(true);
        labels.forEach(function (item) {
          var pos = item.mesh.position.clone();
          item.mesh.parent.localToWorld(pos);
          pos.project(camera);
          var visible = pos.z < 1 && pos.x >= -1.18 && pos.x <= 1.18 && pos.y >= -1.18 && pos.y <= 1.18;
          var important = item.node.__group || state.visualFocus[item.node.id] || item.node.id === selectedID || item.node.id === hoverID || importanceValue(item.node, 0) >= 0.72 || labels.length <= 28;
          item.element.hidden = !visible || !important;
          if (!item.element.hidden) {
            item.element.style.left = ((pos.x * 0.5 + 0.5) * width).toFixed(1) + "px";
            item.element.style.top = ((-pos.y * 0.5 + 0.5) * height).toFixed(1) + "px";
            item.element.toggleAttribute("data-selected", item.node.id === selectedID);
          }
        });
        annotationLabels.forEach(function (item) {
          var pos = item.mesh.position.clone();
          item.mesh.parent.localToWorld(pos);
          pos.y += 0.42;
          pos.project(camera);
          var visible = pos.z < 1 && pos.x >= -1.12 && pos.x <= 1.12 && pos.y >= -1.12 && pos.y <= 1.12;
          var targetID = String(item.annotation.target_id || "");
          var active = !selectedID || selectedID === targetID || state.visualFocus[targetID];
          item.element.hidden = !visible || !active;
          if (!item.element.hidden) {
            item.element.style.left = ((pos.x * 0.5 + 0.5) * width).toFixed(1) + "px";
            item.element.style.top = ((-pos.y * 0.5 + 0.5) * height).toFixed(1) + "px";
          }
        });
        edgeLabels.forEach(function (item) {
          if (!updateEdgeLabelPosition(item)) {
            item.element.hidden = true;
            return;
          }
          var active = selectedID && (item.edge.from === selectedID || item.edge.to === selectedID);
          var important = item.edge.aggregated || importanceValue(item.edge, 0) >= 0.72;
          var overview = !selectedID && important && edgeLabels.length <= 24;
          var pos = item.position.clone();
          root.localToWorld(pos);
          pos.project(camera);
          var visible = pos.z < 1 && pos.x >= -1.12 && pos.x <= 1.12 && pos.y >= -1.12 && pos.y <= 1.12;
          item.element.hidden = !visible || (!active && !overview);
          if (!item.element.hidden) {
            item.element.style.left = ((pos.x * 0.5 + 0.5) * width).toFixed(1) + "px";
            item.element.style.top = ((-pos.y * 0.5 + 0.5) * height).toFixed(1) + "px";
            item.element.toggleAttribute("data-selected", !!active);
          }
        });
      }

      function easeValue(current, target, amount) {
        return current + (target - current) * amount;
      }

      function updateGraphTransitions(t) {
        Object.keys(nodeMap).forEach(function (key) {
          var mesh = nodeMap[key].mesh;
          var targetScale = mesh.userData.targetScale || mesh.userData.baseScale || 1;
          var currentScale = mesh.scale.x || 0.01;
          mesh.scale.setScalar(easeValue(currentScale, targetScale, 0.12));
          if (mesh.userData.targetPosition) {
            var positionEase = dragMode === "node" && draggedMesh === mesh ? 0.36 : 0.12;
            mesh.position.lerp(mesh.userData.targetPosition, positionEase);
          }
          mesh.material.opacity = easeValue(mesh.material.opacity, mesh.userData.targetOpacity !== undefined ? mesh.userData.targetOpacity : mesh.userData.baseOpacity, 0.12);
          if (mesh.material.emissiveIntensity !== undefined) {
            mesh.material.emissiveIntensity = easeValue(mesh.material.emissiveIntensity, mesh.userData.targetEmissive !== undefined ? mesh.userData.targetEmissive : mesh.userData.baseEmissive, 0.12);
          }
        });
        edgeItems.forEach(function (item) {
          updateEdgeGeometry(item);
          item.line.material.opacity = easeValue(item.line.material.opacity, item.line.userData.targetOpacity, 0.1);
          if (item.arrow) {
            item.arrow.material.opacity = easeValue(item.arrow.material.opacity, item.arrow.userData.targetOpacity, 0.12);
          }
          (item.markers || []).forEach(function (marker) {
            marker.material.opacity = easeValue(marker.material.opacity, marker.userData.targetOpacity, 0.12);
            var p = (t * marker.userData.speed + marker.userData.phase) % 1;
            p = 0.18 + p * 0.64;
            if (marker.userData.curve && marker.userData.curve.getPoint) {
              marker.position.copy(marker.userData.curve.getPoint(p));
            }
            var targetScale = marker.userData.targetScale || marker.userData.baseScale || 1;
            var currentScale = marker.scale.x || 1;
            marker.scale.setScalar(easeValue(currentScale, targetScale, 0.12));
          });
        });
      }

      updateCamera();
      function animate() {
        if (!document.body.contains(stage)) {
          if (observer) {
            observer.disconnect();
          }
          renderer.dispose();
          return;
        }
        var t = (Date.now() - start) / 1000;
        if (autoRotateGraph && !dragging && !selectedID) {
          root.rotation.y = Math.sin(t * 0.18) * 0.035;
        }
        particleRoot.rotation.y = t * 0.08;
        updateGraphTransitions(t);
        updateLabels();
        renderer.render(scene, camera);
        window.requestAnimationFrame(animate);
      }
      animate();
      stage.classList.add("visual-three-active");
      return {
        rebuild: rebuild,
        focusNode: function (id) {
          if (nodeMap[id]) {
            selectedID = id;
            applyFocus(id);
          }
        },
        scene: scene,
        renderer: renderer,
        camera: camera
      };
    } catch (err) {
      stage.classList.remove("visual-three-primary");
      stage.classList.add("visual-three-fallback");
      stage.setAttribute("data-three-error", err && err.message ? err.message : String(err));
      return null;
    }
  }

  function uniqueValues(items, key) {
    var seen = {};
    var out = [];
    items.forEach(function (item) {
      var value = item && item[key] ? String(item[key]) : "";
      if (value && !seen[value]) {
        seen[value] = true;
        out.push(value);
      }
    });
    return out.sort();
  }

  function selectControl(label, values) {
    var select = document.createElement("select");
    select.setAttribute("aria-label", label);
    var all = document.createElement("option");
    all.value = "";
    all.textContent = label;
    select.appendChild(all);
    values.forEach(function (value) {
      var option = document.createElement("option");
      option.value = value;
      option.textContent = value;
      select.appendChild(option);
    });
    return select;
  }

  function nodeColor(status) {
    switch (String(status || "").toLowerCase()) {
      case "success":
      case "supported":
      case "ok":
        return "#47c477";
      case "warning":
      case "retry":
        return "#e5a84c";
      case "error":
      case "failed":
      case "refuted":
        return "#ee6b73";
      case "blocked":
      case "busy":
        return "#a77cff";
      default:
        return "#63a9ff";
    }
  }

  function layoutNodes(nodes, preset, width, height) {
    preset = normalizePreset(preset);
    var positions = {};
    var cx = width / 2;
    var cy = height / 2;
    var count = Math.max(nodes.length, 1);
    var cols = Math.max(1, Math.ceil(Math.sqrt(count)));
    nodes.forEach(function (node, index) {
      var angle = (Math.PI * 2 * index) / count;
      var radius = Math.min(width, height) * 0.33;
      var x = cx + Math.cos(angle) * radius;
      var y = cy + Math.sin(angle) * radius;
      if (preset === "layered_stack" || preset === "state_machine") {
        var group = Math.floor(index / cols);
        var col = index % cols;
        x = 110 + col * Math.max(140, (width - 220) / cols);
        y = 90 + group * 120;
      } else if (preset === "timeline_tunnel" || preset === "pipeline_flow" || preset === "flow_particles") {
        x = 90 + index * Math.max(110, (width - 180) / Math.max(1, count - 1));
        y = cy + Math.sin(index * 1.1) * 90;
      } else if (preset === "radial_tree" || preset === "orbit_system") {
        var depth = node.group ? String(node.group).length % 4 : index % 4;
        radius = 90 + depth * 85;
        x = cx + Math.cos(angle) * radius;
        y = cy + Math.sin(angle) * radius;
      } else if (preset === "ripple" || preset === "radar_sphere") {
        radius = 70 + index * 26;
        x = cx + Math.cos(angle) * radius;
        y = cy + Math.sin(angle) * radius;
      } else if (preset === "control_room" || preset === "matrix_board") {
        x = 140 + (index % 4) * 190;
        y = 100 + Math.floor(index / 4) * 140;
      } else if (preset === "city_map") {
        x = 120 + (index % 5) * 150;
        y = height - 90 - (index % 4) * 78;
      } else if (preset === "graph_3d" || preset === "graph_2_5d" || preset === "sankey_3d") {
        x = 120 + (index % cols) * Math.max(150, (width - 240) / cols);
        y = 120 + Math.floor(index / cols) * 120 + (index % 2) * 28;
      } else if (preset === "diff_split_view") {
        x = index % 2 === 0 ? width * 0.28 : width * 0.72;
        y = 90 + Math.floor(index / 2) * 110;
      }
      positions[node.id] = { x: x, y: y };
    });
    return positions;
  }

  function renderGraph(ctx) {
    var data = ctx.data || {};
    var manifest = ctx.manifest || {};
    var events = Array.isArray(data.events) ? data.events : [];
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    var state = buildGraphState(data, manifest);
    var labelEngine = createLabelEngine({ mode: state.visual.labelMode, focusIDs: state.visualFocus });
    var threeGraph = isPrimaryThreeGraph(manifest, preset, profile) ? createThreeGraphScene(shell.stage, manifest, state, preset, profile, shell.inspector, function () {
      rebuildGraph();
    }) : null;
    if (!threeGraph) {
      createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    }
    var search = document.createElement("input");
    search.type = "search";
    search.placeholder = "Search";
    var statusFilter = selectControl("All statuses", uniqueValues(state.rawNodes.concat(state.groupOrder.map(function (id) { return state.groups[id]; })), "status"));
    var kindFilter = selectControl("All kinds", uniqueValues(state.rawNodes.concat(state.groupOrder.map(function (id) { return state.groups[id]; })), "kind"));
    var edgeKindFilter = selectControl("All edges", uniqueValues(state.rawEdges, "kind"));
    var overview = document.createElement("button");
    overview.textContent = "Overview";
    var replay = document.createElement("button");
    replay.textContent = "Replay";
    var exportBtn = document.createElement("button");
    exportBtn.textContent = "Export";
    var countBadge = el("span", "visual-count-badge");
    shell.toolbar.appendChild(search);
    shell.toolbar.appendChild(statusFilter);
    shell.toolbar.appendChild(kindFilter);
    shell.toolbar.appendChild(edgeKindFilter);
    shell.toolbar.appendChild(overview);
    shell.toolbar.appendChild(replay);
    shell.toolbar.appendChild(exportBtn);
    shell.toolbar.appendChild(countBadge);
    if (state.visual.showLegend) {
      var legendSpec = buildLegendItems(data, state, createMarkContext(manifest, data));
      createLegendOverlay(shell.stage, legendSpec.title, legendSpec.items, function (value, active) {
        if (colorByFromVisual(state.visual) === "kind") {
          kindFilter.value = active ? value : "";
          rebuildGraph();
        }
      });
    }

    var width = Math.max(900, shell.stage.clientWidth || 900);
    var height = Math.max(620, shell.stage.clientHeight || 620);
    var canvas = null;
    if (!threeGraph) {
      canvas = svg("svg", { class: "visual-svg", viewBox: "0 0 " + width + " " + height, role: "img" });
      shell.stage.appendChild(canvas);
    }
    var nodeElements = {};
    var edgeElements = [];
    var currentModel = { nodes: [], edges: [] };

    function clearCanvas() {
      if (!canvas) {
        return;
      }
      while (canvas.firstChild) {
        canvas.removeChild(canvas.firstChild);
      }
    }

    function drawOrbitGuides() {
      if (!canvas) {
        return;
      }
      if (preset === "orbit_system" || preset === "ripple" || preset === "radar_sphere") {
        [90, 170, 250].forEach(function (radius) {
          canvas.appendChild(svg("circle", { class: "visual-orbit", cx: width / 2, cy: height / 2, r: radius }));
        });
      }
    }

    function rebuildGraph() {
      nodeElements = {};
      edgeElements = [];
      var filters = {
        query: search.value,
        status: statusFilter.value,
        kind: kindFilter.value,
        edgeKind: edgeKindFilter.value
      };
      currentModel = visibleGraph(state, filters);
      countBadge.textContent = currentModel.nodes.length + "/" + state.rawNodes.length + " nodes · " + currentModel.edges.length + "/" + state.rawEdges.length + " edges";
      if (threeGraph) {
        threeGraph.rebuild(currentModel, filters);
        return;
      }
      clearCanvas();
      drawOrbitGuides();
      var positions = layoutNodes(currentModel.nodes, preset, width, height);

      currentModel.edges.forEach(function (edge, index) {
        var from = positions[edge.from];
        var to = positions[edge.to];
        if (!from || !to) {
          return;
        }
        var edgeClass = "visual-edge";
        if (edge.aggregated) {
          edgeClass += " visual-edge-aggregated";
        }
        if (preset === "sankey_3d" || preset === "pipeline_flow" || preset === "flow_particles") {
          edgeClass += " visual-edge-flow";
        }
        var path = edgePath(from, to, preset, index);
        var line = svg("path", { class: edgeClass, d: path });
        if (edge.weight || edge.value) {
          line.setAttribute("stroke-width", Math.max(1.5, Math.min(12, Number(edge.weight || edge.value) || 1)));
        }
        canvas.appendChild(line);
        edgeElements.push({ element: line, edge: edge });
        if (profile.particles && (preset === "flow_particles" || preset === "pipeline_flow" || preset === "sankey_3d" || preset === "graph_3d" || preset === "constellation")) {
          addFlowParticle(canvas, path, index, edge.status);
        }
        if (edge.label && labelEngine.shouldShow(edge, currentModel.edges.length <= 40 || edge.aggregated)) {
          var edgeLabel = svg("text", { class: "visual-edge-label", x: (from.x + to.x) / 2, y: (from.y + to.y) / 2 - 5 });
          edgeLabel.textContent = runtime.safeText(edge.label);
          canvas.appendChild(edgeLabel);
        }
      });

      currentModel.nodes.forEach(function (node, index) {
        var pos = positions[node.id] || { x: width / 2, y: height / 2 };
        var className = "visual-node" + (node.__group ? " visual-group-node" : "") + (state.visualFocus[node.id] ? " visual-focused" : "");
        var group = svg("g", { class: className, tabindex: "0" });
        var depth = nodeDepth(node, index, preset);
        var zLift = Math.round(depth * 34);
        group.setAttribute("style", "--node-shadow-x:" + Math.round(6 + depth * 10) + "px; --node-shadow-y:" + Math.round(8 + depth * 12) + "px");
        if (profile.grid || profile.city || preset === "graph_3d" || preset === "graph_2_5d" || preset === "sankey_3d") {
          canvas.appendChild(svg("ellipse", { class: "visual-node-shadow", cx: pos.x + zLift * 0.55, cy: pos.y + 38 + zLift * 0.28, rx: node.__group ? 42 + depth * 16 : 30 + depth * 12, ry: 8 + depth * 4 }));
          canvas.appendChild(svg("line", { class: "visual-depth-line", x1: pos.x, y1: pos.y + 26, x2: pos.x + zLift * 0.55, y2: pos.y + 38 + zLift * 0.28 }));
        }
        if (zLift) {
          group.setAttribute("transform", "translate(" + (-zLift * 0.24).toFixed(1) + " " + (-zLift).toFixed(1) + ")");
        }
        var shape;
        if (node.__group) {
          shape = svg("rect", { x: pos.x - 58, y: pos.y - 33, width: 116, height: 66, rx: 10, fill: nodeColor(node.status) });
        } else if (preset === "city_map") {
          var heightValue = 44 + ((node.metrics && Number(node.metrics.risk)) || index % 5) * 10;
          shape = svg("rect", { x: pos.x - 26, y: pos.y - heightValue, width: 52, height: heightValue, rx: 6, fill: nodeColor(node.status) });
        } else if (preset === "layered_stack" || preset === "state_machine" || preset === "control_room" || preset === "diff_split_view") {
          shape = svg("rect", { x: pos.x - 44, y: pos.y - 25, width: 88, height: 50, rx: 8, fill: nodeColor(node.status) });
        } else {
          shape = svg("circle", { cx: pos.x, cy: pos.y, r: 28, fill: nodeColor(node.status) });
        }
        group.appendChild(shape);
        if (node.__group) {
          var sign = svg("text", { class: "visual-group-count", x: pos.x, y: pos.y + 5, "text-anchor": "middle" });
          var hiddenText = Number(node.child_count || 0) === 1 ? "1 hidden" : runtime.safeText(node.child_count || 0) + " hidden";
          sign.textContent = state.collapsed[node.id] ? hiddenText : "expanded";
          group.appendChild(sign);
        }
        if (labelEngine.shouldShow(node, currentModel.nodes.length <= 28 || node.__group)) {
          var label = svg("text", { x: pos.x, y: pos.y + (node.__group ? 52 : 46), "text-anchor": "middle" });
          label.textContent = runtime.safeText(labelEngine.text(node, 42));
          group.appendChild(label);
        }
        visualAnnotationsFor(state.visual, node.id).forEach(function (annotation, noteIndex) {
          var note = svg("text", { x: pos.x, y: pos.y - 42 - noteIndex * 14, "text-anchor": "middle", class: "visual-svg-annotation-label" });
          note.textContent = runtime.safeText(visualAnnotationText(annotation));
          group.appendChild(note);
        });
        group.addEventListener("click", function () {
          if (node.__group) {
            state.collapsed[node.id] = !state.collapsed[node.id];
            shell.inspector.show(itemLabel(node), {
              id: node.id,
              label: itemLabel(node),
              collapsed: state.collapsed[node.id],
              child_count: node.child_count,
              children: (node.children || []).map(function (child) { return child.id; })
            });
            rebuildGraph();
            focusNode(node.id);
            return;
          }
          focusNode(node.id);
          shell.inspector.show(itemLabel(node), node);
        });
        canvas.appendChild(group);
        nodeElements[node.id] = { element: group, node: node };
      });
    }

    function focusNode(id) {
      if (threeGraph) {
        threeGraph.focusNode(id);
        return;
      }
      Object.keys(nodeElements).forEach(function (key) {
        nodeElements[key].element.classList.toggle("visual-focused", key === id);
      });
      edgeElements.forEach(function (item) {
        var active = item.edge.from === id || item.edge.to === id;
        item.element.classList.toggle("visual-focused", active);
      });
    }

    search.addEventListener("input", rebuildGraph);
    statusFilter.addEventListener("change", rebuildGraph);
    kindFilter.addEventListener("change", rebuildGraph);
    edgeKindFilter.addEventListener("change", rebuildGraph);
    overview.addEventListener("click", function () {
      search.value = "";
      statusFilter.value = "";
      kindFilter.value = "";
      edgeKindFilter.value = "";
      state.groupOrder.forEach(function (id) {
        state.collapsed[id] = state.design.defaultCollapseDepth > 0;
      });
      rebuildGraph();
    });
    exportBtn.addEventListener("click", function () {
      runtime.exportJSON(data, "visual-data.json");
    });
    replay.addEventListener("click", function () {
      if (!events.length) {
        shell.inspector.show("Replay", { message: "No events in this input." });
        return;
      }
      var index = 0;
      var timer = window.setInterval(function () {
        var event = events[index];
        if (event && event.node_id) {
          if (nodeElements[event.node_id]) {
            focusNode(event.node_id);
          } else if (state.parentByNode[event.node_id]) {
            focusNode(state.parentByNode[event.node_id]);
          }
        }
        shell.inspector.show(event && (event.label || event.summary || event.id) || "Event", event);
        index += 1;
        if (index >= events.length) {
          window.clearInterval(timer);
        }
      }, 650);
    });
    rebuildGraph();
  }

  function renderTimeline(ctx) {
    var data = ctx.data || {};
    var visual = readVisualHints(data);
    var focusIDs = idSet(visual.initialFocusIDs);
    var hiddenIDs = idSet(visual.hiddenDetailIDs);
    var events = (Array.isArray(data.events) ? data.events : []).filter(function (event) {
      return !event || !event.id || !hiddenIDs[event.id] || focusIDs[event.id];
    });
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    var markContext = createMarkContext(manifest, data);
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var exportBtn = document.createElement("button");
    exportBtn.textContent = "Export";
    shell.toolbar.appendChild(exportBtn);
    if (shouldShowSemanticLegend(visual)) {
      var legendSpec = buildLegendItems(data, { visual: visual, rawNodes: events, rawEdges: [] }, markContext);
      createLegendOverlay(shell.stage, legendSpec.title, legendSpec.items);
    }
    var lane = el("div", "timeline-lane visual-timeline-3d");
    lane.appendChild(el("div", "timeline-track"));
    events.forEach(function (event, index) {
      var markSpec = resolveMarkSpec(event, markContext);
      var colorSpec = resolveColorSpec(event, markContext);
      var eventColor = normalizeColorString(markSpec.color || colorSpec.color);
      var card = el("article", "visual-card timeline-event visual-mark-shape-" + safeClass(markSpec.shape) + (focusIDs[event.id] ? " visual-card-focus" : ""));
      card.setAttribute("data-mark-shape", markSpec.shape || "");
      card.setAttribute("data-mark-mesh", markSpec.mesh || "");
      card.setAttribute("data-mark-icon", markSpec.icon || "");
      card.style.setProperty("--mark-color", eventColor);
      card.style.borderColor = eventColor;
      card.style.setProperty("--event-z", Math.round(((index % 6) / 5) * 64) + "px");
      card.style.setProperty("--event-delay", (index * 0.05).toFixed(2) + "s");
      var dot = el("span", "timeline-dot timeline-mark-dot");
      dot.style.backgroundColor = eventColor;
      card.appendChild(dot);
      var icon = createInlineIcon(markSpec, markContext, "visual-inline-icon timeline-event-icon");
      if (icon) {
        card.appendChild(icon);
      }
      card.appendChild(el("div", "visual-card-title", event.label || event.summary || event.id));
      var status = runtime.formatStatus(event.status || event.kind);
      card.appendChild(el("span", status.className, status.label));
      card.appendChild(el("div", "visual-card-meta", [event.time, event.kind, event.summary].filter(Boolean).join(" · ")));
      visualAnnotationsFor(visual, event.id).forEach(function (annotation) {
        card.appendChild(el("div", "visual-card-annotation", visualAnnotationText(annotation)));
      });
      card.addEventListener("click", function () {
        shell.inspector.show(event.label || event.id, event);
      });
      lane.appendChild(card);
    });
    shell.stage.appendChild(lane);
    exportBtn.addEventListener("click", function () {
      runtime.exportJSON(data, "visual-data.json");
    });
  }

  function renderEvidence(ctx) {
    var data = ctx.data || {};
    var visual = readVisualHints(data);
    var focusIDs = idSet(visual.initialFocusIDs);
    var hiddenIDs = idSet(visual.hiddenDetailIDs);
    var claims = (Array.isArray(data.claims) ? data.claims : []).filter(function (claim) {
      return !claim || !claim.id || !hiddenIDs[claim.id] || focusIDs[claim.id];
    });
    var sources = (Array.isArray(data.sources) ? data.sources : []).filter(function (source) {
      return !source || !source.id || !hiddenIDs[source.id] || focusIDs[source.id];
    });
    var links = Array.isArray(data.links) ? data.links : [];
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    var markContext = createMarkContext(manifest, data);
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var width = Math.max(900, shell.stage.clientWidth || 900);
    var height = Math.max(620, shell.stage.clientHeight || 620);
    var canvas = svg("svg", { class: "visual-svg", viewBox: "0 0 " + width + " " + height });
    shell.stage.appendChild(canvas);
    var defs = svg("defs", {});
    canvas.appendChild(defs);
    if (shouldShowSemanticLegend(visual)) {
      var legendSpec = buildLegendItems(data, { visual: visual, rawNodes: claims.concat(sources), rawEdges: links }, markContext);
      createLegendOverlay(shell.stage, legendSpec.title, legendSpec.items);
    }

    function addArrowMarker(id, color) {
      var marker = svg("marker", {
        id: id,
        viewBox: "0 0 10 10",
        refX: 8.5,
        refY: 5,
        markerWidth: 8,
        markerHeight: 8,
        orient: "auto-start-reverse"
      });
      marker.appendChild(svg("path", { d: "M 0 0 L 10 5 L 0 10 z", fill: color || "#63a9ff" }));
      defs.appendChild(marker);
    }

    function evidenceDash(edgeSpec, relation) {
      relation = normalizeMarkKey(relation);
      if (edgeSpec.lineStyle === "dotted") {
        return "2 7";
      }
      if (edgeSpec.lineStyle === "dashed" || relation === "mentions") {
        return "10 8";
      }
      return "";
    }

    function appendEvidenceShape(group, pos, widthValue, heightValue, spec, color, role) {
      var left = pos.x - widthValue / 2;
      var top = pos.y - heightValue / 2;
      var shape = normalizeMarkKey((spec && spec.shape) || (spec && spec.mesh));
      var attrs = { fill: color, stroke: "rgba(198, 216, 238, 0.34)", "stroke-width": role === "claim" ? 1.8 : 1.4 };
      if (shape === "diamond" || shape === "octahedron") {
        group.appendChild(svg("polygon", Object.assign({
          points: pos.x + "," + top + " " + (left + widthValue) + "," + pos.y + " " + pos.x + "," + (top + heightValue) + " " + left + "," + pos.y
        }, attrs)));
      } else if (shape === "warning_prism" || shape === "cone") {
        group.appendChild(svg("polygon", Object.assign({
          points: pos.x + "," + top + " " + (left + widthValue) + "," + (top + heightValue) + " " + left + "," + (top + heightValue)
        }, attrs)));
      } else if (shape === "queue_capsule" || shape === "stream_rail" || shape === "event_bus" || shape === "capsule") {
        group.appendChild(svg("rect", Object.assign({ x: left, y: top, width: widthValue, height: heightValue, rx: Math.round(heightValue / 2) }, attrs)));
      } else if (shape === "cloud_plate" || shape === "cloud") {
        group.appendChild(svg("ellipse", Object.assign({ cx: pos.x, cy: pos.y, rx: widthValue / 2, ry: heightValue / 2 }, attrs)));
      } else if (shape === "database_cylinder" || shape === "bucket" || shape === "cylinder") {
        group.appendChild(svg("rect", Object.assign({ x: left, y: top + 5, width: widthValue, height: heightValue - 10, rx: 12 }, attrs)));
        group.appendChild(svg("ellipse", Object.assign({ cx: pos.x, cy: top + 7, rx: widthValue / 2, ry: 10 }, attrs)));
        group.appendChild(svg("ellipse", { cx: pos.x, cy: top + heightValue - 7, rx: widthValue / 2, ry: 10, fill: "none", stroke: "rgba(198, 216, 238, 0.28)", "stroke-width": 1.2 }));
      } else if (shape === "hex_service" || shape === "hex_prism") {
        var inset = Math.min(22, widthValue * 0.16);
        group.appendChild(svg("polygon", Object.assign({
          points: (left + inset) + "," + top + " " + (left + widthValue - inset) + "," + top + " " + (left + widthValue) + "," + pos.y + " " + (left + widthValue - inset) + "," + (top + heightValue) + " " + (left + inset) + "," + (top + heightValue) + " " + left + "," + pos.y
        }, attrs)));
      } else {
        group.appendChild(svg("rect", Object.assign({ x: left, y: top, width: widthValue, height: heightValue, rx: 8 }, attrs)));
      }
    }

    var claimPos = {};
    var sourcePos = {};
    claims.forEach(function (claim, index) {
      claimPos[claim.id] = { x: width * 0.62, y: 90 + index * 110 };
    });
    sources.forEach(function (source, index) {
      sourcePos[source.id] = { x: width * 0.22, y: 90 + index * 105 };
    });
    links.forEach(function (link) {
      var a = sourcePos[link.source_id];
      var b = claimPos[link.claim_id];
      if (a && b) {
        var edgeSpec = resolveEdgeSpec(link, markContext);
        var markerID = "visual-evidence-arrow-" + safeClass(link.relation || link.kind || "edge") + "-" + links.indexOf(link);
        if (edgeSpec.directed && edgeSpec.arrow !== "none") {
          addArrowMarker(markerID, edgeSpec.color);
        }
        var attrs = {
          class: "visual-edge visual-evidence-beam visual-evidence-relation-" + safeClass(link.relation || link.kind),
          d: edgePath(a, b, preset, links.indexOf(link)),
          stroke: edgeSpec.color,
          "stroke-width": (1.4 + Math.max(0, Math.min(1, Number(link.weight || 0))) * 1.6).toFixed(2),
          opacity: edgeSpec.opacity,
          "data-relation": link.relation || link.kind || "",
          "data-arrow": edgeSpec.arrow
        };
        var dash = evidenceDash(edgeSpec, link.relation || link.kind);
        if (dash) {
          attrs["stroke-dasharray"] = dash;
        }
        if (edgeSpec.directed && edgeSpec.arrow !== "none") {
          if (edgeSpec.arrow === "reverse") {
            attrs["marker-start"] = "url(#" + markerID + ")";
          } else {
            attrs["marker-end"] = "url(#" + markerID + ")";
          }
        }
        canvas.appendChild(svg("path", attrs));
      }
    });
    sources.forEach(function (source) {
      var pos = sourcePos[source.id];
      var spec = resolveMarkSpec(source, markContext);
      var colorSpec = resolveColorSpec(source, markContext);
      var color = normalizeColorString(spec.color || colorSpec.color);
      var group = svg("g", {
        class: "visual-node visual-evidence-node visual-evidence-source visual-mark-shape-" + safeClass(spec.shape) + (focusIDs[source.id] ? " visual-focused" : ""),
        "data-mark-shape": spec.shape || "",
        "data-mark-icon": spec.icon || "",
        "data-kind": source.kind || ""
      });
      appendEvidenceShape(group, pos, 124, 54, spec, color, "source");
      appendSvgIcon(group, spec, markContext, pos.x - 54, pos.y - 12, 20);
      var label = svg("text", { x: pos.x, y: pos.y + 4, "text-anchor": "middle" });
      label.textContent = runtime.safeText(source.title || source.id);
      group.appendChild(label);
      visualAnnotationsFor(visual, source.id).forEach(function (annotation, noteIndex) {
        var note = svg("text", { x: pos.x, y: pos.y - 38 - noteIndex * 14, "text-anchor": "middle", class: "visual-svg-annotation-label" });
        note.textContent = runtime.safeText(visualAnnotationText(annotation));
        group.appendChild(note);
      });
      group.addEventListener("click", function () {
        shell.inspector.show(source.title || source.id, source);
      });
      canvas.appendChild(group);
    });
    claims.forEach(function (claim) {
      var pos = claimPos[claim.id];
      var spec = resolveMarkSpec(claim, markContext);
      var colorSpec = resolveColorSpec(claim, markContext);
      var color = normalizeColorString(spec.color || colorSpec.color);
      var group = svg("g", {
        class: "visual-node visual-evidence-node visual-evidence-claim visual-mark-shape-" + safeClass(spec.shape) + (focusIDs[claim.id] ? " visual-focused" : ""),
        "data-mark-shape": spec.shape || "",
        "data-mark-icon": spec.icon || "",
        "data-status": claim.status || ""
      });
      appendEvidenceShape(group, pos, 174, 70, spec, color, "claim");
      appendSvgIcon(group, spec, markContext, pos.x - 76, pos.y - 14, 22);
      var label = svg("text", { x: pos.x, y: pos.y - 2, "text-anchor": "middle" });
      label.textContent = runtime.safeText(claim.text || claim.id).slice(0, 34);
      var conf = svg("text", { x: pos.x, y: pos.y + 17, "text-anchor": "middle" });
      conf.textContent = "confidence " + runtime.safeText(claim.confidence);
      group.appendChild(label);
      group.appendChild(conf);
      visualAnnotationsFor(visual, claim.id).forEach(function (annotation, noteIndex) {
        var note = svg("text", { x: pos.x, y: pos.y - 48 - noteIndex * 14, "text-anchor": "middle", class: "visual-svg-annotation-label" });
        note.textContent = runtime.safeText(visualAnnotationText(annotation));
        group.appendChild(note);
      });
      group.addEventListener("click", function () {
        shell.inspector.show(claim.id, claim);
      });
      canvas.appendChild(group);
    });
  }

  function renderMatrix(ctx) {
    var data = ctx.data || {};
    var visual = readVisualHints(data);
    var focusIDs = idSet(visual.initialFocusIDs);
    var hiddenIDs = idSet(visual.hiddenDetailIDs);
    var items = (Array.isArray(data.items) ? data.items : []).filter(function (item) {
      return !item || !item.id || !hiddenIDs[item.id] || focusIDs[item.id];
    });
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    var markContext = createMarkContext(manifest, data);
    var colorBy = colorByFromVisual(visual);
    if (shouldShowSemanticLegend(visual)) {
      var legendSpec = buildLegendItems(data, { visual: visual, rawNodes: items, rawEdges: [] }, markContext);
      createLegendOverlay(shell.stage, legendSpec.title, legendSpec.items);
    }
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var board = el("div", "matrix-stage visual-matrix-3d");
    board.appendChild(el("div", "matrix-axis-y", "Impact"));
    board.appendChild(el("div", "matrix-axis-x", "Confidence"));
    items.forEach(function (item, index) {
      var x = typeof item.x === "number" ? item.x : 0.5;
      var y = typeof item.y === "number" ? item.y : 0.5;
      var z = item.metrics && Number.isFinite(Number(item.metrics.z || item.metrics.impact || item.metrics.risk)) ? Number(item.metrics.z || item.metrics.impact || item.metrics.risk) : index % 7;
      var card = el("article", "visual-card matrix-item" + (focusIDs[item.id] ? " visual-card-focus" : ""));
      card.style.left = Math.max(8, Math.min(92, x * 100)) + "%";
      card.style.top = Math.max(8, Math.min(92, (1 - y) * 100)) + "%";
      var zDepth = Math.max(0, Math.min(1, z > 1 ? z / 100 : z / 7));
      var markSpec = resolveMarkSpec(item, markContext);
      var colorSpec = colorSpecForPolicy(item, markContext, colorBy, markSpec);
      var markColor = normalizeColorString(colorSpec.color || markSpec.color);
      var iconPath = iconPathFor(markSpec, markContext);
      card.style.setProperty("--item-z-offset", Math.round(zDepth * 88) + "px");
      card.style.setProperty("--item-shadow-offset", Math.round(zDepth * 22) + "px");
      card.style.setProperty("--mark-color", markColor);
      card.setAttribute("data-mark-shape", markSpec.shape || "");
      card.setAttribute("data-mark-icon", markSpec.icon || "");
      card.setAttribute("data-mark-color", markColor);
      var markRow = el("div", "matrix-mark-row");
      var emblem = el("span", "visual-mark-emblem visual-mark-shape-" + safeClass(markSpec.shape));
      emblem.style.backgroundColor = markColor;
      emblem.setAttribute("aria-hidden", "true");
      if (iconPath) {
        var icon = document.createElement("img");
        icon.className = "visual-mark-icon";
        icon.src = iconPath;
        icon.alt = "";
        emblem.appendChild(icon);
      }
      var textBox = el("div", "matrix-mark-text");
      textBox.appendChild(el("div", "visual-card-title", item.label || item.id));
      textBox.appendChild(el("div", "visual-card-meta", [item.kind, item.provider && item.service ? item.provider + "." + item.service : item.provider || item.platform || item.service].filter(Boolean).join(" · ")));
      markRow.appendChild(emblem);
      markRow.appendChild(textBox);
      card.appendChild(markRow);
      var status = runtime.formatStatus(item.status || item.kind);
      card.appendChild(el("span", status.className, status.label));
      visualAnnotationsFor(visual, item.id).forEach(function (annotation) {
        card.appendChild(el("div", "visual-card-annotation", visualAnnotationText(annotation)));
      });
      card.addEventListener("click", function () {
        shell.inspector.show(item.label || item.id, item);
      });
      board.appendChild(card);
    });
    shell.stage.appendChild(board);
  }

  function orderedItems(items) {
    return (Array.isArray(items) ? items.slice() : []).sort(function (a, b) {
      var ao = Number(a && (a.order !== undefined ? a.order : a.index));
      var bo = Number(b && (b.order !== undefined ? b.order : b.index));
      if (!Number.isFinite(ao)) {
        ao = 0;
      }
      if (!Number.isFinite(bo)) {
        bo = 0;
      }
      if (ao === bo) {
        return itemLabel(a).localeCompare(itemLabel(b));
      }
      return ao - bo;
    });
  }

  function phaseColor(phase, index) {
    var colors = [0x63a9ff, 0x35c2a1, 0xa77cff, 0xe5a84c, 0xee6b73, 0xcbd5e1];
    if (!phase) {
      return colors[index % colors.length];
    }
    var text = String(phase);
    var hash = 0;
    for (var i = 0; i < text.length; i += 1) {
      hash = (hash * 31 + text.charCodeAt(i)) >>> 0;
    }
    return colors[hash % colors.length];
  }

  function colorStringFromHex(value) {
    return "#" + ("000000" + value.toString(16)).slice(-6);
  }

  function colorValue(value, fallback) {
    var text = String(value || "").trim();
    if (text.charAt(0) === "#") {
      text = text.slice(1);
    }
    if (/^[0-9a-fA-F]{6}$/.test(text)) {
      return parseInt(text, 16);
    }
    return fallback;
  }

  function sequenceOrderBounds(messages, activations, fragments) {
    var min = Infinity;
    var max = -Infinity;
    function read(value) {
      var n = Number(value);
      if (Number.isFinite(n)) {
        min = Math.min(min, n);
        max = Math.max(max, n);
      }
    }
    messages.forEach(function (message, index) {
      read(message.order !== undefined ? message.order : index + 1);
    });
    activations.forEach(function (activation) {
      read(activation.start_order);
      read(activation.end_order);
    });
    fragments.forEach(function (fragment) {
      read(fragment.start_order);
      read(fragment.end_order);
    });
    if (!Number.isFinite(min) || !Number.isFinite(max)) {
      return { min: 1, max: Math.max(2, messages.length || 2) };
    }
    return { min: min, max: Math.max(min + 1, max) };
  }

  function renderUMLSequenceFallback(ctx, shell) {
    var data = ctx.data || {};
    var participants = Array.isArray(data.participants) ? data.participants : [];
    var messages = orderedItems(data.messages);
    var bounds = sequenceOrderBounds(messages, data.activations || [], data.fragments || []);
    var width = 1180;
    var height = 720;
    var canvas = svg("svg", { class: "visual-svg visual-uml-fallback", viewBox: "0 0 " + width + " " + height, role: "img" });
    var xStep = width / Math.max(2, participants.length + 1);
    var yForOrder = function (order) {
      return 94 + ((Number(order) - bounds.min) / Math.max(1, bounds.max - bounds.min)) * (height - 160);
    };
    var positions = {};
    participants.forEach(function (participant, index) {
      var x = xStep * (index + 1);
      positions[participant.id] = x;
      var label = svg("text", { x: x, y: 42, "text-anchor": "middle", class: "visual-uml-svg-label" });
      label.textContent = runtime.safeText(participant.label || participant.name || participant.id);
      canvas.appendChild(label);
      canvas.appendChild(svg("line", { x1: x, y1: 70, x2: x, y2: height - 52, class: "visual-uml-svg-lifeline" }));
    });
    messages.forEach(function (message, index) {
      var from = positions[message.from];
      var to = positions[message.to];
      if (!from || !to) {
        return;
      }
      var y = yForOrder(message.order !== undefined ? message.order : index + 1);
      var color = colorStringFromHex(phaseColor(message.phase || message.kind, index));
      var path = svg("path", { d: "M " + from + " " + y + " C " + ((from + to) / 2) + " " + (y - 28) + " " + ((from + to) / 2) + " " + (y + 28) + " " + to + " " + y, class: "visual-uml-svg-message", stroke: color });
      canvas.appendChild(path);
      var text = svg("text", { x: (from + to) / 2, y: y - 9, "text-anchor": "middle", class: "visual-uml-svg-message-label" });
      text.textContent = runtime.safeText((message.order ? message.order + ". " : "") + (message.label || message.name || message.id));
      canvas.appendChild(text);
    });
    shell.stage.appendChild(canvas);
  }

  function renderUMLSequence(ctx) {
    var data = ctx.data || {};
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    decorateStage(shell.stage, manifest, data, preset);

    var participants = Array.isArray(data.participants) ? data.participants : [];
    var messages = orderedItems(data.messages);
    var activations = Array.isArray(data.activations) ? data.activations : [];
    var fragments = Array.isArray(data.fragments) ? data.fragments : [];
    var phases = Array.isArray(data.phases) ? data.phases : [];
    var visual = readVisualHints(data);
    var sequenceMarkContext = createMarkContext(manifest, data);
    var visualFocus = idSet(visual.initialFocusIDs);
    var visualHidden = idSet(visual.hiddenDetailIDs);
    messages = messages.filter(function (message) {
      return !message || !message.id || !visualHidden[message.id] || visualFocus[message.id];
    });
    var phaseByID = {};
    phases.forEach(function (phase, index) {
      var id = phase.id || phase.label || phase.name || "";
      if (id) {
        phaseByID[id] = { phase: phase, index: index };
      }
    });
    var phaseSelect = selectControl("All phases", phases.map(function (phase) {
      return phase.id || phase.label || phase.name || "";
    }).filter(Boolean));
    var resetBtn = document.createElement("button");
    resetBtn.textContent = "Reset";
    var replayBtn = document.createElement("button");
    replayBtn.textContent = "Replay";
    var exportBtn = document.createElement("button");
    exportBtn.textContent = "Export";
    shell.toolbar.appendChild(phaseSelect);
    shell.toolbar.appendChild(resetBtn);
    shell.toolbar.appendChild(replayBtn);
    shell.toolbar.appendChild(exportBtn);

    var THREE = window.THREE;
    if (!THREE || !THREE.WebGLRenderer) {
      renderUMLSequenceFallback(ctx, shell);
      return;
    }

    try {
      shell.stage.classList.add("visual-three-primary", "visual-uml-sequence-stage");
      var layer = el("div", "visual-three-layer visual-three-primary-layer visual-uml-sequence-layer");
      layer.setAttribute("role", "application");
      layer.setAttribute("aria-label", "Interactive 3D UML sequence diagram. Drag to orbit, wheel to zoom, click lifelines or messages to inspect details.");
      shell.stage.appendChild(layer);
      var width = Math.max(760, shell.stage.clientWidth || 1000);
      var height = Math.max(560, shell.stage.clientHeight || 680);
      var renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
      renderer.setClearColor(0x000000, 0);
      renderer.setPixelRatio(Math.min(2, window.devicePixelRatio || 1));
      renderer.setSize(width, height, false);
      if (THREE.SRGBColorSpace) {
        renderer.outputColorSpace = THREE.SRGBColorSpace;
      }
      layer.appendChild(renderer.domElement);
      var labelLayer = el("div", "visual-uml-label-layer");
      layer.appendChild(labelLayer);
      if (visual.showLegend && phases.length) {
        var legend = el("div", "visual-uml-phase-legend");
        legend.appendChild(el("div", "visual-uml-phase-title", "Phases"));
        phases.forEach(function (phase, index) {
          var color = colorValue(phase.color, sequenceColor(phase.id || phase.label || phase.name, index));
          var item = el("div", "visual-uml-phase-item");
          var swatch = el("span", "visual-uml-phase-swatch");
          swatch.style.backgroundColor = colorStringFromHex(color);
          item.appendChild(swatch);
          item.appendChild(el("span", "", phase.label || phase.name || phase.id || ("Phase " + (index + 1))));
          legend.appendChild(item);
        });
        layer.appendChild(legend);
      }

      var scene = new THREE.Scene();
      var camera = new THREE.PerspectiveCamera(44, width / height, 0.1, 160);
      var orbit = { theta: -0.18, phi: 1.08, radius: 10.5, target: new THREE.Vector3(0, 0, 0) };
      scene.add(new THREE.AmbientLight(0xffffff, 0.56));
      var key = new THREE.DirectionalLight(0xa5dcff, 1.35);
      key.position.set(3.5, 6.5, 5.4);
      scene.add(key);
      var fill = new THREE.DirectionalLight(0x35c2a1, 0.7);
      fill.position.set(-5, 0.5, -3.5);
      scene.add(fill);
      var root = new THREE.Group();
      scene.add(root);
      addThreeGrid(THREE, root, 10, 18, -3.1, 0x2d4254);

      var bounds = sequenceOrderBounds(messages, activations, fragments);
      var participantPositions = {};
      var labels = [];
      var objects = [];
      var pickables = [];
      var basePickables = [];
      var participantCount = Math.max(1, participants.length);
      var xSpacing = Math.min(1.72, Math.max(0.92, 9 / Math.max(4, participantCount)));
      var sequenceHeight = 5.6;

      function yForOrder(order) {
        var n = Number(order);
        if (!Number.isFinite(n)) {
          n = bounds.min;
        }
        return 2.7 - ((n - bounds.min) / Math.max(1, bounds.max - bounds.min)) * sequenceHeight;
      }

      function addHTMLLabel(text, world, className, payload) {
        var label = el("div", "visual-uml-label " + (className || ""), text);
        labelLayer.appendChild(label);
        labels.push({ node: label, world: world.clone(), payload: payload });
        return label;
      }

      var sequenceLabelEngine = createLabelEngine({ mode: visual.labelMode, focusIDs: visualFocus });

      function sequenceColor(value, index) {
        var entry = phaseByID[String(value || "")];
        if (entry && entry.phase) {
          return colorValue(entry.phase.color, phaseColor(entry.phase.id || entry.phase.label || value, entry.index));
        }
        return phaseColor(value, index);
      }

      function participantLabel(participant) {
        return participant.display_name || participant.label || participant.name || participant.id;
      }

      function participantPosition(participant, index) {
        var center = (participantCount - 1) / 2;
        var lane = Number(participant.lane_index);
        if (!Number.isFinite(lane)) {
          lane = index;
        }
        var x = (lane - center) * xSpacing;
        var depth = Number(participant.depth);
        var z = Number.isFinite(depth) ? depth : ((index % 3) - 1) * 0.58 + (index % 2 ? 0.22 : -0.12);
        return new THREE.Vector3(x, 0, z);
      }

      participants.forEach(function (participant, index) {
        var pos = participantPosition(participant, index);
        participantPositions[participant.id] = pos;
        var color = colorValue(participant.color, phaseColor(participant.kind || participant.group || participant.id, index));
        var geometry = THREE.CylinderGeometry ? new THREE.CylinderGeometry(0.065, 0.065, sequenceHeight + 0.85, 18) : new THREE.BoxGeometry(0.14, sequenceHeight + 0.85, 0.14);
        var material = new THREE.MeshPhysicalMaterial({
          color: color,
          emissive: color,
          emissiveIntensity: 0.22,
          metalness: 0.2,
          roughness: 0.32,
          transparent: true,
          opacity: 0.88,
          clearcoat: 0.42
        });
        var mesh = new THREE.Mesh(geometry, material);
        mesh.position.set(pos.x, 0, pos.z);
        mesh.userData = { label: participant.label || participant.name || participant.id, payload: participant };
        root.add(mesh);
        objects.push(mesh);
        pickables.push(mesh);
        basePickables.push(mesh);
        var label = addHTMLLabel(participantLabel(participant), new THREE.Vector3(pos.x, 3.15, pos.z), "visual-uml-participant-label" + (visualFocus[participant.id] ? " visual-uml-focus-label" : ""), { __static_label: true, payload: participant });
        if (participant.subtitle) {
          label.appendChild(el("span", "visual-uml-participant-subtitle", participant.subtitle));
        }
        visualAnnotationsFor(visual, participant.id).forEach(function (annotation) {
          addHTMLLabel(visualAnnotationText(annotation), new THREE.Vector3(pos.x, 2.72, pos.z + 0.22), "visual-uml-annotation-label", annotation);
        });
      });

      activations.forEach(function (activation, index) {
        var base = participantPositions[activation.participant_id];
        if (!base) {
          return;
        }
        var startY = yForOrder(activation.start_order);
        var endY = yForOrder(activation.end_order);
        var length = Math.max(0.22, Math.abs(endY - startY));
        var color = sequenceColor(activation.phase || activation.kind || activation.participant_id, index);
        var box = new THREE.Mesh(new THREE.BoxGeometry(0.18, length, 0.13), new THREE.MeshPhysicalMaterial({
          color: color,
          emissive: color,
          emissiveIntensity: 0.25,
          roughness: 0.3,
          transparent: true,
          opacity: 0.72
        }));
        box.position.set(base.x + 0.12, (startY + endY) / 2, base.z + 0.04);
        box.userData = { label: activation.label || activation.id || "activation", payload: activation };
        root.add(box);
        pickables.push(box);
        basePickables.push(box);
      });

      var messageRoot = new THREE.Group();
      root.add(messageRoot);
      var selectedPhase = "";
      function rebuildMessages(replayProgress) {
        messageRoot.children.slice().forEach(function (child) {
          messageRoot.remove(child);
          disposeThreeObject(child);
        });
        pickables = basePickables.slice();
        labels = labels.filter(function (label) {
          var payload = label.payload || {};
          var keep = payload.__static_label === true || payload.type === "fragment";
          if (!keep && label.node.parentNode) {
            label.node.parentNode.removeChild(label.node);
          }
          return keep;
        });
        messages.forEach(function (message, index) {
          if (selectedPhase && String(message.phase || "") !== selectedPhase) {
            return;
          }
          if (replayProgress !== undefined && index > replayProgress) {
            return;
          }
          var from = participantPositions[message.from];
          var to = participantPositions[message.to];
          if (!from || !to) {
            return;
          }
          var order = message.order !== undefined ? message.order : index + 1;
          var y = yForOrder(order);
          var edgeSpec = resolveEdgeSpec(message, sequenceMarkContext);
          var phaseBasedColor = sequenceColor(message.phase || message.kind || message.status, index);
          var color = colorValue(edgePresentation(message).color || message.color, colorValue(edgeSpec.color, phaseBasedColor));
          var a = new THREE.Vector3(from.x, y, from.z);
          var b = new THREE.Vector3(to.x, y, to.z);
          if (message.self === true || message.from === message.to) {
            b = new THREE.Vector3(from.x + 0.62, y - 0.28, from.z + 0.45);
          }
          var mid = a.clone().lerp(b, 0.5);
          var curveKind = String(message.curve || message.kind || "").toLowerCase();
          mid.y += curveKind === "return" ? -0.1 : 0.22 + importanceValue(message, 0) * 0.28;
          if (curveKind === "high_arc" || curveKind === "arc") {
            mid.y += 0.42;
            mid.z += 0.34;
          } else if (curveKind === "loop" || curveKind === "self") {
            mid.x += 0.32;
            mid.z += 0.52;
          }
          mid.z += Number(message.depth || 0) || 0;
          var geometry = new THREE.BufferGeometry();
          var routePoints = [a, mid, b];
          if (THREE.CatmullRomCurve3) {
            routePoints = new THREE.CatmullRomCurve3(routePoints).getPoints(18);
          }
          geometry.setFromPoints(routePoints);
          var line = new THREE.Line(geometry, new THREE.LineBasicMaterial({
            color: color,
            transparent: true,
            opacity: visualFocus[message.id] ? 0.98 : message.kind === "return" ? 0.58 : 0.88,
            linewidth: 2
          }));
          line.userData = { label: message.label || message.name || message.id, payload: message };
          messageRoot.add(line);
          pickables.push(line);
          var direction = b.clone().sub(mid).normalize();
          if (edgeSpec.arrow !== "none") {
            var coneGeometry = THREE.ConeGeometry ? new THREE.ConeGeometry(0.08, 0.24, 18) : new THREE.IcosahedronGeometry(0.08, 1);
            var cone = new THREE.Mesh(coneGeometry, new THREE.MeshBasicMaterial({ color: color }));
            cone.position.copy(edgeSpec.arrow === "reverse" ? a : b);
            cone.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), edgeSpec.arrow === "reverse" ? direction.clone().multiplyScalar(-1) : direction);
            cone.userData = line.userData;
            messageRoot.add(cone);
            pickables.push(cone);
          }
          if (edgeSpec.flow && THREE.SphereGeometry) {
            var flowParticle = new THREE.Mesh(new THREE.SphereGeometry(0.04, 10, 8), new THREE.MeshBasicMaterial({ color: color, transparent: true, opacity: 0.72, blending: THREE.AdditiveBlending }));
            flowParticle.position.copy(mid);
            flowParticle.userData = line.userData;
            messageRoot.add(flowParticle);
            pickables.push(flowParticle);
          }
          var messageLabel = addHTMLLabel((message.order ? message.order + ". " : "") + (message.label || message.name || message.id), mid, "visual-uml-message-label" + (visualFocus[message.id] ? " visual-uml-focus-label" : ""), message);
          visualAnnotationsFor(visual, message.id).forEach(function (annotation) {
            addHTMLLabel(visualAnnotationText(annotation), mid.clone().add(new THREE.Vector3(0, 0.32, 0.16)), "visual-uml-annotation-label", annotation);
          });
          if (message.summary && visualFocus[message.id]) {
            messageLabel.setAttribute("title", String(message.summary));
          }
        });
      }

      fragments.forEach(function (fragment, index) {
        var y = (yForOrder(fragment.start_order) + yForOrder(fragment.end_order)) / 2;
        var x = -((participantCount - 1) * xSpacing) / 2 - 0.42;
        var color = phaseColor(fragment.kind || fragment.label || fragment.id, index);
        addHTMLLabel((fragment.kind || "fragment") + (fragment.condition ? " · " + fragment.condition : ""), new THREE.Vector3(x, y, -1.05), "visual-uml-fragment-label", { __static_label: true, type: "fragment", payload: fragment });
        var band = new THREE.Mesh(new THREE.BoxGeometry(participantCount * xSpacing + 0.9, 0.035, 1.05), new THREE.MeshBasicMaterial({
          color: color,
          transparent: true,
          opacity: 0.12
        }));
        band.position.set(0, y, -0.7);
        root.add(band);
      });

      rebuildMessages();

      var pointer = new THREE.Vector2();
      var raycaster = new THREE.Raycaster();
      raycaster.params.Line = { threshold: 0.14 };
      var dragging = false;
      var lastX = 0;
      var lastY = 0;
      function updateCamera() {
        orbit.phi = Math.max(0.24, Math.min(Math.PI - 0.2, orbit.phi));
        orbit.radius = Math.max(5.2, Math.min(24, orbit.radius));
        var sinPhi = Math.sin(orbit.phi);
        camera.position.set(
          orbit.target.x + orbit.radius * sinPhi * Math.sin(orbit.theta),
          orbit.target.y + orbit.radius * Math.cos(orbit.phi),
          orbit.target.z + orbit.radius * sinPhi * Math.cos(orbit.theta)
        );
        camera.lookAt(orbit.target);
      }
      function updateLabels() {
        labels.forEach(function (label) {
          var p = label.world.clone();
          p.applyMatrix4(root.matrixWorld);
          p.project(camera);
          var visible = p.z >= -1 && p.z <= 1;
          label.node.style.display = visible ? "block" : "none";
          label.node.style.left = ((p.x * 0.5 + 0.5) * width).toFixed(1) + "px";
          label.node.style.top = ((-p.y * 0.5 + 0.5) * height).toFixed(1) + "px";
        });
      }
      function resize() {
        width = Math.max(760, shell.stage.clientWidth || width);
        height = Math.max(560, shell.stage.clientHeight || height);
        camera.aspect = width / height;
        camera.updateProjectionMatrix();
        renderer.setSize(width, height, false);
      }
      renderer.domElement.addEventListener("pointerdown", function (event) {
        dragging = true;
        lastX = event.clientX;
        lastY = event.clientY;
        renderer.domElement.setPointerCapture(event.pointerId);
      });
      renderer.domElement.addEventListener("pointermove", function (event) {
        if (!dragging) {
          return;
        }
        var dx = event.clientX - lastX;
        var dy = event.clientY - lastY;
        lastX = event.clientX;
        lastY = event.clientY;
        orbit.theta -= dx * 0.006;
        orbit.phi += dy * 0.005;
      });
      renderer.domElement.addEventListener("pointerup", function (event) {
        dragging = false;
        try {
          renderer.domElement.releasePointerCapture(event.pointerId);
        } catch (ignore) {
        }
      });
      renderer.domElement.addEventListener("wheel", function (event) {
        event.preventDefault();
        orbit.radius *= event.deltaY > 0 ? 1.08 : 0.92;
      }, { passive: false });
      renderer.domElement.addEventListener("click", function (event) {
        var rect = renderer.domElement.getBoundingClientRect();
        pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
        raycaster.setFromCamera(pointer, camera);
        var hits = raycaster.intersectObjects(pickables, false);
        if (hits.length && hits[0].object && hits[0].object.userData) {
          shell.inspector.show(hits[0].object.userData.label, hits[0].object.userData.payload);
        }
      });
      var observer = typeof ResizeObserver !== "undefined" ? new ResizeObserver(resize) : null;
      if (observer) {
        observer.observe(shell.stage);
      }
      var replayIndex = null;
      replayBtn.addEventListener("click", function () {
        replayIndex = 0;
        rebuildMessages(replayIndex);
      });
      resetBtn.addEventListener("click", function () {
        orbit.theta = -0.18;
        orbit.phi = 1.08;
        orbit.radius = 10.5;
        replayIndex = null;
        rebuildMessages();
      });
      phaseSelect.addEventListener("change", function () {
        selectedPhase = phaseSelect.value;
        replayIndex = null;
        rebuildMessages();
      });
      exportBtn.addEventListener("click", function () {
        runtime.exportJSON(data, "visual-data.json");
      });
      var start = Date.now();
      function animate() {
        if (!document.body.contains(shell.stage)) {
          if (observer) {
            observer.disconnect();
          }
          renderer.dispose();
          return;
        }
        if (replayIndex !== null) {
          var elapsed = (Date.now() - start) / 520;
          var next = Math.min(messages.length - 1, Math.floor(elapsed));
          if (next !== replayIndex) {
            replayIndex = next;
            rebuildMessages(replayIndex);
          }
          if (replayIndex >= messages.length - 1) {
            replayIndex = null;
          }
        }
        updateCamera();
        var t = (Date.now() - start) / 1000;
        root.rotation.y += (Math.sin(t * 0.18) * 0.03 - root.rotation.y) * 0.02;
        renderer.render(scene, camera);
        updateLabels();
        window.requestAnimationFrame(animate);
      }
      animate();
    } catch (err) {
      shell.stage.setAttribute("data-three-error", err && err.message ? err.message : String(err));
      renderUMLSequenceFallback(ctx, shell);
    }
  }

  function cloneManifestWith(manifest, renderer, preset) {
    var copy = JSON.parse(JSON.stringify(manifest || {}));
    copy.renderer = copy.renderer || {};
    copy.renderer.contract = renderer;
    copy.layout = copy.layout || {};
    copy.layout.preset = preset;
    copy.effects = copy.effects || {};
    copy.effects.engine = "three.v1";
    copy.effects.scene = copy.effects.scene || preset;
    copy.effects.camera = copy.effects.camera || "orbit";
    copy.effects.particles = copy.effects.particles || "ambient_dust";
    copy.effects.material = copy.effects.material || "holographic";
    copy.effects.motion = copy.effects.motion || "slow_orbit";
    copy.visual_design = copy.visual_design || {};
    copy.visual_design.default_collapse_depth = copy.visual_design.default_collapse_depth || 0;
    copy.visual_design.max_initial_nodes = copy.visual_design.max_initial_nodes || 80;
    copy.visual_design.max_initial_edges = copy.visual_design.max_initial_edges || 160;
    return copy;
  }

  function graphFromUMLClass(data) {
    var nodes = (Array.isArray(data.classes) ? data.classes : []).map(function (klass) {
      return {
        id: klass.id,
        label: klass.name || klass.label || klass.id,
        kind: klass.stereotype || "class",
        status: klass.status || "ok",
        group: klass.package || klass.module || "classes",
        metadata: { attributes: klass.attributes || [], operations: klass.operations || [], responsibility: klass.responsibility || "" },
        metrics: { members: (klass.attributes || []).length + (klass.operations || []).length, importance: klass.importance || 0.5 }
      };
    });
    var edges = (Array.isArray(data.relationships) ? data.relationships : []).map(function (rel) {
      return {
        from: rel.from,
        to: rel.to,
        kind: rel.kind || "relationship",
        label: rel.label || rel.kind || "",
        status: rel.status || "ok",
        weight: rel.weight || 1,
        metadata: rel.metadata || {}
      };
    });
    return { schema: "efp.visual.input.graph.v1", title: data.title, nodes: nodes, edges: edges };
  }

  function graphFromUMLState(data) {
    var nodes = (Array.isArray(data.states) ? data.states : []).map(function (state) {
      return {
        id: state.id,
        label: state.label || state.name || state.id,
        kind: state.kind || "state",
        status: state.status || "ok",
        group: state.region || "state-machine",
        metadata: { entry: state.entry || "", exit: state.exit || "" },
        metrics: state.metrics || {}
      };
    });
    var edges = (Array.isArray(data.transitions) ? data.transitions : []).map(function (transition) {
      return {
        from: transition.from,
        to: transition.to,
        kind: transition.kind || "transition",
        label: transition.trigger || transition.label || "",
        status: transition.status || "ok",
        weight: transition.weight || 1,
        metadata: { guard: transition.guard || "", action: transition.action || "" }
      };
    });
    return { schema: "efp.visual.input.graph.v1", title: data.title, nodes: nodes, edges: edges };
  }

  function graphFromUMLActivity(data) {
    var laneByID = {};
    (Array.isArray(data.lanes) ? data.lanes : []).forEach(function (lane) {
      laneByID[lane.id] = lane;
    });
    var nodes = (Array.isArray(data.actions) ? data.actions : []).map(function (action) {
      var lane = laneByID[action.lane_id] || {};
      return {
        id: action.id,
        label: action.label || action.name || action.id,
        kind: action.kind || "action",
        status: action.status || "ok",
        group: lane.label || lane.name || action.lane_id || "activity",
        metadata: action.metadata || {},
        metrics: action.metrics || {}
      };
    });
    var edges = (Array.isArray(data.flows) ? data.flows : []).map(function (flow) {
      return {
        from: flow.from,
        to: flow.to,
        kind: flow.kind || "flow",
        label: flow.label || flow.condition || "",
        status: flow.status || "ok",
        weight: flow.weight || 1,
        metadata: flow.metadata || {}
      };
    });
    return { schema: "efp.visual.input.graph.v1", title: data.title, nodes: nodes, edges: edges };
  }

  function graphFromUMLComponent(data) {
    var nodes = [];
    (Array.isArray(data.deployments) ? data.deployments : []).forEach(function (deployment) {
      nodes.push({
        id: deployment.id,
        label: deployment.label || deployment.name || deployment.id,
        kind: deployment.kind || "deployment",
        status: deployment.status || "ok",
        group: "deployments",
        metadata: deployment.metadata || {},
        metrics: deployment.metrics || {}
      });
    });
    (Array.isArray(data.components) ? data.components : []).forEach(function (component) {
      nodes.push({
        id: component.id,
        label: component.label || component.name || component.id,
        kind: component.kind || "component",
        status: component.status || "ok",
        group: component.deployment_id || component.layer || "components",
        parent_id: component.deployment_id || "",
        metadata: { interfaces: component.interfaces || [], responsibilities: component.responsibilities || [] },
        metrics: component.metrics || {}
      });
    });
    var edges = (Array.isArray(data.links) ? data.links : []).map(function (link) {
      return {
        from: link.from,
        to: link.to,
        kind: link.kind || "link",
        label: link.label || link.protocol || link.kind || "",
        status: link.status || "ok",
        weight: link.weight || 1,
        metadata: link.metadata || {}
      };
    });
    return { schema: "efp.visual.input.graph.v1", title: data.title, nodes: nodes, edges: edges };
  }

  function renderUMLClass(ctx) {
    renderGraph({ container: ctx.container, manifest: cloneManifestWith(ctx.manifest, "offline.graph.v1", "class_cards"), data: graphFromUMLClass(ctx.data || {}) });
  }

  function renderUMLState(ctx) {
    renderGraph({ container: ctx.container, manifest: cloneManifestWith(ctx.manifest, "offline.graph.v1", "state_machine"), data: graphFromUMLState(ctx.data || {}) });
  }

  function renderUMLActivity(ctx) {
    renderGraph({ container: ctx.container, manifest: cloneManifestWith(ctx.manifest, "offline.graph.v1", "activity_swimlanes"), data: graphFromUMLActivity(ctx.data || {}) });
  }

  function renderUMLComponent(ctx) {
    renderGraph({ container: ctx.container, manifest: cloneManifestWith(ctx.manifest, "offline.graph.v1", "component_deployment"), data: graphFromUMLComponent(ctx.data || {}) });
  }

  function isometricArray(data, field) {
    return Array.isArray(data && data[field]) ? data[field].filter(function (item) { return item && typeof item === "object"; }) : [];
  }

  function isometricBounds(zone) {
    var bounds = zone && zone.bounds && typeof zone.bounds === "object" ? zone.bounds : {};
    return {
      x: numberValue(bounds.x, 0),
      y: numberValue(bounds.y, 0),
      w: Math.max(1, numberValue(bounds.w, 4)),
      h: Math.max(1, numberValue(bounds.h, 3))
    };
  }

  function isometricPosition(item, zone, index) {
    var position = item && item.position && typeof item.position === "object" ? item.position : {};
    if (Number.isFinite(Number(position.x)) && Number.isFinite(Number(position.y))) {
      return { x: Number(position.x), y: Number(position.y), auto: false };
    }
    var bounds = isometricBounds(zone || {});
    var col = index % 4;
    var row = Math.floor(index / 4);
    return {
      x: bounds.x + Math.min(bounds.w - 0.7, 1 + col * Math.max(1.2, bounds.w / 4)),
      y: bounds.y + Math.min(bounds.h - 0.7, 1 + row * 1.35),
      auto: true
    };
  }

  function isometricSize(item) {
    var size = item && item.size && typeof item.size === "object" ? item.size : {};
    return {
      w: Math.max(0.55, numberValue(size.w, 1.35)),
      d: Math.max(0.55, numberValue(size.d, 1.05)),
      h: Math.max(0.35, numberValue(size.h, 0.85))
    };
  }

  function isometricWorld(point, scale, center) {
    return {
      x: (point.x - center.x) * scale,
      z: (point.y - center.y) * scale
    };
  }

  function createIsometricButton(label, action, handler) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "visual-isometric-control";
    button.setAttribute("data-action", action);
    button.textContent = label;
    button.addEventListener("click", handler);
    return button;
  }

  function createIsometricShell(container, manifest, data) {
    container.textContent = "";
    var app = el("div", "visual-isometric-app");
    app.setAttribute("data-isometric-renderer", "true");
    app.setAttribute("data-architecture-light", "true");
    var header = el("header", "visual-isometric-header");
    header.appendChild(el("h1", "visual-isometric-title", data.title || manifest.title || "Isometric Architecture"));
    header.appendChild(el("div", "visual-isometric-subtitle", data.subtitle || data.goal || "Offline Three.js architecture overview"));
    var controls = el("div", "visual-isometric-toolbar");
    var body = el("main", "visual-isometric-body");
    var stage = el("section", "visual-isometric-stage");
    stage.setAttribute("role", "application");
    stage.setAttribute("aria-label", "Interactive isometric architecture scene");
    var labelLayer = el("div", "visual-isometric-label-layer");
    var inspector = el("aside", "visual-isometric-inspector visual-inspector");
    inspector.setAttribute("aria-label", "Architecture inspector");
    stage.appendChild(labelLayer);
    body.appendChild(stage);
    body.appendChild(inspector);
    app.appendChild(header);
    app.appendChild(controls);
    app.appendChild(body);
    container.appendChild(app);
    return { app: app, controls: controls, stage: stage, labelLayer: labelLayer, inspector: runtime.createInspector(inspector) };
  }

  function isometricEntityGeometry(THREE, item, spec, size) {
    var kind = normalizeMarkKey(item.kind || item.type || spec.shape || spec.mesh);
    var mesh = normalizeMarkKey(spec.mesh || spec.shape);
    if (kind === "cdn") {
      return new THREE.SphereGeometry(Math.max(size.w, size.d) * 0.18, 28, 18);
    }
    if (kind === "database" || kind === "mysql" || kind === "postgres" || kind === "mongodb" || kind === "redis" || kind === "cache" || mesh === "cylinder" || mesh === "database_cylinder" || mesh === "bucket" || mesh === "stacked_cylinder") {
      return isometricCylinderGeometry(THREE, size.w * 0.18, size.h * 0.42, 30);
    }
    if (kind === "queue" || kind === "event_stream" || kind === "kafka" || kind === "rocketmq" || kind === "rabbitmq" || kind === "registry" || mesh === "capsule") {
      if (THREE.CapsuleGeometry) {
        return new THREE.CapsuleGeometry(size.d * 0.16, size.w * 0.25, 8, 18);
      }
      return isometricCylinderGeometry(THREE, size.d * 0.16, size.w * 0.5, 20);
    }
    if (kind === "kubernetes" || kind === "cluster" || mesh === "cluster") {
      return new THREE.BoxGeometry(size.w * 0.42, size.h * 0.5, size.d * 0.36);
    }
    if (kind === "ingress" || kind === "load_balancer" || mesh === "gateway_card" || mesh === "tower") {
      return new THREE.BoxGeometry(size.w * 0.36, size.h * 0.62, size.d * 0.26);
    }
    if (kind === "mobile") {
      return new THREE.BoxGeometry(size.w * 0.18, size.h * 0.52, size.d * 0.08);
    }
    if (kind === "decision" || mesh === "octahedron" || mesh === "diamond") {
      return new THREE.OctahedronGeometry(size.w * 0.2, 0);
    }
    if (kind === "risk" || kind === "warning" || mesh === "cone") {
      return isometricConeGeometry(THREE, size.w * 0.18, size.h * 0.5, 5);
    }
    return new THREE.BoxGeometry(size.w * 0.32, size.h * 0.42, size.d * 0.32);
  }

  function isometricCylinderGeometry(THREE, radius, height, segments) {
    if (THREE.CylinderGeometry) {
      return new THREE.CylinderGeometry(radius, radius, height, segments || 24);
    }
    return new THREE.BoxGeometry(radius * 1.8, height, radius * 1.8);
  }

  function isometricConeGeometry(THREE, radius, height, segments) {
    if (THREE.ConeGeometry) {
      return new THREE.ConeGeometry(radius, height, segments || 8);
    }
    return new THREE.IcosahedronGeometry(Math.max(radius, height * 0.28), 1);
  }

  function isometricDetailMaterial(THREE, color, opacity) {
    return new THREE.MeshStandardMaterial({
      color: colorValue(color, 0xffffff),
      roughness: 0.5,
      metalness: 0.12,
      transparent: opacity !== undefined && opacity < 1,
      opacity: opacity === undefined ? 1 : opacity
    });
  }

  function addIsometricBoxDetail(THREE, group, w, h, d, x, y, z, color, opacity) {
    var detail = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), isometricDetailMaterial(THREE, color, opacity));
    detail.position.set(x || 0, y || 0, z || 0);
    group.add(detail);
    return detail;
  }

  function addIsometricCylinderDetail(THREE, group, radius, height, x, y, z, color, opacity, segments) {
    var detail = new THREE.Mesh(isometricCylinderGeometry(THREE, radius, height, segments || 24), isometricDetailMaterial(THREE, color, opacity));
    detail.position.set(x || 0, y || 0, z || 0);
    group.add(detail);
    return detail;
  }

  function decorateIsometricEntity(THREE, group, item, spec, size, color) {
    var kind = normalizeMarkKey(item.kind || item.type || spec.shape || spec.mesh);
    var mesh = normalizeMarkKey(spec.mesh || spec.shape);
    if (kind === "pc" || kind === "client") {
      addIsometricBoxDetail(THREE, group, size.w * 0.34, size.h * 0.27, size.d * 0.045, 0, size.h * 0.22, size.d * 0.2, "#111827", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.06, size.h * 0.16, size.d * 0.04, 0, size.h * 0.02, size.d * 0.17, "#6b7280", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.23, size.h * 0.035, size.d * 0.16, 0, -size.h * 0.08, size.d * 0.1, "#e5e7eb", 1);
      return;
    }
    if (kind === "mobile") {
      addIsometricBoxDetail(THREE, group, size.w * 0.13, size.h * 0.36, size.d * 0.025, 0, size.h * 0.04, size.d * 0.08, "#111827", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.09, size.h * 0.28, size.d * 0.028, 0, size.h * 0.04, size.d * 0.1, "#dbeafe", 0.9);
      return;
    }
    if (kind === "database" || kind === "mysql" || kind === "postgres" || kind === "mongodb" || kind === "redis" || kind === "cache") {
      addIsometricCylinderDetail(THREE, group, size.w * 0.19, 0.035, 0, size.h * 0.22, 0, "#ffffff", 0.54, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.19, 0.025, 0, size.h * 0.04, 0, "#ffffff", 0.28, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.19, 0.025, 0, -size.h * 0.12, 0, "#111827", 0.16, 30);
      return;
    }
    if (kind === "storage" || kind === "oss" || kind === "minio" || mesh === "bucket") {
      addIsometricCylinderDetail(THREE, group, size.w * 0.2, 0.04, 0, size.h * 0.2, 0, "#ffffff", 0.72, 28);
      addIsometricCylinderDetail(THREE, group, size.w * 0.13, 0.035, 0, size.h * 0.24, 0, "#e0f2fe", 0.78, 28);
      return;
    }
    if (kind === "file_storage" || kind === "block_storage") {
      for (var i = 0; i < 3; i += 1) {
        addIsometricBoxDetail(THREE, group, size.w * 0.36, size.h * 0.075, size.d * 0.3, 0, -size.h * 0.12 + i * size.h * 0.12, 0, i % 2 ? "#5b6ee1" : "#4661d9", 1);
      }
      addIsometricBoxDetail(THREE, group, size.w * 0.12, size.h * 0.025, size.d * 0.04, 0, size.h * 0.2, size.d * 0.16, "#dbeafe", 0.92);
      return;
    }
    if (kind === "service" || kind === "microservice" || kind === "api" || kind === "api_gateway" || kind === "gateway" || kind === "nginx" || kind === "ingress" || kind === "load_balancer") {
      addIsometricBoxDetail(THREE, group, size.w * 0.28, size.h * 0.04, size.d * 0.22, 0, size.h * 0.19, size.d * 0.17, "#ffffff", 0.45);
      addIsometricBoxDetail(THREE, group, size.w * 0.08, size.h * 0.025, size.d * 0.03, -size.w * 0.1, size.h * 0.07, size.d * 0.18, "#fbbf24", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.08, size.h * 0.025, size.d * 0.03, size.w * 0.08, size.h * 0.07, size.d * 0.18, "#60a5fa", 1);
      if (kind === "api_gateway" || kind === "gateway" || kind === "nginx" || kind === "ingress" || kind === "load_balancer") {
        addIsometricBoxDetail(THREE, group, size.w * 0.18, size.h * 0.18, size.d * 0.035, 0, size.h * 0.02, size.d * 0.2, "#111827", 0.86);
      }
      return;
    }
    if (kind === "registry" || kind === "nacos" || kind === "event_stream" || kind === "queue" || kind === "kafka" || kind === "rocketmq" || kind === "rabbitmq") {
      addIsometricCylinderDetail(THREE, group, size.w * 0.24, 0.035, 0, -size.h * 0.22, 0, "#ffffff", 0.88, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.16, 0.025, 0, -size.h * 0.18, 0, "#bfdbfe", 0.76, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.055, 0.06, -size.w * 0.08, size.h * 0.18, size.d * 0.02, "#22d3ee", 1, 14);
      addIsometricCylinderDetail(THREE, group, size.w * 0.055, 0.06, size.w * 0.08, size.h * 0.18, size.d * 0.02, "#22d3ee", 1, 14);
      return;
    }
    if (kind === "log" || kind === "search" || kind === "elasticsearch") {
      addIsometricBoxDetail(THREE, group, size.w * 0.24, size.h * 0.03, size.d * 0.22, 0, size.h * 0.19, 0, "#fef3c7", 0.84);
    }
  }

  function createIsometricEntity(THREE, item, spec, size, markContext) {
    var color = colorValue(spec.color, 0x2f80ed);
    var group = new THREE.Group();
    var material = new THREE.MeshStandardMaterial({
      color: color,
      roughness: 0.58,
      metalness: 0.08,
      emissive: color,
      emissiveIntensity: 0.02
    });
    var mesh = new THREE.Mesh(isometricEntityGeometry(THREE, item, spec, size), material);
    mesh.castShadow = false;
    mesh.receiveShadow = true;
    group.add(mesh);
    decorateIsometricEntity(THREE, group, item, spec, size, color);
    var modelBadge = createModelBadge(THREE, spec, item, size, markContext);
    if (modelBadge) {
      group.add(modelBadge);
    }
    var settings = badgeSettings(markContext);
    var icon = createIconBillboard(spec, item, THREE, markContext, size);
    if (icon) {
      if (modelBadge && settings.placement === "front") {
        icon.position.y += size.h * 0.18;
        icon.position.z += size.d * 0.12;
      }
      group.add(icon);
    }
    group.userData = { label: itemLabel(item), payload: item, id: item.id, mark: { icon: spec.icon || "", model: spec.model || "", modelPath: modelPathFor(spec, markContext) || "" } };
    group.traverse(function (child) {
      child.userData = Object.assign({}, group.userData, child.userData || {});
    });
    return group;
  }

  function isometricLineMaterial(THREE, color, opacity) {
    return new THREE.LineBasicMaterial({ color: colorValue(color, 0x1f2937), transparent: true, opacity: opacity === undefined ? 0.8 : opacity });
  }

  function createDashedPolyline(THREE, points, color, opacity, dashLength, gapLength) {
    var dashed = new THREE.Group();
    var material = isometricLineMaterial(THREE, color || "#111827", opacity === undefined ? 0.86 : opacity);
    var dash = Math.max(0.03, dashLength || 0.14);
    var gap = Math.max(0.02, gapLength || 0.09);
    for (var i = 0; i < points.length - 1; i += 1) {
      var start = points[i];
      var end = points[i + 1];
      var delta = end.clone().sub(start);
      var length = delta.length();
      if (length < 0.001) continue;
      var direction = delta.clone().normalize();
      var cursor = 0;
      while (cursor < length) {
        var segStart = start.clone().add(direction.clone().multiplyScalar(cursor));
        var segEnd = start.clone().add(direction.clone().multiplyScalar(Math.min(length, cursor + dash)));
        var segment = new THREE.Line(new THREE.BufferGeometry().setFromPoints([segStart, segEnd]), material.clone());
        segment.userData.isDashedSegment = true;
        dashed.add(segment);
        cursor += dash + gap;
      }
    }
    return dashed;
  }

  function createVerticalDashedLeader(THREE, x, z, yStart, yEnd) {
    var leader = new THREE.Group();
    var from = Math.min(yStart, yEnd);
    var to = Math.max(yStart, yEnd);
    var material = new THREE.MeshBasicMaterial({ color: 0x111827, transparent: true, opacity: 0.74 });
    var radius = 0.008;
    var dash = 0.09;
    var gap = 0.07;
    for (var y = from; y < to; y += dash + gap) {
      var height = Math.min(dash, to - y);
      if (height <= 0.002) continue;
      var geometry = isometricCylinderGeometry(THREE, radius, height, 8);
      var segment = new THREE.Mesh(geometry, material.clone());
      segment.position.set(x, y + height / 2, z);
      segment.userData.isLeaderLine = true;
      leader.add(segment);
    }
    leader.userData.isLeaderLine = true;
    return leader;
  }

  function addIsometricZone(THREE, root, zone, scale, center) {
    var bounds = isometricBounds(zone);
    var world = isometricWorld({ x: bounds.x + bounds.w / 2, y: bounds.y + bounds.h / 2 }, scale, center);
    var presentation = zone && zone.presentation && typeof zone.presentation === "object" ? zone.presentation : {};
    var color = colorValue(presentation.color || "#e6edf5", 0xe6edf5);
    var plane = new THREE.Mesh(new THREE.PlaneGeometry(bounds.w * scale, bounds.h * scale), new THREE.MeshBasicMaterial({
      color: color,
      transparent: true,
      opacity: 0.18,
      depthWrite: false
    }));
    plane.rotation.x = -Math.PI / 2;
    plane.position.set(world.x, 0.006, world.z);
    plane.userData = { zone: zone.id, label: itemLabel(zone) };
    root.add(plane);
    var points = [
      isometricWorld({ x: bounds.x, y: bounds.y }, scale, center),
      isometricWorld({ x: bounds.x + bounds.w, y: bounds.y }, scale, center),
      isometricWorld({ x: bounds.x + bounds.w, y: bounds.y + bounds.h }, scale, center),
      isometricWorld({ x: bounds.x, y: bounds.y + bounds.h }, scale, center),
      isometricWorld({ x: bounds.x, y: bounds.y }, scale, center)
    ].map(function (p) { return new THREE.Vector3(p.x, 0.035, p.z); });
    var boundaryStyle = normalizeMarkKey(presentation.boundary || presentation.lineStyle || zone.style || "solid");
    var boundaryColor = presentation.color || zone.color || "#111827";
    var boundary = boundaryStyle === "dashed" || boundaryStyle === "dash" ? createDashedPolyline(THREE, points, boundaryColor, 0.78, 0.16, 0.09) : new THREE.Line(new THREE.BufferGeometry().setFromPoints(points), isometricLineMaterial(THREE, boundaryColor, 0.86));
    boundary.userData = { isZoneBoundary: true, style: boundaryStyle };
    root.add(boundary);
    return { zone: zone, bounds: bounds, labelPoint: { x: bounds.x + 0.35, y: bounds.y + 0.35 }, plane: plane, boundary: boundary };
  }

  function labelHTML(className, text) {
    var node = el("div", className, text || "");
    node.setAttribute("data-label", text || "");
    return node;
  }

  function renderIsometricArchitecture(ctx) {
    var manifest = ctx.manifest || {};
    var data = ctx.data || {};
    var shell = createIsometricShell(ctx.container, manifest, data);
    var THREE = window.THREE;
    if (!THREE || !THREE.WebGLRenderer || !THREE.OrthographicCamera) {
      shell.stage.appendChild(el("p", "isometric-fallback", "Three.js is required for this offline architecture renderer."));
      return;
    }
    var zones = isometricArray(data, "zones");
    var entities = isometricArray(data, "entities");
    var links = isometricArray(data, "links");
    var zoneByID = {};
    zones.forEach(function (zone) { zoneByID[zone.id] = zone; });
    var allBounds = zones.length ? zones.map(isometricBounds) : [{ x: 0, y: 0, w: 18, h: 12 }];
    var minX = Math.min.apply(null, allBounds.map(function (b) { return b.x; }));
    var minY = Math.min.apply(null, allBounds.map(function (b) { return b.y; }));
    var maxX = Math.max.apply(null, allBounds.map(function (b) { return b.x + b.w; }));
    var maxY = Math.max.apply(null, allBounds.map(function (b) { return b.y + b.h; }));
    var center = { x: (minX + maxX) / 2, y: (minY + maxY) / 2 };
    var span = Math.max(8, maxX - minX, maxY - minY);
    var scale = 8 / span;
    var width = Math.max(760, shell.stage.clientWidth || 1024);
    var height = Math.max(540, shell.stage.clientHeight || 680);
    var renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
    renderer.setClearColor(0xffffff, 0);
    renderer.setPixelRatio(Math.min(2, window.devicePixelRatio || 1));
    renderer.setSize(width, height, false);
    if (THREE.SRGBColorSpace) {
      renderer.outputColorSpace = THREE.SRGBColorSpace;
    }
    shell.stage.insertBefore(renderer.domElement, shell.labelLayer);

    var scene = new THREE.Scene();
    var camera = new THREE.OrthographicCamera(-6, 6, 4, -4, 0.1, 100);
    var target = new THREE.Vector3(0, 0, 0);
    var cameraState = { theta: Math.PI / 4, phi: Math.PI / 3.2, radius: 11, panX: 0, panZ: 0, zoom: 1 };
    var root = new THREE.Group();
    var zoneRoot = new THREE.Group();
    var entityRoot = new THREE.Group();
    var linkRoot = new THREE.Group();
    var leaderRoot = new THREE.Group();
    root.add(zoneRoot);
    root.add(linkRoot);
    root.add(entityRoot);
    root.add(leaderRoot);
    scene.add(root);
    if (THREE.HemisphereLight) {
      scene.add(new THREE.HemisphereLight(0xffffff, 0xcbd5e1, 1.15));
    } else {
      scene.add(new THREE.AmbientLight(0xffffff, 0.78));
    }
    var sun = new THREE.DirectionalLight(0xffffff, 1.08);
    sun.position.set(6, 8, 5);
    scene.add(sun);
    var base = new THREE.Mesh(new THREE.PlaneGeometry((maxX - minX + 3) * scale, (maxY - minY + 3) * scale), new THREE.MeshBasicMaterial({ color: 0xf8fafc, transparent: true, opacity: 0.94 }));
    base.rotation.x = -Math.PI / 2;
    base.position.y = -0.018;
    base.userData.isBasePlane = true;
    root.add(base);
    var grid = addThreeGrid(THREE, root, Math.max(10, span * scale + 1.5), Math.max(10, Math.ceil(span)), 0.002, 0x94a3b8);
    grid.userData.isIsometricGrid = true;

    var labels = [];
    var entityByID = {};
    var markContext = createMarkContext(manifest, data);
    zones.forEach(function (zone) {
      var info = addIsometricZone(THREE, zoneRoot, zone, scale, center);
      var pos = isometricWorld(info.labelPoint, scale, center);
      var label = labelHTML("visual-isometric-zone-label", itemLabel(zone) || zone.id);
      label.setAttribute("data-zone-label", zone.id || "");
      shell.labelLayer.appendChild(label);
      labels.push({ element: label, point: new THREE.Vector3(pos.x, 0.16, pos.z), visible: true, type: "zone", priority: 0.56 });
    });

    entities.forEach(function (item, index) {
      var zone = zoneByID[item.zone] || zones[index % Math.max(1, zones.length)] || {};
      var pos = isometricPosition(item, zone, index);
      var world = isometricWorld(pos, scale, center);
      var size = isometricSize(item);
      var spec = resolveMarkSpec(item, markContext);
      var object = createIsometricEntity(THREE, item, spec, size, markContext);
      object.position.set(world.x, size.h * 0.22, world.z);
      object.scale.setScalar(1 + Math.min(0.35, importanceValue(item, 0.45) * 0.18));
      entityRoot.add(object);
      entityByID[item.id] = { object: object, item: item, pos: pos, world: new THREE.Vector3(world.x, size.h * 0.22, world.z), size: size };
      var label = labelHTML("visual-isometric-label", itemLabel(item) || item.id);
      label.setAttribute("data-entity-label", item.id || "");
      var inlineIcon = badgeSettings(markContext).labelIcon ? createInlineIcon(spec, markContext, "visual-isometric-label-icon") : null;
      if (inlineIcon) {
        label.textContent = "";
        label.appendChild(inlineIcon);
        label.appendChild(el("span", "", itemLabel(item) || item.id));
      }
      shell.labelLayer.appendChild(label);
      var anchor = new THREE.Vector3(world.x, size.h * 0.72 + 0.28, world.z);
      labels.push({ element: label, point: anchor, visible: true, type: "entity", priority: 0.84 + importanceValue(item, 0.45) * 0.32, id: item.id });
      var leader = createVerticalDashedLeader(THREE, world.x, world.z, size.h * 0.38, anchor.y);
      leaderRoot.add(leader);
    });

    links.forEach(function (link, index) {
      var from = entityByID[link.from];
      var to = entityByID[link.to];
      if (!from || !to) {
        return;
      }
      var edgeSpec = resolveEdgeSpec(link, markContext);
      edgeSpec.opacity = Math.max(edgeSpec.opacity || 0.68, 0.82);
      edgeSpec.lightBackground = true;
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      if (!presentation.color && !link.color) {
        edgeSpec.color = "#111827";
      }
      var pathPoints = isometricLinkPathPoints(link, from, to);
      var curve;
      if (pathPoints.length > 2 && THREE.CatmullRomCurve3) {
        curve = new THREE.CatmullRomCurve3(pathPoints);
      } else {
        curve = edgeCurveFor(THREE, pathPoints[0], pathPoints[pathPoints.length - 1], link, Object.assign({}, edgeSpec, { curve: "straight", flow: false }), index);
      }
      var tube = createEdgeTube(THREE, curve, edgeSpec, 0.032);
      tube.userData.isDirectedArrow = !!edgeSpec.directed;
      linkRoot.add(tube);
      var arrow = createArrowHead(THREE, curve, edgeSpec);
      if (arrow) {
        arrow.userData.isDirectedArrow = true;
        linkRoot.add(arrow);
      }
      createFlowParticles(THREE, curve, edgeSpec, edgeSpec.flow ? 2 : 0).forEach(function (marker) { linkRoot.add(marker); });
      if (link.label) {
        var mid = curve.getPoint(0.52);
        var label = labelHTML("visual-isometric-link-label", link.label);
        label.setAttribute("data-link-label", link.id || "");
        if (importanceValue(link, 0.35) < 0.74) {
          label.setAttribute("data-low-priority", "true");
        }
        shell.labelLayer.appendChild(label);
        labels.push({ element: label, point: mid.clone().add(new THREE.Vector3(0, 0.18, 0)), visible: true, type: "link", priority: 0.34 + importanceValue(link, 0.35) * 0.5, id: link.id });
      }
    });

    var selected = "";
    var labelsVisible = true;
    var boundariesVisible = true;
    var arrowsVisible = true;
    var pointer = new THREE.Vector2();
    var raycaster = new THREE.Raycaster();
    var meshes = entities.map(function (item) { return entityByID[item.id] && entityByID[item.id].object; }).filter(Boolean);

    function updateCamera() {
      var aspect = width / height;
      var view = 5.2 / cameraState.zoom;
      camera.left = -view * aspect;
      camera.right = view * aspect;
      camera.top = view;
      camera.bottom = -view;
      target.set(cameraState.panX, 0, cameraState.panZ);
      var sinPhi = Math.sin(cameraState.phi);
      camera.position.set(
        target.x + cameraState.radius * sinPhi * Math.sin(cameraState.theta),
        target.y + cameraState.radius * Math.cos(cameraState.phi),
        target.z + cameraState.radius * sinPhi * Math.cos(cameraState.theta)
      );
      camera.lookAt(target);
      camera.updateProjectionMatrix();
    }

    function project(point) {
      var projected = point.clone().project(camera);
      return { x: (projected.x * 0.5 + 0.5) * width, y: (-projected.y * 0.5 + 0.5) * height };
    }

    function isometricLinkGroundPoint(node) {
      return node.world.clone().setY(0.07);
    }

    function isometricLinkPathPoints(link, from, to) {
      var start = isometricLinkGroundPoint(from);
      var end = isometricLinkGroundPoint(to);
      var route = Array.isArray(link.route) ? link.route : [];
      if (route.length) {
        return [start].concat(route.map(function (point) {
          var world = isometricWorld(point, scale, center);
          return new THREE.Vector3(world.x, 0.07, world.z);
        })).concat([end]);
      }
      var routeStyle = normalizeMarkKey(link.routeStyle || link.route_style || link.presentation && link.presentation.routeStyle || "");
      if (routeStyle === "orthogonal" || routeStyle === "elbow" || Math.abs(start.x - end.x) > 0.5 && Math.abs(start.z - end.z) > 0.5) {
        return [start, new THREE.Vector3(end.x, 0.07, start.z), end];
      }
      return [start, end];
    }

    function labelBudget(label) {
      if (label.type === "link") return 10;
      if (label.type === "zone") return 12;
      return 28;
    }

    function labelRect(label, projected) {
      var rect = label.element.getBoundingClientRect();
      var w = Math.max(28, rect.width || 80);
      var h = Math.max(18, rect.height || 28);
      var mode = label.type === "link" || label.type === "zone" ? "center" : "top";
      if (mode === "center") {
        return { x: projected.x - w / 2, y: projected.y - h / 2, w: w, h: h };
      }
      return { x: projected.x - w / 2, y: projected.y - h, w: w, h: h };
    }

    function rectOverlaps(a, b, padding) {
      return a.x < b.x + b.w + padding && a.x + a.w + padding > b.x && a.y < b.y + b.h + padding && a.y + a.h + padding > b.y;
    }

    function insideViewport(rect) {
      return rect.x > 6 && rect.y > 6 && rect.x + rect.w < width - 6 && rect.y + rect.h < height - 6;
    }

    function updateLabels() {
      var projected = labels.map(function (label, index) {
        var p = project(label.point);
        label.element.style.display = labelsVisible && label.visible ? "" : "none";
        label.element.style.visibility = "hidden";
        var transformMode = label.type === "link" || label.type === "zone" ? "translate(-50%, -50%)" : "translate(-50%, -100%)";
        label.element.style.transform = "translate(" + p.x.toFixed(1) + "px, " + p.y.toFixed(1) + "px) " + transformMode;
        return { label: label, index: index, projected: p, rect: labelRect(label, p), priority: label.priority || 0.5 };
      });
      var selectedID = selected || "";
      var counts = { entity: 0, link: 0, zone: 0 };
      var occupied = [];
      projected.sort(function (a, b) {
        var aSelected = a.label.id && a.label.id === selectedID ? 1 : 0;
        var bSelected = b.label.id && b.label.id === selectedID ? 1 : 0;
        if (aSelected !== bSelected) return bSelected - aSelected;
        return b.priority - a.priority;
      }).forEach(function (item) {
        var label = item.label;
        var type = label.type || "entity";
        var allowed = labelsVisible && label.visible && insideViewport(item.rect) && counts[type] < labelBudget(label);
        if (allowed) {
          var padding = type === "link" ? 8 : 10;
          allowed = !occupied.some(function (rect) { return rectOverlaps(item.rect, rect, padding); });
        }
        label.element.style.visibility = allowed ? "visible" : "hidden";
        label.element.style.opacity = allowed ? "1" : "0";
        if (allowed) {
          counts[type] += 1;
          occupied.push(item.rect);
        }
      });
    }

    function showSelected(item) {
      selected = item && item.id ? item.id : "";
      if (shell.inspector) {
        shell.inspector.show(itemLabel(item) || selected || "Architecture", item || { title: data.title, entities: entities.length, links: links.length });
      }
      meshes.forEach(function (mesh) {
        mesh.traverse(function (child) {
          if (child.material && child.material.emissiveIntensity !== undefined) {
            child.material.emissiveIntensity = mesh.userData.id === selected ? 0.22 : 0.02;
          }
        });
      });
    }

    shell.controls.appendChild(createIsometricButton("Overview", "overview", function () {
      meshes.forEach(function (mesh) { mesh.visible = true; });
      linkRoot.visible = true;
      showSelected(null);
    }));
    shell.controls.appendChild(createIsometricButton("Reset", "reset_camera", function () {
      cameraState.theta = Math.PI / 4;
      cameraState.phi = Math.PI / 3.2;
      cameraState.radius = 11;
      cameraState.panX = 0;
      cameraState.panZ = 0;
      cameraState.zoom = 1;
    }));
    shell.controls.appendChild(createIsometricButton("Focus", "focus", function () {
      var focusIDs = data.visual && Array.isArray(data.visual.initial_focus_ids) ? data.visual.initial_focus_ids : [];
      if (focusIDs.length && entityByID[focusIDs[0]]) {
        var point = entityByID[focusIDs[0]].world;
        cameraState.panX = point.x;
        cameraState.panZ = point.z;
      }
    }));
    shell.controls.appendChild(createIsometricButton("Isolate zone", "isolate", function () {
      if (!selected || !entityByID[selected]) return;
      var zoneID = entityByID[selected].item.zone;
      entities.forEach(function (item) {
        var node = entityByID[item.id];
        if (node) node.object.visible = item.zone === zoneID;
      });
    }));
    shell.controls.appendChild(createIsometricButton("Labels", "toggle_labels", function () {
      labelsVisible = !labelsVisible;
    }));
    shell.controls.appendChild(createIsometricButton("Boundaries", "toggle_boundaries", function () {
      boundariesVisible = !boundariesVisible;
      zoneRoot.visible = boundariesVisible;
    }));
    shell.controls.appendChild(createIsometricButton("Arrows", "toggle_arrows", function () {
      arrowsVisible = !arrowsVisible;
      linkRoot.visible = arrowsVisible;
    }));
    shell.controls.appendChild(createIsometricButton("Export", "export_json", function () {
      runtime.exportJSON(data, "isometric-architecture-data.json");
    }));

    shell.stage.addEventListener("click", function (event) {
      var rect = renderer.domElement.getBoundingClientRect();
      pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
      pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
      raycaster.setFromCamera(pointer, camera);
      var hits = raycaster.intersectObjects(meshes, true);
      if (hits.length && hits[0].object && hits[0].object.userData) {
        showSelected(hits[0].object.userData.payload);
      }
    });

    var drag = { active: false, mode: "", x: 0, y: 0 };
    shell.stage.addEventListener("pointerdown", function (event) {
      drag.active = true;
      drag.mode = event.shiftKey ? "rotate" : "pan";
      drag.x = event.clientX;
      drag.y = event.clientY;
      shell.stage.setPointerCapture(event.pointerId);
    });
    shell.stage.addEventListener("pointermove", function (event) {
      if (!drag.active) return;
      var dx = event.clientX - drag.x;
      var dy = event.clientY - drag.y;
      drag.x = event.clientX;
      drag.y = event.clientY;
      if (drag.mode === "rotate") {
        cameraState.theta += dx * 0.004;
        cameraState.phi = Math.max(0.72, Math.min(1.28, cameraState.phi + dy * 0.002));
      } else {
        cameraState.panX -= dx * 0.01 / cameraState.zoom;
        cameraState.panZ -= dy * 0.01 / cameraState.zoom;
      }
    });
    shell.stage.addEventListener("pointerup", function (event) {
      drag.active = false;
      try { shell.stage.releasePointerCapture(event.pointerId); } catch (err) { /* ignore */ }
    });
    shell.stage.addEventListener("wheel", function (event) {
      event.preventDefault();
      cameraState.zoom = Math.max(0.55, Math.min(2.6, cameraState.zoom * (event.deltaY > 0 ? 0.92 : 1.08)));
    }, { passive: false });

    function resize() {
      width = Math.max(760, shell.stage.clientWidth || width);
      height = Math.max(540, shell.stage.clientHeight || height);
      renderer.setSize(width, height, false);
      updateCamera();
      updateLabels();
    }
    var observer = typeof ResizeObserver !== "undefined" ? new ResizeObserver(resize) : null;
    if (observer) observer.observe(shell.stage);
    function animate() {
      if (!document.body.contains(shell.stage)) {
        if (observer) observer.disconnect();
        renderer.dispose();
        return;
      }
      updateCamera();
      linkRoot.children.forEach(function (child) {
        if (child.userData && child.userData.curve) {
          var t = ((Date.now() / 1000) * child.userData.speed + child.userData.phase) % 1;
          child.position.copy(child.userData.curve.getPoint(t));
          if (child.material) child.material.opacity = child.userData.baseOpacity || 0.6;
        } else if (child.material && child.userData && child.userData.baseOpacity !== undefined) {
          child.material.opacity += ((child.userData.baseOpacity || 0.7) - child.material.opacity) * 0.09;
        }
      });
      entityRoot.children.forEach(function (object) {
        object.traverse(function (child) {
          if (child.userData && (child.userData.isIconBillboard || child.userData.isGeneratedModelBadgeLabel)) {
            child.quaternion.copy(camera.quaternion);
          } else if (child.isMesh && child.material && child.material.map) {
            child.quaternion.copy(camera.quaternion);
          }
        });
      });
      updateLabels();
      renderer.render(scene, camera);
      window.requestAnimationFrame(animate);
    }
    updateCamera();
    showSelected(null);
    shell.stage.classList.add("visual-isometric-ready");
    animate();
  }

  runtime.registerRenderer("offline.graph.v1", { render: renderGraph });
  runtime.registerRenderer("offline.architecture.isometric.v1", { render: renderIsometricArchitecture });
  runtime.registerRenderer("offline.timeline.v1", { render: renderTimeline });
  runtime.registerRenderer("offline.evidence.v1", { render: renderEvidence });
  runtime.registerRenderer("offline.matrix.v1", { render: renderMatrix });
  runtime.registerRenderer("offline.uml.sequence.3d.v1", { render: renderUMLSequence });
  runtime.registerRenderer("offline.uml.class.2_5d.v1", { render: renderUMLClass });
  runtime.registerRenderer("offline.uml.state.3d.v1", { render: renderUMLState });
  runtime.registerRenderer("offline.uml.activity.3d.v1", { render: renderUMLActivity });
  runtime.registerRenderer("offline.uml.component.3d.v1", { render: renderUMLComponent });
}());
