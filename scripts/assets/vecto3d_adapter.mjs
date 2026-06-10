#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import { spawn } from "node:child_process";

function parseArgs(argv) {
  const out = { svg: "", out: "", probe: false };
  for (let i = 2; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--probe") out.probe = true;
    else if (arg === "--svg") out.svg = argv[++i] || "";
    else if (arg === "--out") out.out = argv[++i] || "";
    else if (arg === "--help" || arg === "-h") usage(0);
    else usage(2, `unknown argument: ${arg}`);
  }
  return out;
}

function usage(code, message = "") {
  if (message) console.error(message);
  console.error("usage: node scripts/assets/vecto3d_adapter.mjs --probe");
  console.error("       node scripts/assets/vecto3d_adapter.mjs --svg in.svg --out out.glb");
  process.exit(code);
}

async function probe() {
  const command = process.env.VECTO3D_COMMAND || "";
  const dir = process.env.VECTO3D_DIR || "";
  if (command) {
    return { available: true, mode: "command", command };
  }
  if (!dir) {
    return { available: false, reason: "VECTO3D_DIR or VECTO3D_COMMAND is not configured" };
  }
  const pkgPath = path.join(dir, "package.json");
  try {
    const pkg = JSON.parse(await fs.readFile(pkgPath, "utf8"));
    const hasCLI = Boolean(pkg.bin);
    const exports = pkg.exports || {};
    const scripts = pkg.scripts || {};
    const candidates = Object.keys(scripts).filter((key) => /export|build|convert/i.test(key));
    if (hasCLI) return { available: true, mode: "package-bin", dir, bin: pkg.bin };
    return {
      available: false,
      reason: "vecto3d checkout is a Next.js/browser app and does not expose a headless SVG-to-GLB export API",
      dir,
      package: pkg.name || "vecto3d",
      scripts: candidates,
      exports
    };
  } catch (err) {
    return { available: false, reason: `cannot inspect vecto3d package: ${err.message}`, dir };
  }
}

function runCommand(command, args) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: "inherit", shell: true });
    child.on("close", (code) => resolve(code || 0));
  });
}

async function main() {
  const opts = parseArgs(process.argv);
  const status = await probe();
  if (opts.probe || !opts.svg || !opts.out) {
    console.log(JSON.stringify(status, null, 2));
    process.exit(status.available ? 0 : 1);
  }
  if (!status.available) {
    console.log(JSON.stringify(status, null, 2));
    process.exit(1);
  }
  if (status.mode === "command") {
    const code = await runCommand(status.command, [opts.svg, opts.out]);
    process.exit(code);
  }
  console.log(JSON.stringify({ available: false, reason: "configured vecto3d mode is not executable by this adapter" }, null, 2));
  process.exit(1);
}

main().catch((err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});
