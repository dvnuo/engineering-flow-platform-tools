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
go run ./cmd/visual schema inspect-input --json | Out-Null
go run ./cmd/visual schema inspect-plan --json | Out-Null
go run ./cmd/visual schema inspect-render --json | Out-Null
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
go run ./cmd/visual template list --template-dir ./templates/visual --category mermaid --json | Out-Null
go run ./cmd/visual template schema mermaid.sequence --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template guide mermaid.sequence --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template schema mermaid.flowchart --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual template schema mermaid.architecture --template-dir ./templates/visual --json | Out-Null
go run ./cmd/visual inspect-input --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --json | Out-Null
go run ./cmd/visual template doctor --template-dir ./templates/visual --json | Out-Null
$planOut = Join-Path ([System.IO.Path]::GetTempPath()) "visual-plan-smoke"
go run ./cmd/visual inspect-plan --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --out $planOut --json | Out-Null

$tmp = New-Item -ItemType Directory -Force -Path (Join-Path ([System.IO.Path]::GetTempPath()) ("visual-" + [System.Guid]::NewGuid().ToString("N")))
$templates = @(
  'mermaid.sequence',
  'mermaid.flowchart',
  'mermaid.timeline',
  'mermaid.sankey',
  'mermaid.mindmap',
  'mermaid.pie',
  'mermaid.wardley'
)
foreach ($template in $templates) {
  $out = Join-Path $tmp.FullName ($template -replace '\.', '-')
  $inputPath = Join-Path (Join-Path (Join-Path 'templates' 'visual') $template) (Join-Path 'examples' 'basic.mmd')
  go run ./cmd/visual render --template $template --template-dir ./templates/visual --input $inputPath --out $out --title "Smoke $template" --json | Out-Null
  if (-not (Test-Path (Join-Path $out 'index.html'))) { throw "visual smoke did not create index.html for $template" }
  go run ./cmd/visual inspect-render --template-dir ./templates/visual --out $out --json | Out-Null
}

$gallery = Join-Path $tmp.FullName 'mermaid-architecture'
go run ./cmd/visual render `
  --template mermaid.architecture `
  --template-dir ./templates/visual `
  --input ./templates/visual/mermaid.architecture/examples/basic.mmd `
  --out $gallery `
  --json | Out-Null

if ($env:EFP_SKIP_BROWSER_SMOKE -ne '1') {
  $screenshot = Join-Path $gallery 'screenshot.png'
  go run ./cmd/visual inspect-browser `
    --template-dir ./templates/visual `
    --out $gallery `
    --screenshot $screenshot `
    --timeout 90 `
    --json | Out-Null
  go run ./cmd/visual inspect-render `
    --template-dir ./templates/visual `
    --out $gallery `
    --screenshot $screenshot `
    --json | Out-Null
}
