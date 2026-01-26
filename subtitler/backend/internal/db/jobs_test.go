package db

import (
	"path/filepath"
	"testing"
)

func setupJobsTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Run migrations
	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db
}

func TestCreateJob(t *testing.T) {
	// Create temporary database
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	// Create a test user first
	user, err := tmpDB.CreateUser("test@example.com", "hashedpassword123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a job
	job := &Job{
		UserID:           user.ID,
		OriginalFilename: "test.mp3",
		FilePath:         "/path/to/test.mp3",
		FileSize:         1024,
		OutputFormat:     "srt",
	}
	err = tmpDB.CreateJob(job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	if job.ID == 0 {
		t.Error("Expected job ID to be non-zero")
	}
	if job.UserID != user.ID {
		t.Errorf("Expected user_id %d, got %d", user.ID, job.UserID)
	}
	if job.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", job.Status)
	}
	if job.OriginalFilename != "test.mp3" {
		t.Errorf("Expected filename 'test.mp3', got %s", job.OriginalFilename)
	}
	if job.OutputFormat != "srt" {
		t.Errorf("Expected output_format 'srt', got %s", job.OutputFormat)
	}
}

func TestGetJobByID(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	// Create test user and job
	user, _ := tmpDB.CreateUser("test@example.com", "hash")
	job := &Job{
		UserID:           user.ID,
		OriginalFilename: "test.mp3",
		FilePath:         "/path/to/test.mp3",
		FileSize:         1024,
		OutputFormat:     "srt",
	}
	tmpDB.CreateJob(job)

	// Retrieve the job
	retrieved, err := tmpDB.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected job to be found")
	}
	if retrieved.ID != job.ID {
		t.Errorf("Expected ID %d, got %d", job.ID, retrieved.ID)
	}
	if retrieved.OriginalFilename != "test.mp3" {
		t.Errorf("Expected filename 'test.mp3', got %s", retrieved.OriginalFilename)
	}

	// Test non-existent job
	notFound, err := tmpDB.GetJobByID(99999)
	if err != nil {
		t.Fatalf("Expected no error for non-existent job, got: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for non-existent job")
	}
}

func TestGetJobsByUserID(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	// Create two users
	user1, _ := tmpDB.CreateUser("user1@example.com", "hash1")
	user2, _ := tmpDB.CreateUser("user2@example.com", "hash2")

	// Create jobs for user1
	tmpDB.CreateJob(&Job{UserID: user1.ID, OriginalFilename: "file1.mp3", FilePath: "/path/to/file1.mp3", FileSize: 1024, OutputFormat: "srt"})
	tmpDB.CreateJob(&Job{UserID: user1.ID, OriginalFilename: "file2.mp3", FilePath: "/path/to/file2.mp3", FileSize: 2048, OutputFormat: "vtt"})

	// Create job for user2
	tmpDB.CreateJob(&Job{UserID: user2.ID, OriginalFilename: "file3.mp3", FilePath: "/path/to/file3.mp3", FileSize: 512, OutputFormat: "srt"})

	// Get jobs for user1
	jobs, err := tmpDB.GetJobsByUserID(user1.ID)
	if err != nil {
		t.Fatalf("Failed to get jobs: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs for user1, got %d", len(jobs))
	}

	// Verify both jobs are present
	if len(jobs) == 2 {
		filenames := []string{jobs[0].OriginalFilename, jobs[1].OriginalFilename}
		hasFile1 := false
		hasFile2 := false
		for _, name := range filenames {
			if name == "file1.mp3" {
				hasFile1 = true
			}
			if name == "file2.mp3" {
				hasFile2 = true
			}
		}
		if !hasFile1 || !hasFile2 {
			t.Errorf("Expected jobs to contain file1.mp3 and file2.mp3, got %v", filenames)
		}
	}

	// Get jobs for user2
	jobs2, err := tmpDB.GetJobsByUserID(user2.ID)
	if err != nil {
		t.Fatalf("Failed to get jobs for user2: %v", err)
	}

	if len(jobs2) != 1 {
		t.Errorf("Expected 1 job for user2, got %d", len(jobs2))
	}

	// Test user with no jobs
	user3, _ := tmpDB.CreateUser("user3@example.com", "hash3")
	jobs3, err := tmpDB.GetJobsByUserID(user3.ID)
	if err != nil {
		t.Fatalf("Failed to get jobs for user3: %v", err)
	}
	if len(jobs3) != 0 {
		t.Errorf("Expected 0 jobs for user3, got %d", len(jobs3))
	}
}

func TestUpdateJobStatus(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")
	job := &Job{UserID: user.ID, OriginalFilename: "test.mp3", FilePath: "/path/to/test.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(job)

	// Update status
	err := tmpDB.UpdateJobStatus(job.ID, "processing")
	if err != nil {
		t.Fatalf("Failed to update job status: %v", err)
	}

	// Verify status changed
	updated, _ := tmpDB.GetJobByID(job.ID)
	if updated.Status != "processing" {
		t.Errorf("Expected status 'processing', got %s", updated.Status)
	}
}

func TestUpdateJobCompleted(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")
	job := &Job{UserID: user.ID, OriginalFilename: "test.mp3", FilePath: "/path/to/test.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(job)

	// Mark as completed
	transcriptPath := "/path/to/transcript.srt"
	err := tmpDB.UpdateJobCompleted(job.ID, transcriptPath)
	if err != nil {
		t.Fatalf("Failed to mark job as completed: %v", err)
	}

	// Verify changes
	updated, _ := tmpDB.GetJobByID(job.ID)
	if updated.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", updated.Status)
	}
	if updated.TranscriptPath == nil || *updated.TranscriptPath != transcriptPath {
		t.Errorf("Expected transcript_path '%s', got %v", transcriptPath, updated.TranscriptPath)
	}
	if updated.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

func TestUpdateJobFailed(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")
	job := &Job{UserID: user.ID, OriginalFilename: "test.mp3", FilePath: "/path/to/test.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(job)

	// Mark as failed
	errorMsg := "Transcription failed: invalid format"
	err := tmpDB.UpdateJobFailed(job.ID, errorMsg)
	if err != nil {
		t.Fatalf("Failed to mark job as failed: %v", err)
	}

	// Verify changes
	updated, _ := tmpDB.GetJobByID(job.ID)
	if updated.Status != "failed" {
		t.Errorf("Expected status 'failed', got %s", updated.Status)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != errorMsg {
		t.Errorf("Expected error_message '%s', got %v", errorMsg, updated.ErrorMessage)
	}
	if updated.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

func TestGetOldJobs(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")

	// Create a job with old created_at (manually update using raw SQL)
	job := &Job{UserID: user.ID, OriginalFilename: "old.mp3", FilePath: "/path/to/old.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(job)

	// Set created_at to 60 days ago and mark as completed
	_, err := tmpDB.Exec(`
		UPDATE jobs
		SET created_at = datetime('now', '-60 days'),
		    status = 'completed',
		    completed_at = datetime('now', '-60 days')
		WHERE id = ?
	`, job.ID)
	if err != nil {
		t.Fatalf("Failed to update job created_at: %v", err)
	}

	// Create a recent job (should not be returned)
	recentJob := &Job{UserID: user.ID, OriginalFilename: "recent.mp3", FilePath: "/path/to/recent.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(recentJob)
	tmpDB.UpdateJobCompleted(recentJob.ID, "/path/to/transcript.srt")

	// Create a pending job that's old (should not be returned - only completed/failed)
	pendingJob := &Job{UserID: user.ID, OriginalFilename: "pending.mp3", FilePath: "/path/to/pending.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(pendingJob)
	_, err = tmpDB.Exec(`
		UPDATE jobs
		SET created_at = datetime('now', '-60 days')
		WHERE id = ?
	`, pendingJob.ID)
	if err != nil {
		t.Fatalf("Failed to update pending job created_at: %v", err)
	}

	// Get old jobs (>30 days)
	oldJobs, err := tmpDB.GetOldJobs(30)
	if err != nil {
		t.Fatalf("Failed to get old jobs: %v", err)
	}

	if len(oldJobs) != 1 {
		t.Errorf("Expected 1 old job, got %d", len(oldJobs))
	}

	if len(oldJobs) == 1 && oldJobs[0].OriginalFilename != "old.mp3" {
		t.Errorf("Expected old job 'old.mp3', got %s", oldJobs[0].OriginalFilename)
	}
}

func TestJobLanguageField(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")

	// Test 1: Job without language (auto-detect)
	jobAutoDetect := &Job{
		UserID:           user.ID,
		OriginalFilename: "auto.mp3",
		FilePath:         "/path/to/auto.mp3",
		FileSize:         1024,
		OutputFormat:     "srt",
		Language:         nil, // Auto-detect
	}
	err := tmpDB.CreateJob(jobAutoDetect)
	if err != nil {
		t.Fatalf("Failed to create job without language: %v", err)
	}

	retrieved, err := tmpDB.GetJobByID(jobAutoDetect.ID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if retrieved.Language != nil {
		t.Errorf("Expected nil language for auto-detect, got %v", retrieved.Language)
	}

	// Test 2: Job with language specified
	spanish := "es"
	jobWithLang := &Job{
		UserID:           user.ID,
		OriginalFilename: "spanish.mp3",
		FilePath:         "/path/to/spanish.mp3",
		FileSize:         2048,
		OutputFormat:     "vtt",
		Language:         &spanish,
	}
	err = tmpDB.CreateJob(jobWithLang)
	if err != nil {
		t.Fatalf("Failed to create job with language: %v", err)
	}

	retrieved2, err := tmpDB.GetJobByID(jobWithLang.ID)
	if err != nil {
		t.Fatalf("Failed to get job with language: %v", err)
	}
	if retrieved2.Language == nil {
		t.Fatal("Expected language to be set")
	}
	if *retrieved2.Language != "es" {
		t.Errorf("Expected language 'es', got %s", *retrieved2.Language)
	}

	// Test 3: GetJobsByUserID includes language
	jobs, err := tmpDB.GetJobsByUserID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get jobs by user: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(jobs))
	}

	// Find the job with Spanish language
	var foundSpanishJob *Job
	for _, j := range jobs {
		if j.Language != nil && *j.Language == "es" {
			foundSpanishJob = j
			break
		}
	}
	if foundSpanishJob == nil {
		t.Error("Expected to find job with Spanish language in user's jobs")
	}
}

func TestDeleteJob(t *testing.T) {
	tmpDB := setupJobsTestDB(t)
	defer tmpDB.Close()

	user, _ := tmpDB.CreateUser("test@example.com", "hash")
	job := &Job{UserID: user.ID, OriginalFilename: "test.mp3", FilePath: "/path/to/test.mp3", FileSize: 1024, OutputFormat: "srt"}
	tmpDB.CreateJob(job)

	// Verify job exists
	existing, err := tmpDB.GetJobByID(job.ID)
	if err != nil || existing == nil {
		t.Fatal("Job should exist before deletion")
	}

	// Delete the job
	err = tmpDB.DeleteJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	// Verify job is deleted
	deleted, err := tmpDB.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("Unexpected error checking deleted job: %v", err)
	}
	if deleted != nil {
		t.Error("Job should not exist after deletion")
	}
}
