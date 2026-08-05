<div align="center">

# 🛡 NullGate WAF

**A fail-closed, multi-layer Web Application Firewall & reverse proxy, built in Go.**
**جدار حماية تطبيقات ويب متعدد الطبقات مع بروكسي عكسي، مبني بلغة Go.**

`v2.0.0` — 64 detection rules · 12 attack classes · 10+ defense layers · 100% self-test detection, 0% false positives

</div>

---

## ✨ What makes it different / ما الذي يميزه

NullGate is a full-stack WAF, not a signature file. It combines an OWASP-CRS-style
detection engine with rate limiting, proof-of-work, bot mitigation, a honeypot,
behavioral analysis, a tarpit, CSRF protection, and response hardening — all in one
static binary with zero runtime dependencies.

| Layer / الطبقة | What it stops / ماذا توقف |
|---|---|
| Protocol guard | Request smuggling, oversized/malformed requests, null & control bytes |
| Multi-layer decoder | Encoded/obfuscated payloads: double-URL, HTML entities, unicode, base64, hex, SQL comments, whitespace |
| 64-rule detection engine | SQLi, XSS, RCE, LFI, SSRF, XXE, SSTI, NoSQLi, deserialization, prototype pollution, CRLF, scanners |
| Rate limiter | Token-bucket, graduated Allow → PoW → Block |
| Proof-of-Work | HMAC-bound, replay-safe client cost under attack |
| Bot detector | sqlmap/nikto/curl/requests, empty-UA, scripted fingerprints |
| Honeypot | `/wp-login.php`, `/.env`, `/.git/config` … one probe → blocked |
| Behavioral analysis | Scanner patterns *over time*, shared blocklist escalation |
| CSRF | Double-submit, single-use tokens |
| Tarpit | Escalating latency that ties up attackers' connections |
| Response hardening | Security headers, server/version stripping, cookie redaction |

**Why it wins on correctness / لماذا يفوز في الدقة:**
- **Fail-closed** — any error denies, never forwards.
- **No information leak** — every block returns a generic body; matched rules go only to logs.
- **Bounded work** — decode depth, payload size, and decompression all capped (no decode-bomb DoS).
- **XFF spoofing-proof** — forwarding headers trusted only from configured proxies.

---

## 🚀 Quick start / البدء السريع

### Requirements / المتطلبات
- Go **1.21+** (1.22+ recommended)
- The demo backend runs on `:3000`; the WAF listens on `:8080`

### One command: build, run, attack demo / أمر واحد: بناء وتشغيل وعرض هجمات

```bash
bash scripts/quickstart.sh
```

This builds NullGate, starts the vulnerable demo backend, starts the WAF in front
of it, and fires a battery of real attacks at it — you'll watch each one get blocked
while benign traffic passes.

### Manual / يدوي

```bash
# 1. Build
go build -o nullgate ./cmd/nullgate

# 2. Start the demo backend (intentionally vulnerable — only ever behind the WAF)
go run ./examples/vulnerable-backend

# 3. Start the WAF in front of it
./nullgate --config configs/config.yaml
# WAF:  http://127.0.0.1:8080  (default)
```

### Verify / تحقق

```bash
# The correctness gate: 51-case labeled corpus
./nullgate --selftest
# → ✅ ALL CHECKS PASSED — 100% detection, 0% false positives.

# See the effective configuration after defaults & clamping
./nullgate --show-config

# Generate a strong HMAC secret for pow.hmac_secret
./nullgate --gen-secret
```

---

---

## 🎛 Control Panel / لوحة التحكم

NullGate includes a **professional, self-contained live control panel** with session-based authentication that gives you real-time visibility and operational control over the WAF — KPIs, live traffic, attack categories, defense-layer breakdowns, security events, the effective posture, **plus live control**: block/unblock IPs, retune paranoia on the fly, view the blocklist, and scan your own site's protection. The dashboard is bilingual (English/Arabic + RTL), themeable (dark/light), and updates every 2 seconds. It runs on a separate localhost socket, isolated from the public attack surface.

### Launch it / تشغيلها

**One command:**

```bash
# Unix/macOS/Linux
make dashboard
# or: bash scripts/dashboard.sh

# Windows
scripts\dashboard.cmd
```

This builds NullGate + the demo backend, starts the WAF with `--admin` (admin socket enabled on `127.0.0.1:9091`), and opens the control panel in your browser.

Alternatively, enable it manually:

```bash
./nullgate --config configs/config.yaml --admin
# then open http://127.0.0.1:9091/ in a browser
```

The `--admin` flag overrides `admin.enabled` in the config and **always** keeps `require_localhost: true` — the dashboard is never exposed to the network without explicit configuration.

### Secure Login / تسجيل الدخول الآمن

When `admin.password_hash` is configured, the dashboard requires authentication. Set it in `configs/config.yaml`:

```yaml
admin:
  enabled: true
  username: "admin"
  password_hash: "sha256:abc123..."  # Generate: echo -n "your-password" | sha256sum
  session_ttl_min: 720  # 12 hours
```

Session tokens are stored in-memory only (restart = re-auth) and use HttpOnly/SameSite cookies for CSRF protection.

### What's inside / ما بالداخل

**Monitoring:**
- **9 live KPI cards:** total/allowed/blocked requests, block rate, latency, uptime, tracked clients, blocklist entries, active rules.
- **Live traffic chart:** rolling canvas chart, allowed/blocked split.
- **Attack categories bar chart:** top 10 attack classes by count.
- **Defense layers grid:** 11 layers with per-layer block counts + enabled/disabled state.
- **Security posture:** paranoia level, anomaly threshold, TLS status, fail-closed indicator, listener + backend addresses.
- **Security events table:** 100 most-recent events, filterable by level (warn/error) and search.

**Live Control (when `read_only: false`):**
- **Block IP:** Add an IP or CIDR to the blocklist with custom duration.
- **Unblock IP:** Remove an entry from the blocklist.
- **Set Paranoia:** Retune the rule-engine paranoia level (1-4) live, without restarting the WAF.
- **View Blocklist:** See all currently blocked IPs with expiry times and remaining seconds.

**Protection Scanner:**
- **Test Your Own Site:** Run the detection corpus against your protected site to measure how many attacks the WAF blocks. Ownership verification required (DNS TXT or meta tag).
- **Engine-Only Mode:** Measure raw detection coverage without network traffic.

**UI Features:**
- **Bilingual + RTL:** toggle English ↔ Arabic; RTL layout auto-switches.
- **Theme switcher:** dark/light mode, persisted to localStorage.
- **Configurable refresh:** 1s/2s/5s/10s interval; pause/resume button.

Everything is **self-contained** — zero external CDNs, zero runtime dependencies. The dashboard HTML is embedded at compile time.

### API endpoints / نقاط النهاية

All served on the admin socket (`127.0.0.1:9091` by default):

| Endpoint | Method | Description |
|---|---|---|
| `/` | GET | The live control panel (HTML) |
| `/login` | GET | Login page (when password auth enabled) |
| `/api/login` | POST | Authenticate and receive session cookie |
| `/api/logout` | POST | Destroy session |
| `/api/overview` | GET | Combined snapshot: info + metrics + blocklist + events |
| `/api/status` | GET | WAF status summary: version, uptime, blocklist, rules |
| `/api/metrics` | GET | Full metrics snapshot (JSON) |
| `/api/events` | GET | Recent security events (JSON) |
| `/api/info` | GET | Non-secret posture (no secrets leaked) |
| `/api/admin/block-ip` | POST | Manually block an IP/CIDR (requires auth + !read_only) |
| `/api/admin/unblock-ip` | POST | Remove from blocklist (requires auth + !read_only) |
| `/api/admin/set-paranoia` | POST | Change paranoia level 1-4 live (requires auth + !read_only) |
| `/api/admin/blocklist` | GET | List all blocklist entries with expiry |
| `/api/admin/scan` | POST | Run protection scan (engine or live mode) |
| `/healthz` | GET | Liveness probe |

The dashboard calls `/api/overview` every refresh; the other endpoints are available for external monitoring/alerting integrations.

---

## 📐 CLI reference / مرجع الأوامر

| Flag / العلم | Purpose / الغرض |
|---|---|
| `--config PATH` | Path to YAML config (default `configs/config.yaml`) |
| `--admin` | Force-enable the live control panel on `127.0.0.1:9091` (loopback-only) |
| `--auto-tls` | Enable automatic HTTPS via Let's Encrypt (requires `--domain`) |
| `--domain LIST` | Comma-separated domains for auto-TLS (e.g. `example.com,www.example.com`) |
| `--version` | Print version |
| `--selftest` | Run detection self-test against a labeled corpus, exit non-zero on any miss |
| `--show-config` | Dump effective config after defaults and clamping |
| `--gen-secret` | Generate a random HMAC secret for PoW |

---

## ⚙️ Configuration / الإعداد

All tuning lives in one file: **`configs/config.yaml`** — a fully-commented
reference with production-safe defaults. Every subsystem has a built-in safe
default in code, so even a minimal config yields a secure posture.

Highlights / أبرز النقاط:

```yaml
rules:
  paranoia_level: 2        # 1 (fewest FPs) … 4 (most aggressive)
  anomaly_threshold: 8     # cumulative medium/low score that triggers a block
  rule_actions:
    critical: block        # decisive severities block immediately…
    high: block
    medium: log            # …medium/low accumulate toward the threshold
    low: log

decoding:
  max_decode_depth: 5      # bounded — defeats decode-bomb DoS
  enable_base64_scan: true
  enable_hex_scan: true
  enable_unicode_scan: true

rate_limit:
  requests_per_second: 20
  burst_size: 60
  escalation_after: 20     # violations → PoW challenge
  block_after: 40          # violations → hard block 300s

pow:
  enabled: false           # enable under attack; requires --gen-secret first
  difficulty: 4            # leading zero bits of SHA-256
```

Run `./nullgate --show-config` to see exactly what your file resolves to.

---

## 🐳 Docker / نشر بالحاويات

```bash
# Build the WAF image (multi-stage, scratch runtime, ~11 MB)
make docker

# Full demo stack: WAF + vulnerable backend, backend not exposed publicly
docker compose up --build
# WAF: http://localhost:8080 · admin: http://localhost:9091
```

The compose stack runs the container `read_only`, drops all capabilities, and
never publishes the backend port — the demo backend is reachable **only** through
the WAF.

---

## 🧪 Testing / الاختبارات

69 tests covering every subsystem plus end-to-end pipeline integration:

```bash
make test        # go test ./...
make vet         # go vet ./...
make selftest    # 100%-detection correctness gate
make bench       # rule-engine benchmark
make cover       # coverage report (currently ~58% statements)
```

Key suites / أبرز المجموعات:

| Suite | Proves / يثبت |
|---|---|
| `rules_test.go` | Full self-test corpus passes; paranoia is monotonic; classic attacks block; benign inputs pass |
| `decoding_test.go` | Every decode layer; decode-depth bound enforced |
| `pow_test.go` | Round-trip; wrong-nonce/forged/expired rejected; **challenge bound to IP** |
| `ip_test.go` | **XFF spoofing rejected**; trusted-proxy chain walk; overlong-chain fallback |
| `protocol_test.go` | Smuggling (CL/TE conflict, length mismatch); null/control bytes; method & content-type policy |
| `ratelimit_test.go` | Burst, refill, graduated escalation, per-client isolation |
| `csrf_test.go` | Single-use tokens; required only for state-changing; extraction sources |
| `engine_test.go` | End-to-end: benign forwards, attacks block at the right layer, CSRF flow |

---

## 📁 Layout / هيكل المشروع

```
NullGate/
├── cmd/nullgate/            # CLI entry point
├── internal/waf/            # the whole WAF
│   ├── engine.go            # request pipeline (order matters)
│   ├── rules.go             # 64-rule detection engine (paranoia-tiered)
│   ├── decoding.go          # multi-layer decoder (bounded)
│   ├── protocol.go          # request-smuggling & hygiene guard
│   ├── ratelimit.go         # token-bucket with graduated response
│   ├── pow.go               # stateless HMAC proof-of-work
│   ├── ip.go                # XFF-spoofing-proof client IP
│   ├── bot.go · honeypot.go · behavior.go · tarpit.go · blocklist.go
│   ├── csrf.go · response.go · tls.go · admin.go · metrics.go
│   ├── selftest.go          # 51-case correctness corpus
│   └── *_test.go            # 69 tests
├── configs/config.yaml      # fully-commented configuration
├── examples/                # demo backends (never expose directly!)
├── scripts/                 # quickstart + live attack demo
├── docs/ARCHITECTURE.md     # pipeline & trust-boundary deep dive
├── deploy/                  # operational extras
├── Dockerfile · docker-compose.yml · Makefile
└── .github/workflows/ci.yml # CI: fmt → vet → build → test → selftest
```

---

## 🔍 Want a deeper tour? / تريد الغوص أعمق؟

- **`docs/ARCHITECTURE.md`** — the request pipeline diagram, why layers are ordered
  as they are, the scoring model, and every trust boundary.
- **`configs/config.yaml`** — every option explained inline.
- **`scripts/demo-attacks.sh`** — the live demo, payload by payload.

---

## ⚠️ Production notes / ملاحظات للإنتاج

1. **Change the default secrets.** Before enabling `pow`, set `pow.hmac_secret` to output of `./nullgate --gen-secret`. Generate admin password hash: `echo -n "your-password" | sha256sum` then prefix with `sha256:`.

2. **Put the admin socket on localhost** (`admin.listen_addr: 127.0.0.1:9091`) and **always** set a strong `password_hash`. Access via SSH tunnel from your local machine.

3. **Enable HTTPS:**
   - **Cloud deployment (recommended):** Use `--auto-tls --domain yourdomain.com` for automatic Let's Encrypt certificates. See `deploy/DEPLOY.md`.
   - **Manual TLS:** Set `server.enable_tls: true` with cert/key paths.
   - After HTTPS is working, enable `response.add_hsts: true` for HSTS.

4. **`trusted_proxies`** must list only proxies you control. Empty means internet-facing: `X-Forwarded-For` is ignored entirely.

5. The demo backend is **deliberately vulnerable** — never publish it directly.

6. **Live control:** Set `admin.read_only: false` to enable IP blocking and paranoia tuning from the dashboard.

7. **Protection scanner:** The scan feature requires ownership verification (DNS TXT or meta tag) before testing a live target. Use engine mode for offline testing.

---

## 🚀 Cloud Deployment / النشر السحابي

Deploy to DigitalOcean, AWS, Hetzner, Vultr, or any VPS with **automatic HTTPS**:

```bash
# One command on your server:
./nullgate --config configs/config-cloud.yaml --auto-tls --domain yourdomain.com

# Or via config:
server:
  auto_tls: true
  auto_tls_domains:
    - "yourdomain.com"
    - "www.yourdomain.com"
  auto_tls_email: "admin@yourdomain.com"
```

Let's Encrypt will provision your certificate automatically. The WAF handles HTTP→HTTPS redirect and ACME challenges on port 80, serves HTTPS on 443.

**See `deploy/DEPLOY.md` for:**
- Step-by-step VPS setup (5 minutes)
- Systemd service configuration
- Docker Compose for production
- Troubleshooting certificate provisioning
- Security hardening checklist
- Platform-specific notes (DigitalOcean, AWS, Hetzner)

---

## 🆕 What's New in v2.0 / الجديد في النسخة 2.0

- ✅ **Public landing page** served at `/` on the proxy when `server.serve_landing: true`
- ✅ **Session-based admin authentication** with SHA-256 password hashing
- ✅ **Live control endpoints**: block/unblock IPs, retune paranoia level on the fly, view blocklist
- ✅ **Protection scanner**: test your own site's defenses with ownership verification
- ✅ **Automatic HTTPS** via Let's Encrypt with `--auto-tls` flag
- ✅ **Cloud deployment guide** with Docker, systemd, and VPS quickstart
- ✅ **Bilingual admin login page** (English/Arabic + RTL)
- ✅ **Enhanced security**: session cookies with HttpOnly/SameSite, constant-time auth
