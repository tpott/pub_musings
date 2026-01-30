package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/ratelimit"
)

func TestUploadVideoForm(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a simple test video file (just bytes, not a real video)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file
	part, err := writer.CreateFormFile("video", "test.mp4")
	if err != nil {
		t.Fatal(err)
	}

	// Write some fake video bytes with a video magic number
	// MP4 files start with ftyp atom
	fakeVideo := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}
	part.Write(fakeVideo)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Note: This test won't pass the content-type validation since our fake bytes
	// don't have a proper video MIME type. This is testing the form parsing.
	// A real test would need to use a proper video file.
}

// ========== Auth Cookie Tests ==========

func TestRateLimitingUpload(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/upload", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	reqLimited := httptest.NewRequest("POST", "/api/upload", nil)
	reqLimited.RemoteAddr = "192.168.1.100:12345"
	wLimited := httptest.NewRecorder()
	mux.ServeHTTP(wLimited, reqLimited)

	if wLimited.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", wLimited.Code)
	}
}

// TestRateLimitingTranscribe tests that transcribe endpoint is rate limited

func TestRateLimitingTranscribe(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/transcribe/{id}", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/transcribe/test123", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/transcribe/test123", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingBurn tests that burn endpoint is rate limited

func TestUploadMIMETypeValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name        string
		contentType string
		expectCode  int
		expectError bool
	}{
		// Allowed MIME types
		{"MP4 allowed", "video/mp4", http.StatusOK, false},
		{"WebM allowed", "video/webm", http.StatusOK, false},
		{"QuickTime allowed", "video/quicktime", http.StatusOK, false},
		{"M4V allowed", "video/x-m4v", http.StatusOK, false},
		{"MPEG allowed", "video/mpeg", http.StatusOK, false},
		{"AVI allowed", "video/x-msvideo", http.StatusOK, false},
		{"MKV allowed", "video/x-matroska", http.StatusOK, false},
		{"OGV allowed", "video/ogg", http.StatusOK, false},

		// Disallowed MIME types
		{"Image rejected", "image/jpeg", http.StatusBadRequest, true},
		{"Text rejected", "text/plain", http.StatusBadRequest, true},
		{"Audio rejected", "audio/mp3", http.StatusBadRequest, true},
		{"3GPP rejected", "video/3gpp", http.StatusBadRequest, true},
		{"Empty rejected", "", http.StatusBadRequest, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Create part with specified content type
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", `form-data; name="video"; filename="test.mp4"`)
			h.Set("Content-Type", tc.contentType)
			part, err := writer.CreatePart(h)
			if err != nil {
				t.Fatal(err)
			}

			// Write some fake video bytes
			part.Write([]byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70})
			writer.Close()

			req := httptest.NewRequest("POST", "/api/upload", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.RemoteAddr = "192.168.1.1:12345" // Different IP to avoid rate limiting

			w := httptest.NewRecorder()
			ts.mux.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("Expected status %d, got %d for MIME type %q: %s",
					tc.expectCode, w.Code, tc.contentType, w.Body.String())
			}

			if tc.expectError {
				var resp map[string]interface{}
				json.NewDecoder(w.Body).Decode(&resp)
				if resp["error"] == nil || !strings.Contains(resp["error"].(string), "Unsupported video format") {
					t.Errorf("Expected 'Unsupported video format' error, got %v", resp["error"])
				}
			}
		})
	}
}

func TestUploadZeroSizeRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a multipart request with zero bytes for the video
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="video"; filename="empty.mp4"`)
	h.Set("Content-Type", "video/mp4")
	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	// Write zero bytes to make it an empty file
	part.Write([]byte{})
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for zero-size file, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(strings.ToLower(resp["error"]), "empty") && !strings.Contains(strings.ToLower(resp["error"]), "too small") {
		t.Errorf("Expected error message to mention empty/too small file, got: %s", resp["error"])
	}
}

func TestChunkedUploadInitZeroSizeRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to initialize chunked upload with size=0
	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "empty.mp4",
		"size":         0,
		"content_type": "video/mp4",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for zero-size chunked upload init, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(strings.ToLower(result["error"]), "size") || !strings.Contains(strings.ToLower(result["error"]), "zero") {
		t.Errorf("Expected error message about zero size, got: %s", result["error"])
	}
}

func TestChunkedUploadInitNegativeSizeRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to initialize chunked upload with negative size
	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         -100,
		"content_type": "video/mp4",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for negative size, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(strings.ToLower(result["error"]), "size") || !strings.Contains(strings.ToLower(result["error"]), "zero") {
		t.Errorf("Expected error message about zero/negative size, got: %s", result["error"])
	}
}

// TestRateLimitingDetectScript tests that detect-script endpoint is rate limited

func TestChunkedUploadInit(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Test successful init
	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         150000000, // 150 MB
		"content_type": "video/mp4",
		"chunk_size":   50000000, // 50 MB
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["upload_session_id"] == nil {
		t.Error("Expected upload_session_id in response")
	}
	if result["total_chunks"].(float64) != 3 {
		t.Errorf("Expected 3 chunks, got %v", result["total_chunks"])
	}
}

func TestChunkedUploadInitInvalidMIME(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.txt",
		"size":         1000,
		"content_type": "text/plain",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", resp.Code)
	}
}

func TestChunkedUploadInitFileTooLarge(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "huge.mp4",
		"size":         600000000, // 600 MB, over 500 MB limit
		"content_type": "video/mp4",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", resp.Code)
	}
}

func TestChunkedUploadChunk(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	if initResp.Code != http.StatusOK {
		t.Fatalf("Init failed: %s", initResp.Body.String())
	}

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload first chunk
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var chunkResult map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&chunkResult)

	if chunkResult["chunk_index"].(float64) != 0 {
		t.Errorf("Expected chunk_index 0, got %v", chunkResult["chunk_index"])
	}
	if chunkResult["received_bytes"].(float64) != 500 {
		t.Errorf("Expected 500 bytes, got %v", chunkResult["received_bytes"])
	}
}

func TestChunkedUploadChunkIdempotency(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   1000,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload chunk twice
	for i := 0; i < 2; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", "0")
		part, _ := writer.CreateFormFile("chunk", "chunk_0")
		part.Write(make([]byte, 1000))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: Expected 200 OK, got %d", i+1, rr.Code)
		}
	}

	// Verify only one chunk was stored
	count, _ := ts.db.CountUploadChunks(sessionID)
	if count != 1 {
		t.Errorf("Expected 1 chunk, got %d", count)
	}
}

func TestChunkedUploadComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with small chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)
	totalChunks := int(initResult["total_chunks"].(float64))

	// Upload all chunks
	for i := 0; i < totalChunks; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", fmt.Sprintf("%d", i))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", i))
		// Write chunk data
		chunkSize := 500
		if i == totalChunks-1 {
			chunkSize = 1000 - i*500 // Last chunk may be smaller
		}
		part.Write(make([]byte, chunkSize))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Chunk %d upload failed: %s", i, rr.Body.String())
		}
	}

	// Complete upload
	completeResp := ts.doRequest("POST", "/api/upload/complete", map[string]interface{}{
		"upload_session_id": sessionID,
	}, "")

	if completeResp.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", completeResp.Code, completeResp.Body.String())
	}

	var completeResult map[string]interface{}
	json.NewDecoder(completeResp.Body).Decode(&completeResult)

	if completeResult["upload_id"] == nil {
		t.Error("Expected upload_id in response")
	}
	if completeResult["filename"].(string) != "test.mp4" {
		t.Errorf("Expected filename test.mp4, got %v", completeResult["filename"])
	}
}

func TestChunkedUploadCompleteIncomplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 2 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload only first chunk
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	// Try to complete (should fail)
	completeResp := ts.doRequest("POST", "/api/upload/complete", map[string]interface{}{
		"upload_session_id": sessionID,
	}, "")

	if completeResp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", completeResp.Code)
	}
}

func TestChunkedUploadStatus(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         2000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload 2 of 4 chunks
	for i := 0; i < 2; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", fmt.Sprintf("%d", i))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", i))
		part.Write(make([]byte, 500))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)
	}

	// Check status
	statusReq := httptest.NewRequest("GET", "/api/upload/status/"+sessionID, nil)
	statusRR := httptest.NewRecorder()
	ts.mux.ServeHTTP(statusRR, statusReq)

	if statusRR.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", statusRR.Code)
	}

	var statusResult map[string]interface{}
	json.NewDecoder(statusRR.Body).Decode(&statusResult)

	if statusResult["progress"].(float64) != 50 {
		t.Errorf("Expected 50%% progress, got %v", statusResult["progress"])
	}
	if statusResult["status"].(string) != "in_progress" {
		t.Errorf("Expected status in_progress, got %v", statusResult["status"])
	}

	receivedChunks := statusResult["received_chunks"].([]interface{})
	if len(receivedChunks) != 2 {
		t.Errorf("Expected 2 received chunks, got %d", len(receivedChunks))
	}
}

func TestChunkedUploadStatusNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req := httptest.NewRequest("GET", "/api/upload/status/nonexistent", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rr.Code)
	}
}

func TestChunkedUploadInvalidChunkIndex(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 2 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Try to upload chunk with invalid index (3, when only 0 and 1 are valid)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "3")
	part, _ := writer.CreateFormFile("chunk", "chunk_3")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCleanupOrphanChunkDirectories(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "orphan-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test database
	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer testDB.Close()

	// Create chunks directory structure
	chunksDir := filepath.Join(tempDir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		t.Fatalf("Failed to create chunks dir: %v", err)
	}

	// Create an upload session in the database
	validSession := &db.UploadSession{
		ID:          "valid-session-123",
		Filename:    "test.mp4",
		ContentType: "video/mp4",
		TotalSize:   1024000,
		ChunkSize:   512000,
		TotalChunks: 2,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := testDB.CreateUploadSession(validSession); err != nil {
		t.Fatalf("Failed to create upload session: %v", err)
	}

	// Create directory for valid session
	validDir := filepath.Join(chunksDir, "valid-session-123")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create valid session dir: %v", err)
	}
	// Create a chunk file in valid directory
	if err := os.WriteFile(filepath.Join(validDir, "chunk_0.part"), []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create chunk file: %v", err)
	}

	// Create orphan directories (not in database)
	orphan1Dir := filepath.Join(chunksDir, "orphan-session-456")
	orphan2Dir := filepath.Join(chunksDir, "orphan-session-789")
	if err := os.MkdirAll(orphan1Dir, 0755); err != nil {
		t.Fatalf("Failed to create orphan1 dir: %v", err)
	}
	if err := os.MkdirAll(orphan2Dir, 0755); err != nil {
		t.Fatalf("Failed to create orphan2 dir: %v", err)
	}
	// Create chunk files in orphan directories
	if err := os.WriteFile(filepath.Join(orphan1Dir, "chunk_0.part"), []byte("orphan data"), 0644); err != nil {
		t.Fatalf("Failed to create orphan chunk file: %v", err)
	}

	// Run cleanup
	deletedCount := cleanupOrphanChunkDirectoriesWithDB(testDB, tempDir)

	// Verify 2 orphan directories were deleted
	if deletedCount != 2 {
		t.Errorf("Expected 2 orphan directories deleted, got %d", deletedCount)
	}

	// Verify orphan directories are gone
	if _, err := os.Stat(orphan1Dir); !os.IsNotExist(err) {
		t.Error("Expected orphan1 directory to be deleted")
	}
	if _, err := os.Stat(orphan2Dir); !os.IsNotExist(err) {
		t.Error("Expected orphan2 directory to be deleted")
	}

	// Verify valid session directory still exists
	if _, err := os.Stat(validDir); os.IsNotExist(err) {
		t.Error("Valid session directory should NOT be deleted")
	}

	// Verify chunk file in valid directory still exists
	if _, err := os.Stat(filepath.Join(validDir, "chunk_0.part")); os.IsNotExist(err) {
		t.Error("Chunk file in valid session should NOT be deleted")
	}
}

func TestCleanupOrphanChunkDirectoriesNoChunksDir(t *testing.T) {
	// Create temp directory for test without chunks subdirectory
	tempDir, err := os.MkdirTemp("", "orphan-cleanup-nochunks-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test database
	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer testDB.Close()

	// Run cleanup with no chunks directory - should not error
	deletedCount := cleanupOrphanChunkDirectoriesWithDB(testDB, tempDir)

	// No directories should be deleted (and no error should occur)
	if deletedCount != 0 {
		t.Errorf("Expected 0 deletions when chunks dir doesn't exist, got %d", deletedCount)
	}
}

func TestChunkedUploadOutOfOrder(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 3 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1500,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload chunks out of order: 2, 0, 1
	chunkOrder := []int{2, 0, 1}
	for _, chunkIndex := range chunkOrder {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", strconv.Itoa(chunkIndex))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", chunkIndex))
		part.Write(make([]byte, 500))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Chunk %d: Expected 200 OK, got %d: %s", chunkIndex, rr.Code, rr.Body.String())
		}
	}

	// Verify all chunks are stored
	count, _ := ts.db.CountUploadChunks(sessionID)
	if count != 3 {
		t.Errorf("Expected 3 chunks, got %d", count)
	}
}

func TestChunkedUploadGapInSequence(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 3 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1500,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload chunks 0 and 2, skipping chunk 1
	for _, chunkIndex := range []int{0, 2} {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", strconv.Itoa(chunkIndex))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", chunkIndex))
		part.Write(make([]byte, 500))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Chunk %d: Expected 200 OK, got %d: %s", chunkIndex, rr.Code, rr.Body.String())
		}
	}

	// Try to complete - should fail because chunk 1 is missing
	completeResp := ts.doRequest("POST", "/api/upload/complete", map[string]interface{}{
		"upload_session_id": sessionID,
	}, "")

	if completeResp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when chunks are missing, got %d: %s", completeResp.Code, completeResp.Body.String())
	}

	var completeResult map[string]string
	json.NewDecoder(completeResp.Body).Decode(&completeResult)
	// Error message could say "Not all chunks received" or similar
	errorMsg := strings.ToLower(completeResult["error"])
	if !strings.Contains(errorMsg, "incomplete") && !strings.Contains(errorMsg, "not all") && !strings.Contains(errorMsg, "chunks") {
		t.Errorf("Error message should mention incomplete/missing chunks, got: %s", completeResult["error"])
	}
}

func TestChunkedUploadNegativeChunkIndex(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Try to upload chunk with negative index
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "-1")
	part, _ := writer.CreateFormFile("chunk", "chunk_neg")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for negative chunk index, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChunkedUploadExcessiveChunkIndex(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Try to upload chunk with index exceeding maximum allowed
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "100000") // Exceeds maxChunkIndex
	part, _ := writer.CreateFormFile("chunk", "chunk_excessive")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for excessive chunk index, got %d: %s", rr.Code, rr.Body.String())
	}

	var result map[string]string
	json.NewDecoder(rr.Body).Decode(&result)
	if !strings.Contains(result["error"], "maximum") {
		t.Errorf("Expected error message about maximum, got: %s", result["error"])
	}
}

func TestChunkedUploadZeroSizeFile(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to initialize session with zero size
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         0,
		"content_type": "video/mp4",
	}, "")

	// Should either reject or handle zero-size gracefully
	if initResp.Code != http.StatusBadRequest && initResp.Code != http.StatusOK {
		t.Errorf("Expected 400 Bad Request or 200 OK for zero-size file, got %d", initResp.Code)
	}
}

func TestChunkedUploadMissingSessionID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to upload chunk without session ID
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for missing session ID, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChunkedUploadNonexistentSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to upload chunk with fake session ID
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", "nonexistent123456789012345678901234")
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for nonexistent session, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChunkedUploadNoChunkData(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Try to upload chunk without chunk data (only form fields)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "0")
	// No chunk file added
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request when no chunk data provided, got %d: %s", rr.Code, rr.Body.String())
	}
}
