# Visual Shared Asset Attributions

The visual renderer uses local assets only. Build-time helpers may download allowlisted SVG logos, but rendered artifacts must not depend on remote URLs.

## Local Generic Symbols

- `efp-generic`: original vector symbols created for this repository.
- `efp-aws-generic`: local AWS-like semantic placeholders. These are not official AWS Architecture Icons.
- `efp-jenkins-generic`: local CI semantic placeholder. This is not Jenkins project artwork.

## Simple Icons Derived Assets

Entries with attribution id `simple-icons` are derived from the Simple Icons project and are fetched at build time from its public repository. The Simple Icons project publishes icons under CC0-1.0; brand and trademark rights remain with their owners.

Generated `.glb` files under `assets/models/generated` are local 3D badges derived from the corresponding local SVG and source catalog metadata. They are not official vendor 3D models.

vecto3d may be used as an optional build-time helper or source reference when a local checkout/command is configured. vecto3d is MIT licensed, but the generated badge license and trademark constraints follow the source SVG/logo license, not the converter license.

## Manual Review Required

Some products, such as Nacos or specific cloud service icons, are intentionally not vendored unless a reviewed source and license are recorded. Use the generic fallback glyphs for these products.
