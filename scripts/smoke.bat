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

go run ./cmd/browser schema open --json >nul
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

exit /b 0
