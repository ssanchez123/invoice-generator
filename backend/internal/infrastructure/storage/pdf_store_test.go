package storage

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

func TestLocalPDFStoreRoundTripIsTenantScoped(t *testing.T) {
	store, err := NewLocalPDFStore(t.TempDir())
	if err != nil {
		t.Fatalf("create local PDF store: %v", err)
	}
	tenantID, invoiceID := uuid.NewString(), uuid.NewString()
	want := []byte("%PDF-local-test")

	if err := store.PutInvoicePDF(context.Background(), tenantID, invoiceID, want); err != nil {
		t.Fatalf("put PDF: %v", err)
	}
	got, err := store.GetInvoicePDF(context.Background(), tenantID, invoiceID)
	if err != nil {
		t.Fatalf("get PDF: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := store.GetInvoicePDF(context.Background(), uuid.NewString(), invoiceID); !errors.Is(err, port.ErrPDFArtifactNotFound) {
		t.Fatalf("cross-tenant lookup error = %v, want not found", err)
	}
}

func TestLocalPDFStoreRejectsInvalidIDsAndEmptyArtifacts(t *testing.T) {
	store, err := NewLocalPDFStore(t.TempDir())
	if err != nil {
		t.Fatalf("create local PDF store: %v", err)
	}
	if err := store.PutInvoicePDF(context.Background(), "../outside", uuid.NewString(), []byte("pdf")); err == nil {
		t.Fatal("expected invalid tenant ID to be rejected")
	}
	if err := store.PutInvoicePDF(context.Background(), uuid.NewString(), uuid.NewString(), nil); err == nil {
		t.Fatal("expected empty PDF to be rejected")
	}
}

func TestNewPDFArtifactStoreFromEnvRejectsLocalProductionStorage(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PDF_STORAGE_BACKEND", "local")
	if _, err := NewPDFArtifactStoreFromEnv(context.Background()); err == nil {
		t.Fatal("expected local storage to be rejected in production")
	}
}
