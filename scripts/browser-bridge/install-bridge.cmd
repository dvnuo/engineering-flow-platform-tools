@echo off
setlocal
REM ============================================================================
REM install-bridge.cmd - register the efp-bridge:// protocol handler for the
REM EFP Portal local browser connector, using the browser.exe that sits next
REM to this script (the Portal download zip contains browser.exe,
REM install-bridge.cmd, and README.md).
REM
REM Usage:  install-bridge.cmd https://portal.example.com
REM
REM No administrator rights are needed: the handler lives under
REM HKCU\Software\Classes\efp-bridge and points at this browser.exe.
REM ============================================================================

set "HERE=%~dp0"
set "ORIGIN=%~1"
set "BROWSER_EXE=%HERE%browser.exe"

if "%ORIGIN%"=="" (
  echo Usage: install-bridge.cmd ^<portal-origin^>
  echo Example: install-bridge.cmd https://portal.example.com
  exit /b 2
)
if not exist "%BROWSER_EXE%" (
  echo browser.exe was not found next to this script: %BROWSER_EXE%
  echo Unzip the whole package and run install-bridge.cmd from that folder.
  exit /b 1
)

echo Registering the efp-bridge:// protocol handler for %ORIGIN% ...
"%BROWSER_EXE%" serve --register-protocol --origin "%ORIGIN%" --json
if errorlevel 1 (
  echo Registration failed. Read the JSON envelope above for error.code and error.hint.
  exit /b 1
)

echo.
echo Next steps:
echo   1. Go back to the Portal Connectors page and click "Start bridge"
echo      (it opens efp-bridge://start?origin=...^&port=8765).
echo   2. Windows may ask once whether to open "EFP Bridge"; allow it and optionally
echo      tick "always allow". The bridge then starts browser serve in the background.
echo   3. Click "Test connection" on the Portal page. A Chrome window with a dedicated
echo      profile opens; log in to the Portal inside that window once.
echo   4. Turn on the browser switch in the chat composer when you want the agent to
echo      use your local browser for that chat.
echo.
echo Manual start (if the protocol link is blocked):
echo   "%BROWSER_EXE%" serve --origin "%ORIGIN%"
echo Remove the handler later with:
echo   "%BROWSER_EXE%" serve --unregister-protocol
endlocal
exit /b 0
