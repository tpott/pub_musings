package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// WebSocket tests for WebM container validity across buffer splits.

// ebmlMagic is the EBML header element ID that starts every valid WebM file.
var ebmlMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}

// TestBufferSplitProducesValidWebM proves the WebM container corruption bug.
//
// When the backend splits the audio buffer at the 3-second threshold,
// the first buffer is a valid WebM (contains the EBML header from chunk 1),
// but subsequent buffers are INVALID because they start mid-Cluster
// without the required EBML header + Track info (init segment).
//
// This test reads a real WebM fixture, sends it as 1KB chunks with 100ms
// delays, and asserts that EVERY whisper request receives audio starting
// with the EBML magic bytes 0x1A45DFA3.
//
// Expected result: FAILS. The first request passes (has EBML header),
// but request 2+ fail (missing EBML header = corrupted WebM container).
func TestBufferSplitProducesValidWebM(t *testing.T) {
	// KNOWN BUG: This test proves WebM container corruption on buffer split.
	// It will be un-skipped when task 137 implements EBML parsing to fix the bug.
	// Run with: go test -v -run TestBufferSplitProducesValidWebM ./api/
	t.Skip("Known bug: WebM container corruption on buffer split (task 137 will fix)")

	// Read the real WebM fixture
	fixtureData, err := os.ReadFile("../../tests/fixtures/me-show-me-a-cat.webm")
	if err != nil {
		t.Fatalf("failed to read WebM fixture: %v", err)
	}

	if len(fixtureData) < 4 {
		t.Fatal("fixture file too small")
	}

	// Verify fixture starts with EBML magic
	if fixtureData[0] != ebmlMagic[0] || fixtureData[1] != ebmlMagic[1] ||
		fixtureData[2] != ebmlMagic[2] || fixtureData[3] != ebmlMagic[3] {
		t.Fatalf("fixture does not start with EBML magic: got %02x %02x %02x %02x",
			fixtureData[0], fixtureData[1], fixtureData[2], fixtureData[3])
	}

	// Mock whisper server that captures every request's audio bytes
	var mu sync.Mutex
	var capturedAudio [][]byte

	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form to extract the audio file
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Logf("whisper mock: failed to parse form: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Logf("whisper mock: no file field: %v", err)
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		audioBytes, err := io.ReadAll(file)
		if err != nil {
			t.Logf("whisper mock: failed to read file: %v", err)
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}

		mu.Lock()
		capturedAudio = append(capturedAudio, audioBytes)
		mu.Unlock()

		// Return a valid whisper response
		resp := WhisperResponse{Text: "show me a cat"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	mockProvider := &mockLLMProvider{subject: "cat"}
	handler := NewAudioWebSocketHandler(mockWhisper.URL, mockProvider, nil)
	handler.BufferThreshold = 500 * time.Millisecond // Short threshold to trigger multiple buffer grabs

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// Send WebM data as 1KB binary chunks with 100ms delays
	chunkSize := 1024
	for offset := 0; offset < len(fixtureData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fixtureData) {
			end = len(fixtureData)
		}
		chunk := fixtureData[offset:end]

		if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
			t.Fatalf("failed to send chunk at offset %d: %v", offset, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Stop recording to flush any remaining buffer
	stopMsg := ClientMessage{Type: MsgTypeStopRecording}
	data, _ = json.Marshal(stopMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send stop_recording: %v", err)
	}

	// Drain all WebSocket messages (transcripts, errors, media lookups)
	// until we time out or connection closes
	drainCtx, drainCancel := context.WithTimeout(ctx, 5*time.Second)
	defer drainCancel()
	for {
		_, _, err := conn.Read(drainCtx)
		if err != nil {
			break
		}
	}

	// Wait briefly for any in-flight whisper requests to complete
	time.Sleep(500 * time.Millisecond)

	// Verify we got at least 2 whisper requests
	mu.Lock()
	numRequests := len(capturedAudio)
	mu.Unlock()

	if numRequests < 2 {
		t.Fatalf("expected at least 2 whisper requests, got %d (buffer threshold may not have triggered multiple splits)", numRequests)
	}

	t.Logf("captured %d whisper requests", numRequests)

	// Assert: EVERY request's audio starts with EBML magic 0x1A45DFA3
	mu.Lock()
	defer mu.Unlock()
	for i, audio := range capturedAudio {
		t.Logf("request %d: %d bytes, first 4 bytes: %02x %02x %02x %02x",
			i, len(audio), safeByteAt(audio, 0), safeByteAt(audio, 1),
			safeByteAt(audio, 2), safeByteAt(audio, 3))

		if len(audio) < 4 {
			t.Errorf("request %d: audio too short (%d bytes), cannot check EBML magic", i, len(audio))
			continue
		}

		if audio[0] != ebmlMagic[0] || audio[1] != ebmlMagic[1] ||
			audio[2] != ebmlMagic[2] || audio[3] != ebmlMagic[3] {
			t.Errorf("request %d: audio does NOT start with EBML magic 0x1A45DFA3, got 0x%02X%02X%02X%02X — WebM container is CORRUPTED",
				i, audio[0], audio[1], audio[2], audio[3])
		}
	}
}

// safeByteAt returns the byte at position i, or 0x00 if out of bounds.
func safeByteAt(data []byte, i int) byte {
	if i < len(data) {
		return data[i]
	}
	return 0x00
}
