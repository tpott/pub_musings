package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiKeyEncryptor_LoadOrInitialize(t *testing.T) {
	// Create temp directory for keys
	tmpDir, err := os.MkdirTemp("", "multi-key-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")

	// Create new MultiKeyEncryptor
	m := NewMultiKeyEncryptor(keysDir)

	// Initialize should create first key
	if err := m.LoadOrInitialize(); err != nil {
		t.Fatalf("LoadOrInitialize failed: %v", err)
	}

	// Should have version 1
	if m.GetCurrentVersion() != 1 {
		t.Errorf("expected current version 1, got %d", m.GetCurrentVersion())
	}

	// Should have exactly one key
	versions := m.GetVersions()
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("expected versions [1], got %v", versions)
	}

	// Key file should exist
	keyPath := filepath.Join(keysDir, "key_v1.age")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("key_v1.age file was not created")
	}

	// Version file should exist
	versionPath := filepath.Join(keysDir, "current_version")
	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		t.Error("current_version file was not created")
	}
}

func TestMultiKeyEncryptor_LegacyMigration(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-legacy-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a legacy key at data/age.key location
	legacyEnc, err := NewEncryptor("")
	if err != nil {
		t.Fatal(err)
	}

	legacyKeyContent := "# age secret key - DO NOT SHARE\n# public key: " +
		legacyEnc.PublicKey() + "\n" + legacyEnc.PrivateKey() + "\n"

	legacyPath := filepath.Join(tmpDir, "age.key")
	if err := os.WriteFile(legacyPath, []byte(legacyKeyContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Create MultiKeyEncryptor pointing to keys subdirectory
	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	// Initialize should migrate legacy key
	if err := m.LoadOrInitialize(); err != nil {
		t.Fatalf("LoadOrInitialize failed: %v", err)
	}

	// Should have version 1 with same public key
	if m.GetCurrentVersion() != 1 {
		t.Errorf("expected current version 1, got %d", m.GetCurrentVersion())
	}

	enc := m.GetEncryptor()
	if enc.PublicKey() != legacyEnc.PublicKey() {
		t.Error("migrated key has different public key")
	}
}

func TestMultiKeyEncryptor_Rotate(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-rotate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Get original key
	v1Enc := m.GetEncryptor()
	v1PubKey := v1Enc.PublicKey()

	// Rotate to version 2
	newVersion, err := m.Rotate()
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	if newVersion != 2 {
		t.Errorf("expected new version 2, got %d", newVersion)
	}

	if m.GetCurrentVersion() != 2 {
		t.Errorf("current version should be 2, got %d", m.GetCurrentVersion())
	}

	// New key should be different
	v2Enc := m.GetEncryptor()
	if v2Enc.PublicKey() == v1PubKey {
		t.Error("new key should have different public key")
	}

	// Both versions should be available
	versions := m.GetVersions()
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// Should be able to get both encryptors
	if _, err := m.GetEncryptorForVersion(1); err != nil {
		t.Errorf("failed to get encryptor for version 1: %v", err)
	}
	if _, err := m.GetEncryptorForVersion(2); err != nil {
		t.Errorf("failed to get encryptor for version 2: %v", err)
	}
}

func TestMultiKeyEncryptor_EncryptDecryptFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-file-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Create test file
	testContent := []byte("Hello, this is a test file for encryption!")
	testPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Encrypt with version 1
	encPath, version, err := m.EncryptFile(testPath)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}
	defer os.Remove(encPath)

	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Decrypt with version 1
	decrypted, err := m.DecryptFile(encPath, 1)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	if string(decrypted) != string(testContent) {
		t.Error("decrypted content doesn't match original")
	}

	// Rotate to version 2
	if _, err := m.Rotate(); err != nil {
		t.Fatal(err)
	}

	// Create another test file
	testContent2 := []byte("Another test file for version 2!")
	testPath2 := filepath.Join(tmpDir, "test2.txt")
	if err := os.WriteFile(testPath2, testContent2, 0644); err != nil {
		t.Fatal(err)
	}

	// Encrypt with version 2
	encPath2, version2, err := m.EncryptFile(testPath2)
	if err != nil {
		t.Fatalf("EncryptFile v2 failed: %v", err)
	}
	defer os.Remove(encPath2)

	if version2 != 2 {
		t.Errorf("expected version 2, got %d", version2)
	}

	// Can still decrypt version 1 file
	decrypted1, err := m.DecryptFile(encPath, 1)
	if err != nil {
		t.Fatalf("DecryptFile v1 after rotate failed: %v", err)
	}
	if string(decrypted1) != string(testContent) {
		t.Error("v1 decrypted content doesn't match")
	}

	// Can decrypt version 2 file
	decrypted2, err := m.DecryptFile(encPath2, 2)
	if err != nil {
		t.Fatalf("DecryptFile v2 failed: %v", err)
	}
	if string(decrypted2) != string(testContent2) {
		t.Error("v2 decrypted content doesn't match")
	}

	// Should fail with wrong version
	if _, err := m.DecryptFile(encPath, 2); err == nil {
		t.Error("expected error when decrypting v1 file with v2 key")
	}
}

func TestMultiKeyEncryptor_ReencryptFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-reencrypt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Create and encrypt test file with v1
	testContent := []byte("Content to be re-encrypted!")
	testPath := filepath.Join(tmpDir, "reencrypt.txt")
	if err := os.WriteFile(testPath, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	encPath, _, err := m.EncryptFile(testPath)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate to version 2
	if _, err := m.Rotate(); err != nil {
		t.Fatal(err)
	}

	// Re-encrypt from v1 to v2
	newEncPath, newVersion, err := m.ReencryptFile(encPath, 1)
	if err != nil {
		t.Fatalf("ReencryptFile failed: %v", err)
	}
	defer os.Remove(newEncPath)

	if newVersion != 2 {
		t.Errorf("expected new version 2, got %d", newVersion)
	}

	// Decrypt with v2
	decrypted, err := m.DecryptFile(newEncPath, 2)
	if err != nil {
		t.Fatalf("DecryptFile after reencrypt failed: %v", err)
	}

	if string(decrypted) != string(testContent) {
		t.Error("re-encrypted content doesn't match original")
	}

	// Old file should still exist (caller's responsibility to clean up)
	if _, err := os.Stat(encPath); os.IsNotExist(err) {
		t.Error("original encrypted file should still exist")
	}
}

func TestMultiKeyEncryptor_EncryptDecryptBytes(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-bytes-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	testData := []byte("Test data for byte encryption")

	// Encrypt
	encrypted, version, err := m.EncryptBytes(testData)
	if err != nil {
		t.Fatalf("EncryptBytes failed: %v", err)
	}

	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Decrypt
	decrypted, err := m.DecryptBytes(encrypted, 1)
	if err != nil {
		t.Fatalf("DecryptBytes failed: %v", err)
	}

	if string(decrypted) != string(testData) {
		t.Error("decrypted bytes don't match original")
	}
}

func TestMultiKeyEncryptor_LoadKeys(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-load-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")

	// Create first encryptor and initialize
	m1 := NewMultiKeyEncryptor(keysDir)
	if err := m1.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Rotate twice
	if _, err := m1.Rotate(); err != nil {
		t.Fatal(err)
	}
	if _, err := m1.Rotate(); err != nil {
		t.Fatal(err)
	}

	// Create new encryptor and load existing keys
	m2 := NewMultiKeyEncryptor(keysDir)
	if err := m2.LoadKeys(); err != nil {
		t.Fatalf("LoadKeys failed: %v", err)
	}

	// Should have all 3 versions
	versions := m2.GetVersions()
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	// Current version should be 3
	if m2.GetCurrentVersion() != 3 {
		t.Errorf("expected current version 3, got %d", m2.GetCurrentVersion())
	}
}

func TestMultiKeyEncryptor_DecryptToTempFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-tempfile-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Create and encrypt test file
	testContent := []byte("Temp file content")
	testPath := filepath.Join(tmpDir, "original.mp4")
	if err := os.WriteFile(testPath, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	encPath, version, err := m.EncryptFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(encPath)

	// Decrypt to temp file
	tempPath, err := m.DecryptToTempFile(encPath, version)
	if err != nil {
		t.Fatalf("DecryptToTempFile failed: %v", err)
	}
	defer os.Remove(tempPath)

	// Check content
	data, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(testContent) {
		t.Error("temp file content doesn't match")
	}

	// Extension should be preserved
	if filepath.Ext(tempPath) != ".mp4" {
		t.Errorf("expected .mp4 extension, got %s", filepath.Ext(tempPath))
	}
}

func TestMultiKeyEncryptor_VersionNotFound(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "multi-key-version-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	keysDir := filepath.Join(tmpDir, "keys")
	m := NewMultiKeyEncryptor(keysDir)

	if err := m.LoadOrInitialize(); err != nil {
		t.Fatal(err)
	}

	// Try to decrypt with non-existent version
	_, err = m.DecryptFile("/some/path.age", 99)
	if err == nil {
		t.Error("expected error for non-existent key version")
	}

	// Try to get encryptor for non-existent version
	_, err = m.GetEncryptorForVersion(99)
	if err == nil {
		t.Error("expected error for non-existent key version")
	}
}
