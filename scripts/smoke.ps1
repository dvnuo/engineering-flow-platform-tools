$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

go run ./cmd/jira --help | Out-Null
go run ./cmd/confluence --help | Out-Null
go run ./cmd/browser --help | Out-Null
go run ./cmd/inspect-image --help | Out-Null
go run ./cmd/jira commands --json | Out-Null
go run ./cmd/confluence commands --json | Out-Null
go run ./cmd/browser commands --json | Out-Null
go run ./cmd/inspect-image commands --json | Out-Null
go run ./cmd/browser schema probe --json | Out-Null
go run ./cmd/inspect-image schema inspect --json | Out-Null
go run ./cmd/inspect-image help llm | Out-Null
go run ./cmd/inspect-image models --json | Out-Null
go run ./cmd/inspect-image auth status --json | Out-Null
go run ./cmd/jira version --json | Out-Null
go run ./cmd/confluence version --json | Out-Null
go run ./cmd/browser version --json | Out-Null
go run ./cmd/inspect-image version --json | Out-Null
