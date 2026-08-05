package waf

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// AutoTLSManager wraps autocert.Manager for Let's Encrypt certificate
// provisioning. It handles the ACME challenge, caches issued certs, and
// automatically renews them before expiry. The cache directory must be
// writable and persistent across restarts so renewals work correctly.
type AutoTLSManager struct {
	manager *autocert.Manager
}

// NewAutoTLSManager creates a manager configured for the given domains. The
// cache directory stores issued certificates; email is the ACME account contact.
func NewAutoTLSManager(domains []string, cacheDir, email string) (*AutoTLSManager, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("auto-TLS requires at least one domain")
	}
	if cacheDir == "" {
		cacheDir = ".autocert-cache"
	}
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cacheDir),
		Email:      email,
	}
	return &AutoTLSManager{manager: m}, nil
}

// TLSConfig returns a *tls.Config that fetches certificates from the manager.
// Pass this to http.Server.TLSConfig to enable auto-TLS.
func (m *AutoTLSManager) TLSConfig() *tls.Config {
	return m.manager.TLSConfig()
}

// HTTPHandler returns the ACME challenge handler. Mount it on port 80 so
// Let's Encrypt can verify domain ownership via the HTTP-01 challenge.
func (m *AutoTLSManager) HTTPHandler(fallback http.Handler) http.Handler {
	return m.manager.HTTPHandler(fallback)
}

// StartAutoTLS provisions a certificate for the configured domains and starts
// an HTTPS server on port 443. It also starts an HTTP server on port 80 that
// handles ACME challenges and redirects everything else to HTTPS.
//
// This is the all-in-one setup for production cloud deployments: the operator
// points DNS at the server, and the WAF provisions its own certificate with
// zero manual steps.
func StartAutoTLS(cfg *Config, handler http.Handler) error {
	if !cfg.Server.AutoTLS {
		return fmt.Errorf("auto-TLS not enabled in config")
	}
	if len(cfg.Server.AutoTLSDomains) == 0 {
		return fmt.Errorf("auto_tls_domains is required when auto_tls is enabled")
	}

	cacheDir := cfg.Server.AutoTLSCacheDir
	if cacheDir == "" {
		cacheDir = ".autocert-cache"
	}
	cacheDir = filepath.Clean(cacheDir)

	mgr, err := NewAutoTLSManager(cfg.Server.AutoTLSDomains, cacheDir, cfg.Server.AutoTLSEmail)
	if err != nil {
		return err
	}

	// HTTPS server on :443 with auto-provisioned cert.
	httpsServer := &http.Server{
		Addr:              ":https",
		Handler:           handler,
		TLSConfig:         mgr.TLSConfig(),
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	// HTTP server on :80 for ACME challenges + redirect to HTTPS.
	httpServer := &http.Server{
		Addr: ":http",
		Handler: mgr.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Start HTTP redirect + ACME handler in the background.
	go func() {
		log.Printf("auto-TLS HTTP server (ACME + redirect) listening on :80")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("auto-TLS HTTP server failed: %v", err)
		}
	}()

	log.Printf("auto-TLS HTTPS server starting on :443 for domains: %v", cfg.Server.AutoTLSDomains)
	log.Printf("auto-TLS cache directory: %s", cacheDir)
	if cfg.Server.AutoTLSEmail != "" {
		log.Printf("auto-TLS ACME account email: %s", cfg.Server.AutoTLSEmail)
	}

	// Start HTTPS server (blocking).
	return httpsServer.ListenAndServeTLS("", "")
}

// ProvisionCert pre-fetches a certificate for the first configured domain.
// Call this before starting the server to catch ACME errors early rather than
// on first request. It returns nil when the certificate was successfully
// provisioned or already cached.
func (m *AutoTLSManager) ProvisionCert(ctx context.Context, domain string) error {
	_, err := m.manager.GetCertificate(&tls.ClientHelloInfo{ServerName: domain})
	return err
}
