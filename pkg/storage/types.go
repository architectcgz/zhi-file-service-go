package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

type Provider string

const (
	ProviderS3    Provider = "S3"
	ProviderMinIO Provider = "MINIO"
)

type AccessLevel string

const (
	AccessLevelPublic  AccessLevel = "PUBLIC"
	AccessLevelPrivate AccessLevel = "PRIVATE"
)

type BucketRef struct {
	Provider   Provider
	BucketName string
	PublicBase string
}

func (r BucketRef) Validate() error {
	if r.Provider == "" {
		return fmt.Errorf("%w: provider is required", ErrInvalidBucketConfig)
	}
	if r.BucketName == "" {
		return fmt.Errorf("%w: bucket name is required", ErrInvalidBucketConfig)
	}

	return nil
}

type ObjectRef struct {
	Provider   Provider
	BucketName string
	ObjectKey  string
}

func (r ObjectRef) Validate() error {
	if r.Provider == "" {
		return fmt.Errorf("%w: provider is required", ErrInvalidBucketConfig)
	}
	if r.BucketName == "" {
		return fmt.Errorf("%w: bucket name is required", ErrInvalidBucketConfig)
	}
	if r.ObjectKey == "" {
		return fmt.Errorf("%w: object key is required", ErrInvalidBucketConfig)
	}

	return nil
}

type UploadedPart struct {
	PartNumber int
	ETag       string
	SizeBytes  int64
	Checksum   string
}

type ObjectMetadata struct {
	SizeBytes   int64
	ContentType string
	ETag        string
	Checksum    string
	VersionID   string
}

type BucketResolver interface {
	Resolve(accessLevel AccessLevel) (BucketRef, error)
	Normalize(bucketName string) string
}

type ObjectLocator interface {
	ResolveObjectURL(ref ObjectRef) (string, error)
}

type ObjectReader interface {
	HeadObject(ctx context.Context, ref ObjectRef) (ObjectMetadata, error)
}

type ObjectWriter interface {
	DeleteObject(ctx context.Context, ref ObjectRef) error
	CopyObject(ctx context.Context, source ObjectRef, target ObjectRef) error
}

type InlineObjectWriter interface {
	PutObject(ctx context.Context, ref ObjectRef, contentType string, body io.Reader, size int64) error
}

type MultipartManager interface {
	CreateMultipartUpload(ctx context.Context, ref ObjectRef, contentType string) (string, error)
	UploadPart(ctx context.Context, ref ObjectRef, uploadID string, partNumber int, body io.Reader, size int64) (string, error)
	ListUploadedParts(ctx context.Context, ref ObjectRef, uploadID string) ([]UploadedPart, error)
	CompleteMultipartUpload(ctx context.Context, ref ObjectRef, uploadID string, parts []UploadedPart) error
	AbortMultipartUpload(ctx context.Context, ref ObjectRef, uploadID string) error
}

type PresignManager interface {
	PresignPutObject(ctx context.Context, ref ObjectRef, contentType string, ttl time.Duration) (string, map[string]string, error)
	PresignUploadPart(ctx context.Context, ref ObjectRef, uploadID string, partNumber int, ttl time.Duration) (string, map[string]string, error)
	PresignGetObject(ctx context.Context, ref ObjectRef, ttl time.Duration) (string, error)
}
