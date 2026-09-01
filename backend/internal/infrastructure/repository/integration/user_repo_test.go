//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestUserRepository_SaveAndFindByEmail(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	userRepo := repository.NewUserRepository(testDB.DB)

	now := time.Now()
	user := &entity.User{
		ID:           "cccccccc-cccc-cccc-cccc-cccccccccccc",
		TenantID:     TestTenantID,
		Email:        "testuser@test.com",
		PasswordHash: "$2a$10$somehashvalue",
		Role:         entity.UserRoleMember,
		FirstName:    "Test",
		LastName:     "User",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Save
	if err := userRepo.Save(ctx, user); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// FindByEmail
	loaded, err := userRepo.FindByEmail(ctx, TestTenantID, "testuser@test.com")
	if err != nil {
		t.Fatalf("FindByEmail failed: %v", err)
	}
	if loaded.Email != "testuser@test.com" {
		t.Errorf("Email = %s, want testuser@test.com", loaded.Email)
	}
	if loaded.Role != entity.UserRoleMember {
		t.Errorf("Role = %s, want member", loaded.Role)
	}
	if loaded.FirstName != "Test" {
		t.Errorf("FirstName = %s, want Test", loaded.FirstName)
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	userRepo := repository.NewUserRepository(testDB.DB)

	// Find the seed admin user
	user, err := userRepo.FindByID(ctx, "00000000-0000-0000-0000-000000000100")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if user.Email != "admin@acme.com" {
		t.Errorf("Email = %s, want admin@acme.com", user.Email)
	}
	if user.Role != entity.UserRoleAdmin {
		t.Errorf("Role = %s, want admin", user.Role)
	}
}

func TestUserRepository_EmailExists(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	userRepo := repository.NewUserRepository(testDB.DB)

	// Seed user email should exist
	exists, err := userRepo.EmailExists(ctx, TestTenantID, "admin@acme.com")
	if err != nil {
		t.Fatalf("EmailExists failed: %v", err)
	}
	if !exists {
		t.Error("admin@acme.com should exist")
	}

	// Non-existent email should not exist
	exists, err = userRepo.EmailExists(ctx, TestTenantID, "ghost@test.com")
	if err != nil {
		t.Fatalf("EmailExists (2) failed: %v", err)
	}
	if exists {
		t.Error("ghost@test.com should not exist")
	}
}

func TestUserRepository_RLSBlocksCrossTenant(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(testDB.DB)

	// Create a second tenant
	tenantB := "22222222-2222-2222-2222-222222222222"
	now := time.Now()

	// Insert tenant B directly (bypass RLS)
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO tenants (id, name, address, settings, created_at, updated_at)
		VALUES ($1, 'Tenant B', '{}', '{}', NOW(), NOW())
		ON CONFLICT DO NOTHING
	`, tenantB)
	if err != nil {
		t.Fatalf("failed to create tenant B: %v", err)
	}

	// Create a user in tenant B
	userB := &entity.User{
		ID:           "dddddddd-dddd-dddd-dddd-dddddddddddd",
		TenantID:     tenantB,
		Email:        "userb@tenantb.com",
		PasswordHash: "hash",
		Role:         entity.UserRoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	ctxB := ContextWithTenant(tenantB)
	if err := userRepo.Save(ctxB, userB); err != nil {
		t.Fatalf("Save user B failed: %v", err)
	}

	// Try to find user B from tenant A's context — should NOT find it
	ctxA := ContextWithTenant(TestTenantID)
	_, err = userRepo.FindByEmail(ctxA, TestTenantID, "userb@tenantb.com")
	if err == nil {
		t.Error("RLS should block cross-tenant access: found user B from tenant A")
	}

	// Verify user B IS accessible from tenant B's context
	_, err = userRepo.FindByEmail(ctxB, tenantB, "userb@tenantb.com")
	if err != nil {
		t.Errorf("user B should be accessible from tenant B: %v", err)
	}
}
