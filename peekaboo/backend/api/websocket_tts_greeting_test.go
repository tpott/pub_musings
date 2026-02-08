package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// createTTSTestUser creates a user and session in the DB for TTS greeting tests.
func createTTSTestUser(t *testing.T, database *db.DB) string {
	t.Helper()
	hash, err := auth.HashPassword("testpass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	userID, err := auth.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	now := time.Now().UTC()
	if err := database.CreateUser(&db.User{
		ID: userID, Email: "tts@example.com", PasswordHash: hash,
		EmailVerified: true, VerifiedAt: &now, CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	token, err := auth.GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	sessionID, err := auth.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	if err := database.CreateSession(&db.Session{
		ID: sessionID, UserID: userID, Token: token,
		ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return token
}

// TestWelcomeGreetingOnFirstConnection verifies that an authenticated user
// receives a TTS welcome greeting on their first WebSocket connection.
func TestWelcomeGreetingOnFirstConnection(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	sessionToken := createTTSTestUser(t, database)

	ttsWAV := []byte("RIFF-WAV-greeting")
	mockTTS := &mockTTSProviderWS{audioData: ttsWAV}

	handler := &AudioWebSocketHandler{
		Database:        database,
		TTSProvider:     mockTTS,
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

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{"session=" + sessionToken},
		},
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Read the greeting message
	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	_, respData, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("expected to read greeting message: %v", err)
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(respData, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg["type"] != "tts_audio" {
		t.Errorf("expected tts_audio message, got %q", msg["type"])
	}
	if msg["text"] != "Welcome back!" {
		t.Errorf("greeting text = %q, want %q", msg["text"], "Welcome back!")
	}

	if mockTTS.getCallCount() != 1 {
		t.Errorf("TTS called %d times, want 1", mockTTS.getCallCount())
	}
}

// TestWelcomeGreetingNotRepeated verifies that the greeting is only sent
// on the first connection in a session, not on subsequent connections.
func TestWelcomeGreetingNotRepeated(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	sessionToken := createTTSTestUser(t, database)

	mockTTS := &mockTTSProviderWS{audioData: []byte("audio")}

	handler := &AudioWebSocketHandler{
		Database:        database,
		TTSProvider:     mockTTS,
		Client:          &http.Client{Timeout: 5 * time.Second},
		BufferThreshold: 30 * time.Second,
		IdleTimeout:     30 * time.Second,
		MaxMessageSize:  defaultMaxMessageSize,
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialOpts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie": []string{"session=" + sessionToken},
		},
	}

	// First connection — should get greeting
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	conn1, _, err := websocket.Dial(ctx1, wsURL, dialOpts)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	// Drain the greeting
	readCtx1, readCancel1 := context.WithTimeout(ctx1, 2*time.Second)
	defer readCancel1()
	_, _, _ = conn1.Read(readCtx1)
	conn1.Close(websocket.StatusNormalClosure, "done")
	time.Sleep(100 * time.Millisecond)

	// Second connection — should NOT get greeting
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	conn2, _, err := websocket.Dial(ctx2, wsURL, dialOpts)
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "done")

	// Try to read — should time out with no message
	readCtx2, readCancel2 := context.WithTimeout(ctx2, 1*time.Second)
	defer readCancel2()
	_, _, err = conn2.Read(readCtx2)
	if err == nil {
		t.Error("second connection should not receive a greeting")
	}

	// TTS should have been called exactly once (from first connection)
	if mockTTS.getCallCount() != 1 {
		t.Errorf("TTS called %d times, want 1", mockTTS.getCallCount())
	}
}

// TestWelcomeGreetingAnonymousSkipped verifies that anonymous connections
// do not receive a TTS welcome greeting.
func TestWelcomeGreetingAnonymousSkipped(t *testing.T) {
	mockTTS := &mockTTSProviderWS{audioData: []byte("audio")}

	handler := &AudioWebSocketHandler{
		TTSProvider:     mockTTS,
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

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Try to read — should time out with no greeting
	readCtx, readCancel := context.WithTimeout(ctx, 1*time.Second)
	defer readCancel()
	_, _, err = conn.Read(readCtx)
	if err == nil {
		t.Error("anonymous connection should not receive a greeting")
	}

	if mockTTS.getCallCount() != 0 {
		t.Errorf("TTS called %d times, want 0", mockTTS.getCallCount())
	}
}
