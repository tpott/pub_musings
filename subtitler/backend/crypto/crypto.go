package crypto

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/age"
)

// encryptionEnabled controls whether encryption is active.
// Set via ENCRYPTION_ENABLED env var. Default is true.
var encryptionEnabled = os.Getenv("ENCRYPTION_ENABLED") != "false" && os.Getenv("ENCRYPTION_ENABLED") != "0"

// SetEncryptionEnabled allows programmatic control of encryption (mainly for testing)
func SetEncryptionEnabled(enabled bool) {
	encryptionEnabled = enabled
}

// IsEncryptionEnabled returns whether encryption is active
func IsEncryptionEnabled() bool {
	return encryptionEnabled
}

// Encryptor handles file encryption/decryption using age
type Encryptor struct {
	identity  *age.X25519Identity
	recipient *age.X25519Recipient
	mu        sync.RWMutex
}

// NewEncryptor creates a new Encryptor from an age private key string
// If privateKey is empty, it generates a new key
func NewEncryptor(privateKey string) (*Encryptor, error) {
	var identity *age.X25519Identity
	var err error

	if privateKey == "" {
		identity, err = age.GenerateX25519Identity()
		if err != nil {
			return nil, fmt.Errorf("failed to generate age identity: %w", err)
		}
	} else {
		identity, err = age.ParseX25519Identity(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse age private key: %w", err)
		}
	}

	return &Encryptor{
		identity:  identity,
		recipient: identity.Recipient(),
	}, nil
}

// LoadOrGenerateKey loads an age key from file or generates a new one
// Returns the Encryptor and whether a new key was generated
func LoadOrGenerateKey(keyPath string) (*Encryptor, bool, error) {
	// Try to load existing key
	data, err := os.ReadFile(keyPath)
	if err == nil {
		// Key file exists, parse it
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
				enc, err := NewEncryptor(line)
				if err != nil {
					return nil, false, err
				}
				return enc, false, nil
			}
		}
		return nil, false, fmt.Errorf("no valid age key found in %s", keyPath)
	}

	if !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("failed to read key file: %w", err)
	}

	// Generate new key
	enc, err := NewEncryptor("")
	if err != nil {
		return nil, false, err
	}

	// Save to file
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, false, fmt.Errorf("failed to create key directory: %w", err)
	}

	keyContent := fmt.Sprintf("# age secret key - DO NOT SHARE\n# public key: %s\n%s\n",
		enc.recipient.String(), enc.identity.String())

	if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err != nil {
		return nil, false, fmt.Errorf("failed to save key file: %w", err)
	}

	return enc, true, nil
}

// PublicKey returns the public key string
func (e *Encryptor) PublicKey() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.recipient.String()
}

// PrivateKey returns the private key string
func (e *Encryptor) PrivateKey() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.identity.String()
}

// EncryptReader encrypts data from a reader and returns the ciphertext as bytes
func (e *Encryptor) EncryptReader(r io.Reader) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, e.recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypt writer: %w", err)
	}

	if _, err := io.Copy(w, r); err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return buf.Bytes(), nil
}

// DecryptReader decrypts data from a reader and returns a reader for the plaintext
func (e *Encryptor) DecryptReader(r io.Reader) (io.Reader, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	reader, err := age.Decrypt(r, e.identity)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return reader, nil
}

// EncryptFile encrypts a file and saves it with .age extension
// Returns the path to the encrypted file
// If encryption is disabled, returns the original path unchanged
func (e *Encryptor) EncryptFile(srcPath string) (string, error) {
	if !encryptionEnabled {
		// When encryption is disabled, return original path
		return srcPath, nil
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dstPath := srcPath + ".age"
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("failed to create encrypted file: %w", err)
	}
	defer dst.Close()

	e.mu.RLock()
	w, err := age.Encrypt(dst, e.recipient)
	e.mu.RUnlock()
	if err != nil {
		os.Remove(dstPath)
		return "", fmt.Errorf("failed to create encrypt writer: %w", err)
	}

	if _, err := io.Copy(w, src); err != nil {
		os.Remove(dstPath)
		return "", fmt.Errorf("failed to encrypt file: %w", err)
	}

	if err := w.Close(); err != nil {
		os.Remove(dstPath)
		return "", fmt.Errorf("failed to finalize encryption: %w", err)
	}

	return dstPath, nil
}

// DecryptFile decrypts a .age file and returns the plaintext as bytes
func (e *Encryptor) DecryptFile(encPath string) ([]byte, error) {
	f, err := os.Open(encPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer f.Close()

	e.mu.RLock()
	reader, err := age.Decrypt(f, e.identity)
	e.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt file: %w", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	return data, nil
}

// DecryptToFile decrypts a .age file and saves plaintext to a destination path
func (e *Encryptor) DecryptToFile(encPath, dstPath string) error {
	f, err := os.Open(encPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer f.Close()

	e.mu.RLock()
	reader, err := age.Decrypt(f, e.identity)
	e.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to decrypt file: %w", err)
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		os.Remove(dstPath)
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	return nil
}

// DecryptToTempFile decrypts a .age file to a temporary file and returns its path
// The caller is responsible for removing the temp file when done
// Note: This always attempts decryption, regardless of encryptionEnabled setting,
// to support reading files that were encrypted before the setting was disabled.
func (e *Encryptor) DecryptToTempFile(encPath string) (string, error) {
	// Create temp file with same extension as original (minus .age)
	origExt := filepath.Ext(strings.TrimSuffix(encPath, ".age"))
	if origExt == "" {
		origExt = ".tmp"
	}

	tmpFile, err := os.CreateTemp("", "subtitler-*"+origExt)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := e.DecryptToFile(encPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

// EncryptBytes encrypts data and returns the ciphertext
func (e *Encryptor) EncryptBytes(data []byte) ([]byte, error) {
	return e.EncryptReader(bytes.NewReader(data))
}

// DecryptBytes decrypts ciphertext and returns the plaintext
func (e *Encryptor) DecryptBytes(ciphertext []byte) ([]byte, error) {
	reader, err := e.DecryptReader(bytes.NewReader(ciphertext))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}
