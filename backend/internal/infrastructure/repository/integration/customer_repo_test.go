//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/entity"
	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestCustomerRepository_SaveAndFindByID(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	customerRepo := repository.NewCustomerRepository(testDB.DB)

	customer, err := entity.NewCustomer(
		"77777777-7777-7777-7777-777777777777",
		TestTenantID,
		"Integration Test Customer",
		"integration@test.com",
		valueobject.Address{
			Line1:   "Calle 123",
			City:    "Bogotá",
			Country: "CO",
		},
	)
	if err != nil {
		t.Fatalf("NewCustomer failed: %v", err)
	}

	// Save
	if err := customerRepo.Save(ctx, customer); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// FindByID
	loaded, err := customerRepo.FindByID(ctx, customer.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if loaded.Name != "Integration Test Customer" {
		t.Errorf("Name = %s, want 'Integration Test Customer'", loaded.Name)
	}
	if loaded.Email != "integration@test.com" {
		t.Errorf("Email = %s, want 'integration@test.com'", loaded.Email)
	}
	if loaded.Address.Country != "CO" {
		t.Errorf("Address.Country = %s, want CO", loaded.Address.Country)
	}
}

func TestCustomerRepository_FindByTenant(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	customerRepo := repository.NewCustomerRepository(testDB.DB)

	// Create a few customers
	for i := 0; i < 3; i++ {
		customer, _ := entity.NewCustomer(
			generateUUID(100+i),
			TestTenantID,
			"Customer "+intToStr(i),
			"cust"+intToStr(i)+"@test.com",
			valueobject.Address{Line1: "Addr", City: "City", Country: "CO"},
		)
		customerRepo.Save(ctx, customer)
	}

	// List
	customers, err := customerRepo.FindByTenant(ctx, TestTenantID, 100, 0)
	if err != nil {
		t.Fatalf("FindByTenant failed: %v", err)
	}
	// Should include seed customer + 3 new ones
	if len(customers) < 4 {
		t.Errorf("customer count = %d, want >= 4", len(customers))
	}
}

func TestCustomerRepository_SoftDelete(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	customerRepo := repository.NewCustomerRepository(testDB.DB)

	customer, _ := entity.NewCustomer(
		"88888888-8888-8888-8888-888888888888",
		TestTenantID,
		"To Delete",
		"delete@test.com",
		valueobject.Address{Line1: "Addr", City: "City", Country: "CO"},
	)
	customerRepo.Save(ctx, customer)

	// Soft delete
	if err := customerRepo.Delete(ctx, customer.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// FindByID should return ErrNotFound (soft-deleted records are filtered)
	_, err := customerRepo.FindByID(ctx, customer.ID)
	if err == nil {
		t.Error("FindByID should fail for soft-deleted customer")
	}
}

func TestSeedCustomerExists(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)

	customerRepo := repository.NewCustomerRepository(testDB.DB)

	// The seed customer should exist
	customer, err := customerRepo.FindByID(ctx, TestCustomerID)
	if err != nil {
		t.Fatalf("seed customer not found: %v", err)
	}
	if customer.Name != "Cliente Demo S.A.S" {
		t.Errorf("seed customer name = %s, want 'Cliente Demo S.A.S'", customer.Name)
	}
}

var _ = time.Now
