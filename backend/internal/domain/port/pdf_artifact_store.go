package port

import (
	"context"
	"errors"
)

var ErrPDFArtifactNotFound = errors.New("PDF artifact not found")

// PDFArtifactStore stores invoice PDFs under tenant-scoped keys.
type PDFArtifactStore interface {
	PutInvoicePDF(ctx context.Context, tenantID, invoiceID string, contents []byte) error
	GetInvoicePDF(ctx context.Context, tenantID, invoiceID string) ([]byte, error)
}
