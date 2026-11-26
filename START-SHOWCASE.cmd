@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Go 1.25 or newer is required. Install Go and reopen CMD.
  pause
  exit /b 1
)

call :check_showcase
if not errorlevel 1 goto already_running

curl.exe -sS --max-time 2 -o nul http://127.0.0.1:8080/healthz >nul 2>&1
if not errorlevel 1 goto port_in_use
curl.exe -sS --max-time 2 -o nul http://127.0.0.1:9091/healthz >nul 2>&1
if not errorlevel 1 goto port_in_use

echo Starting NullGate in a separate window...
start "NullGate Showcase" /D "%~dp0" cmd /k "go run .\cmd\nullgate --config .\configs\config.yaml --showcase"

for /L %%i in (1,1,45) do (
  call :check_showcase
  if not errorlevel 1 goto ready
  timeout /t 2 /nobreak >nul
)

echo NullGate did not become ready within 90 seconds.
echo Check the "NullGate Showcase" window for the startup error.
pause
exit /b 1

:ready
echo NullGate is ready.
start "" "http://127.0.0.1:8080/original/"
start "" "http://127.0.0.1:8080/cyberlab/"
echo Original site: http://127.0.0.1:8080/original/
echo Cyber lab:     http://127.0.0.1:8080/cyberlab/
echo Admin panel:   http://127.0.0.1:9091/
echo Keep the "NullGate Showcase" window open while using the sites.
exit /b 0

:already_running
echo The NullGate showcase is already running.
start "" "http://127.0.0.1:8080/original/"
start "" "http://127.0.0.1:8080/cyberlab/"
echo Original site: http://127.0.0.1:8080/original/
echo Cyber lab:     http://127.0.0.1:8080/cyberlab/
echo Admin panel:   http://127.0.0.1:9091/
exit /b 0

:port_in_use
echo Port 8080 or 9091 is already in use by another process or an older NullGate.
echo Stop that process first, then run START-SHOWCASE.cmd again.
echo If it is an older NullGate window, press Ctrl+C in that window.
  pause
exit /b 1

:check_showcase
curl.exe -fsSL --max-time 3 -A "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36" -H "Accept: text/html,application/xhtml+xml" http://127.0.0.1:8080/ -o nul >nul 2>&1
if errorlevel 1 exit /b 1
curl.exe -fsS --max-time 3 -A "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36" -H "Accept: text/html,application/xhtml+xml" http://127.0.0.1:8080/cyberlab/ -o nul >nul 2>&1
if errorlevel 1 exit /b 1
exit /b 0
