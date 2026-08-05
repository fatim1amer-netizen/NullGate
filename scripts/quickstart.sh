#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
#  NullGate WAF — local quick start  /  تشغيل محلي سريع
#
#  Builds NullGate, starts the vulnerable demo backend on :3000, starts the WAF
#  on :8080 in front of it, then runs the live attack demo. Ctrl-C tears down
#  everything cleanly.
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

WAF_PORT=8080
BACKEND_PORT=3000
pids=()

cleanup() {
  echo
  echo "shutting down…"
  for pid in "${pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
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

echo "▸ starting NullGate WAF on :$WAF_PORT…"
./nullgate --config configs/config.yaml >/tmp/nullgate-waf.log 2>&1 &
pids+=($!)

# Wait for the WAF to accept connections.
echo -n "▸ waiting for WAF to come up"
for _ in $(seq 1 30); do
  if curl -s -o /dev/null "http://127.0.0.1:$WAF_PORT/" 2>/dev/null; then
    echo " — up."
    break
  fi
  echo -n "."
  sleep 0.5
done

echo
echo "▸ running live attack demo…"
echo
bash scripts/demo-attacks.sh "http://127.0.0.1:$WAF_PORT" || true

echo
echo "WAF log:     /tmp/nullgate-waf.log"
echo "backend log: /tmp/nullgate-backend.log"
echo "Press Ctrl-C to stop."
wait
