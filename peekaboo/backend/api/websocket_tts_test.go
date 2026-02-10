package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
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

// TestShowMediaWithoutTTS verifies that when the LLM returns only show_media
// (after the parser strips TTS), no TTS audio is played.
func TestShowMediaWithoutTTS(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat"}
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

	// Verify no tts_audio message was sent
	for _, msg := range messages {
		if msg["type"] == "tts_audio" {
			t.Error("unexpected tts_audio message — TTS should not play when showing media")
		}
	}

	// Verify TTS provider was NOT called
	if mockTTS.getCallCount() != 0 {
		t.Errorf("TTS provider called %d times, want 0", mockTTS.getCallCount())
	}
}
