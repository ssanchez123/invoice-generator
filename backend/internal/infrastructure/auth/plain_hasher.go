package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// PlainPasswordHasher is a test double for PasswordHasher.
// It uses a simple SHA-256 hash — NOT secure for production, only for tests.
type PlainPasswordHasher struct{}

func NewPlainPasswordHasher() *PlainPasswordHasher {
	return &PlainPasswordHasher{}
}

func (h *PlainPasswordHasher) Hash(plain string) (string, error) {
	sum := sha256.Sum256([]byte(plain))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (h *PlainPasswordHasher) Verify(plain, hash string) bool {
	expected, err := h.Hash(plain)
	if err != nil {
		return false
	}
	return strings.EqualFold(expected, hash)
}

// Ensure interface compliance
var _ port.PasswordHasher = (*PlainPasswordHasher)(nil)
