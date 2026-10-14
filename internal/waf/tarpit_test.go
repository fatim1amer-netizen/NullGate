package waf

import (
	"testing"
	"time"
)

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
	for i := 0; i < 3; i++ {
		if d := tp.Delay(ip); d != 0 {
			t.Fatalf("request %d under threshold should have no delay, got %v", i+1, d)
		}
	}
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
