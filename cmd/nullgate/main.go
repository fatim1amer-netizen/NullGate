package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"nullgate/internal/waf"
)

const Version = "2.0.0"

func main() {
	if err := run(); err != nil {
		log.Printf("nullgate: %v", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", defaultConfigPath(), "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	runSelfTest := flag.Bool("selftest", false, "Run the detection self-test and exit")
	genSecret := flag.Bool("gen-secret", false, "Generate a random HMAC secret and exit")
	showConfig := flag.Bool("show-config", false, "Print the effective configuration and exit")
	enableAdmin := flag.Bool("admin", false, "Enable the admin dashboard on the configured localhost address")
	autoTLS := flag.Bool("auto-tls", false, "Enable automatic HTTPS with Let's Encrypt")
	domains := flag.String("domain", "", "Comma-separated domains for automatic HTTPS")
	flag.Parse()

	if *showVersion {
		fmt.Printf("NullGate WAF v%s\n", Version)
		return nil
	}
	if *genSecret {
		secret, err := randomSecret(32)
		if err != nil {
			return fmt.Errorf("generate secret: %w", err)
		}
		fmt.Println(secret)
		return nil
	}

	cfg, err := waf.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if *enableAdmin {
		cfg.Admin.Enabled = true
		cfg.Admin.RequireLocalhost = true
		if cfg.Admin.ListenAddr == "" {
			cfg.Admin.ListenAddr = "127.0.0.1:9091"
		}
	}
	if *autoTLS {
		cfg.Server.AutoTLS = true
	}
	if *domains != "" {
		cfg.Server.AutoTLSDomains = splitCSV(*domains)
	}

	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("PORT must be between 1 and 65535")
		}
		cfg.Server.ListenAddr = net.JoinHostPort("", port)
	}
	if hash := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_HASH")); hash != "" {
		cfg.Admin.PasswordHash = hash
	}
	if password := os.Getenv("ADMIN_PASSWORD"); password != "" {
		hash, err := waf.HashPassword(password)
		if err != nil {
			return fmt.Errorf("hash ADMIN_PASSWORD: %w", err)
		}
		cfg.Admin.PasswordHash = hash
	}
	if backend := strings.TrimSpace(os.Getenv("TARGET_BACKEND")); backend != "" {
		cfg.Server.TargetBackend = backend
		cfg.Server.UpstreamHost = strings.TrimSpace(os.Getenv("UPSTREAM_HOST"))
	}
	if err := waf.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("invalid effective configuration: %w", err)
	}

	if *runSelfTest {
		report := waf.RunSelfTest(cfg)
		fmt.Print(report.String())
		if report.FalseNegatives > 0 || report.FalsePositives > 0 {
			return fmt.Errorf("self-test corpus did not pass")
		}
		return nil
	}
	if *showConfig {
		cfg.Print()
		return nil
	}

	engine, err := waf.NewEngine(cfg)
	if err != nil {
		return fmt.Errorf("initialize WAF: %w", err)
	}
	handler := http.Handler(engine)
	if cfg.Metrics.Enabled {
		metricsPath := cfg.Metrics.Path
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		handler = withMetrics(handler, metricsPath, engine)
	}
	handler = withHealthz(handler, engine)

	server := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           handler,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	var adminServer *http.Server
	var adminListener net.Listener
	if cfg.Admin.Enabled {
		admin := waf.NewAdminServer(
			cfg.Admin, cfg, engine.Metrics(), engine.Logger(), engine.Blocklist(),
			engine.Rules(), Version, engine.Rules().RuleCount(), engine.Rules().TotalRuleCount(), engine.ProbeBackend,
		)
		adminServer = &http.Server{
			Addr:              cfg.Admin.ListenAddr,
			Handler:           admin.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
		}
		adminListener, err = net.Listen("tcp", cfg.Admin.ListenAddr)
		if err != nil {
			return fmt.Errorf("listen on admin address %s: %w", cfg.Admin.ListenAddr, err)
		}
		go func() {
			if err := adminServer.Serve(adminListener); err != nil && err != http.ErrServerClosed {
				log.Printf("admin server error: %v", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		if cfg.Server.AutoTLS {
			errCh <- waf.StartAutoTLS(cfg, handler)
			return
		}
		if cfg.Server.EnableTLS {
			errCh <- server.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
			return
		}
		errCh <- server.ListenAndServe()
	}()

	log.Printf("NullGate WAF v%s starting on %s; proxying to %s",
		Version, cfg.Server.ListenAddr, cfg.Server.TargetBackend)
	if cfg.Admin.Enabled {
		log.Printf("admin dashboard listening on %s", cfg.Admin.ListenAddr)
	}
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			if adminServer != nil {
				_ = adminServer.Close()
			}
			return fmt.Errorf("public server failed: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := shutdownServers(shutdownCtx, server, adminServer); err != nil {
		return err
	}
	return nil
}

func shutdownServers(ctx context.Context, public, admin *http.Server) error {
	if err := public.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down public server: %w", err)
	}
	if admin != nil {
		if err := admin.Shutdown(ctx); err != nil {
			return fmt.Errorf("shut down admin server: %w", err)
		}
	}
	return nil
}

func defaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("NULLGATE_CONFIG")); path != "" {
		return path
	}
	if executable, err := os.Executable(); err == nil {
		for _, name := range []string{"configs/config.yaml", "config.yaml"} {
			path := filepath.Join(filepath.Dir(executable), name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return filepath.Join("configs", "config.yaml")
}

func withMetrics(next http.Handler, path string, engine *waf.Engine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
			fmt.Fprint(w, engine.Metrics().Prometheus())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withHealthz(next http.Handler, engine *waf.Engine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if r.URL.Path == "/readyz" {
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer cancel()
				health := engine.ProbeBackend(ctx)
				if health.State != "healthy" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprintf(w, `{"status":"not_ready","backend":%q,"http_status":%d}`, health.State, health.HTTPStatus)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.URL.Path == "/readyz" {
				fmt.Fprint(w, `{"status":"ready","backend":"healthy"}`)
			} else {
				fmt.Fprint(w, `{"status":"ok"}`)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
