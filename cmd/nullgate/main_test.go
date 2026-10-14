package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("NULLGATE_CONFIG", "")
		if got := defaultConfigPath(); got != "configs/config.yaml" {
			t.Fatalf("defaultConfigPath() = %q, want default config", got)
		}
	})

	t.Run("environment override", func(t *testing.T) {
		const path = "/configs/graduation-platform.yaml"
		t.Setenv("NULLGATE_CONFIG", "  "+path+"  ")
		if got := defaultConfigPath(); got != path {
			t.Fatalf("defaultConfigPath() = %q, want trimmed environment path", got)
		}
	})
}

func TestWithHealthz(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := withHealthz(next)

	t.Run("GET health", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK || rec.Body.String() != `{"status":"ok"}` {
			t.Fatalf("health response = %d %q, want 200 and status JSON", rec.Code, rec.Body.String())
		}
	})

	t.Run("reject non-GET", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST health status = %d, want 405", rec.Code)
		}
	})

	t.Run("forward other paths", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app", nil))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("application path status = %d, want 202", rec.Code)
		}
	})
}
