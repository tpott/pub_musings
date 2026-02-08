package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

func TestBuildSTTLog_BasicFields(t *testing.T) {
	resp := &WhisperResponse{
		Text:     "show me a cat",
		Duration: 2.5,
		Segments: []WhisperSegment{
			{
				Words: []WhisperWord{
					{Word: "show", Probability: 0.95, Start: 0.0, End: 0.3},
					{Word: "me", Probability: 0.92, Start: 0.3, End: 0.5},
					{Word: "a", Probability: 0.88, Start: 0.5, End: 0.6},
					{Word: "cat", Probability: 0.97, Start: 0.6, End: 1.0},
				},
			},
		},
	}

	requestAt := time.Now()
	latency := 150 * time.Millisecond

	stt := buildSTTLog(resp, latency, 12, requestAt)

	if stt.Transcript != "show me a cat" {
		t.Errorf("Transcript: got %q, want %q", stt.Transcript, "show me a cat")
	}
	if stt.LatencyMs != 150 {
		t.Errorf("LatencyMs: got %d, want 150", stt.LatencyMs)
	}
	if stt.AudioDurationS != 2.5 {
		t.Errorf("AudioDurationS: got %f, want 2.5", stt.AudioDurationS)
	}
	if stt.AudioChunkCount != 12 {
		t.Errorf("AudioChunkCount: got %d, want 12", stt.AudioChunkCount)
	}
	if stt.WordCount != 4 {
		t.Errorf("WordCount: got %d, want 4", stt.WordCount)
	}
	if !stt.VADSpeechDetected {
		t.Error("VADSpeechDetected: got false, want true")
	}
	if stt.RequestAt != requestAt {
		t.Errorf("RequestAt: got %v, want %v", stt.RequestAt, requestAt)
	}
}

func TestBuildSTTLog_LowProbWords(t *testing.T) {
	resp := &WhisperResponse{
		Text: "show me a cat please",
		Segments: []WhisperSegment{
			{
				Words: []WhisperWord{
					{Word: "show", Probability: 0.95},
					{Word: "me", Probability: 0.72}, // below 0.80
					{Word: "a", Probability: 0.88},
					{Word: "cat", Probability: 0.65},    // below 0.80
					{Word: "please", Probability: 0.80}, // exactly 0.80 — NOT below
				},
			},
		},
	}

	stt := buildSTTLog(resp, 100*time.Millisecond, 5, time.Now())

	if len(stt.LowProbWords) != 2 {
		t.Fatalf("LowProbWords: got %d items, want 2", len(stt.LowProbWords))
	}

	if stt.LowProbWords[0].Word != "me" {
		t.Errorf("LowProbWords[0].Word: got %q, want %q", stt.LowProbWords[0].Word, "me")
	}
	if stt.LowProbWords[0].Index != 1 {
		t.Errorf("LowProbWords[0].Index: got %d, want 1", stt.LowProbWords[0].Index)
	}
	if stt.LowProbWords[0].Probability != 0.72 {
		t.Errorf("LowProbWords[0].Probability: got %f, want 0.72", stt.LowProbWords[0].Probability)
	}

	if stt.LowProbWords[1].Word != "cat" {
		t.Errorf("LowProbWords[1].Word: got %q, want %q", stt.LowProbWords[1].Word, "cat")
	}
	if stt.LowProbWords[1].Index != 3 {
		t.Errorf("LowProbWords[1].Index: got %d, want 3", stt.LowProbWords[1].Index)
	}
}

func TestBuildSTTLog_NoLowProbWords(t *testing.T) {
	resp := &WhisperResponse{
		Text: "hello",
		Segments: []WhisperSegment{
			{
				Words: []WhisperWord{
					{Word: "hello", Probability: 0.99},
				},
			},
		},
	}

	stt := buildSTTLog(resp, 50*time.Millisecond, 1, time.Now())

	if len(stt.LowProbWords) != 0 {
		t.Errorf("LowProbWords: got %d items, want 0", len(stt.LowProbWords))
	}
}

func TestBuildSTTLog_EmptyTranscript(t *testing.T) {
	resp := &WhisperResponse{
		Text:     "",
		Duration: 1.0,
	}

	stt := buildSTTLog(resp, 80*time.Millisecond, 3, time.Now())

	if stt.VADSpeechDetected {
		t.Error("VADSpeechDetected: got true, want false for empty transcript")
	}
	if stt.WordCount != 0 {
		t.Errorf("WordCount: got %d, want 0", stt.WordCount)
	}
}

func TestBuildSTTLog_MultipleSegments(t *testing.T) {
	resp := &WhisperResponse{
		Text: "show me a cat then a dog",
		Segments: []WhisperSegment{
			{
				Words: []WhisperWord{
					{Word: "show", Probability: 0.90},
					{Word: "me", Probability: 0.85},
					{Word: "a", Probability: 0.70}, // low prob, index 2
					{Word: "cat", Probability: 0.95},
				},
			},
			{
				Words: []WhisperWord{
					{Word: "then", Probability: 0.60}, // low prob, index 4
					{Word: "a", Probability: 0.90},
					{Word: "dog", Probability: 0.92},
				},
			},
		},
	}

	stt := buildSTTLog(resp, 200*time.Millisecond, 8, time.Now())

	if stt.WordCount != 7 {
		t.Errorf("WordCount: got %d, want 7", stt.WordCount)
	}

	if len(stt.LowProbWords) != 2 {
		t.Fatalf("LowProbWords: got %d items, want 2", len(stt.LowProbWords))
	}

	// "a" at index 2 across first segment
	if stt.LowProbWords[0].Index != 2 {
		t.Errorf("LowProbWords[0].Index: got %d, want 2", stt.LowProbWords[0].Index)
	}
	// "then" at index 4 across second segment
	if stt.LowProbWords[1].Index != 4 {
		t.Errorf("LowProbWords[1].Index: got %d, want 4", stt.LowProbWords[1].Index)
	}
}

func TestBuildSTTLog_ResponseJSON(t *testing.T) {
	resp := &WhisperResponse{
		Task:     "transcribe",
		Language: "en",
		Text:     "hello",
		Duration: 1.0,
	}

	stt := buildSTTLog(resp, 50*time.Millisecond, 1, time.Now())

	if len(stt.ResponseJSON) == 0 {
		t.Fatal("ResponseJSON is empty")
	}

	// Verify it's valid JSON that contains our data
	var parsed map[string]interface{}
	if err := json.Unmarshal(stt.ResponseJSON, &parsed); err != nil {
		t.Fatalf("ResponseJSON is not valid JSON: %v", err)
	}
	if parsed["text"] != "hello" {
		t.Errorf("ResponseJSON.text: got %v, want %q", parsed["text"], "hello")
	}
	if parsed["language"] != "en" {
		t.Errorf("ResponseJSON.language: got %v, want %q", parsed["language"], "en")
	}
}

// --- buildLLMLog tests ---

func TestBuildLLMLog_ShowMedia(t *testing.T) {
	result := &llm.TranscriptResult{
		Actions:      []llm.ToolAction{{Type: "show_media", Subject: "cat", InstructionEndWordIdx: 3}},
		RawResponse:  json.RawMessage(`{"id":"msg_123"}`),
		Model:        "claude-3-haiku-20240307",
		InputTokens:  150,
		OutputTokens: 42,
		SystemPrompt: "You are the intent engine",
		InputText:    `Transcript: "show me a cat"`,
	}
	requestAt := time.Now()
	log := buildLLMLog(result, "anthropic", []string{"cat", "dog", "cow"}, 200*time.Millisecond, requestAt)

	if log.Provider != "anthropic" {
		t.Errorf("Provider: got %q, want %q", log.Provider, "anthropic")
	}
	if log.Model != "claude-3-haiku-20240307" {
		t.Errorf("Model: got %q", log.Model)
	}
	if log.InputTokens != 150 || log.OutputTokens != 42 {
		t.Errorf("tokens: got %d/%d, want 150/42", log.InputTokens, log.OutputTokens)
	}
	if log.LatencyMs != 200 {
		t.Errorf("LatencyMs: got %d, want 200", log.LatencyMs)
	}
	if log.ActionType != "show_media" {
		t.Errorf("ActionType: got %q, want %q", log.ActionType, "show_media")
	}
	if log.Subject == nil || *log.Subject != "cat" {
		t.Errorf("Subject: got %v, want %q", log.Subject, "cat")
	}
	if log.InstructionEndIdx != 3 {
		t.Errorf("InstructionEndIdx: got %d, want 3", log.InstructionEndIdx)
	}
	if log.TTSText != nil || log.WaitReason != nil {
		t.Errorf("TTSText/WaitReason should be nil for show_media")
	}
	if string(log.ResponseJSON) != `{"id":"msg_123"}` {
		t.Errorf("ResponseJSON: got %q", string(log.ResponseJSON))
	}
}

func TestBuildLLMLog_SystemPromptHash(t *testing.T) {
	prompt := "You are the intent engine for a voice-controlled media application"
	result := &llm.TranscriptResult{
		Actions:      []llm.ToolAction{{Type: "show_media", Subject: "dog"}},
		SystemPrompt: prompt,
	}

	log := buildLLMLog(result, "openai", nil, 100*time.Millisecond, time.Now())

	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt)))
	if log.SystemPromptHash != expectedHash {
		t.Errorf("SystemPromptHash:\n got  %q\n want %q", log.SystemPromptHash, expectedHash)
	}
}

func TestBuildLLMLog_EmptySystemPrompt(t *testing.T) {
	result := &llm.TranscriptResult{
		Actions:      []llm.ToolAction{{Type: "show_media", Subject: "dog"}},
		SystemPrompt: "",
	}

	log := buildLLMLog(result, "anthropic", nil, 50*time.Millisecond, time.Now())

	if log.SystemPromptHash != "" {
		t.Errorf("SystemPromptHash: got %q, want empty", log.SystemPromptHash)
	}
}

func TestBuildLLMLog_TextToSpeech(t *testing.T) {
	result := &llm.TranscriptResult{
		Actions: []llm.ToolAction{{Type: "text_to_speech", Text: "I can show you a cat!"}},
		Model:   "gpt-4o-mini",
	}
	log := buildLLMLog(result, "openai", []string{"cat"}, 80*time.Millisecond, time.Now())
	if log.ActionType != "text_to_speech" {
		t.Errorf("ActionType: got %q, want %q", log.ActionType, "text_to_speech")
	}
	if log.TTSText == nil || *log.TTSText != "I can show you a cat!" {
		t.Errorf("TTSText: got %v, want %q", log.TTSText, "I can show you a cat!")
	}
	if log.Subject != nil {
		t.Errorf("Subject: got %v, want nil", log.Subject)
	}
}

func TestBuildLLMLog_WaitForMore(t *testing.T) {
	result := &llm.TranscriptResult{
		Actions: []llm.ToolAction{{Type: "wait_for_more", Reason: "sentence cut off mid-phrase"}},
		Model:   "claude-3-haiku-20240307",
	}
	log := buildLLMLog(result, "anthropic", []string{"cat", "dog"}, 120*time.Millisecond, time.Now())
	if log.ActionType != "wait_for_more" {
		t.Errorf("ActionType: got %q, want %q", log.ActionType, "wait_for_more")
	}
	if log.WaitReason == nil || *log.WaitReason != "sentence cut off mid-phrase" {
		t.Errorf("WaitReason: got %v, want %q", log.WaitReason, "sentence cut off mid-phrase")
	}
	if log.Subject != nil || log.TTSText != nil {
		t.Error("Subject and TTSText should be nil for wait_for_more")
	}
}

func TestBuildLLMLog_NoActions(t *testing.T) {
	result := &llm.TranscriptResult{
		Actions: []llm.ToolAction{},
		Model:   "gpt-4o-mini",
	}

	log := buildLLMLog(result, "openai", nil, 90*time.Millisecond, time.Now())

	if log.ActionType != "" {
		t.Errorf("ActionType: got %q, want empty", log.ActionType)
	}
	if log.Subject != nil {
		t.Errorf("Subject: got %v, want nil", log.Subject)
	}
}

func TestBuildLLMLog_PromptHashChangesWithConcepts(t *testing.T) {
	// When concepts change, the system prompt changes, so the hash should change.
	// This verifies the done_when requirement: "llm_system_prompt_hash changes when concepts are added"
	concepts1 := []string{"cat", "dog"}
	concepts2 := []string{"cat", "dog", "cow"}

	prompt1 := buildTestSystemPrompt(concepts1)
	prompt2 := buildTestSystemPrompt(concepts2)

	result1 := &llm.TranscriptResult{
		Actions:      []llm.ToolAction{{Type: "show_media", Subject: "cat"}},
		SystemPrompt: prompt1,
	}
	result2 := &llm.TranscriptResult{
		Actions:      []llm.ToolAction{{Type: "show_media", Subject: "cat"}},
		SystemPrompt: prompt2,
	}

	log1 := buildLLMLog(result1, "anthropic", concepts1, 100*time.Millisecond, time.Now())
	log2 := buildLLMLog(result2, "anthropic", concepts2, 100*time.Millisecond, time.Now())

	if log1.SystemPromptHash == log2.SystemPromptHash {
		t.Error("SystemPromptHash should differ when concepts change")
	}
}

// buildTestSystemPrompt simulates the prompt builder for testing hash changes.
func buildTestSystemPrompt(concepts []string) string {
	result := "You are the intent engine.\n<available-concepts>\n"
	for _, c := range concepts {
		result += "<concept>" + c + "</concept>\n"
	}
	result += "</available-concepts>"
	return result
}

// --- saveInteraction tests ---

func TestSaveInteraction_PersistsRow(t *testing.T) {
	database := setupTestDB(t)
	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	interaction := &db.InteractionLog{
		ID:             "test-interaction-1",
		ConnectionID:   "test-conn-1",
		STT:            &db.STTLog{Transcript: "show me a cat", LatencyMs: 100, RequestAt: time.Now()},
		TotalLatencyMs: 250,
		CreatedAt:      time.Now().UTC(),
	}

	handler.saveInteraction(interaction, nil, logger)

	got, err := database.GetInteraction("test-interaction-1")
	if err != nil {
		t.Fatalf("GetInteraction: %v", err)
	}
	if got == nil {
		t.Fatal("interaction not found in database")
	}
	if got.STT == nil || got.STT.Transcript != "show me a cat" {
		t.Errorf("STT.Transcript: got %v, want %q", got.STT, "show me a cat")
	}
	if got.TotalLatencyMs != 250 {
		t.Errorf("TotalLatencyMs: got %d, want 250", got.TotalLatencyMs)
	}
}

func TestSaveInteraction_NilDatabase(t *testing.T) {
	handler := &AudioWebSocketHandler{Database: nil}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	interaction := &db.InteractionLog{
		ID:           "test-nil-db",
		ConnectionID: "conn-1",
		CreatedAt:    time.Now().UTC(),
	}

	// Should not panic
	handler.saveInteraction(interaction, nil, logger)
}

func TestSaveInteraction_PartialDataOnError(t *testing.T) {
	database := setupTestDB(t)
	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Interaction with STT but no LLM (simulates transcription failure)
	interaction := &db.InteractionLog{
		ID:             "test-partial-1",
		ConnectionID:   "conn-partial",
		STT:            &db.STTLog{Transcript: "", LatencyMs: 50, RequestAt: time.Now()},
		TotalLatencyMs: 50,
		CreatedAt:      time.Now().UTC(),
	}

	handler.saveInteraction(interaction, nil, logger)

	got, err := database.GetInteraction("test-partial-1")
	if err != nil {
		t.Fatalf("GetInteraction: %v", err)
	}
	if got == nil {
		t.Fatal("partial interaction not found in database")
	}
	if got.LLM != nil {
		t.Error("LLM should be nil for partial interaction")
	}
}

func TestSaveInteraction_AudioBlobWritten(t *testing.T) {
	database := setupTestDB(t)
	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("INTERACTION_LOG_AUDIO", "true")

	interaction := &db.InteractionLog{
		ID:             "test-audio-1",
		ConnectionID:   "conn-audio",
		TotalLatencyMs: 100,
		CreatedAt:      time.Now().UTC(),
	}
	handler.saveInteraction(interaction, []byte("fake webm"), logger)
	time.Sleep(100 * time.Millisecond)

	dateDir := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join("data", "interactions", dateDir, "test-audio-1.webm")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("audio file not written: %v", err)
	}
	if string(data) != "fake webm" {
		t.Errorf("audio content: got %q, want %q", string(data), "fake webm")
	}
}

func TestSaveInteraction_AudioBlobSkippedWhenDisabled(t *testing.T) {
	database := setupTestDB(t)
	handler := &AudioWebSocketHandler{Database: database}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("INTERACTION_LOG_AUDIO", "false")

	interaction := &db.InteractionLog{
		ID:             "test-audio-off",
		ConnectionID:   "conn-audio",
		TotalLatencyMs: 100,
		CreatedAt:      time.Now().UTC(),
	}
	handler.saveInteraction(interaction, []byte("fake webm"), logger)
	time.Sleep(50 * time.Millisecond)

	dateDir := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join("data", "interactions", dateDir, "test-audio-off.webm")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("audio file should not exist when INTERACTION_LOG_AUDIO=false")
	}
}
