@echo off
REM ==========================================================================
REM  NullGate WAF - launch the live control panel (Windows)
REM
REM  Builds NullGate + the demo backend, starts the WAF on :8080 with the admin
REM  dashboard enabled on 127.0.0.1:9091, and opens it in your browser. The
REM  admin socket is loopback-only and never exposed to the network.
REM  Close this window (or press Ctrl-C) to stop.
REM ==========================================================================
setlocal
cd /d "%~dp0.."

echo == building nullgate...
go build -trimpath -o nullgate.exe .\cmd\nullgate || goto :err

echo == building demo backend...
go build -trimpath -o demo-backend.exe .\examples\vulnerable-backend || goto :err

echo == starting vulnerable backend on :3000...
start "nullgate-backend" /b demo-backend.exe

echo == starting NullGate WAF on :8080 (admin on 127.0.0.1:9091)...
start "nullgate-waf" /b nullgate.exe --config configs\config.yaml --admin

echo == waiting for the control panel...
powershell -NoProfile -Command "for($i=0;$i -lt 40;$i++){try{Invoke-WebRequest -UseBasicParsing http://127.0.0.1:9091/ -TimeoutSec 1 ^| Out-Null; break}catch{Start-Sleep -Milliseconds 500}}"

start "" "http://127.0.0.1:9091/"

echo.
echo   Control panel:  http://127.0.0.1:9091/
echo   WAF proxy:      http://127.0.0.1:8080
echo.
echo Press any key to stop NullGate and the demo backend.
pause >nul
taskkill /f /im nullgate.exe   >nul 2>&1
taskkill /f /im demo-backend.exe >nul 2>&1
goto :eof

:err
echo Build failed. Make sure Go is installed and on PATH.
pause >nul
