package port

import (
	"context"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
)

// UserRepository is the port for user persistence.
type UserRepository interface {
	// Save creates or updates a user.
	Save(ctx context.Context, user *entity.User) error

	// FindByEmail finds a user by email within a tenant.
	// Returns ErrNotFound if the user doesn't exist.
	FindByEmail(ctx context.Context, tenantID, email string) (*entity.User, error)

	// FindByID finds a user by ID.
	// Returns ErrNotFound if the user doesn't exist.
	FindByID(ctx context.Context, id string) (*entity.User, error)

	// EmailExists checks if an email is already registered for a tenant.
	EmailExists(ctx context.Context, tenantID, email string) (bool, error)
}

// PasswordHasher is the port for password hashing and verification.
type PasswordHasher interface {
	// Hash hashes a plain-text password.
	Hash(plain string) (string, error)

	// Verify checks a plain-text password against a hash.
	Verify(plain, hash string) bool
}

// TokenService is the port for JWT token generation and validation.
type TokenService interface {
	// Generate creates a JWT token for the given claims.
	Generate(claims entity.TokenClaims) (entity.AuthToken, error)

	// Validate parses and validates a JWT token, returning its claims.
	Validate(token string) (*entity.TokenClaims, error)
}
