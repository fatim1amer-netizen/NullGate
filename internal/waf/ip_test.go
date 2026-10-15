package waf

import (
	"net/http"
	"testing"
)

func newXFFResolver(trustXFF, trustXRI bool, trusted ...string) *IPResolver {
	return NewIPResolver(IPConfig{
		TrustXForwardedFor: trustXFF,
		TrustXRealIP:       trustXRI,
		MaxForwardedChain:  5,
		TrustedProxies:     trusted,
	}, nil)
}

func reqWith(remoteAddr string, headers map[string]string) *http.Request {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// The headline security property: when XFF is NOT trusted, a client cannot
// spoof its identity via the header — the peer address wins.
func TestClientIPIgnoresUntrustedXFF(t *testing.T) {
	r := newXFFResolver(false, false)
	req := reqWith("203.0.113.5:44321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	if got := r.ClientIP(req); got != "203.0.113.5" {
		t.Fatalf("spoofed XFF was trusted: got %q, want peer 203.0.113.5", got)
	}
}

// Even with XFF trust enabled, a request arriving directly (peer not a trusted
// proxy) must not have its forwarding header honored.
func TestClientIPRequiresTrustedPeer(t *testing.T) {
	r := newXFFResolver(true, true, "10.0.0.0/8")
	// Peer 203.0.113.5 is public, not in the trusted 10/8 range.
	req := reqWith("203.0.113.5:44321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	})
	if got := r.ClientIP(req); got != "203.0.113.5" {
		t.Fatalf("XFF honored from untrusted peer: got %q, want 203.0.113.5", got)
	}
}

// When the peer IS a trusted proxy, the real client (leftmost untrusted hop
// from the right) is extracted from the chain.
func TestClientIPHonorsTrustedProxyChain(t *testing.T) {
	r := newXFFResolver(true, false, "10.0.0.0/8")
	req := reqWith("10.0.0.1:5000", map[string]string{
		// client, then two trusted proxies.
		"X-Forwarded-For": "198.51.100.23, 10.0.0.9, 10.0.0.1",
	})
	if got := r.ClientIP(req); got != "198.51.100.23" {
		t.Fatalf("chain walk wrong: got %q, want 198.51.100.23", got)
	}
}

// An overlong forwarded chain is suspicious and must fall back to the peer.
func TestClientIPRejectsOverlongChain(t *testing.T) {
	r := newXFFResolver(true, false, "10.0.0.0/8")
	req := reqWith("10.0.0.1:5000", map[string]string{
		"X-Forwarded-For": "1.1.1.1, 2.2.2.2, 3.3.3.3, 4.4.4.4, 5.5.5.5, 6.6.6.6, 7.7.7.7",
	})
	if got := r.ClientIP(req); got != "10.0.0.1" {
		t.Fatalf("overlong chain not rejected: got %q, want peer 10.0.0.1", got)
	}
}

// X-Real-IP from a trusted proxy is honored.
func TestClientIPTrustsXRealIPFromProxy(t *testing.T) {
	r := newXFFResolver(false, true, "10.0.0.0/8")
	req := reqWith("10.0.0.1:5000", map[string]string{
		"X-Real-IP": "198.51.100.77",
	})
	if got := r.ClientIP(req); got != "198.51.100.77" {
		t.Fatalf("trusted X-Real-IP not honored: got %q, want 198.51.100.77", got)
	}
}

func TestClientIPCanonicalizesIPv4AndIPv6Loopback(t *testing.T) {
	r := newXFFResolver(false, false)
	for _, remoteAddr := range []string{"127.0.0.1:40000", "[::1]:40000"} {
		req := reqWith(remoteAddr, nil)
		if got := r.ClientIP(req); got != "127.0.0.1" {
			t.Errorf("ClientIP(%q) = %q, want canonical loopback 127.0.0.1", remoteAddr, got)
		}
	}
}

func TestIsPrivateIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"10.1.2.3":        true,
		"172.16.5.5":      true,
		"192.168.1.1":     true,
		"169.254.169.254": true, // link-local (cloud metadata)
		"::1":             true,
		"8.8.8.8":         false,
		"203.0.113.10":    false,
		"not-an-ip":       false,
	}
	for ip, want := range cases {
		if got := IsPrivateIP(ip); got != want {
			t.Errorf("IsPrivateIP(%q) = %v, want %v", ip, got, want)
		}
	}
}
