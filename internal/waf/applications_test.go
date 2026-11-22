package waf

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplicationManagerRoutesAndRejectsUnknownHosts(t *testing.T) {
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend-a:"+r.URL.Path)
	}))
	defer backendA.Close()
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend-b:"+r.URL.Path)
	}))
	defer backendB.Close()

	manager := newTestApplicationManager(t, backendA.URL, backendA.URL, backendB.URL)
	for _, test := range []struct {
		host string
		path string
		want string
	}{
		{"store.example.test", "/catalog", "backend-a:/catalog"},
		{"booking.example.test", "/reservations", "backend-b:/reservations"},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://"+test.host+test.path, nil)
		req.Host = test.host
		browserHeaders(req)
		rec := httptest.NewRecorder()
		manager.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != test.want {
			t.Errorf("%s route: got %d %q, want 200 %q", test.host, rec.Code, rec.Body.String(), test.want)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "http://unknown.example.test/", nil)
	browserHeaders(req)
	rec := httptest.NewRecorder()
	manager.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("unknown host: got %d, want 421", rec.Code)
	}
}

func TestApplicationHealthCheckIsLoopbackOnly(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()
	manager := newTestApplicationManager(t, backend.URL, backend.URL)
	for _, test := range []struct {
		remote string
		want   int
	}{
		{"127.0.0.1:1234", http.StatusOK},
		{"192.0.2.10:1234", http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil)
		req.RemoteAddr = test.remote
		rec := httptest.NewRecorder()
		manager.ServeHTTP(rec, req)
		if rec.Code != test.want {
			t.Errorf("health check from %s: got %d, want %d", test.remote, rec.Code, test.want)
		}
	}
}

func TestRemovingLastApplicationDoesNotReenableLegacyFallback(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "legacy")
	}))
	defer backend.Close()
	manager := newTestApplicationManager(t, backend.URL, backend.URL)
	for _, id := range []string{"store", "booking"} {
		if err := manager.DeleteApplication(id); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "http://unknown.example.test/", nil)
	browserHeaders(req)
	rec := httptest.NewRecorder()
	manager.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("deleting all apps restored legacy fallback: got %d, want 421", rec.Code)
	}
}

func TestApplicationPoliciesRulesAndMetricsAreIsolated(t *testing.T) {
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "A")
	}))
	defer backendA.Close()
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "B")
	}))
	defer backendB.Close()
	manager := newTestApplicationManager(t, backendA.URL, backendA.URL, backendB.URL)

	if err := manager.SetRules([]ApplicationRule{
		{ID: "only-store", Pattern: `APP_A_MARKER`, ApplicationIDs: []string{"store"}},
		{ID: "store-email-test", Pattern: `alice\.smith@example\.com`, ApplicationIDs: []string{"store"}},
		{ID: "global-marker", Pattern: `GLOBAL_RULE_MARKER`, Global: true},
	}); err != nil {
		t.Fatalf("SetRules: %v", err)
	}

	makeRequest := func(host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/APP_A_MARKER", nil)
		browserHeaders(req)
		rec := httptest.NewRecorder()
		manager.ServeHTTP(rec, req)
		return rec
	}
	if got := makeRequest("store.example.test"); got.Code != http.StatusForbidden {
		t.Fatalf("assigned rule should block store: got %d", got.Code)
	}
	if got := makeRequest("booking.example.test"); got.Code != http.StatusOK || got.Body.String() != "B" {
		t.Fatalf("store rule affected booking: got %d %q", got.Code, got.Body.String())
	}
	globalReq := httptest.NewRequest(http.MethodGet, "http://booking.example.test/GLOBAL_RULE_MARKER", nil)
	browserHeaders(globalReq)
	globalRec := httptest.NewRecorder()
	manager.ServeHTTP(globalRec, globalReq)
	if globalRec.Code != http.StatusForbidden {
		t.Fatalf("global custom rule should apply to booking: got %d", globalRec.Code)
	}

	storeMetrics, ok := manager.ApplicationMetrics("store")
	if !ok || storeMetrics.Metrics.Blocked != 1 || storeMetrics.Metrics.Total != 1 {
		t.Fatalf("unexpected store metrics: %+v", storeMetrics)
	}
	bookingMetrics, ok := manager.ApplicationMetrics("booking")
	if !ok || bookingMetrics.Metrics.Allowed != 1 || bookingMetrics.Metrics.Blocked != 1 || bookingMetrics.Metrics.Total != 2 {
		t.Fatalf("unexpected booking metrics: %+v", bookingMetrics)
	}
	evaluations, err := manager.EvaluateApplications()
	if err != nil {
		t.Fatalf("EvaluateApplications: %v", err)
	}
	if len(evaluations) != 2 || evaluations[0].ApplicationID != "booking" || evaluations[1].ApplicationID != "store" {
		t.Fatalf("unexpected per-app evaluation results: %+v", evaluations)
	}
	if evaluations[1].Report.FalsePositives <= evaluations[0].Report.FalsePositives {
		t.Fatalf("scoped custom rule did not affect only the store's labeled evaluation: booking=%d store=%d",
			evaluations[0].Report.FalsePositives, evaluations[1].Report.FalsePositives)
	}
	if evaluations[1].F1Score != evaluations[1].Report.F1Score() {
		t.Fatalf("F1 metric differs from report: %+v", evaluations[1])
	}
}

func TestApplicationBlocklistsAndMetricsSurviveUnrelatedRegistryUpdate(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	}))
	defer backend.Close()
	manager := newTestApplicationManager(t, backend.URL, backend.URL, backend.URL)
	store := manager.snapshot.Load().byID["store"]
	store.engine.blocklist.Add("192.0.2.42", time.Hour)

	makeRequest := func(host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		browserHeaders(req)
		req.RemoteAddr = "192.0.2.42:1234"
		rec := httptest.NewRecorder()
		manager.ServeHTTP(rec, req)
		return rec
	}
	if got := makeRequest("store.example.test"); got.Code != http.StatusForbidden {
		t.Fatalf("store blocklist should reject client: got %d", got.Code)
	}
	if got := makeRequest("booking.example.test"); got.Code != http.StatusOK {
		t.Fatalf("store blocklist affected booking: got %d", got.Code)
	}
	before, ok := manager.ApplicationMetrics("store")
	if !ok || before.Metrics.Total != 1 {
		t.Fatalf("unexpected store metrics before registry update: %+v", before)
	}
	if err := manager.UpsertApplication(ProtectedApplication{
		ID: "third-app", Name: "Third App", Hostname: "third.example.test",
		BackendURL: backend.URL, Enabled: true,
	}); err != nil {
		t.Fatalf("register unrelated app: %v", err)
	}
	after, ok := manager.ApplicationMetrics("store")
	if !ok || after.Metrics.Total != before.Metrics.Total || after.Metrics.Blocked != before.Metrics.Blocked {
		t.Fatalf("unrelated registry update reset store metrics: before=%+v after=%+v", before.Metrics, after.Metrics)
	}
	if got := makeRequest("store.example.test"); got.Code != http.StatusForbidden {
		t.Fatalf("unrelated registry update reset store blocklist: got %d", got.Code)
	}
}

func TestApplicationRateLimitsAreIndependent(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	}))
	defer backend.Close()

	cfg := appManagerConfig(t, backend.URL)
	cfg.RateLimit.Enabled = false
	manager, err := NewApplicationManager(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "legacy", http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	enabled := true
	for _, app := range []ProtectedApplication{
		{ID: "store", Name: "Store", Hostname: "store.example.test", BackendURL: backend.URL, Enabled: true, Policy: ApplicationPolicy{RateLimitEnabled: &enabled, RequestsPerSecond: 1, BurstSize: 1}},
		{ID: "booking", Name: "Booking", Hostname: "booking.example.test", BackendURL: backend.URL, Enabled: true, Policy: ApplicationPolicy{RateLimitEnabled: &enabled, RequestsPerSecond: 1, BurstSize: 1}},
	} {
		if err := manager.UpsertApplication(app); err != nil {
			t.Fatalf("register %s: %v", app.ID, err)
		}
	}

	makeRequest := func(host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		browserHeaders(req)
		rec := httptest.NewRecorder()
		manager.ServeHTTP(rec, req)
		return rec
	}
	if got := makeRequest("store.example.test"); got.Code != http.StatusOK {
		t.Fatalf("first store request: got %d", got.Code)
	}
	if got := makeRequest("store.example.test"); got.Code != http.StatusTooManyRequests {
		t.Fatalf("second store request: got %d, want 429", got.Code)
	}
	if got := makeRequest("booking.example.test"); got.Code != http.StatusOK {
		t.Fatalf("booking should have an independent bucket: got %d", got.Code)
	}
}

func TestApplicationPolicyOverridesAreAppliedIndependently(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	}))
	defer backend.Close()
	manager := newTestApplicationManager(t, backend.URL, backend.URL)
	manager.cfg.RateLimit.Enabled = true
	disabled := false
	store := manager.Applications()[0]
	store.Policy = ApplicationPolicy{
		Profile: "strict", ParanoiaLevel: 3, AnomalyThreshold: 2,
		RateLimitEnabled: &disabled, RequestsPerSecond: 5, BurstSize: 10,
	}
	if err := manager.UpsertApplication(store); err != nil {
		t.Fatal(err)
	}
	storeConfig := manager.snapshot.Load().byID["store"].engine.Config()
	bookingConfig := manager.snapshot.Load().byID["booking"].engine.Config()
	if storeConfig.Rules.ParanoiaLevel != 3 || storeConfig.Rules.AnomalyThreshold != 2 || storeConfig.RateLimit.Enabled {
		t.Fatalf("store policy override not applied: paranoia=%d threshold=%d rate_limit=%v",
			storeConfig.Rules.ParanoiaLevel, storeConfig.Rules.AnomalyThreshold, storeConfig.RateLimit.Enabled)
	}
	if bookingConfig.Rules.ParanoiaLevel != 2 || bookingConfig.Rules.AnomalyThreshold != 8 || !bookingConfig.RateLimit.Enabled {
		t.Fatalf("store policy leaked into booking: paranoia=%d threshold=%d rate_limit=%v",
			bookingConfig.Rules.ParanoiaLevel, bookingConfig.Rules.AnomalyThreshold, bookingConfig.RateLimit.Enabled)
	}
}

func TestApplicationRegistryPersistsAndRejectsUnlistedUpstream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "persisted")
	}))
	defer backend.Close()

	manager := newTestApplicationManager(t, backend.URL, backend.URL)
	app := ProtectedApplication{ID: "persisted", Name: "Persisted", Hostname: "persisted.example.test", BackendURL: backend.URL, Enabled: true}
	if err := manager.UpsertApplication(app); err != nil {
		t.Fatalf("UpsertApplication: %v", err)
	}
	reloaded, err := NewApplicationManager(manager.cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "legacy", http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatalf("reload registry: %v", err)
	}
	t.Cleanup(reloaded.Close)
	req := httptest.NewRequest(http.MethodGet, "http://persisted.example.test/", nil)
	browserHeaders(req)
	rec := httptest.NewRecorder()
	reloaded.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "persisted" {
		t.Fatalf("reloaded app route: got %d %q", rec.Code, rec.Body.String())
	}

	app.ID = "bad-upstream"
	app.Hostname = "bad.example.test"
	app.BackendURL = "http://169.254.169.254/latest/meta-data"
	if err := manager.UpsertApplication(app); err == nil {
		t.Fatal("unlisted upstream was accepted")
	}
}

func TestApplicationManagerRejectsInvalidHostsAndSNIConfusion(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	}))
	defer backend.Close()
	manager := newTestApplicationManager(t, backend.URL, backend.URL)

	if err := manager.UpsertApplication(ProtectedApplication{
		ID: "invalid", Name: "Invalid", Hostname: "bad host.example", BackendURL: backend.URL, Enabled: true,
	}); err == nil {
		t.Fatal("invalid hostname accepted")
	}

	req := httptest.NewRequest(http.MethodGet, "https://store.example.test/", nil)
	req.Host = "store.example.test"
	req.TLS = &tls.ConnectionState{ServerName: "booking.example.test"}
	browserHeaders(req)
	rec := httptest.NewRecorder()
	manager.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("SNI/Host mismatch: got %d, want 421", rec.Code)
	}
}

func TestDemoComparisonUsesProtectedPipelineAndIsolatedBackend(t *testing.T) {
	legacy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "legacy")
	}))
	defer legacy.Close()
	cfg := appManagerConfig(t, legacy.URL)
	manager, err := NewApplicationManager(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "legacy", http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	for _, app := range []ProtectedApplication{
		{ID: "store-demo", Name: "Online Store", Hostname: "store.demo.test", BackendURL: "internal://demo/online-store", DemoKind: "online-store", Enabled: true},
		{ID: "booking-demo", Name: "Booking System", Hostname: "booking.demo.test", BackendURL: "internal://demo/booking-system", DemoKind: "booking-system", Enabled: true},
	} {
		if err := manager.UpsertApplication(app); err != nil {
			t.Fatalf("register demo: %v", err)
		}
	}

	result, err := manager.CompareDemo("store-demo", http.MethodGet, "/search?q=%27%20OR%20%271%27%3D%271", "", "")
	if err != nil {
		t.Fatalf("CompareDemo: %v", err)
	}
	if result.Protected.StatusCode != http.StatusForbidden || !result.Decision.Blocked || result.Decision.Layer != "rules" {
		t.Fatalf("protected request was not blocked by the live WAF: %+v", result)
	}
	if len(result.Decision.Rules) == 0 || result.Decision.Score == 0 {
		t.Fatalf("comparison omitted real matched rules and score: %+v", result.Decision)
	}
	if !strings.Contains(result.Unprotected.Body, "All synthetic records") {
		t.Fatalf("unprotected synthetic backend did not execute lab behavior: %q", result.Unprotected.Body)
	}
	if result.Unprotected.StatusCode != http.StatusOK {
		t.Fatalf("unprotected synthetic response: got %d", result.Unprotected.StatusCode)
	}

	wafRoute := httptest.NewRequest(http.MethodGet, "http://store.demo.test/", nil)
	wafRoute.Host = "store.demo.test"
	browserHeaders(wafRoute)
	rec := httptest.NewRecorder()
	manager.ServeHTTP(rec, wafRoute)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Online Store Lab") {
		t.Fatalf("demo must be served through the app WAF: got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSyntheticDemoFormsAndAuthorizationFlows(t *testing.T) {
	store := NewDemoBackend("online-store")
	search := httptest.NewRecorder()
	store.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/search", nil))
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `name="q"`) {
		t.Fatalf("store search form unavailable: %d %q", search.Code, search.Body.String())
	}
	cartPost := httptest.NewRequest(http.MethodPost, "/cart", strings.NewReader("product=101"))
	cartPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cart := httptest.NewRecorder()
	store.ServeHTTP(cart, cartPost)
	if cart.Code != http.StatusOK || len(cart.Result().Cookies()) == 0 {
		t.Fatalf("store cart submission failed: %d %q", cart.Code, cart.Body.String())
	}
	cartGet := httptest.NewRequest(http.MethodGet, "/cart", nil)
	cartGet.AddCookie(cart.Result().Cookies()[0])
	cartView := httptest.NewRecorder()
	store.ServeHTTP(cartView, cartGet)
	if !strings.Contains(cartView.Body.String(), "Notebook") {
		t.Fatalf("store cart state was not observed: %q", cartView.Body.String())
	}

	booking := NewDemoBackend("booking-system")
	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("role=admin"))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResult := httptest.NewRecorder()
	booking.ServeHTTP(loginResult, login)
	if loginResult.Code != http.StatusOK || len(loginResult.Result().Cookies()) == 0 {
		t.Fatalf("booking lab login failed: %d %q", loginResult.Code, loginResult.Body.String())
	}
	management := httptest.NewRequest(http.MethodGet, "/management", nil)
	management.AddCookie(loginResult.Result().Cookies()[0])
	managementResult := httptest.NewRecorder()
	booking.ServeHTTP(managementResult, management)
	if managementResult.Code != http.StatusOK {
		t.Fatalf("synthetic admin role was not authorized: %d %q", managementResult.Code, managementResult.Body.String())
	}
	unauthorized := httptest.NewRecorder()
	booking.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/management", nil))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("management route without admin role: got %d, want 403", unauthorized.Code)
	}
}

func TestApplicationRegistryWriteIsNotActivatedWhenPersistenceFails(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	}))
	defer backend.Close()
	cfg := appManagerConfig(t, backend.URL)
	cfg.Applications.RegistryPath = filepath.Join(t.TempDir(), "apps.json")
	manager, err := NewApplicationManager(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "legacy")
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Close)
	// Force persistence to fail after construction/validation succeeds.
	badParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(badParent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager.path = filepath.Join(badParent, "child.json")
	app := ProtectedApplication{ID: "store", Name: "Store", Hostname: "store.example.test", BackendURL: backend.URL, Enabled: true}
	if err := manager.UpsertApplication(app); err == nil {
		t.Fatal("expected registry write failure")
	}
	if len(manager.Applications()) != 0 {
		t.Fatal("failed write changed active registry")
	}
	req := httptest.NewRequest(http.MethodGet, "http://store.example.test/", nil)
	browserHeaders(req)
	rec := httptest.NewRecorder()
	manager.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "legacy" {
		t.Fatalf("failed write changed active routes: got %d %q", rec.Code, rec.Body.String())
	}
}

func newTestApplicationManager(t *testing.T, legacyURL string, appURLs ...string) *ApplicationManager {
	t.Helper()
	if len(appURLs) == 1 {
		appURLs = append(appURLs, appURLs[0])
	}
	cfg := appManagerConfig(t, legacyURL)
	for _, raw := range appURLs {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Applications.AllowedUpstreamHosts = append(cfg.Applications.AllowedUpstreamHosts, u.Host)
	}
	manager, err := NewApplicationManager(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "legacy", http.StatusInternalServerError)
	}))
	if err != nil {
		t.Fatalf("NewApplicationManager: %v", err)
	}
	t.Cleanup(manager.Close)
	for _, app := range []ProtectedApplication{
		{ID: "store", Name: "Online Store", Hostname: "store.example.test", BackendURL: appURLs[0], Enabled: true},
		{ID: "booking", Name: "Booking System", Hostname: "booking.example.test", BackendURL: appURLs[1], Enabled: true},
	} {
		if err := manager.UpsertApplication(app); err != nil {
			t.Fatalf("register %s: %v", app.ID, err)
		}
	}
	return manager
}

func appManagerConfig(t *testing.T, target string) *Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Server.TargetBackend = target
	cfg.Applications.RegistryPath = filepath.Join(t.TempDir(), "apps.json")
	cfg.Logging.Level = "error"
	cfg.Logging.EnableAccessLog = false
	cfg.CSRF.Enabled = false
	cfg.RateLimit.Enabled = false
	cfg.Bot.Enabled = false
	cfg.Honeypot.Enabled = false
	cfg.Behavior.Enabled = false
	return cfg
}
