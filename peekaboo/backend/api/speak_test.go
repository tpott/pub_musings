package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockTTSProvider implements tts.Provider for testing
type mockTTSProvider struct {
	synthesizeFunc func(ctx context.Context, text string) ([]byte, error)
}

func (m *mockTTSProvider) Synthesize(ctx context.Context, text string) ([]byte, error) {
	return m.synthesizeFunc(ctx, text)
}

func TestSpeakHandler_Success(t *testing.T) {
	expectedAudio := []byte("RIFF....WAVEfmt ...data...")

	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			if text != "Here is a cat" {
				t.Errorf("Expected text 'Here is a cat', got %q", text)
			}
			return expectedAudio, nil
		},
	}

	handler := NewSpeakHandler(provider)

	reqBody, _ := json.Marshal(SpeakRequest{Text: "Here is a cat"})
	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if rr.Header().Get("Content-Type") != "audio/wav" {
		t.Errorf("Expected Content-Type audio/wav, got %s", rr.Header().Get("Content-Type"))
	}

	if !bytes.Equal(rr.Body.Bytes(), expectedAudio) {
		t.Errorf("Unexpected audio response")
	}
}

func TestSpeakHandler_MissingText(t *testing.T) {
	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			t.Fatal("Synthesize should not be called")
			return nil, nil
		},
	}

	handler := NewSpeakHandler(provider)

	reqBody, _ := json.Marshal(SpeakRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp SpeakResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "missing text field" {
		t.Errorf("Expected 'missing text field' error, got %q", resp.Error)
	}
}

func TestSpeakHandler_TextTooLong(t *testing.T) {
	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			t.Fatal("Synthesize should not be called")
			return nil, nil
		},
	}

	handler := NewSpeakHandler(provider)

	// Create text longer than 256 characters
	longText := strings.Repeat("a", 257)

	reqBody, _ := json.Marshal(SpeakRequest{Text: longText})
	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp SpeakResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "text too long (max 256 characters)" {
		t.Errorf("Expected 'text too long' error, got %q", resp.Error)
	}
}

func TestSpeakHandler_TextAtMaxLength(t *testing.T) {
	expectedAudio := []byte("RIFF....WAVEfmt ...data...")

	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			return expectedAudio, nil
		},
	}

	handler := NewSpeakHandler(provider)

	// Text at exactly 256 characters should be accepted
	maxText := strings.Repeat("a", 256)

	reqBody, _ := json.Marshal(SpeakRequest{Text: maxText})
	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for text at max length, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSpeakHandler_InvalidJSON(t *testing.T) {
	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			t.Fatal("Synthesize should not be called")
			return nil, nil
		},
	}

	handler := NewSpeakHandler(provider)

	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp SpeakResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "invalid JSON body" {
		t.Errorf("Expected 'invalid JSON body' error, got %q", resp.Error)
	}
}

func TestSpeakHandler_WrongMethod(t *testing.T) {
	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			t.Fatal("Synthesize should not be called")
			return nil, nil
		},
	}

	handler := NewSpeakHandler(provider)

	req := httptest.NewRequest(http.MethodGet, "/api/speak", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestSpeakHandler_SynthesisError(t *testing.T) {
	provider := &mockTTSProvider{
		synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
			return nil, errors.New("synthesis failed")
		},
	}

	handler := NewSpeakHandler(provider)

	reqBody, _ := json.Marshal(SpeakRequest{Text: "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	var resp SpeakResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	// Error should be generic
	if resp.Error != "speech synthesis failed" {
		t.Errorf("Expected generic error 'speech synthesis failed', got %q", resp.Error)
	}
}

func TestSpeakHandler_DifferentPhrases(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"cat", "Here is a cat!"},
		{"dog", "This is a dog!"},
		{"duck", "Look at the duck!"},
		{"unknown", "I don't know that animal"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := &mockTTSProvider{
				synthesizeFunc: func(ctx context.Context, text string) ([]byte, error) {
					if text != tc.text {
						t.Errorf("Expected text %q, got %q", tc.text, text)
					}
					return []byte("RIFF....WAVEfmt ...data..."), nil
				},
			}

			handler := NewSpeakHandler(provider)

			reqBody, _ := json.Marshal(SpeakRequest{Text: tc.text})
			req := httptest.NewRequest(http.MethodPost, "/api/speak", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}
