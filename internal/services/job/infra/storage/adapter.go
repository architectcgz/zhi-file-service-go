package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Adapter struct {
	client   *s3.Client
	provider pkgstorage.Provider
}

func NewAdapter(client *platformstorage.Client, cfg config.StorageConfig) (*Adapter, error) {
	if client == nil || client.Raw() == nil {
		return nil, fmt.Errorf("%w: storage client is nil", pkgstorage.ErrInvalidBucketConfig)
	}

	return &Adapter{
		client:   client.Raw(),
		provider: detectProvider(cfg),
	}, nil
}

func (a *Adapter) DeleteObject(ctx context.Context, ref pkgstorage.ObjectRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}

	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(ref.BucketName),
		Key:    aws.String(ref.ObjectKey),
	})
	return mapStorageError(err)
}

func (a *Adapter) AbortMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string) error {
	if err := ref.Validate(); err != nil {
		return err
	}

	_, err := a.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(ref.BucketName),
		Key:      aws.String(ref.ObjectKey),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
	})
	return mapStorageError(err)
}

func detectProvider(cfg config.StorageConfig) pkgstorage.Provider {
	endpoint := strings.ToLower(strings.TrimSpace(cfg.Endpoint))
	if strings.Contains(endpoint, "minio") {
		return pkgstorage.ProviderMinIO
	}
	return pkgstorage.ProviderS3
}

func mapStorageError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchUpload":
			return fmt.Errorf("%w: %v", pkgstorage.ErrObjectNotFound, err)
		}
	}
	return fmt.Errorf("%w: %v", pkgstorage.ErrProviderUnavailable, err)
}
