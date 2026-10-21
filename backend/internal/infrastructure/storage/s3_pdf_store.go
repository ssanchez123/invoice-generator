package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
)

type s3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type S3PDFStore struct {
	client s3API
	bucket string
}

type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	ForcePathStyle  bool
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func NewS3PDFStore(ctx context.Context, cfg S3Config) (*S3PDFStore, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("PDF S3 bucket is required")
	}
	if cfg.Region == "" {
		return nil, errors.New("PDF S3 region is required")
	}
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, errors.New("both PDF S3 access key ID and secret access key are required")
	}
	if cfg.Endpoint != "" && cfg.AccessKeyID == "" {
		return nil, errors.New("explicit PDF S3 credentials are required for a custom endpoint")
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" {
		provider := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(provider))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 credentials and configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		options.UsePathStyle = cfg.ForcePathStyle
	})
	return &S3PDFStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3PDFStore) PutInvoicePDF(ctx context.Context, tenantID, invoiceID string, contents []byte) error {
	if len(contents) == 0 || len(contents) > maxPDFSize {
		return fmt.Errorf("PDF size must be between 1 and %d bytes", maxPDFSize)
	}
	key, err := artifactKey(tenantID, invoiceID)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(contents),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		return fmt.Errorf("upload PDF artifact: %w", err)
	}
	return nil
}

func (s *S3PDFStore) GetInvoicePDF(ctx context.Context, tenantID, invoiceID string) ([]byte, error) {
	key, err := artifactKey(tenantID, invoiceID)
	if err != nil {
		return nil, err
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return nil, port.ErrPDFArtifactNotFound
		}
		return nil, fmt.Errorf("download PDF artifact: %w", err)
	}
	defer output.Body.Close()

	contents, err := io.ReadAll(io.LimitReader(output.Body, maxPDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("read PDF artifact: %w", err)
	}
	if len(contents) > maxPDFSize {
		return nil, fmt.Errorf("PDF exceeds maximum size of %d bytes", maxPDFSize)
	}
	return contents, nil
}

var _ port.PDFArtifactStore = (*S3PDFStore)(nil)
