package waf

import (
	"net/http"
	"strings"
)

// BotDetector classifies requests as automated based on User-Agent
// signatures and request-header heuristics. It intentionally errs toward the
// obvious: aggressive UA-only blocking causes collateral damage, so higher
// confidence comes from the behavioral subsystem, not here.
type BotDetector struct {
	cfg          BotMitigationConfig
	badUA        []string
	requireFetch bool
}

// BotMitigationConfig is defined here (rather than config.go) because bot
// mitigation gained fields during the redesign; kept in its own type for
// clarity and backward-compatible YAML.
type BotMitigationConfig struct {
	Enabled         bool `yaml:"enabled"`
	BlockKnownBots  bool `yaml:"block_known_bots"`
	RequireSecFetch bool `yaml:"require_sec_fetch"`
	BlockEmptyUA    bool `yaml:"block_empty_ua"`
}

// NewBotDetector builds a detector with a curated signature list.
func NewBotDetector(cfg BotMitigationConfig) *BotDetector {
	return &BotDetector{
		cfg: cfg,
		badUA: []string{
			// High-confidence security scanners / offensive tooling. Generic
			// HTTP clients and search crawlers are not blocked by their UA alone.
			"sqlmap", "nikto", "nmap", "masscan", "dirbuster", "gobuster",
			"wpscan", "acunetix", "nessus", "openvas", "nuclei", "feroxbuster",
			"whatweb", "hydra", "metasploit", "burpsuite", "zaproxy",
		},
		requireFetch: cfg.RequireSecFetch,
	}
}

// BotVerdict summarizes a classification with a reason for logging.
type BotVerdict struct {
	IsBot  bool
	Reason string
}

// Inspect classifies a request.
func (b *BotDetector) Inspect(r *http.Request) BotVerdict {
	if !b.cfg.Enabled {
		return BotVerdict{}
	}
	ua := strings.ToLower(strings.TrimSpace(r.UserAgent()))

	if ua == "" || ua == "-" {
		if b.cfg.BlockEmptyUA {
			return BotVerdict{IsBot: true, Reason: "empty user-agent"}
		}
	}

	if b.cfg.BlockKnownBots {
		for _, sig := range b.badUA {
			if strings.Contains(ua, sig) {
				return BotVerdict{IsBot: true, Reason: "ua signature: " + sig}
			}
		}
	}

	// Sec-Fetch-* headers are sent by modern browsers on navigations and
	// resource loads. Their total absence on a non-GET is a weak automation
	// signal; only enforced when explicitly enabled.
	if b.requireFetch && r.Method != http.MethodGet {
		if r.Header.Get("Sec-Fetch-Site") == "" && r.Header.Get("Sec-Fetch-Mode") == "" {
			return BotVerdict{IsBot: true, Reason: "missing Sec-Fetch headers"}
		}
	}

	return BotVerdict{}
}
