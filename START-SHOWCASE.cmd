@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Go is required. Install Go 1.25 or newer, then run this file again.
  pause
  exit /b 1
)

go run .\cmd\nullgate --config .\configs\config.yaml --showcase
if errorlevel 1 (
  echo NullGate showcase stopped with an error.
  pause
)
