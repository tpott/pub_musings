package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type WebhookHandler struct {
	secret     string
	config     *Config
	siteMutexs map[string]*sync.Mutex
	mu         sync.Mutex // protects siteMutexs
}

type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Pusher struct {
		Name string `json:"name"`
	} `json:"pusher"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

func NewWebhookHandler(secret string, config *Config) *WebhookHandler {
	return &WebhookHandler{
		secret:     secret,
		config:     config,
		siteMutexs: make(map[string]*sync.Mutex),
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

	// Extract branch name from ref
	branch := extractBranch(payload.Ref)
	if branch == "" {
		log.Printf("Could not extract branch from ref: %s", payload.Ref)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Invalid ref format"))
		return
	}

	// Collect all changed files from all commits
	changedFiles := extractChangedFiles(payload.Commits)

	// Find matching sites
	matchingSites := h.config.FindMatchingSites(branch, payload.Repository.FullName, changedFiles)

	if len(matchingSites) == 0 {
		log.Printf("No matching sites for branch %q, repo %q, files: %v", branch, payload.Repository.FullName, changedFiles)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No matching sites"))
		return
	}

	log.Printf("Deploy triggered by %s for %s (branch: %s)", payload.Pusher.Name, payload.Repository.FullName, branch)
	log.Printf("Matching sites: %v", siteNames(matchingSites))

	// Run deploys in background
	for _, site := range matchingSites {
		go h.deploySite(site)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deploy started for: " + strings.Join(siteNames(matchingSites), ", ")))
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

// getSiteMutex returns a mutex for the given site, creating one if needed
func (h *WebhookHandler) getSiteMutex(siteName string) *sync.Mutex {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.siteMutexs[siteName] == nil {
		h.siteMutexs[siteName] = &sync.Mutex{}
	}
	return h.siteMutexs[siteName]
}

func (h *WebhookHandler) deploySite(site SiteConfig) {
	// Get per-site mutex to prevent concurrent deploys of the same site
	mutex := h.getSiteMutex(site.Name)
	mutex.Lock()
	defer mutex.Unlock()

	log.Printf("[%s] Starting deploy...", site.Name)

	var err error
	var output []byte

	if site.DeployScript != "" {
		output, err = h.runDeployScript(site)
	} else {
		output, err = h.runInlineCommands(site)
	}

	if err != nil {
		log.Printf("[%s] Deploy failed: %v\nOutput: %s", site.Name, err, output)
		return
	}

	log.Printf("[%s] Deploy completed successfully:\n%s", site.Name, output)
}

func (h *WebhookHandler) runDeployScript(site SiteConfig) ([]byte, error) {
	cmd := exec.Command("bash", site.DeployScript)
	cmd.Dir = site.Path

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range site.Environment {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd.CombinedOutput()
}

func (h *WebhookHandler) runInlineCommands(site SiteConfig) ([]byte, error) {
	// Build a single bash script from all commands
	script := strings.Join(site.Commands, " && ")

	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = site.Path

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range site.Environment {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd.CombinedOutput()
}

// extractBranch extracts the branch name from a git ref
// e.g., "refs/heads/trunk" -> "trunk"
func extractBranch(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	return ""
}

// extractChangedFiles collects all added, modified, and removed files from commits
func extractChangedFiles(commits []struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
}) []string {
	seen := make(map[string]bool)
	var files []string

	for _, commit := range commits {
		for _, f := range commit.Added {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
		for _, f := range commit.Modified {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
		for _, f := range commit.Removed {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}

	return files
}

// siteNames extracts the names from a slice of SiteConfig
func siteNames(sites []SiteConfig) []string {
	names := make([]string, len(sites))
	for i, s := range sites {
		names[i] = s.Name
	}
	return names
}
