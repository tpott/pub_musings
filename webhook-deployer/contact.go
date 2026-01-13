package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type ContactHandler struct {
	resendAPIKey string
	emailFrom    string
	emailTo      string
	rateLimiter  *RateLimiter
}

type ContactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
	Website string `json:"website"` // honeypot
}

type ContactResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ResendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

func NewContactHandler(resendAPIKey, emailFrom, emailTo string) *ContactHandler {
	return &ContactHandler{
		resendAPIKey: resendAPIKey,
		emailFrom:    emailFrom,
		emailTo:      emailTo,
		rateLimiter:  NewRateLimiter(),
	}
}

func (h *ContactHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get real IP from Cloudflare header
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip != "" {
			// Take first IP if multiple
			ip = strings.Split(ip, ",")[0]
			ip = strings.TrimSpace(ip)
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	// Check rate limit
	if !h.rateLimiter.Allow(ip) {
		log.Printf("Rate limit exceeded for IP: %s", ip)
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(ContactResponse{
			Success: false,
			Error:   "Too many requests. Please try again later.",
		})
		return
	}

	// Parse request
	var req ContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ContactResponse{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Check honeypot - if filled, silently succeed (fool the bot)
	if req.Website != "" {
		log.Printf("Honeypot triggered from IP: %s", ip)
		json.NewEncoder(w).Encode(ContactResponse{Success: true})
		return
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ContactResponse{
			Success: false,
			Error:   "Name, email, and message are required",
		})
		return
	}

	// Basic email validation
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ContactResponse{
			Success: false,
			Error:   "Invalid email address",
		})
		return
	}

	// Send email via Resend
	if err := h.sendEmail(req); err != nil {
		log.Printf("Failed to send email: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ContactResponse{
			Success: false,
			Error:   "Failed to send message. Please try again later.",
		})
		return
	}

	log.Printf("Contact form submitted: %s <%s>", req.Name, req.Email)
	json.NewEncoder(w).Encode(ContactResponse{Success: true})
}

func (h *ContactHandler) sendEmail(req ContactRequest) error {
	emailBody := fmt.Sprintf(`New contact form submission:

From: %s <%s>

Message:
%s
`, req.Name, req.Email, req.Message)

	resendReq := ResendRequest{
		From:    h.emailFrom,
		To:      h.emailTo,
		Subject: fmt.Sprintf("Contact form: %s", req.Name),
		Text:    emailBody,
	}

	body, err := json.Marshal(resendReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+h.resendAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error (status %d): %s", resp.StatusCode, respBody)
	}

	return nil
}
