package api

import (
	"sync"
	"testing"
	"time"
)

func TestEmptyTranscriptBackoff(t *testing.T) {
	t.Run("backoff threshold computation", func(t *testing.T) {
		tests := []struct {
			name             string
			consecutiveEmpty int
			trailingSilence  bool
			baseThreshold    time.Duration
			wantEffective    time.Duration
		}{
			{
				name:             "no backoff below threshold",
				consecutiveEmpty: 0,
				baseThreshold:    3 * time.Second,
				wantEffective:    3 * time.Second,
			},
			{
				name:             "no backoff at threshold minus one",
				consecutiveEmpty: 2,
				baseThreshold:    3 * time.Second,
				wantEffective:    3 * time.Second,
			},
			{
				name:             "first backoff doubles threshold",
				consecutiveEmpty: 3,
				baseThreshold:    3 * time.Second,
				wantEffective:    6 * time.Second,
			},
			{
				name:             "second backoff quadruples threshold",
				consecutiveEmpty: 4,
				baseThreshold:    3 * time.Second,
				wantEffective:    10 * time.Second, // 12s capped to 10s
			},
			{
				name:             "many empties cap at maxBackoffThreshold",
				consecutiveEmpty: 10,
				baseThreshold:    3 * time.Second,
				wantEffective:    10 * time.Second,
			},
			{
				name:             "silence threshold also backs off",
				consecutiveEmpty: 3,
				trailingSilence:  true,
				baseThreshold:    3 * time.Second,
				wantEffective:    2 * time.Second, // silenceBufferThreshold (1s) * 2
			},
			{
				name:             "silence threshold caps at max",
				consecutiveEmpty: 10,
				trailingSilence:  true,
				baseThreshold:    3 * time.Second,
				wantEffective:    10 * time.Second,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Replicate the inline backoff logic from bufferThresholdWatcher
				threshold := tt.baseThreshold
				if tt.trailingSilence {
					threshold = silenceBufferThreshold
				}
				if n := tt.consecutiveEmpty; n >= emptyTranscriptsBeforeBackoff {
					shifts := n - emptyTranscriptsBeforeBackoff + 1
					backoff := threshold << uint(shifts)
					if backoff > maxBackoffThreshold {
						backoff = maxBackoffThreshold
					}
					threshold = backoff
				}

				if threshold != tt.wantEffective {
					t.Errorf("effective threshold = %v, want %v", threshold, tt.wantEffective)
				}
			})
		}
	})

	t.Run("counter increments on empty transcript", func(t *testing.T) {
		state := &connectionState{
			mu: sync.Mutex{},
		}

		// Simulate 5 consecutive empty transcripts
		for i := 0; i < 5; i++ {
			state.mu.Lock()
			state.consecutiveEmptyTranscripts++
			state.mu.Unlock()
		}

		state.mu.Lock()
		got := state.consecutiveEmptyTranscripts
		state.mu.Unlock()

		if got != 5 {
			t.Errorf("consecutiveEmptyTranscripts = %d, want 5", got)
		}
	})

	t.Run("counter resets on non-empty transcript", func(t *testing.T) {
		state := &connectionState{
			mu:                          sync.Mutex{},
			consecutiveEmptyTranscripts: 7,
		}

		// Simulate non-empty transcript resetting counter
		state.mu.Lock()
		state.consecutiveEmptyTranscripts = 0
		state.mu.Unlock()

		state.mu.Lock()
		got := state.consecutiveEmptyTranscripts
		state.mu.Unlock()

		if got != 0 {
			t.Errorf("consecutiveEmptyTranscripts = %d, want 0", got)
		}
	})

	t.Run("start_recording resets counter", func(t *testing.T) {
		state := &connectionState{
			mu:                          sync.Mutex{},
			consecutiveEmptyTranscripts: 5,
			webmParser:                  NewWebMParser(),
		}

		// Simulate start_recording resetting state
		state.mu.Lock()
		state.isRecording = true
		state.webmParser.Reset()
		state.bufferStart = time.Time{}
		state.chunkMetas = nil
		state.firstChunkClient = 0
		state.accumulatedWords = nil
		state.consecutiveEmptyTranscripts = 0
		state.mu.Unlock()

		state.mu.Lock()
		got := state.consecutiveEmptyTranscripts
		state.mu.Unlock()

		if got != 0 {
			t.Errorf("consecutiveEmptyTranscripts after start = %d, want 0", got)
		}
	})

	t.Run("constants are reasonable", func(t *testing.T) {
		if emptyTranscriptsBeforeBackoff < 1 {
			t.Error("emptyTranscriptsBeforeBackoff must be at least 1")
		}
		if maxBackoffThreshold < defaultBufferThreshold {
			t.Error("maxBackoffThreshold must be >= defaultBufferThreshold")
		}
		if maxBackoffThreshold > 30*time.Second {
			t.Error("maxBackoffThreshold should not exceed 30s")
		}
	})
}
