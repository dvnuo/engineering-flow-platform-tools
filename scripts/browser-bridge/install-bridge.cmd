@echo off
setlocal
REM ============================================================================
REM install-bridge.cmd - register the efp-bridge:// protocol handler for the
REM EFP Portal local browser connector, using the browser.exe that sits next
REM to this script (the Portal download zip contains browser.exe,
REM install-bridge.cmd, and README.md).
REM
REM Usage:  install-bridge.cmd https://portal.example.com
REM         (double-clicked without an argument it asks for the address)
REM
REM No administrator rights are needed: the handler lives under
REM HKCU\Software\Classes\efp-bridge and points at this browser.exe.
REM ============================================================================

set "HERE=%~dp0"
set "ORIGIN=%~1"
set "BROWSER_EXE=%HERE%browser.exe"
set "RC=0"

if "%ORIGIN%"=="" (
  echo This installer needs the address of your EFP Portal, for example https://portal.example.com
  echo ^(the Portal's Connectors page shows the exact address^).
  set /p "ORIGIN=Portal address: "
)
if "%ORIGIN%"=="" (
  echo No Portal address given; nothing was changed.
  set "RC=2"
  goto :finish
)
if not exist "%BROWSER_EXE%" (
  echo browser.exe was not found next to this script: %BROWSER_EXE%
  echo Unzip the whole package and run install-bridge.cmd from that folder.
  set "RC=1"
  goto :finish
)

echo Registering the efp-bridge:// protocol handler for %ORIGIN% ...
"%BROWSER_EXE%" serve --register-protocol --origin "%ORIGIN%" --json
if errorlevel 1 (
  echo Registration failed. Read the JSON envelope above for error.code and error.hint.
  set "RC=1"
  goto :finish
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

:finish
REM A double-clicked script runs in a window that closes with it; keep the
REM outcome readable in that case (cmd /c) and stay quiet in a terminal.
echo %cmdcmdline% | find /i "/c" >nul 2>&1 && pause
endlocal & exit /b %RC%
