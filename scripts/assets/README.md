# Visual Asset Build Pipeline

This directory contains build-time helpers for the visual asset layer. They are intentionally not part of the runtime path.

## Commands

```bash
node --version
node scripts/assets/fetch_logo_assets.mjs --only nginx,redis,mysql,jenkins,kubernetes
node scripts/assets/fetch_logo_assets.mjs --only nginx,redis,mysql,jenkins,kubernetes --dry-run
node scripts/assets/convert_svg_to_3d.mjs --only nginx,redis,mysql,jenkins,kubernetes --write-registry
VECTO3D_DIR=/path/to/vecto3d node scripts/assets/convert_svg_to_3d.mjs --only nginx,redis,mysql --write-registry
node scripts/assets/validate_asset_registry.mjs
```

This repository currently has no Node package install requirement for these scripts. `npm install` is not needed unless a future script adds optional devDependencies. If vecto3d is unavailable or does not expose a headless conversion API, run:

```bash
node scripts/assets/convert_svg_to_3d.mjs --only nginx,redis,mysql --write-registry
```

`fetch_logo_assets.mjs` downloads only allowlisted SVG assets from `logo_catalog.json` and writes source records under `templates/visual/_shared/assets/attributions`.

`convert_svg_to_3d.mjs` tries the optional `vecto3d_adapter.mjs` first when `VECTO3D_DIR` or `VECTO3D_COMMAND` is configured. If no headless vecto3d export path is available, it uses the local fallback GLB badge generator. The fallback does not preserve exact SVG path geometry; it creates a small 3D semantic badge with the catalog color and source metadata.

`vecto3d_adapter.mjs` never launches the vecto3d browser UI and never calls remote services. vecto3d is MIT licensed, but generated output licensing follows the source SVG/logo license and trademark terms.

`validate_asset_registry.mjs` checks that every registry path is local, every referenced file exists, and every vendor/generated asset has attribution metadata.

Runtime HTML must never fetch remote logos, models, fonts, or scripts.
