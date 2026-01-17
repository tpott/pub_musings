package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandleUpload(t *testing.T) {
	// Ensure uploads directory exists
	defer os.RemoveAll("./uploads")

	tests := []struct {
		name           string
		setupRequest   func() (*http.Request, error)
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "valid file upload",
			setupRequest: func() (*http.Request, error) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", "test.mp3")
				if err != nil {
					return nil, err
				}
				part.Write([]byte("test audio content"))
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req, nil
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "invalid file type",
			setupRequest: func() (*http.Request, error) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", "test.pdf")
				if err != nil {
					return nil, err
				}
				part.Write([]byte("test pdf content"))
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "missing file",
			setupRequest: func() (*http.Request, error) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "empty file",
			setupRequest: func() (*http.Request, error) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", "empty.mp3")
				if err != nil {
					return nil, err
				}
				part.Write([]byte{})
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				return req, nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
		{
			name: "GET method not allowed",
			setupRequest: func() (*http.Request, error) {
				req := httptest.NewRequest(http.MethodGet, "/api/upload", nil)
				return req, nil
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  true,
		},
		{
			name: "OPTIONS preflight request",
			setupRequest: func() (*http.Request, error) {
				req := httptest.NewRequest(http.MethodOptions, "/api/upload", nil)
				return req, nil
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := tt.setupRequest()
			if err != nil {
				t.Fatalf("Failed to setup request: %v", err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleUpload)
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check CORS headers are set
			if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4321" {
				t.Errorf("CORS header not set correctly")
			}
		})
	}
}

func TestHandleUploadOversizedFile(t *testing.T) {
	// Skip in short mode as this test creates a large file
	if testing.Short() {
		t.Skip("Skipping oversized file test in short mode")
	}

	defer os.RemoveAll("./uploads")

	// Create a file that's larger than the max size
	oversizedContent := make([]byte, 201*1024*1024) // 201MB

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "huge.mp4")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	// Write in chunks to avoid memory issues
	chunkSize := 10 * 1024 * 1024 // 10MB chunks
	for i := 0; i < len(oversizedContent); i += chunkSize {
		end := i + chunkSize
		if end > len(oversizedContent) {
			end = len(oversizedContent)
		}
		_, err := part.Write(oversizedContent[i:end])
		if err != nil {
			t.Fatalf("Failed to write chunk: %v", err)
		}
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleUpload)
	handler.ServeHTTP(rr, req)

	// Should return 400 Bad Request due to file size validation
	if rr.Code != http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(rr.Body)
		t.Errorf("Expected status 400, got %d. Body: %s", rr.Code, string(bodyBytes))
	}
}
