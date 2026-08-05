package waf

import (
	"testing"
	"time"
)

func rlConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:            true,
		RequestsPerSecond:  5,
		BurstSize:          5,
		EscalationAfter:    3,
		BlockAfter:         6,
		BlockDurationSec:   300,
		CleanupIntervalSec: 60,
		PerIP:              true,
	}
}

// A fresh client with a full bucket may make BurstSize requests immediately.
func TestRateLimiterAllowsBurst(t *testing.T) {
	rl := NewRateLimiter(rlConfig())
	ip := "203.0.113.1"
	for i := 0; i < 5; i++ {
		if got := rl.Check(ip); got != ActionAllow {
			t.Fatalf("request %d within burst should be allowed, got %v", i+1, got)
		}
	}
}

// After the bucket drains, sustained excess escalates Allow → PoW → Block.
func TestRateLimiterGraduatedEscalation(t *testing.T) {
	rl := NewRateLimiter(rlConfig())
	ip := "203.0.113.2"

	// Drain the burst.
	for i := 0; i < 5; i++ {
		rl.Check(ip)
	}

	sawPoW := false
	sawBlock := false
	// Hammer well past BlockAfter without letting tokens refill.
	for i := 0; i < 10; i++ {
		switch rl.Check(ip) {
		case ActionRequirePoW:
			sawPoW = true
		case ActionBlock:
			sawBlock = true
		}
	}
	if !sawPoW {
		t.Error("expected a RequirePoW verdict during escalation")
	}
	if !sawBlock {
		t.Error("expected a Block verdict after sustained abuse")
	}
	if !rl.Blocked(ip) {
		t.Error("client should be under a hard block after crossing BlockAfter")
	}
}

// Tokens refill over time, so a client that pauses recovers capacity.
func TestRateLimiterRefills(t *testing.T) {
	cfg := rlConfig()
	cfg.RequestsPerSecond = 100 // fast refill for a quick test
	cfg.BurstSize = 2
	rl := NewRateLimiter(cfg)
	ip := "203.0.113.3"

	if rl.Check(ip) != ActionAllow || rl.Check(ip) != ActionAllow {
		t.Fatal("first two requests should pass")
	}
	// Bucket now empty; wait for refill.
	time.Sleep(50 * time.Millisecond) // ~5 tokens at 100/s
	if got := rl.Check(ip); got != ActionAllow {
		t.Fatalf("request after refill should be allowed, got %v", got)
	}
}

// A disabled limiter is a pure pass-through.
func TestRateLimiterDisabled(t *testing.T) {
	cfg := rlConfig()
	cfg.Enabled = false
	rl := NewRateLimiter(cfg)
	for i := 0; i < 1000; i++ {
		if rl.Check("203.0.113.4") != ActionAllow {
			t.Fatal("disabled limiter must always allow")
		}
	}
	if rl.Blocked("203.0.113.4") {
		t.Fatal("disabled limiter must never report blocked")
	}
}

// Distinct clients get independent buckets — one abuser can't starve another.
func TestRateLimiterPerClientIsolation(t *testing.T) {
	rl := NewRateLimiter(rlConfig())
	abuser := "203.0.113.5"
	for i := 0; i < 20; i++ {
		rl.Check(abuser)
	}
	// A different client still has a full burst available.
	victim := "203.0.113.6"
	if got := rl.Check(victim); got != ActionAllow {
		t.Fatalf("second client wrongly affected by first's abuse, got %v", got)
	}
	if rl.TrackedClients() < 2 {
		t.Errorf("expected at least 2 tracked clients, got %d", rl.TrackedClients())
	}
}
