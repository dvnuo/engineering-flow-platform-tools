#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

build_one() {
  local goos="$1" goarch="$2" exe="$3"
  local outdir="dist/${goos}-${goarch}"
  mkdir -p "$outdir"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -o "$outdir/jira$exe" ./cmd/jira
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -o "$outdir/confluence$exe" ./cmd/confluence
}

build_one linux amd64 ""
build_one darwin amd64 ""
build_one darwin arm64 ""
build_one windows amd64 ".exe"
