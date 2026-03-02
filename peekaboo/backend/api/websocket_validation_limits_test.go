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

// WebSocket tests for transcript and subject length/size validation.

func TestAudioWebSocketHandler_OversizedTranscript(t *testing.T) {
	// Whisper returns a transcript exceeding maxTranscriptLength (500 chars)
	longTranscript := strings.Repeat("a", maxTranscriptLength+1)
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: longTranscript}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &mockLLMProvider{subject: "cat"}
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

	// First message should be transcript (still sent even if too long)
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

	// Second message should be error about intent extraction failing
	_, respData, err = conn.Read(ctx)
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
	if !strings.Contains(errMsg.Message, "intent extraction failed") {
		t.Errorf("expected 'intent extraction failed' error, got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_TranscriptAtMaxLength(t *testing.T) {
	// Whisper returns a transcript at exactly maxTranscriptLength (should pass validation)
	exactTranscript := strings.Repeat("a", maxTranscriptLength)
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: exactTranscript}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &mockLLMProvider{subject: "cat"}
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
	// NOT a transcript-too-long error — validation should pass at exactly max length
	_, respData, err = conn.Read(ctx)
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
		t.Errorf("expected 'media lookup unavailable' (transcript length validation should pass), got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_ValidSubjectAtMaxLength(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a thing"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns exactly 50-character subject (valid)
	maxLengthSubject := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv12"
	mockProvider := &mockLLMProvider{subject: maxLengthSubject}

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
	_, respData, err = conn.Read(ctx)
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
