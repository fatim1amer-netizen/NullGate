package waf

import (
	"net/url"
	"testing"
)

func TestGraduationPlatformProfileTargetsLiveSite(t *testing.T) {
	cfg, err := LoadConfig("../../configs/graduation-platform.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	const siteHost = "7a08f55f.graduation-platform-2026.pages.dev"
	if cfg.Server.TargetBackend != "https://"+siteHost {
		t.Errorf("backend = %q, want live site URL", cfg.Server.TargetBackend)
	}
	if cfg.Server.UpstreamHost != siteHost {
		t.Errorf("upstream Host = %q, want %q", cfg.Server.UpstreamHost, siteHost)
	}
	if cfg.Server.ServeLanding {
		t.Error("live site profile must proxy the real application, not NullGate's landing page")
	}
	if cfg.Response.AddCSP {
		t.Error("generic CSP must not be injected into the Flutter site")
	}
	if cfg.CSRF.Enabled {
		t.Error("WAF CSRF token requirement is incompatible with an unknown upstream application")
	}
	u, err := url.Parse(cfg.Server.TargetBackend)
	if err != nil || u.Hostname() != siteHost {
		t.Errorf("backend URL must resolve to the published site host, got %q", cfg.Server.TargetBackend)
	}
}

func TestDefaultConfigKeepsAdminLocalAndDisabled(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Admin.Enabled {
		t.Error("admin surface must be disabled by default")
	}
	if !cfg.Admin.RequireLocalhost || cfg.Admin.ListenAddr != "127.0.0.1:9091" {
		t.Errorf("admin defaults must be loopback-only, got address=%q require_localhost=%t",
			cfg.Admin.ListenAddr, cfg.Admin.RequireLocalhost)
	}
}

func TestValidateConfigRejectsCredentialsInBackendURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.TargetBackend = "https://user:secret@example.test/"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("backend URL with embedded credentials should be rejected")
	}
}

func TestValidateConfigRejectsPlaintextAdminPassword(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Admin.PasswordHash = "plaintext-password"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("plaintext admin password configuration should be rejected")
	}
}
