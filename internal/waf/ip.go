package waf

import (
	"net"
	"net/http"
	"strings"
)

// IPResolver determines the true client IP for a request. This is a
// security-critical component: every per-client control (rate limiting,
// blocklists, PoW, behavioral tracking) keys off the value it returns.
//
// The naive approach — blindly trusting X-Forwarded-For — lets any client
// spoof its identity and evade every per-IP defense. NullGate therefore only
// honors forwarding headers when the immediate peer is a configured trusted
// proxy, and walks the forwarded chain right-to-left skipping trusted hops.
type IPResolver struct {
	trustXFF     bool
	trustXRealIP bool
	maxChain     int
	trustedNets  []*net.IPNet
	trustedExact map[string]bool
}

// NewIPResolver compiles the trusted-proxy CIDR/host list from config.
func NewIPResolver(cfg IPConfig, serverTrusted []string) *IPResolver {
	r := &IPResolver{
		trustXFF:     cfg.TrustXForwardedFor,
		trustXRealIP: cfg.TrustXRealIP,
		maxChain:     cfg.MaxForwardedChain,
		trustedExact: make(map[string]bool),
	}
	if r.maxChain <= 0 {
		r.maxChain = 5
	}

	all := append([]string{}, cfg.TrustedProxies...)
	all = append(all, serverTrusted...)
	for _, entry := range all {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			r.trustedNets = append(r.trustedNets, ipnet)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			r.trustedExact[ip.String()] = true
		}
	}
	return r
}

// ClientIP returns the best-effort real client IP as a string.
func (r *IPResolver) ClientIP(req *http.Request) string {
	peer := peerIP(req.RemoteAddr)

	// If we don't trust any forwarding headers, or the direct peer isn't a
	// trusted proxy, the peer address is the only trustworthy identity.
	if (!r.trustXFF && !r.trustXRealIP) || !r.isTrusted(peer) {
		return clientIdentity(peer)
	}

	// X-Real-IP is a single value set by the trusted proxy.
	if r.trustXRealIP {
		if xri := strings.TrimSpace(req.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return clientIdentity(ip)
			}
		}
	}

	// X-Forwarded-For is "client, proxy1, proxy2". Walk from the right,
	// discarding trusted proxies, and return the first untrusted address —
	// that is the closest hop we cannot vouch for, i.e. the real client edge.
	if r.trustXFF {
		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > r.maxChain {
				// Overlong chains are suspicious; fall back to the peer.
				return peer.String()
			}
			for i := len(parts) - 1; i >= 0; i-- {
				candidate := net.ParseIP(strings.TrimSpace(parts[i]))
				if candidate == nil {
					continue
				}
				if r.isTrusted(candidate) {
					continue
				}
				return clientIdentity(candidate)
			}
		}
	}

	return clientIdentity(peer)
}

// clientIdentity gives IPv4 and IPv6 loopback connections one stable identity.
// Browsers commonly resolve localhost to ::1 while command-line clients use
// 127.0.0.1; per-client controls should not treat them as separate local users.
func clientIdentity(ip net.IP) string {
	if ip != nil && ip.IsLoopback() {
		return net.IPv4(127, 0, 0, 1).String()
	}
	return ip.String()
}

// isTrusted reports whether an IP belongs to the trusted-proxy set.
func (r *IPResolver) isTrusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if r.trustedExact[ip.String()] {
		return true
	}
	for _, n := range r.trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// peerIP extracts the IP portion from a RemoteAddr ("ip:port"), tolerating
// IPv6 bracket notation and missing ports.
func peerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.Trim(host, "[]")
	return net.ParseIP(host)
}

// cidrContains reports whether key (an IP or CIDR string) covers ipStr.
// A plain-IP key matches only its exact equal; a CIDR key matches by network
// containment. Malformed keys never match.
func cidrContains(key, ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if _, ipnet, err := net.ParseCIDR(key); err == nil {
		return ipnet.Contains(ip)
	}
	if k := net.ParseIP(key); k != nil {
		return k.Equal(ip)
	}
	return false
}

// IsPrivateIP reports whether an address is loopback, link-local, or in an
// RFC1918 private range — used by SSRF-aware logic and admin binding checks.
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return ip.IsPrivate()
}
