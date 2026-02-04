package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/trevorsmith/peekaboo/api"
	"github.com/trevorsmith/peekaboo/crypto"
	"github.com/trevorsmith/peekaboo/db"
	"github.com/trevorsmith/peekaboo/llm"
)

func main() {
	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/peekaboo.db"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Initialize schema and seed concepts
	if err := database.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Seed media sets from data/media directory if they don't exist
	if err := seedMediaFromDisk(database); err != nil {
		log.Printf("Warning: Failed to seed media from disk: %v", err)
	}

	// Create LLM provider from environment
	llmProvider, err := llm.NewProviderFromEnv()
	if err != nil {
		log.Fatalf("Failed to create LLM provider: %v", err)
	}

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.Handle("GET /health/live", api.NewLivenessHandler())
	mux.Handle("GET /health/ready", api.NewReadinessHandler(database))

	// API endpoints
	mux.Handle("POST /api/transcribe", api.NewTranscribeHandler(""))
	mux.Handle("POST /api/intent", api.NewIntentHandlerWithProvider(llmProvider))
	mux.Handle("GET /api/media/{concept}", api.NewMediaHandler(database))

	// Static file server for media files
	mediaDir := os.Getenv("MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "data/media"
	}

	// Check for age key file - if present, use encrypted file server
	ageKeyFile := os.Getenv("AGE_KEY_FILE")
	if ageKeyFile == "" {
		ageKeyFile = "data/age.key"
	}

	if _, err := os.Stat(ageKeyFile); err == nil {
		// Age key exists - use encrypted file server
		identity, err := crypto.LoadIdentityFromFile(ageKeyFile)
		if err != nil {
			log.Fatalf("Failed to load age key from %s: %v", ageKeyFile, err)
		}
		log.Printf("Using encrypted media serving with key from %s", ageKeyFile)
		mux.Handle("/data/media/", http.StripPrefix("/data/media/", api.NewEncryptedFileServer(mediaDir, identity)))
	} else {
		// No age key - use plain file server
		log.Printf("No age key found at %s, serving media files unencrypted", ageKeyFile)
		mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir(mediaDir))))
	}

	// Static file server for test fixtures (for e2e tests)
	mux.Handle("/fixtures/", http.StripPrefix("/fixtures/", http.FileServer(http.Dir("tests/fixtures"))))

	// Get allowed origin from environment
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		// Default to wildcard for development; set ALLOWED_ORIGIN in production
		allowedOrigin = "*"
		log.Printf("Warning: ALLOWED_ORIGIN not set, using wildcard '*'. Set ALLOWED_ORIGIN for production.")
	} else {
		log.Printf("CORS: allowing origin %s", allowedOrigin)
	}

	// Wrap with CORS middleware
	handler := corsMiddleware(mux, allowedOrigin)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Starting peekaboo server on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// corsMiddleware adds CORS headers for frontend access.
// allowedOrigin specifies the allowed origin for CORS requests.
// Use "*" to allow any origin (development only), or a specific origin like "https://peekaboo.example.com".
func corsMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// seedMediaFromDisk scans data/media/{concept}/set* directories and seeds the database.
// Supports both plain files (photo.jpg) and encrypted files (photo.jpg.age).
func seedMediaFromDisk(database *db.DB) error {
	mediaDir := os.Getenv("MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "data/media"
	}

	// Check if media directory exists
	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
		log.Printf("Media directory %s does not exist, skipping media seeding", mediaDir)
		return nil
	}

	// Iterate over concept directories
	concepts := []string{"cat", "dog", "duck", "pig", "chicken", "cow"}
	for _, concept := range concepts {
		conceptDir := filepath.Join(mediaDir, concept)
		if _, err := os.Stat(conceptDir); os.IsNotExist(err) {
			continue
		}

		// Find set directories
		entries, err := os.ReadDir(conceptDir)
		if err != nil {
			log.Printf("Warning: cannot read concept directory %s: %v", conceptDir, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "set") {
				continue
			}

			setDir := filepath.Join(conceptDir, entry.Name())

			// Check for required photo file (plain or encrypted)
			photoPath := filepath.Join(setDir, "photo.jpg")
			photoExists := false
			if _, err := os.Stat(photoPath); err == nil {
				photoExists = true
			} else if _, err := os.Stat(photoPath + ".age"); err == nil {
				photoExists = true
			}
			if !photoExists {
				continue
			}

			// Check for optional audio file (plain or encrypted)
			audioPath := ""
			audioFile := filepath.Join(setDir, "audio.mp3")
			if _, err := os.Stat(audioFile); err == nil {
				audioPath = filepath.Join("data/media", concept, entry.Name(), "audio.mp3")
			} else if _, err := os.Stat(audioFile + ".age"); err == nil {
				audioPath = filepath.Join("data/media", concept, entry.Name(), "audio.mp3")
			}

			// Relative path for database (without .age extension - handler adds it)
			relPhotoPath := filepath.Join("data/media", concept, entry.Name(), "photo.jpg")

			// Seed the media set (database SeedMediaSet handles duplicates)
			if err := database.SeedMediaSet(concept, relPhotoPath, audioPath, ""); err != nil {
				log.Printf("Warning: failed to seed media set for %s/%s: %v", concept, entry.Name(), err)
			}
		}
	}

	return nil
}
