package waf

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for NullGate WAF.
// Every subsystem reads its tuning parameters from here so that a
// single YAML file controls the whole security posture.
type Config struct {
	Server    ServerConfig        `yaml:"server"`
	Protocol  ProtocolConfig      `yaml:"protocol"`
	Rules     RulesConfig         `yaml:"rules"`
	Decoding  DecodingConfig      `yaml:"decoding"`
	RateLimit RateLimitConfig     `yaml:"rate_limit"`
	PoW       PoWConfig           `yaml:"pow"`
	Bot       BotMitigationConfig `yaml:"bot"`
	Behavior  BehaviorConfig      `yaml:"behavior"`
	CSRF      CSRFConfig          `yaml:"csrf"`
	Response  ResponseConfig      `yaml:"response"`
	Tarpit    TarpitConfig        `yaml:"tarpit"`
	Admin     AdminConfig         `yaml:"admin"`
	Metrics   MetricsConfig       `yaml:"metrics"`
	Logging   LoggingConfig       `yaml:"logging"`
	IP        IPConfig            `yaml:"ip"`
}

// DefaultConfig returns a fully populated, production-sane configuration.
// Keeping defaults in code (not only in the sample YAML) guarantees that a
// user who hand-writes a minimal config still gets a safe posture.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddr:        ":8080",
			TargetBackend:     "http://127.0.0.1:3000",
			EnableTLS:         false,
			TLSCert:           "",
			TLSKey:            "",
			ReadTimeout:       15,
			ReadHeaderTimeout: 5,
			WriteTimeout:      15,
			IdleTimeout:       60,
			MaxHeaderBytes:    1 << 20,
			TrustedProxies:    []string{},
		},
		Protocol: ProtocolConfig{
			MaxURLLength:                8192,
			MaxHeaderCount:              60,
			MaxQueryParams:              128,
			MaxMethodLength:             16,
			AllowedMethods:              []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
			StrictVersionCheck:          true,
			EnforceContentType:          true,
			RejectChunkedBodies:         false,
			RejectContentLengthMismatch: true,
			RejectConflictingEncoding:   true,
			MaxAllowedContentTypes:      []string{"application/x-www-form-urlencoded", "multipart/form-data", "application/json", "text/plain", "application/xml", "text/xml", "application/octet-stream"},
			BlockNullBytes:              true,
			BlockControlChars:           true,
			MaxRequestSize:              32 << 20,
		},
		Rules: RulesConfig{
			ParanoiaLevel:    2,
			AnomalyThreshold: 8,
			BlockOnCritical:  true,
			RuleActions: map[string]string{
				"critical": "block",
				"high":     "block",
				"medium":   "log",
				"low":      "log",
			},
		},
		Decoding: DecodingConfig{
			MaxDecodeDepth:       5,
			MaxPayloadSize:       10 << 20,
			MaxRequestSize:       32 << 20,
			EnableGzipScan:       true,
			EnableDeflateScan:    true,
			EnableBrotliScan:     false,
			EnableJSONScan:       true,
			EnableMultipartScan:  true,
			EnableBase64Scan:     true,
			EnableHexScan:        true,
			EnableUnicodeScan:    true,
			EnableCommentScan:    true,
			EnableWhitespaceScan: true,
		},
		RateLimit: RateLimitConfig{
			Enabled:            true,
			RequestsPerSecond:  10,
			BurstSize:          30,
			EscalationAfter:    10,
			BlockAfter:         20,
			BlockDurationSec:   300,
			CleanupIntervalSec: 60,
			PerIP:              true,
			PerRoute:           false,
		},
		PoW: PoWConfig{
			Enabled:      false,
			Difficulty:   4,
			ChallengeTTL: 300,
			HMACSecret:   "change-me-in-production",
		},
		Bot: BotMitigationConfig{
			Enabled:         true,
			BlockKnownBots:  true,
			RequireSecFetch: false,
			BlockEmptyUA:    false,
		},
		Behavior: BehaviorConfig{
			Enabled:                   true,
			MaxPathDepth:              12,
			MaxExtensionsInPath:       8,
			ConsecutiveBlockThreshold: 5,
			CooldownSec:               600,
			TrackEmptyUA:              true,
			TrackSuspiciousUA:         true,
			TrackMethods:              true,
			BlockScannerProbe:         true,
			ScannerBlockSec:           3600,
		},
		CSRF: CSRFConfig{
			Enabled:        true,
			TokenTTLMin:    30,
			ExcludePaths:   []string{"/healthz", "/metrics"},
			ExcludeMethods: []string{"GET", "HEAD", "OPTIONS"},
		},
		Response: ResponseConfig{
			Enabled:            true,
			AddSecurityHeaders: true,
			StripServerHeader:  true,
			StripXPoweredBy:    true,
			AddCSP:             true,
			AddHSTS:            false,
			RedactCookies:      true,
			RedactHeaders:      []string{"authorization", "x-api-key", "proxy-authorization", "cookie"},
		},
		Tarpit: TarpitConfig{
			Enabled:               true,
			MaxDelayMS:            30000,
			MinDelayMS:            1000,
			StepMS:                2000,
			RequestCountThreshold: 100,
			WindowSec:             10,
		},
		Admin: AdminConfig{
			Enabled:          false,
			ListenAddr:       "127.0.0.1:9091",
			Username:         "admin",
			PasswordHash:     "",
			ReadOnly:         true,
			RequireLocalhost: true,
		},
		Metrics: MetricsConfig{
			Enabled:          true,
			Path:             "/metrics",
			PrometheusFormat: true,
		},
		Logging: LoggingConfig{
			Level:           "info",
			Format:          "json",
			Output:          "stdout",
			FilePath:        "logs/waf.log",
			MaxSizeMB:       100,
			MaxBackups:      7,
			MaxAgeDays:      30,
			EnableAccessLog: true,
			AccessLogTarget: "stdout",
		},
		IP: IPConfig{
			TrustedProxies:     []string{},
			TrustXForwardedFor: false,
			TrustXRealIP:       false,
			MaxForwardedChain:  5,
			EnableGeolocation:  false,
		},
	}
}

type ServerConfig struct {
	ListenAddr        string   `yaml:"listen_addr"`
	TargetBackend     string   `yaml:"target_backend"`
	UpstreamHost      string   `yaml:"upstream_host"`
	ServeLanding      bool     `yaml:"serve_landing"` // serve the NullGate landing page at "/" instead of proxying it
	EnableTLS         bool     `yaml:"enable_tls"`
	TLSCert           string   `yaml:"tls_cert"`
	TLSKey            string   `yaml:"tls_key"`
	AutoTLS           bool     `yaml:"auto_tls"` // provision certs from Let's Encrypt
	AutoTLSDomains    []string `yaml:"auto_tls_domains"`
	AutoTLSCacheDir   string   `yaml:"auto_tls_cache_dir"` // where to persist issued certs
	AutoTLSEmail      string   `yaml:"auto_tls_email"`     // ACME account contact
	ReadTimeout       int      `yaml:"read_timeout_sec"`
	ReadHeaderTimeout int      `yaml:"read_header_timeout_sec"`
	WriteTimeout      int      `yaml:"write_timeout_sec"`
	IdleTimeout       int      `yaml:"idle_timeout_sec"`
	MaxHeaderBytes    int      `yaml:"max_header_bytes"`
	TrustedProxies    []string `yaml:"trusted_proxies"`
}

type ProtocolConfig struct {
	MaxURLLength                int      `yaml:"max_url_length"`
	MaxHeaderCount              int      `yaml:"max_header_count"`
	MaxQueryParams              int      `yaml:"max_query_params"`
	MaxMethodLength             int      `yaml:"max_method_length"`
	AllowedMethods              []string `yaml:"allowed_methods"`
	StrictVersionCheck          bool     `yaml:"strict_version_check"`
	EnforceContentType          bool     `yaml:"enforce_content_type"`
	RejectChunkedBodies         bool     `yaml:"reject_chunked_bodies"`
	RejectContentLengthMismatch bool     `yaml:"reject_content_length_mismatch"`
	RejectConflictingEncoding   bool     `yaml:"reject_conflicting_encoding"`
	MaxAllowedContentTypes      []string `yaml:"max_allowed_content_types"`
	BlockNullBytes              bool     `yaml:"block_null_bytes"`
	BlockControlChars           bool     `yaml:"block_control_chars"`
	MaxRequestSize              int      `yaml:"max_request_size"`
}

type RulesConfig struct {
	ParanoiaLevel    int               `yaml:"paranoia_level"`
	AnomalyThreshold int               `yaml:"anomaly_threshold"`
	BlockOnCritical  bool              `yaml:"block_on_critical"`
	RuleActions      map[string]string `yaml:"rule_actions"`
}

type DecodingConfig struct {
	MaxDecodeDepth       int  `yaml:"max_decode_depth"`
	MaxPayloadSize       int  `yaml:"max_payload_size"`
	MaxRequestSize       int  `yaml:"max_request_size"`
	EnableGzipScan       bool `yaml:"enable_gzip_scan"`
	EnableDeflateScan    bool `yaml:"enable_deflate_scan"`
	EnableBrotliScan     bool `yaml:"enable_brotli_scan"`
	EnableJSONScan       bool `yaml:"enable_json_scan"`
	EnableMultipartScan  bool `yaml:"enable_multipart_scan"`
	EnableBase64Scan     bool `yaml:"enable_base64_scan"`
	EnableHexScan        bool `yaml:"enable_hex_scan"`
	EnableUnicodeScan    bool `yaml:"enable_unicode_scan"`
	EnableCommentScan    bool `yaml:"enable_comment_scan"`
	EnableWhitespaceScan bool `yaml:"enable_whitespace_scan"`
}

type RateLimitConfig struct {
	Enabled            bool `yaml:"enabled"`
	RequestsPerSecond  int  `yaml:"requests_per_second"`
	BurstSize          int  `yaml:"burst_size"`
	EscalationAfter    int  `yaml:"escalation_after"`
	BlockAfter         int  `yaml:"block_after"`
	BlockDurationSec   int  `yaml:"block_duration_sec"`
	CleanupIntervalSec int  `yaml:"cleanup_interval_sec"`
	PerIP              bool `yaml:"per_ip"`
	PerRoute           bool `yaml:"per_route"`
}

type PoWConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Difficulty   int    `yaml:"difficulty"`
	ChallengeTTL int    `yaml:"challenge_ttl"`
	HMACSecret   string `yaml:"hmac_secret"`
}

type BehaviorConfig struct {
	Enabled                   bool `yaml:"enabled"`
	MaxPathDepth              int  `yaml:"max_path_depth"`
	MaxExtensionsInPath       int  `yaml:"max_extensions_in_path"`
	ConsecutiveBlockThreshold int  `yaml:"consecutive_block_threshold"`
	CooldownSec               int  `yaml:"cooldown_sec"`
	TrackEmptyUA              bool `yaml:"track_empty_ua"`
	TrackSuspiciousUA         bool `yaml:"track_suspicious_ua"`
	TrackMethods              bool `yaml:"track_methods"`
	BlockScannerProbe         bool `yaml:"block_scanner_probe"`
	ScannerBlockSec           int  `yaml:"scanner_block_sec"`
}

type CSRFConfig struct {
	Enabled        bool     `yaml:"enabled"`
	TokenTTLMin    int      `yaml:"token_ttl_min"`
	ExcludePaths   []string `yaml:"exclude_paths"`
	ExcludeMethods []string `yaml:"exclude_methods"`
}

type ResponseConfig struct {
	Enabled            bool     `yaml:"enabled"`
	AddSecurityHeaders bool     `yaml:"add_security_headers"`
	StripServerHeader  bool     `yaml:"strip_server_header"`
	StripXPoweredBy    bool     `yaml:"strip_x_powered_by"`
	AddCSP             bool     `yaml:"add_csp"`
	AddHSTS            bool     `yaml:"add_hsts"`
	RedactCookies      bool     `yaml:"redact_cookies"`
	RedactHeaders      []string `yaml:"redact_headers"`
}

type TarpitConfig struct {
	Enabled               bool `yaml:"enabled"`
	MaxDelayMS            int  `yaml:"max_delay_ms"`
	MinDelayMS            int  `yaml:"min_delay_ms"`
	StepMS                int  `yaml:"step_ms"`
	RequestCountThreshold int  `yaml:"request_count_threshold"`
	WindowSec             int  `yaml:"window_sec"`
}

type AdminConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ListenAddr       string `yaml:"listen_addr"`
	Username         string `yaml:"username"`
	PasswordHash     string `yaml:"password_hash"`
	ReadOnly         bool   `yaml:"read_only"`
	RequireLocalhost bool   `yaml:"require_localhost"`
	// SessionTTLMin controls how long a dashboard login session stays valid.
	// When 0 a sane default (720 min = 12h) is used.
	SessionTTLMin int `yaml:"session_ttl_min"`
}

type MetricsConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Path             string `yaml:"path"`
	PrometheusFormat bool   `yaml:"prometheus_format"`
}

type LoggingConfig struct {
	Level           string `yaml:"level"`
	Format          string `yaml:"format"`
	Output          string `yaml:"output"`
	FilePath        string `yaml:"file_path"`
	MaxSizeMB       int    `yaml:"max_size_mb"`
	MaxBackups      int    `yaml:"max_backups"`
	MaxAgeDays      int    `yaml:"max_age_days"`
	EnableAccessLog bool   `yaml:"enable_access_log"`
	AccessLogTarget string `yaml:"access_log_target"`
}

type IPConfig struct {
	TrustedProxies     []string `yaml:"trusted_proxies"`
	TrustXForwardedFor bool     `yaml:"trust_x_forwarded_for"`
	TrustXRealIP       bool     `yaml:"trust_x_real_ip"`
	MaxForwardedChain  int      `yaml:"max_forwarded_chain"`
	EnableGeolocation  bool     `yaml:"enable_geolocation"`
}

// Print renders the effective configuration as a readable tree. Called by
// `nullgate --show-config` so operators can verify what a config file actually
// resolved to (after clamping and default-filling).
func (c *Config) Print() {
	fmt.Printf("NullGate WAF configuration\n")
	fmt.Printf("───────────────────────────\n")
	fmt.Printf("server.listen_addr:          %s\n", c.Server.ListenAddr)
	fmt.Printf("server.target_backend:       %s\n", c.Server.TargetBackend)
	fmt.Printf("server.upstream_host:        %s\n", c.Server.UpstreamHost)
	fmt.Printf("server.tls:                  %v\n", c.Server.EnableTLS)
	fmt.Printf("protocol.max_url_length:     %d\n", c.Protocol.MaxURLLength)
	fmt.Printf("protocol.max_header_count:   %d\n", c.Protocol.MaxHeaderCount)
	fmt.Printf("protocol.max_request_size:   %d bytes\n", c.Protocol.MaxRequestSize)
	fmt.Printf("rules.paranoia_level:        %d\n", c.Rules.ParanoiaLevel)
	fmt.Printf("rules.anomaly_threshold:     %d\n", c.Rules.AnomalyThreshold)
	fmt.Printf("rules.block_on_critical:     %v\n", c.Rules.BlockOnCritical)
	fmt.Printf("decoding.max_decode_depth:   %d\n", c.Decoding.MaxDecodeDepth)
	fmt.Printf("decoding.max_payload_size:   %d bytes\n", c.Decoding.MaxPayloadSize)
	fmt.Printf("decoding.json_scan:          %v\n", c.Decoding.EnableJSONScan)
	fmt.Printf("decoding.multipart_scan:     %v\n", c.Decoding.EnableMultipartScan)
	fmt.Printf("rate_limit.enabled:          %v (%.0f/s, burst %d)\n",
		c.RateLimit.Enabled, float64(c.RateLimit.RequestsPerSecond), c.RateLimit.BurstSize)
	fmt.Printf("rate_limit.escalate@:        %d violations → PoW, %d → block %ds\n",
		c.RateLimit.EscalationAfter, c.RateLimit.BlockAfter, c.RateLimit.BlockDurationSec)
	fmt.Printf("pow.enabled:                 %v (difficulty %d, ttl %ds)\n",
		c.PoW.Enabled, c.PoW.Difficulty, c.PoW.ChallengeTTL)
	fmt.Printf("bot.enabled:                 %v (known-bots %v, empty-ua %v)\n",
		c.Bot.Enabled, c.Bot.BlockKnownBots, c.Bot.BlockEmptyUA)
	fmt.Printf("behavior.enabled:            %v\n", c.Behavior.Enabled)
	fmt.Printf("csrf.enabled:                %v (ttl %d min)\n", c.CSRF.Enabled, c.CSRF.TokenTTLMin)
	fmt.Printf("tarpit.enabled:              %v\n", c.Tarpit.Enabled)
	fmt.Printf("response.harden:             %v\n", c.Response.Enabled)
	fmt.Printf("admin.enabled:               %v (%s)\n", c.Admin.Enabled, c.Admin.ListenAddr)
	fmt.Printf("metrics.enabled:             %v (path %s)\n", c.Metrics.Enabled, c.Metrics.Path)
	fmt.Printf("logging.level:               %s (format %s)\n", c.Logging.Level, c.Logging.Format)
	fmt.Printf("ip.trusted_proxies:          %v\n", c.IP.TrustedProxies)
	fmt.Printf("ip.trust_x_forwarded_for:    %v\n", c.IP.TrustXForwardedFor)
}

// LoadConfig reads and validates a YAML configuration file.
// Values omitted from the file inherit the safe defaults.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// ValidateConfig runs sanity checks and normalizes a config, clamping values
// that would otherwise crash the runtime or open security holes.
func ValidateConfig(cfg *Config) error {
	if cfg.Server.ListenAddr == "" {
		return fmt.Errorf("server.listen_addr is required")
	}
	if cfg.Server.TargetBackend == "" {
		return fmt.Errorf("server.target_backend is required")
	}
	backend, err := url.Parse(cfg.Server.TargetBackend)
	if err != nil || (backend.Scheme != "http" && backend.Scheme != "https") || backend.Hostname() == "" {
		return fmt.Errorf("server.target_backend must be an absolute http or https URL")
	}
	if strings.ContainsAny(cfg.Server.UpstreamHost, "/\\ \t\r\n") {
		return fmt.Errorf("server.upstream_host must contain only a host name and optional port")
	}
	if cfg.Server.UpstreamHost != "" {
		hostURL, err := url.Parse("http://" + cfg.Server.UpstreamHost)
		if err != nil || hostURL.Hostname() == "" || hostURL.Host != cfg.Server.UpstreamHost {
			return fmt.Errorf("server.upstream_host is invalid")
		}
	}
	if cfg.Server.EnableTLS {
		if cfg.Server.TLSCert == "" || cfg.Server.TLSKey == "" {
			return fmt.Errorf("TLS enabled but tls_cert or tls_key is missing")
		}
	}

	// Clamp decode depth to bound CPU work per request.
	if cfg.Decoding.MaxDecodeDepth < 1 {
		cfg.Decoding.MaxDecodeDepth = 1
	}
	if cfg.Decoding.MaxDecodeDepth > 10 {
		cfg.Decoding.MaxDecodeDepth = 10
	}
	if cfg.Decoding.MaxPayloadSize <= 0 {
		cfg.Decoding.MaxPayloadSize = 1 << 20
	}
	if cfg.Decoding.MaxRequestSize < cfg.Decoding.MaxPayloadSize {
		cfg.Decoding.MaxRequestSize = cfg.Decoding.MaxPayloadSize
	}

	// The protocol guard rejects a Content-Length larger than it will read;
	// keep it aligned with the decoder's request ceiling so the two layers
	// agree on the maximum acceptable body.
	if cfg.Protocol.MaxRequestSize <= 0 {
		cfg.Protocol.MaxRequestSize = cfg.Decoding.MaxRequestSize
	}

	if cfg.Rules.ParanoiaLevel < 1 {
		cfg.Rules.ParanoiaLevel = 1
	}
	if cfg.Rules.ParanoiaLevel > 4 {
		cfg.Rules.ParanoiaLevel = 4
	}
	if cfg.Rules.AnomalyThreshold < 1 {
		cfg.Rules.AnomalyThreshold = 1
	}

	if cfg.PoW.Enabled {
		if cfg.PoW.HMACSecret == "" || cfg.PoW.HMACSecret == "change-me-in-production" {
			return fmt.Errorf("pow.hmac_secret must be set to a strong random value in production")
		}
		if cfg.PoW.Difficulty < 1 || cfg.PoW.Difficulty > 10 {
			return fmt.Errorf("pow.difficulty must be between 1 and 10")
		}
		if cfg.PoW.ChallengeTTL < 30 || cfg.PoW.ChallengeTTL > 3600 {
			return fmt.Errorf("pow.challenge_ttl must be between 30 and 3600 seconds")
		}
	}

	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.RequestsPerSecond <= 0 {
			return fmt.Errorf("rate_limit.requests_per_second must be > 0")
		}
		if cfg.RateLimit.BurstSize <= 0 {
			cfg.RateLimit.BurstSize = cfg.RateLimit.RequestsPerSecond
		}
		if cfg.RateLimit.EscalationAfter <= 0 {
			cfg.RateLimit.EscalationAfter = cfg.RateLimit.RequestsPerSecond
		}
		if cfg.RateLimit.BlockAfter <= cfg.RateLimit.EscalationAfter {
			cfg.RateLimit.BlockAfter = cfg.RateLimit.EscalationAfter + 1
		}
	}

	if cfg.Behavior.Enabled {
		if cfg.Behavior.ConsecutiveBlockThreshold <= 0 {
			cfg.Behavior.ConsecutiveBlockThreshold = 5
		}
		if cfg.Behavior.CooldownSec <= 0 {
			cfg.Behavior.CooldownSec = 600
		}
	}

	// Normalize rule actions to a known set, ignoring unknown severities.
	for sev, action := range cfg.Rules.RuleActions {
		action = strings.ToLower(strings.TrimSpace(action))
		switch action {
		case "block", "log", "allow":
			cfg.Rules.RuleActions[sev] = action
		default:
			delete(cfg.Rules.RuleActions, sev)
		}
	}

	return nil
}
