@echo off
REM ==========================================================================
REM  NullGate WAF - Automatic Public Deployment (Windows)
REM  النشر الفوري على الإنترنت
REM ==========================================================================

echo.
echo ╔═══════════════════════════════════════════════════════════╗
echo ║   NullGate WAF - Automatic Cloud Deployment              ║
echo ║   النشر الفوري على الإنترنت                              ║
echo ╚═══════════════════════════════════════════════════════════╝
echo.

set REPO_URL=https://github.com/fat1322im-hub/NullGate

echo [1/3] Checking deployment options...
echo.

REM Check for Railway CLI
where railway >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo ✓ Railway CLI found - deploying...
    railway up
    echo.
    echo ════════════════════════════════════════════════════════════
    echo   🌐 Your URL is ready!
    echo   Run: railway open
    echo ════════════════════════════════════════════════════════════
    goto :eof
)

REM Check for Fly CLI
where fly >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo ✓ Fly.io CLI found - deploying...
    fly launch --yes --now
    echo.
    fly status
    echo.
    echo ════════════════════════════════════════════════════════════
    echo   🌐 Your URL is shown above
    echo   Or run: fly open
    echo ════════════════════════════════════════════════════════════
    goto :eof
)

REM Fallback: Direct user to Render (easiest)
echo No CLI found. Using Render (easiest option)...
echo.
echo ════════════════════════════════════════════════════════════
echo   📋 AUTOMATIC DEPLOYMENT STEPS:
echo.
echo   1. Opening Render in browser...
echo   2. Click 'Create Web Service' (free)
echo   3. Wait 2-3 minutes for build
echo   4. Copy your URL from Render dashboard
echo.
echo   Your URL will look like:
echo   https://nullgate-waf-xxxx.onrender.com
echo ════════════════════════════════════════════════════════════
echo.

REM Open browser to Render deploy page
start "" "https://render.com/deploy?repo=%REPO_URL%"

echo.
echo Press any key when deployment finishes...
pause >nul

echo.
echo ✅ Check your Render dashboard for the public URL!
echo    Login details are in Environment variables
pause
