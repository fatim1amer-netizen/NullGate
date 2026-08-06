@echo off
REM ==========================================================================
REM  NullGate WAF - Public Deployment Script (Windows)
REM
REM  This script deploys NullGate with a public URL accessible from anywhere.
REM  It automatically sets up Cloudflare Tunnel (free) or ngrok.
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
echo [1/4] Building NullGate...
go build -trimpath -ldflags="-s -w" -o nullgate.exe .\cmd\nullgate || goto :error

echo [2/4] Building demo backend...
go build -trimpath -ldflags="-s -w" -o demo-backend.exe .\examples\vulnerable-backend || goto :error

echo.
echo [3/4] Starting services...
echo.

REM Start backend
start "NullGate-Backend" /MIN demo-backend.exe
timeout /t 2 /nobreak >nul

REM Start WAF with admin enabled
start "NullGate-WAF" /MIN nullgate.exe --config configs\config.yaml --admin
timeout /t 3 /nobreak >nul

echo [4/4] Creating public tunnel...
echo.

if "%TUNNEL_TYPE%"=="cloudflare" (
    echo Starting Cloudflare Tunnel...
    echo.
    start "NullGate-Tunnel" cloudflared tunnel --url http://localhost:9091
    timeout /t 5 /nobreak >nul
    echo.
    echo ============================================
    echo   YOUR PUBLIC URL IS SHOWN ABOVE
    echo   Look for: https://xxxxx.trycloudflare.com
    echo ============================================
) else if "%TUNNEL_TYPE%"=="ngrok" (
    echo Starting ngrok tunnel...
    echo.
    start "NullGate-Tunnel" ngrok http 9091 --log stdout
    timeout /t 3 /nobreak >nul
    echo.
    echo ============================================
    echo   Open http://localhost:4040 to get your URL
    echo   Or check the ngrok window above
    echo ============================================
)

echo.
echo [SUCCESS] NullGate is now publicly accessible!
echo.
echo Services running:
echo   - WAF:      http://localhost:8080
echo   - Admin:    http://localhost:9091
echo   - Backend:  http://localhost:3000
echo.
echo Default login: admin / admin123
echo CHANGE THE PASSWORD IMMEDIATELY!
echo.
echo Press any key to stop all services...
pause >nul

echo.
echo Stopping services...
taskkill /f /im nullgate.exe >nul 2>&1
taskkill /f /im demo-backend.exe >nul 2>&1
taskkill /f /im cloudflared.exe >nul 2>&1
taskkill /f /im ngrok.exe >nul 2>&1

echo Done.
goto :eof

:error
echo.
echo [ERROR] Build failed. Make sure Go 1.21+ is installed.
pause
exit /b 1
