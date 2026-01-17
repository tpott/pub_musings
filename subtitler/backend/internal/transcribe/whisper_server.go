package transcribe

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/trevor/subtitler/internal/config"
)

// WhisperServer manages the whisper-server subprocess
type WhisperServer struct {
	cfg     *config.Config
	cmd     *exec.Cmd
	started bool
	mu      sync.Mutex
}

// NewWhisperServer creates a new WhisperServer instance
func NewWhisperServer(cfg *config.Config) *WhisperServer {
	return &WhisperServer{
		cfg:     cfg,
		started: false,
	}
}

// Start starts the whisper-server subprocess
func (ws *WhisperServer) Start() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.started {
		return fmt.Errorf("whisper-server already started")
	}

	// Build command with arguments
	ws.cmd = exec.Command(
		ws.cfg.WhisperServerPath,
		"-m", ws.cfg.WhisperModelPath,
		"-t", strconv.Itoa(ws.cfg.WhisperThreads),
		"--port", strconv.Itoa(ws.cfg.WhisperServerPort),
		"--convert", // Enable ffmpeg conversion for audio format support
	)

	// Start the process
	if err := ws.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start whisper-server: %w", err)
	}

	ws.started = true
	log.Printf("Started whisper-server on port %d (PID: %d)", ws.cfg.WhisperServerPort, ws.cmd.Process.Pid)

	// Wait for server to be ready
	if err := ws.waitForReady(); err != nil {
		ws.Stop()
		return fmt.Errorf("whisper-server failed to become ready: %w", err)
	}

	log.Println("whisper-server is ready")
	return nil
}

// Stop stops the whisper-server subprocess
func (ws *WhisperServer) Stop() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.started || ws.cmd == nil || ws.cmd.Process == nil {
		return nil
	}

	log.Println("Stopping whisper-server...")
	if err := ws.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to stop whisper-server: %w", err)
	}

	ws.cmd.Wait()
	ws.started = false
	log.Println("whisper-server stopped")
	return nil
}

// IsRunning returns true if the whisper-server is running
func (ws *WhisperServer) IsRunning() bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.started
}

// GetEndpoint returns the HTTP endpoint for the whisper-server
func (ws *WhisperServer) GetEndpoint() string {
	return fmt.Sprintf("http://localhost:%d", ws.cfg.WhisperServerPort)
}

// waitForReady waits for whisper-server to respond to health checks
func (ws *WhisperServer) waitForReady() error {
	endpoint := ws.GetEndpoint()
	maxAttempts := 30
	delay := 1 * time.Second

	for i := 0; i < maxAttempts; i++ {
		resp, err := http.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMethodNotAllowed {
				// Server is responding (even if it doesn't accept GET on /)
				return nil
			}
		}

		time.Sleep(delay)
	}

	return fmt.Errorf("whisper-server did not become ready after %d attempts", maxAttempts)
}
