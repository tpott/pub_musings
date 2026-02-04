package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	identity, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if identity == nil {
		t.Fatal("GenerateKey returned nil identity")
	}

	// Verify identity string format (starts with AGE-SECRET-KEY-)
	idStr := identity.String()
	if len(idStr) < 50 {
		t.Errorf("Identity string too short: %s", idStr)
	}

	// Verify recipient string format (starts with age1)
	recipStr := identity.Recipient().String()
	if len(recipStr) < 50 {
		t.Errorf("Recipient string too short: %s", recipStr)
	}
}

func TestLoadIdentity(t *testing.T) {
	// Generate a key first
	original, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Load it back
	loaded, err := LoadIdentity(original.String())
	if err != nil {
		t.Fatalf("LoadIdentity failed: %v", err)
	}

	// Verify they're the same
	if original.String() != loaded.String() {
		t.Errorf("Loaded identity doesn't match original")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	dir := t.TempDir()

	// Create test data
	plaintext := []byte("Hello, this is test data for encryption!")
	srcPath := filepath.Join(dir, "plaintext.txt")
	encPath := filepath.Join(dir, "encrypted.age")
	decPath := filepath.Join(dir, "decrypted.txt")

	if err := os.WriteFile(srcPath, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Generate key
	identity, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Encrypt
	if err := EncryptFile(srcPath, encPath, identity); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Verify encrypted file exists and is different from plaintext
	encData, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatalf("Read encrypted file failed: %v", err)
	}
	if bytes.Equal(encData, plaintext) {
		t.Error("Encrypted data should not equal plaintext")
	}

	// Decrypt
	if err := DecryptFile(encPath, decPath, identity); err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify decrypted content matches original
	decData, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatalf("Read decrypted file failed: %v", err)
	}
	if !bytes.Equal(decData, plaintext) {
		t.Errorf("Decrypted data doesn't match original.\nGot: %s\nWant: %s", decData, plaintext)
	}
}

func TestEncryptDecryptBytes(t *testing.T) {
	plaintext := []byte("Test data for in-memory encryption")

	// Generate key
	identity, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Encrypt
	ciphertext, err := EncryptBytes(plaintext, identity)
	if err != nil {
		t.Fatalf("EncryptBytes failed: %v", err)
	}

	// Verify encrypted data is different
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := DecryptBytes(ciphertext, identity)
	if err != nil {
		t.Fatalf("DecryptBytes failed: %v", err)
	}

	// Verify roundtrip
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data doesn't match original.\nGot: %s\nWant: %s", decrypted, plaintext)
	}
}

func TestEncryptDecryptLargeFile(t *testing.T) {
	dir := t.TempDir()

	// Create large test data (1MB)
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	srcPath := filepath.Join(dir, "large.bin")
	encPath := filepath.Join(dir, "large.age")
	decPath := filepath.Join(dir, "large_dec.bin")

	if err := os.WriteFile(srcPath, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Generate key
	identity, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Encrypt
	if err := EncryptFile(srcPath, encPath, identity); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt
	if err := DecryptFile(encPath, decPath, identity); err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify roundtrip
	decData, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatalf("Read decrypted file failed: %v", err)
	}
	if !bytes.Equal(decData, plaintext) {
		t.Error("Decrypted large file doesn't match original")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	dir := t.TempDir()

	plaintext := []byte("Secret data")
	srcPath := filepath.Join(dir, "plaintext.txt")
	encPath := filepath.Join(dir, "encrypted.age")
	decPath := filepath.Join(dir, "decrypted.txt")

	if err := os.WriteFile(srcPath, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Generate two different keys
	identity1, _ := GenerateKey()
	identity2, _ := GenerateKey()

	// Encrypt with key 1
	if err := EncryptFile(srcPath, encPath, identity1); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Try to decrypt with key 2 (should fail)
	err := DecryptFile(encPath, decPath, identity2)
	if err == nil {
		t.Error("DecryptFile should fail with wrong key")
	}
}
