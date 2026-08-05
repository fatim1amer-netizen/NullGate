package waf

import (
	"regexp"
	"strings"
	"sync/atomic"
)

// Severity classifies how dangerous a matched rule is. The engine converts
// severity into an anomaly score; enough accumulated score blocks a request
// even when no single rule is individually decisive (collaborative detection).
type Severity int

const (
	SevLow Severity = iota
	SevMedium
	SevHigh
	SevCritical
)

func (s Severity) String() string {
	switch s {
	case SevCritical:
		return "critical"
	case SevHigh:
		return "high"
	case SevMedium:
		return "medium"
	default:
		return "low"
	}
}

// Score is the anomaly weight contributed by a rule of this severity.
func (s Severity) Score() int {
	switch s {
	case SevCritical:
		return 10
	case SevHigh:
		return 5
	case SevMedium:
		return 3
	default:
		return 1
	}
}

// Category groups rules by attack class for reporting and per-class toggles.
type Category string

const (
	CatSQLi           Category = "sqli"
	CatXSS            Category = "xss"
	CatRCE            Category = "rce"
	CatLFI            Category = "lfi"
	CatSSRF           Category = "ssrf"
	CatXXE            Category = "xxe"
	CatSSTI           Category = "ssti"
	CatNoSQLi         Category = "nosqli"
	CatDeserialize    Category = "deserialize"
	CatProtoPollution Category = "prototype_pollution"
	CatCRLF           Category = "crlf"
	CatScanner        Category = "scanner"
)

// Rule is a single compiled detection signature.
type Rule struct {
	ID       string
	Category Category
	Severity Severity
	Paranoia int // minimum paranoia level at which this rule is active (1-4)
	Desc     string
	re       *regexp.Regexp
}

// Match reports whether the (already-decoded) payload triggers this rule.
func (r *Rule) Match(payload string) bool {
	return r.re.MatchString(payload)
}

// MatchResult describes one rule hit within a payload.
type MatchResult struct {
	RuleID   string   `json:"rule_id"`
	Category Category `json:"category"`
	Severity Severity `json:"severity"`
	Desc     string   `json:"description"`
}

// ScanResult is the aggregate verdict for a payload after all rules ran.
type ScanResult struct {
	Matches      []MatchResult `json:"matches"`
	AnomalyScore int           `json:"anomaly_score"`
	HasCritical  bool          `json:"has_critical"`
}

// Blocked decides, given the config, whether this scan warrants a block.
// The decision honors the per-severity rule_actions table first (a severity
// configured to "block" stops the request immediately — decisive attacks do
// not need to accumulate score), then falls back to cumulative anomaly score
// for the low-and-slow mix that no single rule catches.
func (sr *ScanResult) Blocked(cfg *RulesConfig) bool {
	if len(sr.Matches) == 0 {
		return false
	}
	if sr.HasCritical && cfg.BlockOnCritical {
		return true
	}
	for _, m := range sr.Matches {
		if cfg.RuleActions[m.Severity.String()] == "block" {
			return true
		}
	}
	return sr.AnomalyScore >= cfg.AnomalyThreshold
}

// TopCategory returns the category of the highest-severity match, for logging.
func (sr *ScanResult) TopCategory() Category {
	best := SevLow
	var cat Category = "none"
	for _, m := range sr.Matches {
		if m.Severity >= best {
			best = m.Severity
			cat = m.Category
		}
	}
	return cat
}

// RuleEngine holds the compiled rule set and evaluates payloads against it.
// All regexes are compiled once at construction; hot-path scanning performs
// no compilation, which keeps per-request latency low and predictable.
//
// paranoia is stored atomically so an operator can retune the effective rule
// set live from the admin dashboard without a restart; the hot path reads it
// with a single relaxed load.
type RuleEngine struct {
	rules       []*Rule
	paranoia    atomic.Int32
	customRules []*Rule
}

// NewRuleEngine compiles the built-in rule set plus any custom patterns from
// the configuration. Custom patterns are treated as high-severity generic rules.
func NewRuleEngine(paranoia int, customPatterns []string) *RuleEngine {
	e := &RuleEngine{}
	e.paranoia.Store(int32(paranoia))
	e.rules = compileBuiltinRules()

	for i, pat := range customPatterns {
		re, err := regexp.Compile(pat)
		if err != nil {
			continue // skip invalid custom patterns rather than crash
		}
		e.customRules = append(e.customRules, &Rule{
			ID:       "CUSTOM-" + itoa(i+1),
			Category: "custom",
			Severity: SevHigh,
			Paranoia: 1,
			Desc:     "custom pattern: " + pat,
			re:       re,
		})
	}
	return e
}

// Scan evaluates a decoded payload and returns the aggregate result.
// It short-circuits scanning of empty payloads for performance.
func (e *RuleEngine) Scan(payload string) ScanResult {
	var res ScanResult
	if payload == "" {
		return res
	}

	currentParanoia := int(e.paranoia.Load())
	eval := func(r *Rule) {
		if r.Paranoia > currentParanoia {
			return
		}
		if r.Match(payload) {
			res.Matches = append(res.Matches, MatchResult{
				RuleID:   r.ID,
				Category: r.Category,
				Severity: r.Severity,
				Desc:     r.Desc,
			})
			res.AnomalyScore += r.Severity.Score()
			if r.Severity == SevCritical {
				res.HasCritical = true
			}
		}
	}

	for _, r := range e.rules {
		eval(r)
	}
	for _, r := range e.customRules {
		eval(r)
	}
	return res
}

// RuleCount reports how many rules are active at the current paranoia level.
func (e *RuleEngine) RuleCount() int {
	currentParanoia := int(e.paranoia.Load())
	n := 0
	for _, r := range e.rules {
		if r.Paranoia <= currentParanoia {
			n++
		}
	}
	return n + len(e.customRules)
}

// SetParanoia updates the active paranoia level atomically, allowing live
// re-tuning from the dashboard without restart. Clamped to [1, 4].
func (e *RuleEngine) SetParanoia(level int) {
	if level < 1 {
		level = 1
	}
	if level > 4 {
		level = 4
	}
	e.paranoia.Store(int32(level))
}

// Paranoia returns the current paranoia level.
func (e *RuleEngine) Paranoia() int {
	return int(e.paranoia.Load())
}

// TotalRuleCount reports the full compiled rule set regardless of paranoia.
func (e *RuleEngine) TotalRuleCount() int {
	return len(e.rules) + len(e.customRules)
}

// rspec is a compact spec used to build the rule table before compilation.
type rspec struct {
	id       string
	cat      Category
	sev      Severity
	paranoia int
	desc     string
	pattern  string
}

// compileBuiltinRules builds the full detection corpus. Patterns are grouped
// by attack class and tagged with a paranoia level so operators can trade
// coverage for false-positive rate. All patterns are case-insensitive.
func compileBuiltinRules() []*Rule {
	specs := []rspec{
		// ---------------- SQL Injection ----------------
		{"SQLI-001", CatSQLi, SevCritical, 1, "SQL boolean-based tautology", `(?i)('|"|\s)(or|and)(\s+|\/\*.*?\*\/)('?\w+'?\s*(=|<>|like)\s*'?\w+'?|'\d+'?\s*=\s*'?\d+)`},
		{"SQLI-002", CatSQLi, SevCritical, 1, "SQL UNION SELECT", `(?i)union\s+(all\s+)?select`},
		{"SQLI-003", CatSQLi, SevHigh, 1, "SQL SELECT ... FROM", `(?i)select\s+.+?\s+from\s+`},
		{"SQLI-004", CatSQLi, SevHigh, 1, "SQL INSERT INTO", `(?i)insert\s+into\s+`},
		{"SQLI-005", CatSQLi, SevHigh, 1, "SQL UPDATE ... SET", `(?i)update\s+\w+\s+set\s+`},
		{"SQLI-006", CatSQLi, SevCritical, 1, "SQL DELETE FROM", `(?i)delete\s+from\s+`},
		{"SQLI-007", CatSQLi, SevCritical, 1, "SQL DROP TABLE/DATABASE", `(?i)drop\s+(table|database|schema)\s+`},
		{"SQLI-008", CatSQLi, SevCritical, 1, "SQL stacked exec/xp_cmdshell", `(?i)(exec(\s|\+)+(x|s)p_\w+|xp_cmdshell|sp_executesql)`},
		{"SQLI-009", CatSQLi, SevMedium, 2, "SQL inline comment terminator", `(?i)(--\s|#\s|\/\*!?)`},
		{"SQLI-010", CatSQLi, SevHigh, 1, "SQL time-based blind (WAITFOR/SLEEP/BENCHMARK)", `(?i)(waitfor\s+delay|sleep\s*\(\s*\d|benchmark\s*\(|pg_sleep\s*\()`},
		{"SQLI-011", CatSQLi, SevHigh, 2, "SQL info schema enumeration", `(?i)(information_schema|sysobjects|syscolumns|pg_catalog|mysql\.user)`},
		{"SQLI-012", CatSQLi, SevMedium, 2, "SQL hex/char obfuscation", `(?i)(0x[0-9a-f]{4,}|char\s*\(\s*\d+|concat\s*\()`},
		{"SQLI-013", CatSQLi, SevMedium, 3, "SQL conditional error extraction", `(?i)(extractvalue\s*\(|updatexml\s*\(|floor\s*\(\s*rand)`},
		{"SQLI-014", CatSQLi, SevMedium, 3, "SQL OR with always-true numeric", `(?i)\bor\b\s+\d+\s*=\s*\d+`},

		// ---------------- Cross-Site Scripting ----------------
		{"XSS-001", CatXSS, SevCritical, 1, "XSS script tag", `(?i)<\s*script[\s>/]`},
		{"XSS-002", CatXSS, SevHigh, 1, "XSS javascript: URI", `(?i)javascript\s*:`},
		{"XSS-003", CatXSS, SevHigh, 1, "XSS inline event handler", `(?i)\son\w+\s*=`},
		{"XSS-004", CatXSS, SevHigh, 1, "XSS iframe/object/embed", `(?i)<\s*(iframe|object|embed|applet)[\s>/]`},
		{"XSS-005", CatXSS, SevMedium, 1, "XSS img/svg with handler", `(?i)<\s*(img|svg|body|video|audio)[^>]*\son\w+`},
		{"XSS-006", CatXSS, SevHigh, 2, "XSS DOM sink access", `(?i)(document\.(cookie|write|location)|window\.(location|name))`},
		{"XSS-007", CatXSS, SevMedium, 2, "XSS eval/Function constructor", `(?i)(eval|settimeout|setinterval)\s*\(|new\s+function\s*\(`},
		{"XSS-008", CatXSS, SevMedium, 2, "XSS alert/prompt/confirm", `(?i)(alert|prompt|confirm)\s*(\(|` + "`" + `)`},
		{"XSS-009", CatXSS, SevMedium, 3, "XSS data URI with base64", `(?i)data:\s*text/html\s*(;\s*base64)?,`},
		{"XSS-010", CatXSS, SevMedium, 3, "XSS expression()/behavior CSS", `(?i)(expression\s*\(|-moz-binding|behavior\s*:)`},
		{"XSS-011", CatXSS, SevLow, 3, "XSS angle-bracket entity smuggling", `(?i)(&#x?0*(3c|60)|&lt;\s*script)`},

		// ---------------- Remote Code Execution / Command Injection ----------------
		{"RCE-001", CatRCE, SevCritical, 1, "Shell command chaining", "(?i)(;|\\||&&|\\|\\||`|\\$\\()\\s*(cat|ls|id|whoami|uname|pwd|nc|ncat|bash|sh|curl|wget|python|perl|ruby|php)\\b"},
		{"RCE-002", CatRCE, SevCritical, 1, "PHP code execution sinks", `(?i)\b(system|exec|passthru|shell_exec|popen|proc_open|assert|pcntl_exec)\s*\(`},
		{"RCE-003", CatRCE, SevHigh, 1, "Generic eval sink", `(?i)\beval\s*\(`},
		{"RCE-004", CatRCE, SevCritical, 1, "Windows command injection", `(?i)(cmd(\.exe)?\s*/c|powershell(\.exe)?\s|&\s*(dir|type|net|whoami))`},
		{"RCE-005", CatRCE, SevHigh, 2, "Reverse shell primitive", `(?i)(/dev/tcp/|bash\s+-i|nc\s+-e|mkfifo|/bin/sh\s+-i)`},
		{"RCE-006", CatRCE, SevHigh, 2, "Env/proc read via command", `(?i)(cat\s+/etc/(passwd|shadow)|/proc/self/environ)`},
		{"RCE-007", CatRCE, SevMedium, 3, "IFS/whitespace bypass", `(?i)\$\{ifs\}|\$ifs`},

		// ---------------- Local/Remote File Inclusion & Path Traversal ----------------
		{"LFI-001", CatLFI, SevCritical, 1, "Directory traversal sequence", `(?i)(\.\.[\\/]){1,}`},
		{"LFI-002", CatLFI, SevCritical, 1, "Sensitive unix file access", `(?i)/etc/(passwd|shadow|hosts|group|hostname)`},
		{"LFI-003", CatLFI, SevHigh, 1, "Proc/self and var/log access", `(?i)(/proc/self/|/var/log/|/proc/version)`},
		{"LFI-004", CatLFI, SevHigh, 1, "PHP wrapper stream", `(?i)php://(filter|input|data|expect)`},
		{"LFI-005", CatLFI, SevHigh, 1, "Dangerous URI schemes", `(?i)(file|data|expect|phar|zip|glob)://`},
		{"LFI-006", CatLFI, SevMedium, 2, "Encoded traversal (%2e/%2f/dot)", `(?i)(%2e%2e|%252e|\.%2e|%c0%ae)`},
		{"LFI-007", CatLFI, SevHigh, 2, "Windows sensitive path", `(?i)([a-z]:\\+(windows|winnt|boot\.ini)|\\+windows\\+system32)`},
		{"LFI-008", CatLFI, SevMedium, 3, "Null-byte path truncation", `(?i)(%00|\x00)`},

		// ---------------- Server-Side Request Forgery ----------------
		{"SSRF-001", CatSSRF, SevHigh, 1, "Loopback target", `(?i)(https?|gopher|dict|ftp)://(127\.0\.0\.1|localhost|0\.0\.0\.0|\[::1\])`},
		{"SSRF-002", CatSSRF, SevCritical, 1, "Cloud metadata endpoint", `(?i)(169\.254\.169\.254|metadata\.google\.internal|100\.100\.100\.200)`},
		{"SSRF-003", CatSSRF, SevHigh, 1, "RFC1918 private range target", `(?i)(https?|gopher|dict)://(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)`},
		{"SSRF-004", CatSSRF, SevHigh, 2, "Dangerous SSRF schemes", `(?i)(gopher|dict|ftp|ldap|tftp)://`},
		{"SSRF-005", CatSSRF, SevMedium, 3, "Decimal/octal/hex IP obfuscation", `(?i)://(0x[0-9a-f]+|0\d{3,}|\d{8,})`},
		{"SSRF-006", CatSSRF, SevMedium, 3, "IPv6-mapped or rare-notation host", `(?i)://\[?(::ffff:|0:0:0:0)`},

		// ---------------- XML External Entity ----------------
		{"XXE-001", CatXXE, SevCritical, 1, "DOCTYPE with ENTITY", `(?i)<!doctype[^>]*<!entity`},
		{"XXE-002", CatXXE, SevCritical, 1, "External entity SYSTEM/PUBLIC", `(?i)<!entity\s+\S+\s+(system|public)`},
		{"XXE-003", CatXXE, SevHigh, 2, "Parameter entity DTD", `(?i)<!entity\s+%`},

		// ---------------- Server-Side Template Injection ----------------
		{"SSTI-001", CatSSTI, SevHigh, 2, "Template braces with arithmetic", `(\{\{|\{%|\$\{|<%=)\s*[\d'"]*\s*[\*\+]\s*[\d'"]`},
		{"SSTI-002", CatSSTI, SevHigh, 2, "Python object traversal gadget", `(?i)(__class__|__mro__|__subclasses__|__globals__|__builtins__)`},
		{"SSTI-003", CatSSTI, SevHigh, 2, "Jinja/Twig config/self access", `(?i)(\{\{\s*(config|self|request|cycler|joiner)\b)`},
		{"SSTI-004", CatSSTI, SevMedium, 3, "Freemarker/OGNL exec gadget", `(?i)(freemarker\.template|@java\.lang\.runtime@|#\{|%\{)`},

		// ---------------- NoSQL Injection ----------------
		{"NOSQL-001", CatNoSQLi, SevHigh, 2, "Mongo operator injection", `(?i)(\$where|\$ne|\$gt|\$lt|\$regex|\$exists)\b`},
		{"NOSQL-002", CatNoSQLi, SevMedium, 3, "JS injection in Mongo $where", `(?i)\$where\s*:\s*['"].*[\(\)]`},

		// ---------------- Deserialization ----------------
		{"DESER-001", CatDeserialize, SevCritical, 2, "Java serialized stream marker", `(?i)(rO0AB|aced0005|java\.lang\.runtime)`},
		{"DESER-002", CatDeserialize, SevHigh, 2, "PHP object injection", `(?i)(O:\d+:"|a:\d+:\{)`},
		{"DESER-003", CatDeserialize, SevHigh, 3, "Python pickle/marshal marker", `(?i)(c__builtin__|cposix\s|\bcos\ssystem)`},

		// ---------------- Prototype Pollution ----------------
		{"PROTO-001", CatProtoPollution, SevHigh, 2, "__proto__/constructor pollution", `(?i)(__proto__|constructor\s*\[\s*['"]prototype|prototype\s*\[)`},

		// ---------------- CRLF / Header & Response Splitting ----------------
		{"CRLF-001", CatCRLF, SevHigh, 2, "CRLF injection sequence", `(?i)(%0d%0a|%0a|%0d|\r\n)(set-cookie|location|content-length|content-type)\s*:`},
		{"CRLF-002", CatCRLF, SevMedium, 3, "Raw CRLF in parameter", `(\r\n|%0d%0a)`},

		// ---------------- Scanner / Recon Signatures ----------------
		{"SCAN-001", CatScanner, SevHigh, 1, "Known scanner/attack-tool UA signature", `(?i)(sqlmap|nikto|nmap|masscan|dirbuster|gobuster|wpscan|acunetix|nessus|openvas|nuclei|feroxbuster|whatweb)`},
		{"SCAN-002", CatScanner, SevHigh, 1, "Known webshell filename", `(?i)(c99|r57|c100|wso|b374k|webshell|shell|cmd|backdoor)\.(php|asp|aspx|jsp)`},
		{"SCAN-003", CatScanner, SevMedium, 2, "Common admin/probe endpoint", `(?i)(/wp-admin|/phpmyadmin|/xmlrpc\.php|/administrator/)`},
	}

	rules := make([]*Rule, 0, len(specs))
	for _, s := range specs {
		re, err := regexp.Compile(s.pattern)
		if err != nil {
			// A malformed built-in pattern is a programming error; skipping it
			// keeps the engine running rather than taking the process down.
			continue
		}
		rules = append(rules, &Rule{
			ID:       s.id,
			Category: s.cat,
			Severity: s.sev,
			Paranoia: s.paranoia,
			Desc:     s.desc,
			re:       re,
		})
	}
	return rules
}

// itoa is a tiny dependency-free integer-to-string helper for rule IDs.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// normalizeForScan lowercases and trims a payload snapshot for cheap prefiltering.
// The rule regexes are already case-insensitive, so this is only used by callers
// that want a canonical form for logging.
func normalizeForScan(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
