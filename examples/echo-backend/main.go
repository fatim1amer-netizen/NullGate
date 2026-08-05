package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "✅ Welcome! Your request passed NullGate WAF safely.\nQuery: %s", r.URL.RawQuery)
	})
	fmt.Println("🚀 Backend server running on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
