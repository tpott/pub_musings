package db

import (
	"encoding/json"
	"testing"
	"time"
)

func TestInitInteractionsCreatesTable(t *testing.T) {
	db := openTestDB(t)

	var name string
	err := db.conn.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='interactions'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("interactions table not found: %v", err)
	}
}

func TestInitInteractionsCreatesIndexes(t *testing.T) {
	db := openTestDB(t)

	indexes := []string{
		"idx_interactions_connection_id",
		"idx_interactions_user_id",
		"idx_interactions_action_type",
		"idx_interactions_created_at",
		"idx_interactions_stt_latency",
		"idx_interactions_llm_latency",
	}
	for _, idx := range indexes {
		var name string
		err := db.conn.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

func TestInsertInteraction_FullRoundtrip(t *testing.T) {
	db := openTestDB(t)

	userID := "user-123"
	sessionID := "session-456"
	subject := "cat"
	mediaSetID := int64(42)
	blobPath := "interactions/2026-02-08/abc123.webm"
	now := time.Now().UTC().Truncate(time.Second)

	log := &InteractionLog{
		ID:           "interaction-001",
		ConnectionID: "conn-abc",
		UserID:       &userID,
		SessionID:    &sessionID,
		STT: &STTLog{
			ResponseJSON:      json.RawMessage(`{"text":"show me a cat"}`),
			Transcript:        "show me a cat",
			WordCount:         4,
			LowProbWords:      []LowProbWord{{Word: "cat", Probability: 0.72, Index: 3}},
			LatencyMs:         150,
			VADSpeechDetected: true,
			AudioDurationS:    2.5,
			AudioChunkCount:   12,
			AudioBlobPath:     &blobPath,
			RequestAt:         now,
		},
		LLM: &LLMLog{
			Provider:          "anthropic",
			Model:             "claude-haiku-4-5-20251001",
			SystemPromptHash:  "abc123hash",
			InputText:         "[0]show [1]me [2]a [3]cat",
			ConceptsAvailable: []string{"cat", "dog", "duck"},
			ResponseJSON:      json.RawMessage(`{"tool_calls":[{"name":"show_media"}]}`),
			InputTokens:       100,
			OutputTokens:      25,
			LatencyMs:         200,
			ActionType:        "show_media",
			Subject:           &subject,
			MediaSetID:        &mediaSetID,
			InstructionEndIdx: 3,
			RequestAt:         now,
		},
		TTS: &TTSLog{
			LatencyMs:      80,
			AudioSizeBytes: 44100,
			RequestAt:      now,
		},
		Buffer: &BufferLog{
			TrimTimeMs:            1500,
			BytesBeforeTrim:       65536,
			BytesAfterTrim:        8192,
			AccumulatedTranscript: "show me a cat",
		},
		TotalLatencyMs: 430,
		CreatedAt:      now,
	}

	if err := db.InsertInteraction(log); err != nil {
		t.Fatalf("InsertInteraction failed: %v", err)
	}

	got, err := db.GetInteraction("interaction-001")
	if err != nil {
		t.Fatalf("GetInteraction failed: %v", err)
	}

	// Verify top-level fields
	if got.ID != "interaction-001" {
		t.Errorf("ID: got %q, want %q", got.ID, "interaction-001")
	}
	if got.ConnectionID != "conn-abc" {
		t.Errorf("ConnectionID: got %q, want %q", got.ConnectionID, "conn-abc")
	}
	if got.UserID == nil || *got.UserID != "user-123" {
		t.Errorf("UserID: got %v, want %q", got.UserID, "user-123")
	}
	if got.SessionID == nil || *got.SessionID != "session-456" {
		t.Errorf("SessionID: got %v, want %q", got.SessionID, "session-456")
	}
	if got.TotalLatencyMs != 430 {
		t.Errorf("TotalLatencyMs: got %d, want 430", got.TotalLatencyMs)
	}

	// Verify STT
	if got.STT == nil {
		t.Fatal("STT is nil")
	}
	if got.STT.Transcript != "show me a cat" {
		t.Errorf("STT.Transcript: got %q, want %q", got.STT.Transcript, "show me a cat")
	}
	if got.STT.WordCount != 4 {
		t.Errorf("STT.WordCount: got %d, want 4", got.STT.WordCount)
	}
	if got.STT.LatencyMs != 150 {
		t.Errorf("STT.LatencyMs: got %d, want 150", got.STT.LatencyMs)
	}
	if !got.STT.VADSpeechDetected {
		t.Error("STT.VADSpeechDetected: got false, want true")
	}
	if got.STT.AudioDurationS != 2.5 {
		t.Errorf("STT.AudioDurationS: got %f, want 2.5", got.STT.AudioDurationS)
	}
	if got.STT.AudioChunkCount != 12 {
		t.Errorf("STT.AudioChunkCount: got %d, want 12", got.STT.AudioChunkCount)
	}
	if got.STT.AudioBlobPath == nil || *got.STT.AudioBlobPath != blobPath {
		t.Errorf("STT.AudioBlobPath: got %v, want %q", got.STT.AudioBlobPath, blobPath)
	}
	if len(got.STT.LowProbWords) != 1 {
		t.Fatalf("STT.LowProbWords: got %d items, want 1", len(got.STT.LowProbWords))
	}
	if got.STT.LowProbWords[0].Word != "cat" {
		t.Errorf("LowProbWord.Word: got %q, want %q", got.STT.LowProbWords[0].Word, "cat")
	}
	if got.STT.LowProbWords[0].Probability != 0.72 {
		t.Errorf("LowProbWord.Probability: got %f, want 0.72", got.STT.LowProbWords[0].Probability)
	}

	// Verify LLM
	if got.LLM == nil {
		t.Fatal("LLM is nil")
	}
	if got.LLM.Provider != "anthropic" {
		t.Errorf("LLM.Provider: got %q, want %q", got.LLM.Provider, "anthropic")
	}
	if got.LLM.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("LLM.Model: got %q, want %q", got.LLM.Model, "claude-haiku-4-5-20251001")
	}
	if got.LLM.ActionType != "show_media" {
		t.Errorf("LLM.ActionType: got %q, want %q", got.LLM.ActionType, "show_media")
	}
	if got.LLM.Subject == nil || *got.LLM.Subject != "cat" {
		t.Errorf("LLM.Subject: got %v, want %q", got.LLM.Subject, "cat")
	}
	if got.LLM.MediaSetID == nil || *got.LLM.MediaSetID != 42 {
		t.Errorf("LLM.MediaSetID: got %v, want 42", got.LLM.MediaSetID)
	}
	if got.LLM.InputTokens != 100 {
		t.Errorf("LLM.InputTokens: got %d, want 100", got.LLM.InputTokens)
	}
	if got.LLM.OutputTokens != 25 {
		t.Errorf("LLM.OutputTokens: got %d, want 25", got.LLM.OutputTokens)
	}
	if got.LLM.LatencyMs != 200 {
		t.Errorf("LLM.LatencyMs: got %d, want 200", got.LLM.LatencyMs)
	}
	if got.LLM.InstructionEndIdx != 3 {
		t.Errorf("LLM.InstructionEndIdx: got %d, want 3", got.LLM.InstructionEndIdx)
	}
	if len(got.LLM.ConceptsAvailable) != 3 {
		t.Fatalf("LLM.ConceptsAvailable: got %d items, want 3", len(got.LLM.ConceptsAvailable))
	}

	// Verify TTS
	if got.TTS == nil {
		t.Fatal("TTS is nil")
	}
	if got.TTS.LatencyMs != 80 {
		t.Errorf("TTS.LatencyMs: got %d, want 80", got.TTS.LatencyMs)
	}
	if got.TTS.AudioSizeBytes != 44100 {
		t.Errorf("TTS.AudioSizeBytes: got %d, want 44100", got.TTS.AudioSizeBytes)
	}

	// Verify Buffer
	if got.Buffer == nil {
		t.Fatal("Buffer is nil")
	}
	if got.Buffer.TrimTimeMs != 1500 {
		t.Errorf("Buffer.TrimTimeMs: got %d, want 1500", got.Buffer.TrimTimeMs)
	}
	if got.Buffer.BytesBeforeTrim != 65536 {
		t.Errorf("Buffer.BytesBeforeTrim: got %d, want 65536", got.Buffer.BytesBeforeTrim)
	}
	if got.Buffer.BytesAfterTrim != 8192 {
		t.Errorf("Buffer.BytesAfterTrim: got %d, want 8192", got.Buffer.BytesAfterTrim)
	}
	if got.Buffer.AccumulatedTranscript != "show me a cat" {
		t.Errorf("Buffer.AccumulatedTranscript: got %q, want %q", got.Buffer.AccumulatedTranscript, "show me a cat")
	}
}

func TestInsertInteraction_AnonymousUser(t *testing.T) {
	db := openTestDB(t)

	log := &InteractionLog{
		ID:           "interaction-anon",
		ConnectionID: "conn-xyz",
		UserID:       nil,
		SessionID:    nil,
		LLM: &LLMLog{
			Provider:   "openai",
			Model:      "gpt-4",
			ActionType: "show_media",
			RequestAt:  time.Now().UTC(),
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := db.InsertInteraction(log); err != nil {
		t.Fatalf("InsertInteraction failed: %v", err)
	}

	got, err := db.GetInteraction("interaction-anon")
	if err != nil {
		t.Fatalf("GetInteraction failed: %v", err)
	}

	if got.UserID != nil {
		t.Errorf("UserID: got %v, want nil", got.UserID)
	}
	if got.SessionID != nil {
		t.Errorf("SessionID: got %v, want nil", got.SessionID)
	}
	if got.STT != nil {
		t.Errorf("STT: got %v, want nil", got.STT)
	}
	if got.TTS != nil {
		t.Errorf("TTS: got %v, want nil", got.TTS)
	}
	if got.Buffer != nil {
		t.Errorf("Buffer: got %v, want nil", got.Buffer)
	}
}

func TestInsertInteraction_WaitForMore(t *testing.T) {
	db := openTestDB(t)

	waitReason := "incomplete sentence"
	log := &InteractionLog{
		ID:           "interaction-wait",
		ConnectionID: "conn-wait",
		STT: &STTLog{
			Transcript:        "show me",
			WordCount:         2,
			VADSpeechDetected: true,
			LatencyMs:         100,
			RequestAt:         time.Now().UTC(),
		},
		LLM: &LLMLog{
			Provider:   "anthropic",
			Model:      "claude-haiku-4-5-20251001",
			ActionType: "wait_for_more",
			WaitReason: &waitReason,
			LatencyMs:  50,
			RequestAt:  time.Now().UTC(),
		},
		Buffer: &BufferLog{
			AccumulatedTranscript: "show me",
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := db.InsertInteraction(log); err != nil {
		t.Fatalf("InsertInteraction failed: %v", err)
	}

	got, err := db.GetInteraction("interaction-wait")
	if err != nil {
		t.Fatalf("GetInteraction failed: %v", err)
	}

	if got.LLM.ActionType != "wait_for_more" {
		t.Errorf("ActionType: got %q, want %q", got.LLM.ActionType, "wait_for_more")
	}
	if got.LLM.WaitReason == nil || *got.LLM.WaitReason != "incomplete sentence" {
		t.Errorf("WaitReason: got %v, want %q", got.LLM.WaitReason, "incomplete sentence")
	}
	if got.Buffer.AccumulatedTranscript != "show me" {
		t.Errorf("AccumulatedTranscript: got %q, want %q", got.Buffer.AccumulatedTranscript, "show me")
	}
}

func TestInsertInteraction_TTSAction(t *testing.T) {
	db := openTestDB(t)

	ttsText := "Here is a cat!"
	log := &InteractionLog{
		ID:           "interaction-tts",
		ConnectionID: "conn-tts",
		LLM: &LLMLog{
			Provider:   "anthropic",
			Model:      "claude-haiku-4-5-20251001",
			ActionType: "text_to_speech",
			TTSText:    &ttsText,
			RequestAt:  time.Now().UTC(),
		},
		TTS: &TTSLog{
			LatencyMs:      120,
			AudioSizeBytes: 88200,
			RequestAt:      time.Now().UTC(),
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := db.InsertInteraction(log); err != nil {
		t.Fatalf("InsertInteraction failed: %v", err)
	}

	got, err := db.GetInteraction("interaction-tts")
	if err != nil {
		t.Fatalf("GetInteraction failed: %v", err)
	}

	if got.LLM.ActionType != "text_to_speech" {
		t.Errorf("ActionType: got %q, want %q", got.LLM.ActionType, "text_to_speech")
	}
	if got.LLM.TTSText == nil || *got.LLM.TTSText != "Here is a cat!" {
		t.Errorf("TTSText: got %v, want %q", got.LLM.TTSText, "Here is a cat!")
	}
	if got.TTS.LatencyMs != 120 {
		t.Errorf("TTS.LatencyMs: got %d, want 120", got.TTS.LatencyMs)
	}
	if got.TTS.AudioSizeBytes != 88200 {
		t.Errorf("TTS.AudioSizeBytes: got %d, want 88200", got.TTS.AudioSizeBytes)
	}
}

func TestInitInteractionsIdempotent(t *testing.T) {
	db := openTestDB(t)

	// Should not error when called a second time
	if err := db.InitInteractions(); err != nil {
		t.Fatalf("second InitInteractions failed: %v", err)
	}
}
