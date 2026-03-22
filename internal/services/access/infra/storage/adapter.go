package storageinfra

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const defaultPresignTTL = 2 * time.Minute

type Adapter struct {
	cfg     config.StorageConfig
	client  *s3.Client
	presign *s3.PresignClient
}

func NewAdapter(client *platformstorage.Client, cfg config.StorageConfig) (*Adapter, error) {
	if client == nil || client.Raw() == nil {
		return nil, fmt.Errorf("%w: storage client is nil", pkgstorage.ErrInvalidBucketConfig)
	}

	return &Adapter{
		cfg:     cfg,
		client:  client.Raw(),
		presign: s3.NewPresignClient(client.Raw()),
	}, nil
}

func (a *Adapter) ResolveObjectURL(ref pkgstorage.ObjectRef) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}

	base := strings.TrimSpace(a.cfg.PublicBaseURL)
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(a.cfg.Endpoint), "/") + "/" + strings.TrimSpace(ref.BucketName)
	}
	return joinURL(base, ref.ObjectKey)
}

func (a *Adapter) PresignGetObject(ctx context.Context, ref pkgstorage.ObjectRef, ttl time.Duration) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}

	request, err := a.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(ref.BucketName),
		Key:    aws.String(ref.ObjectKey),
	}, func(options *s3.PresignOptions) {
		options.Expires = normalizePresignTTL(ttl)
	})
	if err != nil {
		return "", mapStorageError(err)
	}

	return request.URL, nil
}

func (a *Adapter) PresignPutObject(context.Context, pkgstorage.ObjectRef, string, time.Duration) (string, map[string]string, error) {
	return "", nil, fmt.Errorf("%w: access-service does not support put presign", pkgstorage.ErrInvalidPresignRequest)
}

func (a *Adapter) PresignUploadPart(context.Context, pkgstorage.ObjectRef, string, int, time.Duration) (string, map[string]string, error) {
	return "", nil, fmt.Errorf("%w: access-service does not support multipart part presign", pkgstorage.ErrInvalidPresignRequest)
}

func normalizePresignTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultPresignTTL
	}
	return ttl
}

func joinURL(base string, objectKey string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("%w: invalid public base url: %v", pkgstorage.ErrInvalidBucketConfig, err)
	}

	trimmedKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if trimmedKey == "" {
		return "", fmt.Errorf("%w: object key is required", pkgstorage.ErrInvalidBucketConfig)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + trimmedKey
	return parsed.String(), nil
}

func mapStorageError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return fmt.Errorf("%w: %v", pkgstorage.ErrObjectNotFound, err)
		}
	}

	return fmt.Errorf("%w: %v", pkgstorage.ErrProviderUnavailable, err)
}

var (
	_ ports.ObjectLocator  = (*Adapter)(nil)
	_ ports.PresignManager = (*Adapter)(nil)
)
