#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="0.1.0"
TARGET_OS=""
TARGET_ARCH=""

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/build.sh [--snapshot] [--os linux|darwin|windows] [--arch amd64|arm64]
USAGE
}

valid_os() {
  case "$1" in
    linux|darwin|windows) return 0 ;;
    *) return 1 ;;
  esac
}

valid_arch() {
  case "$1" in
    amd64|arm64) return 0 ;;
    *) return 1 ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --snapshot)
      VERSION="0.1.0-snapshot"
      shift
      ;;
    --os)
      if [[ $# -lt 2 ]] || ! valid_os "$2"; then
        usage
        exit 2
      fi
      TARGET_OS="$2"
      shift 2
      ;;
    --arch)
      if [[ $# -lt 2 ]] || ! valid_arch "$2"; then
        usage
        exit 2
      fi
      TARGET_ARCH="$2"
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
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X engineering-flow-platform-tools/internal/version.Version=${VERSION} -X engineering-flow-platform-tools/internal/version.Commit=${COMMIT} -X engineering-flow-platform-tools/internal/version.Date=${DATE}"

build_one() {
  local goos="$1" goarch="$2" exe="$3"
  local outdir="dist/${goos}-${goarch}"
  mkdir -p "$outdir"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/jira$exe" ./cmd/jira
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/confluence$exe" ./cmd/confluence
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/jenkins$exe" ./cmd/jenkins
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/browser$exe" ./cmd/browser
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/inspect-image$exe" ./cmd/inspect-image
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/visual$exe" ./cmd/visual
}

TARGETS=(
  "linux amd64 "
  "linux arm64 "
  "darwin amd64 "
  "darwin arm64 "
  "windows amd64 .exe"
  "windows arm64 .exe"
)

selected=0
for target in "${TARGETS[@]}"; do
  read -r goos goarch exe <<<"$target"
  if [[ -n "$TARGET_OS" && "$goos" != "$TARGET_OS" ]]; then
    continue
  fi
  if [[ -n "$TARGET_ARCH" && "$goarch" != "$TARGET_ARCH" ]]; then
    continue
  fi
  build_one "$goos" "$goarch" "$exe"
  selected=$((selected + 1))
done

if [[ "$selected" -eq 0 ]]; then
  usage
  exit 2
fi
