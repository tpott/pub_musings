package cleanup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/trevor/subtitler/internal/db"
)

// Service handles cleanup of old job files
type Service struct {
	db            *db.DB
	maxAgeDays    int
	intervalMins  int
	stopCh        chan struct{}
	stoppedCh     chan struct{}
	mu            sync.Mutex
	running       bool
	cleanupCount  int64
	bytesFreed    int64
	lastCleanup   time.Time
}

// NewService creates a new cleanup service
func NewService(database *db.DB, maxAgeDays, intervalMins int) *Service {
	return &Service{
		db:           database,
		maxAgeDays:   maxAgeDays,
		intervalMins: intervalMins,
		stopCh:       make(chan struct{}),
		stoppedCh:    make(chan struct{}),
	}
}

// Start begins the cleanup job loop
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop gracefully stops the cleanup service
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	close(s.stopCh)
	<-s.stoppedCh
}

// Stats returns cleanup statistics
func (s *Service) Stats() (cleanupCount int64, bytesFreed int64, lastCleanup time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cleanupCount, s.bytesFreed, s.lastCleanup
}

func (s *Service) run() {
	defer close(s.stoppedCh)

	// Run initial cleanup
	s.runCleanup()

	ticker := time.NewTicker(time.Duration(s.intervalMins) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Println("Cleanup service stopping...")
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.runCleanup()
		}
	}
}

func (s *Service) runCleanup() {
	log.Printf("Starting cleanup of jobs older than %d days...", s.maxAgeDays)

	jobs, err := s.db.GetOldJobs(s.maxAgeDays)
	if err != nil {
		log.Printf("Error getting old jobs: %v", err)
		return
	}

	if len(jobs) == 0 {
		log.Println("No old jobs to clean up")
		s.mu.Lock()
		s.lastCleanup = time.Now()
		s.mu.Unlock()
		return
	}

	log.Printf("Found %d old jobs to clean up", len(jobs))

	var cleaned int64
	var freed int64

	for _, job := range jobs {
		bytesDeleted, err := s.cleanupJob(job)
		if err != nil {
			log.Printf("Error cleaning up job %d: %v", job.ID, err)
			continue
		}
		cleaned++
		freed += bytesDeleted
	}

	s.mu.Lock()
	s.cleanupCount += cleaned
	s.bytesFreed += freed
	s.lastCleanup = time.Now()
	s.mu.Unlock()

	log.Printf("Cleanup complete: %d jobs cleaned, %s freed", cleaned, formatBytes(freed))
}

func (s *Service) cleanupJob(job *db.Job) (int64, error) {
	var bytesDeleted int64

	// Delete uploaded file
	if job.FilePath != "" {
		bytes, err := deleteFileIfExists(job.FilePath)
		if err != nil {
			log.Printf("Warning: failed to delete upload file %s: %v", job.FilePath, err)
		} else {
			bytesDeleted += bytes
		}

		// Try to remove parent directories if empty (job dir, then user dir)
		removeEmptyParentDirs(job.FilePath, 2)
	}

	// Delete transcript/result file
	if job.TranscriptPath != nil && *job.TranscriptPath != "" {
		bytes, err := deleteFileIfExists(*job.TranscriptPath)
		if err != nil {
			log.Printf("Warning: failed to delete result file %s: %v", *job.TranscriptPath, err)
		} else {
			bytesDeleted += bytes
		}

		// Try to remove parent directories if empty
		removeEmptyParentDirs(*job.TranscriptPath, 2)
	}

	// Delete database record
	if err := s.db.DeleteJob(job.ID); err != nil {
		return bytesDeleted, err
	}

	log.Printf("Cleaned up job %d (%s): freed %s", job.ID, job.OriginalFilename, formatBytes(bytesDeleted))
	return bytesDeleted, nil
}

func deleteFileIfExists(path string) (int64, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	size := info.Size()
	if err := os.Remove(path); err != nil {
		return 0, err
	}
	return size, nil
}

func removeEmptyParentDirs(filePath string, levels int) {
	dir := filepath.Dir(filePath)
	for i := 0; i < levels; i++ {
		if dir == "" || dir == "." || dir == "/" {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}

		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
