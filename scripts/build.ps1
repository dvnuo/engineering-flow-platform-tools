$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$Version = '0.1.0'
if ($args.Count -gt 0 -and ($args[0] -eq '--snapshot' -or $args[0] -eq '-Snapshot')) {
  $Version = '0.1.0-snapshot'
}
$Commit = (git rev-parse --short HEAD 2>$null)
if (-not $Commit) { $Commit = 'unknown' }
$Date = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$Ldflags = "-X engineering-flow-platform-tools/internal/version.Version=$Version -X engineering-flow-platform-tools/internal/version.Commit=$Commit -X engineering-flow-platform-tools/internal/version.Date=$Date"

function Build-One($goos, $goarch, $exe) {
  $outdir = "dist/$goos-$goarch"
  New-Item -ItemType Directory -Force -Path $outdir | Out-Null
  $env:CGO_ENABLED = '0'
  $env:GOOS = $goos
  $env:GOARCH = $goarch
  go build -ldflags "$Ldflags" -o "$outdir/jira$exe" ./cmd/jira
  go build -ldflags "$Ldflags" -o "$outdir/confluence$exe" ./cmd/confluence
  go build -ldflags "$Ldflags" -o "$outdir/jenkins$exe" ./cmd/jenkins
  go build -ldflags "$Ldflags" -o "$outdir/browser$exe" ./cmd/browser
  go build -ldflags "$Ldflags" -o "$outdir/inspect-image$exe" ./cmd/inspect-image
  go build -ldflags "$Ldflags" -o "$outdir/log$exe" ./cmd/log
}

Build-One 'linux' 'amd64' ''
Build-One 'linux' 'arm64' ''
Build-One 'darwin' 'amd64' ''
Build-One 'darwin' 'arm64' ''
Build-One 'windows' 'amd64' '.exe'
Build-One 'windows' 'arm64' '.exe'
