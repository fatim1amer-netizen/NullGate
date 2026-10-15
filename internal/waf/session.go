package waf

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
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

// HashPassword returns a bcrypt password hash suitable for admin configuration.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash admin password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a plaintext attempt against a stored credential. Bcrypt
// is preferred; legacy SHA-256 hashes remain readable for existing configs.
func VerifyPassword(stored, attempt string) bool {
	if stored == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(attempt)) == nil
	}
	if strings.HasPrefix(stored, "sha256:") {
		want := strings.TrimPrefix(stored, "sha256:")
		sum := sha256.Sum256([]byte(attempt))
		got := hex.EncodeToString(sum[:])
		return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
	}
	return false
}

func supportedPasswordHash(stored string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		_, err := bcrypt.Cost([]byte(stored))
		return err == nil
	}
	if strings.HasPrefix(stored, "sha256:") {
		encoded := strings.TrimPrefix(stored, "sha256:")
		if len(encoded) != sha256.Size*2 {
			return false
		}
		_, err := hex.DecodeString(encoded)
		return err == nil
	}
	return false
}
