package db

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWithTransaction(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
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

	// Create a user
	user := &User{
		ID:           "txtest123",
		Email:        "txtest@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test successful transaction
	err = db.WithTransaction(func(tx *Tx) error {
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "newemail@example.com", user.ID)
		return err
	})
	if err != nil {
		t.Fatalf("Transaction should have succeeded: %v", err)
	}

	// Verify the change was committed
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.Email != "newemail@example.com" {
		t.Errorf("Expected email 'newemail@example.com', got '%s'", updatedUser.Email)
	}
}

func TestWithTransactionRollback(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
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

	// Create a user
	user := &User{
		ID:           "txrollback123",
		Email:        "txrollback@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test transaction that fails and rolls back
	testErr := fmt.Errorf("intentional test error")
	err = db.WithTransaction(func(tx *Tx) error {
		// Make a change
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "shouldrollback@example.com", user.ID)
		if err != nil {
			return err
		}
		// Then fail
		return testErr
	})
	if err != testErr {
		t.Errorf("Expected test error, got: %v", err)
	}

	// Verify the change was rolled back
	unchangedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if unchangedUser.Email != "txrollback@example.com" {
		t.Errorf("Expected original email 'txrollback@example.com', got '%s' (rollback failed)", unchangedUser.Email)
	}
}

func TestWithTransactionUsesContext(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
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

	// Verify that WithTransaction uses a timeout context by setting a very short timeout
	// and performing a simple operation - it should still work since SQLite is fast
	db.SetQueryTimeout(100 * time.Millisecond)

	user := &User{
		ID:           "txtimeout123",
		Email:        "txtimeout@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Transaction should succeed with short timeout for fast operations
	err = db.WithTransaction(func(tx *Tx) error {
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "updated@example.com", user.ID)
		return err
	})
	if err != nil {
		t.Errorf("Transaction with short timeout should succeed for fast operations: %v", err)
	}

	// Verify the change was committed
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.Email != "updated@example.com" {
		t.Errorf("Expected email 'updated@example.com', got '%s'", updatedUser.Email)
	}
}

func TestWithTransactionPanicRecovery(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-tx-panic-*.db")
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

	// Create a user
	user := &User{
		ID:           "panic-test-user",
		Email:        "panictest@example.com",
		PasswordHash: "hash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user exists
	originalUser, err := db.GetUserByID(user.ID)
	if err != nil || originalUser == nil {
		t.Fatalf("User should exist before transaction")
	}

	// Execute transaction that panics after making a change
	panicMsg := "test panic in transaction"
	err = db.WithTransaction(func(tx *Tx) error {
		// Make a change
		_, err := tx.tx.Exec(`UPDATE users SET email = 'panic-changed@example.com' WHERE id = ?`, user.ID)
		if err != nil {
			return err
		}
		// Panic after the change
		panic(panicMsg)
	})

	// Verify panic was converted to PanicError
	if err == nil {
		t.Fatal("Expected error from panicking transaction")
	}
	panicErr, ok := err.(*PanicError)
	if !ok {
		t.Fatalf("Expected PanicError, got %T: %v", err, err)
	}
	if panicErr.Value != panicMsg {
		t.Errorf("Expected panic value '%s', got '%v'", panicMsg, panicErr.Value)
	}
	if panicErr.Stack == "" {
		t.Error("Expected stack trace in PanicError, got empty string")
	}

	// Verify the change was rolled back
	unchangedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user after panic: %v", err)
	}
	if unchangedUser.Email != "panictest@example.com" {
		t.Errorf("Expected email to be rolled back to 'panictest@example.com', got '%s' (rollback on panic failed)", unchangedUser.Email)
	}
}
