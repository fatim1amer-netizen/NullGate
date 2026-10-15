package waf

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects operational counters exposed in Prometheus text format at
// the configured metrics path. Everything is lock-free on the hot path
// (atomic counters) except the per-category and per-rule maps, which use a
// short-lived mutex only on write.
type Metrics struct {
	startTime time.Time

	totalRequests   atomic.Int64
	allowedRequests atomic.Int64
	blockedRequests atomic.Int64
	proxyErrors     atomic.Int64
	powChallenges   atomic.Int64
	rateLimited     atomic.Int64
	botBlocked      atomic.Int64
	protocolBlocked atomic.Int64
	csrfBlocked     atomic.Int64
	behaviorBlocked atomic.Int64

	latencyNanos atomic.Int64 // cumulative, for average
	latencyCount atomic.Int64

	trackedClients atomic.Int64 // gauge, refreshed by the engine periodically

	mu           sync.Mutex
	byCategory   map[string]int64
	byRule       map[string]int64
	byStatusCode map[int]int64
}

// NewMetrics returns an initialized metrics registry.
func NewMetrics() *Metrics {
	return &Metrics{
		startTime:    time.Now(),
		byCategory:   make(map[string]int64),
		byRule:       make(map[string]int64),
		byStatusCode: make(map[int]int64),
	}
}

// The Inc* helpers are called from the request path.
func (m *Metrics) IncTotal()       { m.totalRequests.Add(1) }
func (m *Metrics) IncAllowed()     { m.allowedRequests.Add(1) }
func (m *Metrics) IncBlocked()     { m.blockedRequests.Add(1) }
func (m *Metrics) IncProxyError()  { m.proxyErrors.Add(1) }
func (m *Metrics) IncPoW()         { m.powChallenges.Add(1) }
func (m *Metrics) IncRateLimited() { m.rateLimited.Add(1) }
func (m *Metrics) IncBot()         { m.botBlocked.Add(1) }
func (m *Metrics) IncProtocol()    { m.protocolBlocked.Add(1) }
func (m *Metrics) IncCSRF()        { m.csrfBlocked.Add(1) }
func (m *Metrics) IncBehavior()    { m.behaviorBlocked.Add(1) }

// ObserveLatency records a request duration for averaging.
func (m *Metrics) ObserveLatency(d time.Duration) {
	m.latencyNanos.Add(int64(d))
	m.latencyCount.Add(1)
}

// SetTracked updates the tracked-clients gauge (sum across subsystems).
func (m *Metrics) SetTracked(n int64) { m.trackedClients.Store(n) }

// TotalTracked returns the last-reported tracked-clients gauge value.
func (m *Metrics) TotalTracked() int64 { return m.trackedClients.Load() }

// RecordCategory increments the counter for an attack category and rule.
func (m *Metrics) RecordCategory(cat Category, ruleID string) {
	m.mu.Lock()
	m.byCategory[string(cat)]++
	if ruleID != "" {
		m.byRule[ruleID]++
	}
	m.mu.Unlock()
}

// RecordStatus records an upstream/response status code.
func (m *Metrics) RecordStatus(code int) {
	m.mu.Lock()
	m.byStatusCode[code]++
	m.mu.Unlock()
}

// Snapshot is a plain-data view of metrics for the admin dashboard/JSON API.
type Snapshot struct {
	UptimeSeconds   float64          `json:"uptime_seconds"`
	Total           int64            `json:"total_requests"`
	Allowed         int64            `json:"allowed_requests"`
	Blocked         int64            `json:"blocked_requests"`
	ProxyErrors     int64            `json:"proxy_errors"`
	PoWChallenges   int64            `json:"pow_challenges"`
	RateLimited     int64            `json:"rate_limited"`
	BotBlocked      int64            `json:"bot_blocked"`
	ProtocolBlocked int64            `json:"protocol_blocked"`
	CSRFBlocked     int64            `json:"csrf_blocked"`
	BehaviorBlocked int64            `json:"behavior_blocked"`
	AvgLatencyMS    float64          `json:"avg_latency_ms"`
	LatencySamples  int64            `json:"latency_samples"`
	BlockRate       float64          `json:"block_rate"`
	ByCategory      map[string]int64 `json:"by_category"`
	ByRule          map[string]int64 `json:"by_rule"`
	ByStatusCode    map[int]int64    `json:"by_status_code"`
}

// Snapshot captures a consistent copy of all counters.
func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	cat := make(map[string]int64, len(m.byCategory))
	for k, v := range m.byCategory {
		cat[k] = v
	}
	rule := make(map[string]int64, len(m.byRule))
	for k, v := range m.byRule {
		rule[k] = v
	}
	status := make(map[int]int64, len(m.byStatusCode))
	for k, v := range m.byStatusCode {
		status[k] = v
	}
	m.mu.Unlock()

	total := m.totalRequests.Load()
	blocked := m.blockedRequests.Load()
	var avgLatency, blockRate float64
	latencySamples := m.latencyCount.Load()
	if latencySamples > 0 {
		avgLatency = float64(m.latencyNanos.Load()) / float64(latencySamples) / 1e6
	}
	if total > 0 {
		blockRate = float64(blocked) / float64(total)
	}

	return Snapshot{
		UptimeSeconds:   time.Since(m.startTime).Seconds(),
		Total:           total,
		Allowed:         m.allowedRequests.Load(),
		Blocked:         blocked,
		ProxyErrors:     m.proxyErrors.Load(),
		PoWChallenges:   m.powChallenges.Load(),
		RateLimited:     m.rateLimited.Load(),
		BotBlocked:      m.botBlocked.Load(),
		ProtocolBlocked: m.protocolBlocked.Load(),
		CSRFBlocked:     m.csrfBlocked.Load(),
		BehaviorBlocked: m.behaviorBlocked.Load(),
		AvgLatencyMS:    avgLatency,
		LatencySamples:  latencySamples,
		BlockRate:       blockRate,
		ByCategory:      cat,
		ByRule:          rule,
		ByStatusCode:    status,
	}
}

// Prometheus renders the current metrics in Prometheus text exposition format.
func (m *Metrics) Prometheus() string {
	s := m.Snapshot()
	var b strings.Builder

	writeGauge := func(name, help string, val float64) {
		fmt.Fprintf(&b, "# HELP nullgate_%s %s\n", name, help)
		fmt.Fprintf(&b, "# TYPE nullgate_%s gauge\n", name)
		fmt.Fprintf(&b, "nullgate_%s %g\n", name, val)
	}
	writeCounter := func(name, help string, val int64) {
		fmt.Fprintf(&b, "# HELP nullgate_%s %s\n", name, help)
		fmt.Fprintf(&b, "# TYPE nullgate_%s counter\n", name)
		fmt.Fprintf(&b, "nullgate_%s %d\n", name, val)
	}

	writeGauge("uptime_seconds", "WAF uptime in seconds", s.UptimeSeconds)
	writeCounter("requests_total", "Total requests seen", s.Total)
	writeCounter("requests_allowed", "Requests forwarded to backend", s.Allowed)
	writeCounter("requests_blocked", "Requests blocked", s.Blocked)
	writeCounter("proxy_errors", "Upstream proxy errors", s.ProxyErrors)
	writeCounter("pow_challenges", "PoW challenges issued", s.PoWChallenges)
	writeCounter("rate_limited", "Requests rate-limited", s.RateLimited)
	writeCounter("bot_blocked", "Requests blocked as bots", s.BotBlocked)
	writeCounter("protocol_blocked", "Protocol violations blocked", s.ProtocolBlocked)
	writeCounter("csrf_blocked", "CSRF validations failed", s.CSRFBlocked)
	writeCounter("behavior_blocked", "Behavioral blocks", s.BehaviorBlocked)
	writeGauge("avg_latency_ms", "Average request latency in ms", s.AvgLatencyMS)
	writeGauge("block_rate", "Fraction of requests blocked", s.BlockRate)

	// Attack categories as a labeled counter.
	fmt.Fprintf(&b, "# HELP nullgate_attacks_by_category Attacks detected per category\n")
	fmt.Fprintf(&b, "# TYPE nullgate_attacks_by_category counter\n")
	cats := make([]string, 0, len(s.ByCategory))
	for k := range s.ByCategory {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	for _, k := range cats {
		fmt.Fprintf(&b, "nullgate_attacks_by_category{category=%q} %d\n", k, s.ByCategory[k])
	}

	return b.String()
}
