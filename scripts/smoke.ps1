$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

go run ./cmd/jira --help | Out-Null
go run ./cmd/confluence --help | Out-Null
go run ./cmd/jenkins --help | Out-Null
go run ./cmd/browser --help | Out-Null
go run ./cmd/inspect-image --help | Out-Null
go run ./cmd/visual --help | Out-Null
go run ./cmd/jira commands --json | Out-Null
go run ./cmd/confluence commands --json | Out-Null
go run ./cmd/jenkins commands --json | Out-Null
go run ./cmd/browser commands --json | Out-Null
go run ./cmd/inspect-image commands --json | Out-Null
go run ./cmd/visual commands --json | Out-Null
go run ./cmd/browser schema probe --json | Out-Null
go run ./cmd/jenkins schema job.build --json | Out-Null
go run ./cmd/inspect-image schema inspect --json | Out-Null
go run ./cmd/visual schema render --json | Out-Null
go run ./cmd/inspect-image help llm | Out-Null
go run ./cmd/inspect-image models --json | Out-Null
go run ./cmd/inspect-image auth status --json | Out-Null
go run ./cmd/jira version --json | Out-Null
go run ./cmd/confluence version --json | Out-Null
go run ./cmd/jenkins version --json | Out-Null
go run ./cmd/browser version --json | Out-Null
go run ./cmd/inspect-image version --json | Out-Null
go run ./cmd/visual version --json | Out-Null
go run ./cmd/visual template list --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json | Out-Null
$tmp = New-Item -ItemType Directory -Force -Path (Join-Path ([System.IO.Path]::GetTempPath()) ("visual-" + [System.Guid]::NewGuid().ToString("N")))
$out = Join-Path $tmp.FullName 'run-trace'
go run ./cmd/visual render --template agent.run_trace --template-dir ./templates/visual --input ./templates/visual/agent.run_trace/examples/basic.input.json --out $out --title "Smoke Agent Run Trace" --json | Out-Null
if (-not (Test-Path (Join-Path $out 'index.html'))) { throw 'visual smoke did not create index.html' }
