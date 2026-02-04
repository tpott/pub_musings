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
