package waf

import (
	"net/http"
	"strings"
)

// ResponseGuard hardens responses that the backend generates before they reach
// the client. Input filtering stops attacks at the door, but output hardening
// stops data leakage, clickjacking, MIME sniffing, and cookie exposure —
// including when the backend itself forgets to set the right headers.
type ResponseGuard struct {
	cfg ResponseConfig
}

// NewResponseGuard builds a guard from the response section of config.
func NewResponseGuard(cfg ResponseConfig) *ResponseGuard {
	return &ResponseGuard{cfg: cfg}
}

// RedactHeaders is the default set of headers whose values must never appear
// in logs or dashboards, even if config does not enumerate them.
var RedactHeaders = []string{
	"authorization", "proxy-authorization", "x-api-key", "api-key",
	"cookie", "set-cookie", "x-auth-token", "x-csrf-token",
}

// Apply mutates the response headers in place: strips server-identifying
// headers, adds security headers, and redacts sensitive values from reflected
// copies. It is called on the proxied response before writing.
func (g *ResponseGuard) Apply(header http.Header, req *http.Request) {
	if !g.cfg.Enabled {
		return
	}

	if g.cfg.StripServerHeader {
		header.Del("Server")
	}
	if g.cfg.StripXPoweredBy {
		header.Del("X-Powered-By")
	}
	// Also strip our own proxy's default server header.
	header.Del("X-WAF-Server")

	if g.cfg.AddSecurityHeaders {
		// Frame-busting: prevents clickjacking of the protected app.
		if header.Get("X-Frame-Options") == "" {
			header.Set("X-Frame-Options", "DENY")
		}
		// MIME sniffing defense; also blocks some content-type confusion.
		if header.Get("X-Content-Type-Options") == "" {
			header.Set("X-Content-Type-Options", "nosniff")
		}
		// Referrer policy: don't leak tokens/URLs to third parties.
		if header.Get("Referrer-Policy") == "" {
			header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		}
		// Permissions policy: disable risky browser features by default.
		if header.Get("Permissions-Policy") == "" {
			header.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")
		}
	}
	if g.cfg.AddCSP && header.Get("Content-Security-Policy") == "" {
		// Default-deny CSP. Backends may override with a tighter policy; this
		// is the baseline that keeps stored/reflected XSS from executing.
		header.Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self'; "+
				"img-src 'self' data:; font-src 'self'; connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	}
	if g.cfg.AddHSTS && header.Get("Strict-Transport-Security") == "" {
		header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

// SafeHeaderValue returns a header value with embedded CR/LF stripped, so a
// backend-injected header value can never smuggle additional headers to the
// client (response-splitting defense).
func SafeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

// RedactValue masks the middle of a secret-shaped value for logs, keeping
// only enough for an operator to recognize the source.
func RedactValue(v string) string {
	if v == "" {
		return "<empty>"
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + "****" + v[len(v)-2:]
}
