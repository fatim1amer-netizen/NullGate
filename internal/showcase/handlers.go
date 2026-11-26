package showcase

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func GraduationHandler() (http.Handler, error) {
	site, err := fs.Sub(pages, "assets/graduation")
	if err != nil {
		return nil, err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		for _, prefix := range []string{"graduation/", "original/"} {
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "" || name == "." {
			name = "index.html"
		}
		data, err := fs.ReadFile(site, name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) || path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
			data, err = fs.ReadFile(site, name)
			if err != nil {
				http.Error(w, "Graduation application unavailable", http.StatusInternalServerError)
				return
			}
		}
		contentType := mime.TypeByExtension(path.Ext(name))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
	return handler, nil
}

func OriginalHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/login" {
			http.Redirect(w, r, "http://127.0.0.1:9091/login", http.StatusSeeOther)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := pages.ReadFile("assets/original.html")
		if err != nil {
			http.Error(w, "Original website unavailable", http.StatusInternalServerError)
			return
		}
		styleStart := strings.Index(string(data), "<style>")
		styleEnd := strings.Index(string(data), "</style>")
		if styleStart < 0 || styleEnd <= styleStart+len("<style>") {
			http.Error(w, "Original website security policy unavailable", http.StatusInternalServerError)
			return
		}
		style := data[styleStart+len("<style>") : styleEnd]
		styleHash := sha256.Sum256(style)
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; script-src 'self'; style-src 'self' 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'; "+
				"img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
}
