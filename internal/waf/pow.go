package waf

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// PoWValidator issues and verifies Proof-of-Work challenges. PoW imposes an
// asymmetric cost: trivial for the server to verify, expensive for a client
// to solve at scale. It is the WAF's answer to distributed low-and-slow abuse
// that slips under rate limits.
//
// Security properties enforced here (all missing from the original):
//   - The challenge is an HMAC over (clientIP || issuedAt) keyed by a server
//     secret, so a client cannot pre-compute or reuse another client's
//     challenge, and the server is stateless (no challenge table to exhaust).
//   - The embedded timestamp is verified against ChallengeTTL, so a solved
//     token cannot be replayed indefinitely.
//   - The solution nonce is verified to actually produce the required number
//     of leading zero *bits* in SHA-256(challenge || nonce).
type PoWValidator struct {
	cfg PoWConfig
}

// NewPoWValidator builds a validator from config.
func NewPoWValidator(cfg PoWConfig) *PoWValidator {
	return &PoWValidator{cfg: cfg}
}

// challenge encodes "<issuedAt>.<mac>" where mac authenticates issuedAt+IP.
func (p *PoWValidator) sign(clientIP string, issuedAt int64) string {
	mac := hmac.New(sha256.New, []byte(p.cfg.HMACSecret))
	mac.Write([]byte(clientIP))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatInt(issuedAt, 10)))
	sum := mac.Sum(nil)
	return strconv.FormatInt(issuedAt, 10) + "." + hex.EncodeToString(sum)
}

// GenerateChallenge returns a fresh challenge string and the difficulty the
// client must satisfy. issuedAtOverride, when non-zero, pins the timestamp
// (used only by tests); production callers pass 0 to use the current time.
func (p *PoWValidator) GenerateChallenge(clientIP string, issuedAtOverride int64) (string, int) {
	issuedAt := issuedAtOverride
	if issuedAt == 0 {
		issuedAt = time.Now().Unix()
	}
	return p.sign(clientIP, issuedAt), p.cfg.Difficulty
}

// verifyChallenge confirms a challenge was issued by us for this IP and is not
// expired.
func (p *PoWValidator) verifyChallenge(challenge, clientIP string) bool {
	dot := strings.IndexByte(challenge, '.')
	if dot <= 0 {
		return false
	}
	issuedAtStr := challenge[:dot]
	issuedAt, err := strconv.ParseInt(issuedAtStr, 10, 64)
	if err != nil {
		return false
	}
	// Recompute the MAC and compare in constant time.
	expected := p.sign(clientIP, issuedAt)
	if !hmac.Equal([]byte(expected), []byte(challenge)) {
		return false
	}
	// Enforce TTL.
	age := time.Now().Unix() - issuedAt
	if age < 0 || age > int64(p.cfg.ChallengeTTL) {
		return false
	}
	return true
}

// Validate checks the "nullgate_pow" cookie: it must carry a challenge we
// issued to this IP, still within TTL, plus a nonce whose hash meets difficulty.
func (p *PoWValidator) Validate(r *http.Request, clientIP string) bool {
	cookie, err := r.Cookie("nullgate_pow")
	if err != nil {
		return false
	}
	// Cookie format: "<challenge>:<nonce>". The challenge itself contains a
	// '.', so split only on the final ':' boundary.
	raw := cookie.Value
	sep := strings.LastIndexByte(raw, ':')
	if sep <= 0 || sep == len(raw)-1 {
		return false
	}
	challenge := raw[:sep]
	nonce := raw[sep+1:]

	if !p.verifyChallenge(challenge, clientIP) {
		return false
	}
	return meetsDifficulty(challenge+nonce, p.cfg.Difficulty)
}

// meetsDifficulty reports whether SHA-256(data) begins with `bits` zero bits.
// Using bit-level difficulty (not hex-char difficulty) makes the cost curve
// fine-grained and predictable.
func meetsDifficulty(data string, bits int) bool {
	if bits <= 0 {
		return true
	}
	sum := sha256.Sum256([]byte(data))
	fullBytes := bits / 8
	remBits := bits % 8
	for i := 0; i < fullBytes; i++ {
		if sum[i] != 0 {
			return false
		}
	}
	if remBits > 0 {
		mask := byte(0xFF << (8 - remBits))
		if sum[fullBytes]&mask != 0 {
			return false
		}
	}
	return true
}
