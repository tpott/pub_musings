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

// Basic WebSocket handler tests: ping/pong, start/stop, audio buffering, etc.

func TestAudioWebSocketHandler_Ping(t *testing.T) {
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
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

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

func TestAudioWebSocketHandler_EmptyBuffer_ImmediateStop(t *testing.T) {
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

	if errMsg.Message != "No audio recorded" {
		t.Errorf("expected 'No audio recorded' error, got %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_BufferThresholdAutoProcess(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a dog"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &mockLLMProvider{subject: "dog"}
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)
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

	// Should also receive error because database is nil
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
