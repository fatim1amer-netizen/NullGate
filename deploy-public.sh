#!/usr/bin/env bash
# ==========================================================================
#  NullGate WAF - Public Deployment Script (Linux/macOS)
#
#  This script exposes only the protected WAF listener through a public tunnel.
#  The admin dashboard stays on loopback; configure a trusted real upstream.
# ==========================================================================

set -e

cd "$(dirname "$0")"

echo ""
echo "========================================"
echo "  NullGate WAF - Public Deployment"
echo "========================================"
echo ""

# Detect tunnel service
if command -v cloudflared &> /dev/null; then
    TUNNEL_TYPE="cloudflare"
    echo "[*] Cloudflare Tunnel detected"
elif command -v ngrok &> /dev/null; then
    TUNNEL_TYPE="ngrok"
    echo "[*] ngrok detected"
else
    TUNNEL_TYPE="none"
    echo "[!] No tunnel service found."
    echo ""
    echo "Install Cloudflare Tunnel:"
    echo "  macOS:  brew install cloudflared"
    echo "  Linux:  wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
    echo ""
    echo "Or install ngrok:"
    echo "  https://ngrok.com/download"
    echo ""
    exit 1
fi

echo ""
echo "[1/3] Building NullGate..."
go build -trimpath -ldflags="-s -w" -o nullgate ./cmd/nullgate

echo ""
echo "[2/3] Starting NullGate..."
echo ""

# Start WAF with admin enabled
./nullgate --config configs/config.yaml --admin &
WAF_PID=$!
sleep 3

echo "[3/3] Creating public tunnel..."
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Stopping services..."
    if [ -n "${TUNNEL_PID:-}" ]; then kill "$TUNNEL_PID" 2>/dev/null || true; fi
    kill "$WAF_PID" 2>/dev/null || true
    echo "Done."
    exit 0
}

trap cleanup SIGINT SIGTERM

if [ "$TUNNEL_TYPE" = "cloudflare" ]; then
    echo "Starting Cloudflare Tunnel..."
    echo ""
    cloudflared tunnel --url http://localhost:8080 &
    TUNNEL_PID=$!
    sleep 5
    echo ""
    echo "============================================"
    echo "  YOUR PUBLIC URL IS SHOWN ABOVE"
    echo "  Look for: https://xxxxx.trycloudflare.com"
    echo "============================================"
elif [ "$TUNNEL_TYPE" = "ngrok" ]; then
    echo "Starting ngrok tunnel..."
    echo ""
    ngrok http 8080 &
    TUNNEL_PID=$!
    sleep 3
    echo ""
    echo "============================================"
    echo "  Open http://localhost:4040 to get your URL"
    echo "  Or use: curl http://localhost:4040/api/tunnels"
    echo "============================================"
fi

echo ""
echo "[SUCCESS] The protected WAF listener is now publicly accessible."
echo ""
echo "Services running:"
echo "  - WAF:      http://localhost:8080"
echo "  - Admin:    http://127.0.0.1:9091 (local only)"
echo ""
echo "The tunnel forwards only to port 8080. No default admin credentials are configured."
echo "Configure server.target_backend to a trusted application before deployment; do not use the vulnerable example backend."
echo ""
echo "Press Ctrl+C to stop all services..."

# Wait indefinitely
wait
