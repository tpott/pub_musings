// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
	"github.com/tpott/pub_musings/peekaboo/backend/tts"
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
	MsgTypeTTSAudio   = "tts_audio"
	MsgTypeError      = "error"
	MsgTypePong       = "pong"
)

// WebSocket handler configuration defaults
const (
	// defaultBufferThreshold is how long to accumulate audio before auto-processing.
	defaultBufferThreshold = 3 * time.Second

	// bufferCheckInterval is how often the buffer threshold watcher ticks.
	bufferCheckInterval = 500 * time.Millisecond

	// idleCheckInterval is how often the idle timeout watcher ticks.
	idleCheckInterval = 30 * time.Second

	// intentTimeout is the context timeout for LLM intent extraction.
	intentTimeout = 30 * time.Second

	// whisperClientTimeout is the HTTP client timeout for whisper-server requests.
	whisperClientTimeout = 120 * time.Second

	// defaultMaxMessageSize is the maximum WebSocket binary message size (5MB).
	defaultMaxMessageSize = 5 << 20
)

// Audio frame header constants.
const (
	// AudioFrameMagic is the 2-byte magic prefix for framed audio messages (0xAB01).
	AudioFrameMagic = 0xAB01

	// AudioFrameHeaderSize is the size of the audio frame header in bytes:
	// 2 (magic) + 2 (sequence number) + 8 (float64 timestamp) = 12 bytes.
	AudioFrameHeaderSize = 12
)

// ClientMessage represents a control message from the client.
type ClientMessage struct {
	Type       string  `json:"type"`
	ClientTime float64 `json:"client_time,omitempty"` // milliseconds since epoch, sent with start_recording
}

// AudioChunkMeta records metadata from a framed audio chunk's header.
type AudioChunkMeta struct {
	Seq      uint16  // sequence number from the frame header
	ClientTS float64 // client Date.now() in milliseconds from the frame header
	ServerTS time.Time
	Offset   int // byte offset of this chunk's audio data in the raw buffer
}

// TranscriptMessage is sent to client with transcript text and optional
// word-level timing data from whisper's verbose_json response.
type TranscriptMessage struct {
	Type           string           `json:"type"`
	Text           string           `json:"text"`
	Segments       []WhisperSegment `json:"segments,omitempty"`
	AudioStartTime float64          `json:"audio_start_time,omitempty"` // client wall-clock ms of first audio sample
}

// MediaMessage is sent to client with media URLs.
type MediaMessage struct {
	Type     string `json:"type"`
	Subject  string `json:"subject"`
	PhotoURL string `json:"photo_url"`
	AudioURL string `json:"audio_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
}

// TTSAudioMessage is sent to client with base64-encoded TTS audio.
type TTSAudioMessage struct {
	Type      string `json:"type"`
	AudioData string `json:"audio_data"` // base64-encoded WAV audio
	Text      string `json:"text"`       // the text that was spoken
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

// ConnectionTracker tracks the number of active WebSocket connections.
type ConnectionTracker struct {
	count atomic.Int32
	max   int32
}

// NewConnectionTracker creates a new connection tracker with the given max limit.
func NewConnectionTracker(maxConnections int) *ConnectionTracker {
	return &ConnectionTracker{
		max: int32(maxConnections),
	}
}

// TryAcquire attempts to acquire a connection slot.
// Returns true if successful, false if at capacity.
func (ct *ConnectionTracker) TryAcquire() bool {
	for {
		current := ct.count.Load()
		if current >= ct.max {
			return false
		}
		if ct.count.CompareAndSwap(current, current+1) {
			return true
		}
		// CAS failed, another goroutine changed the value; retry
	}
}

// Release releases a connection slot.
func (ct *ConnectionTracker) Release() {
	ct.count.Add(-1)
}

// Count returns the current number of active connections.
func (ct *ConnectionTracker) Count() int {
	return int(ct.count.Load())
}

// Max returns the maximum number of allowed connections.
func (ct *ConnectionTracker) Max() int {
	return int(ct.max)
}

// AudioWebSocketHandler handles WebSocket connections for audio streaming.
type AudioWebSocketHandler struct {
	WhisperURL    string
	LLMProvider   llm.Provider
	TTSProvider   tts.Provider // Optional - nil means TTS disabled
	Database      *db.DB
	Client        *http.Client
	RateLimiter   *RateLimiter       // Optional - nil means no rate limiting
	AllowedOrigin string             // Optional - "*" or empty means allow all
	ConnTracker   *ConnectionTracker // Optional - nil means no connection limit

	// Configuration
	BufferThreshold time.Duration // How long to buffer before processing
	IdleTimeout     time.Duration // Connection idle timeout
	MaxMessageSize  int64         // Max binary message size
}

// getIdleTimeout returns the WebSocket idle timeout from WEBSOCKET_IDLE_TIMEOUT_SECS env var.
// Defaults to 300 seconds (5 minutes) if not set or invalid.
func getIdleTimeout() time.Duration {
	val := os.Getenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
	if val == "" {
		return 5 * time.Minute
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(secs) * time.Second
}

// defaultMaxConnections is the default maximum number of concurrent WebSocket connections.
const defaultMaxConnections = 100

// getMaxConnections returns the max WebSocket connections from WEBSOCKET_MAX_CONNECTIONS env var.
// Defaults to 100 if not set or invalid.
func getMaxConnections() int {
	val := os.Getenv("WEBSOCKET_MAX_CONNECTIONS")
	if val == "" {
		return defaultMaxConnections
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultMaxConnections
	}
	return n
}

// NewAudioWebSocketHandler creates a new WebSocket handler.
func NewAudioWebSocketHandler(whisperURL string, provider llm.Provider, database *db.DB) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: whisperClientTimeout},
		BufferThreshold: defaultBufferThreshold,
		IdleTimeout:     getIdleTimeout(),
		MaxMessageSize:  defaultMaxMessageSize,
	}
}

// NewAudioWebSocketHandlerWithRateLimiter creates a new WebSocket handler with rate limiting.
// The rate limiter checks connections per IP before upgrading to WebSocket.
func NewAudioWebSocketHandlerWithRateLimiter(whisperURL string, provider llm.Provider, database *db.DB, rateLimiter *RateLimiter) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: whisperClientTimeout},
		RateLimiter:     rateLimiter,
		BufferThreshold: defaultBufferThreshold,
		IdleTimeout:     getIdleTimeout(),
		MaxMessageSize:  defaultMaxMessageSize,
	}
}

// NewAudioWebSocketHandlerWithOptions creates a WebSocket handler with all options.
// allowedOrigin: "*" or "" means allow all; specific origin (e.g., "https://example.com") restricts to that origin.
// IdleTimeout is read from WEBSOCKET_IDLE_TIMEOUT_SECS env var (default: 300 seconds).
// MaxConnections is read from WEBSOCKET_MAX_CONNECTIONS env var (default: 100).
func NewAudioWebSocketHandlerWithOptions(whisperURL string, provider llm.Provider, database *db.DB, rateLimiter *RateLimiter, allowedOrigin string) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: whisperClientTimeout},
		RateLimiter:     rateLimiter,
		AllowedOrigin:   allowedOrigin,
		ConnTracker:     NewConnectionTracker(getMaxConnections()),
		BufferThreshold: defaultBufferThreshold,
		IdleTimeout:     getIdleTimeout(),
		MaxMessageSize:  defaultMaxMessageSize,
	}
}

// NewAudioWebSocketHandlerWithConnTracker creates a WebSocket handler with a custom connection tracker.
// This is useful for testing or sharing a tracker across multiple handlers.
func NewAudioWebSocketHandlerWithConnTracker(whisperURL string, provider llm.Provider, database *db.DB, rateLimiter *RateLimiter, allowedOrigin string, connTracker *ConnectionTracker) *AudioWebSocketHandler {
	return &AudioWebSocketHandler{
		WhisperURL:      whisperURL,
		LLMProvider:     provider,
		Database:        database,
		Client:          &http.Client{Timeout: whisperClientTimeout},
		RateLimiter:     rateLimiter,
		AllowedOrigin:   allowedOrigin,
		ConnTracker:     connTracker,
		BufferThreshold: defaultBufferThreshold,
		IdleTimeout:     getIdleTimeout(),
		MaxMessageSize:  defaultMaxMessageSize,
	}
}

// connectionState manages per-connection state.
type connectionState struct {
	mu           sync.Mutex
	webmParser   *WebMParser // WebM-aware audio buffer with init segment caching
	isRecording  bool
	isProcessing bool // true while processAudio is running (prevents overlapping calls)
	lastActivity time.Time
	bufferStart  time.Time

	// Framed audio protocol metadata
	chunkMetas       []AudioChunkMeta // metadata for each received audio chunk
	firstChunkClient float64          // client timestamp (ms) of the first chunk in current buffer window

	// Transcript accumulation across buffer cycles.
	// When the LLM returns wait_for_more, accumulated words carry forward
	// to the next transcription cycle so the LLM sees the full context.
	accumulatedWords []llm.WordData
}

// ServeHTTP upgrades the connection to WebSocket and handles audio streaming.
func (h *AudioWebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := logging.GetRequestID(r.Context())
	clientIP := getClientIP(r)
	logger := slog.With("request_id", requestID, "handler", "websocket", "client_ip", clientIP)

	// Check rate limit before upgrading to WebSocket
	if h.RateLimiter != nil {
		ip := getClientIP(r)
		if !h.RateLimiter.Allow(ip) {
			logger.Warn("websocket rate limit exceeded", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error":"rate limit exceeded, try again later"}`)); err != nil {
				logger.Debug("failed to write rate limit response", "error", err)
			}
			return
		}
	}

	// Check connection limit before upgrading to WebSocket
	if h.ConnTracker != nil {
		if !h.ConnTracker.TryAcquire() {
			logger.Warn("websocket connection limit reached", "current", h.ConnTracker.Count(), "max", h.ConnTracker.Max())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte(`{"error":"server at capacity, try again later"}`)); err != nil {
				logger.Debug("failed to write capacity response", "error", err)
			}
			return
		}
		// Release the connection slot when done
		defer h.ConnTracker.Release()
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
		webmParser:   NewWebMParser(),
		lastActivity: time.Now(),
	}

	// Create context for this connection
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start idle timeout goroutine
	go h.idleTimeoutWatcher(ctx, cancel, conn, state, logger)

	// Start buffer threshold goroutine
	go h.bufferThresholdWatcher(ctx, conn, state, logger)

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
// Supports both framed (12-byte header with magic 0xAB01) and legacy raw binary.
func (h *AudioWebSocketHandler) handleAudioChunk(ctx context.Context, conn *websocket.Conn, state *connectionState, data []byte, logger *slog.Logger) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.isRecording {
		// Ignore audio when not recording
		return
	}

	audioData := data
	var chunkMeta *AudioChunkMeta

	// Check for framed audio protocol (12-byte header with magic 0xAB01)
	if len(data) >= AudioFrameHeaderSize {
		magic := uint16(data[0])<<8 | uint16(data[1])
		if magic == AudioFrameMagic {
			seq := uint16(data[2])<<8 | uint16(data[3])
			clientTS := math.Float64frombits(
				uint64(data[4])<<56 | uint64(data[5])<<48 |
					uint64(data[6])<<40 | uint64(data[7])<<32 |
					uint64(data[8])<<24 | uint64(data[9])<<16 |
					uint64(data[10])<<8 | uint64(data[11]),
			)
			audioData = data[AudioFrameHeaderSize:]
			chunkMeta = &AudioChunkMeta{
				Seq:      seq,
				ClientTS: clientTS,
				ServerTS: time.Now(),
				Offset:   state.webmParser.BufferLen(),
			}
		}
	}

	// Start buffer timer on first chunk
	if state.webmParser.BufferLen() == 0 {
		state.bufferStart = time.Now()
		if chunkMeta != nil {
			state.firstChunkClient = chunkMeta.ClientTS
		}
	}

	// Append audio (without header) to WebM-aware buffer
	state.webmParser.Append(audioData)

	// Record chunk metadata
	if chunkMeta != nil {
		state.chunkMetas = append(state.chunkMetas, *chunkMeta)
	}

	logger.Debug("received audio chunk", "size", len(audioData), "buffer_size", state.webmParser.BufferLen())
}

// handleControlMessage processes JSON control messages.
func (h *AudioWebSocketHandler) handleControlMessage(ctx context.Context, conn *websocket.Conn, state *connectionState, data []byte, logger *slog.Logger) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Warn("invalid control message", "error", err)
		h.sendError(ctx, conn, "invalid message format", logger)
		return
	}

	switch msg.Type {
	case MsgTypeStartRecording:
		state.mu.Lock()
		state.isRecording = true
		state.webmParser.Reset() // Clear previous buffer and init segment
		state.bufferStart = time.Time{}
		state.chunkMetas = nil
		state.firstChunkClient = 0
		state.accumulatedWords = nil
		state.mu.Unlock()
		logger.Debug("recording started")

	case MsgTypeStopRecording:
		state.mu.Lock()
		state.isRecording = false
		audioData := state.webmParser.GrabAudio()
		state.webmParser.Clear()
		firstChunkTS := state.firstChunkClient
		state.accumulatedWords = nil // clear accumulation on stop
		state.mu.Unlock()

		logger.Debug("recording stopped", "buffer_size", len(audioData))

		// Process the buffered audio (no state needed — stop is final)
		if len(audioData) >= minAudioSize {
			go h.processAudio(ctx, conn, state, audioData, firstChunkTS, logger)
		} else if len(audioData) > 0 {
			h.sendError(ctx, conn, "audio too short", logger)
		} else {
			// Empty buffer - user stopped recording without speaking
			h.sendError(ctx, conn, "No audio recorded", logger)
		}

	case MsgTypePing:
		h.sendPong(ctx, conn, logger)
	}
}

// processAudio transcribes audio and processes transcript through LLM.
// state is the connection state used for transcript accumulation and buffer
// trimming. firstChunkClientTS is the client wall-clock timestamp (ms since
// epoch) of the first audio chunk in this buffer.
func (h *AudioWebSocketHandler) processAudio(ctx context.Context, conn *websocket.Conn, state *connectionState, audioData []byte, firstChunkClientTS float64, logger *slog.Logger) {
	defer func() {
		state.mu.Lock()
		state.isProcessing = false
		state.mu.Unlock()
	}()

	// 1. Send to whisper for transcription
	whisperResp, err := h.transcribeAudio(audioData)
	if err != nil {
		logger.Error("transcription failed", "error", err)
		h.sendError(ctx, conn, "transcription failed", logger)
		return
	}

	transcript := whisperResp.Text
	logger.Info("transcription complete", "text", transcript)

	// Check for empty transcript (silence or no recognizable speech)
	if strings.TrimSpace(transcript) == "" {
		logger.Debug("empty transcript from whisper")
		h.sendError(ctx, conn, "No speech detected. Please try again.", logger)
		return
	}

	// Send rich transcript to client (includes word-level timing if available)
	h.sendRichTranscript(ctx, conn, whisperResp, firstChunkClientTS, logger)

	// 2. Collect word data from whisper segments for this buffer
	var currentWords []llm.WordData
	for _, seg := range whisperResp.Segments {
		for _, w := range seg.Words {
			currentWords = append(currentWords, llm.WordData{
				Word:        w.Word,
				Start:       w.Start,
				End:         w.End,
				Probability: w.Probability,
			})
		}
	}

	// 3. Use accumulated words from previous cycles (if any) merged with current.
	// Since we re-transcribe the full buffer each time (GrabAudio returns all
	// cluster data), the current whisper response already covers accumulated audio.
	// We use currentWords directly — accumulation happens at the buffer level.
	words := currentWords

	// 4. Get available concepts from DB
	var concepts []string
	if h.Database != nil {
		concepts, err = h.Database.ListConceptIDs()
		if err != nil {
			logger.Warn("failed to list concepts", "error", err)
		}
	}

	// 5. Process transcript through LLM with tool_choice:any
	result, err := h.processTranscript(ctx, transcript, words, concepts)
	if err != nil {
		logger.Error("transcript processing failed", "error", err)
		h.sendError(ctx, conn, "intent extraction failed", logger)
		return
	}

	// 6. Execute actions from LLM response
	for _, action := range result.Actions {
		switch action.Type {
		case "show_media":
			h.executeShowMedia(ctx, conn, action.Subject, logger)

			// Trim audio buffer at instruction boundary
			state.mu.Lock()
			if action.InstructionEndWordIdx >= 0 && action.InstructionEndWordIdx < len(words) {
				endWord := words[action.InstructionEndWordIdx]
				// Convert word end time (seconds) to cluster timecode (milliseconds)
				trimTimeMs := uint64(endWord.End * 1000)
				trimmed := state.webmParser.TrimBefore(trimTimeMs)
				if trimmed == 0 {
					// TrimBefore couldn't find cluster boundaries — clear buffer
					state.webmParser.Clear()
				}
				logger.Debug("trimmed audio buffer at instruction boundary",
					"word_idx", action.InstructionEndWordIdx,
					"word_end_s", endWord.End,
					"trim_time_ms", trimTimeMs,
					"bytes_trimmed", trimmed)
			} else {
				// No valid word index — clear the entire buffer
				state.webmParser.Clear()
			}
			state.accumulatedWords = nil
			state.bufferStart = time.Now()
			state.mu.Unlock()

		case "text_to_speech":
			h.executeTTS(ctx, conn, action.Text, logger)

		case "wait_for_more":
			logger.Debug("wait_for_more action received", "reason", action.Reason)
			// Keep buffer intact for next transcription cycle.
			// Reset bufferStart so the threshold timer waits for more audio
			// before triggering the next transcription.
			state.mu.Lock()
			state.bufferStart = time.Now()
			state.mu.Unlock()
		}
	}
}

// processTranscript uses LLM to process transcript with word-level data.
// Uses a 30-second timeout consistent with the HTTP endpoint.
func (h *AudioWebSocketHandler) processTranscript(ctx context.Context, text string, words []llm.WordData, concepts []string) (*llm.TranscriptResult, error) {
	if h.LLMProvider == nil {
		return nil, fmt.Errorf("LLM provider not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, intentTimeout)
	defer cancel()

	result, err := h.LLMProvider.ProcessTranscript(ctx, llm.TranscriptRequest{
		Text:     text,
		Words:    words,
		Concepts: concepts,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("LLM returned nil result")
	}
	return result, nil
}

// executeShowMedia validates a subject and sends media to the client.
func (h *AudioWebSocketHandler) executeShowMedia(ctx context.Context, conn *websocket.Conn, subject string, logger *slog.Logger) {
	logger.Info("show_media action", "subject", subject)

	// Validate subject format (same validation as HTTP media endpoint)
	if subject == "" {
		logger.Debug("empty subject from LLM")
		h.sendError(ctx, conn, "I didn't understand what you want to see. Please try again.", logger)
		return
	}
	if !validConceptPattern.MatchString(subject) {
		logger.Debug("invalid subject format", "subject", subject)
		h.sendError(ctx, conn, fmt.Sprintf("I don't have media for '%s'. Try a simple animal name like 'cat' or 'dog'.", subject), logger)
		return
	}
	if len(subject) > maxConceptLength {
		logger.Debug("subject too long", "subject", subject, "length", len(subject))
		h.sendError(ctx, conn, "That's too long! Try a simple animal name like 'cat' or 'dog'.", logger)
		return
	}

	// Look up media for subject
	if h.Database == nil {
		logger.Error("database not configured")
		h.sendError(ctx, conn, "media lookup unavailable", logger)
		return
	}
	mediaSet, err := h.Database.GetRandomMediaSet(subject)
	if err != nil {
		logger.Warn("media lookup failed", "subject", subject, "error", err)
		h.sendError(ctx, conn, fmt.Sprintf("no media found for %s", subject), logger)
		return
	}
	if mediaSet == nil {
		logger.Debug("no media set found", "subject", subject)
		h.sendError(ctx, conn, fmt.Sprintf("no media found for %s", subject), logger)
		return
	}

	// Send media to client
	h.sendMedia(ctx, conn, subject, mediaSet, logger)
}

// executeTTS synthesizes speech from text via the TTS provider and sends
// the resulting WAV audio to the client as a base64-encoded tts_audio message.
// If the TTS provider is not configured or fails, a warning is logged and
// execution continues (TTS is non-critical).
func (h *AudioWebSocketHandler) executeTTS(ctx context.Context, conn *websocket.Conn, text string, logger *slog.Logger) {
	if h.TTSProvider == nil {
		logger.Debug("text_to_speech skipped: TTS provider not configured")
		return
	}
	if text == "" {
		logger.Debug("text_to_speech skipped: empty text")
		return
	}

	logger.Info("text_to_speech action", "text", text)

	ttsCtx, ttsCancel := context.WithTimeout(ctx, intentTimeout)
	defer ttsCancel()

	audioData, err := h.TTSProvider.Synthesize(ttsCtx, text)
	if err != nil {
		logger.Warn("TTS synthesis failed", "error", err, "text", text)
		return
	}

	h.sendTTSAudio(ctx, conn, audioData, text, logger)
}

// transcribeAudio sends audio to whisper-server and returns the full response
// including word-level timing and probabilities.
func (h *AudioWebSocketHandler) transcribeAudio(audioData []byte) (*WhisperResponse, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(audioData)); err != nil {
		return nil, fmt.Errorf("copy audio: %w", err)
	}

	// Add required fields
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("write response_format: %w", err)
	}
	if err := writer.WriteField("temperature", "0"); err != nil {
		return nil, fmt.Errorf("write temperature: %w", err)
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return nil, fmt.Errorf("write language: %w", err)
	}
	if err := writer.WriteField("split_on_word", "true"); err != nil {
		return nil, fmt.Errorf("write split_on_word: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// Send request to whisper-server
	req, err := http.NewRequest(http.MethodPost, h.WhisperURL+"/inference", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("whisper-server error: %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var whisperResp WhisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &whisperResp, nil
}

// Helper functions to send messages
// Note: These log errors but don't return them because:
// 1. Callers can't meaningfully recover from send failures
// 2. The connection is likely closing anyway if writes fail
// The logger parameter should have client IP in its attributes for connection context.

func (h *AudioWebSocketHandler) sendRichTranscript(ctx context.Context, conn *websocket.Conn, whisperResp *WhisperResponse, firstChunkClientTS float64, logger *slog.Logger) {
	msg := TranscriptMessage{
		Type:           MsgTypeTranscript,
		Text:           whisperResp.Text,
		Segments:       whisperResp.Segments,
		AudioStartTime: firstChunkClientTS,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal transcript message", "error", err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		logWriteError(logger, "transcript", err)
	}
}

func (h *AudioWebSocketHandler) sendMedia(ctx context.Context, conn *websocket.Conn, subject string, media *db.MediaSet, logger *slog.Logger) {
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
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal media message", "error", err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		logWriteError(logger, "media", err)
	}
}

func (h *AudioWebSocketHandler) sendTTSAudio(ctx context.Context, conn *websocket.Conn, audioData []byte, text string, logger *slog.Logger) {
	msg := TTSAudioMessage{
		Type:      MsgTypeTTSAudio,
		AudioData: base64.StdEncoding.EncodeToString(audioData),
		Text:      text,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal tts_audio message", "error", err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		logWriteError(logger, "tts_audio", err)
	}
}

func (h *AudioWebSocketHandler) sendError(ctx context.Context, conn *websocket.Conn, message string, logger *slog.Logger) {
	msg := ErrorMessage{Type: MsgTypeError, Message: message}
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal error message", "error", err, "original_message", message)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		logWriteError(logger, "error", err)
	}
}

func (h *AudioWebSocketHandler) sendPong(ctx context.Context, conn *websocket.Conn, logger *slog.Logger) {
	msg := PongMessage{Type: MsgTypePong}
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("failed to marshal pong message", "error", err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		logWriteError(logger, "pong", err)
	}
}

// logWriteError logs WebSocket write errors with appropriate level.
// Connection closed normally is logged at DEBUG, other errors at WARN.
func logWriteError(logger *slog.Logger, msgType string, err error) {
	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		logger.Debug("connection closed while sending message", "message_type", msgType)
	} else {
		logger.Warn("failed to send message", "message_type", msgType, "error", err)
	}
}

// idleTimeoutWatcher closes connection after idle timeout.
func (h *AudioWebSocketHandler) idleTimeoutWatcher(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, state *connectionState, logger *slog.Logger) {
	ticker := time.NewTicker(idleCheckInterval)
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
func (h *AudioWebSocketHandler) bufferThresholdWatcher(ctx context.Context, conn *websocket.Conn, state *connectionState, logger *slog.Logger) {
	ticker := time.NewTicker(bufferCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state.mu.Lock()
			if !state.isRecording || state.webmParser.BufferLen() == 0 || state.isProcessing {
				state.mu.Unlock()
				continue
			}

			elapsed := time.Since(state.bufferStart)
			if elapsed >= h.BufferThreshold {
				// Threshold reached — grab valid WebM audio.
				// GrabAudio always returns init segment + cluster data,
				// ensuring every whisper request gets a valid WebM file.
				// Note: buffer is NOT cleared here — processAudio will
				// trim at the instruction boundary (if LLM returns one)
				// or keep all data for the next cycle (wait_for_more).
				audioData := state.webmParser.GrabAudio()
				firstChunkTS := state.firstChunkClient
				state.isProcessing = true
				state.mu.Unlock()

				logger.Debug("buffer threshold reached", "elapsed", elapsed, "size", len(audioData))

				if len(audioData) >= minAudioSize {
					go h.processAudio(ctx, conn, state, audioData, firstChunkTS, logger)
				} else {
					state.mu.Lock()
					state.isProcessing = false
					state.mu.Unlock()
				}
			} else {
				state.mu.Unlock()
			}
		}
	}
}
