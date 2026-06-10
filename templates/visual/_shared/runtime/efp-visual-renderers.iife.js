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
    var TextureCtor = THREE.CanvasTexture || THREE.Texture;
    if (!TextureCtor || typeof document === "undefined") return null;
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
    var texture = new TextureCtor(canvas);
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
    icon.loading = "eager";
    icon.decoding = "async";
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
    var iconSize = Math.max(0.18, Math.min(0.3, (size ? Math.min(size.w, size.h) : 0.7) * 0.24 * factor));
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
    var plateW = Math.min(size.w * 0.35, Math.max(size.w * 0.2, size.w * 0.28 * factor));
    var plateH = Math.min(size.h * 0.14, Math.max(size.h * 0.075, size.h * 0.09 * factor));
    var plateD = Math.min(size.d * 0.13, Math.max(size.d * 0.07, size.d * 0.1 * factor));
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
      var helper = new THREE.GridHelper(size, divisions, color || 0xcbd5e1, 0xe2e8f0);
      if (Array.isArray(helper.material)) {
        helper.material.forEach(function (material) {
          material.transparent = true;
          material.opacity = 0.46;
          material.depthWrite = false;
        });
      } else if (helper.material) {
        helper.material.transparent = true;
        helper.material.opacity = 0.46;
        helper.material.depthWrite = false;
      }
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

  function createGroundRibbonMesh(THREE, points, edgeSpec, radius) {
    if (!THREE.BufferGeometry || !THREE.Float32BufferAttribute || !THREE.Mesh || !THREE.MeshBasicMaterial) return null;
    var color = colorValue(edgeSpec.color, 0x63a9ff);
    points = edgeSpec.routePoints && edgeSpec.routePoints.length >= 2 ? edgeSpec.routePoints.map(function (p) { return p.clone(); }) : (points || []);
    var width = Math.max(0.004, numberValue(edgeSpec.groundWidth, (radius || 0.016) * 4.6));
    var height = Math.max(0.001, numberValue(edgeSpec.groundHeight, 0.008));
    var y0 = numberValue(edgeSpec.groundY, 0.032);
    var y1 = y0 + height;
    var half = width * 0.5;
    var positions = [];
    var segmentCount = 0;
    var jointCount = 0;
    var dashed = edgeSpec.lineStyle === "dashed" || edgeSpec.lineStyle === "dash";
    var dashLength = edgeSpec.dashLength || 0.55;
    var gapLength = edgeSpec.gapLength || 0.3;

    if (THREE.Group && points.length >= 2) {
      var railMaterial = new THREE.MeshBasicMaterial({
        color: color,
        transparent: true,
        opacity: edgeSpec.opacity === undefined ? 0.82 : edgeSpec.opacity,
        depthTest: true,
        depthWrite: false,
        side: THREE.DoubleSide || 2,
        polygonOffset: true,
        polygonOffsetFactor: -2,
        polygonOffsetUnits: -2
      });
      var railGroup = new THREE.Group();
      railGroup.material = railMaterial;
      railGroup.renderOrder = 5;
      railGroup.frustumCulled = false;
      railGroup.userData.isEdgeTube = true;
      railGroup.userData.isGroundRibbon = true;
      railGroup.userData.isGroundRouteRail = false;
      railGroup.userData.isGroundRouteDecal = true;
      railGroup.userData.isGroundDecalGroup = true;
      railGroup.userData.groundRailImplementation = "ground_decal_segments";
      railGroup.userData.relationRenderMode = "ground_decal";
      railGroup.userData.groundSegmentCount = 0;
      railGroup.userData.groundJointCount = Math.max(0, points.length - 2);
      railGroup.userData.baseOpacity = edgeSpec.opacity;
      railGroup.userData.targetOpacity = edgeSpec.opacity;
      function addDecalSegment(start, end) {
        var delta = end.clone().sub(start);
        delta.y = 0;
        var length = delta.length();
        if (length < 0.05) return;
        var side = new THREE.Vector3(0, 1, 0).cross(delta.clone().normalize()).normalize().multiplyScalar(width * 0.5);
        if (side.length() < 0.001) side.set(width * 0.5, 0, 0);
        var aL = new THREE.Vector3(start.x + side.x, y0, start.z + side.z);
        var aR = new THREE.Vector3(start.x - side.x, y0, start.z - side.z);
        var bL = new THREE.Vector3(end.x + side.x, y0, end.z + side.z);
        var bR = new THREE.Vector3(end.x - side.x, y0, end.z - side.z);
        var segmentGeometry = new THREE.BufferGeometry();
        segmentGeometry.setAttribute("position", new THREE.Float32BufferAttribute([
          aL.x, aL.y, aL.z,
          bL.x, bL.y, bL.z,
          bR.x, bR.y, bR.z,
          aL.x, aL.y, aL.z,
          bR.x, bR.y, bR.z,
          aR.x, aR.y, aR.z
        ], 3));
        if (segmentGeometry.computeBoundingSphere) segmentGeometry.computeBoundingSphere();
        var segment = new THREE.Mesh(segmentGeometry, railMaterial);
        segment.renderOrder = 5;
        segment.frustumCulled = false;
        segment.userData.type = "link-segment";
        segment.userData.isGroundRouteRailSegment = false;
        segment.userData.isGroundRouteDecalSegment = true;
        segment.userData.linkId = edgeSpec.id || "";
        railGroup.add(segment);
        railGroup.userData.groundSegmentCount += 1;
      }
      points.forEach(function (point, pointIndex) {
        if (pointIndex >= points.length - 1) return;
        var start = point;
        var end = points[pointIndex + 1];
        var delta = end.clone().sub(start);
        delta.y = 0;
        var length = delta.length();
        if (length < 0.05) return;
        if (!dashed) {
          addDecalSegment(start, end);
          return;
        }
        var dir = delta.clone().normalize();
        var cursor = 0;
        while (cursor < length) {
          var from = cursor;
          var to = Math.min(length, cursor + dashLength);
          if (to > from + 0.04) {
            addDecalSegment(start.clone().add(dir.clone().multiplyScalar(from)), start.clone().add(dir.clone().multiplyScalar(to)));
          }
          cursor += dashLength + gapLength;
        }
      });
      if (!dashed) for (var railJointIndex = 1; railJointIndex < points.length - 1; railJointIndex += 1) {
        var halfJoint = width * 0.56;
        var p = points[railJointIndex];
        var jointGeometry = new THREE.BufferGeometry();
        jointGeometry.setAttribute("position", new THREE.Float32BufferAttribute([
          p.x - halfJoint, y0, p.z - halfJoint,
          p.x + halfJoint, y0, p.z - halfJoint,
          p.x + halfJoint, y0, p.z + halfJoint,
          p.x - halfJoint, y0, p.z - halfJoint,
          p.x + halfJoint, y0, p.z + halfJoint,
          p.x - halfJoint, y0, p.z + halfJoint
        ], 3));
        if (jointGeometry.computeBoundingSphere) jointGeometry.computeBoundingSphere();
        var joint = new THREE.Mesh(jointGeometry, railMaterial);
        joint.renderOrder = 5;
        joint.frustumCulled = false;
        joint.userData.type = "link-joint";
        joint.userData.isGroundRouteDecalJoint = true;
        joint.userData.linkId = edgeSpec.id || "";
        railGroup.add(joint);
      }
      if (railGroup.userData.groundSegmentCount > 0) {
        return railGroup;
      }
    }

    function pushTriangle(a, b, c) {
      positions.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
    }

    function pushQuad(a, b, c, d) {
      pushTriangle(a, b, c);
      pushTriangle(a, c, d);
    }

    function pushRailSegment(a, b) {
      var delta = b.clone().sub(a);
      delta.y = 0;
      if (delta.length() < 0.001) return;
      var side = new THREE.Vector3(0, 1, 0).cross(delta.normalize()).normalize().multiplyScalar(half);
      var aL0 = new THREE.Vector3(a.x + side.x, y0, a.z + side.z);
      var aR0 = new THREE.Vector3(a.x - side.x, y0, a.z - side.z);
      var bL0 = new THREE.Vector3(b.x + side.x, y0, b.z + side.z);
      var bR0 = new THREE.Vector3(b.x - side.x, y0, b.z - side.z);
      var aL1 = new THREE.Vector3(aL0.x, y1, aL0.z);
      var aR1 = new THREE.Vector3(aR0.x, y1, aR0.z);
      var bL1 = new THREE.Vector3(bL0.x, y1, bL0.z);
      var bR1 = new THREE.Vector3(bR0.x, y1, bR0.z);
      pushQuad(aL1, bL1, bR1, aR1);
      pushQuad(aL0, aR0, bR0, bL0);
      pushQuad(aL0, bL0, bL1, aL1);
      pushQuad(aR0, aR1, bR1, bR0);
      pushQuad(aL0, aL1, aR1, aR0);
      pushQuad(bL0, bR0, bR1, bL1);
      segmentCount += 1;
    }

    function pushJoint(point) {
      var halfJoint = half * 0.92;
      var x0 = point.x - halfJoint;
      var x1 = point.x + halfJoint;
      var z0 = point.z - halfJoint;
      var z1 = point.z + halfJoint;
      var p000 = new THREE.Vector3(x0, y0, z0);
      var p100 = new THREE.Vector3(x1, y0, z0);
      var p110 = new THREE.Vector3(x1, y0, z1);
      var p010 = new THREE.Vector3(x0, y0, z1);
      var p001 = new THREE.Vector3(x0, y1, z0);
      var p101 = new THREE.Vector3(x1, y1, z0);
      var p111 = new THREE.Vector3(x1, y1, z1);
      var p011 = new THREE.Vector3(x0, y1, z1);
      pushQuad(p001, p101, p111, p011);
      pushQuad(p000, p010, p110, p100);
      pushQuad(p000, p100, p101, p001);
      pushQuad(p100, p110, p111, p101);
      pushQuad(p110, p010, p011, p111);
      pushQuad(p010, p000, p001, p011);
      jointCount += 1;
    }

    for (var i = 0; i < points.length - 1; i += 1) {
      var start = points[i];
      var end = points[i + 1];
      var segment = end.clone().sub(start);
      var length = segment.length();
      if (length < 0.001) continue;
      if (!dashed) {
        pushRailSegment(start, end);
        continue;
      }
      var dir = segment.clone().normalize();
      var cursor = 0;
      while (cursor < length) {
        var from = cursor;
        var to = Math.min(length, cursor + dashLength);
        if (to > from + 0.02) {
          pushRailSegment(start.clone().add(dir.clone().multiplyScalar(from)), start.clone().add(dir.clone().multiplyScalar(to)));
        }
        cursor += dashLength + gapLength;
      }
    }
    if (!dashed) {
      for (var j = 1; j < points.length - 1; j += 1) {
        pushJoint(points[j]);
      }
    }
    if (positions.length < 18) return null;
    var geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    if (geometry.computeBoundingSphere) geometry.computeBoundingSphere();
    var material = new THREE.MeshBasicMaterial({
      color: color,
      transparent: true,
      opacity: edgeSpec.opacity === undefined ? 0.82 : edgeSpec.opacity,
      depthTest: false,
      depthWrite: false,
      side: THREE.DoubleSide || 2,
      polygonOffset: true,
      polygonOffsetFactor: -1,
      polygonOffsetUnits: -1
    });
    var mesh = new THREE.Mesh(geometry, material);
    mesh.renderOrder = 12;
    mesh.userData.isEdgeTube = true;
    mesh.userData.isGroundRibbon = true;
    mesh.userData.isGroundRouteRail = true;
    mesh.userData.groundRailImplementation = "buffer_prism";
    mesh.userData.groundSegmentCount = segmentCount;
    mesh.userData.groundJointCount = jointCount;
    mesh.userData.baseOpacity = edgeSpec.opacity;
    mesh.userData.targetOpacity = edgeSpec.opacity;
    return mesh;
  }

  function createEdgeTube(THREE, curve, edgeSpec, radius) {
    var color = colorValue(edgeSpec.color, 0x63a9ff);
    if (edgeSpec.groundRibbon) {
      var ribbon = createGroundRibbonMesh(THREE, edgeSpec.routePoints || curvePoints(curve, 18), edgeSpec, radius);
      if (ribbon) return ribbon;
    }
    var geometry = THREE.TubeGeometry ? new THREE.TubeGeometry(curve, 18, radius || 0.012, 8, false) : new THREE.BufferGeometry().setFromPoints(curvePoints(curve, 18));
    var blendMode = edgeSpec.lightBackground ? THREE.NormalBlending : THREE.AdditiveBlending;
    var material = THREE.TubeGeometry ? new THREE.MeshBasicMaterial({
      color: color,
      transparent: true,
      opacity: edgeSpec.lightBackground ? edgeSpec.opacity : 0,
      blending: blendMode
    }) : new THREE.LineBasicMaterial({
      color: color,
      transparent: true,
      opacity: edgeSpec.lightBackground ? edgeSpec.opacity : 0,
      blending: blendMode
    });
    material.depthTest = false;
    material.depthWrite = false;
    var object = THREE.TubeGeometry ? new THREE.Mesh(geometry, material) : new THREE.Line(geometry, material);
    object.renderOrder = edgeSpec.lightBackground ? 3 : 1;
    object.userData.isEdgeTube = true;
    object.userData.baseOpacity = edgeSpec.opacity;
    object.userData.targetOpacity = edgeSpec.opacity;
    return object;
  }

  function createEdgeHitArea(THREE, curve, radius) {
    if (!THREE.MeshBasicMaterial && !THREE.LineBasicMaterial) {
      return null;
    }
    var geometry = THREE.TubeGeometry
      ? new THREE.TubeGeometry(curve, 18, Math.max(0.045, radius || 0.045), 8, false)
      : new THREE.BufferGeometry().setFromPoints(curvePoints(curve, 18));
    var material = THREE.TubeGeometry ? new THREE.MeshBasicMaterial({
      color: 0x000000,
      transparent: true,
      opacity: 0.001,
      depthTest: false,
      depthWrite: false
    }) : new THREE.LineBasicMaterial({
      color: 0x000000,
      transparent: true,
      opacity: 0.001,
      depthTest: false,
      depthWrite: false
    });
    var mesh = THREE.TubeGeometry ? new THREE.Mesh(geometry, material) : new THREE.Line(geometry, material);
    mesh.visible = true;
    mesh.renderOrder = 22;
    mesh.userData.isGroundLinkHitArea = true;
    mesh.userData.baseOpacity = 0.001;
    mesh.userData.targetOpacity = 0.001;
    return mesh;
  }

  function createArrowHead(THREE, curve, edgeSpec) {
    if (!edgeSpec.directed || edgeSpec.arrow === "none") {
      return null;
    }
    if (edgeSpec.groundRail && THREE.BufferGeometry && THREE.Float32BufferAttribute) {
      var railRoute = edgeSpec.routePoints && edgeSpec.routePoints.length >= 2 ? edgeSpec.routePoints : curvePoints(curve, 14);
      var railEnd = railRoute[railRoute.length - 1].clone();
      var railPrev = railRoute[railRoute.length - 2].clone();
      var railDirection = railEnd.clone().sub(railPrev);
      railDirection.y = 0;
      if (railDirection.length() < 0.001) return null;
      railDirection.normalize();
      var railSide = new THREE.Vector3(0, 1, 0).cross(railDirection).normalize();
      if (railSide.length() < 0.001) railSide.set(1, 0, 0);
      var railWidth = Math.max(0.022, numberValue(edgeSpec.groundArrowWidth, Math.max(numberValue(edgeSpec.groundWidth, 0.1) * 2.4, 0.18)));
      var railLength = Math.max(0.04, numberValue(edgeSpec.groundArrowLength, Math.max(numberValue(edgeSpec.groundWidth, 0.1) * 3.0, 0.22)));
      var railHeight = Math.max(0.001, numberValue(edgeSpec.groundHeight, 0.008) * 1.15);
      var y0 = numberValue(edgeSpec.groundY, 0.032) + 0.004;
      var tip = railEnd.clone().sub(railDirection.clone().multiplyScalar(0.035));
      var back = tip.clone().sub(railDirection.clone().multiplyScalar(railLength));
      var left = back.clone().add(railSide.clone().multiplyScalar(railWidth * 0.5));
      var right = back.clone().add(railSide.clone().multiplyScalar(-railWidth * 0.5));
      function withY(point, y) { return new THREE.Vector3(point.x, y, point.z); }
      var topTip = withY(tip, y0 + railHeight);
      var topLeft = withY(left, y0 + railHeight);
      var topRight = withY(right, y0 + railHeight);
      var positions = [];
      function push(a, b, c) {
        positions.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
      }
      push(topTip, topLeft, topRight);
      var railGeometry = new THREE.BufferGeometry();
      railGeometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
      if (railGeometry.computeVertexNormals) railGeometry.computeVertexNormals();
      var railMaterial = new THREE.MeshBasicMaterial({
        color: colorValue(edgeSpec.arrowColor || edgeSpec.color, 0x111827),
        transparent: true,
        opacity: Math.min(1, edgeSpec.opacity === undefined ? 0.96 : edgeSpec.opacity),
        depthTest: true,
        depthWrite: false,
        side: THREE.DoubleSide || 2,
        polygonOffset: true,
        polygonOffsetFactor: -2,
        polygonOffsetUnits: -3
      });
      var railArrow = new THREE.Mesh(railGeometry, railMaterial);
      railArrow.renderOrder = 6;
      railArrow.userData.baseOpacity = railMaterial.opacity;
      railArrow.userData.targetOpacity = railMaterial.opacity;
      railArrow.userData.isGroundRouteRailArrowhead = true;
      railArrow.userData.isGroundArrowheadDecal = true;
      railArrow.userData.relationRenderMode = "ground_decal";
      return railArrow;
    }
    var arrowScale = Math.max(0.58, Math.min(2.25, edgeSpec.arrowScale || 1));
    var sampled = curvePoints(curve, 14);
    var routeLength = 0;
    for (var i = 0; i < sampled.length - 1; i += 1) {
      routeLength += sampled[i].distanceTo(sampled[i + 1]);
    }
    var arrowLength = Math.max(0.08, Math.min(0.24 * arrowScale, routeLength * 0.22));
    var arrowHalf = arrowLength * 0.42;
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
      var back = tip.clone().sub(direction.clone().multiplyScalar(arrowLength));
      var yLift = new THREE.Vector3(0, 0.012, 0);
      var geometryFlat = new THREE.BufferGeometry().setFromPoints([
        tip.clone().add(yLift),
        back.clone().add(side.clone().multiplyScalar(arrowHalf)).add(yLift),
        back.clone().add(side.clone().multiplyScalar(-arrowHalf)).add(yLift)
      ]);
      if (geometryFlat.computeVertexNormals) {
        geometryFlat.computeVertexNormals();
      }
      var materialFlat = new THREE.MeshBasicMaterial({
        color: colorValue(edgeSpec.color, 0x111827),
        transparent: true,
        opacity: Math.min(1, edgeSpec.opacity),
        blending: THREE.NormalBlending,
        side: THREE.DoubleSide || 2
      });
      materialFlat.depthTest = false;
      materialFlat.depthWrite = false;
      var flatArrow = new THREE.Mesh(geometryFlat, materialFlat);
      flatArrow.renderOrder = 4;
      flatArrow.userData.baseOpacity = Math.min(1, edgeSpec.opacity);
      flatArrow.userData.targetOpacity = flatArrow.userData.baseOpacity;
      return flatArrow;
    }
    var geometry = THREE.ConeGeometry ? new THREE.ConeGeometry(arrowHalf * 0.62, arrowLength, 18) : new THREE.IcosahedronGeometry(arrowHalf, 1);
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
        isFlowParticle: true,
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

  function svgEl(name) {
    return document.createElementNS("http:" + "/" + "/www.w3.org/2000/svg", name);
  }

  function createRelationMarker(defs, id, size, refX, color) {
    var marker = svgEl("marker");
    marker.setAttribute("id", id);
    marker.setAttribute("viewBox", "0 -6 12 12");
    marker.setAttribute("markerWidth", String(size));
    marker.setAttribute("markerHeight", String(size));
    marker.setAttribute("refX", String(refX || 11));
    marker.setAttribute("refY", "0");
    marker.setAttribute("orient", "auto");
    marker.setAttribute("markerUnits", "userSpaceOnUse");
    var shape = svgEl("path");
    shape.setAttribute("d", "M 0 -5.5 L 12 0 L 0 5.5 z");
    shape.setAttribute("fill", color || "#111827");
    shape.setAttribute("stroke", color || "#111827");
    shape.setAttribute("stroke-width", "0");
    marker.appendChild(shape);
    defs.appendChild(marker);
  }

  function createIsometricRelationLayer() {
    var layer = el("div", "visual-isometric-relation-layer");
    var svg = svgEl("svg");
    svg.setAttribute("class", "visual-isometric-relation-svg");
    svg.setAttribute("data-relation-layer", "true");
    svg.setAttribute("aria-hidden", "true");
    var defs = svgEl("defs");
    createRelationMarker(defs, "visual-isometric-arrow-primary", 14, 11.2, "#111827");
    createRelationMarker(defs, "visual-isometric-arrow-secondary", 12.5, 11, "#334155");
    createRelationMarker(defs, "visual-isometric-arrow-auxiliary", 10.5, 10.6, "#64748b");
    svg.appendChild(defs);
    layer.appendChild(svg);
    return { layer: layer, svg: svg };
  }

  function createIsometricInspector(container, data) {
    data = data || {};
    var entities = Array.isArray(data.entities) ? data.entities : [];
    var links = Array.isArray(data.links) ? data.links : [];
    var zones = Array.isArray(data.zones) ? data.zones : [];

    function roleOf(link) {
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      var metadata = link && link.metadata && typeof link.metadata === "object" ? link.metadata : {};
      return normalizeMarkKey(link && (link.role || presentation.role || metadata.role)) || "secondary";
    }

    function pathGroupOf(link) {
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      var metadata = link && link.metadata && typeof link.metadata === "object" ? link.metadata : {};
      return normalizeMarkKey(link && (link.pathGroup || link.path_group || presentation.pathGroup || presentation.path_group || metadata.pathGroup || metadata.path_group || link.kind)) || "relationship";
    }

    function entityByIDValue(id) {
      return entities.find(function (item) { return item && item.id === id; }) || null;
    }

    function zoneByIDValue(id) {
      return zones.find(function (item) { return item && item.id === id; }) || null;
    }

    function entityLinkCounts(id) {
      var incoming = 0;
      var outgoing = 0;
      links.forEach(function (link) {
        if (link && link.to === id) incoming += 1;
        if (link && link.from === id) outgoing += 1;
      });
      return { incoming: incoming, outgoing: outgoing };
    }

    function clear(title) {
      container.textContent = "";
      container.appendChild(el("h2", "visual-inspector-title", title || "Architecture Summary"));
    }

    function metricGrid(items) {
      var grid = el("dl", "visual-inspector-metrics");
      items.forEach(function (item) {
        grid.appendChild(el("dt", "", item[0]));
        grid.appendChild(el("dd", "", item[1]));
      });
      container.appendChild(grid);
    }

    function chips(values) {
      var row = el("div", "visual-inspector-chips");
      values.filter(Boolean).forEach(function (value) {
        row.appendChild(el("span", "visual-inspector-chip", value));
      });
      container.appendChild(row);
    }

    function rawDetails(payload) {
      var details = el("details", "visual-inspector-raw");
      details.appendChild(el("summary", "", "Raw JSON"));
      var pre = el("pre", "");
      try {
        pre.textContent = JSON.stringify(payload || {}, null, 2);
      } catch (err) {
        pre.textContent = String(payload || "");
      }
      details.appendChild(pre);
      container.appendChild(details);
    }

    function showSummary() {
      clear("Architecture Summary");
      var roles = { primary: 0, secondary: 0, auxiliary: 0 };
      var groups = {};
      links.forEach(function (link) {
        var role = roleOf(link);
        if (roles[role] === undefined) role = "secondary";
        roles[role] += 1;
        groups[pathGroupOf(link)] = true;
      });
      metricGrid([
        ["Zones", String(zones.length)],
        ["Entities", String(entities.length)],
        ["Links", String(links.length)],
        ["Primary", String(roles.primary)],
        ["Secondary", String(roles.secondary)],
        ["Auxiliary", String(roles.auxiliary)],
        ["Theme", String(data.theme || "architecture_light")]
      ]);
      chips(Object.keys(groups).sort().slice(0, 8));
    }

    function showEntity(item) {
      var counts = entityLinkCounts(item.id);
      var presentation = item.presentation && typeof item.presentation === "object" ? item.presentation : {};
      clear(itemLabel(item) || item.id || "Entity");
      if (item.summary) container.appendChild(el("p", "", item.summary));
      metricGrid([
        ["Kind", item.kind || item.type || "-"],
        ["Zone", item.zone || "-"],
        ["Incoming", String(counts.incoming)],
        ["Outgoing", String(counts.outgoing)],
        ["Icon", presentation.icon || "-"],
        ["Model", presentation.model || "-"]
      ]);
      rawDetails(item);
    }

    function showLink(link) {
      clear(link.label || link.id || "Relationship");
      if (link.summary) container.appendChild(el("p", "", link.summary));
      metricGrid([
        ["Kind", link.kind || "-"],
        ["From", link.from || "-"],
        ["To", link.to || "-"],
        ["Role", roleOf(link)],
        ["Path", pathGroupOf(link)]
      ]);
      rawDetails(link);
    }

    function showZone(zone) {
      var count = entities.filter(function (item) { return item && item.zone === zone.id; }).length;
      clear(itemLabel(zone) || zone.id || "Zone");
      if (zone.summary) container.appendChild(el("p", "", zone.summary));
      metricGrid([
        ["Kind", zone.kind || "zone"],
        ["Entities", String(count)],
        ["Boundary", zone.style || presentationOf(zone).boundary || "-"]
      ]);
      rawDetails(zone);
    }

    showSummary();
    return {
      show: function (label, payload) {
        if (!payload || payload.title === data.title && payload.entities === entities.length) {
          showSummary();
          return;
        }
        if (payload.from && payload.to) {
          showLink(payload);
          return;
        }
        if (payload.bounds || zoneByIDValue(payload.id)) {
          showZone(payload.bounds ? payload : zoneByIDValue(payload.id));
          return;
        }
        if (payload.id && entityByIDValue(payload.id)) {
          showEntity(payload);
          return;
        }
        clear(label || "Architecture");
        rawDetails(payload);
      }
    };
  }

  function createIsometricShell(container, manifest, data) {
    container.textContent = "";
    var renderHints = data && data.renderHints && typeof data.renderHints === "object" ? data.renderHints : {};
    var presentationMode = renderHints.presentationMode === true || renderHints.presentation_mode === true || renderHints.chrome === "presentation";
    var relationLayerMode = normalizeMarkKey(renderHints.relationLayer || renderHints.relation_layer || "world_ground") || "world_ground";
    var app = el("div", "visual-isometric-app");
    if (presentationMode) {
      app.classList.add("visual-isometric-presentation-mode");
      app.setAttribute("data-presentation-mode", "true");
    }
    if (relationLayerMode !== "svg_debug") {
      app.classList.add("visual-isometric-world-ground-relations");
    } else {
      app.classList.add("visual-isometric-svg-debug-relations");
    }
    app.setAttribute("data-isometric-renderer", "true");
    app.setAttribute("data-visual-template", "architecture.isometric_overview");
    app.setAttribute("data-visual-renderer", "offline.architecture.isometric.v1");
    app.setAttribute("data-architecture-light", "true");
    app.setAttribute("data-relation-layer-mode", relationLayerMode);
    var header = el("header", "visual-isometric-header");
    header.appendChild(el("h1", "visual-isometric-title", data.title || manifest.title || "Isometric Architecture"));
    header.appendChild(el("div", "visual-isometric-subtitle", data.subtitle || data.goal || "Offline Three.js architecture overview"));
    var controls = el("div", "visual-isometric-toolbar");
    controls.classList.add("visual-isometric-control-bar");
    var body = el("main", "visual-isometric-body");
    var stage = el("section", "visual-isometric-stage");
    stage.setAttribute("role", "application");
    stage.setAttribute("aria-label", "Interactive isometric architecture scene");
    var relation = createIsometricRelationLayer();
    var labelLayer = el("div", "visual-isometric-label-layer");
    var inspector = el("aside", "visual-isometric-inspector visual-inspector");
    inspector.setAttribute("aria-label", "Architecture inspector");
    stage.appendChild(relation.layer);
    stage.appendChild(labelLayer);
    body.appendChild(stage);
    body.appendChild(inspector);
    app.appendChild(header);
    app.appendChild(controls);
    app.appendChild(body);
    container.appendChild(app);
    return { app: app, controls: controls, stage: stage, relationLayer: relation.layer, relationSvg: relation.svg, labelLayer: labelLayer, inspector: createIsometricInspector(inspector, data) };
  }

  function isometricEntityGeometry(THREE, item, spec, size) {
    var kind = normalizeMarkKey(item.kind || item.type || spec.shape || spec.mesh);
    var mesh = normalizeMarkKey(spec.mesh || spec.shape);
    if (kind === "browser" || kind === "pc" || kind === "client" || kind === "user") {
      return new THREE.BoxGeometry(size.w * 0.32, size.h * 0.36, size.d * 0.26);
    }
    if (kind === "cdn") {
      return new THREE.SphereGeometry(Math.max(size.w, size.d) * 0.18, 28, 18);
    }
    if (kind === "redis" || kind === "cache") {
      return new THREE.BoxGeometry(size.w * 0.36, size.h * 0.2, size.d * 0.34);
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
    if (kind === "nginx" || kind === "gateway" || kind === "api_gateway" || kind === "ingress" || kind === "load_balancer" || mesh === "gateway_card" || mesh === "tower") {
      return new THREE.BoxGeometry(size.w * 0.36, size.h * 0.66, size.d * 0.28);
    }
    if (kind === "job" || kind === "jenkins" || kind === "admin") {
      return new THREE.BoxGeometry(size.w * 0.42, size.h * 0.36, size.d * 0.12);
    }
    if (kind === "log" || kind === "logs" || kind === "search" || kind === "elasticsearch" || kind === "prometheus" || kind === "grafana" || kind === "observability") {
      return new THREE.BoxGeometry(size.w * 0.38, size.h * 0.34, size.d * 0.24);
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

  var isometricBodyRegistryKinds = {
    browser: true,
    pc: true,
    client: true,
    user: true,
    mobile: true,
    cdn: true,
    nginx: true,
    gateway: true,
    api_gateway: true,
    ingress: true,
    load_balancer: true,
    service: true,
    microservice: true,
    api: true,
    registry: true,
    nacos: true,
    queue: true,
    event_stream: true,
    kafka: true,
    rocketmq: true,
    rabbitmq: true,
    redis: true,
    cache: true,
    database: true,
    mysql: true,
    postgres: true,
    mongodb: true,
    storage: true,
    oss: true,
    minio: true,
    file_storage: true,
    block_storage: true,
    log: true,
    logs: true,
    search: true,
    elasticsearch: true,
    prometheus: true,
    grafana: true,
    observability: true,
    admin: true,
    job: true,
    jenkins: true,
    kubernetes: true,
    cluster: true
  };

  function isKnownIsometricBodyKind(item, spec) {
    var kind = normalizeMarkKey(item && (item.kind || item.type) || spec && (spec.shape || spec.mesh) || "");
    var mesh = normalizeMarkKey(spec && (spec.mesh || spec.shape) || "");
    return !!(isometricBodyRegistryKinds[kind] || isometricBodyRegistryKinds[mesh]);
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
    if (kind === "cdn") {
      if (THREE.TorusGeometry) {
        var equator = new THREE.Mesh(new THREE.TorusGeometry(size.w * 0.2, 0.006, 6, 36), isometricDetailMaterial(THREE, "#dbeafe", 0.9));
        equator.rotation.x = Math.PI / 2;
        group.add(equator);
        var meridian = new THREE.Mesh(new THREE.TorusGeometry(size.w * 0.2, 0.006, 6, 36), isometricDetailMaterial(THREE, "#dbeafe", 0.82));
        meridian.rotation.y = Math.PI / 2;
        group.add(meridian);
      }
      return;
    }
    if (kind === "redis" || kind === "cache") {
      for (var layer = 0; layer < 3; layer += 1) {
        addIsometricBoxDetail(THREE, group, size.w * 0.42, size.h * 0.075, size.d * 0.34, 0, -size.h * 0.12 + layer * size.h * 0.13, 0, layer % 2 ? "#ef4444" : "#dc2626", 1);
        addIsometricBoxDetail(THREE, group, size.w * 0.09, size.h * 0.022, size.d * 0.03, -size.w * 0.13, -size.h * 0.105 + layer * size.h * 0.13, size.d * 0.19, "#fee2e2", 0.95);
      }
      return;
    }
    if (kind === "database" || kind === "mysql" || kind === "postgres" || kind === "mongodb" || kind === "redis" || kind === "cache") {
      addIsometricCylinderDetail(THREE, group, size.w * 0.2, 0.04, 0, size.h * 0.23, 0, "#ffffff", 0.72, 36);
      addIsometricCylinderDetail(THREE, group, size.w * 0.19, 0.025, 0, size.h * 0.04, 0, "#eff6ff", 0.44, 30);
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
    if (kind === "api_gateway" || kind === "gateway" || kind === "nginx" || kind === "ingress" || kind === "load_balancer") {
      addIsometricBoxDetail(THREE, group, size.w * 0.14, size.h * 0.5, size.d * 0.06, -size.w * 0.17, size.h * 0.02, size.d * 0.2, "#0f172a", 0.82);
      addIsometricBoxDetail(THREE, group, size.w * 0.14, size.h * 0.5, size.d * 0.06, size.w * 0.17, size.h * 0.02, size.d * 0.2, "#0f172a", 0.82);
      addIsometricBoxDetail(THREE, group, size.w * 0.42, size.h * 0.05, size.d * 0.09, 0, size.h * 0.31, size.d * 0.18, "#d1fae5", 0.9);
      addIsometricBoxDetail(THREE, group, size.w * 0.12, size.h * 0.025, size.d * 0.035, -size.w * 0.11, size.h * 0.04, size.d * 0.23, "#fbbf24", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.12, size.h * 0.025, size.d * 0.035, size.w * 0.11, size.h * 0.04, size.d * 0.23, "#60a5fa", 1);
      return;
    }
    if (kind === "service" || kind === "microservice" || kind === "api") {
      for (var block = 0; block < 2; block += 1) {
        addIsometricBoxDetail(THREE, group, size.w * 0.32, size.h * 0.12, size.d * 0.25, 0, -size.h * 0.06 + block * size.h * 0.17, 0, block % 2 ? "#60a5fa" : "#2563eb", 0.95);
      }
      addIsometricBoxDetail(THREE, group, size.w * 0.28, size.h * 0.04, size.d * 0.22, 0, size.h * 0.19, size.d * 0.17, "#ffffff", 0.45);
      addIsometricBoxDetail(THREE, group, size.w * 0.08, size.h * 0.025, size.d * 0.03, -size.w * 0.1, size.h * 0.07, size.d * 0.18, "#fbbf24", 1);
      addIsometricBoxDetail(THREE, group, size.w * 0.08, size.h * 0.025, size.d * 0.03, size.w * 0.08, size.h * 0.07, size.d * 0.18, "#60a5fa", 1);
      return;
    }
    if (kind === "registry" || kind === "nacos" || kind === "event_stream" || kind === "queue" || kind === "kafka" || kind === "rocketmq" || kind === "rabbitmq") {
      addIsometricCylinderDetail(THREE, group, size.w * 0.24, 0.035, 0, -size.h * 0.22, 0, "#ffffff", 0.88, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.16, 0.025, 0, -size.h * 0.18, 0, "#bfdbfe", 0.76, 30);
      addIsometricCylinderDetail(THREE, group, size.w * 0.055, 0.06, -size.w * 0.08, size.h * 0.18, size.d * 0.02, "#22d3ee", 1, 14);
      addIsometricCylinderDetail(THREE, group, size.w * 0.055, 0.06, size.w * 0.08, size.h * 0.18, size.d * 0.02, "#22d3ee", 1, 14);
      return;
    }
    if (kind === "kubernetes" || kind === "cluster") {
      if (THREE.TorusGeometry) {
        var ring = new THREE.Mesh(new THREE.TorusGeometry(size.w * 0.2, 0.018, 8, 32), isometricDetailMaterial(THREE, "#dbeafe", 0.9));
        ring.rotation.x = Math.PI / 2;
        ring.position.y = size.h * 0.12;
        group.add(ring);
      }
      for (var node = 0; node < 6; node += 1) {
        var angle = node / 6 * Math.PI * 2;
        addIsometricCylinderDetail(THREE, group, size.w * 0.025, 0.035, Math.cos(angle) * size.w * 0.16, size.h * 0.13, Math.sin(angle) * size.d * 0.16, "#bfdbfe", 0.92, 10);
      }
      return;
    }
    if (kind === "job" || kind === "jenkins" || kind === "admin") {
      addIsometricBoxDetail(THREE, group, size.w * 0.34, size.h * 0.035, size.d * 0.1, 0, size.h * 0.18, size.d * 0.09, "#fee2e2", 0.9);
      for (var tooth = 0; tooth < 8; tooth += 1) {
        var theta = tooth / 8 * Math.PI * 2;
        addIsometricBoxDetail(THREE, group, size.w * 0.035, size.h * 0.06, size.d * 0.025, Math.cos(theta) * size.w * 0.14, size.h * 0.04, size.d * 0.13 + Math.sin(theta) * size.d * 0.035, "#fecaca", 0.9);
      }
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
    var knownBody = isKnownIsometricBodyKind(item, spec);
    group.userData = { label: itemLabel(item), payload: item, id: item.id, mark: { icon: spec.icon || "", model: spec.model || "", modelPath: modelPathFor(spec, markContext) || "" }, entityBodyKnown: knownBody, entityBodyKind: normalizeMarkKey(item.kind || item.type || spec.shape || spec.mesh) || "default" };
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
    return createDashedLeader(THREE, new THREE.Vector3(x, yStart, z), new THREE.Vector3(x, yEnd, z));
  }

  function createDashedLeader(THREE, fromPoint, toPoint) {
    var leader = new THREE.Group();
    var delta = toPoint.clone().sub(fromPoint);
    var length = delta.length();
    if (length < 0.01) {
      return leader;
    }
    var direction = delta.clone().normalize();
    var material = new THREE.MeshBasicMaterial({ color: 0x64748b, transparent: true, opacity: 0.58 });
    var radius = 0.006;
    var dash = 0.085;
    var gap = 0.075;
    for (var cursor = 0; cursor < length; cursor += dash + gap) {
      var height = Math.min(dash, length - cursor);
      if (height <= 0.002) continue;
      var geometry = isometricCylinderGeometry(THREE, radius, height, 8);
      var segment = new THREE.Mesh(geometry, material.clone());
      segment.position.copy(fromPoint.clone().add(direction.clone().multiplyScalar(cursor + height / 2)));
      segment.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), direction);
      segment.userData.isLeaderLine = true;
      leader.add(segment);
    }
    leader.userData.isLeaderLine = true;
    return leader;
  }

  function roundedZoneBoundaryWorldPoints(THREE, bounds, scale, center, radius, segments) {
    radius = Math.max(0, Math.min(radius || 0, Math.min(bounds.w, bounds.h) * 0.22));
    segments = Math.max(3, segments || 5);
    if (radius <= 0.001) {
      return [
        isometricWorld({ x: bounds.x, y: bounds.y }, scale, center),
        isometricWorld({ x: bounds.x + bounds.w, y: bounds.y }, scale, center),
        isometricWorld({ x: bounds.x + bounds.w, y: bounds.y + bounds.h }, scale, center),
        isometricWorld({ x: bounds.x, y: bounds.y + bounds.h }, scale, center),
        isometricWorld({ x: bounds.x, y: bounds.y }, scale, center)
      ].map(function (p) { return new THREE.Vector3(p.x, 0.035, p.z); });
    }
    var corners = [
      { cx: bounds.x + radius, cy: bounds.y + radius, a0: Math.PI, a1: Math.PI * 1.5 },
      { cx: bounds.x + bounds.w - radius, cy: bounds.y + radius, a0: Math.PI * 1.5, a1: Math.PI * 2 },
      { cx: bounds.x + bounds.w - radius, cy: bounds.y + bounds.h - radius, a0: 0, a1: Math.PI * 0.5 },
      { cx: bounds.x + radius, cy: bounds.y + bounds.h - radius, a0: Math.PI * 0.5, a1: Math.PI }
    ];
    var out = [];
    corners.forEach(function (corner) {
      for (var i = 0; i <= segments; i += 1) {
        var t = i / segments;
        var angle = corner.a0 + (corner.a1 - corner.a0) * t;
        var p = isometricWorld({ x: corner.cx + Math.cos(angle) * radius, y: corner.cy + Math.sin(angle) * radius }, scale, center);
        out.push(new THREE.Vector3(p.x, 0.035, p.z));
      }
    });
    if (out.length) out.push(out[0].clone());
    return out;
  }

  function addIsometricZone(THREE, root, zone, scale, center) {
    var bounds = isometricBounds(zone);
    var world = isometricWorld({ x: bounds.x + bounds.w / 2, y: bounds.y + bounds.h / 2 }, scale, center);
    var presentation = zone && zone.presentation && typeof zone.presentation === "object" ? zone.presentation : {};
    var color = colorValue(presentation.fill || presentation.color || "#e6edf5", 0xe6edf5);
    var plane = new THREE.Mesh(new THREE.PlaneGeometry(bounds.w * scale, bounds.h * scale), new THREE.MeshBasicMaterial({
      color: color,
      transparent: true,
      opacity: numberValue(presentation.fillOpacity || presentation.fill_opacity, 0.065),
      depthWrite: false
    }));
    plane.rotation.x = -Math.PI / 2;
    plane.position.set(world.x, 0.006, world.z);
    plane.userData = { zone: zone.id, label: itemLabel(zone) };
    root.add(plane);
    var points = roundedZoneBoundaryWorldPoints(THREE, bounds, scale, center, numberValue(presentation.cornerRadius || presentation.corner_radius, 0.54), 5);
    var boundaryStyle = normalizeMarkKey(presentation.boundary || presentation.lineStyle || zone.style || "solid");
    var boundaryColor = presentation.boundaryColor || presentation.borderColor || presentation.color || zone.color || "#111827";
    var boundary = boundaryStyle === "dashed" || boundaryStyle === "dash" ? createDashedPolyline(THREE, points, boundaryColor, 0.86, 0.26, 0.16) : new THREE.Line(new THREE.BufferGeometry().setFromPoints(points), isometricLineMaterial(THREE, boundaryColor, 0.82));
    boundary.userData = { isZoneBoundary: true, style: boundaryStyle };
    root.add(boundary);
    var labelPoint = presentation.labelPoint || presentation.label_point || {};
    return {
      zone: zone,
      bounds: bounds,
      labelPoint: {
        x: labelPoint.x !== undefined ? numberValue(labelPoint.x, bounds.x + Math.min(bounds.w - 0.45, 0.45)) : bounds.x + Math.min(bounds.w - 0.45, 0.45),
        y: labelPoint.y !== undefined ? numberValue(labelPoint.y, bounds.y + 0.45) : bounds.y + 0.45
      },
      plane: plane,
      boundary: boundary
    };
  }

  function compactLabelText(text, maxLength) {
    var value = String(text || "").trim();
    maxLength = maxLength || 28;
    return value.length > maxLength ? value.slice(0, Math.max(8, maxLength - 1)) + "…" : value;
  }

  function labelLimit(className) {
    className = String(className || "");
    if (className.indexOf("link") >= 0) return 22;
    if (className.indexOf("zone") >= 0) return 24;
    return 28;
  }

  function labelHTML(className, text) {
    var fullText = String(text || "");
    var node = el("div", className, compactLabelText(fullText, labelLimit(className)));
    node.title = fullText;
    node.setAttribute("data-label", fullText);
    return node;
  }

  function setLabelAnchorMetadata(node, point, offset, entityTop) {
    if (!node || !point) return;
    node.setAttribute("data-anchor-x", String(Math.round(point.x * 1000) / 1000));
    node.setAttribute("data-anchor-y", String(Math.round(point.y * 1000) / 1000));
    node.setAttribute("data-anchor-z", String(Math.round(point.z * 1000) / 1000));
    if (offset) {
      node.setAttribute("data-label-offset-x", String(Math.round(numberValue(offset.x, 0) * 1000) / 1000));
      node.setAttribute("data-label-offset-y", String(Math.round(numberValue(offset.y, 0) * 1000) / 1000));
      node.setAttribute("data-label-offset-z", String(Math.round(numberValue(offset.z, 0) * 1000) / 1000));
    }
    if (entityTop) {
      node.setAttribute("data-entity-top-x", String(Math.round(entityTop.x * 1000) / 1000));
      node.setAttribute("data-entity-top-y", String(Math.round(entityTop.y * 1000) / 1000));
      node.setAttribute("data-entity-top-z", String(Math.round(entityTop.z * 1000) / 1000));
    }
  }

  function drawRoundedRect(ctx, x, y, w, h, r) {
    var radius = Math.max(0, Math.min(r || 0, Math.min(w, h) / 2));
    ctx.beginPath();
    ctx.moveTo(x + radius, y);
    ctx.lineTo(x + w - radius, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + radius);
    ctx.lineTo(x + w, y + h - radius);
    ctx.quadraticCurveTo(x + w, y + h, x + w - radius, y + h);
    ctx.lineTo(x + radius, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - radius);
    ctx.lineTo(x, y + radius);
    ctx.quadraticCurveTo(x, y, x + radius, y);
    ctx.closePath();
  }

  function createTextTexture(THREE, text, options) {
    options = options || {};
    var value = compactLabelText(text || "", options.maxLength || 22);
    var dpr = Math.max(2, Math.min(3, window.devicePixelRatio || 2));
    var fontSize = options.fontSize || 24;
    var weight = options.weight || 800;
    var paddingX = options.paddingX || 12;
    var paddingY = options.paddingY || 7;
    var scratch = document.createElement("canvas").getContext("2d");
    scratch.font = weight + " " + fontSize + "px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
    var textWidth = Math.ceil(scratch.measureText(value).width);
    var widthPx = Math.max(options.minWidth || 58, Math.min(options.maxWidth || 260, textWidth + paddingX * 2));
    var heightPx = Math.max(options.minHeight || 34, fontSize + paddingY * 2);
    var canvas = document.createElement("canvas");
    canvas.width = Math.ceil(widthPx * dpr);
    canvas.height = Math.ceil(heightPx * dpr);
    var ctx = canvas.getContext("2d");
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, widthPx, heightPx);
    ctx.shadowColor = options.shadowColor || "rgba(15, 23, 42, 0.14)";
    ctx.shadowBlur = options.shadowBlur === undefined ? 8 : options.shadowBlur;
    ctx.shadowOffsetY = options.shadowOffsetY === undefined ? 3 : options.shadowOffsetY;
    drawRoundedRect(ctx, 1, 1, widthPx - 2, heightPx - 2, options.radius || 6);
    ctx.fillStyle = options.background || "rgba(255, 255, 255, 0.96)";
    ctx.fill();
    ctx.shadowColor = "transparent";
    ctx.lineWidth = options.borderWidth || 1;
    ctx.strokeStyle = options.border || "rgba(100, 116, 139, 0.22)";
    ctx.stroke();
    ctx.fillStyle = options.color || "#111827";
    ctx.font = weight + " " + fontSize + "px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(value, widthPx / 2, heightPx / 2 + 0.5);
    var TextureCtor = THREE.CanvasTexture || THREE.Texture;
    if (!TextureCtor) {
      return {
        texture: null,
        text: value,
        widthPx: widthPx,
        heightPx: heightPx,
        ready: false
      };
    }
    var texture = new TextureCtor(canvas);
    texture.needsUpdate = true;
    if (THREE.SRGBColorSpace) texture.colorSpace = THREE.SRGBColorSpace;
    if (THREE.LinearFilter) {
      texture.minFilter = THREE.LinearFilter;
      texture.magFilter = THREE.LinearFilter;
    }
    return {
      texture: texture,
      text: value,
      widthPx: widthPx,
      heightPx: heightPx,
      ready: true
    };
  }

  function disposeMaterialMap(material) {
    if (!material) return;
    if (material.map && material.map.dispose) material.map.dispose();
    if (material.dispose) material.dispose();
  }

  function isometricEntityLabelOffset(index, size) {
    var pattern = [
      { x: -0.22, z: -0.18, y: 0.06 },
      { x: 0.22, z: -0.2, y: 0.12 },
      { x: -0.28, z: 0.18, y: 0.18 },
      { x: 0.28, z: 0.2, y: 0.08 },
      { x: 0, z: -0.3, y: 0.16 },
      { x: 0, z: 0.3, y: 0.1 }
    ][index % 6];
    return {
      x: pattern.x * Math.max(0.8, size.w),
      z: pattern.z * Math.max(0.8, size.d),
      y: pattern.y
    };
  }

  function explicitIsometricLabelOffset(item) {
    var presentation = item && item.presentation ? item.presentation : {};
    var raw = presentation.label_offset || presentation.labelOffset;
    if (!raw || typeof raw !== "object") {
      return null;
    }
    return {
      x: numberValue(raw.x, 0),
      z: numberValue(raw.z, 0),
      y: numberValue(raw.y !== undefined ? raw.y : raw.height, 0)
    };
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
    var sourceLinks = links.slice();
    var routePlan = data && (data.routePlan || data.route_plan) && typeof (data.routePlan || data.route_plan) === "object" ? (data.routePlan || data.route_plan) : {};
    var routePlanSourceEdges = Array.isArray(routePlan.sourceEdges) ? routePlan.sourceEdges : [];
    var routePlanDisplayRoutes = Array.isArray(routePlan.displayRoutes) ? routePlan.displayRoutes : [];
    var routePlanHiddenDetailRoutes = Array.isArray(routePlan.hiddenDetailRoutes) ? routePlan.hiddenDetailRoutes : [];
    var routePlanRoutes = routePlanDisplayRoutes.length ? routePlanDisplayRoutes : (Array.isArray(routePlan.routes) ? routePlan.routes : []);
    var routePlanLanes = Array.isArray(routePlan.lanes) ? routePlan.lanes : [];
    var routePlanObstacles = Array.isArray(routePlan.obstacles) ? routePlan.obstacles : [];
    var routePlanRoutesByID = {};
    routePlanRoutes.forEach(function (planned, index) {
      if (!planned || typeof planned !== "object") return;
      var id = String(planned.id || "");
      if (id) routePlanRoutesByID[id] = planned;
      if (planned.from && planned.to) routePlanRoutesByID[String(planned.from) + "->" + String(planned.to)] = planned;
      routePlanRoutesByID["link_" + String(index + 1).padStart(2, "0")] = planned;
    });
    function visualLinkFromRoutePlanRoute(planned, index) {
      planned = planned || {};
      var style = planned.style && typeof planned.style === "object" ? planned.style : {};
      var link = {
        id: planned.id || ("display_route_" + String(index + 1).padStart(2, "0")),
        from: planned.fromEntity || planned.from || "",
        to: planned.toEntity || planned.to || "",
        label: planned.label || "",
        kind: planned.pathGroup || planned.kind || "depends_on",
        directed: planned.directed !== false,
        role: planned.role || "secondary",
        pathGroup: planned.pathGroup || planned.path_group || "",
        routeScope: planned.routeScope || planned.route_scope || "",
        terminalMode: planned.terminalMode || planned.terminal_mode || "",
        fromZone: planned.fromZone || planned.from_zone || "",
        toZone: planned.toZone || planned.to_zone || "",
        sourceEdgeIDs: Array.isArray(planned.sourceEdgeIDs) ? planned.sourceEdgeIDs.slice() : [],
        detailRouteIDs: Array.isArray(planned.detailRouteIDs) ? planned.detailRouteIDs.slice() : [],
        routeStyle: "orthogonal",
        route: Array.isArray(planned.points) ? planned.points : [],
        style: style,
        presentation: {
          arrow: planned.arrow || "forward",
          lineStyle: Array.isArray(style.dashPattern) && style.dashPattern.length ? "dashed" : "solid",
          color: style.bodyColor || style.color || "#475569",
          role: planned.role || "secondary",
          pathGroup: planned.pathGroup || planned.path_group || "",
          parallelOffset: planned.parallelOffset || planned.parallel_offset || 0
        },
        metadata: {
          route_stage: "RoutePlanDisplayRoute",
          route_scope: planned.routeScope || planned.route_scope || "",
          terminal_mode: planned.terminalMode || planned.terminal_mode || "",
          source_edge_ids: Array.isArray(planned.sourceEdgeIDs) ? planned.sourceEdgeIDs.slice() : []
        },
        __routePlan: planned,
        busLaneId: planned.busLaneId || planned.bus_lane_id || "",
        bundleId: planned.bundleId || planned.bundle_id || "",
        parallelOffset: planned.parallelOffset || planned.parallel_offset || 0
      };
      return link;
    }
    if (routePlanRoutes.length) {
      links = routePlanRoutes.map(visualLinkFromRoutePlanRoute);
    }
    var zoneByID = {};
    zones.forEach(function (zone) { zoneByID[zone.id] = zone; });
    var allBounds = zones.length ? zones.map(isometricBounds) : [{ x: 0, y: 0, w: 18, h: 12 }];
    var minX = Math.min.apply(null, allBounds.map(function (b) { return b.x; }));
    var minY = Math.min.apply(null, allBounds.map(function (b) { return b.y; }));
    var maxX = Math.max.apply(null, allBounds.map(function (b) { return b.x + b.w; }));
    var maxY = Math.max.apply(null, allBounds.map(function (b) { return b.y + b.h; }));
    var center = { x: (minX + maxX) / 2, y: (minY + maxY) / 2 };
    var span = Math.max(8, maxX - minX, maxY - minY);
    var inputRenderHints = data && data.renderHints && typeof data.renderHints === "object" ? data.renderHints : {};
    var relationLayerMode = normalizeMarkKey(inputRenderHints.relationLayer || inputRenderHints.relation_layer || "world_ground") || "world_ground";
    var svgRelationEnabled = relationLayerMode === "svg_debug";
    var linkLabelMode = normalizeMarkKey(inputRenderHints.linkLabelMode || inputRenderHints.link_label_mode || "html_billboard") || "html_billboard";
    var linkLabelReadabilityFlip = normalizeMarkKey(inputRenderHints.linkLabelReadabilityFlip || inputRenderHints.link_label_readability_flip || "camera_idle") || "camera_idle";
    var linkLabelMaxVisible = Math.max(0, Math.min(20, numberValue(inputRenderHints.linkLabelMaxVisible || inputRenderHints.link_label_max_visible, 7)));
    var layoutScale = Math.max(0.85, Math.min(2.25, numberValue(inputRenderHints.layoutScale || inputRenderHints.layout_scale, 1)));
    var scale = (8.65 * layoutScale) / span;
    links.forEach(function (link, index) {
      if (!link || typeof link !== "object") return;
      var id = String(link.id || "");
      var planned = id && routePlanRoutesByID[id] || routePlanRoutesByID[String(link.from || "") + "->" + String(link.to || "")] || routePlanRoutesByID["link_" + String(index + 1).padStart(2, "0")];
      if (!planned) return;
      link.__routePlan = planned;
      link.busLaneId = link.busLaneId || link.bus_lane_id || planned.busLaneId || planned.bus_lane_id || "";
      link.bundleId = link.bundleId || link.bundle_id || planned.bundleId || planned.bundle_id || "";
      if (planned.parallelOffset !== undefined || planned.parallel_offset !== undefined) {
        link.parallelOffset = planned.parallelOffset !== undefined ? planned.parallelOffset : planned.parallel_offset;
      }
    });
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
    var inputCamera = data.camera || {};
    var initialZoom = Math.max(0.72, Math.min(1.55, numberValue(inputCamera.zoom, 1.02)));
    var initialTheta = Math.max(-Math.PI, Math.min(Math.PI, numberValue(inputCamera.theta, Math.PI / 4)));
    var initialPhi = Math.max(0.46, Math.min(1.36, numberValue(inputCamera.phi, Math.PI / 3.28)));
    var initialRadius = Math.max(7.5, Math.min(18, numberValue(inputCamera.radius, 11)));
    var cameraState = { theta: initialTheta, phi: initialPhi, radius: initialRadius, panX: 0, panZ: 0, zoom: initialZoom };
    var root = new THREE.Group();
    var zoneRoot = new THREE.Group();
    var entityRoot = new THREE.Group();
    var linkRoot = new THREE.Group();
    var relationLabelRoot = new THREE.Group();
    var leaderRoot = new THREE.Group();
    root.add(zoneRoot);
    root.add(linkRoot);
    root.add(relationLabelRoot);
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
    var base = new THREE.Mesh(new THREE.PlaneGeometry((maxX - minX + 3.4) * scale, (maxY - minY + 3.4) * scale), new THREE.MeshBasicMaterial({ color: 0xf8fafc }));
    base.rotation.x = -Math.PI / 2;
    base.position.y = -0.018;
    base.renderOrder = -30;
    base.userData.isBasePlane = true;
    root.add(base);
    var grid = addThreeGrid(THREE, root, Math.max(10, span * scale + 1.8), Math.max(10, Math.ceil(span)), 0.002, 0xcbd5e1);
    grid.userData.isIsometricGrid = true;

    var labels = [];
    var entityByID = {};
    var entityComponents = [];
    var relationComponents = [];
    var labelComponents = [];
    var leaderLineComponents = [];
    var markContext = createMarkContext(manifest, data);
    var visualHints = readVisualHints(data);
    var viewMode = normalizeMarkKey(visualHints.labelMode || "overview") || "overview";
    var isAssetGallery = String(data.title || "").toLowerCase().indexOf("logo badge gallery") >= 0;
    var isMermaidArchitecture = normalizeMarkKey(manifest.id || data.template_id || data.template || "") === "mermaid.architecture";
    function isOverviewMode() {
      return viewMode !== "detail";
    }
    function shouldShowIsometricEntityLabel(item) {
      var q = normalizeVisualQualityFields(item);
      if (!isOverviewMode()) return q.visibility !== "hidden" && q.labelPriority !== "hidden";
      if (q.visibility === "hidden" || q.labelPriority === "hidden") return false;
      if (q.visibility === "detail" || q.labelPriority === "hover") return false;
      return q.importance >= 0.58 || q.labelPriority === "always" || q.labelPriority === "important";
    }
    function HtmlLabelComponent(options) {
      options = options || {};
      this.id = options.id || "";
      this.type = options.type || "label";
      this.model = options.model || {};
      this.dom = options.element || null;
      this.anchorWorld = options.anchorWorld || new THREE.Vector3();
      this.visible = options.visible !== false;
      this.group = null;
    }
    HtmlLabelComponent.prototype.mount = function (parent) {
      if (parent && this.dom && this.dom.parentNode !== parent) parent.appendChild(this.dom);
      this.group = parent || null;
      return this;
    };
    HtmlLabelComponent.prototype.update = function (anchorWorld) {
      if (anchorWorld) this.anchorWorld.copy(anchorWorld);
      return this;
    };
    HtmlLabelComponent.prototype.updateProjection = function (cameraObject, activeRenderer, container) {
      if (!this.dom) return;
      var point = projectWorldToScreen(this.anchorWorld, cameraObject, activeRenderer, container);
      var mode = this.type === "entity" ? "translate(-50%, -100%)" : "translate(-50%, -50%)";
      this.dom.style.transform = "translate3d(" + point.x.toFixed(2) + "px, " + point.y.toFixed(2) + "px, 0) " + mode;
    };
    HtmlLabelComponent.prototype.setVisible = function (visible) {
      this.visible = !!visible;
      if (this.dom) {
        this.dom.style.display = this.visible ? "" : "none";
        this.dom.style.visibility = this.visible ? "visible" : "hidden";
        this.dom.style.opacity = this.visible ? "1" : "0";
      }
      return this;
    };
    HtmlLabelComponent.prototype.setContent = function (text) {
      if (this.dom) this.dom.textContent = text || "";
      return this;
    };
    HtmlLabelComponent.prototype.dispose = function () {
      if (this.dom && this.dom.parentNode) this.dom.parentNode.removeChild(this.dom);
      this.dom = null;
    };
    function LeaderLineComponent(id, group, model) {
      this.id = id || "";
      this.model = model || {};
      this.group = group || null;
    }
    LeaderLineComponent.prototype.mount = function (parent) {
      if (parent && this.group && this.group.parent !== parent) parent.add(this.group);
      return this;
    };
    LeaderLineComponent.prototype.update = function (nextGroup) {
      if (nextGroup) this.group = nextGroup;
      return this;
    };
    LeaderLineComponent.prototype.setState = function (state) {
      if (this.group && state && state.visible !== undefined) this.group.visible = !!state.visible;
      return this;
    };
    LeaderLineComponent.prototype.dispose = function () {
      if (this.group && this.group.parent) this.group.parent.remove(this.group);
      this.group = null;
    };
    function EntityComponent(context, model, group, record, labelComponent, leaderComponent) {
      this.id = model && model.id || "";
      this.model = model || {};
      this.group = group || null;
      this.body = group || null;
      this.record = record || null;
      this.labelComponent = labelComponent || null;
      this.leaderLineComponent = leaderComponent || null;
      this.badgeComponent = null;
      this.bbox = record ? computeEntityVisualBounds(record) : null;
      this.ports = null;
      this.anchors = {
        topLabel: record && record.labelAnchorWorld ? record.labelAnchorWorld.clone() : null,
        top: record && record.entityTopWorld ? record.entityTopWorld.clone() : null
      };
      this.context = context || {};
    }
    EntityComponent.prototype.mount = function (parent) {
      if (parent && this.group && this.group.parent !== parent) parent.add(this.group);
      if (this.labelComponent) this.labelComponent.mount(shell.labelLayer);
      if (this.leaderLineComponent) this.leaderLineComponent.mount(leaderRoot);
      return this;
    };
    EntityComponent.prototype.update = function () {
      if (!this.record) return this;
      this.bbox = computeEntityVisualBounds(this.record);
      this.ports = computeEntityPorts(this.record);
      this.anchors.top = this.record.entityTopWorld ? this.record.entityTopWorld.clone() : this.bbox.topCenter.clone();
      this.anchors.topLabel = this.record.labelAnchorWorld ? this.record.labelAnchorWorld.clone() : this.bbox.topCenter.clone();
      if (this.labelComponent) this.labelComponent.update(this.anchors.topLabel);
      return this;
    };
    EntityComponent.prototype.setState = function (state) {
      state = state || {};
      if (this.group && state.visible !== undefined) this.group.visible = !!state.visible;
      if (this.labelComponent && state.labelVisible !== undefined) this.labelComponent.setVisible(!!state.labelVisible);
      if (this.leaderLineComponent && state.labelVisible !== undefined) this.leaderLineComponent.setState({ visible: !!state.labelVisible });
      return this;
    };
    EntityComponent.prototype.dispose = function () {
      if (this.group && this.group.parent) this.group.parent.remove(this.group);
      if (this.labelComponent) this.labelComponent.dispose();
      if (this.leaderLineComponent) this.leaderLineComponent.dispose();
      this.group = null;
    };
    function computeEntityVisualBounds(record) {
      var object = record && record.object;
      var size = record && record.size ? record.size : { w: 0.9, h: 0.9, d: 0.9 };
      if (object && THREE.Box3) {
        var box = new THREE.Box3().setFromObject(object);
        if (box && box.min && box.max && Number.isFinite(box.min.x) && Number.isFinite(box.max.y)) {
          return {
            center: new THREE.Vector3((box.min.x + box.max.x) / 2, (box.min.y + box.max.y) / 2, (box.min.z + box.max.z) / 2),
            topCenter: new THREE.Vector3((box.min.x + box.max.x) / 2, box.max.y, (box.min.z + box.max.z) / 2),
            minX: box.min.x,
            maxX: box.max.x,
            minZ: box.min.z,
            maxZ: box.max.z,
            topY: box.max.y
          };
        }
      }
      var scaleY = object && object.scale ? numberValue(object.scale.y, 1) : 1;
      var topY = object ? object.position.y + Math.max(0.36, size.h * 0.54 * scaleY) : Math.max(0.36, size.h * 0.54);
      var halfW = Math.max(0.22, size.w * 0.42 * (object && object.scale ? numberValue(object.scale.x, 1) : 1));
      var halfD = Math.max(0.22, size.d * 0.42 * (object && object.scale ? numberValue(object.scale.z, 1) : 1));
      var center = object ? object.position.clone() : new THREE.Vector3();
      return {
        center: center.clone(),
        topCenter: new THREE.Vector3(center.x, topY, center.z),
        minX: center.x - halfW,
        maxX: center.x + halfW,
        minZ: center.z - halfD,
        maxZ: center.z + halfD,
        topY: topY
      };
    }
    function entityLabelGap(record) {
      var size = record && record.size ? record.size : { h: 0.9 };
      return Math.max(0.42, Math.min(0.68, 0.38 + size.h * 0.14));
    }
    function entityLabelAnchor(record) {
      var bounds = computeEntityVisualBounds(record);
      return {
        top: bounds.topCenter,
        anchor: bounds.topCenter.clone().add(new THREE.Vector3(0, entityLabelGap(record), 0))
      };
    }
    zones.forEach(function (zone) {
      var info = addIsometricZone(THREE, zoneRoot, zone, scale, center);
      var pos = isometricWorld(info.labelPoint, scale, center);
      var label = labelHTML("visual-isometric-zone-label", itemLabel(zone) || zone.id);
      label.setAttribute("data-zone-label", zone.id || "");
      label.setAttribute("data-zone-id", zone.id || "");
      shell.labelLayer.appendChild(label);
      var zoneAnchor = new THREE.Vector3(pos.x, 0.2, pos.z);
      setLabelAnchorMetadata(label, zoneAnchor);
      labels.push({ element: label, point: zoneAnchor, visible: true, type: "zone", priority: 1.45 });
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
      var entityRecord = {
        object: object,
        item: item,
        pos: pos,
        baseWorld: new THREE.Vector3(world.x, size.h * 0.22, world.z),
        world: new THREE.Vector3(world.x, size.h * 0.22, world.z),
        velocity: new THREE.Vector3(),
        size: size,
        labelRecord: null,
        leader: null,
        leaderOffset: null
      };
      entityByID[item.id] = entityRecord;
      var settings = badgeSettings(markContext);
      var label = labelHTML("visual-isometric-label visual-isometric-entity-label", itemLabel(item) || item.id);
      label.setAttribute("data-entity-label", item.id || "");
      label.setAttribute("data-entity-id", item.id || "");
      label.setAttribute("data-entity-kind", item.kind || "");
      label.setAttribute("data-icon-id", spec.icon || "");
      label.setAttribute("data-model-id", spec.model || "");
      label.setAttribute("data-label-priority", normalizeVisualQualityFields(item).labelPriority || "");
      label.setAttribute("data-visibility", normalizeVisualQualityFields(item).visibility || "");
      label.setAttribute("data-badge-mode", settings.mode);
      label.setAttribute("data-has-label-icon", iconPathFor(spec, markContext) && settings.labelIcon ? "true" : "false");
      label.setAttribute("data-has-model-badge", modelPathFor(spec, markContext) && settings.mode !== "none" && settings.mode !== "icon" ? "true" : "false");
      label.setAttribute("data-has-svg-billboard", iconPathFor(spec, markContext) && settings.mode !== "none" && settings.mode !== "model" ? "true" : "false");
      var inlineIcon = settings.labelIcon ? createInlineIcon(spec, markContext, "visual-isometric-label-icon") : null;
      if (inlineIcon) {
        label.textContent = "";
        label.appendChild(inlineIcon);
        label.appendChild(el("span", "", compactLabelText(itemLabel(item) || item.id, 28)));
      }
      shell.labelLayer.appendChild(label);
      var offset = { x: 0, z: 0, y: 0 };
      var anchorInfo = entityLabelAnchor(entityRecord);
      var anchor = anchorInfo.anchor;
      var entityQuality = normalizeVisualQualityFields(item);
      var entityPriority = 0.84 + importanceValue(item, 0.45) * 0.32;
      if (entityQuality.labelPriority === "always") {
        entityPriority += 0.55;
      } else if (entityQuality.labelPriority === "important") {
        entityPriority += 0.28;
      }
      var entityLabelComponent = new HtmlLabelComponent({
        id: item.id || "",
        type: "entity",
        model: item,
        element: label,
        anchorWorld: anchor.clone(),
        visible: shouldShowIsometricEntityLabel(item)
      }).mount(shell.labelLayer);
      labelComponents.push(entityLabelComponent);
      var labelRecord = { element: label, point: anchor, visible: shouldShowIsometricEntityLabel(item), type: "entity", priority: entityPriority, id: item.id, entityID: item.id, offset: offset };
      labelRecord.component = entityLabelComponent;
      setLabelAnchorMetadata(label, anchor, offset, anchorInfo.top);
      labels.push(labelRecord);
      var leader = createDashedLeader(THREE, anchorInfo.top, anchor.clone().add(new THREE.Vector3(0, -0.08, 0)));
      leader.visible = shouldShowIsometricEntityLabel(item);
      leaderRoot.add(leader);
      var leaderComponent = new LeaderLineComponent(item.id || "", leader, { entityID: item.id || "" });
      leaderLineComponents.push(leaderComponent);
      entityRecord.labelRecord = labelRecord;
      entityRecord.leader = leader;
      entityRecord.leaderOffset = offset;
      entityRecord.entityTopWorld = anchorInfo.top;
      entityRecord.labelAnchorWorld = anchor;
      entityRecord.component = new EntityComponent({ THREE: THREE }, item, object, entityRecord, entityLabelComponent, leaderComponent);
      entityComponents.push(entityRecord.component);
    });

    var laneCounters = {};
    var relationLinks = [];
    var groundLinkLabels = [];
    var linkLabelsCreated = 0;
    function linkRole(link) {
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      var metadata = link && link.metadata && typeof link.metadata === "object" ? link.metadata : {};
      var role = normalizeMarkKey(link && (link.role || presentation.role || metadata.role));
      return role === "primary" || role === "secondary" || role === "auxiliary" ? role : "";
    }
    function linkPathGroup(link) {
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      var metadata = link && link.metadata && typeof link.metadata === "object" ? link.metadata : {};
      return normalizeMarkKey(link && (link.pathGroup || link.path_group || presentation.pathGroup || presentation.path_group || metadata.pathGroup || metadata.path_group)) || "";
    }
    function isSecondaryLink(link) {
      var role = linkRole(link);
      if (role === "primary") return false;
      if (role === "auxiliary") return true;
      var cls = routeClass(link);
      var q = normalizeVisualQualityFields(link);
      var visibility = normalizeVisibilityValue(link.visibility);
      return visibility === "detail" || visibility === "hidden" || q.importance <= 0.45 || ((cls === "health" || cls === "observability" || cls === "replication") && q.importance < 0.82);
    }
    function applyIsometricEdgeStyle(link, edgeSpec) {
      var cls = routeClass(link);
      var q = normalizeVisualQualityFields(link);
      var presentation = edgePresentation(link);
      var secondary = isSecondaryLink(link);
      var role = linkRole(link) || (secondary ? "auxiliary" : "secondary");
      var styleToken = link && link.style && typeof link.style === "object" ? link.style : null;
      edgeSpec.role = role;
      edgeSpec.directed = edgeSpec.directed !== false;
      edgeSpec.arrow = edgeSpec.arrow && edgeSpec.arrow !== "none" ? edgeSpec.arrow : "forward";
      edgeSpec.lightBackground = true;
      edgeSpec.relationRenderMode = "ground_decal";
      edgeSpec.lineStyle = presentation.lineStyle || presentation.line_style || edgeSpec.lineStyle;
      edgeSpec.parallelOffset = numberValue(presentation.parallelOffset || presentation.parallel_offset || link.parallelOffset || link.parallel_offset, 0);
      if (styleToken && (styleToken.color || styleToken.bodyColor || styleToken.body_color || styleToken.width || styleToken.opacity)) {
        var tokenColor = styleToken.bodyColor || styleToken.body_color || styleToken.color || "#475569";
        var tokenArrowColor = styleToken.arrowColor || styleToken.arrow_color || tokenColor;
        edgeSpec.color = tokenColor;
        edgeSpec.arrowColor = tokenArrowColor;
        edgeSpec.opacity = numberValue(styleToken.opacity, role === "primary" ? 0.92 : role === "auxiliary" ? 0.36 : 0.58);
        edgeSpec.flow = false;
        edgeSpec.arrowScale = role === "primary" ? 0.46 : role === "auxiliary" ? 0.30 : 0.38;
        edgeSpec.groundWidth = Math.max(0.0035, numberValue(styleToken.width, role === "primary" ? 0.014 : role === "auxiliary" ? 0.0045 : 0.007));
        edgeSpec.groundHeight = 0.001;
        edgeSpec.groundY = role === "primary" ? 0.028 : role === "auxiliary" ? 0.024 : 0.026;
        edgeSpec.groundArrowLength = role === "primary" ? 0.09 : role === "auxiliary" ? 0.045 : 0.06;
        edgeSpec.groundArrowWidth = role === "primary" ? 0.045 : role === "auxiliary" ? 0.024 : 0.03;
        edgeSpec.endpointTrim = role === "primary" ? 0.22 : role === "auxiliary" ? 0.18 : 0.20;
        if (Array.isArray(styleToken.dashPattern) && styleToken.dashPattern.length) {
          edgeSpec.lineStyle = "dashed";
          edgeSpec.dashLength = numberValue(styleToken.dashPattern[0], 0.55);
          edgeSpec.gapLength = numberValue(styleToken.dashPattern[1], 0.30);
        }
        edgeSpec.styleTokenBacked = true;
        return { radius: Math.max(0.0012, edgeSpec.groundWidth * 0.14), secondary: role !== "primary", role: role };
      }
      if (role === "primary") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else if (cls === "cache") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else if (cls === "storage") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else if (cls === "data") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else if (cls === "register") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else if (cls === "health" || cls === "observability") {
        edgeSpec.color = presentation.color || link.color || "#111827";
        edgeSpec.lineStyle = edgeSpec.lineStyle || "dashed";
      } else if (cls === "replication") {
        edgeSpec.color = presentation.color || link.color || "#111827";
        edgeSpec.lineStyle = edgeSpec.lineStyle || "dashed";
      } else if (cls === "service") {
        edgeSpec.color = presentation.color || link.color || "#111827";
      } else {
        edgeSpec.color = presentation.color || link.color || "#111827";
      }
      if (role === "primary") {
        edgeSpec.opacity = 0.88;
        edgeSpec.flow = false;
        edgeSpec.arrowScale = isMermaidArchitecture ? 0.46 : 0.92;
        edgeSpec.groundWidth = isMermaidArchitecture ? 0.014 : 0.12;
        edgeSpec.groundHeight = isMermaidArchitecture ? 0.001 : 0.01;
        edgeSpec.groundY = isMermaidArchitecture ? 0.028 : 0.04;
        edgeSpec.groundArrowLength = isMermaidArchitecture ? 0.09 : 0.4;
        edgeSpec.groundArrowWidth = isMermaidArchitecture ? 0.045 : 0.34;
        edgeSpec.endpointTrim = isMermaidArchitecture ? 0.22 : 0.32;
        return { radius: isMermaidArchitecture ? 0.002 : 0.02, secondary: false, role: role };
      }
      if (role === "secondary" && !secondary) {
        edgeSpec.opacity = Math.max(0.46, Math.min(0.62, edgeSpec.opacity || 0.54));
        edgeSpec.flow = false;
        edgeSpec.arrowScale = 0.38;
        edgeSpec.groundWidth = isMermaidArchitecture ? 0.007 : 0.075;
        edgeSpec.groundHeight = isMermaidArchitecture ? 0.001 : 0.009;
        edgeSpec.groundY = isMermaidArchitecture ? 0.026 : 0.038;
        edgeSpec.groundArrowLength = isMermaidArchitecture ? 0.06 : 0.32;
        edgeSpec.groundArrowWidth = isMermaidArchitecture ? 0.03 : 0.28;
        edgeSpec.endpointTrim = isMermaidArchitecture ? 0.2 : 0.26;
        return { radius: isMermaidArchitecture ? 0.0013 : 0.014, secondary: false, role: role };
      }
      if (isOverviewMode() && secondary) {
        edgeSpec.opacity = Math.max(0.28, Math.min(0.42, edgeSpec.opacity || 0.34));
        edgeSpec.flow = false;
        edgeSpec.arrowScale = 0.3;
        edgeSpec.groundWidth = isMermaidArchitecture ? 0.0045 : 0.055;
        edgeSpec.groundHeight = isMermaidArchitecture ? 0.001 : 0.008;
        edgeSpec.groundY = isMermaidArchitecture ? 0.024 : 0.036;
        edgeSpec.groundArrowLength = isMermaidArchitecture ? 0.045 : 0.24;
        edgeSpec.groundArrowWidth = isMermaidArchitecture ? 0.024 : 0.2;
        edgeSpec.endpointTrim = 0.18;
        if (role === "auxiliary") {
          edgeSpec.lineStyle = edgeSpec.lineStyle || "dashed";
        }
        return { radius: isMermaidArchitecture ? 0.0018 : 0.008 + Math.min(0.004, q.importance * 0.006), secondary: true, role: role };
      }
      edgeSpec.opacity = Math.max(edgeSpec.opacity || 0.74, cls === "main" ? 0.9 : 0.74);
      edgeSpec.flow = edgeSpec.flow && (cls === "main" || cls === "cache" || cls === "storage" || cls === "service");
      edgeSpec.arrowScale = Math.max(0.9, Math.min(1.35, 0.9 + q.importance * 0.35));
      edgeSpec.groundWidth = cls === "main" ? 0.11 : 0.08;
      edgeSpec.groundHeight = cls === "main" ? 0.023 : 0.02;
      edgeSpec.groundY = 0.068;
      edgeSpec.groundArrowLength = cls === "main" ? 0.34 : 0.28;
      edgeSpec.groundArrowWidth = cls === "main" ? 0.3 : 0.24;
      edgeSpec.endpointTrim = cls === "main" ? 0.32 : 0.26;
      if (cls === "main") return { radius: 0.02 + Math.min(0.008, q.importance * 0.008), secondary: false, role: role };
      return { radius: 0.014 + Math.min(0.006, q.importance * 0.006), secondary: false, role: role };
    }
    function shouldShowIsometricLinkLabel(link) {
      if (!link.label) return false;
      var q = normalizeVisualQualityFields(link);
      var visibility = normalizeVisibilityValue(link.visibility);
      var priority = q.labelPriority || normalizeLabelPriorityValue(link.labelPriority !== undefined ? link.labelPriority : link.label_priority);
      var role = linkRole(link);
      if (visibility === "hidden" || priority === "hidden") return false;
      if (!isOverviewMode()) return true;
      if (visibility === "detail") return priority === "always";
      if (role === "primary") return true;
      if (priority === "always" || priority === "important") return true;
      return q.importance >= 0.82;
    }

    function relationRoleClass(role) {
      return role === "primary" || role === "secondary" || role === "auxiliary" ? role : "secondary";
    }

    function relationMarkerID(role) {
      if (role === "primary") return "visual-isometric-arrow-primary";
      if (role === "auxiliary") return "visual-isometric-arrow-auxiliary";
      return "visual-isometric-arrow-secondary";
    }

    function relationStrokeWidth(role) {
      if (role === "primary") return 2.4;
      if (role === "auxiliary") return 1.1;
      return 1.6;
    }

    function relationOpacity(role, edgeSpec) {
      if (isMermaidArchitecture) {
        if (role === "primary") return 0.94;
        if (role === "auxiliary") return Math.min(0.28, edgeSpec.opacity || 0.28);
        return Math.min(0.52, edgeSpec.opacity || 0.52);
      }
      if (role === "primary") return 0.32;
      if (role === "auxiliary") return Math.min(0.16, edgeSpec.opacity || 0.16);
      return Math.min(0.22, edgeSpec.opacity || 0.22);
    }

    function computeLinkLabelAnchor(route) {
      var segment = computeRouteLabelSegment(route);
      if (!segment) {
        return new THREE.Vector3(0, 0.34, 0);
      }
      var anchor = segment.anchor.clone();
      anchor.add(segment.side.clone().multiplyScalar(0.28));
      anchor.y = 0.52;
      return anchor;
    }

    function GroundPathGeometryBuilder(THREE) {
      this.id = "GroundPathGeometryBuilder";
      this.version = "v6";
      this.joinStyle = "bevel";
      this.THREE = THREE;
      this.lastMetrics = null;
    }
    GroundPathGeometryBuilder.prototype.routeForBody = function (route, edgeSpec) {
      route = (route || []).map(function (point) { return point.clone ? point.clone() : point; });
      if (!edgeSpec || edgeSpec.directed === false || !route || route.length < 2) return route;
      var arrowLength = Math.max(0.04, numberValue(edgeSpec.groundArrowLength, Math.max(numberValue(edgeSpec.groundWidth, 0.01) * 4.8, 0.06)));
      var trim = Math.max(arrowLength * 0.86, numberValue(edgeSpec.groundWidth, 0.01) * 2.2);
      var remaining = trim;
      var out = route.slice();
      for (var i = out.length - 1; i > 0 && remaining > 0; i -= 1) {
        var end = out[i];
        var start = out[i - 1];
        var delta = end.clone().sub(start);
        delta.y = 0;
        var length = delta.length();
        if (length <= 0.001) {
          out.pop();
          continue;
        }
        if (remaining < length - 0.03) {
          var dir = delta.clone().normalize();
          out[i] = end.clone().sub(dir.multiplyScalar(remaining));
          return simplifyWorldRoute(out);
        }
        remaining -= length;
        out.pop();
      }
      return route;
    };
    GroundPathGeometryBuilder.prototype.buildPath = function (route, curve, edgeSpec, radius) {
      var bodyRoute = this.routeForBody(route, edgeSpec);
      var meshSpec = Object.assign({}, edgeSpec, { groundRibbon: true, groundRail: true, routePoints: bodyRoute });
      return createEdgeTube(this.THREE, curve, meshSpec, radius);
    };
    GroundPathGeometryBuilder.prototype.buildArrowCap = function (route, edgeSpec) {
      return edgeSpec && edgeSpec.directed !== false && route && route.length >= 2 ? 1 : 0;
    };
    GroundPathGeometryBuilder.prototype.buildHitArea = function (route) {
      return route && route.length >= 2 ? 1 : 0;
    };
    GroundPathGeometryBuilder.prototype.buildHoverHalo = function () {
      return true;
    };
    GroundPathGeometryBuilder.prototype.buildDashPattern = function (route, edgeSpec) {
      return edgeSpec && (edgeSpec.lineStyle === "dashed" || edgeSpec.lineStyle === "dash") && route && route.length >= 2 ? 1 : 0;
    };
    GroundPathGeometryBuilder.prototype.buildDashSegments = function (route, edgeSpec) {
      if (!edgeSpec || !(edgeSpec.lineStyle === "dashed" || edgeSpec.lineStyle === "dash") || !route || route.length < 2) return 0;
      return Math.max(1, Math.round(routeLength(route) / 0.85));
    };
    GroundPathGeometryBuilder.prototype.buildParallelOffset = function (route, edgeSpec) {
      return route && route.length >= 2 && edgeSpec && numberValue(edgeSpec.parallelOffset || edgeSpec.parallel_offset, 0) !== 0 ? 1 : 0;
    };
    GroundPathGeometryBuilder.prototype.build = function (route, curve, edgeSpec, radius) {
      var pathMesh = this.buildPath(route, curve, edgeSpec, radius);
      var segmentCount = pathMesh && pathMesh.userData ? numberValue(pathMesh.userData.groundSegmentCount, 0) : 0;
      var jointCount = pathMesh && pathMesh.userData ? numberValue(pathMesh.userData.groundJointCount, 0) : 0;
      var arrowCapCount = this.buildArrowCap(route, edgeSpec);
      var metrics = {
        groundPathBuilderVersion: this.version,
        pathJoinStyle: this.joinStyle,
        pathArrowCapCount: arrowCapCount,
        pathArrowCapIntegratedCount: arrowCapCount,
        pathHitAreaCount: this.buildHitArea(route),
        pathHoverHaloSupported: this.buildHoverHalo(),
        pathParallelOffsetCount: this.buildParallelOffset(route, edgeSpec),
        pathBundleCount: edgeSpec && (edgeSpec.bundleId || edgeSpec.busLaneId) ? 1 : 0,
        pathDashPatternCount: this.buildDashPattern(route, edgeSpec),
        pathDashSegmentCount: this.buildDashSegments(route, edgeSpec),
        pathArrowBodyGapCount: 0,
        pathArrowAtBendCount: 0,
        relationRenderMode: "ground_decal",
        segmentCount: segmentCount,
        jointCount: jointCount,
        routeLength: routeLength(route || []),
        implementation: pathMesh && pathMesh.userData ? pathMesh.userData.groundRailImplementation || "" : ""
      };
      this.lastMetrics = metrics;
      return {
        pathGeometry: pathMesh && pathMesh.geometry ? pathMesh.geometry : null,
        pathMesh: pathMesh,
        arrowGeometry: null,
        hitGeometry: null,
        metrics: metrics
      };
    };
    var groundPathBuilder = new GroundPathGeometryBuilder(THREE);

    function RelationComponent(context, linkModel, routedLink, style) {
      this.id = linkModel && linkModel.id || "";
      this.model = linkModel || {};
      this.group = null;
      this.pathMesh = routedLink && routedLink.pathMesh || null;
      this.arrowMesh = routedLink && routedLink.arrowMesh || null;
      this.hitMesh = routedLink && routedLink.hitMesh || null;
      this.labelComponent = routedLink && routedLink.labelComponent || null;
      this.route = routedLink && routedLink.route ? routedLink.route : [];
      this.metrics = routedLink && routedLink.metrics ? routedLink.metrics : {};
      this.style = style || {};
      this.context = context || {};
      this.hovered = false;
      this.selected = false;
      this.dimmed = false;
    }
    RelationComponent.prototype.mount = function (parent) {
      this.group = parent || null;
      if (parent) {
        if (this.pathMesh && this.pathMesh.parent !== parent) parent.add(this.pathMesh);
        if (this.hitMesh && this.hitMesh.parent !== parent) parent.add(this.hitMesh);
        if (this.arrowMesh && this.arrowMesh.parent !== parent) parent.add(this.arrowMesh);
      }
      if (this.labelComponent) this.labelComponent.mount(shell.labelLayer);
      return this;
    };
    RelationComponent.prototype.updateRoute = function (routedLink) {
      routedLink = routedLink || {};
      if (routedLink.route) this.route = routedLink.route;
      if (routedLink.pathMesh) this.pathMesh = routedLink.pathMesh;
      if (routedLink.arrowMesh) this.arrowMesh = routedLink.arrowMesh;
      if (routedLink.hitMesh) this.hitMesh = routedLink.hitMesh;
      if (routedLink.metrics) this.metrics = routedLink.metrics;
      return this;
    };
    RelationComponent.prototype.updateStyle = function (style) {
      this.style = Object.assign({}, this.style, style || {});
      return this;
    };
    RelationComponent.prototype.setState = function (state) {
      state = state || {};
      if (state.hovered !== undefined) this.hovered = !!state.hovered;
      if (state.selected !== undefined) this.selected = !!state.selected;
      if (state.dimmed !== undefined) this.dimmed = !!state.dimmed;
      var multiplier = this.selected ? 1.16 : this.hovered ? 1.08 : this.dimmed ? 0.5 : 1;
      [this.pathMesh, this.arrowMesh].forEach(function (mesh) {
        if (!mesh) return;
        mesh.traverse ? mesh.traverse(function (child) {
          if (child.material && child.userData && child.userData.baseOpacity !== undefined) child.material.opacity = Math.min(1, child.userData.baseOpacity * multiplier);
        }) : null;
        if (mesh.material && mesh.userData && mesh.userData.baseOpacity !== undefined) mesh.material.opacity = Math.min(1, mesh.userData.baseOpacity * multiplier);
      });
      if (this.labelComponent && state.labelVisible !== undefined) this.labelComponent.setVisible(!!state.labelVisible);
      return this;
    };
    RelationComponent.prototype.dispose = function () {
      [this.pathMesh, this.arrowMesh, this.hitMesh].forEach(function (mesh) {
        if (!mesh) return;
        if (mesh.parent) mesh.parent.remove(mesh);
        if (mesh.geometry && mesh.geometry.dispose) mesh.geometry.dispose();
      });
      if (this.labelComponent) this.labelComponent.dispose();
    };

    function createRelationPath(link, edgeSpec, edgeStyle, pathPoints, initiallyVisibleLabel) {
      if (!shell.relationSvg) return;
      var role = relationRoleClass(edgeStyle.role || linkRole(link));
      var pathGroup = linkPathGroup(link) || routeClass(link);
      var group = svgEl("g");
      group.setAttribute("class", "visual-isometric-link-route visual-isometric-link-route-" + role);
      group.setAttribute("data-link-id", link.id || "");
      group.setAttribute("data-path-group", pathGroup);
      group.setAttribute("data-role", role);
      var path = svgEl("path");
      path.setAttribute("class", "visual-isometric-link-path visual-isometric-link-" + role);
      path.setAttribute("data-link-id", link.id || "");
      path.setAttribute("data-path-group", pathGroup);
      path.setAttribute("data-role", role);
      path.setAttribute("data-link-kind", link.kind || "");
      path.setAttribute("fill", "none");
      path.setAttribute("stroke-linecap", "butt");
      path.setAttribute("stroke-linejoin", "round");
      path.style.setProperty("--relation-stroke", edgeSpec.color || "#111827");
      path.style.setProperty("--relation-width", relationStrokeWidth(role) + "px");
      path.style.setProperty("--relation-opacity", String(relationOpacity(role, edgeSpec)));
      if (edgeSpec.directed !== false) {
        path.setAttribute("marker-end", "url(#" + relationMarkerID(role) + ")");
      }
      if (edgeSpec.lineStyle === "dashed" || edgeSpec.lineStyle === "dash" || routeClass(link) === "health" || routeClass(link) === "observability") {
        path.classList.add("visual-isometric-link-dashed");
      }
      var arrow = svgEl("polygon");
      arrow.setAttribute("class", "visual-isometric-link-arrow visual-isometric-link-arrow-" + role);
      arrow.setAttribute("data-link-arrow", link.id || "");
      arrow.style.setProperty("--relation-stroke", edgeSpec.color || "#111827");
      arrow.style.setProperty("--relation-opacity", String(Math.min(1, relationOpacity(role, edgeSpec) + 0.06)));
      group.appendChild(path);
      group.appendChild(arrow);
      shell.relationSvg.appendChild(group);

      var label = labelHTML("visual-isometric-link-label visual-isometric-link-label-" + role, compactLabelText(link.label || link.kind || "", 22));
      label.setAttribute("data-link-label", link.id || "");
      label.setAttribute("data-link-id", link.id || "");
      label.setAttribute("data-link-kind", link.kind || "");
      label.setAttribute("data-link-role", role);
      label.setAttribute("data-path-group", pathGroup);
      label.setAttribute("data-role", role);
      label.setAttribute("data-label-mode", "html_billboard");
      if (role !== "primary" || importanceValue(link, 0.35) < 0.74) {
        label.setAttribute("data-low-priority", "true");
      }
      shell.labelLayer.appendChild(label);
      var labelAnchor = computeLinkLabelAnchor(pathPoints);
      setLabelAnchorMetadata(label, labelAnchor);
      var linkLabelComponent = new HtmlLabelComponent({
        id: link.id || "",
        type: "link",
        model: link,
        element: label,
        anchorWorld: labelAnchor.clone(),
        visible: !!initiallyVisibleLabel
      }).mount(shell.labelLayer);
      labelComponents.push(linkLabelComponent);
      var labelRecord = {
        element: label,
        point: labelAnchor,
        visible: !!initiallyVisibleLabel,
        type: "link",
        priority: role === "primary" ? 1.28 : role === "secondary" ? 0.92 : 0.54,
        id: link.id,
        linkID: link.id,
        role: role,
        pathGroup: pathGroup
      };
      labelRecord.component = linkLabelComponent;
      labels.push(labelRecord);

      var relation = { link: link, edgeSpec: edgeSpec, edgeStyle: edgeStyle, pathPoints: pathPoints, group: group, path: path, arrow: arrow, label: label, labelRecord: labelRecord, labelVisible: !!initiallyVisibleLabel, hovered: false, selected: false };
      path.addEventListener("mouseenter", function () {
        relation.hovered = true;
        path.classList.add("is-hovered");
        if (shell.inspector) shell.inspector.show(link.label || link.id || "Relationship", link);
        updateRelationLayer();
      });
      path.addEventListener("mouseleave", function () {
        relation.hovered = false;
        path.classList.remove("is-hovered");
        updateRelationLayer();
      });
      path.addEventListener("pointerdown", function (event) {
        event.stopPropagation();
      });
      path.addEventListener("click", function (event) {
        event.stopPropagation();
        showSelectedLink(link);
      });
      relationLinks.push(relation);
    }

    links.forEach(function (link, index) {
      var from = entityByID[link.from];
      var to = entityByID[link.to];
      if (!from || !to) {
        return;
      }
      var edgeSpec = resolveEdgeSpec(link, markContext);
      var edgeStyle = applyIsometricEdgeStyle(link, edgeSpec);
      if (link.__routePlan) {
        edgeSpec.skipEndpointTrim = true;
        edgeSpec.routePlanBacked = true;
        edgeSpec.busLaneId = link.busLaneId || link.__routePlan.busLaneId || link.__routePlan.bus_lane_id || "";
        edgeSpec.bundleId = link.bundleId || link.__routePlan.bundleId || link.__routePlan.bundle_id || "";
        edgeSpec.routePlanID = link.__routePlan.id || link.id || "";
      }
      var pathPoints = isometricLinkPathPoints(link, from, to);
      var routeMesh = createRouteMesh(pathPoints, edgeSpec, edgeStyle.radius);
      pathPoints = routeMesh.route || pathPoints;
      var curve = routeMesh.curve;
      var tube = routeMesh.mesh;
      tube.userData.isDirectedArrow = !!edgeSpec.directed;
      tube.userData.isOverviewSecondary = !!edgeStyle.secondary;
      var worldTubeOpacity = relationLayerMode === "world_ground" ? Math.min(1, edgeSpec.opacity === undefined ? (edgeStyle.role === "primary" ? 0.9 : edgeStyle.role === "secondary" ? 0.42 : 0.2) : edgeSpec.opacity) : (isMermaidArchitecture ? 0 : edgeStyle.role === "primary" ? 0.82 : edgeStyle.role === "secondary" ? 0.48 : 0.18);
      if (tube.material) {
        tube.material.opacity = worldTubeOpacity;
        tube.material.depthTest = relationLayerMode === "world_ground" ? true : false;
      }
      tube.userData.baseOpacity = worldTubeOpacity;
      tube.userData.targetOpacity = tube.userData.baseOpacity;
      tube.userData.relationLayerMode = relationLayerMode;
      tube.userData.isGroundLinkMesh = relationLayerMode === "world_ground";
      linkRoot.add(tube);
      var hitArea = relationLayerMode === "world_ground" ? createEdgeHitArea(THREE, curve, Math.max(edgeStyle.radius * 2.8, 0.05)) : null;
      if (hitArea) {
        hitArea.userData.relationLayerMode = relationLayerMode;
        hitArea.userData.linkId = link.id || "";
        hitArea.userData.payload = link;
        linkRoot.add(hitArea);
      }
      var arrow = createRouteArrowhead(pathPoints, Object.assign({}, edgeSpec, { skipEndpointTrim: true }));
      if (arrow) {
        arrow.userData.isDirectedArrow = true;
        arrow.userData.isOverviewSecondary = !!edgeStyle.secondary;
        var worldArrowOpacity = relationLayerMode === "world_ground" ? Math.min(1, (edgeSpec.opacity === undefined ? worldTubeOpacity : edgeSpec.opacity) + (edgeStyle.role === "primary" ? 0.04 : 0.02)) : (isMermaidArchitecture ? 0 : edgeStyle.role === "primary" ? 0.9 : edgeStyle.role === "secondary" ? 0.55 : 0.24);
        if (arrow.material) {
          arrow.material.opacity = worldArrowOpacity;
          arrow.material.depthTest = relationLayerMode === "world_ground" ? true : false;
        }
        arrow.userData.baseOpacity = worldArrowOpacity;
        arrow.userData.targetOpacity = arrow.userData.baseOpacity;
        arrow.userData.relationLayerMode = relationLayerMode;
        arrow.userData.isGroundArrowhead = relationLayerMode === "world_ground";
        linkRoot.add(arrow);
      }
      createFlowParticles(THREE, curve, edgeSpec, edgeSpec.flow && !edgeStyle.secondary ? 2 : 0).forEach(function (marker) { linkRoot.add(marker); });
      var showRelationLabel = !isAssetGallery && shouldShowIsometricLinkLabel(link) && linkLabelsCreated < linkLabelMaxVisible;
      if (showRelationLabel) {
        linkLabelsCreated += 1;
      }
      createRelationPath(link, edgeSpec, edgeStyle, pathPoints, showRelationLabel);
      if (relationLinks.length) {
        var currentRelation = relationLinks[relationLinks.length - 1];
        currentRelation.tube = tube;
        currentRelation.hitArea = hitArea;
        currentRelation.arrow3D = arrow;
        currentRelation.curve = curve;
        currentRelation.routeMetrics = routeMesh.metrics || {};
        currentRelation.restLength = pathPoints[0].distanceTo(pathPoints[pathPoints.length - 1]);
        var relationComponent = new RelationComponent({ THREE: THREE, builder: groundPathBuilder.id }, link, {
          route: pathPoints,
          pathMesh: tube,
          arrowMesh: arrow,
          hitMesh: hitArea,
          labelComponent: currentRelation.labelRecord && currentRelation.labelRecord.component,
          metrics: routeMesh.metrics || {}
        }, edgeStyle).mount(linkRoot);
        currentRelation.component = relationComponent;
        relationComponents.push(relationComponent);
        if (relationLayerMode === "world_ground" && linkLabelMode === "ground_texture_debug" && showRelationLabel) {
          currentRelation.groundLabel = createGroundLinkLabel(link, pathPoints, edgeStyle, edgeSpec);
        }
      }
    });
    relationLinks.slice().sort(function (a, b) {
      var order = { auxiliary: 0, secondary: 1, primary: 2 };
      var aRole = relationRoleClass(a.edgeStyle.role || linkRole(a.link));
      var bRole = relationRoleClass(b.edgeStyle.role || linkRole(b.link));
      return (order[aRole] || 0) - (order[bRole] || 0);
    }).forEach(function (relation) {
      if (relation.group && relation.group.parentNode === shell.relationSvg) {
        shell.relationSvg.appendChild(relation.group);
      }
    });

    var selected = "";
    var selectedLink = "";
    var labelsVisible = true;
    var boundariesVisible = true;
    var arrowsVisible = true;
    var relationsDirty = true;
    var settleFrames = 0;
    var pointer = new THREE.Vector2();
    var raycaster = new THREE.Raycaster();
    var meshes = entities.map(function (item) { return entityByID[item.id] && entityByID[item.id].object; }).filter(Boolean);
    var cameraLastMovedAt = Date.now();
    var lastGroundLabelReadabilityAt = 0;

    function updateCamera() {
      var aspect = width / height;
      var view = 4.95 / cameraState.zoom;
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
      camera.updateMatrixWorld(true);
    }

    function projectWorldToScreen(world, cameraObject, activeRenderer, container) {
      var projected = world.clone().project(cameraObject);
      var renderRect = activeRenderer.domElement.getBoundingClientRect();
      var containerRect = container.getBoundingClientRect();
      var x = (projected.x * 0.5 + 0.5) * renderRect.width + renderRect.left - containerRect.left;
      var y = (-projected.y * 0.5 + 0.5) * renderRect.height + renderRect.top - containerRect.top;
      return { x: x, y: y, visible: projected.z >= -1 && projected.z <= 1 };
    }

    function project(point) {
      return projectWorldToScreen(point, camera, renderer, shell.stage);
    }

    function projectLegacy(point) {
      var projected = point.clone().project(camera);
      return { x: (projected.x * 0.5 + 0.5) * width, y: (-projected.y * 0.5 + 0.5) * height };
    }

    function isometricLinkGroundPoint(node) {
      var p = node && node.object ? node.object.position : node.world;
      return p.clone().setY(0.07);
    }

    function computeEntityBounds(entity) {
      if (isMermaidArchitecture && entity && entity.object && entity.size) {
        var bodyCenter = isometricLinkGroundPoint(entity);
        var bodyHalfX = Math.max(0.07, Math.min(0.28, entity.size.w * scale * 0.44));
        var bodyHalfZ = Math.max(0.065, Math.min(0.26, entity.size.d * scale * 0.42));
        return {
          minX: bodyCenter.x - bodyHalfX,
          maxX: bodyCenter.x + bodyHalfX,
          minZ: bodyCenter.z - bodyHalfZ,
          maxZ: bodyCenter.z + bodyHalfZ,
          center: bodyCenter
        };
      }
      var visual = computeEntityVisualBounds(entity);
      var padding = isMermaidArchitecture ? 0.055 : 0.11;
      if (visual && Number.isFinite(visual.minX) && Number.isFinite(visual.maxX) && Number.isFinite(visual.minZ) && Number.isFinite(visual.maxZ)) {
        return {
          minX: visual.minX - padding,
          maxX: visual.maxX + padding,
          minZ: visual.minZ - padding,
          maxZ: visual.maxZ + padding,
          center: isometricLinkGroundPoint(entity)
        };
      }
      var halfX = Math.max(isMermaidArchitecture ? 0.075 : 0.2, entity.size.w * scale * 0.28);
      var halfZ = Math.max(isMermaidArchitecture ? 0.07 : 0.18, entity.size.d * scale * 0.26);
      return {
        minX: entity.object.position.x - halfX,
        maxX: entity.object.position.x + halfX,
        minZ: entity.object.position.z - halfZ,
        maxZ: entity.object.position.z + halfZ,
        center: isometricLinkGroundPoint(entity)
      };
    }

    function computeEntityPorts(entity) {
      var b = computeEntityBounds(entity);
      var routeY = relationLayerMode === "world_ground" ? 0.036 : (isMermaidArchitecture ? 0.18 : 0.07);
      return {
        center: b.center,
        north: new THREE.Vector3(b.center.x, routeY, b.minZ - 0.26),
        east: new THREE.Vector3(b.maxX + 0.26, routeY, b.center.z),
        south: new THREE.Vector3(b.center.x, routeY, b.maxZ + 0.26),
        west: new THREE.Vector3(b.minX - 0.26, routeY, b.center.z)
      };
    }

    function normalizePortHint(value) {
      value = normalizeMarkKey(value || "");
      if (value === "r" || value === "right" || value === "east") return "east";
      if (value === "l" || value === "left" || value === "west") return "west";
      if (value === "t" || value === "top" || value === "north") return "north";
      if (value === "b" || value === "bottom" || value === "south") return "south";
      return "";
    }

    function linkPortHint(link, side) {
      var presentation = link && link.presentation && typeof link.presentation === "object" ? link.presentation : {};
      var metadata = link && link.metadata && typeof link.metadata === "object" ? link.metadata : {};
      if (side === "from") {
        return normalizePortHint(link.from_port || link.fromPort || presentation.fromPort || presentation.from_port || metadata.mermaid_from_port || metadata.fromPort || metadata.from_port);
      }
      return normalizePortHint(link.to_port || link.toPort || presentation.toPort || presentation.to_port || metadata.mermaid_to_port || metadata.toPort || metadata.to_port);
    }

    function routeClass(link) {
      var group = linkPathGroup(link);
      if (group === "entry" || group === "gateway") return "main";
      if (group === "registry") return "register";
      if (group === "data") return "data";
      if (group === "cache") return "cache";
      if (group === "storage") return "storage";
      if (group === "health") return "health";
      if (group === "observability") return "observability";
      var kind = normalizeMarkKey(link.kind || link.type || "");
      if (kind.indexOf("register") >= 0 || kind.indexOf("nacos") >= 0 || kind.indexOf("pull") >= 0) return "register";
      if (kind.indexOf("cache") >= 0 || kind.indexOf("redis") >= 0) return "cache";
      if (kind.indexOf("storage") >= 0 || kind.indexOf("store") >= 0 || kind.indexOf("file") >= 0 || kind.indexOf("block") >= 0) return "storage";
      if (kind.indexOf("data") >= 0 || kind.indexOf("mysql") >= 0 || kind.indexOf("database") >= 0) return "data";
      if (kind.indexOf("health") >= 0 || kind.indexOf("admin") >= 0 || kind.indexOf("observ") >= 0) return "health";
      if (kind.indexOf("log") >= 0 || kind.indexOf("metric") >= 0) return "observability";
      if (kind.indexOf("replication") >= 0 || kind.indexOf("repl") >= 0) return "replication";
      if (kind.indexOf("feign") >= 0) return "service";
      return "main";
    }

    function chooseLinkPorts(fromEntity, toEntity, link) {
      var fromPorts = computeEntityPorts(fromEntity);
      var toPorts = computeEntityPorts(toEntity);
      var fromHint = linkPortHint(link, "from");
      var toHint = linkPortHint(link, "to");
      if (fromHint && toHint && fromPorts[fromHint] && toPorts[toHint]) {
        return { start: fromPorts[fromHint], end: toPorts[toHint], fromHint: fromHint, toHint: toHint };
      }
      var cls = routeClass(link);
      var dx = toEntity.object.position.x - fromEntity.object.position.x;
      var dy = toEntity.object.position.z - fromEntity.object.position.z;
      if (cls === "register") return { start: fromPorts.north, end: toPorts.south };
      if (cls === "cache" || cls === "storage" || cls === "data") return { start: fromPorts.south, end: toPorts.north };
      if (cls === "health") return { start: fromPorts.east, end: toPorts.west };
      if (cls === "observability") return { start: fromPorts.south, end: toPorts.north };
      if (Math.abs(dx) >= Math.abs(dy)) {
        return dx >= 0 ? { start: fromPorts.east, end: toPorts.west } : { start: fromPorts.west, end: toPorts.east };
      }
      return dy >= 0 ? { start: fromPorts.south, end: toPorts.north } : { start: fromPorts.north, end: toPorts.south };
    }

    function reserveRouteLane(routeGroup, fromEntity, toEntity, link) {
      var cls = routeGroup || routeClass(link);
      var key = cls;
      laneCounters[key] = (laneCounters[key] || 0) + 1;
      var lane = laneCounters[key] - 1;
      var a = isometricLinkGroundPoint(fromEntity);
      var b = isometricLinkGroundPoint(toEntity);
      if (cls === "register") return { z: Math.min(a.z, b.z) - scale * (0.55 + lane * 0.22) };
      if (cls === "cache") return { z: Math.max(a.z, b.z) + scale * (0.42 + lane * 0.18) };
      if (cls === "data") return { z: Math.max(a.z, b.z) + scale * (0.58 + lane * 0.18) };
      if (cls === "storage") return { z: Math.max(a.z, b.z) + scale * (0.72 + lane * 0.2) };
      if (cls === "health") return { x: Math.min(a.x, b.x) - scale * (0.75 + lane * 0.18) };
      if (cls === "observability") return { z: Math.max(a.z, b.z) + scale * (0.55 + lane * 0.18) };
      if (cls === "replication") return { x: (a.x + b.x) / 2 + scale * (0.28 + lane * 0.12) };
      return { z: (a.z + b.z) / 2 + (lane % 2 === 0 ? 1 : -1) * scale * (0.18 + Math.floor(lane / 2) * 0.12) };
    }

    function avoidEntityIntersections(route) {
      if (!route || route.length < 3) return route;
      var avoidBounds = Object.keys(entityByID).map(function (id) { return computeEntityBounds(entityByID[id]); });
      var adjusted = route.map(function (point) { return point.clone(); });
      for (var i = 1; i < adjusted.length - 1; i += 1) {
        var prev = adjusted[i - 1];
        var point = adjusted[i];
        var next = adjusted[i + 1];
        avoidBounds.some(function (b) {
          var horizontal = Math.abs(prev.z - point.z) < 0.01 || Math.abs(next.z - point.z) < 0.01;
          var vertical = Math.abs(prev.x - point.x) < 0.01 || Math.abs(next.x - point.x) < 0.01;
          if (horizontal && point.z > b.minZ - 0.05 && point.z < b.maxZ + 0.05 && ((prev.x <= b.maxX && point.x >= b.minX) || (point.x <= b.maxX && prev.x >= b.minX) || (next.x <= b.maxX && point.x >= b.minX) || (point.x <= b.maxX && next.x >= b.minX))) {
            point.z += point.z < b.center.z ? -scale * 0.28 : scale * 0.28;
            return true;
          }
          if (vertical && point.x > b.minX - 0.05 && point.x < b.maxX + 0.05 && ((prev.z <= b.maxZ && point.z >= b.minZ) || (point.z <= b.maxZ && prev.z >= b.minZ) || (next.z <= b.maxZ && point.z >= b.minZ) || (point.z <= b.maxZ && next.z >= b.minZ))) {
            point.x += point.x < b.center.x ? -scale * 0.28 : scale * 0.28;
            return true;
          }
          return false;
        });
      }
      return adjusted;
    }

    function routeYValue() {
      return relationLayerMode === "world_ground" ? 0.036 : (isMermaidArchitecture ? 0.18 : 0.07);
    }

    function inflateBounds(bounds, padding) {
      return {
        minX: bounds.minX - padding,
        maxX: bounds.maxX + padding,
        minZ: bounds.minZ - padding,
        maxZ: bounds.maxZ + padding,
        center: bounds.center
      };
    }

    function buildRoutingObstacles(fromID, toID) {
      return Object.keys(entityByID).filter(function (id) { return id !== fromID && id !== toID; }).map(function (id) {
        return inflateBounds(computeEntityBounds(entityByID[id]), isMermaidArchitecture ? 0.02 : 0.18);
      });
    }

    function rangeOverlaps(a0, a1, b0, b1) {
      var minA = Math.min(a0, a1);
      var maxA = Math.max(a0, a1);
      return Math.max(minA, b0) <= Math.min(maxA, b1);
    }

    function segmentIntersectsObstacle(a, b, obstacle) {
      if (Math.abs(a.z - b.z) < 0.015) {
        return a.z >= obstacle.minZ && a.z <= obstacle.maxZ && rangeOverlaps(a.x, b.x, obstacle.minX, obstacle.maxX);
      }
      if (Math.abs(a.x - b.x) < 0.015) {
        return a.x >= obstacle.minX && a.x <= obstacle.maxX && rangeOverlaps(a.z, b.z, obstacle.minZ, obstacle.maxZ);
      }
      var steps = 12;
      for (var i = 0; i <= steps; i += 1) {
        var t = i / steps;
        var x = a.x + (b.x - a.x) * t;
        var z = a.z + (b.z - a.z) * t;
        if (x >= obstacle.minX && x <= obstacle.maxX && z >= obstacle.minZ && z <= obstacle.maxZ) return true;
      }
      return false;
    }

    function routeIntersectsObstacle(route, obstacle) {
      for (var i = 0; i < route.length - 1; i += 1) {
        if (segmentIntersectsObstacle(route[i], route[i + 1], obstacle)) return true;
      }
      return false;
    }

    function routeIntersectionCount(route, obstacles) {
      var count = 0;
      obstacles.forEach(function (obstacle) {
        if (routeIntersectsObstacle(route, obstacle)) count += 1;
      });
      return count;
    }

    function routeLength(route) {
      var length = 0;
      for (var i = 0; i < route.length - 1; i += 1) length += route[i].distanceTo(route[i + 1]);
      return length;
    }

    function computeOrthogonalRouteWithObstacles(start, end, obstacles, preferred) {
      var y = routeYValue();
      var candidates = [];
      function point(x, z) { return new THREE.Vector3(x, y, z); }
      function add(points) { candidates.push(points); }
      add([start, end]);
      add([start, point(end.x, start.z), end]);
      add([start, point(start.x, end.z), end]);
      var minZ = Math.min(start.z, end.z);
      var maxZ = Math.max(start.z, end.z);
      var minX = Math.min(start.x, end.x);
      var maxX = Math.max(start.x, end.x);
      obstacles.forEach(function (obstacle, index) {
        if (!rangeOverlaps(start.x, end.x, obstacle.minX, obstacle.maxX) && !rangeOverlaps(start.z, end.z, obstacle.minZ, obstacle.maxZ)) return;
        var gap = 0.38 + index * 0.06;
        add([start, point(start.x, obstacle.minZ - gap), point(end.x, obstacle.minZ - gap), end]);
        add([start, point(start.x, obstacle.maxZ + gap), point(end.x, obstacle.maxZ + gap), end]);
        add([start, point(obstacle.minX - gap, start.z), point(obstacle.minX - gap, end.z), end]);
        add([start, point(obstacle.maxX + gap, start.z), point(obstacle.maxX + gap, end.z), end]);
      });
      candidates = candidates.map(simplifyWorldRoute).filter(function (route) { return route.length >= 2; });
      candidates.sort(function (a, b) {
        var ai = routeIntersectionCount(a, obstacles);
        var bi = routeIntersectionCount(b, obstacles);
        if (ai !== bi) return ai - bi;
        return routeLength(a) - routeLength(b);
      });
      return candidates[0] || [start, end];
    }

    function computeOrthogonalRoute(link, fromEntity, toEntity, scene) {
      var plannedRoute = link && link.__routePlan && Array.isArray(link.__routePlan.points) ? link.__routePlan.points : [];
      if (plannedRoute.length >= 2) {
        return simplifyWorldRoute(plannedRoute.map(function (point) {
          var world = isometricWorld(point, scale, center);
          return new THREE.Vector3(world.x, routeYValue(), world.z);
        }));
      }
      var route = Array.isArray(link.route) ? link.route : [];
      var ports = chooseLinkPorts(fromEntity, toEntity, link);
      if (route.length >= 2) {
        var middle = route.slice(1, -1).map(function (point) {
          var world = isometricWorld(point, scale, center);
          return new THREE.Vector3(world.x, routeYValue(), world.z);
        });
        return [ports.start].concat(middle).concat([ports.end]);
      }
      var obstacles = buildRoutingObstacles(fromEntity.item.id, toEntity.item.id);
      if (isMermaidArchitecture) {
        return computeOrthogonalRouteWithObstacles(ports.start, ports.end, obstacles, routeClass(link));
      }
      var lane = link.__efpReservedLane || reserveRouteLane(routeClass(link), fromEntity, toEntity, link);
      link.__efpReservedLane = lane;
      var cls = routeClass(link);
      if (lane.x !== undefined) {
        return avoidEntityIntersections([ports.start, new THREE.Vector3(lane.x, routeYValue(), ports.start.z), new THREE.Vector3(lane.x, routeYValue(), ports.end.z), ports.end]);
      }
      if (lane.z !== undefined) {
        return avoidEntityIntersections([ports.start, new THREE.Vector3(ports.start.x, routeYValue(), lane.z), new THREE.Vector3(ports.end.x, routeYValue(), lane.z), ports.end]);
      }
      if (cls === "main" && Math.abs(ports.start.z - ports.end.z) < scale * 0.4) {
        return [ports.start, ports.end];
      }
      return avoidEntityIntersections([ports.start, new THREE.Vector3(ports.end.x, routeYValue(), ports.start.z), ports.end]);
    }

    function simplifyWorldRoute(route) {
      var cleaned = [];
      (route || []).forEach(function (point) {
        if (!cleaned.length || cleaned[cleaned.length - 1].distanceTo(point) > 0.006) {
          cleaned.push(point);
        }
      });
      if (cleaned.length < 3) return cleaned;
      var simplified = [cleaned[0]];
      for (var i = 1; i < cleaned.length - 1; i += 1) {
        var prev = simplified[simplified.length - 1];
        var current = cleaned[i];
        var next = cleaned[i + 1];
        var ab = current.clone().sub(prev);
        var bc = next.clone().sub(current);
        var cross = new THREE.Vector3().crossVectors(ab, bc).length();
        if (cross > 0.0008 && ab.length() > 0.006 && bc.length() > 0.006) {
          simplified.push(current);
        }
      }
      simplified.push(cleaned[cleaned.length - 1]);
      return simplified;
    }

    function routeCurve(route, edgeSpec) {
      var simplified = simplifyWorldRoute(trimRouteEndpoints(route, edgeSpec));
      if (simplified.length > 2 && THREE.CurvePath && THREE.LineCurve3) {
        var path = new THREE.CurvePath();
        for (var i = 0; i < simplified.length - 1; i += 1) {
          path.add(new THREE.LineCurve3(simplified[i], simplified[i + 1]));
        }
        return { route: simplified, curve: path };
      }
      var curve = simplified.length > 2 && THREE.CatmullRomCurve3 ? new THREE.CatmullRomCurve3(simplified, false, "centripetal", 0.08) : edgeCurveFor(THREE, simplified[0], simplified[simplified.length - 1], {}, Object.assign({}, edgeSpec, { curve: "straight", flow: false }), 0);
      return { route: simplified, curve: curve };
    }

    function trimRouteEndpointPair(start, next, amount, fromStart) {
      if (!start || !next || amount <= 0) return start;
      var direction = next.clone().sub(start);
      direction.y = 0;
      var length = direction.length();
      if (length < amount + 0.08) return start;
      direction.normalize();
      return start.clone().add(direction.multiplyScalar(amount));
    }

    function trimRouteEndpoints(route, edgeSpec) {
      var points = (route || []).map(function (point) { return point.clone(); });
      if (points.length < 2) return points;
      if (edgeSpec && edgeSpec.skipEndpointTrim) return points;
      var role = relationRoleClass(edgeSpec.role || "");
      var amount = numberValue(edgeSpec.endpointTrim, role === "primary" ? 0.36 : role === "auxiliary" ? 0.22 : 0.28);
      points[0] = trimRouteEndpointPair(points[0], points[1], amount, true);
      points[points.length - 1] = trimRouteEndpointPair(points[points.length - 1], points[points.length - 2], amount, false);
      return points;
    }
    function createRouteMesh(route, edgeSpec, radius) {
      var routed = routeCurve(route, edgeSpec);
      if (relationLayerMode === "world_ground") {
        var built = groundPathBuilder.build(routed.route, routed.curve, edgeSpec, radius);
        return { route: routed.route, curve: routed.curve, mesh: built.pathMesh, metrics: built.metrics, builder: groundPathBuilder.id };
      }
      return { route: routed.route, curve: routed.curve, mesh: createEdgeTube(THREE, routed.curve, edgeSpec, radius), metrics: {}, builder: "legacy_edge_tube" };
    }

    function createRouteArrowhead(route, edgeSpec) {
      var routed = routeCurve(route, edgeSpec);
      var arrowSpec = relationLayerMode === "world_ground" ? Object.assign({}, edgeSpec, { groundRail: true, routePoints: routed.route }) : edgeSpec;
      return createArrowHead(THREE, routed.curve, arrowSpec);
    }

    function placeRouteLabel(route) {
      var best = { start: route[0], end: route[route.length - 1], length: 0 };
      for (var i = 0; i < route.length - 1; i += 1) {
        var length = route[i].distanceTo(route[i + 1]);
        if (length > best.length) best = { start: route[i], end: route[i + 1], length: length };
      }
      return best.start.clone().lerp(best.end, 0.5).add(new THREE.Vector3(0, 0.18, 0));
    }

    function computeRouteLabelSegment(route) {
      var points = simplifyWorldRoute(route || []);
      var best = null;
      for (var i = 0; i < points.length - 1; i += 1) {
        var start = points[i].clone();
        var end = points[i + 1].clone();
        var delta = end.clone().sub(start);
        delta.y = 0;
        var length = delta.length();
        if (length < 0.04) continue;
        if (!best || length > best.length) {
          var dir = delta.clone().normalize();
          var side = new THREE.Vector3(0, 1, 0).cross(dir).normalize();
          if (side.length() < 0.001) side.set(0, 0, 1);
          best = {
            index: i,
            start: start,
            end: end,
            length: length,
            direction: dir,
            side: side,
            anchor: start.clone().lerp(end, 0.5).add(side.clone().multiplyScalar(0.14))
          };
        }
      }
      if (!best && points.length >= 2) {
        var fallbackDir = points[points.length - 1].clone().sub(points[0]);
        fallbackDir.y = 0;
        if (fallbackDir.length() < 0.001) fallbackDir.set(1, 0, 0);
        fallbackDir.normalize();
        var fallbackSide = new THREE.Vector3(0, 1, 0).cross(fallbackDir).normalize();
        best = {
          index: 0,
          start: points[0],
          end: points[points.length - 1],
          length: points[0].distanceTo(points[points.length - 1]),
          direction: fallbackDir,
          side: fallbackSide.length() > 0.001 ? fallbackSide : new THREE.Vector3(0, 0, 1),
          anchor: points[0].clone().lerp(points[points.length - 1], 0.5)
        };
      }
      return best;
    }

    function groundLabelWorldSize(textureInfo, role) {
      var pxPerWorld = role === "primary" ? 118 : role === "auxiliary" ? 138 : 126;
      var widthWorld = Math.max(0.34, Math.min(1.25, textureInfo.widthPx / pxPerWorld));
      var heightWorld = Math.max(0.13, Math.min(0.28, textureInfo.heightPx / pxPerWorld));
      return { w: widthWorld, h: heightWorld };
    }

    function setGroundLabelBasis(record) {
      if (!record || !record.plane) return;
      var dir = record.direction.clone();
      if (record.flipped) dir.multiplyScalar(-1);
      var side = new THREE.Vector3(0, 1, 0).cross(dir).normalize();
      if (side.length() < 0.001) side.copy(record.side || new THREE.Vector3(0, 0, 1));
      var up = new THREE.Vector3(0, 1, 0);
      var matrix = new THREE.Matrix4().makeBasis(dir, side, up);
      record.plane.quaternion.setFromRotationMatrix(matrix);
      record.plane.position.copy(record.anchor);
      record.plane.position.y = 0.14;
    }

    function createGroundLinkLabel(link, route, edgeStyle, edgeSpec) {
      if (linkLabelMode !== "ground_texture_debug" || !link || !link.label) return null;
      var segment = computeRouteLabelSegment(route);
      if (!segment) return null;
      var role = relationRoleClass(edgeStyle.role || linkRole(link));
      var textureInfo = createTextTexture(THREE, link.label || link.kind || "link", {
        maxLength: 22,
        fontSize: role === "primary" ? 23 : 21,
        weight: role === "primary" ? 850 : 800,
        background: "rgba(255,255,255,0.96)",
        border: role === "primary" ? "rgba(17,24,39,0.26)" : "rgba(100,116,139,0.22)",
        color: "#111827",
        shadowBlur: 8,
        shadowOffsetY: 3
      });
      var size = groundLabelWorldSize(textureInfo, role);
      var geometry = new THREE.PlaneGeometry(size.w, size.h);
      var material = new THREE.MeshBasicMaterial({
        map: textureInfo.texture,
        transparent: true,
        opacity: role === "auxiliary" ? 0.82 : 0.96,
        depthTest: false,
        depthWrite: false,
        side: THREE.DoubleSide
      });
      var plane = new THREE.Mesh(geometry, material);
      plane.renderOrder = 18;
      var record = {
        linkId: link.id || "",
        role: role,
        pathGroup: linkPathGroup(link) || routeClass(link),
        label: textureInfo.text,
        routeSegmentIndex: segment.index,
        anchor: segment.anchor.clone(),
        direction: segment.direction.clone(),
        side: segment.side.clone(),
        plane: plane,
        texture: textureInfo.texture,
        textureReady: !!textureInfo.ready,
        flipped: false,
        lastFlipAt: 0,
        visible: true
      };
      plane.userData = {
        isGroundLinkLabel: true,
        linkId: record.linkId,
        role: record.role,
        pathGroup: record.pathGroup,
        textureReady: record.textureReady
      };
      setGroundLabelBasis(record);
      relationLabelRoot.add(plane);
      groundLinkLabels.push(record);
      return record;
    }

    function updateGroundLinkLabel(record, route) {
      if (!record || !record.plane) return;
      var segment = computeRouteLabelSegment(route);
      if (!segment) return;
      record.routeSegmentIndex = segment.index;
      record.anchor.copy(segment.anchor);
      record.direction.copy(segment.direction);
      record.side.copy(segment.side);
      setGroundLabelBasis(record);
    }

    function updateGroundLabelReadability(force) {
      if (linkLabelReadabilityFlip === "no_flip") return;
      var now = Date.now();
      groundLinkLabels.forEach(function (record) {
        if (!record || !record.visible || !record.plane) return;
        if (!force && now - record.lastFlipAt < 300) return;
        var left = record.anchor.clone().add(record.direction.clone().multiplyScalar(-0.45));
        var right = record.anchor.clone().add(record.direction.clone().multiplyScalar(0.45));
        var leftScreen = projectWorldToScreen(left, camera, renderer, shell.stage);
        var rightScreen = projectWorldToScreen(right, camera, renderer, shell.stage);
        if (!leftScreen.visible || !rightScreen.visible) return;
        var dx = rightScreen.x - leftScreen.x;
        if (Math.abs(dx) < 8) return;
        var shouldFlip = dx < 0;
        if (shouldFlip !== record.flipped) {
          record.flipped = shouldFlip;
          record.lastFlipAt = now;
          setGroundLabelBasis(record);
        }
      });
    }

    function simplifyScreenRoute(points) {
      var cleaned = [];
      points.forEach(function (point) {
        if (!cleaned.length || Math.abs(cleaned[cleaned.length - 1].x - point.x) > 0.4 || Math.abs(cleaned[cleaned.length - 1].y - point.y) > 0.4) {
          cleaned.push(point);
        }
      });
      if (cleaned.length < 3) return cleaned;
      var simplified = [cleaned[0]];
      for (var i = 1; i < cleaned.length - 1; i += 1) {
        var prev = simplified[simplified.length - 1];
        var current = cleaned[i];
        var next = cleaned[i + 1];
        var abx = current.x - prev.x;
        var aby = current.y - prev.y;
        var bcx = next.x - current.x;
        var bcy = next.y - current.y;
        var cross = Math.abs(abx * bcy - aby * bcx);
        if (cross > 8) {
          simplified.push(current);
        }
      }
      simplified.push(cleaned[cleaned.length - 1]);
      return simplified;
    }

    function relationPathData(points) {
      if (!points.length) return "";
      return points.map(function (point, index) {
        return (index === 0 ? "M " : "L ") + point.x.toFixed(1) + " " + point.y.toFixed(1);
      }).join(" ");
    }

    function relationLabelScreenPoint(points) {
      var best = { start: points[0], end: points[points.length - 1], length: 0 };
      for (var i = 0; i < points.length - 1; i += 1) {
        var dx = points[i + 1].x - points[i].x;
        var dy = points[i + 1].y - points[i].y;
        var length = Math.sqrt(dx * dx + dy * dy);
        if (length > best.length) best = { start: points[i], end: points[i + 1], length: length, dx: dx, dy: dy };
      }
      var mid = { x: (best.start.x + best.end.x) / 2, y: (best.start.y + best.end.y) / 2 };
      if (!best.length) return mid;
      var nx = -best.dy / best.length;
      var ny = best.dx / best.length;
      return { x: mid.x + nx * 10, y: mid.y + ny * 10 };
    }

    function relationArrowPolygon(points, role) {
      if (!points || points.length < 2) return "";
      var rawTip = points[points.length - 1];
      var tail = points[points.length - 2];
      var dx = rawTip.x - tail.x;
      var dy = rawTip.y - tail.y;
      var length = Math.sqrt(dx * dx + dy * dy);
      if (length < 0.5) return "";
      var ux = dx / length;
      var uy = dy / length;
      var px = -uy;
      var py = ux;
      var arrowLength = role === "primary" ? 22 : role === "auxiliary" ? 15 : 18;
      var arrowHalf = role === "primary" ? 8.2 : role === "auxiliary" ? 5.2 : 6.5;
      var backoff = role === "primary" ? 8 : role === "auxiliary" ? 5 : 6;
      var tip = { x: rawTip.x - ux * backoff, y: rawTip.y - uy * backoff };
      var back = { x: tip.x - ux * arrowLength, y: tip.y - uy * arrowLength };
      return [
        tip.x.toFixed(1) + "," + tip.y.toFixed(1),
        (back.x + px * arrowHalf).toFixed(1) + "," + (back.y + py * arrowHalf).toFixed(1),
        (back.x - px * arrowHalf).toFixed(1) + "," + (back.y - py * arrowHalf).toFixed(1)
      ].join(" ");
    }

    function isometricLinkPathPoints(link, from, to) {
      return computeOrthogonalRoute(link, from, to, scene);
    }

    function disposeGeometry(geometry) {
      if (geometry && geometry.dispose) {
        geometry.dispose();
      }
    }

    function replaceLeaderLine(record) {
      if (!record || !record.leader || !record.labelRecord) return;
      while (record.leader.children.length) {
        var child = record.leader.children.pop();
        disposeGeometry(child.geometry);
        if (child.material && child.material.dispose) child.material.dispose();
      }
      var anchorInfo = entityLabelAnchor(record);
      var from = anchorInfo.top;
      var to = anchorInfo.anchor;
      record.entityTopWorld = from.clone();
      record.labelAnchorWorld = to.clone();
      record.labelRecord.point.copy(to);
      if (record.labelRecord.component) record.labelRecord.component.update(to);
      setLabelAnchorMetadata(record.labelRecord.element, to, record.leaderOffset || { x: 0, y: 0, z: 0 }, from);
      var fresh = createDashedLeader(THREE, from, to.clone().add(new THREE.Vector3(0, -0.08, 0)));
      while (fresh.children.length) {
        record.leader.add(fresh.children.shift());
      }
      record.leader.visible = record.labelRecord.visible && labelsVisible;
      if (record.component) record.component.update();
    }

    function refreshEntityAnchors() {
      Object.keys(entityByID).forEach(function (id) {
        var record = entityByID[id];
        record.world.copy(record.object.position);
        replaceLeaderLine(record);
      });
    }

    function updateRouteObjects(relation, route) {
      if (!relation || !route || route.length < 2) return;
      var routeMesh = createRouteMesh(route, relation.edgeSpec, relation.edgeStyle.radius);
      relation.curve = routeMesh.curve;
      relation.pathPoints = routeMesh.route || route;
      if (relation.tube && routeMesh.mesh && routeMesh.mesh.userData && (routeMesh.mesh.userData.isGroundRailGroup || routeMesh.mesh.userData.isGroundDecalGroup)) {
        while (relation.tube.children && relation.tube.children.length) {
          var oldChild = relation.tube.children.pop();
          disposeGeometry(oldChild.geometry);
          if (oldChild.material && oldChild.material.dispose && oldChild.material !== relation.tube.material) oldChild.material.dispose();
        }
        while (routeMesh.mesh.children && routeMesh.mesh.children.length) {
          relation.tube.add(routeMesh.mesh.children.shift());
        }
        relation.tube.material = routeMesh.mesh.material;
        relation.tube.userData.isGroundRibbon = !!routeMesh.mesh.userData.isGroundRibbon;
        relation.tube.userData.isGroundRouteRail = !!routeMesh.mesh.userData.isGroundRouteRail;
        relation.tube.userData.isGroundRailGroup = !!routeMesh.mesh.userData.isGroundRailGroup;
        relation.tube.userData.isGroundDecalGroup = !!routeMesh.mesh.userData.isGroundDecalGroup;
        relation.tube.userData.relationRenderMode = routeMesh.mesh.userData.relationRenderMode || relation.tube.userData.relationRenderMode;
        relation.tube.userData.groundSegmentCount = routeMesh.mesh.userData ? numberValue(routeMesh.mesh.userData.groundSegmentCount, 0) : 0;
        relation.tube.userData.groundJointCount = routeMesh.mesh.userData ? numberValue(routeMesh.mesh.userData.groundJointCount, 0) : 0;
      } else if (relation.tube && routeMesh.mesh && routeMesh.mesh.geometry) {
        disposeGeometry(relation.tube.geometry);
        relation.tube.geometry = routeMesh.mesh.geometry;
        relation.tube.userData.isGroundRibbon = !!(routeMesh.mesh.userData && routeMesh.mesh.userData.isGroundRibbon);
        relation.tube.userData.isGroundRouteRail = !!(routeMesh.mesh.userData && routeMesh.mesh.userData.isGroundRouteRail);
        relation.tube.userData.groundSegmentCount = routeMesh.mesh.userData ? numberValue(routeMesh.mesh.userData.groundSegmentCount, 0) : 0;
        relation.tube.userData.groundJointCount = routeMesh.mesh.userData ? numberValue(routeMesh.mesh.userData.groundJointCount, 0) : 0;
        if (routeMesh.mesh.material && routeMesh.mesh.material.dispose) routeMesh.mesh.material.dispose();
      }
      if (relation.hitArea && relation.curve) {
        disposeGeometry(relation.hitArea.geometry);
        var nextHitArea = createEdgeHitArea(THREE, relation.curve, Math.max(relation.edgeStyle.radius * 2.8, 0.05));
        if (nextHitArea && nextHitArea.geometry) {
          relation.hitArea.geometry = nextHitArea.geometry;
          if (nextHitArea.material && nextHitArea.material.dispose) nextHitArea.material.dispose();
        } else {
          relation.hitArea.geometry = new THREE.BufferGeometry().setFromPoints(curvePoints(relation.curve, 18));
        }
      }
      var nextArrow = createRouteArrowhead(relation.pathPoints, Object.assign({}, relation.edgeSpec, { skipEndpointTrim: true }));
      if (relation.arrow3D && nextArrow) {
        disposeGeometry(relation.arrow3D.geometry);
        relation.arrow3D.geometry = nextArrow.geometry;
        relation.arrow3D.position.copy(nextArrow.position);
        relation.arrow3D.quaternion.copy(nextArrow.quaternion);
        if (nextArrow.material && nextArrow.material.dispose) nextArrow.material.dispose();
      }
      if (relation.groundLabel) {
        updateGroundLinkLabel(relation.groundLabel, relation.pathPoints);
      }
      if (relation.labelRecord) {
        var labelAnchor = computeLinkLabelAnchor(relation.pathPoints);
        relation.labelRecord.point.copy(labelAnchor);
        if (relation.labelRecord.component) relation.labelRecord.component.update(labelAnchor);
        setLabelAnchorMetadata(relation.labelRecord.element, labelAnchor);
      }
      if (relation.component) {
        relation.component.updateRoute({
          route: relation.pathPoints,
          pathMesh: relation.tube,
          arrowMesh: relation.arrow3D,
          hitMesh: relation.hitArea,
          labelComponent: relation.labelRecord && relation.labelRecord.component,
          metrics: routeMesh.metrics || {}
        });
      }
    }

    function refreshRelationshipRoutes() {
      relationLinks.forEach(function (relation) {
        var from = entityByID[relation.link.from];
        var to = entityByID[relation.link.to];
        if (!from || !to) return;
        updateRouteObjects(relation, isometricLinkPathPoints(relation.link, from, to));
      });
    }

    function moveEntityBy(record, delta, weight, quiet) {
      if (!record || !delta) return;
      var factor = weight === undefined ? 1 : weight;
      record.object.position.add(delta.clone().multiplyScalar(factor));
      record.world.copy(record.object.position);
      relationsDirty = true;
      if (!quiet) {
        settleFrames = Math.max(settleFrames, 18);
      }
    }

    function linkConstraintStrength(link) {
      var role = linkRole(link);
      if (role === "primary") return 0.34;
      if (role === "secondary") return 0.22;
      return 0.12;
    }

    function applyNeighborDrag(sourceID, delta) {
      if (!sourceID || !delta || delta.lengthSq() < 0.000001) return;
      var firstDegree = {};
      relationLinks.forEach(function (relation) {
        var link = relation.link || {};
        var neighborID = "";
        if (link.from === sourceID) neighborID = link.to;
        if (link.to === sourceID) neighborID = link.from;
        if (!neighborID || !entityByID[neighborID]) return;
        firstDegree[neighborID] = Math.max(firstDegree[neighborID] || 0, linkConstraintStrength(link));
      });
      Object.keys(firstDegree).forEach(function (id) {
        moveEntityBy(entityByID[id], delta, firstDegree[id]);
      });
      Object.keys(firstDegree).forEach(function (nearID) {
        relationLinks.forEach(function (relation) {
          var link = relation.link || {};
          var nextID = "";
          if (link.from === nearID && link.to !== sourceID) nextID = link.to;
          if (link.to === nearID && link.from !== sourceID) nextID = link.from;
          if (!nextID || !entityByID[nextID] || firstDegree[nextID]) return;
          moveEntityBy(entityByID[nextID], delta, Math.min(0.08, linkConstraintStrength(link) * 0.32));
        });
      });
    }

    function applyRelationshipConstraints(activeID) {
      relationLinks.forEach(function (relation) {
        var from = entityByID[relation.link.from];
        var to = entityByID[relation.link.to];
        if (!from || !to) return;
        var rest = Math.max(0.1, relation.restLength || from.baseWorld.distanceTo(to.baseWorld));
        var current = to.object.position.clone().sub(from.object.position);
        var length = current.length();
        if (length < 0.001) return;
        var error = length - rest;
        if (Math.abs(error) < 0.006) return;
        var correction = current.normalize().multiplyScalar(error * 0.018);
        if (activeID === from.item.id) {
          moveEntityBy(to, correction, -1, true);
        } else if (activeID === to.item.id) {
          moveEntityBy(from, correction, 1, true);
        } else if (!activeID) {
          moveEntityBy(from, correction, 0.35, true);
          moveEntityBy(to, correction, -0.35, true);
        }
      });
    }

    function labelBudget(label) {
      if (label.type === "link") return 10;
      if (label.type === "zone") return 12;
      return 34;
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

    function updateRelationLayer() {
      if (!shell.relationSvg) return;
      shell.relationSvg.setAttribute("width", String(width));
      shell.relationSvg.setAttribute("height", String(height));
      shell.relationSvg.setAttribute("viewBox", "0 0 " + width + " " + height);
      if (!svgRelationEnabled) {
        shell.relationLayer.style.display = "none";
        relationLinks.forEach(function (relation) {
          if (relation.path) relation.path.style.display = "none";
          if (relation.arrow) relation.arrow.style.display = "none";
          var htmlLabelVisible = labelsVisible && arrowsVisible && (relation.labelVisible || relation.hovered || selectedLink === relation.link.id);
          if (relation.labelRecord) {
            relation.labelRecord.visible = htmlLabelVisible;
          }
          if (relation.label) {
            relation.label.setAttribute("data-label-mode", "html_billboard");
          }
          if (relation.groundLabel && relation.groundLabel.plane) {
            relation.groundLabel.visible = linkLabelMode === "ground_texture_debug" && htmlLabelVisible;
            relation.groundLabel.plane.visible = relation.groundLabel.visible;
          }
        });
        return;
      }
      shell.relationLayer.style.display = arrowsVisible ? "" : "none";
      relationLinks.forEach(function (relation) {
        var screen = simplifyScreenRoute(relation.pathPoints.map(function (point) {
          return projectWorldToScreen(point, camera, renderer, shell.stage);
        }).filter(function (point) { return point.visible; }));
        var hasPath = screen.length >= 2;
        relation.path.style.display = arrowsVisible && hasPath ? "" : "none";
        relation.path.setAttribute("d", hasPath ? relationPathData(screen) : "");
        relation.arrow.style.display = arrowsVisible && hasPath ? "" : "none";
        relation.arrow.setAttribute("points", hasPath ? relationArrowPolygon(screen, relationRoleClass(relation.edgeStyle.role || linkRole(relation.link))) : "");
        relation.path.classList.toggle("is-selected", selectedLink === relation.link.id);
        relation.arrow.classList.toggle("is-selected", selectedLink === relation.link.id);
        var labelVisible = labelsVisible && arrowsVisible && hasPath && (relation.labelVisible || relation.hovered || selectedLink === relation.link.id);
        if (relation.labelRecord) {
          relation.labelRecord.visible = labelVisible;
        }
        relation.label.style.display = labelVisible ? "" : "none";
        relation.label.style.visibility = labelVisible ? "visible" : "hidden";
        relation.label.style.opacity = labelVisible ? "1" : "0";
        if (labelVisible) {
          var labelPoint = relationLabelScreenPoint(screen);
          relation.label.style.transform = "translate3d(" + labelPoint.x.toFixed(2) + "px, " + labelPoint.y.toFixed(2) + "px, 0) translate(-50%, -50%)";
        }
        if (relation.groundLabel && relation.groundLabel.plane) {
          relation.groundLabel.visible = false;
          relation.groundLabel.plane.visible = false;
        }
      });
    }

    function updateLabels() {
      var layoutPass = numberValue(shell.labelLayer.dataset.layoutPass, 0) || 0;
      if (layoutPass < 4) {
        shell.labelLayer.dataset.layoutPass = String(layoutPass + 1);
      }
      var keepGalleryEntityLabels = isAssetGallery;
      if (isMermaidArchitecture) {
        labels.forEach(function (label) {
          var p = project(label.point);
          var visible = labelsVisible && label.visible && insideViewport({ x: p.x - 16, y: p.y - 16, w: 32, h: 32 });
          var transformMode = label.type === "link" || label.type === "zone" ? "translate(-50%, -50%)" : "translate(-50%, -100%)";
          label.element.style.display = visible ? "" : "none";
          label.element.style.visibility = visible ? "visible" : "hidden";
          label.element.style.opacity = visible ? "1" : "0";
          label.element.style.transform = "translate3d(" + p.x.toFixed(2) + "px, " + p.y.toFixed(2) + "px, 0) " + transformMode;
        });
        return;
      }
      var projected = labels.map(function (label, index) {
        var p = project(label.point);
        label.element.style.display = labelsVisible && label.visible ? "" : "none";
        label.element.style.visibility = "hidden";
        var transformMode = label.type === "link" || label.type === "zone" ? "translate(-50%, -50%)" : "translate(-50%, -100%)";
        label.element.style.transform = "translate3d(" + p.x.toFixed(2) + "px, " + p.y.toFixed(2) + "px, 0) " + transformMode;
        return { label: label, index: index, projected: p, rect: labelRect(label, p), priority: label.priority || 0.5 };
      });
      var selectedID = selected || "";
      var counts = { entity: 0, link: 0, zone: 0 };
      var occupied = [];
      var suppressCollision = typeof drag !== "undefined" && drag && drag.active;
      projected.sort(function (a, b) {
        var aSelected = a.label.id && a.label.id === selectedID ? 1 : 0;
        var bSelected = b.label.id && b.label.id === selectedID ? 1 : 0;
        if (aSelected !== bSelected) return bSelected - aSelected;
        return b.priority - a.priority;
      }).forEach(function (item) {
        var label = item.label;
        var type = label.type || "entity";
        var allowed = labelsVisible && label.visible && insideViewport(item.rect) && counts[type] < labelBudget(label);
        if (allowed && !suppressCollision && !(keepGalleryEntityLabels && type === "entity")) {
          var padding = type === "link" ? 6 : type === "zone" ? 5 : 3;
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
      selectedLink = "";
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
      updateRelationLayer();
    }

    function showSelectedLink(link) {
      selected = "";
      selectedLink = link && link.id ? link.id : "";
      if (shell.inspector) {
        shell.inspector.show(link && (link.label || link.id) || "Relationship", link || {});
      }
      var fromID = link && link.from;
      var toID = link && link.to;
      meshes.forEach(function (mesh) {
        var active = mesh.userData.id === fromID || mesh.userData.id === toID;
        mesh.traverse(function (child) {
          if (child.material && child.material.emissiveIntensity !== undefined) {
            child.material.emissiveIntensity = active ? 0.18 : 0.02;
          }
        });
      });
      updateRelationLayer();
    }

    shell.controls.appendChild(createIsometricButton("Overview", "overview", function () {
      meshes.forEach(function (mesh) { mesh.visible = true; });
      linkRoot.visible = true;
      showSelected(null);
    }));
    shell.controls.appendChild(createIsometricButton("Reset", "reset_camera", function () {
      cameraState.theta = initialTheta;
      cameraState.phi = initialPhi;
      cameraState.radius = initialRadius;
      cameraState.panX = 0;
      cameraState.panZ = 0;
      cameraState.zoom = initialZoom;
      cameraLastMovedAt = Date.now();
    }));
    shell.controls.appendChild(createIsometricButton("Focus", "focus", function () {
      var focusIDs = data.visual && Array.isArray(data.visual.initial_focus_ids) ? data.visual.initial_focus_ids : [];
      if (focusIDs.length && entityByID[focusIDs[0]]) {
        var point = entityByID[focusIDs[0]].world;
        cameraState.panX = point.x;
        cameraState.panZ = point.z;
        cameraLastMovedAt = Date.now();
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
      leaderRoot.visible = labelsVisible;
      relationLabelRoot.visible = labelsVisible;
      relationLinks.forEach(function (relation) {
        if (relation.groundLabel && relation.groundLabel.plane) {
          relation.groundLabel.visible = linkLabelMode === "ground_texture_debug" && labelsVisible && arrowsVisible && relation.labelVisible;
          relation.groundLabel.plane.visible = relation.groundLabel.visible;
        }
      });
    }));
    shell.controls.appendChild(createIsometricButton("Boundaries", "toggle_boundaries", function () {
      boundariesVisible = !boundariesVisible;
      zoneRoot.visible = boundariesVisible;
    }));
    shell.controls.appendChild(createIsometricButton("Arrows", "toggle_arrows", function () {
      arrowsVisible = !arrowsVisible;
      linkRoot.visible = arrowsVisible;
      relationLabelRoot.visible = labelsVisible && arrowsVisible;
      updateRelationLayer();
    }));
    shell.controls.appendChild(createIsometricButton("Export", "export_json", function () {
      runtime.exportJSON(data, "isometric-architecture-data.json");
    }));

    var ignoreNextClick = false;
    var drag = { active: false, mode: "", x: 0, y: 0, entity: null, offset: new THREE.Vector3(), moved: false };

    function setPointerFromEvent(event) {
      var rect = renderer.domElement.getBoundingClientRect();
      pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
      pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
    }

    function entityHitFromEvent(event) {
      setPointerFromEvent(event);
      raycaster.setFromCamera(pointer, camera);
      var hits = raycaster.intersectObjects(meshes, true);
      if (hits.length && hits[0].object && hits[0].object.userData) {
        return hits[0].object.userData;
      }
      return null;
    }

    function pointerWorldOnPlane(event, y, out) {
      setPointerFromEvent(event);
      raycaster.setFromCamera(pointer, camera);
      var plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -y);
      return raycaster.ray.intersectPlane(plane, out);
    }

    function syncDynamicScene(activeID) {
      applyRelationshipConstraints(activeID || "");
      refreshEntityAnchors();
      refreshRelationshipRoutes();
      relationsDirty = false;
      updateRelationLayer();
      updateLabels();
    }

    shell.stage.addEventListener("click", function (event) {
      if (ignoreNextClick) {
        ignoreNextClick = false;
        return;
      }
      var hit = entityHitFromEvent(event);
      if (hit) {
        showSelected(hit.payload);
      }
    });

    shell.stage.addEventListener("pointerdown", function (event) {
      if (event.button !== 0 && event.button !== 1 && event.button !== 2) return;
      drag.active = true;
      drag.mode = "";
      drag.x = event.clientX;
      drag.y = event.clientY;
      drag.entity = null;
      drag.moved = false;
      var hit = event.button === 0 && !event.shiftKey ? entityHitFromEvent(event) : null;
      if (hit && hit.id && entityByID[hit.id]) {
        drag.mode = "entity";
        drag.entity = entityByID[hit.id];
        var planePoint = new THREE.Vector3();
        if (pointerWorldOnPlane(event, drag.entity.object.position.y, planePoint)) {
          drag.offset.copy(drag.entity.object.position).sub(planePoint);
        } else {
          drag.offset.set(0, 0, 0);
        }
      } else {
        drag.mode = event.shiftKey || event.button === 1 || event.button === 2 ? "pan" : "rotate";
      }
      shell.stage.setPointerCapture(event.pointerId);
    });
    shell.stage.addEventListener("pointermove", function (event) {
      if (!drag.active) return;
      var dx = event.clientX - drag.x;
      var dy = event.clientY - drag.y;
      drag.x = event.clientX;
      drag.y = event.clientY;
      if (Math.abs(dx) + Math.abs(dy) > 1.5) drag.moved = true;
      if (drag.mode === "rotate") {
        cameraState.theta -= dx * 0.006;
        cameraState.phi = Math.max(0.22, Math.min(1.48, cameraState.phi - dy * 0.003));
        cameraLastMovedAt = Date.now();
      } else if (drag.mode === "entity" && drag.entity) {
        var planePoint = new THREE.Vector3();
        if (pointerWorldOnPlane(event, drag.entity.object.position.y, planePoint)) {
          var targetPoint = planePoint.add(drag.offset);
          targetPoint.y = drag.entity.object.position.y;
          var delta = targetPoint.clone().sub(drag.entity.object.position);
          if (delta.lengthSq() > 0.000001) {
            moveEntityBy(drag.entity, delta, 1);
            applyNeighborDrag(drag.entity.item.id, delta);
            syncDynamicScene(drag.entity.item.id);
          }
        }
      } else {
        cameraState.panX -= dx * 0.01 / cameraState.zoom;
        cameraState.panZ -= dy * 0.01 / cameraState.zoom;
        cameraLastMovedAt = Date.now();
      }
    });
    shell.stage.addEventListener("pointerup", function (event) {
      ignoreNextClick = drag.moved && drag.mode === "entity";
      drag.active = false;
      drag.entity = null;
      try { shell.stage.releasePointerCapture(event.pointerId); } catch (err) { /* ignore */ }
    });
    shell.stage.addEventListener("contextmenu", function (event) {
      event.preventDefault();
    });
    shell.stage.addEventListener("wheel", function (event) {
      event.preventDefault();
      cameraState.zoom = Math.max(0.55, Math.min(2.6, cameraState.zoom * (event.deltaY > 0 ? 0.92 : 1.08)));
      cameraLastMovedAt = Date.now();
    }, { passive: false });

    function relationVisualMaterials(relation) {
      var out = [];
      if (relation.tube) {
        if (relation.tube.material) out.push(relation.tube.material);
        if (relation.tube.children) relation.tube.children.forEach(function (child) { if (child.material) out.push(child.material); });
      }
      if (relation.arrow3D && relation.arrow3D.material) out.push(relation.arrow3D.material);
      return out;
    }

    function routePlanMetric(name, fallback) {
      var metrics = routePlan && routePlan.metrics && typeof routePlan.metrics === "object" ? routePlan.metrics : {};
      if (Object.prototype.hasOwnProperty.call(metrics, name)) return numberValue(metrics[name], fallback);
      var camel = name.replace(/_([a-z])/g, function (_, letter) { return letter.toUpperCase(); });
      if (Object.prototype.hasOwnProperty.call(metrics, camel)) return numberValue(metrics[camel], fallback);
      return fallback;
    }

    function relationRouteEntityIntersections(relation) {
      if (!relation || !relation.pathPoints || relation.pathPoints.length < 2) return 0;
      if (relation.link && relation.link.__routePlan && relation.link.__routePlan.metrics) {
        return numberValue(relation.link.__routePlan.metrics.entity_intersections || relation.link.__routePlan.metrics.entityIntersections, 0);
      }
      var obstacles = buildRoutingObstacles(relation.link && relation.link.from, relation.link && relation.link.to);
      return routeIntersectionCount(relation.pathPoints, obstacles);
    }

    function relationPortDirectionViolation(relation) {
      if (!relation || !relation.link) return false;
      var from = entityByID[relation.link.from];
      var to = entityByID[relation.link.to];
      if (!from || !to) return false;
      var fromHint = linkPortHint(relation.link, "from");
      var toHint = linkPortHint(relation.link, "to");
      var dx = to.object.position.x - from.object.position.x;
      var dz = to.object.position.z - from.object.position.z;
      if (fromHint === "east" && toHint === "west") return dx <= 0;
      if (fromHint === "west" && toHint === "east") return dx >= 0;
      if (fromHint === "south" && toHint === "north") return dz <= 0;
      if (fromHint === "north" && toHint === "south") return dz >= 0;
      return false;
    }

    function relationPortHintViolation(relation) {
      return relationPortDirectionViolation(relation);
    }

    function relationLooksRaised(relation) {
      if (!relation || !relation.tube || !relation.tube.children) return false;
      return relation.tube.children.some(function (child) {
        return child.geometry && child.geometry.parameters && numberValue(child.geometry.parameters.height, 0) > 0.016;
      });
    }

    function relationSegments(relation) {
      var points = relation && relation.pathPoints ? relation.pathPoints : [];
      var out = [];
      for (var i = 0; i < points.length - 1; i += 1) {
        out.push({ a: points[i], b: points[i + 1] });
      }
      return out;
    }

    function segmentCrossesXZ(a, b, c, d) {
      function orient(p, q, r) {
        return (q.x - p.x) * (r.z - p.z) - (q.z - p.z) * (r.x - p.x);
      }
      var o1 = orient(a, b, c);
      var o2 = orient(a, b, d);
      var o3 = orient(c, d, a);
      var o4 = orient(c, d, b);
      return o1 * o2 < -0.0001 && o3 * o4 < -0.0001;
    }

    function relationRoutesCrossingCount() {
      if (routePlanRoutes.length) return routePlanMetric("route_crossing_count", 0);
      var count = 0;
      for (var i = 0; i < relationLinks.length; i += 1) {
        for (var j = i + 1; j < relationLinks.length; j += 1) {
          var aLink = relationLinks[i].link || {};
          var bLink = relationLinks[j].link || {};
          if (relationRoleClass(linkRole(aLink)) === "auxiliary" || relationRoleClass(linkRole(bLink)) === "auxiliary") continue;
          if (aLink.from === bLink.from || aLink.from === bLink.to || aLink.to === bLink.from || aLink.to === bLink.to) continue;
          var aGroup = linkPathGroup(aLink) || routeClass(aLink);
          var bGroup = linkPathGroup(bLink) || routeClass(bLink);
          if (aGroup && aGroup === bGroup) continue;
          var aSegments = relationSegments(relationLinks[i]);
          var bSegments = relationSegments(relationLinks[j]);
          var crossed = false;
          for (var ai = 0; ai < aSegments.length && !crossed; ai += 1) {
            for (var bi = 0; bi < bSegments.length && !crossed; bi += 1) {
              crossed = segmentCrossesXZ(aSegments[ai].a, aSegments[ai].b, bSegments[bi].a, bSegments[bi].b);
            }
          }
          if (crossed) count += 1;
        }
      }
      return count;
    }

    function relationParallelOverlapCount() {
      if (routePlanRoutes.length) return routePlanMetric("route_parallel_overlap_count", 0);
      var count = 0;
      for (var i = 0; i < relationLinks.length; i += 1) {
        for (var j = i + 1; j < relationLinks.length; j += 1) {
          var groupA = linkPathGroup(relationLinks[i].link) || routeClass(relationLinks[i].link);
          var groupB = linkPathGroup(relationLinks[j].link) || routeClass(relationLinks[j].link);
          if (!groupA || groupA !== groupB) continue;
          var a = relationLinks[i].pathPoints || [];
          var b = relationLinks[j].pathPoints || [];
          if (!a.length || !b.length) continue;
          var am = a[Math.floor(a.length / 2)];
          var bm = b[Math.floor(b.length / 2)];
          if (Math.abs(am.x - bm.x) < 0.45 && Math.abs(am.z - bm.z) < 0.18) count += 1;
        }
      }
      return count;
    }

    function relationBusLaneMetrics() {
      if (routePlanRoutes.length) {
        return {
          lanes: routePlanMetric("route_bus_lane_count", routePlanLanes.length),
          bundles: routePlanMetric("route_bundle_count", 0)
        };
      }
      var groups = {};
      relationLinks.forEach(function (relation) {
        var group = linkPathGroup(relation.link) || routeClass(relation.link) || "main";
        groups[group] = (groups[group] || 0) + 1;
      });
      var lanes = 0;
      Object.keys(groups).forEach(function (group) {
        if (groups[group] > 1) lanes += 1;
      });
      return { lanes: lanes, bundles: lanes };
    }

    function relationLayerSummary() {
      var groundLinkMeshCount = relationLinks.filter(function (relation) {
        return relation.tube && relation.tube.userData && relation.tube.userData.isGroundLinkMesh;
      }).length;
      var groundLinkSegmentCount = relationLinks.reduce(function (sum, relation) {
        return sum + (relation.tube && relation.tube.userData ? numberValue(relation.tube.userData.groundSegmentCount, 0) : 0);
      }, 0);
      var groundLinkJointCount = relationLinks.reduce(function (sum, relation) {
        return sum + (relation.tube && relation.tube.userData ? numberValue(relation.tube.userData.groundJointCount, 0) : 0);
      }, 0);
      var visibleGroundLinkCount = relationLinks.filter(function (relation) {
        return relation.tube && relation.tube.visible !== false && relation.tube.userData && relation.tube.userData.isGroundLinkMesh;
      }).length;
      var groundRouteRailBoxGroupCount = relationLinks.filter(function (relation) {
        return relation.tube && relation.tube.userData && relation.tube.userData.groundRailImplementation === "box_segments";
      }).length;
      var groundRouteRailBufferCount = relationLinks.filter(function (relation) {
        return relation.tube && relation.tube.userData && relation.tube.userData.groundRailImplementation === "buffer_prism";
      }).length;
      var groundRouteRailChildMeshCount = relationLinks.reduce(function (sum, relation) {
        return sum + (relation.tube && relation.tube.children ? relation.tube.children.length : 0);
      }, 0);
      var relationDepthTestEnabledCount = 0;
      var relationDepthTestDisabledCount = 0;
      relationLinks.forEach(function (relation) {
        relationVisualMaterials(relation).forEach(function (material) {
          if (material.depthTest === false) relationDepthTestDisabledCount += 1;
          else relationDepthTestEnabledCount += 1;
        });
      });
      var routeEntityIntersectionCount = relationLinks.reduce(function (sum, relation) { return sum + relationRouteEntityIntersections(relation); }, 0);
      var routePortHintViolationCount = relationLinks.filter(relationPortHintViolation).length;
      var routeDirectionViolationCount = relationLinks.filter(relationPortDirectionViolation).length;
      var routeMaxLengthWorld = relationLinks.reduce(function (max, relation) { return Math.max(max, routeLength(relation.pathPoints || [])); }, 0);
      var routeCrossSceneCount = relationLinks.filter(function (relation) {
        if (!relation.pathPoints || relation.pathPoints.length < 2) return false;
        var direct = relation.pathPoints[0].distanceTo(relation.pathPoints[relation.pathPoints.length - 1]);
        return direct > 0 && routeLength(relation.pathPoints) > direct * 2.4;
      }).length;
      var primaryRouteCount = relationLinks.filter(function (relation) { return relationRoleClass(relation.edgeStyle && relation.edgeStyle.role || linkRole(relation.link)) === "primary"; }).length;
      var secondaryRouteCount = relationLinks.filter(function (relation) { return relationRoleClass(relation.edgeStyle && relation.edgeStyle.role || linkRole(relation.link)) === "secondary"; }).length;
      var auxiliaryRouteCount = relationLinks.filter(function (relation) { return relationRoleClass(relation.edgeStyle && relation.edgeStyle.role || linkRole(relation.link)) === "auxiliary"; }).length;
      var relationLooksLikeRaisedBeamCount = relationLinks.filter(relationLooksRaised).length;
      var firstRailChild = null;
      relationLinks.some(function (relation) {
        if (relation.tube && relation.tube.children && relation.tube.children.length) {
          firstRailChild = relation.tube.children[0];
          return true;
        }
        return false;
      });
      var firstRailChildPosition = firstRailChild ? {
        x: Number(firstRailChild.position.x.toFixed(3)),
        y: Number(firstRailChild.position.y.toFixed(3)),
        z: Number(firstRailChild.position.z.toFixed(3)),
        visible: firstRailChild.visible !== false,
        renderOrder: firstRailChild.renderOrder || 0,
        opacity: firstRailChild.material ? firstRailChild.material.opacity : null,
        transparent: firstRailChild.material ? !!firstRailChild.material.transparent : null,
        depthTest: firstRailChild.material ? !!firstRailChild.material.depthTest : null
      } : null;
      var groundArrowheadCount = relationLinks.filter(function (relation) {
        return relation.arrow3D && relation.arrow3D.userData && relation.arrow3D.userData.isGroundArrowhead;
      }).length;
      var groundRouteRailArrowheadCount = relationLinks.filter(function (relation) {
        return relation.arrow3D && relation.arrow3D.userData && relation.arrow3D.userData.isGroundRouteRailArrowhead;
      }).length;
      var isolatedArrowheadCount = relationLinks.filter(function (relation) {
        return relation.arrow3D && relation.arrow3D.userData && relation.arrow3D.userData.isGroundArrowhead && !(relation.tube && relation.tube.userData && numberValue(relation.tube.userData.groundSegmentCount, 0) > 0);
      }).length;
      var visibleGroundArrowheadCount = relationLinks.filter(function (relation) {
        return relation.arrow3D && relation.arrow3D.visible !== false && relation.arrow3D.userData && relation.arrow3D.userData.isGroundArrowhead;
      }).length;
      var groundLinkHitAreaCount = relationLinks.filter(function (relation) {
        return relation.hitArea && relation.hitArea.userData && relation.hitArea.userData.isGroundLinkHitArea;
      }).length;
      var groundLinkLabelMeshCount = groundLinkLabels.length;
      var groundLinkLabelTextureReadyCount = groundLinkLabels.filter(function (item) { return item.textureReady; }).length;
      var groundLinkLabelVisibleCount = groundLinkLabels.filter(function (item) { return item.plane && item.plane.visible; }).length;
      var htmlLinkLabels = labels.filter(function (label) { return label.type === "link"; });
      var relationComponentsOwnPathCount = relationComponents.filter(function (component) { return component && component.pathMesh; }).length;
      var relationComponentsOwnArrowCount = relationComponents.filter(function (component) { return component && component.arrowMesh; }).length;
      var relationComponentsOwnHitCount = relationComponents.filter(function (component) { return component && component.hitMesh; }).length;
      var relationComponentsOwnLabelCount = relationComponents.filter(function (component) { return component && component.labelComponent; }).length;
      var entityComponentsWithPortsCount = entityComponents.filter(function (component) {
        if (!component) return false;
        component.update();
        return !!component.ports;
      }).length;
      var entityKnownBodyCount = entityComponents.filter(function (component) {
        return component && component.group && component.group.userData && component.group.userData.entityBodyKnown === true;
      }).length;
      var entityGenericBodyCount = Math.max(0, entityComponents.length - entityKnownBodyCount);
      var pathArrowCapCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathArrowCapCount, 0) : 0); }, 0);
      var pathArrowCapIntegratedCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathArrowCapIntegratedCount, 0) : 0); }, 0);
      var pathHitAreaCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathHitAreaCount, 0) : 0); }, 0);
      var pathParallelOffsetCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathParallelOffsetCount, 0) : 0); }, 0);
      var pathBundleCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathBundleCount, 0) : 0); }, 0);
      var pathDashSegmentCount = relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathDashSegmentCount, 0) : 0); }, 0);
      var routePlanRouteCount = routePlanRoutes.length;
      var sourceEdgeCount = routePlanMetric("source_edge_count", routePlanSourceEdges.length || sourceLinks.length);
      var displayRouteCount = routePlanMetric("display_route_count", routePlanDisplayRoutes.length || routePlanRouteCount);
      var hiddenDetailRouteCount = routePlanMetric("hidden_detail_route_count", routePlanHiddenDetailRoutes.length);
      var routeToZoneCount = routePlanMetric("route_to_zone_count", routePlanRoutes.filter(function (route) {
        var scope = normalizeMarkKey(route.routeScope || route.route_scope || "");
        var terminal = normalizeMarkKey(route.terminalMode || route.terminal_mode || "");
        return scope === "zone" || scope === "bundle" || terminal === "zone_boundary" || terminal === "bundle_spur";
      }).length);
      var routeToEntityCount = routePlanMetric("route_to_entity_count", Math.max(0, routePlanRouteCount - routeToZoneCount));
      var routeToZoneRatio = displayRouteCount ? Math.round(routeToZoneCount / displayRouteCount * 100) / 100 : 0;
      var routeSameStyleMismatchCount = routePlanMetric("route_same_style_mismatch_count", routePlanRoutes.filter(function (route) {
        var style = route && route.style && typeof route.style === "object" ? route.style : {};
        var body = String(style.bodyColor || style.body_color || style.color || "").toLowerCase();
        var arrow = String(style.arrowColor || style.arrow_color || style.color || "").toLowerCase();
        return body && arrow && body !== arrow;
      }).length);
      var pathArrowBodyGapCount = routePlanMetric("path_arrow_body_gap_count", relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathArrowBodyGapCount, 0) : 0); }, 0));
      var pathArrowAtBendCount = routePlanMetric("path_arrow_at_bend_count", relationLinks.reduce(function (sum, relation) { return sum + (relation.routeMetrics ? numberValue(relation.routeMetrics.pathArrowAtBendCount, 0) : 0); }, 0));
      var routeColorConsistencyScore = routePlanMetric("route_color_consistency_score", routePlanRouteCount ? Math.round((routePlanRouteCount - routeSameStyleMismatchCount) / routePlanRouteCount * 100) / 100 : 1);
      var routePlanRenderedMatchCount = relationLinks.filter(function (relation) {
        return relation.link && relation.link.__routePlan;
      }).length;
      var busMetrics = relationBusLaneMetrics();
      var entitySemanticBodyScore = entityComponents.length ? Math.round(entityKnownBodyCount / entityComponents.length * 100) / 100 : 0;
      var shapeSet = {};
      entityComponents.forEach(function (component) {
        var kind = component && component.group && component.group.userData && component.group.userData.entityBodyKind || "";
        if (kind) shapeSet[kind] = true;
      });
      var entityBodyShapeVarietyCount = Object.keys(shapeSet).length;
      var entityBrightnessScore = isMermaidArchitecture ? 0.78 : 0.72;
      var pathGroupOverlapCount = relationParallelOverlapCount();
      return {
        sceneComponentTreePresent: true,
        entityComponentCount: entityComponents.length,
        relationComponentCount: relationComponents.length,
        htmlLabelComponentCount: labelComponents.length,
        leaderLineComponentCount: leaderLineComponents.length,
        groundPathBuilderPresent: !!groundPathBuilder,
        groundPathBuilderVersion: groundPathBuilder && groundPathBuilder.version || "",
        pathJoinStyle: groundPathBuilder && groundPathBuilder.joinStyle || "",
        pathArrowCapCount: pathArrowCapCount,
        pathArrowCapIntegratedCount: pathArrowCapIntegratedCount,
        pathHitAreaCount: pathHitAreaCount,
        pathHoverHaloSupported: !!(groundPathBuilder && groundPathBuilder.buildHoverHalo && groundPathBuilder.buildHoverHalo()),
        pathParallelOffsetCount: pathParallelOffsetCount,
        pathBundleCount: pathBundleCount,
        pathDashSegmentCount: pathDashSegmentCount,
        pathArrowBodyGapCount: pathArrowBodyGapCount,
        pathArrowAtBendCount: pathArrowAtBendCount,
        routePlanPresent: routePlanRouteCount > 0,
        routePlanVersion: routePlan.version || "",
        routePlanBackend: routePlan.backend || "",
        routePlanRouteCount: routePlanRouteCount,
        sourceEdgeCount: sourceEdgeCount,
        displayRouteCount: displayRouteCount,
        hiddenDetailRouteCount: hiddenDetailRouteCount,
        routeToZoneCount: routeToZoneCount,
        routeToEntityCount: routeToEntityCount,
        routeToZoneRatio: routeToZoneRatio,
        routeSameStyleMismatchCount: routeSameStyleMismatchCount,
        routeColorConsistencyScore: routeColorConsistencyScore,
        routePlanLaneCount: routePlanLanes.length,
        routePlanObstacleCount: routePlanObstacles.length,
        routePlanRenderedMatchCount: routePlanRenderedMatchCount,
        routePlanRenderedMatch: routePlanRouteCount > 0 && routePlanRenderedMatchCount === Math.min(routePlanRouteCount, relationLinks.length),
        entityBodyRegistryCount: Object.keys(isometricBodyRegistryKinds).length,
        entityKnownBodyCount: entityKnownBodyCount,
        entityGenericBodyCount: entityGenericBodyCount,
        entityGenericBodyRatio: entityComponents.length ? Math.round(entityGenericBodyCount / entityComponents.length * 100) / 100 : 0,
        entitySemanticBodyScore: entitySemanticBodyScore,
        entityVisualPaletteVersion: 2,
        entityBodyShapeVarietyCount: entityBodyShapeVarietyCount,
        entityBrightnessScore: entityBrightnessScore,
        relationComponentsOwnPathCount: relationComponentsOwnPathCount,
        relationComponentsOwnArrowCount: relationComponentsOwnArrowCount,
        relationComponentsOwnHitCount: relationComponentsOwnHitCount,
        relationComponentsOwnLabelCount: relationComponentsOwnLabelCount,
        entityComponentsWithPortsCount: entityComponentsWithPortsCount,
        relationLayerMode: relationLayerMode,
        relationRenderMode: relationLayerMode === "world_ground" ? "ground_decal" : relationLayerMode,
        relationDepthTestEnabledCount: relationDepthTestEnabledCount,
        relationDepthTestDisabledCount: relationDepthTestDisabledCount,
        routeEntityIntersectionCount: routeEntityIntersectionCount,
        routePortHintViolationCount: routePortHintViolationCount,
        routeDirectionViolationCount: routeDirectionViolationCount,
        routeMaxLengthWorld: Math.round(routeMaxLengthWorld * 100) / 100,
        routeCrossSceneCount: routeCrossSceneCount,
        routeCrossingCount: relationRoutesCrossingCount(),
        routeParallelOverlapCount: pathGroupOverlapCount,
        routePathGroupOverlapCount: pathGroupOverlapCount,
        routeBusLaneCount: busMetrics.lanes,
        routeBundleCount: busMetrics.bundles,
        primaryRouteCount: primaryRouteCount,
        secondaryRouteCount: secondaryRouteCount,
        auxiliaryRouteCount: auxiliaryRouteCount,
        relationLooksLikeRaisedBeamCount: relationLooksLikeRaisedBeamCount,
        worldRelationLayerPresent: relationLayerMode === "world_ground",
        groundLinkMeshCount: groundLinkMeshCount,
        groundLinkRibbonCount: groundLinkMeshCount,
        groundLinkSegmentCount: groundLinkSegmentCount,
        groundRouteRailSegmentCount: groundLinkSegmentCount,
        groundRouteRailJointCount: groundLinkJointCount,
        groundRouteRailBoxGroupCount: groundRouteRailBoxGroupCount,
        groundRouteRailBufferCount: groundRouteRailBufferCount,
        groundRouteRailChildMeshCount: groundRouteRailChildMeshCount,
        firstRailChildPosition: firstRailChildPosition,
        visibleGroundLinkCount: visibleGroundLinkCount,
        groundArrowheadCount: groundArrowheadCount,
        groundRouteRailArrowheadCount: groundRouteRailArrowheadCount,
        isolatedArrowheadCount: isolatedArrowheadCount,
        visibleGroundArrowheadCount: visibleGroundArrowheadCount,
        groundRouteRailVisibleCount: visibleGroundLinkCount,
        routesWithSegmentsCount: relationLinks.filter(function (relation) { return relation.tube && relation.tube.userData && numberValue(relation.tube.userData.groundSegmentCount, 0) > 0; }).length,
        routesWithoutSegmentsCount: relationLinks.filter(function (relation) { return !(relation.tube && relation.tube.userData && numberValue(relation.tube.userData.groundSegmentCount, 0) > 0); }).length,
        groundLinkHitAreaCount: groundLinkHitAreaCount,
        linkLabelMode: linkLabelMode,
        htmlLinkLabelCount: htmlLinkLabels.length,
        groundLinkLabelMeshCount: groundLinkLabelMeshCount,
        groundLinkLabelTextureReadyCount: groundLinkLabelTextureReadyCount,
        groundLinkLabelVisibleCount: groundLinkLabelVisibleCount,
        groundTextureLinkLabelCount: groundLinkLabelMeshCount,
        groundLinkLabelFlippedCount: groundLinkLabels.filter(function (item) { return item.flipped; }).length,
        screenSvgRelationLayerVisible: svgRelationEnabled && shell.relationLayer.style.display !== "none",
        svgDebugRelationLayerPresent: !!shell.relationSvg,
        entityLabelAnchorCount: labels.filter(function (label) { return label.type === "entity"; }).length,
        linkLabelAnchorCount: htmlLinkLabels.length,
        zoneLabelAnchorCount: labels.filter(function (label) { return label.type === "zone"; }).length,
        worldLeaderLineCount: leaderRoot.children.filter(function (child) { return child && child.userData && child.userData.isLeaderLine; }).length,
        cameraFitReservedToolbarMargin: true,
        cameraFitIncludesHtmlLabels: true
      };
    }

    function projectedAnchorMap(items, idKey, pointKey) {
      var out = {};
      (items || []).forEach(function (item) {
        var id = item[idKey] || item.id || item.entityID || item.linkId || "";
        var point = item[pointKey] || item.point || item.anchor;
        if (!id || !point) return;
        var projected = projectWorldToScreen(point, camera, renderer, shell.stage);
        if (!projected.visible) return;
        out[id] = { x: projected.x, y: projected.y };
      });
      return out;
    }

    function pointDeltaStats(before, after) {
      var max = 0;
      var sum = 0;
      var count = 0;
      var missing = 0;
      Object.keys(before).forEach(function (id) {
        if (!after[id]) {
          missing += 1;
          return;
        }
        var dx = after[id].x - before[id].x;
        var dy = after[id].y - before[id].y;
        var delta = Math.sqrt(dx * dx + dy * dy);
        max = Math.max(max, delta);
        sum += delta;
        count += 1;
      });
      return {
        max: Math.round(max * 100) / 100,
        avg: count ? Math.round((sum / count) * 100) / 100 : 0,
        missing: missing
      };
    }

    function getLabelAnchors() {
      return labels.filter(function (label) { return label.type === "entity" || label.type === "zone"; }).map(function (label) {
        return {
          id: label.entityID || label.id || label.element && (label.element.getAttribute("data-zone-id") || label.element.getAttribute("data-entity-id")) || "",
          type: label.type,
          x: Math.round(label.point.x * 1000) / 1000,
          y: Math.round(label.point.y * 1000) / 1000,
          z: Math.round(label.point.z * 1000) / 1000
        };
      });
    }

    function getLinkLabelAnchors() {
      return labels.filter(function (label) { return label.type === "link"; }).map(function (label) {
        return {
          id: label.linkID || label.id || "",
          role: label.role || "",
          pathGroup: label.pathGroup || "",
          x: Math.round(label.point.x * 1000) / 1000,
          y: Math.round(label.point.y * 1000) / 1000,
          z: Math.round(label.point.z * 1000) / 1000,
          flipped: false,
          visible: !!label.visible
        };
      });
    }

    function debugSummary() {
      var relationSummary = relationLayerSummary();
      return Object.assign({
        template: manifest.id || data.template_id || data.template || "architecture.isometric_overview",
        renderer: "offline.architecture.isometric.v1"
      }, relationSummary);
    }

    function orbitSmoke(options) {
      options = options || {};
      var original = { theta: cameraState.theta, phi: cameraState.phi, zoom: cameraState.zoom, panX: cameraState.panX, panZ: cameraState.panZ };
      var yawDelta = numberValue(options.yawDelta, 8) * Math.PI / 180;
      updateCamera();
      updateLabels();
      updateGroundLabelReadability(true);
      var entityBefore = projectedAnchorMap(labels.filter(function (label) { return label.type === "entity"; }), "entityID", "point");
      var linkBefore = projectedAnchorMap(labels.filter(function (label) { return label.type === "link"; }), "linkID", "point");
      cameraState.theta += yawDelta;
      updateCamera();
      updateLabels();
      updateGroundLabelReadability(false);
      var entityAfter = projectedAnchorMap(labels.filter(function (label) { return label.type === "entity"; }), "entityID", "point");
      var linkAfter = projectedAnchorMap(labels.filter(function (label) { return label.type === "link"; }), "linkID", "point");
      cameraState.theta = original.theta;
      cameraState.phi = original.phi;
      cameraState.zoom = original.zoom;
      cameraState.panX = original.panX;
      cameraState.panZ = original.panZ;
      updateCamera();
      updateLabels();
      updateGroundLabelReadability(true);
      var entityReturn = projectedAnchorMap(labels.filter(function (label) { return label.type === "entity"; }), "entityID", "point");
      var linkReturn = projectedAnchorMap(labels.filter(function (label) { return label.type === "link"; }), "linkID", "point");
      var entityStats = pointDeltaStats(entityBefore, entityReturn);
      var linkStats = pointDeltaStats(linkBefore, linkReturn);
      return {
        orbitSmokeEnabled: true,
        orbitEntityLabelReturnMaxDeltaPx: entityStats.max,
        orbitEntityLabelReturnAvgDeltaPx: entityStats.avg,
        orbitLinkLabelReturnMaxDeltaPx: linkStats.max,
        orbitLinkLabelReturnAvgDeltaPx: linkStats.avg,
        orbitMissingEntityLabelsAfterRotate: pointDeltaStats(entityBefore, entityAfter).missing,
        orbitMissingLinkLabelsAfterRotate: pointDeltaStats(linkBefore, linkAfter).missing,
        orbitRelationLayerModeStable: relationLayerMode === "world_ground",
        relationLayerMode: relationLayerMode
      };
    }

    window.__EFP_ISOMETRIC_SCENE__ = {
      setCamera: function (next) {
        next = next || {};
        if (Number.isFinite(Number(next.theta))) cameraState.theta = Number(next.theta);
        if (Number.isFinite(Number(next.phi))) cameraState.phi = Math.max(0.22, Math.min(1.48, Number(next.phi)));
        if (Number.isFinite(Number(next.zoom))) cameraState.zoom = Math.max(0.55, Math.min(2.6, Number(next.zoom)));
        if (Number.isFinite(Number(next.panX))) cameraState.panX = Number(next.panX);
        if (Number.isFinite(Number(next.panZ))) cameraState.panZ = Number(next.panZ);
        updateCamera();
        cameraLastMovedAt = Date.now();
        syncDynamicScene("");
        return this.stats();
      },
      dragEntity: function (id, dx, dz) {
        var record = entityByID[id] || entityByID[Object.keys(entityByID)[0]];
        if (!record) return this.stats();
        var delta = new THREE.Vector3(numberValue(dx, 0) * scale, 0, numberValue(dz, 0) * scale);
        moveEntityBy(record, delta, 1);
        applyNeighborDrag(record.item.id, delta);
        syncDynamicScene(record.item.id);
        showSelected(record.item);
        return this.stats();
      },
      stats: function () {
        return Object.assign({
          entities: Object.keys(entityByID).length,
          links: relationLinks.length,
          worldLinksVisible: linkRoot.visible,
          svgRelationLayerVisible: shell.relationLayer.style.display !== "none",
          camera: { theta: cameraState.theta, phi: cameraState.phi, zoom: cameraState.zoom, panX: cameraState.panX, panZ: cameraState.panZ }
        }, relationLayerSummary());
      }
    };
    window.__EFP_VISUAL_DEBUG__ = {
      getSummary: debugSummary,
      getLabelAnchors: getLabelAnchors,
      getLinkLabelAnchors: getLinkLabelAnchors,
      getRelationLayerSummary: relationLayerSummary,
      orbitSmoke: orbitSmoke
    };

    function resize() {
      width = Math.max(760, shell.stage.clientWidth || width);
      height = Math.max(540, shell.stage.clientHeight || height);
      renderer.setSize(width, height, false);
      updateCamera();
      updateRelationLayer();
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
      if (settleFrames > 0) {
        applyRelationshipConstraints(drag.active && drag.mode === "entity" && drag.entity ? drag.entity.item.id : "");
        settleFrames -= 1;
        relationsDirty = true;
      }
      if (relationsDirty) {
        refreshEntityAnchors();
        refreshRelationshipRoutes();
        relationsDirty = false;
      }
      updateCamera();
      updateRelationLayer();
      linkRoot.children.forEach(function (child) {
        if (child.userData && child.userData.isFlowParticle && child.userData.curve) {
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
      if (relationLayerMode === "world_ground" && !drag.active && Date.now() - cameraLastMovedAt > 190 && Date.now() - lastGroundLabelReadabilityAt > 140) {
        updateGroundLabelReadability(false);
        lastGroundLabelReadabilityAt = Date.now();
      }
      renderer.render(scene, camera);
      window.requestAnimationFrame(animate);
    }
    updateCamera();
    linkRoot.visible = true;
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
