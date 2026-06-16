@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."

set "VERSION=0.1.0"
set "TARGET_OS="
set "TARGET_ARCH="

:parse_args
if "%~1"=="" goto after_args
if /I "%~1"=="--snapshot" (
  set "VERSION=0.1.0-snapshot"
  shift
  goto parse_args
)
if /I "%~1"=="-snapshot" (
  set "VERSION=0.1.0-snapshot"
  shift
  goto parse_args
)
if /I "%~1"=="--os" goto parse_os
if /I "%~1"=="-os" goto parse_os
if /I "%~1"=="--arch" goto parse_arch
if /I "%~1"=="-arch" goto parse_arch
if /I "%~1"=="-h" (
  call :usage
  exit /b 0
)
if /I "%~1"=="--help" (
  call :usage
  exit /b 0
)
call :fail_usage "Unknown argument: %~1"
exit /b 2

:parse_os
if "%~2"=="" (
  call :fail_usage "Unknown or missing OS value."
  exit /b 2
)
call :set_target_os "%~2"
if errorlevel 1 (
  call :fail_usage "Unknown or missing OS value."
  exit /b 2
)
shift
shift
goto parse_args

:parse_arch
if "%~2"=="" (
  call :fail_usage "Unknown or missing Arch value."
  exit /b 2
)
call :set_target_arch "%~2"
if errorlevel 1 (
  call :fail_usage "Unknown or missing Arch value."
  exit /b 2
)
shift
shift
goto parse_args

:after_args
set "COMMIT=unknown"
for /f "delims=" %%I in ('git rev-parse --short HEAD 2^>nul') do (
  set "COMMIT=%%I"
  goto commit_done
)
:commit_done

set "BUILD_DATE=unknown"
for /f "delims=" %%I in ('git show -s --format=%%cI HEAD 2^>nul') do (
  set "BUILD_DATE=%%I"
  goto date_done
)
:date_done

set "LDFLAGS=-X engineering-flow-platform-tools/internal/version.Version=!VERSION! -X engineering-flow-platform-tools/internal/version.Commit=!COMMIT! -X engineering-flow-platform-tools/internal/version.Date=!BUILD_DATE!"

set /a SELECTED=0
call :maybe_build linux amd64 ""
if errorlevel 1 exit /b 1
call :maybe_build linux arm64 ""
if errorlevel 1 exit /b 1
call :maybe_build darwin amd64 ""
if errorlevel 1 exit /b 1
call :maybe_build darwin arm64 ""
if errorlevel 1 exit /b 1
call :maybe_build windows amd64 ".exe"
if errorlevel 1 exit /b 1
call :maybe_build windows arm64 ".exe"
if errorlevel 1 exit /b 1

if "!SELECTED!"=="0" (
  call :fail_usage "No build targets selected."
  exit /b 2
)

exit /b 0

:set_target_os
if /I "%~1"=="linux" (
  set "TARGET_OS=linux"
  exit /b 0
)
if /I "%~1"=="darwin" (
  set "TARGET_OS=darwin"
  exit /b 0
)
if /I "%~1"=="windows" (
  set "TARGET_OS=windows"
  exit /b 0
)
exit /b 1

:set_target_arch
if /I "%~1"=="amd64" (
  set "TARGET_ARCH=amd64"
  exit /b 0
)
if /I "%~1"=="arm64" (
  set "TARGET_ARCH=arm64"
  exit /b 0
)
exit /b 1

:maybe_build
set "GOOS_VALUE=%~1"
set "GOARCH_VALUE=%~2"
set "EXE_VALUE=%~3"
if defined TARGET_OS if /I not "!GOOS_VALUE!"=="!TARGET_OS!" exit /b 0
if defined TARGET_ARCH if /I not "!GOARCH_VALUE!"=="!TARGET_ARCH!" exit /b 0
call :build_one "!GOOS_VALUE!" "!GOARCH_VALUE!" "!EXE_VALUE!"
if errorlevel 1 exit /b 1
set /a SELECTED+=1
exit /b 0

:build_one
set "GOOS_VALUE=%~1"
set "GOARCH_VALUE=%~2"
set "EXE_VALUE=%~3"
set "OUTDIR=dist\!GOOS_VALUE!-!GOARCH_VALUE!"
if not exist "!OUTDIR!" mkdir "!OUTDIR!"
set "CGO_ENABLED=0"
set "GOOS=!GOOS_VALUE!"
set "GOARCH=!GOARCH_VALUE!"
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\jira!EXE_VALUE!" ./cmd/jira
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\confluence!EXE_VALUE!" ./cmd/confluence
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\jenkins!EXE_VALUE!" ./cmd/jenkins
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\aws-auth!EXE_VALUE!" ./cmd/aws-auth
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\browser!EXE_VALUE!" ./cmd/browser
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\inspect-image!EXE_VALUE!" ./cmd/inspect-image
if errorlevel 1 exit /b 1
go build -ldflags "!LDFLAGS!" -o "!OUTDIR!\visual!EXE_VALUE!" ./cmd/visual
if errorlevel 1 exit /b 1
exit /b 0

:usage
echo Usage: scripts\build.bat [--snapshot] [--os linux^|darwin^|windows] [--arch amd64^|arm64] 1>&2
exit /b 0

:fail_usage
echo %~1 1>&2
call :usage
exit /b 2
