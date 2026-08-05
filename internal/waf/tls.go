package waf

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
)

// TLSFingerprinter computes a JA3-style hash of a TLS handshake from the
// negotiated connection state. JA3 is a widely-adopted client fingerprint:
// browsers, bots, and offensive tools produce distinguishable hashes because
// they offer different cipher suites, extensions, and version combinations.
//
// Note on limitation: Go's tls.Conn.ConnectionState() exposes the *negotiated*
// handshake, not the *offered* ClientHello. The hash we compute is therefore a
// weaker, post-negotiation fingerprint than a true JA3 from a packet capture.
// It is still useful as an additional signal for the dashboard and for
// recognizing clearly anomalous clients (e.g. a TLS 1.0 client). Where real
// JA3 is required, pair NullGate with a packet-level sensor.
type TLSFingerprinter struct {
	knownBots map[string]string // hash -> label
}

// NewTLSFingerprinter seeds the fingerprint table with well-known automation
// hashes. These are deliberately approximate — see the note above — and are
// best used for dashboard labeling rather than hard blocking.
func NewTLSFingerprinter() *TLSFingerprinter {
	return &TLSFingerprinter{
		knownBots: map[string]string{
			"76d793a13335f982fc7b46db61780ea5": "tls12-synthetic-client",
		},
	}
}

// JA3 computes the fingerprint string from a TLS connection state.
func (t *TLSFingerprinter) JA3(state *tls.ConnectionState) (string, error) {
	if state == nil {
		return "", fmt.Errorf("nil TLS connection state")
	}

	// Go exposes only the negotiated cipher suite (not the full offered
	// ClientHello), so we fingerprint on the combination that is available:
	// version + negotiated cipher + ALPN. This still distinguishes broad
	// client classes (a modern browser negotiating TLS 1.3 + h2 looks very
	// different from a scripted TLS 1.2 client offering an RSA suite with no
	// ALPN).
	alpn := state.NegotiatedProtocol
	if alpn == "" {
		alpn = "-"
	}
	raw := fmt.Sprintf("%d,%d,%s",
		state.Version,
		state.CipherSuite,
		alpn,
	)
	return raw, nil
}

// Hash returns a compact hex digest of the JA3 string (stable, easy to log).
func (t *TLSFingerprinter) Hash(state *tls.ConnectionState) (string, error) {
	ja3, err := t.JA3(state)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(ja3))
	return hex.EncodeToString(sum[:16]), nil
}

// Label returns a human-readable name for a fingerprint if it is known, else
// the hash itself.
func (t *TLSFingerprinter) Label(hash string) string {
	if lbl, ok := t.knownBots[hash]; ok {
		return lbl
	}
	return hash
}

// IsKnownBot reports whether a fingerprint hash matches a known automation
// signature.
func (t *TLSFingerprinter) IsKnownBot(hash string) bool {
	_, ok := t.knownBots[hash]
	return ok
}
