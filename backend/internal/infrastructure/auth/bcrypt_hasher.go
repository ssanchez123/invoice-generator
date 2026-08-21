package auth

import (
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"golang.org/x/crypto/bcrypt"
)

// BcryptPasswordHasher implements port.PasswordHasher using bcrypt.
type BcryptPasswordHasher struct{}

// NewBcryptPasswordHasher creates a new BcryptPasswordHasher.
func NewBcryptPasswordHasher() *BcryptPasswordHasher {
	return &BcryptPasswordHasher{}
}

// Hash hashes a plain-text password using bcrypt with default cost.
func (h *BcryptPasswordHasher) Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify checks a plain-text password against a bcrypt hash.
func (h *BcryptPasswordHasher) Verify(plain, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	return err == nil
}

// Ensure interface compliance
var _ port.PasswordHasher = (*BcryptPasswordHasher)(nil)
