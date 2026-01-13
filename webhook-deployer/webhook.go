package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

type WebhookHandler struct {
	secret   string
	sitePath string
}

type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Pusher struct {
		Name string `json:"name"`
	} `json:"pusher"`
}

func NewWebhookHandler(secret, sitePath string) *WebhookHandler {
	return &WebhookHandler{
		secret:   secret,
		sitePath: sitePath,
	}
}

func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request", http.StatusBadRequest)
		return
	}

	// Validate signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.validateSignature(body, signature) {
		log.Printf("Invalid signature from %s", r.RemoteAddr)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Check event type
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		log.Printf("Ignoring event type: %s", event)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event ignored"))
		return
	}

	// Parse payload
	var payload GitHubPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing payload: %v", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Only deploy on push to trunk
	if payload.Ref != "refs/heads/trunk" {
		log.Printf("Ignoring push to %s", payload.Ref)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Not trunk branch, ignored"))
		return
	}

	log.Printf("Deploy triggered by %s for %s", payload.Pusher.Name, payload.Repository.FullName)

	// Run deploy in background
	go h.deploy()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deploy started"))
}

func (h *WebhookHandler) validateSignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Signature format: sha256=<hex>
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

func (h *WebhookHandler) deploy() {
	log.Println("Starting deploy...")

	// Run deploy script
	cmd := exec.Command("bash", "-c", `
		cd `+h.sitePath+` && \
		git pull origin trunk && \
		. ~/.nvm/nvm.sh && \
		nvm use && \
		npm ci && \
		npm run build
	`)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Deploy failed: %v\nOutput: %s", err, output)
		return
	}

	log.Printf("Deploy completed successfully:\n%s", output)
}
