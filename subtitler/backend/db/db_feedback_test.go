package db

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestListFeedbackLimitValidation(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-feedback-limit-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create some test feedback entries
	for i := 0; i < 5; i++ {
		feedback := &Feedback{
			ID:        fmt.Sprintf("feedback-limit-test-%d", i),
			PageURL:   "http://test.com",
			Text:      fmt.Sprintf("Feedback %d", i),
			Type:      "general",
			Status:    FeedbackStatusNew,
			CreatedAt: time.Now(),
		}
		if err := db.CreateFeedback(feedback); err != nil {
			t.Fatalf("Failed to create feedback: %v", err)
		}
	}

	t.Run("default limit when zero provided", func(t *testing.T) {
		feedbackList, total, err := db.ListFeedback("", "", 0, 0, "")
		if err != nil {
			t.Fatalf("ListFeedback failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected total 5, got %d", total)
		}
		if len(feedbackList) != 5 {
			t.Errorf("Expected 5 feedback items, got %d", len(feedbackList))
		}
	})

	t.Run("default limit when negative provided", func(t *testing.T) {
		feedbackList, total, err := db.ListFeedback("", "", -10, 0, "")
		if err != nil {
			t.Fatalf("ListFeedback failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected total 5, got %d", total)
		}
		if len(feedbackList) != 5 {
			t.Errorf("Expected 5 feedback items, got %d", len(feedbackList))
		}
	})

	t.Run("limit capped at MaxFeedbackLimit", func(t *testing.T) {
		// Request more than max, should be capped
		feedbackList, _, err := db.ListFeedback("", "", 200, 0, "")
		if err != nil {
			t.Fatalf("ListFeedback failed: %v", err)
		}
		// We only have 5 items, so we expect 5 back (but limit was capped to 100)
		if len(feedbackList) != 5 {
			t.Errorf("Expected 5 feedback items, got %d", len(feedbackList))
		}
	})

	t.Run("negative offset treated as zero", func(t *testing.T) {
		feedbackList, _, err := db.ListFeedback("", "", 10, -5, "")
		if err != nil {
			t.Fatalf("ListFeedback failed: %v", err)
		}
		if len(feedbackList) != 5 {
			t.Errorf("Expected 5 feedback items, got %d", len(feedbackList))
		}
	})

	t.Run("offset works correctly", func(t *testing.T) {
		feedbackList, total, err := db.ListFeedback("", "", 2, 2, "")
		if err != nil {
			t.Fatalf("ListFeedback failed: %v", err)
		}
		if total != 5 {
			t.Errorf("Expected total 5, got %d", total)
		}
		if len(feedbackList) != 2 {
			t.Errorf("Expected 2 feedback items, got %d", len(feedbackList))
		}
	})

	t.Run("after parameter filters by timestamp", func(t *testing.T) {
		// The after parameter is passed as RFC3339 from the API.
		// go-sqlite3 stores time.Time differently from JSON marshaling,
		// so ListFeedback must parse the string into time.Time before querying.
		// Use a future timestamp so only the item we create "after" it is returned.
		futureTime := time.Now().Add(1 * time.Hour)
		newFeedback := &Feedback{
			ID:        "feedback-after-test",
			PageURL:   "http://test.com",
			Text:      "New feedback after cutoff",
			Type:      "general",
			Status:    FeedbackStatusNew,
			CreatedAt: futureTime.Add(1 * time.Minute),
		}
		if err := db.CreateFeedback(newFeedback); err != nil {
			t.Fatalf("Failed to create feedback: %v", err)
		}
		// Cutoff is the futureTime; only the item 1 minute later should match
		cutoff := futureTime.UTC().Format(time.RFC3339)
		feedbackList, total, err := db.ListFeedback("", "", 50, 0, cutoff)
		if err != nil {
			t.Fatalf("ListFeedback with after failed: %v", err)
		}
		if total != 1 {
			t.Errorf("Expected 1 feedback item after cutoff, got %d", total)
		}
		if len(feedbackList) != 1 {
			t.Errorf("Expected 1 feedback item, got %d", len(feedbackList))
		}
		if len(feedbackList) > 0 && feedbackList[0].ID != "feedback-after-test" {
			t.Errorf("Expected feedback-after-test, got %s", feedbackList[0].ID)
		}
	})

	t.Run("after parameter rejects invalid timestamp", func(t *testing.T) {
		_, _, err := db.ListFeedback("", "", 50, 0, "not-a-timestamp")
		if err == nil {
			t.Error("Expected error for invalid after timestamp, got nil")
		}
	})

	t.Run("MaxFeedbackLimit constant is reasonable", func(t *testing.T) {
		if MaxFeedbackLimit < 50 {
			t.Errorf("MaxFeedbackLimit too low: %d", MaxFeedbackLimit)
		}
		if MaxFeedbackLimit > 200 {
			t.Errorf("MaxFeedbackLimit too high: %d", MaxFeedbackLimit)
		}
	})
}
