package waf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// rawURIRequest builds a request whose RequestURI carries raw (unescaped) bytes
// in the query string. httptest.NewRequest and url.Parse escape control bytes in
// the path, so RawQuery is the channel through which such bytes actually reach
// the guard — exactly as they would from a hand-crafted attack request.
func rawURIRequest(t *testing.T, method, rawQueryBytes string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, "http://example.com/", nil)
	r.URL.RawQuery = rawQueryBytes
	return r
}

func protoConfig() ProtocolConfig {
	cfg := DefaultConfig().Protocol
	return cfg
}

func checkReq(t *testing.T, g *ProtocolGuard, r *http.Request) error {
	t.Helper()
	err := g.Check(r)
	if err != nil {
		if pe, ok := err.(*ProtocolError); ok {
			t.Logf("rejected: %s (status %d)", pe.Reason, pe.Status)
		}
	}
	return err
}

func TestProtocolAllowsValidRequest(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	r := httptest.NewRequest("GET", "http://example.com/api/users?page=2", nil)
	if err := checkReq(t, g, r); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestProtocolRejectsDisallowedMethod(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	for _, m := range []string{"TRACE", "CONNECT", "BREW"} {
		r := httptest.NewRequest(m, "http://example.com/", nil)
		if err := checkReq(t, g, r); err == nil {
			t.Errorf("method %s should be rejected", m)
		}
	}
}

func TestProtocolRejectsNullByte(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	// Build the request manually: httptest.NewRequest pre-validates the URL and
	// would reject a null byte before our guard runs. We craft the raw URL so
	// the guard itself is what performs the rejection.
	r := rawURIRequest(t, "GET", "foo\x00bar")
	if err := checkReq(t, g, r); err == nil {
		t.Error("null byte in URI should be rejected")
	}
}

func TestProtocolRejectsControlChar(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	r := rawURIRequest(t, "GET", "a\x07b")
	if err := checkReq(t, g, r); err == nil {
		t.Error("control character in URI should be rejected")
	}
}

func TestProtocolRejectsOversizedBody(t *testing.T) {
	cfg := protoConfig()
	cfg.MaxRequestSize = 1024
	g := NewProtocolGuard(cfg)
	r := httptest.NewRequest("POST", "http://example.com/", nil)
	r.ContentLength = 2048 // declares more than allowed
	if err := checkReq(t, g, r); err == nil {
		t.Error("content-length above limit should be rejected")
	}
}

// The canonical request-smuggling primitive: declaring both a Content-Length
// and Transfer-Encoding lets a front proxy and a back-end disagree about where
// the body ends. Refuse to guess.
func TestProtocolRejectsConflictingEncoding(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	r := httptest.NewRequest("POST", "http://example.com/", nil)
	r.TransferEncoding = []string{"chunked"}
	r.ContentLength = 100
	if err := checkReq(t, g, r); err == nil {
		t.Error("conflicting Content-Length + Transfer-Encoding should be rejected")
	}
}

func TestProtocolRejectsChunkedWhenDisabled(t *testing.T) {
	cfg := protoConfig()
	cfg.RejectChunkedBodies = true
	g := NewProtocolGuard(cfg)
	r := httptest.NewRequest("POST", "http://example.com/", nil)
	r.TransferEncoding = []string{"chunked"}
	r.ContentLength = -1
	if err := checkReq(t, g, r); err == nil {
		t.Error("chunked body should be rejected when configured")
	}
}

func TestProtocolEnforcesContentType(t *testing.T) {
	cfg := protoConfig()
	cfg.EnforceContentType = true
	g := NewProtocolGuard(cfg)

	// A body request with no Content-Type at all.
	r := httptest.NewRequest("POST", "http://example.com/", nil)
	r.ContentLength = 10
	if err := checkReq(t, g, r); err == nil {
		t.Error("missing content-type on body request should be rejected")
	}

	// A disallowed type.
	r2 := httptest.NewRequest("POST", "http://example.com/", nil)
	r2.Header.Set("Content-Type", "text/html")
	r2.ContentLength = 10
	if err := checkReq(t, g, r2); err == nil {
		t.Error("disallowed content-type should be rejected")
	}

	// An allowed type with parameters passes.
	r3 := httptest.NewRequest("POST", "http://example.com/", nil)
	r3.Header.Set("Content-Type", "application/json; charset=utf-8")
	r3.ContentLength = 10
	if err := checkReq(t, g, r3); err != nil {
		t.Errorf("allowed content-type with params rejected: %v", err)
	}
}

// Content-Length vs actual body mismatch — the tool smuggling relies on.
func TestProtocolRejectsContentLengthMismatch(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	r := httptest.NewRequest("POST", "http://example.com/", nil)
	r.Header.Set("Content-Length", "100") // declares 100...
	r.ContentLength = 100
	// ...but the buffered body is actually 3 bytes.
	if err := g.ValidateContentLengthMismatch(r, 3); err == nil {
		t.Error("content-length mismatch should be rejected")
	}
	// Consistent lengths pass.
	r2 := httptest.NewRequest("POST", "http://example.com/", nil)
	r2.Header.Set("Content-Length", "3")
	r2.ContentLength = 3
	if err := g.ValidateContentLengthMismatch(r2, 3); err != nil {
		t.Errorf("consistent content-length rejected: %v", err)
	}
}

func TestProtocolRejectsTooManyHeaders(t *testing.T) {
	cfg := protoConfig()
	cfg.MaxHeaderCount = 5
	g := NewProtocolGuard(cfg)
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	for i := 0; i < 10; i++ {
		r.Header.Set("X-Test-Header-"+string(rune('A'+i%26)), "value")
	}
	if err := checkReq(t, g, r); err == nil {
		t.Error("too many headers should be rejected")
	}
}

func TestProtocolRejectsUnsupportedVersion(t *testing.T) {
	g := NewProtocolGuard(protoConfig())
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	r.ProtoMajor = 0 // unsupported (0.x)
	if err := checkReq(t, g, r); err == nil {
		t.Error("unsupported HTTP version should be rejected")
	}
}
