$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

go run ./cmd/jira --help | Out-Null
go run ./cmd/confluence --help | Out-Null
go run ./cmd/jira commands --json | Out-Null
go run ./cmd/confluence commands --json | Out-Null
go run ./cmd/jira version --json | Out-Null
go run ./cmd/confluence version --json | Out-Null
