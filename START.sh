#!/usr/bin/env bash
# One-click public deployment for NullGate WAF

set -e

echo ""
echo "========================================"
echo "  NullGate WAF - Quick Start"
echo "========================================"
echo ""

# Check for cloudflared
if ! command -v cloudflared &> /dev/null; then
    echo "[!] Cloudflare Tunnel not found"
    echo ""

    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Install with: brew install cloudflared"
    else
        echo "Downloading cloudflared..."
        wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O cloudflared
        chmod +x cloudflared
        sudo mv cloudflared /usr/local/bin/ 2>/dev/null || mv cloudflared ~/bin/
    fi
fi

echo "[OK] Cloudflare Tunnel ready"
echo ""

echo "Building NullGate..."
go build -o nullgate ./cmd/nullgate
go build -o demo-backend ./examples/vulnerable-backend

echo ""
echo "Starting services..."
./demo-backend &
sleep 2

./nullgate --config configs/config.yaml --admin &
sleep 4

echo ""
echo "Creating public URL..."
echo ""
echo "========================================"
cloudflared tunnel --url http://localhost:8080 &

echo ""
echo "YOUR PUBLIC URL WILL APPEAR ABOVE"
echo "Look for: https://xxxxx.trycloudflare.com"
echo ""
echo "The URL reaches only the protected WAF listener."
echo "The admin dashboard and backend are not tunneled."
echo ""
echo "Press Ctrl+C to stop"

wait
