@echo off
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "PID_FILE=%SCRIPT_DIR%cohesion.pid"

if not exist "%PID_FILE%" (
  echo cohesion.pid not found. Cohesion may already be stopped.
  exit /b 0
)

set /p PID=<"%PID_FILE%"
set "PID=%PID: =%"
if "%PID%"=="" (
  echo cohesion.pid is empty. Removing stale PID file.
  del /f /q "%PID_FILE%" >nul 2>&1
  exit /b 0
)

tasklist /FI "PID eq %PID%" | find "%PID%" >nul
if errorlevel 1 (
  echo Process %PID% is not running. Removing stale PID file.
  del /f /q "%PID_FILE%" >nul 2>&1
  exit /b 0
)

echo Stopping Cohesion (PID: %PID%)...
taskkill /PID %PID% /T >nul 2>&1
if errorlevel 1 (
  echo Graceful stop failed. Forcing termination...
  taskkill /PID %PID% /T /F >nul 2>&1
)

del /f /q "%PID_FILE%" >nul 2>&1
echo Cohesion stopped.
exit /b 0
