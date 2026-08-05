package main

import (
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// محاكاة قاعدة بيانات بسيطة لتجنب مشاكل تثبيت SQLite
var users = map[string]string{
	"admin": "admin123",
	"user":  "password",
}

func main() {
	// إنشاء ملف للتجربة
	os.WriteFile("readme.txt", []byte("هذا ملف readme.txt - محتوى تجريبي"), 0644)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		htmlPage := `<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
	<meta charset="UTF-8">
	<title>موقع اختبار الثغرات - Vulnerable App</title>
	<style>
		body { font-family: Arial; background: #1a1a1a; color: #fff; margin: 0; padding: 20px; }
		.container { max-width: 800px; margin: 0 auto; }
		h1 { color: #ff6b6b; text-align: center; }
		.section { background: #2d2d2d; padding: 20px; margin: 20px 0; border-radius: 10px; border-right: 4px solid #ff6b6b; }
		.section h2 { margin-top: 0; color: #ffd93d; }
		input, button { padding: 10px; margin: 5px; width: 70%; background: #1a1a1a; color: #fff; border: 1px solid #555; border-radius: 5px; }
		button { background: #ff6b6b; cursor: pointer; width: auto; }
		button:hover { background: #ff4757; }
		.warning { background: #8b0000; padding: 10px; border-radius: 5px; margin-bottom: 20px; text-align: center; }
		code { background: #000; padding: 2px 6px; border-radius: 3px; color: #0f0; }
	</style>
</head>
<body>
<div class="container">
	<h1>⚠️ موقع اختبار الثغرات ⚠️</h1>
	<div class="warning">هذا الموقع يحتوي على ثغرات حقيقية لأغراض الاختبار فقط!</div>

	<div class="section">
		<h2>1. ثغرة XSS (Cross-Site Scripting)</h2>
		<p>جرب: <code>&lt;script&gt;alert('XSS')&lt;/script&gt;</code></p>
		<form action="/xss" method="GET">
			<input type="text" name="search" placeholder="ابحث عن شيء...">
			<button type="submit">بحث</button>
		</form>
	</div>

	<div class="section">
		<h2>2. ثغرة SQL Injection</h2>
		<p>جرب كلمة المرور: <code>' OR '1'='1</code></p>
		<form action="/login" method="POST">
			<input type="text" name="username" placeholder="اسم المستخدم" value="admin"><br>
			<input type="password" name="password" placeholder="كلمة المرور"><br>
			<button type="submit">تسجيل الدخول</button>
		</form>
	</div>

	<div class="section">
		<h2>3. ثغرة LFI (Local File Inclusion)</h2>
		<p>جرب: <code>../vulnerable_app.go</code></p>
		<form action="/file" method="GET">
			<input type="text" name="path" placeholder="مسار الملف" value="readme.txt">
			<button type="submit">قراءة الملف</button>
		</form>
	</div>

	<div class="section">
		<h2>4. ثغرة SSRF (Server-Side Request Forgery)</h2>
		<p>جرب: <code>http://127.0.0.1:8080</code></p>
		<form action="/fetch" method="GET">
			<input type="text" name="url" placeholder="URL لجلبه" value="https://example.com">
			<button type="submit">جلب المحتوى</button>
		</form>
	</div>
</div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlPage)
	})

	// XSS Vulnerable Endpoint
	http.HandleFunc("/xss", func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<h1>نتائج البحث عن: %s</h1><br><a href="/">رجوع</a>`, search)
	})

	// SQLi Vulnerable Endpoint (Simulated)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		// محاكاة الاستعلام الضعيف
		query := fmt.Sprintf("SELECT * FROM users WHERE username='%s' AND password='%s'", username, password)

		// محاكاة نجاح الدخول إذا كانت كلمة المرور صحيحة أو تم حقن الكود
		isAuthorized := false
		if realPass, exists := users[username]; exists {
			if password == realPass || strings.Contains(strings.ToLower(query), "' or '1'='1") {
				isAuthorized = true
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if isAuthorized {
			fmt.Fprintf(w, `<h1 style="color:green">✅ تم تسجيل الدخول!</h1><p>مرحباً %s</p><p>الاستعلام المنفذ: <code>%s</code></p><a href="/">رجوع</a>`, html.EscapeString(username), html.EscapeString(query))
		} else {
			fmt.Fprintf(w, `<h1 style="color:red">❌ فشل تسجيل الدخول</h1><p>الاستعلام المنفذ: <code>%s</code></p><a href="/">رجوع</a>`, html.EscapeString(query))
		}
	})

	// LFI Vulnerable Endpoint
	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "فشل: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)
	})

	// SSRF Vulnerable Endpoint
	http.HandleFunc("/fetch", func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("url")
		resp, err := http.Get(targetURL)
		if err != nil {
			http.Error(w, "فشل: "+err.Error(), 500)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<h1>محتوى: %s</h1><pre style="background:#000;padding:10px;">%s</pre><a href="/">رجوع</a>`,
			html.EscapeString(targetURL), html.EscapeString(string(body)))
	})

	fmt.Println("🚀 Vulnerable App running on http://127.0.0.1:3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
