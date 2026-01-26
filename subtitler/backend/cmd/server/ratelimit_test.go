package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/trevor/subtitler/internal/analytics"
	"github.com/trevor/subtitler/internal/db"
	"github.com/trevor/subtitler/internal/ratelimit"
)

func TestRateLimit_Register(t *testing.T) {
	// Create test database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Run migrations
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := database.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize analytics service
	analyticsService := analytics.NewService(database.DB)

	// Create rate limiter with 5 requests per minute
	limiter := ratelimit.NewLimiter(5)
	defer limiter.Close()

	// Create handler with rate limiting
	handler := ratelimit.IPMiddleware(limiter)(
		http.HandlerFunc(handleRegister(database, "test-secret", false, analyticsService)),
	)

	// Make 5 requests (should all succeed)
	for i := 0; i < 5; i++ {
		reqBody := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// First request should succeed (201), others may fail due to duplicate email
		// but should not be rate limited
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("Request %d should not be rate limited, got status %d", i, w.Code)
		}

		// Check rate limit headers
		if w.Header().Get("X-RateLimit-Limit") != "5" {
			t.Errorf("Expected X-RateLimit-Limit: 5, got %s", w.Header().Get("X-RateLimit-Limit"))
		}
	}

	// 6th request should be rate limited
	reqBody := map[string]string{
		"email":    "test6@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th request should be rate limited, got status %d", w.Code)
	}

	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("Expected X-RateLimit-Remaining: 0, got %s", w.Header().Get("X-RateLimit-Remaining"))
	}

	if w.Header().Get("Retry-After") == "" {
		t.Error("Expected Retry-After header to be set")
	}
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	// Create test database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Run migrations
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := database.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize analytics service
	analyticsService := analytics.NewService(database.DB)

	// Create rate limiter with 5 requests per minute
	limiter := ratelimit.NewLimiter(5)
	defer limiter.Close()

	// Create handler with rate limiting
	handler := ratelimit.IPMiddleware(limiter)(
		http.HandlerFunc(handleRegister(database, "test-secret", false, analyticsService)),
	)

	// Make 5 requests from IP1
	for i := 0; i < 5; i++ {
		reqBody := map[string]string{
			"email":    "test1@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			t.Errorf("Request %d from IP1 should not be rate limited", i)
		}
	}

	// 6th request from IP1 should be rate limited
	reqBody := map[string]string{
		"email":    "test6@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Error("6th request from IP1 should be rate limited")
	}

	// Request from IP2 should succeed (independent limit)
	reqBody2 := map[string]string{
		"email":    "test2@example.com",
		"password": "password123",
	}
	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.RemoteAddr = "192.168.1.2:12345"
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code == http.StatusTooManyRequests {
		t.Error("Request from IP2 should not be rate limited (independent limit)")
	}
}

func TestRateLimit_XForwardedFor(t *testing.T) {
	// Create test database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Run migrations
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := database.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize analytics service
	analyticsService := analytics.NewService(database.DB)

	// Create rate limiter with 5 requests per minute
	limiter := ratelimit.NewLimiter(5)
	defer limiter.Close()

	// Create handler with rate limiting
	handler := ratelimit.IPMiddleware(limiter)(
		http.HandlerFunc(handleRegister(database, "test-secret", false, analyticsService)),
	)

	// Make 5 requests with X-Forwarded-For header
	for i := 0; i < 5; i++ {
		reqBody := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			t.Errorf("Request %d should not be rate limited", i)
		}
	}

	// 6th request should be rate limited
	reqBody := map[string]string{
		"email":    "test6@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Error("6th request should be rate limited based on X-Forwarded-For IP")
	}
}
