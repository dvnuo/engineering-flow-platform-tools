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
go run ./cmd/visual --help >/dev/null
go run ./cmd/jira commands --json >/dev/null
go run ./cmd/confluence commands --json >/dev/null
go run ./cmd/jenkins commands --json >/dev/null
go run ./cmd/aws-auth commands --json >/dev/null
go run ./cmd/browser commands --json >/dev/null
go run ./cmd/mobile-auto commands --json >/dev/null
go run ./cmd/inspect-image commands --json >/dev/null
go run ./cmd/visual commands --json >/dev/null
go run ./cmd/browser schema probe --json >/dev/null
go run ./cmd/mobile-auto schema run.start --json >/dev/null
go run ./cmd/mobile-auto schema observe --json >/dev/null
go run ./cmd/jenkins schema job.build --json >/dev/null
go run ./cmd/aws-auth schema login --json >/dev/null
go run ./cmd/inspect-image schema inspect --json >/dev/null
go run ./cmd/visual schema render --json >/dev/null
go run ./cmd/visual schema inspect-input --json >/dev/null
go run ./cmd/visual schema inspect-plan --json >/dev/null
go run ./cmd/visual schema inspect-render --json >/dev/null
go run ./cmd/visual schema inspect-browser --json >/dev/null
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
go run ./cmd/visual version --json >/dev/null

go run ./cmd/visual template categories --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template list --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template list --template-dir ./templates/visual --category mermaid --json >/dev/null
go run ./cmd/visual template schema mermaid.sequence --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template guide mermaid.sequence --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual template schema mermaid.flowchart --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual inspect-input --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --json >/dev/null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json >/dev/null
go run ./cmd/visual inspect-plan --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --out "${TMPDIR:-/tmp}/visual-plan-smoke" --json >/dev/null

tmp="$(mktemp -d)"
templates=(
  mermaid.sequence
  mermaid.flowchart
  mermaid.timeline
  mermaid.sankey
  mermaid.mindmap
  mermaid.pie
  mermaid.wardley
)
for template in "${templates[@]}"; do
  out="$tmp/${template//./-}"
  go run ./cmd/visual render --template "$template" --template-dir ./templates/visual --input "./templates/visual/$template/examples/basic.mmd" --out "$out" --title "Smoke $template" --json >/dev/null
  test -f "$out/index.html"
  go run ./cmd/visual inspect-render --template-dir ./templates/visual --out "$out" --json >/dev/null
done

gallery="$tmp/mermaid-architecture"
go run ./cmd/visual render \
  --template mermaid.architecture \
  --template-dir ./templates/visual \
  --input ./templates/visual/mermaid.architecture/examples/basic.mmd \
  --out "$gallery" \
  --json >/dev/null
if [[ "${EFP_SKIP_BROWSER_SMOKE:-}" != "1" ]]; then
  go run ./cmd/visual inspect-browser \
    --template-dir ./templates/visual \
    --out "$gallery" \
    --screenshot "$gallery/screenshot.png" \
    --timeout 90 \
    --json >/dev/null
  go run ./cmd/visual inspect-render \
    --template-dir ./templates/visual \
    --out "$gallery" \
    --screenshot "$gallery/screenshot.png" \
    --json >/dev/null
fi
