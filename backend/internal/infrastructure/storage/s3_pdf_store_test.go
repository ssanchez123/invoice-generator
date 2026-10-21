package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

func TestS3PDFStoreRoundTripAgainstLocalS3Endpoint(t *testing.T) {
	tenantID, invoiceID := uuid.NewString(), uuid.NewString()
	want := []byte("%PDF-local-s3-test")
	keyPath := fmt.Sprintf("/invoice-pdfs/%s/%s.pdf", tenantID, invoiceID)
	putSeen := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=test-access-key/") {
			t.Errorf("S3 request was not signed with configured test credentials")
		}
		switch r.Method {
		case http.MethodPut:
			if r.URL.Path != keyPath {
				t.Errorf("PUT path = %q, want %q", r.URL.Path, keyPath)
			}
			if got := r.Header.Get("Content-Type"); got != "application/pdf" {
				t.Errorf("PUT Content-Type = %q, want application/pdf", got)
			}
			contents, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read PUT body: %v", err)
			}
			if !bytes.Equal(contents, want) {
				t.Errorf("PUT body = %q, want %q", contents, want)
			}
			putSeen <- struct{}{}
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if r.URL.Path == keyPath {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write(want)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>")
		default:
			t.Errorf("unexpected S3 method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	store, err := NewS3PDFStore(context.Background(), S3Config{
		Bucket:          "invoice-pdfs",
		Region:          "us-east-1",
		Endpoint:        server.URL,
		ForcePathStyle:  true,
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
	})
	if err != nil {
		t.Fatalf("create S3 PDF store: %v", err)
	}

	if err := store.PutInvoicePDF(context.Background(), tenantID, invoiceID, want); err != nil {
		t.Fatalf("put PDF: %v", err)
	}
	select {
	case <-putSeen:
	default:
		t.Fatal("S3 endpoint did not receive a PUT")
	}
	got, err := store.GetInvoicePDF(context.Background(), tenantID, invoiceID)
	if err != nil {
		t.Fatalf("get PDF: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := store.GetInvoicePDF(context.Background(), tenantID, uuid.NewString()); !errors.Is(err, port.ErrPDFArtifactNotFound) {
		t.Fatalf("missing artifact error = %v, want %v", err, port.ErrPDFArtifactNotFound)
	}
}

func TestS3StoreRequiresExplicitCredentialsForCustomEndpoint(t *testing.T) {
	_, err := NewS3PDFStore(context.Background(), S3Config{
		Bucket:   "invoice-pdfs",
		Region:   "us-east-1",
		Endpoint: "https://storage.example.test",
	})
	if err == nil {
		t.Fatal("expected custom S3 endpoint without explicit credentials to be rejected")
	}
}
