package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

func createAdminSession(t *testing.T, database *db.DB, email string) (*db.User, string) {
	t.Helper()
	user := createTestUser(t, database, email, "password123", true)

	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: auth.HashToken(sessionToken),
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return user, sessionToken
}

func insertTestFeedback(t *testing.T, database *db.DB, id, feedbackType, message string) {
	t.Helper()
	f := &db.Feedback{
		ID:           id,
		FeedbackType: feedbackType,
		Message:      message,
		SessionID:    "test-session",
		PageURL:      "/",
	}
	if err := database.InsertFeedback(f); err != nil {
		t.Fatalf("InsertFeedback failed: %v", err)
	}
}

func TestAdminFeedback_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	insertTestFeedback(t, database, "fb-1", "bug", "Something broke")
	insertTestFeedback(t, database, "fb-2", "feature", "Add elephants")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=new", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected total=2, got %d", resp.Total)
	}
	if len(resp.Feedback) != 2 {
		t.Errorf("Expected 2 items, got %d", len(resp.Feedback))
	}
	if resp.Limit != 50 {
		t.Errorf("Expected limit=50, got %d", resp.Limit)
	}
}

func TestAdminFeedback_DefaultStatusNew(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	insertTestFeedback(t, database, "fb-1", "bug", "Bug report")

	// No status param — should default to "new"
	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("Expected total=1, got %d", resp.Total)
	}
}

func TestAdminFeedback_StatusAll(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	insertTestFeedback(t, database, "fb-1", "bug", "Bug report")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("Expected total=1, got %d", resp.Total)
	}
}

func TestAdminFeedback_Limit(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	for i := 0; i < 5; i++ {
		insertTestFeedback(t, database, "fb-"+string(rune('a'+i)), "general", "Test")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=new&limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Feedback) != 2 {
		t.Errorf("Expected 2 items with limit=2, got %d", len(resp.Feedback))
	}
	if resp.Total != 5 {
		t.Errorf("Expected total=5, got %d", resp.Total)
	}
	if resp.Limit != 2 {
		t.Errorf("Expected limit=2, got %d", resp.Limit)
	}
}

func TestAdminFeedback_AfterCursor(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	insertTestFeedback(t, database, "fb-old", "bug", "Old feedback")

	// Use a future timestamp so the old feedback is excluded
	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=new&after="+future, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 0 {
		t.Errorf("Expected total=0 with future cursor, got %d", resp.Total)
	}
}

func TestAdminFeedback_InvalidAfter(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?after=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminFeedback_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAdminHandler(database, "some-user-id")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestAdminFeedback_Forbidden(t *testing.T) {
	database := setupAuthTestDB(t)
	_, token := createAdminSession(t, database, "normie@example.com")
	handler := NewAdminHandler(database, "other-user-id") // different trusted user

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminFeedback_NoTrustedUsers(t *testing.T) {
	database := setupAuthTestDB(t)
	_, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, "") // empty TRUSTED_USERS

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 when TRUSTED_USERS empty, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Error != "TRUSTED_USERS not configured" {
		t.Errorf("Expected error='TRUSTED_USERS not configured', got %q", resp.Error)
	}
}

func TestAdminFeedback_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAdminHandler(database, "user-id")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/feedback", nil)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestAdminFeedback_ResponseFormat(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	rating := 4
	f := &db.Feedback{
		ID:           "fb-format",
		FeedbackType: "bug",
		Rating:       &rating,
		Message:      "Button doesn't work",
		SessionID:    "session-123",
		PageURL:      "/home",
	}
	if err := database.InsertFeedback(f); err != nil {
		t.Fatalf("InsertFeedback failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=new", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminFeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Feedback) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(resp.Feedback))
	}

	item := resp.Feedback[0]
	if item.ID != "fb-format" {
		t.Errorf("Expected id=fb-format, got %q", item.ID)
	}
	if item.Type != "bug" {
		t.Errorf("Expected type=bug, got %q", item.Type)
	}
	if item.Rating == nil || *item.Rating != 4 {
		t.Errorf("Expected rating=4, got %v", item.Rating)
	}
	if item.Message != "Button doesn't work" {
		t.Errorf("Expected message='Button doesn't work', got %q", item.Message)
	}
	if item.PageURL != "/home" {
		t.Errorf("Expected page_url=/home, got %q", item.PageURL)
	}
	if item.CreatedAt == "" {
		t.Error("Expected non-empty created_at")
	}
	if item.Status != "new" {
		t.Errorf("Expected status=new, got %q", item.Status)
	}
}

func TestAdminFeedback_BearerAuth(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminFeedback_CookieAuth(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminFeedback_MultipleTrustedUsers(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin2@example.com")
	// User is the second in a comma-separated list
	handler := NewAdminHandler(database, "other-id,"+user.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleListFeedback(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 for trusted user in list, got %d: %s", w.Code, w.Body.String())
	}
}
