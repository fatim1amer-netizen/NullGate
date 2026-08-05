#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
#  NullGate WAF — launch the live control panel  /  تشغيل لوحة التحكم المباشرة
#
#  Builds NullGate + the demo backend, starts the WAF on :8080 with the admin
#  dashboard enabled on 127.0.0.1:9091, and opens it in your browser. The admin
#  socket is loopback-only — it is never exposed to the network. Ctrl-C stops
#  everything.
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

WAF_PORT=8080
BACKEND_PORT=3000
ADMIN_ADDR="127.0.0.1:9091"
pids=()

cleanup() {
  echo
  echo "shutting down…"
  for pid in "${pids[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "▸ building nullgate…"
go build -trimpath -o nullgate ./cmd/nullgate

echo "▸ building demo backend…"
go build -trimpath -o demo-backend ./examples/vulnerable-backend

echo "▸ starting vulnerable backend on :$BACKEND_PORT…"
./demo-backend >/tmp/nullgate-backend.log 2>&1 &
pids+=($!)

echo "▸ starting NullGate WAF on :$WAF_PORT (admin on $ADMIN_ADDR)…"
./nullgate --config configs/config.yaml --admin >/tmp/nullgate-waf.log 2>&1 &
pids+=($!)

# Wait for the admin socket to accept connections.
echo -n "▸ waiting for the control panel"
URL="http://$ADMIN_ADDR/"
for _ in $(seq 1 40); do
  if curl -s -o /dev/null "$URL" 2>/dev/null; then echo " — up."; break; fi
  echo -n "."; sleep 0.5
done

# Best-effort browser open across platforms.
if   command -v xdg-open >/dev/null 2>&1; then xdg-open "$URL" >/dev/null 2>&1 || true
elif command -v open     >/dev/null 2>&1; then open "$URL"     >/dev/null 2>&1 || true
elif command -v start    >/dev/null 2>&1; then start "" "$URL" >/dev/null 2>&1 || true
fi

echo
echo "  Control panel:  $URL"
echo "  WAF proxy:      http://127.0.0.1:$WAF_PORT"
echo "  WAF log:        /tmp/nullgate-waf.log"
echo
echo "Send some traffic (e.g. bash scripts/demo-attacks.sh http://127.0.0.1:$WAF_PORT)"
echo "and watch the panel light up. Press Ctrl-C to stop."
wait
