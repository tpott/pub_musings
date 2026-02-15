package captcha

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_Disabled(t *testing.T) {
	v := New(Config{})
	if v.IsEnabled() {
		t.Error("Verifier should be disabled when no secret key provided")
	}

	// Should always succeed when disabled
	err := v.Verify(context.Background(), "", "")
	if err != nil {
		t.Errorf("Disabled verifier should always succeed, got: %v", err)
	}
}

func TestNew_Enabled(t *testing.T) {
	v := New(Config{
		SiteKey:   "test-site-key",
		SecretKey: "test-secret-key",
	})
	if !v.IsEnabled() {
		t.Error("Verifier should be enabled when secret key provided")
	}
}

func TestHCaptchaVerifier_MissingToken(t *testing.T) {
	v := &hCaptchaVerifier{
		secretKey: "test-secret",
		client:    http.DefaultClient,
	}

	err := v.Verify(context.Background(), "", "127.0.0.1")
	if err != ErrMissingToken {
		t.Errorf("Expected ErrMissingToken, got: %v", err)
	}
}

func TestHCaptchaVerifier_SuccessfulVerification(t *testing.T) {
	// This test uses the mock verifier since we can't inject URLs
	// into the real hCaptcha verifier (const URL).
	// Real integration tests would use hCaptcha's test keys.
	v := NewMockVerifier(true)

	err := v.Verify(context.Background(), "valid-token", "127.0.0.1")
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
	if v.LastToken != "valid-token" {
		t.Errorf("Expected token 'valid-token', got '%s'", v.LastToken)
	}
	if v.LastRemoteIP != "127.0.0.1" {
		t.Errorf("Expected IP '127.0.0.1', got '%s'", v.LastRemoteIP)
	}
}

func TestHCaptchaVerifier_FailedVerification(t *testing.T) {
	// Test uses the mock verifier since we can't inject the URL into the real one
	v := NewMockVerifier(true)
	v.ShouldFail = true

	err := v.Verify(context.Background(), "invalid-token", "127.0.0.1")
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got: %v", err)
	}
}

func TestMockVerifier(t *testing.T) {
	t.Run("enabled state", func(t *testing.T) {
		v := NewMockVerifier(true)
		if !v.IsEnabled() {
			t.Error("MockVerifier should report enabled when created with true")
		}

		v2 := NewMockVerifier(false)
		if v2.IsEnabled() {
			t.Error("MockVerifier should report disabled when created with false")
		}
	})

	t.Run("tracks calls", func(t *testing.T) {
		v := NewMockVerifier(true)

		if v.VerifyCalls != 0 {
			t.Error("VerifyCalls should start at 0")
		}

		v.Verify(context.Background(), "token1", "ip1")
		if v.VerifyCalls != 1 {
			t.Error("VerifyCalls should be 1 after first call")
		}
		if v.LastToken != "token1" {
			t.Errorf("LastToken should be 'token1', got '%s'", v.LastToken)
		}
		if v.LastRemoteIP != "ip1" {
			t.Errorf("LastRemoteIP should be 'ip1', got '%s'", v.LastRemoteIP)
		}

		v.Verify(context.Background(), "token2", "ip2")
		if v.VerifyCalls != 2 {
			t.Error("VerifyCalls should be 2 after second call")
		}
		if v.LastToken != "token2" {
			t.Error("LastToken should be updated to 'token2'")
		}
	})

	t.Run("configurable failure", func(t *testing.T) {
		v := NewMockVerifier(true)

		// Default: succeeds
		err := v.Verify(context.Background(), "token", "ip")
		if err != nil {
			t.Errorf("Should succeed by default, got: %v", err)
		}

		// Configure to fail
		v.ShouldFail = true
		err = v.Verify(context.Background(), "token", "ip")
		if err != ErrInvalidToken {
			t.Errorf("Should fail with ErrInvalidToken, got: %v", err)
		}
	})

	t.Run("require token", func(t *testing.T) {
		v := NewMockVerifier(true)
		v.RequireToken = true

		err := v.Verify(context.Background(), "", "ip")
		if err != ErrMissingToken {
			t.Errorf("Should fail with ErrMissingToken when token empty, got: %v", err)
		}

		err = v.Verify(context.Background(), "valid-token", "ip")
		if err != nil {
			t.Errorf("Should succeed with valid token, got: %v", err)
		}
	})
}

func TestHCaptchaVerifier_WithTestServer(t *testing.T) {
	t.Run("successful verification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success": true}`)
		}))
		defer server.Close()

		v := &hCaptchaVerifier{
			secretKey: "test-secret",
			verifyURL: server.URL,
			client:    server.Client(),
		}

		err := v.Verify(context.Background(), "valid-token", "127.0.0.1")
		if err != nil {
			t.Errorf("Expected success, got: %v", err)
		}
	})

	t.Run("failed verification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success": false, "error-codes": ["invalid-input-response"]}`)
		}))
		defer server.Close()

		v := &hCaptchaVerifier{
			secretKey: "test-secret",
			verifyURL: server.URL,
			client:    server.Client(),
		}

		err := v.Verify(context.Background(), "invalid-token", "127.0.0.1")
		if err != ErrInvalidToken {
			t.Errorf("Expected ErrInvalidToken, got: %v", err)
		}
	})

	t.Run("oversized response rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Send a response larger than maxCaptchaResponseSize
			// The JSON decoder will fail because the body is truncated mid-stream
			w.Write([]byte(`{"success": true, "padding": "`))
			w.Write([]byte(strings.Repeat("x", maxCaptchaResponseSize+1)))
			w.Write([]byte(`"}`))
		}))
		defer server.Close()

		v := &hCaptchaVerifier{
			secretKey: "test-secret",
			verifyURL: server.URL,
			client:    server.Client(),
		}

		err := v.Verify(context.Background(), "valid-token", "127.0.0.1")
		if err != ErrServiceUnavailable {
			t.Errorf("Expected ErrServiceUnavailable for oversized response, got: %v", err)
		}
	})

	t.Run("invalid JSON rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>Server Error</html>")
		}))
		defer server.Close()

		v := &hCaptchaVerifier{
			secretKey: "test-secret",
			verifyURL: server.URL,
			client:    server.Client(),
		}

		err := v.Verify(context.Background(), "valid-token", "127.0.0.1")
		if err != ErrServiceUnavailable {
			t.Errorf("Expected ErrServiceUnavailable for invalid JSON, got: %v", err)
		}
	})
}

func TestDisabledVerifier(t *testing.T) {
	v := &disabledVerifier{}

	if v.IsEnabled() {
		t.Error("disabledVerifier should always return IsEnabled=false")
	}

	// Should always succeed
	tests := []struct {
		token    string
		remoteIP string
	}{
		{"", ""},
		{"any-token", "127.0.0.1"},
		{"", "192.168.1.1"},
	}

	for _, tt := range tests {
		err := v.Verify(context.Background(), tt.token, tt.remoteIP)
		if err != nil {
			t.Errorf("disabledVerifier.Verify(%q, %q) should always succeed, got: %v", tt.token, tt.remoteIP, err)
		}
	}
}
