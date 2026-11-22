package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nullgate/internal/waf"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to YAML configuration")
	forceAdmin := flag.Bool("admin", false, "enable the localhost admin dashboard")
	autoTLS := flag.Bool("auto-tls", false, "enable Let's Encrypt HTTPS")
	domains := flag.String("domain", "", "comma-separated domains for automatic TLS")
	demo := flag.Bool("demo", false, "register the two synthetic demo applications")
	appRegistry := flag.String("app-registry", "", "override the persistent application registry path")
	backend := flag.String("backend", "", "override server.target_backend")
	selftest := flag.Bool("selftest", false, "run the labeled rule-engine self-test")
	showConfig := flag.Bool("show-config", false, "print effective configuration and exit")
	generateSecret := flag.Bool("gen-secret", false, "print a random 32-byte secret and exit")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("NullGate", Version)
		return
	}
	if *generateSecret {
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			log.Fatal(err)
		}
		fmt.Println(hex.EncodeToString(secret[:]))
		return
	}

	cfg, err := waf.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *forceAdmin {
		cfg.Admin.Enabled = true
		if cfg.Admin.ListenAddr == "" {
			cfg.Admin.ListenAddr = "127.0.0.1:9091"
		}
		cfg.Admin.RequireLocalhost = true
	}
	if *autoTLS {
		cfg.Server.AutoTLS = true
	}
	if *domains != "" {
		for _, domain := range strings.Split(*domains, ",") {
			if trimmed := strings.TrimSpace(domain); trimmed != "" {
				cfg.Server.AutoTLSDomains = append(cfg.Server.AutoTLSDomains, trimmed)
			}
		}
	}
	if *appRegistry != "" {
		cfg.Applications.RegistryPath = *appRegistry
	}
	targetBackend := *backend
	if targetBackend == "" {
		targetBackend = os.Getenv("NULLGATE_TARGET")
	}
	if targetBackend != "" {
		cfg.Server.TargetBackend = targetBackend
	}
	if err := waf.ValidateConfig(cfg); err != nil {
		log.Fatal(err)
	}
	if *showConfig {
		cfg.Print()
		return
	}
	if *selftest {
		report := waf.RunSelfTest(cfg)
		fmt.Print(report.String())
		if report.FalseNegatives > 0 || report.FalsePositives > 0 {
			os.Exit(1)
		}
		return
	}

	legacyEngine, err := waf.NewEngine(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer legacyEngine.Close()
	apps, err := waf.NewApplicationManager(cfg, legacyEngine)
	if err != nil {
		log.Fatal(err)
	}
	defer apps.Close()
	if *demo {
		if err := apps.InstallDemoApplications(); err != nil {
			log.Fatal(err)
		}
	}

	publicServer := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           apps,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeout) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}

	var adminServer *http.Server
	if cfg.Admin.Enabled {
		admin := waf.NewAdminServer(
			cfg.Admin, cfg, legacyEngine.Metrics(), legacyEngine.Logger(),
			legacyEngine.Blocklist(), legacyEngine.Rules(), Version,
			legacyEngine.Rules().RuleCount(), legacyEngine.Rules().TotalRuleCount(),
		)
		admin.SetApplicationManager(apps)
		adminServer = &http.Server{
			Addr:              cfg.Admin.ListenAddr,
			Handler:           admin.Handler(),
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 2)
	if adminServer != nil {
		go func() {
			log.Printf("NullGate admin dashboard listening on %s", adminServer.Addr)
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("admin server: %w", err)
			}
		}()
	}

	go func() {
		var err error
		switch {
		case cfg.Server.AutoTLS:
			err = waf.StartAutoTLS(cfg, apps)
		case cfg.Server.EnableTLS:
			err = publicServer.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
		default:
			err = publicServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("public server: %w", err)
		}
	}()
	log.Printf("NullGate %s starting on %s", Version, cfg.Server.ListenAddr)

	select {
	case err := <-errCh:
		log.Printf("server stopped: %v", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = publicServer.Shutdown(shutdownCtx)
	if adminServer != nil {
		_ = adminServer.Shutdown(shutdownCtx)
	}
}
