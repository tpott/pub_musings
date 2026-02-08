package db

import (
	"encoding/json"
	"fmt"
	"time"
)

// interactionSchema defines the interaction logging table and indexes.
const interactionSchema = `
CREATE TABLE IF NOT EXISTS interactions (
	id TEXT PRIMARY KEY,
	connection_id TEXT NOT NULL,
	user_id TEXT,
	session_id TEXT,

	-- Audio input
	audio_blob_path TEXT,
	audio_duration_s REAL,
	audio_chunk_count INTEGER,

	-- STT (whisper)
	stt_request_at DATETIME,
	stt_response_json TEXT,
	stt_transcript TEXT,
	stt_word_count INTEGER,
	stt_low_prob_words TEXT,
	stt_latency_ms INTEGER,
	stt_vad_speech_detected INTEGER,

	-- LLM
	llm_request_at DATETIME,
	llm_provider TEXT,
	llm_model TEXT,
	llm_system_prompt_hash TEXT,
	llm_input_text TEXT,
	llm_concepts_available TEXT,
	llm_response_json TEXT,
	llm_input_tokens INTEGER,
	llm_output_tokens INTEGER,
	llm_latency_ms INTEGER,

	-- LLM outcome
	action_type TEXT NOT NULL,
	action_subject TEXT,
	action_media_set_id INTEGER,
	action_tts_text TEXT,
	action_wait_reason TEXT,
	instruction_end_word_idx INTEGER,

	-- TTS (piper)
	tts_request_at DATETIME,
	tts_latency_ms INTEGER,
	tts_audio_size_bytes INTEGER,

	-- Buffer management
	buffer_trim_time_ms INTEGER,
	buffer_bytes_before_trim INTEGER,
	buffer_bytes_after_trim INTEGER,
	accumulated_transcript TEXT,

	-- Totals
	total_latency_ms INTEGER,

	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_interactions_connection_id ON interactions(connection_id);
CREATE INDEX IF NOT EXISTS idx_interactions_user_id ON interactions(user_id);
CREATE INDEX IF NOT EXISTS idx_interactions_action_type ON interactions(action_type);
CREATE INDEX IF NOT EXISTS idx_interactions_created_at ON interactions(created_at);
CREATE INDEX IF NOT EXISTS idx_interactions_stt_latency ON interactions(stt_latency_ms);
CREATE INDEX IF NOT EXISTS idx_interactions_llm_latency ON interactions(llm_latency_ms);
`

// InteractionLog captures all data from a single processAudio cycle.
type InteractionLog struct {
	ID           string
	ConnectionID string
	UserID       *string // nil if anonymous
	SessionID    *string // nil if anonymous

	STT    *STTLog
	LLM    *LLMLog
	TTS    *TTSLog
	Buffer *BufferLog

	TotalLatencyMs int64
	CreatedAt      time.Time
}

// STTLog captures speech-to-text data from whisper.
type STTLog struct {
	ResponseJSON      json.RawMessage
	Transcript        string
	WordCount         int
	LowProbWords      []LowProbWord
	LatencyMs         int64
	VADSpeechDetected bool
	AudioDurationS    float64
	AudioChunkCount   int
	AudioBlobPath     *string
	RequestAt         time.Time
}

// LowProbWord is a word with low confidence from whisper.
type LowProbWord struct {
	Word        string  `json:"word"`
	Probability float64 `json:"prob"`
	Index       int     `json:"idx"`
}

// LLMLog captures language model data.
type LLMLog struct {
	Provider          string
	Model             string
	SystemPromptHash  string
	InputText         string
	ConceptsAvailable []string
	ResponseJSON      json.RawMessage
	InputTokens       int
	OutputTokens      int
	LatencyMs         int64
	ActionType        string
	Subject           *string
	MediaSetID        *int64
	TTSText           *string
	WaitReason        *string
	InstructionEndIdx int
	RequestAt         time.Time
}

// TTSLog captures text-to-speech data from piper.
type TTSLog struct {
	LatencyMs      int64
	AudioSizeBytes int
	RequestAt      time.Time
}

// BufferLog captures audio buffer management data.
type BufferLog struct {
	TrimTimeMs            int64
	BytesBeforeTrim       int
	BytesAfterTrim        int
	AccumulatedTranscript string
}

// InitInteractions creates the interactions table and indexes.
func (db *DB) InitInteractions() error {
	if _, err := db.conn.Exec(interactionSchema); err != nil {
		return fmt.Errorf("create interactions schema: %w", err)
	}
	return nil
}

// InsertInteraction persists an interaction log to the database.
func (db *DB) InsertInteraction(log *InteractionLog) error {
	// Marshal STT fields
	var (
		audioBlobPath   *string
		audioDuration   *float64
		audioChunkCount *int
		sttRequestAt    *time.Time
		sttResponseJSON *string
		sttTranscript   *string
		sttWordCount    *int
		sttLowProbWords *string
		sttLatencyMs    *int64
		sttVADDetected  *int
	)

	if log.STT != nil {
		sttRequestAt = &log.STT.RequestAt
		sttTranscript = &log.STT.Transcript
		sttWordCount = &log.STT.WordCount
		sttLatencyMs = &log.STT.LatencyMs
		audioDuration = &log.STT.AudioDurationS
		audioChunkCount = &log.STT.AudioChunkCount
		audioBlobPath = log.STT.AudioBlobPath

		if len(log.STT.ResponseJSON) > 0 {
			s := string(log.STT.ResponseJSON)
			sttResponseJSON = &s
		}

		if len(log.STT.LowProbWords) > 0 {
			b, err := json.Marshal(log.STT.LowProbWords)
			if err != nil {
				return fmt.Errorf("marshal low prob words: %w", err)
			}
			s := string(b)
			sttLowProbWords = &s
		}

		vadInt := 0
		if log.STT.VADSpeechDetected {
			vadInt = 1
		}
		sttVADDetected = &vadInt
	}

	// Marshal LLM fields
	var (
		llmRequestAt      *time.Time
		llmProvider       *string
		llmModel          *string
		llmPromptHash     *string
		llmInputText      *string
		llmConcepts       *string
		llmResponseJSON   *string
		llmInputTokens    *int
		llmOutputTokens   *int
		llmLatencyMs      *int64
		actionType        string
		actionSubject     *string
		actionMediaSetID  *int64
		actionTTSText     *string
		actionWaitReason  *string
		instructionEndIdx *int
	)

	if log.LLM != nil {
		llmRequestAt = &log.LLM.RequestAt
		llmProvider = &log.LLM.Provider
		llmModel = &log.LLM.Model
		llmPromptHash = &log.LLM.SystemPromptHash
		llmInputText = &log.LLM.InputText
		llmInputTokens = &log.LLM.InputTokens
		llmOutputTokens = &log.LLM.OutputTokens
		llmLatencyMs = &log.LLM.LatencyMs
		actionType = log.LLM.ActionType
		actionSubject = log.LLM.Subject
		actionMediaSetID = log.LLM.MediaSetID
		actionTTSText = log.LLM.TTSText
		actionWaitReason = log.LLM.WaitReason
		instructionEndIdx = &log.LLM.InstructionEndIdx

		if len(log.LLM.ConceptsAvailable) > 0 {
			b, err := json.Marshal(log.LLM.ConceptsAvailable)
			if err != nil {
				return fmt.Errorf("marshal concepts: %w", err)
			}
			s := string(b)
			llmConcepts = &s
		}

		if len(log.LLM.ResponseJSON) > 0 {
			s := string(log.LLM.ResponseJSON)
			llmResponseJSON = &s
		}
	} else {
		actionType = "unknown"
	}

	// Marshal TTS fields
	var (
		ttsRequestAt *time.Time
		ttsLatencyMs *int64
		ttsAudioSize *int
	)

	if log.TTS != nil {
		ttsRequestAt = &log.TTS.RequestAt
		ttsLatencyMs = &log.TTS.LatencyMs
		ttsAudioSize = &log.TTS.AudioSizeBytes
	}

	// Marshal Buffer fields
	var (
		bufferTrimMs    *int64
		bufferBytesPre  *int
		bufferBytesPost *int
		accTranscript   *string
	)

	if log.Buffer != nil {
		bufferTrimMs = &log.Buffer.TrimTimeMs
		bufferBytesPre = &log.Buffer.BytesBeforeTrim
		bufferBytesPost = &log.Buffer.BytesAfterTrim
		if log.Buffer.AccumulatedTranscript != "" {
			accTranscript = &log.Buffer.AccumulatedTranscript
		}
	}

	_, err := db.conn.Exec(`
		INSERT INTO interactions (
			id, connection_id, user_id, session_id,
			audio_blob_path, audio_duration_s, audio_chunk_count,
			stt_request_at, stt_response_json, stt_transcript, stt_word_count,
			stt_low_prob_words, stt_latency_ms, stt_vad_speech_detected,
			llm_request_at, llm_provider, llm_model, llm_system_prompt_hash,
			llm_input_text, llm_concepts_available, llm_response_json,
			llm_input_tokens, llm_output_tokens, llm_latency_ms,
			action_type, action_subject, action_media_set_id,
			action_tts_text, action_wait_reason, instruction_end_word_idx,
			tts_request_at, tts_latency_ms, tts_audio_size_bytes,
			buffer_trim_time_ms, buffer_bytes_before_trim, buffer_bytes_after_trim,
			accumulated_transcript,
			total_latency_ms, created_at
		) VALUES (
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?,
			?, ?
		)
	`,
		log.ID, log.ConnectionID, log.UserID, log.SessionID,
		audioBlobPath, audioDuration, audioChunkCount,
		sttRequestAt, sttResponseJSON, sttTranscript, sttWordCount,
		sttLowProbWords, sttLatencyMs, sttVADDetected,
		llmRequestAt, llmProvider, llmModel, llmPromptHash,
		llmInputText, llmConcepts, llmResponseJSON,
		llmInputTokens, llmOutputTokens, llmLatencyMs,
		actionType, actionSubject, actionMediaSetID,
		actionTTSText, actionWaitReason, instructionEndIdx,
		ttsRequestAt, ttsLatencyMs, ttsAudioSize,
		bufferTrimMs, bufferBytesPre, bufferBytesPost,
		accTranscript,
		log.TotalLatencyMs, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert interaction: %w", err)
	}
	return nil
}

// GetInteraction retrieves an interaction log by ID.
func (db *DB) GetInteraction(id string) (*InteractionLog, error) {
	var (
		log            InteractionLog
		audioBlobPath  *string
		audioDuration  *float64
		audioChunkCnt  *int
		sttRequestAt   *time.Time
		sttRespJSON    *string
		sttTranscript  *string
		sttWordCount   *int
		sttLowProb     *string
		sttLatency     *int64
		sttVAD         *int
		llmRequestAt   *time.Time
		llmProvider    *string
		llmModel       *string
		llmPromptHash  *string
		llmInputText   *string
		llmConcepts    *string
		llmRespJSON    *string
		llmInTokens    *int
		llmOutTokens   *int
		llmLatency     *int64
		actionType     string
		actionSubject  *string
		actionMediaID  *int64
		actionTTSText  *string
		actionWaitRsn  *string
		instrEndIdx    *int
		ttsRequestAt   *time.Time
		ttsLatency     *int64
		ttsAudioSize   *int
		bufferTrimMs   *int64
		bufferBytesPre *int
		bufferBytesPst *int
		accTranscript  *string
	)

	err := db.conn.QueryRow(`
		SELECT
			id, connection_id, user_id, session_id,
			audio_blob_path, audio_duration_s, audio_chunk_count,
			stt_request_at, stt_response_json, stt_transcript, stt_word_count,
			stt_low_prob_words, stt_latency_ms, stt_vad_speech_detected,
			llm_request_at, llm_provider, llm_model, llm_system_prompt_hash,
			llm_input_text, llm_concepts_available, llm_response_json,
			llm_input_tokens, llm_output_tokens, llm_latency_ms,
			action_type, action_subject, action_media_set_id,
			action_tts_text, action_wait_reason, instruction_end_word_idx,
			tts_request_at, tts_latency_ms, tts_audio_size_bytes,
			buffer_trim_time_ms, buffer_bytes_before_trim, buffer_bytes_after_trim,
			accumulated_transcript,
			total_latency_ms, created_at
		FROM interactions WHERE id = ?
	`, id).Scan(
		&log.ID, &log.ConnectionID, &log.UserID, &log.SessionID,
		&audioBlobPath, &audioDuration, &audioChunkCnt,
		&sttRequestAt, &sttRespJSON, &sttTranscript, &sttWordCount,
		&sttLowProb, &sttLatency, &sttVAD,
		&llmRequestAt, &llmProvider, &llmModel, &llmPromptHash,
		&llmInputText, &llmConcepts, &llmRespJSON,
		&llmInTokens, &llmOutTokens, &llmLatency,
		&actionType, &actionSubject, &actionMediaID,
		&actionTTSText, &actionWaitRsn, &instrEndIdx,
		&ttsRequestAt, &ttsLatency, &ttsAudioSize,
		&bufferTrimMs, &bufferBytesPre, &bufferBytesPst,
		&accTranscript,
		&log.TotalLatencyMs, &log.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get interaction %s: %w", id, err)
	}

	// Reconstruct STT
	if sttTranscript != nil {
		log.STT = &STTLog{
			Transcript:        *sttTranscript,
			AudioBlobPath:     audioBlobPath,
			VADSpeechDetected: sttVAD != nil && *sttVAD == 1,
		}
		if sttRequestAt != nil {
			log.STT.RequestAt = *sttRequestAt
		}
		if sttRespJSON != nil {
			log.STT.ResponseJSON = json.RawMessage(*sttRespJSON)
		}
		if sttWordCount != nil {
			log.STT.WordCount = *sttWordCount
		}
		if sttLatency != nil {
			log.STT.LatencyMs = *sttLatency
		}
		if audioDuration != nil {
			log.STT.AudioDurationS = *audioDuration
		}
		if audioChunkCnt != nil {
			log.STT.AudioChunkCount = *audioChunkCnt
		}
		if sttLowProb != nil {
			if err := json.Unmarshal([]byte(*sttLowProb), &log.STT.LowProbWords); err != nil {
				return nil, fmt.Errorf("unmarshal low prob words: %w", err)
			}
		}
	}

	// Reconstruct LLM
	if llmProvider != nil {
		log.LLM = &LLMLog{
			ActionType: actionType,
			Subject:    actionSubject,
			MediaSetID: actionMediaID,
			TTSText:    actionTTSText,
			WaitReason: actionWaitRsn,
		}
		if llmRequestAt != nil {
			log.LLM.RequestAt = *llmRequestAt
		}
		log.LLM.Provider = *llmProvider
		if llmModel != nil {
			log.LLM.Model = *llmModel
		}
		if llmPromptHash != nil {
			log.LLM.SystemPromptHash = *llmPromptHash
		}
		if llmInputText != nil {
			log.LLM.InputText = *llmInputText
		}
		if llmRespJSON != nil {
			log.LLM.ResponseJSON = json.RawMessage(*llmRespJSON)
		}
		if llmInTokens != nil {
			log.LLM.InputTokens = *llmInTokens
		}
		if llmOutTokens != nil {
			log.LLM.OutputTokens = *llmOutTokens
		}
		if llmLatency != nil {
			log.LLM.LatencyMs = *llmLatency
		}
		if instrEndIdx != nil {
			log.LLM.InstructionEndIdx = *instrEndIdx
		}
		if llmConcepts != nil {
			if err := json.Unmarshal([]byte(*llmConcepts), &log.LLM.ConceptsAvailable); err != nil {
				return nil, fmt.Errorf("unmarshal concepts: %w", err)
			}
		}
	}

	// Reconstruct TTS
	if ttsLatency != nil {
		log.TTS = &TTSLog{
			LatencyMs: *ttsLatency,
		}
		if ttsRequestAt != nil {
			log.TTS.RequestAt = *ttsRequestAt
		}
		if ttsAudioSize != nil {
			log.TTS.AudioSizeBytes = *ttsAudioSize
		}
	}

	// Reconstruct Buffer
	if bufferTrimMs != nil {
		log.Buffer = &BufferLog{
			TrimTimeMs: *bufferTrimMs,
		}
		if bufferBytesPre != nil {
			log.Buffer.BytesBeforeTrim = *bufferBytesPre
		}
		if bufferBytesPst != nil {
			log.Buffer.BytesAfterTrim = *bufferBytesPst
		}
		if accTranscript != nil {
			log.Buffer.AccumulatedTranscript = *accTranscript
		}
	}

	return &log, nil
}
