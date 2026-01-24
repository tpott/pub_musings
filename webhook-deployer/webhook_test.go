package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractBranch(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"refs/heads/main", "main"},
		{"refs/heads/trunk", "trunk"},
		{"refs/heads/feature/new-thing", "feature/new-thing"},
		{"refs/heads/subtitler_v3", "subtitler_v3"},
		{"refs/tags/v1.0.0", ""},
		{"main", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := extractBranch(tt.ref)
			if got != tt.want {
				t.Errorf("extractBranch(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestExtractChangedFiles(t *testing.T) {
	commits := []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	}{
		{
			Added:    []string{"new-file.txt"},
			Modified: []string{"changed.txt"},
			Removed:  []string{},
		},
		{
			Added:    []string{},
			Modified: []string{"changed.txt", "another.txt"}, // changed.txt appears again
			Removed:  []string{"deleted.txt"},
		},
	}

	files := extractChangedFiles(commits)

	// Should have 4 unique files
	if len(files) != 4 {
		t.Errorf("Expected 4 files, got %d: %v", len(files), files)
	}

	expected := map[string]bool{
		"new-file.txt": true,
		"changed.txt":  true,
		"another.txt":  true,
		"deleted.txt":  true,
	}

	for _, f := range files {
		if !expected[f] {
			t.Errorf("Unexpected file: %s", f)
		}
	}
}

func TestValidateSignature(t *testing.T) {
	secret := "test-secret"
	handler := &WebhookHandler{secret: secret}

	body := []byte(`{"test": "payload"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		want      bool
	}{
		{"valid signature", validSig, true},
		{"empty signature", "", false},
		{"wrong prefix", "sha1=" + hex.EncodeToString(mac.Sum(nil)), false},
		{"invalid hex", "sha256=notvalidhex", false},
		{"wrong secret", "sha256=abcd1234", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.validateSignature(body, tt.signature)
			if got != tt.want {
				t.Errorf("validateSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleNonPushEvent(t *testing.T) {
	config := &Config{Sites: []SiteConfig{}}
	handler := NewWebhookHandler("secret", config)

	body := `{}`
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "ping")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Event ignored") {
		t.Errorf("Expected 'Event ignored', got %q", rr.Body.String())
	}
}

func TestHandleInvalidSignature(t *testing.T) {
	config := &Config{Sites: []SiteConfig{}}
	handler := NewWebhookHandler("secret", config)

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestHandleNoMatchingSites(t *testing.T) {
	config := &Config{
		Sites: []SiteConfig{
			{
				Name:       "test",
				Path:       "/tmp",
				PathPrefix: "src/",
				Branch:     "main",
				Repository: "user/repo",
				Commands:   []string{"echo"},
			},
		},
	}
	handler := NewWebhookHandler("secret", config)

	body := `{
		"ref": "refs/heads/develop",
		"repository": {"full_name": "user/repo"},
		"pusher": {"name": "test"},
		"commits": [{"added": ["src/file.txt"], "modified": [], "removed": []}]
	}`

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "No matching sites") {
		t.Errorf("Expected 'No matching sites', got %q", rr.Body.String())
	}
}

func TestHandleMatchingSite(t *testing.T) {
	config := &Config{
		Sites: []SiteConfig{
			{
				Name:       "test-site",
				Path:       "/tmp",
				PathPrefix: "src/",
				Branch:     "main",
				Repository: "user/repo",
				Commands:   []string{"echo deployed"},
			},
		},
	}
	handler := NewWebhookHandler("secret", config)

	body := `{
		"ref": "refs/heads/main",
		"repository": {"full_name": "user/repo"},
		"pusher": {"name": "test"},
		"commits": [{"added": ["src/file.txt"], "modified": [], "removed": []}]
	}`

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "test-site") {
		t.Errorf("Expected response to contain 'test-site', got %q", rr.Body.String())
	}
}

func TestSiteNames(t *testing.T) {
	sites := []SiteConfig{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}

	names := siteNames(sites)

	if len(names) != 3 {
		t.Fatalf("Expected 3 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" || names[2] != "gamma" {
		t.Errorf("Unexpected names: %v", names)
	}
}
