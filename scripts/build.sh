#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="0.1.0"
if [[ "${1:-}" == "--snapshot" ]]; then
  VERSION="0.1.0-snapshot"
fi
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
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "$LDFLAGS" -o "$outdir/log$exe" ./cmd/log
}

build_one linux amd64 ""
build_one linux arm64 ""
build_one darwin amd64 ""
build_one darwin arm64 ""
build_one windows amd64 ".exe"
build_one windows arm64 ".exe"
