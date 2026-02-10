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

// WebSocket tests for transcript and subject validation.

func TestAudioWebSocketHandler_EmptyTranscript(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: ""}
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

	// Empty transcripts are silently ignored — no error sent to client
	readCtx, readCancel := context.WithTimeout(ctx, 1*time.Second)
	defer readCancel()
	_, _, err = conn.Read(readCtx)
	if err == nil {
		t.Error("expected no message for empty transcript, but received one")
	}
}

func TestAudioWebSocketHandler_WhitespaceOnlyTranscript(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "   \n\t  "}
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

	// Whitespace-only transcripts are silently ignored — no error sent to client
	readCtx, readCancel := context.WithTimeout(ctx, 1*time.Second)
	defer readCancel()
	_, _, err = conn.Read(readCtx)
	if err == nil {
		t.Error("expected no message for whitespace-only transcript, but received one")
	}
}

func TestAudioWebSocketHandler_InvalidSubjectFormat(t *testing.T) {
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
	if !strings.Contains(errMsg.Message, "didn't understand") {
		t.Errorf("expected helpful error message, got: %s", errMsg.Message)
	}
}

func TestAudioWebSocketHandler_SubjectTooLong(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a very long thing"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// LLM returns 52-character subject (exceeds max of 50)
	longSubject := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"
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
	if !strings.Contains(errMsg.Message, "too long") {
		t.Errorf("expected error about length, got: %s", errMsg.Message)
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
