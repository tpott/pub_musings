package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// mockTTSProviderWS implements tts.Provider for WebSocket TTS tests.
type mockTTSProviderWS struct {
	mu        sync.Mutex
	audioData []byte
	err       error
	callCount int
	lastText  string
}

func (m *mockTTSProviderWS) Synthesize(ctx context.Context, text string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastText = text
	if m.err != nil {
		return nil, m.err
	}
	return m.audioData, nil
}

func (m *mockTTSProviderWS) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockTTSProviderWS) getLastText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastText
}

// TestTTSToolSendsAudio verifies that when the LLM returns a text_to_speech
// tool call, the handler synthesizes audio via the TTS provider and sends a
// tts_audio WebSocket message with base64-encoded WAV data.
func TestTTSToolSendsAudio(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	ttsWAV := []byte("RIFF....WAVEfmt test audio data")

	mockTTS := &mockTTSProviderWS{audioData: ttsWAV}

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{{
				Type: "text_to_speech",
				Text: "Here is a cat!",
			}},
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

	// Verify tts_audio message received
	var ttsMsg map[string]interface{}
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			ttsMsg = msg
			break
		}
	}

	if ttsMsg == nil {
		t.Fatal("expected tts_audio message, got none")
	}

	// Verify base64-encoded audio data
	audioDataB64, ok := ttsMsg["audio_data"].(string)
	if !ok {
		t.Fatal("tts_audio message missing audio_data field")
	}

	decoded, err := base64.StdEncoding.DecodeString(audioDataB64)
	if err != nil {
		t.Fatalf("failed to decode base64 audio: %v", err)
	}
	if string(decoded) != string(ttsWAV) {
		t.Errorf("decoded audio mismatch: got %d bytes, want %d", len(decoded), len(ttsWAV))
	}

	// Verify text field
	if ttsMsg["text"] != "Here is a cat!" {
		t.Errorf("tts_audio text = %q, want %q", ttsMsg["text"], "Here is a cat!")
	}

	// Verify TTS provider was called
	if mockTTS.getCallCount() != 1 {
		t.Errorf("TTS provider called %d times, want 1", mockTTS.getCallCount())
	}
	if mockTTS.getLastText() != "Here is a cat!" {
		t.Errorf("TTS provider text = %q, want %q", mockTTS.getLastText(), "Here is a cat!")
	}
}

// TestTTSThenShowMediaMultiTool verifies that when the LLM returns both
// text_to_speech and show_media tool calls, both are executed in order.
// The client should receive tts_audio first, then media.
func TestTTSThenShowMediaMultiTool(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat and a dog"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	ttsWAV := []byte("RIFF-WAV-audio")
	mockTTS := &mockTTSProviderWS{audioData: ttsWAV}

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{
				{
					Type: "text_to_speech",
					Text: "I can only show one at a time!",
				},
				{
					Type:                  "show_media",
					Subject:               "cat",
					InstructionEndWordIdx: 3,
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

	// Verify ordering: tts_audio should come before error (media lookup fails
	// because no DB, which is fine — we're testing tool execution order).
	ttsIdx := -1
	errorIdx := -1
	for i, msg := range messages {
		switch msg["type"] {
		case "tts_audio":
			ttsIdx = i
		case "error":
			// show_media with nil DB produces "media lookup unavailable"
			errMsg, _ := msg["message"].(string)
			if strings.Contains(errMsg, "media") || strings.Contains(errMsg, "unavailable") {
				errorIdx = i
			}
		}
	}

	if ttsIdx == -1 {
		t.Fatal("expected tts_audio message")
	}
	if errorIdx == -1 {
		t.Fatal("expected error message from show_media (nil DB)")
	}
	if ttsIdx >= errorIdx {
		t.Errorf("tts_audio (index %d) should come before show_media error (index %d)", ttsIdx, errorIdx)
	}

	// Verify TTS was called with the right text
	if mockTTS.getLastText() != "I can only show one at a time!" {
		t.Errorf("TTS text = %q, want %q", mockTTS.getLastText(), "I can only show one at a time!")
	}
}

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
