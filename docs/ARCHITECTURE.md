# NullGate WAF — Architecture / معمارية النظام

This document describes how a request flows through NullGate and why the
pipeline is ordered the way it is. For usage, see the top-level `README.md`.

---

## 1. Design principles / مبادئ التصميم

| Principle | What it means in NullGate |
|-----------|---------------------------|
| **Fail-closed** | Any subsystem error denies the request rather than forwarding it. A bug never becomes a bypass. |
| **Defense in depth** | Twelve independent layers. Evading one (e.g. encoding around a rule) still has to survive the decoder, behavioral tracker, and rate limiter. |
| **Cheapest check first** | Structural/protocol checks (nanoseconds) run before deep payload decoding (microseconds), so hostile traffic is dropped early. |
| **Never leak internals** | Every block returns a generic body. The matched rule, score, and category go to the logs, never to the client. |
| **Bounded work per request** | Decode depth, payload size, decompression output, and JSON depth are all capped, so no crafted input can force unbounded CPU or memory. |

---

## 2. Request pipeline / مسار الطلب

Every request passes through this ordered chain in `Engine.ServeHTTP`
(`internal/waf/engine.go`). The first layer to object wins; the request is
denied and nothing downstream runs.

```
 client
   │
   ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 0. Client IP resolution   — trust XFF only from configured proxies    │
├──────────────────────────────────────────────────────────────────────┤
│ 1. Protocol guard         — method, URL/header limits, smuggling,     │
│                             null/control bytes, content-type          │
│ 2. Blocklist              — shared store fed by honeypot + behavior   │
│ 3. Honeypot               — /.env, /wp-login.php … instant block      │
│ 4. Rate limiter           — token bucket → Allow / PoW / Block        │
│ 5. Proof-of-Work          — HMAC-bound challenge under attack         │
│ 6. Bot mitigation         — scanner UAs, empty UA, impoverished hdrs  │
│ 7. Deep inspection        — decode → 64 rules over path/query/hdr/body│
│ 8. Behavioral analysis    — patterns over time (scanner fingerprint)  │
│ 9. CSRF                   — double-submit, single-use tokens          │
│ 10. Tarpit                — escalating delay for persistent floods    │
└──────────────────────────────────────────────────────────────────────┘
   │  (all layers passed)
   ▼
 header sanitization ► reverse proxy ► backend
   ▲
   │ response hardening (security headers, server/version stripping)
```

---

## 3. The detection core / نواة الكشف

Deep inspection (layer 7) is where most attacks die. It has two halves:

### 3.1 Multi-layer decoder — `decoding.go`

WAF bypasses overwhelmingly rely on encoding. The decoder repeatedly applies
every enabled transform until the string stabilizes or `max_decode_depth` is
reached:

1. URL percent-decoding (handles double-encoding: `%253C` → `%3C` → `<`)
2. HTML entities — named, decimal, and hex (`&lt;`, `&#60;`, `&#x3c;`)
3. Unicode / hex escapes (`<`, `\x3c`)
4. Base64 (gated — only decodes plausible tokens, never mangles prose)
5. Hex blobs (`0x...`)
6. SQL inline-comment stripping (`un/**/ion` → `union`)
7. Whitespace collapse (`union     select` → `union select`)

Both the **raw** and **fully-decoded** forms are scanned (`Decoder.Variants`),
because normalization can hide a payload that was malicious in its raw form.

### 3.2 Rule engine — `rules.go`

64 signatures across 12 classes: SQLi, XSS, RCE, LFI, SSRF, XXE, SSTI, NoSQLi,
deserialization, prototype pollution, CRLF, and scanner recon. Each rule carries:

- a **severity** (`low/medium/high/critical`) → an **anomaly score** (1/3/5/10)
- a **paranoia level** (1–4) so operators trade coverage for false-positive rate

The blocking decision (`ScanResult.Blocked`) is OWASP-CRS-style:

```
if any critical rule matched            → block
if any rule's severity is "block" in    → block
   rule_actions (default: critical,high)
else if cumulative anomaly ≥ threshold  → block   (low-and-slow detection)
else                                    → allow
```

This is why a single decisive attack blocks instantly, while a spray of
individually-weak signals still accumulates into a block.

---

## 4. Anti-abuse subsystems / أنظمة مكافحة الإساءة

| Subsystem | File | Role |
|-----------|------|------|
| Rate limiter | `ratelimit.go` | Per-IP token bucket; graduated Allow → PoW → Block. |
| Proof-of-Work | `pow.go` | Stateless HMAC-SHA256 challenge bound to `IP‖timestamp`; bit-level difficulty; TTL-bounded so solutions can't be replayed. |
| Bot detector | `bot.go` | Scanner/tool UA signatures, empty UA, scripted-client header fingerprints. |
| Honeypot | `honeypot.go` | Known-probe paths; one hit → shared blocklist. |
| Behavioral tracker | `behavior.go` | Path/method spread, consecutive blocks → scanner escalation. |
| Tarpit | `honeypot.go` | Escalating latency that ties up an attacker's own connections. |
| Blocklist | `blocklist.go` | Shared, TTL-expiring, IP+CIDR; never persisted (NAT-friendly). |

---

## 5. Trust boundaries / حدود الثقة

- **Client IP** (`ip.go`) is the anchor of every per-client control. `X-Forwarded-For`
  is honored **only** when the direct peer is a configured trusted proxy, and the
  chain is walked right-to-left skipping trusted hops. Overlong chains fall back to
  the peer. This closes the "spoof XFF to evade rate limits/blocklist" hole.
- **Forwarded headers** the client supplied are stripped before proxying
  (`sanitizeForwardHeaders`), so the backend can't be lied to about origin.
- **Admin surface** runs on a **separate socket** (`admin.go`) — the operational
  interface never shares the public attack surface, and defaults to localhost-only.

---

## 6. Observability / المراقبة

- **Prometheus metrics** at `/metrics`: totals, blocks by layer, category counts,
  latency, proxy errors, tracked clients.
- **Structured logs** (JSON or text) with per-block layer/reason/category.
- **Live dashboard** on the admin socket, auto-refreshing.
- **Self-test** (`--selftest`) runs a 51-case labeled corpus and reports
  detection / false-positive rate — the competition correctness gate.
