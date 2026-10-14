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

set REPO_URL=https://github.com/fatim1amer-netizen/NullGate

echo [1/2] Checking deployment options...
echo.

REM Check for Fly CLI
where fly >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo ✓ Fly.io CLI found - deploying the configured WAF...
    fly deploy
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
echo   2. Review the render.yaml blueprint and create the service
echo   3. Wait for build, then verify /healthz and the forwarded site
echo   4. The GitHub changes must be pushed before Render can build them
echo.
echo   Your URL will look like:
echo   https://nullgate-waf-xxxx.onrender.com
echo ════════════════════════════════════════════════════════════
echo.

REM Open browser to Render deploy page
start "" "https://render.com/deploy?repo=%REPO_URL%"

echo.
echo The Render service deploys independently; this script does not wait for it.
echo Check the Render dashboard for the URL and deployment status.
