@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."

go run ./cmd/jira --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/confluence --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/aws-auth --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/browser --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/mobile-auto --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual --help >nul
if errorlevel 1 exit /b 1

go run ./cmd/jira commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/confluence commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/aws-auth commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/browser commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/mobile-auto commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual commands --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/browser schema probe --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/mobile-auto schema run.start --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/mobile-auto schema observe --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins schema job.build --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/aws-auth schema login --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image schema inspect --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema render --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema inspect-input --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema inspect-plan --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema inspect-render --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema inspect-browser --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/inspect-image help llm >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image models --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image auth status --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/jira version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/confluence version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/aws-auth version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/browser version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/mobile-auto version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual version --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/visual template categories --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template list --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template list --template-dir ./templates/visual --category mermaid --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template schema mermaid.sequence --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template guide mermaid.sequence --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template schema mermaid.flowchart --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual inspect-input --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template doctor --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1

set "PLAN_OUT=%TEMP%\visual-plan-smoke-%RANDOM%-%RANDOM%"
go run ./cmd/visual inspect-plan --template mermaid.sequence --template-dir ./templates/visual --input ./templates/visual/mermaid.sequence/examples/basic.mmd --out "%PLAN_OUT%" --json >nul
if errorlevel 1 exit /b 1

set "TMP_ROOT=%TEMP%\visual-%RANDOM%-%RANDOM%"
mkdir "%TMP_ROOT%"
if errorlevel 1 exit /b 1

for %%T in (
  mermaid.sequence
  mermaid.flowchart
  mermaid.timeline
  mermaid.sankey
  mermaid.mindmap
  mermaid.pie
  mermaid.wardley
) do (
  set "TEMPLATE=%%T"
  set "OUT_NAME=!TEMPLATE:.=-!"
  set "OUT_DIR=%TMP_ROOT%\!OUT_NAME!"
  go run ./cmd/visual render --template !TEMPLATE! --template-dir ./templates/visual --input "./templates/visual/!TEMPLATE!/examples/basic.mmd" --out "!OUT_DIR!" --title "Smoke !TEMPLATE!" --json >nul
  if errorlevel 1 exit /b 1
  if not exist "!OUT_DIR!\index.html" (
    echo visual smoke did not create index.html for !TEMPLATE! 1>&2
    exit /b 1
  )
  go run ./cmd/visual inspect-render --template-dir ./templates/visual --out "!OUT_DIR!" --json >nul
  if errorlevel 1 exit /b 1
)

set "ARCH_OUT=%TMP_ROOT%\mermaid-architecture"
go run ./cmd/visual render --template mermaid.architecture --template-dir ./templates/visual --input ./templates/visual/mermaid.architecture/examples/basic.mmd --out "%ARCH_OUT%" --json >nul
if errorlevel 1 exit /b 1

if not "%EFP_SKIP_BROWSER_SMOKE%"=="1" (
  go run ./cmd/visual inspect-browser --template-dir ./templates/visual --out "%ARCH_OUT%" --screenshot "%ARCH_OUT%\screenshot.png" --timeout 90 --json >nul
  if errorlevel 1 exit /b 1
  go run ./cmd/visual inspect-render --template-dir ./templates/visual --out "%ARCH_OUT%" --screenshot "%ARCH_OUT%\screenshot.png" --json >nul
  if errorlevel 1 exit /b 1
)

exit /b 0
