#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
#  NullGate WAF — live attack demo  /  عرض حي للهجمات ضد جدار الحماية
#
#  Fires a battery of real attack payloads at a running NullGate instance and
#  verifies each is blocked, while confirming that benign traffic still passes.
#  يطلق مجموعة من الهجمات الحقيقية على NullGate ويتأكد من حجب كل واحدة،
#  مع التأكد أن الطلبات الطبيعية تمر بشكل سليم.
#
#  Usage:  ./scripts/demo-attacks.sh [BASE_URL]
#          BASE_URL defaults to http://127.0.0.1:8080
# ─────────────────────────────────────────────────────────────────────────────
set -u

BASE="${1:-http://127.0.0.1:8080}"
PASS=0
FAIL=0

# Colors (fall back to empty if not a TTY).
if [ -t 1 ]; then
  G=$'\e[32m'; R=$'\e[31m'; Y=$'\e[33m'; B=$'\e[36m'; N=$'\e[0m'
else
  G=""; R=""; Y=""; B=""; N=""
fi

hr() { printf '%s\n' "────────────────────────────────────────────────────────────"; }

# status CODE = the HTTP code we get back for a given curl invocation.
# expect_block  NAME  CURL_ARGS...   → expects a 4xx (blocked).
# expect_allow  NAME  CURL_ARGS...   → expects a 2xx (forwarded).

http_code() {
  curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$@"
}

expect_block() {
  local name="$1"; shift
  local code; code="$(http_code "$@")"
  if [ "$code" -ge 400 ] && [ "$code" -lt 500 ]; then
    printf '  %s✓ BLOCKED%s  %-38s → HTTP %s\n' "$G" "$N" "$name" "$code"
    PASS=$((PASS+1))
  else
    printf '  %s✗ LEAKED %s  %-38s → HTTP %s%s\n' "$R" "$N" "$name" "$code" ""
    FAIL=$((FAIL+1))
  fi
}

expect_allow() {
  local name="$1"; shift
  local code; code="$(http_code "$@")"
  if [ "$code" -ge 200 ] && [ "$code" -lt 400 ]; then
    printf '  %s✓ ALLOWED%s  %-38s → HTTP %s\n' "$G" "$N" "$name" "$code"
    PASS=$((PASS+1))
  else
    printf '  %s✗ BLOCKED%s  %-38s → HTTP %s  (false positive!)\n' "$R" "$N" "$name" "$code"
    FAIL=$((FAIL+1))
  fi
}

UA_BROWSER='Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36'
CURL_BROWSER=(-A "$UA_BROWSER" -H 'Accept: text/html' -H 'Accept-Language: en-US' -H 'Accept-Encoding: gzip')

printf '%s NullGate WAF — live attack demo %s\n' "$B" "$N"
printf 'target: %s\n' "$BASE"
hr

echo "${Y}» Benign traffic (must pass)${N}"
expect_allow "homepage"                  "${CURL_BROWSER[@]}" "$BASE/"
expect_allow "normal API query"          "${CURL_BROWSER[@]}" "$BASE/api/products?page=2&sort=price"
expect_allow "arabic search term"        "${CURL_BROWSER[@]}" --data-urlencode 'q=قميص أزرق' -G "$BASE/search"
hr

echo "${Y}» SQL Injection${N}"
expect_block "boolean tautology"         "${CURL_BROWSER[@]}" -G --data-urlencode "q=' OR '1'='1"           "$BASE/search"
expect_block "UNION SELECT"              "${CURL_BROWSER[@]}" -G --data-urlencode "id=1 UNION SELECT user,pass FROM users" "$BASE/item"
expect_block "time-based blind"          "${CURL_BROWSER[@]}" -G --data-urlencode "id=1;WAITFOR DELAY '0:0:5'" "$BASE/item"
expect_block "DROP TABLE"                "${CURL_BROWSER[@]}" -G --data-urlencode "q=1); DROP TABLE users;--"  "$BASE/search"
hr

echo "${Y}» Cross-Site Scripting${N}"
expect_block "script tag"                "${CURL_BROWSER[@]}" -G --data-urlencode "c=<script>alert(1)</script>" "$BASE/comment"
expect_block "img onerror"               "${CURL_BROWSER[@]}" -G --data-urlencode "c=<img src=x onerror=alert(1)>" "$BASE/comment"
expect_block "javascript URI"            "${CURL_BROWSER[@]}" -G --data-urlencode "url=javascript:alert(document.cookie)" "$BASE/go"
hr

echo "${Y}» Command Injection / RCE${N}"
expect_block "shell chaining"            "${CURL_BROWSER[@]}" -G --data-urlencode "host=127.0.0.1; cat /etc/passwd" "$BASE/ping"
expect_block "PHP system() sink"         "${CURL_BROWSER[@]}" -G --data-urlencode "x=system('id')"          "$BASE/run"
hr

echo "${Y}» Path Traversal / LFI${N}"
expect_block "dotdot traversal"          "${CURL_BROWSER[@]}" "$BASE/files?f=../../../../etc/passwd"
expect_block "encoded traversal"         "${CURL_BROWSER[@]}" "$BASE/files?f=..%2f..%2f..%2fetc%2fpasswd"
expect_block "php filter wrapper"        "${CURL_BROWSER[@]}" -G --data-urlencode "f=php://filter/convert.base64-encode/resource=index" "$BASE/files"
hr

echo "${Y}» SSRF${N}"
expect_block "cloud metadata"            "${CURL_BROWSER[@]}" -G --data-urlencode "url=http://169.254.169.254/latest/meta-data/" "$BASE/fetch"
expect_block "loopback target"           "${CURL_BROWSER[@]}" -G --data-urlencode "url=http://127.0.0.1:22/"   "$BASE/fetch"
hr

echo "${Y}» Other classes${N}"
expect_block "XXE doctype"               "${CURL_BROWSER[@]}" -H 'Content-Type: application/xml' \
                                          --data '<!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]><x>&e;</x>' "$BASE/xml"
expect_block "SSTI gadget"               "${CURL_BROWSER[@]}" -G --data-urlencode "name={{7*7}}{{config.__class__}}" "$BASE/hello"
expect_block "prototype pollution"       "${CURL_BROWSER[@]}" -H 'Content-Type: application/json' \
                                          --data '{"__proto__":{"admin":true}}' "$BASE/merge"
hr

echo "${Y}» Recon / scanners${N}"
expect_block "sqlmap user-agent"         -A "sqlmap/1.7#stable" "$BASE/"
hr

TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then
  printf '%s✅ ALL %d CHECKS PASSED%s\n' "$G" "$TOTAL" "$N"
  exit 0
else
  printf '%s❌ %d/%d checks failed%s\n' "$R" "$FAIL" "$TOTAL" "$N"
  exit 1
fi
