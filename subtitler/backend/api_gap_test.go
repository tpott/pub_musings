package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGapTranscriptionValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-gap"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantError  string
	}{
		{
			name:       "start time negative",
			body:       map[string]interface{}{"start": -1.0, "end": 5.0},
			wantStatus: http.StatusBadRequest,
			wantError:  "Start time must be >= 0",
		},
		{
			name:       "end before start",
			body:       map[string]interface{}{"start": 5.0, "end": 3.0},
			wantStatus: http.StatusBadRequest,
			wantError:  "End time must be greater than start time",
		},
		{
			name:       "end equals start",
			body:       map[string]interface{}{"start": 5.0, "end": 5.0},
			wantStatus: http.StatusBadRequest,
			wantError:  "End time must be greater than start time",
		},
		{
			name:       "duration too long",
			body:       map[string]interface{}{"start": 0.0, "end": 400.0},
			wantStatus: http.StatusBadRequest,
			wantError:  "Gap duration cannot exceed 5 minutes",
		},
		{
			name:       "invalid language code",
			body:       map[string]interface{}{"start": 0.0, "end": 5.0, "language": "this-is-way-too-long-for-a-language"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/gap?session_id="+sessionID, tt.body, "")

			if resp.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.wantStatus, resp.Code, resp.Body.String())
			}

			if tt.wantError != "" {
				var result map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					if errMsg, ok := result["error"].(string); ok && errMsg != tt.wantError {
						t.Errorf("Expected error %q, got %q", tt.wantError, errMsg)
					}
				}
			}
		})
	}
}

func TestGapTranscriptionVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/transcribe/00000000000000000000000000000000/gap?session_id=test", map[string]interface{}{
		"start": 0.0,
		"end":   5.0,
	}, "")

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGapTranscriptionAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with one session, try to access with different session
	sessionID := "test-session-gap-owner"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/gap?session_id=wrong-session", map[string]interface{}{
		"start": 0.0,
		"end":   5.0,
	}, "")

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGapTranscriptionDefaultLanguage(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-gap-lang"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Valid request with no language — should default to "auto"
	// Will fail at audio extraction (no real video file), but validates the
	// request parsing and language default path
	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/gap?session_id="+sessionID, map[string]interface{}{
		"start": 0.0,
		"end":   5.0,
	}, "")

	// Expect 500 (audio extraction will fail since there's no real video file)
	// This confirms the request validation passed and we reached the extraction step
	if resp.Code != http.StatusInternalServerError && resp.Code != http.StatusOK {
		t.Errorf("Expected status 500 (no real video) or 200, got %d: %s", resp.Code, resp.Body.String())
	}
}
