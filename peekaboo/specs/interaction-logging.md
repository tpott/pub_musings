# Interaction Logging Specification

This document describes how peekaboo captures STT, LLM, and TTS data for
accuracy assessment and debugging.

## Motivation

Today `processAudio()` logs almost nothing useful:
- STT: flat transcript text only — no word probabilities, no timing, no duration
- LLM: subject and action type only — no input message, no raw response, no token counts
- TTS: text sent only — no latency, no audio size

There is no way to answer: "how often does whisper mishear words?", "how often
does the LLM return wait_for_more?", or "what is end-to-end latency?"

## Design Principles

1. **Log at the boundary.** Capture inputs and outputs of each external call
   (whisper, LLM, Piper) — not internal state.
2. **Structured, not strings.** Store JSON blobs that can be queried with
   `json_extract()`, not formatted log lines.
3. **Opt-in for audio blobs.** Audio files are large. Persist them only when
   `INTERACTION_LOG_AUDIO=true`.
4. **No auth dependency.** This spec works with or without user authentication.
   Without auth, interactions are keyed by WebSocket connection ID.

## Database Schema

```sql
CREATE TABLE interactions (
    id TEXT PRIMARY KEY,                     -- 32-char hex (16 random bytes)
    connection_id TEXT NOT NULL,             -- WebSocket connection identifier
    user_id TEXT,                            -- NULL if anonymous (populated by auth spec)
    session_id TEXT,                         -- NULL if anonymous (populated by auth spec)

    -- Audio input
    audio_blob_path TEXT,                    -- Path to saved WebM (NULL if logging disabled)
    audio_duration_s REAL,                   -- Duration from whisper response
    audio_chunk_count INTEGER,               -- Number of chunks in this buffer

    -- STT (whisper)
    stt_request_at DATETIME,                -- When whisper request was sent
    stt_response_json TEXT,                  -- Full verbose_json from whisper
    stt_transcript TEXT,                     -- Flat transcript text
    stt_word_count INTEGER,                  -- Number of words returned
    stt_low_prob_words TEXT,                 -- JSON array of words with prob < 0.80
    stt_latency_ms INTEGER,                  -- Whisper round-trip time
    stt_vad_speech_detected INTEGER,         -- 1 if VAD found speech, 0 if silence

    -- LLM
    llm_request_at DATETIME,                -- When LLM request was sent
    llm_provider TEXT,                       -- "anthropic" or "openai"
    llm_model TEXT,                          -- Model ID (e.g. claude-haiku-4-5-20251001)
    llm_system_prompt_hash TEXT,             -- SHA-256 of system prompt (detect prompt changes)
    llm_input_text TEXT,                     -- Formatted user message with word indices
    llm_concepts_available TEXT,             -- JSON array of concept IDs in system prompt
    llm_response_json TEXT,                  -- Raw response: tool calls, stop reason
    llm_input_tokens INTEGER,
    llm_output_tokens INTEGER,
    llm_latency_ms INTEGER,

    -- LLM outcome
    action_type TEXT NOT NULL,               -- "show_media", "text_to_speech", "wait_for_more"
    action_subject TEXT,                     -- Concept shown (NULL for non-show actions)
    action_media_set_id INTEGER,             -- media_sets.id served (NULL if no media)
    action_tts_text TEXT,                    -- Text spoken (NULL if no TTS)
    action_wait_reason TEXT,                 -- Reason for wait_for_more (NULL otherwise)
    instruction_end_word_idx INTEGER,        -- Word index used for buffer trim (-1 if none)

    -- TTS (piper)
    tts_request_at DATETIME,                -- NULL if no TTS in this cycle
    tts_latency_ms INTEGER,                 -- Piper round-trip time
    tts_audio_size_bytes INTEGER,            -- Size of WAV returned

    -- Buffer management
    buffer_trim_time_ms INTEGER,             -- WebM cluster timecode used for trim
    buffer_bytes_before_trim INTEGER,        -- Buffer size before trim
    buffer_bytes_after_trim INTEGER,         -- Buffer size after trim
    accumulated_transcript TEXT,             -- Running transcript across wait_for_more cycles

    -- Totals
    total_latency_ms INTEGER,                -- End-to-end: audio threshold fired to last WS message sent

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_interactions_connection_id ON interactions(connection_id);
CREATE INDEX idx_interactions_user_id ON interactions(user_id);
CREATE INDEX idx_interactions_action_type ON interactions(action_type);
CREATE INDEX idx_interactions_created_at ON interactions(created_at);
CREATE INDEX idx_interactions_stt_latency ON interactions(stt_latency_ms);
CREATE INDEX idx_interactions_llm_latency ON interactions(llm_latency_ms);
```

## What Gets Captured Where

### STT Phase (whisper)

Captured in `transcribeAudio()` return path:

| Field | Source |
|-------|--------|
| `stt_response_json` | Full `WhisperResponse` marshaled to JSON |
| `stt_transcript` | `whisperResp.Text` |
| `stt_word_count` | `len(segments[*].words[*])` |
| `stt_low_prob_words` | Filter words where `probability < 0.80`, store as JSON array |
| `stt_latency_ms` | `time.Since(requestStart).Milliseconds()` |
| `stt_vad_speech_detected` | `1` if transcript is non-empty, `0` if empty |
| `audio_duration_s` | `whisperResp.Duration` |

### LLM Phase

Captured in `processTranscript()` return path:

| Field | Source |
|-------|--------|
| `llm_input_text` | The formatted user message (`buildUserMessage()` output) |
| `llm_system_prompt_hash` | SHA-256 of `buildSystemPrompt()` output |
| `llm_concepts_available` | JSON array of concept IDs passed to system prompt |
| `llm_response_json` | Raw API response body (tool calls, stop_reason, usage) |
| `llm_model` | Model ID from provider config |
| `llm_input_tokens` / `llm_output_tokens` | From API response `usage` field |
| `llm_latency_ms` | `time.Since(requestStart).Milliseconds()` |
| `action_type` | First tool call name |
| `action_subject` | `show_media.subject` |
| `instruction_end_word_idx` | `show_media.instruction_end_word_index` |
| `action_tts_text` | `text_to_speech.text` |
| `action_wait_reason` | `wait_for_more.reason` |

### TTS Phase (piper)

Captured in `executeTTS()` return path:

| Field | Source |
|-------|--------|
| `tts_latency_ms` | `time.Since(requestStart).Milliseconds()` |
| `tts_audio_size_bytes` | `len(wavBytes)` |

### Buffer Management

Captured in action execution (after LLM):

| Field | Source |
|-------|--------|
| `buffer_trim_time_ms` | Cluster timecode passed to `TrimBefore()` |
| `buffer_bytes_before_trim` | `webmParser.BufferLen()` before trim |
| `buffer_bytes_after_trim` | `webmParser.BufferLen()` after trim |
| `accumulated_transcript` | `state.accumulatedTranscript` |

## Audio Blob Storage

When `INTERACTION_LOG_AUDIO=true`, the WebM buffer sent to whisper is saved:

```
data/interactions/
    {date}/
        {interaction_id}.webm
```

- Files are written asynchronously (goroutine) to avoid blocking the pipeline
- `audio_blob_path` stores the relative path from `data/`
- Retention: files older than `INTERACTION_RETENTION_DAYS` (default 30) are
  deleted by a daily cleanup goroutine

When disabled, `audio_blob_path` is NULL and no files are written.

## Implementation

### InteractionLog struct

```go
type InteractionLog struct {
    ID           string
    ConnectionID string
    UserID       *string  // nil if anonymous
    SessionID    *string  // nil if anonymous

    // Populated by each phase
    STT    *STTLog
    LLM    *LLMLog
    TTS    *TTSLog
    Buffer *BufferLog

    TotalLatencyMs int64
    CreatedAt      time.Time
}

type STTLog struct {
    ResponseJSON     json.RawMessage
    Transcript       string
    WordCount        int
    LowProbWords     []LowProbWord
    LatencyMs        int64
    VADSpeechDetected bool
    AudioDurationS   float64
    AudioChunkCount  int
    AudioBlobPath    *string
}

type LowProbWord struct {
    Word        string  `json:"word"`
    Probability float64 `json:"prob"`
    Index       int     `json:"idx"`
}

type LLMLog struct {
    Provider         string
    Model            string
    SystemPromptHash string
    InputText        string
    ConceptsAvailable []string
    ResponseJSON     json.RawMessage
    InputTokens      int
    OutputTokens     int
    LatencyMs        int64
    ActionType       string
    Subject          *string
    MediaSetID       *int64
    TTSText          *string
    WaitReason       *string
    InstructionEndWordIdx int
}

type TTSLog struct {
    LatencyMs      int64
    AudioSizeBytes int
}

type BufferLog struct {
    TrimTimeMs       int64
    BytesBeforeTrim  int
    BytesAfterTrim   int
    AccumulatedTranscript string
}
```

### Integration with processAudio()

```go
func (h *AudioWebSocketHandler) processAudio(ctx context.Context, conn *websocket.Conn, state *connectionState, logger *slog.Logger) {
    interaction := &InteractionLog{
        ID:           generateID(),
        ConnectionID: state.connectionID,
        UserID:       state.userID,       // set during WS handshake (nil if anon)
        SessionID:    state.sessionID,
        CreatedAt:    time.Now(),
    }
    defer h.saveInteraction(interaction)  // always persist, even on error

    // ... existing STT call, populate interaction.STT ...
    // ... existing LLM call, populate interaction.LLM ...
    // ... existing TTS call, populate interaction.TTS ...
    // ... existing buffer trim, populate interaction.Buffer ...

    interaction.TotalLatencyMs = time.Since(interaction.CreatedAt).Milliseconds()
}
```

`saveInteraction()` writes to the database. If `INTERACTION_LOG_AUDIO=true`,
it also kicks off an async goroutine to write the audio blob to disk.

### Provider Changes

The `llm.Provider` interface needs to return token usage and raw response:

```go
type TranscriptResult struct {
    Actions []ToolAction
    // New fields
    RawResponse  json.RawMessage
    Model        string
    InputTokens  int
    OutputTokens int
}
```

Both `anthropic.go` and `openai.go` already receive this data from their
respective APIs — they just need to pass it through instead of discarding it.

Similarly, `transcribeAudio()` already receives the full `WhisperResponse`
struct — it just needs to marshal it to JSON for the log.

## Queries

### STT Accuracy

```sql
-- Words with low confidence, ordered by frequency
SELECT json_extract(value, '$.word') as word,
       COUNT(*) as occurrences,
       AVG(json_extract(value, '$.prob')) as avg_prob
FROM interactions, json_each(interactions.stt_low_prob_words)
WHERE stt_low_prob_words IS NOT NULL
GROUP BY word
ORDER BY occurrences DESC;
```

### LLM Behavior

```sql
-- Action distribution over time (daily)
SELECT DATE(created_at) as day,
       action_type,
       COUNT(*) as count
FROM interactions
GROUP BY day, action_type
ORDER BY day;

-- wait_for_more reasons
SELECT action_wait_reason, COUNT(*) as count
FROM interactions
WHERE action_type = 'wait_for_more'
GROUP BY action_wait_reason
ORDER BY count DESC;

-- Subjects the LLM picks that aren't in the DB
SELECT action_subject, COUNT(*) as count
FROM interactions
WHERE action_type = 'show_media' AND action_media_set_id IS NULL
GROUP BY action_subject
ORDER BY count DESC;
```

### Latency

```sql
-- P50/P90 latencies by phase
SELECT
    action_type,
    COUNT(*) as n,
    -- Whisper
    AVG(stt_latency_ms) as avg_stt_ms,
    -- LLM
    AVG(llm_latency_ms) as avg_llm_ms,
    -- TTS (only when present)
    AVG(CASE WHEN tts_latency_ms IS NOT NULL THEN tts_latency_ms END) as avg_tts_ms,
    -- Total
    AVG(total_latency_ms) as avg_total_ms
FROM interactions
WHERE created_at > datetime('now', '-7 days')
GROUP BY action_type;
```

### Rapid Re-attempts (Implicit Failure Signal)

```sql
SELECT a.id,
       a.stt_transcript as first_transcript,
       b.stt_transcript as retry_transcript,
       a.action_subject as first_subject,
       b.action_subject as retry_subject,
       ROUND((julianday(b.created_at) - julianday(a.created_at)) * 86400, 1) as gap_s
FROM interactions a
JOIN interactions b ON a.connection_id = b.connection_id
    AND b.created_at > a.created_at
    AND (julianday(b.created_at) - julianday(a.created_at)) * 86400 < 5
WHERE a.action_type = 'show_media'
  AND b.action_type = 'show_media';
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `INTERACTION_LOG_AUDIO` | Save audio blobs to disk | `false` |
| `INTERACTION_RETENTION_DAYS` | Days to keep audio blobs | `30` |

## Related Specs

- [audio-timing.md](audio-timing.md) - STT pipeline and buffer management
- [architecture.md](architecture.md) - System overview
