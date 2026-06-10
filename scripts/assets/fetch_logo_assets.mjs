#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const catalogPath = path.join(root, "scripts/assets/logo_catalog.json");
const sourcesPath = path.join(root, "templates/visual/_shared/assets/attributions/LOGO_SOURCES.json");
const manifestPath = path.join(root, "templates/visual/_shared/assets/manifests/logo-catalog.json");

function parseArgs(argv) {
  const out = { only: "", source: "", dryRun: false, force: false };
  for (let i = 2; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--dry-run") out.dryRun = true;
    else if (arg === "--force") out.force = true;
    else if (arg === "--only") out.only = argv[++i] || "";
    else if (arg.startsWith("--only=")) out.only = arg.slice("--only=".length);
    else if (arg === "--source") out.source = argv[++i] || "";
    else if (arg.startsWith("--source=")) out.source = arg.slice("--source=".length);
    else if (arg === "--help" || arg === "-h") usage(0);
    else usage(2, `unknown argument: ${arg}`);
  }
  return out;
}

function usage(code, message = "") {
  if (message) console.error(message);
  console.error("usage: node scripts/assets/fetch_logo_assets.mjs [--only id,id] [--source simple-icons] [--dry-run] [--force]");
  process.exit(code);
}

async function readJSON(file) {
  return JSON.parse(await fs.readFile(file, "utf8"));
}

async function writeJSON(file, value) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, `${JSON.stringify(value, null, 2)}\n`);
}

function selectLogos(catalog, opts) {
  const only = new Set(String(opts.only || "").split(",").map((item) => item.trim()).filter(Boolean));
  return (catalog.logos || []).filter((logo) => {
    if (only.size && !only.has(logo.id)) return false;
    if (opts.source && logo.source !== opts.source) return false;
    return Boolean(logo.target);
  });
}

function remoteURL(logo) {
  if (logo.url) return logo.url;
  if (logo.source === "simple-icons" && logo.slug) {
    return `https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/${logo.slug}.svg`;
  }
  if (logo.source === "devicon" && logo.path) {
    return `https://raw.githubusercontent.com/devicons/devicon/master/icons/${logo.path}`;
  }
  if ((logo.source === "cncf" || logo.source === "official") && logo.path) {
    return logo.path;
  }
  return "";
}

function offlineRef(url) {
  return String(url || "").replace(/^https?:\/\//i, "");
}

function normalizeSVG(svg, logo) {
  let out = String(svg || "").trim();
  if (!/^<svg[\s>]/i.test(out)) {
    throw new Error("downloaded content is not an SVG document");
  }
  out = out.replace(/<script[\s\S]*?<\/script>/gi, "");
  out = out.replace(/\son[a-z]+\s*=\s*"[^"]*"/gi, "");
  out = out.replace(/\son[a-z]+\s*=\s*'[^']*'/gi, "");
  out = out.replace(/https?:\/\//gi, "");
  out = out.replace(/\bxmlns=["']www\.w3\.org\/2000\/svg["']/i, `xmlns="http:&#47;&#47;www.w3.org/2000/svg"`);
  if (!/\bxmlns=/i.test(out)) {
    out = out.replace(/<svg\b/i, `<svg xmlns="http:&#47;&#47;www.w3.org/2000/svg"`);
  }
  out = out.replace(/<svg\b/i, `<svg data-efp-logo-id="${escapeAttr(logo.id)}"`);
  if (!/fill=/i.test(out) && logo.color) {
    out = out.replace(/<svg\b/i, `<svg fill="${escapeAttr(logo.color)}"`);
  }
  return `${out}\n`;
}

function escapeAttr(value) {
  return String(value || "").replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

async function existingSources() {
  try {
    return await readJSON(sourcesPath);
  } catch {
    return { schema: "efp.visual.logo_sources.v1", generated_at: "", sources: [] };
  }
}

function mergeSources(current, entries) {
  const byID = new Map((current.sources || []).map((entry) => [entry.id, entry]));
  for (const entry of entries) byID.set(entry.id, entry);
  return {
    schema: "efp.visual.logo_sources.v1",
    generated_at: new Date().toISOString(),
    sources: Array.from(byID.values()).sort((a, b) => a.id.localeCompare(b.id))
  };
}

async function main() {
  const opts = parseArgs(process.argv);
  const catalog = await readJSON(catalogPath);
  const logos = selectLogos(catalog, opts);
  const downloaded = [];
  const failed = [];
  const skipped = [];
  for (const logo of logos) {
    const url = remoteURL(logo);
    const target = path.join(root, logo.target);
    if (!url) {
      skipped.push({ id: logo.id, reason: "no remote url configured" });
      continue;
    }
    if (!opts.force) {
      try {
        await fs.access(target);
        skipped.push({ id: logo.id, reason: "already exists" });
        continue;
      } catch {
      }
    }
    if (opts.dryRun) {
      downloaded.push({ id: logo.id, url, target: logo.target, dry_run: true });
      continue;
    }
    try {
      if (typeof fetch !== "function") throw new Error("global fetch is unavailable in this Node runtime");
      const res = await fetch(url, { redirect: "follow" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const svg = normalizeSVG(await res.text(), logo);
      await fs.mkdir(path.dirname(target), { recursive: true });
      await fs.writeFile(target, svg);
      downloaded.push({
        id: logo.id,
        label: logo.label,
        source: logo.source,
        remote_ref: offlineRef(url),
        path: logo.target.replace(/^templates\/visual\/_shared\//, ""),
        license: logo.license,
        color: logo.color || "",
        fetched_at: new Date().toISOString()
      });
    } catch (err) {
      failed.push({ id: logo.id, url, error: err.message });
    }
  }
  if (!opts.dryRun && downloaded.length) {
    await writeJSON(sourcesPath, mergeSources(await existingSources(), downloaded));
  }
  await writeJSON(manifestPath, {
    schema: "efp.visual.logo_catalog_manifest.v1",
    generated_from: "scripts/assets/logo_catalog.json",
    generated_at: new Date().toISOString(),
    logos: (catalog.logos || []).map((logo) => ({
      id: logo.id,
      label: logo.label,
      source: logo.source,
      target: logo.target || "",
      model: logo.model || "",
      license: logo.license || "",
      vendored: Boolean(logo.target)
    }))
  });
  const summary = { ok: failed.length === 0, downloaded, skipped, failed };
  console.log(JSON.stringify(summary, null, 2));
  if (failed.length) process.exitCode = 1;
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
