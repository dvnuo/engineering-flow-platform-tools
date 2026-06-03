(function () {
  "use strict";

  function text(value) {
    if (value === null || value === undefined) {
      return "";
    }
    return String(value);
  }

  function element(tag, className, value) {
    var el = document.createElement(tag);
    if (className) {
      el.className = className;
    }
    if (value !== undefined) {
      el.textContent = text(value);
    }
    return el;
  }

  function showFatal(message) {
    var root = document.getElementById("visual-root") || document.body;
    root.textContent = "";
    var box = element("div", "fatal");
    var title = element("h1", "", "Visual artifact could not be rendered");
    var detail = element("p", "", message);
    box.appendChild(title);
    box.appendChild(detail);
    root.appendChild(box);
  }

  function formatStatus(status) {
    var value = text(status || "info").toLowerCase().replace(/\s+/g, "-");
    var label = value || "info";
    return { label: label, className: "visual-badge status-" + label };
  }

  function getNodeById(data, id) {
    var nodes = Array.isArray(data && data.nodes) ? data.nodes : [];
    for (var i = 0; i < nodes.length; i += 1) {
      if (nodes[i] && nodes[i].id === id) {
        return nodes[i];
      }
    }
    return null;
  }

  function createInspector(container) {
    container.textContent = "";
    var title = element("h2", "", "Inspector");
    var empty = element("div", "visual-empty", "Select an item");
    container.appendChild(title);
    container.appendChild(empty);
    return {
      show: function (label, payload) {
        container.textContent = "";
        container.appendChild(element("h2", "", label || "Inspector"));
        var pre = element("pre", "");
        try {
          pre.textContent = JSON.stringify(payload || {}, null, 2);
        } catch (err) {
          pre.textContent = text(payload);
        }
        container.appendChild(pre);
      }
    };
  }

  function exportJSON(data, filename) {
    var blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    var url = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url;
    a.download = filename || "visual-data.json";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  function boot() {
    var runtime = window.EFPVisualRuntime;
    var root = document.getElementById("visual-root");
    if (!root) {
      showFatal("Missing div#visual-root.");
      return;
    }
    var manifest = window.__EFP_VISUAL_MANIFEST__;
    var data = window.__EFP_VISUAL_DATA__;
    if (!manifest) {
      showFatal("Missing embedded manifest.js.");
      return;
    }
    if (!data) {
      showFatal("Missing embedded data.js.");
      return;
    }
    var contract = manifest.renderer && manifest.renderer.contract;
    var renderer = runtime.renderers[contract];
    if (!renderer || typeof renderer.render !== "function") {
      showFatal("No renderer registered for " + text(contract) + ".");
      return;
    }
    try {
      renderer.render({ container: root, data: data, manifest: manifest, runtime: runtime });
    } catch (err) {
      showFatal(err && err.message ? err.message : text(err));
    }
  }

  window.EFPVisualRuntime = {
    renderers: {},
    registerRenderer: function (contract, renderer) {
      if (contract && renderer) {
        this.renderers[contract] = renderer;
      }
    },
    boot: boot,
    showFatal: showFatal,
    safeText: text,
    formatStatus: formatStatus,
    getNodeById: getNodeById,
    createInspector: createInspector,
    exportJSON: exportJSON,
    element: element
  };
}());
