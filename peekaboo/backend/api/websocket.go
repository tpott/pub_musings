// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// WebSocket message types from client
const (
	MsgTypeStartRecording = "start_recording"
	MsgTypeStopRecording  = "stop_recording"
	MsgTypePing           = "ping"
)

// WebSocket message types to client
const (
	MsgTypeTranscript = "transcript"
	MsgTypeMedia      = "media"
	MsgTypeError      = "error"
	MsgTypePong       = "pong"
)

// ClientMessage represents a control message from the client.
type ClientMessage struct {
	Type string `json:"type"`
}

// TranscriptMessage is sent to client with transcript text.
type TranscriptMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MediaMessage is sent to client with media URLs.
type MediaMessage struct {
	Type     string `json:"type"`
	Subject  string `json:"subject"`
	PhotoURL string `json:"photo_url"`
	AudioURL string `json:"audio_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

// ErrorMessage is sent to client on error.
type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// PongMessage is sent in response to ping.
type PongMessage struct {
	Type string `json:"type"`
}

// AudioWebSocketHandler handles WebSocket connections for audio streaming.
type AudioWebSocketHandler struct {
	WhisperURL    string
	LLMProvider   llm.Provider
	Database      *db.DB
	Client        *http.Client
	RateLimiter   *RateLimiter // Optional - nil means no rate limiting
	AllowedOrigin string       // Optional - "*" or empty means allow all

	// Configuration
	BufferThreshold time.Duration // How long to buffer before processing
	IdleTimeout     time.Duration // Connection idle timeout
	MaxMessageSize  int64         // Max binary message size
}

// NewAudioWebSocketHandler creates a new WebSocket handler.
func NewAudioWebSocketHandler(whisperURL string, provider llm.Provider, database *db.DB) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: 120 * time.Second},
		BufferThreshold: 3 * time.Second,
		IdleTimeout:     5 * time.Minute,
		MaxMessageSize:  5 << 20, // 5MB
	}
}

// NewAudioWebSocketHandlerWithRateLimiter creates a new WebSocket handler with rate limiting.
// The rate limiter checks connections per IP before upgrading to WebSocket.
func NewAudioWebSocketHandlerWithRateLimiter(whisperURL string, provider llm.Provider, database *db.DB, rateLimiter *RateLimiter) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: 120 * time.Second},
		RateLimiter:     rateLimiter,
		BufferThreshold: 3 * time.Second,
		IdleTimeout:     5 * time.Minute,
		MaxMessageSize:  5 << 20, // 5MB
	}
}

// NewAudioWebSocketHandlerWithOptions creates a WebSocket handler with all options.
// allowedOrigin: "*" or "" means allow all; specific origin (e.g., "https://example.com") restricts to that origin.
func NewAudioWebSocketHandlerWithOptions(whisperURL string, provider llm.Provider, database *db.DB, rateLimiter *RateLimiter, allowedOrigin string) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: 120 * time.Second},
		RateLimiter:     rateLimiter,
		AllowedOrigin:   allowedOrigin,
		BufferThreshold: 3 * time.Second,
		IdleTimeout:     5 * time.Minute,
		MaxMessageSize:  5 << 20, // 5MB
	}
}

// connectionState manages per-connection state.
type connectionState struct {
	mu           sync.Mutex
	audioBuffer  []byte
	isRecording  bool
	lastActivity time.Time
	bufferStart  time.Time
}

// ServeHTTP upgrades the connection to WebSocket and handles audio streaming.
func (h *AudioWebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := logging.GetRequestID(r.Context())
	logger := slog.With("request_id", requestID, "handler", "websocket")

	// Check rate limit before upgrading to WebSocket
	if h.RateLimiter != nil {
		ip := getClientIP(r)
		if !h.RateLimiter.Allow(ip) {
			logger.Warn("websocket rate limit exceeded", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded, try again later"}`))
			return
		}
	}

	// Accept WebSocket connection with origin validation
	acceptOpts := &websocket.AcceptOptions{}
	if h.AllowedOrigin == "" || h.AllowedOrigin == "*" {
		// Allow any origin (development mode)
		acceptOpts.InsecureSkipVerify = true
	} else {
		// Validate origin against allowed origin
		acceptOpts.OriginPatterns = []string{h.AllowedOrigin}
	}

	conn, err := websocket.Accept(w, r, acceptOpts)
	if err != nil {
		logger.Error("failed to accept websocket", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "connection closed")

	// Set read limit
	conn.SetReadLimit(h.MaxMessageSize)

	logger.Info("websocket connection established")

	// Initialize connection state
	state := &connectionState{
		lastActivity: time.Now(),
	}

	// Create context for this connection
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start idle timeout goroutine
	go h.idleTimeoutWatcher(ctx, cancel, conn, state, logger)

	// Start buffer threshold goroutine
	go h.bufferThresholdWatcher(ctx, conn, state, logger, h)

	// Message read loop
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				logger.Debug("client closed connection normally")
			} else {
				logger.Debug("websocket read error", "error", err)
			}
			return
		}

		// Update last activity
		state.mu.Lock()
		state.lastActivity = time.Now()
		state.mu.Unlock()

		switch msgType {
		case websocket.MessageBinary:
			// Audio chunk
			h.handleAudioChunk(ctx, conn, state, data, logger)

		case websocket.MessageText:
			// Control message
			h.handleControlMessage(ctx, conn, state, data, logger)
		}
	}
}

// handleAudioChunk buffers incoming audio data.
func (h *AudioWebSocketHandler) handleAudioChunk(ctx context.Context, conn *websocket.Conn, state *connectionState, data []byte, logger *slog.Logger) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.isRecording {
		// Ignore audio when not recording
		return
	}

	// Start buffer timer on first chunk
	if len(state.audioBuffer) == 0 {
		state.bufferStart = time.Now()
	}

	// Append to buffer
	state.audioBuffer = append(state.audioBuffer, data...)

	logger.Debug("received audio chunk", "size", len(data), "buffer_size", len(state.audioBuffer))
}

// handleControlMessage processes JSON control messages.
func (h *AudioWebSocketHandler) handleControlMessage(ctx context.Context, conn *websocket.Conn, state *connectionState, data []byte, logger *slog.Logger) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Warn("invalid control message", "error", err)
		h.sendError(ctx, conn, "invalid message format")
		return
	}

	switch msg.Type {
	case MsgTypeStartRecording:
		state.mu.Lock()
		state.isRecording = true
		state.audioBuffer = nil // Clear previous buffer
		state.bufferStart = time.Time{}
		state.mu.Unlock()
		logger.Debug("recording started")

	case MsgTypeStopRecording:
		state.mu.Lock()
		state.isRecording = false
		audioData := state.audioBuffer
		state.audioBuffer = nil
		state.mu.Unlock()

		logger.Debug("recording stopped", "buffer_size", len(audioData))

		// Process the buffered audio
		if len(audioData) >= minAudioSize {
			go h.processAudio(ctx, conn, audioData, logger)
		} else if len(audioData) > 0 {
			h.sendError(ctx, conn, "audio too short")
		}

	case MsgTypePing:
		h.sendPong(ctx, conn)
	}
}

// processAudio transcribes audio and extracts intent.
func (h *AudioWebSocketHandler) processAudio(ctx context.Context, conn *websocket.Conn, audioData []byte, logger *slog.Logger) {
	// 1. Send to whisper for transcription
	transcript, err := h.transcribeAudio(audioData)
	if err != nil {
		logger.Error("transcription failed", "error", err)
		h.sendError(ctx, conn, "transcription failed")
		return
	}

	logger.Info("transcription complete", "text", transcript)

	// Check for empty transcript (silence or no recognizable speech)
	if strings.TrimSpace(transcript) == "" {
		logger.Debug("empty transcript from whisper")
		h.sendError(ctx, conn, "No speech detected. Please try again.")
		return
	}

	// Send transcript to client
	h.sendTranscript(ctx, conn, transcript)

	// 2. Extract intent from transcript
	subject, err := h.extractIntent(ctx, transcript)
	if err != nil {
		logger.Error("intent extraction failed", "error", err)
		h.sendError(ctx, conn, "intent extraction failed")
		return
	}

	logger.Info("intent extracted", "subject", subject)

	// 3. Look up media for subject
	if h.Database == nil {
		logger.Error("database not configured")
		h.sendError(ctx, conn, "media lookup unavailable")
		return
	}
	mediaSet, err := h.Database.GetRandomMediaSet(subject)
	if err != nil {
		logger.Warn("media lookup failed", "subject", subject, "error", err)
		h.sendError(ctx, conn, fmt.Sprintf("no media found for %s", subject))
		return
	}

	// 4. Send media to client
	h.sendMedia(ctx, conn, subject, mediaSet)
}

// transcribeAudio sends audio to whisper-server.
func (h *AudioWebSocketHandler) transcribeAudio(audioData []byte) (string, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(audioData)); err != nil {
		return "", fmt.Errorf("copy audio: %w", err)
	}

	// Add required fields
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return "", fmt.Errorf("write response_format: %w", err)
	}
	if err := writer.WriteField("temperature", "0"); err != nil {
		return "", fmt.Errorf("write temperature: %w", err)
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return "", fmt.Errorf("write language: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close writer: %w", err)
	}

	// Send request to whisper-server
	req, err := http.NewRequest(http.MethodPost, h.WhisperURL+"/inference", &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper-server error: %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var whisperResp WhisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return whisperResp.Text, nil
}

// extractIntent uses LLM to extract subject from transcript.
func (h *AudioWebSocketHandler) extractIntent(ctx context.Context, transcript string) (string, error) {
	result, err := h.LLMProvider.ExtractIntent(ctx, transcript)
	if err != nil {
		return "", err
	}
	return result.Subject, nil
}

// Helper functions to send messages

func (h *AudioWebSocketHandler) sendTranscript(ctx context.Context, conn *websocket.Conn, text string) {
	msg := TranscriptMessage{Type: MsgTypeTranscript, Text: text}
	data, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, data)
}

func (h *AudioWebSocketHandler) sendMedia(ctx context.Context, conn *websocket.Conn, subject string, media *db.MediaSet) {
	msg := MediaMessage{
		Type:     MsgTypeMedia,
		Subject:  subject,
		PhotoURL: "/" + media.PhotoPath,
	}
	if media.AudioPath != "" {
		msg.AudioURL = "/" + media.AudioPath
	}
	if media.VideoPath != "" {
		msg.VideoURL = "/" + media.VideoPath
	}
	data, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, data)
}

func (h *AudioWebSocketHandler) sendError(ctx context.Context, conn *websocket.Conn, message string) {
	msg := ErrorMessage{Type: MsgTypeError, Message: message}
	data, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, data)
}

func (h *AudioWebSocketHandler) sendPong(ctx context.Context, conn *websocket.Conn) {
	msg := PongMessage{Type: MsgTypePong}
	data, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, data)
}

// idleTimeoutWatcher closes connection after idle timeout.
func (h *AudioWebSocketHandler) idleTimeoutWatcher(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, state *connectionState, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state.mu.Lock()
			idle := time.Since(state.lastActivity)
			state.mu.Unlock()

			if idle > h.IdleTimeout {
				logger.Info("closing idle connection", "idle_duration", idle)
				conn.Close(websocket.StatusGoingAway, "idle timeout")
				cancel()
				return
			}
		}
	}
}

// bufferThresholdWatcher processes audio when buffer threshold is reached.
func (h *AudioWebSocketHandler) bufferThresholdWatcher(ctx context.Context, conn *websocket.Conn, state *connectionState, logger *slog.Logger, handler *AudioWebSocketHandler) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state.mu.Lock()
			if !state.isRecording || len(state.audioBuffer) == 0 {
				state.mu.Unlock()
				continue
			}

			elapsed := time.Since(state.bufferStart)
			if elapsed >= h.BufferThreshold {
				// Threshold reached, process the audio
				audioData := state.audioBuffer
				state.audioBuffer = nil
				state.bufferStart = time.Time{}
				state.mu.Unlock()

				logger.Debug("buffer threshold reached", "elapsed", elapsed, "size", len(audioData))

				if len(audioData) >= minAudioSize {
					go handler.processAudio(ctx, conn, audioData, logger)
				}
			} else {
				state.mu.Unlock()
			}
		}
	}
}
