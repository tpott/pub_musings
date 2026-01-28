package crypto

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"filippo.io/age"
	"github.com/trevor/subtitler/backend/logging"
)

// MultiKeyEncryptor manages multiple encryption keys for key rotation support.
// It allows encrypting new files with the current key while still being able
// to decrypt files encrypted with older keys.
type MultiKeyEncryptor struct {
	keys       map[int]*Encryptor // version -> encryptor
	currentVer int                // current key version for new files
	keysDir    string             // directory containing key files
	mu         sync.RWMutex
}

// NewMultiKeyEncryptor creates a new MultiKeyEncryptor.
// If keysDir is empty, uses "data/keys" as default.
func NewMultiKeyEncryptor(keysDir string) *MultiKeyEncryptor {
	if keysDir == "" {
		keysDir = "data/keys"
	}
	return &MultiKeyEncryptor{
		keys:    make(map[int]*Encryptor),
		keysDir: keysDir,
	}
}

// LoadKeys loads all keys from the keys directory.
// Returns error if no keys are found.
func (m *MultiKeyEncryptor) LoadKeys() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(m.keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Look for key files matching pattern key_vN.age
	entries, err := os.ReadDir(m.keysDir)
	if err != nil {
		return fmt.Errorf("failed to read keys directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "key_v") || !strings.HasSuffix(name, ".age") {
			continue
		}

		// Extract version number
		versionStr := strings.TrimPrefix(name, "key_v")
		versionStr = strings.TrimSuffix(versionStr, ".age")
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue // Skip files with invalid version numbers
		}

		// Load the key
		keyPath := filepath.Join(m.keysDir, name)
		enc, _, err := LoadOrGenerateKey(keyPath)
		if err != nil {
			return fmt.Errorf("failed to load key version %d: %w", version, err)
		}
		m.keys[version] = enc
	}

	// Read current version from file
	versionFile := filepath.Join(m.keysDir, "current_version")
	versionData, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) && len(m.keys) > 0 {
			// No version file but keys exist - use highest version
			for v := range m.keys {
				if v > m.currentVer {
					m.currentVer = v
				}
			}
			// Write the version file
			if err := os.WriteFile(versionFile, []byte(strconv.Itoa(m.currentVer)), 0600); err != nil {
				return fmt.Errorf("failed to write current version file: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read current version: %w", err)
		}
	} else {
		m.currentVer, err = strconv.Atoi(strings.TrimSpace(string(versionData)))
		if err != nil {
			return fmt.Errorf("invalid current version file: %w", err)
		}
	}

	if len(m.keys) == 0 {
		return fmt.Errorf("no keys found in %s", m.keysDir)
	}

	// Verify current version key exists
	if _, ok := m.keys[m.currentVer]; !ok {
		return fmt.Errorf("current version key %d not found", m.currentVer)
	}

	return nil
}

// LoadOrInitialize loads keys or creates the first key if none exist.
func (m *MultiKeyEncryptor) LoadOrInitialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(m.keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Check for legacy key at data/age.key
	legacyPath := filepath.Join(filepath.Dir(m.keysDir), "age.key")
	keyV1Path := filepath.Join(m.keysDir, "key_v1.age")

	// If legacy key exists and key_v1 doesn't, migrate
	if _, err := os.Stat(legacyPath); err == nil {
		if _, err := os.Stat(keyV1Path); os.IsNotExist(err) {
			// Copy legacy key to versioned location
			data, err := os.ReadFile(legacyPath)
			if err != nil {
				return fmt.Errorf("failed to read legacy key: %w", err)
			}
			if err := os.WriteFile(keyV1Path, data, 0600); err != nil {
				return fmt.Errorf("failed to copy legacy key to versioned location: %w", err)
			}
		}
	}

	// Load key_v1 or generate if not exists
	entries, err := os.ReadDir(m.keysDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read keys directory: %w", err)
	}

	keyFound := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "key_v") && strings.HasSuffix(entry.Name(), ".age") {
			keyFound = true
			break
		}
	}

	if !keyFound {
		// Generate first key
		enc, _, err := LoadOrGenerateKey(keyV1Path)
		if err != nil {
			return fmt.Errorf("failed to generate first key: %w", err)
		}
		m.keys[1] = enc
		m.currentVer = 1

		// Write version file
		versionFile := filepath.Join(m.keysDir, "current_version")
		if err := os.WriteFile(versionFile, []byte("1"), 0600); err != nil {
			return fmt.Errorf("failed to write current version file: %w", err)
		}
		return nil
	}

	// Release lock before calling LoadKeys
	m.mu.Unlock()
	err = m.LoadKeys()
	m.mu.Lock()
	return err
}

// GetCurrentVersion returns the current key version for new encryptions.
func (m *MultiKeyEncryptor) GetCurrentVersion() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentVer
}

// GetVersions returns all available key versions.
func (m *MultiKeyEncryptor) GetVersions() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := make([]int, 0, len(m.keys))
	for v := range m.keys {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions
}

// Rotate generates a new key and makes it the current version.
// Returns the new version number.
func (m *MultiKeyEncryptor) Rotate() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newVersion := m.currentVer + 1
	keyPath := filepath.Join(m.keysDir, fmt.Sprintf("key_v%d.age", newVersion))

	// Generate new key
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return 0, fmt.Errorf("failed to generate new key: %w", err)
	}

	// Save key to file
	keyContent := fmt.Sprintf("# age secret key - DO NOT SHARE\n# public key: %s\n%s\n",
		identity.Recipient().String(), identity.String())
	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		return 0, fmt.Errorf("failed to save new key: %w", err)
	}

	// Create encryptor
	enc := &Encryptor{
		identity:  identity,
		recipient: identity.Recipient(),
	}
	m.keys[newVersion] = enc
	m.currentVer = newVersion

	// Update version file
	versionFile := filepath.Join(m.keysDir, "current_version")
	if err := os.WriteFile(versionFile, []byte(strconv.Itoa(newVersion)), 0600); err != nil {
		return 0, fmt.Errorf("failed to update current version file: %w", err)
	}

	return newVersion, nil
}

// EncryptFile encrypts a file with the current key.
// Returns the encrypted file path and the key version used.
// If encryption is disabled, returns the original path and version 0.
func (m *MultiKeyEncryptor) EncryptFile(srcPath string) (string, int, error) {
	if !encryptionEnabled {
		return srcPath, 0, nil
	}

	m.mu.RLock()
	enc, ok := m.keys[m.currentVer]
	version := m.currentVer
	m.mu.RUnlock()

	if !ok {
		return "", 0, fmt.Errorf("current key version %d not loaded", version)
	}

	dstPath, err := enc.EncryptFile(srcPath)
	if err != nil {
		return "", 0, err
	}

	return dstPath, version, nil
}

// DecryptFile decrypts a file using the specified key version.
// Returns the plaintext as bytes.
func (m *MultiKeyEncryptor) DecryptFile(encPath string, version int) ([]byte, error) {
	m.mu.RLock()
	enc, ok := m.keys[version]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return enc.DecryptFile(encPath)
}

// DecryptToTempFile decrypts a file to a temporary file using the specified key version.
// The caller is responsible for removing the temp file.
func (m *MultiKeyEncryptor) DecryptToTempFile(encPath string, version int) (string, error) {
	m.mu.RLock()
	enc, ok := m.keys[version]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("key version %d not found", version)
	}

	return enc.DecryptToTempFile(encPath)
}

// DecryptToFile decrypts a file to a destination path using the specified key version.
func (m *MultiKeyEncryptor) DecryptToFile(encPath, dstPath string, version int) error {
	m.mu.RLock()
	enc, ok := m.keys[version]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("key version %d not found", version)
	}

	return enc.DecryptToFile(encPath, dstPath)
}

// ReencryptFile re-encrypts a file from an old key version to the current version.
// Returns the new encrypted file path and the new key version.
// The old encrypted file is NOT deleted - caller should handle cleanup.
func (m *MultiKeyEncryptor) ReencryptFile(encPath string, oldVersion int) (string, int, error) {
	m.mu.RLock()
	oldEnc, ok := m.keys[oldVersion]
	if !ok {
		m.mu.RUnlock()
		return "", 0, fmt.Errorf("old key version %d not found", oldVersion)
	}
	newEnc, ok := m.keys[m.currentVer]
	if !ok {
		m.mu.RUnlock()
		return "", 0, fmt.Errorf("current key version %d not found", m.currentVer)
	}
	newVersion := m.currentVer
	m.mu.RUnlock()

	if oldVersion == newVersion {
		return encPath, newVersion, nil // Already using current version
	}

	// Decrypt to temp file
	tempPath, err := oldEnc.DecryptToTempFile(encPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to decrypt with old key: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logging.Warn("Failed to remove temp file during re-encryption cleanup", "path", tempPath, "error", removeErr)
		}
	}()

	// Encrypt with new key
	newPath, err := newEnc.EncryptFile(tempPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newPath, newVersion, nil
}

// EncryptBytes encrypts data with the current key.
func (m *MultiKeyEncryptor) EncryptBytes(data []byte) ([]byte, int, error) {
	m.mu.RLock()
	enc, ok := m.keys[m.currentVer]
	version := m.currentVer
	m.mu.RUnlock()

	if !ok {
		return nil, 0, fmt.Errorf("current key version %d not loaded", version)
	}

	ciphertext, err := enc.EncryptBytes(data)
	if err != nil {
		return nil, 0, err
	}

	return ciphertext, version, nil
}

// DecryptBytes decrypts data using the specified key version.
func (m *MultiKeyEncryptor) DecryptBytes(ciphertext []byte, version int) ([]byte, error) {
	m.mu.RLock()
	enc, ok := m.keys[version]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return enc.DecryptBytes(ciphertext)
}

// EncryptReader encrypts data from a reader using the current key.
func (m *MultiKeyEncryptor) EncryptReader(r io.Reader) ([]byte, int, error) {
	m.mu.RLock()
	enc, ok := m.keys[m.currentVer]
	version := m.currentVer
	m.mu.RUnlock()

	if !ok {
		return nil, 0, fmt.Errorf("current key version %d not loaded", version)
	}

	ciphertext, err := enc.EncryptReader(r)
	if err != nil {
		return nil, 0, err
	}

	return ciphertext, version, nil
}

// DecryptReader decrypts data from a reader using the specified key version.
func (m *MultiKeyEncryptor) DecryptReader(r io.Reader, version int) (io.Reader, error) {
	m.mu.RLock()
	enc, ok := m.keys[version]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return enc.DecryptReader(r)
}

// GetEncryptor returns the encryptor for the current version.
// For backward compatibility with code that uses single Encryptor.
func (m *MultiKeyEncryptor) GetEncryptor() *Encryptor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.keys[m.currentVer]
}

// GetEncryptorForVersion returns the encryptor for a specific version.
func (m *MultiKeyEncryptor) GetEncryptorForVersion(version int) (*Encryptor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	enc, ok := m.keys[version]
	if !ok {
		return nil, fmt.Errorf("key version %d not found", version)
	}
	return enc, nil
}
