<#
.SYNOPSIS
  一键验证"本地小程序 + 专用 Chrome + CDP"方案在这台机器上能不能走通。

.DESCRIPTION
  前提：browser CLI 已在 PATH；python 3 已在 PATH；efp_bridge_probe.py 和本脚本在同一目录。
  依次检查：
    1. browser CLI 能运行
    2. Chrome 能带调试端口和独立 profile 启动（RemoteDebuggingAllowed 实测）
    3. CDP 能列 tab、读页面
    4. https 的 Portal 页面能调 127.0.0.1 的本地服务（CORS + 私有网络预检）
    5. 端到端：页面 -> 本地服务 -> browser CLI -> Chrome -> 结果回页面
  中间会停一次让你在弹出的 Chrome 窗口里登录 Portal（-SkipLogin 跳过）。

.EXAMPLE
  .\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com
  .\efp-bridge-verify.ps1 -PortalUrl https://portal.example.com -SkipLogin -StopSession
#>
param(
  [Parameter(Mandatory = $true)] [string] $PortalUrl,
  [int] $ProbePort = 8765,
  [switch] $SkipLogin,
  [switch] $StopSession
)

$ErrorActionPreference = "Stop"
$results = New-Object System.Collections.ArrayList
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$probeScript = Join-Path $here "efp_bridge_probe.py"
$portalOrigin = ([Uri]$PortalUrl).GetLeftPart([UriPartial]::Authority)
$token = [Guid]::NewGuid().ToString("N")
$probeProc = $null

function Add-Result([string] $step, [bool] $pass, [string] $detail) {
  [void]$results.Add([pscustomobject]@{ Step = $step; Result = $(if ($pass) { "PASS" } else { "FAIL" }); Detail = $detail })
  $color = $(if ($pass) { "Green" } else { "Red" })
  Write-Host ("[{0}] {1} - {2}" -f $(if ($pass) { "PASS" } else { "FAIL" }), $step, $detail) -ForegroundColor $color
}

function Invoke-Browser([string[]] $cliArgs) {
  # browser CLI 失败时也会输出 JSON 信封，且可能退出码非 0；两种情况都解析。
  # PowerShell 5.1 里对原生程序做 2>&1 会把 stderr 行包成 ErrorRecord，配合 Stop 会中止脚本，
  # 所以这里临时改成 Continue，并把 ErrorRecord 还原成文本。
  $prev = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try { $out = & browser @cliArgs 2>&1 } finally { $ErrorActionPreference = $prev }
  # stdout 是 JSON 信封；stderr 可能有 chromedp 的日志噪音（例如 cdproto 版本旧于 Chrome 协议时的
  # "unknown IPAddressSpace value: Loopback"），只从 stdout 解析 JSON，stderr 单独保留供诊断。
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

try {
  # ---- 1. CLI 能运行 ----------------------------------------------------
  $r = Invoke-Browser @("version", "--json")
  if ($null -ne $r.Json -and $r.Json.ok) {
    Add-Result "1 browser CLI 可运行" $true ("version " + $r.Json.data.version)
  } else {
    Add-Result "1 browser CLI 可运行" $false (Get-ErrorText $r)
    throw "browser CLI 不可用，后续步骤无意义。若提示被组策略阻止，说明 AppLocker/WDAC 封了用户目录下的 exe。"
  }

  # ---- 2. Chrome 带调试端口 + 独立 profile ---------------------------------
  $r = Invoke-Browser @("open", "--url", $PortalUrl, "--json")
  if ($null -ne $r.Json -and $r.Json.ok -and $r.Json.data.browser_alive) {
    $st = Invoke-Browser @("session", "status", "default", "--json")
    $port = ""; $profile = ""
    if ($null -ne $st.Json -and $null -ne $st.Json.data) { $port = $st.Json.data.debug_port; $profile = $st.Json.data.profile_dir }
    Add-Result "2 Chrome 调试端口 + 独立 profile" $true ("debug_port=" + $port + " profile=" + $profile + " reused=" + $r.Json.data.reused)
  } else {
    $msg = Get-ErrorText $r
    Add-Result "2 Chrome 调试端口 + 独立 profile" $false $msg
    if ($msg -match "devtools_unavailable") {
      throw "DevTools 端点起不来，最可能是 RemoteDebuggingAllowed 被策略设为 false。小程序方案在此环境不可行，转 Electron 壳。"
    }
    throw "Chrome 未能启动，见上面的错误。"
  }

  # ---- 登录停顿 ------------------------------------------------------------
  if (-not $SkipLogin) {
    Write-Host ""
    Write-Host "请在弹出的 Chrome 窗口里登录 Portal，再随手打开一个工作站点的 tab。记录：静默登录 / 登录一次 / 要 MFA / 被拒。" -ForegroundColor Yellow
    Read-Host "完成后回到这里按 Enter 继续"
  }

  # ---- 3. CDP 列 tab + 读页面 ---------------------------------------------
  $tabs = Invoke-Browser @("tab", "list", "--json")
  if ($null -eq $tabs.Json -or -not $tabs.Json.ok) {
    Add-Result "3 CDP 读页面" $false (Get-ErrorText $tabs)
    throw "tab list 失败。"
  }
  $portalTab = $tabs.Json.data.tabs | Where-Object { $_.url -like ($portalOrigin + "*") } | Select-Object -First 1
  if ($null -eq $portalTab) { $portalTab = $tabs.Json.data.tabs | Select-Object -First 1 }
  [void](Invoke-Browser @("tab", "activate", "--target-id", $portalTab.id, "--json"))
  $snap = Invoke-Browser @("page", "snapshot", "--json")
  if ($null -ne $snap.Json -and $snap.Json.ok) {
    Add-Result "3 CDP 读页面" $true ("tabs=" + @($tabs.Json.data.tabs).Count + " snapshot ok on " + $portalTab.url)
  } else {
    Add-Result "3 CDP 读页面" $false (Get-ErrorText $snap)
  }

  # ---- 4. 页面 -> 127.0.0.1 ------------------------------------------------
  if (-not (Test-Path $probeScript)) { throw "找不到 $probeScript" }
  $probeLog = Join-Path $env:TEMP "efp-bridge-probe.log"
  $probeErr = Join-Path $env:TEMP "efp-bridge-probe.err"
  $probeProc = Start-Process -FilePath "python" -ArgumentList @('"' + $probeScript + '"', "--portal-origin", $portalOrigin, "--port", $ProbePort, "--token", $token, "--browser-exe", "browser") `
    -WindowStyle Hidden -PassThru -RedirectStandardOutput $probeLog -RedirectStandardError $probeErr
  $ready = $false
  for ($i = 0; $i -lt 20 -and -not $ready; $i++) {
    Start-Sleep -Milliseconds 250
    try {
      $resp = Invoke-WebRequest -Uri ("http://127.0.0.1:{0}/ping" -f $ProbePort) -Headers @{ Origin = $portalOrigin } -UseBasicParsing -TimeoutSec 2
      if ($resp.StatusCode -eq 200) { $ready = $true }
    } catch {}
  }
  if (-not $ready) {
    Add-Result "4 页面调本地端口" $false ("本地验证服务没起来，看 " + $probeErr)
    throw "probe 未就绪。"
  }
  # 关键：fetch 必须从 Portal 页面的 origin 发出，所以用 browser page fetch 在 Portal tab 里执行。
  $ping = Invoke-Browser @("page", "fetch", "--url", ("http://127.0.0.1:{0}/ping" -f $ProbePort), "--json")
  $pingOk = ($null -ne $ping.Json -and $ping.Json.ok -and $ping.Json.data.status -eq 200)
  if ($pingOk) {
    Add-Result "4 页面调本地端口" $true ("status 200, 预检和 CORS 通过 (origin " + $portalOrigin + ")")
  } else {
    $detail = Get-ErrorText $ping
    if ($null -ne $ping.Json -and $null -ne $ping.Json.data) { $detail = ("status=" + $ping.Json.data.status + " error=" + $ping.Json.data.error) }
    Add-Result "4 页面调本地端口" $false $detail
  }

  # ---- 5. 端到端 -------------------------------------------------------------
  if ($pingOk) {
    # 这里用 browser page fetch 代替页面自己的 JS 去触发本地服务，外层命令握着 default 会话锁，
    # 所以本地服务采用异步作业：先应答 202，等外层命令结束后再执行 tab list；脚本直接轮询作业结果。
    $job = [Guid]::NewGuid().ToString("N")
    $url = ("http://127.0.0.1:{0}/run?token={1}&args=tab,list&job={2}" -f $ProbePort, $token, $job)
    $e2e = Invoke-Browser @("page", "fetch", "--url", $url, "--json")
    $accepted = ($null -ne $e2e.Json -and $e2e.Json.ok -and $e2e.Json.data.status -eq 202)
    if (-not $accepted) {
      $detail = Get-ErrorText $e2e
      if ($null -ne $e2e.Json -and $null -ne $e2e.Json.data) { $detail = ("status=" + $e2e.Json.data.status + " body=" + $e2e.Json.data.body_preview) }
      Add-Result "5 端到端 页面->本地服务->CLI->Chrome" $false ("页面发出的命令未被本地服务接受: " + $detail)
    } else {
      $done = $null
      for ($i = 0; $i -lt 40 -and $null -eq $done; $i++) {
        Start-Sleep -Milliseconds 500
        try {
          $jr = Invoke-WebRequest -Uri ("http://127.0.0.1:{0}/jobs/{1}" -f $ProbePort, $job) -Headers @{ Origin = $portalOrigin } -UseBasicParsing -TimeoutSec 5
          $jj = $jr.Content | ConvertFrom-Json
          if ($jj.data.state -eq "done") { $done = $jj.data }
        } catch {}
      }
      if ($null -ne $done -and $done.payload.ok -and $null -ne $done.payload.result -and $done.payload.result.ok) {
        $n = @($done.payload.result.data.tabs).Count
        Add-Result "5 端到端 页面->本地服务->CLI->Chrome" $true ("页面触发的 tab list 已执行，返回 " + $n + " 个 tab")
      } elseif ($null -eq $done) {
        Add-Result "5 端到端 页面->本地服务->CLI->Chrome" $false "作业 20 秒内未完成"
      } else {
        Add-Result "5 端到端 页面->本地服务->CLI->Chrome" $false (($done.payload | ConvertTo-Json -Compress -Depth 6) -replace "\s+", " ")
      }
    }
  } else {
    Add-Result "5 端到端 页面->本地服务->CLI->Chrome" $false "跳过：第 4 步未通过"
  }
}
catch {
  Write-Host ("中止: " + $_.Exception.Message) -ForegroundColor Red
}
finally {
  if ($null -ne $probeProc -and -not $probeProc.HasExited) { try { Stop-Process -Id $probeProc.Id -Force } catch {} }
  if ($StopSession) { [void](Invoke-Browser @("session", "stop", "default", "--json")) }
  Write-Host ""
  $results | Format-Table -AutoSize -Wrap
  Write-Host "判读：1 失败 = 用户目录 exe 被封，本地方案全出局；2 失败 = 远程调试被策略禁，转 Electron 壳；4 失败 = 改为小程序出站 WebSocket 连 Portal。" -ForegroundColor Cyan
}
