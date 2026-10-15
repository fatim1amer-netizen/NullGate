package waf

import (
	"testing"
	"time"
)

func TestBlocklistAddAndExpire(t *testing.T) {
	bl := NewBlocklist()
	ip := "203.0.113.70"

	bl.Add(ip, 50*time.Millisecond)
	if !bl.IsBlocked(ip) {
		t.Fatal("IP should be blocked immediately after Add")
	}
	time.Sleep(70 * time.Millisecond)
	if bl.IsBlocked(ip) {
		t.Error("block should have expired")
	}
}

func TestBlocklistZeroDurationNoOp(t *testing.T) {
	bl := NewBlocklist()
	bl.Add("203.0.113.71", 0)
	if bl.IsBlocked("203.0.113.71") {
		t.Error("zero-duration Add should not block")
	}
}

func TestBlocklistLenExcludesExpiredEntries(t *testing.T) {
	bl := NewBlocklist()
	bl.Add("203.0.113.75", 20*time.Millisecond)
	if bl.Len() != 1 {
		t.Fatalf("active blocklist length = %d, want 1", bl.Len())
	}
	bl.mu.Lock()
	bl.entries["203.0.113.75"] = time.Now().Add(-time.Second)
	bl.mu.Unlock()
	if bl.Len() != 0 {
		t.Fatalf("expired blocklist length = %d, want 0", bl.Len())
	}
}

func TestBlocklistCIDREntry(t *testing.T) {
	bl := NewBlocklist()
	bl.Add("10.0.0.0/8", time.Minute)
	if !bl.IsBlocked("10.5.6.7") {
		t.Error("IP within a blocked CIDR should be reported blocked")
	}
	if bl.IsBlocked("192.168.1.1") {
		t.Error("IP outside the blocked CIDR should not be blocked")
	}
}

func TestBlocklistLogOnlyMode(t *testing.T) {
	bl := NewBlocklist()
	bl.Add("203.0.113.72", time.Minute)
	bl.SetLogOnly(true)
	// IsBlocked still reports the raw state...
	if !bl.IsBlocked("203.0.113.72") {
		t.Error("IsBlocked should reflect raw state even in log-only mode")
	}
	// ...but Blocked (the enforcement gate) returns false.
	if bl.Blocked("203.0.113.72") {
		t.Error("log-only mode must not enforce blocks")
	}
}

// Behavioral escalation: enough consecutive blocks graduates a client to the
// shared blocklist.
func TestBehaviorEscalatesAfterConsecutiveBlocks(t *testing.T) {
	bl := NewBlocklist()
	bt := NewBehaviorTracker(BehaviorConfig{
		Enabled:                   true,
		ConsecutiveBlockThreshold: 3,
		CooldownSec:               60,
	}, bl)

	ip := "203.0.113.73"
	escalated := false
	for i := 0; i < 3; i++ {
		if bt.RecordBlock(ip) {
			escalated = true
		}
	}
	if !escalated {
		t.Fatal("client should escalate after crossing the consecutive-block threshold")
	}
	if !bl.IsBlocked(ip) {
		t.Error("escalated client should be on the shared blocklist")
	}
}

// A clean request decays the consecutive-block counter, so an occasional block
// does not eventually escalate a legitimate client.
func TestBehaviorAllowDecaysBlocks(t *testing.T) {
	bl := NewBlocklist()
	bt := NewBehaviorTracker(BehaviorConfig{
		Enabled:                   true,
		ConsecutiveBlockThreshold: 3,
		CooldownSec:               60,
	}, bl)

	ip := "203.0.113.74"
	// Two blocks, then a clean request repeatedly — should never escalate.
	for i := 0; i < 10; i++ {
		bt.RecordBlock(ip)
		bt.RecordBlock(ip)
		bt.RecordAllow(ip)
		bt.RecordAllow(ip)
	}
	if bl.IsBlocked(ip) {
		t.Error("interleaved clean requests should prevent escalation")
	}
}
