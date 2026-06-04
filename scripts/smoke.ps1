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
go run ./cmd/visual template categories --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template list --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template list --template-dir ./templates/visual --category agent --json | Out-Null
go run ./cmd/visual template schema agent.run_trace --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template schema codebase.module_dependency_graph --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json | Out-Null
$tmp = New-Item -ItemType Directory -Force -Path (Join-Path ([System.IO.Path]::GetTempPath()) ("visual-" + [System.Guid]::NewGuid().ToString("N")))
$templates = @(
  'foundation.graph_3d',
  'agent.run_trace',
  'codebase.module_dependency_graph',
  'runtime.event_reconcile_loop',
  'debug.incident_timeline',
  'project.requirements_to_code_trace',
  'knowledge.evidence_board',
  'planning.plan_dag',
  'business.kpi_control_room',
  'education.auth_flow_animation'
)
foreach ($template in $templates) {
  $out = Join-Path $tmp.FullName ($template -replace '\.', '-')
  $inputPath = Join-Path (Join-Path (Join-Path 'templates' 'visual') $template) (Join-Path 'examples' 'basic.input.json')
  go run ./cmd/visual render --template $template --template-dir ./templates/visual --input $inputPath --out $out --title "Smoke $template" --json | Out-Null
  if (-not (Test-Path (Join-Path $out 'index.html'))) { throw "visual smoke did not create index.html for $template" }
}
