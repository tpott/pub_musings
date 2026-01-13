package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Fatal("WEBHOOK_SECRET environment variable is required")
	}

	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if resendAPIKey == "" {
		log.Fatal("RESEND_API_KEY environment variable is required")
	}

	emailFrom := os.Getenv("EMAIL_FROM")
	if emailFrom == "" {
		log.Fatal("EMAIL_FROM environment variable is required")
	}

	emailTo := os.Getenv("EMAIL_TO")
	if emailTo == "" {
		log.Fatal("EMAIL_TO environment variable is required")
	}

	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		log.Fatal("ALLOWED_ORIGIN environment variable is required")
	}

	sitePath := os.Getenv("SITE_PATH")
	if sitePath == "" {
		sitePath = "/home/trevor/pub_musings/personal"
	}

	// Create handlers
	webhookHandler := NewWebhookHandler(webhookSecret, sitePath)
	contactHandler := NewContactHandler(resendAPIKey, emailFrom, emailTo)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook", webhookHandler.Handle)
	mux.HandleFunc("POST /api/contact", contactHandler.Handle)
	mux.HandleFunc("GET /health", healthHandler)

	// CORS middleware for contact form
	handler := corsMiddleware(mux, allowedOrigin)

	log.Printf("Starting webhook-deployer on port %s", port)
	log.Printf("Site path: %s", sitePath)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func corsMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	allowedOrigins := map[string]bool{
		allowedOrigin:           true,
		"http://localhost:4321": true,
		"http://127.0.0.1:4321": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow CORS for the contact endpoint
		if r.URL.Path == "/api/contact" {
			origin := r.Header.Get("Origin")
			if allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
