$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

function Build-One($goos, $goarch, $exe) {
  $outdir = "dist/$goos-$goarch"
  New-Item -ItemType Directory -Force -Path $outdir | Out-Null
  $env:CGO_ENABLED = '0'
  $env:GOOS = $goos
  $env:GOARCH = $goarch
  go build -o "$outdir/jira$exe" ./cmd/jira
  go build -o "$outdir/confluence$exe" ./cmd/confluence
}

Build-One 'linux' 'amd64' ''
Build-One 'darwin' 'amd64' ''
Build-One 'darwin' 'arm64' ''
Build-One 'windows' 'amd64' '.exe'
