@echo off
REM ==========================================================================
REM  NullGate WAF - One-Click Public Launch
REM  النشر الفوري بضغطة واحدة
REM ==========================================================================

echo.
echo ========================================
echo   NullGate WAF - Quick Start
echo ========================================
echo.

REM Check for cloudflared
where cloudflared >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo [OK] Cloudflare Tunnel found
    goto :build
)

echo [!] Cloudflare Tunnel not found
echo.
echo Downloading cloudflared...
powershell -Command "Invoke-WebRequest -Uri 'https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe' -OutFile 'cloudflared.exe'"

if not exist cloudflared.exe (
    echo [ERROR] Download failed. Please download manually from:
    echo https://github.com/cloudflare/cloudflared/releases
    pause
    exit /b 1
)

echo [OK] cloudflared downloaded

:build
echo.
echo Building NullGate...
go build -o nullgate.exe .\cmd\nullgate || goto :error
go build -o demo-backend.exe .\examples\vulnerable-backend || goto :error

echo.
echo Starting services...
start /B demo-backend.exe
timeout /t 2 /nobreak >nul

start /B nullgate.exe --config configs\config.yaml --admin
timeout /t 4 /nobreak >nul

echo.
echo Creating public URL...
echo.
echo ========================================
start "" cloudflared tunnel --url http://localhost:9091

echo.
echo YOUR PUBLIC URL WILL APPEAR ABOVE
echo Look for: https://xxxxx.trycloudflare.com
echo.
echo Share that URL with anyone!
echo Login: admin / admin123
echo.
echo Press Ctrl+C to stop
pause
goto :eof

:error
echo Build failed. Install Go from: https://go.dev/dl/
pause
