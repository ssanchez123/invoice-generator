package main

import (
	"testing"

	"github.com/santiagossaa/invoice-generator/internal/domain/valueobject"
)

func TestValidateProvisionInput(t *testing.T) {
	valid := provisionInput{
		TenantName: "  Example SAS  ",
		Address: valueobject.Address{
			Line1:   " Calle 1 ",
			City:    " Bogotá ",
			Country: "co",
		},
		OwnerEmail: "  OWNER@EXAMPLE.COM ",
		FirstName:  " Ana ",
		LastName:   " Pérez ",
	}
	if err := validateProvisionInput(&valid); err != nil {
		t.Fatalf("validateProvisionInput() error = %v", err)
	}
	if valid.OwnerEmail != "owner@example.com" || valid.Address.Country != "CO" {
		t.Fatalf("input was not normalized: %+v", valid)
	}
	if valid.TenantName != "Example SAS" || valid.FirstName != "Ana" {
		t.Fatalf("names were not normalized: %+v", valid)
	}
}

func TestValidateProvisionInputRejectsInvalidEmailAndAddress(t *testing.T) {
	tests := []struct {
		name  string
		input provisionInput
	}{
		{
			name: "invalid email",
			input: provisionInput{
				TenantName: "Example SAS",
				Address:    valueobject.Address{Line1: "Street 1", City: "Bogotá", Country: "CO"},
				OwnerEmail: "not-an-email",
				FirstName:  "Ana",
				LastName:   "Pérez",
			},
		},
		{
			name: "missing address",
			input: provisionInput{
				TenantName: "Example SAS",
				OwnerEmail: "owner@example.com",
				FirstName:  "Ana",
				LastName:   "Pérez",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateProvisionInput(&test.input); err == nil {
				t.Fatal("validateProvisionInput() error = nil, want error")
			}
		})
	}
}

func TestValidateOperatorDatabaseURL(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
		environment string
		wantError   bool
	}{
		{name: "production requires tls", databaseURL: "postgres://operator@db/app?sslmode=disable", environment: "production", wantError: true},
		{name: "production requires verified tls", databaseURL: "postgres://operator@db/app?sslmode=require", environment: "production", wantError: true},
		{name: "production accepts verified tls", databaseURL: "postgres://operator@db/app?sslmode=verify-full", environment: "production"},
		{name: "development allows local ssl mode", databaseURL: "postgres://operator@localhost/app?sslmode=disable", environment: "development"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateOperatorDatabaseURL(test.databaseURL, test.environment)
			if (err != nil) != test.wantError {
				t.Fatalf("validateOperatorDatabaseURL() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
