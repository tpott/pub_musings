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
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

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
		state.consecutiveEmptyTranscripts = 0
		state.mu.Unlock()
		logger.Debug("recording started")

	case MsgTypeStopRecording:
		state.mu.Lock()
		state.isRecording = false
		alreadyProcessing := state.isProcessing
		var audioData []byte
		var firstChunkTS float64
		if !alreadyProcessing {
			audioData = state.webmParser.GrabAudio()
			state.webmParser.Clear()
			firstChunkTS = state.firstChunkClient
			state.accumulatedWords = nil // clear accumulation on stop
		}
		willProcess := !alreadyProcessing && len(audioData) >= minAudioSize
		if willProcess {
			state.isProcessing = true // prevent idle timeout and buffer watcher races
		}
		state.mu.Unlock()

		if alreadyProcessing {
			logger.Debug("recording stopped, audio already being processed")
		} else {
			logger.Debug("recording stopped", "buffer_size", len(audioData))
		}

		if willProcess {
			go h.processAudio(ctx, conn, state, audioData, firstChunkTS, logger)
		} else if !alreadyProcessing && len(audioData) > 0 {
			h.sendError(ctx, conn, "audio too short", logger)
		} else if !alreadyProcessing {
			// Empty buffer - user stopped recording without speaking
			h.sendError(ctx, conn, "No audio recorded", logger)
		}

	case MsgTypeAudioData:
		h.handleAudioDataMessage(ctx, conn, state, data, logger)

	case MsgTypePing:
		h.sendPong(ctx, conn, logger)
	}
}

// handleAudioDataMessage processes a base64-encoded audio chunk sent as a JSON text message.
// This is the preferred transport (replaces raw binary frames).
func (h *AudioWebSocketHandler) handleAudioDataMessage(_ context.Context, _ *websocket.Conn, state *connectionState, data []byte, logger *slog.Logger) {
	var msg AudioDataMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Warn("invalid audio_data message", "error", err)
		return
	}

	// Decode base64 audio
	audioData, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		logger.Warn("invalid base64 in audio_data", "error", err)
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.isRecording {
		return
	}

	chunkMeta := &AudioChunkMeta{
		Seq:      msg.Seq,
		ClientTS: msg.ClientTime,
		ServerTS: time.Now(),
		Offset:   state.webmParser.BufferLen(),
	}

	// Start buffer timer on first chunk
	if state.webmParser.BufferLen() == 0 {
		state.bufferStart = time.Now()
		state.firstChunkClient = chunkMeta.ClientTS
	}

	if !state.webmParser.Append(audioData) {
		logger.Warn("audio buffer overflow, dropping chunk", "buffer_size", state.webmParser.BufferLen(), "chunk_size", len(audioData))
		return
	}
	state.chunkMetas = append(state.chunkMetas, *chunkMeta)

	logger.Debug("received audio_data chunk", "size", len(audioData), "seq", msg.Seq, "buffer_size", state.webmParser.BufferLen())
}

// handleAudioChunk buffers incoming audio data (binary WebSocket messages).
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
	if !state.webmParser.Append(audioData) {
		logger.Warn("audio buffer overflow, dropping chunk", "buffer_size", state.webmParser.BufferLen(), "chunk_size", len(audioData))
		return
	}

	// Record chunk metadata
	if chunkMeta != nil {
		state.chunkMetas = append(state.chunkMetas, *chunkMeta)
	}

	logger.Debug("received audio chunk", "size", len(audioData), "buffer_size", state.webmParser.BufferLen())
}

// processAudio transcribes audio and processes transcript through LLM.
// state is the connection state used for transcript accumulation and buffer
// trimming. firstChunkClientTS is the client wall-clock timestamp (ms since
// epoch) of the first audio chunk in this buffer.
func (h *AudioWebSocketHandler) processAudio(ctx context.Context, conn *websocket.Conn, state *connectionState, audioData []byte, firstChunkClientTS float64, logger *slog.Logger) {
	// Build interaction log progressively; defer ensures it's always saved.
	interactionID, err := auth.GenerateID()
	if err != nil {
		logger.Error("failed to generate interaction ID", "error", err)
		interactionID = fmt.Sprintf("err-%d", time.Now().UnixNano())
	}

	var (
		sttLog    *db.STTLog
		llmLog    *db.LLMLog
		ttsLog    *db.TTSLog
		bufferLog *db.BufferLog
		startTime = time.Now()
	)

	defer func() {
		state.mu.Lock()
		state.isProcessing = false
		state.mu.Unlock()

		// Build and save interaction log
		var userID, sessionID *string
		if state.userID != "" {
			userID = &state.userID
		}
		if state.sessionID != "" {
			sessionID = &state.sessionID
		}

		interaction := &db.InteractionLog{
			ID:             interactionID,
			ConnectionID:   state.connectionID,
			UserID:         userID,
			SessionID:      sessionID,
			STT:            sttLog,
			LLM:            llmLog,
			TTS:            ttsLog,
			Buffer:         bufferLog,
			TotalLatencyMs: time.Since(startTime).Milliseconds(),
			CreatedAt:      time.Now().UTC(),
		}
		h.saveInteraction(interaction, audioData, logger)
	}()

	// Check anonymous interaction rate limit
	if h.AuthTracker != nil && state.userID == "" {
		if !h.AuthTracker.AllowAnonInteraction(state.clientIP) {
			logger.Warn("anonymous interaction rate limit exceeded", "ip", state.clientIP)
			h.sendError(ctx, conn, "rate limit exceeded, try again later", logger)
			return
		}
	}

	// Get chunk count from state (before it's reset)
	state.mu.Lock()
	chunkCount := len(state.chunkMetas)
	state.mu.Unlock()

	// 1. Send to whisper for transcription
	sttRequestAt := time.Now()
	whisperResp, err := h.transcribeAudio(ctx, audioData)
	sttLatency := time.Since(sttRequestAt)
	if err != nil {
		logger.Error("transcription failed", "error", err)
		h.sendError(ctx, conn, "transcription failed", logger)
		return
	}

	// Build STT log data
	sttLog = buildSTTLog(whisperResp, sttLatency, chunkCount, sttRequestAt)

	transcript := whisperResp.Text
	logger.Info("transcription complete", "text", transcript)

	// Check for empty transcript (silence or no recognizable speech)
	// This is normal during continuous listening - don't send an error to the client
	if strings.TrimSpace(transcript) == "" {
		state.mu.Lock()
		state.consecutiveEmptyTranscripts++
		n := state.consecutiveEmptyTranscripts
		state.mu.Unlock()
		logger.Debug("empty transcript from whisper, ignoring", "consecutive_empty", n)
		return
	}

	// Non-empty transcript — reset backoff counter
	state.mu.Lock()
	state.consecutiveEmptyTranscripts = 0
	state.mu.Unlock()

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

	// 3. Detect trailing silence via VAD: if the last word ends well before the
	// audio duration, the user has likely paused. Store this for the buffer
	// threshold watcher to use a shorter trigger interval.
	silenceGap := detectTrailingSilence(whisperResp)
	if silenceGap > 0 {
		logger.Debug("trailing silence detected", "gap_s", silenceGap, "duration_s", whisperResp.Duration)
		state.mu.Lock()
		state.trailingSilenceDetected = true
		state.mu.Unlock()
	}

	// 4. Use accumulated words from previous cycles (if any) merged with current.
	// Since we re-transcribe the full buffer each time (GrabAudio returns all
	// cluster data), the current whisper response already covers accumulated audio.
	// We use currentWords directly — accumulation happens at the buffer level.
	words := currentWords

	// 5. Get available concepts from DB
	var concepts []string
	if h.Database != nil {
		concepts, err = h.Database.ListConceptIDs()
		if err != nil {
			logger.Warn("failed to list concepts", "error", err)
		}
	}

	// 6. Process transcript through LLM with tool_choice:any
	llmRequestAt := time.Now()
	result, err := h.processTranscript(ctx, transcript, words, concepts)
	llmLatency := time.Since(llmRequestAt)
	if err != nil {
		logger.Error("transcript processing failed", "error", err)
		h.sendError(ctx, conn, "intent extraction failed", logger)
		return
	}

	// Build LLM log data
	llmLog = buildLLMLog(result, h.LLMProviderName, concepts, llmLatency, llmRequestAt)

	// 7. Execute actions from LLM response
	for _, action := range result.Actions {
		if ctx.Err() != nil {
			logger.Debug("context canceled, stopping action execution")
			return
		}
		switch action.Type {
		case "show_media":
			tr, mediaSetID := h.executeShowMedia(ctx, conn, action.Subject, logger)
			if tr != nil {
				ttsLog = &db.TTSLog{
					LatencyMs:      tr.latency.Milliseconds(),
					AudioSizeBytes: tr.audioSize,
					RequestAt:      tr.requestAt,
				}
			}
			if mediaSetID != nil && llmLog != nil {
				llmLog.MediaSetID = mediaSetID
			}

			// Trim audio buffer at instruction boundary
			state.mu.Lock()
			bytesBeforeTrim := state.webmParser.BufferLen()
			var trimTimeMs uint64
			if action.InstructionEndWordIdx >= 0 && action.InstructionEndWordIdx < len(words) {
				endWord := words[action.InstructionEndWordIdx]
				// Convert word end time (seconds) to cluster timecode (milliseconds)
				trimTimeMs = uint64(endWord.End * 1000)
				trimmed := state.webmParser.TrimBefore(trimTimeMs)
				if trimmed == 0 {
					// TrimBefore couldn't find cluster boundaries — clear buffer
					logger.Debug("TrimBefore returned 0 bytes, falling back to Clear",
						"buffer_size", bytesBeforeTrim,
						"trim_time_ms", trimTimeMs)
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
			bytesAfterTrim := state.webmParser.BufferLen()
			state.accumulatedWords = nil
			state.trailingSilenceDetected = false
			state.bufferStart = time.Now()
			state.mu.Unlock()

			bufferLog = &db.BufferLog{
				TrimTimeMs:      int64(trimTimeMs),
				BytesBeforeTrim: bytesBeforeTrim,
				BytesAfterTrim:  bytesAfterTrim,
			}

		case "text_to_speech":
			if tr := h.executeTTS(ctx, conn, action.Text, logger); tr != nil {
				ttsLog = &db.TTSLog{
					LatencyMs:      tr.latency.Milliseconds(),
					AudioSizeBytes: tr.audioSize,
					RequestAt:      tr.requestAt,
				}
			}

		case "wait_for_more":
			logger.Debug("wait_for_more action received", "reason", action.Reason)
			// Keep buffer intact for next transcription cycle.
			// Reset bufferStart so the threshold timer waits for more audio
			// before triggering the next transcription.
			state.mu.Lock()
			state.bufferStart = time.Now()
			state.mu.Unlock()

			bufferLog = &db.BufferLog{
				AccumulatedTranscript: transcript,
			}
		}
	}

}

// transcribeAudio sends audio to whisper-server and returns the full response
// including word-level timing and probabilities.
func (h *AudioWebSocketHandler) transcribeAudio(ctx context.Context, audioData []byte) (*WhisperResponse, error) {
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
	if err := writer.WriteField("vad", "true"); err != nil {
		return nil, fmt.Errorf("write vad: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// Send request to whisper-server
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.WhisperURL+"/inference", &buf)
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("whisper-server error: %d: %s", resp.StatusCode, string(body))
	}

	// Parse response (limit read size to prevent memory exhaustion)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxSuccessBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var whisperResp WhisperResponse
	if err := json.Unmarshal(respBody, &whisperResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &whisperResp, nil
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
			processing := state.isProcessing
			state.mu.Unlock()

			if idle > h.IdleTimeout && !processing {
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
			// Use shorter threshold when trailing silence was detected,
			// enabling faster re-processing after the user pauses.
			threshold := h.BufferThreshold
			if state.trailingSilenceDetected {
				threshold = silenceBufferThreshold
			}
			// Back off when whisper keeps returning empty transcripts
			// (e.g., broken whisper, wrong model, background noise only).
			// Doubles the threshold for each cycle past the backoff trigger.
			if n := state.consecutiveEmptyTranscripts; n >= emptyTranscriptsBeforeBackoff {
				shifts := n - emptyTranscriptsBeforeBackoff + 1
				backoff := threshold << uint(shifts) // threshold * 2^shifts
				if backoff > maxBackoffThreshold {
					backoff = maxBackoffThreshold
				}
				threshold = backoff
			}
			if elapsed >= threshold {
				// Threshold reached — grab valid WebM audio.
				// GrabAudio always returns init segment + cluster data,
				// ensuring every whisper request gets a valid WebM file.
				// Note: buffer is NOT cleared here — processAudio will
				// trim at the instruction boundary (if LLM returns one)
				// or keep all data for the next cycle (wait_for_more).
				audioData := state.webmParser.GrabAudio()
				firstChunkTS := state.firstChunkClient
				state.isProcessing = true
				state.trailingSilenceDetected = false // reset for next cycle
				state.mu.Unlock()

				logger.Debug("buffer threshold reached", "elapsed", elapsed, "threshold", threshold, "size", len(audioData))

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
