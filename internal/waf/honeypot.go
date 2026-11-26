package waf

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// Honeypot traps clients that request paths no legitimate user of the
// protected app would ever visit — /wp-login.php on a Go backend, /.env,
// /.git/config, and similar recon bait. A single hit is high-signal: the
// client is enumerating known-vulnerable endpoints, so we block it outright.
type Honeypot struct {
	cfg       HoneypotConfig
	blocklist *Blocklist
	exact     map[string]bool
	prefixes  []string
}

// NewHoneypot compiles the probe-path set into fast exact/prefix lookups.
func NewHoneypot(cfg HoneypotConfig, bl *Blocklist) *Honeypot {
	h := &Honeypot{cfg: cfg, blocklist: bl, exact: make(map[string]bool)}
	for _, p := range cfg.ProbePaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/") {
			h.prefixes = append(h.prefixes, p)
		} else {
			h.exact[p] = true
		}
	}
	return h
}

// Triggered reports whether the request path is a honeypot probe. When it is
// (and not in log-only mode) the offending IP is added to the shared blocklist.
func (h *Honeypot) Triggered(r *http.Request, clientIP string) (bool, string) {
	if !h.cfg.Enabled {
		return false, ""
	}
	path := r.URL.Path
	hit := h.exact[path]
	if !hit {
		lower := strings.ToLower(path)
		for _, pre := range h.prefixes {
			if strings.HasPrefix(lower, strings.ToLower(pre)) {
				hit = true
				break
			}
		}
	}
	if !hit {
		return false, ""
	}
	if !h.cfg.LogOnly {
		h.blocklist.Add(clientIP, time.Duration(h.cfg.BlockDurationSec)*time.Second)
	}
	return true, "honeypot probe: " + path
}

// Tarpit deliberately slows abusive clients. Unlike a hard block (which frees
// the attacker to move on immediately), a tarpit ties up the attacker's own
// connection resources with escalating delays, raising their cost while barely
// touching ours. Delay grows with how far over a rolling request budget the
// client is.
type Tarpit struct {
	cfg     TarpitConfig
	stop    <-chan struct{}
	mu      sync.Mutex
	windows map[string]*tarpitWindow
}

type tarpitWindow struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

// NewTarpit constructs a tarpit and starts its cleanup loop.
func NewTarpit(cfg TarpitConfig, stopChannels ...<-chan struct{}) *Tarpit {
	var stop <-chan struct{}
	if len(stopChannels) > 0 {
		stop = stopChannels[0]
	}
	t := &Tarpit{cfg: cfg, stop: stop, windows: make(map[string]*tarpitWindow)}
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
	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
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
}
