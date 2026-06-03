#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

go run ./cmd/jira --help >/dev/null
go run ./cmd/confluence --help >/dev/null
go run ./cmd/jenkins --help >/dev/null
go run ./cmd/browser --help >/dev/null
go run ./cmd/inspect-image --help >/dev/null
go run ./cmd/log --help >/dev/null
go run ./cmd/jira commands --json >/dev/null
go run ./cmd/confluence commands --json >/dev/null
go run ./cmd/jenkins commands --json >/dev/null
go run ./cmd/browser commands --json >/dev/null
go run ./cmd/inspect-image commands --json >/dev/null
go run ./cmd/log commands --json >/dev/null
go run ./cmd/browser schema probe --json >/dev/null
go run ./cmd/jenkins schema job.build --json >/dev/null
go run ./cmd/inspect-image schema inspect --json >/dev/null
go run ./cmd/log schema analyze --json >/dev/null
go run ./cmd/inspect-image help llm >/dev/null
go run ./cmd/inspect-image models --json >/dev/null
go run ./cmd/inspect-image auth status --json >/dev/null
go run ./cmd/jira version --json >/dev/null
go run ./cmd/confluence version --json >/dev/null
go run ./cmd/jenkins version --json >/dev/null
go run ./cmd/browser version --json >/dev/null
go run ./cmd/inspect-image version --json >/dev/null
go run ./cmd/log version --json >/dev/null

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/app.log" <<'EOF'
2026-06-03T10:00:00Z INFO service started
2026-06-03T10:01:00Z ERROR database password=secret timeout after 3000ms
java.lang.RuntimeException: boom
    at example.Main.main(Main.java:10)
EOF

out="$tmp/log-smoke-output.txt"
go run ./cmd/log analyze --source "$tmp/app.log" --run "$tmp/run" --json > "$out"
go run ./cmd/log profile --run "$tmp/run" --json >> "$out"
go run ./cmd/log search --run "$tmp/run" --query timeout --json >> "$out"
go run ./cmd/log window --run "$tmp/run" --entry-id entry_000002 --before 1 --after 1 --json >> "$out"
go run ./cmd/log extract --run "$tmp/run" --kind stacktrace --json >> "$out"
if grep -R "secret" "$out" "$tmp/run"; then
  echo "log smoke leaked an unredacted secret" >&2
  exit 1
fi
