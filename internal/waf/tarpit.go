package waf

import (
	"sync"
	"time"
)

// Tarpit deliberately slows persistent high-volume clients. Delay grows with
// how far over a rolling request budget the client is.
type Tarpit struct {
	cfg     TarpitConfig
	mu      sync.Mutex
	windows map[string]*tarpitWindow
}

type tarpitWindow struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

// NewTarpit constructs a tarpit and starts its cleanup loop.
func NewTarpit(cfg TarpitConfig) *Tarpit {
	t := &Tarpit{cfg: cfg, windows: make(map[string]*tarpitWindow)}
	if cfg.Enabled {
		go t.cleanup()
	}
	return t
}

// Delay returns how long the caller should sleep before serving this client.
// Zero means no delay. The delay scales linearly with the overage above the
// per-window threshold, capped at MaxDelayMS.
func (t *Tarpit) Delay(clientIP string) time.Duration {
	if !t.cfg.Enabled {
		return 0
	}
	now := time.Now()
	window := time.Duration(t.cfg.WindowSec) * time.Second
	if window <= 0 {
		window = 10 * time.Second
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	w, ok := t.windows[clientIP]
	if !ok || now.Sub(w.windowStart) > window {
		t.windows[clientIP] = &tarpitWindow{count: 1, windowStart: now, lastSeen: now}
		return 0
	}
	w.count++
	w.lastSeen = now

	if w.count <= t.cfg.RequestCountThreshold {
		return 0
	}

	over := w.count - t.cfg.RequestCountThreshold
	delayMS := t.cfg.MinDelayMS + over*t.cfg.StepMS
	if delayMS > t.cfg.MaxDelayMS {
		delayMS = t.cfg.MaxDelayMS
	}
	return time.Duration(delayMS) * time.Millisecond
}

func (t *Tarpit) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		t.mu.Lock()
		for ip, w := range t.windows {
			if now.Sub(w.lastSeen) > 5*time.Minute {
				delete(t.windows, ip)
			}
		}
		t.mu.Unlock()
	}
}
