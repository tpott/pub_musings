package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// --- WebSocket auth integration tests ---

func TestWSAuth_AuthenticatedConnection(t *testing.T) {
	database := setupWSAuthTestDB(t)
	user, sessionToken := createWSTestUser(t, database)

	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		Database:        database,
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect with session cookie
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{"session=" + sessionToken},
		},
	})
	if err != nil {
		t.Fatalf("authenticated connection should succeed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Verify user connection was tracked
	if tracker.UserConnCount(user.ID) != 1 {
		t.Errorf("user connection count should be 1, got %d", tracker.UserConnCount(user.ID))
	}
}

func TestWSAuth_AnonymousConnection(t *testing.T) {
	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect without session cookie
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("anonymous connection should succeed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Second anonymous connection from same test client should be rejected
	_, _, err = websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("second anonymous connection should be rejected (limit is 1)")
	}
}

func TestWSAuth_AnonymousConcurrentLimit(t *testing.T) {
	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First anonymous connection should succeed
	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first anonymous connection should succeed: %v", err)
	}
	defer conn1.Close(websocket.StatusNormalClosure, "done")

	// Second should fail (anon limit is 1 per IP)
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("second anonymous connection should be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Logf("expected 429, got status %d", resp.StatusCode)
	}
}

func TestWSAuth_RegisteredUserMultipleConnections(t *testing.T) {
	database := setupWSAuthTestDB(t)
	_, sessionToken := createWSTestUser(t, database)

	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		Database:        database,
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cookieHeader := http.Header{
		"Cookie": []string{"session=" + sessionToken},
	}

	// Registered users can open 3 concurrent connections
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader: cookieHeader,
		})
		if err != nil {
			t.Fatalf("connection %d should succeed for registered user: %v", i+1, err)
		}
		conns = append(conns, conn)
	}

	// 4th connection should fail
	_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: cookieHeader,
	})
	if err == nil {
		t.Fatal("4th connection should be rejected for registered user")
	}

	for _, conn := range conns {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

func TestWSAuth_ExpiredSessionTreatedAsAnonymous(t *testing.T) {
	database := setupWSAuthTestDB(t)

	// Create user with expired session
	hash, _ := auth.HashPassword("testpassword")
	id, _ := auth.GenerateID()
	now := time.Now().UTC()
	user := &db.User{
		ID:            id,
		Email:         "expired@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		VerifiedAt:    &now,
		CreatedAt:     now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	sessionToken, _ := auth.GenerateToken(32)
	sessionID, _ := auth.GenerateID()
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: auth.HashToken(sessionToken),
		ExpiresAt: now.Add(-1 * time.Hour), // Expired
		CreatedAt: now,
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		Database:        database,
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect with expired session cookie — treated as anonymous
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{"session=" + sessionToken},
		},
	})
	if err != nil {
		t.Fatalf("connection with expired session should succeed as anonymous: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Should be tracked as anonymous, not as user
	if tracker.UserConnCount(user.ID) != 0 {
		t.Error("expired session should not be tracked as user connection")
	}
}

func TestWSAuth_NoAuthTracker(t *testing.T) {
	// When AuthTracker is nil, no per-user/IP limits are applied
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should allow multiple connections without auth tracker
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("connection %d should succeed without auth tracker: %v", i+1, err)
		}
		conns = append(conns, conn)
	}

	for _, conn := range conns {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

func TestWSAuth_BearerTokenAuth(t *testing.T) {
	database := setupWSAuthTestDB(t)
	user, sessionToken := createWSTestUser(t, database)

	tracker := NewWSAuthTracker(DefaultWSAuthLimits())
	handler := &AudioWebSocketHandler{
		Database:        database,
		AuthTracker:     tracker,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect with bearer token instead of cookie
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + sessionToken},
		},
	})
	if err != nil {
		t.Fatalf("bearer token auth should succeed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Verify tracked as authenticated user
	if tracker.UserConnCount(user.ID) != 1 {
		t.Errorf("bearer auth should track user connection, got count %d", tracker.UserConnCount(user.ID))
	}
}
