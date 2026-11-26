package showcase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed assets/original.html assets/lab.html assets/lab.css assets/lab.js assets/f1.html assets/graduation
var pages embed.FS

var emailPattern = regexp.MustCompile(`^[^@\s]{1,128}@[^@\s]{1,128}\.[^@\s]{2,63}$`)

type App struct {
	db        *sql.DB
	mux       *http.ServeMux
	dummyHash []byte
}

type user struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func Open(databasePath string) (*App, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, fmt.Errorf("showcase database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0700); err != nil {
		return nil, fmt.Errorf("create showcase data directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open showcase database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000; PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL UNIQUE COLLATE NOCASE,
			name TEXT NOT NULL,
			password_hash BLOB NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL,
			price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
			category TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS bookings (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			destination TEXT NOT NULL,
			travel_date TEXT NOT NULL,
			guests INTEGER NOT NULL CHECK (guests BETWEEN 1 AND 8),
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS reviews (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id),
			product_id INTEGER NOT NULL REFERENCES products(id),
			rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
			body TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(user_id, product_id)
		);
		CREATE TABLE IF NOT EXISTS lab_events (
			id INTEGER PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			action TEXT NOT NULL,
			details TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize showcase database: %w", err)
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("nullgate-local-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize showcase authentication: %w", err)
	}
	app := &App{db: db, mux: http.NewServeMux(), dummyHash: dummyHash}
	if err := app.seed(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	app.routes()
	if err := os.Chmod(databasePath, 0600); err != nil && !os.IsNotExist(err) {
		_ = db.Close()
		return nil, fmt.Errorf("secure showcase database permissions: %w", err)
	}
	return app, nil
}

func (a *App) Close() error { return a.db.Close() }

func (a *App) Handler() http.Handler { return a.mux }

func (a *App) routes() {
	a.mux.HandleFunc("/", a.page)
	a.mux.HandleFunc("/assets/lab.css", a.asset)
	a.mux.HandleFunc("/assets/lab.js", a.asset)
	a.mux.HandleFunc("/api/products", a.products)
	a.mux.HandleFunc("/api/register", a.register)
	a.mux.HandleFunc("/api/login", a.login)
	a.mux.HandleFunc("/api/logout", a.logout)
	a.mux.HandleFunc("/api/me", a.me)
	a.mux.HandleFunc("/api/bookings", a.bookings)
	a.mux.HandleFunc("/api/reviews", a.reviews)
	a.mux.HandleFunc("/api/lab/events", a.labEvents)
}

func (a *App) asset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, http.MethodGet+", "+http.MethodHead)
		return
	}
	contentType := map[string]string{
		"/assets/lab.css": "text/css; charset=utf-8",
		"/assets/lab.js":  "text/javascript; charset=utf-8",
	}[r.URL.Path]
	if contentType == "" {
		http.NotFound(w, r)
		return
	}
	data, err := pages.ReadFile(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		http.Error(w, "Showcase asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func (a *App) seed(ctx context.Context) error {
	products := []struct {
		name, description string
		price             int
		category          string
	}{
		{"Secure Coding Field Guide", "A practical guide to threat modeling and defensive engineering.", 2499, "Books"},
		{"Network Analysis Kit", "A local training kit for understanding network telemetry.", 8999, "Tools"},
		{"Privacy First Hoodie", "Comfortable apparel for the security-minded.", 4599, "Apparel"},
	}
	for _, product := range products {
		if _, err := a.db.ExecContext(ctx, `INSERT INTO products(name, description, price_cents, category)
			VALUES(?, ?, ?, ?) ON CONFLICT(name) DO NOTHING`, product.name, product.description, product.price, product.category); err != nil {
			return fmt.Errorf("seed showcase products: %w", err)
		}
	}
	return nil
}

func (a *App) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := pages.ReadFile("assets/lab.html")
	if err != nil {
		http.Error(w, "Showcase unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func (a *App) products(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 100 {
		writeError(w, http.StatusBadRequest, "Search is limited to 100 characters.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id, name, description, price_cents, category
		FROM products WHERE name LIKE ? OR description LIKE ? OR category LIKE ? ORDER BY id LIMIT 25`,
		"%"+query+"%", "%"+query+"%", "%"+query+"%")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load products.")
		return
	}
	defer rows.Close()
	type product struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceCents  int    `json:"price_cents"`
		Category    string `json:"category"`
	}
	results := make([]product, 0)
	for rows.Next() {
		var item product
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.PriceCents, &item.Category); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load products.")
			return
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load products.")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var input struct{ Email, Name, Password string }
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	if !emailPattern.MatchString(input.Email) || len(input.Name) < 2 || len(input.Name) > 80 || len(input.Password) < 12 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "Provide a valid email, a 2-80 character name, and a 12-72 character password.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	result, err := a.db.ExecContext(r.Context(), `INSERT INTO users(email, name, password_hash, created_at) VALUES(?, ?, ?, ?)`,
		input.Email, input.Name, hash, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "An account with that email already exists.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	account := user{ID: id, Email: input.Email, Name: input.Name}
	if err := a.createSession(w, r, account.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not start account session.")
		return
	}
	a.recordEvent(r.Context(), &account.ID, "account_created", "New local lab account")
	writeJSON(w, http.StatusCreated, account)
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var input struct{ Email, Password string }
	if !decodeJSON(w, r, &input) {
		return
	}
	var account user
	var hash []byte
	err := a.db.QueryRowContext(r.Context(), `SELECT id, email, name, password_hash FROM users WHERE email = ?`,
		strings.TrimSpace(strings.ToLower(input.Email))).Scan(&account.ID, &account.Email, &account.Name, &hash)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "Could not sign in.")
		return
	}
	passwordHash := hash
	if err == sql.ErrNoRows {
		passwordHash = a.dummyHash
	}
	passwordErr := bcrypt.CompareHashAndPassword(passwordHash, []byte(input.Password))
	if err != nil || passwordErr != nil {
		a.recordEvent(r.Context(), nil, "login_failed", "Invalid credentials")
		writeError(w, http.StatusUnauthorized, "Email or password is incorrect.")
		return
	}
	if err := a.createSession(w, r, account.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not start account session.")
		return
	}
	a.recordEvent(r.Context(), &account.ID, "login_succeeded", "Account session created")
	writeJSON(w, http.StatusOK, account)
}

func (a *App) createSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes[:])
	digest := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(12 * time.Hour)
	if _, err := a.db.ExecContext(r.Context(), `INSERT INTO sessions(token_hash, user_id, expires_at) VALUES(?, ?, ?)`,
		hex.EncodeToString(digest[:]), userID, expires.Format(time.RFC3339)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: "ng_lab_session", Value: token, Path: "/", Expires: expires,
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (a *App) currentUser(r *http.Request) (user, error) {
	cookie, err := r.Cookie("ng_lab_session")
	if err != nil || len(cookie.Value) != 64 {
		return user{}, errors.New("not authenticated")
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	var account user
	err = a.db.QueryRowContext(r.Context(), `SELECT u.id, u.email, u.name FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, hex.EncodeToString(digest[:]), time.Now().UTC().Format(time.RFC3339)).
		Scan(&account.ID, &account.Email, &account.Name)
	return account, err
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if cookie, err := r.Cookie("ng_lab_session"); err == nil && len(cookie.Value) == 64 {
		digest := sha256.Sum256([]byte(cookie.Value))
		if _, err := a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, hex.EncodeToString(digest[:])); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not end session.")
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "ng_lab_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	account, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Sign in to continue.")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (a *App) bookings(w http.ResponseWriter, r *http.Request) {
	account, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Sign in to continue.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.QueryContext(r.Context(), `SELECT id, destination, travel_date, guests, created_at FROM bookings WHERE user_id = ? ORDER BY id DESC LIMIT 50`, account.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load bookings.")
			return
		}
		defer rows.Close()
		type booking struct {
			ID          int64  `json:"id"`
			Destination string `json:"destination"`
			TravelDate  string `json:"travel_date"`
			Guests      int    `json:"guests"`
			CreatedAt   string `json:"created_at"`
		}
		result := make([]booking, 0)
		for rows.Next() {
			var item booking
			if err := rows.Scan(&item.ID, &item.Destination, &item.TravelDate, &item.Guests, &item.CreatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not load bookings.")
				return
			}
			result = append(result, item)
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPost:
		var input struct {
			Destination string
			TravelDate  string
			Guests      int
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		input.Destination = strings.TrimSpace(input.Destination)
		date, err := time.Parse("2006-01-02", input.TravelDate)
		if len(input.Destination) < 2 || len(input.Destination) > 100 || err != nil || date.Before(time.Now().AddDate(0, 0, -1)) || input.Guests < 1 || input.Guests > 8 {
			writeError(w, http.StatusBadRequest, "Provide a destination, a current travel date, and 1-8 guests.")
			return
		}
		result, err := a.db.ExecContext(r.Context(), `INSERT INTO bookings(user_id, destination, travel_date, guests, created_at) VALUES(?, ?, ?, ?, ?)`,
			account.ID, input.Destination, input.TravelDate, input.Guests, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not save booking.")
			return
		}
		id, err := result.LastInsertId()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not save booking.")
			return
		}
		a.recordEvent(r.Context(), &account.ID, "booking_created", "Booking "+fmt.Sprint(id))
		writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "status": "confirmed"})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (a *App) reviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	account, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Sign in to continue.")
		return
	}
	var input struct {
		ProductID int64
		Rating    int
		Body      string
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if input.ProductID <= 0 || input.Rating < 1 || input.Rating > 5 || len(input.Body) < 3 || len(input.Body) > 1000 {
		writeError(w, http.StatusBadRequest, "Provide a valid product, 1-5 rating, and 3-1000 character review.")
		return
	}
	_, err = a.db.ExecContext(r.Context(), `INSERT INTO reviews(user_id, product_id, rating, body, created_at) VALUES(?, ?, ?, ?, ?)`,
		account.ID, input.ProductID, input.Rating, input.Body, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "You have already reviewed this product.")
			return
		}
		writeError(w, http.StatusBadRequest, "Product was not found or review could not be saved.")
		return
	}
	a.recordEvent(r.Context(), &account.ID, "review_created", "Review for product "+fmt.Sprint(input.ProductID))
	writeJSON(w, http.StatusCreated, map[string]string{"status": "saved"})
}

func (a *App) labEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	account, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Sign in to continue.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT action, details, created_at FROM lab_events WHERE user_id = ? ORDER BY id DESC LIMIT 20`, account.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load activity.")
		return
	}
	defer rows.Close()
	type event struct {
		Action    string `json:"action"`
		Details   string `json:"details"`
		CreatedAt string `json:"created_at"`
	}
	result := make([]event, 0)
	for rows.Next() {
		var item event
		if err := rows.Scan(&item.Action, &item.Details, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load activity.")
			return
		}
		result = append(result, item)
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) recordEvent(ctx context.Context, userID *int64, action, details string) {
	if _, err := a.db.ExecContext(ctx, `INSERT INTO lab_events(user_id, action, details, created_at) VALUES(?, ?, ?, ?)`,
		userID, action, details, time.Now().UTC().Format(time.RFC3339)); err != nil {
		log.Printf("showcase audit event %q could not be persisted: %v", action, err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination interface{}) bool {
	if r.ContentLength > 16<<10 {
		writeError(w, http.StatusRequestEntityTooLarge, "Request body is too large.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "Request must contain one JSON value.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "Method not allowed.")
}
