#!/usr/bin/env bash
# ==========================================================================
#  NullGate WAF - Automatic Public Deployment
#  This script creates a REAL public https:// URL instantly
# ==========================================================================

set -e

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║   NullGate WAF - Automatic Cloud Deployment              ║"
echo "║   النشر الفوري على الإنترنت                              ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

REPO_URL="https://github.com/fat1322im-hub/NullGate"

echo "[1/3] Checking deployment options..."
echo ""

# Check for Railway CLI
if command -v railway &> /dev/null; then
    echo "✓ Railway CLI found - deploying..."
    railway up
    echo ""
    echo "════════════════════════════════════════════════════════════"
    echo "  🌐 Your URL is ready!"
    echo "  Run: railway open"
    echo "════════════════════════════════════════════════════════════"
    exit 0
fi

# Check for Fly CLI
if command -v fly &> /dev/null || command -v flyctl &> /dev/null; then
    echo "✓ Fly.io CLI found - deploying..."
    fly launch --yes --now
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
echo "  2. Click 'Create Web Service' (free)"
echo "  3. Wait 2-3 minutes for build"
echo "  4. Copy your URL from Render dashboard"
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

echo "Press Enter when deployment finishes..."
read

echo ""
echo "✅ Check your Render dashboard for the public URL!"
echo "   Login details are in Environment variables"
