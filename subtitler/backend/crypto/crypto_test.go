package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEncryptor(t *testing.T) {
	// Test generating new key
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	if !strings.HasPrefix(enc.PublicKey(), "age1") {
		t.Errorf("Public key should start with 'age1', got: %s", enc.PublicKey())
	}

	if !strings.HasPrefix(enc.PrivateKey(), "AGE-SECRET-KEY-") {
		t.Errorf("Private key should start with 'AGE-SECRET-KEY-', got: %s", enc.PrivateKey())
	}
}

func TestNewEncryptorWithKey(t *testing.T) {
	// First generate a key
	enc1, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create another encryptor with the same key
	enc2, err := NewEncryptor(enc1.PrivateKey())
	if err != nil {
		t.Fatalf("Failed to create encryptor from key: %v", err)
	}

	if enc1.PublicKey() != enc2.PublicKey() {
		t.Error("Public keys should match")
	}

	if enc1.PrivateKey() != enc2.PrivateKey() {
		t.Error("Private keys should match")
	}
}

func TestNewEncryptorInvalidKey(t *testing.T) {
	_, err := NewEncryptor("invalid-key")
	if err == nil {
		t.Error("Expected error for invalid key")
	}
}

func TestEncryptDecryptBytes(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("Hello, World! This is a test message for encryption.")

	ciphertext, err := enc.EncryptBytes(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Error("Ciphertext should be different from plaintext")
	}

	decrypted, err := enc.DecryptBytes(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data doesn't match original: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptDecryptLargeData(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create 1MB of test data
	plaintext := make([]byte, 1<<20)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := enc.EncryptBytes(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt large data: %v", err)
	}

	decrypted, err := enc.DecryptBytes(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt large data: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Decrypted large data doesn't match original")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	plaintext := []byte("This is test file content for encryption.")
	if err := os.WriteFile(testFile, plaintext, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Encrypt file
	encPath, err := enc.EncryptFile(testFile)
	if err != nil {
		t.Fatalf("Failed to encrypt file: %v", err)
	}

	if !strings.HasSuffix(encPath, ".age") {
		t.Errorf("Encrypted file should have .age extension: %s", encPath)
	}

	// Verify encrypted file exists and is different
	encData, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatalf("Failed to read encrypted file: %v", err)
	}

	if bytes.Equal(plaintext, encData) {
		t.Error("Encrypted file should be different from original")
	}

	// Decrypt file
	decrypted, err := enc.DecryptFile(encPath)
	if err != nil {
		t.Fatalf("Failed to decrypt file: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted content doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

func TestDecryptToTempFile(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "video.mp4")
	plaintext := []byte("fake video content")
	if err := os.WriteFile(testFile, plaintext, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Encrypt file
	encPath, err := enc.EncryptFile(testFile)
	if err != nil {
		t.Fatalf("Failed to encrypt file: %v", err)
	}

	// Decrypt to temp file
	tmpPath, err := enc.DecryptToTempFile(encPath)
	if err != nil {
		t.Fatalf("Failed to decrypt to temp file: %v", err)
	}
	defer os.Remove(tmpPath)

	// Verify extension is preserved
	if !strings.HasSuffix(tmpPath, ".mp4") {
		t.Errorf("Temp file should have .mp4 extension: %s", tmpPath)
	}

	// Verify content
	decrypted, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Temp file content doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

func TestLoadOrGenerateKey(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "crypto-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "keys", "age.key")

	// First call should generate a new key
	enc1, isNew, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("Failed to load/generate key: %v", err)
	}

	if !isNew {
		t.Error("First call should generate a new key")
	}

	// Verify key file was created
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file should have been created")
	}

	// Read key file and verify format
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}

	keyContent := string(keyData)
	if !strings.Contains(keyContent, "AGE-SECRET-KEY-") {
		t.Error("Key file should contain secret key")
	}
	if !strings.Contains(keyContent, "public key:") {
		t.Error("Key file should contain public key comment")
	}

	// Second call should load the existing key
	enc2, isNew, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("Failed to load existing key: %v", err)
	}

	if isNew {
		t.Error("Second call should load existing key, not generate new one")
	}

	if enc1.PublicKey() != enc2.PublicKey() {
		t.Error("Loaded key should match original")
	}

	if enc1.PrivateKey() != enc2.PrivateKey() {
		t.Error("Loaded private key should match original")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	enc1, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor 1: %v", err)
	}

	enc2, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor 2: %v", err)
	}

	plaintext := []byte("secret message")

	// Encrypt with key 1
	ciphertext, err := enc1.EncryptBytes(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Try to decrypt with key 2
	_, err = enc2.DecryptBytes(ciphertext)
	if err == nil {
		t.Error("Should fail to decrypt with wrong key")
	}
}

func TestEncryptionEnabledToggle(t *testing.T) {
	// Save original state
	original := IsEncryptionEnabled()
	defer SetEncryptionEnabled(original)

	// Test default is enabled
	SetEncryptionEnabled(true)
	if !IsEncryptionEnabled() {
		t.Error("Expected encryption to be enabled")
	}

	// Test disable
	SetEncryptionEnabled(false)
	if IsEncryptionEnabled() {
		t.Error("Expected encryption to be disabled")
	}
}

func TestEncryptFileWithEncryptionDisabled(t *testing.T) {
	// Save original state
	original := IsEncryptionEnabled()
	defer SetEncryptionEnabled(original)

	// Disable encryption
	SetEncryptionEnabled(false)

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test data for encryption toggle test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create encryptor
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt file - should return original path when disabled
	resultPath, err := enc.EncryptFile(testFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Should return original path, not .age path
	if resultPath != testFile {
		t.Errorf("Expected original path %s, got %s", testFile, resultPath)
	}

	// .age file should NOT exist
	ageFile := testFile + ".age"
	if _, err := os.Stat(ageFile); !os.IsNotExist(err) {
		t.Errorf("Expected .age file to not exist when encryption disabled")
	}

	// Original file should still exist with original content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if !bytes.Equal(content, testData) {
		t.Error("File content changed unexpectedly")
	}
}

func TestEncryptFileWithEncryptionEnabled(t *testing.T) {
	// Save original state
	original := IsEncryptionEnabled()
	defer SetEncryptionEnabled(original)

	// Enable encryption
	SetEncryptionEnabled(true)

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test data for encryption test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create encryptor
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt file - should create .age file
	resultPath, err := enc.EncryptFile(testFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Should return .age path
	expectedPath := testFile + ".age"
	if resultPath != expectedPath {
		t.Errorf("Expected .age path %s, got %s", expectedPath, resultPath)
	}

	// .age file should exist
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Expected .age file to exist: %v", err)
	}

	// Verify it's actually encrypted (can be decrypted)
	decrypted, err := enc.DecryptFile(resultPath)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, testData) {
		t.Error("Decrypted content doesn't match original")
	}
}
