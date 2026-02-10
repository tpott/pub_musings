// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
	"github.com/tpott/pub_musings/peekaboo/backend/tts"
)

// WebSocket message types from client
const (
	MsgTypeStartRecording = "start_recording"
	MsgTypeStopRecording  = "stop_recording"
	MsgTypeAudioData      = "audio_data"
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

	// silenceBufferThreshold is the shorter buffer threshold used when trailing
	// silence was detected in the previous transcription cycle. This enables
	// faster re-processing when the user has paused speaking.
	silenceBufferThreshold = 1 * time.Second
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

// AudioDataMessage represents a base64-encoded audio chunk from the client.
type AudioDataMessage struct {
	Type       string  `json:"type"`
	Data       string  `json:"data"`                  // base64-encoded audio bytes
	Seq        uint16  `json:"seq"`                   // sequence number (wraps at 65535)
	ClientTime float64 `json:"client_time,omitempty"` // Date.now() in milliseconds
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

// AudioWebSocketHandler handles WebSocket connections for audio streaming.
type AudioWebSocketHandler struct {
	WhisperURL      string
	LLMProvider     llm.Provider
	LLMProviderName string       // "anthropic" or "openai" — for interaction logging
	TTSProvider     tts.Provider // Optional - nil means TTS disabled
	Database        *db.DB
	Client          *http.Client
	RateLimiter     *RateLimiter       // Optional - nil means no rate limiting
	AllowedOrigin   string             // Optional - "*" or empty means allow all
	ConnTracker     *ConnectionTracker // Optional - nil means no connection limit
	AuthTracker     *WSAuthTracker     // Optional - nil means no per-user/IP auth limits

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

	// Connection identity for interaction logging.
	connectionID string

	// Auth: populated during WS upgrade from session cookie/bearer token.
	// Empty strings for anonymous connections.
	userID    string
	sessionID string
	clientIP  string

	// Framed audio protocol metadata
	chunkMetas       []AudioChunkMeta // metadata for each received audio chunk
	firstChunkClient float64          // client timestamp (ms) of the first chunk in current buffer window

	// Transcript accumulation across buffer cycles.
	// When the LLM returns wait_for_more, accumulated words carry forward
	// to the next transcription cycle so the LLM sees the full context.
	accumulatedWords []llm.WordData

	// VAD silence detection: true when whisper detected trailing silence
	// after speech in the most recent transcription. Used by the buffer
	// threshold watcher to apply a shorter threshold for faster re-triggering.
	trailingSilenceDetected bool
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

	// Extract session from cookie or bearer token for auth-aware limits
	var wsUserID, wsSessionID string
	if h.Database != nil {
		if token := extractSessionToken(r); token != "" {
			session, err := h.Database.GetSessionByToken(token)
			if err != nil {
				logger.Error("websocket auth: failed to look up session", "error", err)
			} else if session != nil && time.Now().UTC().Before(session.ExpiresAt) {
				wsUserID = session.UserID
				wsSessionID = session.ID
				logger = logger.With("user_id", wsUserID)
			}
		}
	}

	// Apply per-user/per-IP connection limits
	if h.AuthTracker != nil {
		if wsUserID != "" {
			if !h.AuthTracker.TryAcquireUser(wsUserID) {
				logger.Warn("websocket user connection limit reached", "user_id", wsUserID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				if _, err := w.Write([]byte(`{"error":"too many concurrent connections"}`)); err != nil {
					logger.Debug("failed to write user limit response", "error", err)
				}
				return
			}
			defer h.AuthTracker.ReleaseUser(wsUserID)
		} else {
			if !h.AuthTracker.TryAcquireAnon(clientIP) {
				logger.Warn("websocket anonymous connection limit reached", "ip", clientIP)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				if _, err := w.Write([]byte(`{"error":"too many concurrent connections"}`)); err != nil {
					logger.Debug("failed to write anon limit response", "error", err)
				}
				return
			}
			defer h.AuthTracker.ReleaseAnon(clientIP)
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

	// Generate connection ID for interaction logging
	connID, err := auth.GenerateID()
	if err != nil {
		logger.Error("failed to generate connection ID", "error", err)
		connID = fmt.Sprintf("err-%d", time.Now().UnixNano())
	}

	// Initialize connection state
	state := &connectionState{
		webmParser:   NewWebMParser(),
		lastActivity: time.Now(),
		connectionID: connID,
		userID:       wsUserID,
		sessionID:    wsSessionID,
		clientIP:     clientIP,
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
