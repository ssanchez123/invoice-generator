package entity

import "time"

// AuthToken represents a JWT token with its metadata.
type AuthToken struct {
	AccessToken string
	TokenType   string // "Bearer"
	ExpiresAt   time.Time
}

// TokenClaims contains the claims embedded in a JWT.
type TokenClaims struct {
	UserID   string
	TenantID string
	Email    string
	Role     string
}
