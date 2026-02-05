package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// WebSocket tests for rate limiting, origin validation, connection limits, and multi-utterance.

func TestAudioWebSocketHandler_RateLimiting(t *testing.T) {
	rateLimiter := NewRateLimiter(2, time.Minute)
	handler := NewAudioWebSocketHandlerWithRateLimiter("", nil, nil, rateLimiter)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First connection should succeed
	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	conn1.Close(websocket.StatusNormalClosure, "done")

	// Second connection should succeed
	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("second connection should succeed: %v", err)
	}
	conn2.Close(websocket.StatusNormalClosure, "done")

	// Third connection should be rate limited
	_, _, err = websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("third connection should fail due to rate limiting")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "failed") {
		t.Logf("rate limit error (expected): %v", err)
	}
}

func TestAudioWebSocketHandler_NoRateLimiter(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Open multiple connections - all should succeed
	var connections []*websocket.Conn
	for i := 0; i < 5; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("connection %d failed: %v", i+1, err)
		}
		connections = append(connections, conn)
	}

	for _, conn := range connections {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

func TestAudioWebSocketHandler_RateLimitResponseFormat(t *testing.T) {
	rateLimiter := NewRateLimiter(1, time.Minute)
	handler := NewAudioWebSocketHandlerWithRateLimiter("", nil, nil, rateLimiter)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// First connection uses up the quota
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "done")

	// Second connection should get HTTP 429 response
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGVzdC1rZXk=")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After: 60, got %s", resp.Header.Get("Retry-After"))
	}
}

func TestAudioWebSocketHandler_OriginValidation_AllowsMatchingOrigin(t *testing.T) {
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "http://localhost")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://localhost"},
		},
	})
	if err != nil {
		t.Fatalf("connection with matching origin should succeed: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "done")
}

func TestAudioWebSocketHandler_OriginValidation_RejectsNonMatchingOrigin(t *testing.T) {
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "https://example.com")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"https://evil.com"},
		},
	})
	if err == nil {
		t.Fatal("connection with non-matching origin should be rejected")
	}
	t.Logf("origin rejected (expected): %v", err)
}

func TestAudioWebSocketHandler_OriginValidation_WildcardAllowsAll(t *testing.T) {
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "*")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"https://any-origin.com"},
		},
	})
	if err != nil {
		t.Fatalf("connection with wildcard origin should succeed: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "done")
}

func TestAudioWebSocketHandler_OriginValidation_EmptyAllowsAll(t *testing.T) {
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"https://any-origin.com"},
		},
	})
	if err != nil {
		t.Fatalf("connection with empty origin config should succeed: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "done")
}

func TestAudioWebSocketHandler_ConnectionLimit(t *testing.T) {
	connTracker := NewConnectionTracker(2)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	defer conn1.Close(websocket.StatusNormalClosure, "done")

	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("second connection should succeed: %v", err)
	}
	defer conn2.Close(websocket.StatusNormalClosure, "done")

	// Third connection should fail with 503
	_, _, err = websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("third connection should fail due to connection limit")
	}
	if !strings.Contains(err.Error(), "503") && !strings.Contains(err.Error(), "failed") {
		t.Logf("connection limit error (expected): %v", err)
	}
}

func TestAudioWebSocketHandler_ConnectionLimitResponseFormat(t *testing.T) {
	connTracker := NewConnectionTracker(1)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Second connection should get HTTP 503 response
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGVzdC1rZXk=")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}
}

func TestAudioWebSocketHandler_ConnectionReleasedOnClose(t *testing.T) {
	connTracker := NewConnectionTracker(1)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}

	// Second connection should fail
	_, _, err = websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("second connection should fail while first is active")
	}

	// Close first connection
	conn1.Close(websocket.StatusNormalClosure, "done")

	// Wait for server to process the close
	time.Sleep(100 * time.Millisecond)

	// Now a new connection should succeed
	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("connection after close should succeed: %v", err)
	}
	conn2.Close(websocket.StatusNormalClosure, "done")
}

func TestAudioWebSocketHandler_NoConnectionTracker(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var connections []*websocket.Conn
	for i := 0; i < 10; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("connection %d failed (no tracker should allow all): %v", i+1, err)
		}
		connections = append(connections, conn)
	}

	for _, conn := range connections {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

// TestAudioWebSocketHandler_MultiUtteranceWithoutReconnect tests that multiple
// utterances can be processed in a single WebSocket session without reconnecting.
func TestAudioWebSocketHandler_MultiUtteranceWithoutReconnect(t *testing.T) {
	callCount := 0
	transcripts := []string{"show me a cat", "show me a dog"}
	subjects := []string{"cat", "dog"}

	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transcript := transcripts[callCount%len(transcripts)]
		resp := WhisperResponse{Text: transcript}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &dynamicMockLLMProvider{subjects: subjects}

	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)
	handler.BufferThreshold = 500 * time.Millisecond

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Start recording
	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	// --- First utterance ---
	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("utterance 1: failed to send audio: %v", err)
	}

	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("utterance 1: failed to read transcript: %v", err)
	}

	var transcript1 TranscriptMessage
	if err := json.Unmarshal(respData, &transcript1); err != nil {
		t.Fatalf("utterance 1: failed to unmarshal transcript: %v", err)
	}
	if transcript1.Text != "show me a cat" {
		t.Errorf("utterance 1: expected 'show me a cat', got %s", transcript1.Text)
	}
	callCount++

	// Read error (database is nil)
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("utterance 1: failed to read error: %v", err)
	}

	// --- Second utterance (without reconnecting) ---
	audioData2 := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData2); err != nil {
		t.Fatalf("utterance 2: failed to send audio: %v", err)
	}

	_, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("utterance 2: failed to read transcript: %v", err)
	}

	var transcript2 TranscriptMessage
	if err := json.Unmarshal(respData, &transcript2); err != nil {
		t.Fatalf("utterance 2: failed to unmarshal transcript: %v", err)
	}
	if transcript2.Text != "show me a dog" {
		t.Errorf("utterance 2: expected 'show me a dog', got %s", transcript2.Text)
	}

	// Read error (database is nil)
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("utterance 2: failed to read error: %v", err)
	}

	// Success! Both utterances processed in single WebSocket session
}
