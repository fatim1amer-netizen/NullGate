package waf

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

//go:embed landing.html
var landingHTML string

// Engine is the NullGate request pipeline. Every request flows through an
// ordered chain of defenses, cheapest and most decisive first, so that
// obviously-hostile traffic is rejected before expensive deep inspection runs:
//
//	protocol → blocklist → rate limit → PoW → bot → deep inspection
//	→ CSRF → tarpit → proxy(+response hardening)
//
// The engine is fail-closed: any subsystem error results in the request being
// denied rather than silently forwarded.
type Engine struct {
	config *Config

	proxy       *httputil.ReverseProxy
	ipResolver  *IPResolver
	protocol    *ProtocolGuard
	rateLimiter *RateLimiter
	pow         *PoWValidator
	bot         *BotDetector
	behavior    *BehaviorTracker
	tarpit      *Tarpit
	csrf        *CSRFManager
	response    *ResponseGuard
	rules       *RuleEngine
	decoder     *Decoder
	blocklist   *Blocklist
	logger      *Logger
	metrics     *Metrics
}

// NewEngine constructs and wires all subsystems from a validated config.
func NewEngine(cfg *Config) (*Engine, error) {
	targetURL, err := url.Parse(cfg.Server.TargetBackend)
	if err != nil {
		return nil, fmt.Errorf("invalid target backend URL: %w", err)
	}

	logger := NewLogger(cfg.Logging)
	metrics := NewMetrics()
	blocklist := NewBlocklist()

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = newUpstreamTransport()
	// Preserve the original director but add header sanitization so a client
	// cannot inject forwarding/hop-by-hop headers to the backend.
	origDirector := proxy.Director
	responseGuard := NewResponseGuard(cfg.Response)
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		if cfg.Server.UpstreamHost != "" {
			r.Host = cfg.Server.UpstreamHost
		}
		sanitizeForwardHeaders(r)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		responseGuard.Apply(resp.Header, resp.Request)
		metrics.RecordStatus(resp.StatusCode)
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		metrics.IncProxyError()
		logger.Error("proxy error", map[string]interface{}{
			"error": sanitizedProxyError(err),
			"host":  targetURL.Host,
			"path":  r.URL.Path,
		})
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	e := &Engine{
		config:      cfg,
		proxy:       proxy,
		ipResolver:  NewIPResolver(cfg.IP, cfg.Server.TrustedProxies),
		protocol:    NewProtocolGuard(cfg.Protocol),
		rateLimiter: NewRateLimiter(cfg.RateLimit),
		pow:         NewPoWValidator(cfg.PoW),
		bot:         NewBotDetector(cfg.Bot),
		blocklist:   blocklist,
		tarpit:      NewTarpit(cfg.Tarpit),
		csrf:        NewCSRFManager(cfg.CSRF),
		response:    responseGuard,
		rules:       NewRuleEngine(cfg.Rules.ParanoiaLevel, nil),
		decoder:     NewDecoder(cfg.Decoding),
		logger:      logger,
		metrics:     metrics,
	}
	e.behavior = NewBehaviorTracker(cfg.Behavior, blocklist)

	// Background maintenance.
	go blocklist.Cleanup(time.Minute)
	go e.refreshGauges()

	return e, nil
}

type upstreamTransport struct {
	primary  http.RoundTripper
	fallback http.RoundTripper
}

func newUpstreamTransport() *upstreamTransport {
	base := http.DefaultTransport.(*http.Transport)
	primary := base.Clone()
	primaryProtocols := new(http.Protocols)
	primaryProtocols.SetHTTP1(true)
	primaryProtocols.SetHTTP2(true)
	primary.Protocols = primaryProtocols

	fallback := base.Clone()
	fallback.ForceAttemptHTTP2 = false
	fallbackProtocols := new(http.Protocols)
	fallbackProtocols.SetHTTP1(true)
	fallback.Protocols = fallbackProtocols
	fallbackTLS := &tls.Config{}
	if base.TLSClientConfig != nil {
		fallbackTLS = base.TLSClientConfig.Clone()
	}
	fallbackTLS.NextProtos = []string{"http/1.1"}
	fallback.TLSClientConfig = fallbackTLS

	return &upstreamTransport{primary: primary, fallback: fallback}
}

func (t *upstreamTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.primary.RoundTrip(r)
	if err == nil || !safeToRetryWithoutBody(r) || !isNegotiatedProtocolError(err) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return resp, err
	}

	resp, fallbackErr := t.fallback.RoundTrip(r)
	if fallbackErr != nil {
		return nil, fmt.Errorf("HTTP/1.1 fallback after negotiated transport failure: %w", fallbackErr)
	}
	return resp, nil
}

func isNegotiatedProtocolError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "bad record mac") ||
		strings.Contains(message, "http2") ||
		strings.Contains(message, "http/2") ||
		strings.Contains(message, "protocol error")
}

func safeToRetryWithoutBody(r *http.Request) bool {
	if r.Body != nil && r.Body != http.NoBody {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func sanitizedProxyError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Sprintf("%s upstream request: %v", urlErr.Op, urlErr.Err)
	}
	return fmt.Sprintf("%T: %v", err, err)
}

// Logger/Metrics/Blocklist/Rules expose subsystems for the runner (admin, selftest).
func (e *Engine) Logger() *Logger       { return e.logger }
func (e *Engine) Metrics() *Metrics     { return e.metrics }
func (e *Engine) Blocklist() *Blocklist { return e.blocklist }
func (e *Engine) Rules() *RuleEngine    { return e.rules }
func (e *Engine) Config() *Config       { return e.config }

// BackendHealth reports a live probe of the configured upstream independently
// from NullGate's own process health.
type BackendHealth struct {
	State      string    `json:"state"`
	HTTPStatus int       `json:"http_status,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

func (e *Engine) ProbeBackend(ctx context.Context) BackendHealth {
	health := BackendHealth{State: "unknown", CheckedAt: time.Now().UTC()}
	target, err := url.Parse(e.config.Server.TargetBackend)
	if err != nil {
		return health
	}
	probe := func(method string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Host = e.config.Server.UpstreamHost
		req.Header.Set("User-Agent", "NullGate-Health-Check")
		req.Header.Set("Range", "bytes=0-0")
		return e.proxy.Transport.RoundTrip(req)
	}

	resp, err := probe(http.MethodHead)
	if err != nil {
		health.State = "unhealthy"
		return health
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		resp.Body.Close()
		resp, err = probe(http.MethodGet)
		if err != nil {
			health.State = "unhealthy"
			return health
		}
	}
	health.HTTPStatus = resp.StatusCode
	resp.Body.Close()
	if resp.StatusCode < http.StatusInternalServerError {
		health.State = "healthy"
	} else {
		health.State = "unhealthy"
	}
	return health
}

func (e *Engine) refreshGauges() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		clients := make(map[string]struct{})
		for _, ip := range e.rateLimiter.trackedClientIDs() {
			clients[ip] = struct{}{}
		}
		for _, ip := range e.behavior.trackedClientIDs() {
			clients[ip] = struct{}{}
		}
		e.metrics.SetTracked(int64(len(clients)))
	}
}

// ServeHTTP is the entry point for every request.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() { e.metrics.ObserveLatency(time.Since(start)) }()
	e.metrics.IncTotal()
	clientIP := e.ipResolver.ClientIP(r)

	e.logger.Debug("incoming request", map[string]interface{}{
		"method": r.Method, "path": r.URL.Path, "client_ip": clientIP,
	})

	// 1. Protocol hygiene (fail-closed on any structural violation).
	if err := e.protocol.Check(r); err != nil {
		pe, _ := err.(*ProtocolError)
		status := http.StatusBadRequest
		if pe != nil {
			status = pe.Status
		}
		e.metrics.IncProtocol()
		e.deny(w, r, clientIP, "protocol", err.Error(), status)
		return
	}

	// 2. Shared blocklist (behavioral escalation).
	if e.blocklist.Blocked(clientIP) {
		e.deny(w, r, clientIP, "blocklist", "client is on the blocklist", http.StatusForbidden)
		return
	}

	// 2b. Landing page — when enabled, NullGate serves its own marketing/home
	// page at "/" instead of proxying it to the backend. Only a bare GET "/"
	// is intercepted; every other path still flows through the proxy so the
	// protected application is otherwise untouched.
	if e.config.Server.ServeLanding && r.Method == http.MethodGet && r.URL.Path == "/" {
		e.metrics.IncAllowed()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		e.response.Apply(w.Header(), r)
		_, _ = io.WriteString(w, landingHTML)
		return
	}

	// 4. Rate limiting with graduated response.
	if e.config.RateLimit.Enabled {
		switch e.rateLimiter.Check(clientIP) {
		case ActionBlock:
			e.metrics.IncRateLimited()
			e.deny(w, r, clientIP, "rate_limit", "too many requests", http.StatusTooManyRequests)
			return
		case ActionRequirePoW:
			if e.config.PoW.Enabled {
				e.serveChallenge(w, r, clientIP)
				return
			}
		}
	}

	// 5. Proof-of-Work gate (when globally enabled).
	if e.config.PoW.Enabled && !e.pow.Validate(r, clientIP) {
		e.serveChallenge(w, r, clientIP)
		return
	}

	// 6. Bot mitigation.
	if verdict := e.bot.Inspect(r); verdict.IsBot {
		e.metrics.IncBot()
		e.deny(w, r, clientIP, "bot", verdict.Reason, http.StatusForbidden)
		return
	}

	// 7. Deep payload inspection (query, headers, path, body — all decoded).
	body, actualLen, err := e.readBody(r)
	if err != nil {
		e.deny(w, r, clientIP, "protocol", err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	if err := e.protocol.ValidateContentLengthMismatch(r, actualLen); err != nil {
		e.metrics.IncProtocol()
		e.deny(w, r, clientIP, "protocol", err.Error(), http.StatusBadRequest)
		return
	}
	scan, hit := e.inspect(r, body)
	if len(scan.Matches) > 0 {
		e.recordDetections(scan, clientIP, hit)
	}
	if hit {
		match := topMatch(scan)
		reason := fmt.Sprintf("%s (score %d)", scan.TopCategory(), scan.AnomalyScore)
		if match != nil {
			reason = fmt.Sprintf("%s: %s (score %d)", match.RuleID, match.Desc, scan.AnomalyScore)
		}
		e.deny(w, r, clientIP, "rules",
			reason,
			http.StatusForbidden)
		return
	}

	// 8. Behavioral analysis (patterns over time).
	if blocked, reason := e.behavior.Observe(r, clientIP); blocked {
		e.metrics.IncBehavior()
		e.deny(w, r, clientIP, "behavior", reason, http.StatusForbidden)
		return
	}

	// 9. CSRF for state-changing requests.
	if e.config.CSRF.Enabled && e.csrf.Required(r) {
		token := e.csrf.ExtractToken(r)
		if !e.csrf.ValidateToken(token) {
			e.metrics.IncCSRF()
			e.deny(w, r, clientIP, "csrf", "missing or invalid CSRF token", http.StatusForbidden)
			return
		}
	}

	// Issue a fresh CSRF cookie on safe requests so forms can submit later.
	if e.config.CSRF.Enabled && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if _, err := r.Cookie("csrf_token"); err != nil {
			if tok := e.csrf.GenerateToken(); tok != "" {
				http.SetCookie(w, &http.Cookie{
					Name: "csrf_token", Value: tok, Path: "/",
					HttpOnly: false, SameSite: http.SameSiteStrictMode, MaxAge: 1800,
				})
			}
		}
	}

	// 10. Tarpit persistent high-volume clients (adds latency, not a block).
	if delay := e.tarpit.Delay(clientIP); delay > 0 {
		time.Sleep(delay)
	}

	// Passed all gates — forward to the backend.
	e.behavior.RecordAllow(clientIP)
	e.metrics.IncAllowed()
	e.proxy.ServeHTTP(w, r)

	if e.config.Logging.EnableAccessLog {
		e.logger.Info("request allowed", map[string]interface{}{
			"method": r.Method, "path": r.URL.Path, "client_ip": clientIP,
			"duration_ms": time.Since(start).Milliseconds(),
		})
	}
}

func (e *Engine) recordDetections(scan ScanResult, clientIP string, blocked bool) {
	for _, match := range scan.Matches {
		e.metrics.RecordCategory(match.Category, match.RuleID)
		if !blocked {
			e.logger.Warn("rule matched", map[string]interface{}{
				"layer": "rules", "rule_id": match.RuleID, "category": match.Category,
				"severity": match.Severity.String(), "description": match.Desc,
				"score": scan.AnomalyScore, "client_ip": clientIP,
			})
		}
	}
}

// deny writes a rejection, records metrics, and feeds the behavioral escalator.
func (e *Engine) deny(w http.ResponseWriter, r *http.Request, clientIP, layer, reason string, status int) {
	e.metrics.IncBlocked()
	e.logger.Warn("request blocked", map[string]interface{}{
		"layer": layer, "reason": reason, "client_ip": clientIP,
		"method": r.Method, "path": r.URL.Path, "user_agent": r.UserAgent(),
	})
	if e.behavior.RecordBlock(clientIP) {
		e.logger.Warn("client escalated to blocklist", map[string]interface{}{"client_ip": clientIP})
	}
	// Generic, information-poor error body: never reveal which rule fired.
	http.Error(w, "Forbidden", statusOrDefault(status))
}

func statusOrDefault(s int) int {
	if s == 0 {
		return http.StatusForbidden
	}
	return s
}

// inspect scans every attacker-controllable field. Returns the aggregate scan
// and whether it warrants a block. Each value is scanned in both raw and
// fully-decoded form (see Decoder.Variants).
func (e *Engine) inspect(r *http.Request, body []byte) (ScanResult, bool) {
	var agg ScanResult
	seenRules := make(map[string]struct{})

	scan := func(value string, skipSSRF bool) bool {
		if value == "" {
			return false
		}
		for _, variant := range e.decoder.Variants(value) {
			var res ScanResult
			if skipSSRF {
				res = e.rules.ScanExceptCategories(variant, CatSSRF)
			} else {
				res = e.rules.Scan(variant)
			}
			for _, match := range res.Matches {
				if _, seen := seenRules[match.RuleID]; seen {
					continue
				}
				seenRules[match.RuleID] = struct{}{}
				agg.Matches = append(agg.Matches, match)
				agg.AnomalyScore += match.Severity.Score()
				if match.Severity == SevCritical {
					agg.HasCritical = true
				}
			}
			if agg.Blocked(&e.config.Rules) {
				return true
			}
		}
		return false
	}

	// Request path.
	if scan(r.URL.Path, false) {
		return agg, true
	}
	// Query parameters.
	for _, vals := range r.URL.Query() {
		for _, v := range vals {
			if scan(v, false) {
				return agg, true
			}
		}
	}
	// Headers (skip hop-by-hop noise; scan values only).
	for name, vals := range r.Header {
		if isSkippableHeader(name) {
			continue
		}
		skipSSRF := strings.EqualFold(name, "Referer") || strings.EqualFold(name, "Origin")
		for _, v := range vals {
			if scan(v, skipSSRF) {
				return agg, true
			}
		}
	}
	// Body — scanned raw plus structure-aware (JSON keys/values, multipart).
	if len(body) > 0 {
		if scan(string(body), false) {
			return agg, true
		}
		for _, s := range e.extractStructured(r, body) {
			if scan(s, false) {
				return agg, true
			}
		}
	}

	return agg, agg.Blocked(&e.config.Rules)
}

// readBody buffers the request body (respecting size limits and decompression)
// and restores it so the proxy can forward it unchanged. Returns the decoded
// bytes used for inspection and the actual consumed length.
func (e *Engine) readBody(r *http.Request) ([]byte, int64, error) {
	if r.Body == nil {
		return nil, 0, nil
	}
	// Chunked requests report ContentLength -1; we still inspect them, unlike
	// the original which skipped any request without a positive length.
	maxBody := int64(e.config.Decoding.MaxRequestSize)
	limited := io.LimitReader(r.Body, maxBody+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read body: %w", err)
	}
	actualLen := int64(len(raw))
	if actualLen > maxBody {
		return nil, actualLen, fmt.Errorf("request body exceeds maximum size")
	}

	// Restore the body for the proxy exactly as received.
	r.Body = io.NopCloser(bytes.NewReader(raw))

	// For inspection only, transparently decompress a bounded amount.
	inspectBytes := raw
	if enc := strings.ToLower(r.Header.Get("Content-Encoding")); enc != "" {
		if dec, ok := e.decompress(enc, raw); ok {
			inspectBytes = dec
		}
	}
	// Cap the slice actually scanned to MaxPayloadSize to bound CPU.
	if len(inspectBytes) > e.config.Decoding.MaxPayloadSize {
		inspectBytes = inspectBytes[:e.config.Decoding.MaxPayloadSize]
	}
	return inspectBytes, actualLen, nil
}

// decompress inflates gzip/deflate bodies for inspection, bounded by
// MaxPayloadSize to defeat decompression bombs.
func (e *Engine) decompress(encoding string, data []byte) ([]byte, bool) {
	limit := int64(e.config.Decoding.MaxPayloadSize)
	switch {
	case strings.Contains(encoding, "gzip") && e.config.Decoding.EnableGzipScan:
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, limit))
		if err != nil {
			return nil, false
		}
		return out, true
	case strings.Contains(encoding, "deflate") && e.config.Decoding.EnableDeflateScan:
		fr := flate.NewReader(bytes.NewReader(data))
		defer fr.Close()
		out, err := io.ReadAll(io.LimitReader(fr, limit))
		if err != nil {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

// extractStructured pulls individual field values out of JSON and multipart
// bodies so each is scanned in isolation (a payload split across JSON fields
// still trips the right rule).
func (e *Engine) extractStructured(r *http.Request, body []byte) []string {
	var out []string
	ct := r.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(ct)

	if e.config.Decoding.EnableJSONScan && strings.Contains(mediaType, "json") {
		var v interface{}
		if err := json.Unmarshal(body, &v); err == nil {
			collectJSONStrings(v, &out, 0)
		}
	}

	if e.config.Decoding.EnableMultipartScan && strings.HasPrefix(mediaType, "multipart/") {
		if boundary := params["boundary"]; boundary != "" {
			mr := multipart.NewReader(bytes.NewReader(body), boundary)
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				out = append(out, part.FormName(), part.FileName())
				val, _ := io.ReadAll(io.LimitReader(part, int64(e.config.Decoding.MaxPayloadSize)))
				out = append(out, string(val))
				part.Close()
			}
		}
	}
	return out
}

// collectJSONStrings recursively gathers all string keys and values from a
// decoded JSON tree, bounded in depth to avoid stack abuse.
func collectJSONStrings(v interface{}, out *[]string, depth int) {
	if depth > 32 {
		return
	}
	switch t := v.(type) {
	case string:
		*out = append(*out, t)
	case map[string]interface{}:
		for k, val := range t {
			*out = append(*out, k)
			collectJSONStrings(val, out, depth+1)
		}
	case []interface{}:
		for _, val := range t {
			collectJSONStrings(val, out, depth+1)
		}
	}
}

// serveChallenge returns an interstitial PoW page that the browser solves in JS.
func (e *Engine) serveChallenge(w http.ResponseWriter, r *http.Request, clientIP string) {
	e.metrics.IncPoW()
	challenge, difficulty := e.pow.GenerateChallenge(clientIP, 0)
	page := fmt.Sprintf(powChallengeHTML, challenge, difficulty)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(page))
}

// sanitizeForwardHeaders removes client-supplied forwarding and hop-by-hop
// headers before proxying, so a client cannot spoof its origin to the backend
// or smuggle connection-control directives.
func sanitizeForwardHeaders(r *http.Request) {
	for _, h := range []string{
		"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto",
		"X-Real-IP", "Forwarded", "X-Original-URL", "X-Rewrite-URL",
		"Proxy-Connection", "Keep-Alive", "TE", "Trailer", "Upgrade",
	} {
		r.Header.Del(h)
	}
	// Re-stamp a trustworthy forwarded chain for the backend.
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("Via", "1.1 nullgate")
}

func isSkippableHeader(name string) bool {
	switch strings.ToLower(name) {
	case "cookie", "accept-encoding", "connection", "content-length":
		return true
	}
	return false
}

func topMatch(sr ScanResult) *MatchResult {
	best := SevLow
	var bestMatch *MatchResult
	for _, m := range sr.Matches {
		if m.Severity >= best {
			best = m.Severity
			match := m
			bestMatch = &match
		}
	}
	return bestMatch
}

// powChallengeHTML is the interstitial page. The JS solves SHA-256 PoW and
// reloads with the solution cookie set.
const powChallengeHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Security Check</title>
<style>body{font-family:system-ui;text-align:center;padding:60px;background:#0d1117;color:#c9d1d9}
.loader{border:4px solid #21262d;border-top:4px solid #58a6ff;border-radius:50%%;width:48px;height:48px;animation:spin 1s linear infinite;margin:24px auto}
@keyframes spin{to{transform:rotate(360deg)}}</style></head>
<body><h1>🛡 NullGate Security Verification</h1><p>Verifying your browser, please wait…</p><div class="loader"></div>
<script>
const challenge=%q,difficulty=%d;
function zeroBits(hex){let bits=0;for(const c of hex){const n=parseInt(c,16);if(n===0){bits+=4;continue}bits+=Math.clz32(n)-28;break}return bits}
async function sha256(m){const b=new TextEncoder().encode(m);const h=await crypto.subtle.digest('SHA-256',b);return Array.from(new Uint8Array(h)).map(x=>x.toString(16).padStart(2,'0')).join('')}
(async()=>{let nonce=0;for(;;){const h=await sha256(challenge+nonce);if(zeroBits(h)>=difficulty)break;nonce++}
document.cookie='nullgate_pow='+challenge+':'+nonce+'; path=/; max-age=300; SameSite=Strict';location.reload()})();
</script></body></html>`
