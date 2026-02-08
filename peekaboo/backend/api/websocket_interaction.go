package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
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
