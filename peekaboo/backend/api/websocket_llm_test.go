package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// WebSocket tests for LLM transcript processing integration.

// capturingMockLLMProvider captures the TranscriptRequest passed to ProcessTranscript.
type capturingMockLLMProvider struct {
	mu      sync.Mutex
	subject string
	result  *llm.TranscriptResult // custom result; if nil, defaults to show_media
	err     error

	// Captured values from the last call
	lastRequest *llm.TranscriptRequest
}

func (m *capturingMockLLMProvider) ExtractIntent(ctx context.Context, text string) (*llm.IntentResult, error) {
	return &llm.IntentResult{Subject: m.subject}, nil
}

func (m *capturingMockLLMProvider) ProcessTranscript(ctx context.Context, req llm.TranscriptRequest) (*llm.TranscriptResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqCopy := req
	m.lastRequest = &reqCopy
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &llm.TranscriptResult{
		Actions: []llm.ToolAction{{
			Type:    "show_media",
			Subject: m.subject,
		}},
	}, nil
}

func (m *capturingMockLLMProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *capturingMockLLMProvider) getLastRequest() *llm.TranscriptRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastRequest
}

// TestProcessTranscriptReceivesWordData verifies that word-level timing data
// from whisper's verbose_json is forwarded to the LLM's ProcessTranscript method.
func TestProcessTranscriptReceivesWordData(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Text: "show me a cat",
			Segments: []WhisperSegment{{
				ID:    0,
				Start: 0.0,
				End:   2.5,
				Text:  " show me a cat",
				Words: []WhisperWord{
					{Word: " show", Start: 0.0, End: 0.52, Probability: 0.95},
					{Word: " me", Start: 0.52, End: 0.76, Probability: 0.98},
					{Word: " a", Start: 0.76, End: 0.92, Probability: 0.97},
					{Word: " cat", Start: 0.92, End: 1.30, Probability: 0.99},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &capturingMockLLMProvider{subject: "cat"}
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

	// Drain responses
	drainCtx, drainCancel := context.WithTimeout(ctx, 3*time.Second)
	defer drainCancel()
	for {
		_, _, err := conn.Read(drainCtx)
		if err != nil {
			break
		}
	}

	// Wait for processAudio goroutine to finish
	time.Sleep(500 * time.Millisecond)

	// Verify ProcessTranscript received word data
	req := mockProvider.getLastRequest()
	if req == nil {
		t.Fatal("ProcessTranscript was not called")
	}

	if req.Text != "show me a cat" {
		t.Errorf("expected text 'show me a cat', got %q", req.Text)
	}

	if len(req.Words) != 4 {
		t.Fatalf("expected 4 words, got %d", len(req.Words))
	}

	// Verify word data fidelity
	if req.Words[0].Word != " show" {
		t.Errorf("expected word ' show', got %q", req.Words[0].Word)
	}
	if req.Words[0].Start != 0.0 || req.Words[0].End != 0.52 {
		t.Errorf("expected start=0.0 end=0.52, got start=%v end=%v", req.Words[0].Start, req.Words[0].End)
	}
	if req.Words[3].Word != " cat" {
		t.Errorf("expected word ' cat', got %q", req.Words[3].Word)
	}
	if req.Words[3].Probability != 0.99 {
		t.Errorf("expected probability 0.99, got %v", req.Words[3].Probability)
	}
}

// TestProcessTranscriptReceivesConcepts verifies that available concept IDs
// from the database are forwarded to the LLM's ProcessTranscript method.
func TestProcessTranscriptReceivesConcepts(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// Set up in-memory database with concepts
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()
	if err := database.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	mockProvider := &capturingMockLLMProvider{subject: "cat"}
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

	// Drain responses (transcript + media)
	drainCtx, drainCancel := context.WithTimeout(ctx, 3*time.Second)
	defer drainCancel()
	for {
		_, _, err := conn.Read(drainCtx)
		if err != nil {
			break
		}
	}

	time.Sleep(500 * time.Millisecond)

	req := mockProvider.getLastRequest()
	if req == nil {
		t.Fatal("ProcessTranscript was not called")
	}

	// DB seeds 6 concepts: cat, chicken, cow, dog, duck, pig (sorted)
	if len(req.Concepts) != 6 {
		t.Fatalf("expected 6 concepts, got %d: %v", len(req.Concepts), req.Concepts)
	}

	expected := []string{"cat", "chicken", "cow", "dog", "duck", "pig"}
	for i, want := range expected {
		if req.Concepts[i] != want {
			t.Errorf("concept[%d] = %q, want %q", i, req.Concepts[i], want)
		}
	}
}

// TestWaitForMoreSkipsMediaLookup verifies that when the LLM returns a
// wait_for_more action, no media lookup or media message is sent.
func TestWaitForMoreSkipsMediaLookup(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "show me"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &capturingMockLLMProvider{
		result: &llm.TranscriptResult{
			Actions: []llm.ToolAction{{
				Type:   "wait_for_more",
				Reason: "transcript appears incomplete",
			}},
		},
	}
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

	// Collect all messages (should only get transcript, no media)
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

	// Should have exactly one message: transcript
	// No media message because wait_for_more was returned
	for _, msg := range messages {
		if msg["type"] == "media" {
			t.Error("should not have received media message with wait_for_more action")
		}
	}

	// Should have at least the transcript message
	hasTranscript := false
	for _, msg := range messages {
		if msg["type"] == "transcript" {
			hasTranscript = true
		}
	}
	if !hasTranscript {
		t.Error("expected transcript message")
	}
}

// TestProcessTranscriptNoWordsFromWhisper verifies graceful handling when
// whisper returns no word-level data (empty segments or no words).
func TestProcessTranscriptNoWordsFromWhisper(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return transcript without word data
		resp := WhisperResponse{Text: "show me a dog"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &capturingMockLLMProvider{subject: "dog"}
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

	// Drain responses
	drainCtx, drainCancel := context.WithTimeout(ctx, 3*time.Second)
	defer drainCancel()
	for {
		_, _, err := conn.Read(drainCtx)
		if err != nil {
			break
		}
	}

	time.Sleep(500 * time.Millisecond)

	req := mockProvider.getLastRequest()
	if req == nil {
		t.Fatal("ProcessTranscript was not called")
	}

	if req.Text != "show me a dog" {
		t.Errorf("expected text 'show me a dog', got %q", req.Text)
	}

	// Words should be empty (not nil necessarily, just length 0)
	if len(req.Words) != 0 {
		t.Errorf("expected 0 words when whisper returns no segments, got %d", len(req.Words))
	}
}
