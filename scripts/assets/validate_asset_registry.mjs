#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const registryPath = path.join(root, "templates/visual/_shared/asset-registry.json");

function isRemote(value) {
  return /^(https?:)?\/\//i.test(String(value || ""));
}

async function exists(rel) {
  try {
    await fs.access(path.join(root, "templates/visual/_shared", rel));
    return true;
  } catch {
    return false;
  }
}

async function fileSize(rel) {
  try {
    return (await fs.stat(path.join(root, "templates/visual/_shared", rel))).size;
  } catch {
    return -1;
  }
}

async function validateEntries(kind, entries, attributions, errors, warnings) {
  const ids = Object.keys(entries || {}).sort();
  for (const id of ids) {
    const entry = entries[id] || {};
    if (!entry.path) {
      errors.push(`${kind}.${id}: missing path`);
      continue;
    }
    if (isRemote(entry.path)) {
      errors.push(`${kind}.${id}: remote path is forbidden`);
      continue;
    }
    if (entry.path.includes("..") || path.isAbsolute(entry.path)) {
      errors.push(`${kind}.${id}: path must stay under shared assets`);
      continue;
    }
    if (!(await exists(entry.path))) {
      errors.push(`${kind}.${id}: file does not exist: ${entry.path}`);
    }
    if (!entry.attribution_id) {
      errors.push(`${kind}.${id}: missing attribution_id`);
    } else if (!attributions.has(entry.attribution_id)) {
      errors.push(`${kind}.${id}: attribution_id not declared: ${entry.attribution_id}`);
    }
    if (kind === "models") {
      if (!entry.source_icon && entry.kind !== "placeholder") {
        warnings.push(`${kind}.${id}: missing source_icon`);
      }
      if (!entry.generated_by && entry.kind !== "placeholder") {
        warnings.push(`${kind}.${id}: missing generated_by`);
      }
      const size = await fileSize(entry.path);
      if (size > 250000) {
        warnings.push(`${kind}.${id}: model exceeds 250KB (${size} bytes)`);
      }
    }
    if (/vendor|official|simple-icons|devicon|aws|jenkins/i.test(entry.kind || id) && !entry.attribution_id) {
      errors.push(`${kind}.${id}: vendor-like asset must declare attribution_id`);
    }
  }
}

async function main() {
  const registry = JSON.parse(await fs.readFile(registryPath, "utf8"));
  const errors = [];
  const warnings = [];
  const attributions = new Set((registry.attributions || []).map((entry) => entry.id));
  await validateEntries("icons", registry.icons || {}, attributions, errors, warnings);
  await validateEntries("models", registry.models || {}, attributions, errors, warnings);
  for (const attr of registry.attributions || []) {
    if (!attr.id) errors.push("attributions: entry missing id");
    if (!attr.license) warnings.push(`attributions.${attr.id}: missing license`);
    if (!attr.source) warnings.push(`attributions.${attr.id}: missing source`);
  }
  const out = { ok: errors.length === 0, errors, warnings };
  console.log(JSON.stringify(out, null, 2));
  if (errors.length) process.exitCode = 1;
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
