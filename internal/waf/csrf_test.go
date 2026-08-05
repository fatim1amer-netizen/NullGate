package waf

import (
	"net/http"
	"testing"
)

func csrfManager() *CSRFManager {
	return NewCSRFManager(CSRFConfig{
		Enabled:        true,
		TokenTTLMin:    30,
		ExcludePaths:   []string{"/healthz"},
		ExcludeMethods: []string{"GET", "HEAD", "OPTIONS"},
	})
}

func TestCSRFTokenRoundTrip(t *testing.T) {
	m := csrfManager()
	tok := m.GenerateToken()
	if tok == "" {
		t.Fatal("GenerateToken returned empty")
	}
	if !m.ValidateToken(tok) {
		t.Fatal("freshly generated token failed validation")
	}
}

// Tokens are single-use: a second validation of the same token must fail.
func TestCSRFTokenSingleUse(t *testing.T) {
	m := csrfManager()
	tok := m.GenerateToken()
	if !m.ValidateToken(tok) {
		t.Fatal("first validation should succeed")
	}
	if m.ValidateToken(tok) {
		t.Fatal("token was accepted twice — replay not prevented")
	}
}

func TestCSRFRejectsUnknownToken(t *testing.T) {
	m := csrfManager()
	if m.ValidateToken("deadbeef-not-a-real-token") {
		t.Fatal("unknown token accepted")
	}
	if m.ValidateToken("") {
		t.Fatal("empty token accepted")
	}
}

func TestCSRFRequiredOnlyForStateChanging(t *testing.T) {
	m := csrfManager()
	cases := map[string]bool{
		"GET":     false,
		"HEAD":    false,
		"OPTIONS": false,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"PATCH":   true,
	}
	for method, want := range cases {
		r, _ := http.NewRequest(method, "/api/resource", nil)
		if got := m.Required(r); got != want {
			t.Errorf("Required(%s) = %v, want %v", method, got, want)
		}
	}
}

func TestCSRFExcludedPathBypasses(t *testing.T) {
	m := csrfManager()
	r, _ := http.NewRequest("POST", "/healthz", nil)
	if m.Required(r) {
		t.Fatal("excluded path should not require a CSRF token")
	}
}

func TestCSRFExtractTokenSources(t *testing.T) {
	m := csrfManager()

	// Header.
	r1, _ := http.NewRequest("POST", "/", nil)
	r1.Header.Set("X-CSRF-Token", "hdr-token")
	if got := m.ExtractToken(r1); got != "hdr-token" {
		t.Errorf("header token not extracted: %q", got)
	}

	// Cookie (when no header/form present).
	r2, _ := http.NewRequest("POST", "/", nil)
	r2.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
	if got := m.ExtractToken(r2); got != "cookie-token" {
		t.Errorf("cookie token not extracted: %q", got)
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !ConstantTimeEqual("abc123", "abc123") {
		t.Error("equal strings should compare equal")
	}
	if ConstantTimeEqual("abc123", "abc124") {
		t.Error("different strings should not compare equal")
	}
	if ConstantTimeEqual("abc", "abcd") {
		t.Error("different-length strings should not compare equal")
	}
}
