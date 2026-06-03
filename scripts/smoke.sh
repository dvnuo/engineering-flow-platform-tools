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
go run ./cmd/visual template categories --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template list --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template list --template-dir ./templates/visual --category agent --json >/dev/null
go run ./cmd/visual template schema agent.run_trace --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template schema codebase.module_dependency_graph --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json >/dev/null
tmp="$(mktemp -d)"
templates=(
  foundation.graph_3d
  agent.run_trace
  codebase.module_dependency_graph
  runtime.event_reconcile_loop
  debug.incident_timeline
  project.requirements_to_code_trace
  knowledge.evidence_board
  planning.plan_dag
  business.kpi_control_room
  education.auth_flow_animation
)
for template in "${templates[@]}"; do
  out="$tmp/${template//./-}"
  go run ./cmd/visual render --template "$template" --template-dir ./templates/visual --input "./templates/visual/$template/examples/basic.input.json" --out "$out" --title "Smoke $template" --json >/dev/null
  test -f "$out/index.html"
done
