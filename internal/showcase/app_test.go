package showcase

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOriginalPageMatchesGitHubPagesSource(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "docs", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	OriginalHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://nullgate.localhost/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("original page status = %d", recorder.Code)
	}
	if !bytes.Equal(recorder.Body.Bytes(), source) {
		t.Fatal("embedded original page differs from docs/index.html")
	}
	start := bytes.Index(source, []byte("<style>"))
	end := bytes.Index(source, []byte("</style>"))
	hash := sha256.Sum256(source[start+len("<style>") : end])
	if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), "sha256-"+base64.StdEncoding.EncodeToString(hash[:])) {
		t.Fatal("original page CSP does not allow only its trusted inline stylesheet")
	}
}

func TestLabAssetsAreServedUnderSameOriginCSP(t *testing.T) {
	app, err := Open(filepath.Join(t.TempDir(), "showcase.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	for _, path := range []string{"/", "/assets/lab.css", "/assets/lab.js"} {
		recorder := httptest.NewRecorder()
		app.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://cyberlab.localhost"+path, nil))
		if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
			t.Fatalf("%s returned %d and %q", path, recorder.Code, recorder.Body.String())
		}
		if path == "/" && (!strings.Contains(recorder.Body.String(), `href="assets/lab.css"`) || !strings.Contains(recorder.Body.String(), `src="assets/lab.js" defer`)) {
			t.Fatal("lab page does not load external same-origin assets")
		}
	}
}

func TestLocalLabPersistsAccountsAndBookings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "showcase.db")
	app, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	register := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"Email":"student@example.test","Name":"Student","Password":"local-lab-password-123"}`))
	register.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, register)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register status = %d: %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "ng_lab_session" || !cookies[0].HttpOnly {
		t.Fatalf("registration did not set a secure session cookie: %#v", cookies)
	}

	bookRequest := httptest.NewRequest(http.MethodPost, "/api/bookings", strings.NewReader(`{"Destination":"Helsinki","TravelDate":"2099-01-02","Guests":2}`))
	bookRequest.Header.Set("Content-Type", "application/json")
	bookRequest.AddCookie(cookies[0])
	bookRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(bookRecorder, bookRequest)
	if bookRecorder.Code != http.StatusCreated {
		t.Fatalf("booking status = %d: %s", bookRecorder.Code, bookRecorder.Body.String())
	}
	reviewRequest := httptest.NewRequest(http.MethodPost, "/api/reviews", strings.NewReader(`{"ProductID":1,"Rating":5,"Body":"Clear and practical."}`))
	reviewRequest.Header.Set("Content-Type", "application/json")
	reviewRequest.AddCookie(cookies[0])
	reviewRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusCreated {
		t.Fatalf("review status = %d: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	_ = app.Close()

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	me := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	me.AddCookie(cookies[0])
	meRecorder := httptest.NewRecorder()
	reopened.Handler().ServeHTTP(meRecorder, me)
	if meRecorder.Code != http.StatusOK || !strings.Contains(meRecorder.Body.String(), "student@example.test") {
		t.Fatalf("account/session did not persist across restart: %d %s", meRecorder.Code, meRecorder.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/api/bookings", nil)
	list.AddCookie(cookies[0])
	listRecorder := httptest.NewRecorder()
	reopened.Handler().ServeHTTP(listRecorder, list)
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), "Helsinki") {
		t.Fatalf("booking did not persist across restart: %d %s", listRecorder.Code, listRecorder.Body.String())
	}

	login := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"Email":"student@example.test","Password":"local-lab-password-123"}`))
	login.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	reopened.Handler().ServeHTTP(loginRecorder, login)
	if loginRecorder.Code != http.StatusOK || len(loginRecorder.Result().Cookies()) == 0 {
		t.Fatalf("login did not authenticate persisted account: %d %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestLabSearchUsesDataAndDoesNotExecuteSQLLikeInput(t *testing.T) {
	app, err := Open(filepath.Join(t.TempDir(), "showcase.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	normal := httptest.NewRecorder()
	app.Handler().ServeHTTP(normal, httptest.NewRequest(http.MethodGet, "/api/products?q=guide", nil))
	if normal.Code != http.StatusOK || !strings.Contains(normal.Body.String(), "Secure Coding Field Guide") {
		t.Fatalf("catalog did not return seeded database record: %d %s", normal.Code, normal.Body.String())
	}

	injection := httptest.NewRecorder()
	app.Handler().ServeHTTP(injection, httptest.NewRequest(http.MethodGet, "/api/products?q=%27%20OR%20%271%27%3D%271", nil))
	if injection.Code != http.StatusOK {
		t.Fatalf("safe injection-like search status = %d: %s", injection.Code, injection.Body.String())
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(injection.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("search input unexpectedly changed SQL semantics: %#v", result)
	}
}

func TestBookingsRequireAuthentication(t *testing.T) {
	app, err := Open(filepath.Join(t.TempDir(), "showcase.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/bookings", strings.NewReader(`{"Destination":"Helsinki","TravelDate":"2099-01-02","Guests":2}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated booking status = %d, want 401", recorder.Code)
	}
}
