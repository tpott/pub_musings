package api

import (
	"encoding/json"
	"testing"
	"time"
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
