package waf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CSRFManager issues and validates anti-CSRF tokens using the double-submit
// cookie pattern reinforced with server-side validation and expiry.
//
// The engine sets a token cookie on first contact; state-changing requests
// (POST/PUT/DELETE/PATCH) must echo that token in a header or form field. A
// token is single-use on validation, which also thwarts replay.
type CSRFManager struct {
	tokens         sync.Map // token -> expiry (time.Time)
	ttl            time.Duration
	excludePaths   map[string]bool
	excludeMethods map[string]bool
}

// NewCSRFManager builds a manager and starts its expiry-sweeping goroutine.
func NewCSRFManager(cfg CSRFConfig) *CSRFManager {
	ttl := time.Duration(cfg.TokenTTLMin) * time.Minute
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	m := &CSRFManager{
		ttl:            ttl,
		excludePaths:   make(map[string]bool),
		excludeMethods: make(map[string]bool),
	}
	for _, p := range cfg.ExcludePaths {
		m.excludePaths[p] = true
	}
	for _, mth := range cfg.ExcludeMethods {
		m.excludeMethods[strings.ToUpper(mth)] = true
	}
	go m.cleanup()
	return m
}

// GenerateToken creates, stores, and returns a new random token.
func (m *CSRFManager) GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is catastrophic; return an empty token which
		// will always fail validation rather than a predictable one.
		return ""
	}
	token := hex.EncodeToString(b)
	m.tokens.Store(token, time.Now().Add(m.ttl))
	return token
}

// ValidateToken checks a token is known and unexpired, then consumes it.
// Comparison against stored keys is not timing-sensitive (the token is the map
// key), but presence is treated as a single-use credential.
func (m *CSRFManager) ValidateToken(token string) bool {
	if token == "" {
		return false
	}
	val, ok := m.tokens.Load(token)
	if !ok {
		return false
	}
	expiry := val.(time.Time)
	if time.Now().After(expiry) {
		m.tokens.Delete(token)
		return false
	}
	m.tokens.Delete(token)
	return true
}

// Required reports whether a request must carry a valid CSRF token.
func (m *CSRFManager) Required(r *http.Request) bool {
	if m.excludeMethods[strings.ToUpper(r.Method)] {
		return false
	}
	if m.excludePaths[r.URL.Path] {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

// ExtractToken pulls a submitted token from header, form, or cookie.
func (m *CSRFManager) ExtractToken(r *http.Request) string {
	if h := r.Header.Get("X-CSRF-Token"); h != "" {
		return h
	}
	if f := r.FormValue("csrf_token"); f != "" {
		return f
	}
	if c, err := r.Cookie("csrf_token"); err == nil {
		return c.Value
	}
	return ""
}

// ConstantTimeEqual compares two tokens without leaking length/content timing.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (m *CSRFManager) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.tokens.Range(func(key, value interface{}) bool {
			if now.After(value.(time.Time)) {
				m.tokens.Delete(key)
			}
			return true
		})
	}
}
