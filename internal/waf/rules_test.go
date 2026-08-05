package waf

import "testing"

// TestSelfTestCorpusPasses is the headline correctness gate: the entire curated
// attack + benign corpus must classify perfectly under the default config.
func TestSelfTestCorpusPasses(t *testing.T) {
	cfg := DefaultConfig()
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("default config failed validation: %v", err)
	}
	report := RunSelfTest(cfg)

	if report.FalseNegatives != 0 {
		t.Errorf("expected 0 false negatives, got %d", report.FalseNegatives)
		for _, r := range report.Results {
			if r.Case.Malicious && !r.Blocked {
				t.Logf("  MISSED: %-30s payload=%q score=%d", r.Case.Name, r.Case.Payload, r.Score)
			}
		}
	}
	if report.FalsePositives != 0 {
		t.Errorf("expected 0 false positives, got %d", report.FalsePositives)
		for _, r := range report.Results {
			if !r.Case.Malicious && r.Blocked {
				t.Logf("  FALSE POSITIVE: %-30s payload=%q score=%d cat=%s",
					r.Case.Name, r.Case.Payload, r.Score, r.TopCat)
			}
		}
	}
	if report.DetectionRate() < 1.0 {
		t.Errorf("detection rate %.1f%% below 100%%", report.DetectionRate()*100)
	}
}

// TestParanoiaLevelsMonotonic verifies that raising paranoia never reduces the
// number of active rules — a higher setting must be at least as strict.
func TestParanoiaLevelsMonotonic(t *testing.T) {
	prev := -1
	for p := 1; p <= 4; p++ {
		n := NewRuleEngine(p, nil).RuleCount()
		if n < prev {
			t.Errorf("paranoia %d has fewer active rules (%d) than %d (%d)", p, n, p-1, prev)
		}
		prev = n
	}
}

func TestRuleEngineBlocksClassicAttacks(t *testing.T) {
	cfg := DefaultConfig()
	eng := NewRuleEngine(cfg.Rules.ParanoiaLevel, nil)
	dec := NewDecoder(cfg.Decoding)

	blocked := func(payload string) bool {
		var agg ScanResult
		for _, v := range dec.Variants(payload) {
			res := eng.Scan(v)
			agg.Matches = append(agg.Matches, res.Matches...)
			agg.AnomalyScore += res.AnomalyScore
			if res.HasCritical {
				agg.HasCritical = true
			}
		}
		return agg.Blocked(&cfg.Rules)
	}

	cases := []string{
		`' OR '1'='1`,
		`<script>alert(1)</script>`,
		`; cat /etc/passwd`,
		`../../../../etc/passwd`,
		`http://169.254.169.254/`,
	}
	for _, c := range cases {
		if !blocked(c) {
			t.Errorf("expected block for %q", c)
		}
	}
}

func TestBenignInputsPass(t *testing.T) {
	cfg := DefaultConfig()
	eng := NewRuleEngine(cfg.Rules.ParanoiaLevel, nil)
	dec := NewDecoder(cfg.Decoding)

	benign := []string{
		`hello world`,
		`user@example.com`,
		`{"name":"Alice","age":30}`,
		`/api/v1/products?page=2&sort=price`,
		`The quick brown fox jumps over the lazy dog`,
	}
	for _, b := range benign {
		var agg ScanResult
		for _, v := range dec.Variants(b) {
			res := eng.Scan(v)
			agg.Matches = append(agg.Matches, res.Matches...)
			agg.AnomalyScore += res.AnomalyScore
			if res.HasCritical {
				agg.HasCritical = true
			}
		}
		if agg.Blocked(&cfg.Rules) {
			t.Errorf("benign input wrongly blocked: %q (score %d, cat %s)", b, agg.AnomalyScore, agg.TopCategory())
		}
	}
}

// BenchmarkScan measures per-payload scan latency across the full rule set.
func BenchmarkScan(b *testing.B) {
	eng := NewRuleEngine(2, nil)
	dec := NewDecoder(DefaultConfig().Decoding)
	payload := `1 UNION SELECT username,password FROM users WHERE '1'='1`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range dec.Variants(payload) {
			eng.Scan(v)
		}
	}
}
