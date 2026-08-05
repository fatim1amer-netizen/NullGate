package waf

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// BehaviorTracker performs stateful, per-client behavioral analysis. Where the
// rule engine judges a single payload and the rate limiter judges request
// volume, this subsystem judges *patterns over time*: a client that trips many
// block rules in a row, sprays 404-bait paths, or cycles HTTP methods is
// behaving like a scanner even when no individual request is decisive.
type BehaviorTracker struct {
	cfg       BehaviorConfig
	mu        sync.Mutex
	clients   map[string]*behaviorState
	blocklist *Blocklist
}

type behaviorState struct {
	consecutiveBlocks int
	distinctPaths     map[string]struct{}
	distinctMethods   map[string]struct{}
	suspicionScore    int
	lastSeen          time.Time
	firstSeen         time.Time
	requestCount      int
}

// NewBehaviorTracker wires the tracker to a shared blocklist so that a client
// found to be a scanner is blocked across all subsystems.
func NewBehaviorTracker(cfg BehaviorConfig, bl *Blocklist) *BehaviorTracker {
	bt := &BehaviorTracker{
		cfg:       cfg,
		clients:   make(map[string]*behaviorState),
		blocklist: bl,
	}
	if cfg.Enabled {
		go bt.cleanup()
	}
	return bt
}

// Observe records a request and returns true if the client should be blocked
// as a result of its accumulated behavior. It also records path/method spread
// and suspicious-UA signals.
func (bt *BehaviorTracker) Observe(r *http.Request, clientIP string) (blocked bool, reason string) {
	if !bt.cfg.Enabled {
		return false, ""
	}

	now := time.Now()
	bt.mu.Lock()
	defer bt.mu.Unlock()

	st, ok := bt.clients[clientIP]
	if !ok {
		st = &behaviorState{
			distinctPaths:   make(map[string]struct{}),
			distinctMethods: make(map[string]struct{}),
			firstSeen:       now,
		}
		bt.clients[clientIP] = st
	}
	st.lastSeen = now
	st.requestCount++
	st.distinctPaths[r.URL.Path] = struct{}{}
	st.distinctMethods[r.Method] = struct{}{}

	// Deep, extension-laden paths are traversal/probe fingerprints.
	if bt.cfg.MaxPathDepth > 0 {
		if depth := strings.Count(strings.Trim(r.URL.Path, "/"), "/") + 1; depth > bt.cfg.MaxPathDepth {
			st.suspicionScore += 2
		}
	}
	if bt.cfg.MaxExtensionsInPath > 0 {
		if exts := strings.Count(r.URL.Path, "."); exts > bt.cfg.MaxExtensionsInPath {
			st.suspicionScore += 2
		}
	}

	// Suspicious or empty UA adds a small amount of suspicion.
	if bt.cfg.TrackEmptyUA && strings.TrimSpace(r.UserAgent()) == "" {
		st.suspicionScore++
	}
	if bt.cfg.TrackSuspiciousUA && looksSuspiciousUA(r.UserAgent()) {
		st.suspicionScore += 2
	}

	// A client cycling many methods over few paths looks like fuzzing.
	if bt.cfg.TrackMethods && len(st.distinctMethods) >= 5 {
		st.suspicionScore += 3
	}

	// High path spread in a short window is directory-bruteforce behavior.
	if len(st.distinctPaths) > 50 && now.Sub(st.firstSeen) < time.Minute {
		st.suspicionScore += 5
	}

	if bt.cfg.BlockScannerProbe && st.suspicionScore >= 8 {
		bt.blocklist.Add(clientIP, time.Duration(bt.cfg.ScannerBlockSec)*time.Second)
		return true, "behavioral: scanner pattern (score high)"
	}

	return false, ""
}

// RecordBlock is called when another subsystem blocks a request from clientIP.
// Enough consecutive blocks graduates the client to the shared blocklist,
// stopping the WAF from having to re-evaluate an obviously hostile client.
func (bt *BehaviorTracker) RecordBlock(clientIP string) (escalated bool) {
	if !bt.cfg.Enabled {
		return false
	}
	bt.mu.Lock()
	defer bt.mu.Unlock()

	st, ok := bt.clients[clientIP]
	if !ok {
		st = &behaviorState{
			distinctPaths:   make(map[string]struct{}),
			distinctMethods: make(map[string]struct{}),
			firstSeen:       time.Now(),
		}
		bt.clients[clientIP] = st
	}
	st.consecutiveBlocks++
	st.lastSeen = time.Now()

	if st.consecutiveBlocks >= bt.cfg.ConsecutiveBlockThreshold {
		bt.blocklist.Add(clientIP, time.Duration(bt.cfg.CooldownSec)*time.Second)
		st.consecutiveBlocks = 0
		return true
	}
	return false
}

// RecordAllow resets the consecutive-block counter after a clean request, so a
// client is not permanently penalized for a burst of past mistakes.
func (bt *BehaviorTracker) RecordAllow(clientIP string) {
	if !bt.cfg.Enabled {
		return
	}
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if st, ok := bt.clients[clientIP]; ok && st.consecutiveBlocks > 0 {
		st.consecutiveBlocks--
	}
}

// TrackedClients returns the number of clients under behavioral observation.
func (bt *BehaviorTracker) TrackedClients() int {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return len(bt.clients)
}

func looksSuspiciousUA(ua string) bool {
	ua = strings.ToLower(ua)
	if ua == "" {
		return false // handled separately as empty
	}
	// Very short or obviously templated UAs.
	if len(ua) < 8 {
		return true
	}
	suspicious := []string{"scan", "test", "probe", "attack", "exploit", "payload", "<", ">", "'", "\""}
	for _, s := range suspicious {
		if strings.Contains(ua, s) {
			return true
		}
	}
	return false
}

func (bt *BehaviorTracker) cleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		bt.mu.Lock()
		for ip, st := range bt.clients {
			if now.Sub(st.lastSeen) > 10*time.Minute {
				delete(bt.clients, ip)
			}
		}
		bt.mu.Unlock()
	}
}
