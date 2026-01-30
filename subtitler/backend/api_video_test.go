package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/ratelimit"
)

func TestListVideos(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user and video
	token := ts.createTestUser(t, "videos@example.com", "ValidPassword123!")

	// Get user ID from /me
	resp := ts.doRequest("GET", "/api/auth/me", nil, token)
	var meResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&meResult)
	userID := meResult.User.ID

	// Create video for this user
	ts.createTestVideo(t, &userID, nil)

	// List videos
	resp = ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)

	if len(listResult.Videos) != 1 {
		t.Errorf("Expected 1 video, got %d", len(listResult.Videos))
	}
}

func TestListVideosBySession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-123"

	// Create video with session ID
	ts.createTestVideo(t, nil, &sessionID)

	// List videos by session
	resp := ts.doRequest("GET", "/api/videos?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)

	if len(listResult.Videos) != 1 {
		t.Errorf("Expected 1 video, got %d", len(listResult.Videos))
	}
}

func TestListVideosNoFilterRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create videos that should NOT be accessible without filter
	userID := "some-user"
	sessionID := "some-session"
	ts.createTestVideo(t, &userID, nil)
	ts.createTestVideo(t, nil, &sessionID)

	// Try to list videos without auth or session_id
	// This MUST be rejected to prevent privacy leak
	resp := ts.doRequest("GET", "/api/videos", nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 (Bad Request), got %d - anonymous requests without session_id should be rejected", resp.Code)
	}

	var errResult struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResult)

	if errResult.Error == "" {
		t.Error("Expected error message in response")
	}
}

func TestListVideosExpiresAt(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Test 1: Anonymous video should expire in 48 hours
	sessionID := "anon-session"
	ts.createTestVideo(t, nil, &sessionID)

	resp := ts.doRequest("GET", "/api/videos?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var anonResult struct {
		Videos []struct {
			ID        string  `json:"id"`
			UserID    *string `json:"user_id"`
			ExpiresAt string  `json:"expires_at"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&anonResult)

	if len(anonResult.Videos) != 1 {
		t.Fatalf("Expected 1 video, got %d", len(anonResult.Videos))
	}

	if anonResult.Videos[0].ExpiresAt == "" {
		t.Error("Expected expires_at to be set for anonymous video")
	}

	// Parse the expiry time and verify it's ~48 hours from now
	expiresAt, err := time.Parse(time.RFC3339Nano, anonResult.Videos[0].ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse expires_at: %v", err)
	}
	hoursUntilExpiry := time.Until(expiresAt).Hours()
	if hoursUntilExpiry < 47 || hoursUntilExpiry > 49 {
		t.Errorf("Expected ~48 hours until expiry, got %.1f", hoursUntilExpiry)
	}

	// Test 2: Registered user video should expire in 90 days
	token := ts.createTestUser(t, "expires@example.com", "ValidPassword123!")
	resp = ts.doRequest("GET", "/api/auth/me", nil, token)
	var meResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&meResult)
	userID := meResult.User.ID

	ts.createTestVideo(t, &userID, nil)

	resp = ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var userResult struct {
		Videos []struct {
			ID        string  `json:"id"`
			UserID    *string `json:"user_id"`
			ExpiresAt string  `json:"expires_at"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&userResult)

	if len(userResult.Videos) != 1 {
		t.Fatalf("Expected 1 video, got %d", len(userResult.Videos))
	}

	if userResult.Videos[0].ExpiresAt == "" {
		t.Error("Expected expires_at to be set for registered user video")
	}

	// Parse the expiry time and verify it's ~90 days from now
	expiresAt, err = time.Parse(time.RFC3339Nano, userResult.Videos[0].ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse expires_at: %v", err)
	}
	daysUntilExpiry := time.Until(expiresAt).Hours() / 24
	if daysUntilExpiry < 89 || daysUntilExpiry > 91 {
		t.Errorf("Expected ~90 days until expiry, got %.1f", daysUntilExpiry)
	}
}

func TestListVideosPagination(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with multiple videos
	userID, token := ts.createTestUserWithID(t, "pagination@example.com", "Password123!")

	// Create 10 videos
	for i := 0; i < 10; i++ {
		ts.createTestVideo(t, &userID, nil)
	}

	type paginatedResult struct {
		Videos     []struct{ ID string } `json:"videos"`
		TotalCount int                   `json:"total_count"`
		HasMore    bool                  `json:"has_more"`
	}

	// Test 1: Default pagination (should return all 10 with metadata)
	resp := ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var result1 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result1)

	if result1.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result1.TotalCount)
	}
	if len(result1.Videos) != 10 {
		t.Errorf("Expected 10 videos, got %d", len(result1.Videos))
	}
	if result1.HasMore {
		t.Error("Expected has_more false when all videos returned")
	}

	// Test 2: First page with limit
	resp = ts.doRequest("GET", "/api/videos?limit=3", nil, token)
	var result2 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result2)

	if result2.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result2.TotalCount)
	}
	if len(result2.Videos) != 3 {
		t.Errorf("Expected 3 videos, got %d", len(result2.Videos))
	}
	if !result2.HasMore {
		t.Error("Expected has_more true when more videos exist")
	}

	// Test 3: Second page
	resp = ts.doRequest("GET", "/api/videos?limit=3&offset=3", nil, token)
	var result3 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result3)

	if len(result3.Videos) != 3 {
		t.Errorf("Expected 3 videos on second page, got %d", len(result3.Videos))
	}
	if !result3.HasMore {
		t.Error("Expected has_more true on second page")
	}

	// Test 4: Last page (partial)
	resp = ts.doRequest("GET", "/api/videos?limit=3&offset=9", nil, token)
	var result4 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result4)

	if len(result4.Videos) != 1 {
		t.Errorf("Expected 1 video on last page, got %d", len(result4.Videos))
	}
	if result4.HasMore {
		t.Error("Expected has_more false on last page")
	}

	// Test 5: Max limit enforcement (>100 should be capped to 100)
	resp = ts.doRequest("GET", "/api/videos?limit=200", nil, token)
	var result5 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result5)

	if len(result5.Videos) != 10 { // Only 10 videos exist
		t.Errorf("Expected all 10 videos (capped by data size), got %d", len(result5.Videos))
	}

	// Test 6: Invalid limit/offset should be ignored (defaults used)
	resp = ts.doRequest("GET", "/api/videos?limit=invalid&offset=invalid", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 with invalid params, got %d", resp.Code)
	}
}

func TestListVideosExcessiveOffset(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "offset@example.com", "Password123!")

	// Offset at the maximum should succeed
	resp := ts.doRequest("GET", "/api/videos?offset=100000", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 for offset at max, got %d", resp.Code)
	}

	// Offset exceeding the maximum should return 400
	resp = ts.doRequest("GET", "/api/videos?offset=100001", nil, token)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for excessive offset, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Offset exceeds maximum allowed value" {
		t.Errorf("Expected offset error message, got '%s'", result["error"])
	}
}

func TestDeleteVideoAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "delete@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a transcription for the video
	ts.createTestTranscription(t, video.ID)

	// Delete should succeed
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["message"] != "Video deleted successfully" {
		t.Errorf("Expected message 'Video deleted successfully', got '%s'", result["message"])
	}

	// Verify video is gone
	getResp := ts.doRequest("GET", "/api/videos?user_id="+userID, nil, token)
	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(getResp.Body).Decode(&listResult)

	for _, v := range listResult.Videos {
		if v.ID == video.ID {
			t.Error("Video should have been deleted but still appears in list")
		}
	}
}

func TestDeleteVideoAnonymous(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "test-delete-session-123"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Delete with matching session should succeed
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID+"?session_id="+sessionID, nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteVideoNotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by user1
	user1ID := testGenerateID()
	video := ts.createTestVideo(t, &user1ID, nil)

	// Create another user and try to delete
	_, token := ts.createTestUserWithID(t, "other@example.com", "Password123!")

	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, token)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestDeleteVideoAnonymousWrongSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "original-session"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Try to delete with different session
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID+"?session_id=wrong-session", nil, "")

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestDeleteVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	resp := ts.doRequest("DELETE", "/api/videos/nonexistent", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestDeleteVideoNoAuth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by a user
	userID := testGenerateID()
	video := ts.createTestVideo(t, &userID, nil)

	// Try to delete without auth or session_id
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, "")

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestGetTranscriptionStatus(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	sessionID := "test-session-transcription"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Get transcription status
	resp := ts.doRequest("GET", "/api/transcribe/"+video.ID+"?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Status string `json:"status"`
		Result struct {
			Segments []struct {
				Text string `json:"text"`
			} `json:"segments"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result.Status)
	}

	if len(result.Result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Result.Segments))
	}
}

func TestGetTranscriptionNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/transcribe/nonexistent", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestGetTranscriptionAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-tx-denied"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Without session_id should be forbidden
	resp := ts.doRequest("GET", "/api/transcribe/"+video.ID, nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without session_id, got %d: %s", resp.Code, resp.Body.String())
	}

	// With wrong session_id should be forbidden
	resp = ts.doRequest("GET", "/api/transcribe/"+video.ID+"?session_id=wrong-session", nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 with wrong session_id, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateSegmentsAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-seg-denied"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Without session_id should be forbidden
	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments", map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: 0.0, End: 3.0, Text: "Updated text."},
		},
	}, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without session_id, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAlignTranscriptAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-denied"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Without session_id should be forbidden
	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align", map[string]string{
		"text": "Hello world",
	}, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without session_id, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestTranscriptionErrorMessageSanitization verifies that whisper error messages
// are sanitized before being sent to clients (Task 205)

func TestTranscriptionErrorMessageSanitization(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Ensure verbose mode is off (production behavior)
	errmsg.SetVerbose(false)
	defer errmsg.SetVerbose(false)

	// Create video and failing transcription with sensitive error message
	sessionID := "test-session-sanitization"
	video := ts.createTestVideo(t, nil, &sessionID)
	sensitiveError := "whisper-server request failed after 3 attempts: connection refused at 10.0.2.2:8765 for file /opt/subtitler/uploads/abc123.mp4"

	transcription := &db.Transcription{
		VideoID:   video.ID,
		Status:    "pending",
		Message:   "",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}
	if err := ts.db.FailTranscription(video.ID, sensitiveError); err != nil {
		t.Fatalf("Failed to fail transcription: %v", err)
	}

	// Get transcription status
	resp := ts.doRequest("GET", "/api/transcribe/"+video.ID+"?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", result.Status)
	}

	// Verify the message does NOT contain sensitive information
	sensitiveStrings := []string{
		"10.0.2.2",           // IP address
		"8765",               // Port
		"/opt/subtitler",     // Server path
		"abc123.mp4",         // Filename
		"whisper-server",     // Internal service name
		"connection refused", // Technical error detail
	}
	for _, s := range sensitiveStrings {
		if strings.Contains(result.Message, s) {
			t.Errorf("Response message contains sensitive string %q: %s", s, result.Message)
		}
	}

	// Verify the message IS the sanitized user-friendly message
	if result.Message != errmsg.ErrTranscribeFailed {
		t.Errorf("Expected sanitized message %q, got %q", errmsg.ErrTranscribeFailed, result.Message)
	}
}

// TestTranscriptionErrorVerboseMode verifies that verbose mode returns detailed errors

func TestTranscriptionErrorVerboseMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Enable verbose mode (development behavior)
	errmsg.SetVerbose(true)
	defer errmsg.SetVerbose(false)

	// Create video and failing transcription with sensitive error message
	sessionID := "test-session-verbose"
	video := ts.createTestVideo(t, nil, &sessionID)
	sensitiveError := "whisper-server request failed: connection refused at 10.0.2.2:8765"

	transcription := &db.Transcription{
		VideoID:   video.ID,
		Status:    "pending",
		Message:   "",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}
	if err := ts.db.FailTranscription(video.ID, sensitiveError); err != nil {
		t.Fatalf("Failed to fail transcription: %v", err)
	}

	// Get transcription status
	resp := ts.doRequest("GET", "/api/transcribe/"+video.ID+"?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	// In verbose mode, should return the raw error message
	if result.Message != sensitiveError {
		t.Errorf("Expected raw error message in verbose mode, got %q", result.Message)
	}
}

func TestDownloadSRT(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-srt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download SRT with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}

	// Check SRT content
	srtContent := resp.Body.String()
	if !strings.Contains(srtContent, "Hello world.") {
		t.Error("SRT content should contain 'Hello world.'")
	}
	if !strings.Contains(srtContent, "00:00:00,000 --> 00:00:02,500") {
		t.Error("SRT content should contain timestamps")
	}
}

func TestDownloadSRTAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video owned by a session
	sessionID := "owner-session"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Try to download without session_id - should be denied
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}

	// Try with wrong session_id - should be denied
	resp = ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id=wrong-session", nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for wrong session, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDownloadSRTAuthenticatedUserAccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create authenticated user and their video
	userID, token := ts.createTestUserWithID(t, "srt-user@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)
	ts.createTestTranscription(t, video.ID)

	// Download SRT with auth token - should succeed
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Different user should be denied
	_, token2 := ts.createTestUserWithID(t, "srt-other@example.com", "Password123!")
	resp = ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, token2)
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for different user, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDownloadSRTNotComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with pending transcription
	sessionID := "test-session-srt-nc"
	video := ts.createTestVideo(t, nil, &sessionID)
	transcription := &db.Transcription{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Processing...",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	ts.db.CreateTranscription(transcription)

	// Try to download SRT
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

func TestDownloadVTT(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-vtt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download VTT with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/vtt") {
		t.Errorf("Expected Content-Type text/vtt, got %s", contentType)
	}

	// Check VTT content
	vttContent := resp.Body.String()
	if !strings.HasPrefix(vttContent, "WEBVTT") {
		t.Error("VTT content should start with 'WEBVTT'")
	}
	if !strings.Contains(vttContent, "Hello world.") {
		t.Error("VTT content should contain 'Hello world.'")
	}
	// VTT uses period instead of comma for milliseconds
	if !strings.Contains(vttContent, "00:00:00.000 --> 00:00:02.500") {
		t.Error("VTT content should contain timestamps with period separator")
	}
}

func TestDownloadVTTAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "owner-session-vtt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// No session_id - should be denied
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt", nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDownloadVTTNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to download VTT for non-existent video
	resp := ts.doRequest("GET", "/api/videos/nonexistent/subtitles.vtt", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestDownloadJSON(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-json"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download JSON with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check JSON content
	var jsonResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
		return
	}

	// Verify structure
	if jsonResp["video_id"] != video.ID {
		t.Errorf("Expected video_id %s, got %v", video.ID, jsonResp["video_id"])
	}
	if jsonResp["full_text"] == nil {
		t.Error("Expected full_text field in response")
	}
	segments, ok := jsonResp["segments"].([]interface{})
	if !ok || len(segments) == 0 {
		t.Error("Expected non-empty segments array in response")
	}
}

func TestDownloadJSONAccessDenied(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "owner-session-json"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// No session_id - should be denied
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json", nil, "")
	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDownloadJSONNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to download JSON for non-existent video
	resp := ts.doRequest("GET", "/api/videos/nonexistent/subtitles.json", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestUpdateSegments(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	sessionID := "test-session-segments"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Update segments
	newSegments := []db.Segment{
		{ID: 0, Start: 0.0, End: 3.0, Text: "Updated text."},
		{ID: 1, Start: 3.5, End: 6.0, Text: "Also updated."},
	}

	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments?session_id="+sessionID, map[string]interface{}{
		"segments": newSegments,
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify update
	transcription, err := ts.db.GetTranscription(video.ID)
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}
	segments, err := transcription.GetSegments()
	if err != nil {
		t.Fatalf("Failed to get segments: %v", err)
	}

	if segments[0].Text != "Updated text." {
		t.Errorf("Expected 'Updated text.', got '%s'", segments[0].Text)
	}
}

func TestUpdateSegmentsInvalidTiming(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-invalid-timing"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Try with invalid timing (start > end)
	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments?session_id="+sessionID, map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: 5.0, End: 2.0, Text: "Invalid"},
		},
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid timing, got %d", resp.Code)
	}
}

func TestUpdateSegmentsNegativeTiming(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-neg-timing"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Try with negative timing
	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments?session_id="+sessionID, map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: -1.0, End: 2.0, Text: "Negative start"},
		},
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for negative timing, got %d", resp.Code)
	}
}

func TestUpdateSegmentsTextTooLong(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-text-long"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Create a text that exceeds the 10KB limit
	longText := strings.Repeat("a", 11*1024) // 11KB

	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments?session_id="+sessionID, map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: 0.0, End: 2.0, Text: longText},
		},
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for text too long, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(result["error"], "too long") {
		t.Errorf("Expected error message about text too long, got: %s", result["error"])
	}
}

// ========== Upload Tests (with mock file) ==========

func TestRateLimitingBurn(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/videos/{id}/burn", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/videos/test123/burn", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/videos/test123/burn", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestBurnStatusWithETA tests that the burn status endpoint returns progress and ETA

func TestBurnStatusWithETA(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Create transcription with duration (needed for ETA calculation)
	// First create the transcription record
	if err := ts.db.CreateTranscription(&db.Transcription{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}
	// Then complete it with duration (this sets the duration field)
	if err := ts.db.CompleteTranscription(video.ID, "en", 120.0, "Test transcript", []db.Segment{}); err != nil {
		t.Fatalf("Failed to complete transcription: %v", err)
	}

	// Create burn job in progress (started 10 seconds ago with 50% progress)
	burnJob := &db.BurnJob{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Burning subtitles...",
		Progress:  50,
		CreatedAt: time.Now().Add(-10 * time.Second), // Started 10 seconds ago
	}
	if err := ts.db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Get burn status
	req := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Status                    string  `json:"status"`
		Message                   string  `json:"message"`
		Progress                  int     `json:"progress"`
		Duration                  float64 `json:"duration"`
		EstimatedRemainingSeconds int     `json:"estimated_remaining_seconds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response fields
	if response.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", response.Status)
	}
	if response.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", response.Progress)
	}
	if response.Duration != 120.0 {
		t.Errorf("Expected duration 120.0, got %f", response.Duration)
	}
	// ETA should be approximately 10 seconds (50% done in 10 seconds = ~10 seconds remaining)
	// Allow for some variance due to timing
	if response.EstimatedRemainingSeconds < 5 || response.EstimatedRemainingSeconds > 15 {
		t.Errorf("Expected estimated_remaining_seconds ~10, got %d", response.EstimatedRemainingSeconds)
	}
}

// TestBurnStatusComplete tests that completed burn jobs don't include ETA

func TestBurnStatusComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Complete the burn job
	burnJob := &db.BurnJob{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "complete",
		Message:   "Done",
		Progress:  100,
		CreatedAt: time.Now().Add(-30 * time.Second),
	}
	if err := ts.db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Get burn status
	req := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify no ETA for complete job
	if _, ok := response["estimated_remaining_seconds"]; ok {
		t.Error("Expected no estimated_remaining_seconds for complete job")
	}
	if response["status"] != "complete" {
		t.Errorf("Expected status 'complete', got '%v'", response["status"])
	}
}

// TestBurnStartSuccess tests starting a burn job successfully

func TestBurnStartSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video with completed transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["status"] != "processing" {
		t.Errorf("Expected status 'processing', got '%v'", response["status"])
	}
	if response["message"] != "Starting subtitle burn..." {
		t.Errorf("Expected message 'Starting subtitle burn...', got '%v'", response["message"])
	}
}

// TestBurnStartNoTranscription tests burn request when no transcription exists

func TestBurnStartNoTranscription(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBurnStartIncompleteTranscription tests burn request when transcription is not complete

func TestBurnStartIncompleteTranscription(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)

	// Create a pending transcription (not completed)
	transcription := &db.Transcription{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Extracting audio...",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["error"] != "Cannot burn subtitles - transcription not complete" {
		t.Errorf("Expected transcription not complete error, got '%v'", response["error"])
	}
}

// TestBurnStartInvalidMode tests burn request with invalid mode parameter

func TestBurnStartInvalidMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn?mode=invalid", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBurnStartEmbedMode tests burn request with embed mode

func TestBurnStartEmbedMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn?mode=embed", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["status"] != "processing" {
		t.Errorf("Expected status 'processing', got '%v'", response["status"])
	}
}

// TestBurnStartAlreadyProcessing tests burn request when job is already processing

func TestBurnStartAlreadyProcessing(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Create an existing processing burn job
	burnJob := &db.BurnJob{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Burning subtitles...",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	if err := ts.db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["status"] != "processing" {
		t.Errorf("Expected status 'processing', got '%v'", response["status"])
	}
	if response["message"] != "Burning subtitles..." {
		t.Errorf("Expected message 'Burning subtitles...', got '%v'", response["message"])
	}
	// Should return existing progress, not 0
	if response["progress"] != float64(50) {
		t.Errorf("Expected progress 50, got '%v'", response["progress"])
	}
}

// TestBurnStartNonexistentVideo tests burn request for a nonexistent video

func TestBurnStartNonexistentVideo(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req := httptest.NewRequest("POST", "/api/videos/"+testGenerateID()+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetSessions tests listing user's sessions

func TestReprocessVideoSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "reprocess@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a failed transcription
	ts.createTestFailedTranscription(t, video.ID)

	// Reprocess should succeed
	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", result["status"])
	}
	if result["message"] != "Reprocessing started" {
		t.Errorf("Expected message 'Reprocessing started', got '%s'", result["message"])
	}
}

func TestReprocessVideoAnonymous(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "test-session-123"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Create a failed transcription
	ts.createTestFailedTranscription(t, video.ID)

	// Reprocess with matching session should succeed
	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess?session_id="+sessionID, nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestReprocessVideoNotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by user1
	user1ID := testGenerateID()
	video := ts.createTestVideo(t, &user1ID, nil)
	ts.createTestFailedTranscription(t, video.ID)

	// Create another user and try to reprocess
	_, token := ts.createTestUserWithID(t, "other@example.com", "Password123!")

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestReprocessVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	resp := ts.doRequest("POST", "/api/videos/nonexistent/reprocess", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestReprocessVideoNoTranscription(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)
	// No transcription created

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "No transcription found for this video" {
		t.Errorf("Unexpected error: %s", result["error"])
	}
}

func TestReprocessVideoNotError(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)
	ts.createTestTranscription(t, video.ID) // Complete transcription, not error

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Can only reprocess failed transcriptions" {
		t.Errorf("Unexpected error: %s", result["error"])
	}
	if result["status"] != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result["status"])
	}
}

// ========== Request ID Middleware Tests ==========

func TestSRTDownloadCachingHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-cache-srt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download SRT with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check ETag header exists
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in SRT response")
	}
	if !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
		t.Errorf("ETag should be quoted, got: %s", etag)
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in SRT response")
	}
	if !strings.Contains(cacheControl, "private") {
		t.Errorf("Cache-Control should contain 'private', got: %s", cacheControl)
	}
}

func TestSRTDownloadConditionalRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-cond-srt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// First request to get ETag
	resp1 := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil, "")
	if resp1.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp1.Code)
	}
	etag := resp1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("Expected ETag header in first response")
	}

	// Second request with If-None-Match should return 304
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil)
	req.Header.Set("If-None-Match", etag)
	resp2 := httptest.NewRecorder()
	ts.mux.ServeHTTP(resp2, req)

	if resp2.Code != http.StatusNotModified {
		t.Errorf("Expected status 304 Not Modified, got %d", resp2.Code)
	}

	// Body should be empty for 304
	if resp2.Body.Len() > 0 {
		t.Errorf("Expected empty body for 304, got %d bytes", resp2.Body.Len())
	}
}

func TestVTTDownloadCachingHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-cache-vtt"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download VTT with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Check ETag header
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in VTT response")
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in VTT response")
	}
}

func TestJSONDownloadCachingHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-cache-json"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Download JSON with matching session_id
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Check ETag header
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in JSON response")
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in JSON response")
	}
}

func TestDifferentFormatsHaveDifferentETags(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with session ID and transcription
	sessionID := "test-session-etag-diff"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Get ETags for all formats
	respSRT := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt?session_id="+sessionID, nil, "")
	respVTT := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt?session_id="+sessionID, nil, "")
	respJSON := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json?session_id="+sessionID, nil, "")

	etagSRT := respSRT.Header().Get("ETag")
	etagVTT := respVTT.Header().Get("ETag")
	etagJSON := respJSON.Header().Get("ETag")

	// All ETags should be different (they include format in the hash)
	if etagSRT == etagVTT {
		t.Error("SRT and VTT should have different ETags")
	}
	if etagSRT == etagJSON {
		t.Error("SRT and JSON should have different ETags")
	}
	if etagVTT == etagJSON {
		t.Error("VTT and JSON should have different ETags")
	}
}

// createTestVideoWithFile creates a video record AND an actual file on disk for testing
func (ts *testServer) createTestVideoWithFile(t *testing.T, userID *string, sessionID *string, content []byte) *db.Video {
	t.Helper()

	videoID := testGenerateID()
	filename := videoID + ".mp4"
	filePath := filepath.Join(ts.uploadDir, filename)

	// Write actual content to file
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test video file: %v", err)
	}

	video := &db.Video{
		ID:          videoID,
		Filename:    filename,
		Size:        int64(len(content)),
		ContentType: "video/mp4",
		FilePath:    filePath,
		UserID:      userID,
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	return video
}

func TestVideoRangeRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test Range request for bytes 0-49 (first 50 bytes)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=0-49")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 0-49/100") {
		t.Errorf("Expected Content-Range 'bytes 0-49/100', got '%s'", contentRange)
	}

	// Verify only requested bytes were returned
	if rr.Body.Len() != 50 {
		t.Errorf("Expected 50 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches
	for i := 0; i < 50; i++ {
		if rr.Body.Bytes()[i] != byte(i) {
			t.Errorf("Byte at position %d: expected %d, got %d", i, i, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestMiddleRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test Range request for bytes 25-74 (middle 50 bytes)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=25-74")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 25-74/100") {
		t.Errorf("Expected Content-Range 'bytes 25-74/100', got '%s'", contentRange)
	}

	// Verify only requested bytes were returned
	if rr.Body.Len() != 50 {
		t.Errorf("Expected 50 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches
	for i := 0; i < 50; i++ {
		if rr.Body.Bytes()[i] != byte(25+i) {
			t.Errorf("Byte at position %d: expected %d, got %d", i, 25+i, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestSuffix(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test suffix Range request for last 20 bytes (bytes=-20)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=-20")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify only 20 bytes were returned
	if rr.Body.Len() != 20 {
		t.Errorf("Expected 20 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches last 20 bytes
	for i := 0; i < 20; i++ {
		expected := byte(80 + i)
		if rr.Body.Bytes()[i] != expected {
			t.Errorf("Byte at position %d: expected %d, got %d", i, expected, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestOpenEnd(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test open-end Range request (bytes=80-)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=80-")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 80-99/100") {
		t.Errorf("Expected Content-Range 'bytes 80-99/100', got '%s'", contentRange)
	}

	// Verify only 20 bytes were returned
	if rr.Body.Len() != 20 {
		t.Errorf("Expected 20 bytes, got %d", rr.Body.Len())
	}
}

func TestVideoRangeRequestInvalidRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test invalid Range request (beyond file size)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=150-200")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	// Should return 416 Range Not Satisfiable
	if rr.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected status 416 Range Not Satisfiable, got %d", rr.Code)
	}
}

func TestVideoNoRangeRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test request without Range header
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	// Verify all content was returned
	if rr.Body.Len() != 100 {
		t.Errorf("Expected 100 bytes, got %d", rr.Body.Len())
	}

	// Should have Accept-Ranges header indicating range support
	acceptRanges := rr.Header().Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("Expected Accept-Ranges 'bytes', got '%s'", acceptRanges)
	}
}

// createTestVideoWithThumbnail creates a video record with an actual thumbnail file
func (ts *testServer) createTestVideoWithThumbnail(t *testing.T, userID *string, sessionID *string, videoContent []byte, thumbContent []byte) *db.Video {
	t.Helper()

	videoID := testGenerateID()
	filename := videoID + ".mp4"
	filePath := filepath.Join(ts.uploadDir, filename)
	thumbPath := filepath.Join(ts.uploadDir, videoID+"_thumb.jpg")

	// Write actual content to files
	if err := os.WriteFile(filePath, videoContent, 0644); err != nil {
		t.Fatalf("Failed to create test video file: %v", err)
	}
	if err := os.WriteFile(thumbPath, thumbContent, 0644); err != nil {
		t.Fatalf("Failed to create test thumbnail file: %v", err)
	}

	video := &db.Video{
		ID:            videoID,
		Filename:      filename,
		Size:          int64(len(videoContent)),
		ContentType:   "video/mp4",
		FilePath:      filePath,
		ThumbnailPath: &thumbPath,
		UserID:        userID,
		SessionID:     sessionID,
		CreatedAt:     time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	return video
}

func TestThumbnailEndpointSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test thumbnail content (fake JPEG)
	thumbContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46} // JPEG header bytes
	videoContent := []byte("fake video content")

	video := ts.createTestVideoWithThumbnail(t, nil, nil, videoContent, thumbContent)

	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "image/jpeg" {
		t.Errorf("Expected Content-Type 'image/jpeg', got '%s'", contentType)
	}

	// Verify content matches
	if !bytes.Equal(rr.Body.Bytes(), thumbContent) {
		t.Errorf("Thumbnail content mismatch")
	}

	// Should have caching headers
	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header to be set")
	}

	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header to be set")
	}
}

func TestThumbnailEndpointNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req, _ := http.NewRequest("GET", "/api/videos/nonexistent123/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found, got %d", rr.Code)
	}
}

func TestThumbnailEndpointNoThumbnail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video without thumbnail
	videoContent := []byte("fake video content")
	video := ts.createTestVideoWithFile(t, nil, nil, videoContent)

	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found for video without thumbnail, got %d", rr.Code)
	}
}

func TestThumbnailEndpointConditionalRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	thumbContent := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	videoContent := []byte("fake video")

	video := ts.createTestVideoWithThumbnail(t, nil, nil, videoContent, thumbContent)

	// First request to get ETag
	req1, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr1 := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK on first request, got %d", rr1.Code)
	}

	etag := rr1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("Expected ETag header on first request")
	}

	// Second request with If-None-Match
	req2, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("Expected status 304 Not Modified, got %d", rr2.Code)
	}
}

// ========== Chunked Upload Tests ==========

func TestDownloadRateLimiting(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "download-ratelimit@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a small downloadLimiter for testing (5 requests per minute)
	testDownloadLimiter := ratelimit.New(5, time.Minute)

	// Create a dedicated test server with the stricter limiter
	testMux := http.NewServeMux()
	testMux.HandleFunc("GET /api/videos/{id}/video", testDownloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, video.FilePath)
	}))

	testServer := httptest.NewServer(testMux)
	defer testServer.Close()

	// Make requests until rate limited
	var lastResp *http.Response
	for i := 0; i < 7; i++ {
		req, _ := http.NewRequest("GET", testServer.URL+"/api/videos/"+video.ID+"/video", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		lastResp, _ = http.DefaultClient.Do(req)
		if lastResp.StatusCode == http.StatusTooManyRequests {
			break
		}
		lastResp.Body.Close()
	}

	// Should eventually get 429
	if lastResp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 after exceeding rate limit, got %d", lastResp.StatusCode)
	}

	// Check Retry-After header
	retryAfter := lastResp.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header in rate limited response")
	}
	lastResp.Body.Close()
}

func TestVideoDownloadPathValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and get token
	userID, token := ts.createTestUserWithID(t, "path-validation@example.com", "Password123!")

	// Test 1: Valid path within uploads directory - create actual file first
	t.Run("valid path download succeeds", func(t *testing.T) {
		// Create an actual test video file
		testFilePath := filepath.Join(ts.uploadDir, "valid_test_video.mp4")
		testContent := []byte("fake video content for testing")
		if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Create a video record pointing to the valid file
		video := &db.Video{
			ID:          testGenerateID(),
			Filename:    "valid_test_video.mp4",
			FilePath:    testFilePath,
			Size:        int64(len(testContent)),
			ContentType: "video/mp4",
			UserID:      &userID,
			CreatedAt:   time.Now(),
		}
		if err := ts.db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create test video: %v", err)
		}

		req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 for valid video path, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// Test 2: Manually craft a video with a path traversal attempt
	// This simulates a database compromise where an attacker modified the file path
	t.Run("path traversal blocked", func(t *testing.T) {
		// Create a video with a malicious path pointing outside uploads directory
		maliciousVideo := &db.Video{
			ID:          testGenerateID(),
			Filename:    "../../etc/passwd",
			FilePath:    "/etc/passwd", // Absolute path outside uploads
			Size:        100,
			ContentType: "video/mp4",
			UserID:      &userID,
			CreatedAt:   time.Now(),
		}
		if err := ts.db.CreateVideo(maliciousVideo); err != nil {
			t.Fatalf("Failed to create malicious test video: %v", err)
		}

		req, _ := http.NewRequest("GET", "/api/videos/"+maliciousVideo.ID+"/video", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		// Should get 403 Forbidden due to path validation
		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for path traversal attempt, got %d: %s", rr.Code, rr.Body.String())
		}

		// Verify error message
		var response map[string]string
		json.Unmarshal(rr.Body.Bytes(), &response)
		if response["error"] != "Access denied" {
			t.Errorf("Expected 'Access denied' error, got: %s", response["error"])
		}
	})

	// Test 3: Path traversal in thumbnail blocked
	t.Run("thumbnail path traversal blocked", func(t *testing.T) {
		// Create a video with a malicious thumbnail path
		maliciousPath := "/etc/shadow"
		maliciousVideo := &db.Video{
			ID:            testGenerateID(),
			Filename:      "test.mp4",
			FilePath:      filepath.Join(ts.uploadDir, "test_video.mp4"), // Valid video path
			ThumbnailPath: &maliciousPath,                                // Malicious thumbnail path
			Size:          100,
			ContentType:   "video/mp4",
			UserID:        &userID,
			CreatedAt:     time.Now(),
		}
		if err := ts.db.CreateVideo(maliciousVideo); err != nil {
			t.Fatalf("Failed to create malicious test video: %v", err)
		}

		req, _ := http.NewRequest("GET", "/api/videos/"+maliciousVideo.ID+"/thumbnail", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		// Should get 403 Forbidden due to path validation
		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for thumbnail path traversal attempt, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ========== Admin Role Tests ==========

func TestAlignTranscriptStandard(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	sessionID := "test-session-align-std"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Align with standard mode (default)
	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]string{
		"text": "Hello world\nThis is a test",
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", result["status"])
	}
	if result["mode"] != "standard" {
		t.Errorf("Expected mode 'standard', got %v", result["mode"])
	}
	if result["segments"] == nil {
		t.Error("Expected segments count in response")
	}
	if result["stats"] == nil {
		t.Error("Expected stats in response")
	}
}

// TestAlignTranscriptLyricsMode tests lyrics mode alignment

func TestAlignTranscriptLyricsMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	sessionID := "test-session-align-lyrics"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	// Align with lyrics mode
	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]interface{}{
		"text": "Hello world\nThis is a test",
		"mode": "lyrics",
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["mode"] != "lyrics" {
		t.Errorf("Expected mode 'lyrics', got %v", result["mode"])
	}
}

// TestAlignTranscriptNoTranscription tests alignment with no transcription

func TestAlignTranscriptNoTranscription(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video without transcription
	sessionID := "test-session-align-notx"
	video := ts.createTestVideo(t, nil, &sessionID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]string{
		"text": "Hello world",
	}, "")

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "No transcription found for this upload" {
		t.Errorf("Expected 'No transcription found' error, got: %s", result["error"])
	}
}

// TestAlignTranscriptIncomplete tests alignment with incomplete transcription

func TestAlignTranscriptIncomplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with pending transcription (not complete)
	sessionID := "test-session-align-incomplete"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Create a pending (incomplete) transcription
	transcription := &db.Transcription{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing", // Not complete
		Message:   "Test",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]string{
		"text": "Hello world",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Cannot align - transcription not complete" {
		t.Errorf("Expected 'transcription not complete' error, got: %v", result["error"])
	}
	if result["status"] != "processing" {
		t.Errorf("Expected status 'processing' in error response, got: %v", result["status"])
	}
}

// TestAlignTranscriptInvalidID tests alignment with invalid video ID

func TestAlignTranscriptInvalidID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try with invalid ID format
	resp := ts.doRequest("POST", "/api/transcribe/invalid-id/align", map[string]string{
		"text": "Hello world",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestAlignTranscriptEmptyText tests alignment with empty text

func TestAlignTranscriptEmptyText(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-empty"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]string{
		"text": "",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestAlignTranscriptInvalidMode tests alignment with invalid mode

func TestAlignTranscriptInvalidMode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-badmode"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]interface{}{
		"text": "Hello world",
		"mode": "invalid_mode",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

// TestAlignTranscriptWithScriptConversion tests alignment with script conversion

func TestAlignTranscriptWithScriptConversion(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-script"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]interface{}{
		"text":              "namaste dost",
		"convert_to_script": "Devanagari",
		"language":          "hi",
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["script_converted"] != true {
		t.Errorf("Expected script_converted=true, got %v", result["script_converted"])
	}
	if result["target_script"] != "Devanagari" {
		t.Errorf("Expected target_script='Devanagari', got %v", result["target_script"])
	}
}

// TestAlignTranscriptUnsupportedScript tests alignment with unsupported script

func TestAlignTranscriptUnsupportedScript(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-unsup"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]interface{}{
		"text":              "Hello world",
		"convert_to_script": "UnsupportedScript",
		"language":          "en",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Unsupported target script" {
		t.Errorf("Expected 'Unsupported target script' error, got: %v", result["error"])
	}
	if result["supported_scripts"] == nil {
		t.Error("Expected supported_scripts in error response")
	}
}

// TestAlignTranscriptMissingLanguage tests alignment with script conversion but missing language

func TestAlignTranscriptMissingLanguage(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-nolang"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	resp := ts.doRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, map[string]interface{}{
		"text":              "Hello world",
		"convert_to_script": "Devanagari",
		// No language specified
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Language required for script conversion" {
		t.Errorf("Expected 'Language required' error, got: %v", result["error"])
	}
}

// TestAlignTranscriptInvalidBody tests alignment with invalid request body

func TestAlignTranscriptInvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-align-badbody"
	video := ts.createTestVideo(t, nil, &sessionID)
	ts.createTestTranscription(t, video.ID)

	req := httptest.NewRequest("POST", "/api/transcribe/"+video.ID+"/align?session_id="+sessionID, strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	ts.mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// ============================================
// Feedback endpoint tests
// ============================================

func TestLanguageHintsEndpointVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Request language hints for non-existent video
	resp := ts.doRequest("GET", "/api/videos/"+testGenerateID()+"/language-hints", nil, "")

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != "Video not found" {
		t.Errorf("Expected 'Video not found' error, got: %v", result["error"])
	}
}

func TestLanguageHintsEndpointInvalidID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Request with invalid ID format
	resp := ts.doRequest("GET", "/api/videos/invalid-id/language-hints", nil, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestLanguageHintsEndpointFilenameDetection(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test video with language in filename
	sessionID := "test-session-123"
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "movie.en.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(ts.uploadDir, "test.mp4"),
		SessionID:   &sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	// Request language hints
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/language-hints", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Hints []struct {
			Source       string `json:"source"`
			Language     string `json:"language"`
			LanguageName string `json:"language_name"`
			Confidence   string `json:"confidence"`
			RawValue     string `json:"raw_value"`
		} `json:"hints"`
		SuggestedLanguage   string `json:"suggested_language"`
		SuggestedConfidence string `json:"suggested_confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(result.Hints) == 0 {
		t.Error("Expected at least one language hint")
	}

	// Should detect English from filename pattern "movie.en.mp4"
	if result.SuggestedLanguage != "en" {
		t.Errorf("Expected suggested language 'en', got '%s'", result.SuggestedLanguage)
	}

	if result.SuggestedConfidence != "high" {
		t.Errorf("Expected high confidence for ISO code before extension, got '%s'", result.SuggestedConfidence)
	}
}

func TestLanguageHintsEndpointNoHints(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test video with no language indicators
	sessionID := "test-session-456"
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "random_video.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(ts.uploadDir, "test.mp4"),
		SessionID:   &sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	// Request language hints
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/language-hints", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Hints               []interface{} `json:"hints"`
		SuggestedLanguage   string        `json:"suggested_language"`
		SuggestedConfidence string        `json:"suggested_confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return empty hints when no language detected
	if len(result.Hints) != 0 {
		t.Errorf("Expected no hints, got %d", len(result.Hints))
	}

	if result.SuggestedLanguage != "" {
		t.Errorf("Expected empty suggested language, got '%s'", result.SuggestedLanguage)
	}
}

func TestLanguageHintsEndpointSpanishFilename(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test video with Spanish language in filename
	sessionID := "test-session-789"
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "pelicula_spanish.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(ts.uploadDir, "test.mp4"),
		SessionID:   &sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	// Request language hints
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/language-hints", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Hints []struct {
			Language     string `json:"language"`
			LanguageName string `json:"language_name"`
		} `json:"hints"`
		SuggestedLanguage string `json:"suggested_language"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should detect Spanish from filename
	if result.SuggestedLanguage != "es" {
		t.Errorf("Expected suggested language 'es', got '%s'", result.SuggestedLanguage)
	}

	// Verify language name is set
	if len(result.Hints) > 0 && result.Hints[0].LanguageName != "Spanish" {
		t.Errorf("Expected language name 'Spanish', got '%s'", result.Hints[0].LanguageName)
	}
}

// ========== Admin Feedback Tests ==========

func TestExtractEmbeddedSubtitleValidTrack(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "embed@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Set embedded subtitles with a text-based track
	tracks := []audio.SubtitleTrack{
		{Index: 2, Language: "eng", Title: "English", Codec: "subrip", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	// Extract track 2 as SRT (default format)
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/2", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	ct := resp.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/plain; charset=utf-8', got %q", ct)
	}

	cd := resp.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "test_eng_embedded.srt") {
		t.Errorf("Expected Content-Disposition containing 'test_eng_embedded.srt', got %q", cd)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "Hello world.") {
		t.Errorf("Expected body to contain 'Hello world.', got %q", body)
	}
}

func TestExtractEmbeddedSubtitleVTTFormat(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "embedvtt@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	tracks := []audio.SubtitleTrack{
		{Index: 3, Language: "fra", Codec: "ass", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	// Extract as VTT format
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/3?format=vtt", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	ct := resp.Header().Get("Content-Type")
	if ct != "text/vtt; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/vtt; charset=utf-8', got %q", ct)
	}

	cd := resp.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "test_fra_embedded.vtt") {
		t.Errorf("Expected Content-Disposition containing 'test_fra_embedded.vtt', got %q", cd)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "WEBVTT") {
		t.Errorf("Expected body to contain 'WEBVTT', got %q", body)
	}
}

func TestExtractEmbeddedSubtitleInvalidTrackIndex(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "embedinvalid@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	tracks := []audio.SubtitleTrack{
		{Index: 2, Language: "eng", Codec: "subrip", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	tests := []struct {
		name       string
		trackParam string
		wantCode   int
		wantError  string
	}{
		{"negative index", "-1", http.StatusBadRequest, "Invalid track index"},
		{"non-numeric", "abc", http.StatusBadRequest, "Invalid track index"},
		{"nonexistent index", "99", http.StatusNotFound, "Subtitle track not found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/"+tc.trackParam, nil, token)

			if resp.Code != tc.wantCode {
				t.Errorf("Expected status %d, got %d: %s", tc.wantCode, resp.Code, resp.Body.String())
			}

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			if result["error"] != tc.wantError {
				t.Errorf("Expected error %q, got %q", tc.wantError, result["error"])
			}
		})
	}
}

func TestExtractEmbeddedSubtitleImageBasedRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "embedimage@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Set up an image-based subtitle track (PGS Blu-ray)
	tracks := []audio.SubtitleTrack{
		{Index: 4, Language: "eng", Codec: "hdmv_pgs_subtitle", TextBased: false},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/4", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(result["error"], "image-based") {
		t.Errorf("Expected error about image-based subtitles, got %q", result["error"])
	}
}

func TestExtractEmbeddedSubtitleAccessControl(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video owned by user1
	userID1, _ := ts.createTestUserWithID(t, "owner@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID1, nil)

	tracks := []audio.SubtitleTrack{
		{Index: 2, Language: "eng", Codec: "subrip", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	// Create a different user
	_, token2 := ts.createTestUserWithID(t, "other@example.com", "Password123!")

	// User2 should be forbidden from accessing user1's video
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/2", nil, token2)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(result["error"], "permission") {
		t.Errorf("Expected error about permission, got %q", result["error"])
	}
}

func TestExtractEmbeddedSubtitleAnonymousAccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an anonymous video with a session ID
	sessionID := testGenerateID()
	video := ts.createTestVideo(t, nil, &sessionID)

	tracks := []audio.SubtitleTrack{
		{Index: 2, Language: "eng", Codec: "subrip", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	// Access with matching session_id should succeed
	req := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/2?session_id="+sessionID, nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 with matching session_id, got %d: %s", rr.Code, rr.Body.String())
	}

	// Access with wrong session_id should be forbidden
	req2 := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/2?session_id=wrongsession", nil)
	rr2 := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 with wrong session_id, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestExtractEmbeddedSubtitleVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "notfound@example.com", "Password123!")

	// Use a valid 32-char hex ID format that doesn't exist in the database
	resp := ts.doRequest("GET", "/api/videos/aabbccdd11223344aabbccdd11223344/embedded-subtitles/0", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Video not found" {
		t.Errorf("Expected error 'Video not found', got %q", result["error"])
	}
}

func TestExtractEmbeddedSubtitleNoSubtitles(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "nosubs@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Don't set any embedded subtitles - video.EmbeddedSubtitlesJSON is nil

	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/0", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "This video has no embedded subtitles" {
		t.Errorf("Expected error 'This video has no embedded subtitles', got %q", result["error"])
	}
}

func TestExtractEmbeddedSubtitleInvalidFormat(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "badfmt@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	tracks := []audio.SubtitleTrack{
		{Index: 2, Language: "eng", Codec: "subrip", TextBased: true},
	}
	ts.setVideoEmbeddedSubtitles(t, video.ID, tracks)

	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/embedded-subtitles/2?format=ass", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Format must be 'srt' or 'vtt'" {
		t.Errorf("Expected error about format, got %q", result["error"])
	}
}
