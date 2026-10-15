package waf

import (
	"context"
	_ "embed"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed admin_dashboard.html
var adminIndexHTML string

//go:embed admin_login.html
var adminLoginHTML string

// sessionCookie is the name of the cookie carrying the dashboard session token.
const sessionCookie = "nullgate_session"

// AdminServer exposes operational visibility: a live dashboard, a JSON
// metrics API, and a read-only view of the current blocklist. It listens on a
// separate address from the public proxy and is disabled by default — the
// operational surface must never share a port with the attack surface.
type AdminServer struct {
	cfg          AdminConfig
	metrics      *Metrics
	logger       *Logger
	blocklist    *Blocklist
	rules        *RuleEngine
	sessions     *SessionStore
	cfgRoot      *Config
	version      string
	ruleCount    int
	totalRules   int
	backendProbe func(context.Context) BackendHealth
}

// NewAdminServer wires the admin handler to live subsystems. The full config
// is passed so the dashboard can report the effective security posture; only
// non-secret fields are ever serialized out (see AdminInfo). The rule engine is
// held so the dashboard can retune the paranoia level live without a restart.
func NewAdminServer(cfg AdminConfig, root *Config, m *Metrics, l *Logger, bl *Blocklist, re *RuleEngine, version string, ruleCount, totalRules int, backendProbe func(context.Context) BackendHealth) *AdminServer {
	ttl := time.Duration(cfg.SessionTTLMin) * time.Minute
	sessions := NewSessionStore(ttl)
	go sessions.Cleanup(time.Minute)
	return &AdminServer{
		cfg:          cfg,
		metrics:      m,
		logger:       l,
		blocklist:    bl,
		rules:        re,
		sessions:     sessions,
		cfgRoot:      root,
		version:      version,
		ruleCount:    ruleCount,
		totalRules:   totalRules,
		backendProbe: backendProbe,
	}
}

// Handler returns the mux serving the admin endpoints. Auth-exempt routes
// (login page, login/logout, liveness) are registered on the outer mux; every
// other route is gated behind the session guard.
func (a *AdminServer) Handler() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("/", a.handleIndex)
	protected.HandleFunc("/api/status", a.handleStatus)
	protected.HandleFunc("/api/metrics", a.handleMetricsJSON)
	protected.HandleFunc("/api/events", a.handleEvents)
	protected.HandleFunc("/api/info", a.handleInfo)
	protected.HandleFunc("/api/overview", a.handleOverview)
	// Live control surface (all POST, all gated on ReadOnly).
	protected.HandleFunc("/api/admin/block-ip", a.handleBlockIP)
	protected.HandleFunc("/api/admin/unblock-ip", a.handleUnblockIP)
	protected.HandleFunc("/api/admin/set-paranoia", a.handleSetParanoia)
	protected.HandleFunc("/api/admin/blocklist", a.handleBlocklist)
	protected.HandleFunc("/api/admin/scan", a.handleScan)

	mux := http.NewServeMux()
	mux.HandleFunc("/login", a.handleLoginPage)
	mux.HandleFunc("/api/login", a.handleLogin)
	mux.HandleFunc("/api/logout", a.handleLogout)
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.Handle("/", a.requireSession(protected))

	return a.withLocalhost(mux)
}

// withLocalhost enforces loopback-only access when configured. It wraps the
// entire admin surface — including the login page — so the operational console
// is never reachable from the network unless the operator explicitly opts out.
func (a *AdminServer) withLocalhost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.RequireLocalhost {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			ip := net.ParseIP(strings.Trim(host, "[]"))
			if ip == nil || !ip.IsLoopback() {
				http.Error(w, "admin restricted to localhost", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireSession admits a request only when it carries a valid session cookie.
// When no password hash is configured the console is open (localhost-only dev
// mode); otherwise unauthenticated browsers are redirected to the login page
// and API calls receive a 401 so the dashboard JS can react.
func (a *AdminServer) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.PasswordHash == "" {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err == nil && a.sessions.Valid(c.Value) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *AdminServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminIndexHTML))
}

func (a *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := a.metrics.Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":        a.version,
		"uptime_seconds": snap.UptimeSeconds,
		"blocklist": map[string]interface{}{
			"entries":  a.blocklist.Len(),
			"log_only": a.blocklist.LogOnly(),
		},
		"rules": map[string]interface{}{
			"active": a.ruleCount,
			"total":  a.totalRules,
		},
		"tracked": map[string]interface{}{
			"clients": a.metrics.TotalTracked(),
		},
		"backend_health": a.backendHealth(r.Context()),
	})
}

func (a *AdminServer) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.metrics.Snapshot())
}

func (a *AdminServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": a.logger.RecentEvents(100),
	})
}

// AdminInfo is the non-secret, display-only view of the effective security
// posture. Secrets (HMAC secret, admin password hash) are deliberately never
// included so the JSON API can be consumed by the dashboard without leaking.
type AdminInfo struct {
	Version          string      `json:"version"`
	PublicListen     string      `json:"public_listen"`
	Backend          string      `json:"backend"`
	TLS              bool        `json:"tls"`
	FailClosed       bool        `json:"fail_closed"`
	ParanoiaLevel    int         `json:"paranoia_level"`
	AnomalyThreshold int         `json:"anomaly_threshold"`
	RulesActive      int         `json:"rules_active"`
	RulesTotal       int         `json:"rules_total"`
	Layers           []LayerInfo `json:"layers"`
}

// LayerInfo reports whether a single defense layer is enabled.
type LayerInfo struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// info assembles the non-secret posture view from the live config.
func (a *AdminServer) info() AdminInfo {
	c := a.cfgRoot
	inf := AdminInfo{
		Version:     a.version,
		FailClosed:  true,
		RulesActive: a.ruleCount,
		RulesTotal:  a.totalRules,
	}
	if c != nil {
		inf.PublicListen = c.Server.ListenAddr
		inf.Backend = c.Server.TargetBackend
		inf.TLS = c.Server.EnableTLS
		inf.ParanoiaLevel = c.Rules.ParanoiaLevel
		inf.AnomalyThreshold = c.Rules.AnomalyThreshold
		inf.Layers = []LayerInfo{
			{"protocol", true},
			{"decoding", true},
			{"rules", true},
			{"rate_limit", c.RateLimit.Enabled},
			{"pow", c.PoW.Enabled},
			{"bot", c.Bot.Enabled},
			{"behavior", c.Behavior.Enabled},
			{"csrf", c.CSRF.Enabled},
			{"tarpit", c.Tarpit.Enabled},
			{"response", c.Response.Enabled},
		}
	}
	return inf
}

func (a *AdminServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.info())
}

// handleOverview returns everything the dashboard needs in a single round-trip:
// posture info, the full metrics snapshot, blocklist state, and recent events.
// One request per refresh keeps the UI flicker-free and the socket quiet.
func (a *AdminServer) handleOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"info":    a.info(),
		"metrics": a.metrics.Snapshot(),
		"blocklist": map[string]interface{}{
			"entries":  a.blocklist.Len(),
			"log_only": a.blocklist.LogOnly(),
		},
		"tracked": map[string]interface{}{
			"clients": a.metrics.TotalTracked(),
		},
		"events":         a.logger.RecentEvents(100),
		"backend_health": a.backendHealth(r.Context()),
	})
}

func (a *AdminServer) backendHealth(ctx context.Context) BackendHealth {
	if a.backendProbe == nil {
		return BackendHealth{State: "unknown"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return a.backendProbe(probeCtx)
}

// handleHealthz is an unauthenticated-friendly liveness probe (still subject to
// the localhost guard) used by the dashboard's connection indicator.
func (a *AdminServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
