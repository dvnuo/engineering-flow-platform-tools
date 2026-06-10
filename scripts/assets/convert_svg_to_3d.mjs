#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import { spawn } from "node:child_process";

const root = process.cwd();
const catalogPath = path.join(root, "scripts/assets/logo_catalog.json");
const generatedManifestPath = path.join(root, "templates/visual/_shared/assets/manifests/generated-models.json");
const registryGeneratedPath = path.join(root, "templates/visual/_shared/assets/manifests/asset-registry.generated.json");
const assetRegistryPath = path.join(root, "templates/visual/_shared/asset-registry.json");

function parseArgs(argv) {
  const out = { only: "", source: "", dryRun: false, force: false, writeRegistry: false, all: false, format: "glb", skipComplex: false, maxPaths: 64 };
  for (let i = 2; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--dry-run") out.dryRun = true;
    else if (arg === "--force") out.force = true;
    else if (arg === "--write-registry") out.writeRegistry = true;
    else if (arg === "--all") out.all = true;
    else if (arg === "--skip-complex") out.skipComplex = true;
    else if (arg === "--only") out.only = argv[++i] || "";
    else if (arg.startsWith("--only=")) out.only = arg.slice("--only=".length);
    else if (arg === "--source") out.source = argv[++i] || "";
    else if (arg.startsWith("--source=")) out.source = arg.slice("--source=".length);
    else if (arg === "--format") out.format = argv[++i] || "";
    else if (arg.startsWith("--format=")) out.format = arg.slice("--format=".length);
    else if (arg === "--max-paths") out.maxPaths = Number(argv[++i] || 64);
    else if (arg.startsWith("--max-paths=")) out.maxPaths = Number(arg.slice("--max-paths=".length));
    else if (arg === "--help" || arg === "-h") usage(0);
    else usage(2, `unknown argument: ${arg}`);
  }
  if (out.format !== "glb" && out.format !== "gltf") {
    usage(2, "--format must be glb or gltf");
  }
  return out;
}

function usage(code, message = "") {
  if (message) console.error(message);
  console.error("usage: node scripts/assets/convert_svg_to_3d.mjs [--all] [--only id,id] [--source simple-icons] [--format glb|gltf] [--skip-complex] [--max-paths 64] [--dry-run] [--force] [--write-registry]");
  process.exit(code);
}

async function readJSON(file, fallback = null) {
  try {
    return JSON.parse(await fs.readFile(file, "utf8"));
  } catch (err) {
    if (fallback !== null) return fallback;
    throw err;
  }
}

async function writeJSON(file, value) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, `${JSON.stringify(value, null, 2)}\n`);
}

function selectLogos(catalog, opts) {
  const only = new Set(String(opts.only || "").split(",").map((item) => item.trim()).filter(Boolean));
  return (catalog.logos || []).filter((logo) => {
    if (!logo.target || !logo.model) return false;
    if (only.size && !only.has(logo.id)) return false;
    if (opts.source && logo.source !== opts.source) return false;
    return true;
  });
}

function colorFromSVG(svg, fallback) {
  const fill = String(svg || "").match(/\bfill=["'](#[0-9a-fA-F]{3,8})["']/);
  if (fill) return normalizeHex(fill[1]);
  const color = String(svg || "").match(/\bcolor:\s*(#[0-9a-fA-F]{3,8})/);
  if (color) return normalizeHex(color[1]);
  return normalizeHex(fallback || "#64748b");
}

function normalizeHex(value) {
  let text = String(value || "").trim();
  if (!text.startsWith("#")) text = `#${text}`;
  if (/^#[0-9a-fA-F]{3}$/.test(text)) {
    text = `#${text[1]}${text[1]}${text[2]}${text[2]}${text[3]}${text[3]}`;
  }
  return /^#[0-9a-fA-F]{6}$/.test(text) ? text.toLowerCase() : "#64748b";
}

function hexToRGB(hex) {
  const value = normalizeHex(hex).slice(1);
  return [
    parseInt(value.slice(0, 2), 16) / 255,
    parseInt(value.slice(2, 4), 16) / 255,
    parseInt(value.slice(4, 6), 16) / 255
  ];
}

function pad4(buffer, fill = 0x20) {
  const pad = (4 - (buffer.length % 4)) % 4;
  if (!pad) return buffer;
  return Buffer.concat([buffer, Buffer.alloc(pad, fill)]);
}

function makeGLB({ id, label, color, sourceSVG }) {
  const positions = new Float32Array([
    -0.55, -0.08, -0.35,
    0.55, -0.08, -0.35,
    0.55, 0.08, -0.35,
    -0.55, 0.08, -0.35,
    -0.55, -0.08, 0.35,
    0.55, -0.08, 0.35,
    0.55, 0.08, 0.35,
    -0.55, 0.08, 0.35
  ]);
  const indices = new Uint16Array([
    0, 1, 2, 0, 2, 3,
    4, 6, 5, 4, 7, 6,
    0, 4, 5, 0, 5, 1,
    1, 5, 6, 1, 6, 2,
    2, 6, 7, 2, 7, 3,
    3, 7, 4, 3, 4, 0
  ]);
  const indexBytes = Buffer.from(indices.buffer);
  const positionBytes = Buffer.from(positions.buffer);
  const bin = pad4(Buffer.concat([indexBytes, positionBytes]), 0x00);
  const positionOffset = indexBytes.length;
  const [r, g, b] = hexToRGB(color);
  const json = {
    asset: {
      version: "2.0",
      generator: "engineering-flow-platform-tools scripts/assets/convert_svg_to_3d.mjs"
    },
    extras: {
      id,
      label,
      source_icon: sourceSVG,
      generation_mode: "local_fallback_badge",
      warning: "This GLB is a semantic 3D badge, not a full SVG path extrusion."
    },
    scenes: [{ nodes: [0] }],
    scene: 0,
    nodes: [{ mesh: 0, name: label || id }],
    meshes: [{
      name: `${id}.badge`,
      primitives: [{
        attributes: { POSITION: 1 },
        indices: 0,
        material: 0
      }]
    }],
    materials: [{
      name: `${id}.material`,
      pbrMetallicRoughness: {
        baseColorFactor: [r, g, b, 1],
        metallicFactor: 0.08,
        roughnessFactor: 0.52
      }
    }],
    buffers: [{ byteLength: bin.length }],
    bufferViews: [
      { buffer: 0, byteOffset: 0, byteLength: indexBytes.length, target: 34963 },
      { buffer: 0, byteOffset: positionOffset, byteLength: positionBytes.length, target: 34962 }
    ],
    accessors: [
      { bufferView: 0, componentType: 5123, count: indices.length, type: "SCALAR" },
      {
        bufferView: 1,
        componentType: 5126,
        count: positions.length / 3,
        type: "VEC3",
        min: [-0.55, -0.08, -0.35],
        max: [0.55, 0.08, 0.35]
      }
    ]
  };
  const jsonChunk = pad4(Buffer.from(JSON.stringify(json), "utf8"), 0x20);
  const totalLength = 12 + 8 + jsonChunk.length + 8 + bin.length;
  const header = Buffer.alloc(12);
  header.write("glTF", 0);
  header.writeUInt32LE(2, 4);
  header.writeUInt32LE(totalLength, 8);
  const jsonHeader = Buffer.alloc(8);
  jsonHeader.writeUInt32LE(jsonChunk.length, 0);
  jsonHeader.write("JSON", 4);
  const binHeader = Buffer.alloc(8);
  binHeader.writeUInt32LE(bin.length, 0);
  binHeader.write("BIN\0", 4);
  return Buffer.concat([header, jsonHeader, jsonChunk, binHeader, bin]);
}

function runAdapter(svgPath, outPath) {
  return new Promise((resolve) => {
    if (!process.env.VECTO3D_DIR && !process.env.VECTO3D_COMMAND) {
      resolve({ ok: false, reason: "vecto3d not configured" });
      return;
    }
    const child = spawn(process.execPath, ["scripts/assets/vecto3d_adapter.mjs", "--svg", svgPath, "--out", outPath], { cwd: root, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.on("close", (code) => {
      resolve({ ok: code === 0, code, stdout: stdout.trim(), stderr: stderr.trim(), reason: code === 0 ? "" : "vecto3d adapter failed" });
    });
  });
}

async function convertOne(logo, opts) {
  const svgPath = path.join(root, logo.target);
  const outPath = path.join(root, logo.model);
  const relModelPath = logo.model.replace(/^templates\/visual\/_shared\//, "");
  const relIconPath = logo.target.replace(/^templates\/visual\/_shared\//, "");
  if (!opts.force) {
    try {
      await fs.access(outPath);
      return { id: logo.id, skipped: true, reason: "already exists", path: relModelPath };
    } catch {
    }
  }
  const svg = await fs.readFile(svgPath, "utf8");
  const pathCount = (svg.match(/<path\b/gi) || []).length;
  const unsupported = /<(linearGradient|radialGradient|mask|filter|clipPath|image)\b/i.test(svg);
  if (opts.skipComplex && (pathCount > opts.maxPaths || unsupported)) {
    return {
      id: logo.id,
      skipped: true,
      reason: "asset_svg_too_complex_for_3d",
      path_count: pathCount,
      unsupported_features: unsupported
    };
  }
  if (opts.dryRun) {
    return { id: logo.id, dry_run: true, path: relModelPath };
  }
  await fs.mkdir(path.dirname(outPath), { recursive: true });
  const adapter = await runAdapter(svgPath, outPath);
  let mode = "vecto3d";
  if (!adapter.ok) {
    mode = "local_fallback_badge";
    await fs.writeFile(outPath, makeGLB({ id: logo.id, label: logo.label, color: colorFromSVG(svg, logo.color), sourceSVG: relIconPath }));
  }
  const stat = await fs.stat(outPath);
  return {
    id: logo.id,
    label: logo.label,
    path: relModelPath,
    source_icon: relIconPath,
    color: colorFromSVG(svg, logo.color),
    generated_by: mode,
    byte_length: stat.size,
    generated_at: new Date().toISOString(),
    warning: mode === "local_fallback_badge" ? "Fallback badge geometry; not full SVG path extrusion." : "",
    adapter_reason: adapter.ok ? "" : adapter.reason || adapter.stderr || adapter.stdout || ""
  };
}

function mergeModelManifest(current, entries) {
  const byID = new Map((current.models || []).map((entry) => [entry.id, entry]));
  for (const entry of entries) {
    if (!entry.skipped && !entry.dry_run) byID.set(entry.id, entry);
  }
  return {
    schema: "efp.visual.generated_models.v1",
    generated_at: new Date().toISOString(),
    models: Array.from(byID.values()).sort((a, b) => a.id.localeCompare(b.id))
  };
}

function generatedRegistry(models) {
  const out = {};
  for (const model of models) {
    out[`${model.id}.logo3d`] = {
      path: model.path,
      kind: "generated-logo-badge",
      official: false,
      attribution_id: model.source_icon && model.source_icon.includes("simple-icons/") ? "simple-icons" : "efp-generic",
      source_icon: model.source_icon,
      generated_by: model.generated_by,
      byte_length: model.byte_length
    };
  }
  return out;
}

async function mergeIntoAssetRegistry(models) {
  const registry = await readJSON(assetRegistryPath);
  registry.models = registry.models || {};
  const generated = generatedRegistry(models);
  for (const [id, entry] of Object.entries(generated)) {
    registry.models[id] = entry;
  }
  registry.attributions = registry.attributions || [];
  if (!registry.attributions.some((entry) => entry.id === "simple-icons")) {
    registry.attributions.push({
      id: "simple-icons",
      name: "Simple Icons",
      license: "CC0-1.0 for icon artwork; trademarks remain with brand owners.",
      source: "github.com/simple-icons/simple-icons"
    });
  }
  registry.attributions.sort((a, b) => String(a.id).localeCompare(String(b.id)));
  await writeJSON(assetRegistryPath, registry);
  await writeJSON(registryGeneratedPath, { schema: "efp.visual.asset_registry.generated.v1", generated_at: new Date().toISOString(), models: generated });
}

async function main() {
  const opts = parseArgs(process.argv);
  const catalog = await readJSON(catalogPath);
  const logos = selectLogos(catalog, opts);
  const converted = [];
  const failed = [];
  for (const logo of logos) {
    try {
      converted.push(await convertOne(logo, opts));
    } catch (err) {
      failed.push({ id: logo.id, error: err.message });
    }
  }
  const current = await readJSON(generatedManifestPath, { schema: "efp.visual.generated_models.v1", models: [] });
  const manifest = mergeModelManifest(current, converted);
  if (!opts.dryRun) {
    await writeJSON(generatedManifestPath, manifest);
    await writeJSON(registryGeneratedPath, {
      schema: "efp.visual.asset_registry.generated.v1",
      generated_at: new Date().toISOString(),
      models: generatedRegistry(manifest.models)
    });
    if (opts.writeRegistry) {
      await mergeIntoAssetRegistry(manifest.models);
    }
  }
  const summary = { ok: failed.length === 0, converted, failed };
  console.log(JSON.stringify(summary, null, 2));
  if (failed.length) process.exitCode = 1;
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
