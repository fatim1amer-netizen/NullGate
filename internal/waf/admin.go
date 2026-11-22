package waf

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
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
	cfg        AdminConfig
	metrics    *Metrics
	logger     *Logger
	blocklist  *Blocklist
	rules      *RuleEngine
	sessions   *SessionStore
	cfgRoot    *Config
	version    string
	ruleCount  int
	totalRules int
	apps       *ApplicationManager
}

// SetApplicationManager enables app management on this admin server.
func (a *AdminServer) SetApplicationManager(apps *ApplicationManager) {
	a.apps = apps
}

// NewAdminServer wires the admin handler to live subsystems. The full config
// is passed so the dashboard can report the effective security posture; only
// non-secret fields are ever serialized out (see AdminInfo). The rule engine is
// held so the dashboard can retune the paranoia level live without a restart.
func NewAdminServer(cfg AdminConfig, root *Config, m *Metrics, l *Logger, bl *Blocklist, re *RuleEngine, version string, ruleCount, totalRules int) *AdminServer {
	ttl := time.Duration(cfg.SessionTTLMin) * time.Minute
	sessions := NewSessionStore(ttl)
	go sessions.Cleanup(time.Minute)
	return &AdminServer{
		cfg:        cfg,
		metrics:    m,
		logger:     l,
		blocklist:  bl,
		rules:      re,
		sessions:   sessions,
		cfgRoot:    root,
		version:    version,
		ruleCount:  ruleCount,
		totalRules: totalRules,
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
	protected.HandleFunc("/api/apps", a.handleApplications)
	protected.HandleFunc("/api/apps/metrics", a.handleApplicationMetrics)
	protected.HandleFunc("/api/apps/evaluation", a.handleApplicationEvaluation)
	// Live control surface (all POST, all gated on ReadOnly).
	protected.HandleFunc("/api/admin/block-ip", a.handleBlockIP)
	protected.HandleFunc("/api/admin/unblock-ip", a.handleUnblockIP)
	protected.HandleFunc("/api/admin/set-paranoia", a.handleSetParanoia)
	protected.HandleFunc("/api/admin/blocklist", a.handleBlocklist)
	protected.HandleFunc("/api/admin/scan", a.handleScan)
	protected.HandleFunc("/api/admin/apps/upsert", a.handleApplicationUpsert)
	protected.HandleFunc("/api/admin/apps/delete", a.handleApplicationDelete)
	protected.HandleFunc("/api/admin/apps/rules", a.handleApplicationRules)
	protected.HandleFunc("/api/admin/apps/compare", a.handleApplicationCompare)

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
			if isApplicationAdminPath(r.URL.Path) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "application APIs require admin password authentication"})
				return
			}
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

func isApplicationAdminPath(path string) bool {
	return strings.HasPrefix(path, "/api/apps") || strings.HasPrefix(path, "/api/admin/apps/")
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
			{"honeypot", c.Honeypot.Enabled},
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
		"events": a.logger.RecentEvents(100),
	})
}

func (a *AdminServer) handleApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	type appView struct {
		Application ProtectedApplication `json:"application"`
		Metrics     *Snapshot            `json:"metrics,omitempty"`
		Events      []Event              `json:"events,omitempty"`
	}
	apps := a.apps.Applications()
	views := make([]appView, 0, len(apps))
	for _, app := range apps {
		view := appView{Application: app}
		if metrics, ok := a.apps.ApplicationMetrics(app.ID); ok {
			view.Metrics = &metrics.Metrics
			view.Events = metrics.Events
		}
		views = append(views, view)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"applications": views,
		"rules":        a.apps.ApplicationRules(),
	})
}

func (a *AdminServer) handleApplicationMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	id := r.URL.Query().Get("id")
	metrics, ok := a.apps.ApplicationMetrics(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "application not found or disabled"})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

func (a *AdminServer) handleApplicationEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	results, err := a.apps.EvaluateApplications()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "application evaluation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"corpus":  "built-in labeled self-test corpus; not production traffic",
		"results": results,
	})
}

func (a *AdminServer) handleApplicationUpsert(w http.ResponseWriter, r *http.Request) {
	if !a.guardApplicationWrite(w, r) {
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	var app ProtectedApplication
	if err := decodeAdminJSON(w, r, &app, 32<<10); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid application request"})
		return
	}
	if app.ID == "" {
		id, err := NewApplicationID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate application ID"})
			return
		}
		app.ID = id
	}
	if err := a.apps.UpsertApplication(app); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.logger.Info("admin application saved", map[string]interface{}{
		"application_id": app.ID, "hostname": app.Hostname, "by": clientHost(r),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "saved", "application_id": app.ID})
}

func (a *AdminServer) handleApplicationDelete(w http.ResponseWriter, r *http.Request) {
	if !a.guardApplicationWrite(w, r) {
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeAdminJSON(w, r, &req, 4096); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid application ID"})
		return
	}
	if err := a.apps.DeleteApplication(req.ID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	a.logger.Warn("admin application removed", map[string]interface{}{
		"application_id": req.ID, "by": clientHost(r),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "removed", "application_id": req.ID})
}

func (a *AdminServer) handleApplicationRules(w http.ResponseWriter, r *http.Request) {
	if !a.guardApplicationWrite(w, r) {
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	var req struct {
		Rules []ApplicationRule `json:"rules"`
	}
	if err := decodeAdminJSON(w, r, &req, 64<<10); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid rule request"})
		return
	}
	if err := a.apps.SetRules(req.Rules); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.logger.Info("admin application rules updated", map[string]interface{}{
		"count": len(req.Rules), "by": clientHost(r),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "saved", "count": len(req.Rules)})
}

func (a *AdminServer) handleApplicationCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if a.cfg.PasswordHash == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "comparison requires admin password authentication"})
		return
	}
	ip := net.ParseIP(strings.Trim(clientHost(r), "[]"))
	if ip == nil || !ip.IsLoopback() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "demo comparisons are restricted to localhost"})
		return
	}
	if a.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "application manager unavailable"})
		return
	}
	var req struct {
		ApplicationID string `json:"application_id"`
		Method        string `json:"method"`
		Target        string `json:"target"`
		Body          string `json:"body"`
		ContentType   string `json:"content_type"`
	}
	if err := decodeAdminJSON(w, r, &req, 24<<10); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid comparison request"})
		return
	}
	result, err := a.apps.CompareDemo(req.ApplicationID, req.Method, req.Target, req.Body, req.ContentType)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *AdminServer) guardApplicationWrite(w http.ResponseWriter, r *http.Request) bool {
	if !a.guardWrite(w, r) {
		return false
	}
	if a.cfg.PasswordHash == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "application management requires admin password authentication"})
		return false
	}
	return true
}

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, dst interface{}, maxBytes int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request must contain exactly one JSON value")
	}
	return nil
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
