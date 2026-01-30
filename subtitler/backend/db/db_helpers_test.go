package db

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashToken creates a SHA-256 hash of a token (same as email.HashToken)
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
