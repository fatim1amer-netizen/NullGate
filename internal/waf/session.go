package waf

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// SessionStore is an in-memory store of authenticated dashboard sessions.
// Sessions are opaque random tokens mapped to an expiry; they never persist to
// disk, so a restart forces re-authentication — a deliberate security choice
// for an operational control surface.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token -> expiry
	ttl      time.Duration
}

// NewSessionStore creates a store whose sessions live for ttl. A background
// goroutine evicts expired tokens so the map cannot grow without bound.
func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	s := &SessionStore{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
	}
	return s
}

// Create issues a new session token valid for the store's TTL.
func (s *SessionStore) Create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return token, nil
}

// Valid reports whether a token is present and unexpired. Expired tokens are
// removed lazily on lookup.
func (s *SessionStore) Valid(token string) bool {
	if token == "" {
		return false
	}
	s.mu.RLock()
	exp, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

// Destroy invalidates a session token (logout).
func (s *SessionStore) Destroy(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Cleanup periodically evicts expired sessions. Call in a goroutine.
func (s *SessionStore) Cleanup(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for tok, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, tok)
			}
		}
		s.mu.Unlock()
	}
}

// HashPassword returns the canonical stored form of a password:
// "sha256:<hex>". Storing a hash (not the plaintext) means a leaked config
// file does not immediately hand over the admin password.
func HashPassword(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// VerifyPassword checks a plaintext attempt against a stored credential in
// constant time. The stored value may be either the "sha256:<hex>" form
// (preferred) or, for backward compatibility, a plaintext value.
func VerifyPassword(stored, attempt string) bool {
	if stored == "" {
		return false
	}
	if strings.HasPrefix(stored, "sha256:") {
		want := strings.TrimPrefix(stored, "sha256:")
		sum := sha256.Sum256([]byte(attempt))
		got := hex.EncodeToString(sum[:])
		return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
	}
	// Legacy plaintext comparison (still constant-time).
	return subtle.ConstantTimeCompare([]byte(stored), []byte(attempt)) == 1
}
