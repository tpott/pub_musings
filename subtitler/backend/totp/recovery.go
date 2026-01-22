package totp

import (
	"crypto/rand"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// CodeLength is the number of characters in a recovery code (before formatting)
	CodeLength = 8

	// NumCodes is the number of recovery codes to generate
	NumCodes = 10

	// Alphabet for recovery codes - excludes ambiguous characters (0,O,1,I,L)
	Alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

	// bcryptCost for hashing recovery codes (lower than passwords since codes are random)
	bcryptCost = 10
)

// GenerateRecoveryCodes generates n random recovery codes
func GenerateRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, n)
	for i := 0; i < n; i++ {
		code, err := generateCode()
		if err != nil {
			return nil, err
		}
		codes[i] = FormatCode(code)
	}
	return codes, nil
}

// generateCode generates a single unformatted recovery code
func generateCode() (string, error) {
	bytes := make([]byte, CodeLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	code := make([]byte, CodeLength)
	for i := 0; i < CodeLength; i++ {
		code[i] = Alphabet[int(bytes[i])%len(Alphabet)]
	}
	return string(code), nil
}

// FormatCode formats a code with hyphen (XXXX-XXXX)
func FormatCode(code string) string {
	if len(code) != CodeLength {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// NormalizeCode removes hyphens and converts to uppercase for comparison
func NormalizeCode(input string) string {
	return strings.ToUpper(strings.ReplaceAll(input, "-", ""))
}

// HashCode hashes a recovery code for storage using bcrypt
func HashCode(code string) (string, error) {
	normalized := NormalizeCode(code)
	hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckCode verifies a code against its bcrypt hash
func CheckCode(code, hash string) bool {
	normalized := NormalizeCode(code)
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized))
	return err == nil
}
