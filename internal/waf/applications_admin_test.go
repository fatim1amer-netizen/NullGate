package waf

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApplicationAdminAPIRequiresSessionAndWriteAccess(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	cfg := appManagerConfig(t, backend.URL)
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)
	apps, err := NewApplicationManager(cfg, engine)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(apps.Close)
	adminCfg := DefaultConfig().Admin
	adminCfg.PasswordHash = HashPassword("admin-test-password")
	adminCfg.ReadOnly = false
	adminCfg.RequireLocalhost = true
	admin := NewAdminServer(adminCfg, cfg, engine.Metrics(), engine.Logger(), engine.Blocklist(), engine.Rules(), "test", 0, 0)
	admin.SetApplicationManager(apps)
	handler := admin.Handler()

	payload := `{"name":"Store","hostname":"store.admin.test","backend_url":"` + backend.URL + `","enabled":true}`
	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/admin/apps/upsert", strings.NewReader(payload))
	unauthenticated.RemoteAddr = "127.0.0.1:12345"
	unauthenticated.Header.Set("Content-Type", "application/json")
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthenticated)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("app mutation without session: got %d, want 401", unauthorizedResponse.Code)
	}

	token, err := admin.sessions.Create()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/apps/upsert", bytes.NewBufferString(payload))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated app mutation: got %d %q", response.Code, response.Body.String())
	}
	var result struct {
		ApplicationID string `json:"application_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.ApplicationID == "" {
		t.Fatalf("missing generated app ID: result=%+v err=%v", result, err)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	get.RemoteAddr = "127.0.0.1:12345"
	get.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, get)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "store.admin.test") {
		t.Fatalf("app list did not show saved app: %d %s", list.Code, list.Body.String())
	}

	admin.cfg.ReadOnly = true
	request = httptest.NewRequest(http.MethodPost, "/api/admin/apps/delete", strings.NewReader(`{"id":"`+result.ApplicationID+`"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("read-only app mutation: got %d, want 403", response.Code)
	}
}
