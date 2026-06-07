#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const manifestPath = path.join(root, "templates/visual/_shared/assets/manifests/generated-models.json");

async function main() {
  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
  const models = [];
  for (const model of manifest.models || []) {
    const file = path.join(root, "templates/visual/_shared", model.path);
    try {
      const stat = await fs.stat(file);
      models.push({ ...model, byte_length: stat.size, too_large: stat.size > 250000 });
    } catch (err) {
      models.push({ ...model, missing: true, error: err.message });
    }
  }
  models.sort((a, b) => a.id.localeCompare(b.id));
  const out = { ...manifest, optimized_at: new Date().toISOString(), models };
  await fs.writeFile(manifestPath, `${JSON.stringify(out, null, 2)}\n`);
  console.log(JSON.stringify({ ok: !models.some((model) => model.missing || model.too_large), models }, null, 2));
  if (models.some((model) => model.missing || model.too_large)) process.exitCode = 1;
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
