package waf

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	eng, _ := newTestEngine(t, nil)
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
