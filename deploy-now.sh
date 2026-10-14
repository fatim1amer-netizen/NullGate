#!/usr/bin/env bash
# ==========================================================================
#  NullGate WAF - Automatic Public Deployment
#  This script starts a deployment flow; a provider account is required.
# ==========================================================================

set -e

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   NullGate WAF - Automatic Cloud Deployment              ║"
echo "║   النشر الفوري على الإنترنت                              ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

REPO_URL="https://github.com/fatim1amer-netizen/NullGate"

echo "[1/2] Checking deployment options..."
echo ""

# Check for Fly CLI
if command -v fly &> /dev/null || command -v flyctl &> /dev/null; then
    echo "✓ Fly.io CLI found - deploying the configured WAF..."
    fly deploy
    echo ""
    URL=$(fly status --json 2>/dev/null | grep -o '"Hostname":"[^"]*"' | cut -d'"' -f4)
    echo "════════════════════════════════════════════════════════════"
    echo "  🌐 Your URL: https://$URL"
    echo "════════════════════════════════════════════════════════════"
    exit 0
fi

# Fallback: Direct user to Render
echo "No CLI found. Using Render (easiest option)..."
echo ""
echo "════════════════════════════════════════════════════════════"
echo "  📋 AUTOMATIC DEPLOYMENT STEPS:"
echo ""
echo "  1. Open: https://render.com/deploy?repo=$REPO_URL"
echo "  2. Review the render.yaml blueprint and create the service"
echo "  3. Wait for build and verify /healthz and the forwarded site"
echo "  4. The GitHub changes must be pushed before Render can build them"
echo ""
echo "  Your URL will look like:"
echo "  https://nullgate-waf-xxxx.onrender.com"
echo "════════════════════════════════════════════════════════════"
echo ""

# Try to open browser automatically
if command -v xdg-open &> /dev/null; then
    xdg-open "https://render.com/deploy?repo=$REPO_URL" &
elif command -v open &> /dev/null; then
    open "https://render.com/deploy?repo=$REPO_URL" &
fi

echo ""
echo "The Render service deploys independently; this script does not wait for it."
echo "Check the Render dashboard for the URL and deployment status."
