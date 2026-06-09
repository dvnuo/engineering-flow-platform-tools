#!/usr/bin/env node
import { spawn } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { tmpdir } from "node:os";

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) continue;
    const key = arg.slice(2);
    const next = argv[i + 1];
    if (!next || next.startsWith("--")) {
      out[key] = "true";
    } else {
      out[key] = next;
      i += 1;
    }
  }
  return out;
}

function fail(code, message, hint) {
  console.log(JSON.stringify({ ok: false, error: { code, message, hint } }, null, 2));
  process.exit(1);
}

const args = parseArgs(process.argv.slice(2));
const url = args.url || "";
const browserPath = args.browser || "";
const screenshot = args.screenshot || "";
const timeoutMs = Math.max(1000, Number(args.timeout || 90) * 1000);
const scenario = String(args.scenario || "overview").toLowerCase();
const scenarioEntity = String(args.entity || "");
const dragX = Number(args["drag-x"] || 0);
const dragZ = Number(args["drag-z"] || 0);
const cameraTheta = Number(args["camera-theta"] || 0);
const cameraPhi = Number(args["camera-phi"] || 0);
const cameraZoom = Number(args["camera-zoom"] || 0);
const orbitSmokeEnabled = args["orbit-smoke"] === "true";
if (!url) fail("browser_url_missing", "--url is required.", "Pass the local HTTP URL for the rendered index.html.");
if (!browserPath) fail("browser_runtime_missing", "--browser is required.", "Pass a Chrome or Chromium executable path.");
if (!screenshot) fail("browser_screenshot_missing", "--screenshot is required.", "Pass the screenshot output path.");
if (!existsSync(browserPath)) fail("browser_runtime_missing", "Chrome or Chromium was not found.", "Install Chrome/Chromium or pass --browser <path>.");

await mkdir(dirname(screenshot), { recursive: true });
const userDataDir = await mkdtemp(join(tmpdir(), "efp-visual-cdp-"));
let chrome;
let browserPort = "";
let browserPathFromDevtools = "";
const deadline = Date.now() + timeoutMs;
const screenshotReserveMs = Math.min(10000, Math.max(5000, Math.floor(timeoutMs * 0.25)));
const consoleErrors = [];
const networkErrors = [];
const remoteRequests = [];
const requests = [];
const requestByID = new Map();

function remaining() {
  return Math.max(1, deadline - Date.now());
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitForFile(path) {
  while (Date.now() < deadline) {
    if (existsSync(path)) return;
    await sleep(100);
  }
  throw new Error(`timed out waiting for ${path}`);
}

async function requestJSON(requestUrl, options = {}) {
  const response = await fetch(requestUrl, options);
  if (!response.ok) throw new Error(`${requestUrl} returned ${response.status}`);
  return response.json();
}

class CDP {
  constructor(wsURL) {
    this.ws = new WebSocket(wsURL);
    this.nextID = 1;
    this.pending = new Map();
    this.events = [];
  }
  async open() {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("CDP websocket timeout")), remaining());
      this.ws.addEventListener("open", () => {
        clearTimeout(timer);
        resolve();
      }, { once: true });
      this.ws.addEventListener("error", () => {
        clearTimeout(timer);
        reject(new Error("CDP websocket failed"));
      }, { once: true });
    });
    this.ws.addEventListener("message", (event) => {
      const msg = JSON.parse(event.data);
      if (msg.id && this.pending.has(msg.id)) {
        const item = this.pending.get(msg.id);
        this.pending.delete(msg.id);
        if (msg.error) item.reject(new Error(msg.error.message || JSON.stringify(msg.error)));
        else item.resolve(msg.result || {});
        return;
      }
      this.events.push(msg);
      if (msg.method === "Runtime.exceptionThrown") {
        consoleErrors.push(String(msg.params?.exceptionDetails?.text || "Runtime exception"));
      }
      if (msg.method === "Runtime.consoleAPICalled" && msg.params?.type === "error") {
        consoleErrors.push((msg.params.args || []).map((arg) => arg.value || arg.description || "").join(" "));
      }
      if (msg.method === "Log.entryAdded" && msg.params?.entry?.level === "error") {
        consoleErrors.push(String(msg.params.entry.text || "Log error"));
      }
      if (msg.method === "Network.requestWillBeSent") {
        const requestURL = String(msg.params?.request?.url || "");
        if (msg.params?.requestId) requestByID.set(msg.params.requestId, requestURL);
        requests.push(requestURL);
        if (requestURL && !requestURL.startsWith(new URL(url).origin)) {
          remoteRequests.push(requestURL);
        }
      }
      if (msg.method === "Network.loadingFailed") {
        const requestURL = requestByID.get(msg.params?.requestId) || "";
        if (requestURL.endsWith("/favicon.ico")) return;
        networkErrors.push(`${requestURL || msg.params?.requestId || "request"}: ${String(msg.params?.errorText || "Network loading failed")}`);
      }
    });
  }
  send(method, params = {}) {
    const id = this.nextID++;
    this.ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`${method} timed out`));
      }, remaining());
      this.pending.set(id, {
        resolve: (value) => {
          clearTimeout(timer);
          resolve(value);
        },
        reject: (err) => {
          clearTimeout(timer);
          reject(err);
        }
      });
    });
  }
  close() {
    try { this.ws.close(); } catch {
      // ignore
    }
  }
}

const expression = `(() => {
  const q = (selector) => document.querySelector(selector);
  const qa = (selector) => Array.from(document.querySelectorAll(selector));
  const rect = (node) => {
    if (!node) return null;
    const r = node.getBoundingClientRect();
    return { x: Math.round(r.x), y: Math.round(r.y), width: Math.round(r.width), height: Math.round(r.height) };
  };
  const isVisible = (node) => {
    if (!node) return false;
    const style = getComputedStyle(node);
    const r = node.getBoundingClientRect();
    return style.visibility !== "hidden" && style.display !== "none" && Number(style.opacity || 1) > 0.01 && r.width > 0 && r.height > 0;
  };
  const labelIconNodes = qa(".visual-isometric-label-icon");
  const entityLabelNodes = qa(".visual-isometric-entity-label");
  const linkLabelNodes = qa(".visual-isometric-link-label");
  const zoneLabelNodes = qa(".visual-isometric-zone-label");
  const relationLayer = q(".visual-isometric-relation-svg[data-relation-layer='true']");
  const svgLinkPathNodes = qa(".visual-isometric-link-path");
  const debugApi = window.__EFP_VISUAL_DEBUG__;
  const debugSummary = debugApi?.getSummary ? (debugApi.getSummary() || {}) : {};
  const relationLayerMode = String(debugSummary.relationLayerMode || q("[data-relation-layer-mode]")?.getAttribute("data-relation-layer-mode") || "");
  const worldGroundMode = relationLayerMode === "world_ground";
  const linkLabelMode = String(debugSummary.linkLabelMode || "html_billboard");
  const groundLinkLabelVisibleCount = Number(debugSummary.groundLinkLabelVisibleCount || 0);
  const groundLinkLabelMeshCount = Number(debugSummary.groundLinkLabelMeshCount || 0);
  const imageLoaded = (node) => !(node instanceof HTMLImageElement) || (node.complete && node.naturalWidth > 0 && node.naturalHeight > 0);
  const imageBroken = (node) => node instanceof HTMLImageElement && (!node.complete || node.naturalWidth === 0 || node.naturalHeight === 0);
  const labelRect = (node) => {
    const r = node.getBoundingClientRect();
    return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height };
  };
  const overlapCountFor = (nodes) => {
    const rects = nodes.filter(isVisible).map(labelRect);
    let count = 0;
    for (let i = 0; i < rects.length; i += 1) {
      for (let j = i + 1; j < rects.length; j += 1) {
        const a = rects[i];
        const b = rects[j];
        const w = Math.min(a.right, b.right) - Math.max(a.left, b.left);
        const h = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (w > 0 && h > 0 && w * h >= 12) count += 1;
      }
    }
    return count;
  };
  const outsideCountFor = (nodes) => nodes.filter(isVisible).filter((node) => {
    const r = labelRect(node);
    return r.left < 0 || r.top < 0 || r.right > window.innerWidth || r.bottom > window.innerHeight;
  }).length;
  const entityLabelOverlapCount = overlapCountFor(entityLabelNodes);
  const linkLabelOverlapCount = overlapCountFor(linkLabelNodes);
  const zoneLabelOverlapCount = overlapCountFor(zoneLabelNodes);
  const totalLabelOverlapCount = entityLabelOverlapCount + linkLabelOverlapCount + zoneLabelOverlapCount;
  const labelsOutsideStageCount = outsideCountFor([...entityLabelNodes, ...linkLabelNodes, ...zoneLabelNodes]);
  const canvas = q("canvas");
  const layer = q(".visual-isometric-label-layer");
  const labelLayoutPass = Number(layer?.dataset?.layoutPass || 0);
  const visualData = window.__EFP_VISUAL_DATA__ || {};
  const linkData = Array.isArray(visualData.links) ? visualData.links : [];
  const roleOf = (link) => {
    const role = String(link?.role || link?.presentation?.role || link?.metadata?.role || "secondary").toLowerCase().replace(/[_\\s]+/g, "-");
    return ["primary", "secondary", "auxiliary"].includes(role) ? role : "secondary";
  };
  const pathGroupOf = (link) => String(link?.pathGroup || link?.path_group || link?.presentation?.pathGroup || link?.presentation?.path_group || link?.metadata?.pathGroup || link?.metadata?.path_group || link?.kind || "relationship").toLowerCase().replace(/[_\\s]+/g, "-");
  const primaryLinkCount = linkData.filter((link) => roleOf(link) === "primary").length;
  const secondaryLinkCount = linkData.filter((link) => roleOf(link) === "secondary").length;
  const auxiliaryLinkCount = linkData.filter((link) => roleOf(link) === "auxiliary").length;
  const domVisibleLinkLabels = linkLabelNodes.filter(isVisible).length;
  const visiblePrimaryLinkLabelCount = linkLabelNodes.filter((node) => isVisible(node) && node.getAttribute("data-link-role") === "primary").length;
  const visibleSecondaryLinkLabelCount = linkLabelNodes.filter((node) => isVisible(node) && node.getAttribute("data-link-role") === "secondary").length;
  const visibleAuxiliaryLinkLabelCount = linkLabelNodes.filter((node) => isVisible(node) && node.getAttribute("data-link-role") === "auxiliary").length;
  const routeGroups = Array.from(new Set(linkData.map(pathGroupOf))).filter(Boolean).sort();
  const primaryPathGroupsVisible = Array.from(new Set(linkLabelNodes.filter((node) => isVisible(node) && node.getAttribute("data-link-role") === "primary").map((node) => node.getAttribute("data-path-group") || ""))).filter(Boolean).sort();
  const hasExplicitRoute = (link) => Array.isArray(link?.route) && link.route.length >= 2;
  const meaningfulLinkLabel = (value) => {
    const text = String(value || "").trim().toLowerCase();
    return !!text && text !== "link" && text !== "relationship" && text !== "depends_on";
  };
  const visibleGenericLinkLabelCount = linkLabelNodes.filter((node) => isVisible(node) && String(node.textContent || "").trim().toLowerCase() === "link").length;
  const explicitLinkLabelCount = linkData.filter((link) => meaningfulLinkLabel(link?.label)).length;
  const inferredLinkLabelCount = linkLabelNodes.filter((node) => {
    if (!isVisible(node) || !meaningfulLinkLabel(node.textContent)) return false;
    const id = String(node.getAttribute("data-link-id") || "");
    return !linkData.some((link) => String(link?.id || "") === id && meaningfulLinkLabel(link?.label));
  }).length;
  const explicitRouteLinkCount = linkData.filter(hasExplicitRoute).length;
  const heuristicRouteLinkCount = Math.max(0, linkData.length - explicitRouteLinkCount);
  const primaryExplicitRouteCount = linkData.filter((link) => roleOf(link) === "primary" && hasExplicitRoute(link)).length;
  const inspector = q(".visual-isometric-inspector");
  const inspectorRawJSONDefault = !!inspector?.querySelector(":scope > pre");
  const isVisibleSvgPath = (node) => {
    if (!node) return false;
    const style = getComputedStyle(node);
    if (style.visibility === "hidden" || style.display === "none" || Number(style.opacity || 1) <= 0.01) return false;
    if (!node.getAttribute("d")) return false;
    try {
      return node.getTotalLength() > 1;
    } catch (err) {
      return false;
    }
  };
  const svgPrimaryLinkPathCount = svgLinkPathNodes.filter((node) => node.getAttribute("data-role") === "primary").length;
  const svgSecondaryLinkPathCount = svgLinkPathNodes.filter((node) => node.getAttribute("data-role") === "secondary").length;
  const svgAuxiliaryLinkPathCount = svgLinkPathNodes.filter((node) => node.getAttribute("data-role") === "auxiliary").length;
  const linkPathsWithMarkerCount = svgLinkPathNodes.filter((node) => !!node.getAttribute("marker-end")).length;
  const linkPathsWithoutMarkerCount = svgLinkPathNodes.filter((node) => !node.getAttribute("marker-end")).length;
  const relationColorPalette = Array.from(new Set(svgLinkPathNodes.map((node) => {
    const style = getComputedStyle(node);
    return (style.stroke && style.stroke !== "none") ? style.stroke : (node.style.getPropertyValue("--relation-stroke") || "");
  }).filter(Boolean))).sort();
  const visibleAuxiliaryOpacityValues = svgLinkPathNodes
    .filter((node) => node.getAttribute("data-role") === "auxiliary" && isVisibleSvgPath(node))
    .map((node) => Number(getComputedStyle(node).opacity || 1))
    .filter((value) => Number.isFinite(value));
  const visibleAuxiliaryOpacityAverage = visibleAuxiliaryOpacityValues.length
    ? Math.round((visibleAuxiliaryOpacityValues.reduce((sum, value) => sum + value, 0) / visibleAuxiliaryOpacityValues.length) * 100) / 100
    : 0;
  const summary = {
    title: document.title || "",
    template: q("[data-visual-template]")?.getAttribute("data-visual-template") || "",
    renderer: q("[data-visual-renderer]")?.getAttribute("data-visual-renderer") || "",
    isometricReady: !!q(".visual-isometric-ready"),
    stage: !!q(".visual-isometric-stage"),
    labelLayer: !!q(".visual-isometric-label-layer"),
    labelLayoutPass,
    entityLabels: qa("[data-entity-id]").length,
    linkLabels: Math.max(linkLabelNodes.length, groundLinkLabelMeshCount),
    zoneLabels: qa("[data-zone-id]").length,
    labelIcons: qa('[data-has-label-icon="true"]').length,
    labelIconsLoaded: labelIconNodes.filter(imageLoaded).length,
    brokenLabelIcons: labelIconNodes.filter(imageBroken).length,
    visibleEntityLabels: entityLabelNodes.filter(isVisible).length,
    visibleLinkLabels: linkLabelMode === "ground_texture_debug" ? Math.max(domVisibleLinkLabels, groundLinkLabelVisibleCount) : domVisibleLinkLabels,
    visibleZoneLabels: zoneLabelNodes.filter(isVisible).length,
    visibleLabelIcons: labelIconNodes.filter((node) => isVisible(node.closest(".visual-isometric-entity-label") || node) && imageLoaded(node)).length,
    primaryLinkCount,
    secondaryLinkCount,
    auxiliaryLinkCount,
    visiblePrimaryLinkLabelCount,
    primaryVisibleLabelCount: visiblePrimaryLinkLabelCount,
    visibleSecondaryLinkLabelCount,
    visibleAuxiliaryLinkLabelCount,
    overviewLinkLabelCount: linkLabelMode === "ground_texture_debug" ? Math.max(domVisibleLinkLabels, groundLinkLabelVisibleCount) : domVisibleLinkLabels,
    linkOpacityBuckets: { strong: primaryLinkCount, medium: secondaryLinkCount, low: auxiliaryLinkCount },
    zoneCountVisible: zoneLabelNodes.filter(isVisible).length,
    primaryPathGroupsVisible,
    routeGroups,
    explicitRouteLinkCount,
    heuristicRouteLinkCount,
    primaryExplicitRouteCount,
    relationColorPaletteSize: relationColorPalette.length,
    relationColorPalette,
    visibleAuxiliaryOpacityAverage,
    inspectorRawJSONDefault,
    svgRelationLayerPresent: !!relationLayer,
    svgLinkPathCount: svgLinkPathNodes.length,
    svgPrimaryLinkPathCount,
    svgSecondaryLinkPathCount,
    svgAuxiliaryLinkPathCount,
    visibleSvgLinkPathCount: svgLinkPathNodes.filter(isVisibleSvgPath).length,
    relationLayerBounds: rect(relationLayer),
    linkPathsWithMarkerCount,
    linkPathsWithoutMarkerCount,
    genericLinkLabelCount: visibleGenericLinkLabelCount,
    inferredLinkLabelCount,
    explicitLinkLabelCount,
    modelBadges: qa('[data-has-model-badge="true"]').length,
    svgBillboards: qa('[data-has-svg-billboard="true"]').length,
    fallbackBadges: qa('[data-icon-id=""], [data-model-id=""]').length,
    controls: qa(".visual-isometric-control").length,
    controlBar: !!q(".visual-isometric-control-bar"),
    canvas: qa("canvas").length,
    approximateLabelOverlapCount: entityLabelOverlapCount,
    entityLabelOverlapCount,
    linkLabelOverlapCount,
    zoneLabelOverlapCount,
    totalLabelOverlapCount,
    labelsOutsideStageCount,
    labelLayerBounds: rect(layer),
    canvasBounds: rect(canvas),
    screenshotSize: { width: Math.round(window.innerWidth || 0), height: Math.round(window.innerHeight || 0) },
    ...debugSummary,
    relationLayerMode,
    ready: !!q("[data-visual-renderer='offline.architecture.isometric.v1']") &&
      !!q(".visual-isometric-ready") &&
      !!q(".visual-isometric-label-layer") &&
      (linkData.length === 0 || worldGroundMode || !!relationLayer) &&
      labelLayoutPass >= 2 &&
      qa("[data-entity-id]").length > 0
  };
  return summary;
})()`;

function scenarioExpression() {
  const payload = {
    scenario,
    entity: scenarioEntity,
    dragX: Number.isFinite(dragX) ? dragX : 0,
    dragZ: Number.isFinite(dragZ) ? dragZ : 0,
    cameraTheta: Number.isFinite(cameraTheta) ? cameraTheta : 0,
    cameraPhi: Number.isFinite(cameraPhi) ? cameraPhi : 0,
    cameraZoom: Number.isFinite(cameraZoom) ? cameraZoom : 0
  };
  return `(() => {
    const input = ${JSON.stringify(payload)};
    const api = window.__EFP_ISOMETRIC_SCENE__;
    if (!api) return { ok: false, reason: "isometric_api_missing" };
    const applyCamera = (camera) => api.setCamera && api.setCamera(camera);
    if (input.scenario === "angle-left") {
      applyCamera({ theta: -0.78, phi: 0.98, zoom: input.cameraZoom || 1.06 });
    } else if (input.scenario === "angle-right") {
      applyCamera({ theta: 1.28, phi: 0.94, zoom: input.cameraZoom || 1.04 });
    } else if (input.scenario === "top") {
      applyCamera({ theta: 0.78, phi: 0.36, zoom: input.cameraZoom || 1.05 });
    } else if (input.scenario === "drag") {
      if (input.cameraTheta || input.cameraPhi || input.cameraZoom) {
        applyCamera({ theta: input.cameraTheta || undefined, phi: input.cameraPhi || undefined, zoom: input.cameraZoom || undefined });
      }
      if (api.dragEntity) api.dragEntity(input.entity, input.dragX || 1.2, input.dragZ || 0.55);
    } else if (input.cameraTheta || input.cameraPhi || input.cameraZoom) {
      applyCamera({ theta: input.cameraTheta || undefined, phi: input.cameraPhi || undefined, zoom: input.cameraZoom || undefined });
    }
    return { ok: true, scenario: input.scenario, stats: api.stats ? api.stats() : {} };
  })()`;
}

try {
  chrome = spawn(browserPath, [
    "--headless=new",
    "--disable-dev-shm-usage",
    "--enable-unsafe-swiftshader",
    "--ignore-gpu-blocklist",
    "--use-angle=swiftshader",
    "--no-sandbox",
    "--hide-scrollbars",
    "--window-size=1440,1000",
    "--remote-debugging-port=0",
    `--user-data-dir=${userDataDir}`,
    "about:blank"
  ], { stdio: ["ignore", "ignore", "pipe"] });
  chrome.stderr.on("data", (chunk) => {
    const text = String(chunk);
    if (/Uncaught|TypeError|ReferenceError|SyntaxError/.test(text)) consoleErrors.push(text.trim());
    if (/net::ERR_|Failed to load resource/.test(text)) networkErrors.push(text.trim());
  });
  await waitForFile(join(userDataDir, "DevToolsActivePort"));
  const active = (await readFile(join(userDataDir, "DevToolsActivePort"), "utf8")).trim().split(/\r?\n/);
  browserPort = active[0];
  browserPathFromDevtools = active[1] || "";
  await requestJSON(`http://127.0.0.1:${browserPort}/json/new?${encodeURIComponent(url)}`, { method: "PUT" }).catch(async () => {
    const list = await requestJSON(`http://127.0.0.1:${browserPort}/json/list`);
    if (!list[0]) throw new Error("Chrome did not expose a page target");
  });
  let targets = await requestJSON(`http://127.0.0.1:${browserPort}/json/list`);
  let page = targets.find((target) => target.type === "page" && target.url === url) || targets.find((target) => target.type === "page");
  if (!page || !page.webSocketDebuggerUrl) throw new Error("Chrome did not expose a page websocket");
  const cdp = new CDP(page.webSocketDebuggerUrl);
  await cdp.open();
  await cdp.send("Page.enable");
  await cdp.send("Runtime.enable");
  await cdp.send("Network.enable");
  await cdp.send("Log.enable").catch(() => ({}));
  await cdp.send("Page.navigate", { url });
  let summary = {};
  const readyDeadline = deadline - screenshotReserveMs;
  while (Date.now() < readyDeadline) {
    await sleep(250);
    const result = await cdp.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }).catch(() => ({}));
    summary = result.result?.value || {};
    if (summary.ready) break;
  }
  await cdp.send("Runtime.evaluate", { expression: scenarioExpression(), returnByValue: true, awaitPromise: true }).catch(() => ({}));
  await sleep(700);
  const finalResult = await cdp.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }).catch(() => ({}));
  summary = finalResult.result?.value || summary;
  if (orbitSmokeEnabled) {
    const orbitResult = await cdp.send("Runtime.evaluate", {
      expression: `(() => {
        const api = window.__EFP_VISUAL_DEBUG__;
        if (!api || !api.orbitSmoke) {
          return { orbitSmokeEnabled: true, orbitMissingEntityLabelsAfterRotate: -1, orbitMissingLinkLabelsAfterRotate: -1, orbitRelationLayerModeStable: false };
        }
        return api.orbitSmoke({ yawDelta: 8 });
      })()`,
      returnByValue: true,
      awaitPromise: true
    }).catch(() => ({}));
    const orbitSummary = orbitResult.result?.value || { orbitSmokeEnabled: true, orbitRelationLayerModeStable: false };
    await sleep(350);
    const afterOrbitResult = await cdp.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }).catch(() => ({}));
    summary = { ...(afterOrbitResult.result?.value || summary), ...orbitSummary };
  }
  const shot = await cdp.send("Page.captureScreenshot", { format: "png", fromSurface: true, captureBeyondViewport: false });
  await writeFile(screenshot, Buffer.from(shot.data || "", "base64"));
  cdp.close();
  console.log(JSON.stringify({
    ok: true,
    data: {
      summary,
      requests,
      remote_requests: remoteRequests,
      console_errors: consoleErrors.filter(Boolean),
      network_errors: networkErrors.filter(Boolean),
      screenshot
    }
  }, null, 2));
} catch (err) {
  fail("browser_page_not_ready", err.message || String(err), "Ensure Chrome/Chromium can run headless and the rendered output is valid.");
} finally {
  if (chrome && !chrome.killed) chrome.kill();
  await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
}
