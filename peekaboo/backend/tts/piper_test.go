package tts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewPiperProvider_RequiresServerURL(t *testing.T) {
	_, err := NewPiperProvider(PiperConfig{})
	if err == nil {
		t.Error("Expected error for missing server URL")
	}
	if !strings.Contains(err.Error(), "server URL is required") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestNewPiperProvider_Success(t *testing.T) {
	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: "http://localhost:5000",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}

func TestPiperProvider_Synthesize_EmptyText(t *testing.T) {
	provider, _ := NewPiperProvider(PiperConfig{
		ServerURL: "http://localhost:5000",
	})

	_, err := provider.Synthesize(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty text")
	}
	if !strings.Contains(err.Error(), "text is required") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestPiperProvider_Synthesize_Success(t *testing.T) {
	// Create mock Piper server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Read and verify request body
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"text":"Hello world"`) {
			t.Errorf("Expected text in request body, got: %s", string(body))
		}

		// Return fake WAV data
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("RIFF....WAVEfmt ...data..."))
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	audio, err := provider.Synthesize(context.Background(), "Hello world")
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}

	if len(audio) == 0 {
		t.Error("Expected non-empty audio data")
	}

	// Verify it starts with RIFF (WAV header)
	if !strings.HasPrefix(string(audio), "RIFF") {
		t.Error("Expected WAV audio data starting with RIFF")
	}
}

func TestPiperProvider_Synthesize_WithLengthScale(t *testing.T) {
	// Create mock server that verifies length_scale is passed
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"length_scale":0.8`) {
			t.Errorf("Expected length_scale in request body, got: %s", string(body))
		}

		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("RIFF....WAVEfmt ...data..."))
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL:   mockServer.URL,
		LengthScale: 0.8,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "Fast speech")
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
}

func TestPiperProvider_Synthesize_DefaultLengthScale(t *testing.T) {
	// Create mock server that verifies length_scale is NOT included when default
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "length_scale") {
			t.Errorf("Expected no length_scale when default, got: %s", string(body))
		}

		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("RIFF....WAVEfmt ...data..."))
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: mockServer.URL,
		// LengthScale not set, should default to 1.0 and not be included in JSON
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "Normal speech")
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
}

func TestPiperProvider_Synthesize_ServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "Test")
	if err == nil {
		t.Error("Expected error for server error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("Expected status code in error, got: %v", err)
	}
}

func TestPiperProvider_Synthesize_OversizedResponse(t *testing.T) {
	// Create mock server that returns a response exceeding the limit
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		// Write just over the limit
		data := make([]byte, maxAudioResponseSize+1)
		copy(data, []byte("RIFF"))
		w.Write(data)
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	_, err = provider.Synthesize(context.Background(), "Test")
	if err == nil {
		t.Error("Expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("Expected size limit error, got: %v", err)
	}
}

func TestPiperProvider_Synthesize_ContextCanceled(t *testing.T) {
	// Create mock server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context is canceled
		select {
		case <-r.Context().Done():
			return
		default:
			w.Write([]byte("RIFF....WAVEfmt ...data..."))
		}
	}))
	defer mockServer.Close()

	provider, err := NewPiperProvider(PiperConfig{
		ServerURL: mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = provider.Synthesize(ctx, "Test")
	if err == nil {
		t.Error("Expected error for canceled context")
	}
}

func TestNewProvider_Piper(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider:  "piper",
		ServerURL: "http://localhost:5000",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}

func TestNewProvider_Default(t *testing.T) {
	// Empty provider should default to piper
	provider, err := NewProvider(Config{
		ServerURL: "http://localhost:5000",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected provider to be non-nil")
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider(Config{
		Provider:  "unknown",
		ServerURL: "http://localhost:5000",
	})
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown TTS provider") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestNewProvider_MissingServerURL(t *testing.T) {
	_, err := NewProvider(Config{
		Provider: "piper",
		// ServerURL missing
	})
	if err == nil {
		t.Error("Expected error for missing server URL")
	}
}
