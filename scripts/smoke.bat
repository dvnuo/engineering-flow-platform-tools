@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."

go run ./cmd/jira --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/confluence --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins --help >nul
if errorlevel 1 exit /b 1
go run ./cmd/browser --help >nul
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
go run ./cmd/browser commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image commands --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual commands --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/browser schema probe --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/jenkins schema job.build --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image schema inspect --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual schema render --json >nul
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
go run ./cmd/browser version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/inspect-image version --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual version --json >nul
if errorlevel 1 exit /b 1

go run ./cmd/visual template categories --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template list --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template list --template-dir ./templates/visual --category agent --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template schema agent.run_trace --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template schema codebase.module_dependency_graph --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1
go run ./cmd/visual template doctor --template-dir ./templates/visual --json >nul
if errorlevel 1 exit /b 1

set "TMP_ROOT=%TEMP%\visual-%RANDOM%-%RANDOM%"
mkdir "%TMP_ROOT%"
if errorlevel 1 exit /b 1

for %%T in (
  foundation.graph_3d
  agent.run_trace
  codebase.module_dependency_graph
  runtime.event_reconcile_loop
  debug.incident_timeline
  project.requirements_to_code_trace
  knowledge.evidence_board
  planning.plan_dag
  business.kpi_control_room
  education.auth_flow_animation
) do (
  set "TEMPLATE=%%T"
  set "OUT_NAME=!TEMPLATE:.=-!"
  set "OUT_DIR=%TMP_ROOT%\!OUT_NAME!"
  set "INPUT_PATH=templates\visual\!TEMPLATE!\examples\basic.input.json"
  go run ./cmd/visual render --template !TEMPLATE! --template-dir ./templates/visual --input "!INPUT_PATH!" --out "!OUT_DIR!" --title "Smoke !TEMPLATE!" --json >nul
  if errorlevel 1 exit /b 1
  if not exist "!OUT_DIR!\index.html" (
    echo visual smoke did not create index.html for !TEMPLATE! 1>&2
    exit /b 1
  )
)

exit /b 0
