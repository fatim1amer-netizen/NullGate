package waf

import (
	"crypto/sha256"
	"net/http"
	"strconv"
	"testing"
)

func powTestConfig() PoWConfig {
	return PoWConfig{
		Enabled:      true,
		HMACSecret:   "test-secret-key-do-not-use-in-prod",
		Difficulty:   8, // one zero byte — cheap to solve in-test
		ChallengeTTL: 300,
	}
}

// solve brute-forces a nonce meeting difficulty for the given challenge.
func solve(challenge string, bits int) string {
	for i := 0; ; i++ {
		nonce := strconv.Itoa(i)
		if meetsDifficulty(challenge+nonce, bits) {
			return nonce
		}
	}
}

func TestPoWRoundTripValid(t *testing.T) {
	p := NewPoWValidator(powTestConfig())
	ip := "203.0.113.7"

	challenge, bits := p.GenerateChallenge(ip, 0)
	nonce := solve(challenge, bits)

	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "nullgate_pow", Value: challenge + ":" + nonce})

	if !p.Validate(req, ip) {
		t.Fatal("valid PoW solution was rejected")
	}
}

func TestPoWRejectsWrongNonce(t *testing.T) {
	p := NewPoWValidator(powTestConfig())
	ip := "203.0.113.7"
	challenge, _ := p.GenerateChallenge(ip, 0)

	req, _ := http.NewRequest("GET", "/", nil)
	// "0" almost certainly does not satisfy 8-bit difficulty.
	req.AddCookie(&http.Cookie{Name: "nullgate_pow", Value: challenge + ":0"})
	if p.Validate(req, ip) && meetsDifficulty(challenge+"0", 8) {
		t.Skip("unlucky: nonce 0 happened to solve; skipping")
	}
	if p.Validate(req, ip) {
		t.Fatal("PoW accepted a nonce that does not meet difficulty")
	}
}

// A challenge issued to one IP must not validate for another — this is the
// core anti-replay/anti-sharing property of the HMAC binding.
func TestPoWChallengeBoundToIP(t *testing.T) {
	p := NewPoWValidator(powTestConfig())
	victimIP := "198.51.100.1"
	attackerIP := "203.0.113.9"

	challenge, bits := p.GenerateChallenge(victimIP, 0)
	nonce := solve(challenge, bits)

	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "nullgate_pow", Value: challenge + ":" + nonce})

	if p.Validate(req, attackerIP) {
		t.Fatal("challenge issued for one IP validated for a different IP")
	}
	if !p.Validate(req, victimIP) {
		t.Fatal("challenge failed to validate for its own IP")
	}
}

// An expired challenge must be rejected even with a correct nonce.
func TestPoWRejectsExpiredChallenge(t *testing.T) {
	cfg := powTestConfig()
	cfg.ChallengeTTL = 60
	p := NewPoWValidator(cfg)
	ip := "203.0.113.7"

	// Pin issuedAt far in the past.
	old := int64(1000)
	challenge, bits := p.GenerateChallenge(ip, old)
	nonce := solve(challenge, bits)

	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "nullgate_pow", Value: challenge + ":" + nonce})

	if p.Validate(req, ip) {
		t.Fatal("expired challenge was accepted (replay window not enforced)")
	}
}

// A forged challenge (not signed by our secret) must not validate.
func TestPoWRejectsForgedChallenge(t *testing.T) {
	p := NewPoWValidator(powTestConfig())
	ip := "203.0.113.7"

	// Attacker fabricates "<now>.<garbage-mac>".
	forged := strconv.FormatInt(2_000_000_000, 10) + ".deadbeef"
	nonce := solve(forged, 8)

	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "nullgate_pow", Value: forged + ":" + nonce})

	if p.Validate(req, ip) {
		t.Fatal("forged challenge (bad MAC) was accepted")
	}
}

func TestPoWMissingCookie(t *testing.T) {
	p := NewPoWValidator(powTestConfig())
	req, _ := http.NewRequest("GET", "/", nil)
	if p.Validate(req, "203.0.113.7") {
		t.Fatal("validation passed with no PoW cookie present")
	}
}

func TestMeetsDifficultyBitLevel(t *testing.T) {
	// difficulty 0 always passes.
	if !meetsDifficulty("anything", 0) {
		t.Fatal("difficulty 0 should always pass")
	}
	// Construct data and assert consistency with a direct hash check.
	data := "nullgate"
	sum := sha256.Sum256([]byte(data))
	// The number of leading zero bits the sum actually has:
	leading := 0
	for _, b := range sum {
		if b == 0 {
			leading += 8
			continue
		}
		for m := byte(0x80); m != 0; m >>= 1 {
			if b&m == 0 {
				leading++
			} else {
				break
			}
		}
		break
	}
	if !meetsDifficulty(data, leading) {
		t.Fatalf("meetsDifficulty should accept exactly %d leading zero bits", leading)
	}
	if meetsDifficulty(data, leading+1) {
		t.Fatalf("meetsDifficulty should reject %d bits when only %d present", leading+1, leading)
	}
}
