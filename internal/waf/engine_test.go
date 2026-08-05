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

func TestEngineHoneypotBlocks(t *testing.T) {
	eng, _ := newTestEngine(t, nil)
	r := httptest.NewRequest("GET", "http://waf.local/.env", nil)
	browserHeaders(r)
	rec := do(eng, r)
	// Honeypot returns 404 to avoid confirming the trap exists.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("honeypot: got %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "BACKEND_OK") {
		t.Error("honeypot probe must not reach backend")
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
