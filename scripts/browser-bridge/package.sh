#!/usr/bin/env bash
# package.sh - build the EFP browser bridge download packages: one zip per
# platform holding only what a member needs to install the bridge.
#
#   browser (browser.exe on Windows)   the EFP browser CLI; the Portal uses its serve bridge
#   install-bridge.cmd | .sh           registers the efp-bridge:// link for the current user
#   README.md                          install steps (PACKAGE_README.md from this directory)
#
# Usage: scripts/browser-bridge/package.sh [--os linux|darwin|windows] [--arch amd64|arm64]
#                                          [--version V] [--out DIR]
#
# Writes <out>/efp-browser-bridge-<os>-<arch>.zip (default out: dist/bridge).
# The Portal serves these names from app/static/downloads/ or from
# LOCAL_BROWSER_CLI_DOWNLOAD_URL with {platform} = <os>-<arch>.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
HERE="scripts/browser-bridge"
VERSION="0.1.0"
OUT="dist/bridge"
TARGET_OS=""
TARGET_ARCH=""

usage() {
  echo "Usage: scripts/browser-bridge/package.sh [--os linux|darwin|windows] [--arch amd64|arm64] [--version V] [--out DIR]" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --os)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      TARGET_OS="$2"
      shift 2
      ;;
    --arch)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      TARGET_ARCH="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      VERSION="$2"
      shift 2
      ;;
    --out)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      OUT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

case "$OUT" in
  /*) ;;
  *) OUT="$ROOT/$OUT" ;;
esac

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X engineering-flow-platform-tools/internal/version.Version=${VERSION} -X engineering-flow-platform-tools/internal/version.Commit=${COMMIT} -X engineering-flow-platform-tools/internal/version.Date=${DATE}"

# make_zip <absolute zip path> <staging dir>: zip(1) keeps the execute bits the
# macOS and Linux packages need. Where it is missing (Git Bash on Windows)
# Python writes the same layout and sets those bits by hand.
make_zip() {
  local zip_path="$1" stage="$2"
  rm -f "$zip_path"
  if command -v zip >/dev/null 2>&1; then
    (cd "$stage" && zip -q -r "$zip_path" .)
    return
  fi
  local py=""
  if command -v python3 >/dev/null 2>&1; then
    py="python3"
  elif command -v python >/dev/null 2>&1; then
    py="python"
  else
    echo "package.sh: neither zip nor python is available to build $zip_path" >&2
    exit 1
  fi
  (cd "$stage" && "$py" - "$zip_path" <<'PY'
import os
import sys
import zipfile

target = sys.argv[1]
with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED) as archive:
    for name in sorted(os.listdir(".")):
        info = zipfile.ZipInfo.from_file(name, name)
        info.compress_type = zipfile.ZIP_DEFLATED
        executable = name in ("browser", "install-bridge.sh")
        info.external_attr = (0o755 if executable else 0o644) << 16
        with open(name, "rb") as handle:
            archive.writestr(info, handle.read())
PY
  )
}

package_one() {
  local goos="$1" goarch="$2"
  local exe=""
  if [[ "$goos" == "windows" ]]; then
    exe=".exe"
  fi
  local name="efp-browser-bridge-${goos}-${goarch}"
  local stage="$OUT/stage/$name"
  rm -rf "$stage"
  mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$stage/browser$exe" ./cmd/browser
  if [[ "$goos" == "windows" ]]; then
    cp "$HERE/install-bridge.cmd" "$stage/"
  else
    cp "$HERE/install-bridge.sh" "$stage/"
    chmod +x "$stage/install-bridge.sh" "$stage/browser"
  fi
  cp "$HERE/PACKAGE_README.md" "$stage/README.md"
  make_zip "$OUT/$name.zip" "$stage"
  rm -rf "$stage"
  echo "$OUT/$name.zip"
}

TARGETS=(
  "windows amd64"
  "windows arm64"
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
)

selected=0
for target in "${TARGETS[@]}"; do
  read -r goos goarch <<<"$target"
  if [[ -n "$TARGET_OS" && "$goos" != "$TARGET_OS" ]]; then
    continue
  fi
  if [[ -n "$TARGET_ARCH" && "$goarch" != "$TARGET_ARCH" ]]; then
    continue
  fi
  package_one "$goos" "$goarch"
  selected=$((selected + 1))
done
rmdir "$OUT/stage" 2>/dev/null || true

if [[ "$selected" -eq 0 ]]; then
  usage
  exit 2
fi
