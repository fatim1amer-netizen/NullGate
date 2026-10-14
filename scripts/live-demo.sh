#!/usr/bin/env bash
# NullGate WAF — Live Attack Demonstration
# Fires real attack payloads at the WAF, shows live blocking + metrics updates.
# Perfect for graduation defense presentations.

set -e

WAF_URL="${WAF_URL:-http://127.0.0.1:8080}"
ADMIN_URL="${ADMIN_URL:-http://127.0.0.1:9091}"
ATTACK_DELAY="${ATTACK_DELAY:-2}"  # seconds between attacks

C_RED='\033[0;31m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[1;33m'
C_BLUE='\033[0;34m'
C_CYAN='\033[0;36m'
C_BOLD='\033[1m'
C_RESET='\033[0m'

echo -e "${C_BOLD}${C_CYAN}"
cat << 'EOF'
╔═══════════════════════════════════════════════════════════════╗
║                   NullGate WAF — Live Demo                    ║
║           Real-time Attack Detection & Blocking               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${C_RESET}"

echo -e "${C_BOLD}Setup:${C_RESET}"
echo -e "  ${C_CYAN}►${C_RESET} WAF:          ${C_BLUE}${WAF_URL}${C_RESET}"
echo -e "  ${C_CYAN}►${C_RESET} Admin Panel:  ${C_BLUE}${ADMIN_URL}${C_RESET}"
echo -e "  ${C_CYAN}►${C_RESET} Attack delay: ${C_YELLOW}${ATTACK_DELAY}s${C_RESET}"
echo ""
echo -e "${C_BOLD}Instructions:${C_RESET}"
echo -e "  1. Open the admin dashboard: ${C_BLUE}${ADMIN_URL}${C_RESET}"
echo -e "  2. Watch the KPIs, traffic chart, and attack categories update live"
echo -e "  3. Observe security events appearing in real-time"
echo ""
read -p "Press ENTER to start the attack simulation..."
echo ""

# Attack corpus — matches internal/waf/scan.go
declare -a ATTACKS=(
  "sqli|' OR '1'='1' --|SQL Injection (authentication bypass)"
  "sqli|1 UNION SELECT username,password FROM users|SQL Injection (data exfiltration)"
  "xss|<script>document.cookie</script>|Cross-Site Scripting (cookie theft)"
  "xss|<img src=x onerror=alert(1)>|XSS (DOM-based)"
  "rce|;cat /etc/passwd|Remote Code Execution (command injection)"
  "rce|\$(whoami)|RCE (shell substitution)"
  "lfi|../../../../etc/passwd|Local File Inclusion"
  "lfi|php://filter/convert.base64-encode/resource=index|LFI (PHP wrapper)"
  "ssrf|http://169.254.169.254/latest/meta-data/|Server-Side Request Forgery (AWS metadata)"
  "ssrf|http://127.0.0.1:8080/admin|SSRF (internal service scan)"
  "xxe|<!DOCTYPE foo [<!ENTITY x SYSTEM \"file:///etc/passwd\">]>|XML External Entity"
  "ssti|{{7*7}}|Server-Side Template Injection"
  "nosqli|{\"user\":{\"\$ne\":null}}|NoSQL Injection (MongoDB)"
  "deserialize|rO0ABXNy|Java Deserialization"
  "proto_pollution|{\"__proto__\":{\"admin\":true}}|Prototype Pollution"
  "crlf|%0d%0aSet-Cookie: injected=1|CRLF Injection (header manipulation)"
  "scanner|sqlmap/1.7|Scanner Detection (automated tool)"
)

TOTAL=${#ATTACKS[@]}
BLOCKED=0
ALLOWED=0

echo -e "${C_BOLD}${C_YELLOW}▶ Starting attack sequence (${TOTAL} attacks)...${C_RESET}"
echo ""

for i in "${!ATTACKS[@]}"; do
  IFS='|' read -r cat payload desc <<< "${ATTACKS[$i]}"
  NUM=$((i + 1))

  echo -e "${C_BOLD}Attack ${NUM}/${TOTAL}${C_RESET} — ${C_CYAN}${desc}${C_RESET}"
  echo -e "  Category: ${cat}"
  echo -e "  Payload:  ${C_YELLOW}${payload}${C_RESET}"

  # URL-encode the payload
  encoded=$(printf '%s' "$payload" | jq -sRr @uri)

  # Fire the attack
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "User-Agent: Mozilla/5.0" \
    -m 5 \
    "${WAF_URL}/?q=${encoded}" 2>/dev/null || echo "000")

  if [[ "$HTTP_CODE" == "403" ]] || [[ "$HTTP_CODE" == "406" ]] || [[ "$HTTP_CODE" == "400" ]]; then
    echo -e "  Result:   ${C_RED}✖ BLOCKED${C_RESET} (HTTP ${HTTP_CODE})"
    BLOCKED=$((BLOCKED + 1))
  elif [[ "$HTTP_CODE" == "200" ]] || [[ "$HTTP_CODE" == "301" ]] || [[ "$HTTP_CODE" == "302" ]]; then
    echo -e "  Result:   ${C_GREEN}✔ ALLOWED${C_RESET} (HTTP ${HTTP_CODE}) ${C_RED}[VULNERABILITY]${C_RESET}"
    ALLOWED=$((ALLOWED + 1))
  else
    echo -e "  Result:   ${C_YELLOW}? UNKNOWN${C_RESET} (HTTP ${HTTP_CODE})"
  fi

  echo ""

  # Pause between attacks for dramatic effect + dashboard refresh
  if [[ $NUM -lt $TOTAL ]]; then
    sleep "$ATTACK_DELAY"
  fi
done

echo -e "${C_BOLD}${C_CYAN}════════════════════════════════════════════════════════════════${C_RESET}"
echo -e "${C_BOLD}Demo Complete!${C_RESET}"
echo ""
echo -e "${C_BOLD}Summary:${C_RESET}"
echo -e "  ${C_RED}Blocked:${C_RESET}  ${C_BOLD}${BLOCKED}${C_RESET} / ${TOTAL}"
echo -e "  ${C_GREEN}Allowed:${C_RESET}  ${C_BOLD}${ALLOWED}${C_RESET} / ${TOTAL}"

BLOCK_RATE=$(awk "BEGIN {printf \"%.1f\", ($BLOCKED/$TOTAL)*100}")
echo -e "  ${C_CYAN}Detection Rate:${C_RESET} ${C_BOLD}${BLOCK_RATE}%${C_RESET}"
echo ""

if [[ $BLOCKED -eq $TOTAL ]]; then
  echo -e "${C_GREEN}${C_BOLD}✅ PERFECT SCORE — All attacks blocked!${C_RESET}"
elif [[ $BLOCKED -ge $((TOTAL * 90 / 100)) ]]; then
  echo -e "${C_YELLOW}${C_BOLD}⚠️  GOOD — Review missed samples and tune rules against the labeled corpus.${C_RESET}"
else
  echo -e "${C_RED}${C_BOLD}❌ FAILED — Significant attacks passed through${C_RESET}"
fi

echo ""
echo -e "Check the ${C_BLUE}admin dashboard${C_RESET} to see:"
echo -e "  • Live traffic chart showing the attack burst"
echo -e "  • Attack categories breakdown (SQLi, XSS, RCE, ...)"
echo -e "  • Security events log with blocked payloads"
echo -e "  • Updated block rate and metrics"
echo ""
