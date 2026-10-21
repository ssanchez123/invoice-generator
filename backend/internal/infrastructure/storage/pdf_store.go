package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

const maxPDFSize = 25 << 20

type LocalPDFStore struct {
	root string
}

func NewLocalPDFStore(root string) (*LocalPDFStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("PDF storage path is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve PDF storage path: %w", err)
	}
	return &LocalPDFStore{root: absoluteRoot}, nil
}

func (s *LocalPDFStore) PutInvoicePDF(ctx context.Context, tenantID, invoiceID string, contents []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(contents) == 0 || len(contents) > maxPDFSize {
		return fmt.Errorf("PDF size must be between 1 and %d bytes", maxPDFSize)
	}

	filePath, err := s.filePath(tenantID, invoiceID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return fmt.Errorf("create PDF directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(filePath), ".pdf-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary PDF: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(0640); err != nil {
		file.Close()
		return fmt.Errorf("set PDF permissions: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return fmt.Errorf("write temporary PDF: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync temporary PDF: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary PDF: %w", err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("publish PDF: %w", err)
	}
	return nil
}

func (s *LocalPDFStore) GetInvoicePDF(ctx context.Context, tenantID, invoiceID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	filePath, err := s.filePath(tenantID, invoiceID)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, port.ErrPDFArtifactNotFound
		}
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, maxPDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("read PDF: %w", err)
	}
	if len(contents) > maxPDFSize {
		return nil, fmt.Errorf("PDF exceeds maximum size of %d bytes", maxPDFSize)
	}
	return contents, nil
}

func (s *LocalPDFStore) filePath(tenantID, invoiceID string) (string, error) {
	key, err := artifactKey(tenantID, invoiceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}

func artifactKey(tenantID, invoiceID string) (string, error) {
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return "", fmt.Errorf("invalid tenant ID: %w", err)
	}
	invoiceUUID, err := uuid.Parse(invoiceID)
	if err != nil {
		return "", fmt.Errorf("invalid invoice ID: %w", err)
	}
	return tenantUUID.String() + "/" + invoiceUUID.String() + ".pdf", nil
}

var _ port.PDFArtifactStore = (*LocalPDFStore)(nil)
