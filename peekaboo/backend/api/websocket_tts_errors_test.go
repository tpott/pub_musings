package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// TestTTSFailureGraceful verifies that when the TTS provider fails, the
// handler logs a warning and continues to the next tool call.
func TestTTSFailureGraceful(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// TTS provider that returns an error
	mockTTS := &mockTTSProviderWS{err: fmt.Errorf("piper server unreachable")}

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{
				{
					Type: "text_to_speech",
					Text: "Here is a cat!",
				},
				{
					Type:    "show_media",
					Subject: "cat",
				},
			},
		},
	}

	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)
	handler.TTSProvider = mockTTS

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

	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Collect messages
	var messages []map[string]interface{}
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	for {
		_, respData, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(respData, &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	// Should NOT have tts_audio (TTS failed)
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			t.Error("should not have received tts_audio when TTS fails")
		}
	}

	// Should still have show_media error (continues after TTS failure)
	hasMediaError := false
	for _, msg := range messages {
		if msg["type"] == "error" {
			errMsg, _ := msg["message"].(string)
			if strings.Contains(errMsg, "media") || strings.Contains(errMsg, "unavailable") {
				hasMediaError = true
			}
		}
	}

	if !hasMediaError {
		t.Error("expected show_media error (TTS failure should not block subsequent tools)")
	}

	// Verify TTS was attempted
	if mockTTS.getCallCount() != 1 {
		t.Errorf("TTS provider called %d times, want 1", mockTTS.getCallCount())
	}
}

// TestTTSNilProviderSkips verifies that text_to_speech is silently skipped
// when no TTS provider is configured.
func TestTTSNilProviderSkips(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{
				{
					Type: "text_to_speech",
					Text: "Here is a cat!",
				},
				{
					Type:    "show_media",
					Subject: "cat",
				},
			},
		},
	}

	// No TTS provider configured
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

	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Collect messages
	var messages []map[string]interface{}
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	for {
		_, respData, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(respData, &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	// Should NOT have tts_audio (no provider)
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			t.Error("should not have received tts_audio when no TTS provider configured")
		}
	}

	// Should still process show_media (error because nil DB, but that's expected)
	hasMediaError := false
	for _, msg := range messages {
		if msg["type"] == "error" {
			errMsg, _ := msg["message"].(string)
			if strings.Contains(errMsg, "media") || strings.Contains(errMsg, "unavailable") {
				hasMediaError = true
			}
		}
	}

	if !hasMediaError {
		t.Error("expected show_media error (should still execute after skipping TTS)")
	}
}

// TestUnrecognizedSubjectTTSFeedback verifies that when the LLM returns
// show_media with a subject that has no media in the database, the handler
// speaks a friendly TTS message and sends a text error.
func TestUnrecognizedSubjectTTSFeedback(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me an elephant"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	ttsWAV := []byte("RIFF-WAV-audio")
	mockTTS := &mockTTSProviderWS{audioData: ttsWAV}

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{{
				Type:    "show_media",
				Subject: "elephant",
			}},
		},
	}

	// Set up DB with concepts but no media for "elephant"
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatalf("db.Init: %v", err)
	}

	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, database)
	handler.TTSProvider = mockTTS

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

	// Start recording, send audio, stop recording
	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Collect messages
	var messages []map[string]interface{}
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	for {
		_, respData, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(respData, &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	// Verify error message was sent
	hasError := false
	for _, msg := range messages {
		if msg["type"] == "error" {
			errMsg, _ := msg["message"].(string)
			if strings.Contains(errMsg, "I don't know that one yet") {
				hasError = true
			}
		}
	}
	if !hasError {
		t.Error("expected error message with friendly unrecognized subject text")
	}

	// Verify TTS was called with the friendly message
	if mockTTS.getCallCount() != 1 {
		t.Errorf("TTS provider called %d times, want 1", mockTTS.getCallCount())
	}
	if !strings.Contains(mockTTS.getLastText(), "I don't know that one yet") {
		t.Errorf("TTS text = %q, want message containing 'I don't know that one yet'", mockTTS.getLastText())
	}

	// Verify tts_audio message was sent
	hasTTSAudio := false
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			hasTTSAudio = true
		}
	}
	if !hasTTSAudio {
		t.Error("expected tts_audio message for unrecognized subject")
	}
}

// TestUnrecognizedSubjectNoTTS verifies that when the subject is not found
// and no TTS provider is configured, only the text error is sent.
func TestUnrecognizedSubjectNoTTS(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a giraffe"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{{
				Type:    "show_media",
				Subject: "giraffe",
			}},
		},
	}

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatalf("db.Init: %v", err)
	}

	// No TTS provider configured
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, database)

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

	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	audioData := make([]byte, 2000)
	if err := conn.Write(ctx, websocket.MessageBinary, audioData); err != nil {
		t.Fatalf("failed to send audio: %v", err)
	}

	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Collect messages
	var messages []map[string]interface{}
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	for {
		_, respData, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(respData, &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	// Verify error message was sent
	hasError := false
	for _, msg := range messages {
		if msg["type"] == "error" {
			errMsg, _ := msg["message"].(string)
			if strings.Contains(errMsg, "I don't know that one yet") {
				hasError = true
			}
		}
	}
	if !hasError {
		t.Error("expected error message for unrecognized subject")
	}

	// Verify NO tts_audio message (no TTS provider)
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			t.Error("should not have received tts_audio when no TTS provider configured")
		}
	}
}
