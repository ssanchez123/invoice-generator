package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

// JWTTokenService implements port.TokenService using golang-jwt/v5.
type JWTTokenService struct {
	secret      []byte
	tokenExpiry time.Duration
	issuer      string
}

// JWTConfig holds configuration for the JWT token service.
type JWTConfig struct {
	Secret      string
	TokenExpiry time.Duration // default: 24h
	Issuer      string        // default: "invoice-generator"
}

// NewJWTTokenService creates a new JWTTokenService.
func NewJWTTokenService(cfg JWTConfig) *JWTTokenService {
	if cfg.TokenExpiry == 0 {
		cfg.TokenExpiry = 24 * time.Hour
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "invoice-generator"
	}
	return &JWTTokenService{
		secret:      []byte(cfg.Secret),
		tokenExpiry: cfg.TokenExpiry,
		issuer:      cfg.Issuer,
	}
}

// jwtClaims is the internal claims struct mapped to JWT claims.
type jwtClaims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Generate creates a JWT token for the given claims.
func (s *JWTTokenService) Generate(claims entity.TokenClaims) (entity.AuthToken, error) {
	now := time.Now()
	expiresAt := now.Add(s.tokenExpiry)

	registeredClaims := jwtClaims{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Email:    claims.Email,
		Role:     claims.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   claims.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredClaims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return entity.AuthToken{}, fmt.Errorf("sign token: %w", err)
	}

	return entity.AuthToken{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}, nil
}

// Validate parses and validates a JWT token, returning its claims.
func (s *JWTTokenService) Validate(tokenString string) (*entity.TokenClaims, error) {
	claims := &jwtClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("validate token: %w", err)
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	return &entity.TokenClaims{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Email:    claims.Email,
		Role:     claims.Role,
	}, nil
}

// Sentinel errors for JWT validation.
var (
	ErrTokenExpired = errors.New("token has expired")
	ErrTokenInvalid = errors.New("invalid token")
)

// Ensure interface compliance
var _ port.TokenService = (*JWTTokenService)(nil)
