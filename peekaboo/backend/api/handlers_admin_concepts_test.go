package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateConcept_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	body := `{"id": "horse", "name": "Horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminConceptResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.ID != "horse" {
		t.Errorf("Expected id=horse, got %q", resp.ID)
	}
	if resp.Name != "Horse" {
		t.Errorf("Expected name=Horse, got %q", resp.Name)
	}

	// Verify it's in the database
	name, err := database.GetConcept("horse")
	if err != nil {
		t.Fatalf("GetConcept failed: %v", err)
	}
	if name != "Horse" {
		t.Errorf("GetConcept(horse) = %q, want Horse", name)
	}
}

func TestCreateConcept_Conflict(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// "cat" already exists from seed data
	body := `{"id": "cat", "name": "Cat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminConceptResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "concept already exists" {
		t.Errorf("Expected error='concept already exists', got %q", resp.Error)
	}
}

func TestCreateConcept_MissingID(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	body := `{"name": "Horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_MissingName(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	body := `{"id": "horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_InvalidID(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	tests := []struct {
		name string
		id   string
	}{
		{"uppercase", "Horse"},
		{"spaces", "sea horse"},
		{"special chars", "horse!"},
		{"path traversal", "../etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"id": "` + tt.id + `", "name": "Horse"}`
			req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			handler.HandleCreateConcept(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for id=%q, got %d: %s", tt.id, w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateConcept_IDTooLong(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	longID := strings.Repeat("a", 51)
	body := `{"id": "` + longID + `", "name": "Long"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_NameTooLong(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	longName := strings.Repeat("A", 101)
	body := `{"id": "test", "name": "` + longName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAdminHandler(database, "some-user-id")

	body := `{"id": "horse", "name": "Horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestCreateConcept_Forbidden(t *testing.T) {
	database := setupAuthTestDB(t)
	_, token := createAdminSession(t, database, "normie@example.com")
	handler := NewAdminHandler(database, "other-user-id")

	body := `{"id": "horse", "name": "Horse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_BodyTooLarge(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	bigBody := `{"id": "horse", "name": "` + strings.Repeat("x", 2000) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(bigBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListConcepts_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/concepts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListConcepts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminConceptsListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have 6 seed concepts
	if len(resp.Concepts) != 6 {
		t.Fatalf("Expected 6 concepts, got %d", len(resp.Concepts))
	}

	// All should have 0 media sets (none seeded)
	for _, c := range resp.Concepts {
		if c.MediaSetCount != 0 {
			t.Errorf("Concept %q has %d media sets, want 0", c.ID, c.MediaSetCount)
		}
	}
}

func TestListConcepts_WithMediaSets(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// Seed media for cat
	if err := database.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3", ""); err != nil {
		t.Fatalf("SeedMediaSet failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/concepts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListConcepts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminConceptsListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	for _, c := range resp.Concepts {
		if c.ID == "cat" {
			if c.MediaSetCount != 1 {
				t.Errorf("cat media_set_count = %d, want 1", c.MediaSetCount)
			}
		}
	}
}

func TestListConcepts_IncludesNewConcepts(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// Create a new concept first
	if err := database.InsertConcept("horse", "Horse"); err != nil {
		t.Fatalf("InsertConcept failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/concepts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListConcepts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminConceptsListResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Concepts) != 7 {
		t.Fatalf("Expected 7 concepts (6+horse), got %d", len(resp.Concepts))
	}

	found := false
	for _, c := range resp.Concepts {
		if c.ID == "horse" {
			found = true
			if c.Name != "Horse" {
				t.Errorf("horse name = %q, want Horse", c.Name)
			}
		}
	}
	if !found {
		t.Error("horse not found in response")
	}
}

func TestListConcepts_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAdminHandler(database, "some-user-id")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/concepts", nil)
	w := httptest.NewRecorder()
	handler.HandleListConcepts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestListConcepts_Forbidden(t *testing.T) {
	database := setupAuthTestDB(t)
	_, token := createAdminSession(t, database, "normie@example.com")
	handler := NewAdminHandler(database, "other-user-id")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/concepts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListConcepts(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConcept_ValidIDFormats(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	tests := []struct {
		id   string
		name string
	}{
		{"sea_turtle", "Sea Turtle"},
		{"animal123", "Animal 123"},
		{"a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			body := `{"id": "` + tt.id + `", "name": "` + tt.name + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			handler.HandleCreateConcept(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected 201 for id=%q, got %d: %s", tt.id, w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateConcept_InvalidJSON(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/concepts", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleCreateConcept(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
