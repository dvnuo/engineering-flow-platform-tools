$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

go run ./cmd/jira --help | Out-Null
go run ./cmd/confluence --help | Out-Null
go run ./cmd/jenkins --help | Out-Null
go run ./cmd/browser --help | Out-Null
go run ./cmd/inspect-image --help | Out-Null
go run ./cmd/log --help | Out-Null
go run ./cmd/jira commands --json | Out-Null
go run ./cmd/confluence commands --json | Out-Null
go run ./cmd/jenkins commands --json | Out-Null
go run ./cmd/browser commands --json | Out-Null
go run ./cmd/inspect-image commands --json | Out-Null
go run ./cmd/log commands --json | Out-Null
go run ./cmd/browser schema probe --json | Out-Null
go run ./cmd/jenkins schema job.build --json | Out-Null
go run ./cmd/inspect-image schema inspect --json | Out-Null
go run ./cmd/log schema analyze --json | Out-Null
go run ./cmd/inspect-image help llm | Out-Null
go run ./cmd/inspect-image models --json | Out-Null
go run ./cmd/inspect-image auth status --json | Out-Null
go run ./cmd/jira version --json | Out-Null
go run ./cmd/confluence version --json | Out-Null
go run ./cmd/jenkins version --json | Out-Null
go run ./cmd/browser version --json | Out-Null
go run ./cmd/inspect-image version --json | Out-Null
go run ./cmd/log version --json | Out-Null

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("log-smoke-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
  $logPath = Join-Path $tmp 'app.log'
  @'
2026-06-03T10:00:00Z INFO service started
2026-06-03T10:01:00Z ERROR database password=secret timeout after 3000ms
java.lang.RuntimeException: boom
    at example.Main.main(Main.java:10)
'@ | Set-Content -Path $logPath -Encoding UTF8
  $runDir = Join-Path $tmp 'run'
  $out = Join-Path $tmp 'log-smoke-output.txt'
  go run ./cmd/log analyze --source $logPath --run $runDir --json | Tee-Object -FilePath $out | Out-Null
  go run ./cmd/log profile --run $runDir --json | Tee-Object -FilePath $out -Append | Out-Null
  go run ./cmd/log search --run $runDir --query timeout --json | Tee-Object -FilePath $out -Append | Out-Null
  go run ./cmd/log window --run $runDir --entry-id entry_000002 --before 1 --after 1 --json | Tee-Object -FilePath $out -Append | Out-Null
  go run ./cmd/log extract --run $runDir --kind stacktrace --json | Tee-Object -FilePath $out -Append | Out-Null
  $paths = @($out, (Join-Path $runDir 'manifest.json'), (Join-Path $runDir 'entries.jsonl'), (Join-Path $runDir 'templates.json'))
  if (Select-String -Path $paths -Pattern 'secret' -SimpleMatch -Quiet) {
    throw 'log smoke leaked an unredacted secret'
  }
}
finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
