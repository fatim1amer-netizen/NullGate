package waf

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

var applicationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// ApplicationPolicy contains the per-application overrides. Zero values use
// the corresponding global configuration unless a profile is selected.
type ApplicationPolicy struct {
	Profile           string `json:"profile,omitempty"`
	ParanoiaLevel     int    `json:"paranoia_level,omitempty"`
	AnomalyThreshold  int    `json:"anomaly_threshold,omitempty"`
	RateLimitEnabled  *bool  `json:"rate_limit_enabled,omitempty"`
	RequestsPerSecond int    `json:"requests_per_second,omitempty"`
	BurstSize         int    `json:"burst_size,omitempty"`
}

// ProtectedApplication is a trusted host-to-upstream mapping. The upstream is
// never taken from an incoming request.
type ProtectedApplication struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Hostname     string            `json:"hostname"`
	BackendURL   string            `json:"backend_url"`
	DemoKind     string            `json:"demo_kind,omitempty"`
	InternalKind string            `json:"internal_kind,omitempty"`
	Enabled      bool              `json:"enabled"`
	Policy       ApplicationPolicy `json:"policy"`
}

// ApplicationRule can be shared globally or assigned to a specific set of
// applications. Built-in rules are global and cannot be disabled per app.
type ApplicationRule struct {
	ID             string   `json:"id"`
	Pattern        string   `json:"pattern"`
	Global         bool     `json:"global,omitempty"`
	ApplicationIDs []string `json:"application_ids,omitempty"`
}

// ApplicationRegistry is the atomically persisted administrator-managed state.
type ApplicationRegistry struct {
	Version        int                    `json:"version"`
	LegacyFallback bool                   `json:"legacy_fallback,omitempty"`
	Applications   []ProtectedApplication `json:"applications"`
	Rules          []ApplicationRule      `json:"rules,omitempty"`
}

type managedApplication struct {
	config     ProtectedApplication
	engine     *Engine
	demo       http.Handler
	comparable bool
	hash       string
}

type applicationSnapshot struct {
	byHost         map[string]*managedApplication
	byID           map[string]*managedApplication
	legacyFallback bool
}

// ApplicationManager owns persistent configuration and publishes immutable
// routing snapshots only after validation, engine construction, and disk write.
type ApplicationManager struct {
	cfg      *Config
	legacy   http.Handler
	path     string
	allowed  map[string]struct{}
	internal map[string]http.Handler
	mu       sync.Mutex
	registry ApplicationRegistry
	snapshot atomic.Pointer[applicationSnapshot]
}

// NewApplicationManager loads the registry and builds all enabled app engines.
// A missing registry file is an empty registry, which keeps the legacy
// single-backend configuration active until apps are explicitly registered.
func NewApplicationManager(cfg *Config, legacy http.Handler, internalBackends ...map[string]http.Handler) (*ApplicationManager, error) {
	if cfg == nil || legacy == nil {
		return nil, fmt.Errorf("application manager requires config and legacy handler")
	}
	path := cfg.Applications.RegistryPath
	if path == "" {
		path = "data/apps.json"
	}
	m := &ApplicationManager{
		cfg:      cfg,
		legacy:   legacy,
		path:     path,
		allowed:  make(map[string]struct{}),
		internal: make(map[string]http.Handler),
	}
	if len(internalBackends) > 0 {
		for kind, handler := range internalBackends[0] {
			if handler != nil {
				m.internal[kind] = handler
			}
		}
	}
	if authority := normalizeUpstreamAuthority(cfg.Server.TargetBackend); authority != "" {
		m.allowed[authority] = struct{}{}
	}
	for _, host := range cfg.Applications.AllowedUpstreamHosts {
		authorities, err := normalizeAllowedUpstreamHost(host)
		if err != nil {
			return nil, fmt.Errorf("invalid applications.allowed_upstream_hosts entry %q: %w", host, err)
		}
		for _, authority := range authorities {
			m.allowed[authority] = struct{}{}
		}
	}

	registry, err := readApplicationRegistry(path)
	if err != nil {
		return nil, err
	}
	if registry.Version == 0 {
		registry.Version = 1
		registry.LegacyFallback = true
	}
	snapshot, err := m.buildSnapshot(registry)
	if err != nil {
		return nil, fmt.Errorf("invalid application registry: %w", err)
	}
	m.registry = registry
	m.snapshot.Store(snapshot)
	return m, nil
}

func readApplicationRegistry(path string) (ApplicationRegistry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ApplicationRegistry{Version: 1, LegacyFallback: true}, nil
	}
	if err != nil {
		return ApplicationRegistry{}, fmt.Errorf("read application registry: %w", err)
	}
	var registry ApplicationRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return ApplicationRegistry{}, fmt.Errorf("parse application registry: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ApplicationRegistry{}, fmt.Errorf("parse application registry: expected one JSON value")
	}
	if registry.Version != 1 {
		return ApplicationRegistry{}, fmt.Errorf("unsupported application registry version %d", registry.Version)
	}
	if registry.Applications == nil {
		registry.Applications = []ProtectedApplication{}
	}
	return registry, nil
}

// ServeHTTP routes by the validated request hostname. Once an application is
// registered, unknown hosts are rejected instead of falling through to the
// legacy upstream.
func (m *ApplicationManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if serveLocalHealthz(w, r) {
		return
	}
	snapshot := m.snapshot.Load()
	if snapshot == nil || (len(snapshot.byHost) == 0 && snapshot.legacyFallback) {
		m.legacy.ServeHTTP(w, r)
		return
	}
	host, err := requestHostname(r.Host)
	if err != nil {
		http.Error(w, "Misdirected Request", http.StatusMisdirectedRequest)
		return
	}
	if r.TLS != nil && r.TLS.ServerName != "" {
		sni, sniErr := normalizeHostname(r.TLS.ServerName)
		if sniErr != nil || sni != host {
			http.Error(w, "Misdirected Request", http.StatusMisdirectedRequest)
			return
		}
	}
	if (host == "localhost" || host == "127.0.0.1" || host == "::1") &&
		r.URL.Path == "/" && isLoopbackRequest(r) {
		if app, ok := snapshot.byID["original-site"]; ok && app.engine != nil && app.config.InternalKind == "original-site" {
			http.Redirect(w, r, "/original/", http.StatusFound)
			return
		}
	}
	if app, routedRequest := m.localShowcaseRoute(snapshot, host, r); app != nil {
		app.engine.ServeHTTP(w, routedRequest)
		return
	}
	app, ok := snapshot.byHost[host]
	if !ok {
		http.Error(w, "Misdirected Request", http.StatusMisdirectedRequest)
		return
	}
	if app.engine == nil {
		http.NotFound(w, r)
		return
	}
	app.engine.ServeHTTP(w, r)
}

func (m *ApplicationManager) localShowcaseRoute(snapshot *applicationSnapshot, host string, r *http.Request) (*managedApplication, *http.Request) {
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, nil
	}
	if !isLoopbackRequest(r) {
		return nil, nil
	}
	var appID, prefix string
	switch {
	case r.URL.Path == "/original" || r.URL.Path == "/original/":
		appID, prefix = "original-site", "/original"
	case r.URL.Path == "/cyberlab" || r.URL.Path == "/cyberlab/":
		appID, prefix = "cyber-attack-lab", "/cyberlab"
	case strings.HasPrefix(r.URL.Path, "/original/"):
		appID, prefix = "original-site", "/original"
	case strings.HasPrefix(r.URL.Path, "/cyberlab/"):
		appID, prefix = "cyber-attack-lab", "/cyberlab"
	default:
		return nil, nil
	}
	app, ok := snapshot.byID[appID]
	if !ok || app.engine == nil || app.config.InternalKind == "" {
		return nil, nil
	}
	routed := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = strings.TrimPrefix(urlCopy.Path, prefix)
	if urlCopy.Path == "" {
		urlCopy.Path = "/"
	}
	urlCopy.RawPath = ""
	routed.URL = &urlCopy
	routed.Host = app.config.Hostname
	rewriteLocalShowcaseReferer(routed, prefix, app.config.Hostname)
	return app, routed
}

func rewriteLocalShowcaseReferer(r *http.Request, prefix, appHostname string) {
	referer, err := url.Parse(r.Header.Get("Referer"))
	if err != nil || referer.User != nil ||
		(referer.Scheme != "http" && referer.Scheme != "https") {
		return
	}
	host := strings.ToLower(strings.TrimSuffix(referer.Hostname(), "."))
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return
	}
	referer.Host = appHostname
	if referer.Path == prefix || referer.Path == prefix+"/" {
		referer.Path = "/"
	} else if strings.HasPrefix(referer.Path, prefix+"/") {
		referer.Path = strings.TrimPrefix(referer.Path, prefix)
	}
	referer.RawPath = ""
	r.Header.Set("Referer", referer.String())
}

func isLoopbackRequest(r *http.Request) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	remoteIP := net.ParseIP(strings.Trim(remoteHost, "[]"))
	return err == nil && remoteIP != nil && remoteIP.IsLoopback()
}

func serveLocalHealthz(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/healthz" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "Not Found", http.StatusNotFound)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}
	return true
}

// Close releases the app engines owned by this manager.
func (m *ApplicationManager) Close() {
	closeApplicationSnapshot(m.snapshot.Load())
}

// Applications returns a copy of the active registry entries.
func (m *ApplicationManager) Applications() []ProtectedApplication {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneRegistry(m.registry).Applications
}

// ApplicationMetrics returns the live metrics for an application.
func (m *ApplicationManager) ApplicationMetrics(id string) (MetricsSnapshot, bool) {
	snapshot := m.snapshot.Load()
	if snapshot == nil {
		return MetricsSnapshot{}, false
	}
	app, ok := snapshot.byID[id]
	if !ok || app.engine == nil {
		return MetricsSnapshot{}, false
	}
	return MetricsSnapshot{
		ApplicationID: id,
		Metrics:       app.engine.Metrics().Snapshot(),
		Events:        app.engine.Logger().RecentEvents(100),
	}, true
}

// MetricsSnapshot keeps app metrics and security events bound to their app ID.
type MetricsSnapshot struct {
	ApplicationID string   `json:"application_id"`
	Metrics       Snapshot `json:"metrics"`
	Events        []Event  `json:"events"`
}

// ApplicationEvaluation reports measured false-positive test outcomes against
// the built-in labeled self-test corpus for one app's effective policy.
type ApplicationEvaluation struct {
	ApplicationID     string          `json:"application_id"`
	Precision         float64         `json:"precision"`
	Recall            float64         `json:"recall"`
	FalsePositiveRate float64         `json:"false_positive_rate"`
	F1Score           float64         `json:"f1_score"`
	Report            *SelfTestReport `json:"report"`
}

func (m *ApplicationManager) EvaluateApplications() ([]ApplicationEvaluation, error) {
	m.mu.Lock()
	registry := cloneRegistry(m.registry)
	m.mu.Unlock()
	results := make([]ApplicationEvaluation, 0, len(registry.Applications))
	for _, app := range registry.Applications {
		cfg, err := cloneConfig(m.cfg)
		if err != nil {
			return nil, err
		}
		applyApplicationPolicy(cfg, app.Policy)
		cfg.Rules.CustomPatterns = append(cfg.Rules.CustomPatterns, m.patternsForApp(registry.Rules, app.ID)...)
		rules := NewRuleEngine(cfg.Rules.ParanoiaLevel, cfg.Rules.CustomPatterns)
		report := evaluateSelfTest(cfg, rules)
		results = append(results, ApplicationEvaluation{
			ApplicationID:     app.ID,
			Precision:         report.Precision(),
			Recall:            report.Recall(),
			FalsePositiveRate: report.FalsePositiveRate(),
			F1Score:           report.F1Score(),
			Report:            report,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ApplicationID < results[j].ApplicationID })
	return results, nil
}

// UpsertApplication validates, persists, and atomically activates one app.
func (m *ApplicationManager) UpsertApplication(app ProtectedApplication) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	app = cloneApplication(app)
	host, err := normalizeHostname(app.Hostname)
	if err != nil {
		return err
	}
	app.Hostname = host
	next := cloneRegistry(m.registry)
	found := false
	for i := range next.Applications {
		if next.Applications[i].ID == app.ID {
			next.Applications[i] = app
			found = true
			break
		}
	}
	if !found {
		next.Applications = append(next.Applications, app)
	}
	next.LegacyFallback = false
	return m.commit(next)
}

// DeleteApplication removes an app and drops rules that no longer have a
// scope. The legacy backend remains available only when no apps are registered.
func (m *ApplicationManager) DeleteApplication(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := cloneRegistry(m.registry)
	apps := next.Applications[:0]
	found := false
	for _, app := range next.Applications {
		if app.ID == id {
			found = true
			continue
		}
		apps = append(apps, app)
	}
	if !found {
		return fmt.Errorf("application %q not found", id)
	}
	next.Applications = apps
	if len(next.Applications) == 0 {
		next.LegacyFallback = false
	}
	rules := next.Rules[:0]
	for _, rule := range next.Rules {
		ids := rule.ApplicationIDs[:0]
		for _, appID := range rule.ApplicationIDs {
			if appID != id {
				ids = append(ids, appID)
			}
		}
		rule.ApplicationIDs = ids
		if rule.Global || len(rule.ApplicationIDs) > 0 {
			rules = append(rules, rule)
		}
	}
	next.Rules = rules
	return m.commit(next)
}

// SetRules replaces custom assignments only after compiling each expression
// and verifying every app reference.
func (m *ApplicationManager) SetRules(rules []ApplicationRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := cloneRegistry(m.registry)
	next.Rules = append([]ApplicationRule(nil), rules...)
	for i := range next.Rules {
		next.Rules[i].ApplicationIDs = append([]string(nil), rules[i].ApplicationIDs...)
	}
	return m.commit(next)
}

func (m *ApplicationManager) commit(next ApplicationRegistry) error {
	next.Version = 1
	snapshot, err := m.buildSnapshot(next)
	if err != nil {
		return err
	}
	if err := writeApplicationRegistry(m.path, next); err != nil {
		closeApplicationSnapshotExcept(snapshot, m.snapshot.Load())
		return err
	}
	m.registry = next
	previous := m.snapshot.Swap(snapshot)
	closeApplicationSnapshotExcept(previous, snapshot)
	return nil
}

func (m *ApplicationManager) buildSnapshot(registry ApplicationRegistry) (*applicationSnapshot, error) {
	if registry.Version != 1 {
		return nil, fmt.Errorf("unsupported registry version %d", registry.Version)
	}
	ids := make(map[string]struct{}, len(registry.Applications))
	hosts := make(map[string]struct{}, len(registry.Applications))
	for _, app := range registry.Applications {
		host, err := normalizeHostname(app.Hostname)
		if err != nil {
			return nil, fmt.Errorf("application %q hostname: %w", app.ID, err)
		}
		if !applicationIDPattern.MatchString(app.ID) {
			return nil, fmt.Errorf("invalid application ID %q", app.ID)
		}
		if _, exists := ids[app.ID]; exists {
			return nil, fmt.Errorf("application ID %q is assigned more than once", app.ID)
		}
		if _, exists := hosts[host]; exists {
			return nil, fmt.Errorf("hostname %q is assigned more than once", host)
		}
		ids[app.ID] = struct{}{}
		hosts[host] = struct{}{}
		if strings.TrimSpace(app.Name) == "" || len(app.Name) > 120 {
			return nil, fmt.Errorf("application %q must have a name no longer than 120 characters", app.ID)
		}
		if err := m.validateApplicationBackend(app); err != nil {
			return nil, fmt.Errorf("application %q backend: %w", app.ID, err)
		}
		if err := validateApplicationPolicy(app.Policy); err != nil {
			return nil, fmt.Errorf("application %q policy: %w", app.ID, err)
		}
	}
	ruleIDs := make(map[string]struct{}, len(registry.Rules))
	for _, rule := range registry.Rules {
		if !applicationIDPattern.MatchString(rule.ID) {
			return nil, fmt.Errorf("invalid custom rule ID %q", rule.ID)
		}
		if _, exists := ruleIDs[rule.ID]; exists {
			return nil, fmt.Errorf("duplicate custom rule ID %q", rule.ID)
		}
		ruleIDs[rule.ID] = struct{}{}
		if len(rule.Pattern) == 0 || len(rule.Pattern) > 512 {
			return nil, fmt.Errorf("custom rule %q pattern must contain 1 to 512 characters", rule.ID)
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return nil, fmt.Errorf("custom rule %q has invalid pattern: %w", rule.ID, err)
		}
		if !rule.Global && len(rule.ApplicationIDs) == 0 {
			return nil, fmt.Errorf("custom rule %q must be global or assigned to at least one app", rule.ID)
		}
		for _, id := range rule.ApplicationIDs {
			if _, ok := ids[id]; !ok {
				return nil, fmt.Errorf("custom rule %q references unknown application %q", rule.ID, id)
			}
		}
	}
	result := &applicationSnapshot{
		byHost:         make(map[string]*managedApplication, len(registry.Applications)),
		byID:           make(map[string]*managedApplication, len(registry.Applications)),
		legacyFallback: registry.LegacyFallback,
	}
	previous := m.snapshot.Load()
	completed := false
	defer func() {
		if !completed {
			closeApplicationSnapshotExcept(result, previous)
		}
	}()
	for _, app := range registry.Applications {
		host, err := normalizeHostname(app.Hostname)
		if err != nil {
			return nil, fmt.Errorf("application %q hostname: %w", app.ID, err)
		}
		if !applicationIDPattern.MatchString(app.ID) {
			return nil, fmt.Errorf("invalid application ID %q", app.ID)
		}
		if strings.TrimSpace(app.Name) == "" || len(app.Name) > 120 {
			return nil, fmt.Errorf("application %q must have a name no longer than 120 characters", app.ID)
		}
		if _, exists := result.byHost[host]; exists {
			return nil, fmt.Errorf("hostname %q is assigned more than once", host)
		}
		if _, exists := result.byID[app.ID]; exists {
			return nil, fmt.Errorf("application ID %q is assigned more than once", app.ID)
		}
		if err := m.validateApplicationBackend(app); err != nil {
			return nil, fmt.Errorf("application %q backend: %w", app.ID, err)
		}
		if err := validateApplicationPolicy(app.Policy); err != nil {
			return nil, fmt.Errorf("application %q policy: %w", app.ID, err)
		}
		if !app.Enabled {
			entry := &managedApplication{config: app}
			result.byHost[host] = entry
			result.byID[app.ID] = entry
			continue
		}
		appCfg, err := cloneConfig(m.cfg)
		if err != nil {
			return nil, err
		}
		appCfg.Server.TargetBackend = app.BackendURL
		applyApplicationPolicy(appCfg, app.Policy)
		patterns := m.patternsForApp(registry.Rules, app.ID)
		appCfg.Rules.CustomPatterns = append(appCfg.Rules.CustomPatterns, patterns...)
		if appCfg.Logging.Output == "file" && appCfg.Logging.FilePath != "" {
			extension := filepath.Ext(appCfg.Logging.FilePath)
			base := strings.TrimSuffix(appCfg.Logging.FilePath, extension)
			appCfg.Logging.FilePath = base + "." + app.ID + extension
		}
		hashInput, err := json.Marshal(struct {
			ApplicationID string
			BackendURL    string
			DemoKind      string
			InternalKind  string
			Policy        ApplicationPolicy
			Patterns      []string
			Config        *Config
		}{app.ID, app.BackendURL, app.DemoKind, app.InternalKind, app.Policy, patterns, appCfg})
		if err != nil {
			return nil, fmt.Errorf("fingerprint configuration for application %q: %w", app.ID, err)
		}
		hash := sha256.Sum256(hashInput)
		signature := hex.EncodeToString(hash[:])
		if previous != nil {
			if old, ok := previous.byID[app.ID]; ok && old.engine != nil && old.hash == signature {
				entry := &managedApplication{config: app, engine: old.engine, demo: old.demo, comparable: old.comparable, hash: signature}
				result.byHost[host] = entry
				result.byID[app.ID] = entry
				continue
			}
		}
		var demo http.Handler
		var internal http.Handler
		var engine *Engine
		if app.DemoKind != "" {
			demo = NewDemoBackend(app.DemoKind)
			engine, err = NewEngineWithBackend(appCfg, demo)
		} else if app.InternalKind != "" {
			internal = m.internal[app.InternalKind]
			engine, err = NewEngineWithBackend(appCfg, internal)
		} else {
			engine, err = NewEngine(appCfg)
		}
		if err != nil {
			return nil, fmt.Errorf("create engine for application %q: %w", app.ID, err)
		}
		engine.applicationID = app.ID
		entry := &managedApplication{
			config: app, engine: engine, demo: demo,
			comparable: app.DemoKind != "" || app.InternalKind == "cyber-attack-lab",
			hash:       signature,
		}
		result.byHost[host] = entry
		result.byID[app.ID] = entry
	}
	completed = true
	return result, nil
}

func (m *ApplicationManager) patternsForApp(rules []ApplicationRule, appID string) []string {
	var patterns []string
	for _, rule := range rules {
		if rule.Global || containsString(rule.ApplicationIDs, appID) {
			patterns = append(patterns, rule.Pattern)
		}
	}
	return patterns
}

func (m *ApplicationManager) validateUpstream(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return fmt.Errorf("must be an http(s) URL with a host and without credentials, query, or fragment")
	}
	if u.Port() != "" {
		port, err := net.LookupPort("tcp", u.Port())
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid upstream port")
		}
	}
	if _, err := normalizeHostname(u.Hostname()); err != nil {
		return fmt.Errorf("invalid upstream hostname: %w", err)
	}
	authority := normalizeUpstreamAuthority(raw)
	if _, ok := m.allowed[authority]; !ok {
		return fmt.Errorf("upstream authority %q is not in applications.allowed_upstream_hosts", authority)
	}
	return nil
}

func (m *ApplicationManager) validateApplicationBackend(app ProtectedApplication) error {
	if app.DemoKind != "" && app.InternalKind != "" {
		return fmt.Errorf("demo_kind and internal_kind cannot both be set")
	}
	if app.DemoKind != "" {
		if (app.DemoKind != "online-store" && app.DemoKind != "booking-system") || app.BackendURL != "internal://demo/"+app.DemoKind {
			return fmt.Errorf("invalid internal demo backend")
		}
		return nil
	}
	if app.InternalKind != "" {
		expected := map[string]string{
			"original-site":    "internal://showcase/original",
			"cyber-attack-lab": "internal://showcase/cyber-attack-lab",
		}
		if app.BackendURL != expected[app.InternalKind] {
			return fmt.Errorf("invalid internal showcase backend")
		}
		if m.internal[app.InternalKind] == nil {
			return fmt.Errorf("internal application %q requires the --showcase runtime", app.InternalKind)
		}
		return nil
	}
	return m.validateUpstream(app.BackendURL)
}

func validateApplicationPolicy(policy ApplicationPolicy) error {
	if policy.Profile != "" && policy.Profile != "relaxed" && policy.Profile != "balanced" && policy.Profile != "strict" {
		return fmt.Errorf("profile must be relaxed, balanced, or strict")
	}
	if policy.ParanoiaLevel < 0 || policy.ParanoiaLevel > 4 {
		return fmt.Errorf("paranoia_level must be between 1 and 4 when set")
	}
	if policy.AnomalyThreshold < 0 {
		return fmt.Errorf("anomaly_threshold cannot be negative")
	}
	if policy.RequestsPerSecond < 0 || policy.BurstSize < 0 {
		return fmt.Errorf("rate-limit values cannot be negative")
	}
	if policy.RequestsPerSecond > 0 && policy.BurstSize == 0 {
		return fmt.Errorf("burst_size must be set when requests_per_second is set")
	}
	return nil
}

func applyApplicationPolicy(cfg *Config, policy ApplicationPolicy) {
	switch policy.Profile {
	case "relaxed":
		cfg.Rules.ParanoiaLevel = 1
		cfg.Rules.AnomalyThreshold = 12
	case "strict":
		cfg.Rules.ParanoiaLevel = 4
		cfg.Rules.AnomalyThreshold = 4
	case "balanced":
		cfg.Rules.ParanoiaLevel = 2
		cfg.Rules.AnomalyThreshold = 8
	}
	if policy.ParanoiaLevel > 0 {
		cfg.Rules.ParanoiaLevel = policy.ParanoiaLevel
	}
	if policy.AnomalyThreshold > 0 {
		cfg.Rules.AnomalyThreshold = policy.AnomalyThreshold
	}
	if policy.RateLimitEnabled != nil {
		cfg.RateLimit.Enabled = *policy.RateLimitEnabled
	}
	if policy.RequestsPerSecond > 0 {
		cfg.RateLimit.RequestsPerSecond = policy.RequestsPerSecond
		cfg.RateLimit.BurstSize = policy.BurstSize
	}
}

func cloneConfig(cfg *Config) (*Config, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("clone configuration: %w", err)
	}
	clone := &Config{}
	if err := yaml.Unmarshal(data, clone); err != nil {
		return nil, fmt.Errorf("clone configuration: %w", err)
	}
	return clone, nil
}

func cloneRegistry(registry ApplicationRegistry) ApplicationRegistry {
	clone := ApplicationRegistry{
		Version:        registry.Version,
		LegacyFallback: registry.LegacyFallback,
		Applications:   make([]ProtectedApplication, len(registry.Applications)),
		Rules:          make([]ApplicationRule, len(registry.Rules)),
	}
	copy(clone.Applications, registry.Applications)
	for i := range clone.Applications {
		if enabled := clone.Applications[i].Policy.RateLimitEnabled; enabled != nil {
			value := *enabled
			clone.Applications[i].Policy.RateLimitEnabled = &value
		}
	}
	copy(clone.Rules, registry.Rules)
	for i := range clone.Rules {
		clone.Rules[i].ApplicationIDs = append([]string(nil), registry.Rules[i].ApplicationIDs...)
	}
	return clone
}

func writeApplicationRegistry(path string, registry ApplicationRegistry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create application registry directory: %w", err)
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode application registry: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(dir, ".apps-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary application registry: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary application registry: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write application registry: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync application registry: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary application registry: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("activate application registry: %w", err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

func normalizeHostname(host string) (string, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " /\\\t\r\n") {
		return "", fmt.Errorf("invalid hostname")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid hostname")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return "", fmt.Errorf("invalid hostname")
			}
		}
	}
	return host, nil
}

func requestHostname(hostport string) (string, error) {
	host := hostport
	if parsed, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsed
	} else if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		host = strings.Trim(hostport, "[]")
	} else if strings.Count(hostport, ":") > 1 {
		return "", fmt.Errorf("invalid host")
	}
	return normalizeHostname(host)
}

func normalizeUpstreamAuthority(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host, err := normalizeHostname(u.Hostname())
	if err != nil {
		return ""
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else if u.Scheme == "http" {
			port = "80"
		} else {
			return ""
		}
	}
	return net.JoinHostPort(host, port)
}

func normalizeAllowedUpstreamHost(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			host = strings.Trim(raw, "[]")
		} else if strings.Count(raw, ":") > 1 {
			return nil, fmt.Errorf("IPv6 hosts with ports must use brackets")
		} else {
			host = raw
		}
	}
	normalized, err := normalizeHostname(host)
	if err != nil {
		return nil, err
	}
	if port != "" {
		number, err := net.LookupPort("tcp", port)
		if err != nil || number < 1 || number > 65535 {
			return nil, fmt.Errorf("invalid port")
		}
		return []string{net.JoinHostPort(normalized, port)}, nil
	}
	return []string{net.JoinHostPort(normalized, "80"), net.JoinHostPort(normalized, "443")}, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// NewApplicationID returns an unpredictable, URL-safe ID for a new app.
func NewApplicationID() (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate application ID: %w", err)
	}
	return "app-" + hex.EncodeToString(random[:]), nil
}

// ApplicationRules returns the current custom rules with stable ordering.
func (m *ApplicationManager) ApplicationRules() []ApplicationRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	rules := append([]ApplicationRule(nil), m.registry.Rules...)
	for i := range rules {
		rules[i].ApplicationIDs = append([]string(nil), rules[i].ApplicationIDs...)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules
}

func cloneApplication(app ProtectedApplication) ProtectedApplication {
	if enabled := app.Policy.RateLimitEnabled; enabled != nil {
		value := *enabled
		app.Policy.RateLimitEnabled = &value
	}
	return app
}

func closeApplicationSnapshot(snapshot *applicationSnapshot) {
	closeApplicationSnapshotExcept(snapshot, nil)
}

func closeApplicationSnapshotExcept(snapshot, preserve *applicationSnapshot) {
	if snapshot == nil {
		return
	}
	preservedEngines := make(map[*Engine]struct{})
	if preserve != nil {
		for _, app := range preserve.byID {
			if app.engine != nil {
				preservedEngines[app.engine] = struct{}{}
			}
		}
	}
	for _, app := range snapshot.byID {
		if app.engine != nil {
			if _, keep := preservedEngines[app.engine]; !keep {
				app.engine.Close()
			}
		}
	}
}
