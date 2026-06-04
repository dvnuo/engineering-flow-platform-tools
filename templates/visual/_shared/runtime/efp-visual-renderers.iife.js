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
      citation_map: "document_wall"
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
    return ["nodes", "edges", "events", "claims", "sources", "links", "items"].reduce(function (sum, key) {
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
    return String((item && (item.label || item.name || item.title || item.summary || item.text || item.id)) || "");
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
    });
    return {
      design: design,
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
    var visibleIDs = {};
    var visibleNodes = [];
    var groupMatches = {};
    var hasGroups = state.groupOrder.length > 0;

    function passesNode(node) {
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
      visibleNodes = visibleNodes.slice(0, maxNodes);
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
      nodes = nodes.slice().sort(compareImportance).slice(0, design.maxInitialNodes);
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

  function createThreeMaterial(THREE, item, effects) {
    var color = hexColor(item.status);
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
      var objects = [];
      var positions = {};
      var sphere = new THREE.IcosahedronGeometry(0.19, 1);
      var box = new THREE.BoxGeometry(0.36, 0.28, 0.36);
      var panel = new THREE.BoxGeometry(0.64, 0.32, 0.08);
      items.forEach(function (item, index) {
        var pos = threePosition(THREE, item, index, items.length, effects, preset);
        positions[item.id] = pos;
        var materialName = safeClass(effects.material);
        var geometry = sphere;
        if (materialName.indexOf("height") >= 0 || materialName.indexOf("city") >= 0 || item.type === "item") {
          geometry = box;
        } else if (materialName.indexOf("glass") >= 0 || item.type === "claim" || item.type === "source") {
          geometry = panel;
        }
        var mesh = new THREE.Mesh(geometry, createThreeMaterial(THREE, item, effects));
        mesh.position.copy(pos);
        if (geometry === box) {
          var lift = item.payload && item.payload.metrics ? Number(item.payload.metrics.risk || item.payload.metrics.impact || item.payload.metrics.score || item.payload.metrics.value) : NaN;
          if (!Number.isFinite(lift)) {
            lift = index % 6;
          }
          mesh.scale.y = 0.75 + Math.min(2.8, Math.max(0, lift > 1 ? lift / 35 : lift * 0.35));
          mesh.position.y += mesh.scale.y * 0.08;
        }
        mesh.userData = { label: item.label, payload: item.payload || item };
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
    if (effects.engine !== "three.v1") {
      return false;
    }
    preset = normalizePreset(preset);
    var scene = safeClass(effects.scene);
    return preset === "constellation" || preset === "graph_3d" || preset === "graph_2_5d" || preset === "orbit_system" || scene.indexOf("galaxy") >= 0 || scene.indexOf("constellation") >= 0 || scene.indexOf("orbit") >= 0 || (profile && profile.key === "space");
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
      if (node.__group || preset === "orbit_system") {
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
      var selectedID = "";
      var hoverID = "";
      var currentModel = { nodes: [], edges: [] };
      var currentFilters = {};
      var pointer = new THREE.Vector2();
      var raycaster = new THREE.Raycaster();
      var dragging = false;
      var moved = false;
      var dragMode = "orbit";
      var draggedMesh = null;
      var lastExpandOrigin = null;
      var lastX = 0;
      var lastY = 0;
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

      function addLabel(node, mesh) {
        var label = el("div", "visual-three-label" + (node.__group ? " visual-three-group-label" : ""), itemLabel(node));
        labelLayer.appendChild(label);
        labels.push({ element: label, mesh: mesh, node: node });
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
        item.line.geometry.setFromPoints([endpoints.from.position, endpoints.to.position]);
        if (item.line.geometry.computeBoundingSphere) {
          item.line.geometry.computeBoundingSphere();
        }
        (item.markers || []).forEach(function (marker) {
          marker.userData.from.copy(endpoints.from.position);
          marker.userData.to.copy(endpoints.to.position);
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

      function dragNode(mesh, dx, dy) {
        if (!mesh || !mesh.userData) {
          return;
        }
        var right = new THREE.Vector3();
        var up = new THREE.Vector3();
        var forward = new THREE.Vector3();
        camera.updateMatrixWorld(true);
        camera.matrixWorld.extractBasis(right, up, forward);
        var scale = orbit.radius * 0.0022;
        var delta = right.multiplyScalar(dx * scale).add(up.multiplyScalar(-dy * scale));
        if (!mesh.userData.targetPosition) {
          mesh.userData.targetPosition = mesh.position.clone();
        }
        mesh.userData.targetPosition.add(delta);
        mesh.position.add(delta);
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
          neighborMesh.position.add(nudge.clone().multiplyScalar(0.35));
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
        var sphere = new THREE.IcosahedronGeometry(0.22, 2);
        var groupSphere = new THREE.SphereGeometry(0.34, 24, 16);
        var panel = new THREE.BoxGeometry(0.46, 0.34, 0.22);
        currentModel.nodes.forEach(function (node) {
          var geometry = node.__group ? groupSphere : preset === "graph_2_5d" ? panel : sphere;
          var material = createThreeMaterial(THREE, node, effects);
          material.transparent = true;
          material.opacity = node.__group ? 0.9 : 0.82;
          var mesh = new THREE.Mesh(geometry, material);
          var targetPosition = (positions[node.id] || new THREE.Vector3(0, 0, 0)).clone();
          mesh.position.copy(startPositionForNode(node, targetPosition, previousPositions));
          var scale = node.__group ? 1 + Math.min(1.7, (node.child_count || 1) / 18) : 0.72 + importanceValue(node, 0.35) * 0.7;
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
          material.opacity = 0;
          nodeRoot.add(mesh);
          objects.push(mesh);
          nodeMap[node.id] = { mesh: mesh, node: node };
          if (node.__group || currentModel.nodes.length <= 28 || importanceValue(node, 0) >= 0.72) {
            addLabel(node, mesh);
          }
        });
        currentModel.edges.forEach(function (edge) {
          var endpoints = endpointMeshes(edge);
          if (!endpoints) {
            return;
          }
          var lineGeo = new THREE.BufferGeometry();
          lineGeo.setFromPoints([endpoints.from.position, endpoints.to.position]);
          var baseOpacity = edge.aggregated ? 0.86 : 0.64;
          var line = new THREE.Line(lineGeo, new THREE.LineBasicMaterial({
            color: hexColor(edge.status),
            transparent: true,
            opacity: 0,
            blending: THREE.AdditiveBlending
          }));
          line.material.depthTest = false;
          line.material.depthWrite = false;
          line.renderOrder = 1;
          line.userData = { baseOpacity: baseOpacity, targetOpacity: baseOpacity };
          edgeRoot.add(line);
          var markers = [];
          var markerMaterial = new THREE.MeshBasicMaterial({
            color: hexColor(edge.status),
            transparent: true,
            opacity: 0,
            blending: THREE.AdditiveBlending
          });
          markerMaterial.depthTest = false;
          markerMaterial.depthWrite = false;
          var markerGeometry = new THREE.SphereGeometry(edge.aggregated ? 0.055 : 0.04, 12, 8);
          var marker = new THREE.Mesh(markerGeometry, markerMaterial);
          marker.position.copy(endpoints.from.position);
          marker.renderOrder = 2;
          marker.userData = {
            from: endpoints.from.position.clone(),
            to: endpoints.to.position.clone(),
            phase: (edgeItems.length % 17) / 17,
            speed: 0.18 + (edgeItems.length % 5) * 0.025,
            baseOpacity: edge.aggregated ? 0.86 : 0.64,
            targetOpacity: edge.aggregated ? 0.86 : 0.64,
            baseScale: edge.aggregated ? 1.25 : 1,
            targetScale: edge.aggregated ? 1.25 : 1
          };
          edgeRoot.add(marker);
          markers.push(marker);
          addEdgeLabel(edge);
          edgeItems.push({ line: line, edge: edge, markers: markers });
        });
        buildParticles(currentModel.nodes.length + currentModel.edges.length);
        applyFocus(selectedID && nodeMap[selectedID] ? selectedID : "");
      }

      function raycast(event) {
        if (!objects.length) {
          return [];
        }
        var rect = renderer.domElement.getBoundingClientRect();
        pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
        pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
        raycaster.setFromCamera(pointer, camera);
        return raycaster.intersectObjects(objects, false);
      }

      function selectNode(node) {
        if (!node) {
          return;
        }
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
        dragging = true;
        moved = false;
        draggedMesh = null;
        dragMode = event.shiftKey || event.button === 2 ? "pan" : "orbit";
        if (!event.shiftKey && event.button !== 2) {
          var hits = raycast(event);
          if (hits.length && hits[0].object.userData) {
            draggedMesh = hits[0].object;
            dragMode = "node";
            hoverID = draggedMesh.userData.id || "";
            applyFocus(hoverID);
            renderer.domElement.style.cursor = "grabbing";
          }
        }
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
          moved = moved || Math.abs(dx) + Math.abs(dy) > 4;
          if (dragMode === "node" && draggedMesh) {
            dragNode(draggedMesh, dx, dy);
          } else if (dragMode === "pan") {
            orbit.target.x -= dx * orbit.radius * 0.0018;
            orbit.target.y += dy * orbit.radius * 0.0018;
          } else {
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
        var releasedMesh = draggedMesh;
        dragging = false;
        draggedMesh = null;
        if (renderer.domElement.releasePointerCapture) {
          renderer.domElement.releasePointerCapture(event.pointerId);
        }
        if (!moved && releasedMesh && releasedMesh.userData) {
          selectNode(releasedMesh.userData.node);
        } else if (moved && releasedMesh && releasedMesh.userData) {
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
        orbit.radius *= event.deltaY > 0 ? 1.08 : 0.92;
        updateCamera();
        event.preventDefault();
        event.stopPropagation();
      }, { passive: false });
      renderer.domElement.addEventListener("dblclick", function (event) {
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
          var important = item.node.__group || item.node.id === selectedID || item.node.id === hoverID || importanceValue(item.node, 0) >= 0.72 || labels.length <= 28;
          item.element.hidden = !visible || !important;
          if (!item.element.hidden) {
            item.element.style.left = ((pos.x * 0.5 + 0.5) * width).toFixed(1) + "px";
            item.element.style.top = ((-pos.y * 0.5 + 0.5) * height).toFixed(1) + "px";
            item.element.toggleAttribute("data-selected", item.node.id === selectedID);
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
          (item.markers || []).forEach(function (marker) {
            marker.material.opacity = easeValue(marker.material.opacity, marker.userData.targetOpacity, 0.12);
            var p = (t * marker.userData.speed + marker.userData.phase) % 1;
            p = 0.18 + p * 0.64;
            marker.position.copy(marker.userData.from).lerp(marker.userData.to, p);
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
        if (!dragging && !selectedID) {
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
        if (edge.label && (currentModel.edges.length <= 80 || importanceValue(edge, 0) >= 0.65 || edge.aggregated)) {
          var edgeLabel = svg("text", { class: "visual-edge-label", x: (from.x + to.x) / 2, y: (from.y + to.y) / 2 - 5 });
          edgeLabel.textContent = runtime.safeText(edge.label);
          canvas.appendChild(edgeLabel);
        }
      });

      currentModel.nodes.forEach(function (node, index) {
        var pos = positions[node.id] || { x: width / 2, y: height / 2 };
        var className = "visual-node" + (node.__group ? " visual-group-node" : "");
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
          sign.textContent = (state.collapsed[node.id] ? "+" : "-") + " " + runtime.safeText(node.child_count || 0);
          group.appendChild(sign);
        }
        if (currentModel.nodes.length <= 60 || node.__group || importanceValue(node, 0) >= 0.7) {
          var label = svg("text", { x: pos.x, y: pos.y + (node.__group ? 52 : 46), "text-anchor": "middle" });
          label.textContent = runtime.safeText(itemLabel(node));
          group.appendChild(label);
        }
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
    var events = Array.isArray(data.events) ? data.events : [];
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var exportBtn = document.createElement("button");
    exportBtn.textContent = "Export";
    shell.toolbar.appendChild(exportBtn);
    var lane = el("div", "timeline-lane visual-timeline-3d");
    lane.appendChild(el("div", "timeline-track"));
    events.forEach(function (event, index) {
      var card = el("article", "visual-card timeline-event");
      card.style.setProperty("--event-z", Math.round(((index % 6) / 5) * 64) + "px");
      card.style.setProperty("--event-delay", (index * 0.05).toFixed(2) + "s");
      card.appendChild(el("span", "timeline-dot"));
      card.appendChild(el("div", "visual-card-title", event.label || event.summary || event.id));
      var status = runtime.formatStatus(event.status || event.kind);
      card.appendChild(el("span", status.className, status.label));
      card.appendChild(el("div", "visual-card-meta", [event.time, event.kind, event.summary].filter(Boolean).join(" · ")));
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
    var claims = Array.isArray(data.claims) ? data.claims : [];
    var sources = Array.isArray(data.sources) ? data.sources : [];
    var links = Array.isArray(data.links) ? data.links : [];
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var width = Math.max(900, shell.stage.clientWidth || 900);
    var height = Math.max(620, shell.stage.clientHeight || 620);
    var canvas = svg("svg", { class: "visual-svg", viewBox: "0 0 " + width + " " + height });
    shell.stage.appendChild(canvas);
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
        canvas.appendChild(svg("path", { class: "visual-edge visual-evidence-beam", d: edgePath(a, b, preset, links.indexOf(link)) }));
      }
    });
    sources.forEach(function (source) {
      var pos = sourcePos[source.id];
      var group = svg("g", { class: "visual-node" });
      group.appendChild(svg("rect", { x: pos.x - 58, y: pos.y - 26, width: 116, height: 52, rx: 8, fill: "#202832" }));
      var label = svg("text", { x: pos.x, y: pos.y + 4, "text-anchor": "middle" });
      label.textContent = runtime.safeText(source.title || source.id);
      group.appendChild(label);
      group.addEventListener("click", function () {
        shell.inspector.show(source.title || source.id, source);
      });
      canvas.appendChild(group);
    });
    claims.forEach(function (claim) {
      var pos = claimPos[claim.id];
      var group = svg("g", { class: "visual-node" });
      group.appendChild(svg("rect", { x: pos.x - 84, y: pos.y - 34, width: 168, height: 68, rx: 8, fill: nodeColor(claim.status) }));
      var label = svg("text", { x: pos.x, y: pos.y - 2, "text-anchor": "middle" });
      label.textContent = runtime.safeText(claim.text || claim.id).slice(0, 34);
      var conf = svg("text", { x: pos.x, y: pos.y + 17, "text-anchor": "middle" });
      conf.textContent = "confidence " + runtime.safeText(claim.confidence);
      group.appendChild(label);
      group.appendChild(conf);
      group.addEventListener("click", function () {
        shell.inspector.show(claim.id, claim);
      });
      canvas.appendChild(group);
    });
  }

  function renderMatrix(ctx) {
    var data = ctx.data || {};
    var items = Array.isArray(data.items) ? data.items : [];
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    createThreeScene(shell.stage, manifest, data, preset, profile, shell.inspector);
    var board = el("div", "matrix-stage visual-matrix-3d");
    board.appendChild(el("div", "matrix-axis-y", "Impact"));
    board.appendChild(el("div", "matrix-axis-x", "Confidence"));
    items.forEach(function (item, index) {
      var x = typeof item.x === "number" ? item.x : 0.5;
      var y = typeof item.y === "number" ? item.y : 0.5;
      var z = item.metrics && Number.isFinite(Number(item.metrics.z || item.metrics.impact || item.metrics.risk)) ? Number(item.metrics.z || item.metrics.impact || item.metrics.risk) : index % 7;
      var card = el("article", "visual-card matrix-item");
      card.style.left = Math.max(8, Math.min(92, x * 100)) + "%";
      card.style.top = Math.max(8, Math.min(92, (1 - y) * 100)) + "%";
      var zDepth = Math.max(0, Math.min(1, z > 1 ? z / 100 : z / 7));
      card.style.setProperty("--item-z-offset", Math.round(zDepth * 88) + "px");
      card.style.setProperty("--item-shadow-offset", Math.round(zDepth * 22) + "px");
      card.appendChild(el("div", "visual-card-title", item.label || item.id));
      var status = runtime.formatStatus(item.status || item.kind);
      card.appendChild(el("span", status.className, status.label));
      card.appendChild(el("div", "visual-card-meta", item.kind || ""));
      card.addEventListener("click", function () {
        shell.inspector.show(item.label || item.id, item);
      });
      board.appendChild(card);
    });
    shell.stage.appendChild(board);
  }

  runtime.registerRenderer("offline.graph.v1", { render: renderGraph });
  runtime.registerRenderer("offline.timeline.v1", { render: renderTimeline });
  runtime.registerRenderer("offline.evidence.v1", { render: renderEvidence });
  runtime.registerRenderer("offline.matrix.v1", { render: renderMatrix });
}());
