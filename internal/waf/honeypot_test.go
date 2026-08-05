package waf

import (
	"net/http"
	"testing"
	"time"
)

func TestHoneypotTriggersAndBlocks(t *testing.T) {
	bl := NewBlocklist()
	h := NewHoneypot(HoneypotConfig{
		Enabled:          true,
		ProbePaths:       []string{"/.env", "/.git/config", "/cgi-bin/"},
		BlockDurationSec: 3600,
	}, bl)

	ip := "203.0.113.50"
	r, _ := http.NewRequest("GET", "/.env", nil)
	triggered, reason := h.Triggered(r, ip)
	if !triggered {
		t.Fatal("honeypot should trigger on /.env")
	}
	if reason == "" {
		t.Error("expected a non-empty reason")
	}
	if !bl.IsBlocked(ip) {
		t.Error("honeypot hit should add the client to the blocklist")
	}
}

func TestHoneypotPrefixMatch(t *testing.T) {
	bl := NewBlocklist()
	h := NewHoneypot(HoneypotConfig{
		Enabled:          true,
		ProbePaths:       []string{"/cgi-bin/"},
		BlockDurationSec: 60,
	}, bl)
	r, _ := http.NewRequest("GET", "/cgi-bin/test.cgi", nil)
	if triggered, _ := h.Triggered(r, "203.0.113.51"); !triggered {
		t.Error("prefix probe path should trigger")
	}
}

func TestHoneypotIgnoresLegitPaths(t *testing.T) {
	bl := NewBlocklist()
	h := NewHoneypot(HoneypotConfig{
		Enabled:    true,
		ProbePaths: []string{"/.env"},
	}, bl)
	r, _ := http.NewRequest("GET", "/api/users", nil)
	if triggered, _ := h.Triggered(r, "203.0.113.52"); triggered {
		t.Error("legitimate path should not trigger honeypot")
	}
}

func TestHoneypotLogOnlyDoesNotBlock(t *testing.T) {
	bl := NewBlocklist()
	h := NewHoneypot(HoneypotConfig{
		Enabled:          true,
		ProbePaths:       []string{"/.env"},
		BlockDurationSec: 60,
		LogOnly:          true,
	}, bl)
	r, _ := http.NewRequest("GET", "/.env", nil)
	triggered, _ := h.Triggered(r, "203.0.113.53")
	if !triggered {
		t.Error("log-only honeypot should still report a trigger")
	}
	if bl.IsBlocked("203.0.113.53") {
		t.Error("log-only honeypot must not add to the blocklist")
	}
}

func TestTarpitDelaysPersistentClients(t *testing.T) {
	tp := NewTarpit(TarpitConfig{
		Enabled:               true,
		RequestCountThreshold: 3,
		MinDelayMS:            10,
		StepMS:                5,
		MaxDelayMS:            100,
		WindowSec:             10,
	})
	ip := "203.0.113.60"
	// First requests within threshold: no delay.
	for i := 0; i < 3; i++ {
		if d := tp.Delay(ip); d != 0 {
			t.Fatalf("request %d under threshold should have no delay, got %v", i+1, d)
		}
	}
	// Next request crosses threshold: must impose a delay.
	if d := tp.Delay(ip); d <= 0 {
		t.Error("request over threshold should be delayed")
	}
}

func TestTarpitDisabled(t *testing.T) {
	tp := NewTarpit(TarpitConfig{Enabled: false})
	for i := 0; i < 100; i++ {
		if tp.Delay("203.0.113.61") != 0 {
			t.Fatal("disabled tarpit must never delay")
		}
	}
}

func TestTarpitCapsAtMax(t *testing.T) {
	tp := NewTarpit(TarpitConfig{
		Enabled:               true,
		RequestCountThreshold: 1,
		MinDelayMS:            10,
		StepMS:                1000,
		MaxDelayMS:            50,
		WindowSec:             10,
	})
	ip := "203.0.113.62"
	var maxSeen time.Duration
	for i := 0; i < 20; i++ {
		if d := tp.Delay(ip); d > maxSeen {
			maxSeen = d
		}
	}
	if maxSeen > 50*time.Millisecond {
		t.Errorf("delay exceeded MaxDelayMS cap: %v", maxSeen)
	}
}
