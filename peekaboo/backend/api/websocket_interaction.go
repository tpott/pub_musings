package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/coder/websocket"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// buildSTTLog constructs an STTLog from a WhisperResponse and timing info.
func buildSTTLog(resp *WhisperResponse, latency time.Duration, chunkCount int, requestAt time.Time) *db.STTLog {
	sttLog := &db.STTLog{
		Transcript:      resp.Text,
		LatencyMs:       latency.Milliseconds(),
		AudioDurationS:  resp.Duration,
		AudioChunkCount: chunkCount,
		RequestAt:       requestAt,
	}

	// Marshal full response to JSON
	respJSON, err := json.Marshal(resp)
	if err == nil {
		sttLog.ResponseJSON = respJSON
	}

	// Count words and find low-probability ones
	wordIdx := 0
	for _, seg := range resp.Segments {
		for _, w := range seg.Words {
			sttLog.WordCount++
			if w.Probability < lowProbThreshold {
				sttLog.LowProbWords = append(sttLog.LowProbWords, db.LowProbWord{
					Word:        w.Word,
					Probability: w.Probability,
					Index:       wordIdx,
				})
			}
			wordIdx++
		}
	}

	// VAD: speech detected if transcript is non-empty
	sttLog.VADSpeechDetected = len(resp.Text) > 0 && resp.Text != ""

	return sttLog
}

// lowProbThreshold is the probability below which a word is considered low-confidence.
const lowProbThreshold = 0.80

// buildLLMLog constructs an LLMLog from an LLM TranscriptResult, provider name, concepts, and timing info.
func buildLLMLog(result *llm.TranscriptResult, provider string, concepts []string, latency time.Duration, requestAt time.Time) *db.LLMLog {
	llmLog := &db.LLMLog{
		Provider:          provider,
		Model:             result.Model,
		InputText:         result.InputText,
		ConceptsAvailable: concepts,
		ResponseJSON:      result.RawResponse,
		InputTokens:       result.InputTokens,
		OutputTokens:      result.OutputTokens,
		LatencyMs:         latency.Milliseconds(),
		RequestAt:         requestAt,
	}

	// SHA-256 hash of system prompt
	if result.SystemPrompt != "" {
		hash := sha256.Sum256([]byte(result.SystemPrompt))
		llmLog.SystemPromptHash = fmt.Sprintf("%x", hash)
	}

	// Extract action details from first action
	if len(result.Actions) > 0 {
		action := result.Actions[0]
		llmLog.ActionType = action.Type

		switch action.Type {
		case "show_media":
			llmLog.Subject = &action.Subject
			llmLog.InstructionEndIdx = action.InstructionEndWordIdx
		case "text_to_speech":
			llmLog.TTSText = &action.Text
		case "wait_for_more":
			llmLog.WaitReason = &action.Reason
		}
	}

	return llmLog
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

// ttsResult holds timing and size data from a TTS synthesis call.
type ttsResult struct {
	latency   time.Duration
	audioSize int
	requestAt time.Time
}

// executeTTS synthesizes speech from text via the TTS provider and sends
// the resulting WAV audio to the client as a base64-encoded tts_audio message.
// Returns timing/size data for interaction logging, or nil if TTS was skipped or failed.
func (h *AudioWebSocketHandler) executeTTS(ctx context.Context, conn *websocket.Conn, text string, logger *slog.Logger) *ttsResult {
	if h.TTSProvider == nil {
		logger.Debug("text_to_speech skipped: TTS provider not configured")
		return nil
	}
	if text == "" {
		logger.Debug("text_to_speech skipped: empty text")
		return nil
	}

	logger.Info("text_to_speech action", "text", text)

	ttsCtx, ttsCancel := context.WithTimeout(ctx, intentTimeout)
	defer ttsCancel()

	requestAt := time.Now()
	audioData, err := h.TTSProvider.Synthesize(ttsCtx, text)
	latency := time.Since(requestAt)
	if err != nil {
		logger.Warn("TTS synthesis failed", "error", err, "text", text)
		return nil
	}

	h.sendTTSAudio(ctx, conn, audioData, text, logger)
	return &ttsResult{
		latency:   latency,
		audioSize: len(audioData),
		requestAt: requestAt,
	}
}

// saveInteraction persists an interaction log to the database.
// If INTERACTION_LOG_AUDIO=true, the audio blob is written to disk asynchronously.
func (h *AudioWebSocketHandler) saveInteraction(interaction *db.InteractionLog, audioData []byte, logger *slog.Logger) {
	if h.Database == nil {
		return
	}

	if err := h.Database.InsertInteraction(interaction); err != nil {
		logger.Error("failed to save interaction", "error", err, "id", interaction.ID)
		return
	}

	// Optionally save audio blob to disk
	if os.Getenv("INTERACTION_LOG_AUDIO") != "true" || len(audioData) == 0 {
		return
	}

	id := interaction.ID
	go func() {
		dateDir := time.Now().UTC().Format("2006-01-02")
		dir := filepath.Join("data", "interactions", dateDir)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			logger.Error("failed to create interaction audio dir", "error", err, "dir", dir)
			return
		}

		path := filepath.Join(dir, id+".webm")
		if err := os.WriteFile(path, audioData, 0o640); err != nil {
			logger.Error("failed to write interaction audio", "error", err, "path", path)
			return
		}

		// Update the interaction row with the audio blob path
		if err := h.Database.UpdateInteractionAudioPath(id, path); err != nil {
			logger.Error("failed to update interaction audio path", "error", err, "id", id)
		}
	}()
}

// executeShowMedia validates a subject and sends media to the client.
// Returns a *ttsResult if TTS was used for an unrecognized subject, and
// the media set ID if a media set was found and sent.
func (h *AudioWebSocketHandler) executeShowMedia(ctx context.Context, conn *websocket.Conn, subject string, logger *slog.Logger) (*ttsResult, *int64) {
	logger.Info("show_media action", "subject", subject)

	// Validate subject format (same validation as HTTP media endpoint)
	if subject == "" {
		logger.Debug("empty subject from LLM")
		h.sendError(ctx, conn, "I didn't understand what you want to see. Please try again.", logger)
		return nil, nil
	}
	if !validConceptPattern.MatchString(subject) {
		logger.Debug("invalid subject format", "subject", subject)
		h.sendError(ctx, conn, fmt.Sprintf("I don't have media for '%s'. Try a simple animal name like 'cat' or 'dog'.", subject), logger)
		return nil, nil
	}
	if len(subject) > maxConceptLength {
		logger.Debug("subject too long", "subject", subject, "length", len(subject))
		h.sendError(ctx, conn, "That's too long! Try a simple animal name like 'cat' or 'dog'.", logger)
		return nil, nil
	}

	// Look up media for subject
	if h.Database == nil {
		logger.Error("database not configured")
		h.sendError(ctx, conn, "media lookup unavailable", logger)
		return nil, nil
	}
	mediaSet, err := h.Database.GetRandomMediaSet(subject)
	if err != nil {
		logger.Warn("media lookup failed", "subject", subject, "error", err)
		h.sendError(ctx, conn, fmt.Sprintf("no media found for %s", subject), logger)
		return nil, nil
	}
	if mediaSet == nil {
		logger.Debug("no media set found", "subject", subject)
		msg := "I don't know that one yet! Try saying cat, dog, or duck."
		h.sendError(ctx, conn, msg, logger)
		return h.executeTTS(ctx, conn, msg, logger), nil
	}

	// Send media to client
	h.sendMedia(ctx, conn, subject, mediaSet, logger)
	return nil, &mediaSet.ID
}
