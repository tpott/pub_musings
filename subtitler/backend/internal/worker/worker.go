package worker

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/trevor/subtitler/internal/db"
	"github.com/trevor/subtitler/internal/transcribe"
)

// JobQueue is a buffered channel for queuing job IDs
type JobQueue chan int64

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	numWorkers        int
	queue             JobQueue
	db                *db.DB
	transcribeService *transcribe.Service
	wg                sync.WaitGroup
	stopChan          chan struct{}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int, queueSize int, database *db.DB, transcribeService *transcribe.Service) *WorkerPool {
	return &WorkerPool{
		numWorkers:        numWorkers,
		queue:             make(JobQueue, queueSize),
		db:                database,
		transcribeService: transcribeService,
		stopChan:          make(chan struct{}),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers", wp.numWorkers)
	for i := 1; i <= wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	close(wp.stopChan)
	close(wp.queue)
	wp.wg.Wait()
	log.Println("Worker pool stopped")
}

// Enqueue adds a job ID to the queue
func (wp *WorkerPool) Enqueue(jobID int64) error {
	select {
	case wp.queue <- jobID:
		log.Printf("Job %d enqueued", jobID)
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// worker is the main worker goroutine
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case <-wp.stopChan:
			log.Printf("Worker %d stopping", id)
			return
		case jobID, ok := <-wp.queue:
			if !ok {
				log.Printf("Worker %d: queue closed", id)
				return
			}
			wp.processJob(id, jobID)
		}
	}
}

// processJob processes a single job
func (wp *WorkerPool) processJob(workerID int, jobID int64) {
	log.Printf("Worker %d: Processing job %d", workerID, jobID)

	// Fetch job from database
	job, err := wp.db.GetJobByID(jobID)
	if err != nil {
		log.Printf("Worker %d: Failed to fetch job %d: %v", workerID, jobID, err)
		return
	}

	// Update status to processing
	err = wp.db.UpdateJobStatus(jobID, "processing")
	if err != nil {
		log.Printf("Worker %d: Failed to update job %d to processing: %v", workerID, jobID, err)
		return
	}

	// Transcribe the file
	transcript, err := wp.transcribeService.TranscribeFile(job.FilePath, transcribe.TranscribeOptions{
		Format: transcribe.FormatSRT, // Default to SRT for now
	})
	if err != nil {
		log.Printf("Worker %d: Transcription failed for job %d: %v", workerID, jobID, err)
		wp.db.UpdateJobFailed(jobID, fmt.Sprintf("Transcription failed: %v", err))
		return
	}

	// Build result path
	transcriptPath, err := wp.buildResultPath(job)
	if err != nil {
		log.Printf("Worker %d: Failed to build result path for job %d: %v", workerID, jobID, err)
		wp.db.UpdateJobFailed(jobID, fmt.Sprintf("Failed to build result path: %v", err))
		return
	}

	// Create result directory if it doesn't exist
	resultDir := filepath.Dir(transcriptPath)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		log.Printf("Worker %d: Failed to create result directory for job %d: %v", workerID, jobID, err)
		wp.db.UpdateJobFailed(jobID, fmt.Sprintf("Failed to create result directory: %v", err))
		return
	}

	// Save transcript to file
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0644); err != nil {
		log.Printf("Worker %d: Failed to save transcript for job %d: %v", workerID, jobID, err)
		wp.db.UpdateJobFailed(jobID, fmt.Sprintf("Failed to save transcript: %v", err))
		return
	}

	// Update job as completed
	err = wp.db.UpdateJobCompleted(jobID, transcriptPath)
	if err != nil {
		log.Printf("Worker %d: Failed to mark job %d as completed: %v", workerID, jobID, err)
		return
	}

	log.Printf("Worker %d: Job %d completed successfully", workerID, jobID)
}

// buildResultPath generates the file path for the transcription result
func (wp *WorkerPool) buildResultPath(job *db.Job) (string, error) {
	// Format: data/files/results/{user_id}/{job_id}/{original_filename}.{format}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Get base filename without extension
	baseFilename := filepath.Base(job.OriginalFilename)
	ext := filepath.Ext(baseFilename)
	nameWithoutExt := baseFilename[:len(baseFilename)-len(ext)]

	// Build result filename
	resultFilename := fmt.Sprintf("%s.%s", nameWithoutExt, job.OutputFormat)

	// Build full path
	transcriptPath := filepath.Join(
		dataDir,
		"files",
		"results",
		fmt.Sprintf("%d", job.UserID),
		fmt.Sprintf("%d", job.ID),
		resultFilename,
	)

	return transcriptPath, nil
}
