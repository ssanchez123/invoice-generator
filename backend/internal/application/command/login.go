package command

import (
	"context"
	"strings"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// LoginCommand authenticates a user and returns a JWT token.
type LoginCommand struct {
	TenantID string
	Email    string
	Password string
}

type LoginResult struct {
	Token     entity.AuthToken
	User      *entity.User
	ExpiresIn int64 // seconds until expiry
}

type LoginHandler struct {
	userRepo     port.UserRepository
	hasher       port.PasswordHasher
	tokenService port.TokenService
}

func NewLoginHandler(ur port.UserRepository, h port.PasswordHasher, ts port.TokenService) *LoginHandler {
	return &LoginHandler{userRepo: ur, hasher: h, tokenService: ts}
}

func (h *LoginHandler) Handle(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	// Normalize email
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	if email == "" {
		return nil, ErrInvalidEmail
	}

	// Find user by email
	user, err := h.userRepo.FindByEmail(ctx, cmd.TenantID, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if !h.hasher.Verify(cmd.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Generate JWT token
	claims := entity.TokenClaims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     string(user.Role),
	}

	token, err := h.tokenService.Generate(claims)
	if err != nil {
		return nil, err
	}

	expiresIn := int64(time.Until(token.ExpiresAt).Seconds())

	return &LoginResult{
		Token:     token,
		User:      user,
		ExpiresIn: expiresIn,
	}, nil
}
