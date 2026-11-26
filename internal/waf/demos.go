package waf

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

const demoPage = `<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>body{font:16px system-ui;max-width:760px;margin:3rem auto;padding:0 1rem;background:#101722;color:#e8eef7}
a{color:#7daeff}form{margin:1rem 0}input,button{padding:.5rem}li{margin:.5rem 0}</style></head>
<body><h1>{{.Title}}</h1><p>{{.Description}}</p><ul>{{range .Links}}<li><a href="{{.URL}}">{{.Label}}</a></li>{{end}}</ul></body></html>`

const demoFormPage = `<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>body{font:16px system-ui;max-width:760px;margin:3rem auto;padding:0 1rem;background:#101722;color:#e8eef7}
a{color:#7daeff}form{margin:1rem 0}input,select,button{padding:.5rem}</style></head>
<body><h1>{{.Title}}</h1><p>{{.Description}}</p>{{.Form}}<p><a href="/">Back to application</a></p></body></html>`

type demoLink struct {
	URL   string
	Label string
}

func renderDemoPage(w http.ResponseWriter, title, description string, links []demoLink) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = template.Must(template.New("demo").Parse(demoPage)).Execute(w, map[string]interface{}{
		"Title": title, "Description": description, "Links": links,
	})
}

func renderDemoFormPage(w http.ResponseWriter, title, description, form string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = template.Must(template.New("demo-form").Parse(demoFormPage)).Execute(w, map[string]interface{}{
		"Title": title, "Description": description, "Form": template.HTML(form),
	})
}

// NewDemoBackend returns an in-process synthetic application. The comparison
// API is the only path that directly invokes this intentionally weak handler;
// no backend listener is opened.
func NewDemoBackend(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch kind {
		case "online-store":
			serveOnlineStoreDemo(w, r)
		case "booking-system":
			serveBookingDemo(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// InstallDemoApplications registers the two built-in, synthetic applications
// without replacing any existing operator configuration.
func (m *ApplicationManager) InstallDemoApplications() error {
	existing := make(map[string]bool)
	for _, app := range m.Applications() {
		existing[app.ID] = true
	}
	demos := []ProtectedApplication{
		{
			ID: "online-store", Name: "Online Store Lab", Hostname: "store.local",
			BackendURL: "internal://demo/online-store", DemoKind: "online-store", Enabled: true,
			Policy: ApplicationPolicy{Profile: "balanced"},
		},
		{
			ID: "booking-system", Name: "Booking System Lab", Hostname: "booking.local",
			BackendURL: "internal://demo/booking-system", DemoKind: "booking-system", Enabled: true,
			Policy: ApplicationPolicy{Profile: "balanced"},
		},
	}
	for _, app := range demos {
		if existing[app.ID] {
			continue
		}
		if err := m.UpsertApplication(app); err != nil {
			return fmt.Errorf("install %s demo: %w", app.ID, err)
		}
	}
	return nil
}

// InstallShowcaseApplications adds the graduation platform and database-backed
// attack lab without replacing existing operator policy settings.
func (m *ApplicationManager) InstallShowcaseApplications() error {
	existing := make(map[string]ProtectedApplication)
	for _, app := range m.Applications() {
		existing[app.ID] = app
	}
	applications := []ProtectedApplication{
		{
			ID: "original-site", Name: "Graduation Project Platform", Hostname: "graduation.localhost",
			BackendURL: "internal://showcase/graduation-platform", InternalKind: "graduation-platform", Enabled: true,
			Policy: ApplicationPolicy{Profile: "balanced"},
		},
		{
			ID: "cyber-attack-lab", Name: "Cybersecurity Attack Lab", Hostname: "cyberlab.localhost",
			BackendURL: "internal://showcase/cyber-attack-lab", InternalKind: "cyber-attack-lab", Enabled: true,
			Policy: ApplicationPolicy{Profile: "balanced"},
		},
	}
	for _, app := range applications {
		if previous, ok := existing[app.ID]; ok {
			if app.ID == "original-site" {
				previous.Name = app.Name
				previous.Hostname = app.Hostname
				previous.BackendURL = app.BackendURL
				previous.InternalKind = app.InternalKind
				if err := m.UpsertApplication(previous); err != nil {
					return fmt.Errorf("update %s: %w", app.ID, err)
				}
			}
			continue
		}
		if err := m.UpsertApplication(app); err != nil {
			return fmt.Errorf("install %s: %w", app.ID, err)
		}
	}
	return nil
}

func serveOnlineStoreDemo(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		renderDemoPage(w, "NullGate Online Store Lab", "Synthetic products only. Search, product details, login, and cart requests are handled in memory.", []demoLink{
			{"/search", "Search products"},
			{"/products?id=101", "Product details"},
			{"/login", "Login"},
			{"/cart", "Cart"},
		})
	case "/search":
		query := r.URL.Query().Get("q")
		if query == "" {
			renderDemoFormPage(w, "Search products", "Search the synthetic catalog.", `<form action="/search" method="get"><label>Search <input name="q" value="books"></label><button>Search</button></form>`)
			return
		}
		results := []string{"Notebook", "Desk lamp", "Travel mug"}
		if looksLikeDemoSQLInjection(query) {
			results = append(results, "All synthetic records returned by the lab query simulation")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// This deliberately mirrors an unsafe reflected search result. It is
		// reachable only through the protected route or the localhost admin
		// comparison API; there is no public backend listener.
		fmt.Fprintf(w, "<!doctype html><title>Search</title><h1>Search results</h1><p>Query: %s</p><ul>", query)
		for _, result := range results {
			fmt.Fprintf(w, "<li>%s</li>", result)
		}
		io.WriteString(w, "</ul>")
	case "/products":
		id := r.URL.Query().Get("id")
		name := "Synthetic product"
		if id == "101" {
			name = "Notebook"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		escapedID := template.HTMLEscapeString(id)
		fmt.Fprintf(w, "<!doctype html><title>Product</title><h1>%s</h1><p>Product id=%s (synthetic)</p><form action=\"/cart\" method=\"post\"><input type=\"hidden\" name=\"product\" value=\"%s\"><button>Add to cart</button></form><a href=\"/cart\">View cart</a>", name, escapedID, escapedID)
	case "/login":
		if r.Method == http.MethodGet {
			renderDemoFormPage(w, "Store login", "Synthetic login only; credentials are not stored.", `<form action="/login" method="post"><label>Username <input name="username" value="demo-user"></label><label>Password <input type="password" name="password"></label><button>Sign in</button></form>`)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "store_user", Value: "demo-user", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Error(w, "Synthetic login accepted for demonstration only", http.StatusOK)
	case "/cart":
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid cart form", http.StatusBadRequest)
				return
			}
			product := r.Form.Get("product")
			if product != "101" {
				http.Error(w, "unknown synthetic product", http.StatusBadRequest)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "store_cart", Value: product, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		cart, _ := r.Cookie("store_cart")
		if cart == nil {
			fmt.Fprint(w, "Synthetic cart: 0 items")
		} else {
			fmt.Fprint(w, "Synthetic cart: 1 Notebook")
		}
	default:
		http.NotFound(w, r)
	}
}

func serveBookingDemo(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		renderDemoPage(w, "NullGate Booking Lab", "Synthetic reservations only. Booking, lookup, login, management, and authorization flows run in memory.", []demoLink{
			{"/lookup", "Look up reservation"},
			{"/booking", "Create booking"},
			{"/login", "Login"},
			{"/management", "Management (requires lab-admin cookie)"},
			{"/limited", "Rate-limit demonstration"},
		})
	case "/lookup":
		id := r.URL.Query().Get("id")
		if id == "" {
			renderDemoFormPage(w, "Look up a reservation", "Use a synthetic reservation ID such as R-100.", `<form action="/lookup" method="get"><label>Reservation ID <input name="id" value="R-100"></label><button>Look up</button></form>`)
			return
		}
		if looksLikeDemoSQLInjection(id) {
			fmt.Fprint(w, "Synthetic reservations: R-100 (Demo Guest), R-200 (Demo Guest)")
			return
		}
		if id == "R-100" || id == "R-200" {
			fmt.Fprintf(w, "Synthetic reservation %s: Demo Guest", template.HTMLEscapeString(id))
			return
		}
		http.NotFound(w, r)
	case "/booking":
		if r.Method == http.MethodGet {
			renderDemoFormPage(w, "Create a booking", "Submissions create synthetic reservations only.", `<form action="/booking" method="post"><label>Guest <input name="guest" value="Demo Guest"></label><label>Date <input name="date" type="date"></label><button>Create booking</button></form>`)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Use the synthetic booking form", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid booking form", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "<!doctype html><title>Booking</title><p>Booking created for %s (synthetic)</p>", r.Form.Get("guest"))
	case "/login":
		if r.Method == http.MethodGet {
			renderDemoFormPage(w, "Booking lab login", "Choose a synthetic role to demonstrate authorization; no credentials are stored.", `<form action="/login" method="post"><label>Role <select name="role"><option value="guest">Guest</option><option value="admin">Lab administrator</option></select></label><button>Sign in</button></form>`)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Use the synthetic login form", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid login form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("role") == "admin" {
			http.SetCookie(w, &http.Cookie{Name: "lab_admin", Value: "demo-admin", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			fmt.Fprint(w, "Synthetic lab administrator login accepted")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "lab_user", Value: "demo-guest", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		fmt.Fprint(w, "Synthetic guest login accepted")
	case "/management":
		cookie, err := r.Cookie("lab_admin")
		if err != nil || cookie.Value != "demo-admin" {
			http.Error(w, "synthetic administrator authorization required", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "Synthetic management area")
	case "/limited":
		fmt.Fprint(w, "Synthetic rate-limit demo. Refresh repeatedly to observe the configured application rate limit; no real bookings or accounts are affected.")
	default:
		http.NotFound(w, r)
	}
}

func looksLikeDemoSQLInjection(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "' or ") || strings.Contains(lower, "\" or ") || strings.Contains(lower, " union select")
}

type DemoResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
}

type DemoComparison struct {
	ApplicationID string       `json:"application_id"`
	Protected     DemoResponse `json:"protected"`
	Unprotected   DemoResponse `json:"unprotected"`
	Decision      WAFDecision  `json:"decision"`
}

// CompareDemo executes the same bounded request through the registered WAF
// engine and directly against an isolated in-process lab handler, returning
// observed responses rather than predicted or fabricated outcomes.
func (m *ApplicationManager) CompareDemo(id, method, requestTarget, body, contentType string) (DemoComparison, error) {
	if len(body) > 16<<10 || len(requestTarget) > 8192 {
		return DemoComparison{}, fmt.Errorf("comparison request exceeds its size limit")
	}
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
		return DemoComparison{}, fmt.Errorf("unsupported comparison method")
	}
	target, err := url.ParseRequestURI(requestTarget)
	if err != nil || target.IsAbs() || target.Host != "" || !strings.HasPrefix(target.Path, "/") || strings.HasPrefix(target.Path, "//") {
		return DemoComparison{}, fmt.Errorf("request target must be an origin-form path")
	}
	snapshot := m.snapshot.Load()
	if snapshot == nil {
		return DemoComparison{}, fmt.Errorf("application manager is unavailable")
	}
	app, ok := snapshot.byID[id]
	if !ok || app.engine == nil {
		return DemoComparison{}, fmt.Errorf("application not found or disabled")
	}
	unprotectedBackend := app.demo
	if unprotectedBackend == nil && app.config.InternalKind != "" {
		unprotectedBackend = m.internal[app.config.InternalKind]
	}
	if !app.comparable || unprotectedBackend == nil {
		return DemoComparison{}, fmt.Errorf("unprotected comparisons are available only for isolated lab applications")
	}
	makeRequest := func() *http.Request {
		req := httptest.NewRequest(method, requestTarget, bytes.NewBufferString(body))
		req.Host = app.config.Hostname
		req.RemoteAddr = "198.51.100.245:40123"
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/json")
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		return req
	}
	decision := &WAFDecision{}
	protectedRequest := makeRequest()
	if app.engine.config.CSRF.Enabled && app.engine.csrf.Required(protectedRequest) {
		token := app.engine.csrf.GenerateToken()
		if token == "" {
			return DemoComparison{}, fmt.Errorf("could not create comparison CSRF token")
		}
		protectedRequest.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	}
	protectedRequest = protectedRequest.WithContext(context.WithValue(protectedRequest.Context(), wafDecisionContextKey{}, decision))
	protectedRecorder := httptest.NewRecorder()
	app.engine.ServeHTTP(protectedRecorder, protectedRequest)

	unprotectedRequest := makeRequest()
	unprotectedRecorder := httptest.NewRecorder()
	unprotectedBackend.ServeHTTP(unprotectedRecorder, unprotectedRequest)

	return DemoComparison{
		ApplicationID: id,
		Protected:     demoResponseFromRecorder(protectedRecorder),
		Unprotected:   demoResponseFromRecorder(unprotectedRecorder),
		Decision:      *decision,
	}, nil
}

func demoResponseFromRecorder(recorder *httptest.ResponseRecorder) DemoResponse {
	const maxOutput = 64 << 10
	body := recorder.Body.Bytes()
	if len(body) > maxOutput {
		body = body[:maxOutput]
	}
	headers := make(map[string][]string)
	for key, values := range recorder.Header() {
		headers[key] = append([]string(nil), values...)
	}
	return DemoResponse{
		StatusCode: recorder.Code,
		Headers:    headers,
		Body:       string(body),
	}
}
