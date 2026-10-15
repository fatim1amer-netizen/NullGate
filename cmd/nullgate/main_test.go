package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"nullgate/internal/waf"
)

func TestDefaultConfigPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("NULLGATE_CONFIG", "")
		if got := defaultConfigPath(); got != filepath.Join("configs", "config.yaml") {
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
	handler := withHealthz(next, nil)

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

func TestReadinessChecksConfiguredBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cfg := waf.DefaultConfig()
	cfg.Server.TargetBackend = backend.URL
	engine, err := waf.NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	handler := withHealthz(http.NotFoundHandler(), engine)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthy readiness status = %d %q, want 200", rec.Code, rec.Body.String())
	}

	backend.Close()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable backend readiness status = %d %q, want 503", rec.Code, rec.Body.String())
	}
}

func TestGracefulShutdownStopsPublicAndAdminServers(t *testing.T) {
	newServer := func() (*http.Server, chan error) {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})}
		serveErr := make(chan error, 1)
		go func() { serveErr <- server.Serve(listener) }()
		return server, serveErr
	}

	public, publicErr := newServer()
	admin, adminErr := newServer()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := shutdownServers(ctx, public, admin); err != nil {
		t.Fatalf("shutdownServers: %v", err)
	}
	for name, serveErr := range map[string]chan error{"public": publicErr, "admin": adminErr} {
		select {
		case err := <-serveErr:
			if !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("%s Serve returned %v, want http.ErrServerClosed", name, err)
			}
		case <-time.After(time.Second):
			t.Errorf("%s server did not stop after graceful shutdown", name)
		}
	}
}
