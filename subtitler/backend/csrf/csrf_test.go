package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	// Generate token for a session
	sessionToken := "test-session-token-abc123"
	csrfToken := GenerateToken(sessionToken)

	if csrfToken == "" {
		t.Error("GenerateToken returned empty string")
	}

	// Token should be 64 hex chars (32 bytes SHA256)
	if len(csrfToken) != 64 {
		t.Errorf("Expected token length 64, got %d", len(csrfToken))
	}

	// Same session should produce same token
	csrfToken2 := GenerateToken(sessionToken)
	if csrfToken != csrfToken2 {
		t.Error("GenerateToken not deterministic for same input")
	}

	// Different session should produce different token
	otherToken := GenerateToken("different-session")
	if csrfToken == otherToken {
		t.Error("Different sessions produced same CSRF token")
	}
}

func TestGenerateTokenEmpty(t *testing.T) {
	csrfToken := GenerateToken("")
	if csrfToken != "" {
		t.Error("GenerateToken should return empty for empty session")
	}
}

func TestValidateToken(t *testing.T) {
	sessionToken := "test-session-token-xyz789"
	csrfToken := GenerateToken(sessionToken)

	// Valid token should pass
	if !ValidateToken(sessionToken, csrfToken) {
		t.Error("ValidateToken failed for valid token")
	}

	// Invalid token should fail
	if ValidateToken(sessionToken, "invalid-token") {
		t.Error("ValidateToken passed for invalid token")
	}

	// Wrong session should fail
	if ValidateToken("other-session", csrfToken) {
		t.Error("ValidateToken passed for wrong session")
	}

	// Empty tokens should fail
	if ValidateToken("", csrfToken) {
		t.Error("ValidateToken passed for empty session")
	}
	if ValidateToken(sessionToken, "") {
		t.Error("ValidateToken passed for empty CSRF token")
	}
}

func TestGetTokenFromRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set(HeaderName, "test-csrf-token")

	token := GetTokenFromRequest(req)
	if token != "test-csrf-token" {
		t.Errorf("Expected 'test-csrf-token', got '%s'", token)
	}

	// Missing header should return empty
	req2 := httptest.NewRequest("POST", "/api/test", nil)
	token2 := GetTokenFromRequest(req2)
	if token2 != "" {
		t.Errorf("Expected empty string, got '%s'", token2)
	}
}

func TestIsExemptPath(t *testing.T) {
	exemptPaths := []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/health",
		"/api/log",
	}

	for _, path := range exemptPaths {
		if !isExemptPath(path) {
			t.Errorf("Path %s should be exempt", path)
		}
	}

	nonExemptPaths := []string{
		"/api/auth/logout",
		"/api/upload",
		"/api/videos/123",
		"/api/auth/totp/setup",
	}

	for _, path := range nonExemptPaths {
		if isExemptPath(path) {
			t.Errorf("Path %s should NOT be exempt", path)
		}
	}
}

func TestIsStateChangingMethod(t *testing.T) {
	stateChanging := []string{"POST", "PUT", "DELETE", "PATCH"}
	for _, method := range stateChanging {
		if !isStateChangingMethod(method) {
			t.Errorf("Method %s should be state-changing", method)
		}
	}

	notStateChanging := []string{"GET", "HEAD", "OPTIONS"}
	for _, method := range notStateChanging {
		if isStateChangingMethod(method) {
			t.Errorf("Method %s should NOT be state-changing", method)
		}
	}
}

func TestProtectFuncAllowsGET(t *testing.T) {
	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return "session-token"
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	// No CSRF token on GET
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("Handler was not called for GET request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProtectFuncAllowsExemptPOST(t *testing.T) {
	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return "session-token"
	})

	req := httptest.NewRequest("POST", "/api/auth/login", nil)
	// No CSRF token on exempt path
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("Handler was not called for exempt POST request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProtectFuncBlocksPOSTWithoutToken(t *testing.T) {
	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return "session-token"
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	// No CSRF token
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("Handler should not be called without CSRF token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestProtectFuncBlocksPOSTWithInvalidToken(t *testing.T) {
	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return "session-token"
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set(HeaderName, "invalid-csrf-token")
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("Handler should not be called with invalid CSRF token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}

func TestProtectFuncAllowsPOSTWithValidToken(t *testing.T) {
	sessionToken := "valid-session-token"
	csrfToken := GenerateToken(sessionToken)

	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return sessionToken
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set(HeaderName, csrfToken)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("Handler was not called with valid CSRF token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestProtectFuncAllowsPOSTWithoutSession(t *testing.T) {
	// If there's no session, CSRF protection is skipped
	// (the request will fail auth anyway)
	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return "" // No session
	})

	req := httptest.NewRequest("POST", "/api/test", nil)
	// No CSRF token, but no session either
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("Handler should be called when no session (auth will fail anyway)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestMiddleware(t *testing.T) {
	sessionToken := "middleware-test-session"
	csrfToken := GenerateToken(sessionToken)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(func(r *http.Request) string {
		return sessionToken
	})

	protected := middleware(handler)

	// Test with valid token
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set(HeaderName, csrfToken)
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, req)

	if !called {
		t.Error("Middleware did not call handler with valid token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Test without token
	called = false
	req2 := httptest.NewRequest("POST", "/api/test", nil)
	w2 := httptest.NewRecorder()
	protected.ServeHTTP(w2, req2)

	if called {
		t.Error("Middleware called handler without CSRF token")
	}
	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w2.Code)
	}
}

func TestProtectFuncDELETE(t *testing.T) {
	sessionToken := "delete-test-session"
	csrfToken := GenerateToken(sessionToken)

	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return sessionToken
	})

	// DELETE should be protected
	req := httptest.NewRequest("DELETE", "/api/sessions/123", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("DELETE should be blocked without CSRF token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}

	// DELETE with valid token should pass
	called = false
	req2 := httptest.NewRequest("DELETE", "/api/sessions/123", nil)
	req2.Header.Set(HeaderName, csrfToken)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if !called {
		t.Error("DELETE should be allowed with valid CSRF token")
	}
	if w2.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w2.Code)
	}
}

func TestProtectFuncPUT(t *testing.T) {
	sessionToken := "put-test-session"
	csrfToken := GenerateToken(sessionToken)

	called := false
	handler := ProtectFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}, func(r *http.Request) string {
		return sessionToken
	})

	// PUT should be protected
	req := httptest.NewRequest("PUT", "/api/segments", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Error("PUT should be blocked without CSRF token")
	}

	// PUT with valid token should pass
	called = false
	req2 := httptest.NewRequest("PUT", "/api/segments", nil)
	req2.Header.Set(HeaderName, csrfToken)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if !called {
		t.Error("PUT should be allowed with valid CSRF token")
	}
}
