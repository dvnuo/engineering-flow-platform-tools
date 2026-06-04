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
    var nodes = Array.isArray(data.nodes) ? data.nodes : [];
    var edges = Array.isArray(data.edges) ? data.edges : [];
    var events = Array.isArray(data.events) ? data.events : [];
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    var profile = decorateStage(shell.stage, manifest, data, preset);
    var search = document.createElement("input");
    search.type = "search";
    search.placeholder = "Search";
    var statusFilter = selectControl("All statuses", uniqueValues(nodes, "status"));
    var kindFilter = selectControl("All kinds", uniqueValues(nodes, "kind"));
    var replay = document.createElement("button");
    replay.textContent = "Replay";
    var exportBtn = document.createElement("button");
    exportBtn.textContent = "Export";
    shell.toolbar.appendChild(search);
    shell.toolbar.appendChild(statusFilter);
    shell.toolbar.appendChild(kindFilter);
    shell.toolbar.appendChild(replay);
    shell.toolbar.appendChild(exportBtn);

    var width = Math.max(900, shell.stage.clientWidth || 900);
    var height = Math.max(620, shell.stage.clientHeight || 620);
    var canvas = svg("svg", { class: "visual-svg", viewBox: "0 0 " + width + " " + height, role: "img" });
    shell.stage.appendChild(canvas);
    var positions = layoutNodes(nodes, preset, width, height);
    var nodeElements = {};
    var edgeElements = [];
    if (preset === "orbit_system" || preset === "ripple" || preset === "radar_sphere") {
      [90, 170, 250].forEach(function (radius) {
        canvas.appendChild(svg("circle", { class: "visual-orbit", cx: width / 2, cy: height / 2, r: radius }));
      });
    }

    edges.forEach(function (edge, index) {
      var from = positions[edge.from];
      var to = positions[edge.to];
      if (!from || !to) {
        return;
      }
      var edgeClass = "visual-edge";
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
      if (edge.label) {
        var label = svg("text", { class: "visual-edge-label", x: (from.x + to.x) / 2, y: (from.y + to.y) / 2 - 5 });
        label.textContent = runtime.safeText(edge.label);
        canvas.appendChild(label);
      }
    });

    nodes.forEach(function (node, index) {
      var pos = positions[node.id] || { x: width / 2, y: height / 2 };
      var group = svg("g", { class: "visual-node", tabindex: "0" });
      var depth = nodeDepth(node, index, preset);
      var zLift = Math.round(depth * 34);
      group.setAttribute("style", "--node-shadow-x:" + Math.round(6 + depth * 10) + "px; --node-shadow-y:" + Math.round(8 + depth * 12) + "px");
      if (profile.grid || profile.city || preset === "graph_3d" || preset === "graph_2_5d" || preset === "sankey_3d") {
        canvas.appendChild(svg("ellipse", { class: "visual-node-shadow", cx: pos.x + zLift * 0.55, cy: pos.y + 38 + zLift * 0.28, rx: 30 + depth * 12, ry: 8 + depth * 4 }));
        canvas.appendChild(svg("line", { class: "visual-depth-line", x1: pos.x, y1: pos.y + 26, x2: pos.x + zLift * 0.55, y2: pos.y + 38 + zLift * 0.28 }));
      }
      if (zLift) {
        group.setAttribute("transform", "translate(" + (-zLift * 0.24).toFixed(1) + " " + (-zLift).toFixed(1) + ")");
      }
      var shape;
      if (preset === "city_map") {
        var heightValue = 44 + ((node.metrics && Number(node.metrics.risk)) || index % 5) * 10;
        shape = svg("rect", { x: pos.x - 26, y: pos.y - heightValue, width: 52, height: heightValue, rx: 6, fill: nodeColor(node.status) });
      } else if (preset === "layered_stack" || preset === "state_machine" || preset === "control_room" || preset === "diff_split_view") {
        shape = svg("rect", { x: pos.x - 44, y: pos.y - 25, width: 88, height: 50, rx: 8, fill: nodeColor(node.status) });
      } else {
        shape = svg("circle", { cx: pos.x, cy: pos.y, r: 28, fill: nodeColor(node.status) });
      }
      var label = svg("text", { x: pos.x, y: pos.y + 46, "text-anchor": "middle" });
      label.textContent = runtime.safeText(node.label || node.id);
      group.appendChild(shape);
      group.appendChild(label);
      group.addEventListener("click", function () {
        focusNode(node.id);
        shell.inspector.show(node.label || node.id, node);
      });
      canvas.appendChild(group);
      nodeElements[node.id] = { element: group, node: node };
    });

    function matches(node) {
      var q = search.value.toLowerCase();
      var label = String(node.label || node.id || "").toLowerCase();
      return (!q || label.indexOf(q) >= 0) && (!statusFilter.value || node.status === statusFilter.value) && (!kindFilter.value || node.kind === kindFilter.value);
    }

    function applyFilters() {
      nodes.forEach(function (node) {
        var item = nodeElements[node.id];
        if (item) {
          item.element.classList.toggle("visual-hidden", !matches(node));
        }
      });
    }

    function focusNode(id) {
      Object.keys(nodeElements).forEach(function (key) {
        nodeElements[key].element.classList.toggle("visual-focused", key === id);
      });
      edgeElements.forEach(function (item) {
        var active = item.edge.from === id || item.edge.to === id;
        item.element.classList.toggle("visual-focused", active);
      });
    }

    search.addEventListener("input", applyFilters);
    statusFilter.addEventListener("change", applyFilters);
    kindFilter.addEventListener("change", applyFilters);
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
          focusNode(event.node_id);
        }
        shell.inspector.show(event && (event.label || event.summary || event.id) || "Event", event);
        index += 1;
        if (index >= events.length) {
          window.clearInterval(timer);
        }
      }, 650);
    });
    applyFilters();
  }

  function renderTimeline(ctx) {
    var data = ctx.data || {};
    var events = Array.isArray(data.events) ? data.events : [];
    var manifest = ctx.manifest || {};
    var shell = appShell(ctx.container, manifest);
    var preset = normalizePreset(manifest.layout && manifest.layout.preset);
    decorateStage(shell.stage, manifest, data, preset);
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
    decorateStage(shell.stage, manifest, data, preset);
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
    decorateStage(shell.stage, manifest, data, preset);
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
