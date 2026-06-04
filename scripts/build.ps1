$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$Version = '0.1.0'
$TargetOS = ''
$TargetArch = ''

function Show-Usage {
  [Console]::Error.WriteLine('Usage: scripts/build.ps1 [-Snapshot] [-OS linux|darwin|windows] [-Arch amd64|arm64]')
}

function Test-OSValue($value) {
  return @('linux', 'darwin', 'windows') -contains $value
}

function Test-ArchValue($value) {
  return @('amd64', 'arm64') -contains $value
}

function Fail-Usage($message) {
  [Console]::Error.WriteLine($message)
  Show-Usage
  exit 2
}

for ($i = 0; $i -lt $args.Count; $i++) {
  $arg = $args[$i]
  switch ($arg) {
    { $_ -in @('--snapshot', '-Snapshot', '-snapshot') } {
      $Version = '0.1.0-snapshot'
      continue
    }
    { $_ -in @('--os', '-OS', '-Os', '-os') } {
      if ($i + 1 -ge $args.Count -or -not (Test-OSValue $args[$i + 1])) {
        Fail-Usage "Unknown or missing OS value."
      }
      $TargetOS = $args[$i + 1]
      $i++
      continue
    }
    { $_ -in @('--arch', '-Arch', '-arch') } {
      if ($i + 1 -ge $args.Count -or -not (Test-ArchValue $args[$i + 1])) {
        Fail-Usage "Unknown or missing Arch value."
      }
      $TargetArch = $args[$i + 1]
      $i++
      continue
    }
    { $_ -in @('-h', '--help') } {
      Show-Usage
      exit 0
    }
    default {
      Fail-Usage "Unknown argument: $arg"
    }
  }
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
  go build -ldflags "$Ldflags" -o "$outdir/visual$exe" ./cmd/visual
}

$Targets = @(
  @{ OS = 'linux'; Arch = 'amd64'; Exe = '' },
  @{ OS = 'linux'; Arch = 'arm64'; Exe = '' },
  @{ OS = 'darwin'; Arch = 'amd64'; Exe = '' },
  @{ OS = 'darwin'; Arch = 'arm64'; Exe = '' },
  @{ OS = 'windows'; Arch = 'amd64'; Exe = '.exe' },
  @{ OS = 'windows'; Arch = 'arm64'; Exe = '.exe' }
)

$selected = 0
foreach ($target in $Targets) {
  if ($TargetOS -and $target['OS'] -ne $TargetOS) { continue }
  if ($TargetArch -and $target['Arch'] -ne $TargetArch) { continue }
  Build-One $target['OS'] $target['Arch'] $target['Exe']
  $selected++
}

if ($selected -eq 0) {
  Fail-Usage 'No build targets selected.'
}
