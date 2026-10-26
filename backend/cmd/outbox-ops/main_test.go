package main

import (
	"testing"
)

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := run([]string{"retry"}); err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestRunValidatesReplayArgumentsBeforeConnecting(t *testing.T) {
	err := run([]string{
		"retry-failed",
		"--tenant-id", "not-a-uuid",
		"--event-id", "also-not-a-uuid",
		"--consumer", "pdf-service",
		"--reason", "operator replay",
	})
	if err == nil {
		t.Fatal("expected invalid tenant UUID error")
	}
}

func TestValidateOperatorDatabaseURLRequiresTLSInProduction(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
		wantErr     bool
	}{
		{name: "verified TLS", databaseURL: "postgres://user:pass@db.example.com:5432/invoicegen?sslmode=verify-full"},
		{name: "required TLS", databaseURL: "postgres://user:pass@db.example.com:5432/invoicegen?sslmode=require"},
		{name: "TLS disabled", databaseURL: "postgres://user:pass@db.example.com:5432/invoicegen?sslmode=disable", wantErr: true},
		{name: "TLS mode missing", databaseURL: "postgres://user:pass@db.example.com:5432/invoicegen", wantErr: true},
		{name: "invalid URL", databaseURL: "not-a-database-url", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateOperatorDatabaseURL(test.databaseURL, "production")
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateOperatorDatabaseURLAllowsLocalDevelopment(t *testing.T) {
	if err := validateOperatorDatabaseURL("postgres://user:pass@localhost:5432/invoicegen?sslmode=disable", "development"); err != nil {
		t.Fatalf("development URL should be allowed: %v", err)
	}
}
