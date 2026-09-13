@echo off
REM efp-bridge-verify.bat - run the PowerShell verification with the execution
REM policy bypassed for this process only, so it works from cmd.exe or a
REM double-click without changing machine settings.
REM
REM Usage: efp-bridge-verify.bat https://portal.example.com [-SkipLogin] [-StopSession] [-Port 8765]
setlocal
if "%~1"=="" (
  echo Usage: efp-bridge-verify.bat ^<portal-url^> [-SkipLogin] [-StopSession] [-Port 8765]
  exit /b 2
)
set "PORTAL_URL=%~1"
shift
set "EXTRA="
:collect
if not "%~1"=="" (
  set "EXTRA=%EXTRA% %1"
  shift
  goto collect
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0efp-bridge-verify.ps1" -PortalUrl "%PORTAL_URL%"%EXTRA%
exit /b %ERRORLEVEL%
