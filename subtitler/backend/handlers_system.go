package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/security"
	"github.com/tpott/subtitler/backend/validation"
)

func registerSystemHandlers(mux *http.ServeMux) {

	// Health check endpoint
	// Unauthenticated: returns only {"status": "ok"} or {"status": "degraded"}
	// Authenticated: returns full response with all dependency details
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {

		status := HealthStatus{
			Status:           "ok",
			DBConnected:      true,
			WhisperAvailable: true,
			DiskSpaceOK:      true,
			Errors:           []string{},
		}

		// Check database connectivity
		if err := database.Ping(); err != nil {
			status.DBConnected = false
			status.Errors = append(status.Errors, fmt.Sprintf("database: %v", err))
		}

		// Check whisper-server availability (if configured)
		whisperOK, whisperErr := checkWhisperServerHealth()
		if !whisperOK {
			status.WhisperAvailable = false
			status.Errors = append(status.Errors, whisperErr)
		}

		// Check disk space (minimum 1GB free for uploads)
		diskOK, freeGB, diskErr := checkDiskSpace(".", 1.0)
		status.DiskFreeGB = freeGB
		if !diskOK {
			status.DiskSpaceOK = false
			status.Errors = append(status.Errors, diskErr)
		}

		// Determine overall status
		if !status.DBConnected || !status.WhisperAvailable || !status.DiskSpaceOK {
			status.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		if user != nil {
			// Authenticated: return full response
			httputil.RespondJSON(w, http.StatusOK, status)
		} else {
			// Unauthenticated: return minimal response
			httputil.RespondJSON(w, http.StatusOK, map[string]string{
				"status": status.Status,
			})
		}
	})

	// Prometheus metrics endpoint
	// Protected by API key (via METRICS_API_KEY env var) or admin authentication
	// Rate limited to prevent reconnaissance attacks
	mux.HandleFunc("GET /metrics", metricsLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Check API key first
		apiKey := r.Header.Get("X-Metrics-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if metricsAPIKey != "" && apiKey == metricsAPIKey {
			// Valid API key, serve metrics
			metrics.Handler().ServeHTTP(w, r)
			return
		}

		// Fall back to checking for admin user
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid session")
			return
		}

		// Check if user has admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/metrics")
			httputil.RespondError(w, http.StatusForbidden, "Admin access required")
			return
		}

		metrics.Handler().ServeHTTP(w, r)
	}))

	// Frontend log forwarding endpoint (for dev mode debugging)
	mux.HandleFunc("POST /api/log", logLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Parse request body
		var req struct {
			Level   string        `json:"level"`   // log, warn, error, info, debug
			Message string        `json:"message"` // formatted message string
			Args    []interface{} `json:"args"`    // additional arguments (optional)
			URL     string        `json:"url"`     // page URL where log originated
			Line    int           `json:"line"`    // line number (optional)
			Column  int           `json:"column"`  // column number (optional)
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate level
		validLevels := map[string]bool{"log": true, "warn": true, "error": true, "info": true, "debug": true}
		if !validLevels[req.Level] {
			req.Level = "log"
		}

		// Log frontend message
		attrs := []any{"level", req.Level, "message", req.Message}
		if req.URL != "" {
			attrs = append(attrs, "url", req.URL)
			if req.Line > 0 {
				attrs = append(attrs, "line", req.Line)
				if req.Column > 0 {
					attrs = append(attrs, "column", req.Column)
				}
			}
		}
		logging.DebugContext(r.Context(), "Frontend log", attrs...)

		httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// Feedback submission endpoint (rate limited)
	mux.HandleFunc("POST /api/feedback", feedbackLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Parse request body (limited to 32KB - feedback text is capped at 10KB)
		var req struct {
			Text        string  `json:"text"`
			Type        string  `json:"type"`
			Rating      *int    `json:"rating"`
			PageURL     string  `json:"page_url"`
			VideoID     *string `json:"video_id"`
			SessionID   *string `json:"session_id"`
			BrowserInfo string  `json:"browser_info"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 32*1024); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate required fields
		if req.Text == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Feedback text is required")
			return
		}

		// Validate text length (max 10KB)
		if len(req.Text) > 10240 {
			httputil.RespondError(w, http.StatusBadRequest, "Feedback text is too long (max 10KB)")
			return
		}

		// Validate type
		validTypes := map[string]bool{
			db.FeedbackTypeGeneral: true,
			db.FeedbackTypeBug:     true,
			db.FeedbackTypeFeature: true,
		}
		if req.Type == "" {
			req.Type = db.FeedbackTypeGeneral
		} else if !validTypes[req.Type] {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid feedback type")
			return
		}

		// Validate rating if provided
		if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
			httputil.RespondError(w, http.StatusBadRequest, "Rating must be between 1 and 5")
			return
		}

		// Get user context if authenticated
		var userID *string
		token := auth.GetTokenFromRequest(r)
		if token != "" {
			user, _, _ := auth.ValidateSession(database, token)
			if user != nil {
				userID = &user.ID
			}
		}

		// Generate feedback ID
		feedbackID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Failed to generate feedback ID", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save feedback")
			return
		}

		// Create feedback record
		feedback := &db.Feedback{
			ID:          feedbackID,
			UserID:      userID,
			SessionID:   req.SessionID,
			VideoID:     req.VideoID,
			PageURL:     req.PageURL,
			Text:        req.Text,
			Rating:      req.Rating,
			Type:        req.Type,
			BrowserInfo: req.BrowserInfo,
			CreatedAt:   time.Now(),
			Status:      db.FeedbackStatusNew,
		}

		if err := database.CreateFeedback(feedback); err != nil {
			logging.ErrorContext(r.Context(), "Failed to save feedback", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save feedback")
			return
		}

		logging.InfoContext(r.Context(), "Feedback submitted",
			"feedback_id", feedbackID,
			"type", req.Type,
			"has_rating", req.Rating != nil,
			"authenticated", userID != nil,
		)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status": "ok",
			"id":     feedbackID,
		})
	}))

	// Admin: List feedback (admin only)
	mux.HandleFunc("GET /api/admin/feedback", feedbackLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid session")
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/api/admin/feedback")
			httputil.RespondError(w, http.StatusForbidden, "Admin access required")
			return
		}

		// Parse query parameters
		status := r.URL.Query().Get("status")
		feedbackType := r.URL.Query().Get("type")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")
		after := r.URL.Query().Get("after")

		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		// List feedback
		feedbackList, total, err := database.ListFeedback(status, feedbackType, limit, offset, after)
		if err != nil {
			logging.ErrorContext(r.Context(), "Failed to list feedback", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to list feedback")
			return
		}

		logging.InfoContext(r.Context(), "Admin listed feedback",
			"user_id", user.ID,
			"status_filter", status,
			"type_filter", feedbackType,
			"total", total,
		)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"feedback": feedbackList,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		})
	}))

	// Admin: Get feedback by ID (admin only)
	mux.HandleFunc("GET /api/admin/feedback/{id}", feedbackLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid session")
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/api/admin/feedback/{id}")
			httputil.RespondError(w, http.StatusForbidden, "Admin access required")
			return
		}

		// Validate ID format
		feedbackID := r.PathValue("id")
		if err := validation.ValidateHexID(feedbackID); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid feedback ID format")
			return
		}

		// Get feedback
		feedback, err := database.GetFeedback(feedbackID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Failed to get feedback", "error", err, "feedback_id", feedbackID)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get feedback")
			return
		}

		if feedback == nil {
			httputil.RespondError(w, http.StatusNotFound, "Feedback not found")
			return
		}

		logging.InfoContext(r.Context(), "Admin viewed feedback",
			"user_id", user.ID,
			"feedback_id", feedbackID,
		)

		httputil.RespondJSON(w, http.StatusOK, feedback)
	}))

	// Admin: Update feedback status (admin only)
	mux.HandleFunc("PATCH /api/admin/feedback/{id}", feedbackLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid session")
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/api/admin/feedback/{id}")
			httputil.RespondError(w, http.StatusForbidden, "Admin access required")
			return
		}

		// Validate ID format
		feedbackID := r.PathValue("id")
		if err := validation.ValidateHexID(feedbackID); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid feedback ID format")
			return
		}

		// Parse request body
		var req struct {
			Status string `json:"status"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate status
		validStatuses := map[string]bool{
			db.FeedbackStatusNew:      true,
			db.FeedbackStatusRead:     true,
			db.FeedbackStatusResolved: true,
		}
		if !validStatuses[req.Status] {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid status. Must be: new, read, or resolved")
			return
		}

		// Update status
		if err := database.UpdateFeedbackStatus(feedbackID, req.Status); err != nil {
			logging.ErrorContext(r.Context(), "Failed to update feedback status", "error", err, "feedback_id", feedbackID)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to update feedback status")
			return
		}

		// Log security event for admin action
		security.AdminFeedbackUpdated(r.Context(), ratelimit.GetClientIP(r), user.ID, feedbackID, req.Status)

		logging.InfoContext(r.Context(), "Admin updated feedback status",
			"user_id", user.ID,
			"feedback_id", feedbackID,
			"new_status", req.Status,
		)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"id":      feedbackID,
			"updated": req.Status,
		})
	}))

	// CAPTCHA config endpoint (returns site key if CAPTCHA is enabled)
	mux.HandleFunc("GET /api/captcha/config", func(w http.ResponseWriter, r *http.Request) {
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled":  captchaVerifier.IsEnabled(),
			"site_key": os.Getenv("CAPTCHA_SITE_KEY"),
		})
	})
}
