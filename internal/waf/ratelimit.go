package waf

import (
	"sync"
	"time"
)

// Action is the rate limiter's verdict for a request.
type Action int

const (
	ActionAllow Action = iota
	ActionRequirePoW
	ActionBlock
)

// RateLimiter implements a per-client token-bucket limiter with graduated
// response: normal traffic passes, elevated traffic is challenged with
// Proof-of-Work, and sustained abuse is hard-blocked for a cooldown window.
//
// The original implementation reset a counter every second and never honored
// burst size or block duration; this version uses a continuously-refilling
// token bucket, which smooths bursts correctly and cannot be gamed by aligning
// requests to second boundaries.
type RateLimiter struct {
	cfg     RateLimitConfig
	stop    <-chan struct{}
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens      float64   // current tokens available
	lastRefill  time.Time // last time tokens were added
	violations  int       // consecutive over-limit hits (drives escalation)
	blockedTill time.Time // hard-block expiry
	lastSeen    time.Time // for cleanup
}

// NewRateLimiter constructs a limiter and starts its cleanup loop.
func NewRateLimiter(cfg RateLimitConfig, stopChannels ...<-chan struct{}) *RateLimiter {
	var stop <-chan struct{}
	if len(stopChannels) > 0 {
		stop = stopChannels[0]
	}
	rl := &RateLimiter{cfg: cfg, stop: stop, buckets: make(map[string]*bucket)}
	if cfg.Enabled {
		interval := time.Duration(cfg.CleanupIntervalSec) * time.Second
		if interval <= 0 {
			interval = time.Minute
		}
		go rl.cleanup(interval)
	}
	return rl
}

// Check consumes one token for the client and returns the action to take.
func (rl *RateLimiter) Check(clientIP string) Action {
	if !rl.cfg.Enabled {
		return ActionAllow
	}

	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[clientIP]
	if !ok {
		b = &bucket{
			tokens:     float64(rl.cfg.BurstSize),
			lastRefill: now,
			lastSeen:   now,
		}
		rl.buckets[clientIP] = b
	}
	b.lastSeen = now

	// Honor an active hard block.
	if now.Before(b.blockedTill) {
		return ActionBlock
	}

	// Refill tokens based on elapsed time and the configured rate.
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * float64(rl.cfg.RequestsPerSecond)
	if b.tokens > float64(rl.cfg.BurstSize) {
		b.tokens = float64(rl.cfg.BurstSize)
	}
	b.lastRefill = now

	// A request costs one token. If tokens remain, allow and decay violations.
	if b.tokens >= 1 {
		b.tokens--
		if b.violations > 0 {
			b.violations--
		}
		return ActionAllow
	}

	// Out of tokens: this is a violation. Escalate based on how persistent it is.
	b.violations++

	if b.violations >= rl.cfg.BlockAfter {
		b.blockedTill = now.Add(time.Duration(rl.cfg.BlockDurationSec) * time.Second)
		return ActionBlock
	}
	if b.violations >= rl.cfg.EscalationAfter {
		return ActionRequirePoW
	}
	// Below escalation threshold we still deny the excess request.
	return ActionBlock
}

// Blocked reports whether the client is currently under a hard block without
// consuming a token (used for fast pre-checks).
func (rl *RateLimiter) Blocked(clientIP string) bool {
	if !rl.cfg.Enabled {
		return false
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[clientIP]
	if !ok {
		return false
	}
	return time.Now().Before(b.blockedTill)
}

// TrackedClients returns the number of clients currently held in memory.
func (rl *RateLimiter) TrackedClients() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.buckets)
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			now := time.Now()
			rl.mu.Lock()
			for ip, b := range rl.buckets {
				// Drop clients idle for 5+ minutes that are not actively blocked.
				if now.Sub(b.lastSeen) > 5*time.Minute && now.After(b.blockedTill) {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}
