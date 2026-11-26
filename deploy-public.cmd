@echo off
REM ==========================================================================
REM  NullGate WAF - Public Deployment Script (Windows)
REM
REM  This script exposes only NullGate's protected listener.
REM  Configure a trusted production upstream in configs\config.yaml first.
REM ==========================================================================
setlocal enabledelayedexpansion

cd /d "%~dp0"

echo.
echo ========================================
echo   NullGate WAF - Public Deployment
echo ========================================
echo.

REM Check if cloudflared is installed
where cloudflared >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    set TUNNEL_TYPE=cloudflare
    echo [*] Cloudflare Tunnel detected
) else (
    REM Check if ngrok is installed
    where ngrok >nul 2>&1
    if %ERRORLEVEL% EQU 0 (
        set TUNNEL_TYPE=ngrok
        echo [*] ngrok detected
    ) else (
        set TUNNEL_TYPE=none
        echo [!] No tunnel service found. Installing Cloudflare Tunnel...
        echo.
        echo Please download cloudflared from:
        echo https://github.com/cloudflare/cloudflared/releases
        echo.
        echo Or install ngrok from:
        echo https://ngrok.com/download
        echo.
        pause
        exit /b 1
    )
)

echo.
echo [1/3] Building NullGate...
go build -trimpath -ldflags="-s -w" -o nullgate.exe .\cmd\nullgate || goto :error

echo.
echo [2/3] Starting NullGate...
echo.

REM Start WAF with admin enabled
for /f "usebackq" %%P in (`powershell -NoProfile -Command "$p=Start-Process -FilePath '%CD%\nullgate.exe' -ArgumentList '--config','configs\config.yaml','--admin' -PassThru; $p.Id"`) do set WAF_PID=%%P
timeout /t 3 /nobreak >nul

echo [3/3] Creating public tunnel...
echo.

if "%TUNNEL_TYPE%"=="cloudflare" (
    echo Starting Cloudflare Tunnel...
    echo.
    for /f "usebackq" %%P in (`powershell -NoProfile -Command "$p=Start-Process -FilePath cloudflared -ArgumentList 'tunnel','--url','http://localhost:8080' -PassThru; $p.Id"`) do set TUNNEL_PID=%%P
    timeout /t 5 /nobreak >nul
    echo.
    echo ============================================
    echo   YOUR PUBLIC URL IS SHOWN ABOVE
    echo   Look for: https://xxxxx.trycloudflare.com
    echo ============================================
) else if "%TUNNEL_TYPE%"=="ngrok" (
    echo Starting ngrok tunnel...
    echo.
    for /f "usebackq" %%P in (`powershell -NoProfile -Command "$p=Start-Process -FilePath ngrok -ArgumentList 'http','8080','--log','stdout' -PassThru; $p.Id"`) do set TUNNEL_PID=%%P
    timeout /t 3 /nobreak >nul
    echo.
    echo ============================================
    echo   Open http://localhost:4040 to get your URL
    echo   Or check the ngrok window above
    echo ============================================
)

echo.
echo [SUCCESS] The protected WAF listener is now publicly accessible.
echo.
echo Services running:
echo   - WAF:      http://localhost:8080
echo   - Admin:    http://127.0.0.1:9091 (local only)
echo.
echo The tunnel forwards only to port 8080. No default admin credentials are configured.
echo Configure server.target_backend to a trusted application before deployment; do not use the vulnerable example backend.
echo.
echo Press any key to stop this session's services...
pause >nul

echo.
echo Stopping services...
if defined WAF_PID powershell -NoProfile -Command "Stop-Process -Id %WAF_PID% -ErrorAction SilentlyContinue"
if defined TUNNEL_PID powershell -NoProfile -Command "Stop-Process -Id %TUNNEL_PID% -ErrorAction SilentlyContinue"

echo Done.
goto :eof

:error
echo.
echo [ERROR] Build failed. Make sure Go 1.25+ is installed.
pause
exit /b 1
