package waf

import (
	"sync"
	"time"
)

// Blocklist is a shared in-memory store of IP addresses and CIDRs that are
// temporarily blocked by the behavioral subsystem. Entries expire
// automatically; the Cleanup goroutine keeps the
// table from growing without bound.
//
// Blocklists deliberately never persist: transient blocks from suspicious
// behavior should decay on restart so a legitimately-shared IP (e.g. a NAT
// gateway) is not permanently cut off by a past false positive.
type Blocklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // key -> expiry
	enabled bool
	logOnly bool
}

// NewBlocklist creates an empty, enabled blocklist.
func NewBlocklist() *Blocklist {
	return &Blocklist{entries: make(map[string]time.Time), enabled: true}
}

// SetLogOnly toggles the blocklist into a "log everything, block nothing"
// mode, useful for tuning before going live.
func (b *Blocklist) SetLogOnly(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logOnly = v
}

// LogOnly reports the current log-only mode.
func (b *Blocklist) LogOnly() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.logOnly
}

// Add records a block for an IP or CIDR lasting for the given duration.
func (b *Blocklist) Add(ip string, duration time.Duration) {
	if duration <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[ip] = time.Now().Add(duration)
}

// IsBlocked reports whether the given IP (or any covering CIDR) is blocked.
func (b *Blocklist) IsBlocked(ip string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	exp, ok := b.entries[ip]
	if ok {
		if time.Now().After(exp) {
			return false // expired; will be reaped by Cleanup
		}
		return true
	}
	// Also check CIDR entries (best-effort containment test).
	for key, exp := range b.entries {
		if time.Now().After(exp) {
			continue
		}
		if cidrContains(key, ip) {
			return true
		}
	}
	return false
}

// Blocked reports whether the blocklist is currently enforcing a block on ip.
// Unlike IsBlocked, it respects log-only mode (blocking is disabled).
func (b *Blocklist) Blocked(ip string) bool {
	if !b.enabled || b.logOnly {
		return false
	}
	return b.IsBlocked(ip)
}

// Len returns the current number of entries (including expired, pre-cleanup).
func (b *Blocklist) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

// Remove deletes an entry from the blocklist immediately (manual unblock).
func (b *Blocklist) Remove(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, ip)
}

// Entries returns a snapshot of currently blocked IPs (active and expired).
// Expired entries stay until Cleanup removes them, so callers should check the
// expiry time. This is used by the admin dashboard to display the blocklist.
func (b *Blocklist) Entries() map[string]time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snap := make(map[string]time.Time, len(b.entries))
	for k, v := range b.entries {
		snap[k] = v
	}
	return snap
}

// Cleanup periodically removes expired entries. It must be started once.
func (b *Blocklist) Cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		b.mu.Lock()
		for k, exp := range b.entries {
			if now.After(exp) {
				delete(b.entries, k)
			}
		}
		b.mu.Unlock()
	}
}
