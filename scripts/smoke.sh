#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

go run ./cmd/jira --help >/dev/null
go run ./cmd/confluence --help >/dev/null
go run ./cmd/jenkins --help >/dev/null
go run ./cmd/browser --help >/dev/null
go run ./cmd/inspect-image --help >/dev/null
go run ./cmd/visual --help >/dev/null
go run ./cmd/jira commands --json >/dev/null
go run ./cmd/confluence commands --json >/dev/null
go run ./cmd/jenkins commands --json >/dev/null
go run ./cmd/browser commands --json >/dev/null
go run ./cmd/inspect-image commands --json >/dev/null
go run ./cmd/visual commands --json >/dev/null
go run ./cmd/browser schema probe --json >/dev/null
go run ./cmd/jenkins schema job.build --json >/dev/null
go run ./cmd/inspect-image schema inspect --json >/dev/null
go run ./cmd/visual schema render --json >/dev/null
go run ./cmd/inspect-image help llm >/dev/null
go run ./cmd/inspect-image models --json >/dev/null
go run ./cmd/inspect-image auth status --json >/dev/null
go run ./cmd/jira version --json >/dev/null
go run ./cmd/confluence version --json >/dev/null
go run ./cmd/jenkins version --json >/dev/null
go run ./cmd/browser version --json >/dev/null
go run ./cmd/inspect-image version --json >/dev/null
go run ./cmd/visual version --json >/dev/null
go run ./cmd/visual template list --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json >/dev/null
tmp="$(mktemp -d)"
go run ./cmd/visual render --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --out "$tmp/run-trace" --title "Smoke Agent Run Trace" --json >/dev/null
test -f "$tmp/run-trace/index.html"
