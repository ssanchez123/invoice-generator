package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/application/command"
	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/auth"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

// testTenant is a helper to create a tenant for tests.
func testTenant(id string) *entity.Tenant {
	return &entity.Tenant{
		ID:        id,
		Name:      "Test Tenant",
		Address:   valueobject.Address{Line1: "123 Test St", City: "Test City", Country: "CO"},
		TaxID:     "900123456-7",
		Settings:  map[string]any{"default_currency": "USD"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// setupAuthTest creates in-memory repos and services for auth tests.
func setupAuthTest() (userRepo *repository.InMemoryUserRepository, tenantRepo *repository.InMemoryTenantRepository, hasher *auth.PlainPasswordHasher, tokenService *auth.JWTTokenService) {
	userRepo = repository.NewInMemoryUserRepository()
	tenantRepo = repository.NewInMemoryTenantRepository()
	hasher = auth.NewPlainPasswordHasher()
	tokenService = auth.NewJWTTokenService(auth.JWTConfig{
		Secret:      "test-secret",
		TokenExpiry: time.Hour,
	})
	return
}

func TestRegisterHandler(t *testing.T) {
	userRepo, tenantRepo, hasher, _ := setupAuthTest()

	// Seed a tenant
	tenant := testTenant("tenant-001")
	_ = tenantRepo.Save(context.Background(), tenant)

	handler := command.NewRegisterHandler(userRepo, hasher, tenantRepo)

	t.Run("successful registration", func(t *testing.T) {
		user, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID:  "tenant-001",
			Email:     "newuser@test.com",
			Password:  "Password123!",
			Role:      "member",
			FirstName: "John",
			LastName:  "Doe",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID == "" {
			t.Error("user ID should not be empty")
		}
		if user.Email != "newuser@test.com" {
			t.Errorf("email = %q, want newuser@test.com", user.Email)
		}
		if user.Role != entity.UserRoleMember {
			t.Errorf("role = %q, want member", user.Role)
		}
		if user.PasswordHash == "" || user.PasswordHash == "Password123!" {
			t.Error("password should be hashed, not plain")
		}
	})

	t.Run("duplicate email fails", func(t *testing.T) {
		_, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "tenant-001",
			Email:    "newuser@test.com",
			Password: "Password123!",
		})
		if err == nil {
			t.Error("expected error for duplicate email")
		}
	})

	t.Run("short password fails", func(t *testing.T) {
		_, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "tenant-001",
			Email:    "short@test.com",
			Password: "short",
		})
		if err == nil {
			t.Error("expected error for short password")
		}
	})

	t.Run("invalid role fails", func(t *testing.T) {
		_, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "tenant-001",
			Email:    "role@test.com",
			Password: "Password123!",
			Role:     "superadmin",
		})
		if err == nil {
			t.Error("expected error for invalid role")
		}
	})

	t.Run("non-existent tenant fails", func(t *testing.T) {
		_, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "nonexistent-tenant",
			Email:    "ghost@test.com",
			Password: "Password123!",
		})
		if err == nil {
			t.Error("expected error for non-existent tenant")
		}
	})

	t.Run("default role is member", func(t *testing.T) {
		user, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "tenant-001",
			Email:    "default@test.com",
			Password: "Password123!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != entity.UserRoleMember {
			t.Errorf("role = %q, want member (default)", user.Role)
		}
	})

	t.Run("email is normalized to lowercase", func(t *testing.T) {
		user, err := handler.Handle(context.Background(), command.RegisterCommand{
			TenantID: "tenant-001",
			Email:    "  UPPER@TEST.COM  ",
			Password: "Password123!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Email != "upper@test.com" {
			t.Errorf("email = %q, want upper@test.com", user.Email)
		}
	})
}

func TestLoginHandler(t *testing.T) {
	userRepo, tenantRepo, hasher, tokenService := setupAuthTest()

	// Seed tenant
	tenant := testTenant("tenant-001")
	_ = tenantRepo.Save(context.Background(), tenant)

	// Seed a user via Register handler
	registerH := command.NewRegisterHandler(userRepo, hasher, tenantRepo)
	_, err := registerH.Handle(context.Background(), command.RegisterCommand{
		TenantID:  "tenant-001",
		Email:     "login@test.com",
		Password:  "Password123!",
		FirstName: "Login",
		LastName:  "Test",
	})
	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	loginH := command.NewLoginHandler(userRepo, hasher, tokenService)

	t.Run("successful login returns valid token", func(t *testing.T) {
		result, err := loginH.Handle(context.Background(), command.LoginCommand{
			TenantID: "tenant-001",
			Email:    "login@test.com",
			Password: "Password123!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Token.AccessToken == "" {
			t.Error("access token should not be empty")
		}
		if result.Token.TokenType != "Bearer" {
			t.Errorf("token type = %q, want Bearer", result.Token.TokenType)
		}
		if result.ExpiresIn <= 0 {
			t.Error("expires_in should be positive")
		}
		if result.User.Email != "login@test.com" {
			t.Errorf("user email = %q, want login@test.com", result.User.Email)
		}
	})

	t.Run("wrong password fails", func(t *testing.T) {
		_, err := loginH.Handle(context.Background(), command.LoginCommand{
			TenantID: "tenant-001",
			Email:    "login@test.com",
			Password: "WrongPassword!",
		})
		if err == nil {
			t.Error("expected error for wrong password")
		}
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		_, err := loginH.Handle(context.Background(), command.LoginCommand{
			TenantID: "tenant-001",
			Email:    "ghost@test.com",
			Password: "Password123!",
		})
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})

	t.Run("token is valid", func(t *testing.T) {
		result, err := loginH.Handle(context.Background(), command.LoginCommand{
			TenantID: "tenant-001",
			Email:    "login@test.com",
			Password: "Password123!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claims, err := tokenService.Validate(result.Token.AccessToken)
		if err != nil {
			t.Fatalf("token validation failed: %v", err)
		}
		if claims.Email != "login@test.com" {
			t.Errorf("claims email = %q, want login@test.com", claims.Email)
		}
		if claims.TenantID != "tenant-001" {
			t.Errorf("claims tenant_id = %q, want tenant-001", claims.TenantID)
		}
	})
}
