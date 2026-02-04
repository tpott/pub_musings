// Package crypto provides encryption utilities for media files using age.
package crypto

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// GenerateKey generates a new X25519 identity (key pair) and returns it.
func GenerateKey() (*age.X25519Identity, error) {
	return age.GenerateX25519Identity()
}

// LoadIdentity loads an X25519 identity from a string.
func LoadIdentity(key string) (*age.X25519Identity, error) {
	return age.ParseX25519Identity(key)
}

// EncryptFile encrypts the contents of srcPath and writes to dstPath.
// The recipient is derived from the provided identity.
func EncryptFile(srcPath, dstPath string, identity *age.X25519Identity) error {
	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dst.Close()

	// Create encrypted writer using the recipient (public key)
	w, err := age.Encrypt(dst, identity.Recipient())
	if err != nil {
		return fmt.Errorf("create encryptor: %w", err)
	}

	// Copy source to encrypted writer
	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("encrypt data: %w", err)
	}

	// Must close to flush final chunk
	if err := w.Close(); err != nil {
		return fmt.Errorf("close encryptor: %w", err)
	}

	return nil
}

// DecryptFile decrypts the contents of srcPath and writes to dstPath.
func DecryptFile(srcPath, dstPath string, identity *age.X25519Identity) error {
	// Open encrypted source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dst.Close()

	// Create decrypted reader
	r, err := age.Decrypt(src, identity)
	if err != nil {
		return fmt.Errorf("create decryptor: %w", err)
	}

	// Copy decrypted data to destination
	if _, err := io.Copy(dst, r); err != nil {
		return fmt.Errorf("decrypt data: %w", err)
	}

	return nil
}

// EncryptBytes encrypts data in memory and returns the ciphertext.
func EncryptBytes(plaintext []byte, identity *age.X25519Identity) ([]byte, error) {
	var buf ByteBuffer
	w, err := age.Encrypt(&buf, identity.Recipient())
	if err != nil {
		return nil, fmt.Errorf("create encryptor: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("encrypt data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close encryptor: %w", err)
	}

	return buf.Bytes(), nil
}

// DecryptBytes decrypts ciphertext in memory and returns the plaintext.
func DecryptBytes(ciphertext []byte, identity *age.X25519Identity) ([]byte, error) {
	r, err := age.Decrypt(NewByteReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}

	return plaintext, nil
}

// ByteBuffer is a simple bytes.Buffer replacement for writing.
type ByteBuffer struct {
	data []byte
}

func (b *ByteBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *ByteBuffer) Bytes() []byte {
	return b.data
}

// ByteReader wraps a byte slice for reading.
type ByteReader struct {
	data []byte
	pos  int
}

func NewByteReader(data []byte) *ByteReader {
	return &ByteReader{data: data}
}

func (b *ByteReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

// DecryptReader creates an io.Reader that decrypts data from the source reader.
// The caller is responsible for closing the source reader after reading is complete.
func DecryptReader(src io.Reader, identity *age.X25519Identity) (io.Reader, error) {
	r, err := age.Decrypt(src, identity)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}
	return r, nil
}

// ErrInsecureKeyPermissions is returned when the key file has permissions
// that allow others to read it (mode > 0600).
var ErrInsecureKeyPermissions = fmt.Errorf("key file has insecure permissions (should be 0600 or stricter)")

// LoadIdentityFromFile loads an X25519 identity from a file.
// The file should contain a line with the format: AGE-SECRET-KEY-1...
// Returns ErrInsecureKeyPermissions if the file is world-readable or group-readable.
func LoadIdentityFromFile(path string) (*age.X25519Identity, error) {
	// Check file permissions before reading
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat key file: %w", err)
	}

	// Get file mode and check if it's too permissive
	// We want mode <= 0600 (only owner can read/write)
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		// File is readable/writable by group or others
		return nil, fmt.Errorf("%w: got mode %04o, want 0600 or stricter", ErrInsecureKeyPermissions, mode)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	// Parse the identity from the file content
	// The key file from age-keygen contains comments and the key on separate lines
	lines := splitLines(data)
	for _, line := range lines {
		line = trimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			return age.ParseX25519Identity(line)
		}
	}

	return nil, fmt.Errorf("no identity found in key file")
}

// splitLines splits data into lines without using strings package
func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// trimSpace trims leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
