package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

func NewPDFArtifactStoreFromEnv(ctx context.Context) (port.PDFArtifactStore, error) {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("PDF_STORAGE_BACKEND")))
	if backend == "" {
		if environment == "production" {
			return nil, errors.New("PDF_STORAGE_BACKEND must be s3 in production")
		}
		backend = "local"
	}

	switch backend {
	case "local":
		if environment == "production" {
			return nil, errors.New("local PDF storage is not allowed in production")
		}
		storePath := os.Getenv("PDF_STORAGE_PATH")
		if storePath == "" {
			storePath = "/tmp/pdfs"
		}
		return NewLocalPDFStore(storePath)
	case "s3":
		return newS3PDFStoreFromEnv(ctx, environment)
	default:
		return nil, fmt.Errorf("unsupported PDF_STORAGE_BACKEND %q", backend)
	}
}

func newS3PDFStoreFromEnv(ctx context.Context, environment string) (port.PDFArtifactStore, error) {
	endpoint := strings.TrimSpace(os.Getenv("PDF_S3_ENDPOINT"))
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, errors.New("PDF_S3_ENDPOINT must be an absolute HTTP(S) URL")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, errors.New("PDF_S3_ENDPOINT must not contain credentials, query, or fragment")
		}
		if environment == "production" && parsed.Scheme != "https" {
			return nil, errors.New("PDF_S3_ENDPOINT must use HTTPS in production")
		}
	}

	bucket := strings.TrimSpace(os.Getenv("PDF_S3_BUCKET"))
	if bucket == "" {
		return nil, errors.New("PDF_S3_BUCKET is required when PDF_STORAGE_BACKEND=s3")
	}
	region := strings.TrimSpace(os.Getenv("PDF_S3_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = "us-east-1"
	}

	forcePathStyle := endpoint != ""
	if raw := strings.TrimSpace(os.Getenv("PDF_S3_FORCE_PATH_STYLE")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid PDF_S3_FORCE_PATH_STYLE: %w", err)
		}
		forcePathStyle = parsed
	}

	return NewS3PDFStore(ctx, S3Config{
		Bucket:          bucket,
		Region:          region,
		Endpoint:        endpoint,
		ForcePathStyle:  forcePathStyle,
		AccessKeyID:     strings.TrimSpace(os.Getenv("PDF_S3_ACCESS_KEY_ID")),
		SecretAccessKey: os.Getenv("PDF_S3_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("PDF_S3_SESSION_TOKEN"),
	})
}
