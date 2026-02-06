package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// WebSocket tests for the framed audio protocol (12-byte header).

// buildFramedChunk constructs a binary message with the 12-byte audio frame header.
func buildFramedChunk(seq uint16, clientTS float64, audioData []byte) []byte {
	frame := make([]byte, AudioFrameHeaderSize+len(audioData))
	binary.BigEndian.PutUint16(frame[0:2], AudioFrameMagic)
	binary.BigEndian.PutUint16(frame[2:4], seq)
	binary.BigEndian.PutUint64(frame[4:12], math.Float64bits(clientTS))
	copy(frame[AudioFrameHeaderSize:], audioData)
	return frame
}

// TestFramedAudioChunkParsing verifies that the backend correctly parses
// the 12-byte frame header, stripping it from the audio data while
// recording chunk metadata (sequence number, client timestamp).
func TestFramedAudioChunkParsing(t *testing.T) {
	// Mock whisper returns a response with word-level timing
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

	// Send start_recording with client_time
	clientStartTime := 1738764000000.0
	startMsg := map[string]interface{}{
		"type":        "start_recording",
		"client_time": clientStartTime,
	}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	// Send framed audio chunks with timestamps
	// Create enough data to pass the minimum audio size (1KB)
	audioPayload := make([]byte, 2000)
	chunk := buildFramedChunk(0, clientStartTime+100, audioPayload)
	if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
		t.Fatalf("failed to send framed chunk: %v", err)
	}

	// Stop recording
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Read the transcript response
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var transcriptMsg TranscriptMessage
	if err := json.Unmarshal(respData, &transcriptMsg); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	if transcriptMsg.Type != MsgTypeTranscript {
		t.Errorf("expected transcript message, got %s", transcriptMsg.Type)
	}

	if transcriptMsg.Text != "show me a cat" {
		t.Errorf("expected 'show me a cat', got %q", transcriptMsg.Text)
	}

	// Verify audio_start_time is the client timestamp from the first chunk
	if transcriptMsg.AudioStartTime != clientStartTime+100 {
		t.Errorf("expected audio_start_time=%v, got %v",
			clientStartTime+100, transcriptMsg.AudioStartTime)
	}

	// Verify segments with word-level timing are present
	if len(transcriptMsg.Segments) == 0 {
		t.Fatal("expected segments in transcript")
	}
	if len(transcriptMsg.Segments[0].Words) != 4 {
		t.Errorf("expected 4 words, got %d", len(transcriptMsg.Segments[0].Words))
	}
}

// TestLegacyRawAudioStillWorks verifies that sending raw binary without
// the frame header still works (backward compatibility).
func TestLegacyRawAudioStillWorks(t *testing.T) {
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{Text: "hello world"}
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

	// Start recording (no client_time — legacy)
	startMsg := ClientMessage{Type: MsgTypeStartRecording}
	data, _ := json.Marshal(startMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send start_recording: %v", err)
	}

	// Send raw binary (no frame header — legacy format)
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

	// Read transcript
	_, respData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var transcriptMsg TranscriptMessage
	if err := json.Unmarshal(respData, &transcriptMsg); err != nil {
		t.Fatalf("failed to unmarshal transcript: %v", err)
	}

	if transcriptMsg.Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", transcriptMsg.Text)
	}

	// audio_start_time should be 0 (no framing data available)
	if transcriptMsg.AudioStartTime != 0 {
		t.Errorf("expected audio_start_time=0 for legacy, got %v", transcriptMsg.AudioStartTime)
	}
}

// TestFramedChunkStripsHeader verifies that the 12-byte header is stripped
// from the audio data before it reaches the WebM parser / whisper.
func TestFramedChunkStripsHeader(t *testing.T) {
	// Track what whisper actually receives
	var receivedSize int
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "bad", 400)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", 400)
			return
		}
		defer file.Close()
		buf := make([]byte, 100000)
		n, _ := file.Read(buf)
		receivedSize = n

		resp := WhisperResponse{Text: "test"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	handler := NewAudioWebSocketHandler(mockWhisper.URL, &mockLLMProvider{subject: "cat"}, nil)

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

	// Send a framed chunk with 2000 bytes of audio data
	audioPayload := make([]byte, 2000)
	for i := range audioPayload {
		audioPayload[i] = byte(i % 256)
	}
	chunk := buildFramedChunk(0, 1738764000000, audioPayload)
	// Total frame size = 12 + 2000 = 2012
	if len(chunk) != AudioFrameHeaderSize+2000 {
		t.Fatalf("unexpected chunk size: %d", len(chunk))
	}

	if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
		t.Fatalf("failed to send chunk: %v", err)
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

	// Wait for whisper request to complete
	time.Sleep(500 * time.Millisecond)

	// The audio sent to whisper should be exactly 2000 bytes (header stripped)
	if receivedSize != 2000 {
		t.Errorf("expected whisper to receive 2000 bytes of audio, got %d "+
			"(header may not have been stripped)", receivedSize)
	}
}

// TestFrameHeaderConstants verifies the protocol constants match the spec.
func TestFrameHeaderConstants(t *testing.T) {
	if AudioFrameMagic != 0xAB01 {
		t.Errorf("AudioFrameMagic should be 0xAB01, got 0x%04X", AudioFrameMagic)
	}
	if AudioFrameHeaderSize != 12 {
		t.Errorf("AudioFrameHeaderSize should be 12, got %d", AudioFrameHeaderSize)
	}
}
