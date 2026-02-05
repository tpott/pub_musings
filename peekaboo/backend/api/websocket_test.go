package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

func TestGetIdleTimeout(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   time.Duration
	}{
		{"default when not set", "", 5 * time.Minute},
		{"custom value", "120", 120 * time.Second},
		{"invalid value returns default", "not-a-number", 5 * time.Minute},
		{"zero returns default", "0", 5 * time.Minute},
		{"negative returns default", "-100", 5 * time.Minute},
		{"large value", "3600", 3600 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv("WEBSOCKET_IDLE_TIMEOUT_SECS", tt.envVal)
				defer os.Unsetenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
			} else {
				os.Unsetenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
			}

			got := getIdleTimeout()
			if got != tt.want {
				t.Errorf("getIdleTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockLLMProvider implements llm.Provider for testing.
type mockLLMProvider struct {
	subject   string
	err       error
	healthErr error
}

func (m *mockLLMProvider) ExtractIntent(ctx context.Context, text string) (*llm.IntentResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.IntentResult{Subject: m.subject}, nil
}

func (m *mockLLMProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func TestAudioWebSocketHandler_Ping(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send ping
	pingMsg := ClientMessage{Type: MsgTypePing}
	data, _ := json.Marshal(pingMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send ping: %v", err)
	}

	// Read pong
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var pong PongMessage
	if err := json.Unmarshal(respData, &pong); err != nil {
		t.Fatalf("failed to unmarshal pong: %v", err)
	}

	if pong.Type != MsgTypePong {
		t.Errorf("expected pong type, got %s", pong.Type)
	}
}

func TestAudioWebSocketHandler_StartStopRecording(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send start_recording
	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	// Send some audio (smaller than minimum, so no processing)
	audioData := make([]byte, 100)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Send stop_recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive error because audio is too short
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}

	if errMsg.Message != "audio too short" {
		t.Errorf("expected 'audio too short' error, got %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_AudioBuffering(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Send multiple audio chunks
	for i := 0; i < 3; i++ {
		audioData := make([]byte, 500) // 500 bytes each
		if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
			t.Fatalf("failed to send audio chunk %d: %v", i, err)
		}
	}

	// Stop recording - should trigger processing but fail because no whisper URL
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive error because whisper isn't available
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
}

func TestAudioWebSocketHandler_InvalidControlMessage(t *testing.T) {
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send invalid JSON
	if err := conn.Write(ctx, websocket.MessageText, []byte("not json")); err != nil {
		t.Fatalf("failed to send invalid message: %v", err)
	}

	// Should receive error
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}

	if errMsg.Message != "invalid message format" {
		t.Errorf("expected 'invalid message format' error, got %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_WithMockedWhisper(t *testing.T) {
	// Create mock whisper server
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Text: "show me a cat",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// Create mock LLM provider
	mockProvider := &mockLLMProvider{subject: "cat"}

	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Send enough audio data (1KB+)
	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive transcript
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}

	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	if transcript.Type != MsgTypeTranscript {
		t.Errorf("expected transcript type, got %s", transcript.Type)
	}

	if transcript.Text != "show me a cat" {
		t.Errorf("expected 'show me a cat', got %s", transcript.Text)
	}

	// Should receive error because database is nil
	_, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read media error: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
}

func TestAudioWebSocketHandler_RateLimiting(t *testing.T) {
	// Create rate limiter: 2 connections per minute
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

	// Third connection should be rate limited - can't upgrade to WebSocket
	// The server will return HTTP 429 before upgrade
	_, _, err = websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("third connection should fail due to rate limiting")
	}
	// The error should indicate the upgrade failed
	if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "failed") {
		t.Logf("rate limit error (expected): %v", err)
	}
}

func TestAudioWebSocketHandler_NoRateLimiter(t *testing.T) {
	// Handler without rate limiter should accept unlimited connections
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

	// Clean up
	for _, conn := range connections {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

func TestAudioWebSocketHandler_RateLimitResponseFormat(t *testing.T) {
	// Create rate limiter: 1 connection per minute
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
	// Use regular HTTP client to verify response format
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
	// Create handler that allows only http://localhost
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "http://localhost")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connection with matching origin should succeed
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
	// Create handler that allows only https://example.com
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "https://example.com")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connection with non-matching origin should fail
	_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"https://evil.com"},
		},
	})
	if err == nil {
		t.Fatal("connection with non-matching origin should be rejected")
	}
	// The error should indicate the connection was rejected
	t.Logf("origin rejected (expected): %v", err)
}

func TestAudioWebSocketHandler_OriginValidation_WildcardAllowsAll(t *testing.T) {
	// Create handler with wildcard origin (allows all)
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "*")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connection with any origin should succeed
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
	// Create handler with empty origin (allows all)
	handler := NewAudioWebSocketHandlerWithOptions("", nil, nil, nil, "")

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connection with any origin should succeed
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

func TestAudioWebSocketHandler_InvalidSubjectFormat(t *testing.T) {
	// Test: LLM returns subject with invalid characters (e.g., "tabby cat" with space)
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a tabby cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns "tabby cat" which has a space (invalid format)
	mockProvider := &mockLLMProvider{subject: "tabby cat"}
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Send some audio
	audioData := make([]byte, 5000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// First message should be transcript
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}
	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}
	if transcript.Type != MsgTypeTranscript {
		t.Errorf("expected transcript type, got %s", transcript.Type)
	}

	// Second message should be error about invalid format
	msgType, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
	if !strings.Contains(errMsg.Message, "tabby cat") {
		t.Errorf("expected error to mention invalid subject, got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_EmptySubject(t *testing.T) {
	// Test: LLM returns empty subject
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "hello world"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns empty subject (couldn't extract intent)
	mockProvider := &mockLLMProvider{subject: ""}
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Send some audio
	audioData := make([]byte, 5000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// First message should be transcript
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}
	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	// Second message should be error about empty subject
	msgType, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
	if !strings.Contains(errMsg.Message, "didn't understand") {
		t.Errorf("expected helpful error message, got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_SubjectTooLong(t *testing.T) {
	// Test: LLM returns subject that exceeds max length (50 chars)
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a very long thing"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns 51-character subject
	longSubject := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz" // 52 chars
	mockProvider := &mockLLMProvider{subject: longSubject}
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Send some audio
	audioData := make([]byte, 5000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// First message should be transcript
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}
	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	// Second message should be error about subject too long
	msgType, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
	if !strings.Contains(errMsg.Message, "too long") {
		t.Errorf("expected error about length, got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_ValidSubjectAtMaxLength(t *testing.T) {
	// Test: LLM returns subject exactly at max length (50 chars) - should pass validation
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a thing"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns exactly 50-character subject (valid)
	maxLengthSubject := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv12" // exactly 50 chars
	mockProvider := &mockLLMProvider{subject: maxLengthSubject}

	// Create mock database that returns nil (no media found, but validation passed)
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	// Send some audio
	audioData := make([]byte, 5000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// First message should be transcript
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}
	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	// Second message should be error about "media lookup unavailable" (no database)
	// NOT a validation error - this proves validation passed
	msgType, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
	// Should fail at database lookup, not at validation
	if !strings.Contains(errMsg.Message, "media lookup unavailable") {
		t.Errorf("expected 'media lookup unavailable' (validation should pass), got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_EmptyBuffer_ImmediateStop(t *testing.T) {
	// Test: start_recording immediately followed by stop_recording with no audio chunks
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send start_recording
	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	// Immediately send stop_recording WITHOUT any audio chunks
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive error about no audio
	msgType, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("expected text message, got %v", msgType)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}

	// Should get a user-friendly error about no audio recorded
	if errMsg.Message != "No audio recorded" {
		t.Errorf("expected 'No audio recorded' error, got %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_EmptyTranscript(t *testing.T) {
	// Create mock whisper server that returns empty text
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Text: "", // Empty transcript
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	handler := NewAudioWebSocketHandler(mockWhisper.URL, nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Send enough audio data (1KB+)
	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive error about no speech detected
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}

	if errMsg.Message != "No speech detected. Please try again." {
		t.Errorf("expected 'No speech detected' error, got %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_BufferThresholdAutoProcess(t *testing.T) {
	// Create mock whisper server
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Text: "show me a dog",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// Create mock LLM provider
	mockProvider := &mockLLMProvider{subject: "dog"}

	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)
	// Set a shorter buffer threshold for faster test (1 second)
	handler.BufferThreshold = 1 * time.Second

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Send enough audio data (1KB+) to pass minimum threshold
	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// DO NOT send stop_recording - let buffer threshold auto-trigger
	// Wait for auto-processing (buffer threshold + processing time)
	// The ticker runs every 500ms, so we need to wait at least 1.5s

	// Read transcript - should arrive automatically after ~1-1.5 seconds
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read auto-triggered transcript: %v", err)
	}

	var transcript TranscriptMessage
	if err := json.Unmarshal(respData, &transcript); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	if transcript.Type != MsgTypeTranscript {
		t.Errorf("expected transcript type, got %s", transcript.Type)
	}

	if transcript.Text != "show me a dog" {
		t.Errorf("expected 'show me a dog', got %s", transcript.Text)
	}

	// Should also receive error because database is nil (after transcript is sent)
	_, respData, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read media error: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}
}

func TestAudioWebSocketHandler_WhitespaceOnlyTranscript(t *testing.T) {
	// Create mock whisper server that returns whitespace-only text
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Text: "   \n\t  ", // Whitespace only
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	handler := NewAudioWebSocketHandler(mockWhisper.URL, nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Send enough audio data (1KB+)
	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Should receive error about no speech detected
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var errMsg ErrorMessage
	if err := json.Unmarshal(respData, &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errMsg.Type != MsgTypeError {
		t.Errorf("expected error type, got %s", errMsg.Type)
	}

	if errMsg.Message != "No speech detected. Please try again." {
		t.Errorf("expected 'No speech detected' error, got %s", errMsg.Message)
	}
}

func TestGetMaxConnections(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   int
	}{
		{"default when not set", "", 100},
		{"custom value", "50", 50},
		{"invalid value returns default", "not-a-number", 100},
		{"zero returns default", "0", 100},
		{"negative returns default", "-10", 100},
		{"large value", "1000", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv("WEBSOCKET_MAX_CONNECTIONS", tt.envVal)
				defer os.Unsetenv("WEBSOCKET_MAX_CONNECTIONS")
			} else {
				os.Unsetenv("WEBSOCKET_MAX_CONNECTIONS")
			}

			got := getMaxConnections()
			if got != tt.want {
				t.Errorf("getMaxConnections() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionTracker(t *testing.T) {
	t.Run("TryAcquire succeeds under limit", func(t *testing.T) {
		tracker := NewConnectionTracker(3)

		if !tracker.TryAcquire() {
			t.Error("first TryAcquire should succeed")
		}
		if tracker.Count() != 1 {
			t.Errorf("expected count 1, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("second TryAcquire should succeed")
		}
		if tracker.Count() != 2 {
			t.Errorf("expected count 2, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("third TryAcquire should succeed")
		}
		if tracker.Count() != 3 {
			t.Errorf("expected count 3, got %d", tracker.Count())
		}
	})

	t.Run("TryAcquire fails at limit", func(t *testing.T) {
		tracker := NewConnectionTracker(2)

		tracker.TryAcquire()
		tracker.TryAcquire()

		if tracker.TryAcquire() {
			t.Error("third TryAcquire should fail when at limit")
		}
		if tracker.Count() != 2 {
			t.Errorf("expected count to stay at 2, got %d", tracker.Count())
		}
	})

	t.Run("Release frees slot", func(t *testing.T) {
		tracker := NewConnectionTracker(2)

		tracker.TryAcquire()
		tracker.TryAcquire()

		if tracker.TryAcquire() {
			t.Error("should be at limit")
		}

		tracker.Release()
		if tracker.Count() != 1 {
			t.Errorf("expected count 1 after release, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("TryAcquire should succeed after Release")
		}
	})

	t.Run("Max returns correct value", func(t *testing.T) {
		tracker := NewConnectionTracker(42)
		if tracker.Max() != 42 {
			t.Errorf("expected max 42, got %d", tracker.Max())
		}
	})
}

func TestAudioWebSocketHandler_ConnectionLimit(t *testing.T) {
	// Create handler with limit of 2 connections
	connTracker := NewConnectionTracker(2)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

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
	defer conn1.Close(websocket.StatusNormalClosure, "done")

	// Second connection should succeed
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
	// The error should indicate the connection was rejected
	if !strings.Contains(err.Error(), "503") && !strings.Contains(err.Error(), "failed") {
		t.Logf("connection limit error (expected): %v", err)
	}
}

func TestAudioWebSocketHandler_ConnectionLimitResponseFormat(t *testing.T) {
	// Create handler with limit of 1 connection
	connTracker := NewConnectionTracker(1)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// First connection uses up the slot
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// Second connection should get HTTP 503 response
	// Use regular HTTP client to verify response format
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
	// Create handler with limit of 1 connection
	connTracker := NewConnectionTracker(1)
	handler := NewAudioWebSocketHandlerWithConnTracker("", nil, nil, nil, "", connTracker)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// First connection uses the slot
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

	// Wait a moment for the server to process the close
	time.Sleep(100 * time.Millisecond)

	// Now a new connection should succeed
	conn2, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("connection after close should succeed: %v", err)
	}
	conn2.Close(websocket.StatusNormalClosure, "done")
}

func TestAudioWebSocketHandler_NoConnectionTracker(t *testing.T) {
	// Handler without connection tracker should accept unlimited connections
	handler := NewAudioWebSocketHandler("", nil, nil)

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Open multiple connections - all should succeed
	var connections []*websocket.Conn
	for i := 0; i < 10; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("connection %d failed (no tracker should allow all): %v", i+1, err)
		}
		connections = append(connections, conn)
	}

	// Clean up
	for _, conn := range connections {
		conn.Close(websocket.StatusNormalClosure, "done")
	}
}
