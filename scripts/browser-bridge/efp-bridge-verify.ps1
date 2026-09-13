<#
.SYNOPSIS
  Verify the EFP browser bridge (browser serve) on this Windows workstation.

.DESCRIPTION
  Requires the `browser` CLI from engineering-flow-platform-tools on PATH.
  Runs the checks a workstation must pass before the Portal "Local browser"
  connector can work here:

    1. the browser CLI runs at all (user-directory executables are not blocked)
    2. `browser serve` starts and Chrome opens with a DevTools port and a
       dedicated profile (the RemoteDebuggingAllowed policy test)
    3. a page served from the Portal origin can call 127.0.0.1
       (CORS plus the private-network preflight Chrome requires)
    4. the bridge executes commands end to end (POST /run tab.list), answers
       the preflight, and refuses requests from other origins

  After step 2 the script pauses so you can sign in to the Portal inside the
  EFP browser window (-SkipLogin skips the pause). The bridge is stopped at
  the end; the Chrome window stays open unless -StopSession is given.

.EXAMPLE
  .\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com
  .\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com -SkipLogin -StopSession
#>
param(
  [Parameter(Mandatory = $true)] [string] $PortalUrl,
  [int] $Port = 8765,
  [switch] $SkipLogin,
  [switch] $StopSession
)

$ErrorActionPreference = "Stop"
$results = New-Object System.Collections.ArrayList
$portalOrigin = ([Uri]$PortalUrl).GetLeftPart([UriPartial]::Authority)
$serveProc = $null
$bridgePort = 0
$logDir = Join-Path $env:TEMP "efp-bridge-verify"
New-Item -ItemType Directory -Force $logDir | Out-Null
$serveOut = Join-Path $logDir "serve.out.log"
$serveErr = Join-Path $logDir "serve.err.log"

function Add-Result([string] $step, [bool] $pass, [string] $detail) {
  $label = $(if ($pass) { "PASS" } else { "FAIL" })
  [void]$results.Add([pscustomobject]@{ Step = $step; Result = $label; Detail = $detail })
  Write-Host ("[{0}] {1} - {2}" -f $label, $step, $detail) -ForegroundColor $(if ($pass) { "Green" } else { "Red" })
}

function Invoke-Browser([string[]] $cliArgs) {
  # The CLI prints a JSON envelope on stdout even when a command fails; stderr
  # may carry log noise, so only stdout is parsed.
  $prev = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try { $out = & browser @cliArgs 2>&1 } finally { $ErrorActionPreference = $prev }
  $stdoutLines = @($out | Where-Object { -not ($_ -is [System.Management.Automation.ErrorRecord]) } | ForEach-Object { [string]$_ })
  $stderrLines = @($out | Where-Object { $_ -is [System.Management.Automation.ErrorRecord] } | ForEach-Object { $_.Exception.Message })
  $raw = ($stdoutLines -join "`n")
  try { $obj = $raw | ConvertFrom-Json } catch { $obj = $null }
  return [pscustomobject]@{ Raw = $raw; Json = $obj; Stderr = ($stderrLines -join "`n") }
}

function Get-ErrorText($r) {
  if ($null -ne $r.Json -and $null -ne $r.Json.error) {
    return ("{0}: {1} {2}" -f $r.Json.error.code, $r.Json.error.message, $r.Json.error.hint)
  }
  $text = (($r.Raw + " " + $r.Stderr) -replace "\s+", " ").Trim()
  if ($text.Length -gt 400) { $text = $text.Substring(0, 400) + "..." }
  return $text
}

function Invoke-BridgePing([int] $candidatePort) {
  try {
    $resp = Invoke-WebRequest -Uri ("http://127.0.0.1:{0}/ping" -f $candidatePort) -Headers @{ Origin = $portalOrigin } -UseBasicParsing -TimeoutSec 3
    if ($resp.StatusCode -ne 200) { return $null }
    return ($resp.Content | ConvertFrom-Json)
  } catch {
    return $null
  }
}

try {
  # ---- 1. CLI runs -----------------------------------------------------------
  $r = Invoke-Browser @("version", "--json")
  if ($null -ne $r.Json -and $r.Json.ok) {
    Add-Result "1 browser CLI runs" $true ("version " + $r.Json.data.version)
  } else {
    Add-Result "1 browser CLI runs" $false (Get-ErrorText $r)
    throw "The browser CLI cannot run. If Windows reports it is blocked by group policy, executables in user directories are blocked (AppLocker/WDAC) and no local bridge can work on this machine."
  }

  # ---- 2. browser serve starts, Chrome opens with a DevTools port --------------
  $serveProc = Start-Process -FilePath "browser" -ArgumentList @("serve", "--origin", $portalOrigin, "--port", $Port, "--url", $PortalUrl, "--session", "default", "--json") `
    -WindowStyle Hidden -PassThru -RedirectStandardOutput $serveOut -RedirectStandardError $serveErr
  $ping = $null
  for ($i = 0; $i -lt 60 -and $null -eq $ping; $i++) {
    Start-Sleep -Milliseconds 500
    foreach ($candidate in ($Port..($Port + 5))) {
      $probe = Invoke-BridgePing $candidate
      if ($null -ne $probe -and $probe.ok) { $ping = $probe; $bridgePort = $candidate; break }
    }
    if ($serveProc.HasExited) { break }
  }
  if ($null -eq $ping) {
    $detail = "browser serve did not answer /ping within 30 s"
    if (Test-Path $serveErr) { $detail += "; stderr: " + ((Get-Content $serveErr -Raw) -replace "\s+", " ").Trim() }
    Add-Result "2 browser serve starts with Chrome" $false $detail
    throw "The bridge did not start."
  }
  # Chrome may still be starting; give the managed session a few seconds.
  for ($i = 0; $i -lt 20 -and -not $ping.data.session.alive; $i++) {
    Start-Sleep -Milliseconds 500
    $again = Invoke-BridgePing $bridgePort
    if ($null -ne $again) { $ping = $again }
  }
  if ($ping.data.session.alive) {
    Add-Result "2 browser serve starts with Chrome" $true ("bridge v" + $ping.data.version + " on port " + $bridgePort + ", DevTools port " + $ping.data.session.debug_port + ", profile ~/.efp/browser/profiles/default")
  } else {
    $st = Invoke-Browser @("session", "status", "default", "--json")
    Add-Result "2 browser serve starts with Chrome" $false ("bridge is up on port " + $bridgePort + " but the Chrome session is not alive: " + (Get-ErrorText $st))
    throw "Chrome did not expose a DevTools port. The most likely cause is the RemoteDebuggingAllowed policy set to false (check chrome://policy); the local bridge cannot work on this machine in that case."
  }

  # ---- login pause -------------------------------------------------------------
  if (-not $SkipLogin) {
    Write-Host ""
    Write-Host "Sign in to the Portal inside the EFP browser window that just opened, then open one work site in a new tab. Note whether the login was silent, needed one sign-in, asked for MFA, or was refused." -ForegroundColor Yellow
    Read-Host "Press Enter here when done"
  }

  # ---- 3. Portal page -> 127.0.0.1 ---------------------------------------------
  $tabs = Invoke-Browser @("tab", "list", "--json")
  if ($null -eq $tabs.Json -or -not $tabs.Json.ok) {
    Add-Result "3 page can call the bridge" $false (Get-ErrorText $tabs)
    throw "tab list failed."
  }
  $portalTab = $tabs.Json.data.tabs | Where-Object { $_.url -like ($portalOrigin + "*") } | Select-Object -First 1
  if ($null -eq $portalTab) { $portalTab = $tabs.Json.data.tabs | Select-Object -First 1 }
  [void](Invoke-Browser @("tab", "activate", "--target-id", $portalTab.id, "--json"))
  # The fetch must be issued from the Portal page's origin: `page fetch` runs it
  # inside that tab. /ping is lock-free on the bridge, so this cannot deadlock
  # with the session lock the CLI holds while the fetch runs.
  $fetch = Invoke-Browser @("page", "fetch", "--url", ("http://127.0.0.1:{0}/ping" -f $bridgePort), "--json")
  if ($null -ne $fetch.Json -and $fetch.Json.ok -and $fetch.Json.data.status -eq 200) {
    Add-Result "3 page can call the bridge" $true ("HTTP 200 from " + $portalTab.url + " to 127.0.0.1:" + $bridgePort + " (CORS and private-network preflight accepted)")
  } else {
    $detail = Get-ErrorText $fetch
    if ($null -ne $fetch.Json -and $null -ne $fetch.Json.data) { $detail = ("status=" + $fetch.Json.data.status + " error=" + $fetch.Json.data.error) }
    Add-Result "3 page can call the bridge" $false $detail
  }

  # ---- 4. bridge executes commands, preflight, foreign origin ------------------
  $preflightOk = $false
  try {
    $pre = Invoke-WebRequest -Uri ("http://127.0.0.1:{0}/run" -f $bridgePort) -Method Options -Headers @{ Origin = $portalOrigin; "Access-Control-Request-Method" = "POST"; "Access-Control-Request-Private-Network" = "true" } -UseBasicParsing -TimeoutSec 5
    $preflightOk = ($pre.StatusCode -eq 204 -and $pre.Headers["Access-Control-Allow-Origin"] -eq $portalOrigin -and $pre.Headers["Access-Control-Allow-Private-Network"] -eq "true")
  } catch { $preflightOk = $false }
  $foreignRejected = $false
  try {
    Invoke-WebRequest -Uri ("http://127.0.0.1:{0}/ping" -f $bridgePort) -Headers @{ Origin = "https://evil.example.test" } -UseBasicParsing -TimeoutSec 5 | Out-Null
  } catch {
    $foreignRejected = ($null -ne $_.Exception.Response -and [int]$_.Exception.Response.StatusCode -eq 403)
  }
  $run = $null
  try {
    $run = Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/run" -f $bridgePort) -Method Post -ContentType "application/json" -Headers @{ Origin = $portalOrigin } -Body (@{ command = "tab.list"; params = @{}; session = "default"; timeout_seconds = 20 } | ConvertTo-Json -Compress) -TimeoutSec 40
  } catch { $run = $null }
  $tabCount = $(if ($null -ne $run -and $run.ok) { @($run.data.tabs).Count } else { -1 })
  if ($tabCount -ge 0 -and $preflightOk -and $foreignRejected) {
    Add-Result "4 bridge executes commands" $true ("POST /run tab.list returned " + $tabCount + " tabs; preflight headers present; foreign origin rejected with 403")
  } else {
    $detail = "run ok=" + ($tabCount -ge 0) + " preflight ok=" + $preflightOk + " foreign origin rejected=" + $foreignRejected
    if ($null -ne $run -and -not $run.ok -and $null -ne $run.error) { $detail += "; error " + $run.error.code + " " + $run.error.message }
    Add-Result "4 bridge executes commands" $false $detail
  }
}
catch {
  Write-Host ("Stopped: " + $_.Exception.Message) -ForegroundColor Red
}
finally {
  if ($null -ne $serveProc -and -not $serveProc.HasExited) { try { Stop-Process -Id $serveProc.Id -Force } catch {} }
  if ($StopSession) { [void](Invoke-Browser @("session", "stop", "default", "--json")) }
  Write-Host ""
  $results | Format-Table -AutoSize -Wrap
  Write-Host "How to read this: step 1 failing means user-directory executables are blocked (AppLocker/WDAC); step 2 failing means Chrome refused a DevTools port (RemoteDebuggingAllowed policy); step 3 failing means the page cannot reach loopback and the bridge would need an outbound connection instead; step 4 failing is a bridge bug or a port conflict. Logs: $logDir" -ForegroundColor Cyan
}
