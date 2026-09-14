#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

go run ./cmd/jira --help >/dev/null
go run ./cmd/confluence --help >/dev/null
go run ./cmd/jenkins --help >/dev/null
go run ./cmd/aws-auth --help >/dev/null
go run ./cmd/browser --help >/dev/null
go run ./cmd/mobile-auto --help >/dev/null
go run ./cmd/inspect-image --help >/dev/null
go run ./cmd/jira commands --json >/dev/null
go run ./cmd/confluence commands --json >/dev/null
go run ./cmd/jenkins commands --json >/dev/null
go run ./cmd/aws-auth commands --json >/dev/null
go run ./cmd/browser commands --json >/dev/null
go run ./cmd/mobile-auto commands --json >/dev/null
go run ./cmd/inspect-image commands --json >/dev/null
go run ./cmd/browser schema open --json >/dev/null
go run ./cmd/browser schema probe --json >/dev/null
go run ./cmd/mobile-auto schema run.start --json >/dev/null
go run ./cmd/mobile-auto schema observe --json >/dev/null
go run ./cmd/jenkins schema job.build --json >/dev/null
go run ./cmd/aws-auth schema login --json >/dev/null
go run ./cmd/inspect-image schema inspect --json >/dev/null
go run ./cmd/inspect-image help llm >/dev/null
go run ./cmd/inspect-image models --json >/dev/null
go run ./cmd/inspect-image auth status --json >/dev/null
go run ./cmd/jira version --json >/dev/null
go run ./cmd/confluence version --json >/dev/null
go run ./cmd/jenkins version --json >/dev/null
go run ./cmd/aws-auth version --json >/dev/null
go run ./cmd/browser version --json >/dev/null
go run ./cmd/mobile-auto version --json >/dev/null
go run ./cmd/inspect-image version --json >/dev/null
