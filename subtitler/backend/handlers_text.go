package main

import (
	"math"
	"net/http"

	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/script"
)

func registerTextHandlers(mux *http.ServeMux) {
	// Script detection endpoint (rate limited)
	mux.HandleFunc("POST /api/text/detect-script", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Text string `json:"text"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		if len(req.Text) > 10240 { // 10KB limit
			httputil.RespondError(w, http.StatusBadRequest, "Text too long (max 10KB)")
			return
		}

		detectedScript := script.DetectScript(req.Text)
		detectedLang := script.DetectLanguageFromRomanized(req.Text)

		// Calculate confidence based on character count
		confidence := 0.0
		if detectedScript != script.ScriptUnknown {
			// Simple confidence: more characters = higher confidence
			confidence = math.Min(float64(len(req.Text))/100.0, 1.0)
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"detected_script":   string(detectedScript),
			"detected_language": detectedLang,
			"confidence":        confidence,
		})
	}))

	// Script conversion endpoint (rate limited)
	mux.HandleFunc("POST /api/text/convert", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Text         string `json:"text"`
			SourceScript string `json:"source_script,omitempty"` // Optional, auto-detected if omitted
			TargetScript string `json:"target_script"`
			Language     string `json:"language"` // Required for romanized input
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate text length (10KB limit)
		if len(req.Text) > 10240 {
			httputil.RespondError(w, http.StatusBadRequest, "Text too long (max 10KB)")
			return
		}

		// Validate language
		if !script.IsLanguageSupported(req.Language) {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":               "Unsupported language",
				"supported_languages": script.SupportedLanguages(),
			})
			return
		}

		// Validate target script
		targetScript := script.Script(req.TargetScript)
		if !script.IsScriptSupported(targetScript) {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":             "Unsupported target script",
				"supported_scripts": script.SupportedScripts(),
			})
			return
		}

		// Auto-detect source script if not provided
		sourceScript := script.Script(req.SourceScript)
		if sourceScript == "" {
			sourceScript = script.DetectScript(req.Text)
		}

		// Perform conversion
		converter := script.NewConverter()
		converted, err := converter.Convert(req.Text, req.Language, targetScript)
		if err != nil {
			httputil.RespondErrorf(w, http.StatusInternalServerError, "Conversion failed: %v", err)
			return
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"original":      req.Text,
			"converted":     converted,
			"source_script": string(sourceScript),
			"target_script": string(targetScript),
			"language":      req.Language,
		})
	}))
}
