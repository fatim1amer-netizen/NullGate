package waf

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestEngine wires a real Engine in front of an httptest backend that echoes
// a fixed marker, so tests can distinguish "forwarded to backend" from "blocked
// by the WAF". Logging is quieted to keep test output clean.
func newTestEngine(t *testing.T, tune func(*Config)) (*Engine, *httptest.Server) {
	t.Helper()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("BACKEND_OK"))
	}))
	t.Cleanup(backend.Close)

	cfg := DefaultConfig()
	cfg.Server.TargetBackend = backend.URL
	cfg.Logging.Level = "error"
	cfg.Logging.EnableAccessLog = false
	// CSRF off by default in these tests unless a case turns it on, so POST
	// attack payloads are judged by the rule engine, not bounced for a token.
	cfg.CSRF.Enabled = false
	if tune != nil {
		tune(cfg)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("config invalid: %v", err)
	}
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng, backend
}

// do sends a request through the engine and returns the recorded response.
func do(eng *Engine, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	eng.ServeHTTP(rec, r)
	return rec
}

func browserHeaders(r *http.Request) {
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	r.Header.Set("Accept-Encoding", "gzip, deflate")
	r.RemoteAddr = "203.0.113.100:40000"
}

func TestEngineForwardsBenignRequest(t *testing.T) {
	eng, _ := newTestEngine(t, func(cfg *Config) {
		cfg.Logging.Level = "warn"
	})
	r := httptest.NewRequest("GET", "http://waf.local/api/products?page=2&sort=price", nil)
	browserHeaders(r)
	rec := do(eng, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("benign request: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Errorf("benign request should reach backend; body=%q", rec.Body.String())
	}
}

func TestEngineForwardsBrowserNavigationFromIPv4AndIPv6Loopback(t *testing.T) {
	eng, _ := newTestEngine(t, func(cfg *Config) {
		cfg.Logging.Level = "warn"
	})
	for _, remoteAddr := range []string{"127.0.0.1:40000", "[::1]:40000"} {
		r := httptest.NewRequest("GET", "http://waf.local/", nil)
		browserHeaders(r)
		r.RemoteAddr = remoteAddr
		r.Header.Set("Sec-Fetch-Dest", "document")
		r.Header.Set("Sec-Fetch-Mode", "navigate")
		r.Header.Set("Sec-Fetch-Site", "none")
		r.Header.Set("Sec-Fetch-User", "?1")
		rec := do(eng, r)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "BACKEND_OK") {
			t.Errorf("browser navigation from %s got %d and body %q, want forwarded 200 response",
				remoteAddr, rec.Code, rec.Body.String())
		}
	}
}

func TestEngineUsesConfiguredUpstreamHost(t *testing.T) {
	seenHost := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost <- r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	cfg := DefaultConfig()
	cfg.Server.TargetBackend = backend.URL
	cfg.Server.UpstreamHost = "7a08f55f.graduation-platform-2026.pages.dev"
	cfg.Logging.Level = "error"
	cfg.Logging.EnableAccessLog = false
	cfg.Bot.BlockEmptyUA = false
	cfg.CSRF.Enabled = false
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	r := httptest.NewRequest("GET", "http://nullgate.local/flutter_bootstrap.js", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("request: got %d, want 200", rec.Code)
	}
	if got := <-seenHost; got != cfg.Server.UpstreamHost {
		t.Errorf("upstream Host = %q, want %q", got, cfg.Server.UpstreamHost)
	}
}

func TestUpstreamTransportSupportsHTTP1AndHTTP2(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	transport, ok := eng.proxy.Transport.(*upstreamTransport)
	if !ok {
		t.Fatalf("proxy transport has type %T, want *upstreamTransport", eng.proxy.Transport)
	}
	primary, ok := transport.primary.(*http.Transport)
	if !ok {
		t.Fatalf("primary round tripper has type %T, want *http.Transport", transport.primary)
	}
	fallback, ok := transport.fallback.(*http.Transport)
	if !ok {
		t.Fatalf("fallback round tripper has type %T, want *http.Transport", transport.fallback)
	}
	if !primary.Protocols.HTTP1() || !primary.Protocols.HTTP2() {
		t.Fatalf("primary upstream protocols = %s, want HTTP/1.1 and HTTP/2", primary.Protocols)
	}
	if !fallback.Protocols.HTTP1() || fallback.Protocols.HTTP2() {
		t.Fatalf("fallback upstream protocols = %s, want HTTP/1.1 only", fallback.Protocols)
	}
	if len(fallback.TLSClientConfig.NextProtos) != 1 || fallback.TLSClientConfig.NextProtos[0] != "http/1.1" {
		t.Fatalf("fallback ALPN protocols = %v, want HTTP/1.1 only", fallback.TLSClientConfig.NextProtos)
	}
	if primary.TLSClientConfig != nil && primary.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("upstream TLS certificate verification must remain enabled")
	}
	if fallback.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("HTTP/1.1 fallback must retain TLS certificate verification")
	}
}

func TestEngineProxiesHTTP2WithVerifiedCertificate(t *testing.T) {
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream-Protocol", r.Proto)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "HTTP2_BACKEND_OK")
	}))
	backend.EnableHTTP2 = true
	backend.StartTLS()
	defer backend.Close()

	cfg := DefaultConfig()
	cfg.Server.TargetBackend = backend.URL
	cfg.Logging.Level = "error"
	cfg.Logging.EnableAccessLog = false
	cfg.CSRF.Enabled = false
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	transport := eng.proxy.Transport.(*upstreamTransport)
	roots := x509.NewCertPool()
	roots.AddCert(backend.Certificate())
	transport.primary.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}
	transport.fallback.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}

	r := httptest.NewRequest(http.MethodGet, "http://waf.local/", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusOK || rec.Body.String() != "HTTP2_BACKEND_OK" {
		t.Fatalf("HTTP/2 upstream response = %d %q, want 200 and backend body", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Upstream-Protocol"); got != "HTTP/2.0" {
		t.Fatalf("upstream protocol = %q, want HTTP/2.0", got)
	}
}

func TestUpstreamTransportFallsBackToHTTP1ForSafeRequests(t *testing.T) {
	var fallbackCalls int
	primaryErr := errors.New("HTTP/2 protocol failure")
	fallback := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		fallbackCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("fallback")),
			Request:    r,
		}, nil
	})
	transport := &upstreamTransport{
		primary:  roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, primaryErr }),
		fallback: fallback,
	}
	request := httptest.NewRequest(http.MethodGet, "https://upstream.test/", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("safe request fallback failed: %v", err)
	}
	response.Body.Close()
	if fallbackCalls != 1 {
		t.Fatalf("HTTP/1.1 fallback calls = %d, want 1", fallbackCalls)
	}

	fallbackCalls = 0
	request = httptest.NewRequest(http.MethodPost, "https://upstream.test/", strings.NewReader("body"))
	_, err = transport.RoundTrip(request)
	if err == nil {
		t.Fatal("POST primary failure should be returned without unsafe retry")
	}
	if fallbackCalls != 0 {
		t.Fatalf("POST request was retried %d times; want no retry", fallbackCalls)
	}

	primaryErr = errors.New("connection refused")
	request = httptest.NewRequest(http.MethodGet, "https://upstream.test/", nil)
	_, err = transport.RoundTrip(request)
	if err == nil || fallbackCalls != 0 {
		t.Fatalf("ordinary connection failure err=%v fallback calls=%d; want returned failure without retry",
			err, fallbackCalls)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEngineReturnsBadGatewayWhenUpstreamIsUnavailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := backend.URL
	backend.Close()

	cfg := DefaultConfig()
	cfg.Server.TargetBackend = target
	cfg.Logging.Level = "error"
	cfg.Logging.EnableAccessLog = false
	cfg.CSRF.Enabled = false
	eng, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	r := httptest.NewRequest(http.MethodGet, "http://waf.local/", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unavailable upstream returned %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Bad Gateway") {
		t.Errorf("unavailable upstream body = %q, want generic Bad Gateway", rec.Body.String())
	}
}

func TestInspectionScoresEachRuleOnlyOncePerRequest(t *testing.T) {
	eng, _ := newTestEngine(t, func(cfg *Config) {
		cfg.Rules.AnomalyThreshold = 4
		cfg.Rules.RuleActions = map[string]string{
			"critical": "log", "high": "log", "medium": "log", "low": "log",
		}
	})
	r := httptest.NewRequest("GET", "http://waf.local/", nil)
	browserHeaders(r)
	r.Header.Add("X-First-Comment", "/* note */")
	r.Header.Add("X-Second-Comment", "/* note */")

	result, blocked := eng.inspect(r, nil)
	if blocked {
		t.Fatalf("duplicated rule hit should not accumulate enough score to block: score=%d", result.AnomalyScore)
	}
	if result.AnomalyScore != 3 || len(result.Matches) != 1 {
		t.Errorf("same rule should contribute once: score=%d matches=%d", result.AnomalyScore, len(result.Matches))
	}

	eng, _ = newTestEngine(t, func(cfg *Config) {
		cfg.Rules.AnomalyThreshold = 6
		cfg.Rules.RuleActions = map[string]string{
			"critical": "log", "high": "log", "medium": "log", "low": "log",
		}
	})
	r = httptest.NewRequest("GET", "http://waf.local/", nil)
	browserHeaders(r)
	r.Header.Set("X-SQL-Comment", "/* note */")
	r.Header.Set("X-JavaScript", "alert(1)")
	result, blocked = eng.inspect(r, nil)
	if !blocked || result.AnomalyScore != 6 || len(result.Matches) != 2 {
		t.Errorf("different medium rules should still accumulate: blocked=%t score=%d matches=%d",
			blocked, result.AnomalyScore, len(result.Matches))
	}
}

func TestEngineBlocksSQLiInQuery(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("GET", "http://waf.local/search?q="+urlEnc("' OR '1'='1"), nil)
	browserHeaders(r)
	rec := do(eng, r)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("SQLi query: got %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Error("blocked request must not reach the backend")
	}
	// The generic error body must not leak which rule fired.
	if strings.Contains(strings.ToLower(rec.Body.String()), "sqli") {
		t.Error("error body leaked the matched category")
	}
}

func TestEngineBlocksXSSInBody(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	body := `{"comment":"<script>alert(document.cookie)</script>"}`
	r := httptest.NewRequest("POST", "http://waf.local/comments", strings.NewReader(body))
	browserHeaders(r)
	r.Header.Set("Content-Type", "application/json")
	rec := do(eng, r)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("XSS body: got %d, want 403", rec.Code)
	}
}

func TestEngineBlocksTraversalInPath(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("GET", "http://waf.local/files/..%2f..%2f..%2fetc%2fpasswd", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("path traversal: got %d, want 403", rec.Code)
	}
}

func TestEngineForwardsUnknownPathNormally(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("GET", "http://waf.local/.env", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown path: got %d, want 200 from backend", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Error("unknown path should use the normal inspection and proxy flow")
	}
}

func TestEngineForwardsFlutterBenignRequests(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	longNotes := strings.Repeat("Graduation project progress and academic requirements. ", 100)
	requests := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        string
	}{
		{"main dart asset", http.MethodGet, "http://waf.local/main.dart.js?v=1.0.0&build=release", "", ""},
		{"service worker asset", http.MethodGet, "http://waf.local/flutter_service_worker.js?version=123456", "", ""},
		{"Flutter API JSON", http.MethodPost, "http://waf.local/api/projects", "application/json",
			`{"title":"Graduation project","summary":"Academic project submission","notes":"` + longNotes + `"}`},
		{"base64 image asset", http.MethodGet, "http://waf.local/assets/logo.png?data=iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI%2FScL%2FwAAAABJRU5ErkJggg%3D%3D", "", ""},
		{"Flutter API query", http.MethodGet, "http://waf.local/api/submissions?semester=2026&status=review", "", ""},
	}

	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			browserHeaders(r)
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}
			rec := do(eng, r)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "BACKEND_OK") {
				t.Fatalf("benign Flutter request got %d and body %q, want forwarded backend response", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestEngineAllowsBrowserFaviconWithLoopbackOriginAndReferer(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest(http.MethodGet, "http://waf.local/favicon.ico", nil)
	browserHeaders(r)
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("Referer", "http://localhost:8080/")
	rec := do(eng, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Fatalf("browser favicon request got %d and body %q, want forwarded 200", rec.Code, rec.Body.String())
	}
}

func TestEngineStillScansNonSSRFRulesInOriginAndReferer(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest(http.MethodGet, "http://waf.local/", nil)
	browserHeaders(r)
	r.Header.Set("Referer", "http://localhost:8080/<script>alert(1)</script>")
	rec := do(eng, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("XSS in Referer got %d, want 403; only SSRF checks should be context-exempt", rec.Code)
	}
}

func TestEngineBlocksSSRFTargetsAndReportsRuleReason(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"loopback", "http://127.0.0.1:8080/admin"},
		{"IPv6 loopback", "http://[::1]:8080/admin"},
		{"encoded loopback", "http%3A%2F%2F127.0.0.1%3A8080%2Fadmin"},
		{"10 private range", "http://10.20.30.40/private"},
		{"172 private range", "https://172.20.1.10/private"},
		{"private IPv4", "https://192.168.1.10/private"},
		{"metadata address", "http://169.254.169.254/latest/meta-data/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng, _ := newTestEngine(t, func(cfg *Config) {
				cfg.Logging.Level = "warn"
			})
			r := httptest.NewRequest(http.MethodGet, "http://waf.local/fetch?url="+url.QueryEscape(tc.value), nil)
			browserHeaders(r)
			rec := do(eng, r)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("SSRF target %q returned %d, want 403", tc.value, rec.Code)
			}
			events := eng.logger.RecentEvents(1)
			if len(events) != 1 || events[0].Fields["layer"] != "rules" {
				t.Fatalf("expected a rule-block security event, got %#v", events)
			}
			reason, _ := events[0].Fields["reason"].(string)
			if !strings.Contains(reason, "SSRF-") || !strings.Contains(reason, " (score ") {
				t.Errorf("security event reason = %q, want SSRF rule and score", reason)
			}
		})
	}
}

func TestLogOnlyDetectionsFeedRuntimeCategoriesAndEvents(t *testing.T) {
	eng, _ := newTestEngine(t, func(cfg *Config) {
		cfg.Rules.AnomalyThreshold = 100
		cfg.Rules.RuleActions["high"] = "log"
		cfg.Logging.Level = "warn"
	})
	r := httptest.NewRequest(http.MethodGet,
		"http://waf.local/fetch?url="+url.QueryEscape("http://127.0.0.1/admin"), nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("log-only SSRF match got %d, want allowed upstream response", rec.Code)
	}
	snapshot := eng.Metrics().Snapshot()
	if snapshot.ByCategory[string(CatSSRF)] != 1 || snapshot.ByRule["SSRF-001"] != 1 {
		t.Fatalf("log-only detection metrics category=%d rule=%d, want 1/1",
			snapshot.ByCategory[string(CatSSRF)], snapshot.ByRule["SSRF-001"])
	}
	events := eng.logger.RecentEvents(1)
	if len(events) != 1 || events[0].Message != "rule matched" {
		t.Fatalf("log-only security event = %#v, want one real rule match event", events)
	}
	if events[0].Fields["rule_id"] != "SSRF-001" || events[0].Fields["description"] != "Loopback target" {
		t.Fatalf("detection event fields = %#v, want rule id and description", events[0].Fields)
	}
}

func TestEngineAllowsLegitimatePublicURLAndLookalikeHost(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest(http.MethodGet,
		"http://waf.local/search?next="+url.QueryEscape("https://docs.example.com/path?next=http://localhost.example/help"), nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Fatalf("legitimate URL request got %d and body %q, want forwarded 200", rec.Code, rec.Body.String())
	}
}

func TestRuntimeMetricsDistinguishAllowedBlockedAndProxyFailure(t *testing.T) {
	eng, backend := newTestEngine(t, nil)

	allowed := httptest.NewRequest(http.MethodGet, "http://waf.local/healthy", nil)
	browserHeaders(allowed)
	if rec := do(eng, allowed); rec.Code != http.StatusOK {
		t.Fatalf("benign request status = %d, want 200", rec.Code)
	}

	blocked := httptest.NewRequest(http.MethodGet,
		"http://waf.local/fetch?url="+url.QueryEscape("http://127.0.0.1/admin"), nil)
	browserHeaders(blocked)
	if rec := do(eng, blocked); rec.Code != http.StatusForbidden {
		t.Fatalf("SSRF request status = %d, want 403", rec.Code)
	}

	backend.Close()
	failed := httptest.NewRequest(http.MethodGet, "http://waf.local/upstream-failure", nil)
	browserHeaders(failed)
	if rec := do(eng, failed); rec.Code != http.StatusBadGateway {
		t.Fatalf("upstream failure status = %d, want 502", rec.Code)
	}

	snapshot := eng.Metrics().Snapshot()
	if snapshot.Total != 3 || snapshot.Allowed != 2 || snapshot.Blocked != 1 || snapshot.ProxyErrors != 1 {
		t.Fatalf("runtime metrics = total %d allowed %d blocked %d proxy-errors %d, want 3/2/1/1",
			snapshot.Total, snapshot.Allowed, snapshot.Blocked, snapshot.ProxyErrors)
	}
	if snapshot.AvgLatencyMS <= 0 {
		t.Fatalf("average latency = %f ms, want a measured positive value", snapshot.AvgLatencyMS)
	}
	if snapshot.LatencySamples != 3 {
		t.Fatalf("latency sample count = %d, want one measurement per request", snapshot.LatencySamples)
	}
	if snapshot.ByCategory[string(CatSSRF)] != 1 {
		t.Fatalf("SSRF category count = %d, want 1", snapshot.ByCategory[string(CatSSRF)])
	}

	adminCfg := eng.config.Admin
	adminCfg.RequireLocalhost = false
	admin := NewAdminServer(adminCfg, eng.config, eng.Metrics(), eng.Logger(), eng.Blocklist(),
		eng.Rules(), "test", eng.Rules().RuleCount(), eng.Rules().TotalRuleCount(), eng.ProbeBackend)
	overviewRequest := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	overviewRequest.RemoteAddr = "203.0.113.100:40000"
	overview := httptest.NewRecorder()
	admin.Handler().ServeHTTP(overview, overviewRequest)
	if overview.Code != http.StatusOK {
		t.Fatalf("dashboard overview status = %d, want 200", overview.Code)
	}
	var dashboardData struct {
		Metrics       Snapshot      `json:"metrics"`
		BackendHealth BackendHealth `json:"backend_health"`
	}
	if err := json.Unmarshal(overview.Body.Bytes(), &dashboardData); err != nil {
		t.Fatalf("decode dashboard overview: %v", err)
	}
	if dashboardData.Metrics.Total != 3 || dashboardData.Metrics.Blocked != 1 || dashboardData.Metrics.ProxyErrors != 1 {
		t.Fatalf("dashboard metrics = total %d blocked %d proxy errors %d, want 3/1/1",
			dashboardData.Metrics.Total, dashboardData.Metrics.Blocked, dashboardData.Metrics.ProxyErrors)
	}
	if dashboardData.BackendHealth.State != "unhealthy" {
		t.Fatalf("dashboard backend health = %q, want unhealthy after upstream failure", dashboardData.BackendHealth.State)
	}
}

func TestEngineBlocksScannerUA(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("GET", "http://waf.local/", nil)
	r.RemoteAddr = "203.0.113.101:40000"
	r.Header.Set("User-Agent", "sqlmap/1.7.2#stable")
	rec := do(eng, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("scanner UA: got %d, want 403", rec.Code)
	}
}

func TestEngineRejectsBadMethod(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("TRACE", "http://waf.local/", nil)
	browserHeaders(r)
	rec := do(eng, r)
	if rec.Code == http.StatusOK {
		t.Fatalf("TRACE should be rejected, got %d", rec.Code)
	}
}

// With CSRF enabled, a state-changing request without a token is rejected;
// a GET first mints a token cookie that a following POST can echo.
func TestEngineCSRFFlow(t *testing.T) {
	eng, _ := newTestEngine(t, func(c *Config) {
		c.CSRF.Enabled = true
	})

	// POST with no token → blocked.
	post := httptest.NewRequest("POST", "http://waf.local/api/update", strings.NewReader("x=1"))
	browserHeaders(post)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec := do(eng, post); rec.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF token: got %d, want 403", rec.Code)
	}

	// GET → should hand out a csrf_token cookie.
	get := httptest.NewRequest("GET", "http://waf.local/form", nil)
	browserHeaders(get)
	rec := do(eng, get)
	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("GET should issue a csrf_token cookie")
	}

	// POST echoing the token → allowed through to backend.
	post2 := httptest.NewRequest("POST", "http://waf.local/api/update", strings.NewReader("x=1"))
	browserHeaders(post2)
	post2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post2.Header.Set("X-CSRF-Token", token)
	rec2 := do(eng, post2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("POST with valid CSRF token: got %d, want 200", rec2.Code)
	}
}

// urlEnc is a tiny query-escaper local to the test (avoids importing net/url
// just for one call and keeps intent obvious at the call site).
func urlEnc(s string) string {
	repl := strings.NewReplacer(
		" ", "%20", "'", "%27", "\"", "%22",
		"<", "%3C", ">", "%3E", "=", "%3D",
	)
	return repl.Replace(s)
}
