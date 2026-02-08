package api

import (
	"encoding/json"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
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
