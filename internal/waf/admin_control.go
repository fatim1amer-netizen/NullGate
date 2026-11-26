package waf

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// Authentication: session-based login/logout
// ---------------------------------------------------------------------------

// handleLoginPage serves the standalone login form. Already-authenticated
// browsers are bounced straight to the dashboard.
func (a *AdminServer) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if a.cfg.PasswordHash == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && a.sessions.Valid(c.Value) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(adminLoginHTML))
}

// handleLogin verifies credentials and, on success, issues a session cookie.
// Credentials are checked in constant time via VerifyPassword.
func (a *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	userOK := subtleEqual(req.Username, a.cfg.Username)
	passOK := VerifyPassword(a.cfg.PasswordHash, req.Password)
	if !userOK || !passOK {
		a.logger.Warn("admin login failed", map[string]interface{}{
			"username": req.Username, "client_ip": clientHost(r),
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := a.sessions.Create()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session creation failed"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   a.cfgRoot != nil && (a.cfgRoot.Server.EnableTLS || a.cfgRoot.Server.AutoTLS),
		MaxAge:   int((time.Duration(a.cfg.SessionTTLMin) * time.Minute).Seconds()),
	})
	a.logger.Info("admin login", map[string]interface{}{"client_ip": clientHost(r)})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleLogout destroys the current session and clears the cookie.
func (a *AdminServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.sessions.Destroy(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Live control: block/unblock IP, set paranoia, list blocklist
// ---------------------------------------------------------------------------

// guardWrite rejects mutating calls when the admin surface is read-only or the
// method is not POST. Returns false when the request has been handled (denied).
func (a *AdminServer) guardWrite(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return false
	}
	if a.cfg.ReadOnly {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin is read-only; set admin.read_only=false to allow control actions"})
		return false
	}
	if a.cfg.PasswordHash == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin password authentication is required for control actions"})
		return false
	}
	return true
}

func (a *AdminServer) handleBlockIP(w http.ResponseWriter, r *http.Request) {
	if !a.guardWrite(w, r) {
		return
	}
	var req struct {
		IP       string `json:"ip"`
		Duration int    `json:"duration_sec"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if net.ParseIP(req.IP) == nil && !isCIDR(req.IP) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not a valid IP or CIDR"})
		return
	}
	dur := time.Duration(req.Duration) * time.Second
	if dur <= 0 {
		dur = time.Hour
	}
	a.blocklist.Add(req.IP, dur)
	a.logger.Warn("admin manual block", map[string]interface{}{
		"ip": req.IP, "duration_sec": int(dur.Seconds()), "by": clientHost(r),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "blocked", "ip": req.IP, "duration_sec": int(dur.Seconds())})
}

func (a *AdminServer) handleUnblockIP(w http.ResponseWriter, r *http.Request) {
	if !a.guardWrite(w, r) {
		return
	}
	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	a.blocklist.Remove(req.IP)
	a.logger.Info("admin manual unblock", map[string]interface{}{"ip": req.IP, "by": clientHost(r)})
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "unblocked", "ip": req.IP})
}

func (a *AdminServer) handleSetParanoia(w http.ResponseWriter, r *http.Request) {
	if !a.guardWrite(w, r) {
		return
	}
	var req struct {
		Level int `json:"level"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Level < 1 || req.Level > 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paranoia level must be between 1 and 4"})
		return
	}
	if a.rules == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rule engine unavailable"})
		return
	}
	a.rules.SetParanoia(req.Level)
	a.ruleCount = a.rules.RuleCount()
	if a.cfgRoot != nil {
		a.cfgRoot.Rules.ParanoiaLevel = req.Level
	}
	a.logger.Info("admin set paranoia", map[string]interface{}{
		"level": req.Level, "active_rules": a.ruleCount, "by": clientHost(r),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok", "level": req.Level, "rules_active": a.ruleCount, "rules_total": a.totalRules,
	})
}

// handleBlocklist returns the current blocklist entries with expiry timestamps.
func (a *AdminServer) handleBlocklist(w http.ResponseWriter, r *http.Request) {
	entries := a.blocklist.Entries()
	now := time.Now()
	list := make([]map[string]interface{}, 0, len(entries))
	for ip, exp := range entries {
		list = append(list, map[string]interface{}{
			"ip":            ip,
			"expires_at":    exp.UTC().Format(time.RFC3339),
			"remaining_sec": int(exp.Sub(now).Seconds()),
			"expired":       now.After(exp),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": list, "count": len(list)})
}

// subtleEqual is a constant-time string comparison used for the username.
func subtleEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// clientHost extracts the remote IP (without port) for logging.
func clientHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isCIDR reports whether s parses as a CIDR block.
func isCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}
