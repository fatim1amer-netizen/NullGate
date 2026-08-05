package waf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// scanProbe is one attack payload fired during a protection scan, tagged with
// the attack class it represents so the report can break results down by type.
type scanProbe struct {
	Category string
	Payload  string
}

// scanCorpus is the battery of representative attacks used to measure how well
// the rule engine detects each major class. It mirrors the self-test corpus but
// is trimmed to one strong exemplar per category for a fast, readable report.
var scanCorpus = []scanProbe{
	{"sqli", "' OR '1'='1' --"},
	{"sqli", "1 UNION SELECT username,password FROM users"},
	{"xss", "<script>document.cookie</script>"},
	{"xss", "<img src=x onerror=alert(1)>"},
	{"rce", ";cat /etc/passwd"},
	{"rce", "$(whoami)"},
	{"lfi", "../../../../etc/passwd"},
	{"lfi", "php://filter/convert.base64-encode/resource=index"},
	{"ssrf", "http://169.254.169.254/latest/meta-data/"},
	{"ssrf", "http://127.0.0.1:8080/admin"},
	{"xxe", `<!DOCTYPE foo [<!ENTITY x SYSTEM "file:///etc/passwd">]>`},
	{"ssti", "{{7*7}}"},
	{"nosqli", `{"user":{"$ne":null}}`},
	{"deserialize", "rO0ABXNy"},
	{"proto_pollution", `{"__proto__":{"admin":true}}`},
	{"crlf", "%0d%0aSet-Cookie: injected=1"},
	{"scanner", "sqlmap/1.7"},
}

// ScanResultReport is the outcome of a protection scan.
type ScanResultReport struct {
	Target        string               `json:"target"`
	Mode          string               `json:"mode"` // "engine" or "live"
	Verified      bool                 `json:"ownership_verified"`
	Total         int                  `json:"total_probes"`
	Detected      int                  `json:"detected"`
	Missed        int                  `json:"missed"`
	DetectionRate float64              `json:"detection_rate"`
	ByCategory    map[string]scanTally `json:"by_category"`
	Findings      []scanFinding        `json:"findings"`
	DurationMS    int64                `json:"duration_ms"`
}

type scanTally struct {
	Total    int `json:"total"`
	Detected int `json:"detected"`
}

type scanFinding struct {
	Category string `json:"category"`
	Payload  string `json:"payload"`
	Detected bool   `json:"detected"`
	Status   int    `json:"status,omitempty"` // live mode: backend/WAF HTTP status
}

// handleScan runs a protection scan. Two modes:
//   - engine (default): fires the corpus through the in-process rule engine.
//     No network traffic leaves the host; it measures detection coverage.
//   - live: additionally probes a caller-supplied target URL, but ONLY after
//     ownership is verified. Ownership is proven by publishing the token
//     returned by ?action=token either as a DNS TXT record on the host or as a
//     <meta name="nullgate-verify" content="..."> tag on the site root.
//
// We refuse to fire attack traffic at a host the caller has not proven they
// control: the scanner is a "test my own defenses" tool, not an attack proxy.
func (a *AdminServer) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Target string `json:"target"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	start := time.Now()
	engine := a.rules
	if engine == nil {
		engine = NewRuleEngine(2, nil)
	}

	// Engine mode: pure in-process detection measurement, no target needed.
	if req.Mode != "live" {
		rep := runEngineScan(engine, req.Target)
		rep.DurationMS = time.Since(start).Milliseconds()
		writeJSON(w, http.StatusOK, rep)
		return
	}

	// Live mode: validate target and require proven ownership.
	target, err := validateTarget(req.Target)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !verifyOwnership(target) {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"error":         "ownership not verified for this target",
			"how_to_verify": "publish the token from POST /api/admin/scan?action=token as a DNS TXT record, or a <meta name=\"nullgate-verify\"> tag on the site root",
			"verify_token":  ownershipToken(target),
		})
		return
	}
	rep := runLiveScan(engine, target)
	rep.DurationMS = time.Since(start).Milliseconds()
	writeJSON(w, http.StatusOK, rep)
}

// runEngineScan measures detection coverage by scanning each corpus payload
// through the rule engine directly.
func runEngineScan(engine *RuleEngine, target string) ScanResultReport {
	rep := ScanResultReport{
		Target:     target,
		Mode:       "engine",
		Verified:   true, // engine mode touches no external host
		ByCategory: map[string]scanTally{},
	}
	for _, p := range scanCorpus {
		res := engine.Scan(p.Payload)
		detected := len(res.Matches) > 0
		tally := rep.ByCategory[p.Category]
		tally.Total++
		if detected {
			tally.Detected++
			rep.Detected++
		} else {
			rep.Missed++
		}
		rep.ByCategory[p.Category] = tally
		rep.Findings = append(rep.Findings, scanFinding{
			Category: p.Category, Payload: p.Payload, Detected: detected,
		})
		rep.Total++
	}
	if rep.Total > 0 {
		rep.DetectionRate = float64(rep.Detected) / float64(rep.Total)
	}
	return rep
}

// runLiveScan fires each corpus payload at the verified target as a query
// parameter and records whether the WAF/backend blocked it (non-2xx/3xx).
func runLiveScan(engine *RuleEngine, target string) ScanResultReport {
	rep := ScanResultReport{
		Target:     target,
		Mode:       "live",
		Verified:   true,
		ByCategory: map[string]scanTally{},
	}
	client := &http.Client{Timeout: 8 * time.Second}
	for _, p := range scanCorpus {
		u := target
		if strings.Contains(u, "?") {
			u += "&q=" + url.QueryEscape(p.Payload)
		} else {
			u += "?q=" + url.QueryEscape(p.Payload)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("User-Agent", "NullGate-Scanner/2.0")
		status := 0
		detected := false
		if resp, err := client.Do(req); err == nil {
			status = resp.StatusCode
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			// A protective response blocks the attack: 403/406/429/400.
			detected = status == http.StatusForbidden || status == http.StatusNotAcceptable ||
				status == http.StatusTooManyRequests || status == http.StatusBadRequest
		}
		cancel()

		tally := rep.ByCategory[p.Category]
		tally.Total++
		if detected {
			tally.Detected++
			rep.Detected++
		} else {
			rep.Missed++
		}
		rep.ByCategory[p.Category] = tally
		rep.Findings = append(rep.Findings, scanFinding{
			Category: p.Category, Payload: p.Payload, Detected: detected, Status: status,
		})
		rep.Total++
	}
	if rep.Total > 0 {
		rep.DetectionRate = float64(rep.Detected) / float64(rep.Total)
	}
	return rep
}

// validateTarget parses and constrains a scan target: http/https only, a real
// host, and never a bare IP in a private/loopback range (which would let the
// scanner be pointed at internal infrastructure).
func validateTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("target URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid target URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("only http and https targets are allowed")
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("target URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return "", fmt.Errorf("scanning private, loopback, or link-local addresses is not allowed")
		}
	}
	return u.String(), nil
}

// ownershipToken derives a deterministic per-target verification token. The
// operator publishes this value to prove control of the host before a live
// scan is permitted.
func ownershipToken(target string) string {
	u, err := url.Parse(target)
	host := target
	if err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	sum := sha256.Sum256([]byte("nullgate-ownership:" + strings.ToLower(host)))
	return "nullgate-verify=" + hex.EncodeToString(sum[:16])
}

// verifyOwnership checks whether the expected token is published either as a
// DNS TXT record on the host or as a <meta name="nullgate-verify"> tag served
// at the site root.
func verifyOwnership(target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := u.Hostname()
	want := ownershipToken(target)
	wantVal := strings.TrimPrefix(want, "nullgate-verify=")

	// 1. DNS TXT record on the host.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if txts, err := net.DefaultResolver.LookupTXT(ctx, host); err == nil {
		for _, t := range txts {
			if strings.Contains(t, wantVal) {
				return true
			}
		}
	}

	// 2. Meta tag on the site root.
	client := &http.Client{Timeout: 6 * time.Second}
	rootReq, _ := http.NewRequest(http.MethodGet, u.Scheme+"://"+u.Host+"/", nil)
	if resp, err := client.Do(rootReq); err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
		if strings.Contains(string(body), wantVal) {
			return true
		}
	}
	return false
}
