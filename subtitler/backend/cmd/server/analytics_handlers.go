package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/trevor/subtitler/internal/analytics"
	"github.com/trevor/subtitler/internal/auth"
)

// TrackEventRequest represents the request body for tracking events
type TrackEventRequest struct {
	VisitorID   string                 `json:"visitor_id"`
	EventName   string                 `json:"event_name"`
	Properties  map[string]interface{} `json:"properties"`
	UTMSource   *string                `json:"utm_source"`
	UTMMedium   *string                `json:"utm_medium"`
	UTMCampaign *string                `json:"utm_campaign"`
}

// handleTrackEvent handles POST /api/analytics/events
func handleTrackEvent(analyticsService *analytics.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req TrackEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.VisitorID == "" {
			http.Error(w, "visitor_id is required", http.StatusBadRequest)
			return
		}
		if req.EventName == "" {
			http.Error(w, "event_name is required", http.StatusBadRequest)
			return
		}

		// Extract user ID from JWT if authenticated (optional)
		var userID *int64
		if claims, ok := auth.GetClaims(r); ok {
			userID = &claims.UserID
		}

		// Track the event
		err := analyticsService.TrackEvent(r.Context(), req.VisitorID, userID, req.EventName, req.Properties, req.UTMSource, req.UTMMedium, req.UTMCampaign)
		if err != nil {
			http.Error(w, "Failed to track event", http.StatusInternalServerError)
			return
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

// handleGetFunnel handles GET /api/analytics/funnel
func handleGetFunnel(analyticsService *analytics.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse date range from query params
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		// Default to last 30 days if not provided
		end := time.Now()
		start := end.AddDate(0, 0, -30)

		if startStr != "" {
			parsedStart, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				http.Error(w, "Invalid start date format (use YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			start = parsedStart
		}

		if endStr != "" {
			parsedEnd, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				http.Error(w, "Invalid end date format (use YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			end = parsedEnd
		}

		// Get funnel data
		report, err := analyticsService.GetFunnel(r.Context(), start, end)
		if err != nil {
			http.Error(w, "Failed to get funnel data", http.StatusInternalServerError)
			return
		}

		// Return report
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}

// handleGetExperiment handles GET /api/analytics/experiments/{experiment_id}
func handleGetExperiment(analyticsService *analytics.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract experiment_id from URL path
		// URL format: /api/analytics/experiments/{experiment_id}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/analytics/experiments/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "experiment_id is required", http.StatusBadRequest)
			return
		}
		experimentID := parts[0]

		// Parse date range from query params
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		// Default to last 30 days if not provided
		end := time.Now()
		start := end.AddDate(0, 0, -30)

		if startStr != "" {
			parsedStart, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				http.Error(w, "Invalid start date format (use YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			start = parsedStart
		}

		if endStr != "" {
			parsedEnd, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				http.Error(w, "Invalid end date format (use YYYY-MM-DD)", http.StatusBadRequest)
				return
			}
			end = parsedEnd
		}

		// Get experiment results
		report, err := analyticsService.GetExperimentResults(r.Context(), experimentID, start, end)
		if err != nil {
			http.Error(w, "Failed to get experiment results", http.StatusInternalServerError)
			return
		}

		// Return report
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}
