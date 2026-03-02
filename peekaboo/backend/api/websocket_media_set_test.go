package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestExecuteShowMediaReturnsMediaSetID verifies that executeShowMedia
// returns the media set ID when a matching media set is found.
func TestExecuteShowMediaReturnsMediaSetID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	if err := database.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3", ""); err != nil {
		t.Fatalf("SeedMediaSet: %v", err)
	}

	// Get the seeded media set ID
	ms, err := database.GetRandomMediaSet("cat")
	if err != nil || ms == nil {
		t.Fatalf("GetRandomMediaSet: err=%v, ms=%v", err, ms)
	}
	expectedID := ms.ID

	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a WebSocket pair via httptest
	var serverConn *websocket.Conn
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept: %v", err)
			return
		}
		serverConn = c
		close(ready)
		// Keep connection open until test is done
		select {}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientConn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close(websocket.StatusNormalClosure, "done")

	// Wait for server to accept
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("timeout waiting for WebSocket accept")
	}
	defer serverConn.Close(websocket.StatusNormalClosure, "done")

	// Call executeShowMedia with a known subject
	tr, mediaSetID := handler.executeShowMedia(ctx, serverConn, "cat", logger)

	if tr != nil {
		t.Errorf("ttsResult: got %+v, want nil (media was found)", tr)
	}
	if mediaSetID == nil {
		t.Fatal("mediaSetID is nil, expected non-nil")
	}
	if *mediaSetID != expectedID {
		t.Errorf("mediaSetID: got %d, want %d", *mediaSetID, expectedID)
	}

	// Read the media message from the client side
	_, data, err := clientConn.Read(ctx)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	var msg struct {
		Type    string `json:"type"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != MsgTypeMedia || msg.Subject != "cat" {
		t.Errorf("media message: got type=%q subject=%q, want type=%q subject=%q",
			msg.Type, msg.Subject, MsgTypeMedia, "cat")
	}
}

// TestExecuteShowMediaReturnsNilForUnknownSubject verifies that
// executeShowMedia returns nil mediaSetID when no media is found.
func TestExecuteShowMediaReturnsNilForUnknownSubject(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create WebSocket pair
	var serverConn *websocket.Conn
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket.Accept: %v", err)
			return
		}
		serverConn = c
		close(ready)
		select {}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientConn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close(websocket.StatusNormalClosure, "done")

	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("timeout waiting for WebSocket accept")
	}
	defer serverConn.Close(websocket.StatusNormalClosure, "done")

	// Call with subject that has no media — returns TTS result + nil mediaSetID
	tr, mediaSetID := handler.executeShowMedia(ctx, serverConn, "elephant", logger)

	// No TTS provider configured, so tr should be nil
	if tr != nil {
		t.Errorf("ttsResult: got %+v, want nil (no TTS provider)", tr)
	}
	if mediaSetID != nil {
		t.Errorf("mediaSetID: got %d, want nil for unknown subject", *mediaSetID)
	}
}
