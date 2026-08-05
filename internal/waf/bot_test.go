package waf

import (
	"net/http"
	"testing"
)

func botDetector() *BotDetector {
	return NewBotDetector(BotMitigationConfig{
		Enabled:        true,
		BlockKnownBots: true,
		BlockEmptyUA:   true,
	})
}

func TestBotBlocksKnownScanners(t *testing.T) {
	b := botDetector()
	for _, ua := range []string{
		"sqlmap/1.7",
		"Nikto/2.5",
		"Mozilla/5.0 nuclei",
		"python-requests/2.31",
		"curl/8.4.0",
	} {
		r, _ := http.NewRequest("GET", "/", nil)
		r.Header.Set("User-Agent", ua)
		if v := b.Inspect(r); !v.IsBot {
			t.Errorf("UA %q should be flagged as bot", ua)
		}
	}
}

func TestBotBlocksEmptyUA(t *testing.T) {
	b := botDetector()
	r, _ := http.NewRequest("GET", "/", nil)
	// No User-Agent set at all.
	if v := b.Inspect(r); !v.IsBot {
		t.Error("empty user-agent should be flagged")
	}
}

func TestBotAllowsRealBrowser(t *testing.T) {
	b := botDetector()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	r.Header.Set("Accept-Encoding", "gzip, deflate, br")
	if v := b.Inspect(r); v.IsBot {
		t.Errorf("legitimate browser flagged as bot: %s", v.Reason)
	}
}

func TestBotDisabledPassesEverything(t *testing.T) {
	b := NewBotDetector(BotMitigationConfig{Enabled: false})
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent", "sqlmap/1.7")
	if v := b.Inspect(r); v.IsBot {
		t.Error("disabled detector must not flag anything")
	}
}
